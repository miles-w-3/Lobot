package keys

import (
	"github.com/miles-w-3/lobot/internal/command"
)

type ManifestCmd int

const (
	ManifestCmdNil ManifestCmd = iota
	ManifestCmdBack
	ManifestCmdEdit
	ManifestCmdCopy
	ManifestCmdScrollUp
	ManifestCmdScrollDown
	ManifestCmdPageUp
	ManifestCmdPageDown
)

func NewManifestRegistry() *command.Registry[ManifestCmd] {
	r := command.NewRegistry[ManifestCmd]()

	navigation := r.NewUnifiedCommandGroup("Navigation", "scroll manifest")
	navigation.Add("up", ManifestCmdScrollUp).WithAlternates("k")
	navigation.Add("down", ManifestCmdScrollDown).WithAlternates("j")

	actions := r.NewCommandGroup("Actions")
	actions.Add("e", ManifestCmdEdit).WithDescription("edit")
	actions.Add("ctrl+y", ManifestCmdCopy).WithDescription("copy document")
	actions.Add("pgup", ManifestCmdPageUp).WithDescription("page up")
	actions.Add("pgdown", ManifestCmdPageDown).WithDescription("page down")
	actions.Add("esc", ManifestCmdBack).WithAlternates("q").WithDescription("back")

	r.Build()

	return r
}
