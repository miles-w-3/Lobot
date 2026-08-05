package modes

import "github.com/miles-w-3/lobot/internal/command"

// ResourceUpdateMsg indicates that the resource cache has changed.
type ResourceUpdateMsg struct{}

// SplashServiceReadyMsg indicates that informer startup has completed and
// Splash may finish its transition to Home.
type SplashServiceReadyMsg struct{}

// ErrorMsg reports an application-level error to the active screen.
type ErrorMsg struct {
	Error error
}

// PaletteBackMsg is sent when the command palette should close.
type PaletteBackMsg struct{}

// PaletteSelectedMsg is sent when a command is selected from the palette.
type PaletteSelectedMsg struct {
	Entry command.PaletteEntry
}
