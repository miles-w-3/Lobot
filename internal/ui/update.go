package ui

import (
	"fmt"
	"strings"

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

		// Update table size
		tableHeight := m.height - 10 // Account for header, status, help, borders
		if tableHeight < 5 {
			tableHeight = 5
		}
		m.table.SetHeight(tableHeight)
		m.table.SetWidth(m.width - 4)

		// Update manifest viewport size if in manifest mode
		if m.viewMode == ViewModeManifest {
			m.manifestViewport.Width = m.width - 4
			m.manifestViewport.Height = m.height - 6
		}

		// Update modal size
		modalWidth := min(80, m.width-10)
		modalHeight := min(20, m.height-10)
		m.alertModal.SetSize(modalWidth, modalHeight)

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

	case SelectorFinishedMsg:
		if !msg.Cancelled {
			switch msg.SelectorType {
			case SelectorTypeNamespace:
				m.ApplyNamespaceSelection(msg.SelectedValue)
			case SelectorTypeContext:
				// TODO: Implement context switching
				m.statusMessage = "Context switching not yet implemented"
			case SelectorTypeResourceType:
				return m, m.ApplyResourceTypeSelection(msg.SelectedValue)
			}
		}
		return m, nil

	case BuildGraphMsg:
		// Build the graph for the resource
		if msg.Resource != nil && m.graphBuilder != nil {
			resourceGraph := m.graphBuilder.BuildGraph(msg.Resource)
			visualizer := NewVisualizerModel(resourceGraph, m.width, m.height)
			m.visualizer = &visualizer
			m.viewMode = ViewModeVisualize
			m.statusMessage = fmt.Sprintf("Visualizing %s/%s", msg.Resource.Kind, msg.Resource.Name)
		}
		return m, nil

	case EditorFinishedMsg:
		if msg.Err != nil {
			// Show error in modal instead of status message
			errStr := msg.Err.Error()

			var title string
			var message string

			// Detect specific error types and provide helpful messages
			if strings.Contains(errStr, "conflict:") {
				title = "Conflict Detected"
				message = "The resource was modified on the cluster after you opened the editor.\n\n" +
					"The resource version has changed. Please try editing again to get the latest version."
			} else if strings.Contains(errStr, "validation failed:") {
				title = "Validation Failed"
				message = "The edited manifest failed Kubernetes validation.\n\n" +
					"Please check that all required fields are present and valid.\n\n" +
					"Your changes have been saved to /tmp for recovery."
			} else if strings.Contains(errStr, "not found:") {
				title = "Resource Not Found"
				message = "The resource no longer exists on the cluster.\n\n" +
					"It may have been deleted while you were editing.\n\n" +
					"Your changes have been saved to /tmp for recovery."
			} else if strings.Contains(errStr, "cannot change resource") {
				title = "Invalid Edit"
				message = "Cannot change immutable fields (name, kind, or namespace).\n\n" +
					"These fields are read-only after resource creation.\n\n" +
					"Your changes have been saved to /tmp for recovery."
			} else if strings.Contains(errStr, "failed to parse edited YAML") {
				title = "YAML Syntax Error"
				message = "The edited YAML contains syntax errors.\n\n" +
					"Please check your YAML formatting.\n\n" +
					"Your changes have been saved to /tmp for recovery."
			} else if strings.Contains(errStr, "editor exited with error") {
				title = "Editor Error"
				message = "The editor exited with an error.\n\n" +
					"Your changes were not saved."
			} else if strings.Contains(errStr, "forbidden:") {
				title = "Permission Denied"
				message = "You don't have permission to update this resource.\n\n" +
					"Check your RBAC permissions.\n\n" +
					"Your changes have been saved to /tmp for recovery."
			} else {
				title = "Edit Failed"
				message = fmt.Sprintf("An error occurred while editing the resource:\n\n%s", errStr)
			}

			m.alertModal.ShowError(title, message)
			m.statusMessage = "⚠ Edit failed - see modal for details"
		} else {
			// Success case - show success modal briefly
			m.alertModal.ShowSuccess("Edit Successful", "Resource has been updated on the cluster.")
			m.statusMessage = "✓ Resource updated successfully"
			// Trigger a resource refresh to show any updates
			m.UpdateResources()
		}
		return m, nil

	case tea.KeyMsg:
		// Global quit handler - highest priority
		if msg.String() == "ctrl+c" {
			return m, tea.Quit
		}

		// Selector gets highest priority after ctrl+c
		if m.selector != nil && m.selector.IsVisible() {
			m.selector, cmd = m.selector.Update(msg)
			return m, cmd
		}

		// Modal gets priority for key handling
		if m.alertModal.IsVisible() {
			m.alertModal, cmd = m.alertModal.Update(msg)
			return m, cmd
		}
		return m.handleKeyPress(msg)

	case tea.MouseMsg:
		// Modal gets priority for mouse events too
		if m.alertModal.IsVisible() {
			m.alertModal, cmd = m.alertModal.Update(msg)
			return m, cmd
		}
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

	// Handle visualizer updates in visualize mode
	if m.viewMode == ViewModeVisualize && m.visualizer != nil {
		updatedVisualizer, cmd := m.visualizer.Update(msg)
		m.visualizer = &updatedVisualizer
		return m, cmd
	}

	// Handle table updates in normal mode
	if m.viewMode == ViewModeNormal {
		m.table, cmd = m.table.Update(msg)
		m.selectedIndex = m.table.Cursor()
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
	case ViewModeVisualize:
		return m.handleVisualizeModeKeys(msg)
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
	// Navigation (handled by table component, but we keep these for consistency)
	case "up", "k":
		m.MoveUp()
	case "down", "j":
		m.MoveDown()
	case "pgup":
		m.PageUp()
	case "pgdown":
		m.PageDown()
	case "home", "g":
		// Move to top
		for m.table.Cursor() > 0 {
			m.table.MoveUp(1)
		}
		m.selectedIndex = 0
	case "end", "G":
		// Move to bottom
		for m.table.Cursor() < len(m.filteredResources)-1 {
			m.table.MoveDown(1)
		}
		m.selectedIndex = len(m.filteredResources) - 1

	// Resource type switching
	case "tab", "l", "right":
		m.NextResourceType()
	case "shift+tab", "h", "left":
		m.PrevResourceType()

	// Filter by resource name
	case "/":
		m.EnterFilterMode()

	// Namespace selector
	case "ctrl+n":
		return m, m.OpenNamespaceSelector()

	// Resource type selector (ctrl+t for "Type")
	case "ctrl+t":
		return m, m.OpenResourceTypeSelector()

	// View manifest
	case "enter":
		m.EnterManifestMode()

	// Edit resource with external editor (Shift+E)
	case "E":
		m.statusMessage = "Opening editor..."
		return m, m.EditSelectedResource()

	// Visualize resource relationships (Shift+V)
	case "V":
		resource := m.GetSelectedResource()
		if resource != nil {
			return m, func() tea.Msg {
				return BuildGraphMsg{Resource: resource}
			}
		}

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
	var cmd tea.Cmd

	switch msg.String() {
	case "esc", "q":
		m.ExitManifestMode()
		return m, nil

	// Edit resource with external editor (E key in manifest mode)
	case "e":
		m.statusMessage = "Opening editor..."
		return m, m.EditSelectedResource()
	}

	// Pass message to viewport for scrolling
	m.manifestViewport, cmd = m.manifestViewport.Update(msg)
	return m, cmd
}

// handleVisualizeModeKeys handles keys in visualization mode
func (m Model) handleVisualizeModeKeys(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "esc", "q":
		m.ExitVisualizeMode()
		return m, nil
	}

	// Other keys are handled by the visualizer component itself
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

