//go:build legacyui

package ui

import (
	"fmt"
	"strings"
	"time"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/x/ansi"
)

// View renders the UI
func (m Model) View() tea.View {
	var content string
	if m.viewMode == ViewModeSplash {
		content = m.splash.View()
		if m.splash.IsError() {
			content = m.renderSplashErrorHelp(content)
		}
	} else if m.viewMode == ViewModeManifest {
		content = m.renderManifestView()
	} else if m.viewMode == ViewModeVisualize {
		content = m.renderVisualizeView()
	} else if m.viewMode == ViewModeUtilization {
		content = m.renderUtilizationView()
	} else {
		content = m.renderNormalView()
	}

	if m.selector != nil && m.selector.IsVisible() {
		content = m.renderSelectorOverlay(content)
	}
	if m.modal.IsVisible() {
		content = m.renderModalOverlay(content)
	}
	if m.paletteVisible {
		content = m.renderPaletteOverlay(content)
	}

	view := tea.NewView(content)
	view.AltScreen = true
	if m.viewMode == ViewModeManifest {
		view.MouseMode = tea.MouseModeNone
	} else {
		view.MouseMode = tea.MouseModeCellMotion
	}
	return view
}

// renderSplashErrorHelp renders registry-backed recovery shortcuts on the splash error screen.
func (m Model) renderSplashErrorHelp(baseView string) string {
	if m.width <= 0 || m.height <= 0 {
		return baseView
	}

	help := lipgloss.PlaceHorizontal(m.width, lipgloss.Center, m.renderHelp())
	return overlayAt(baseView, help, 0, m.height-2, m.width, m.height)
}

// renderNormalView renders the main resource list view
func (m Model) renderNormalView() string {
	// Calculate dimensions
	contentHeight := m.height - 5

	// Status line (cluster and resource type)
	statusLine := m.renderStatusLine()

	// Main content area with border
	mainContent := m.renderMainContent(contentHeight - 2)
	// Extend with favorite types details as needed
	if m.showingFavoriteTypes {
		favoriteTypesContent := m.renderFavoriteTypesContent()
		mainContent = lipgloss.JoinHorizontal(lipgloss.Top, favoriteTypesContent, mainContent)
	}

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

func (m Model) renderFavoriteTypesContent() string {
	favoriteTypesBox := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		Padding(0, 1).
		Width(10).
		Height(26)
	return favoriteTypesBox.Render(m.favoriteTypesViewport.View())
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

	// Add error indicator if errors have been logged
	if m.errorTracker != nil && m.errorTracker.HasErrors() {
		errorStyle := lipgloss.NewStyle().
			Foreground(lipgloss.Color("#FF6B6B")).
			Bold(true)
		left += "  " + errorStyle.Render(fmt.Sprintf("⚠ %d errors (see error.log)", m.errorTracker.GetErrorCount()))
	}

	// Build right side with resource type, update time, and refresh interval
	rightParts := []string{
		resourceBadgeStyle.Render(fmt.Sprintf("● %s", currentType.DisplayName)),
	}

	// Add last update time
	lastUpdate := m.resourceService.GetLastUpdateTime(currentType.GVR)
	if !lastUpdate.IsZero() {
		rightParts = append(rightParts, metadataStyle.Render(fmt.Sprintf("updated %s", formatRelativeTime(lastUpdate))))
	}

	// Informers stay current through Kubernetes LIST/WATCH and relist after
	// disconnects; explicit refresh remains available from the command registry.
	rightParts = append(rightParts, metadataStyle.Render("watch: live"))

	right := strings.Join(rightParts, " • ")

	// Calculate spacing to push right content to the right
	spacing := m.width - lipgloss.Width(left) - ansi.StringWidth(right) - 4
	if spacing < 1 {
		spacing = 1
	}

	line := left + strings.Repeat(" ", spacing) + right

	return lipgloss.NewStyle().
		Padding(0, 1).
		MarginBottom(1).
		Render(line)
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
	if !m.resourceService.IsResourceReady(m.CurrentResourceType().GVR) {
		return helpStyle.Render("Loading resources…")
	}
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

// renderHelp renders the help text using the command registry
func (m Model) renderHelp() string {
	separator := m.globalRegistry.ShortSeparator(" • ")

	// Always show global help first
	helpView := m.globalRegistry.ShortView()

	// Special handling for visualizer mode to show mode-specific commands
	if (m.selector == nil || !m.selector.IsVisible()) && m.viewMode == ViewModeVisualize && m.visualizer != nil {
		// Show specific mode commands
		var specificHelp string
		if m.visualizer.mode == VisualizationModeGraph {
			specificHelp = m.graphRegistry.ShortView()
		} else {
			specificHelp = m.treeRegistry.ShortView()
		}

		if specificHelp != "" {
			helpView += separator + specificHelp
		}

		return helpStyle.Render(helpView)
	}

	// Append current mode help if different from global
	// Note: ShortView returns empty string if no help items
	modeHelp := m.CurrentRegistry().ShortView()
	if modeHelp != "" {
		helpView += separator + modeHelp
	}

	return helpStyle.Render(helpView)
}

// renderModalOverlay renders the modal as an overlay on top of the base view
func (m Model) renderModalOverlay(baseView string) string {
	modalView := m.modal.View()

	// Center and overlay the modal on the base view
	return overlayCenter(baseView, modalView, m.width, m.height)
}

// renderPaletteOverlay renders the palette as an overlay
func (m Model) renderPaletteOverlay(baseView string) string {
	paletteView := m.paletteModel.View()
	return overlayCenter(baseView, paletteView, m.width, m.height)
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
		trimmed := strings.TrimSpace(ansi.Strip(selectorLine))
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

	content := m.visualizer.View()
	help := m.renderHelp()

	return lipgloss.JoinVertical(
		lipgloss.Left,
		content,
		help,
	)
}

// renderUtilizationView renders the utilization dashboard
func (m Model) renderUtilizationView() string {
	if m.utilizationDashboard == nil {
		return "Loading metrics..."
	}

	help := m.renderHelp()
	helpHeight := lipgloss.Height(help)
	contentHeight := m.height - helpHeight
	if contentHeight < 10 {
		contentHeight = 10
	}

	dashboard := *m.utilizationDashboard
	dashboard.width = m.width
	dashboard.height = contentHeight
	content := dashboard.View()

	return lipgloss.JoinVertical(
		lipgloss.Left,
		content,
		help,
	)
}

// overlayCenter overlays content centered on a base view
func overlayCenter(base, overlay string, width, height int) string {
	overlayLines := strings.Split(overlay, "\n")
	overlayHeight := len(overlayLines)
	overlayWidth := 0
	for _, line := range overlayLines {
		w := ansi.StringWidth(line)
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

	baseWidth := ansi.StringWidth(base)
	overlayWidth := ansi.StringWidth(overlay)
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
