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

	ToggleShowFavoriteTypes key.Binding

	// Selectors
	NamespaceSelector    key.Binding
	ResourceTypeSelector key.Binding
	ContextSelector      key.Binding

	// Utilization Dashboard
	UtilizationDashboard key.Binding

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
			key.WithKeys("l", "right"),
			key.WithHelp("l/→", "next resource type"),
		),
		PrevType: key.NewBinding(
			key.WithKeys("h", "left"),
			key.WithHelp("h/←", "previous resource type"),
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

		ToggleShowFavoriteTypes: key.NewBinding(
			key.WithKeys("tab"),
			key.WithHelp("tab", "toggle favoirtes"),
		),

		// Utilization Dashboard
		UtilizationDashboard: key.NewBinding(
			key.WithKeys("ctrl+u"),
			key.WithHelp("ctrl+u", "utilization dashboard"),
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
		{k.NamespaceSelector, k.ResourceTypeSelector, k.ContextSelector, k.UtilizationDashboard},
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

// VisualizerModeKeyMap defines key bindings for visualizer mode (base for tree/graph)
type VisualizerModeKeyMap struct {
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
		// Panel focus
		FocusLeft: key.NewBinding(
			key.WithKeys("left", "h"),
			key.WithHelp("←/h", "focus left"),
		),
		FocusRight: key.NewBinding(
			key.WithKeys("right", "l"),
			key.WithHelp("→/l", "focus right"),
		),

		// Actions
		ToggleDetails: key.NewBinding(
			key.WithKeys("d"),
			key.WithHelp("d", "toggle details"),
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
	return []key.Binding{k.FocusLeft, k.FocusRight, k.ToggleDetails, k.Back}
}

// FullHelp returns the full list of key bindings organized by category
func (k VisualizerModeKeyMap) FullHelp() [][]key.Binding {
	return [][]key.Binding{
		{k.FocusLeft, k.FocusRight},
		{k.ToggleDetails},
		{k.Back},
	}
}

// TreeVisualizerKeyMap defines key bindings specific to tree visualizer
type TreeVisualizerKeyMap struct {
	// Embed base visualizer keys
	VisualizerModeKeyMap

	// Navigation
	Up       key.Binding
	Down     key.Binding
	PageUp   key.Binding
	PageDown key.Binding
	Home     key.Binding
	End      key.Binding

	// Expand/Collapse
	Toggle      key.Binding
	ExpandAll   key.Binding
	CollapseAll key.Binding
}

// DefaultTreeVisualizerKeyMap returns the default key bindings for tree visualizer
func DefaultTreeVisualizerKeyMap() TreeVisualizerKeyMap {
	return TreeVisualizerKeyMap{
		VisualizerModeKeyMap: DefaultVisualizerModeKeyMap(),

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
			key.WithKeys("pgup", "ctrl+u"),
			key.WithHelp("pgup", "page up"),
		),
		PageDown: key.NewBinding(
			key.WithKeys("pgdown", "ctrl+d"),
			key.WithHelp("pgdown", "page down"),
		),
		Home: key.NewBinding(
			key.WithKeys("home", "g"),
			key.WithHelp("g", "go to top"),
		),
		End: key.NewBinding(
			key.WithKeys("end", "G"),
			key.WithHelp("G", "go to bottom"),
		),

		// Expand/Collapse
		Toggle: key.NewBinding(
			key.WithKeys("enter", " "),
			key.WithHelp("enter/space", "toggle"),
		),
		ExpandAll: key.NewBinding(
			key.WithKeys("E"),
			key.WithHelp("E", "expand all"),
		),
		CollapseAll: key.NewBinding(
			key.WithKeys("C"),
			key.WithHelp("C", "collapse all"),
		),
	}
}

// ShortHelp returns a short list of key bindings
func (k TreeVisualizerKeyMap) ShortHelp() []key.Binding {
	return []key.Binding{
		k.Up, k.Down, k.Toggle, k.FocusLeft, k.FocusRight, k.Back,
	}
}

// FullHelp returns the full list of key bindings organized by category
func (k TreeVisualizerKeyMap) FullHelp() [][]key.Binding {
	return [][]key.Binding{
		{k.Up, k.Down, k.PageUp, k.PageDown, k.Home, k.End},
		{k.Toggle, k.ExpandAll, k.CollapseAll},
		{k.FocusLeft, k.FocusRight},
		{k.ToggleDetails},
		{k.Back},
	}
}

// GraphVisualizerKeyMap defines key bindings specific to graph visualizer
type GraphVisualizerKeyMap struct {
	// Embed base visualizer keys
	VisualizerModeKeyMap

	// Node selection (arrow keys)
	Up    key.Binding
	Down  key.Binding
	Left  key.Binding
	Right key.Binding

	// Canvas panning (i/j/k/l)
	PanUp    key.Binding
	PanDown  key.Binding
	PanLeft  key.Binding
	PanRight key.Binding

	// Jump to first/last
	Home key.Binding
	End  key.Binding
}

// DefaultGraphVisualizerKeyMap returns the default key bindings for graph visualizer
func DefaultGraphVisualizerKeyMap() GraphVisualizerKeyMap {
	return GraphVisualizerKeyMap{
		VisualizerModeKeyMap: DefaultVisualizerModeKeyMap(),

		// Node selection (arrow keys)
		Up: key.NewBinding(
			key.WithKeys("up"),
			key.WithHelp("↑", "select up"),
		),
		Down: key.NewBinding(
			key.WithKeys("down"),
			key.WithHelp("↓", "select down"),
		),
		Left: key.NewBinding(
			key.WithKeys("left"),
			key.WithHelp("←", "select left"),
		),
		Right: key.NewBinding(
			key.WithKeys("right"),
			key.WithHelp("→", "select right"),
		),

		// Canvas panning (i/j/k/l)
		PanUp: key.NewBinding(
			key.WithKeys("i"),
			key.WithHelp("i", "pan up"),
		),
		PanDown: key.NewBinding(
			key.WithKeys("k"),
			key.WithHelp("k", "pan down"),
		),
		PanLeft: key.NewBinding(
			key.WithKeys("j"),
			key.WithHelp("j", "pan left"),
		),
		PanRight: key.NewBinding(
			key.WithKeys("l"),
			key.WithHelp("l", "pan right"),
		),

		// Jump to first/last
		Home: key.NewBinding(
			key.WithKeys("home", "g"),
			key.WithHelp("g", "first node"),
		),
		End: key.NewBinding(
			key.WithKeys("end", "G"),
			key.WithHelp("G", "last node"),
		),
	}
}

// ShortHelp returns a short list of key bindings
func (k GraphVisualizerKeyMap) ShortHelp() []key.Binding {
	return []key.Binding{
		k.Up, k.Down, k.PanUp, k.PanDown, k.Back,
	}
}

// FullHelp returns the full list of key bindings organized by category
func (k GraphVisualizerKeyMap) FullHelp() [][]key.Binding {
	return [][]key.Binding{
		{k.Up, k.Down, k.Left, k.Right, k.Home, k.End},
		{k.PanUp, k.PanDown, k.PanLeft, k.PanRight},
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
