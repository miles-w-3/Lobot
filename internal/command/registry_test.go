package command

import (
	"image/color"
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"
	"github.com/charmbracelet/x/ansi"
)

type testCommand int

const (
	testCommandNone testCommand = iota
	testCommandOpen
	testCommandClose
)

func assertStyleForeground(t *testing.T, name string, got color.Color, want color.RGBA) {
	t.Helper()
	r, g, b, a := got.RGBA()
	actual := color.RGBA{R: uint8(r >> 8), G: uint8(g >> 8), B: uint8(b >> 8), A: uint8(a >> 8)}
	if actual != want {
		t.Fatalf("%s foreground = %#v, want %#v", name, actual, want)
	}
}

func TestFullHelpStylesUseHighContrastPalette(t *testing.T) {
	dark := DefaultHelpStyles(true)
	assertStyleForeground(t, "dark key", dark.FullKey.GetForeground(), color.RGBA{R: 0x5E, G: 0xE6, B: 0xB0, A: 0xFF})
	assertStyleForeground(t, "dark description", dark.FullDesc.GetForeground(), color.RGBA{R: 0xD0, G: 0xD5, B: 0xDD, A: 0xFF})
	assertStyleForeground(t, "dark group", dark.FullGroup.GetForeground(), color.RGBA{R: 0xF2, G: 0xF4, B: 0xF7, A: 0xFF})
	assertStyleForeground(t, "dark separator", dark.FullSeparator.GetForeground(), color.RGBA{R: 0x7C, G: 0x87, B: 0x99, A: 0xFF})

	light := DefaultHelpStyles(false)
	assertStyleForeground(t, "light key", light.FullKey.GetForeground(), color.RGBA{R: 0x08, G: 0x7F, B: 0x5B, A: 0xFF})
	assertStyleForeground(t, "light description", light.FullDesc.GetForeground(), color.RGBA{R: 0x34, G: 0x40, B: 0x54, A: 0xFF})
	assertStyleForeground(t, "light group", light.FullGroup.GetForeground(), color.RGBA{R: 0x10, G: 0x18, B: 0x28, A: 0xFF})
	assertStyleForeground(t, "light separator", light.FullSeparator.GetForeground(), color.RGBA{R: 0x98, G: 0xA2, B: 0xB3, A: 0xFF})
}

func TestRegistryDispatchStringIncludesPrimaryAndAlternateKeys(t *testing.T) {
	r := NewRegistry[testCommand]()
	group := r.NewCommandGroup("Actions")
	group.Add("enter", testCommandOpen).
		WithAlternates("o").
		WithDescription("open")
	group.Add("esc", testCommandClose).
		WithAlternates("q").
		WithDescription("close")

	if err := r.Build(); err != nil {
		t.Fatalf("Build() error = %v", err)
	}

	for key, want := range map[string]testCommand{
		"enter": testCommandOpen,
		"o":     testCommandOpen,
		"esc":   testCommandClose,
		"q":     testCommandClose,
	} {
		t.Run(key, func(t *testing.T) {
			got, err := r.DispatchString(key)
			if err != nil {
				t.Fatalf("DispatchString(%q) error = %v", key, err)
			}
			if got != want {
				t.Fatalf("DispatchString(%q) = %v, want %v", key, got, want)
			}
		})
	}

	if _, err := r.DispatchString("missing"); err == nil {
		t.Fatal("DispatchString(missing) unexpectedly succeeded")
	}
}

func TestRegistryDispatchesV2KeyPress(t *testing.T) {
	r := NewRegistry[testCommand]()
	group := r.NewCommandGroup("Actions")
	group.Add("space", testCommandOpen)
	if err := r.Build(); err != nil {
		t.Fatalf("Build() error = %v", err)
	}

	got, err := r.Dispatch(tea.KeyPressMsg{Code: tea.KeySpace, Text: " "})
	if err != nil {
		t.Fatalf("Dispatch(space) error = %v", err)
	}
	if got != testCommandOpen {
		t.Fatalf("Dispatch(space) = %v, want %v", got, testCommandOpen)
	}
}

func TestRegistryViewsAndPaletteComeFromBindings(t *testing.T) {
	r := NewRegistry[testCommand]()
	group := r.NewCommandGroup("Actions")
	group.Add("enter", testCommandOpen).
		WithAlternates("o").
		WithDisplay("↵").
		WithDescription("open item").
		WithSearchTerms("select")

	if err := r.Build(); err != nil {
		t.Fatalf("Build() error = %v", err)
	}

	presentation := r.Presentation()
	config := NewHelpConfig()
	short := presentation.ShortView(config)
	if !strings.Contains(short, "↵") || !strings.Contains(short, "open item") {
		t.Fatalf("ShortView() = %q, want binding display and description", short)
	}

	full := ansi.Strip(presentation.FullView(config))
	for _, want := range []string{"Actions:", "enter, o", "open item"} {
		if !strings.Contains(full, want) {
			t.Fatalf("FullView() = %q, want %q", full, want)
		}
	}

	entries := presentation.PaletteEntries
	if len(entries) != 1 {
		t.Fatalf("len(PaletteEntries()) = %d, want 1", len(entries))
	}
	entry := entries[0]
	if entry.Key != "enter" || entry.Display != "↵" || entry.Description != "open item" || entry.Group != "Actions" {
		t.Fatalf("PaletteEntries()[0] = %#v", entry)
	}
	if len(entry.AltKeys) != 1 || entry.AltKeys[0] != "o" {
		t.Fatalf("PaletteEntries()[0].AltKeys = %#v, want [o]", entry.AltKeys)
	}
}

func TestRegistryBuildRejectsConflictingKeys(t *testing.T) {
	r := NewRegistry[testCommand]()
	first := r.NewCommandGroup("First")
	first.Add("x", testCommandOpen)
	second := r.NewCommandGroup("Second")
	second.Add("x", testCommandClose)

	if err := r.Build(); err == nil {
		t.Fatal("Build() unexpectedly accepted a duplicate key across groups")
	}
}
