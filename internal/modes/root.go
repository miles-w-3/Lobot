package modes

import (
	"context"
	"fmt"
	"log/slog"
	"strings"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/x/ansi"
	"github.com/miles-w-3/lobot/internal/command"
	"github.com/miles-w-3/lobot/internal/graph"
	"github.com/miles-w-3/lobot/internal/k8s"
	"github.com/miles-w-3/lobot/internal/keys"
	"github.com/miles-w-3/lobot/internal/util"
)

// RootModel is the Bubble Tea program model. It owns global routing, shared
// overlays, terminal state, and the active screen; screen-specific behavior
// lives in the screen itself.
type RootModel struct {
	log *slog.Logger

	current         Screen
	currentID       ScreenID
	screenFactories map[ScreenID]func() (Screen, tea.Cmd)
	initialCmd      tea.Cmd

	commandPalette        *PaletteModel
	commandPaletteVisible bool
	modal                 *ModalModel

	width  int
	height int
	isDark bool

	globalRegistry  *command.Registry[keys.GlobalCmd]
	resourceService *k8s.ResourceService
	graphBuilder    *graph.Builder
}

func NewRootModel(resourceService *k8s.ResourceService, log *slog.Logger) *RootModel {
	if log == nil {
		log = slog.Default()
	}

	palette := NewPaletteModel(0, 0, true)
	root := &RootModel{
		log:             log.With("component", "root"),
		commandPalette:  &palette,
		modal:           NewModalModel(),
		globalRegistry:  keys.NewGlobalRegistry(),
		resourceService: resourceService,
		graphBuilder:    graph.NewBuilder(resourceService, log),
		isDark:          true,
	}

	// Keep screen construction in one place. Screens request transitions;
	// RootModel owns the destination lifecycle and initial commands. Home is
	// the one persistent screen: its factory keeps the instance private while
	// returning a refresh command whenever we navigate back to it.
	root.screenFactories = map[ScreenID]func() (Screen, tea.Cmd){
		ScreenSplash: func() (Screen, tea.Cmd) {
			screen := NewSplashScreen(log)
			return screen, screen.start()
		},
		ScreenHome: memoizedScreenFactory(
			func() (Screen, tea.Cmd) {
				return NewHomeScreen(resourceService, log), nil
			},
			func() tea.Cmd {
				return func() tea.Msg { return HomeActivatedMsg{} }
			},
		),
		ScreenVisualizer: func() (Screen, tea.Cmd) {
			return NewVisualizerScreen(nil, log), nil
		},
		ScreenUtilization: func() (Screen, tea.Cmd) {
			return newLoadingUtilizationScreen(0, 0), nil
		},
		ScreenManifest: func() (Screen, tea.Cmd) {
			return NewManifestScreen(), nil
		},
	}

	initialScreen, initialCmd := root.screenFactories[ScreenSplash]()
	root.current = initialScreen
	root.currentID = ScreenSplash
	root.initialCmd = initialCmd
	return root
}

// memoizedScreenFactory keeps persistence local to the factory entry. It is
// intentionally opt-in: transient screens continue to use ordinary factories
// and are eligible for collection as soon as RootModel replaces them.
func memoizedScreenFactory(
	create func() (Screen, tea.Cmd),
	onActivate func() tea.Cmd,
) func() (Screen, tea.Cmd) {
	var screen Screen
	var initialized bool

	return func() (Screen, tea.Cmd) {
		if !initialized {
			created, initialCmd := create()
			screen = created
			initialized = true
			return screen, tea.Batch(initialCmd, onActivate())
		}

		return screen, onActivate()
	}
}

func (r *RootModel) Init() tea.Cmd {
	initialCmd := r.initialCmd
	r.initialCmd = nil
	return tea.Batch(initialCmd, tea.RequestBackgroundColor)
}

// Update routes global commands and root-owned overlays before forwarding
// messages to the active screen.
func (r *RootModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	if handled, cmd := r.handleWindowMessage(msg); handled {
		return r, cmd
	}

	// Global commands have priority everywhere, including overlays. This keeps
	// quit/help/palette behavior deterministic and prevents double dispatch.
	if key, ok := msg.(tea.KeyPressMsg); ok {
		if globalCmd, err := r.globalRegistry.Dispatch(key); err == nil {
			return r.handleGlobalCommand(globalCmd)
		}
	}

	// Application requests/results are messages rather than key events. Route
	// them before overlays so an async result cannot be consumed by the palette
	// or a modal and silently disappear.
	switch msg := msg.(type) {
	case UtilizationRequestedMsg:
		return r, r.handleUtilizationRequest()
	case UtilizationReadyMsg:
		return r, r.handleUtilizationReady(msg)
	case ContextSwitchRequestedMsg:
		return r, r.handleContextSwitch(msg)
	case ManifestRequestedMsg:
		return r, r.handleManifestRequest(msg)
	case ManifestEditRequestedMsg:
		return r, r.startManifestEdit(msg)
	case ManifestCopyRequestedMsg:
		return r, r.copyManifest(msg)
	case ManifestEditFinishedMsg:
		return r, r.handleManifestEditFinished(msg)
	case VisualizerRequestedMsg:
		return r, r.buildVisualizer(msg.Resource)
	case VisualizerReadyMsg:
		if msg.Graph == nil {
			r.showModal("Visualizer Error", "Unable to build a resource graph")
			return r, nil
		}
		activationCmd := r.activateScreen(ScreenVisualizer)
		return r, tea.Batch(activationCmd, r.updateCurrentScreen(msg))
	case NamespaceOptionsReadyMsg, ContextOptionsReadyMsg, ResourceTypeOptionsReadyMsg:
		return r, r.updateCurrentScreen(msg)
	}

	if r.commandPaletteVisible {
		switch msg := msg.(type) {
		case PaletteBackMsg:
			r.commandPaletteVisible = false
			return r, nil
		case PaletteSelectedMsg:
			return r.handlePaletteSelection(msg)
		}

		updated, cmd := r.commandPalette.Update(msg)
		r.commandPalette = &updated
		return r, cmd
	}

	if r.modal.IsVisible() {
		return r, r.modal.Update(msg)
	}

	switch msg := msg.(type) {
	case NavigateMsg:
		return r, r.activateScreen(msg.Target)
	case ErrorMsg:
		if r.currentID != ScreenSplash && msg.Error != nil {
			r.showModal("Error", msg.Error.Error())
			return r, nil
		}
	}

	return r, r.updateCurrentScreen(msg)
}

func (r *RootModel) buildVisualizer(resource k8s.TrackedObject) tea.Cmd {
	if resource == nil || r.graphBuilder == nil {
		return func() tea.Msg {
			return ErrorMsg{Error: fmt.Errorf("cannot visualize an empty resource")}
		}
	}

	builder := r.graphBuilder
	return func() tea.Msg {
		return VisualizerReadyMsg{Graph: builder.BuildGraph(resource)}
	}
}

func (r *RootModel) handleUtilizationRequest() tea.Cmd {
	if r.currentID != ScreenUtilization {
		r.commandPaletteVisible = false
		r.modal.Hide()
		return tea.Batch(r.activateScreen(ScreenUtilization), r.fetchUtilization())
	}

	return r.fetchUtilization()
}

func (r *RootModel) handleUtilizationReady(msg UtilizationReadyMsg) tea.Cmd {
	if r.currentID != ScreenUtilization {
		return nil
	}

	cmd := r.updateCurrentScreen(msg)
	if msg.Error != nil {
		if msg.Unavailable {
			r.showModal("Metrics Unavailable",
				"The Kubernetes metrics API is not available.\n\n"+
					"Please ensure metrics-server is installed and running:\n"+
					"kubectl apply -f https://github.com/kubernetes-sigs/metrics-server/releases/latest/download/components.yaml")
		} else {
			r.showModal("Metrics Error", "Failed to fetch metrics: "+msg.Error.Error())
		}
	}
	return cmd
}

func (r *RootModel) fetchUtilization() tea.Cmd {
	service := r.resourceService
	logger := r.log
	return func() tea.Msg {
		if service == nil {
			return UtilizationReadyMsg{Error: fmt.Errorf("resource service is unavailable")}
		}

		client := service.GetClient()
		if client == nil {
			return UtilizationReadyMsg{Error: fmt.Errorf("Kubernetes client is unavailable")}
		}

		ctx := context.Background()
		if !client.CheckMetricsAPIAvailable(ctx) {
			return UtilizationReadyMsg{
				Error:       fmt.Errorf("metrics.k8s.io API is unavailable"),
				Unavailable: true,
			}
		}

		metricsClient, err := k8s.NewMetricsClient(client, logger)
		if err != nil {
			return UtilizationReadyMsg{Error: err}
		}

		nodeMetrics, podMetrics, err := metricsClient.GetMetricsFromServer(ctx)
		if err != nil {
			return UtilizationReadyMsg{Error: err}
		}

		return UtilizationReadyMsg{
			NodeMetrics: nodeMetrics,
			PodMetrics:  podMetrics,
		}
	}
}

func (r *RootModel) activateScreen(id ScreenID) tea.Cmd {
	factory, ok := r.screenFactories[id]
	if !ok {
		r.log.Error("unknown screen requested", "screen", id)
		return nil
	}

	screen, startCmd := factory()
	r.log.Debug("screen transition", "from", r.currentID, "to", id)
	r.current = screen
	r.currentID = id

	return tea.Batch(startCmd, r.forwardWindowSize(screen))
}

func (r *RootModel) handleWindowMessage(msg tea.Msg) (bool, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		r.width = msg.Width
		r.height = msg.Height
		r.commandPalette.SetSize(msg.Width, msg.Height)
		r.modal.SetSize(msg.Width, msg.Height)
		return true, r.forwardWindowSize(r.current)
	case tea.BackgroundColorMsg:
		r.isDark = msg.IsDark()
		r.commandPalette.SetTheme(r.isDark)
		return true, nil
	default:
		return false, nil
	}
}

func (r *RootModel) handleGlobalCommand(cmd keys.GlobalCmd) (tea.Model, tea.Cmd) {
	switch cmd {
	case keys.GlobalCmdQuit:
		return r, tea.Quit
	case keys.GlobalCmdPalette:
		r.modal.Hide()
		return r, r.openPalette()
	case keys.GlobalCmdHelp:
		r.commandPaletteVisible = false
		if r.modal.IsVisible() {
			r.modal.Hide()
		} else {
			r.showHelp()
		}
		return r, nil
	default:
		return r, nil
	}
}

func (r *RootModel) openPalette() tea.Cmd {
	entries := r.globalRegistry.Presentation().PaletteEntries
	if presentation := r.currentPresentation(); !presentation.Empty() {
		entries = append(entries, presentation.PaletteEntries...)
	}

	r.commandPalette.SetEntries(entries)
	r.commandPalette.Reset()
	r.commandPaletteVisible = true
	return r.commandPalette.Init()
}

func (r *RootModel) handlePaletteSelection(msg PaletteSelectedMsg) (tea.Model, tea.Cmd) {
	r.commandPaletteVisible = false

	if cmd, err := r.globalRegistry.DispatchString(msg.Entry.Key); err == nil {
		return r.handleGlobalCommand(cmd)
	}

	// Replay the canonical binding directly into the active screen. Do not
	// emit it back through RootModel.Update: that would run global dispatch a
	// second time and could turn a screen command into a global command.
	return r, r.updateCurrentScreen(replayKey(msg.Entry.Key))
}

// replayKey creates the logical key event expected by the registry boundary.
// Screens dispatch shortcut messages by msg.String(), so the canonical key
// string is sufficient without reconstructing terminal-specific key fields.
func replayKey(key string) tea.KeyPressMsg {
	return tea.KeyPressMsg{Text: key}
}

func (r *RootModel) currentPresentation() command.Presentation {
	if r.current == nil {
		return command.Presentation{}
	}
	return r.current.CommandPresentation()
}

func (r *RootModel) updateCurrentScreen(msg tea.Msg) tea.Cmd {
	if r.current == nil {
		return nil
	}

	oldHelpHeight := r.helpBarHeight()
	cmd := r.current.Update(msg)
	if oldHelpHeight != r.helpBarHeight() {
		cmd = tea.Batch(cmd, r.forwardWindowSize(r.current))
	}
	return cmd
}

func (r *RootModel) forwardWindowSize(screen Screen) tea.Cmd {
	if screen == nil {
		return nil
	}
	return screen.Update(tea.WindowSizeMsg{
		Width:  r.width,
		Height: r.contentHeight(),
	})
}

func (r *RootModel) contentHeight() int {
	height := r.height - r.helpBarHeight()
	if height < 0 {
		return 0
	}
	return height
}

func (r *RootModel) helpConfig() command.HelpConfig {
	config := command.NewHelpConfig()
	config.Styles = command.DefaultHelpStyles(r.isDark)
	return config
}

func (r *RootModel) shortHelp() string {
	config := r.helpConfig()
	help := r.globalRegistry.Presentation().ShortView(config)
	if presentation := r.currentPresentation(); !presentation.Empty() {
		if screenHelp := presentation.ShortView(config); screenHelp != "" {
			help += config.Styles.ShortSeparator.Render(" • ") + screenHelp
		}
	}
	return help
}

func (r *RootModel) fullHelp() string {
	config := r.helpConfig()
	parts := []string{r.globalRegistry.Presentation().FullView(config)}
	if presentation := r.currentPresentation(); !presentation.Empty() {
		if screenHelp := presentation.FullView(config); screenHelp != "" {
			parts = append(parts, screenHelp)
		}
	}
	return strings.Join(parts, "\n\n")
}

func (r *RootModel) showHelp() {
	r.modal.Show("Help", r.fullHelp())
	r.modal.SetSize(r.width, r.height)
}

func (r *RootModel) showModal(title, content string) {
	r.modal.Show(title, content)
	r.modal.SetSize(r.width, r.height)
}

func (r *RootModel) helpBarHeight() int {
	if r.shortHelp() == "" {
		return 0
	}
	return 1
}

func (r *RootModel) renderHelpBar() string {
	help := r.shortHelp()
	if help == "" {
		return ""
	}

	if r.width > 2 {
		help = ansi.Truncate(help, r.width-2, "…")
	}
	return lipgloss.NewStyle().
		Foreground(util.ColorMuted).
		Padding(0, 1).
		Render(help)
}

func (r *RootModel) View() tea.View {
	content := ""
	if r.current != nil {
		content = r.current.View()
	}

	if help := r.renderHelpBar(); help != "" {
		content = lipgloss.JoinVertical(lipgloss.Left, content, help)
	}

	view := tea.NewView(content)
	if r.modal.IsVisible() {
		view.Content = overlayCenter(view.Content, r.modal.View(), r.width, r.height)
	}
	if r.commandPaletteVisible {
		view.Content = overlayCenter(view.Content, r.commandPalette.View(), r.width, r.height)
	}

	view.AltScreen = true
	view.MouseMode = tea.MouseModeCellMotion
	return view
}
