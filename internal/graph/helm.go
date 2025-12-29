package graph

import (
	"fmt"
	"strings"

	"github.com/miles-w-3/lobot/internal/k8s"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"sigs.k8s.io/yaml"
)

// BuildHelmGraph builds a graph for a Helm release showing all deployed resources
func (b *Builder) BuildHelmGraph(helmRelease k8s.TrackedObject) *ResourceGraph {
	graph := NewResourceGraph(helmRelease)
	
	// Type assert to HelmRelease to access specific fields
	release, ok := helmRelease.(*k8s.HelmRelease)
	if !ok {
		b.logger.Error("Failed to cast TrackedObject to HelmRelease", "type", fmt.Sprintf("%T", helmRelease))
		return graph
	}

	b.logger.Debug("Building Helm release graph",
		"name", helmRelease.GetName(),
		"namespace", helmRelease.GetNamespace())

	b.logger.Debug("Helm release manifest length", "length", len(release.HelmManifest))

	// Parse the Helm manifest to extract individual resource definitions
	manifestResources := b.parseHelmManifest(release.HelmManifest, helmRelease.GetNamespace())

	b.logger.Debug("Parsed Helm manifest",
		"resources", len(manifestResources))

	// For each resource in the manifest, try to find it in the cluster
	for _, manifestRes := range manifestResources {
		// Try to find the actual resource in the cluster
		actualResource := b.findResourceInCluster(manifestRes)

		if actualResource != nil {
			// Resource exists in cluster - add it to graph
			node := graph.AddNode(actualResource, RelationshipHelm)
			graph.AddEdge(graph.Root, node, EdgeTypeHelmPart)

			// Now traverse its ownership chain
			visited := make(map[string]bool)
			b.traverseOwned(graph, node, visited, 0)
		} else {
			// Resource is missing from cluster - create a "missing" node
			// We need to modify the resource to mark it as missing
			// Since manifestRes is an interface holding a pointer, we can type assert and modify
			if k8sRes, ok := manifestRes.(*k8s.K8sResource); ok {
				k8sRes.Status = "Missing"
				k8sRes.Kind = k8sRes.Kind + " [Missing]"
				
				node := graph.AddNode(k8sRes, RelationshipHelm)
				node.Metadata["missing"] = "true"
				graph.AddEdge(graph.Root, node, EdgeTypeHelmPart)
			}
		}
	}

	b.logger.Debug("Helm graph built",
		"nodes", len(graph.Nodes),
		"edges", len(graph.Edges))

	return graph
}

// parseHelmManifest parses a Helm manifest YAML string into individual resources
func (b *Builder) parseHelmManifest(manifest string, defaultNamespace string) []k8s.TrackedObject {
	if manifest == "" {
		return nil
	}

	var resources []k8s.TrackedObject

	// Split by "---" (YAML document separator)
	documents := strings.SplitSeq(manifest, "\n---\n")

	for doc := range documents {
		doc = strings.TrimSpace(doc)
		if doc == "" {
			continue
		}

		// Parse each document as an unstructured object
		var obj unstructured.Unstructured
		if err := yaml.Unmarshal([]byte(doc), &obj); err != nil {
			b.logger.Debug("Failed to parse manifest document", "error", err)
			continue
		}

		// Skip if it's not a valid K8s resource
		if obj.GetKind() == "" || obj.GetAPIVersion() == "" {
			continue
		}

		// Extract resource info
		namespace := obj.GetNamespace()
		if namespace == "" {
			namespace = defaultNamespace
		}

		gvr, err := b.manifestResourceToGVR(obj)
		if err != nil {
			continue
		}

		resource := k8s.ConvertUnstructuredToTrackedObject(&obj, gvr)

		resources = append(resources, resource)
	}

	return resources
}

// findResourceInCluster tries to find a resource in the cluster that matches the manifest
func (b *Builder) findResourceInCluster(manifestResource k8s.TrackedObject) k8s.TrackedObject {
	// Get GVR from the manifest resource
	// K8sResource has GVR field
	var gvr schema.GroupVersionResource
	var kind, apiVersion string
	
	if k8sRes, ok := manifestResource.(*k8s.K8sResource); ok {
		gvr = k8sRes.GVR
		kind = k8sRes.Kind
		apiVersion = k8sRes.APIVersion
	} else {
		return nil
	}



	// Get all cached resources of this type
	cachedResources := b.provider.GetResources(gvr)

	// Find the resource by name and namespace
	for i := range cachedResources {
		res := cachedResources[i]
		if res.GetName() == manifestResource.GetName() &&
			res.GetNamespace() == manifestResource.GetNamespace() {
			return res
		}
	}

	b.logger.Debug("Resource not found in cache",
		"kind", kind,
		"apiVersion", apiVersion,
		"name", manifestResource.GetName(),
		"namespace", manifestResource.GetNamespace())
	// Not found in cache - try fetching via API
	// We won't have the UIDs for these resources
	resource := b.provider.FetchResource(gvr, manifestResource.GetName(), manifestResource.GetNamespace(), "")

	// TODO: If resource down't have helm annotation, add a warning or note
	if resource != nil {
		b.logger.Debug("Resource found via dynamic client",
			"kind", kind)
	} else {
		b.logger.Debug("Resource not found via dynamic client", "name", manifestResource.GetName())
	}
	return resource
}

// manifestResourceToGVR converts a manifest resource (apiVersion + kind) to GVR
func (b *Builder) manifestResourceToGVR(resource unstructured.Unstructured) (schema.GroupVersionResource, error) {
	// Parse apiVersion into group/version
	gv, err := parseGroupVersion(resource.GetAPIVersion())
	if err != nil {
		return schema.GroupVersionResource{}, err
	}

	// Check cache first
	cacheKey := gv.String() + "/" + resource.GetKind()
	if gvr, exists := b.kindToGVR[cacheKey]; exists {
		return gvr, nil
	}

	// Use discovery API to find the resource name for this Kind
	schemaGV := schema.GroupVersion{Group: gv.Group, Version: gv.Version}
	resourceName, err := b.discoverResourceName(schemaGV, resource.GetKind())
	if err != nil {
		return schema.GroupVersionResource{}, err
	}

	gvr := schema.GroupVersionResource{
		Group:    gv.Group,
		Version:  gv.Version,
		Resource: resourceName,
	}

	// Cache the result
	b.kindToGVR[cacheKey] = gvr

	return gvr, nil
}

// parseGroupVersion is a helper to parse apiVersion strings
func parseGroupVersion(apiVersion string) (GroupVersion, error) {
	parts := strings.Split(apiVersion, "/")
	if len(parts) == 1 {
		// Core API (e.g., "v1")
		return GroupVersion{Group: "", Version: parts[0]}, nil
	} else if len(parts) == 2 {
		// Named API group (e.g., "apps/v1")
		return GroupVersion{Group: parts[0], Version: parts[1]}, nil
	}
	return GroupVersion{}, nil
}

// GroupVersion represents a Kubernetes API group and version
type GroupVersion struct {
	Group   string
	Version string
}

// String returns the string representation of a GroupVersion
func (gv GroupVersion) String() string {
	if gv.Group == "" {
		return gv.Version
	}
	return gv.Group + "/" + gv.Version
}
