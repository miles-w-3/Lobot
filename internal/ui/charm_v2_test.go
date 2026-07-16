package ui

import (
	"image/color"
	"log/slog"
	"strings"
	"testing"

	"charm.land/bubbles/v2/table"
	"charm.land/bubbles/v2/viewport"
	tea "charm.land/bubbletea/v2"
	"github.com/charmbracelet/x/ansi"
	"github.com/miles-w-3/lobot/internal/k8s"
	"github.com/miles-w-3/lobot/internal/keys"
)

func TestRootViewDeclaresTerminalState(t *testing.T) {
	tests := []struct {
		name      string
		mode      ViewMode
		mouseMode tea.MouseMode
	}{
		{name: "splash enables mouse", mode: ViewModeSplash, mouseMode: tea.MouseModeCellMotion},
		{name: "manifest disables mouse", mode: ViewModeManifest, mouseMode: tea.MouseModeNone},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			m := Model{
				viewMode: tt.mode,
				modal:    &Modal{},
			}
			view := m.View()
			if !view.AltScreen {
				t.Fatal("View().AltScreen = false, want true")
			}
			if view.MouseMode != tt.mouseMode {
				t.Fatalf("View().MouseMode = %v, want %v", view.MouseMode, tt.mouseMode)
			}
		})
	}
}

func TestPaletteAcceptsPasteMessages(t *testing.T) {
	m := NewPaletteModel(80, 22, true)
	updated, _ := m.Update(tea.PasteMsg{Content: "deploy"})
	if got, want := updated.input.Value(), "deploy"; got != want {
		t.Fatalf("palette input after paste = %q, want %q", got, want)
	}
}

func TestRootRoutesPaletteBlinkMessages(t *testing.T) {
	palette := NewPaletteModel(80, 22, true)
	initialBlink := palette.Init()
	if initialBlink == nil {
		t.Fatal("palette Init() returned nil")
	}

	m := Model{paletteVisible: true, paletteModel: palette}
	updated, cmd := m.Update(initialBlink())
	if cmd == nil {
		t.Fatal("palette blink message did not produce its next blink command")
	}
	if !updated.(Model).paletteVisible {
		t.Fatal("palette was hidden while routing blink message")
	}
}

func TestSelectorCancelUsesRegistryCommand(t *testing.T) {
	selector := NewContextSelector(
		[]string{"first", "second"},
		"first",
		keys.NewGlobalRegistry(),
	)

	updated, cmd := selector.Update(tea.KeyPressMsg{Code: tea.KeyEscape})
	if updated.IsVisible() {
		t.Fatal("selector remained visible after cancel")
	}
	if cmd == nil {
		t.Fatal("selector cancel returned a nil command")
	}
	result := cmd()
	msg, ok := result.(SelectorFinishedMsg)
	if !ok {
		t.Fatalf("selector cancel command returned %T, want SelectorFinishedMsg", result)
	}
	if !msg.Cancelled || msg.SelectorType != SelectorTypeContext {
		t.Fatalf("selector cancel message = %#v", msg)
	}
}

func TestSelectorAcceptDoesNotPropagatePromptkitQuit(t *testing.T) {
	selector := NewContextSelector(
		[]string{"first", "second"},
		"first",
		keys.NewGlobalRegistry(),
	)
	if cmd := selector.Init(); cmd == nil {
		t.Fatal("selector Init() returned nil")
	}

	updated, cmd := selector.Update(tea.KeyPressMsg{Code: tea.KeyEnter})
	if updated.IsVisible() {
		t.Fatal("selector remained visible after accept")
	}
	if cmd == nil {
		t.Fatal("selector accept returned nil command")
	}
	result := cmd()
	msg, ok := result.(SelectorFinishedMsg)
	if !ok {
		t.Fatalf("selector accept returned %T, want SelectorFinishedMsg", result)
	}
	if msg.Cancelled || msg.SelectedValue != "first" {
		t.Fatalf("selector accept message = %#v", msg)
	}
}

func TestSelectorAcceptsPasteInFilter(t *testing.T) {
	selector := NewContextSelector(
		[]string{"deployment", "service"},
		"deployment",
		keys.NewGlobalRegistry(),
	)
	selector.Init()

	root := Model{selector: selector}
	updated, _ := root.Update(tea.PasteMsg{Content: "ser"})
	view := ansi.Strip(updated.(Model).selector.View())
	if !strings.Contains(view, "ser") || !strings.Contains(view, "service") {
		t.Fatalf("selector view after paste = %q, want filter text and matching choice", view)
	}
	if strings.Contains(view, "deployment") {
		t.Fatalf("selector view after paste still contains non-matching choice: %q", view)
	}

	shortcutText := NewContextSelector([]string{"enterprise", "service"}, "enterprise", keys.NewGlobalRegistry())
	shortcutText.Init()
	root = Model{selector: shortcutText}
	updated, _ = root.Update(tea.PasteMsg{Content: "enter"})
	got := updated.(Model).selector
	if !got.IsVisible() {
		t.Fatal("pasting the word enter triggered the accept shortcut")
	}
	if view := ansi.Strip(got.View()); !strings.Contains(view, "enterprise") {
		t.Fatalf("selector view after pasting enter = %q, want matching choice", view)
	}
}

func TestSelectorRegistryDrivesActiveHelpAndPalette(t *testing.T) {
	selector := NewContextSelector([]string{"first"}, "first", keys.NewGlobalRegistry())
	m := Model{selector: selector}

	if got := m.CurrentRegistry(); got != selector.selectorRegistry {
		t.Fatal("CurrentRegistry() did not return the visible selector registry")
	}
	entries := m.CurrentRegistry().PaletteEntries()
	if len(entries) != 2 {
		t.Fatalf("selector palette entry count = %d, want 2 accept/cancel actions", len(entries))
	}
}

func TestKeyReleaseDoesNotDispatchCommands(t *testing.T) {
	m := Model{viewMode: ViewModeSplash, globalRegistry: keys.NewGlobalRegistry()}
	_, cmd := m.Update(tea.KeyReleaseMsg{Code: 'q', Text: "q"})
	if cmd != nil {
		t.Fatal("key release unexpectedly produced a command")
	}
}

func TestBackgroundColorMessageAppliesTheme(t *testing.T) {
	m := NewModel(nil, slog.Default(), nil)
	updated, cmd := m.Update(tea.BackgroundColorMsg{Color: color.White})
	if cmd != nil {
		t.Fatalf("background update returned command %v, want nil", cmd)
	}
	got := updated.(Model)
	if got.isDark {
		t.Fatal("zero BackgroundColorMsg should select the light theme")
	}
}

func TestWindowResizeUsesV2ComponentSetters(t *testing.T) {
	m := NewModel(nil, slog.Default(), nil)
	m.viewMode = ViewModeManifest
	m.manifestViewport = viewport.New()

	updated, _ := m.Update(tea.WindowSizeMsg{Width: 100, Height: 40})
	got := updated.(Model)
	if got.manifestViewport.Width() != 96 || got.manifestViewport.Height() != 34 {
		t.Fatalf("manifest viewport size = %dx%d, want 96x34", got.manifestViewport.Width(), got.manifestViewport.Height())
	}
	if got.paletteModel.width != 100 || got.paletteModel.height != 40 {
		t.Fatalf("palette terminal size = %dx%d, want 100x40", got.paletteModel.width, got.paletteModel.height)
	}
	if got.modal.viewport.Width() != 74 || got.modal.viewport.Height() != 13 {
		t.Fatalf("modal viewport size = %dx%d, want 74x13", got.modal.viewport.Width(), got.modal.viewport.Height())
	}
}

func TestManifestCommandsUseV2ViewportScrolling(t *testing.T) {
	m := Model{manifestViewport: viewport.New(viewport.WithWidth(20), viewport.WithHeight(2))}
	m.manifestViewport.SetContent("one\ntwo\nthree\nfour")

	m, _ = m.handleManifestCommand(keys.ManifestCmdScrollDown)
	if got := m.manifestViewport.YOffset(); got != 1 {
		t.Fatalf("offset after scroll down = %d, want 1", got)
	}
	m, _ = m.handleManifestCommand(keys.ManifestCmdPageDown)
	if got := m.manifestViewport.YOffset(); got != 2 {
		t.Fatalf("offset after page down = %d, want 2", got)
	}
	m, _ = m.handleManifestCommand(keys.ManifestCmdScrollUp)
	if got := m.manifestViewport.YOffset(); got != 1 {
		t.Fatalf("offset after scroll up = %d, want 1", got)
	}
}

func TestModalCloseLabelMatchesReachableBindings(t *testing.T) {
	modal := NewModal(keys.NewGlobalRegistry())
	label := modal.closeKeysLabel()
	if !strings.Contains(label, "esc") || !strings.Contains(label, "enter") {
		t.Fatalf("close label = %q, want esc and enter", label)
	}
	if strings.Contains(label, "q") {
		t.Fatalf("close label = %q, q is intercepted as a global quit", label)
	}
}

func TestMouseClickSelectsMatchingRow(t *testing.T) {
	m := Model{
		viewMode:          ViewModeNormal,
		filteredResources: make([]k8s.TrackedObject, 3),
		table: table.New(
			table.WithColumns([]table.Column{{Title: "Name", Width: 10}}),
			table.WithRows([]table.Row{{"a"}, {"b"}, {"c"}}),
		),
	}

	updated, _ := m.handleMouseEvent(tea.MouseClickMsg{
		X:      1,
		Y:      4, // Header offset (3) + row index (1).
		Button: tea.MouseLeft,
	})
	got := updated.(Model)
	if got.selectedIndex != 1 || got.table.Cursor() != 1 {
		t.Fatalf("selection after click: index=%d cursor=%d, want 1/1", got.selectedIndex, got.table.Cursor())
	}
}
