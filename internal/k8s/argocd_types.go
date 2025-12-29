package k8s

import (
	"fmt"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// Application represents a minimal ArgoCD Application CRD.
// Only includes the fields needed to extract resource status.
type Application struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata"`
	Status            ApplicationStatus `json:"status,omitempty"`
}

// ApplicationStatus contains the status of an ArgoCD Application.
type ApplicationStatus struct {
	// Resources is a list of Kubernetes resources managed by this application
	Resources []ResourceStatus `json:"resources,omitempty"`
}

// ResourceStatus holds the current synchronization and health status of a Kubernetes resource.
type ResourceStatus struct {
	// Group represents the API group of the resource (e.g., "apps" for Deployments)
	Group string `json:"group,omitempty"`

	// Version indicates the API version of the resource (e.g., "v1", "v1beta1")
	Version string `json:"version,omitempty"`

	// Kind specifies the type of the resource (e.g., "Deployment", "Service")
	Kind string `json:"kind"`

	// Namespace defines the Kubernetes namespace where the resource is located
	Namespace string `json:"namespace,omitempty"`

	// Name is the unique name of the resource within the namespace
	Name string `json:"name"`

	// Status represents the synchronization state of the resource
	Status SyncStatusCode `json:"status,omitempty"`

	// Health indicates the health status of the resource
	Health *HealthStatus `json:"health,omitempty"`

	// Hook is true if the resource is used as a lifecycle hook
	Hook bool `json:"hook,omitempty"`

	// RequiresPruning is true if the resource needs to be pruned (deleted)
	RequiresPruning bool `json:"requiresPruning,omitempty"`

	// RequiresDeletionConfirmation is true if the resource requires explicit user confirmation before deletion
	RequiresDeletionConfirmation bool `json:"requiresDeletionConfirmation,omitempty"`

	// SyncWave determines the order in which resources are applied during a sync operation
	// Lower values are applied first
	SyncWave int64 `json:"syncWave,omitempty"`
}

// HealthStatus represents the health of a Kubernetes resource.
type HealthStatus struct {
	// Status holds the health status code
	Status HealthStatusCode `json:"status,omitempty"`

	// Message is a human-readable informational message describing the health status
	Message string `json:"message,omitempty"`

	// LastTransitionTime is deprecated and not used by ArgoCD
	// Kept for compatibility with the CRD
	LastTransitionTime *metav1.Time `json:"lastTransitionTime,omitempty"`
}

// SyncStatusCode represents the synchronization status of a resource.
type SyncStatusCode string

const (
	// SyncStatusCodeSynced indicates the resource is in sync with the desired state
	SyncStatusCodeSynced SyncStatusCode = "Synced"

	// SyncStatusCodeOutOfSync indicates the resource is not in sync with the desired state
	SyncStatusCodeOutOfSync SyncStatusCode = "OutOfSync"

	// SyncStatusCodeUnknown indicates the sync status cannot be determined
	SyncStatusCodeUnknown SyncStatusCode = "Unknown"
)

// IsValid returns true if the sync status code is one of the recognized values.
func (s SyncStatusCode) IsValid() bool {
	switch s {
	case SyncStatusCodeSynced, SyncStatusCodeOutOfSync, SyncStatusCodeUnknown:
		return true
	default:
		return false
	}
}

// String returns the string representation of the sync status.
func (s SyncStatusCode) String() string {
	return string(s)
}

// HealthStatusCode represents the health status of a resource.
type HealthStatusCode string

const (
	// HealthStatusHealthy indicates the resource is 100% healthy
	HealthStatusHealthy HealthStatusCode = "Healthy"

	// HealthStatusProgressing indicates the resource is not healthy but may recover
	HealthStatusProgressing HealthStatusCode = "Progressing"

	// HealthStatusDegraded indicates the resource has encountered a failure
	HealthStatusDegraded HealthStatusCode = "Degraded"

	// HealthStatusMissing indicates the resource does not exist in the cluster
	HealthStatusMissing HealthStatusCode = "Missing"

	// HealthStatusSuspended indicates the resource is paused or in a suspended state
	HealthStatusSuspended HealthStatusCode = "Suspended"

	// HealthStatusUnknown indicates the health assessment failed
	HealthStatusUnknown HealthStatusCode = "Unknown"
)

// IsValid returns true if the health status code is one of the recognized values.
func (h HealthStatusCode) IsValid() bool {
	switch h {
	case HealthStatusHealthy, HealthStatusProgressing, HealthStatusDegraded,
		HealthStatusMissing, HealthStatusSuspended, HealthStatusUnknown:
		return true
	default:
		return false
	}
}

// String returns the string representation of the health status.
func (h HealthStatusCode) String() string {
	return string(h)
}

// IsHealthy returns true if the status is Healthy.
func (h HealthStatusCode) IsHealthy() bool {
	return h == HealthStatusHealthy
}

// IsDegraded returns true if the status is Degraded.
func (h HealthStatusCode) IsDegraded() bool {
	return h == HealthStatusDegraded
}

// IsProgressing returns true if the status is Progressing.
func (h HealthStatusCode) IsProgressing() bool {
	return h == HealthStatusProgressing
}

// Priority returns the priority of the health status for aggregation.
// Lower values indicate worse health. This matches ArgoCD's aggregation logic:
// Healthy > Suspended > Progressing > Missing > Degraded > Unknown
func (h HealthStatusCode) Priority() int {
	switch h {
	case HealthStatusHealthy:
		return 6
	case HealthStatusSuspended:
		return 5
	case HealthStatusProgressing:
		return 4
	case HealthStatusMissing:
		return 3
	case HealthStatusDegraded:
		return 2
	case HealthStatusUnknown:
		return 1
	default:
		return 0
	}
}

// Validate validates the ResourceStatus fields.
func (r *ResourceStatus) Validate() error {
	if r.Kind == "" {
		return fmt.Errorf("kind is required")
	}
	if r.Name == "" {
		return fmt.Errorf("name is required")
	}
	if r.Status != "" && !r.Status.IsValid() {
		return fmt.Errorf("invalid sync status: %s", r.Status)
	}
	if r.Health != nil && r.Health.Status != "" && !r.Health.Status.IsValid() {
		return fmt.Errorf("invalid health status: %s", r.Health.Status)
	}
	return nil
}

// GroupVersionKind returns a string representation of the resource's GVK.
func (r *ResourceStatus) GroupVersionKind() string {
	if r.Group == "" {
		return fmt.Sprintf("%s/%s", r.Version, r.Kind)
	}
	return fmt.Sprintf("%s/%s/%s", r.Group, r.Version, r.Kind)
}

// QualifiedName returns the namespaced name of the resource.
func (r *ResourceStatus) QualifiedName() string {
	if r.Namespace == "" {
		return r.Name
	}
	return fmt.Sprintf("%s/%s", r.Namespace, r.Name)
}

// IsNamespaced returns true if the resource has a namespace.
func (r *ResourceStatus) IsNamespaced() bool {
	return r.Namespace != ""
}

// IsSynced returns true if the resource sync status is Synced.
func (r *ResourceStatus) IsSynced() bool {
	return r.Status == SyncStatusCodeSynced
}

// IsHealthy returns true if the resource health status is Healthy.
func (r *ResourceStatus) IsHealthy() bool {
	return r.Health != nil && r.Health.Status.IsHealthy()
}

// GetHealthStatus returns the health status or Unknown if not set.
func (r *ResourceStatus) GetHealthStatus() HealthStatusCode {
	if r.Health == nil {
		return HealthStatusUnknown
	}
	if r.Health.Status == "" {
		return HealthStatusUnknown
	}
	return r.Health.Status
}

// GetSyncStatus returns the sync status or Unknown if not set.
func (r *ResourceStatus) GetSyncStatus() SyncStatusCode {
	if r.Status == "" {
		return SyncStatusCodeUnknown
	}
	return r.Status
}

// AggregateHealth determines the worst health status among multiple resources.
// This matches ArgoCD's aggregation logic where the worst health takes precedence.
func AggregateHealth(resources []ResourceStatus) HealthStatusCode {
	if len(resources) == 0 {
		return HealthStatusUnknown
	}

	worst := HealthStatusHealthy
	for _, r := range resources {
		status := r.GetHealthStatus()
		if status.Priority() < worst.Priority() {
			worst = status
		}
	}

	return worst
}

// FilterByHealth returns resources matching the specified health status.
func FilterByHealth(resources []ResourceStatus, status HealthStatusCode) []ResourceStatus {
	var filtered []ResourceStatus
	for _, r := range resources {
		if r.GetHealthStatus() == status {
			filtered = append(filtered, r)
		}
	}
	return filtered
}

// FilterBySyncStatus returns resources matching the specified sync status.
func FilterBySyncStatus(resources []ResourceStatus, status SyncStatusCode) []ResourceStatus {
	var filtered []ResourceStatus
	for _, r := range resources {
		if r.GetSyncStatus() == status {
			filtered = append(filtered, r)
		}
	}
	return filtered
}

// GroupByKind groups resources by their Kind.
func GroupByKind(resources []ResourceStatus) map[string][]ResourceStatus {
	groups := make(map[string][]ResourceStatus)
	for _, r := range resources {
		groups[r.Kind] = append(groups[r.Kind], r)
	}
	return groups
}

// GroupByNamespace groups resources by their Namespace.
func GroupByNamespace(resources []ResourceStatus) map[string][]ResourceStatus {
	groups := make(map[string][]ResourceStatus)
	for _, r := range resources {
		ns := r.Namespace
		if ns == "" {
			ns = "<cluster-scoped>"
		}
		groups[ns] = append(groups[ns], r)
	}
	return groups
}
