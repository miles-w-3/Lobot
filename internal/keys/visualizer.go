package keys

import (
	"github.com/miles-w-3/lobot/internal/command"
)

type VisualizerCmd int

const (
	VisualizerCmdNil VisualizerCmd = iota
	VisualizerCmdBack
	VisualizerCmdToggleMode
	VisualizerCmdZoomIn
	VisualizerCmdZoomOut
	VisualizerCmdPanUp
	VisualizerCmdPanDown
	VisualizerCmdPanLeft
	VisualizerCmdPanRight
	VisualizerCmdCenter
	VisualizerCmdToggleLabels
	VisualizerCmdToggleEdges
	VisualizerCmdNextLayout
	VisualizerCmdRefresh
)

func NewVisualizerRegistry() *command.Registry[VisualizerCmd] {
	r := command.NewRegistry[VisualizerCmd]()

	navigation := r.NewUnifiedCommandGroup("Navigation", "navigate graph")
	navigation.Add("up", VisualizerCmdPanUp).WithAlternates("k")
	navigation.Add("down", VisualizerCmdPanDown).WithAlternates("j")
	navigation.Add("left", VisualizerCmdPanLeft).WithAlternates("h")
	navigation.Add("right", VisualizerCmdPanRight).WithAlternates("l")

	view := r.NewCommandGroup("View")
	view.Add("t", VisualizerCmdToggleLabels).WithDescription("toggle labels")
	view.Add("e", VisualizerCmdToggleEdges).WithDescription("toggle edges")
	view.Add("tab", VisualizerCmdNextLayout).WithDescription("next layout")
	view.Add("+", VisualizerCmdZoomIn).WithDescription("zoom in")
	view.Add("-", VisualizerCmdZoomOut).WithDescription("zoom out")
	view.Add("0", VisualizerCmdCenter).WithDescription("center")

	actions := r.NewCommandGroup("Actions")
	actions.Add("r", VisualizerCmdRefresh).WithDescription("refresh")
	actions.Add("V", VisualizerCmdToggleMode).WithAlternates("toggle view")

	exit := r.NewUnifiedCommandGroup("Exit", "exit")
	exit.Add("esc", VisualizerCmdBack).WithAlternates("q")

	r.Build()

	return r
}
