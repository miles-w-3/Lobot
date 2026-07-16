package ui

import (
	"strings"
	"testing"

	"github.com/charmbracelet/x/ansi"
)

func plainViewportConfig() Viewport2DConfig {
	return Viewport2DConfig{ShowScrollIndicators: false}
}

func TestViewport2DScrollClampsToContent(t *testing.T) {
	v := NewViewport2DWithConfig(4, 2, plainViewportConfig())
	v.SetLines([]string{"abcdef", "ghijkl", "mnopqr"})

	v.ScrollTo(100, 100)
	if got, want := v.XOffset, 2; got != want {
		t.Fatalf("XOffset = %d, want %d", got, want)
	}
	if got, want := v.YOffset, 1; got != want {
		t.Fatalf("YOffset = %d, want %d", got, want)
	}
	if !v.AtRight() || !v.AtBottom() {
		t.Fatalf("viewport should be at bottom-right: x=%d y=%d", v.XOffset, v.YOffset)
	}

	v.ScrollBy(-100, -100)
	if v.XOffset != 0 || v.YOffset != 0 {
		t.Fatalf("negative scroll was not clamped: x=%d y=%d", v.XOffset, v.YOffset)
	}
}

func TestViewport2DEnsureVisible(t *testing.T) {
	v := NewViewport2DWithConfig(5, 3, plainViewportConfig())
	v.SetLines([]string{
		"0123456789",
		"abcdefghij",
		"ABCDEFGHIJ",
		"klmnopqrst",
		"KLMNOPQRST",
	})

	v.EnsureVisible(7, 3, 2, 2)
	if got, want := v.XOffset, 4; got != want {
		t.Fatalf("XOffset = %d, want %d", got, want)
	}
	if got, want := v.YOffset, 2; got != want {
		t.Fatalf("YOffset = %d, want %d", got, want)
	}

	v.EnsureVisible(0, 0, 1, 1)
	if v.XOffset != 0 || v.YOffset != 0 {
		t.Fatalf("EnsureVisible did not return to origin: x=%d y=%d", v.XOffset, v.YOffset)
	}
}

func TestViewport2DViewPreservesVisualWidthWithANSI(t *testing.T) {
	v := NewViewport2DWithConfig(5, 2, plainViewportConfig())
	v.SetLines([]string{"\x1b[31mabcdef\x1b[0m", "xy"})

	view := v.View()
	lines := strings.Split(view, "\n")
	if len(lines) != 2 {
		t.Fatalf("View() line count = %d, want 2: %q", len(lines), view)
	}
	for i, line := range lines {
		if got := ansi.StringWidth(line); got != 5 {
			t.Fatalf("line %d visual width = %d, want 5: %q", i, got, line)
		}
	}
	if got := ansi.Strip(lines[0]); got != "abcde" {
		t.Fatalf("first visible line = %q, want %q", got, "abcde")
	}
	if got := ansi.Strip(lines[1]); got != "xy   " {
		t.Fatalf("second visible line = %q, want %q", got, "xy   ")
	}
}
