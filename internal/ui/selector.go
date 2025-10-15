package ui

import (
	"context"
	"sort"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/erikgeiser/promptkit/selection"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// SelectorType represents the type of selector
type SelectorType int

const (
	SelectorTypeNamespace SelectorType = iota
	SelectorTypeContext
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
	sel.Filter = nil // Disable filtering since we have resource name search on /

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
	sel.Filter = nil

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
	// TODO: Read from kubeconfig to get all available contexts
	// For now, return current context
	if m.client != nil && m.client.Context != "" {
		return []string{m.client.Context}
	}
	return []string{"default"}
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
