package modes

import (
	"github.com/miles-w-3/lobot/internal/graph"
	"github.com/miles-w-3/lobot/internal/k8s"
)

// VisualizerRequestedMsg asks RootModel to build a resource graph and open
// the resource visualizer for the selected resource.
type VisualizerRequestedMsg struct {
	Resource k8s.TrackedObject
}

// VisualizerReadyMsg contains the graph prepared for VisualizerScreen.
type VisualizerReadyMsg struct {
	Graph *graph.ResourceGraph
}
