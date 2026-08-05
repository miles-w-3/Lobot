package visualizer

import (
	"fmt"
	"image/color"
	"sort"
	"strings"

	"github.com/charmbracelet/x/ansi"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	"github.com/miles-w-3/lobot/internal/graph"
	"github.com/miles-w-3/lobot/internal/keys"
	"github.com/miles-w-3/lobot/internal/util"
)

// GraphView represents the graph visualization component
type GraphView struct {
	graph          *graph.ResourceGraph
	layout         *GraphLayout
	viewport2d     *Viewport2D
	width          int
	height         int
	canvasWidth    int
	canvasHeight   int
	selectedIndex  int
	flattenedNodes []*graph.Node
	switchKey      string
	zoomLevel      float64 // Zoom scale (1.0 = 100%)
}

// NewGraphView creates a new graph visualizer.
func NewGraphView(resourceGraph *graph.ResourceGraph, width, height int, switchKey string) *GraphView {
	model := &GraphView{
		graph:     resourceGraph,
		width:     max(1, width),
		height:    max(1, height),
		switchKey: switchKey,
		zoomLevel: 1.0,
	}
	if resourceGraph != nil {
		model.Resize(width, height)
	}
	return model
}

// Resize recalculates layout while preserving selection, zoom, and scroll
// position as far as the new viewport permits.
func (m *GraphView) Resize(width, height int) {
	m.width = max(1, width)
	m.height = max(1, height)
	if m.graph == nil {
		return
	}

	oldX, oldY := 0, 0
	if m.viewport2d != nil {
		oldX, oldY = m.viewport2d.XOffset, m.viewport2d.YOffset
	}

	graphWidth := max(1, m.width-2)
	viewportWidth := max(3, graphWidth-4)
	viewportHeight := max(3, m.height-8)
	layoutWidth := max(1, viewportWidth-2)

	layout := NewGraphLayout()
	layout.Calculate(m.graph, layoutWidth)
	m.layout = layout
	m.canvasWidth = max(layout.maxLayerWidth+2*marginLeft, layoutWidth)
	m.canvasHeight = max(1, layout.totalHeight)
	m.viewport2d = NewViewport2D(viewportWidth, viewportHeight)
	m.flattenedNodes = flattenGraphForNavigation(layout)
	m.selectedIndex = min(max(0, m.selectedIndex), max(0, len(m.flattenedNodes)-1))
	m.updateViewportContent()
	m.viewport2d.ScrollTo(oldX, oldY)
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
func (m *GraphView) updateViewportContent() {
	if m.graph == nil || m.layout == nil || m.viewport2d == nil {
		return
	}

	// 1. Create background canvas with edge lines
	canvas := NewCanvas(m.canvasWidth, m.canvasHeight)

	for _, edge := range m.graph.Edges {
		fromPos, fromExists := m.layout.nodePositions[edge.From]
		toPos, toExists := m.layout.nodePositions[edge.To]
		if fromExists && toExists {
			canvas.DrawEdge(fromPos, toPos)
		}
	}
	// 2. Prepare node box fragments (bucketed by Y position)
	// This avoids repeatedly scanning lines and manipulating strings
	type nodeSegment struct {
		x       int
		content string
	}
	segmentsByRow := make(map[int][]nodeSegment)

	for i, node := range m.flattenedNodes {
		pos, exists := m.layout.nodePositions[node]
		if !exists {
			continue
		}

		selected := i == m.selectedIndex
		box := m.renderNodeBox(node, selected)
		boxLines := strings.Split(box, "\n")

		for dy, line := range boxLines {
			y := pos.Y + dy
			if y >= 0 && y < m.canvasHeight {
				segmentsByRow[y] = append(segmentsByRow[y], nodeSegment{
					x:       pos.X,
					content: line,
				})
			}
		}
	}
	// 3. Render final lines by stitching background and segments
	// This is much faster than repeatedly calling OverlayStyledContent
	lines := make([]string, m.canvasHeight)

	for y := 0; y < m.canvasHeight; y++ {
		rowSegments := segmentsByRow[y]

		// If no segments on this line, just use the background line
		if len(rowSegments) == 0 {
			lines[y] = string(canvas.Row(y))
			continue
		}

		// Sort segments by X position
		sort.Slice(rowSegments, func(i, j int) bool {
			return rowSegments[i].x < rowSegments[j].x
		})

		var sb strings.Builder
		cursor := 0
		bgRow := canvas.Row(y)
		bgLen := len(bgRow)

		for _, seg := range rowSegments {
			// Append background up to the segment start
			if seg.x > cursor {
				if cursor < bgLen {
					limit := seg.x
					if limit > bgLen {
						limit = bgLen
					}
					sb.WriteString(string(bgRow[cursor:limit]))
				} else {
					// Pad with spaces if we're past the background width
					sb.WriteString(strings.Repeat(" ", seg.x-cursor))
				}
				cursor = seg.x
			}

			// Append the segment
			sb.WriteString(seg.content)

			// Move cursor past the segment using visual width
			cursor += ansi.StringWidth(seg.content)
		}

		// Append any remaining background
		if cursor < bgLen {
			sb.WriteString(string(bgRow[cursor:]))
		}

		lines[y] = sb.String()
	}

	// 4. Set the full content on the 2D viewport
	m.viewport2d.SetLines(lines)

	// Ensure selected node is visible
	if m.selectedIndex >= 0 && m.selectedIndex < len(m.flattenedNodes) {
		node := m.flattenedNodes[m.selectedIndex]
		if pos, exists := m.layout.nodePositions[node]; exists {
			m.viewport2d.EnsureVisible(pos.X, pos.Y, pos.Width, pos.Height)
		}
	}

}

// renderNodeBox renders a single node as a Lipgloss box
func (m *GraphView) renderNodeBox(node *graph.Node, selected bool) string {
	res := node.Resource

	// Determine border color from status
	var borderColor color.Color
	status := strings.ToLower(res.GetStatus())
	if strings.Contains(status, "running") ||
		strings.Contains(status, "ready") ||
		strings.Contains(status, "active") ||
		strings.Contains(status, "deployed") {
		borderColor = util.ColorSuccess
	} else if strings.Contains(status, "pending") ||
		strings.Contains(status, "creating") {
		borderColor = util.ColorWarning
	} else if strings.Contains(status, "failed") ||
		strings.Contains(status, "error") {
		borderColor = util.ColorDanger
	} else {
		borderColor = util.ColorMuted
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
			BorderForeground(util.ColorPrimary).
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

// Update handles non-command messages. Key commands are dispatched once by
// the parent VisualizerScreen.
func (m *GraphView) Update(msg tea.Msg) tea.Cmd {
	return nil
}

// Navigation methods
func (m *GraphView) navigateUp() {
	if m.selectedIndex > 0 {
		m.selectedIndex--
	}
}

// navigateHierarchicalDown attempts to select the first child of the current node.
// If no child exists, it does nothing
func (m *GraphView) navigateHierarchicalDown() {
	if m.selectedIndex < 0 || m.selectedIndex >= len(m.flattenedNodes) {
		return
	}

	currentNode := m.flattenedNodes[m.selectedIndex]
	children := m.graph.GetChildren(currentNode)

	if len(children) > 0 {
		// Target the first child
		target := children[0]

		// Find target index in flattened list
		for i, n := range m.flattenedNodes {
			if n == target {
				m.selectedIndex = i
				return
			}
		}
	}
}

func (m *GraphView) navigateHierarchicalUp() {
	if m.selectedIndex < 0 || m.selectedIndex >= len(m.flattenedNodes) {
		return
	}

	currentNode := m.flattenedNodes[m.selectedIndex]
	parents := m.graph.GetParents(currentNode)

	if len(parents) > 0 {
		// Target the first parent
		target := parents[0]

		// Find target index in flattened list
		for i, n := range m.flattenedNodes {
			if n == target {
				m.selectedIndex = i
				return
			}
		}
	}
}

func (m *GraphView) navigateDown() {
	if m.selectedIndex < len(m.flattenedNodes)-1 {
		m.selectedIndex++
	}
}

func (m *GraphView) navigateLeft() {
	if m.selectedIndex > 0 {
		m.selectedIndex--
	}
}

func (m *GraphView) navigateRight() {
	if m.selectedIndex < len(m.flattenedNodes)-1 {
		m.selectedIndex++
	}
}

func (m *GraphView) ensureSelectedVisible() {
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
func (m *GraphView) View() string {
	if m.graph == nil || m.viewport2d == nil {
		return "No resource graph available"
	}
	return m.renderGraphView()
}

// renderGraphView renders the graph visualization panel
func (m *GraphView) renderGraphView() string {
	switchKey := m.switchKey
	title := lipgloss.NewStyle().
		Bold(true).
		Foreground(util.ColorPrimary).
		Render(fmt.Sprintf("▶ Resource Graph (%s: tree view)", switchKey))

	graphWidth := max(1, m.width-2)

	graphBox := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(util.ColorPrimary).
		Padding(0, 1).
		Width(graphWidth).
		Height(max(1, m.height-4))

	// helpView := m.help.View(m.keys)

	return lipgloss.JoinVertical(
		lipgloss.Left,
		title,
		graphBox.Render(m.viewport2d.View()),
		// helpView,
	)
}

// HandleCommand handles a graph command directly
func (m *GraphView) HandleCommand(cmd keys.GraphCmd) tea.Cmd {
	switch cmd {
	case keys.GraphCmdSelectUp:
		m.navigateHierarchicalUp()
		m.ensureSelectedVisible()
		m.updateViewportContent()
	case keys.GraphCmdSelectDown:
		m.navigateHierarchicalDown()
		m.ensureSelectedVisible()
		m.updateViewportContent()
	case keys.GraphCmdSelectLeft:
		m.navigateLeft()
		m.ensureSelectedVisible()
		m.updateViewportContent()
	case keys.GraphCmdSelectRight:
		m.navigateRight()
		m.ensureSelectedVisible()
		m.updateViewportContent()
	case keys.GraphCmdPanUp:
		m.viewport2d.ScrollUp(3)
	case keys.GraphCmdPanDown:
		m.viewport2d.ScrollDown(3)
	case keys.GraphCmdPanLeft:
		m.viewport2d.ScrollLeft(5)
	case keys.GraphCmdPanRight:
		m.viewport2d.ScrollRight(5)
	case keys.GraphCmdHome:
		m.selectedIndex = 0
		m.ensureSelectedVisible()
		m.updateViewportContent()
	case keys.GraphCmdEnd:
		m.selectedIndex = len(m.flattenedNodes) - 1
		m.ensureSelectedVisible()
		m.updateViewportContent()
	case keys.GraphCmdZoomIn:
		// Increase zoom level (max 300%)
		if m.zoomLevel < 3.0 {
			m.zoomLevel += 0.25
			m.updateViewportContent()
		}
	case keys.GraphCmdZoomOut:
		// Decrease zoom level (min 50%)
		if m.zoomLevel > 0.5 {
			m.zoomLevel -= 0.25
			m.updateViewportContent()
		}
	}
	return nil
}
