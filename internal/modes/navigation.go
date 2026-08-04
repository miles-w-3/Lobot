package modes

import tea "charm.land/bubbletea/v2"

// ScreenID identifies a screen that can be constructed by RootModel.
type ScreenID uint8

const (
	ScreenSplash ScreenID = iota
	ScreenHome
)

// NavigateMsg requests a screen transition. The root owns construction and
// lifecycle of the destination screen.
type NavigateMsg struct {
	Target ScreenID
}

func navigateTo(target ScreenID) tea.Cmd {
	return func() tea.Msg {
		return NavigateMsg{Target: target}
	}
}
