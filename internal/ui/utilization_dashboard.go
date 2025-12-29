package ui

import (
	"fmt"
	"sort"
	"strings"

	"github.com/charmbracelet/bubbles/help"
	"github.com/charmbracelet/bubbles/key"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/miles-w-3/lobot/internal/k8s"
)

// podColors defines a consistent color palette for pod visualization.
// Defined at package level to avoid reallocation on each render call.
var podColors = []lipgloss.Color{
	lipgloss.Color("#00D4AA"), // teal
	lipgloss.Color("#7C6BEE"), // purple
	lipgloss.Color("#FF6B9D"), // pink
	lipgloss.Color("#FFD93D"), // yellow
	lipgloss.Color("#6BCB77"), // green
	lipgloss.Color("#4D96FF"), // blue
	lipgloss.Color("#FF8C42"), // orange
	lipgloss.Color("#C9485B"), // red
	lipgloss.Color("#98D8AA"), // light green
	lipgloss.Color("#C4B7CB"), // lavender
}

// ResourceCategory represents the resource type being viewed
type ResourceCategory int

const (
	ResourceCategoryCPU ResourceCategory = iota
	ResourceCategoryMemory
)

// FocusedPanel represents which panel is focused in the dashboard
type FocusedPanel int

const (
	FocusPanelNodes FocusedPanel = iota
	FocusPanelPods
)

// UtilizationDashboardKeyMap defines key bindings for the utilization dashboard
type UtilizationDashboardKeyMap struct {
	Up          key.Binding
	Down        key.Binding
	Left        key.Binding
	Right       key.Binding
	SwitchPanel key.Binding
	Details     key.Binding
	Back        key.Binding
}

// DefaultUtilizationDashboardKeyMap returns the default key bindings
func DefaultUtilizationDashboardKeyMap() UtilizationDashboardKeyMap {
	return UtilizationDashboardKeyMap{
		Up: key.NewBinding(
			key.WithKeys("up", "k"),
			key.WithHelp("↑/k", "move up"),
		),
		Down: key.NewBinding(
			key.WithKeys("down", "j"),
			key.WithHelp("↓/j", "move down"),
		),
		Left: key.NewBinding(
			key.WithKeys("left", "h"),
			key.WithHelp("←/h", "CPU"),
		),
		Right: key.NewBinding(
			key.WithKeys("right", "l"),
			key.WithHelp("→/l", "Memory"),
		),
		SwitchPanel: key.NewBinding(
			key.WithKeys("tab"),
			key.WithHelp("tab", "switch panel"),
		),
		Details: key.NewBinding(
			key.WithKeys("d"),
			key.WithHelp("d", "node details"),
		),
		Back: key.NewBinding(
			key.WithKeys("esc", "q"),
			key.WithHelp("esc/q", "back"),
		),
	}
}

// ShortHelp returns a short list of key bindings
func (k UtilizationDashboardKeyMap) ShortHelp() []key.Binding {
	return []key.Binding{k.Up, k.Down, k.Left, k.Right, k.Details, k.Back}
}

// FullHelp returns the full list of key bindings organized by category
func (k UtilizationDashboardKeyMap) FullHelp() [][]key.Binding {
	return [][]key.Binding{
		{k.Up, k.Down},
		{k.Left, k.Right},
		{k.SwitchPanel, k.Details},
		{k.Back},
	}
}

// UtilizationDashboardModel manages the utilization dashboard view
type UtilizationDashboardModel struct {
	nodeMetrics      []k8s.NodeMetrics
	podMetrics       []k8s.PodMetrics
	filteredPods     []k8s.PodMetrics
	selectedNode     int // -1 means <All> is selected
	selectedPod      int
	focusedPanel     FocusedPanel
	resourceCategory ResourceCategory
	width            int
	height           int
	keys             UtilizationDashboardKeyMap
	help             help.Model
	showNodeDetails  bool             // Modal for node details
	modalSelectedPod int              // Selected pod within modal
	modalPods        []k8s.PodMetrics // Pods displayed in modal
}

// NewUtilizationDashboardModel creates a new utilization dashboard
func NewUtilizationDashboardModel(nodes []k8s.NodeMetrics, pods []k8s.PodMetrics, width, height int) UtilizationDashboardModel {
	m := UtilizationDashboardModel{
		nodeMetrics:      nodes,
		podMetrics:       pods,
		selectedNode:     -1, // Start with <All> selected
		selectedPod:      0,
		focusedPanel:     FocusPanelNodes,
		resourceCategory: ResourceCategoryCPU,
		width:            width,
		height:           height,
		keys:             DefaultUtilizationDashboardKeyMap(),
		help:             help.New(),
		showNodeDetails:  false,
		modalSelectedPod: 0,
	}
	m.filterPodsByNode()
	return m
}

// filterPodsByNode updates filteredPods based on selected node
func (m *UtilizationDashboardModel) filterPodsByNode() {
	if m.selectedNode == -1 {
		// <All> selected - show all pods
		m.filteredPods = make([]k8s.PodMetrics, len(m.podMetrics))
		copy(m.filteredPods, m.podMetrics)
	} else if m.selectedNode >= 0 && m.selectedNode < len(m.nodeMetrics) {
		selectedNodeName := m.nodeMetrics[m.selectedNode].Name
		m.filteredPods = make([]k8s.PodMetrics, 0)
		for _, pod := range m.podMetrics {
			if pod.NodeName == selectedNodeName {
				m.filteredPods = append(m.filteredPods, pod)
			}
		}
	} else {
		m.filteredPods = nil
	}
	m.selectedPod = 0
}

// getSelectedNode returns the currently selected node, or nil if <All> is selected
func (m *UtilizationDashboardModel) getSelectedNode() *k8s.NodeMetrics {
	if m.selectedNode >= 0 && m.selectedNode < len(m.nodeMetrics) {
		return &m.nodeMetrics[m.selectedNode]
	}
	return nil
}

// resortModalPods re-sorts modal pods when category changes
func (m *UtilizationDashboardModel) resortModalPods() {
	if m.resourceCategory == ResourceCategoryCPU {
		sort.Slice(m.modalPods, func(i, j int) bool {
			return m.modalPods[i].CPUUsage.MilliValue() > m.modalPods[j].CPUUsage.MilliValue()
		})
	} else {
		sort.Slice(m.modalPods, func(i, j int) bool {
			return m.modalPods[i].MemoryUsage.Value() > m.modalPods[j].MemoryUsage.Value()
		})
	}
	m.modalSelectedPod = 0 // Reset selection when re-sorting
}

// Update handles messages for the dashboard
func (m UtilizationDashboardModel) Update(msg tea.Msg) (UtilizationDashboardModel, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		// Handle node details modal
		if m.showNodeDetails {
			switch msg.String() {
			case "esc", "q", "d":
				m.showNodeDetails = false
			case ",", "h": // Previous pod (horizontal)
				if m.modalSelectedPod > 0 {
					m.modalSelectedPod--
				}
			case ".", "l": // Next pod (horizontal)
				if m.modalSelectedPod < len(m.modalPods)-1 {
					m.modalSelectedPod++
				}
			case "left": // Switch to CPU
				m.resourceCategory = ResourceCategoryCPU
				m.resortModalPods()
			case "right": // Switch to Memory
				m.resourceCategory = ResourceCategoryMemory
				m.resortModalPods()
			}
			return m, nil
		}

		switch {
		case key.Matches(msg, m.keys.Up):
			if m.focusedPanel == FocusPanelNodes {
				if m.selectedNode > -1 { // -1 is <All>, minimum
					m.selectedNode--
					m.filterPodsByNode()
				}
			} else {
				if m.selectedPod > 0 {
					m.selectedPod--
				}
			}

		case key.Matches(msg, m.keys.Down):
			if m.focusedPanel == FocusPanelNodes {
				if m.selectedNode < len(m.nodeMetrics)-1 {
					m.selectedNode++
					m.filterPodsByNode()
				}
			} else {
				if m.selectedPod < len(m.filteredPods)-1 {
					m.selectedPod++
				}
			}

		case key.Matches(msg, m.keys.Left):
			m.resourceCategory = ResourceCategoryCPU

		case key.Matches(msg, m.keys.Right):
			m.resourceCategory = ResourceCategoryMemory

		case key.Matches(msg, m.keys.SwitchPanel):
			if m.focusedPanel == FocusPanelNodes {
				m.focusedPanel = FocusPanelPods
			} else {
				m.focusedPanel = FocusPanelNodes
			}

		case key.Matches(msg, m.keys.Details):
			// Show details - works for both specific node and <All>
			if m.focusedPanel == FocusPanelNodes {
				m.modalPods = make([]k8s.PodMetrics, 0)
				if m.selectedNode == -1 {
					// <All> selected - include all pods
					m.modalPods = append(m.modalPods, m.podMetrics...)
				} else if m.selectedNode >= 0 && m.selectedNode < len(m.nodeMetrics) {
					// Specific node selected
					node := m.nodeMetrics[m.selectedNode]
					for _, pod := range m.podMetrics {
						if pod.NodeName == node.Name {
							m.modalPods = append(m.modalPods, pod)
						}
					}
				}
				// Sort by usage descending
				if m.resourceCategory == ResourceCategoryCPU {
					sort.Slice(m.modalPods, func(i, j int) bool {
						return m.modalPods[i].CPUUsage.MilliValue() > m.modalPods[j].CPUUsage.MilliValue()
					})
				} else {
					sort.Slice(m.modalPods, func(i, j int) bool {
						return m.modalPods[i].MemoryUsage.Value() > m.modalPods[j].MemoryUsage.Value()
					})
				}
				m.modalSelectedPod = 0
				m.showNodeDetails = true
			}
		}

	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
	}

	return m, nil
}

// View renders the utilization dashboard
func (m *UtilizationDashboardModel) View() string {
	if m.width < 40 || m.height < 10 {
		return "Terminal too small"
	}

	// If showing node details modal, render that
	if m.showNodeDetails {
		return m.renderNodeDetailsModal()
	}

	// Tab bar header
	tabBar := m.renderTabBar()

	// Calculate panel widths
	totalWidth := m.width - 4
	nodesPanelWidth := totalWidth / 2
	podsPanelWidth := totalWidth - nodesPanelWidth

	// Build panels
	nodesPanel := m.renderNodesPanel(nodesPanelWidth, m.height-10)
	podsPanel := m.renderPodsPanel(podsPanelWidth, m.height-10)

	// Join panels horizontally
	panels := lipgloss.JoinHorizontal(lipgloss.Top, nodesPanel, podsPanel)

	// Footer with help
	footer := m.help.View(m.keys)

	// Build full view
	content := lipgloss.JoinVertical(
		lipgloss.Left,
		tabBar,
		"",
		panels,
		"",
		footer,
	)

	// Wrap in border
	borderStyle := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(ColorBorder).
		Padding(0, 1)

	return borderStyle.Render(content)
}

// renderTabBar renders the tab bar showing CPU/Memory selection
func (m *UtilizationDashboardModel) renderTabBar() string {
	titleStyle := lipgloss.NewStyle().
		Bold(true).
		Foreground(ColorAccent).
		Padding(0, 1)

	// Tab styles
	activeTabStyle := lipgloss.NewStyle().
		Bold(true).
		Foreground(lipgloss.Color("#000000")).
		Background(ColorAccent).
		Padding(0, 2)

	inactiveTabStyle := lipgloss.NewStyle().
		Foreground(ColorMuted).
		Padding(0, 2)

	title := titleStyle.Render("CLUSTER UTILIZATION")

	// Build tabs
	var cpuTab, memTab string
	if m.resourceCategory == ResourceCategoryCPU {
		cpuTab = activeTabStyle.Render("CPU")
		memTab = inactiveTabStyle.Render("Memory")
	} else {
		cpuTab = inactiveTabStyle.Render("CPU")
		memTab = activeTabStyle.Render("Memory")
	}

	tabs := lipgloss.JoinHorizontal(lipgloss.Center, cpuTab, " ", memTab)

	// Join title and tabs
	return lipgloss.JoinHorizontal(lipgloss.Center, title, "  ", tabs)
}

// renderNodesPanel renders the nodes list with bar graphs
func (m *UtilizationDashboardModel) renderNodesPanel(width, height int) string {
	panelStyle := lipgloss.NewStyle().
		Width(width).
		Height(height)

	titleStyle := lipgloss.NewStyle().
		Bold(true).
		Foreground(ColorPrimary).
		MarginBottom(1)

	title := titleStyle.Render("NODES")

	var lines []string
	lines = append(lines, title)

	barWidth := width - 22
	if barWidth < 10 {
		barWidth = 10
	}

	// Add <All> option first
	allPrefix := "  "
	if m.selectedNode == -1 {
		if m.focusedPanel == FocusPanelNodes {
			allPrefix = "▶ "
		} else {
			allPrefix = "● "
		}
	}

	allStyle := lipgloss.NewStyle()
	if m.selectedNode == -1 && m.focusedPanel == FocusPanelNodes {
		allStyle = allStyle.Bold(true).Foreground(ColorAccent)
	}
	lines = append(lines, allStyle.Render(fmt.Sprintf("%s<All Nodes>", allPrefix)))

	// Add each node
	for i, node := range m.nodeMetrics {
		var percentage float64
		if m.resourceCategory == ResourceCategoryCPU {
			percentage = m.calculateCPUPercentage(node)
		} else {
			percentage = m.calculateMemoryPercentage(node)
		}

		bar := m.renderBar(percentage, barWidth)

		prefix := "  "
		if i == m.selectedNode {
			if m.focusedPanel == FocusPanelNodes {
				prefix = "▶ "
			} else {
				prefix = "● "
			}
		}

		nodeName := truncateString(node.Name, 12)
		line := fmt.Sprintf("%s%-12s %s", prefix, nodeName, bar)

		lineStyle := lipgloss.NewStyle()
		if i == m.selectedNode && m.focusedPanel == FocusPanelNodes {
			lineStyle = lineStyle.Bold(true).Foreground(ColorAccent)
		}

		lines = append(lines, lineStyle.Render(line))
	}

	return panelStyle.Render(strings.Join(lines, "\n"))
}

// renderPodsPanel renders the pods list with simple usage values (no bar graphs)
func (m *UtilizationDashboardModel) renderPodsPanel(width, height int) string {
	panelStyle := lipgloss.NewStyle().
		Width(width).
		Height(height)

	titleStyle := lipgloss.NewStyle().
		Bold(true).
		Foreground(ColorPrimary).
		MarginBottom(1)

	var titleText string
	if m.selectedNode == -1 {
		titleText = "PODS (All Nodes)"
	} else if m.selectedNode < len(m.nodeMetrics) {
		titleText = fmt.Sprintf("PODS ON: %s", m.nodeMetrics[m.selectedNode].Name)
	}
	title := titleStyle.Render(titleText)

	var lines []string
	lines = append(lines, title)

	if len(m.filteredPods) == 0 {
		lines = append(lines, lipgloss.NewStyle().Foreground(ColorMuted).Render("  No pods"))
	} else {
		// Limit visible pods based on height
		maxVisible := height - 3
		startIdx := 0
		if m.selectedPod >= maxVisible {
			startIdx = m.selectedPod - maxVisible + 1
		}

		for i := startIdx; i < len(m.filteredPods) && i < startIdx+maxVisible; i++ {
			pod := m.filteredPods[i]

			prefix := "  "
			if i == m.selectedPod {
				if m.focusedPanel == FocusPanelPods {
					prefix = "▶ "
				} else {
					prefix = "● "
				}
			}

			// Show pod name and simple usage values instead of bar graph
			podName := truncateString(pod.Name, 20)
			var usageInfo string
			if m.resourceCategory == ResourceCategoryCPU {
				cpuMillis := pod.CPUUsage.MilliValue()
				if cpuMillis >= 1000 {
					usageInfo = fmt.Sprintf("%.2f cores", float64(cpuMillis)/1000)
				} else {
					usageInfo = fmt.Sprintf("%dm", cpuMillis)
				}
			} else {
				memBytes := pod.MemoryUsage.Value()
				usageInfo = formatBytes(memBytes)
			}

			line := fmt.Sprintf("%s%-20s  %s", prefix, podName, usageInfo)

			lineStyle := lipgloss.NewStyle()
			if i == m.selectedPod && m.focusedPanel == FocusPanelPods {
				lineStyle = lineStyle.Bold(true).Foreground(ColorAccent)
			}

			lines = append(lines, lineStyle.Render(line))
		}
	}

	return panelStyle.Render(strings.Join(lines, "\n"))
}

// renderNodeDetailsModal renders a modal with node details and pod breakdown
func (m *UtilizationDashboardModel) renderNodeDetailsModal() string {
	node := m.getSelectedNode()
	isAllNodes := node == nil // <All Nodes> is selected

	// Build modal content
	var content strings.Builder

	// Tab bar at the top (reusing same style)
	tabBar := m.renderTabBar()
	content.WriteString(tabBar)
	content.WriteString("\n\n")

	// Header and metrics - different for all nodes vs single node
	headerStyle := lipgloss.NewStyle().Bold(true).Foreground(ColorAccent)
	specStyle := lipgloss.NewStyle().Foreground(ColorPrimary)
	labelStyle := lipgloss.NewStyle().Foreground(ColorMuted)

	var cpuUsedMillis, cpuTotalMillis, memUsedBytes, memTotalBytes int64

	if isAllNodes {
		// Aggregate metrics across all nodes
		content.WriteString(headerStyle.Render("All Nodes (Cluster Summary)"))
		content.WriteString("\n\n")

		content.WriteString(specStyle.Render(fmt.Sprintf("Cluster Info (%d nodes)\n", len(m.nodeMetrics))))

		// Aggregate all node metrics
		for _, n := range m.nodeMetrics {
			cpuUsedMillis += n.CPUUsage.MilliValue()
			allocatable := n.CPUAllocatable.MilliValue()
			if allocatable == 0 {
				allocatable = n.CPUCapacity.MilliValue()
			}
			cpuTotalMillis += allocatable

			memUsedBytes += n.MemoryUsage.Value()
			memAllocatable := n.MemAllocatable.Value()
			if memAllocatable == 0 {
				memAllocatable = n.MemoryCapacity.Value()
			}
			memTotalBytes += memAllocatable
		}
		content.WriteString("\n")
	} else {
		// Single node metrics
		content.WriteString(headerStyle.Render(fmt.Sprintf("Node: %s", node.Name)))
		content.WriteString("\n\n")

		content.WriteString(specStyle.Render("System Info\n"))
		content.WriteString(fmt.Sprintf("  %s %s  %s %s  %s %s\n",
			labelStyle.Render("OS:"), node.OS,
			labelStyle.Render("Arch:"), node.Architecture,
			labelStyle.Render("Runtime:"), node.ContainerRuntime))
		content.WriteString("\n")

		cpuUsedMillis = node.CPUUsage.MilliValue()
		cpuTotalMillis = node.CPUAllocatable.MilliValue()
		if cpuTotalMillis == 0 {
			cpuTotalMillis = node.CPUCapacity.MilliValue()
		}

		memUsedBytes = node.MemoryUsage.Value()
		memTotalBytes = node.MemAllocatable.Value()
		if memTotalBytes == 0 {
			memTotalBytes = node.MemoryCapacity.Value()
		}
	}

	// Resource usage section
	content.WriteString(specStyle.Render("Resource Usage\n"))

	// CPU info
	cpuPercent := float64(0)
	if cpuTotalMillis > 0 {
		cpuPercent = float64(cpuUsedMillis) / float64(cpuTotalMillis) * 100
	}
	content.WriteString(fmt.Sprintf("%s   %.2f / %.2f cores (%.1f%%)\n",
		labelStyle.Render("CPU:"),
		float64(cpuUsedMillis)/1000,
		float64(cpuTotalMillis)/1000,
		cpuPercent))

	// Memory info
	memPercent := float64(0)
	if memTotalBytes > 0 {
		memPercent = float64(memUsedBytes) / float64(memTotalBytes) * 100
	}
	content.WriteString(fmt.Sprintf("%s %s / %s (%.1f%%)\n",
		labelStyle.Render("Memory:"),
		formatBytes(memUsedBytes),
		formatBytes(memTotalBytes),
		memPercent))
	content.WriteString("\n")

	// Pod breakdown with stacked bar graph
	categoryLabel := "CPU"
	if m.resourceCategory == ResourceCategoryMemory {
		categoryLabel = "Memory"
	}
	content.WriteString(specStyle.Render(fmt.Sprintf("Pod %s Breakdown (%d pods)\n", categoryLabel, len(m.modalPods))))

	// Render the stacked bar with selection indicator
	barWidth := 60
	stackedBar, selectionLine := m.renderStackedBarWithSelection(barWidth, cpuTotalMillis, memTotalBytes, podColors)
	content.WriteString(stackedBar)
	content.WriteString("\n")
	content.WriteString(selectionLine)
	content.WriteString("\n\n")

	// Show selected pod details (compact)
	if m.modalSelectedPod >= 0 && m.modalSelectedPod < len(m.modalPods) {
		selectedPod := m.modalPods[m.modalSelectedPod]
		colorIdx := m.modalSelectedPod % len(podColors)

		colorBox := lipgloss.NewStyle().Foreground(podColors[colorIdx]).Render("█")

		detailStyle := lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(podColors[colorIdx]).
			Padding(0, 1)

		var detailContent strings.Builder
		detailContent.WriteString(fmt.Sprintf("%s %s\n", colorBox, specStyle.Render(selectedPod.Name)))

		// Show usage percentage and actual values
		if m.resourceCategory == ResourceCategoryCPU {
			cpuMillis := selectedPod.CPUUsage.MilliValue()
			percentage := float64(0)
			if cpuTotalMillis > 0 {
				percentage = float64(cpuMillis) / float64(cpuTotalMillis) * 100
			}
			detailContent.WriteString(fmt.Sprintf("  %s %.1f%% • %.2f cores",
				labelStyle.Render("Usage:"), percentage, float64(cpuMillis)/1000))
		} else {
			memBytes := selectedPod.MemoryUsage.Value()
			percentage := float64(0)
			if memTotalBytes > 0 {
				percentage = float64(memBytes) / float64(memTotalBytes) * 100
			}
			detailContent.WriteString(fmt.Sprintf("  %s %.1f%% • %s",
				labelStyle.Render("Usage:"), percentage, formatBytes(memBytes)))
		}

		content.WriteString(detailStyle.Render(detailContent.String()))
	}

	content.WriteString("\n\n")
	helpText := lipgloss.NewStyle().Foreground(ColorMuted).Italic(true)
	content.WriteString(helpText.Render(",/. select pod  ←→ switch category  d/q close"))

	// Wrap in modal box
	modalWidth := min(m.width-8, 80)
	modalBox := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(ColorAccent).
		Padding(1, 2).
		Width(modalWidth)

	return lipgloss.Place(
		m.width,
		m.height,
		lipgloss.Center,
		lipgloss.Center,
		modalBox.Render(content.String()),
	)
}

// renderStackedBarWithSelection renders a stacked bar with white pointer under selected segment
func (m *UtilizationDashboardModel) renderStackedBarWithSelection(width int, cpuTotal int64, memTotal int64, colors []lipgloss.Color) (string, string) {
	if len(m.modalPods) == 0 {
		emptyStyle := lipgloss.NewStyle().Foreground(ColorMuted)
		bar := "[" + emptyStyle.Render(strings.Repeat("▒", width)) + "]"
		return bar, ""
	}

	var bar strings.Builder
	var selectionLine strings.Builder
	bar.WriteString("[")
	selectionLine.WriteString(" ") // One space to align with "["

	usedWidth := 0
	selectedStart := -1
	selectedWidth := 0

	for i, pod := range m.modalPods {
		var percentage float64
		if m.resourceCategory == ResourceCategoryCPU {
			if cpuTotal > 0 {
				percentage = float64(pod.CPUUsage.MilliValue()) / float64(cpuTotal) * 100
			}
		} else {
			if memTotal > 0 {
				percentage = float64(pod.MemoryUsage.Value()) / float64(memTotal) * 100
			}
		}

		segmentWidth := int(float64(width) * percentage / 100)
		if segmentWidth < 1 && percentage > 0 {
			segmentWidth = 1 // At least 1 char for visible pods
		}

		if usedWidth+segmentWidth > width {
			segmentWidth = width - usedWidth
		}

		if segmentWidth > 0 {
			colorIdx := i % len(colors)
			isSelected := i == m.modalSelectedPod

			// Track selected segment position for underline indicator
			if isSelected {
				selectedStart = usedWidth
				selectedWidth = segmentWidth
			}

			// All bars use solid block character
			segmentStyle := lipgloss.NewStyle().Foreground(colors[colorIdx])
			bar.WriteString(segmentStyle.Render(strings.Repeat("█", segmentWidth)))
			usedWidth += segmentWidth
		}
	}

	// Fill remaining with empty
	if usedWidth < width {
		emptyStyle := lipgloss.NewStyle().Foreground(ColorMuted)
		bar.WriteString(emptyStyle.Render(strings.Repeat("▒", width-usedWidth)))
	}

	bar.WriteString("]")

	// Build selection indicator line with white pointer under selected segment
	if selectedStart >= 0 && selectedWidth > 0 {
		// Add spaces up to selected segment
		selectionLine.WriteString(strings.Repeat(" ", selectedStart))
		// Add pointer character centered under the segment
		pointerStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("#FFFFFF")).Bold(true)
		if selectedWidth >= 3 {
			// Center the pointer for wider segments
			padding := (selectedWidth - 1) / 2
			selectionLine.WriteString(strings.Repeat(" ", padding))
			selectionLine.WriteString(pointerStyle.Render("▲"))
		} else {
			// Just show pointer for narrow segments
			selectionLine.WriteString(pointerStyle.Render("▲"))
		}
	}

	return bar.String(), selectionLine.String()
}

// renderStackedBar renders a single stacked bar with color-coded segments
func (m *UtilizationDashboardModel) renderStackedBar(width int, cpuTotal int64, memTotal int64, colors []lipgloss.Color) string {
	if len(m.modalPods) == 0 {
		emptyStyle := lipgloss.NewStyle().Foreground(ColorMuted)
		return "  [" + emptyStyle.Render(strings.Repeat("▒", width)) + "]"
	}

	var bar strings.Builder
	bar.WriteString("  [")

	usedWidth := 0
	for i, pod := range m.modalPods {
		var percentage float64
		if m.resourceCategory == ResourceCategoryCPU {
			if cpuTotal > 0 {
				percentage = float64(pod.CPUUsage.MilliValue()) / float64(cpuTotal) * 100
			}
		} else {
			if memTotal > 0 {
				percentage = float64(pod.MemoryUsage.Value()) / float64(memTotal) * 100
			}
		}

		segmentWidth := int(float64(width) * percentage / 100)
		if segmentWidth < 1 && percentage > 0 {
			segmentWidth = 1 // At least 1 char for visible pods
		}

		if usedWidth+segmentWidth > width {
			segmentWidth = width - usedWidth
		}

		if segmentWidth > 0 {
			colorIdx := i % len(colors)
			segmentStyle := lipgloss.NewStyle().Foreground(colors[colorIdx])
			bar.WriteString(segmentStyle.Render(strings.Repeat("█", segmentWidth)))
			usedWidth += segmentWidth
		}
	}

	// Fill remaining with empty
	if usedWidth < width {
		emptyStyle := lipgloss.NewStyle().Foreground(ColorMuted)
		bar.WriteString(emptyStyle.Render(strings.Repeat("▒", width-usedWidth)))
	}

	bar.WriteString("]")
	return bar.String()
}

// renderBar renders a horizontal bar graph
func (m *UtilizationDashboardModel) renderBar(percentage float64, width int) string {
	if percentage < 0 {
		percentage = 0
	}
	if percentage > 100 {
		percentage = 100
	}

	filled := int(float64(width) * percentage / 100)
	if filled > width {
		filled = width
	}

	// Choose color based on percentage
	var barColor lipgloss.Color
	if percentage >= 90 {
		barColor = ColorDanger
	} else if percentage >= 70 {
		barColor = ColorWarning
	} else {
		barColor = ColorSuccess
	}

	filledStyle := lipgloss.NewStyle().Foreground(barColor)
	emptyStyle := lipgloss.NewStyle().Foreground(ColorMuted)

	bar := filledStyle.Render(strings.Repeat("█", filled)) +
		emptyStyle.Render(strings.Repeat("▒", width-filled))

	return fmt.Sprintf("[%s]", bar)
}

// calculateCPUPercentage calculates CPU usage percentage for a node
func (m *UtilizationDashboardModel) calculateCPUPercentage(node k8s.NodeMetrics) float64 {
	usageMillis := node.CPUUsage.MilliValue()
	capacityMillis := node.CPUAllocatable.MilliValue()
	if capacityMillis == 0 {
		capacityMillis = node.CPUCapacity.MilliValue()
	}
	if capacityMillis == 0 {
		return 0
	}
	return float64(usageMillis) / float64(capacityMillis) * 100
}

// calculateMemoryPercentage calculates memory usage percentage for a node
func (m *UtilizationDashboardModel) calculateMemoryPercentage(node k8s.NodeMetrics) float64 {
	usageBytes := node.MemoryUsage.Value()
	capacityBytes := node.MemAllocatable.Value()
	if capacityBytes == 0 {
		capacityBytes = node.MemoryCapacity.Value()
	}
	if capacityBytes == 0 {
		return 0
	}
	return float64(usageBytes) / float64(capacityBytes) * 100
}

// GetKeyMap returns the key map for help display
func (m *UtilizationDashboardModel) GetKeyMap() help.KeyMap {
	return m.keys
}

// formatBytes formats bytes into human readable format
func formatBytes(bytes int64) string {
	const (
		KB = 1024
		MB = KB * 1024
		GB = MB * 1024
	)

	if bytes >= GB {
		return fmt.Sprintf("%.1fGi", float64(bytes)/float64(GB))
	} else if bytes >= MB {
		return fmt.Sprintf("%.1fMi", float64(bytes)/float64(MB))
	} else if bytes >= KB {
		return fmt.Sprintf("%.1fKi", float64(bytes)/float64(KB))
	}
	return fmt.Sprintf("%dB", bytes)
}

// truncateString truncates a string to the given length with ellipsis
func truncateString(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	if maxLen <= 3 {
		return s[:maxLen]
	}
	return s[:maxLen-3] + "..."
}
