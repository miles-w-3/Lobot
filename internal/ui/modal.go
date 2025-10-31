package ui

import (
	"strings"

	"github.com/charmbracelet/bubbles/help"
	"github.com/charmbracelet/bubbles/key"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

// ModalType represents the type of modal
type ModalType int

const (
	ModalTypeError ModalType = iota
	ModalTypeWarning
	ModalTypeInfo
	ModalTypeSuccess
	ModalTypeHelp
)

// Modal represents a unified modal dialog for all modal types
type Modal struct {
	title       string
	message     string
	modalType   ModalType
	width       int
	height      int
	visible     bool
	detailLines []string        // Additional detail lines for long messages
	helpGroups  [][]key.Binding // For help modal
	helpModel   help.Model      // Help renderer for help modal
}

// NewModal creates a new modal
func NewModal() *Modal {
	// Configure help model with better spacing
	h := help.New()
	h.Styles.ShortKey = lipgloss.NewStyle().Foreground(ColorAccent)
	h.Styles.ShortDesc = lipgloss.NewStyle().Foreground(ColorMuted)
	h.Styles.FullKey = lipgloss.NewStyle().Foreground(ColorAccent).Bold(true)
	h.Styles.FullDesc = lipgloss.NewStyle().Foreground(ColorMuted)
	h.Styles.FullSeparator = lipgloss.NewStyle().Foreground(ColorBorder)
	// Set width to accommodate more content - will be constrained by modal size
	h.Width = 150

	return &Modal{
		visible:   false,
		width:     60,
		height:    10,
		helpModel: h,
	}
}

// Show displays the modal with the given message
func (m *Modal) Show(title, message string, modalType ModalType) {
	m.title = title
	m.message = message
	m.modalType = modalType
	m.visible = true
	m.helpGroups = nil // Clear help groups for non-help modals

	// Parse message into detail lines if it's multi-line
	m.detailLines = strings.Split(message, "\n")
}

// ShowError is a convenience method for showing error modals
func (m *Modal) ShowError(title, message string) {
	m.Show(title, message, ModalTypeError)
}

// ShowWarning is a convenience method for showing warning modals
func (m *Modal) ShowWarning(title, message string) {
	m.Show(title, message, ModalTypeWarning)
}

// ShowInfo is a convenience method for showing info modals
func (m *Modal) ShowInfo(title, message string) {
	m.Show(title, message, ModalTypeInfo)
}

// ShowHelp displays a help modal with key bindings
func (m *Modal) ShowHelp(groups [][]key.Binding) {
	m.title = "Help - Press ? to close"
	m.message = ""
	m.modalType = ModalTypeHelp
	m.visible = true
	m.helpGroups = groups
	m.detailLines = nil
}

// Hide closes the modal
func (m *Modal) Hide() {
	m.visible = false
}

// IsVisible returns whether the modal is currently visible
func (m *Modal) IsVisible() bool {
	return m.visible
}

// SetSize sets the modal dimensions
func (m *Modal) SetSize(width, height int) {
	m.width = width
	m.height = height
}

// Update handles messages for the modal
func (m *Modal) Update(msg tea.Msg) (*Modal, tea.Cmd) {
	if !m.visible {
		return m, nil
	}

	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "enter", "esc", "q":
			m.Hide()
			return m, nil
		case "?":
			// Special handling for help modal toggle
			if m.modalType == ModalTypeHelp {
				m.Hide()
				return m, nil
			}
		}
	}

	return m, nil
}

// View renders the modal
func (m *Modal) View() string {
	if !m.visible {
		return ""
	}

	// Render help modal differently
	if m.modalType == ModalTypeHelp {
		return m.renderHelpModal()
	}

	return m.renderAlertModal()
}

// renderAlertModal renders error/warning/info/success modals
func (m *Modal) renderAlertModal() string {
	// Define styles based on modal type
	var borderColor lipgloss.Color
	var icon string

	switch m.modalType {
	case ModalTypeError:
		borderColor = lipgloss.Color("#FF0000")
		icon = "✗"
	case ModalTypeWarning:
		borderColor = lipgloss.Color("#FFA500")
		icon = "⚠"
	case ModalTypeSuccess:
		borderColor = lipgloss.Color("#00FF00")
		icon = "✓"
	case ModalTypeInfo:
		borderColor = lipgloss.Color("#0000FF")
		icon = "ℹ"
	}

	// Title with icon (same pattern as help modal)
	titleStyle := lipgloss.NewStyle().
		Foreground(borderColor).
		Bold(true).
		Padding(0, 1)

	titleText := titleStyle.Render(icon + " " + m.title)

	// Content lines (already split by newlines)
	contentLines := []string{}
	for _, line := range m.detailLines {
		if line != "" {
			contentLines = append(contentLines, line)
		}
	}
	contentText := strings.Join(contentLines, "\n")

	// Help text
	helpStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color("#666666")).
		Italic(true)
	helpText := helpStyle.Render("Press Enter or Esc to close")

	// Join all content vertically (same pattern as help modal)
	modalContent := lipgloss.JoinVertical(
		lipgloss.Left,
		titleText,
		"",
		contentText,
		"",
		helpText,
	)

	// Style the modal box (same approach as help modal - no background, dynamic sizing)
	modalBox := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(borderColor).
		Padding(1, 2).
		Width(min(80, m.width-4)).
		MaxHeight(m.height - 4).
		Render(modalContent)

	return modalBox
}

// renderHelpModal renders the help modal
func (m *Modal) renderHelpModal() string {
	// Render help content
	helpView := m.helpModel.FullHelpView(m.helpGroups)

	// Create help title
	helpTitle := lipgloss.NewStyle().
		Bold(true).
		Foreground(ColorAccent).
		Padding(0, 1).
		Render(m.title)

	// Join title and help content vertically
	helpContent := lipgloss.JoinVertical(
		lipgloss.Left,
		helpTitle,
		"",
		helpView,
	)

	// Calculate max dimensions to use most of the screen
	// Leave small margins on all sides
	maxWidth := m.width - 8
	if maxWidth < 80 {
		maxWidth = 80
	}

	// Use most of the screen height, leaving room for margins
	maxContentHeight := m.height - 8
	if maxContentHeight < 15 {
		maxContentHeight = 15
	}

	// Split content into lines and truncate if needed
	contentLines := strings.Split(helpContent, "\n")
	if len(contentLines) > maxContentHeight {
		contentLines = contentLines[:maxContentHeight]
		// Add truncation indicator
		contentLines = append(contentLines, lipgloss.NewStyle().
			Foreground(lipgloss.Color("#666666")).
			Italic(true).
			Render("... (screen too small to show all shortcuts)"))
	}
	truncatedContent := strings.Join(contentLines, "\n")

	// Style the help box - use most of the screen
	helpBox := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(ColorAccent).
		Padding(1, 2).
		Width(maxWidth).
		Render(truncatedContent)

	return helpBox
}

// Helper function for min
func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
