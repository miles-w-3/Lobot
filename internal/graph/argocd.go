package graph

import (
	"github.com/miles-w-3/lobot/internal/k8s"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
)

// BuildArgoGraph builds a graph for an ArgoCD Application showing all managed resources
// Uses the ArgoCD Application's .status.resources field for discovery
func (b *Builder) BuildArgoGraph(app k8s.TrackedObject) *ResourceGraph {
	graph := NewResourceGraph(app)

	b.logger.Debug("Building ArgoCD Application graph",
		"name", app.GetName(),
		"namespace", app.GetNamespace())

	argoApp, ok := app.(*k8s.ArgoCDApp)
	if !ok || argoApp.GetRaw() == nil {
		b.logger.Debug("Failed to get raw ArgoCD Application object")
		return graph
	}

	obj := argoApp.GetRaw()

	status, found, err := unstructured.NestedFieldCopy(obj.Object, "status")
	if !found || err != nil {
		b.logger.Debug("ArgoCD Application has no status field or error reading it", "error", err)
		return graph
	}

	statusMap, ok := status.(map[string]interface{})
	if !ok {
		b.logger.Debug("ArgoCD Application status is not a map")
		return graph
	}

	var appStatus k8s.ApplicationStatus
	if err := runtime.DefaultUnstructuredConverter.FromUnstructured(statusMap, &appStatus); err != nil {
		b.logger.Debug("Failed to unmarshal ArgoCD Application status", "error", err)
		return graph
	}

	if len(appStatus.Resources) == 0 {
		b.logger.Debug("ArgoCD Application has no resources in status", "name", app.GetName())
		return graph
	}

	b.logger.Debug("Found resources in ArgoCD Application status",
		"application", app.GetName(),
		"count", len(appStatus.Resources))

	visited := make(map[string]bool)
	visited[string(obj.GetUID())] = true

	for _, resourceStatus := range appStatus.Resources {
		gv := schema.GroupVersion{
			Group:   resourceStatus.Group,
			Version: resourceStatus.Version,
		}

		cacheKey := gv.String() + "/" + resourceStatus.Kind
		var gvr schema.GroupVersionResource

		if cachedGVR, exists := b.kindToGVR[cacheKey]; exists {
			gvr = cachedGVR
		} else {
			resourceName, err := b.discoverResourceName(gv, resourceStatus.Kind)
			if err != nil {
				b.logger.Debug("Failed to discover resource name",
					"kind", resourceStatus.Kind,
					"gv", gv.String(),
					"error", err)
				continue
			}

			gvr = schema.GroupVersionResource{
				Group:    gv.Group,
				Version:  gv.Version,
				Resource: resourceName,
			}

			b.kindToGVR[cacheKey] = gvr
		}

		var actualResource k8s.TrackedObject

		cachedResources := b.provider.GetResources(gvr)
		for i := range cachedResources {
			res := cachedResources[i]
			if res.GetName() == resourceStatus.Name &&
				res.GetNamespace() == resourceStatus.Namespace {
				actualResource = res
				break
			}
		}

		if actualResource == nil {
			actualResource = b.provider.FetchResource(gvr, resourceStatus.Name, resourceStatus.Namespace, "")
		}

		if actualResource != nil {
			node := graph.AddNode(actualResource, RelationshipArgo)
			graph.AddEdge(graph.Root, node, EdgeTypeArgoApp)
			b.traverseOwned(graph, node, visited, 0)
		} else {
			missingRes := &k8s.K8sResource{
				CoreFields: k8s.CoreFields{
					Name:      resourceStatus.Name,
					Namespace: resourceStatus.Namespace,
					Status:    "Missing",
					Age:       0,
					Raw:       nil,
				},
				APIVersion: resourceStatus.Group + "/" + resourceStatus.Version,
				Kind:       resourceStatus.Kind + " [Missing]",
				GVR:        gvr,
			}

			node := graph.AddNode(missingRes, RelationshipArgo)
			node.Metadata["missing"] = "true"
			graph.AddEdge(graph.Root, node, EdgeTypeArgoApp)
		}
	}

	b.logger.Debug("ArgoCD graph built",
		"nodes", len(graph.Nodes),
		"edges", len(graph.Edges))

	return graph
}
