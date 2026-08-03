package ui

import (
	"fmt"
	"strings"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	"github.com/miles-w-3/lobot/internal/command"
	"github.com/miles-w-3/lobot/internal/graph"
	"github.com/miles-w-3/lobot/internal/k8s"
	"github.com/miles-w-3/lobot/internal/keys"
)

// FocusPanel represents which panel is currently focused
type FocusPanel int

const (
	FocusTree FocusPanel = iota
	FocusDetails
)

// VisualizationMode represents the current visualization mode
type VisualizationMode int

const (
	VisualizationModeTree VisualizationMode = iota
	VisualizationModeGraph
)

// VisualizerModel represents the visualization mode component
type VisualizerModel struct {
	mode            VisualizationMode
	treeVisualizer  TreeVisualizerModel
	graphVisualizer *GraphVisualizerModel
	graph           *graph.ResourceGraph
	width           int
	height          int
	rootResource    k8s.TrackedObject
	treeRegistry    *command.Registry[keys.TreeCmd]
	graphRegistry   *command.Registry[keys.GraphCmd]
}

// NewVisualizerModel creates a new visualizer model
func NewVisualizerModel(
	resourceGraph *graph.ResourceGraph,
	width, height int,
	treeRegistry *command.Registry[keys.TreeCmd],
	graphRegistry *command.Registry[keys.GraphCmd],
) VisualizerModel {
	// Create tree visualizer (default mode)
	treeVisualizer := NewTreeVisualizerModel(resourceGraph, width, height, treeRegistry)

	return VisualizerModel{
		mode:            VisualizationModeTree,
		treeVisualizer:  treeVisualizer,
		graphVisualizer: nil, // Lazy initialization
		graph:           resourceGraph,
		width:           width,
		height:          height,
		rootResource:    resourceGraph.Root.Resource,
		treeRegistry:    treeRegistry,
		graphRegistry:   graphRegistry,
	}
}

// Update handles updates for the visualizer
func (m VisualizerModel) Update(msg tea.Msg) (VisualizerModel, tea.Cmd) {
	var cmd tea.Cmd

	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		if m.graphVisualizer != nil {
			m.graphVisualizer = NewGraphVisualizerModel(m.graph, m.width, m.height, m.graphRegistry)
		}
	}

	// Delegate to active visualizer
	if m.mode == VisualizationModeGraph && m.graphVisualizer != nil {
		updated, cmd := m.graphVisualizer.Update(msg)
		m.graphVisualizer = &updated
		return m, cmd
	}

	// Default to tree visualizer
	m.treeVisualizer, cmd = m.treeVisualizer.Update(msg)
	return m, cmd
}

// HandleCommand executes a command directly (e.g. from command palette)
func (m *VisualizerModel) HandleCommand(cmd keys.VisualizerCmd) tea.Cmd {
	switch cmd {
	case keys.VisualizerCmdToggleMode:
		if m.mode == VisualizationModeTree {
			if m.graphVisualizer == nil {
				m.graphVisualizer = NewGraphVisualizerModel(m.graph, m.width, m.height, m.graphRegistry)
			}
			m.mode = VisualizationModeGraph
		} else {
			m.mode = VisualizationModeTree
		}
		return nil
	case keys.VisualizerCmdBack:
		m.mode = VisualizationModeTree
		return nil
	case keys.VisualizerCmdCenter:
		if m.mode == VisualizationModeGraph && m.graphVisualizer != nil {
			// 0 key logic from graph_visualizer.go
			m.graphVisualizer.selectedIndex = 0
			m.graphVisualizer.ensureSelectedVisible()
			m.graphVisualizer.updateViewportContent()
		}
		return nil
	}
	return nil
}

// View renders the visualizer
func (m *VisualizerModel) View() string {
	// Delegate to active visualizer
	if m.mode == VisualizationModeGraph && m.graphVisualizer != nil {
		return m.graphVisualizer.View()
	}
	return m.treeVisualizer.View()
}

// Helper functions shared by visualizers

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

// addNamespaceLabel adds a namespace label to the resource name if needed
func addNamespaceLabel(node *graph.Node, rootResource k8s.TrackedObject, baseName string) string {
	// Don't add namespace for cluster-scoped resources
	if node.Resource.GetNamespace() == "" {
		return baseName
	}

	// For Helm releases: show namespace if different from release target namespace
	if helmRes, ok := rootResource.(*k8s.HelmRelease); ok {
		if node.Resource.GetNamespace() != helmRes.GetNamespace() {
			return fmt.Sprintf("%s (ns: %s)", baseName, node.Resource.GetNamespace())
		}
	} else {
		// For normal resources: show namespace if different from inspected resource
		if !node.IsRoot && node.Resource.GetNamespace() != rootResource.GetNamespace() {
			return fmt.Sprintf("%s (ns: %s)", baseName, node.Resource.GetNamespace())
		}
	}

	return baseName
}

// formatResourceDesc formats the resource description (status indicator)
func formatResourceDesc(node *graph.Node) string {
	status := node.Resource.GetStatus()
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

// formatResourceDescPlain formats the resource description without styling (for selected rows)
func formatResourceDescPlain(node *graph.Node) string {
	status := node.Resource.GetStatus()
	indicator := getStatusIndicator(status)
	return fmt.Sprintf("[%s] %s", status, indicator)
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
