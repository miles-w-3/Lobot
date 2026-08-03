package keys

import (
	"github.com/miles-w-3/lobot/internal/command"
)

type TreeCmd int

const (
	TreeCmdNil TreeCmd = iota
	TreeCmdMoveUp
	TreeCmdMoveDown
	TreeCmdPageUp
	TreeCmdPageDown
	TreeCmdHome
	TreeCmdEnd
	TreeCmdToggle
	TreeCmdExpandAll
	TreeCmdCollapseAll
	TreeCmdFocusLeft
	TreeCmdFocusRight
	TreeCmdToggleDetails
	TreeCmdBack
	TreeCmdSwitchToGraph
)

func NewTreeRegistry() *command.Registry[TreeCmd] {
	r := command.NewRegistry[TreeCmd]()

	navigation := r.NewUnifiedCommandGroup("Navigation", "navigate tree")
	navigation.Add("up", TreeCmdMoveUp).WithAlternates("k")
	navigation.Add("down", TreeCmdMoveDown).WithAlternates("j")
	navigation.Add("pgup", TreeCmdPageUp).WithAlternates("ctrl+u")
	navigation.Add("pgdown", TreeCmdPageDown).WithAlternates("ctrl+d")
	navigation.Add("home", TreeCmdHome).WithAlternates("g")
	navigation.Add("end", TreeCmdEnd).WithAlternates("G")

	tree := r.NewCommandGroup("Tree Actions")
	tree.Add("enter", TreeCmdToggle).WithAlternates("space").WithDescription("toggle expand")
	tree.Add("E", TreeCmdExpandAll).WithDescription("expand all")
	tree.Add("C", TreeCmdCollapseAll).WithDescription("collapse all")

	focus := r.NewUnifiedCommandGroup("Focus", "switch panel")
	focus.Add("left", TreeCmdFocusLeft).WithAlternates("h")
	focus.Add("right", TreeCmdFocusRight).WithAlternates("l")

	view := r.NewCommandGroup("View")
	view.Add("d", TreeCmdToggleDetails).WithDescription("toggle details")

	switchMode := r.NewCommandGroup("Mode")
	switchMode.Add("V", TreeCmdSwitchToGraph).WithDescription("switch to graph view")

	exit := r.NewUnifiedCommandGroup("Exit", "exit tree")
	exit.Add("esc", TreeCmdBack).WithAlternates("q")

	r.Build()

	return r
}
