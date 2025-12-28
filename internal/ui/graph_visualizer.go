package ui

import (
	"fmt"
	"log"
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/help"
	"github.com/charmbracelet/bubbles/key"
	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/miles-w-3/lobot/internal/graph"
	"github.com/miles-w-3/lobot/internal/k8s"
)

// GraphVisualizerModel represents the graph visualization component
type GraphVisualizerModel struct {
	graph           *graph.ResourceGraph
	layout          *GraphLayout
	viewport2d      *Viewport2D    // New 2D viewport for the graph
	detailsViewport viewport.Model // Keep standard viewport for details (vertical only)
	width           int
	height          int
	detailsWidth    int
	canvasWidth     int
	canvasHeight    int
	showDetails     bool
	focusedPanel    FocusPanel
	selectedIndex   int
	flattenedNodes  []*graph.Node
	rootResource    k8s.TrackedObject
	keys            GraphVisualizerKeyMap
	help            help.Model
}

// NewGraphVisualizerModel creates a new graph visualizer
func NewGraphVisualizerModel(resourceGraph *graph.ResourceGraph, width, height int) *GraphVisualizerModel {
	initStart := time.Now()

	detailsWidth := 35
	graphWidth := width - detailsWidth - 2

	// Calculate layout with timing
	layoutStart := time.Now()
	layout := NewGraphLayout()
	layout.Calculate(resourceGraph, graphWidth-4)
	log.Printf("[GraphVisualizer] Layout calculation: duration=%v nodes=%d layers=%d",
		time.Since(layoutStart), len(resourceGraph.Nodes), len(layout.layers))

	// Calculate canvas dimensions - ensure enough space for all content
	canvasWidth := max(layout.maxLayerWidth+2*marginLeft, graphWidth-4)
	canvasHeight := layout.totalHeight
	log.Printf("[GraphVisualizer] Canvas dimensions: width=%d height=%d (estimated memory: %d KB)",
		canvasWidth, canvasHeight, (canvasWidth*canvasHeight*4)/1024)

	// Create viewports
	viewportStart := time.Now()
	viewportHeight := height - 8
	graphViewport := NewViewport2D(graphWidth-4, viewportHeight)
	detailsViewport := viewport.New(detailsWidth-4, viewportHeight)
	log.Printf("[GraphVisualizer] Viewport creation: duration=%v", time.Since(viewportStart))

	// Flatten nodes for navigation (layer by layer, left to right)
	flattenedNodes := flattenGraphForNavigation(layout)

	model := &GraphVisualizerModel{
		graph:           resourceGraph,
		layout:          layout,
		viewport2d:      graphViewport,
		detailsViewport: detailsViewport,
		width:           width,
		height:          height,
		detailsWidth:    detailsWidth,
		canvasWidth:     canvasWidth,
		canvasHeight:    canvasHeight,
		showDetails:     false,
		focusedPanel:    FocusTree,
		selectedIndex:   0,
		flattenedNodes:  flattenedNodes,
		rootResource:    resourceGraph.Root.Resource,
		keys:            DefaultGraphVisualizerKeyMap(),
		help:            help.New(),
	}

	contentStart := time.Now()
	model.updateViewportContent()
	log.Printf("[GraphVisualizer] Content rendering: duration=%v", time.Since(contentStart))

	log.Printf("[GraphVisualizer] Total initialization: duration=%v", time.Since(initStart))
	return model
}

// flattenGraphForNavigation creates a linear array of nodes for navigation
func flattenGraphForNavigation(layout *GraphLayout) []*graph.Node {
	nodes := make([]*graph.Node, 0)
	for _, layer := range layout.layers {
		for _, layoutNode := range layer {
			nodes = append(nodes, layoutNode.graphNode)
		}
	}
	return nodes
}

// updateViewportContent renders the graph and updates the viewport
func (m *GraphVisualizerModel) updateViewportContent() {
	// 1. Create background canvas with edge lines
	canvas := NewCanvas(m.canvasWidth, m.canvasHeight)

	for _, edge := range m.graph.Edges {
		fromPos, fromExists := m.layout.nodePositions[edge.From]
		toPos, toExists := m.layout.nodePositions[edge.To]
		if fromExists && toExists {
			canvas.DrawEdge(fromPos, toPos)
		}
	}

	// 2. Get canvas lines as the base
	lines := canvas.Lines()

	// 3. Overlay styled boxes onto canvas lines
	for i, node := range m.flattenedNodes {
		pos, exists := m.layout.nodePositions[node]
		if !exists {
			continue
		}

		selected := i == m.selectedIndex
		box := m.renderNodeBox(node, selected)
		boxLines := strings.Split(box, "\n")

		// Overlay each line of the box
		for dy, boxLine := range boxLines {
			y := pos.Y + dy
			if y >= 0 && y < len(lines) {
				lines[y] = OverlayStyledContent(lines[y], boxLine, pos.X)
			}
		}
	}

	// 4. Set the full content on the 2D viewport
	m.viewport2d.SetLines(lines)

	// 5. Update details panel for selected node
	if m.selectedIndex >= 0 && m.selectedIndex < len(m.flattenedNodes) {
		m.updateDetailsPanel(m.graph.Root)
	}
}

// renderNodeBox renders a single node as a Lipgloss box
func (m *GraphVisualizerModel) renderNodeBox(node *graph.Node, selected bool) string {
	res := node.Resource

	// Determine border color from status
	var borderColor lipgloss.Color
	status := strings.ToLower(res.GetStatus())
	if strings.Contains(status, "running") ||
		strings.Contains(status, "ready") ||
		strings.Contains(status, "active") ||
		strings.Contains(status, "deployed") {
		borderColor = ColorSuccess
	} else if strings.Contains(status, "pending") ||
		strings.Contains(status, "creating") {
		borderColor = ColorWarning
	} else if strings.Contains(status, "failed") ||
		strings.Contains(status, "error") {
		borderColor = ColorDanger
	} else {
		borderColor = ColorMuted
	}

	// Box style
	boxStyle := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(borderColor).
		Width(nodeWidth-2).
		Height(nodeHeight-2).
		Padding(0).
		Align(lipgloss.Center, lipgloss.Top)

	if selected {
		boxStyle = boxStyle.
			BorderForeground(ColorPrimary).
			BorderStyle(lipgloss.ThickBorder())
	}

	// Content (3 lines: kind, name, status)
	kindStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color(getColorForKind(res.GetKind()))).
		Bold(true)

	name := res.GetName()
	if len(name) > nodeWidth-4 {
		name = name[:nodeWidth-7] + "..."
	}

	statusStyle := lipgloss.NewStyle().Foreground(borderColor)

	content := lipgloss.JoinVertical(
		lipgloss.Center,
		kindStyle.Render(res.GetKind()),
		name,
		statusStyle.Render(res.GetStatus()),
	)

	return boxStyle.Render(content)
}

// updateDetailsPanel updates the details viewport for the selected node
func (m *GraphVisualizerModel) updateDetailsPanel(node *graph.Node) {
	if node == nil {
		return
	}

	res := node.Resource

	var details strings.Builder
	details.WriteString(lipgloss.NewStyle().Bold(true).Render("Resource Details"))
	details.WriteString("\n\n")
	details.WriteString(fmt.Sprintf("Name: %s\n", res.GetName()))
	details.WriteString(fmt.Sprintf("Kind: %s\n", res.GetKind()))
	if res.GetNamespace() != "" {
		details.WriteString(fmt.Sprintf("Namespace: %s\n", res.GetNamespace()))
	}
	details.WriteString(fmt.Sprintf("Status: %s\n", res.GetStatus()))

	// Helm-specific details
	if helmRes, ok := res.(*k8s.HelmRelease); ok && helmRes.HelmRevision > 0 {
		details.WriteString(fmt.Sprintf("Revision: %d\n", helmRes.HelmRevision))
	}

	// ArgoCD-specific details
	if argoApp, ok := res.(*k8s.ArgoCDApp); ok {
		details.WriteString(fmt.Sprintf("Sync Status: %s\n", argoApp.SyncStatus))
		details.WriteString(fmt.Sprintf("Health: %s\n", argoApp.Health))
		if argoApp.SourceRepo != "" {
			details.WriteString(fmt.Sprintf("Repository: %s\n", argoApp.SourceRepo))
		}
		if argoApp.Revision != "" {
			details.WriteString(fmt.Sprintf("Revision: %s\n", argoApp.Revision))
		}
		if argoApp.Destination != "" {
			details.WriteString(fmt.Sprintf("Destination: %s\n", argoApp.Destination))
		}
	}

	// Owner references
	if res.GetRaw() != nil && len(res.GetRaw().GetOwnerReferences()) > 0 {
		details.WriteString("\nOwned by:\n")
		for _, owner := range res.GetRaw().GetOwnerReferences() {
			details.WriteString(fmt.Sprintf("  • %s/%s\n", owner.Kind, owner.Name))
		}
	}

	m.detailsViewport.SetContent(details.String())
}

// Update handles updates for the graph visualizer
func (m GraphVisualizerModel) Update(msg tea.Msg) (GraphVisualizerModel, tea.Cmd) {
	var cmd tea.Cmd

	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch {
		// Arrow keys for node selection
		case key.Matches(msg, m.keys.Up):
			m.navigateUp()
			m.ensureSelectedVisible()
			m.updateViewportContent()
			return m, nil
		case key.Matches(msg, m.keys.Down):
			m.navigateDown()
			m.ensureSelectedVisible()
			m.updateViewportContent()
			return m, nil
		case key.Matches(msg, m.keys.Left):
			m.navigateLeft()
			m.ensureSelectedVisible()
			m.updateViewportContent()
			return m, nil
		case key.Matches(msg, m.keys.Right):
			m.navigateRight()
			m.ensureSelectedVisible()
			m.updateViewportContent()
			return m, nil

		// i/j/k/l for canvas panning
		case key.Matches(msg, m.keys.PanUp):
			m.viewport2d.ScrollUp(3)
			return m, nil
		case key.Matches(msg, m.keys.PanDown):
			m.viewport2d.ScrollDown(3)
			return m, nil
		case key.Matches(msg, m.keys.PanLeft):
			m.viewport2d.ScrollLeft(5)
			return m, nil
		case key.Matches(msg, m.keys.PanRight):
			m.viewport2d.ScrollRight(5)
			return m, nil

		// Jump to first/last node
		case key.Matches(msg, m.keys.Home):
			m.selectedIndex = 0
			m.ensureSelectedVisible()
			m.updateViewportContent()
			return m, nil
		case key.Matches(msg, m.keys.End):
			m.selectedIndex = len(m.flattenedNodes) - 1
			m.ensureSelectedVisible()
			m.updateViewportContent()
			return m, nil
		}
	}

	return m, cmd
}

// Navigation methods
func (m *GraphVisualizerModel) navigateUp() {
	if m.selectedIndex > 0 {
		m.selectedIndex--
	}
}

func (m *GraphVisualizerModel) navigateDown() {
	if m.selectedIndex < len(m.flattenedNodes)-1 {
		m.selectedIndex++
	}
}

func (m *GraphVisualizerModel) navigateLeft() {
	if m.selectedIndex > 0 {
		m.selectedIndex--
	}
}

func (m *GraphVisualizerModel) navigateRight() {
	if m.selectedIndex < len(m.flattenedNodes)-1 {
		m.selectedIndex++
	}
}

func (m *GraphVisualizerModel) ensureSelectedVisible() {
	if m.selectedIndex < 0 || m.selectedIndex >= len(m.flattenedNodes) {
		return
	}

	selectedNode := m.flattenedNodes[m.selectedIndex]
	nodePos, exists := m.layout.nodePositions[selectedNode]
	if !exists {
		return
	}

	// Use the viewport's built-in method to ensure the node box is visible
	m.viewport2d.EnsureVisible(nodePos.X, nodePos.Y, nodeWidth, nodeHeight)
}

// View renders the graph visualizer
func (m *GraphVisualizerModel) View() string {
	return m.renderGraphView()
}

// renderGraphView renders the graph visualization panel
func (m *GraphVisualizerModel) renderGraphView() string {
	title := lipgloss.NewStyle().
		Bold(true).
		Foreground(ColorPrimary).
		Render("▶ Resource Graph (G: tree view)")

	graphWidth := m.width - 2

	borderColor := ColorMuted
	if m.focusedPanel == FocusTree {
		borderColor = ColorPrimary
	}

	graphBox := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(borderColor).
		Padding(0, 1).
		Width(graphWidth).
		Height(m.height - 4)

	helpView := m.help.View(m.keys)

	return lipgloss.JoinVertical(
		lipgloss.Left,
		title,
		graphBox.Render(m.viewport2d.View()),
		helpView,
	)
}

// renderDetailsPanel renders the details panel
func (m *GraphVisualizerModel) renderDetailsPanel() string {
	borderColor := ColorMuted
	if m.focusedPanel == FocusDetails {
		borderColor = ColorPrimary
	}

	detailsBox := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(borderColor).
		Padding(0, 1).
		Width(m.detailsWidth).
		Height(m.height - 4)

	return detailsBox.Render(m.detailsViewport.View())
}
