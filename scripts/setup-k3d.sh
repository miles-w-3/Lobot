#!/usr/bin/env bash
set -Eeuo pipefail

CLUSTER_NAME="${CLUSTER_NAME:-lobot-dev}"
CONTEXT_NAME="k3d-${CLUSTER_NAME}"
DEMO_NAMESPACE="lobot-demo"
HELM_NAMESPACE="lobot-helm"
SERVERS="${K3D_SERVERS:-1}"
AGENTS="${K3D_AGENTS:-1}"
CLUSTER_TIMEOUT="${CLUSTER_TIMEOUT:-180s}"
K3S_IMAGE="${K3S_IMAGE:-rancher/k3s:v1.35.5-k3s1}"
METRICS_ATTEMPTS="${METRICS_ATTEMPTS:-60}"
RECREATE="${RECREATE:-0}"
SKIP_METRICS_WAIT="${SKIP_METRICS_WAIT:-0}"

SCRIPT_DIR="$(cd -- "$(dirname -- "${BASH_SOURCE[0]}")" && pwd)"
REPO_ROOT="$(cd -- "${SCRIPT_DIR}/.." && pwd)"
FIXTURE_FILE="${REPO_ROOT}/testdata/k3d/workloads.yaml"
CHART_DIR="${REPO_ROOT}/testdata/k3d/chart"

log() {
  printf '[lobot-k3d] %s\n' "$*"
}

fail() {
  printf '[lobot-k3d] error: %s\n' "$*" >&2
  exit 1
}

require_command() {
  command -v "$1" >/dev/null 2>&1 || fail "required command not found: $1"
}

cluster_exists() {
  k3d cluster list -o json | grep -Eq "\"name\"[[:space:]]*:[[:space:]]*\"${CLUSTER_NAME}\""
}

wait_for_metrics() {
  local attempt
  for ((attempt = 1; attempt <= METRICS_ATTEMPTS; attempt++)); do
    if kubectl --context "${CONTEXT_NAME}" top nodes >/dev/null 2>&1; then
      log "metrics API is ready"
      return 0
    fi
    sleep 2
  done

  kubectl --context "${CONTEXT_NAME}" -n kube-system get deployment metrics-server >&2 || true
  fail "metrics API did not become ready after $((METRICS_ATTEMPTS * 2)) seconds"
}

require_command docker
require_command k3d
require_command kubectl
require_command helm

[[ -f "${FIXTURE_FILE}" ]] || fail "fixture file not found: ${FIXTURE_FILE}"
[[ -f "${CHART_DIR}/Chart.yaml" ]] || fail "Helm chart not found: ${CHART_DIR}"
docker info >/dev/null 2>&1 || fail "Docker is not running"

if [[ "${RECREATE}" == "1" ]] && cluster_exists; then
  log "deleting existing cluster ${CLUSTER_NAME}"
  k3d cluster delete "${CLUSTER_NAME}"
fi

if cluster_exists; then
  log "reusing cluster ${CLUSTER_NAME}"
  if ! k3d cluster start "${CLUSTER_NAME}" >/dev/null 2>&1; then
    kubectl --context "${CONTEXT_NAME}" get --raw=/readyz >/dev/null 2>&1 \
      || fail "existing cluster could not be started"
  fi
else
  log "creating cluster ${CLUSTER_NAME} (${SERVERS} server, ${AGENTS} agent)"
  create_args=(
    cluster create "${CLUSTER_NAME}"
    --servers "${SERVERS}"
    --agents "${AGENTS}"
    --wait
    --timeout "${CLUSTER_TIMEOUT}"
    --image "${K3S_IMAGE}"
    --k3s-arg "--disable=traefik@server:*"
  )
  k3d "${create_args[@]}"
fi

log "selecting kube context ${CONTEXT_NAME}"
kubectl config use-context "${CONTEXT_NAME}" >/dev/null

log "waiting for nodes and bundled metrics-server"
kubectl --context "${CONTEXT_NAME}" wait --for=condition=Ready nodes --all --timeout="${CLUSTER_TIMEOUT}"
kubectl --context "${CONTEXT_NAME}" -n kube-system rollout status deployment/metrics-server --timeout="${CLUSTER_TIMEOUT}"

log "applying Kubernetes fixtures"
kubectl --context "${CONTEXT_NAME}" apply \
  -f "${FIXTURE_FILE}"

log "installing/upgrading local Helm fixture"
helm upgrade --install lobot-sample "${CHART_DIR}" \
  --kube-context "${CONTEXT_NAME}" \
  --namespace "${HELM_NAMESPACE}" \
  --create-namespace \
  --wait \
  --timeout 3m

log "waiting for healthy fixture workloads"
kubectl --context "${CONTEXT_NAME}" -n "${DEMO_NAMESPACE}" rollout status deployment/lobot-web --timeout="${CLUSTER_TIMEOUT}"
kubectl --context "${CONTEXT_NAME}" -n "${DEMO_NAMESPACE}" rollout status statefulset/lobot-stateful --timeout="${CLUSTER_TIMEOUT}"
kubectl --context "${CONTEXT_NAME}" -n "${DEMO_NAMESPACE}" rollout status daemonset/lobot-agent --timeout="${CLUSTER_TIMEOUT}"
kubectl --context "${CONTEXT_NAME}" -n "${DEMO_NAMESPACE}" wait --for=condition=Complete job/lobot-bootstrap --timeout="${CLUSTER_TIMEOUT}"

if [[ "${SKIP_METRICS_WAIT}" != "1" ]]; then
  wait_for_metrics
fi

log "environment ready"
printf '\nContext:   %s\nNamespaces: %s, %s\n\n' "${CONTEXT_NAME}" "${DEMO_NAMESPACE}" "${HELM_NAMESPACE}"
kubectl --context "${CONTEXT_NAME}" get nodes
kubectl --context "${CONTEXT_NAME}" -n "${DEMO_NAMESPACE}" get pods
printf '\nRun Lobot with: go run ./cmd/lobot\n'
printf 'Delete the environment with: k3d cluster delete %s\n' "${CLUSTER_NAME}"
