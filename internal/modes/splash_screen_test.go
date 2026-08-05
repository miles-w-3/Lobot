package modes

import (
	"log/slog"
	"testing"
)

func TestSplashServiceReadyFinishesWaitingSplash(t *testing.T) {
	screen := NewSplashScreen(slog.Default())
	screen.phase = phaseWaiting

	cmd := screen.Update(SplashServiceReadyMsg{})
	if !screen.IsDone() {
		t.Fatal("splash is not done after service readiness")
	}
	if cmd == nil {
		t.Fatal("service readiness returned no navigation command")
	}

	msg := cmd()
	navigate, ok := msg.(NavigateMsg)
	if !ok {
		t.Fatalf("navigation message = %T, want NavigateMsg", msg)
	}
	if navigate.Target != ScreenHome {
		t.Fatalf("navigation target = %v, want %v", navigate.Target, ScreenHome)
	}
}

func TestSplashHasNoCommandPresentation(t *testing.T) {
	screen := NewSplashScreen(slog.Default())
	if presentation := screen.CommandPresentation(); !presentation.Empty() {
		t.Fatalf("splash presentation = %#v, want empty", presentation)
	}
}
