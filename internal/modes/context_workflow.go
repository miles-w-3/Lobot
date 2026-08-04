package modes

import (
	"fmt"

	tea "charm.land/bubbletea/v2"
)

func (r *RootModel) handleContextSwitch(msg ContextSwitchRequestedMsg) tea.Cmd {
	if msg.ContextName == "" {
		return nil
	}
	if r.resourceService == nil {
		r.showModal("Context Switch Failed", "The Kubernetes resource service is unavailable.")
		return nil
	}

	// Home owns the cluster-specific selection/filter state. Tell it to reset
	// before replacing the active screen with a fresh Splash lifecycle.
	r.updateCurrentScreen(ContextSwitchStartedMsg{})
	r.commandPaletteVisible = false
	r.modal.Hide()
	activateCmd := r.activateScreen(ScreenSplash)
	service := r.resourceService

	return tea.Batch(activateCmd, func() tea.Msg {
		if err := service.SwitchContext(msg.ContextName); err != nil {
			return ErrorMsg{Error: fmt.Errorf("context switch failed: %w", err)}
		}
		return nil
	})
}
