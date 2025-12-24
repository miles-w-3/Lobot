package ui

import (
	"context"
	"sort"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/erikgeiser/promptkit/selection"
	"github.com/miles-w-3/lobot/internal/k8s"
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
	selection    *selection.Model[string]
	selectorType SelectorType
	visible      bool
}

// SelectorFinishedMsg is sent when a selector finishes
type SelectorFinishedMsg struct {
	SelectedValue string
	SelectorType  SelectorType
	Cancelled     bool
}

// NewNamespaceSelector creates a new namespace selector
func NewNamespaceSelector(namespaces []string, current string) *SelectorModel {
	// Add <all> option at the top
	choices := append([]string{"<all>"}, namespaces...)

	// Create the selection
	sel := selection.New("Select Namespace:", choices)
	sel.Filter = selection.FilterContainsCaseInsensitive // Enable searchable filtering
	sel.LoopCursor = true

	// Create the selection model
	model := selection.NewModel(sel)

	return &SelectorModel{
		selection:    model,
		selectorType: SelectorTypeNamespace,
		visible:      true,
	}
}

// NewContextSelector creates a new context selector
func NewContextSelector(contexts []string, current string) *SelectorModel {
	sel := selection.New("Select Cluster Context:", contexts)
	sel.Filter = selection.FilterContainsCaseInsensitive // Enable searchable filtering
	sel.LoopCursor = true

	// Create the selection model
	model := selection.NewModel(sel)

	return &SelectorModel{
		selection:    model,
		selectorType: SelectorTypeContext,
		visible:      true,
	}
}

// Init initializes the selector
func (s *SelectorModel) Init() tea.Cmd {
	return s.selection.Init()
}

// Update handles messages
func (s *SelectorModel) Update(msg tea.Msg) (*SelectorModel, tea.Cmd) {
	if !s.visible {
		return s, nil
	}

	// Check for esc to cancel
	if keyMsg, ok := msg.(tea.KeyMsg); ok {
		if keyMsg.String() == "esc" {
			s.visible = false
			return s, func() tea.Msg {
				return SelectorFinishedMsg{
					SelectorType: s.selectorType,
					Cancelled:    true,
				}
			}
		}

		// Check for enter to confirm selection
		if keyMsg.String() == "enter" {
			choice, err := s.selection.Value()
			s.visible = false
			if err != nil {
				return s, func() tea.Msg {
					return SelectorFinishedMsg{
						SelectorType: s.selectorType,
						Cancelled:    true,
					}
				}
			}
			return s, func() tea.Msg {
				return SelectorFinishedMsg{
					SelectedValue: choice,
					SelectorType:  s.selectorType,
					Cancelled:     false,
				}
			}
		}
	}

	// Pass all other messages to underlying selection model
	_, cmd := s.selection.Update(msg)
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
	namespaces := m.getAllNamespaces()
	current := m.namespaceFilter.GetPattern()
	m.selector = NewNamespaceSelector(namespaces, current)
	return m.selector.Init()
}

// OpenContextSelector opens the context selector
func (m *Model) OpenContextSelector() tea.Cmd {
	contexts := m.getAvailableContexts()
	current := m.resourceService.GetCurrentContext()
	m.selector = NewContextSelector(contexts, current)
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
func NewResourceTypeSelector(resourceTypes []string) *SelectorModel {
	sel := selection.New("Select Resource Type:", resourceTypes)
	sel.Filter = selection.FilterContainsCaseInsensitive // Enable searchable filtering
	sel.LoopCursor = true

	// Create the selection model
	model := selection.NewModel(sel)

	return &SelectorModel{
		selection:    model,
		selectorType: SelectorTypeResourceType,
		visible:      true,
	}
}

// OpenResourceTypeSelector opens the resource type selector
func (m *Model) OpenResourceTypeSelector() tea.Cmd {
	resourceTypes := m.getAllResourceTypes()
	m.selector = NewResourceTypeSelector(resourceTypes)
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


