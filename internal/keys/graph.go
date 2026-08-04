package keys

import (
	"github.com/miles-w-3/lobot/internal/command"
)

type GraphCmd int

const (
	GraphCmdNil GraphCmd = iota
	GraphCmdSelectUp
	GraphCmdSelectDown
	GraphCmdSelectLeft
	GraphCmdSelectRight
	GraphCmdPanUp
	GraphCmdPanDown
	GraphCmdPanLeft
	GraphCmdPanRight
	GraphCmdHome
	GraphCmdEnd
	GraphCmdBack
	GraphCmdZoomIn
	GraphCmdZoomOut
	GraphCmdSwitchToTree
)

func NewGraphRegistry() *command.Registry[GraphCmd] {
	r := command.NewRegistry[GraphCmd]()

	selection := r.NewUnifiedCommandGroup("Node Selection", "select node")
	selection.Add("up", GraphCmdSelectUp)
	selection.Add("down", GraphCmdSelectDown)
	selection.Add("left", GraphCmdSelectLeft)
	selection.Add("right", GraphCmdSelectRight)

	pan := r.NewUnifiedCommandGroup("Canvas Panning", "pan canvas")
	pan.Add("i", GraphCmdPanUp)
	pan.Add("k", GraphCmdPanDown)
	pan.Add("j", GraphCmdPanLeft)
	pan.Add("l", GraphCmdPanRight)

	jump := r.NewCommandGroup("Jump")
	jump.Add("home", GraphCmdHome).WithAlternates("g").WithDescription("first node")
	jump.Add("end", GraphCmdEnd).WithAlternates("G").WithDescription("last node")

	zoom := r.NewCommandGroup("Zoom")
	zoom.Add("+", GraphCmdZoomIn).WithDescription("zoom in")
	zoom.Add("-", GraphCmdZoomOut).WithDescription("zoom out")

	switchMode := r.NewCommandGroup("Mode")
	switchMode.Add("V", GraphCmdSwitchToTree).WithDescription("switch to tree view")

	exit := r.NewUnifiedCommandGroup("Exit", "exit graph")
	exit.Add("esc", GraphCmdBack)

	r.Build()

	return r
}
