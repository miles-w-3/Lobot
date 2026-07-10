package keys

import "github.com/miles-w-3/lobot/internal/command"

type SplashCmd int

const (
	SplashCmdNil SplashCmd = iota
	SplashCmdContextSelector
)

func NewSplashRegistry() *command.Registry[SplashCmd] {
	r := command.NewRegistry[SplashCmd]()

	actions := r.NewCommandGroup("Startup Actions")
	actions.Add("c", SplashCmdContextSelector).WithDescription("switch context")

	r.Build()

	return r
}
