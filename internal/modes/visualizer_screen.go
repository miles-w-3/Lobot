package modes

import (
	"log/slog"

	tea "charm.land/bubbletea/v2"
	"github.com/miles-w-3/lobot/internal/command"
	"github.com/miles-w-3/lobot/internal/graph"
	"github.com/miles-w-3/lobot/internal/keys"
	"github.com/miles-w-3/lobot/internal/visualizer"
)

// visualizationMode identifies the local presentation of a resource graph.
// Tree and graph are views of one visualizer screen, not separate RootModel
// screens.
type visualizationMode uint8

const (
	visualizationTree visualizationMode = iota
	visualizationGraph
)

// VisualizerScreen presents a resource graph as either a tree or a graph. It
// owns mode-specific command routing while the child views own only rendering
// and semantic state changes.
type VisualizerScreen struct {
	log     *slog.Logger
	graph   *graph.ResourceGraph
	loading bool
	mode    visualizationMode

	treeView  *visualizer.TreeView
	graphView *visualizer.GraphView

	treeRegistry  *command.Registry[keys.TreeCmd]
	graphRegistry *command.Registry[keys.GraphCmd]

	width  int
	height int
}

var _ Screen = (*VisualizerScreen)(nil)

func NewVisualizerScreen(resourceGraph *graph.ResourceGraph, log *slog.Logger) *VisualizerScreen {
	if log == nil {
		log = slog.Default()
	}

	treeRegistry := keys.NewTreeRegistry()
	graphRegistry := keys.NewGraphRegistry()
	screen := &VisualizerScreen{
		log:           log.With("component", "visualizer"),
		loading:       resourceGraph == nil,
		mode:          visualizationTree,
		treeRegistry:  treeRegistry,
		graphRegistry: graphRegistry,
		width:         0,
		height:        0,
		// The graph view is lazy: layout and canvas construction are deferred
		// until the user switches to graph mode.
		graphView: nil,
	}
	if resourceGraph != nil {
		screen.setGraph(resourceGraph)
	}
	return screen
}

func commandDisplay[T comparable](registry *command.Registry[T], cmd T) string {
	entry, ok := registry.EntryForCommand(cmd)
	if !ok {
		return ""
	}
	return entry.Display
}

func (s *VisualizerScreen) View() string {
	if s.loading {
		return "Building resource graph…"
	}
	if s.graph == nil {
		return "No resource graph available"
	}
	if s.mode == visualizationGraph {
		if s.graphView == nil {
			return "Loading graph view…"
		}
		return s.graphView.View()
	}
	return s.treeView.View()
}

func (s *VisualizerScreen) Update(msg tea.Msg) tea.Cmd {
	switch msg := msg.(type) {
	case VisualizerReadyMsg:
		s.setGraph(msg.Graph)
		return nil
	case tea.WindowSizeMsg:
		s.resize(msg.Width, msg.Height)
		return nil
	case tea.KeyPressMsg:
		return s.updateKey(msg)
	}

	if s.mode == visualizationGraph && s.graphView != nil {
		return s.graphView.Update(msg)
	}
	if s.treeView != nil {
		return s.treeView.Update(msg)
	}
	return nil
}

func (s *VisualizerScreen) updateKey(msg tea.KeyPressMsg) tea.Cmd {
	if s.mode == visualizationGraph {
		cmd, err := s.graphRegistry.Dispatch(msg)
		if err != nil {
			if s.graphView != nil {
				return s.graphView.Update(msg)
			}
			return nil
		}
		return s.dispatchGraphCommand(cmd)
	}

	cmd, err := s.treeRegistry.Dispatch(msg)
	if err != nil {
		if s.treeView != nil {
			return s.treeView.Update(msg)
		}
		return nil
	}
	return s.dispatchTreeCommand(cmd)
}

func (s *VisualizerScreen) dispatchTreeCommand(cmd keys.TreeCmd) tea.Cmd {
	switch cmd {
	case keys.TreeCmdBack:
		return navigateTo(ScreenHome)
	case keys.TreeCmdSwitchToGraph:
		s.ensureGraphView()
		s.mode = visualizationGraph
		return nil
	default:
		if s.treeView != nil {
			return s.treeView.HandleCommand(cmd)
		}
	}
	return nil
}

func (s *VisualizerScreen) dispatchGraphCommand(cmd keys.GraphCmd) tea.Cmd {
	switch cmd {
	case keys.GraphCmdBack:
		return navigateTo(ScreenHome)
	case keys.GraphCmdSwitchToTree:
		s.mode = visualizationTree
		return nil
	default:
		if s.graphView != nil {
			return s.graphView.HandleCommand(cmd)
		}
	}
	return nil
}

func (s *VisualizerScreen) setGraph(resourceGraph *graph.ResourceGraph) {
	s.graph = resourceGraph
	s.loading = false
	s.mode = visualizationTree
	s.graphView = nil
	if resourceGraph == nil {
		s.treeView = nil
		return
	}

	treeSwitchKey := commandDisplay(s.treeRegistry, keys.TreeCmdSwitchToGraph)
	s.treeView = visualizer.NewTreeView(resourceGraph, s.width, s.height, treeSwitchKey)
}

func (s *VisualizerScreen) ensureGraphView() {
	if s.graphView != nil || s.graph == nil {
		return
	}
	switchKey := commandDisplay(s.graphRegistry, keys.GraphCmdSwitchToTree)
	s.graphView = visualizer.NewGraphView(s.graph, s.width, s.height, switchKey)
}

func (s *VisualizerScreen) resize(width, height int) {
	s.width = width
	s.height = height
	if s.treeView != nil {
		s.treeView.Resize(width, height)
	}
	if s.graphView != nil {
		s.graphView.Resize(width, height)
	}
}

// CommandPresentation exposes the active mode's commands to RootModel. The
// root renders the shared help bar and command palette; this screen only owns
// command semantics.
func (s *VisualizerScreen) CommandPresentation() command.Presentation {
	if s.mode == visualizationGraph {
		return s.graphRegistry.Presentation()
	}
	return s.treeRegistry.Presentation()
}
