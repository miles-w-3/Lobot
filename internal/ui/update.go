package ui

import (
	"fmt"
	"log/slog"
	"strings"

	tea "charm.land/bubbletea/v2"
	"github.com/miles-w-3/lobot/internal/k8s"
	"github.com/miles-w-3/lobot/internal/keys"
	"github.com/miles-w-3/lobot/internal/splash"
)

// MetricsCheckMsg is sent after checking if metrics API is available
type MetricsCheckMsg struct {
	Available bool
}

// MetricsDataMsg is sent when metrics data is fetched
type MetricsDataMsg struct {
	NodeMetrics []k8s.NodeMetrics
	PodMetrics  []k8s.PodMetrics
	Error       error
}

// Update handles messages and updates the model
func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmd tea.Cmd
	var cmds []tea.Cmd

	// The palette owns text and paste input while visible. Application messages
	// such as PaletteSelectedMsg still fall through to the main switch.
	if m.paletteVisible {
		switch input := msg.(type) {
		case tea.KeyPressMsg:
			if cmd, err := m.globalRegistry.Dispatch(input); err == nil {
				m.paletteVisible = false
				return m.handleGlobalCommand(cmd)
			}
			m.paletteModel, cmd = m.paletteModel.Update(input)
			return m, cmd
		case tea.PasteMsg:
			m.paletteModel, cmd = m.paletteModel.Update(input)
			return m, cmd
		default:
			// Cursor blink messages are private implementation details in
			// Bubbles. Forward unknown messages and consume them only when the
			// palette produces a follow-up command.
			m.paletteModel, cmd = m.paletteModel.Update(input)
			if cmd != nil {
				return m, cmd
			}
		}
	}

	if m.selector != nil && m.selector.IsVisible() {
		switch input := msg.(type) {
		case tea.PasteMsg:
			m.selector, cmd = m.selector.Update(input)
			return m, cmd
		case tea.KeyPressMsg:
			// Key presses are routed below so global commands retain priority.
		default:
			m.selector, cmd = m.selector.Update(input)
			if cmd != nil {
				return m, cmd
			}
		}
	}

	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		m.paletteModel.SetSize(m.width, m.height)

		// Update splash screen size
		m.splash.SetSize(m.width, m.height)

		// Update table size
		tableHeight := m.height - 10 // Account for header, status, help, borders
		if tableHeight < 5 {
			tableHeight = 5
		}
		m.table.SetHeight(tableHeight)
		// Table width should fill the border container (border takes 2 chars, content has 4 chars padding)
		m.table.SetWidth(m.width - 6 - 10) // TODO: -10 for favorites

		// Update manifest viewport size if in manifest mode
		if m.viewMode == ViewModeManifest {
			m.manifestViewport.SetWidth(m.width - 4)
			m.manifestViewport.SetHeight(m.height - 6)
		}

		// Update modal size
		modalWidth := min(80, m.width-10)
		modalHeight := min(20, m.height-10)
		m.modal.SetSize(modalWidth, modalHeight)

		return m, nil

	case tea.BackgroundColorMsg:
		m.applyTheme(msg.IsDark())
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
		// Log error to error.log file
		if m.errorTracker != nil {
			m.errorTracker.LogError("system", msg.Error.Error())
		}

		if m.viewMode == ViewModeSplash || !m.ready {
			m.viewMode = ViewModeSplash
			m.ready = false
			m.splash.MarkError(msg.Error)
		} else {
			m.modal.ShowError("Error", msg.Error.Error())
		}
		return m, nil

	case ResourceUpdateMsg:
		m.UpdateResources()
		return m, nil

	case SelectorFinishedMsg:
		logger := slog.Default()
		logger.Debug("SelectorFinishedMsg received",
			"cancelled", msg.Cancelled,
			"selectorType", msg.SelectorType,
			"selectedValue", msg.SelectedValue,
			"selectorIsNil", m.selector == nil)

		if !msg.Cancelled {
			switch msg.SelectorType {
			case SelectorTypeNamespace:
				m.ApplyNamespaceSelection(msg.SelectedValue)
			case SelectorTypeContext:
				// Initiate context switch
				return m, m.SwitchContext(msg.SelectedValue)
			case SelectorTypeResourceType:
				return m, m.ApplyResourceTypeSelection(msg.SelectedValue)
			}
		}
		return m, nil

	case BuildGraphMsg:
		// Build the graph for the resource
		if msg.Resource != nil {
			resourceGraph := m.graphBuilder.BuildGraph(msg.Resource)
			visualizer := NewVisualizerModel(resourceGraph, m.width, m.height, m.treeRegistry, m.graphRegistry)
			m.visualizer = &visualizer
			m.viewMode = ViewModeVisualize
		}
		return m, nil

	case MetricsCheckMsg:
		if !msg.Available {
			m.modal.ShowError("Metrics Unavailable",
				"The Kubernetes metrics API is not available.\n\n"+
					"Please ensure metrics-server is installed and running:\n"+
					"kubectl apply -f https://github.com/kubernetes-sigs/metrics-server/releases/latest/download/components.yaml")
			return m, nil
		}
		// Metrics API is available, fetch metrics data
		return m, m.fetchMetricsData()

	case MetricsDataMsg:
		if msg.Error != nil {
			if m.errorTracker != nil {
				m.errorTracker.LogError("metrics", msg.Error.Error())
			}
			m.modal.ShowError("Metrics Error", "Failed to fetch metrics: "+msg.Error.Error())
			return m, nil
		}
		// Create and show the utilization dashboard
		dashboard := NewUtilizationDashboardModel(msg.NodeMetrics, msg.PodMetrics, m.width, m.height, m.utilizationRegistry)
		m.utilizationDashboard = &dashboard
		m.viewMode = ViewModeUtilization
		return m, nil

	case PaletteSelectedMsg:
		m.paletteVisible = false
		key := msg.Entry.Key

		if cmd, err := m.globalRegistry.DispatchString(key); err == nil {
			return m.handleGlobalCommand(cmd)
		}
		if m.selector != nil && m.selector.IsVisible() {
			if selectorCmd, err := m.selector.selectorRegistry.DispatchString(key); err == nil {
				m.selector, cmd = m.selector.HandleCommand(selectorCmd)
				return m, cmd
			}
		}

		// Mode specific
		switch m.viewMode {
		case ViewModeSplash:
			if cmd, err := m.splashRegistry.DispatchString(key); err == nil {
				return m.handleSplashCommand(cmd)
			}
		case ViewModeNormal:
			if cmd, err := m.normalRegistry.DispatchString(key); err == nil {
				return m.handleNormalCommand(cmd)
			}
		case ViewModeFilter:
			if cmd, err := m.filterRegistry.DispatchString(key); err == nil {
				return m.handleFilterCommand(cmd)
			}
		case ViewModeManifest:
			if cmd, err := m.manifestRegistry.DispatchString(key); err == nil {
				return m.handleManifestCommand(cmd)
			}
		case ViewModeVisualize:
			if m.visualizer != nil {
				if m.visualizer.mode == VisualizationModeTree {
					if cmd, err := m.treeRegistry.DispatchString(key); err == nil {
						return m.handleTreeCommand(cmd)
					}
				} else {
					if cmd, err := m.graphRegistry.DispatchString(key); err == nil {
						return m.handleGraphCommand(cmd)
					}
				}
			}
		case ViewModeUtilization:
			if cmd, err := m.utilizationRegistry.DispatchString(key); err == nil {
				return m.handleUtilizationCommand(cmd)
			}
		}
		return m, nil

	case PaletteBackMsg:
		m.paletteVisible = false
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
					"Please check that all required fields are present and valid."
			} else if strings.Contains(errStr, "not found:") {
				title = "Resource Not Found"
				message = "The resource no longer exists on the cluster.\n\n" +
					"It may have been deleted while you were editing."
			} else if strings.Contains(errStr, "cannot change resource") {
				title = "Invalid Edit"
				message = "Cannot change immutable fields (name, kind, or namespace).\n\n" +
					"These fields are read-only after resource creation."
			} else if strings.Contains(errStr, "failed to parse edited YAML") {
				title = "YAML Syntax Error"
				message = "The edited YAML contains syntax errors.\n\n" +
					"Please check your YAML formatting."
			} else if strings.Contains(errStr, "editor exited with error") {
				title = "Editor Error"
				message = "The editor exited with an error.\n\n" +
					"Your changes were not saved."
			} else if strings.Contains(errStr, "forbidden:") {
				title = "Permission Denied"
				message = "You don't have permission to update this resource.\n\n" +
					"Check your RBAC permissions."
			} else {
				title = "Edit Failed"
				message = fmt.Sprintf("An error occurred while editing the resource:\n\n%s", errStr)
			}

			m.modal.ShowError(title, message)
		} else {
			// Success case - silent (like vim :wq)
			// Trigger a resource refresh to show any updates
			m.UpdateResources()
			// If in manifest mode, refresh the manifest view with the updated resource
			m.RefreshManifestResource()
		}
		return m, nil

	case tea.KeyPressMsg:
		// Global keys - highest priority, always check global registry first
		slog.Debug("Received key msg", "key", msg.String())
		if cmd, err := m.globalRegistry.Dispatch(msg); err == nil {
			slog.Debug("Dispatch succeeded", "cmd", cmd)
			return m.handleGlobalCommand(cmd)
		}

		// Check for visible overlays BEFORE mode-specific registries
		// Overlays should intercept input before underlying view processes it

		// Selector gets priority when visible
		if m.selector != nil && m.selector.IsVisible() {
			m.selector, cmd = m.selector.Update(msg)
			return m, cmd
		}

		// Modal gets priority when visible
		if m.modal != nil && m.modal.IsVisible() {
			var cmd tea.Cmd
			m.modal, cmd = m.modal.Update(msg)
			return m, cmd
		}

		// Mode-specific dispatch only if no overlays are visible
		switch m.viewMode {
		case ViewModeSplash:
			if m.splash.IsError() {
				if cmd, err := m.splashRegistry.Dispatch(msg); err == nil {
					return m.handleSplashCommand(cmd)
				}
			}
		case ViewModeNormal:
			if cmd, err := m.normalRegistry.Dispatch(msg); err == nil {
				return m.handleNormalCommand(cmd)
			}
		case ViewModeFilter:
			if cmd, err := m.filterRegistry.Dispatch(msg); err == nil {
				return m.handleFilterCommand(cmd)
			}
			// If key doesn't match a filter command, pass it to filter input for typing
			m.filterInput, cmd = m.filterInput.Update(msg)
			return m, cmd
		case ViewModeManifest:
			if cmd, err := m.manifestRegistry.Dispatch(msg); err == nil {
				return m.handleManifestCommand(cmd)
			}
		case ViewModeVisualize:
			// Handle visualizer commands based on current mode (tree or graph)
			if m.visualizer != nil {
				if m.visualizer.mode == VisualizationModeTree {
					if cmd, err := m.treeRegistry.Dispatch(msg); err == nil {
						return m.handleTreeCommand(cmd)
					}
				} else {
					if cmd, err := m.graphRegistry.Dispatch(msg); err == nil {
						return m.handleGraphCommand(cmd)
					}
				}
				// Pass unhandled keys to visualizer for direct handling
				updated, cmd := m.visualizer.Update(msg)
				m.visualizer = &updated
				return m, cmd
			}
		case ViewModeUtilization:
			if m.utilizationDashboard != nil {
				if cmd, err := m.utilizationRegistry.Dispatch(msg); err == nil && cmd == keys.UtilizationCmdBack && !m.utilizationDashboard.showNodeDetails {
					m.ExitUtilizationMode()
					return m, nil
				}

				updated, cmd := m.utilizationDashboard.Update(msg)
				m.utilizationDashboard = &updated
				return m, cmd
			}
		}

		return m, nil

	case tea.MouseMsg:
		// Do not let pointer events affect content behind an overlay.
		if m.paletteVisible || (m.selector != nil && m.selector.IsVisible()) {
			return m, nil
		}
		if m.modal.IsVisible() {
			m.modal, cmd = m.modal.Update(msg)
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

// handleMouseEvent handles mouse input
func (m Model) handleMouseEvent(msg tea.MouseMsg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.MouseWheelMsg:
		switch msg.Button {
		case tea.MouseWheelUp:
			m.MoveUp()
		case tea.MouseWheelDown:
			m.MoveDown()
		}
	case tea.MouseClickMsg:
		if msg.Button == tea.MouseLeft {
			// Calculate which row was clicked, accounting for the table header.
			clickedRow := msg.Y - 3
			if m.viewMode == ViewModeFilter {
				clickedRow -= 2
			}

			targetIndex := m.scrollOffset + clickedRow
			if targetIndex >= 0 && targetIndex < len(m.filteredResources) {
				m.selectedIndex = targetIndex
				m.table.SetCursor(targetIndex)
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
