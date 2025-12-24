package ui

import (
	"sort"

	"github.com/miles-w-3/lobot/internal/graph"
)

const (
	nodeWidth     = 30
	nodeHeight    = 5
	horizontalGap = 8
	verticalGap   = 4
	marginLeft    = 2
	marginTop     = 2
)

// Position represents the position and dimensions of a node in the graph layout
type Position struct {
	X      int
	Y      int
	Width  int
	Height int
}

// LayoutNode represents a node with layer and order information for layout
type LayoutNode struct {
	graphNode *graph.Node
	layer     int
	order     int
}

// GraphLayout manages the Sugiyama-style layered graph layout
type GraphLayout struct {
	layers        [][]LayoutNode
	nodePositions map[*graph.Node]Position
	maxLayerWidth int
	totalHeight   int
}

// NewGraphLayout creates a new graph layout manager
func NewGraphLayout() *GraphLayout {
	return &GraphLayout{
		layers:        make([][]LayoutNode, 0),
		nodePositions: make(map[*graph.Node]Position),
		maxLayerWidth: 0,
		totalHeight:   0,
	}
}

// Calculate computes the complete layout for the given graph
func (l *GraphLayout) Calculate(resourceGraph *graph.ResourceGraph, containerWidth int) {
	// Phase 1: Assign nodes to layers
	l.assignLayers(resourceGraph)

	// Phase 2: Minimize crossings
	l.minimizeCrossings(resourceGraph)

	// Phase 3: Calculate horizontal positions
	l.calculatePositions(containerWidth)
}

// assignLayers assigns each node to a layer using BFS from root nodes
func (l *GraphLayout) assignLayers(resourceGraph *graph.ResourceGraph) {
	visited := make(map[*graph.Node]bool)
	layers := make(map[*graph.Node]int)

	// Find root nodes (nodes with no parents)
	rootNodes := findRootNodes(resourceGraph)

	// BFS to assign layers
	queue := make([]*graph.Node, 0)
	for _, root := range rootNodes {
		layers[root] = 0
		queue = append(queue, root)
	}

	for len(queue) > 0 {
		node := queue[0]
		queue = queue[1:]

		if visited[node] {
			continue
		}
		visited[node] = true

		currentLayer := layers[node]
		children := resourceGraph.GetChildren(node)

		for _, child := range children {
			// Child must be at least one layer below parent
			childLayer := currentLayer + 1
			if existingLayer, exists := layers[child]; exists {
				childLayer = max(childLayer, existingLayer)
			}
			layers[child] = childLayer
			queue = append(queue, child)
		}
	}

	// Convert to layer arrays
	maxLayer := 0
	for _, layer := range layers {
		maxLayer = max(maxLayer, layer)
	}

	l.layers = make([][]LayoutNode, maxLayer+1)
	for node, layer := range layers {
		l.layers[layer] = append(l.layers[layer], LayoutNode{
			graphNode: node,
			layer:     layer,
		})
	}

	// Set initial order within each layer
	for layerIdx := range l.layers {
		for i := range l.layers[layerIdx] {
			l.layers[layerIdx][i].order = i
		}
	}
}

// minimizeCrossings uses the barycenter heuristic to minimize edge crossings
func (l *GraphLayout) minimizeCrossings(resourceGraph *graph.ResourceGraph) {
	// Iterate several times for better results
	for iteration := 0; iteration < 3; iteration++ {
		// Forward pass: order based on parent positions
		for i := 1; i < len(l.layers); i++ {
			l.orderLayerByParents(resourceGraph, i)
		}

		// Backward pass: order based on child positions
		for i := len(l.layers) - 2; i >= 0; i-- {
			l.orderLayerByChildren(resourceGraph, i)
		}
	}
}

// orderLayerByParents orders nodes in a layer based on average parent position
func (l *GraphLayout) orderLayerByParents(resourceGraph *graph.ResourceGraph, layerIdx int) {
	layer := l.layers[layerIdx]

	type nodeWithWeight struct {
		node   LayoutNode
		weight float64
	}

	weighted := make([]nodeWithWeight, len(layer))
	for i, node := range layer {
		parents := resourceGraph.GetParents(node.graphNode)
		if len(parents) == 0 {
			weighted[i] = nodeWithWeight{node, float64(i)}
			continue
		}

		// Calculate barycenter (average parent position)
		sum := 0.0
		for _, parent := range parents {
			sum += float64(l.findNodePosition(parent, layerIdx-1))
		}
		weighted[i] = nodeWithWeight{node, sum / float64(len(parents))}
	}

	// Sort by weight
	sort.Slice(weighted, func(i, j int) bool {
		return weighted[i].weight < weighted[j].weight
	})

	// Update layer with new order
	for i, w := range weighted {
		w.node.order = i
		l.layers[layerIdx][i] = w.node
	}
}

// orderLayerByChildren orders nodes in a layer based on average child position
func (l *GraphLayout) orderLayerByChildren(resourceGraph *graph.ResourceGraph, layerIdx int) {
	layer := l.layers[layerIdx]

	type nodeWithWeight struct {
		node   LayoutNode
		weight float64
	}

	weighted := make([]nodeWithWeight, len(layer))
	for i, node := range layer {
		children := resourceGraph.GetChildren(node.graphNode)
		if len(children) == 0 {
			weighted[i] = nodeWithWeight{node, float64(i)}
			continue
		}

		// Calculate barycenter (average child position)
		sum := 0.0
		for _, child := range children {
			sum += float64(l.findNodePosition(child, layerIdx+1))
		}
		weighted[i] = nodeWithWeight{node, sum / float64(len(children))}
	}

	// Sort by weight
	sort.Slice(weighted, func(i, j int) bool {
		return weighted[i].weight < weighted[j].weight
	})

	// Update layer with new order
	for i, w := range weighted {
		w.node.order = i
		l.layers[layerIdx][i] = w.node
	}
}

// findNodePosition finds the position (order) of a node within its layer
func (l *GraphLayout) findNodePosition(node *graph.Node, layer int) int {
	if layer < 0 || layer >= len(l.layers) {
		return 0
	}

	for i, n := range l.layers[layer] {
		if n.graphNode == node {
			return i
		}
	}
	return 0
}

// calculatePositions calculates X and Y coordinates for all nodes
func (l *GraphLayout) calculatePositions(containerWidth int) {
	l.nodePositions = make(map[*graph.Node]Position)

	for layerIdx, layer := range l.layers {
		layerWidth := len(layer)

		// Y position for this layer
		y := marginTop + (layerIdx * (nodeHeight + verticalGap))

		// Calculate total width needed for this layer
		totalNodesWidth := layerWidth * nodeWidth
		totalGapsWidth := (layerWidth - 1) * horizontalGap
		totalLayerWidth := totalNodesWidth + totalGapsWidth

		// Center layer horizontally if it fits in container
		startX := marginLeft
		if totalLayerWidth < containerWidth {
			startX = (containerWidth - totalLayerWidth) / 2
		}

		// Position each node
		for i, node := range layer {
			x := startX + (i * (nodeWidth + horizontalGap))

			l.nodePositions[node.graphNode] = Position{
				X:      x,
				Y:      y,
				Width:  nodeWidth,
				Height: nodeHeight,
			}
		}

		l.maxLayerWidth = max(l.maxLayerWidth, totalLayerWidth)
	}

	// Calculate total canvas height
	l.totalHeight = marginTop +
		(len(l.layers) * nodeHeight) +
		((len(l.layers) - 1) * verticalGap) +
		marginTop
}
