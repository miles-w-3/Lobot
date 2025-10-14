package ui

import (
	"fmt"
	"strings"
	"time"

	"github.com/charmbracelet/lipgloss"
	"github.com/miles-w-3/lobot/internal/k8s"
)

// View renders the UI
func (m Model) View() string {
	// Show splash screen if in splash mode
	if m.viewMode == ViewModeSplash {
		return m.splash.View()
	}

	if m.err != nil {
		return statusErrorStyle.Render(fmt.Sprintf("Error: %v", m.err))
	}

	// Show manifest viewer if in manifest mode
	if m.viewMode == ViewModeManifest {
		return m.renderManifestView()
	}

	// Normal view - full screen with proper layout
	return m.renderNormalView()
}

// renderNormalView renders the main resource list view
func (m Model) renderNormalView() string {
	// Calculate dimensions
	contentHeight := m.height - 5 // Leave room for status line, header, and help

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
		statusLine,
		header,
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
		Padding(0, 1).
		Render(line)
}

// renderHeader renders the header with current resource type
func (m Model) renderHeader() string {
	currentType := m.CurrentResourceType()
	title := titleStyle.Render(fmt.Sprintf("Lobot - %s", currentType.DisplayName))
	return headerStyle.Render(title)
}

// renderFilterBar renders the filter input bar
func (m Model) renderFilterBar() string {
	label := "Namespace filter: "
	input := m.filterInput.View()
	content := label + input
	return filterBarStyle.Render(content)
}

// renderResourceTable renders the table of resources
func (m Model) renderResourceTable() string {
	if len(m.filteredResources) == 0 {
		return helpStyle.Render("No resources found")
	}

	// Calculate visible area
	visibleLines := m.height - 10 // Account for all UI elements and borders
	if visibleLines < 1 {
		visibleLines = 10 // Default minimum
	}

	var rows []string

	// Table header
	currentType := m.CurrentResourceType()
	if currentType.Namespaced {
		header := tableHeaderStyle.Render(
			fmt.Sprintf("%-40s %-20s %-15s %-10s",
				"NAME", "NAMESPACE", "STATUS", "AGE"),
		)
		rows = append(rows, header)
	} else {
		header := tableHeaderStyle.Render(
			fmt.Sprintf("%-40s %-15s %-10s",
				"NAME", "STATUS", "AGE"),
		)
		rows = append(rows, header)
	}

	// Calculate visible range
	startIdx := m.scrollOffset
	endIdx := min(startIdx+visibleLines, len(m.filteredResources))

	// Table rows
	for i := startIdx; i < endIdx; i++ {
		resource := m.filteredResources[i]
		row := m.renderResourceRow(&resource, i == m.selectedIndex, currentType.Namespaced)
		rows = append(rows, row)
	}

	// Scroll indicator
	if len(m.filteredResources) > visibleLines {
		scrollInfo := fmt.Sprintf("  [%d-%d of %d]", startIdx+1, endIdx, len(m.filteredResources))
		rows = append(rows, helpStyle.Render(scrollInfo))
	}

	return strings.Join(rows, "\n")
}

// renderResourceRow renders a single resource row
func (m Model) renderResourceRow(resource *k8s.Resource, selected bool, namespaced bool) string {
	if resource == nil {
		return ""
	}

	name := resource.Name
	namespace := resource.Namespace
	status := resource.Status
	age := formatAge(resource.Age)

	statusStyled := GetStatusStyle(status).Render(status)

	var content string
	if namespaced {
		content = fmt.Sprintf("%-40s %-20s %-15s %-10s",
			truncate(name, 40),
			truncate(namespace, 20),
			statusStyled,
			age,
		)
	} else {
		content = fmt.Sprintf("%-40s %-15s %-10s",
			truncate(name, 40),
			statusStyled,
			age,
		)
	}

	if selected {
		return selectedRowStyle.Render(content)
	}
	return tableRowStyle.Render(content)
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

	// Filter status
	if pattern := m.namespaceFilter.GetPattern(); pattern != "" {
		filterInfo := fmt.Sprintf("Filter: %s", pattern)
		parts = append(parts, statusInfoStyle.Render(filterInfo))
	}

	return statusBarStyle.Render(strings.Join(parts, " | "))
}

// renderHelp renders the help text
func (m Model) renderHelp() string {
	var helpText string

	switch m.viewMode {
	case ViewModeFilter:
		helpText = fmt.Sprintf("%s enter to apply | %s esc to cancel",
			keyStyle.Render("↵"),
			keyStyle.Render("esc"),
		)
	default:
		helpText = fmt.Sprintf(
			"%s/%s nav | %s view | %s filter | %s/%s resource | %s quit",
			keyStyle.Render("↑/↓"),
			keyStyle.Render("j/k"),
			keyStyle.Render("enter"),
			keyStyle.Render("/"),
			keyStyle.Render("tab"),
			keyStyle.Render("shift+tab"),
			keyStyle.Render("q"),
		)
	}

	return helpStyle.Render(helpText)
}

// Helper functions

func formatAge(duration time.Duration) string {
	if duration < time.Minute {
		return fmt.Sprintf("%ds", int(duration.Seconds()))
	} else if duration < time.Hour {
		return fmt.Sprintf("%dm", int(duration.Minutes()))
	} else if duration < 24*time.Hour {
		return fmt.Sprintf("%dh", int(duration.Hours()))
	} else {
		return fmt.Sprintf("%dd", int(duration.Hours()/24))
	}
}

func truncate(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen-3] + "..."
}
