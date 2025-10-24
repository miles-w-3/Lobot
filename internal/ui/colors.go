package ui

import "github.com/charmbracelet/lipgloss"

// Brand Colors - extracted from splash screen for consistency
var (
	// Primary brand color - green from the splash screen robot face
	ColorPrimary = lipgloss.Color("#06bf88")

	// Secondary brand color - purple from splash screen "connecting" text
	ColorSecondary = lipgloss.Color("#7D56F4")

	// Accent color - blue used for help highlights and special accents
	ColorAccent = lipgloss.Color("#00D9FF")

	// Semantic colors
	ColorSuccess = lipgloss.Color("#00FF87")
	ColorWarning = lipgloss.Color("#FFD700")
	ColorDanger  = lipgloss.Color("#FF5F87")

	// UI element colors
	ColorMuted  = lipgloss.Color("#626262")
	ColorBorder = lipgloss.Color("#383838")
	ColorText   = lipgloss.Color("#FFFFFF")
)
