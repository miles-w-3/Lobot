package ui

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/lipgloss"
	"github.com/muesli/reflow/ansi"
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

	// If help is visible, render it as an overlay
	if m.showHelp {
		return m.renderHelpOverlay(baseView)
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

	// Help text - use KeyMap system
	helpText := m.renderHelp()

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

// renderHelp renders the help text using the KeyMap system
func (m Model) renderHelp() string {
	// Get the appropriate keymap for current mode
	var helpView string

	switch m.viewMode {
	case ViewModeFilter:
		helpView = m.help.ShortHelpView(m.filterKeys.ShortHelp())
	case ViewModeManifest:
		// Combine mode-specific and global help
		keys := append(m.manifestKeys.ShortHelp(), m.globalKeys.ShortHelp()...)
		helpView = m.help.ShortHelpView(keys)
	case ViewModeVisualize:
		keys := append(m.visualizerKeys.ShortHelp(), m.globalKeys.ShortHelp()...)
		helpView = m.help.ShortHelpView(keys)
	case ViewModeNormal:
		keys := append(m.normalKeys.ShortHelp(), m.globalKeys.ShortHelp()...)
		helpView = m.help.ShortHelpView(keys)
	default:
		helpView = m.help.ShortHelpView(m.globalKeys.ShortHelp())
	}

	return helpStyle.Render(helpView)
}

// renderModalOverlay renders the modal as an overlay on top of the base view
func (m Model) renderModalOverlay(baseView string) string {
	modalView := m.alertModal.View()

	// Center and overlay the modal on the base view
	return overlayCenter(baseView, modalView, m.width, m.height)
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

	// Place the selector at the bottom left of the screen (fills entire screen)
	positionedSelector := lipgloss.Place(
		m.width,
		m.height,
		lipgloss.Left,
		lipgloss.Bottom,
		selectorView,
	)

	// For selector, we want lipgloss.Place behavior (replaces whole screen)
	// Split both into lines and use selector where it has content
	baseLines := strings.Split(baseView, "\n")
	selectorLines := strings.Split(positionedSelector, "\n")

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

// renderHelpOverlay renders the help menu as an overlay
func (m Model) renderHelpOverlay(baseView string) string {
	// Get mode-specific and global help
	modeHelp := m.GetCurrentModeHelp()

	// Combine all key binding groups
	allGroups := append(modeHelp.FullHelp(), m.globalKeys.FullHelp()...)

	// Render help menu
	helpView := m.help.FullHelpView(allGroups)

	// Create help box with title
	helpTitle := lipgloss.NewStyle().
		Bold(true).
		Foreground(lipgloss.Color("39")).
		Padding(0, 1).
		Render("Help - Press ? to close")

	helpContent := lipgloss.JoinVertical(
		lipgloss.Left,
		helpTitle,
		"",
		helpView,
	)

	// Style the help box
	helpBox := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(lipgloss.Color("39")).
		Padding(1, 2).
		Width(min(80, m.width-4)).
		MaxHeight(m.height - 4).
		Render(helpContent)

	// Center and overlay the help on the base view
	return overlayCenter(baseView, helpBox, m.width, m.height)
}

// overlayCenter overlays content centered on a base view
func overlayCenter(base, overlay string, width, height int) string {
	overlayLines := strings.Split(overlay, "\n")
	overlayHeight := len(overlayLines)
	overlayWidth := 0
	for _, line := range overlayLines {
		w := ansi.PrintableRuneWidth(line)
		if w > overlayWidth {
			overlayWidth = w
		}
	}

	// Calculate centering offsets
	offsetY := (height - overlayHeight) / 2
	offsetX := (width - overlayWidth) / 2

	return overlayAt(base, overlay, offsetX, offsetY, width, height)
}

// overlayAt overlays content at a specific position
func overlayAt(base, overlay string, x, y, width, height int) string {
	baseLines := strings.Split(base, "\n")
	overlayLines := strings.Split(overlay, "\n")

	// Ensure we have enough base lines
	for len(baseLines) < height {
		baseLines = append(baseLines, "")
	}

	result := make([]string, len(baseLines))
	copy(result, baseLines)

	// Overlay each line
	for i, overlayLine := range overlayLines {
		lineIdx := y + i
		if lineIdx >= 0 && lineIdx < len(result) {
			result[lineIdx] = overlayLineAt(result[lineIdx], overlayLine, x)
		}
	}

	return strings.Join(result, "\n")
}

// overlayLineAt inserts overlay string into base line at position x
// Simplified: just replaces the overlay region with spaces + overlay
func overlayLineAt(base, overlay string, x int) string {
	if x < 0 {
		x = 0
	}

	baseWidth := ansi.PrintableRuneWidth(base)
	overlayWidth := ansi.PrintableRuneWidth(overlay)
	overlayEnd := x + overlayWidth

	// If overlay is completely beyond the base, just append with padding
	if x >= baseWidth {
		padding := strings.Repeat(" ", x-baseWidth)
		return base + padding + overlay
	}

	// Rune-by-rune iteration to preserve ANSI codes
	var before, after strings.Builder
	visualPos := 0
	inEscape := false

	for _, r := range base {
		// Track ANSI escape sequences
		if r == '\x1b' {
			inEscape = true
		}

		if inEscape {
			// Always preserve ANSI codes in appropriate section
			if visualPos < x {
				before.WriteRune(r)
			} else if visualPos >= overlayEnd {
				after.WriteRune(r)
			}
			// Check for end of escape sequence (simplified - matches 'm' which ends color codes)
			if r == 'm' {
				inEscape = false
			}
		} else {
			// Visible character - track position and decide where to place it
			if visualPos < x {
				before.WriteRune(r)
			} else if visualPos >= overlayEnd {
				after.WriteRune(r)
			}
			// Only increment visual position for non-ANSI characters
			visualPos++
		}
	}

	return before.String() + overlay + after.String()
}

// countLines returns the number of lines in a string
func countLines(s string) int {
	if s == "" {
		return 0
	}
	return len(strings.Split(s, "\n"))
}
