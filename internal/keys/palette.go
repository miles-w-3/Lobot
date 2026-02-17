package keys

import (
	"github.com/miles-w-3/lobot/internal/command"
)

type PaletteCmd int

const (
	PaletteCmdNil PaletteCmd = iota
	PaletteCmdUp
	PaletteCmdDown
	PaletteCmdEnter
	PaletteCmdBack
)

func NewPaletteRegistry() *command.Registry[PaletteCmd] {
	r := command.NewRegistry[PaletteCmd]()

	navigation := r.NewCommandGroup("Navigation")
	navigation.Add("up", PaletteCmdUp).WithAlternates("ctrl+k")
	navigation.Add("down", PaletteCmdDown).WithAlternates("ctrl+j")

	actions := r.NewCommandGroup("Actions")
	actions.Add("enter", PaletteCmdEnter)

	exit := r.NewCommandGroup("Exit")
	exit.Add("esc", PaletteCmdBack).WithAlternates("q")

	r.Build()

	return r
}
