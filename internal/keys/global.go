package keys

import (
	"github.com/miles-w-3/lobot/internal/command"
)

type GlobalCmd int

const (
	GlobalCmdNil GlobalCmd = iota
	GlobalCmdQuit
	GlobalCmdHelp
	GlobalCmdPalette
)

func NewGlobalRegistry() *command.Registry[GlobalCmd] {
	r := command.NewRegistry[GlobalCmd]()

	actions := r.NewCommandGroup("Global Actions")
	actions.Add("q", GlobalCmdQuit).WithAlternates("ctrl+c").WithDescription("quit")
	actions.Add("?", GlobalCmdHelp).WithDescription("help")
	actions.Add("ctrl+p", GlobalCmdPalette).WithDescription("command palette")

	r.Build()

	return r
}
