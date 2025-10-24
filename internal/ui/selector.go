package ui

import (
	"context"
	"sort"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/erikgeiser/promptkit/selection"
	"github.com/miles-w-3/lobot/internal/k8s"
	"github.com/miles-w-3/lobot/internal/splash"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
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
	if m.client == nil || m.client.Clientset == nil {
		return []string{}
	}

	// Query all namespaces from the cluster
	namespaceList, err := m.client.Clientset.CoreV1().Namespaces().List(context.Background(), metav1.ListOptions{})
	if err != nil {
		// If we can't fetch namespaces, fall back to extracting from resources
		m.client.Logger.Error("Failed to list namespaces", "error", err)
		return m.getNamespacesFromResources()
	}

	namespaces := make([]string, 0, len(namespaceList.Items))
	for _, ns := range namespaceList.Items {
		namespaces = append(namespaces, ns.Name)
	}

	sort.Strings(namespaces)
	return namespaces
}

// getNamespacesFromResources is a fallback that extracts namespaces from current resources
func (m *Model) getNamespacesFromResources() []string {
	namespaceSet := make(map[string]bool)

	for _, resource := range m.resources {
		if resource.Namespace != "" {
			namespaceSet[resource.Namespace] = true
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
	contexts, _, err := k8s.GetAvailableContexts()
	if err != nil {
		// Fallback to current context only if we can't read kubeconfig
		m.client.Logger.Error("Failed to get available contexts", "error", err)
		if m.client != nil && m.client.Context != "" {
			return []string{m.client.Context}
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
	current := ""
	if m.client != nil {
		current = m.client.Context
	}
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
	// Discover all resources if not already cached
	if len(m.discoveredTypes) == 0 && m.resourceDiscovery != nil {
		discovered, err := m.resourceDiscovery.DiscoverAllResources()
		if err != nil {
			m.client.Logger.Error("Failed to discover resources", "error", err)
			// Fallback to current types
			displayNames := make([]string, len(m.resourceTypes))
			for i, rt := range m.resourceTypes {
				displayNames[i] = rt.DisplayName
			}
			return displayNames
		}
		m.discoveredTypes = discovered
	}

	// Return discovered types (alphabetically sorted)
	displayNames := make([]string, len(m.discoveredTypes))
	for i, rt := range m.discoveredTypes {
		displayNames[i] = rt.DisplayName
	}
	return displayNames
}

// ApplyResourceTypeSelection applies the selected resource type
func (m *Model) ApplyResourceTypeSelection(displayName string) tea.Cmd {
	// Find the resource type by display name
	var selectedType *k8s.ResourceType
	for i := range m.discoveredTypes {
		if m.discoveredTypes[i].DisplayName == displayName {
			selectedType = &m.discoveredTypes[i]
			break
		}
	}

	if selectedType == nil {
		m.statusMessage = "Resource type not found"
		return nil
	}

	// Check if this type is already in our rotation
	typeIndex := -1
	for i := range m.resourceTypes {
		if m.resourceTypes[i].GVR == selectedType.GVR {
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
	m.resourceTypes = append(m.resourceTypes, *selectedType)
	m.currentType = len(m.resourceTypes) - 1
	m.selectedIndex = 0
	m.scrollOffset = 0

	// Start informer for this type with splash screen
	return m.startInformerWithSplash(*selectedType)
}

// startInformerWithSplash starts an informer and shows splash screen
func (m *Model) startInformerWithSplash(resourceType k8s.ResourceType) tea.Cmd {
	// Show splash screen
	m.viewMode = ViewModeSplash
	m.splash = splash.NewModel()
	m.splash.SetSize(m.width, m.height)
	m.ready = false

	// Return both the splash init command and the informer start command
	return tea.Batch(
		m.splash.Init(),
		func() tea.Msg {
			// Start the informer in background
			ctx := context.Background()
			err := m.informer.StartInformer(ctx, resourceType)
			if err != nil {
				m.client.Logger.Error("Failed to start informer", "type", resourceType.DisplayName, "error", err)
			}

			// Send ready message
			return ReadyMsg{}
		},
	)
}
