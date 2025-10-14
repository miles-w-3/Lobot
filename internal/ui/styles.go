package ui

import "github.com/charmbracelet/lipgloss"

var (
	// Colors
	colorPrimary   = lipgloss.Color("#7D56F4")
	colorSecondary = lipgloss.Color("#00D9FF")
	colorSuccess   = lipgloss.Color("#00FF87")
	colorWarning   = lipgloss.Color("#FFD700")
	colorDanger    = lipgloss.Color("#FF5F87")
	colorMuted     = lipgloss.Color("#626262")
	colorBorder    = lipgloss.Color("#383838")

	// Base styles
	baseStyle = lipgloss.NewStyle().
			Padding(0, 1)

	// Header styles
	headerStyle = lipgloss.NewStyle().
			Bold(true).
			Foreground(colorPrimary).
			Padding(0, 1).
			MarginBottom(1)

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
