# Local k3d test environment

The setup creates a disposable two-node K3s cluster and installs resources used
to exercise Lobot's table, graph, manifest, Helm, error-state, and utilization
views.

## Prerequisites

- Docker
- k3d
- kubectl
- Helm

On macOS:

```sh
brew install k3d helm
```

## Create or reconcile the environment

```sh
./scripts/setup-k3d.sh
```

The script is idempotent: it reuses the `lobot-dev` cluster, reapplies the
fixtures, and runs `helm upgrade --install` for the sample release.

Force a clean rebuild when needed:

```sh
RECREATE=1 ./scripts/setup-k3d.sh
```

Useful overrides include `K3D_SERVERS`, `K3D_AGENTS`, `K3S_IMAGE`,
`CLUSTER_TIMEOUT`, and `SKIP_METRICS_WAIT=1`.

The intentionally broken `lobot-broken` Deployment remains in
`ImagePullBackOff` so error-state rendering can be tested.

## Use and remove

```sh
kubectl config use-context k3d-lobot-dev
go run ./cmd/lobot

k3d cluster delete lobot-dev
```
