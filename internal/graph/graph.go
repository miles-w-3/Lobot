package graph

import (
	"github.com/miles-w-3/lobot/internal/k8s"
)

// RelationshipType represents the type of relationship between resources
type RelationshipType string

const (
	RelationshipOwner RelationshipType = "owner"  // OwnerReference relationship
	RelationshipHelm  RelationshipType = "helm"   // Helm chart relationship (future)
	RelationshipArgo  RelationshipType = "argocd" // ArgoCD application relationship (future)
)

// EdgeType represents the direction/nature of an edge
type EdgeType string

const (
	EdgeTypeOwns     EdgeType = "owns"       // Parent owns child
	EdgeTypeOwnedBy  EdgeType = "owned-by"   // Child owned by parent
	EdgeTypeHelmPart EdgeType = "helm-part"  // Part of Helm release (future)
	EdgeTypeArgoApp  EdgeType = "argocd-app" // Part of ArgoCD app (future)
)

// Node represents a resource in the graph
type Node struct {
	Resource         k8s.TrackedObject
	RelationshipType RelationshipType
	Metadata         map[string]string
	IsRoot           bool // True if this is the resource that triggered the visualization
}

// Edge represents a relationship between two resources
type Edge struct {
	From *Node
	To   *Node
	Type EdgeType
}

// ResourceGraph represents a graph of related resources
type ResourceGraph struct {
	Nodes   []*Node
	Edges   []*Edge
	Root    *Node            // The resource that triggered the visualization
	nodeMap map[string]*Node // Map for quick lookups: "namespace/name/kind" -> Node
}

// NewResourceGraph creates a new empty resource graph
func NewResourceGraph(rootResource k8s.TrackedObject) *ResourceGraph {
	root := &Node{
		Resource:         rootResource,
		RelationshipType: RelationshipOwner,
		Metadata:         make(map[string]string),
		IsRoot:           true,
	}

	graph := &ResourceGraph{
		Nodes:   []*Node{root},
		Edges:   []*Edge{},
		Root:    root,
		nodeMap: make(map[string]*Node),
	}

	// Add root to node map
	key := graph.getNodeKey(rootResource)
	graph.nodeMap[key] = root

	return graph
}

// AddNode adds a node to the graph if it doesn't already exist
func (g *ResourceGraph) AddNode(resource k8s.TrackedObject, relType RelationshipType) *Node {
	key := g.getNodeKey(resource)

	// Check if node already exists
	if existingNode, exists := g.nodeMap[key]; exists {
		return existingNode
	}

	node := &Node{
		Resource:         resource,
		RelationshipType: relType,
		Metadata:         make(map[string]string),
		IsRoot:           false,
	}

	g.Nodes = append(g.Nodes, node)
	g.nodeMap[key] = node

	return node
}

// AddEdge adds an edge between two nodes
func (g *ResourceGraph) AddEdge(from, to *Node, edgeType EdgeType) {
	// Check if edge already exists
	for _, edge := range g.Edges {
		if edge.From == from && edge.To == to && edge.Type == edgeType {
			return // Edge already exists
		}
	}

	g.Edges = append(g.Edges, &Edge{
		From: from,
		To:   to,
		Type: edgeType,
	})
}

// GetNode retrieves a node by resource key
func (g *ResourceGraph) GetNode(resource k8s.TrackedObject) *Node {
	key := g.getNodeKey(resource)
	return g.nodeMap[key]
}

// GetChildren returns all child nodes of a given node
func (g *ResourceGraph) GetChildren(node *Node) []*Node {
	var children []*Node
	for _, edge := range g.Edges {
		if edge.From == node {
			children = append(children, edge.To)
		}
	}
	return children
}

// GetParents returns all parent nodes of a given node
func (g *ResourceGraph) GetParents(node *Node) []*Node {
	var parents []*Node
	for _, edge := range g.Edges {
		if edge.To == node {
			parents = append(parents, edge.From)
		}
	}
	return parents
}

// getNodeKey generates a unique key for a resource
func (g *ResourceGraph) getNodeKey(resource k8s.TrackedObject) string {
	resourceKind := "Undefined"
	resourceAPIVersion := "Unknown"
	if resource.GetCategory() == k8s.ObjectCategoryHelm {
		resourceKind = "HelmChart"
	} else {
		resourceKind = resource.GetKind()
		if raw := resource.GetRaw(); raw != nil {
			resourceAPIVersion = raw.GetAPIVersion()
		}
	}
	// Use namespace/name/kind/apiVersion as unique identifier
	if resource.GetNamespace() != "" {
		return resource.GetNamespace() + "/" + resource.GetName() + "/" + resourceKind + "/" + resourceAPIVersion
	}

	// For cluster-scoped resources
	return resource.GetName() + "/" + resourceKind + "/" + resourceAPIVersion
}
