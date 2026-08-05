package modes

import tea "charm.land/bubbletea/v2"

// ScreenID identifies a screen that can be constructed by RootModel.
type ScreenID uint8

const (
	ScreenSplash ScreenID = iota
	ScreenHome
	ScreenVisualizer
	ScreenUtilization
	ScreenManifest
)

// NavigateMsg requests a specific screen transition. It remains for lifecycle
// transitions such as Splash completing; ordinary screen exits use BackMsg so
// only RootModel knows their destination.
type NavigateMsg struct {
	Target ScreenID
}

// BackMsg asks RootModel to leave the active screen without exposing its
// destination to that screen.
type BackMsg struct{}

func navigateTo(target ScreenID) tea.Cmd {
	return func() tea.Msg {
		return NavigateMsg{Target: target}
	}
}

func navigateBack() tea.Cmd {
	return func() tea.Msg { return BackMsg{} }
}
