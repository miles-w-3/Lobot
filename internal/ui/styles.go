package ui

import "github.com/charmbracelet/lipgloss"

var (
	// Brand colors - imported from colors.go
	colorPrimary   = ColorPrimary   // Green from splash
	colorSecondary = ColorSecondary // Purple from splash
	colorAccent    = ColorAccent    // Blue for help/accents

	// Semantic colors
	colorSuccess = ColorSuccess
	colorWarning = ColorWarning
	colorDanger  = ColorDanger

	// UI element colors
	colorMuted  = ColorMuted
	colorBorder = ColorBorder

	// Base styles
	baseStyle = lipgloss.NewStyle().
			Padding(0, 1)

	// Header styles
	headerStyle = lipgloss.NewStyle().
			Bold(true).
			Foreground(colorPrimary)

	titleStyle = lipgloss.NewStyle().
			Bold(true).
			Foreground(colorSecondary)

	// Table styles
	tableHeaderStyle = lipgloss.NewStyle().
				Bold(true).
				Foreground(colorSecondary).
				BorderStyle(lipgloss.NormalBorder()).
				BorderBottom(true).
				BorderForeground(colorBorder).
				Padding(0, 1)

	tableRowStyle = lipgloss.NewStyle().
			Padding(0, 1)

	selectedRowStyle = lipgloss.NewStyle().
				Foreground(lipgloss.Color("#000000")).
				Background(colorPrimary).
				Bold(true).
				Padding(0, 1)

	// Status styles
	statusBarStyle = lipgloss.NewStyle().
			Foreground(colorMuted).
			Padding(0, 1).
			MarginTop(1)

	statusReadyStyle = lipgloss.NewStyle().
				Foreground(colorSuccess).
				Bold(true)

	statusErrorStyle = lipgloss.NewStyle().
				Foreground(colorDanger).
				Bold(true)

	statusInfoStyle = lipgloss.NewStyle().
			Foreground(colorSecondary)

	// Filter styles
	filterBarStyle = lipgloss.NewStyle().
			Foreground(colorSecondary).
			BorderStyle(lipgloss.RoundedBorder()).
			BorderForeground(colorBorder).
			Padding(0, 1).
			MarginTop(1).
			MarginBottom(1)

	// Help text styles
	helpStyle = lipgloss.NewStyle().
			Foreground(colorMuted).
			Italic(true)

	keyStyle = lipgloss.NewStyle().
			Foreground(colorSecondary).
			Bold(true)

	// Resource-specific styles
	podRunningStyle = lipgloss.NewStyle().
			Foreground(colorSuccess)

	podPendingStyle = lipgloss.NewStyle().
			Foreground(colorWarning)

	podFailedStyle = lipgloss.NewStyle().
			Foreground(colorDanger)
)

// GetStatusStyle returns the appropriate style for a resource status
func GetStatusStyle(status string) lipgloss.Style {
	switch status {
	case "Running", "Active", "Available", "True":
		return podRunningStyle
	case "Pending", "Progressing":
		return podPendingStyle
	case "Failed", "Error", "False", "CrashLoopBackOff":
		return podFailedStyle
	default:
		return lipgloss.NewStyle()
	}
}
