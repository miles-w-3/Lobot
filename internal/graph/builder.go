package graph

import (
	"log/slog"

	"github.com/miles-w-3/lobot/internal/k8s"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
)

const (
	// MaxDepth defines the maximum depth to traverse in either direction
	MaxDepth = 5 // TODO: User-configurable?
)

// ResourceProvider defines the interface for accessing Kubernetes resources
// This abstraction allows the graph builder to work with any service that can provide resources
type ResourceProvider interface {
	GetResources(gvr schema.GroupVersionResource) []k8s.Resource
	GetResourcesByOwnerUID(uid string) []k8s.Resource
	FetchResource(gvr schema.GroupVersionResource, name, namespace string, expectedUID string) *k8s.Resource
	DiscoverResourceName(gv schema.GroupVersion, kind string) (string, error)
}

// Builder builds resource graphs by discovering relationships
type Builder struct {
	provider  ResourceProvider
	logger    *slog.Logger
	kindToGVR map[string]schema.GroupVersionResource // Cache for Kind -> GVR lookups
}

// NewBuilder creates a new graph builder
func NewBuilder(provider ResourceProvider, logger *slog.Logger) *Builder {
	if logger == nil {
		logger = slog.Default()
	}

	return &Builder{
		provider:  provider,
		logger:    logger,
		kindToGVR: make(map[string]schema.GroupVersionResource),
	}
}

// BuildGraph builds a complete resource graph starting from a root resource
// It traverses both up (to owners) and down (to owned resources)
// Special handling for Helm releases: shows all resources in the release manifest
func (b *Builder) BuildGraph(rootResource *k8s.Resource) *ResourceGraph {
	// Special case: Helm releases
	if rootResource.IsHelmRelease {
		b.logger.Debug("Building graph for Helm release",
			"name", rootResource.Name,
			"namespace", rootResource.Namespace)
		return b.BuildHelmGraph(rootResource)
	}

	graph := NewResourceGraph(rootResource)
	visited := make(map[string]bool)

	b.logger.Debug("Building graph for resource",
		"name", rootResource.Name,
		"namespace", rootResource.Namespace,
		"kind", rootResource.Kind)

	// Traverse upwards to find owners
	b.traverseOwners(graph, graph.Root, visited, 0)

	// Reset visited map for downward traversal (different direction)
	visited = make(map[string]bool)

	// Traverse downwards to find owned resources
	b.traverseOwned(graph, graph.Root, visited, 0)

	b.logger.Debug("Graph built",
		"nodes", len(graph.Nodes),
		"edges", len(graph.Edges))

	return graph
}

// traverseOwners recursively traverses up the ownership chain
func (b *Builder) traverseOwners(graph *ResourceGraph, node *Node, visited map[string]bool, depth int) {
	if depth >= MaxDepth {
		b.logger.Debug("Max depth reached while traversing owners", "depth", depth)
		return
	}

	// Mark this node as visited
	key := graph.getNodeKey(node.Resource)
	if visited[key] {
		return
	}
	visited[key] = true

	// Get owner references from the resource
	if node.Resource.Raw == nil {
		return
	}

	owners := node.Resource.Raw.GetOwnerReferences()

	for _, ownerRef := range owners {
		// Try to find the owner resource
		ownerResource := b.findOwnerResource(node.Resource, ownerRef)
		if ownerResource == nil {
			b.logger.Debug("Owner resource not found",
				"owner", ownerRef.Name,
				"kind", ownerRef.Kind,
				"namespace", node.Resource.Namespace)
			continue
		}

		// Add owner node to graph
		ownerNode := graph.AddNode(ownerResource, RelationshipOwner)

		// Add edge from owner to this resource
		graph.AddEdge(ownerNode, node, EdgeTypeOwns)

		// Recursively traverse this owner's owners
		b.traverseOwners(graph, ownerNode, visited, depth+1)
	}
}

// traverseOwned recursively traverses down to find owned resources
func (b *Builder) traverseOwned(graph *ResourceGraph, node *Node, visited map[string]bool, depth int) {
	if depth >= MaxDepth {
		b.logger.Debug("Max depth reached while traversing owned resources", "depth", depth)
		return
	}

	// Mark this node as visited
	key := graph.getNodeKey(node.Resource) + ":owned"
	if visited[key] {
		return
	}
	visited[key] = true

	if node.Resource.Raw == nil {
		return
	}

	// Use the owner UID index for fast lookup
	ownerUID := string(node.Resource.Raw.GetUID())
	ownedResources := b.provider.GetResourcesByOwnerUID(ownerUID)

	for i := range ownedResources {
		owned := &ownedResources[i]

		// Add owned node to graph
		ownedNode := graph.AddNode(owned, RelationshipOwner)

		// Add edge from this resource to owned resource
		graph.AddEdge(node, ownedNode, EdgeTypeOwns)

		// Recursively traverse this owned resource's owned resources
		b.traverseOwned(graph, ownedNode, visited, depth+1)
	}
}

// findOwnerResource finds an owner resource using the ownerReference metadata
func (b *Builder) findOwnerResource(childResource *k8s.Resource, ownerRef metav1.OwnerReference) *k8s.Resource {
	// Convert ownerRef (apiVersion + kind) to GVR
	gvr, err := b.ownerRefToGVR(ownerRef)
	if err != nil {
		b.logger.Debug("Failed to convert ownerRef to GVR",
			"kind", ownerRef.Kind,
			"apiVersion", ownerRef.APIVersion,
			"error", err)
		return nil
	}

	// First, try to find in cached resources (fast path)
	cachedResources := b.provider.GetResources(gvr)
	for i := range cachedResources {
		res := &cachedResources[i]
		if res.Name == ownerRef.Name &&
			(res.Namespace == childResource.Namespace || res.Namespace == "") &&
			string(res.Raw.GetUID()) == string(ownerRef.UID) {
			return res
		}
	}

	// Not in cache - fetch via API (slow path)
	b.logger.Debug("Owner not in cache, fetching via API",
		"owner", ownerRef.Name,
		"kind", ownerRef.Kind,
		"namespace", childResource.Namespace)

	return b.provider.FetchResource(gvr, ownerRef.Name, childResource.Namespace, string(ownerRef.UID))
}

// ownerRefToGVR converts an ownerReference to a GroupVersionResource
func (b *Builder) ownerRefToGVR(ownerRef metav1.OwnerReference) (schema.GroupVersionResource, error) {
	// Parse group/version from apiVersion
	gv, err := schema.ParseGroupVersion(ownerRef.APIVersion)
	if err != nil {
		return schema.GroupVersionResource{}, err
	}

	// Check cache first
	cacheKey := gv.String() + "/" + ownerRef.Kind
	if gvr, exists := b.kindToGVR[cacheKey]; exists {
		return gvr, nil
	}

	// Use discovery API to find the resource name for this Kind
	resourceName, err := b.discoverResourceName(gv, ownerRef.Kind)
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

// discoverResourceName uses the discovery API to find the resource name for a given Kind
func (b *Builder) discoverResourceName(gv schema.GroupVersion, kind string) (string, error) {
	return b.provider.DiscoverResourceName(gv, kind)
}
