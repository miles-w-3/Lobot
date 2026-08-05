package modes

import (
	"testing"

	tea "charm.land/bubbletea/v2"
)

func TestSelectorDispatchesNavigationAndAcceptsSelection(t *testing.T) {
	selector := NewSelectorModel(
		"Select Namespace",
		[]SelectorOption{
			{Label: "default", Value: "default"},
			{Label: "staging", Value: "staging"},
		},
		"default",
	)
	selector.Update(tea.WindowSizeMsg{Width: 80, Height: 24})

	if result, _ := selector.Update(tea.KeyPressMsg{Text: "down"}); result != nil {
		t.Fatalf("down result = %#v, want nil", result)
	}
	result, _ := selector.Update(tea.KeyPressMsg{Text: "enter"})
	if result == nil || !result.Accepted || result.Value != "staging" {
		t.Fatalf("selection result = %#v, want accepted staging", result)
	}
}

func TestSelectorCancelIsLocalResult(t *testing.T) {
	selector := NewSelectorModel(
		"Select Context",
		[]SelectorOption{{Label: "dev", Value: "dev"}},
		"dev",
	)

	result, cmd := selector.Update(tea.KeyPressMsg{Text: "esc"})
	if cmd != nil {
		t.Fatalf("cancel command = %v, want nil", cmd)
	}
	if result == nil || !result.Cancelled {
		t.Fatalf("cancel result = %#v, want cancelled result", result)
	}
}

func TestHomeOwnsSelectorPresentationAndResult(t *testing.T) {
	screen := NewHomeScreen(nil, nil)
	cmd := screen.Update(tea.KeyPressMsg{Text: "ctrl+n"})
	if cmd == nil {
		t.Fatal("namespace selector request returned no options command")
	}

	msg := cmd()
	options, ok := msg.(NamespaceOptionsReadyMsg)
	if !ok {
		t.Fatalf("options message = %T, want NamespaceOptionsReadyMsg", msg)
	}
	screen.Update(options)
	if screen.selector == nil {
		t.Fatal("Home did not create its selector")
	}
	if screen.CommandPresentation().Empty() {
		t.Fatal("selector presentation is empty")
	}

	screen.Update(tea.KeyPressMsg{Text: "enter"})
	if screen.selector != nil {
		t.Fatal("Home selector remains open after accept")
	}
}
