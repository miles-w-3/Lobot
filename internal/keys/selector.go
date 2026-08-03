package keys

import "github.com/miles-w-3/lobot/internal/command"

// SelectorCmd identifies actions handled by selection overlays.
type SelectorCmd int

const (
	SelectorCmdNil SelectorCmd = iota
	SelectorCmdAccept
	SelectorCmdCancel
	SelectorCmdUp
	SelectorCmdDown
	SelectorCmdPageUp
	SelectorCmdPageDown
)

// NewSelectorRegistry creates the registry shared by namespace, context, and
// resource-type selectors.
func NewSelectorRegistry() *command.Registry[SelectorCmd] {
	r := command.NewRegistry[SelectorCmd]()

	navigation := r.NewUnifiedCommandGroup("Selector Navigation", "navigate choices")
	navigation.Add("up", SelectorCmdUp).WithAlternates("ctrl+k")
	navigation.Add("down", SelectorCmdDown).WithAlternates("ctrl+j")
	navigation.Add("pgup", SelectorCmdPageUp)
	navigation.Add("pgdown", SelectorCmdPageDown)

	actions := r.NewCommandGroup("Selector Actions")
	actions.Add("enter", SelectorCmdAccept).WithDescription("select")
	actions.Add("esc", SelectorCmdCancel).WithDescription("cancel")

	if err := r.Build(); err != nil {
		panic(err)
	}
	return r
}
