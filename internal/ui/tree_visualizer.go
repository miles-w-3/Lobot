//go:build legacyui

package ui

import (
	"fmt"
	"strings"

	"charm.land/bubbles/v2/viewport"
	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	"github.com/miles-w-3/lobot/internal/command"
	"github.com/miles-w-3/lobot/internal/graph"
	"github.com/miles-w-3/lobot/internal/k8s"
	"github.com/miles-w-3/lobot/internal/keys"
)

const (
	treeBottomLeft = " └──"
	treeVertical   = " │  "
	treeBranch     = " ├──"
)

// TreeVisualizerModel represents a custom tree visualizer with proper scrolling
type TreeVisualizerModel struct {
	viewport        viewport.Model
	graph           *graph.ResourceGraph
	width           int
	height          int
	detailsWidth    int
	showDetails     bool
	focusedPanel    FocusPanel
	selectedIndex   int
	flattenedNodes  []*treeNode
	expandedNodes   map[*graph.Node]bool // Track which nodes are expanded
	rootResource    k8s.TrackedObject
	registry        *command.Registry[keys.TreeCmd]
	detailsViewport viewport.Model
}

// treeNode represents a flattened tree node for cursor navigation
type treeNode struct {
	graphNode *graph.Node
	depth     int
	isLast    bool
	prefix    string
}

// NewTreeVisualizerModel creates a new tree visualizer with viewport-based scrolling
func NewTreeVisualizerModel(resourceGraph *graph.ResourceGraph, width, height int, registry *command.Registry[keys.TreeCmd]) TreeVisualizerModel {
	// Calculate panel widths
	detailsWidth := 35                    // Reduced from 40 to give more space to tree
	treeWidth := width - detailsWidth - 2 // Account for gap between panels

	// Create main tree viewport (account for border + padding = 4 total)
	treeViewportWidth := treeWidth - 4
	treeViewportHeight := height - 8 // Account for title, border, help

	treeViewport := viewport.New(
		viewport.WithWidth(treeViewportWidth),
		viewport.WithHeight(treeViewportHeight),
	)

	// Create details viewport (account for border + padding = 4 total)
	detailsViewportWidth := detailsWidth - 4
	detailsViewportHeight := height - 8

	detailsViewport := viewport.New(
		viewport.WithWidth(detailsViewportWidth),
		viewport.WithHeight(detailsViewportHeight),
	)

	model := TreeVisualizerModel{
		viewport:        treeViewport,
		detailsViewport: detailsViewport,
		graph:           resourceGraph,
		width:           width,
		height:          height,
		detailsWidth:    detailsWidth,
		showDetails:     true,
		focusedPanel:    FocusTree,
		selectedIndex:   0,
		expandedNodes:   make(map[*graph.Node]bool),
		rootResource:    resourceGraph.Root.Resource,
		registry:        registry,
	}

	// Expand all nodes by default
	for _, node := range resourceGraph.Nodes {
		model.expandedNodes[node] = true
	}

	// Build flattened tree for navigation
	model.rebuildFlattenedTree()

	// Render initial content
	model.updateViewportContent()

	return model
}

// rebuildFlattenedTree flattens the tree structure for linear navigation
func (m *TreeVisualizerModel) rebuildFlattenedTree() {
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
func (m *TreeVisualizerModel) flattenNode(node *graph.Node, depth int, parentPrefix string, isLast bool, visited map[*graph.Node]bool) {
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
func (m *TreeVisualizerModel) updateViewportContent() {
	var b strings.Builder

	for i, node := range m.flattenedNodes {
		selected := i == m.selectedIndex
		line := m.renderTreeLine(node, selected)
		b.WriteString(line)
		b.WriteString("\n")
	}

	m.viewport.SetContent(b.String())

	// Update details panel
	if m.selectedIndex >= 0 && m.selectedIndex < len(m.flattenedNodes) {
		m.updateDetailsPanel(m.flattenedNodes[m.selectedIndex].graphNode)
	}
}

// renderTreeLine renders a single line of the tree
func (m *TreeVisualizerModel) renderTreeLine(node *treeNode, selected bool) string {
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
			Background(ColorSecondary).
			Foreground(lipgloss.Color("#FFFFFF")).
			Width(m.viewport.Width()).
			Render(line)
	}

	// Normal rendering with colored components
	var styledName string
	if node.graphNode.Metadata["missing"] == "true" {
		styledName = lipgloss.NewStyle().Foreground(ColorMuted).Render(nameWithNamespace)
	} else if node.graphNode.IsRoot {
		styledName = lipgloss.NewStyle().Bold(true).Foreground(ColorWarning).Render(nameWithNamespace)
	} else {
		color := getColorForKind(res.GetKind())
		styledName = lipgloss.NewStyle().Foreground(lipgloss.Color(color)).Render(nameWithNamespace)
	}

	status := formatResourceDesc(node.graphNode)
	line := node.prefix + " " + styledName + "\t" + status

	return line
}

// updateDetailsPanel updates the details viewport for the selected resource
func (m *TreeVisualizerModel) updateDetailsPanel(node *graph.Node) {
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

	// Show revision for Helm releases
	if helmRes, ok := res.(*k8s.HelmRelease); ok && helmRes.HelmRevision > 0 {
		details.WriteString(fmt.Sprintf("Revision: %d\n", helmRes.HelmRevision))
	}

	// Show ArgoCD-specific details
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

	// Show owner references
	if res.GetRaw() != nil && len(res.GetRaw().GetOwnerReferences()) > 0 {
		details.WriteString("\nOwned by:\n")
		for _, owner := range res.GetRaw().GetOwnerReferences() {
			details.WriteString(fmt.Sprintf("  • %s/%s\n", owner.Kind, owner.Name))
		}
	}

	m.detailsViewport.SetContent(details.String())
}

// Update handles updates for the tree visualizer
func (m TreeVisualizerModel) Update(msg tea.Msg) (TreeVisualizerModel, tea.Cmd) {
	var cmd tea.Cmd

	switch msg := msg.(type) {
	case tea.KeyPressMsg:
		if dispatchedCmd, err := m.registry.Dispatch(msg); err == nil {
			switch dispatchedCmd {
			case keys.TreeCmdFocusLeft:
				if m.showDetails {
					m.focusedPanel = FocusTree
				}
				return m, nil

			case keys.TreeCmdFocusRight:
				if m.showDetails {
					m.focusedPanel = FocusDetails
				}
				return m, nil

			case keys.TreeCmdToggleDetails:
				m.showDetails = !m.showDetails
				if !m.showDetails {
					m.focusedPanel = FocusTree
				}
				return m, nil

			case keys.TreeCmdBack:
				// Let parent handle back
				return m, nil
			}

			// Route navigation commands to the focused panel
			if m.focusedPanel == FocusTree {
				switch dispatchedCmd {
				case keys.TreeCmdMoveUp:
					m.navigateUp()
					m.updateViewportContent()
					return m, nil

				case keys.TreeCmdMoveDown:
					m.navigateDown()
					m.updateViewportContent()
					return m, nil

				case keys.TreeCmdHome:
					m.navigateTop()
					m.updateViewportContent()
					return m, nil

				case keys.TreeCmdEnd:
					m.navigateBottom()
					m.updateViewportContent()
					return m, nil

				case keys.TreeCmdPageUp:
					m.pageUp()
					m.updateViewportContent()
					return m, nil

				case keys.TreeCmdPageDown:
					m.pageDown()
					m.updateViewportContent()
					return m, nil

				case keys.TreeCmdToggle:
					m.toggleCurrentNode()
					return m, nil

				case keys.TreeCmdExpandAll:
					m.expandAll()
					return m, nil

				case keys.TreeCmdCollapseAll:
					m.collapseAll()
					return m, nil
				}
			}
		}
	}

	// Handle viewport updates
	if m.focusedPanel == FocusTree {
		m.viewport, cmd = m.viewport.Update(msg)
	} else if m.focusedPanel == FocusDetails && m.showDetails {
		m.detailsViewport, cmd = m.detailsViewport.Update(msg)
	}

	return m, cmd
}

// Navigation methods
func (m *TreeVisualizerModel) navigateUp() {
	if m.selectedIndex > 0 {
		m.selectedIndex--
		m.ensureSelectedVisible()
	}
}

func (m *TreeVisualizerModel) navigateDown() {
	if m.selectedIndex < len(m.flattenedNodes)-1 {
		m.selectedIndex++
		m.ensureSelectedVisible()
	}
}

func (m *TreeVisualizerModel) navigateTop() {
	m.selectedIndex = 0
	m.viewport.GotoTop()
}

func (m *TreeVisualizerModel) navigateBottom() {
	m.selectedIndex = len(m.flattenedNodes) - 1
	m.ensureSelectedVisible()
}

func (m *TreeVisualizerModel) pageUp() {
	m.selectedIndex = max(0, m.selectedIndex-10)
	m.ensureSelectedVisible()
}

func (m *TreeVisualizerModel) pageDown() {
	m.selectedIndex = min(len(m.flattenedNodes)-1, m.selectedIndex+10)
	m.ensureSelectedVisible()
}

func (m *TreeVisualizerModel) ensureSelectedVisible() {
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
func (m *TreeVisualizerModel) toggleCurrentNode() {
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
func (m *TreeVisualizerModel) expandAll() {
	for _, node := range m.graph.Nodes {
		m.expandedNodes[node] = true
	}

	m.rebuildFlattenedTree()
	m.updateViewportContent()
}

// collapseAll collapses all nodes except root nodes
func (m *TreeVisualizerModel) collapseAll() {
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
func (m *TreeVisualizerModel) View() string {
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
func (m *TreeVisualizerModel) renderTreeView() string {
	switchKey := ""
	if entry, ok := m.registry.EntryForCommand(keys.TreeCmdSwitchToGraph); ok {
		switchKey = entry.Display
	}
	title := lipgloss.NewStyle().
		Bold(true).
		Foreground(ColorPrimary).
		Render(fmt.Sprintf("▶ Resource Tree (%s: graph view)", switchKey))

	// Calculate tree panel width
	var treeWidth int
	if !m.showDetails {
		treeWidth = m.width - 2
	} else {
		treeWidth = m.width - m.detailsWidth - 2
	}

	// Highlight border if tree is focused
	borderColor := ColorMuted
	if m.focusedPanel == FocusTree {
		borderColor = ColorPrimary
	}

	treeBox := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(borderColor).
		Padding(0, 1).
		Width(treeWidth).
		Height(m.height - 4)

	return lipgloss.JoinVertical(
		lipgloss.Left,
		title,
		treeBox.Render(m.viewport.View()),
	)
}

// renderDetailsPanel renders the details panel
func (m *TreeVisualizerModel) renderDetailsPanel() string {
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

// HandleCommand handles a tree command directly
func (m *TreeVisualizerModel) HandleCommand(cmd keys.TreeCmd) (*TreeVisualizerModel, tea.Cmd) {
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
			m.focusedPanel = FocusTree
		}
	case keys.TreeCmdFocusRight:
		if m.showDetails {
			m.focusedPanel = FocusDetails
		}
	case keys.TreeCmdToggleDetails:
		m.showDetails = !m.showDetails
		if !m.showDetails {
			m.focusedPanel = FocusTree
		}
	}
	return m, nil
}
