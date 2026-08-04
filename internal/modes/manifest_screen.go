package modes

import (
	"fmt"

	"charm.land/bubbles/v2/viewport"
	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	"github.com/miles-w-3/lobot/internal/command"
	"github.com/miles-w-3/lobot/internal/k8s"
	"github.com/miles-w-3/lobot/internal/keys"
	"github.com/miles-w-3/lobot/internal/util"
)

// ManifestScreen displays a snapshot of one resource's YAML manifest. Editing
// and clipboard access are application workflows owned by RootModel; this
// screen only raises typed requests for them.
type ManifestScreen struct {
	resource k8s.TrackedObject
	content  string
	viewport viewport.Model
	registry *command.Registry[keys.ManifestCmd]
	width    int
	height   int
}

var _ Screen = (*ManifestScreen)(nil)

func NewManifestScreen() *ManifestScreen {
	return &ManifestScreen{
		viewport: viewport.New(),
		registry: keys.NewManifestRegistry(),
	}
}

func (s *ManifestScreen) Update(msg tea.Msg) tea.Cmd {
	switch msg := msg.(type) {
	case ManifestRequestedMsg:
		s.setResource(msg.Resource)
		return nil
	case ManifestEditFinishedMsg:
		if msg.Error == nil && msg.Content != "" {
			s.setContent(msg.Content)
		}
		return nil
	case tea.WindowSizeMsg:
		s.resize(msg.Width, msg.Height)
		return nil
	case tea.KeyPressMsg:
		return s.updateKey(msg)
	default:
		return nil
	}
}

func (s *ManifestScreen) updateKey(msg tea.KeyPressMsg) tea.Cmd {
	cmd, err := s.registry.Dispatch(msg)
	if err != nil {
		return nil
	}

	switch cmd {
	case keys.ManifestCmdBack:
		return navigateTo(ScreenHome)
	case keys.ManifestCmdEdit:
		if s.resource == nil {
			return nil
		}
		return func() tea.Msg {
			return ManifestEditRequestedMsg{Resource: s.resource}
		}
	case keys.ManifestCmdCopy:
		if s.resource == nil {
			return nil
		}
		return func() tea.Msg {
			return ManifestCopyRequestedMsg{Resource: s.resource}
		}
	case keys.ManifestCmdScrollUp:
		s.viewport.ScrollUp(1)
	case keys.ManifestCmdScrollDown:
		s.viewport.ScrollDown(1)
	case keys.ManifestCmdPageUp:
		s.viewport.HalfPageUp()
	case keys.ManifestCmdPageDown:
		s.viewport.HalfPageDown()
	}
	return nil
}

func (s *ManifestScreen) setResource(resource k8s.TrackedObject) {
	s.resource = resource
	if resource == nil || resource.GetRaw() == nil {
		s.content = "No resource selected"
	} else {
		s.content = formatManifest(resource)
	}
	s.setContent(s.content)
}

func (s *ManifestScreen) setContent(content string) {
	s.content = content
	s.viewport.SetContent(content)
	s.viewport.GotoTop()
}

func (s *ManifestScreen) resize(width, height int) {
	s.width = width
	s.height = height
	s.viewport.SetWidth(max(1, width-4))
	s.viewport.SetHeight(max(1, height-4))
}

func (s *ManifestScreen) View() string {
	title := lipgloss.NewStyle().
		Bold(true).
		Foreground(util.ColorPrimary).
		Render(s.title())

	viewportHeight := max(1, s.height-2)
	bordered := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(util.ColorBorder).
		Width(max(1, s.width-2)).
		Height(viewportHeight).
		Render(s.viewport.View())

	return lipgloss.JoinVertical(lipgloss.Left, title, bordered)
}

func (s *ManifestScreen) title() string {
	if s.resource == nil {
		return "Manifest"
	}
	return fmt.Sprintf("Manifest: %s/%s", s.resource.GetKind(), s.resource.GetName())
}

func (s *ManifestScreen) CommandPresentation() command.Presentation {
	return s.registry.Presentation()
}
