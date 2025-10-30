package ui

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/bubbles/key"
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
		// Table width should fill the border container (border takes 2 chars, content has 4 chars padding)
		m.table.SetWidth(m.width - 6)

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
		return m, nil

	case ErrorMsg:
		m.err = msg.Error
		m.logger.Debug("Handling error message", "msg", msg.Error)

		// If we're in splash mode, mark splash as error
		if m.viewMode == ViewModeSplash {
			m.splash.MarkError(msg.Error)
		} else {
			// TODO: Use modal
			// In normal mode, show error in status or modal
			m.statusMessage = fmt.Sprintf("Error: %v", msg.Error)
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
				// Initiate context switch
				m.statusMessage = fmt.Sprintf("Switching to context: %s...", msg.SelectedValue)
				return m, m.SwitchContext(msg.SelectedValue)
			case SelectorTypeResourceType:
				return m, m.ApplyResourceTypeSelection(msg.SelectedValue)
			}
		}
		return m, nil

	case ContextSwitchErrorMsg:
		// Context switch failed
		m.alertModal.Show("Context Switch Failed", msg.Error.Error(), ModalTypeError)
		return m, nil

	case BuildGraphMsg:
		// Build the graph for the resource
		if msg.Resource != nil {
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
			// If in manifest mode, refresh the manifest view with the updated resource
			m.RefreshManifestResource()
		}
		return m, nil

	case tea.KeyMsg:
		// Global keys - highest priority
		if key.Matches(msg, m.globalKeys.Quit) {
			return m, tea.Quit
		}
		if key.Matches(msg, m.globalKeys.Help) {
			m.showHelp = !m.showHelp
			return m, nil
		}

		// Selector gets priority after global keys
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
	case "ctrl+c", "q": // TODO: USe the key scheme/binding names
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
		// Allow context switching if we're in error state
		// // TODO: Use key bindings instead of straight up
		if m.splash.IsError() && msg.String() == "c" {
			contexts := m.getAvailableContexts()
			current := m.resourceService.GetCurrentContext()
			m.selector = NewContextSelector(contexts, current)
			return m, m.selector.Init()
		}
		// No other keys needed in splash mode
		return m, nil
	}

	return m, nil
}

// handleNormalModeKeys handles keys in normal mode
func (m Model) handleNormalModeKeys(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch {
	// Navigation
	case key.Matches(msg, m.normalKeys.Up):
		m.MoveUp()
	case key.Matches(msg, m.normalKeys.Down):
		m.MoveDown()
	case key.Matches(msg, m.normalKeys.PageUp):
		m.PageUp()
	case key.Matches(msg, m.normalKeys.PageDown):
		m.PageDown()
	case key.Matches(msg, m.normalKeys.Home):
		// Move to top
		for m.table.Cursor() > 0 {
			m.table.MoveUp(1)
		}
		m.selectedIndex = 0
	case key.Matches(msg, m.normalKeys.End):
		// Move to bottom
		for m.table.Cursor() < len(m.filteredResources)-1 {
			m.table.MoveDown(1)
		}
		m.selectedIndex = len(m.filteredResources) - 1

	// Resource type switching
	case key.Matches(msg, m.normalKeys.NextType):
		m.NextResourceType()
	case key.Matches(msg, m.normalKeys.PrevType):
		m.PrevResourceType()

	// Filter by resource name
	case key.Matches(msg, m.normalKeys.Filter):
		m.EnterFilterMode()

	// Namespace selector
	case key.Matches(msg, m.normalKeys.NamespaceSelector):
		return m, m.OpenNamespaceSelector()

	// Resource type selector
	case key.Matches(msg, m.normalKeys.ResourceTypeSelector):
		return m, m.OpenResourceTypeSelector()

	// Context selector
	case key.Matches(msg, m.normalKeys.ContextSelector):
		return m, m.OpenContextSelector()

	// View manifest
	case key.Matches(msg, m.normalKeys.Enter):
		return m, m.EnterManifestMode()

	// Edit resource with external editor
	case key.Matches(msg, m.normalKeys.Edit):
		m.statusMessage = "Opening editor..."
		return m, m.EditSelectedResource()

	// Visualize resource relationships
	case key.Matches(msg, m.normalKeys.Visualize):
		resource := m.GetSelectedResource()
		if resource != nil {
			return m, func() tea.Msg {
				return BuildGraphMsg{Resource: resource}
			}
		}

	// Quit
	case key.Matches(msg, m.normalKeys.Quit):
		return m, tea.Quit
	}

	return m, nil
}

// handleFilterModeKeys handles keys in filter mode
func (m Model) handleFilterModeKeys(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	var cmd tea.Cmd

	switch {
	case key.Matches(msg, m.filterKeys.Accept):
		// Apply filter
		pattern := m.filterInput.Value()
		m.UpdateFilter(pattern)
		m.ExitFilterMode()
		return m, nil

	case key.Matches(msg, m.filterKeys.Cancel):
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

	switch {
	case key.Matches(msg, m.manifestKeys.Back):
		return m, m.ExitManifestMode()

	case key.Matches(msg, m.manifestKeys.Edit):
		m.statusMessage = "Opening editor..."
		return m, m.EditSelectedResource()

	case key.Matches(msg, m.manifestKeys.Copy):
		return m.CopyManifestToClipboard()
	}

	// Pass message to viewport for scrolling
	m.manifestViewport, cmd = m.manifestViewport.Update(msg)
	return m, cmd
}

// handleVisualizeModeKeys handles keys in visualization mode
func (m Model) handleVisualizeModeKeys(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch {
	case key.Matches(msg, m.visualizerKeys.Back):
		m.ExitVisualizeMode()
		return m, nil
	}

	// Pass all other keys to the visualizer component
	if m.visualizer != nil {
		updatedVisualizer, cmd := m.visualizer.Update(msg)
		m.visualizer = &updatedVisualizer
		return m, cmd
	}

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
