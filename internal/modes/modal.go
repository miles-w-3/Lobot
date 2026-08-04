package modes

import (
	"strings"

	"charm.land/bubbles/v2/viewport"
	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	"github.com/miles-w-3/lobot/internal/command"
	"github.com/miles-w-3/lobot/internal/keys"
	"github.com/miles-w-3/lobot/internal/util"
)

const (
	modalMaxWidth  = 80
	modalMaxHeight = 20
)

// ModalModel is a root-owned content overlay. It deliberately does not
// implement tea.Model; RootModel controls its lifecycle and composition.
type ModalModel struct {
	title    string
	visible  bool
	width    int
	height   int
	viewport viewport.Model
	registry *command.Registry[keys.ModalCmd]
}

func NewModalModel() *ModalModel {
	return &ModalModel{
		viewport: viewport.New(),
		registry: keys.NewModalRegistry(),
	}
}

func (m *ModalModel) SetSize(width, height int) {
	m.width = width
	m.height = height

	boxWidth := min(modalMaxWidth, max(1, width-4))
	boxHeight := min(modalMaxHeight, max(1, height-4))
	m.viewport.SetWidth(max(1, boxWidth-6))
	m.viewport.SetHeight(max(1, boxHeight-7))
}

func (m *ModalModel) Show(title, content string) {
	m.title = title
	m.viewport.SetContent(content)
	m.viewport.GotoTop()
	m.visible = true
}

func (m *ModalModel) Hide() {
	m.visible = false
}

func (m *ModalModel) IsVisible() bool {
	return m.visible
}

func (m *ModalModel) Update(msg tea.Msg) tea.Cmd {
	if !m.visible {
		return nil
	}

	key, ok := msg.(tea.KeyPressMsg)
	if !ok {
		return nil
	}

	cmd, err := m.registry.Dispatch(key)
	if err != nil {
		return nil
	}

	switch cmd {
	case keys.ModalCmdBack:
		m.Hide()
	case keys.ModalCmdScrollUp:
		m.viewport.ScrollUp(1)
	case keys.ModalCmdScrollDown:
		m.viewport.ScrollDown(1)
	}
	return nil
}

func (m *ModalModel) View() string {
	if !m.visible {
		return ""
	}

	boxWidth := min(modalMaxWidth, max(1, m.width-4))
	boxHeight := min(modalMaxHeight, max(1, m.height-4))

	title := lipgloss.NewStyle().
		Bold(true).
		Foreground(util.ColorAccent).
		Render(m.title)
	footer := lipgloss.NewStyle().
		Foreground(util.ColorMuted).
		Italic(true).
		Render("Press " + m.closeLabel() + " to close")

	content := lipgloss.JoinVertical(
		lipgloss.Left,
		title,
		"",
		m.viewport.View(),
		"",
		footer,
	)

	return lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(util.ColorAccent).
		Padding(1, 2).
		Width(boxWidth).
		Height(boxHeight).
		Render(content)
}

func (m *ModalModel) closeLabel() string {
	entry, ok := m.registry.EntryForCommand(keys.ModalCmdBack)
	if !ok {
		return ""
	}
	keys := append([]string{entry.Display}, entry.AltKeys...)
	return strings.Join(keys, " / ")
}
