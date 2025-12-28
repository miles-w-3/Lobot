package graph

import (
	"github.com/miles-w-3/lobot/internal/k8s"
)

// BuildArgoGraph builds a graph for an ArgoCD Application showing all managed resources
// Uses API label selector for first level (efficient, server-side indexed), then owner references for deeper levels
func (b *Builder) BuildArgoGraph(app k8s.TrackedObject) *ResourceGraph {
	graph := NewResourceGraph(app)

	b.logger.Debug("Building ArgoCD Application graph",
		"name", app.GetName(),
		"namespace", app.GetNamespace())

	// Use API call with server-side label filtering
	// The argocd.argoproj.io/instance label is indexed by Kubernetes for efficient queries
	labelSelector := "argocd.argoproj.io/instance=" + app.GetName()
	managedResources := b.provider.FetchResourcesByLabel(labelSelector)

	b.logger.Debug("Found ArgoCD managed resources via label selector",
		"application", app.GetName(),
		"selector", labelSelector,
		"count", len(managedResources))

	// For each managed resource, add it to the graph and traverse its ownership chain
	visited := make(map[string]bool)
	visited[string(app.GetRaw().GetUID())] = true

	for i := range managedResources {
		resource := managedResources[i]

		// Skip if already visited (shouldn't happen at this level)
		if visited[string(resource.GetRaw().GetUID())] {
			continue
		}

		// Add the managed resource to graph
		node := graph.AddNode(resource, RelationshipArgo)
		graph.AddEdge(graph.Root, node, EdgeTypeArgoApp)

		// Recursively traverse owned resources (Pods owned by Deployments, etc.)
		b.traverseOwned(graph, node, visited, 0)
	}

	b.logger.Debug("ArgoCD graph built",
		"nodes", len(graph.Nodes),
		"edges", len(graph.Edges))

	return graph
}
