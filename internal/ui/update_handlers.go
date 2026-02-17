package ui

import (
	tea "github.com/charmbracelet/bubbletea"
	"github.com/miles-w-3/lobot/internal/keys"
)

// openPalette opens the command palette with entries from global and current mode
func (m Model) openPalette() (Model, tea.Cmd) {
	entries := m.globalRegistry.PaletteEntries()

	// Add entries from current mode
	entries = append(entries, m.CurrentRegistry().PaletteEntries()...)

	m.paletteModel.SetEntries(entries)
	m.paletteModel.Reset() // Clear input and reset state
	m.paletteVisible = true
	return m, m.paletteModel.Init()
}

func (m Model) handleGlobalCommand(cmd keys.GlobalCmd) (Model, tea.Cmd) {
	switch cmd {
	case keys.GlobalCmdQuit:
		return m, tea.Quit
	case keys.GlobalCmdHelp:
		// Toggle help modal
		if m.modal.IsVisible() && m.modal.modalType == ModalTypeHelp {
			m.modal.Hide()
		} else {
			// Combine global and mode-specific help
			globalHelp := m.globalRegistry.FullView()
			var modeHelp string

			switch m.viewMode {
			case ViewModeNormal:
				modeHelp = m.normalRegistry.FullView()
			case ViewModeFilter:
				modeHelp = m.filterRegistry.FullView()
			case ViewModeManifest:
				modeHelp = m.manifestRegistry.FullView()
			case ViewModeVisualize:
				if m.visualizer != nil {
					if m.visualizer.mode == VisualizationModeTree {
						modeHelp = m.treeRegistry.FullView()
					} else {
						modeHelp = m.graphRegistry.FullView()
					}
				}
			case ViewModeUtilization:
				modeHelp = m.utilizationRegistry.FullView()
			}

			helpContent := globalHelp + "\n\n" + modeHelp
			m.modal.ShowHelpContent(helpContent)
		}
		return m, nil
	case keys.GlobalCmdPalette:
		return m.openPalette()
	}
	return m, nil
}

func (m Model) handleNormalCommand(cmd keys.NormalCmd) (Model, tea.Cmd) {
	switch cmd {
	case keys.NormalCmdMoveUp:
		m.MoveUp()
	case keys.NormalCmdMoveDown:
		m.MoveDown()
	case keys.NormalCmdPageUp:
		m.PageUp()
	case keys.NormalCmdPageDown:
		m.PageDown()
	case keys.NormalCmdHome:
		for m.table.Cursor() > 0 {
			m.table.MoveUp(1)
		}
		m.selectedIndex = 0
	case keys.NormalCmdEnd:
		for m.table.Cursor() < len(m.filteredResources)-1 {
			m.table.MoveDown(1)
		}
		m.selectedIndex = len(m.filteredResources) - 1
	case keys.NormalCmdNextType:
		m.NextResourceType()
	case keys.NormalCmdPrevType:
		m.PrevResourceType()
	case keys.NormalCmdEnter:
		return m, m.EnterManifestMode()
	case keys.NormalCmdEdit:
		return m, m.EditSelectedResource()
	case keys.NormalCmdVisualize:
		resource := m.GetSelectedResource()
		if resource != nil {
			return m, func() tea.Msg {
				return BuildGraphMsg{Resource: resource}
			}
		}
	case keys.NormalCmdFilter:
		m.EnterFilterMode()
	case keys.NormalCmdRefresh:
		return m, m.startInformerWithSplash(m.CurrentResourceType())
	case keys.NormalCmdToggleFavorites:
		m.showingFavoriteTypes = !m.showingFavoriteTypes
	case keys.NormalCmdNamespaceSelector:
		return m, m.OpenNamespaceSelector()
	case keys.NormalCmdResourceTypeSelector:
		return m, m.OpenResourceTypeSelector()
	case keys.NormalCmdContextSelector:
		return m, m.OpenContextSelector()
	case keys.NormalCmdUtilizationDashboard:
		return m, m.checkMetricsAPIAndOpen()
	case keys.NormalCmdQuit:
		return m, tea.Quit
	}
	return m, nil
}

func (m Model) handleFilterCommand(cmd keys.FilterCmd) (Model, tea.Cmd) {
	switch cmd {
	case keys.FilterCmdAccept:
		pattern := m.filterInput.Value()
		m.UpdateFilter(pattern)
		m.ExitFilterMode()
	case keys.FilterCmdCancel:
		m.ExitFilterMode()
	case keys.FilterCmdClear:
		m.filterInput.SetValue("")
	}
	return m, nil
}

func (m Model) handleManifestCommand(cmd keys.ManifestCmd) (Model, tea.Cmd) {
	switch cmd {
	case keys.ManifestCmdBack:
		return m, m.ExitManifestMode()
	case keys.ManifestCmdEdit:
		return m, m.EditSelectedResource()
	case keys.ManifestCmdCopy:
		model, cmd := m.CopyManifestToClipboard()
		if mModel, ok := model.(Model); ok {
			return mModel, cmd
		}
		return m, cmd
	case keys.ManifestCmdScrollUp:
		m.manifestViewport.LineUp(1)
		return m, nil
	case keys.ManifestCmdScrollDown:
		m.manifestViewport.LineDown(1)
		return m, nil
	case keys.ManifestCmdPageUp:
		m.manifestViewport.HalfViewUp()
		return m, nil
	case keys.ManifestCmdPageDown:
		m.manifestViewport.HalfViewDown()
		return m, nil
	}
	return m, nil
}

func (m Model) handleVisualizerCommand(cmd keys.VisualizerCmd) (Model, tea.Cmd) {
	if cmd == keys.VisualizerCmdBack {
		m.ExitVisualizeMode()
		return m, nil
	}

	if m.visualizer != nil {
		// Pass command to visualizer model
		// Note: m.visualizer is a pointer, but HandleCommand modifies it in place.
		// We need to capture the cmd returned.
		visualizerCmd := m.visualizer.HandleCommand(cmd)
		return m, visualizerCmd
	}
	return m, nil
}

func (m Model) handleUtilizationCommand(cmd keys.UtilizationCmd) (Model, tea.Cmd) {
	switch cmd {
	case keys.UtilizationCmdBack:
		m.ExitUtilizationMode()
		return m, nil
	}
	return m, nil
}

func (m Model) handleTreeCommand(cmd keys.TreeCmd) (Model, tea.Cmd) {
	switch cmd {
	case keys.TreeCmdBack:
		m.ExitVisualizeMode()
		return m, nil
	case keys.TreeCmdSwitchToGraph:
		if m.visualizer != nil && m.visualizer.mode == VisualizationModeTree {
			// Initialize graph visualizer if needed
			if m.visualizer.graphVisualizer == nil {
				m.visualizer.graphVisualizer = NewGraphVisualizerModel(m.visualizer.graph, m.visualizer.width, m.visualizer.height, m.visualizer.graphRegistry)
			}
			m.visualizer.mode = VisualizationModeGraph
		}
		return m, nil
	}
	// Pass other commands to tree visualizer
	if m.visualizer != nil {
		updatedTree, treeCmd := m.visualizer.treeVisualizer.HandleCommand(cmd)
		if updatedTree != nil {
			m.visualizer.treeVisualizer = *updatedTree
		}
		return m, treeCmd
	}
	return m, nil
}

func (m Model) handleGraphCommand(cmd keys.GraphCmd) (Model, tea.Cmd) {
	switch cmd {
	case keys.GraphCmdBack:
		m.ExitVisualizeMode()
		return m, nil
	case keys.GraphCmdSwitchToTree:
		if m.visualizer != nil {
			m.visualizer.mode = VisualizationModeTree
		}
		return m, nil
	}
	// Pass other commands to graph visualizer
	if m.visualizer != nil && m.visualizer.graphVisualizer != nil {
		updatedGraph, graphCmd := m.visualizer.graphVisualizer.HandleCommand(cmd)
		if updatedGraph != nil {
			m.visualizer.graphVisualizer = updatedGraph
		}
		return m, graphCmd
	}
	return m, nil
}
