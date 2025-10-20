package ui

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/lipgloss"
)

// View renders the UI
func (m Model) View() string {
	var baseView string

	// Show splash screen if in splash mode
	if m.viewMode == ViewModeSplash {
		baseView = m.splash.View()
	} else if m.err != nil {
		baseView = statusErrorStyle.Render(fmt.Sprintf("Error: %v", m.err))
	} else if m.viewMode == ViewModeManifest {
		// Show manifest viewer if in manifest mode
		baseView = m.renderManifestView()
	} else if m.viewMode == ViewModeVisualize {
		// Show visualizer if in visualize mode
		baseView = m.renderVisualizeView()
	} else {
		// Normal view - full screen with proper layout
		baseView = m.renderNormalView()
	}

	// If selector is visible, render it as an overlay
	if m.selector != nil && m.selector.IsVisible() {
		return m.renderSelectorOverlay(baseView)
	}

	// If modal is visible, render it as an overlay on top of the base view
	if m.alertModal.IsVisible() {
		return m.renderModalOverlay(baseView)
	}

	return baseView
}

// renderNormalView renders the main resource list view
func (m Model) renderNormalView() string {
	// Calculate dimensions
	contentHeight := m.height - 6 // Leave room for status line, header, and help

	// Status line (cluster and resource type)
	statusLine := m.renderStatusLine()

	// Header
	header := m.renderHeader()

	// Main content area with border
	mainContent := m.renderMainContent(contentHeight - 2) // -2 for border

	// Wrap main content in border
	borderedContent := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(colorBorder).
		Width(m.width - 2).
		Height(contentHeight).
		Render(mainContent)

	// Help text at bottom
	help := m.renderHelp()

	// Combine all sections
	return lipgloss.JoinVertical(
		lipgloss.Left,
		header,
		statusLine,
		borderedContent,
		help,
	)
}

// renderMainContent renders the resource table and filter
func (m Model) renderMainContent(height int) string {
	var sections []string

	// Filter bar (if active)
	if m.viewMode == ViewModeFilter {
		sections = append(sections, m.renderFilterBar())
	}

	// Resource table
	sections = append(sections, m.renderResourceTable())

	// Status bar
	sections = append(sections, m.renderStatusBar())

	return lipgloss.JoinVertical(lipgloss.Left, sections...)
}

// renderManifestView renders the manifest viewer
func (m Model) renderManifestView() string {
	resource := m.GetSelectedResource()
	if resource == nil {
		return "No resource selected"
	}

	// Title
	title := titleStyle.Render(fmt.Sprintf("Manifest: %s/%s", resource.Kind, resource.Name))

	// Manifest content in bordered viewport
	viewportContent := m.manifestViewport.View()
	borderedViewport := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(colorBorder).
		Width(m.width - 2).
		Height(m.height - 5).
		Render(viewportContent)

	// Help text
	helpText := helpStyle.Render(fmt.Sprintf(
		"%s/%s scroll | %s back",
		keyStyle.Render("↑/↓"),
		keyStyle.Render("pgup/pgdown"),
		keyStyle.Render("esc"),
	))

	return lipgloss.JoinVertical(
		lipgloss.Left,
		title,
		borderedViewport,
		helpText,
	)
}

// renderStatusLine renders the top status line with cluster and resource type
func (m Model) renderStatusLine() string {
	clusterName := "unknown"
	if m.client != nil {
		if m.client.Context != "" {
			clusterName = m.client.Context
		} else if m.client.ClusterName != "" {
			clusterName = m.client.ClusterName
		}
	}

	currentType := m.CurrentResourceType()

	// Cluster on the left, resource type on the right
	leftStyle := lipgloss.NewStyle().
		Foreground(colorSecondary).
		Bold(true)

	rightStyle := lipgloss.NewStyle().
		Foreground(colorPrimary).
		Bold(true)

	left := leftStyle.Render(fmt.Sprintf("Cluster: %s", clusterName))
	right := rightStyle.Render(fmt.Sprintf("Resource: %s", currentType.DisplayName))

	// Calculate spacing to push right content to the right
	spacing := m.width - lipgloss.Width(left) - lipgloss.Width(right) - 4
	if spacing < 1 {
		spacing = 1
	}

	line := left + strings.Repeat(" ", spacing) + right

	// Add padding to make it more visible
	return lipgloss.NewStyle().
		Render(line)
}

// renderHeader renders the header
func (m Model) renderHeader() string {
	title := titleStyle.Render("Lobot")
	return headerStyle.Render(title)
}

// renderFilterBar renders the filter input bar
func (m Model) renderFilterBar() string {
	label := "Resource name filter: "
	input := m.filterInput.View()
	content := label + input
	return filterBarStyle.Render(content)
}

// renderResourceTable renders the table of resources
func (m Model) renderResourceTable() string {
	if len(m.filteredResources) == 0 {
		return helpStyle.Render("No resources found")
	}

	return m.table.View()
}

// renderStatusBar renders the status bar
func (m Model) renderStatusBar() string {
	parts := []string{}

	// Resource count
	countInfo := fmt.Sprintf("Total: %d", len(m.resources))
	if len(m.filteredResources) != len(m.resources) {
		countInfo += fmt.Sprintf(" | Filtered: %d", len(m.filteredResources))
	}
	parts = append(parts, statusInfoStyle.Render(countInfo))

	// Active filters
	var activeFilters []string

	// Namespace filter
	if pattern := m.namespaceFilter.GetPattern(); pattern != "" {
		activeFilters = append(activeFilters, fmt.Sprintf("ns:%s", pattern))
	}

	// Name filter
	if pattern := m.nameFilter.GetPattern(); pattern != "" {
		activeFilters = append(activeFilters, fmt.Sprintf("name:%s", pattern))
	}

	if len(activeFilters) > 0 {
		filterInfo := fmt.Sprintf("Filters: %s", strings.Join(activeFilters, ", "))
		parts = append(parts, statusInfoStyle.Render(filterInfo))
	}

	return statusBarStyle.Render(strings.Join(parts, " | "))
}

// renderHelp renders the help text
func (m Model) renderHelp() string {
	var helpText string

	switch m.viewMode {
	case ViewModeFilter:
		helpText = fmt.Sprintf("%s apply name filter | %s cancel",
			keyStyle.Render("enter"),
			keyStyle.Render("esc"),
		)
	case ViewModeManifest:
		helpText = fmt.Sprintf(
			"%s/%s scroll | %s edit | %s back",
			keyStyle.Render("↑/↓"),
			keyStyle.Render("pgup/pgdown"),
			keyStyle.Render("e"),
			keyStyle.Render("esc"),
		)
	case ViewModeVisualize:
		helpText = fmt.Sprintf(
			"%s/%s nav | %s expand/collapse | %s toggle details | %s exit",
			keyStyle.Render("↑/↓"),
			keyStyle.Render("j/k"),
			keyStyle.Render("space"),
			keyStyle.Render("d"),
			keyStyle.Render("esc/q"),
		)
	default:
		helpText = fmt.Sprintf(
			"%s/%s nav | %s view | %s edit | %s visualize | %s search | %s namespace | %s type | %s/%s resource | %s quit",
			keyStyle.Render("↑/↓"),
			keyStyle.Render("j/k"),
			keyStyle.Render("enter"),
			keyStyle.Render("shift+e"),
			keyStyle.Render("shift+v"),
			keyStyle.Render("/"),
			keyStyle.Render("ctrl+n"),
			keyStyle.Render("ctrl+t"),
			keyStyle.Render("tab"),
			keyStyle.Render("shift+tab"),
			keyStyle.Render("q"),
		)
	}

	return helpStyle.Render(helpText)
}

// renderModalOverlay renders the modal as an overlay on top of the base view
func (m Model) renderModalOverlay(baseView string) string {
	// Use lipgloss.Place to properly center the modal
	// This handles ANSI codes correctly and prevents streaking
	modalView := m.alertModal.View()

	// Place the modal in the center of the screen, using absolute positioning
	// This overlays on top of the base view
	centeredModal := lipgloss.Place(
		m.width,
		m.height,
		lipgloss.Center,
		lipgloss.Center,
		modalView,
		lipgloss.WithWhitespaceChars(" "),
		lipgloss.WithWhitespaceForeground(lipgloss.Color("#000000")),
	)

	// Now we need to composite the centered modal on top of the base view
	// Split both into lines and overlay
	baseLines := strings.Split(baseView, "\n")
	modalLines := strings.Split(centeredModal, "\n")

	// Ensure we have the same number of lines
	maxLines := max(len(baseLines), len(modalLines))
	outputLines := make([]string, maxLines)

	for i := 0; i < maxLines; i++ {
		var baseLine, modalLine string

		if i < len(baseLines) {
			baseLine = baseLines[i]
		}
		if i < len(modalLines) {
			modalLine = modalLines[i]
		}

		// If modal line is effectively empty (only whitespace/transparent), use base
		// Otherwise use modal line (which will overlay the base)
		trimmed := strings.TrimSpace(stripANSI(modalLine))
		if trimmed == "" {
			outputLines[i] = baseLine
		} else {
			outputLines[i] = modalLine
		}
	}

	return strings.Join(outputLines, "\n")
}

// stripANSI removes ANSI escape codes for checking if line has content
func stripANSI(s string) string {
	// Simple ANSI stripper - matches ESC [ ... m
	result := ""
	inEscape := false
	escapeDepth := 0

	for _, r := range s {
		if r == '\x1b' {
			inEscape = true
			escapeDepth = 0
			continue
		}
		if inEscape {
			escapeDepth++
			if r == 'm' || escapeDepth > 20 {
				inEscape = false
			}
			continue
		}
		result += string(r)
	}
	return result
}

// renderSelectorOverlay renders the selector as an overlay on top of the base view
func (m Model) renderSelectorOverlay(baseView string) string {
	if m.selector == nil {
		return baseView
	}

	selectorView := m.selector.View()

	// Place the selector at the bottom left of the screen
	centeredSelector := lipgloss.Place(
		m.width,
		m.height,
		lipgloss.Left,
		lipgloss.Bottom,
		selectorView,
		lipgloss.WithWhitespaceChars(" "),
		lipgloss.WithWhitespaceForeground(lipgloss.Color("#000000")),
	)

	// Composite with base view
	baseLines := strings.Split(baseView, "\n")
	selectorLines := strings.Split(centeredSelector, "\n")

	maxLines := max(len(baseLines), len(selectorLines))
	outputLines := make([]string, maxLines)

	for i := 0; i < maxLines; i++ {
		var baseLine, selectorLine string

		if i < len(baseLines) {
			baseLine = baseLines[i]
		}
		if i < len(selectorLines) {
			selectorLine = selectorLines[i]
		}

		// If selector line has content, use it; otherwise use base
		trimmed := strings.TrimSpace(stripANSI(selectorLine))
		if trimmed == "" {
			outputLines[i] = baseLine
		} else {
			outputLines[i] = selectorLine
		}
	}

	return strings.Join(outputLines, "\n")
}

// renderVisualizeView renders the visualization mode
func (m Model) renderVisualizeView() string {
	if m.visualizer == nil {
		return "Building resource graph..."
	}

	return m.visualizer.View()
}
