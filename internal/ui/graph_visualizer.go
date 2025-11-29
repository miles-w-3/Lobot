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
		showDetails:     true,
		focusedPanel:    FocusTree,
		rootResource:    resourceGraph.Root.Resource,
		keys:            DefaultGraphVisualizerKeyMap(),
	}

	model.updateViewportContent()
	return model
}

// updateViewportContent renders the graph and updates the viewport
func (m *GraphVisualizerModel) updateViewportContent() {
	// Build output line by line
	var lines []string
	for y := 0; y < m.canvasHeight; y++ {
		lines = append(lines, strings.Repeat(" ", m.canvasWidth))
	}

	// 1. Draw edges on canvas
	for _, edge := range m.graph.Edges {
		fromPos, fromExists := m.layout.nodePositions[edge.From]
		toPos, toExists := m.layout.nodePositions[edge.To]
		if !fromExists || !toExists {
			continue
		}

		// Draw edge characters directly into lines
		fromCenterX := fromPos.X + fromPos.Width/2
		fromBottomY := fromPos.Y + fromPos.Height
		toCenterX := toPos.X + toPos.Width/2
		toTopY := toPos.Y

		// Vertical line case
		if fromCenterX == toCenterX {
			for y := fromBottomY; y < toTopY && y < m.canvasHeight; y++ {
				if y >= 0 && fromCenterX >= 0 && fromCenterX < m.canvasWidth {
					lineRunes := []rune(lines[y])
					if y == toTopY-1 {
						lineRunes[fromCenterX] = '▼'
					} else {
						lineRunes[fromCenterX] = '│'
					}
					lines[y] = string(lineRunes)
				}
			}
		} else {
			// L-shaped line
			midY := fromBottomY + (toTopY-fromBottomY)/2

			// Vertical from parent
			for y := fromBottomY; y < midY && y < m.canvasHeight; y++ {
				if y >= 0 && fromCenterX >= 0 && fromCenterX < m.canvasWidth {
					lineRunes := []rune(lines[y])
					lineRunes[fromCenterX] = '│'
					lines[y] = string(lineRunes)
				}
			}

			// Horizontal segment
			startX := min(fromCenterX, toCenterX)
			endX := max(fromCenterX, toCenterX)
			if midY >= 0 && midY < m.canvasHeight {
				lineRunes := []rune(lines[midY])
				for x := startX; x <= endX && x < m.canvasWidth; x++ {
					if x == fromCenterX || x == toCenterX {
						lineRunes[x] = '┼'
					} else {
						lineRunes[x] = '─'
					}
				}
				lines[midY] = string(lineRunes)
			}

			// Vertical to child
			for y := midY + 1; y < toTopY && y < m.canvasHeight; y++ {
				if y >= 0 && toCenterX >= 0 && toCenterX < m.canvasWidth {
					lineRunes := []rune(lines[y])
					lineRunes[toCenterX] = '│'
					lines[y] = string(lineRunes)
				}
			}
			if toTopY-1 >= 0 && toTopY-1 < m.canvasHeight && toCenterX >= 0 && toCenterX < m.canvasWidth {
				lineRunes := []rune(lines[toTopY-1])
				lineRunes[toCenterX] = '▼'
				lines[toTopY-1] = string(lineRunes)
			}
		}
	}

	// 2. Overlay boxes - render each box and place it using lipgloss Place
	for _, layer := range m.layout.layers {
		for _, layoutNode := range layer {
			pos, exists := m.layout.nodePositions[layoutNode.graphNode]
			if !exists {
				continue
			}

			box := m.renderNodeBox(layoutNode.graphNode, false)
			boxLines := strings.Split(box, "\n")

		// Place box lines at the correct position
		for dy, boxLine := range boxLines {
			y := pos.Y + dy
			if y >= 0 && y < len(lines) {
				// Use lipgloss Place to position the box content properly
				lineBefore := ""
				if pos.X > 0 {
					lineBefore = lines[y][:min(pos.X, len(lines[y]))]
				}

				lineAfter := ""
				boxWidth := lipgloss.Width(boxLine)
				afterX := pos.X + boxWidth
				if afterX < len(lines[y]) {
					lineAfter = lines[y][afterX:]
				}

				lines[y] = lineBefore + boxLine + lineAfter
			}
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
		case key.Matches(msg, m.keys.FocusLeft):
			if m.showDetails {
				m.focusedPanel = FocusTree
			}
			return m, nil

		case key.Matches(msg, m.keys.FocusRight):
			if m.showDetails {
				m.focusedPanel = FocusDetails
			}
			return m, nil

		case key.Matches(msg, m.keys.ToggleDetails):
			m.showDetails = !m.showDetails
			if !m.showDetails {
				m.focusedPanel = FocusTree
			}
			return m, nil
		}
	}

	// Navigation when graph is focused - use arrow keys for panning
	if m.focusedPanel == FocusTree {
		// Let viewport handle all scrolling/panning
		m.viewport, cmd = m.viewport.Update(msg)
	} else if m.focusedPanel == FocusDetails && m.showDetails {
		m.detailsViewport, cmd = m.detailsViewport.Update(msg)
	}

	return m, cmd
}

// View renders the graph visualizer
func (m *GraphVisualizerModel) View() string {
	if m.showDetails {
		graphView := m.renderGraphView()
		detailsView := m.renderDetailsPanel()

		return lipgloss.JoinHorizontal(
			lipgloss.Top,
			graphView,
			detailsView,
		)
	}

	return m.renderGraphView()
}

// renderGraphView renders the graph visualization panel
func (m *GraphVisualizerModel) renderGraphView() string {
	title := lipgloss.NewStyle().
		Bold(true).
		Foreground(ColorPrimary).
		Render("▶ Resource Graph (G: tree view)")

	var graphWidth int
	if !m.showDetails {
		graphWidth = m.width - 2
	} else {
		graphWidth = m.width - m.detailsWidth - 2
	}

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
		Render("arrows: pan • d: details • G: tree view • q: back")

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
