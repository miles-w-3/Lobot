package modes

import (
	"log/slog"
	"testing"

	tea "charm.land/bubbletea/v2"
	"github.com/miles-w-3/lobot/internal/command"
	"github.com/miles-w-3/lobot/internal/graph"
	"github.com/miles-w-3/lobot/internal/keys"
)

type recordingScreen struct {
	updates []tea.Msg
}

func (s *recordingScreen) Update(msg tea.Msg) tea.Cmd {
	s.updates = append(s.updates, msg)
	return nil
}

func (s *recordingScreen) View() string {
	return ""
}

func (s *recordingScreen) CommandPresentation() command.Presentation {
	return command.Presentation{}
}

func TestHomeFactoryPreservesScreenState(t *testing.T) {
	root := NewRootModel(nil, slog.Default())
	factory := root.screenFactories[ScreenHome]

	first, _ := factory()
	home, ok := first.(*HomeScreen)
	if !ok {
		t.Fatalf("home factory returned %T, want *HomeScreen", first)
	}
	home.showingFavoriteTypes = true

	second, refresh := factory()
	if second != first {
		t.Fatal("home factory created a new screen instance")
	}
	if !home.showingFavoriteTypes {
		t.Fatal("home state was not preserved")
	}
	if refresh == nil {
		t.Fatal("home reactivation returned no refresh command")
	}
	if _, ok := refresh().(HomeActivatedMsg); !ok {
		t.Fatalf("home activation message = %T, want HomeActivatedMsg", refresh())
	}
}

func TestManifestRequestUsesFactoryAndForwardsResource(t *testing.T) {
	root := NewRootModel(nil, slog.Default())
	root.current = NewHomeScreen(nil, nil)
	root.currentID = ScreenHome

	_, cmd := root.Update(ManifestRequestedMsg{Resource: testManifestResource()})
	if root.currentID != ScreenManifest {
		t.Fatalf("current screen = %v, want manifest", root.currentID)
	}
	screen, ok := root.current.(*ManifestScreen)
	if !ok {
		t.Fatalf("current screen = %T, want *ManifestScreen", root.current)
	}
	if screen.resource == nil {
		t.Fatal("manifest request was not forwarded to screen")
	}
	if cmd != nil {
		t.Fatalf("manifest activation command = %v, want nil", cmd)
	}
}

func TestTransientScreenFactoriesDoNotCache(t *testing.T) {
	root := NewRootModel(nil, slog.Default())
	for _, screenID := range []ScreenID{ScreenVisualizer, ScreenUtilization, ScreenManifest} {
		factory := root.screenFactories[screenID]
		first, _ := factory()
		second, _ := factory()
		if first == second {
			t.Fatalf("%v factory reused its screen instance", screenID)
		}
	}
}

func TestVisualizerReadyUsesVisualizerFactory(t *testing.T) {
	root := NewRootModel(nil, slog.Default())
	root.current = NewHomeScreen(nil, nil)
	root.currentID = ScreenHome

	_, cmd := root.Update(VisualizerReadyMsg{Graph: &graph.ResourceGraph{}})
	if root.currentID != ScreenVisualizer {
		t.Fatalf("current screen = %v, want visualizer", root.currentID)
	}
	if _, ok := root.current.(*VisualizerScreen); !ok {
		t.Fatalf("current screen = %T, want *VisualizerScreen", root.current)
	}
	if cmd != nil {
		t.Fatalf("visualizer activation command = %v, want nil", cmd)
	}
}

func TestPaletteSelectionReplaysKeyToActiveScreen(t *testing.T) {
	screen := &recordingScreen{}
	root := &RootModel{
		current:               screen,
		globalRegistry:        keys.NewGlobalRegistry(),
		commandPaletteVisible: true,
	}

	_, cmd := root.handlePaletteSelection(PaletteSelectedMsg{
		Entry: command.PaletteEntry{Key: "up"},
	})
	if cmd != nil {
		t.Fatalf("palette selection command = %v, want nil", cmd)
	}
	if root.commandPaletteVisible {
		t.Fatal("command palette remains visible after selection")
	}
	if len(screen.updates) != 1 {
		t.Fatalf("screen updates = %d, want 1", len(screen.updates))
	}

	key, ok := screen.updates[0].(tea.KeyPressMsg)
	if !ok {
		t.Fatalf("screen message = %T, want tea.KeyPressMsg", screen.updates[0])
	}
	if key.String() != "up" {
		t.Fatalf("replayed key = %q, want %q", key.String(), "up")
	}
}

func TestPaletteSelectionUsesScreenKeyDispatch(t *testing.T) {
	screen := NewHomeScreen(nil, nil)
	root := &RootModel{
		current:               screen,
		globalRegistry:        keys.NewGlobalRegistry(),
		commandPaletteVisible: true,
	}

	_, cmd := root.handlePaletteSelection(PaletteSelectedMsg{
		Entry: command.PaletteEntry{Key: "tab"},
	})
	if cmd != nil {
		t.Fatalf("palette selection command = %v, want nil", cmd)
	}
	if !screen.showingFavoriteTypes {
		t.Fatal("screen command was not dispatched through its key registry")
	}
}

func TestPaletteGlobalSelectionDoesNotReachScreen(t *testing.T) {
	screen := &recordingScreen{}
	root := &RootModel{
		current:               screen,
		globalRegistry:        keys.NewGlobalRegistry(),
		commandPaletteVisible: true,
	}

	_, cmd := root.handlePaletteSelection(PaletteSelectedMsg{
		Entry: command.PaletteEntry{Key: "q"},
	})
	if cmd == nil {
		t.Fatal("global quit selection returned no command")
	}
	if len(screen.updates) != 0 {
		t.Fatalf("screen updates = %d, want 0", len(screen.updates))
	}
}

func TestUtilizationRequestActivatesLoadingScreen(t *testing.T) {
	root := NewRootModel(nil, slog.Default())
	root.current = &recordingScreen{}
	root.currentID = ScreenHome

	_, cmd := root.Update(UtilizationRequestedMsg{})
	if root.currentID != ScreenUtilization {
		t.Fatalf("current screen = %v, want utilization", root.currentID)
	}
	if screen, ok := root.current.(*UtilizationScreen); !ok || !screen.loading {
		t.Fatal("utilization request did not activate a loading screen")
	}
	if cmd == nil {
		t.Fatal("utilization request returned no fetch command")
	}
}

func TestUtilizationReadyMessageUpdatesActiveScreen(t *testing.T) {
	screen := newLoadingUtilizationScreen(100, 30)
	root := &RootModel{
		current:        screen,
		currentID:      ScreenUtilization,
		globalRegistry: keys.NewGlobalRegistry(),
		modal:          NewModalModel(),
	}

	_, cmd := root.Update(UtilizationReadyMsg{})
	if cmd != nil {
		t.Fatalf("ready command = %v, want nil", cmd)
	}
	if screen.loading {
		t.Fatal("ready message left utilization screen loading")
	}
	if screen.loadError != nil {
		t.Fatalf("ready message set error = %v", screen.loadError)
	}
}
