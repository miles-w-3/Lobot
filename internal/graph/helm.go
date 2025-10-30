package graph

import (
	"strings"

	"github.com/miles-w-3/lobot/internal/k8s"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"sigs.k8s.io/yaml"
)

// BuildHelmGraph builds a graph for a Helm release showing all deployed resources
func (b *Builder) BuildHelmGraph(helmRelease *k8s.Resource) *ResourceGraph {
	graph := NewResourceGraph(helmRelease)

	b.logger.Debug("Building Helm release graph",
		"name", helmRelease.Name,
		"namespace", helmRelease.Namespace)

	b.logger.Debug("Helm release manifest length", "length", len(helmRelease.HelmManifest))

	// Parse the Helm manifest to extract individual resource definitions
	manifestResources := b.parseHelmManifest(helmRelease.HelmManifest, helmRelease.Namespace)

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
			missingResource := manifestRes
			missingResource.Status = "Missing"
			missingResource.Kind = manifestRes.Kind + " [Missing]"

			node := graph.AddNode(&missingResource, RelationshipHelm)
			node.Metadata["missing"] = "true"
			graph.AddEdge(graph.Root, node, EdgeTypeHelmPart)
		}
	}

	b.logger.Debug("Helm graph built",
		"nodes", len(graph.Nodes),
		"edges", len(graph.Edges))

	return graph
}

// parseHelmManifest parses a Helm manifest YAML string into individual resources
func (b *Builder) parseHelmManifest(manifest string, defaultNamespace string) []k8s.Resource {
	if manifest == "" {
		return nil
	}

	var resources []k8s.Resource

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

		resource := k8s.Resource{
			Name:       obj.GetName(),
			Namespace:  namespace,
			Kind:       obj.GetKind(),
			APIVersion: obj.GetAPIVersion(),
			Status:     "Unknown",
			Labels:     obj.GetLabels(),
			Raw:        &obj,
		}

		resources = append(resources, resource)
	}

	return resources
}

// findResourceInCluster tries to find a resource in the cluster that matches the manifest
func (b *Builder) findResourceInCluster(manifestResource k8s.Resource) *k8s.Resource {
	// Convert apiVersion + kind to GVR
	gvr, err := b.manifestResourceToGVR(manifestResource)
	if err != nil {
		b.logger.Debug("Could not determine GVR for resource",
			"kind", manifestResource.Kind,
			"apiVersion", manifestResource.APIVersion,
			"error", err)
		return nil
	}

	// Get all cached resources of this type
	cachedResources := b.provider.GetResources(gvr)

	// Find the resource by name and namespace
	for i := range cachedResources {
		res := &cachedResources[i]
		if res.Name == manifestResource.Name &&
			res.Namespace == manifestResource.Namespace {
			return res
		}
	}

	b.logger.Debug("Resource not found in cache",
		"kind", manifestResource.Kind,
		"apiVersion", manifestResource.APIVersion,
		"name", manifestResource.Name,
		"namespace", manifestResource.Namespace)
	// Not found in cache - try fetching via API
	// We won't have the UIDs for these resources
	resource := b.provider.FetchResource(gvr, manifestResource.Name, manifestResource.Namespace, "")

	// TODO: If resource down't have helm annotation, add a warning or note
	if resource != nil {
		b.logger.Debug("Resource found via dynamic client",
			"kind", manifestResource.Kind)
	} else {
		b.logger.Debug("Resource not found via dynamic client", "name", manifestResource.Name)
	}
	return resource
}

// manifestResourceToGVR converts a manifest resource (apiVersion + kind) to GVR
func (b *Builder) manifestResourceToGVR(resource k8s.Resource) (schema.GroupVersionResource, error) {
	// Parse apiVersion into group/version
	gv, err := parseGroupVersion(resource.APIVersion)
	if err != nil {
		return schema.GroupVersionResource{}, err
	}

	// Check cache first
	cacheKey := gv.String() + "/" + resource.Kind
	if gvr, exists := b.kindToGVR[cacheKey]; exists {
		return gvr, nil
	}

	// Use discovery API to find the resource name for this Kind
	schemaGV := schema.GroupVersion{Group: gv.Group, Version: gv.Version}
	resourceName, err := b.discoverResourceName(schemaGV, resource.Kind)
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
