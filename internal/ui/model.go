package ui

import (
	"fmt"

	"github.com/charmbracelet/bubbles/textinput"
	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/miles-w-3/lobot/internal/filters"
	"github.com/miles-w-3/lobot/internal/k8s"
	"github.com/miles-w-3/lobot/internal/splash"
	"sigs.k8s.io/yaml"
)

// ViewMode represents the current UI mode
type ViewMode int

const (
	ViewModeSplash ViewMode = iota
	ViewModeNormal
	ViewModeFilter
	ViewModeManifest
	ViewModeResourceTypeSelection
)

// Model represents the UI state
type Model struct {
	// Kubernetes data
	client            *k8s.Client
	informer          *k8s.InformerManager
	resourceTypes     []k8s.ResourceType
	currentType       int
	resources         []k8s.Resource
	filteredResources []k8s.Resource

	// UI state
	viewMode      ViewMode
	selectedIndex int
	scrollOffset  int
	width         int
	height        int

	// Filtering
	namespaceFilter *filters.NamespaceFilter
	filterInput     textinput.Model

	// Splash screen
	splash splash.Model

	// Manifest viewer
	manifestViewport viewport.Model
	manifestContent  string

	// Status
	ready bool
	err   error
}

// ResourceUpdateMsg is sent when resources are updated
type ResourceUpdateMsg struct{}

// ReadyMsg is sent when the application is ready
type ReadyMsg struct{}

// NewModel creates a new UI model
func NewModel(client *k8s.Client, informer *k8s.InformerManager) Model {
	filterInput := textinput.New()
	filterInput.Placeholder = "Filter by namespace..."
	filterInput.CharLimit = 100

	return Model{
		client:          client,
		informer:        informer,
		resourceTypes:   k8s.DefaultResourceTypes(),
		currentType:     0,
		selectedIndex:   0,
		scrollOffset:    0,
		viewMode:        ViewModeSplash,
		namespaceFilter: filters.NewNamespaceFilter(),
		filterInput:     filterInput,
		splash:          splash.NewModel(),
		ready:           false,
	}
}

// Init initializes the model
func (m Model) Init() tea.Cmd {
	return m.splash.Init()
}

// UpdateResources updates the displayed resources from the informer
func (m *Model) UpdateResources() {
	if m.informer == nil {
		return
	}

	// Get resources for current type
	currentResourceType := m.resourceTypes[m.currentType]
	m.resources = m.informer.GetResources(currentResourceType.GVR)

	// Apply namespace filter
	m.filteredResources = m.namespaceFilter.FilterResources(m.resources)

	// Adjust selected index if needed
	if m.selectedIndex >= len(m.filteredResources) {
		m.selectedIndex = max(0, len(m.filteredResources)-1)
	}

	// Adjust scroll offset
	m.adjustScrollOffset()
}

// adjustScrollOffset ensures the selected item is visible
func (m *Model) adjustScrollOffset() {
	// Calculate visible area (subtract header and footer lines)
	visibleLines := m.height - 5 // Header, filter bar, status bar

	if visibleLines <= 0 {
		return
	}

	// Scroll down if selected is below visible area
	if m.selectedIndex >= m.scrollOffset+visibleLines {
		m.scrollOffset = m.selectedIndex - visibleLines + 1
	}

	// Scroll up if selected is above visible area
	if m.selectedIndex < m.scrollOffset {
		m.scrollOffset = m.selectedIndex
	}

	// Ensure scroll offset is valid
	if m.scrollOffset < 0 {
		m.scrollOffset = 0
	}
}

// SetReady marks the model as ready
func (m *Model) SetReady() {
	m.ready = true
	// Use pointer to ensure changes persist
	(&m.splash).MarkReady()
	if m.splash.IsDone() {
		m.viewMode = ViewModeNormal
	}
}

// GetSelectedResource returns the currently selected resource
func (m *Model) GetSelectedResource() *k8s.Resource {
	if m.selectedIndex >= 0 && m.selectedIndex < len(m.filteredResources) {
		return &m.filteredResources[m.selectedIndex]
	}
	return nil
}

// EnterManifestMode enters manifest viewing mode for the selected resource
func (m *Model) EnterManifestMode() {
	resource := m.GetSelectedResource()
	if resource == nil || resource.Raw == nil {
		return
	}

	// Format the manifest as YAML
	m.manifestContent = formatManifest(resource.Raw)

	// Create viewport
	m.manifestViewport = viewport.New(m.width-4, m.height-6)
	m.manifestViewport.SetContent(m.manifestContent)

	m.viewMode = ViewModeManifest
}

// ExitManifestMode exits manifest viewing mode
func (m *Model) ExitManifestMode() {
	m.viewMode = ViewModeNormal
}

// SetError sets an error on the model
func (m *Model) SetError(err error) {
	m.err = err
}

// CurrentResourceType returns the currently selected resource type
func (m *Model) CurrentResourceType() k8s.ResourceType {
	if m.currentType >= 0 && m.currentType < len(m.resourceTypes) {
		return m.resourceTypes[m.currentType]
	}
	return k8s.PodResource
}

// NextResourceType moves to the next resource type
func (m *Model) NextResourceType() {
	m.currentType = (m.currentType + 1) % len(m.resourceTypes)
	m.selectedIndex = 0
	m.scrollOffset = 0
	m.UpdateResources()
}

// PrevResourceType moves to the previous resource type
func (m *Model) PrevResourceType() {
	m.currentType--
	if m.currentType < 0 {
		m.currentType = len(m.resourceTypes) - 1
	}
	m.selectedIndex = 0
	m.scrollOffset = 0
	m.UpdateResources()
}

// MoveUp moves the selection up
func (m *Model) MoveUp() {
	if m.selectedIndex > 0 {
		m.selectedIndex--
		m.adjustScrollOffset()
	}
}

// MoveDown moves the selection down
func (m *Model) MoveDown() {
	if m.selectedIndex < len(m.filteredResources)-1 {
		m.selectedIndex++
		m.adjustScrollOffset()
	}
}

// PageUp moves the selection up by one page
func (m *Model) PageUp() {
	visibleLines := m.height - 5
	m.selectedIndex = max(0, m.selectedIndex-visibleLines)
	m.adjustScrollOffset()
}

// PageDown moves the selection down by one page
func (m *Model) PageDown() {
	visibleLines := m.height - 5
	m.selectedIndex = min(len(m.filteredResources)-1, m.selectedIndex+visibleLines)
	m.adjustScrollOffset()
}

// EnterFilterMode enters filter mode
func (m *Model) EnterFilterMode() {
	m.viewMode = ViewModeFilter
	m.filterInput.Focus()
	m.filterInput.SetValue(m.namespaceFilter.GetPattern())
}

// ExitFilterMode exits filter mode
func (m *Model) ExitFilterMode() {
	m.viewMode = ViewModeNormal
	m.filterInput.Blur()
}

// UpdateFilter updates the namespace filter
func (m *Model) UpdateFilter(pattern string) {
	err := m.namespaceFilter.SetPattern(pattern)
	if err == nil {
		m.UpdateResources()
	}
}

// Helper functions
func max(a, b int) int {
	if a > b {
		return a
	}
	return b
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

func formatManifest(obj interface{}) string {
	yamlBytes, err := yaml.Marshal(obj)
	if err != nil {
		return fmt.Sprintf("Error formatting manifest: %v", err)
	}
	return string(yamlBytes)
}
