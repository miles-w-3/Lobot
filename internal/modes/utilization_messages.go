package modes

import "github.com/miles-w-3/lobot/internal/k8s"

// UtilizationRequestedMsg asks RootModel to fetch a point-in-time metrics
// snapshot and open or refresh the utilization screen.
type UtilizationRequestedMsg struct{}

// UtilizationReadyMsg contains the result of a metrics snapshot request.
type UtilizationReadyMsg struct {
	NodeMetrics []k8s.NodeMetrics
	PodMetrics  []k8s.PodMetrics
	Error       error
	Unavailable bool
}
