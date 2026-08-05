package modes

import (
	tea "charm.land/bubbletea/v2"
	"github.com/miles-w-3/lobot/internal/command"
)

// Screen is a full-screen mode owned by RootModel. The root is the only
// Bubble Tea model; screens receive lifecycle messages through Update, return
// content and command metadata, and leave terminal policy to the root.
type Screen interface {
	Update(msg tea.Msg) tea.Cmd
	View() string
	CommandPresentation() command.Presentation
}
