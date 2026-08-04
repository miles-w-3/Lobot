package keys

import (
	"github.com/miles-w-3/lobot/internal/command"
)

type NormalCmd int

const (
	NormalCmdNil NormalCmd = iota
	NormalCmdMoveUp
	NormalCmdMoveDown
	NormalCmdPageUp
	NormalCmdPageDown
	NormalCmdHome
	NormalCmdEnd
	NormalCmdNextType
	NormalCmdPrevType
	NormalCmdEnter
	NormalCmdEdit
	NormalCmdVisualize
	NormalCmdFilter
	NormalCmdRefresh
	NormalCmdToggleFavorites
	NormalCmdNamespaceSelector
	NormalCmdResourceTypeSelector
	NormalCmdContextSelector
	NormalCmdUtilizationDashboard
	NormalCmdWorkloadLogs
	NormalCmdQuit
)

func NewNormalRegistry() *command.Registry[NormalCmd] {
	r := command.NewRegistry[NormalCmd]()

	navigation := r.NewUnifiedCommandGroup("Navigation", "navigate")
	navigation.Add("up", NormalCmdMoveUp).WithAlternates("k")
	navigation.Add("down", NormalCmdMoveDown).WithAlternates("j")
	navigation.Add("pgup", NormalCmdPageUp)
	navigation.Add("pgdown", NormalCmdPageDown)
	navigation.Add("home", NormalCmdHome).WithAlternates("g")
	navigation.Add("end", NormalCmdEnd).WithAlternates("G")

	typeNav := r.NewUnifiedCommandGroup("Type Navigation", "switch resource type")
	typeNav.Add("right", NormalCmdNextType).WithAlternates("l")
	typeNav.Add("left", NormalCmdPrevType).WithAlternates("h")

	actions := r.NewCommandGroup("Resource Actions")
	actions.Add("enter", NormalCmdEnter).WithDescription("open")
	actions.Add("E", NormalCmdEdit).WithDescription("edit")
	actions.Add("V", NormalCmdVisualize).WithDescription("visualize")
	actions.Add("R", NormalCmdRefresh).WithDescription("refresh")
	actions.Add("L", NormalCmdWorkloadLogs).WithDescription("view logs (pod only)")
	actions.Add("tab", NormalCmdToggleFavorites)

	filters := r.NewCommandGroup("Filters")
	filters.Add("/", NormalCmdFilter).WithDescription("resource names")
	filters.Add("ctrl+n", NormalCmdNamespaceSelector).WithDescription("namespace selector")
	filters.Add("ctrl+t", NormalCmdResourceTypeSelector).WithDescription("resource type selector")
	filters.Add("ctrl+k", NormalCmdContextSelector).WithDescription("context selector")

	selectors := r.NewCommandGroup("Mode")
	selectors.Add("ctrl+u", NormalCmdUtilizationDashboard).WithDescription("utilization dashboard")

	exit := r.NewUnifiedCommandGroup("Exit", "exit")
	exit.Add("q", NormalCmdQuit)

	r.Build()

	return r
}
