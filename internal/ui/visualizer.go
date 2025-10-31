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
	tree "github.com/savannahostrowski/tree-bubble"
)

// FocusPanel represents which panel is currently focused
type FocusPanel int

const (
	FocusTree FocusPanel = iota
	FocusDetails
)

// VisualizerModel represents the visualization mode component
type VisualizerModel struct {
	treeView        tree.Model
	detailsViewport viewport.Model
	graph           *graph.ResourceGraph
	width           int
	height          int
	detailsWidth    int
	showDetails     bool
	focusedPanel    FocusPanel
	selectedNode    *graph.Node
	rootResource    *k8s.Resource // The resource that triggered visualization
	keys            VisualizerModeKeyMap
}

// NewVisualizerModel creates a new visualizer model
func NewVisualizerModel(resourceGraph *graph.ResourceGraph, width, height int) VisualizerModel {
	// Build tree nodes from graph, passing root for namespace context
	treeNodes := buildTreeNodesFromGraph(resourceGraph)

	// Calculate dimensions
	detailsWidth := 40
	treeWidth := width - detailsWidth - 4 // Account for borders and padding

	// Create tree view with custom styles
	treeView := tree.New(treeNodes, treeWidth, height-4)

	// Apply custom styles to match application theme
	treeView.Styles = tree.Styles{
		Shapes: lipgloss.NewStyle().
			Margin(0, 0, 0, 0).
			Foreground(ColorSecondary), // Use consistent purple
		Selected: lipgloss.NewStyle().
			Margin(0, 0, 0, 0).
			Background(ColorSecondary). // Use consistent purple background
			Foreground(lipgloss.Color("#FFFFFF")), // White text for better visibility
		Unselected: lipgloss.NewStyle().
			Margin(0, 0, 0, 0).
			Foreground(lipgloss.AdaptiveColor{Light: "#000000", Dark: "#ffffff"}),
		Help: lipgloss.NewStyle().
			Margin(0, 0, 0, 0).
			Foreground(lipgloss.AdaptiveColor{Light: "#000000", Dark: "#ffffff"}),
	}

	// Create details viewport
	detailsViewport := viewport.New(detailsWidth-4, height-8)
	detailsViewport.SetContent("") // Will be populated in renderDetailsPanel

	return VisualizerModel{
		treeView:        treeView,
		detailsViewport: detailsViewport,
		graph:           resourceGraph,
		width:           width,
		height:          height,
		detailsWidth:    detailsWidth,
		showDetails:     true,
		focusedPanel:    FocusTree, // Start with tree focused
		selectedNode:    resourceGraph.Root,
		rootResource:    resourceGraph.Root.Resource,
		keys:            DefaultVisualizerModeKeyMap(),
	}
}

// Update handles updates for the visualizer
func (m VisualizerModel) Update(msg tea.Msg) (VisualizerModel, tea.Cmd) {
	var cmd tea.Cmd

	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch {
		case key.Matches(msg, m.keys.FocusLeft):
			// Focus tree panel
			if m.showDetails {
				m.focusedPanel = FocusTree
			}
			return m, nil
		case key.Matches(msg, m.keys.FocusRight):
			// Focus details panel
			if m.showDetails {
				m.focusedPanel = FocusDetails
			}
			return m, nil
		case key.Matches(msg, m.keys.ToggleDetails):
			// Toggle details panel
			m.showDetails = !m.showDetails
			// If hiding details, focus must be on tree
			if !m.showDetails {
				m.focusedPanel = FocusTree
			}
			return m, nil
		}
	}

	// Route navigation to the focused panel
	if m.focusedPanel == FocusTree {
		// Update tree view with all remaining messages
		m.treeView, cmd = m.treeView.Update(msg)
	} else if m.focusedPanel == FocusDetails && m.showDetails {
		// Update details viewport (for scrolling)
		m.detailsViewport, cmd = m.detailsViewport.Update(msg)
	}

	return m, cmd
}

// View renders the visualizer
func (m *VisualizerModel) View() string {
	// Build the view layout
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

// renderTreeView renders the tree visualization
func (m *VisualizerModel) renderTreeView() string {
	title := lipgloss.NewStyle().
		Bold(true).
		Foreground(ColorPrimary).
		Render("▶ Resource Relationships")

	treeWidth := m.width - m.detailsWidth - 4
	if !m.showDetails {
		treeWidth = m.width - 4
	}

	// Highlight border if tree is focused
	borderColor := ColorMuted
	borderStyle := lipgloss.RoundedBorder()
	if m.focusedPanel == FocusTree {
		borderColor = ColorPrimary // Green when focused
	}

	treeBox := lipgloss.NewStyle().
		Border(borderStyle).
		BorderForeground(borderColor).
		Padding(0, 1).
		Width(treeWidth).
		Height(m.height - 4)

	helpText := lipgloss.NewStyle().
		Foreground(ColorMuted).
		Render("↑/↓/k/j: navigate • space: expand/collapse • ←/→/h/l: switch panel • d: toggle details • esc/q: exit")

	return lipgloss.JoinVertical(
		lipgloss.Left,
		title,
		treeBox.Render(m.treeView.View()),
		helpText,
	)
}

// renderDetailsPanel renders the details panel for the selected resource
func (m *VisualizerModel) renderDetailsPanel() string {
	if m.selectedNode == nil {
		return ""
	}

	res := m.selectedNode.Resource

	// Build details content
	var details strings.Builder
	details.WriteString(lipgloss.NewStyle().Bold(true).Render("Resource Details"))
	details.WriteString("\n\n")
	details.WriteString(fmt.Sprintf("Name: %s\n", res.Name))
	details.WriteString(fmt.Sprintf("Kind: %s\n", res.Kind))
	if res.Namespace != "" {
		details.WriteString(fmt.Sprintf("Namespace: %s\n", res.Namespace))
	}
	details.WriteString(fmt.Sprintf("Status: %s\n", res.Status))
	// Show revision for Helm releases
	if res.IsHelmRelease && res.HelmRevision > 0 {
		details.WriteString(fmt.Sprintf("Revision: %d\n", res.HelmRevision))
	}
	details.WriteString(fmt.Sprintf("Age: %s\n", formatAge(res.Age)))

	// Show owner references
	if res.Raw != nil && len(res.Raw.GetOwnerReferences()) > 0 {
		details.WriteString("\nOwned by:\n")
		for _, owner := range res.Raw.GetOwnerReferences() {
			details.WriteString(fmt.Sprintf("  • %s/%s\n", owner.Kind, owner.Name))
		}
	}

	// Show labels if any
	if len(res.Labels) > 0 {
		details.WriteString("\nLabels:\n")
		for key, value := range res.Labels {
			details.WriteString(fmt.Sprintf("  %s: %s\n", key, truncate(value, 30)))
		}
	}

	// Update viewport content
	m.detailsViewport.SetContent(details.String())

	// Highlight border if details panel is focused
	borderColor := ColorMuted
	if m.focusedPanel == FocusDetails {
		borderColor = ColorPrimary // Green when focused
	}

	detailsBox := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(borderColor).
		Padding(0, 1).
		Width(m.detailsWidth).
		Height(m.height - 4)

	return detailsBox.Render(m.detailsViewport.View())
}

// buildTreeNodesFromGraph converts a resource graph into tree nodes for tree-bubble
func buildTreeNodesFromGraph(resourceGraph *graph.ResourceGraph) []tree.Node {
	// Find root nodes (nodes with no parents)
	rootNodes := findRootNodes(resourceGraph)

	// Build tree nodes recursively, passing root resource for context
	var treeNodes []tree.Node
	visited := make(map[*graph.Node]bool)

	for _, rootNode := range rootNodes {
		treeNode := buildTreeNode(resourceGraph, rootNode, resourceGraph.Root.Resource, visited)
		treeNodes = append(treeNodes, treeNode)
	}

	return treeNodes
}

// findRootNodes finds all nodes in the graph that have no parents
func findRootNodes(resourceGraph *graph.ResourceGraph) []*graph.Node {
	hasParent := make(map[*graph.Node]bool)

	// Mark all nodes that have parents
	for _, edge := range resourceGraph.Edges {
		hasParent[edge.To] = true
	}

	// Find nodes without parents
	var roots []*graph.Node
	for _, node := range resourceGraph.Nodes {
		if !hasParent[node] {
			roots = append(roots, node)
		}
	}

	return roots
}

// buildTreeNode recursively builds a tree node from a graph node
func buildTreeNode(resourceGraph *graph.ResourceGraph, graphNode *graph.Node, rootResource *k8s.Resource, visited map[*graph.Node]bool) tree.Node {
	// Prevent infinite loops
	if visited[graphNode] {
		return tree.Node{
			Value: formatResourceNameWithRoot(graphNode, rootResource, true),
			Desc:  "(circular reference)",
		}
	}
	visited[graphNode] = true

	// Get children
	children := resourceGraph.GetChildren(graphNode)

	// Build tree node
	treeNode := tree.Node{
		Value:    formatResourceNameWithRoot(graphNode, rootResource, graphNode.IsRoot),
		Desc:     formatResourceDesc(graphNode),
		Children: make([]tree.Node, 0, len(children)),
	}

	// Recursively build children
	for _, child := range children {
		childTreeNode := buildTreeNode(resourceGraph, child, rootResource, visited)
		treeNode.Children = append(treeNode.Children, childTreeNode)
	}

	return treeNode
}

// formatResourceNameWithRoot formats a resource name for display in the tree
func formatResourceNameWithRoot(node *graph.Node, rootResource *k8s.Resource, isRoot bool) string {
	res := node.Resource
	name := fmt.Sprintf("%s: %s", res.Kind, res.Name)

	// Add namespace label if needed
	nameWithNamespace := addNamespaceLabel(node, rootResource, name)

	// Check if this is a missing resource
	if node.Metadata["missing"] == "true" {
		// Gray out missing resources
		return lipgloss.NewStyle().
			Foreground(ColorMuted).
			Render("[Missing] " + nameWithNamespace)
	}

	// Highlight the root resource
	if isRoot {
		return lipgloss.NewStyle().
			Bold(true).
			Foreground(ColorWarning). // Yellow highlight for root
			Render(nameWithNamespace + " ●")
	}

	// Color by resource kind
	color := getColorForKind(res.Kind)
	return lipgloss.NewStyle().
		Foreground(lipgloss.Color(color)).
		Render(nameWithNamespace)
}

// addNamespaceLabel adds a namespace label to the resource name if needed
func addNamespaceLabel(node *graph.Node, rootResource *k8s.Resource, baseName string) string {
	// Don't add namespace for cluster-scoped resources
	if node.Resource.Namespace == "" {
		return baseName
	}

	// For Helm releases: show namespace if different from release target namespace
	if rootResource.IsHelmRelease {
		if node.Resource.Namespace != rootResource.Namespace {
			return fmt.Sprintf("%s (ns: %s)", baseName, node.Resource.Namespace)
		}
	} else {
		// For normal resources: show namespace if different from inspected resource
		if !node.IsRoot && node.Resource.Namespace != rootResource.Namespace {
			return fmt.Sprintf("%s (ns: %s)", baseName, node.Resource.Namespace)
		}
	}

	return baseName
}

// formatResourceDesc formats the resource description (status indicator)
func formatResourceDesc(node *graph.Node) string {
	status := node.Resource.Status
	indicator := getStatusIndicator(status)

	style := lipgloss.NewStyle()

	// Color by status
	if strings.Contains(strings.ToLower(status), "running") ||
		strings.Contains(strings.ToLower(status), "ready") ||
		strings.Contains(strings.ToLower(status), "active") {
		style = style.Foreground(ColorSuccess)
	} else if strings.Contains(strings.ToLower(status), "pending") ||
		strings.Contains(strings.ToLower(status), "creating") {
		style = style.Foreground(ColorWarning)
	} else if strings.Contains(strings.ToLower(status), "failed") ||
		strings.Contains(strings.ToLower(status), "error") {
		style = style.Foreground(ColorDanger)
	} else {
		style = style.Foreground(ColorMuted)
	}

	return style.Render(fmt.Sprintf("[%s] %s", status, indicator))
}

// getStatusIndicator returns a visual indicator for the status
func getStatusIndicator(status string) string {
	status = strings.ToLower(status)

	if strings.Contains(status, "running") ||
		strings.Contains(status, "ready") ||
		strings.Contains(status, "active") {
		return "●" // Green dot
	} else if strings.Contains(status, "pending") ||
		strings.Contains(status, "creating") {
		return "◐" // Half-filled circle
	} else if strings.Contains(status, "failed") ||
		strings.Contains(status, "error") {
		return "✗" // X mark
	}

	return "○" // Empty circle for unknown
}

// getColorForKind returns a color code for a given resource kind
func getColorForKind(kind string) string {
	switch kind {
	case "Pod":
		return "39" // Blue
	case "Deployment":
		return "33" // Cyan
	case "ReplicaSet":
		return "37" // Teal
	case "Service":
		return "35" // Purple
	case "StatefulSet":
		return "34" // Blue-purple
	case "DaemonSet":
		return "36" // Cyan-green
	case "Job":
		return "32" // Green
	case "CronJob":
		return "38" // Light blue
	case "ConfigMap":
		return "220" // Yellow
	case "Secret":
		return "208" // Orange
	case "Ingress":
		return "213" // Pink
	case "PersistentVolumeClaim":
		return "178" // Light orange
	case "ServiceAccount":
		return "141" // Lavender
	case "HorizontalPodAutoscaler":
		return "118" // Light green
	default:
		return "252" // Light gray for unknown kinds
	}
}
