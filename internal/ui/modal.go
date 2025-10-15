package ui

import (
	"strings"

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
)

// AlertModal represents a modal dialog for showing alerts
type AlertModal struct {
	title       string
	message     string
	modalType   ModalType
	width       int
	height      int
	visible     bool
	detailLines []string // Additional detail lines for long error messages
}

// NewAlertModal creates a new alert modal
func NewAlertModal() *AlertModal {
	return &AlertModal{
		visible: false,
		width:   60,
		height:  10,
	}
}

// Show displays the modal with the given message
func (m *AlertModal) Show(title, message string, modalType ModalType) {
	m.title = title
	m.message = message
	m.modalType = modalType
	m.visible = true

	// Parse message into detail lines if it's multi-line
	m.detailLines = strings.Split(message, "\n")
}

// ShowError is a convenience method for showing error modals
func (m *AlertModal) ShowError(title, message string) {
	m.Show(title, message, ModalTypeError)
}

// ShowSuccess is a convenience method for showing success modals
func (m *AlertModal) ShowSuccess(title, message string) {
	m.Show(title, message, ModalTypeSuccess)
}

// ShowWarning is a convenience method for showing warning modals
func (m *AlertModal) ShowWarning(title, message string) {
	m.Show(title, message, ModalTypeWarning)
}

// ShowInfo is a convenience method for showing info modals
func (m *AlertModal) ShowInfo(title, message string) {
	m.Show(title, message, ModalTypeInfo)
}

// Hide closes the modal
func (m *AlertModal) Hide() {
	m.visible = false
}

// IsVisible returns whether the modal is currently visible
func (m *AlertModal) IsVisible() bool {
	return m.visible
}

// SetSize sets the modal dimensions
func (m *AlertModal) SetSize(width, height int) {
	m.width = width
	m.height = height
}

// Update handles messages for the modal
func (m *AlertModal) Update(msg tea.Msg) (*AlertModal, tea.Cmd) {
	if !m.visible {
		return m, nil
	}

	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "enter", "esc", "q":
			m.Hide()
			return m, nil
		}
	}

	return m, nil
}

// View renders the modal
func (m *AlertModal) View() string {
	if !m.visible {
		return ""
	}

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

	// Modal style with solid black background
	modalStyle := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(borderColor).
		Padding(1, 2).
		Width(m.width).
		Background(lipgloss.Color("#000000")). // Solid black background
		Foreground(lipgloss.Color("#FFFFFF"))

	// Title style
	titleStyle := lipgloss.NewStyle().
		Foreground(borderColor).
		Bold(true).
		Width(m.width - 4)

	// Content style
	contentStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color("#FFFFFF")).
		Width(m.width - 4)

	// Help text style
	helpStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color("#666666")).
		Italic(true).
		Width(m.width - 4).
		Align(lipgloss.Center)

	// Build content
	var content strings.Builder
	content.WriteString(titleStyle.Render(icon + " " + m.title))
	content.WriteString("\n\n")

	// Render message lines
	for i, line := range m.detailLines {
		if i > 0 {
			content.WriteString("\n")
		}
		// Wrap long lines
		wrapped := wrapText(line, m.width-8)
		content.WriteString(contentStyle.Render(wrapped))
	}

	content.WriteString("\n\n")
	content.WriteString(helpStyle.Render("Press Enter or Esc to close"))

	return modalStyle.Render(content.String())
}

// wrapText wraps text to fit within a given width
func wrapText(text string, width int) string {
	if len(text) <= width {
		return text
	}

	var wrapped strings.Builder
	words := strings.Fields(text)
	lineLen := 0

	for i, word := range words {
		wordLen := len(word)
		if lineLen+wordLen+1 > width {
			if i > 0 {
				wrapped.WriteString("\n")
			}
			wrapped.WriteString(word)
			lineLen = wordLen
		} else {
			if i > 0 {
				wrapped.WriteString(" ")
				lineLen++
			}
			wrapped.WriteString(word)
			lineLen += wordLen
		}
	}

	return wrapped.String()
}
