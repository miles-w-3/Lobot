package ui

import (
	"context"
	"log/slog"
	"sort"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/erikgeiser/promptkit/selection"
	"github.com/miles-w-3/lobot/internal/command"
	"github.com/miles-w-3/lobot/internal/k8s"
	"github.com/miles-w-3/lobot/internal/keys"
)

// SelectorType represents the type of selector
type SelectorType int

const (
	SelectorTypeNamespace SelectorType = iota
	SelectorTypeContext
	SelectorTypeResourceType
)

// SelectorModel wraps the promptkit selection model
type SelectorModel struct {
	selection      *selection.Model[string]
	selectorType   SelectorType
	visible        bool
	globalRegistry *command.Registry[keys.GlobalCmd]
}

// SelectorFinishedMsg is sent when a selector finishes
type SelectorFinishedMsg struct {
	SelectedValue string
	SelectorType  SelectorType
	Cancelled     bool
}

// NewNamespaceSelector creates a new namespace selector
func NewNamespaceSelector(namespaces []string, current string, registry *command.Registry[keys.GlobalCmd]) *SelectorModel {
	// Add <all> option at the top
	choices := append([]string{"<all>"}, namespaces...)

	// Create the selection
	sel := selection.New("Select Namespace:", choices)
	sel.Filter = selection.FilterContainsCaseInsensitive // Enable searchable filtering
	sel.LoopCursor = true

	// Create the selection model
	model := selection.NewModel(sel)

	return &SelectorModel{
		selection:      model,
		selectorType:   SelectorTypeNamespace,
		visible:        true,
		globalRegistry: registry,
	}
}

// NewContextSelector creates a new context selector
func NewContextSelector(contexts []string, current string, registry *command.Registry[keys.GlobalCmd]) *SelectorModel {
	sel := selection.New("Select Cluster Context:", contexts)
	sel.Filter = selection.FilterContainsCaseInsensitive // Enable searchable filtering
	sel.LoopCursor = true

	// Create the selection model
	model := selection.NewModel(sel)

	return &SelectorModel{
		selection:      model,
		selectorType:   SelectorTypeContext,
		visible:        true,
		globalRegistry: registry,
	}
}

// Init initializes the selector
func (s *SelectorModel) Init() tea.Cmd {
	return s.selection.Init()
}

// Update handles messages
func (s *SelectorModel) Update(msg tea.Msg) (*SelectorModel, tea.Cmd) {
	logger := slog.Default()

	if !s.visible {
		return s, nil
	}

	// Track if this is a KeyMsg and what key was pressed
	var keyStr string
	if keyMsg, ok := msg.(tea.KeyMsg); ok {
		keyStr = keyMsg.String()
		logger.Debug("Selector received key", "key", keyStr, "type", s.selectorType)

		// Check for esc to cancel
		if keyStr == "esc" {
			logger.Debug("Selector: esc pressed, cancelling")
			s.visible = false
			return s, func() tea.Msg {
				return SelectorFinishedMsg{
					SelectorType: s.selectorType,
					Cancelled:    true,
				}
			}
		}

		// Check global registry for quit/help
		if cmd, err := s.globalRegistry.Dispatch(keyMsg); err == nil {
			switch cmd {
			case keys.GlobalCmdQuit, keys.GlobalCmdHelp:
				logger.Debug("Selector: global command received, cancelling", "cmd", cmd)
				s.visible = false
				return s, func() tea.Msg {
					return SelectorFinishedMsg{
						SelectorType: s.selectorType,
						Cancelled:    true,
					}
				}
			}
		}
	}

	// Pass all other messages to underlying selection model
	logger.Debug("Selector: passing to promptkit", "msgType", "tea.KeyMsg")
	_, cmd := s.selection.Update(msg)

	// Only check for completion when Enter is pressed
	if keyStr == "enter" {
		if val, err := s.selection.Value(); err == nil && val != "" && s.visible {
			logger.Debug("Selector: enter pressed with value, completing", "value", val)
			s.visible = false
			return s, func() tea.Msg {
				return SelectorFinishedMsg{
					SelectedValue: val,
					SelectorType:  s.selectorType,
					Cancelled:     false,
				}
			}
		}
	}

	return s, cmd
}

// View renders the selector
func (s *SelectorModel) View() string {
	if !s.visible {
		return ""
	}
	return s.selection.View()
}

// IsVisible returns whether the selector is currently visible
func (s *SelectorModel) IsVisible() bool {
	return s.visible
}

// Helper functions for Model

// getAllNamespaces queries the Kubernetes API for all namespaces
func (m *Model) getAllNamespaces() []string {
	namespaces, err := m.resourceService.GetAllNamespaces(context.Background())
	if err != nil {
		return m.getNamespacesFromResources()
	}

	sort.Strings(namespaces)
	return namespaces
}

// getNamespacesFromResources is a fallback that extracts namespaces from current resources
func (m *Model) getNamespacesFromResources() []string {
	namespaceSet := make(map[string]bool)

	for _, resource := range m.resources {
		if resource.GetNamespace() != "" {
			namespaceSet[resource.GetNamespace()] = true
		}
	}

	namespaces := make([]string, 0, len(namespaceSet))
	for ns := range namespaceSet {
		namespaces = append(namespaces, ns)
	}

	sort.Strings(namespaces)
	return namespaces
}

// getAvailableContexts gets available cluster contexts
func (m *Model) getAvailableContexts() []string {
	contexts, _, err := m.resourceService.GetAvailableContexts()
	if err != nil {
		current := m.resourceService.GetCurrentContext()
		if current != "" {
			return []string{current}
		}
		return []string{"default"}
	}
	return contexts
}

// OpenNamespaceSelector opens the namespace selector
func (m *Model) OpenNamespaceSelector() tea.Cmd {
	logger := slog.Default()
	namespaces := m.getAllNamespaces()
	current := m.namespaceFilter.GetPattern()
	logger.Debug("Opening namespace selector", "namespaceCount", len(namespaces), "current", current)
	m.selector = NewNamespaceSelector(namespaces, current, m.globalRegistry)
	return m.selector.Init()
}

// OpenContextSelector opens the context selector
func (m *Model) OpenContextSelector() tea.Cmd {
	logger := slog.Default()
	contexts := m.getAvailableContexts()
	current := m.resourceService.GetCurrentContext()
	logger.Debug("Opening context selector", "contextCount", len(contexts), "current", current)
	m.selector = NewContextSelector(contexts, current, m.globalRegistry)
	return m.selector.Init()
}

// ApplyNamespaceSelection applies the selected namespace filter
func (m *Model) ApplyNamespaceSelection(namespace string) {
	if namespace == "<all>" {
		// Clear the filter to show all namespaces
		m.namespaceFilter.SetPattern("")
	} else {
		// Set exact namespace filter
		m.namespaceFilter.SetPattern(namespace)
	}
	m.UpdateResources()
}

// NewResourceTypeSelector creates a new resource type selector
func NewResourceTypeSelector(resourceTypes []string, registry *command.Registry[keys.GlobalCmd]) *SelectorModel {
	sel := selection.New("Select Resource Type:", resourceTypes)
	sel.Filter = selection.FilterContainsCaseInsensitive // Enable searchable filtering
	sel.LoopCursor = true

	// Create the selection model
	model := selection.NewModel(sel)

	return &SelectorModel{
		selection:      model,
		selectorType:   SelectorTypeResourceType,
		visible:        true,
		globalRegistry: registry,
	}
}

// OpenResourceTypeSelector opens the resource type selector
func (m *Model) OpenResourceTypeSelector() tea.Cmd {
	logger := slog.Default()
	resourceTypes := m.getAllResourceTypes()
	logger.Debug("Opening resource type selector", "typeCount", len(resourceTypes))
	m.selector = NewResourceTypeSelector(resourceTypes, m.globalRegistry)
	return m.selector.Init()
}

// getAllResourceTypes returns all available resource types for selection
func (m *Model) getAllResourceTypes() []string {
	// Discover all resources from the cluster
	discovered, err := m.resourceService.GetAllResourceTypes()
	if err != nil {
		// Fallback to default types if discovery fails
		displayNames := make([]string, len(m.trackedTypes))
		for i, rt := range m.trackedTypes {
			displayNames[i] = rt.DisplayName
		}
		return displayNames
	}

	// Return discovered types (alphabetically sorted by discovery)
	displayNames := make([]string, len(discovered))
	for i, rt := range discovered {
		displayNames[i] = rt.DisplayName
	}
	return displayNames
}

// ApplyResourceTypeSelection applies the selected resource type
func (m *Model) ApplyResourceTypeSelection(displayName string) tea.Cmd {
	// Discover all resource types to find the selected one
	discovered, err := m.resourceService.GetAllResourceTypes()
	if err != nil {
		m.modal.ShowError("Discovery Failed", "Failed to discover resource types: "+err.Error())
		return nil
	}

	// Find the resource type by display name
	var selectedType *k8s.TrackedType
	for i := range discovered {
		if discovered[i].DisplayName == displayName {
			selectedType = discovered[i]
			break
		}
	}

	if selectedType == nil {
		m.modal.ShowError("Not Found", "Resource type not found: "+displayName)
		return nil
	}

	// Check if this type is already in our rotation
	typeIndex := -1
	for i := range m.trackedTypes {
		if m.trackedTypes[i].GVR == selectedType.GVR {
			typeIndex = i
			break
		}
	}

	if typeIndex >= 0 {
		// Already in rotation, just switch to it
		m.currentType = typeIndex
		m.selectedIndex = 0
		m.scrollOffset = 0
		m.UpdateResources()
		return nil
	}

	// New type - add it to rotation and start informer
	m.trackedTypes = append(m.trackedTypes, selectedType)
	m.currentType = len(m.trackedTypes) - 1
	m.selectedIndex = 0
	m.scrollOffset = 0

	// Start informer for this type with splash screen
	return m.startInformerWithSplash(selectedType)
}
