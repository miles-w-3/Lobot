package modes

import (
	"fmt"
	"image/color"
	"sort"
	"strings"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/x/ansi"
	"github.com/miles-w-3/lobot/internal/command"
	"github.com/miles-w-3/lobot/internal/k8s"
	"github.com/miles-w-3/lobot/internal/keys"
	"github.com/miles-w-3/lobot/internal/util"
)

// podColors defines a consistent color palette for pod visualization.
// Defined at package level to avoid reallocation on each render call.
var podColors = []color.Color{
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

// UtilizationScreen manages the utilization dashboard view
type UtilizationScreen struct {
	nodeMetrics         []k8s.NodeMetrics
	podMetrics          []k8s.PodMetrics
	filteredPods        []k8s.PodMetrics
	selectedNode        int
	selectedPod         int
	focusedPanel        FocusedPanel
	resourceCategory    ResourceCategory
	width               int
	height              int
	showNodeDetails     bool
	modalSelectedPod    int
	modalPods           []k8s.PodMetrics
	loading             bool
	loadError           error
	utilizationRegistry *command.Registry[keys.UtilizationCmd]
}

var _ Screen = (*UtilizationScreen)(nil)

func newUtilizationScreen(width, height int) *UtilizationScreen {
	return &UtilizationScreen{
		selectedNode:        -1,
		focusedPanel:        FocusPanelNodes,
		resourceCategory:    ResourceCategoryCPU,
		width:               width,
		height:              height,
		utilizationRegistry: keys.NewUtilizationRegistry(),
	}
}

// NewUtilizationScreen creates a new utilization screen.
func NewUtilizationScreen(nodes []k8s.NodeMetrics, pods []k8s.PodMetrics, width, height int) *UtilizationScreen {
	m := newUtilizationScreen(width, height)
	m.nodeMetrics = append([]k8s.NodeMetrics(nil), nodes...)
	m.podMetrics = append([]k8s.PodMetrics(nil), pods...)
	m.filterPodsByNode()
	return m
}

func newLoadingUtilizationScreen(width, height int) *UtilizationScreen {
	m := newUtilizationScreen(width, height)
	m.loading = true
	return m
}

// CommandPresentation exposes the unchanged utilization registry to RootModel.
func (m *UtilizationScreen) CommandPresentation() command.Presentation {
	if m == nil || m.utilizationRegistry == nil {
		return command.Presentation{}
	}
	return m.utilizationRegistry.Presentation()
}

// filterPodsByNode updates filteredPods based on selected node
func (m *UtilizationScreen) filterPodsByNode() {
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
func (m *UtilizationScreen) getSelectedNode() *k8s.NodeMetrics {
	if m.selectedNode >= 0 && m.selectedNode < len(m.nodeMetrics) {
		return &m.nodeMetrics[m.selectedNode]
	}
	return nil
}

// resortModalPods re-sorts modal pods when category changes
func (m *UtilizationScreen) openNodeDetails() {
	var selectedPod *k8s.PodMetrics
	if m.focusedPanel == FocusPanelPods && m.selectedPod >= 0 && m.selectedPod < len(m.filteredPods) {
		selectedPod = &m.filteredPods[m.selectedPod]
	}

	m.showNodeDetails = true
	m.modalPods = make([]k8s.PodMetrics, len(m.filteredPods))
	copy(m.modalPods, m.filteredPods)
	m.resortModalPods()

	if selectedPod != nil {
		for i, pod := range m.modalPods {
			if pod.Name == selectedPod.Name && pod.Namespace == selectedPod.Namespace && pod.NodeName == selectedPod.NodeName {
				m.modalSelectedPod = i
				break
			}
		}
	}
}

func (m *UtilizationScreen) focusNextPanel() {
	if m.showNodeDetails {
		m.selectNextModalPod()
		return
	}
	if m.focusedPanel == FocusPanelNodes {
		m.focusedPanel = FocusPanelPods
	} else {
		m.focusedPanel = FocusPanelNodes
	}
}

func (m *UtilizationScreen) focusPrevPanel() {
	if m.showNodeDetails {
		m.selectPrevModalPod()
		return
	}
	m.focusNextPanel()
}

func (m *UtilizationScreen) selectNextModalPod() {
	if m.modalSelectedPod < len(m.modalPods)-1 {
		m.modalSelectedPod++
	}
}

func (m *UtilizationScreen) selectPrevModalPod() {
	if m.modalSelectedPod > 0 {
		m.modalSelectedPod--
	}
}

func (m *UtilizationScreen) cycleResourceCategory() {
	if m.resourceCategory == ResourceCategoryCPU {
		m.resourceCategory = ResourceCategoryMemory
	} else {
		m.resourceCategory = ResourceCategoryCPU
	}
	if m.showNodeDetails {
		m.resortModalPods()
	}
}

func (m *UtilizationScreen) resortModalPods() {
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

// Update handles messages for the utilization screen. Key presses are
// dispatched exactly once through the screen's unchanged registry.
func (m *UtilizationScreen) Update(msg tea.Msg) tea.Cmd {
	if m == nil {
		return nil
	}

	switch msg := msg.(type) {
	case UtilizationReadyMsg:
		m.loading = false
		m.loadError = msg.Error
		if msg.Error == nil {
			m.nodeMetrics = append([]k8s.NodeMetrics(nil), msg.NodeMetrics...)
			m.podMetrics = append([]k8s.PodMetrics(nil), msg.PodMetrics...)
			m.selectedNode = -1
			m.selectedPod = 0
			m.modalSelectedPod = 0
			m.showNodeDetails = false
			m.filterPodsByNode()
		}
		return nil
	case tea.KeyPressMsg:
		cmd, err := m.utilizationRegistry.Dispatch(msg)
		if err != nil {
			return nil
		}
		return m.handleCommand(cmd)
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
	}

	return nil
}

func (m *UtilizationScreen) handleCommand(cmd keys.UtilizationCmd) tea.Cmd {
	switch cmd {
	case keys.UtilizationCmdBack:
		if m.showNodeDetails {
			m.showNodeDetails = false
			return nil
		}
		return navigateBack()
	case keys.UtilizationCmdScrollUp:
		if m.showNodeDetails {
			m.selectPrevModalPod()
		} else if m.focusedPanel == FocusPanelNodes {
			if m.selectedNode > -1 {
				m.selectedNode--
				m.filterPodsByNode()
			}
		} else if m.selectedPod > 0 {
			m.selectedPod--
		}
	case keys.UtilizationCmdScrollDown:
		if m.showNodeDetails {
			m.selectNextModalPod()
		} else if m.focusedPanel == FocusPanelNodes {
			if m.selectedNode < len(m.nodeMetrics)-1 {
				m.selectedNode++
				m.filterPodsByNode()
			}
		} else if m.selectedPod < len(m.filteredPods)-1 {
			m.selectedPod++
		}
	case keys.UtilizationCmdPageUp:
		if m.showNodeDetails {
			m.modalSelectedPod = max(0, m.modalSelectedPod-10)
		} else if m.focusedPanel == FocusPanelNodes {
			m.selectedNode = max(-1, m.selectedNode-5)
			m.filterPodsByNode()
		} else {
			m.selectedPod = max(0, m.selectedPod-10)
		}
	case keys.UtilizationCmdPageDown:
		if m.showNodeDetails {
			if len(m.modalPods) > 0 {
				m.modalSelectedPod = min(len(m.modalPods)-1, m.modalSelectedPod+10)
			}
		} else if m.focusedPanel == FocusPanelNodes {
			m.selectedNode = min(len(m.nodeMetrics)-1, m.selectedNode+5)
			m.filterPodsByNode()
		} else if len(m.filteredPods) > 0 {
			m.selectedPod = min(len(m.filteredPods)-1, m.selectedPod+10)
		}
	case keys.UtilizationCmdPrevView, keys.UtilizationCmdNextView:
		m.cycleResourceCategory()
	case keys.UtilizationCmdFocusNext:
		m.focusNextPanel()
	case keys.UtilizationCmdFocusPrev:
		m.focusPrevPanel()
	case keys.UtilizationCmdShowDetails:
		m.openNodeDetails()
	}
	return nil
}

// View renders the utilization dashboard
func (m *UtilizationScreen) View() string {
	if m.width < 40 || m.height < 10 {
		return "Terminal too small"
	}
	if m.loading {
		return lipgloss.NewStyle().Foreground(util.ColorMuted).Render("Loading metrics…")
	}
	if m.loadError != nil {
		return lipgloss.NewStyle().Foreground(util.ColorDanger).Render(
			fmt.Sprintf("Unable to load metrics: %v", m.loadError),
		)
	}

	// If showing node details modal, render that
	if m.showNodeDetails {
		return m.renderNodeDetailsModal()
	}

	outerWidth := m.width
	outerHeight := m.height

	outerStyle := utilizationOuterStyle()
	contentWidth := max(1, outerWidth-outerStyle.GetHorizontalFrameSize())
	contentHeight := max(1, outerHeight-outerStyle.GetVerticalFrameSize())
	if contentWidth < 20 {
		contentWidth = 20
	}
	if contentHeight < 6 {
		contentHeight = 6
	}

	tabBar := m.renderTabBar()
	panelHeight := contentHeight - 2 // tab bar + spacer
	if panelHeight < 4 {
		panelHeight = 4
	}

	gap := 1
	panelsWidth := contentWidth - gap
	nodesPanelWidth := panelsWidth / 2
	podsPanelWidth := panelsWidth - nodesPanelWidth

	nodesPanel := m.renderNodesPanel(nodesPanelWidth, panelHeight)
	podsPanel := m.renderPodsPanel(podsPanelWidth, panelHeight)
	panels := lipgloss.JoinHorizontal(lipgloss.Top, nodesPanel, strings.Repeat(" ", gap), podsPanel)

	content := lipgloss.JoinVertical(
		lipgloss.Left,
		tabBar,
		"",
		panels,
	)

	return outerStyle.
		Width(max(1, outerWidth)).
		Height(max(1, outerHeight)).
		Render(content)
}

// renderTabBar renders the tab bar showing CPU/Memory selection
func (m *UtilizationScreen) renderTabBar() string {
	titleStyle := lipgloss.NewStyle().
		Bold(true).
		Foreground(util.ColorAccent).
		Padding(0, 1)

	// Tab styles
	activeTabStyle := lipgloss.NewStyle().
		Bold(true).
		Foreground(lipgloss.Color("#000000")).
		Background(util.ColorAccent).
		Padding(0, 2)

	inactiveTabStyle := lipgloss.NewStyle().
		Foreground(util.ColorMuted).
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
func (m *UtilizationScreen) renderNodesPanel(width, height int) string {
	innerWidth, innerHeight := utilizationPanelContentSize(width, height)

	titleStyle := lipgloss.NewStyle().Bold(true).Foreground(util.ColorPrimary)
	lines := []string{titleStyle.Render("NODES"), ""}

	maxVisible := innerHeight - len(lines)
	if maxVisible < 1 {
		maxVisible = 1
	}

	selectedOption := m.selectedNode + 1 // <All Nodes> is option 0
	startOption := 0
	if selectedOption >= maxVisible {
		startOption = selectedOption - maxVisible + 1
	}
	optionCount := len(m.nodeMetrics) + 1
	endOption := min(optionCount, startOption+maxVisible)

	for option := startOption; option < endOption; option++ {
		if option == 0 {
			lines = append(lines, m.renderAllNodesRow(innerWidth))
			continue
		}
		lines = append(lines, m.renderNodeRow(option-1, innerWidth))
	}

	return utilizationPanelStyle(width, height).Render(strings.Join(lines, "\n"))
}

func utilizationOuterStyle() lipgloss.Style {
	return lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(util.ColorBorder).
		Padding(0, 1)
}

func utilizationPanelFrame() lipgloss.Style {
	return lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(util.ColorBorder).
		Padding(1, 2)
}

func utilizationPanelContentSize(width, height int) (int, int) {
	frame := utilizationPanelFrame()
	return max(1, width-frame.GetHorizontalFrameSize()), max(1, height-frame.GetVerticalFrameSize())
}

func utilizationPanelStyle(width, height int) lipgloss.Style {
	return utilizationPanelFrame().
		Width(max(1, width)).
		Height(max(1, height))
}

func (m *UtilizationScreen) renderAllNodesRow(width int) string {
	prefix := "  "
	if m.selectedNode == -1 {
		if m.focusedPanel == FocusPanelNodes {
			prefix = "▶ "
		} else {
			prefix = "● "
		}
	}

	line := prefix + truncateVisibleString("<All Nodes>", max(1, width-lipgloss.Width(prefix)))
	lineStyle := lipgloss.NewStyle()
	if m.selectedNode == -1 && m.focusedPanel == FocusPanelNodes {
		lineStyle = lineStyle.Bold(true).Foreground(util.ColorAccent)
	}
	return lineStyle.Render(line)
}

func (m *UtilizationScreen) renderNodeRow(index, width int) string {
	width = max(1, width)
	node := m.nodeMetrics[index]
	var percentage float64
	if m.resourceCategory == ResourceCategoryCPU {
		percentage = m.calculateCPUPercentage(node)
	} else {
		percentage = m.calculateMemoryPercentage(node)
	}

	prefix := "  "
	if index == m.selectedNode {
		if m.focusedPanel == FocusPanelNodes {
			prefix = "▶ "
		} else {
			prefix = "● "
		}
	}

	prefixWidth := lipgloss.Width(prefix)
	separatorWidth := 1
	barWidth := 10
	barRenderedWidth := barWidth + 2 // renderBar adds brackets
	nameWidth := width - prefixWidth - separatorWidth - barRenderedWidth
	if nameWidth < 1 {
		nameWidth = 1
	}

	nodeName := truncateVisibleString(node.Name, nameWidth)
	line := fitVisibleLine(fmt.Sprintf("%s%s %s", prefix, nodeName, m.renderBar(percentage, barWidth)), width)

	lineStyle := lipgloss.NewStyle()
	if index == m.selectedNode && m.focusedPanel == FocusPanelNodes {
		lineStyle = lineStyle.Bold(true).Foreground(util.ColorAccent)
	}
	return lineStyle.Render(line)
}

// renderPodsPanel renders the pods list with simple usage values (no bar graphs)
func (m *UtilizationScreen) renderPodsPanel(width, height int) string {
	innerWidth, innerHeight := utilizationPanelContentSize(width, height)

	titleStyle := lipgloss.NewStyle().Bold(true).Foreground(util.ColorPrimary)
	var titleText string
	if m.selectedNode == -1 {
		titleText = "PODS (All Nodes)"
	} else if m.selectedNode < len(m.nodeMetrics) {
		titleText = fmt.Sprintf("PODS ON: %s", m.nodeMetrics[m.selectedNode].Name)
	}

	lines := []string{titleStyle.Render(truncateVisibleString(titleText, innerWidth)), ""}

	if len(m.filteredPods) == 0 {
		lines = append(lines, lipgloss.NewStyle().Foreground(util.ColorMuted).Render("  No pods"))
	} else {
		maxVisible := innerHeight - len(lines)
		if maxVisible < 1 {
			maxVisible = 1
		}
		startIdx := 0
		if m.selectedPod >= maxVisible {
			startIdx = m.selectedPod - maxVisible + 1
		}

		for i := startIdx; i < len(m.filteredPods) && i < startIdx+maxVisible; i++ {
			lines = append(lines, m.renderPodRow(i, innerWidth))
		}
	}

	return utilizationPanelStyle(width, height).Render(strings.Join(lines, "\n"))
}

func (m *UtilizationScreen) renderPodRow(index, width int) string {
	width = max(1, width)
	pod := m.filteredPods[index]

	prefix := "  "
	if index == m.selectedPod {
		if m.focusedPanel == FocusPanelPods {
			prefix = "▶ "
		} else {
			prefix = "● "
		}
	}

	var usageInfo string
	if m.resourceCategory == ResourceCategoryCPU {
		cpuMillis := pod.CPUUsage.MilliValue()
		if cpuMillis >= 1000 {
			usageInfo = fmt.Sprintf("%.2f cores", float64(cpuMillis)/1000)
		} else {
			usageInfo = fmt.Sprintf("%dm", cpuMillis)
		}
	} else {
		usageInfo = formatBytes(pod.MemoryUsage.Value())
	}

	prefixWidth := lipgloss.Width(prefix)
	usageWidth := max(6, lipgloss.Width(usageInfo))
	gapWidth := 2
	nameWidth := width - prefixWidth - gapWidth - usageWidth
	if nameWidth < 1 {
		nameWidth = 1
	}

	podName := padRightVisible(truncateVisibleString(pod.Name, nameWidth), nameWidth)
	usage := padLeftVisible(truncateVisibleString(usageInfo, usageWidth), usageWidth)
	line := fitVisibleLine(fmt.Sprintf("%s%s%s%s", prefix, podName, strings.Repeat(" ", gapWidth), usage), width)

	lineStyle := lipgloss.NewStyle()
	if index == m.selectedPod && m.focusedPanel == FocusPanelPods {
		lineStyle = lineStyle.Bold(true).Foreground(util.ColorAccent)
	}
	return lineStyle.Render(line)
}

// renderNodeDetailsModal renders a modal with node details and pod breakdown
func (m *UtilizationScreen) renderNodeDetailsModal() string {
	node := m.getSelectedNode()
	isAllNodes := node == nil // <All Nodes> is selected

	// Build modal content
	var content strings.Builder

	// Tab bar at the top (reusing same style)
	tabBar := m.renderTabBar()
	content.WriteString(tabBar)
	content.WriteString("\n\n")

	// Header and metrics - different for all nodes vs single node
	headerStyle := lipgloss.NewStyle().Bold(true).Foreground(util.ColorAccent)
	specStyle := lipgloss.NewStyle().Foreground(util.ColorPrimary)
	labelStyle := lipgloss.NewStyle().Foreground(util.ColorMuted)

	var cpuUsedMillis, cpuTotalMillis, memUsedBytes, memTotalBytes int64

	if isAllNodes {
		// Aggregate metrics across all nodes
		content.WriteString(headerStyle.Render("All Nodes (Cluster Summary)"))
		content.WriteString("\n\n")

		content.WriteString(specStyle.Render(fmt.Sprintf("Cluster Info (%d nodes)", len(m.nodeMetrics))))
		content.WriteString("\n")

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

		content.WriteString(specStyle.Render("System Info"))
		content.WriteString("\n")
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
	content.WriteString(specStyle.Render("Resource Usage"))
	content.WriteString("\n")

	// CPU info
	cpuPercent := float64(0)
	if cpuTotalMillis > 0 {
		cpuPercent = float64(cpuUsedMillis) / float64(cpuTotalMillis) * 100
	}
	content.WriteString(renderMetricLine(
		labelStyle,
		"CPU:",
		fmt.Sprintf("%.2f / %.2f cores (%.1f%%)",
			float64(cpuUsedMillis)/1000,
			float64(cpuTotalMillis)/1000,
			cpuPercent),
	))

	// Memory info
	memPercent := float64(0)
	if memTotalBytes > 0 {
		memPercent = float64(memUsedBytes) / float64(memTotalBytes) * 100
	}
	content.WriteString(renderMetricLine(
		labelStyle,
		"Memory:",
		fmt.Sprintf("%s / %s (%.1f%%)",
			formatBytes(memUsedBytes),
			formatBytes(memTotalBytes),
			memPercent),
	))
	content.WriteString("\n")

	// Pod breakdown with stacked bar graph
	categoryLabel := "CPU"
	if m.resourceCategory == ResourceCategoryMemory {
		categoryLabel = "Memory"
	}
	content.WriteString(specStyle.Render(fmt.Sprintf("Pod %s Breakdown (%d pods)", categoryLabel, len(m.modalPods))))
	content.WriteString("\n")

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

	content.WriteString("\n")

	// Wrap in modal box
	modalWidth := min(m.width-8, 80)
	modalBox := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(util.ColorAccent).
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

func renderMetricLine(labelStyle lipgloss.Style, label, value string) string {
	const labelWidth = len("Memory:")
	padding := labelWidth - lipgloss.Width(label)
	if padding < 0 {
		padding = 0
	}
	return "  " + labelStyle.Render(label) + strings.Repeat(" ", padding) + " " + value + "\n"
}

// renderStackedBarWithSelection renders a stacked bar with white pointer under selected segment
func (m *UtilizationScreen) renderStackedBarWithSelection(width int, cpuTotal int64, memTotal int64, colors []color.Color) (string, string) {
	if len(m.modalPods) == 0 {
		emptyStyle := lipgloss.NewStyle().Foreground(util.ColorMuted)
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
		if segmentWidth < 1 {
			segmentWidth = 1 // Keep zero-usage pods visible
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
		emptyStyle := lipgloss.NewStyle().Foreground(util.ColorMuted)
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

// renderBar renders a horizontal bar graph
func (m *UtilizationScreen) renderBar(percentage float64, width int) string {
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
	var barColor color.Color
	if percentage >= 90 {
		barColor = util.ColorDanger
	} else if percentage >= 70 {
		barColor = util.ColorWarning
	} else {
		barColor = util.ColorSuccess
	}

	filledStyle := lipgloss.NewStyle().Foreground(barColor)
	emptyStyle := lipgloss.NewStyle().Foreground(util.ColorMuted)

	bar := filledStyle.Render(strings.Repeat("█", filled)) +
		emptyStyle.Render(strings.Repeat("▒", width-filled))

	return fmt.Sprintf("[%s]", bar)
}

// calculateCPUPercentage calculates CPU usage percentage for a node
func (m *UtilizationScreen) calculateCPUPercentage(node k8s.NodeMetrics) float64 {
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
func (m *UtilizationScreen) calculateMemoryPercentage(node k8s.NodeMetrics) float64 {
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

func truncateVisibleString(s string, maxWidth int) string {
	if maxWidth <= 0 {
		return ""
	}
	return ansi.Truncate(s, maxWidth, "...")
}

func padRightVisible(s string, width int) string {
	padding := width - lipgloss.Width(s)
	if padding <= 0 {
		return s
	}
	return s + strings.Repeat(" ", padding)
}

func padLeftVisible(s string, width int) string {
	padding := width - lipgloss.Width(s)
	if padding <= 0 {
		return s
	}
	return strings.Repeat(" ", padding) + s
}

func fitVisibleLine(s string, width int) string {
	if width <= 0 {
		return ""
	}
	return ansi.Truncate(s, width, "")
}
