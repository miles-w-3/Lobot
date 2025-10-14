package ui

import (
	tea "github.com/charmbracelet/bubbletea"
	"github.com/miles-w-3/lobot/internal/splash"
)

// Update handles messages and updates the model
func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmd tea.Cmd
	var cmds []tea.Cmd

	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height

		// Update splash screen size
		m.splash.SetSize(m.width, m.height)

		// Update manifest viewport size if in manifest mode
		if m.viewMode == ViewModeManifest {
			m.manifestViewport.Width = m.width - 4
			m.manifestViewport.Height = m.height - 6
		}
		return m, nil

	case splash.TickMsg:
		// Update splash screen
		if m.viewMode == ViewModeSplash {
			m.splash, cmd = m.splash.Update(msg)
			cmds = append(cmds, cmd)

			// Transition to normal mode when splash is done and ready
			if m.splash.IsDone() && m.ready {
				m.viewMode = ViewModeNormal
			}

			return m, tea.Batch(cmds...)
		}

	case ReadyMsg:
		m.SetReady()
		// If splash is done or we're past the animation, transition immediately
		if m.viewMode == ViewModeSplash && m.ready {
			if m.splash.IsDone() {
				m.viewMode = ViewModeNormal
			}
		}
		return m, nil

	case ResourceUpdateMsg:
		m.UpdateResources()
		return m, nil

	case tea.KeyMsg:
		return m.handleKeyPress(msg)

	case tea.MouseMsg:
		return m.handleMouseEvent(msg)
	}

	// Handle filter input updates when in filter mode
	if m.viewMode == ViewModeFilter {
		m.filterInput, cmd = m.filterInput.Update(msg)
		return m, cmd
	}

	// Handle viewport updates in manifest mode
	if m.viewMode == ViewModeManifest {
		m.manifestViewport, cmd = m.manifestViewport.Update(msg)
		return m, cmd
	}

	return m, nil
}

// handleKeyPress handles keyboard input
func (m Model) handleKeyPress(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	// Global keys (work in all modes)
	switch msg.String() {
	case "ctrl+c", "q":
		// Allow quitting from splash mode or normal mode
		if m.viewMode == ViewModeNormal || m.viewMode == ViewModeSplash {
			return m, tea.Quit
		}
	}

	// Mode-specific keys
	switch m.viewMode {
	case ViewModeFilter:
		return m.handleFilterModeKeys(msg)
	case ViewModeManifest:
		return m.handleManifestModeKeys(msg)
	case ViewModeNormal:
		return m.handleNormalModeKeys(msg)
	case ViewModeSplash:
		// No other keys needed in splash mode
		return m, nil
	}

	return m, nil
}

// handleNormalModeKeys handles keys in normal mode
func (m Model) handleNormalModeKeys(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	// Navigation
	case "up", "k":
		m.MoveUp()
	case "down", "j":
		m.MoveDown()
	case "pgup":
		m.PageUp()
	case "pgdown":
		m.PageDown()
	case "home", "g":
		m.selectedIndex = 0
		m.adjustScrollOffset()
	case "end", "G":
		m.selectedIndex = len(m.filteredResources) - 1
		m.adjustScrollOffset()

	// Resource type switching
	case "tab", "l", "right":
		m.NextResourceType()
	case "shift+tab", "h", "left":
		m.PrevResourceType()

	// Filter
	case "/":
		m.EnterFilterMode()

	// View manifest
	case "enter":
		m.EnterManifestMode()

	// Quit
	case "q", "ctrl+c":
		return m, tea.Quit
	}

	return m, nil
}

// handleFilterModeKeys handles keys in filter mode
func (m Model) handleFilterModeKeys(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	var cmd tea.Cmd

	switch msg.String() {
	case "enter":
		// Apply filter
		pattern := m.filterInput.Value()
		m.UpdateFilter(pattern)
		m.ExitFilterMode()
		return m, nil

	case "esc":
		// Cancel filter
		m.ExitFilterMode()
		return m, nil

	default:
		// Update text input
		m.filterInput, cmd = m.filterInput.Update(msg)
	}

	return m, cmd
}

// handleManifestModeKeys handles keys in manifest viewing mode
func (m Model) handleManifestModeKeys(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "esc", "q":
		m.ExitManifestMode()
		return m, nil
	}

	// Viewport handles scrolling
	return m, nil
}

// handleMouseEvent handles mouse input
func (m Model) handleMouseEvent(msg tea.MouseMsg) (tea.Model, tea.Cmd) {
	switch msg.Button {
	case tea.MouseButtonWheelUp:
		m.MoveUp()
	case tea.MouseButtonWheelDown:
		m.MoveDown()
	case tea.MouseButtonLeft:
		if msg.Action == tea.MouseActionPress {
			// Calculate which row was clicked
			// Account for header (2 lines), filter bar (if active), and current scroll
			clickedRow := msg.Y - 3 // Adjust for header and table header
			if m.viewMode == ViewModeFilter {
				clickedRow -= 2 // Account for filter bar
			}

			targetIndex := m.scrollOffset + clickedRow
			if targetIndex >= 0 && targetIndex < len(m.filteredResources) {
				m.selectedIndex = targetIndex
			}
		}
	}

	return m, nil
}

// WaitForResourceUpdate returns a command that waits for resource updates
func WaitForResourceUpdate() tea.Cmd {
	return func() tea.Msg {
		return ResourceUpdateMsg{}
	}
}
