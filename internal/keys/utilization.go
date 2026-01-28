package keys

import (
	"github.com/miles-w-3/lobot/internal/command"
)

type UtilizationCmd int

const (
	UtilizationCmdNil UtilizationCmd = iota
	UtilizationCmdBack
	UtilizationCmdScrollUp
	UtilizationCmdScrollDown
	UtilizationCmdPageUp
	UtilizationCmdPageDown
	UtilizationCmdNextView
	UtilizationCmdPrevView
)

func NewUtilizationRegistry() *command.Registry[UtilizationCmd] {
	r := command.NewRegistry[UtilizationCmd]()

	navigation := r.NewUnifiedCommandGroup("Navigation", "scroll")
	navigation.Add("up", UtilizationCmdScrollUp).WithAlternates("k")
	navigation.Add("down", UtilizationCmdScrollDown).WithAlternates("j")
	navigation.Add("pgup", UtilizationCmdPageUp)
	navigation.Add("pgdown", UtilizationCmdPageDown)

	view := r.NewUnifiedCommandGroup("View", "switch view")
	view.Add("right", UtilizationCmdNextView).WithAlternates("l")
	view.Add("left", UtilizationCmdPrevView).WithAlternates("h")

	exit := r.NewUnifiedCommandGroup("Exit", "exit dashboard")
	exit.Add("esc", UtilizationCmdBack).WithAlternates("q")

	r.Build()

	return r
}
