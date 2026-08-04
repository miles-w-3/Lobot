package keys

import "github.com/miles-w-3/lobot/internal/command"

// HomeCmd identifies commands handled by HomeScreen.
type HomeCmd int

const (
	HomeCmdNil HomeCmd = iota
	HomeCmdMoveUp
	HomeCmdMoveDown
	HomeCmdPageUp
	HomeCmdPageDown
	HomeCmdHome
	HomeCmdEnd
	HomeCmdNextType
	HomeCmdPrevType
	HomeCmdFilter
	HomeCmdRefresh
	HomeCmdOpenManifest
	HomeCmdEdit
	HomeCmdVisualize
	HomeCmdOpenUtilization
	HomeCmdOpenNamespaceSelector
	HomeCmdOpenResourceTypeSelector
	HomeCmdOpenContextSelector
	HomeCmdToggleFavorites
)

func addHomeCommands(r *command.Registry[HomeCmd]) *command.CommandGroup[HomeCmd] {
	navigation := r.NewUnifiedCommandGroup("Navigation", "navigate")
	navigation.Add("up", HomeCmdMoveUp).WithAlternates("k")
	navigation.Add("down", HomeCmdMoveDown).WithAlternates("j")
	navigation.Add("pgup", HomeCmdPageUp)
	navigation.Add("pgdown", HomeCmdPageDown)
	navigation.Add("home", HomeCmdHome).WithAlternates("g")
	navigation.Add("end", HomeCmdEnd).WithAlternates("G")

	typeNav := r.NewUnifiedCommandGroup("Type Navigation", "switch resource type")
	typeNav.Add("right", HomeCmdNextType).WithAlternates("l")
	typeNav.Add("left", HomeCmdPrevType).WithAlternates("h")

	actions := r.NewCommandGroup("Resource Actions")
	actions.Add("enter", HomeCmdOpenManifest).WithDescription("open manifest")
	actions.Add("E", HomeCmdEdit).WithDescription("edit")
	actions.Add("R", HomeCmdRefresh).WithDescription("refresh")
	actions.Add("V", HomeCmdVisualize).WithDescription("visualize")

	filters := r.NewCommandGroup("Filters")
	filters.Add("/", HomeCmdFilter).WithDescription("resource names")
	filters.Add("ctrl+n", HomeCmdOpenNamespaceSelector).WithDescription("namespace selector")
	filters.Add("ctrl+t", HomeCmdOpenResourceTypeSelector).WithDescription("resource type selector")
	filters.Add("ctrl+k", HomeCmdOpenContextSelector).WithDescription("context selector")

	mode := r.NewCommandGroup("Mode")
	mode.Add("ctrl+u", HomeCmdOpenUtilization).WithDescription("utilization dashboard")

	return actions
}

// NewHomeRegistry returns the commands currently implemented by HomeScreen.
func NewHomeRegistry() *command.Registry[HomeCmd] {
	r := command.NewRegistry[HomeCmd]()
	actions := addHomeCommands(r)
	actions.Add("tab", HomeCmdToggleFavorites)
	_ = r.Build()
	return r
}
