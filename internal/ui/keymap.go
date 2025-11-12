package ui

import (
	"github.com/charmbracelet/bubbles/key"
)

// GlobalKeyMap defines key bindings that work in all modes
type GlobalKeyMap struct {
	Quit key.Binding
	Help key.Binding
}

// DefaultGlobalKeyMap returns the default global key bindings
func DefaultGlobalKeyMap() GlobalKeyMap {
	return GlobalKeyMap{
		Quit: key.NewBinding(
			key.WithKeys("ctrl+c"),
			key.WithHelp("ctrl+c", "quit"),
		),
		Help: key.NewBinding(
			key.WithKeys("?"),
			key.WithHelp("?", "toggle help"),
		),
	}
}

// ShortHelp returns a short list of global key bindings
func (k GlobalKeyMap) ShortHelp() []key.Binding {
	return []key.Binding{k.Help, k.Quit}
}

// FullHelp returns the full list of global key bindings
func (k GlobalKeyMap) FullHelp() [][]key.Binding {
	return [][]key.Binding{
		{k.Help, k.Quit},
	}
}

// NormalModeKeyMap defines key bindings for normal mode (resource list view)
type NormalModeKeyMap struct {
	// Navigation
	Up       key.Binding
	Down     key.Binding
	PageUp   key.Binding
	PageDown key.Binding
	Home     key.Binding
	End      key.Binding

	// Resource type navigation
	NextType key.Binding
	PrevType key.Binding

	// Actions
	Enter     key.Binding
	Edit      key.Binding
	Visualize key.Binding
	Filter    key.Binding
	Refresh   key.Binding

	// Selectors
	NamespaceSelector    key.Binding
	ResourceTypeSelector key.Binding
	ContextSelector      key.Binding

	// Exit
	Quit key.Binding
}

// DefaultNormalModeKeyMap returns the default key bindings for normal mode
func DefaultNormalModeKeyMap() NormalModeKeyMap {
	return NormalModeKeyMap{
		// Navigation
		Up: key.NewBinding(
			key.WithKeys("up", "k"),
			key.WithHelp("↑/k", "move up"),
		),
		Down: key.NewBinding(
			key.WithKeys("down", "j"),
			key.WithHelp("↓/j", "move down"),
		),
		PageUp: key.NewBinding(
			key.WithKeys("pgup"),
			key.WithHelp("pgup", "page up"),
		),
		PageDown: key.NewBinding(
			key.WithKeys("pgdown"),
			key.WithHelp("pgdown", "page down"),
		),
		Home: key.NewBinding(
			key.WithKeys("home", "g"),
			key.WithHelp("home/g", "go to top"),
		),
		End: key.NewBinding(
			key.WithKeys("end", "G"),
			key.WithHelp("end/G", "go to bottom"),
		),

		// Resource type navigation
		NextType: key.NewBinding(
			key.WithKeys("tab", "l", "right"),
			key.WithHelp("tab/l/→", "next resource type"),
		),
		PrevType: key.NewBinding(
			key.WithKeys("shift+tab", "h", "left"),
			key.WithHelp("shift+tab/h/←", "previous resource type"),
		),

		// Actions
		Enter: key.NewBinding(
			key.WithKeys("enter"),
			key.WithHelp("enter", "view manifest"),
		),
		Edit: key.NewBinding(
			key.WithKeys("E"),
			key.WithHelp("E", "edit resource"),
		),
		Visualize: key.NewBinding(
			key.WithKeys("V"),
			key.WithHelp("V", "visualize"),
		),
		Filter: key.NewBinding(
			key.WithKeys("/"),
			key.WithHelp("/", "filter by name"),
		),
		Refresh: key.NewBinding(
			key.WithKeys("R"),
			key.WithHelp("R", "refresh resources"),
		),

		// Selectors
		NamespaceSelector: key.NewBinding(
			key.WithKeys("ctrl+n"),
			key.WithHelp("ctrl+n", "select namespace"),
		),
		ResourceTypeSelector: key.NewBinding(
			key.WithKeys("ctrl+t"),
			key.WithHelp("ctrl+t", "select resource type"),
		),
		ContextSelector: key.NewBinding(
			key.WithKeys("ctrl+k"),
			key.WithHelp("ctrl+k", "select context"),
		),

		// Exit
		Quit: key.NewBinding(
			key.WithKeys("q"),
			key.WithHelp("q", "quit"),
		),
	}
}

// ShortHelp returns a short list of key bindings
func (k NormalModeKeyMap) ShortHelp() []key.Binding {
	return []key.Binding{k.Up, k.Down, k.Enter, k.Filter, k.Quit}
}

// FullHelp returns the full list of key bindings organized by category
func (k NormalModeKeyMap) FullHelp() [][]key.Binding {
	return [][]key.Binding{
		{k.Up, k.Down, k.PageUp, k.PageDown, k.Home, k.End},
		{k.NextType, k.PrevType},
		{k.Enter, k.Edit, k.Visualize, k.Filter, k.Refresh},
		{k.NamespaceSelector, k.ResourceTypeSelector, k.ContextSelector},
		{k.Quit},
	}
}

// ManifestModeKeyMap defines key bindings for manifest viewing mode
type ManifestModeKeyMap struct {
	// Navigation
	Up       key.Binding
	Down     key.Binding
	PageUp   key.Binding
	PageDown key.Binding
	Home     key.Binding
	End      key.Binding

	// Actions
	Edit key.Binding
	Copy key.Binding

	// Exit
	Back key.Binding
}

// DefaultManifestModeKeyMap returns the default key bindings for manifest mode
func DefaultManifestModeKeyMap() ManifestModeKeyMap {
	return ManifestModeKeyMap{
		// Navigation (for scrolling)
		Up: key.NewBinding(
			key.WithKeys("up", "k"),
			key.WithHelp("↑/k", "scroll up"),
		),
		Down: key.NewBinding(
			key.WithKeys("down", "j"),
			key.WithHelp("↓/j", "scroll down"),
		),
		PageUp: key.NewBinding(
			key.WithKeys("pgup"),
			key.WithHelp("pgup", "page up"),
		),
		PageDown: key.NewBinding(
			key.WithKeys("pgdown"),
			key.WithHelp("pgdown", "page down"),
		),
		Home: key.NewBinding(
			key.WithKeys("home", "g"),
			key.WithHelp("home/g", "go to top"),
		),
		End: key.NewBinding(
			key.WithKeys("end", "G"),
			key.WithHelp("end/G", "go to bottom"),
		),

		// Actions
		Edit: key.NewBinding(
			key.WithKeys("e"),
			key.WithHelp("e", "edit resource"),
		),
		Copy: key.NewBinding(
			key.WithKeys("c"),
			key.WithHelp("c", "copy to clipboard"),
		),

		// Exit
		Back: key.NewBinding(
			key.WithKeys("esc", "q"),
			key.WithHelp("esc/q", "back to list"),
		),
	}
}

// ShortHelp returns a short list of key bindings
func (k ManifestModeKeyMap) ShortHelp() []key.Binding {
	return []key.Binding{k.Up, k.Down, k.Edit, k.Copy, k.Back}
}

// FullHelp returns the full list of key bindings organized by category
func (k ManifestModeKeyMap) FullHelp() [][]key.Binding {
	return [][]key.Binding{
		{k.Up, k.Down, k.PageUp, k.PageDown, k.Home, k.End},
		{k.Edit, k.Copy},
		{k.Back},
	}
}

// VisualizerModeKeyMap defines key bindings for visualizer mode
type VisualizerModeKeyMap struct {
	// Navigation
	Up     key.Binding
	Down   key.Binding
	Expand key.Binding

	// Panel focus
	FocusLeft  key.Binding
	FocusRight key.Binding

	// Actions
	ToggleDetails key.Binding

	// Exit
	Back key.Binding
}

// DefaultVisualizerModeKeyMap returns the default key bindings for visualizer mode
func DefaultVisualizerModeKeyMap() VisualizerModeKeyMap {
	return VisualizerModeKeyMap{
		// Navigation
		Up: key.NewBinding(
			key.WithKeys("up", "k"),
			key.WithHelp("↑/k", "navigate"),
		),
		Down: key.NewBinding(
			key.WithKeys("down", "j"),
			key.WithHelp("↓/j", "navigate"),
		),
		Expand: key.NewBinding(
			key.WithKeys(" "),
			key.WithHelp("space", "expand/collapse"),
		),

		// Panel focus
		FocusLeft: key.NewBinding(
			key.WithKeys("left", "h"),
			key.WithHelp("←/h", "focus tree"),
		),
		FocusRight: key.NewBinding(
			key.WithKeys("right", "l"),
			key.WithHelp("→/l", "focus details"),
		),

		// Actions
		ToggleDetails: key.NewBinding(
			key.WithKeys("d"),
			key.WithHelp("d", "toggle details panel"),
		),

		// Exit
		Back: key.NewBinding(
			key.WithKeys("esc", "q"),
			key.WithHelp("esc/q", "back to list"),
		),
	}
}

// ShortHelp returns a short list of key bindings
func (k VisualizerModeKeyMap) ShortHelp() []key.Binding {
	return []key.Binding{k.Up, k.Down, k.FocusLeft, k.FocusRight, k.ToggleDetails, k.Back}
}

// FullHelp returns the full list of key bindings organized by category
func (k VisualizerModeKeyMap) FullHelp() [][]key.Binding {
	return [][]key.Binding{
		{k.Up, k.Down, k.Expand},
		{k.FocusLeft, k.FocusRight},
		{k.ToggleDetails},
		{k.Back},
	}
}

// FilterModeKeyMap defines key bindings for filter input mode
type FilterModeKeyMap struct {
	Accept key.Binding
	Cancel key.Binding
}

// DefaultFilterModeKeyMap returns the default key bindings for filter mode
func DefaultFilterModeKeyMap() FilterModeKeyMap {
	return FilterModeKeyMap{
		Accept: key.NewBinding(
			key.WithKeys("enter"),
			key.WithHelp("enter", "apply filter"),
		),
		Cancel: key.NewBinding(
			key.WithKeys("esc"),
			key.WithHelp("esc", "cancel"),
		),
	}
}

// ShortHelp returns a short list of key bindings
func (k FilterModeKeyMap) ShortHelp() []key.Binding {
	return []key.Binding{k.Accept, k.Cancel}
}

// FullHelp returns the full list of key bindings organized by category
func (k FilterModeKeyMap) FullHelp() [][]key.Binding {
	return [][]key.Binding{
		{k.Accept, k.Cancel},
	}
}
