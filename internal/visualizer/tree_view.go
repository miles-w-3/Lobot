package visualizer

import (
	"fmt"
	"strings"

	"charm.land/bubbles/v2/viewport"
	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	"github.com/miles-w-3/lobot/internal/graph"
	"github.com/miles-w-3/lobot/internal/k8s"
	"github.com/miles-w-3/lobot/internal/keys"
	"github.com/miles-w-3/lobot/internal/util"
)

const (
	treeBottomLeft = " └──"
	treeVertical   = " │  "
	treeBranch     = " ├──"
)

type treeFocusPanel uint8

const (
	focusTree treeFocusPanel = iota
	focusDetails
)

// TreeView represents a custom tree visualizer with proper scrolling
type TreeView struct {
	viewport        viewport.Model
	graph           *graph.ResourceGraph
	width           int
	height          int
	detailsWidth    int
	showDetails     bool
	focusedPanel    treeFocusPanel
	selectedIndex   int
	flattenedNodes  []*treeNode
	expandedNodes   map[*graph.Node]bool // Track which nodes are expanded
	rootResource    k8s.TrackedObject
	switchKey       string
	detailsViewport viewport.Model
}

// treeNode represents a flattened tree node for cursor navigation
type treeNode struct {
	graphNode *graph.Node
	depth     int
	isLast    bool
	prefix    string
}

// NewTreeView creates a new tree visualizer with viewport-based scrolling
func NewTreeView(resourceGraph *graph.ResourceGraph, width, height int, switchKey string) *TreeView {
	model := &TreeView{
		viewport:        viewport.New(),
		detailsViewport: viewport.New(),
		graph:           resourceGraph,
		width:           width,
		height:          height,
		detailsWidth:    35,
		showDetails:     false,
		focusedPanel:    focusTree,
		selectedIndex:   0,
		expandedNodes:   make(map[*graph.Node]bool),
		switchKey:       switchKey,
	}
	if resourceGraph == nil {
		return model
	}

	if resourceGraph.Root != nil {
		model.rootResource = resourceGraph.Root.Resource
	}
	for _, node := range resourceGraph.Nodes {
		if node != nil {
			model.expandedNodes[node] = true
		}
	}
	model.rebuildFlattenedTree()
	model.Resize(width, height)
	return model
}

// Resize preserves tree state while updating viewport dimensions.
func (m *TreeView) Resize(width, height int) {
	m.width = max(1, width)
	m.height = max(1, height)
	m.resizeViewports()
	m.updateViewportContent()
}

func (m *TreeView) resizeViewports() {
	treeWidth := m.width - 2
	if m.showDetails {
		treeWidth = m.width - m.detailsWidth - 2
	}
	m.viewport.SetWidth(max(1, treeWidth-4))
	m.viewport.SetHeight(max(1, m.height-8))
	m.detailsViewport.SetWidth(max(1, m.detailsWidth-4))
	m.detailsViewport.SetHeight(max(1, m.height-8))
}

// rebuildFlattenedTree flattens the tree structure for linear navigation
func (m *TreeView) rebuildFlattenedTree() {
	m.flattenedNodes = make([]*treeNode, 0)

	// Find root nodes (nodes with no parents)
	rootNodes := findRootNodes(m.graph)
	visited := make(map[*graph.Node]bool)

	for i, rootNode := range rootNodes {
		isLast := i == len(rootNodes)-1
		m.flattenNode(rootNode, 0, "", isLast, visited)
	}
}

// flattenNode recursively flattens a tree node and its children
func (m *TreeView) flattenNode(node *graph.Node, depth int, parentPrefix string, isLast bool, visited map[*graph.Node]bool) {
	// Prevent infinite loops
	if visited[node] {
		m.flattenedNodes = append(m.flattenedNodes, &treeNode{
			graphNode: node,
			depth:     depth,
			isLast:    isLast,
			prefix:    parentPrefix + "(circular)",
		})
		return
	}
	visited[node] = true

	// Calculate prefix for this node
	var prefix string
	if depth == 0 {
		prefix = ""
	} else if isLast {
		prefix = parentPrefix + treeBottomLeft
	} else {
		prefix = parentPrefix + treeBranch
	}

	// Add this node to flattened list
	m.flattenedNodes = append(m.flattenedNodes, &treeNode{
		graphNode: node,
		depth:     depth,
		isLast:    isLast,
		prefix:    prefix,
	})

	// Only recurse to children if this node is expanded
	if !m.expandedNodes[node] {
		return
	}

	// Get children and recurse
	children := m.graph.GetChildren(node)
	for i, child := range children {
		childIsLast := i == len(children)-1

		// Calculate prefix for children's children
		var childParentPrefix string
		if depth == 0 {
			childParentPrefix = ""
		} else if isLast {
			childParentPrefix = parentPrefix + "    "
		} else {
			childParentPrefix = parentPrefix + treeVertical
		}

		m.flattenNode(child, depth+1, childParentPrefix, childIsLast, visited)
	}
}

// updateViewportContent renders the tree and updates the viewport
func (m *TreeView) updateViewportContent() {
	var b strings.Builder

	for i, node := range m.flattenedNodes {
		selected := i == m.selectedIndex
		line := m.renderTreeLine(node, selected)
		b.WriteString(line)
		b.WriteString("\n")
	}

	m.viewport.SetContent(b.String())

	if m.showDetails && m.selectedIndex >= 0 && m.selectedIndex < len(m.flattenedNodes) {
		m.updateDetailsPanel(m.flattenedNodes[m.selectedIndex].graphNode)
	}
}

// renderTreeLine renders a single line of the tree
func (m *TreeView) renderTreeLine(node *treeNode, selected bool) string {
	res := node.graphNode.Resource

	// Add expand/collapse indicator
	hasChildren := len(m.graph.GetChildren(node.graphNode)) > 0
	var expandIndicator string
	if hasChildren {
		if m.expandedNodes[node.graphNode] {
			expandIndicator = "▼ "
		} else {
			expandIndicator = "▶ "
		}
	} else {
		expandIndicator = "  "
	}

	name := fmt.Sprintf("%s%s: %s", expandIndicator, res.GetKind(), res.GetName())

	// Add namespace label if needed
	nameWithNamespace := addNamespaceLabel(node.graphNode, m.rootResource, name)

	// Add root indicator
	if node.graphNode.IsRoot {
		nameWithNamespace = nameWithNamespace + " ●"
	}

	// Build the line with appropriate styling
	if selected {
		// When selected, use white text on purple background for entire line
		line := node.prefix + " " + nameWithNamespace + "\t" + formatResourceDescPlain(node.graphNode)
		return lipgloss.NewStyle().
			Background(util.ColorSecondary).
			Foreground(lipgloss.Color("#FFFFFF")).
			Width(m.viewport.Width()).
			Render(line)
	}

	// Normal rendering with colored components
	var styledName string
	if node.graphNode.Metadata["missing"] == "true" {
		styledName = lipgloss.NewStyle().Foreground(util.ColorMuted).Render(nameWithNamespace)
	} else if node.graphNode.IsRoot {
		styledName = lipgloss.NewStyle().Bold(true).Foreground(util.ColorWarning).Render(nameWithNamespace)
	} else {
		color := getColorForKind(res.GetKind())
		styledName = lipgloss.NewStyle().Foreground(lipgloss.Color(color)).Render(nameWithNamespace)
	}

	status := formatResourceDesc(node.graphNode)
	line := node.prefix + " " + styledName + "\t" + status

	return line
}

// updateDetailsPanel updates the details viewport for the selected resource.
func (m *TreeView) updateDetailsPanel(node *graph.Node) {
	if node == nil || node.Resource == nil {
		return
	}

	res := node.Resource
	var details strings.Builder
	details.WriteString(lipgloss.NewStyle().Bold(true).Render("Resource Details"))
	details.WriteString("\n\n")
	details.WriteString(fmt.Sprintf("Kind: %s\n", res.GetKind()))
	details.WriteString(fmt.Sprintf("Name: %s\n", res.GetName()))
	if res.GetNamespace() != "" {
		details.WriteString(fmt.Sprintf("Namespace: %s\n", res.GetNamespace()))
	}
	details.WriteString(fmt.Sprintf("Status: %s\n", res.GetStatus()))
	details.WriteString(fmt.Sprintf("Age: %s\n", util.FormatAge(res.GetAge())))

	if raw := res.GetRaw(); raw != nil {
		if apiVersion := raw.GetAPIVersion(); apiVersion != "" {
			details.WriteString(fmt.Sprintf("API version: %s\n", apiVersion))
		}
		if uid := string(raw.GetUID()); uid != "" {
			details.WriteString(fmt.Sprintf("UID: %s\n", uid))
		}
	}

	m.appendRelationships(&details, node)

	// Show type-specific fields only when the tracked object provides them.
	if helmRes, ok := res.(*k8s.HelmRelease); ok {
		if helmRes.HelmChart != "" {
			details.WriteString(fmt.Sprintf("Chart: %s\n", helmRes.HelmChart))
		}
		if helmRes.HelmRevision > 0 {
			details.WriteString(fmt.Sprintf("Revision: %d\n", helmRes.HelmRevision))
		}
	}

	if argoApp, ok := res.(*k8s.ArgoCDApp); ok {
		if argoApp.SyncStatus != "" {
			details.WriteString(fmt.Sprintf("Sync status: %s\n", argoApp.SyncStatus))
		}
		if argoApp.Health != "" {
			details.WriteString(fmt.Sprintf("Health: %s\n", argoApp.Health))
		}
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

	m.detailsViewport.SetContent(details.String())
}

func (m *TreeView) appendRelationships(details *strings.Builder, node *graph.Node) {
	if node.IsRoot {
		details.WriteString("Role: root resource\n")
	} else if node.RelationshipType != "" {
		details.WriteString(fmt.Sprintf("Relationship: %s\n", node.RelationshipType))
	}

	for _, edge := range m.graph.Edges {
		if edge == nil {
			continue
		}
		if edge.To == node {
			details.WriteString(fmt.Sprintf("Parent: %s (%s)\n", formatNodeReference(edge.From), edge.Type))
		}
		if edge.From == node {
			details.WriteString(fmt.Sprintf("Child: %s (%s)\n", formatNodeReference(edge.To), edge.Type))
		}
	}
}

// Update forwards non-command messages to the focused viewport. Key commands
// are dispatched once by the parent VisualizerScreen.
func (m *TreeView) Update(msg tea.Msg) tea.Cmd {
	var cmd tea.Cmd
	if m.focusedPanel == focusTree {
		m.viewport, cmd = m.viewport.Update(msg)
	} else if m.focusedPanel == focusDetails && m.showDetails {
		m.detailsViewport, cmd = m.detailsViewport.Update(msg)
	}
	return cmd
}

// Navigation methods
func (m *TreeView) navigateUp() {
	if m.selectedIndex > 0 {
		m.selectedIndex--
		m.ensureSelectedVisible()
	}
}

func (m *TreeView) navigateDown() {
	if m.selectedIndex < len(m.flattenedNodes)-1 {
		m.selectedIndex++
		m.ensureSelectedVisible()
	}
}

func (m *TreeView) navigateTop() {
	m.selectedIndex = 0
	m.viewport.GotoTop()
}

func (m *TreeView) navigateBottom() {
	m.selectedIndex = len(m.flattenedNodes) - 1
	m.ensureSelectedVisible()
}

func (m *TreeView) pageUp() {
	m.selectedIndex = max(0, m.selectedIndex-10)
	m.ensureSelectedVisible()
}

func (m *TreeView) pageDown() {
	m.selectedIndex = min(len(m.flattenedNodes)-1, m.selectedIndex+10)
	m.ensureSelectedVisible()
}

func (m *TreeView) ensureSelectedVisible() {
	// Calculate the viewport's visible range
	viewportHeight := m.viewport.Height()
	currentYOffset := m.viewport.YOffset()

	// If selected is above viewport, scroll up
	if m.selectedIndex < currentYOffset {
		m.viewport.SetYOffset(m.selectedIndex)
	}

	// If selected is below viewport, scroll down
	if m.selectedIndex >= currentYOffset+viewportHeight {
		m.viewport.SetYOffset(m.selectedIndex - viewportHeight + 1)
	}
}

// toggleCurrentNode toggles expand/collapse for the currently selected node
func (m *TreeView) toggleCurrentNode() {
	if m.selectedIndex < 0 || m.selectedIndex >= len(m.flattenedNodes) {
		return
	}

	node := m.flattenedNodes[m.selectedIndex].graphNode
	children := m.graph.GetChildren(node)

	// Only toggle if node has children
	if len(children) == 0 {
		return
	}

	// Toggle expanded state
	m.expandedNodes[node] = !m.expandedNodes[node]

	// Rebuild tree to reflect new state
	m.rebuildFlattenedTree()
	m.updateViewportContent()
}

// expandAll expands all nodes in the tree
func (m *TreeView) expandAll() {
	for _, node := range m.graph.Nodes {
		m.expandedNodes[node] = true
	}

	m.rebuildFlattenedTree()
	m.updateViewportContent()
}

// collapseAll collapses all nodes except root nodes
func (m *TreeView) collapseAll() {
	rootNodes := findRootNodes(m.graph)
	rootMap := make(map[*graph.Node]bool)
	for _, root := range rootNodes {
		rootMap[root] = true
	}

	for _, node := range m.graph.Nodes {
		// Keep root nodes expanded
		if rootMap[node] {
			m.expandedNodes[node] = true
		} else {
			m.expandedNodes[node] = false
		}
	}

	m.rebuildFlattenedTree()
	m.updateViewportContent()
}

// View renders the tree visualizer
func (m *TreeView) View() string {
	if m.showDetails {
		// Split view: tree on left, details on right
		treeView := m.renderTreeView()
		detailsView := m.renderDetailsPanel()

		return lipgloss.JoinHorizontal(
			lipgloss.Top,
			treeView,
			detailsView,
		)
	}

	// Full-width tree view
	return m.renderTreeView()
}

// renderTreeView renders the tree visualization panel
func (m *TreeView) renderTreeView() string {
	switchKey := m.switchKey
	title := lipgloss.NewStyle().
		Bold(true).
		Foreground(util.ColorPrimary).
		Render(fmt.Sprintf("▶ Resource Tree (%s: graph view)", switchKey))

	// Calculate tree panel width
	var treeWidth int
	if !m.showDetails {
		treeWidth = m.width - 2
	} else {
		treeWidth = m.width - m.detailsWidth - 2
	}
	treeWidth = max(1, treeWidth)

	// Highlight border if tree is focused
	borderColor := util.ColorMuted
	if m.focusedPanel == focusTree {
		borderColor = util.ColorPrimary
	}

	treeBox := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(borderColor).
		Padding(0, 1).
		Width(treeWidth).
		Height(max(1, m.height-4))

	return lipgloss.JoinVertical(
		lipgloss.Left,
		title,
		treeBox.Render(m.viewport.View()),
	)
}

// renderDetailsPanel renders the details panel
func (m *TreeView) renderDetailsPanel() string {
	borderColor := util.ColorMuted
	if m.focusedPanel == focusDetails {
		borderColor = util.ColorPrimary
	}

	detailsBox := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(borderColor).
		Padding(0, 1).
		Width(max(1, m.detailsWidth)).
		Height(max(1, m.height-4))

	return detailsBox.Render(m.detailsViewport.View())
}

// HandleCommand handles a tree command directly
func (m *TreeView) HandleCommand(cmd keys.TreeCmd) tea.Cmd {
	switch cmd {
	case keys.TreeCmdMoveUp:
		m.navigateUp()
		m.updateViewportContent()
	case keys.TreeCmdMoveDown:
		m.navigateDown()
		m.updateViewportContent()
	case keys.TreeCmdHome:
		m.navigateTop()
		m.updateViewportContent()
	case keys.TreeCmdEnd:
		m.navigateBottom()
		m.updateViewportContent()
	case keys.TreeCmdPageUp:
		m.pageUp()
		m.updateViewportContent()
	case keys.TreeCmdPageDown:
		m.pageDown()
		m.updateViewportContent()
	case keys.TreeCmdToggle:
		m.toggleCurrentNode()
	case keys.TreeCmdExpandAll:
		m.expandAll()
	case keys.TreeCmdCollapseAll:
		m.collapseAll()
	case keys.TreeCmdFocusLeft:
		if m.showDetails {
			m.focusedPanel = focusTree
		}
	case keys.TreeCmdFocusRight:
		if m.showDetails {
			m.focusedPanel = focusDetails
		}
	case keys.TreeCmdToggleDetails:
		m.showDetails = !m.showDetails
		if !m.showDetails {
			m.focusedPanel = focusTree
		}
		m.resizeViewports()
		m.updateViewportContent()
	}
	return nil
}
