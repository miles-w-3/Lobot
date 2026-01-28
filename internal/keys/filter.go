package keys

import (
	"github.com/miles-w-3/lobot/internal/command"
)

type FilterCmd int

const (
	FilterCmdNil FilterCmd = iota
	FilterCmdAccept
	FilterCmdCancel
	FilterCmdClear
)

func NewFilterRegistry() *command.Registry[FilterCmd] {
	r := command.NewRegistry[FilterCmd]()

	input := r.NewUnifiedCommandGroup("Input", "text input")
	input.Add("enter", FilterCmdAccept)

	actions := r.NewCommandGroup("Actions")
	actions.Add("esc", FilterCmdCancel)
	actions.Add("ctrl+u", FilterCmdClear)

	r.Build()

	return r
}
