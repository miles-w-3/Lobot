package modes

import (
	"testing"

	tea "charm.land/bubbletea/v2"
	"github.com/miles-w-3/lobot/internal/command"
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
