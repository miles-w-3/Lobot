package ui

import (
	"fmt"
	"strings"

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
	viewport        viewport.Model
	detailsViewport viewport.Model
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
}

// NewGraphVisualizerModel creates a new graph visualizer
func NewGraphVisualizerModel(resourceGraph *graph.ResourceGraph, width, height int) *GraphVisualizerModel {
	detailsWidth := 35
	graphWidth := width - detailsWidth - 2

	// Calculate layout
	layout := NewGraphLayout()
	layout.Calculate(resourceGraph, graphWidth-4)

	// Create viewports
	viewportHeight := height - 8
	graphViewport := viewport.New(graphWidth-4, viewportHeight)
	detailsViewport := viewport.New(detailsWidth-4, viewportHeight)

	// Calculate canvas dimensions
	canvasWidth := max(layout.maxLayerWidth+2*marginLeft, graphWidth-4)
	canvasHeight := layout.totalHeight

	// Flatten nodes for navigation (layer by layer, left to right)
	flattenedNodes := flattenGraphForNavigation(layout)

	model := &GraphVisualizerModel{
		graph:           resourceGraph,
		layout:          layout,
		viewport:        graphViewport,
		detailsViewport: detailsViewport,
		width:           width,
		height:          height,
		detailsWidth:    detailsWidth,
		canvasWidth:     canvasWidth,
		canvasHeight:    canvasHeight,
		showDetails:     false, // Hidden in graph view
		focusedPanel:    FocusTree,
		selectedIndex:   0,
		flattenedNodes:  flattenedNodes,
		rootResource:    resourceGraph.Root.Resource,
		keys:            DefaultGraphVisualizerKeyMap(),
	}

	model.updateViewportContent()
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

	// 2. Build the final view by overlaying boxes on the background
	// Start with the background lines
	lines := canvas.Lines()

	// For each box, we need to overlay it on the correct lines
	for i, node := range m.flattenedNodes {
		pos, exists := m.layout.nodePositions[node]
		if !exists {
			continue
		}

		selected := i == m.selectedIndex
		box := m.renderNodeBox(node, selected)
		boxLines := strings.Split(box, "\n")

		// Overlay box lines at the specified position
		for dy, boxLine := range boxLines {
			y := pos.Y + dy
			if y >= 0 && y < len(lines) {
				// Get the background line
				bgLine := lines[y]
				bgRunes := []rune(bgLine)
				
				// Calculate effective width of the box line (visual width)
				boxWidth := lipgloss.Width(boxLine)
				
				// If the box extends beyond the background line, pad the background
				if pos.X+boxWidth > len(bgRunes) {
					padding := pos.X + boxWidth - len(bgRunes)
					bgRunes = append(bgRunes, []rune(strings.Repeat(" ", padding))...)
				}

				// Create the new line by combining:
				// 1. Background before the box
				// 2. The box content
				// 3. Background after the box
				
				prefix := string(bgRunes[:pos.X])
				suffix := ""
				if pos.X+boxWidth < len(bgRunes) {
					suffix = string(bgRunes[pos.X+boxWidth:])
				}
				
				lines[y] = prefix + boxLine + suffix
			}
		}
	}

	// 3. Set viewport content
	m.viewport.SetContent(strings.Join(lines, "\n"))

	// 4. Update details panel for root resource
	m.updateDetailsPanel(m.graph.Root)
}

// renderNodeBox renders a single node as a Lipgloss box
func (m *GraphVisualizerModel) renderNodeBox(node *graph.Node, selected bool) string {
	res := node.Resource

	// Determine border color from status
	var borderColor lipgloss.Color
	status := strings.ToLower(res.GetStatus())
	if strings.Contains(status, "running") ||
		strings.Contains(status, "ready") ||
		strings.Contains(status, "active") {
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
		Width(nodeWidth - 2).
		Height(nodeHeight - 2).
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
			m.viewport.LineUp(3)
			return m, nil
		case key.Matches(msg, m.keys.PanDown):
			m.viewport.LineDown(3)
			return m, nil
		case key.Matches(msg, m.keys.PanLeft):
			// Horizontal panning not supported by viewport - would need custom scrolling
			// For now, just ignore
			return m, nil
		case key.Matches(msg, m.keys.PanRight):
			// Horizontal panning not supported by viewport - would need custom scrolling
			// For now, just ignore
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
	// Move to previous node in flattened list
	if m.selectedIndex > 0 {
		m.selectedIndex--
	}
}

func (m *GraphVisualizerModel) navigateRight() {
	// Move to next node in flattened list
	if m.selectedIndex < len(m.flattenedNodes)-1 {
		m.selectedIndex++
	}
}

func (m *GraphVisualizerModel) ensureSelectedVisible() {
	if m.selectedIndex < 0 || m.selectedIndex >= len(m.flattenedNodes) {
		return
	}

	// Calculate selected node's Y position
	selectedNode := m.flattenedNodes[m.selectedIndex]
	nodePos, exists := m.layout.nodePositions[selectedNode]
	if !exists {
		return
	}

	viewportHeight := m.viewport.Height
	currentYOffset := m.viewport.YOffset

	// Scroll if selected is above viewport
	if nodePos.Y < currentYOffset {
		m.viewport.SetYOffset(nodePos.Y)
	}

	// Scroll if selected is below viewport
	if nodePos.Y+nodeHeight >= currentYOffset+viewportHeight {
		m.viewport.SetYOffset(nodePos.Y - viewportHeight + nodeHeight + 1)
	}
}

// View renders the graph visualizer
func (m *GraphVisualizerModel) View() string {
	// Details panel is hidden in graph view
	return m.renderGraphView()
}

// renderGraphView renders the graph visualization panel
func (m *GraphVisualizerModel) renderGraphView() string {
	title := lipgloss.NewStyle().
		Bold(true).
		Foreground(ColorPrimary).
		Render("▶ Resource Graph (G: tree view)")

	// Always use full width in graph view (no details panel)
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

	helpText := lipgloss.NewStyle().
		Foreground(ColorMuted).
		Render("arrows: select node • i/j/k/l: pan • g/G: first/last • G: tree view • q: back")

	return lipgloss.JoinVertical(
		lipgloss.Left,
		title,
		graphBox.Render(m.viewport.View()),
		helpText,
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
