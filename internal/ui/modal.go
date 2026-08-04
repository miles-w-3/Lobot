//go:build legacyui

package ui

import (
	"image/color"
	"strings"

	"charm.land/bubbles/v2/viewport"
	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	"github.com/miles-w-3/lobot/internal/command"
	"github.com/miles-w-3/lobot/internal/keys"
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
	title          string
	message        string
	modalType      ModalType
	width          int
	height         int
	visible        bool
	detailLines    []string
	globalRegistry *command.Registry[keys.GlobalCmd]
	modalRegistry  *command.Registry[keys.ModalCmd]
	viewport       viewport.Model
}

// NewModal creates a new modal
func NewModal(registry *command.Registry[keys.GlobalCmd]) *Modal {
	return &Modal{
		visible:        false,
		width:          60,
		height:         10,
		globalRegistry: registry,
		modalRegistry:  keys.NewModalRegistry(),
		viewport:       viewport.New(),
	}
}

// Show displays the modal with the given message
func (m *Modal) Show(title, message string, modalType ModalType) {
	m.title = title
	m.message = message
	m.modalType = modalType
	m.visible = true

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

// ShowHelpContent displays a help modal with pre-rendered content
func (m *Modal) ShowHelpContent(content string) {
	m.title = "Help"
	m.message = content
	m.modalType = ModalTypeHelp
	m.visible = true
	m.detailLines = nil
	m.viewport.SetContent(content)
	m.viewport.GotoTop()
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

	// Update viewport size
	// Height: modal height - border(2) - padding(2) - title/gap(2) - footer(1)
	vpHeight := height - 7
	if vpHeight < 0 {
		vpHeight = 0
	}
	m.viewport.SetHeight(vpHeight)

	// Width: modal width - border(2) - padding(4)
	vpWidth := width - 6
	if vpWidth < 0 {
		vpWidth = 0
	}
	m.viewport.SetWidth(vpWidth)
}

// Update handles messages for the modal
func (m *Modal) Update(msg tea.Msg) (*Modal, tea.Cmd) {
	if !m.visible {
		return m, nil
	}

	var cmd tea.Cmd
	switch msg := msg.(type) {
	case tea.KeyPressMsg:
		// Check global registry first for quit/help
		if gcmd, err := m.globalRegistry.Dispatch(msg); err == nil {
			switch gcmd {
			case keys.GlobalCmdQuit, keys.GlobalCmdHelp:
				m.Hide()
				return m, nil
			}
		}

		// Check modal registry for back/scroll commands
		if mcmd, err := m.modalRegistry.Dispatch(msg); err == nil {
			switch mcmd {
			case keys.ModalCmdBack:
				m.Hide()
				return m, nil
			case keys.ModalCmdScrollUp:
				if m.modalType == ModalTypeHelp {
					m.viewport.ScrollUp(1)
				}
				return m, nil
			case keys.ModalCmdScrollDown:
				if m.modalType == ModalTypeHelp {
					m.viewport.ScrollDown(1)
				}
				return m, nil
			}
		}

		// Handle viewport scrolling if this is a help modal
		if m.modalType == ModalTypeHelp {
			m.viewport, cmd = m.viewport.Update(msg)
			return m, cmd
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
	var borderColor color.Color
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
	helpText := helpStyle.Render("Press " + m.closeKeysLabel() + " to close")

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
func (m *Modal) closeKeysLabel() string {
	entry, ok := m.modalRegistry.EntryForCommand(keys.ModalCmdBack)
	if !ok {
		return ""
	}
	labels := append([]string{entry.Display}, entry.AltKeys...)
	return strings.Join(labels, " / ")
}

func (m *Modal) renderHelpModal() string {
	// Create help title
	helpTitle := lipgloss.NewStyle().
		Bold(true).
		Foreground(ColorAccent).
		Padding(0, 1).
		Render(m.title + " — " + m.closeKeysLabel() + " to close")

	// Join title and viewport content
	// Add scroll indicator if needed
	var footer string
	if !m.viewport.AtBottom() {
		footer = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#666666")).
			Bold(true).
			Render("...")
	} else {
		footer = " "
	}

	content := lipgloss.JoinVertical(
		lipgloss.Left,
		helpTitle,
		"",
		m.viewport.View(),
		footer,
	)

	// Style the help box
	helpBox := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(ColorAccent).
		Padding(1, 2).
		Width(m.width - 2).   // Account for border
		Height(m.height - 2). // Account for border
		Render(content)

	return helpBox
}

// Helper function for min
func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
