package keys

import (
	"github.com/miles-w-3/lobot/internal/command"
)

type ModalCmd int

const (
	ModalCmdNil ModalCmd = iota
	ModalCmdBack
	ModalCmdScrollUp
	ModalCmdScrollDown
)

func NewModalRegistry() *command.Registry[ModalCmd] {
	r := command.NewRegistry[ModalCmd]()

	exit := r.NewCommandGroup("Exit")
	exit.Add("esc", ModalCmdBack).WithAlternates("enter").WithDescription("close")

	scrolling := r.NewCommandGroup("Scrolling")
	scrolling.Add("up", ModalCmdScrollUp).WithAlternates("k")
	scrolling.Add("down", ModalCmdScrollDown).WithAlternates("j")

	r.Build()

	return r
}
