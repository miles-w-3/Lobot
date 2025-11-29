package ui

import (
	"fmt"
	"strings"
	"time"

	"github.com/charmbracelet/lipgloss"
	"github.com/muesli/reflow/ansi"
)

// View renders the UI
func (m Model) View() string {
	var baseView string

	if m.viewMode == ViewModeSplash {
		baseView = m.splash.View()
	} else if m.viewMode == ViewModeManifest {
		baseView = m.renderManifestView()
	} else if m.viewMode == ViewModeVisualize {
		baseView = m.renderVisualizeView()
	} else {
		baseView = m.renderNormalView()
	}

	// If selector is visible, render it as an overlay
	if m.selector != nil && m.selector.IsVisible() {
		return m.renderSelectorOverlay(baseView)
	}

	// If modal is visible (including help modal), render it as an overlay
	if m.modal.IsVisible() {
		return m.renderModalOverlay(baseView)
	}

	return baseView
}

// renderNormalView renders the main resource list view
func (m Model) renderNormalView() string {
	// Calculate dimensions
	contentHeight := m.height - 5

	// Status line (cluster and resource type)
	statusLine := m.renderStatusLine()

	// Main content area with border
	mainContent := m.renderMainContent(contentHeight - 2)

	// Wrap main content in border
	borderedContent := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(colorPrimary).
		Width(m.width - 2).
		Height(contentHeight).
		Render(mainContent)

	// Help text at bottom
	help := m.renderHelp()

	// Combine all sections
	return lipgloss.JoinVertical(
		lipgloss.Left,
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
	// Use the stored manifest resource instead of getting by index
	// This prevents the title from changing when resources are reordered
	resource := m.manifestResource
	if resource == nil {
		return "No resource selected"
	}

	// Title
	title := titleStyle.Render(fmt.Sprintf("Manifest: %s/%s", resource.GetKind(), resource.GetName()))

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
	clusterName := m.resourceService.GetClusterName()
	currentType := m.CurrentResourceType()

	// Cluster on the left
	clusterStyle := lipgloss.NewStyle().
		Foreground(colorPrimary).
		Bold(true)

	// Resource type badge and metadata on the right
	resourceBadgeStyle := lipgloss.NewStyle().
		Foreground(colorSecondary).
		Background(lipgloss.Color("#1a1a1a")).
		Bold(true).
		Padding(0, 1)

	metadataStyle := lipgloss.NewStyle().
		Foreground(colorMuted)

	left := clusterStyle.Render(fmt.Sprintf("▶ %s", clusterName))

	// Build right side with resource type, update time, and refresh interval
	rightParts := []string{
		resourceBadgeStyle.Render(fmt.Sprintf("● %s", currentType.DisplayName)),
	}

	// Add last update time
	lastUpdate := m.resourceService.GetLastUpdateTime(currentType.GVR)
	if !lastUpdate.IsZero() {
		rightParts = append(rightParts, metadataStyle.Render(fmt.Sprintf("updated %s", formatRelativeTime(lastUpdate))))
	}

	// Add refresh interval - all resources use the same 5-minute resync period
	// All updates are event-driven via watch API; resync is just a safety net
	refreshInterval := "resync: 5m"
	rightParts = append(rightParts, metadataStyle.Render(refreshInterval))

	right := strings.Join(rightParts, " • ")

	// Calculate spacing to push right content to the right
	spacing := m.width - lipgloss.Width(left) - ansi.PrintableRuneWidth(right) - 4
	if spacing < 1 {
		spacing = 1
	}

	line := left + strings.Repeat(" ", spacing) + right

	return lipgloss.NewStyle().
		Padding(0, 1).
		MarginBottom(1).
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
	countStyle := lipgloss.NewStyle().Foreground(colorPrimary).Bold(true)
	countInfo := fmt.Sprintf("%d resources", len(m.resources))
	if len(m.filteredResources) != len(m.resources) {
		filteredStyle := lipgloss.NewStyle().Foreground(colorSecondary).Bold(true)
		countInfo += fmt.Sprintf(" • %s", filteredStyle.Render(fmt.Sprintf("%d shown", len(m.filteredResources))))
	}
	parts = append(parts, countStyle.Render(countInfo))

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
		filterStyle := lipgloss.NewStyle().Foreground(colorAccent)
		filterInfo := fmt.Sprintf("filters: %s", strings.Join(activeFilters, ", "))
		parts = append(parts, filterStyle.Render(filterInfo))
	}

	return statusBarStyle.Render(strings.Join(parts, "  "))
}

// renderHelp renders the help text using the KeyMap system
func (m Model) renderHelp() string {
	// Use the modal's help model for rendering
	helpModel := m.modal.helpModel

	// Get the appropriate keymap for current mode
	var helpView string

	switch m.viewMode {
	case ViewModeFilter:
		helpView = helpModel.ShortHelpView(m.filterKeys.ShortHelp())
	case ViewModeManifest:
		// Combine mode-specific and global help
		keys := append(m.manifestKeys.ShortHelp(), m.globalKeys.ShortHelp()...)
		helpView = helpModel.ShortHelpView(keys)
	case ViewModeVisualize:
		keys := append(m.visualizerKeys.ShortHelp(), m.globalKeys.ShortHelp()...)
		helpView = helpModel.ShortHelpView(keys)
	case ViewModeNormal:
		keys := append(m.normalKeys.ShortHelp(), m.globalKeys.ShortHelp()...)
		helpView = helpModel.ShortHelpView(keys)
	default:
		helpView = helpModel.ShortHelpView(m.globalKeys.ShortHelp())
	}

	return helpStyle.Render(helpView)
}

// renderModalOverlay renders the modal as an overlay on top of the base view
func (m Model) renderModalOverlay(baseView string) string {
	modalView := m.modal.View()

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
			// TODO: Handling for other escape eequences
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

// formatRelativeTime formats a time as a relative duration (e.g., "2m ago", "30s ago")
func formatRelativeTime(t time.Time) string {
	if t.IsZero() {
		return "never"
	}

	duration := time.Since(t)

	if duration < time.Second {
		return "just now"
	} else if duration < time.Minute {
		return fmt.Sprintf("%ds ago", int(duration.Seconds()))
	} else if duration < time.Hour {
		return fmt.Sprintf("%dm ago", int(duration.Minutes()))
	} else if duration < 24*time.Hour {
		return fmt.Sprintf("%dh ago", int(duration.Hours()))
	} else {
		return fmt.Sprintf("%dd ago", int(duration.Hours()/24))
	}
}
