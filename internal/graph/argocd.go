package graph

import (
	"github.com/miles-w-3/lobot/internal/k8s"
)

// BuildArgoGraph builds a graph for an ArgoCD Application showing all managed resources
// Uses label-based discovery for first level, then owner references for deeper levels
func (b *Builder) BuildArgoGraph(app k8s.TrackedObject) *ResourceGraph {
	graph := NewResourceGraph(app)

	b.logger.Debug("Building ArgoCD Application graph",
		"name", app.GetName(),
		"namespace", app.GetNamespace())

	// Get all resources across all GVRs
	allResources := b.getAllCachedResources()

	// Filter resources by ArgoCD instance label (first level)
	managedResources := b.filterByArgoLabel(allResources, app.GetName())

	b.logger.Debug("Found ArgoCD managed resources",
		"application", app.GetName(),
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

// getAllCachedResources gets all resources from all GVRs in the cache
func (b *Builder) getAllCachedResources() []k8s.TrackedObject {
	var allResources []k8s.TrackedObject

	// Get resources for all common resource types
	// We need to check all GVRs since ArgoCD can manage any resource type
	resourceTypes := k8s.DefaultResourceTypes()

	for _, rt := range resourceTypes {
		// Skip pseudo-resources (Helm, ArgoCD Applications themselves)
		if rt.DisplayName == "Helm Releases" || rt.DisplayName == "ArgoCD Applications" {
			continue
		}

		resources := b.provider.GetResources(rt.GVR)
		allResources = append(allResources, resources...)
	}

	return allResources
}

// filterByArgoLabel filters resources by the ArgoCD instance label
// Returns resources with label: argocd.argoproj.io/instance=<applicationName>
func (b *Builder) filterByArgoLabel(resources []k8s.TrackedObject, applicationName string) []k8s.TrackedObject {
	var filtered []k8s.TrackedObject

	for i := range resources {
		resource := resources[i]

		raw := resource.GetRaw()
		if raw == nil {
			continue
		}
		
		labels := raw.GetLabels()
		if labels == nil {
			continue
		}

		instanceLabel, exists := labels["argocd.argoproj.io/instance"]
		if !exists {
			continue
		}

		// Check if the label matches the Application name
		if instanceLabel == applicationName {
			filtered = append(filtered, resource)
		}
	}

	return filtered
}
