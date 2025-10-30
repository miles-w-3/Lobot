package ui

import (
	"bytes"
	"context"
	"fmt"
	"log/slog"
	"os"
	"os/exec"
	"strings"
	"time"

	"github.com/alecthomas/chroma/v2/formatters"
	"github.com/alecthomas/chroma/v2/lexers"
	"github.com/alecthomas/chroma/v2/styles"
	"github.com/atotto/clipboard"
	"github.com/charmbracelet/bubbles/help"
	"github.com/charmbracelet/bubbles/table"
	"github.com/charmbracelet/bubbles/textinput"
	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/miles-w-3/lobot/internal/filters"
	"github.com/miles-w-3/lobot/internal/graph"
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
	ViewModeVisualize
)

// Model represents the UI state
type Model struct {
	logger *slog.Logger
	// Kubernetes data
	resourceService   *k8s.ResourceService
	graphBuilder      *graph.Builder
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
	namespaceFilter *filters.NamespaceFilter    // Namespace filter (set via ctrl+n selector)
	nameFilter      *filters.ResourceNameFilter // Resource name filter (set via / search)
	filterInput     textinput.Model

	// Splash screen
	splash splash.Model

	// Table component
	table table.Model

	// Manifest viewer
	manifestViewport viewport.Model
	manifestContent  string
	manifestResource *k8s.Resource // The resource being viewed in manifest mode

	// Status
	ready         bool
	err           error
	statusMessage string

	// Modal
	alertModal *AlertModal

	// Selector (for namespace/context selection)
	selector *SelectorModel

	// Visualizer
	visualizer *VisualizerModel

	// Key bindings
	globalKeys     GlobalKeyMap
	normalKeys     NormalModeKeyMap
	manifestKeys   ManifestModeKeyMap
	visualizerKeys VisualizerModeKeyMap
	filterKeys     FilterModeKeyMap

	// Help
	help     help.Model
	showHelp bool
}

// ResourceUpdateMsg is sent when resources are updated
type ResourceUpdateMsg struct{}

// ReadyMsg is sent when the application is ready
type ReadyMsg struct{}

// ErrorMsg is sent when an error occurs
type ErrorMsg struct {
	Error error
}

// EditorFinishedMsg is sent when external editor finishes
type EditorFinishedMsg struct {
	Err        error
	Cancelled  bool
	BackupPath string
}

// BuildGraphMsg is sent to trigger graph building
type BuildGraphMsg struct {
	Resource *k8s.Resource
}

// ContextSwitchErrorMsg is sent when context switch fails
type ContextSwitchErrorMsg struct {
	Error error
}

// NewModel creates a new UI model
func NewModel(resourceService *k8s.ResourceService, logger *slog.Logger) Model {
	filterInput := textinput.New()
	filterInput.Placeholder = "Search resource name..."
	filterInput.CharLimit = 100

	// Initialize table with empty columns (will be set later)
	columns := []table.Column{
		{Title: "NAME", Width: 40},
		{Title: "NAMESPACE", Width: 20},
		{Title: "STATUS", Width: 15},
		{Title: "AGE", Width: 10},
	}

	t := table.New(
		table.WithColumns(columns),
		table.WithFocused(true),
		table.WithHeight(10),
	)

	// Set table styles
	s := table.DefaultStyles()
	s.Header = s.Header.
		BorderStyle(lipgloss.NormalBorder()).
		BorderForeground(ColorPrimary).
		BorderBottom(true).
		Bold(true)
	s.Selected = s.Selected.
		Foreground(lipgloss.Color("255")).
		Background(ColorSecondary).
		Bold(true)
	t.SetStyles(s)

	// Create graph builder with the resource service as the provider
	graphBuilder := graph.NewBuilder(resourceService, nil)

	return Model{
		logger:          logger,
		resourceService: resourceService,
		graphBuilder:    graphBuilder,
		resourceTypes:   k8s.DefaultResourceTypes(),
		currentType:     0,
		selectedIndex:   0,
		scrollOffset:    0,
		viewMode:        ViewModeSplash,
		namespaceFilter: filters.NewNamespaceFilter(),
		nameFilter:      filters.NewResourceNameFilter(),
		filterInput:     filterInput,
		splash:          splash.NewModel(logger),
		table:           t,
		ready:           false,
		alertModal:      NewAlertModal(),
		globalKeys:      DefaultGlobalKeyMap(),
		normalKeys:      DefaultNormalModeKeyMap(),
		manifestKeys:    DefaultManifestModeKeyMap(),
		visualizerKeys:  DefaultVisualizerModeKeyMap(),
		filterKeys:      DefaultFilterModeKeyMap(),
		help:            configureHelp(),
		showHelp:        false,
	}
}

// configureHelp creates and configures the help model with brand colors
func configureHelp() help.Model {
	h := help.New()
	h.Styles.ShortKey = lipgloss.NewStyle().Foreground(ColorAccent)
	h.Styles.ShortDesc = lipgloss.NewStyle().Foreground(ColorMuted)
	h.Styles.FullKey = lipgloss.NewStyle().Foreground(ColorAccent).Bold(true)
	h.Styles.FullDesc = lipgloss.NewStyle().Foreground(ColorMuted)
	h.Styles.FullSeparator = lipgloss.NewStyle().Foreground(ColorBorder)
	return h
}

// Init initializes the model
func (m Model) Init() tea.Cmd {
	return m.splash.Init()
}

// UpdateResources updates the displayed resources from the informer
func (m *Model) UpdateResources() {
	// Get resources for current type
	currentResourceType := m.resourceTypes[m.currentType]
	m.resources = m.resourceService.GetResources(currentResourceType.GVR)

	// Apply namespace filter first
	m.filteredResources = m.namespaceFilter.FilterResources(m.resources)

	// Then apply name filter
	m.filteredResources = m.nameFilter.FilterResources(m.filteredResources)

	// Clear rows first to avoid column mismatch during rendering
	m.table.SetRows([]table.Row{})

	// Update table columns based on resource type
	var columns []table.Column
	if currentResourceType.DisplayName == "Helm Releases" {
		// Special columns for Helm releases: NAME, NAMESPACE, STATUS, VERSION
		columns = []table.Column{
			{Title: "NAME", Width: 30},
			{Title: "NAMESPACE", Width: 20},
			{Title: "STATUS", Width: 15},
			{Title: "VERSION", Width: 30},
		}
	} else if currentResourceType.Namespaced {
		columns = []table.Column{
			{Title: "NAME", Width: 40},
			{Title: "NAMESPACE", Width: 20},
			{Title: "STATUS", Width: 15},
			{Title: "AGE", Width: 10},
		}
	} else {
		columns = []table.Column{
			{Title: "NAME", Width: 60},
			{Title: "STATUS", Width: 15},
			{Title: "AGE", Width: 10},
		}
	}
	m.table.SetColumns(columns)

	// Update table rows
	rows := make([]table.Row, 0, len(m.filteredResources))
	for _, resource := range m.filteredResources {
		age := formatAge(resource.Age)
		if currentResourceType.DisplayName == "Helm Releases" {
			// Special row format for Helm releases
			rows = append(rows, table.Row{
				truncate(resource.Name, 30),
				truncate(resource.Namespace, 20),
				resource.Status,
				truncate(resource.HelmChart, 30),
			})
		} else if currentResourceType.Namespaced {
			rows = append(rows, table.Row{
				truncate(resource.Name, 40),
				truncate(resource.Namespace, 20),
				resource.Status,
				age,
			})
		} else {
			rows = append(rows, table.Row{
				truncate(resource.Name, 60),
				resource.Status,
				age,
			})
		}
	}
	m.table.SetRows(rows)

	// Adjust selected index if needed
	if m.selectedIndex >= len(m.filteredResources) {
		m.selectedIndex = max(0, len(m.filteredResources)-1)
	}

	// Set cursor on table to match selected index
	if m.selectedIndex >= 0 && m.selectedIndex < len(m.filteredResources) {
		m.table.SetCursor(m.selectedIndex)
	}
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
	m.splash.MarkReady()

	m.viewMode = ViewModeNormal
	m.UpdateResources()
}

// GetSelectedResource returns the currently selected resource
func (m *Model) GetSelectedResource() *k8s.Resource {
	if m.selectedIndex >= 0 && m.selectedIndex < len(m.filteredResources) {
		return &m.filteredResources[m.selectedIndex]
	}
	return nil
}

// EnterManifestMode enters manifest viewing mode for the selected resource
func (m *Model) EnterManifestMode() tea.Cmd {
	resource := m.GetSelectedResource()
	if resource == nil || resource.Raw == nil {
		return nil
	}

	// Store the resource reference to prevent issues when the resource list is reordered
	// This is safe because informer creates new Resource structs on update rather than mutating
	m.manifestResource = resource

	// Format the manifest as YAML
	m.manifestContent = formatManifest(resource.Raw)

	// Create viewport
	m.manifestViewport = viewport.New(m.width-4, m.height-6)
	m.manifestViewport.SetContent(m.manifestContent)

	m.viewMode = ViewModeManifest

	// Disable mouse to allow native terminal text selection
	return tea.DisableMouse
}

// ExitManifestMode exits manifest viewing mode
func (m *Model) ExitManifestMode() tea.Cmd {
	m.viewMode = ViewModeNormal
	m.manifestResource = nil

	// Re-enable mouse when exiting manifest mode
	return tea.EnableMouseCellMotion
}

// RefreshManifestResource refreshes the manifest view with the latest version of the resource
// This should be called after a successful edit to show the updated resource
func (m *Model) RefreshManifestResource() {
	if m.viewMode != ViewModeManifest || m.manifestResource == nil {
		return
	}

	// Find the updated resource in the current resource list by matching name, namespace, and kind
	var updatedResource *k8s.Resource
	for i := range m.resources {
		res := &m.resources[i]
		if res.Name == m.manifestResource.Name &&
			res.Namespace == m.manifestResource.Namespace &&
			res.Kind == m.manifestResource.Kind {
			updatedResource = res
			break
		}
	}

	if updatedResource == nil {
		// Resource might have been deleted or not yet updated in cache
		m.statusMessage = "⚠ Could not find updated resource in cache"
		return
	}

	// Update the stored reference
	m.manifestResource = updatedResource

	// Reformat the manifest with the new data
	m.manifestContent = formatManifest(updatedResource.Raw)
	m.manifestViewport.SetContent(m.manifestContent)
}

// CopyManifestToClipboard copies the raw manifest YAML to clipboard
func (m *Model) CopyManifestToClipboard() (tea.Model, tea.Cmd) {
	resource := m.GetSelectedResource()
	if resource == nil || resource.Raw == nil {
		m.statusMessage = "No manifest to copy"
		return *m, nil
	}

	// Marshal to YAML without formatting
	yamlBytes, err := yaml.Marshal(resource.Raw)
	if err != nil {
		m.statusMessage = fmt.Sprintf("Failed to marshal YAML: %v", err)
		return *m, nil
	}

	// Copy to clipboard
	err = clipboard.WriteAll(string(yamlBytes))
	if err != nil {
		m.statusMessage = fmt.Sprintf("Failed to copy to clipboard: %v", err)
		return *m, nil
	}

	m.statusMessage = "✓ Manifest copied to clipboard"
	return *m, nil
}

// EnterVisualizeMode enters visualization mode for the selected resource
func (m *Model) EnterVisualizeMode() {
	resource := m.GetSelectedResource()
	if resource == nil {
		m.statusMessage = "No resource selected"
		return
	}

	m.statusMessage = "Building resource graph..."
	// The graph building will be done in the update handler
}

// ExitVisualizeMode exits visualization mode
func (m *Model) ExitVisualizeMode() {
	m.viewMode = ViewModeNormal
	m.visualizer = nil
}

// EditSelectedResource opens the selected resource in an external editor
// This properly suspends the BubbleTea program while the editor runs
func (m *Model) EditSelectedResource() tea.Cmd {
	// If in manifest mode, use the stored resource; otherwise get current selection
	var resource *k8s.Resource
	if m.viewMode == ViewModeManifest {
		resource = m.manifestResource
	} else {
		resource = m.GetSelectedResource()
	}

	if resource == nil {
		m.statusMessage = "No resource selected"
		return nil
	}

	// Prepare the edit file BEFORE suspending the TUI
	editResult, err := m.resourceService.PrepareEditFile(resource)
	if err != nil {
		return func() tea.Msg {
			return EditorFinishedMsg{Err: fmt.Errorf("failed to prepare edit: %w", err)}
		}
	}

	// Get editor from environment or default to vim
	editor := os.Getenv("EDITOR")
	if editor == "" {
		editor = "vim"
	}

	// Create a copy of the resource for the callback
	resourceCopy := *resource

	// Use tea.ExecProcess to properly suspend the TUI and run the editor
	return tea.ExecProcess(exec.Command(editor, editResult.TmpFilePath), func(err error) tea.Msg {
		// Cleanup: remove temporary file when done
		defer os.Remove(editResult.TmpFilePath)

		// Check if the editor exited with an error
		if err != nil {
			return EditorFinishedMsg{
				Err: fmt.Errorf("editor exited with error: %w", err),
			}
		}

		// Process the edited file (validates and applies changes)
		processErr := m.resourceService.ProcessEditedFile(context.Background(), &resourceCopy, editResult)

		// Check if the user just cancelled (no changes made)
		if processErr == nil {
			// Could be either successful update or cancellation (no changes)
			// ProcessEditedFile returns nil for both cases
			// The logging in ProcessEditedFile will distinguish these
			return EditorFinishedMsg{Err: nil}
		}

		// An error occurred during processing
		// Check if a backup file was created (will be in /tmp)
		return EditorFinishedMsg{
			Err: processErr,
		}
	})
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
	if len(m.resourceTypes) == 0 {
		return // No resource types available
	}
	m.currentType = (m.currentType + 1) % len(m.resourceTypes)
	m.selectedIndex = 0
	m.scrollOffset = 0
	m.UpdateResources()
}

// PrevResourceType moves to the previous resource type
func (m *Model) PrevResourceType() {
	if len(m.resourceTypes) == 0 {
		return // No resource types available
	}
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
	m.table.MoveUp(1)
	m.selectedIndex = m.table.Cursor()
}

// MoveDown moves the selection down
func (m *Model) MoveDown() {
	m.table.MoveDown(1)
	m.selectedIndex = m.table.Cursor()
}

// PageUp moves the selection up by one page
func (m *Model) PageUp() {
	// Move up by visible height
	for i := 0; i < 10 && m.table.Cursor() > 0; i++ {
		m.table.MoveUp(1)
	}
	m.selectedIndex = m.table.Cursor()
}

// PageDown moves the selection down by one page
func (m *Model) PageDown() {
	// Move down by visible height
	for i := 0; i < 10 && m.table.Cursor() < len(m.filteredResources)-1; i++ {
		m.table.MoveDown(1)
	}
	m.selectedIndex = m.table.Cursor()
}

// EnterFilterMode enters filter mode
func (m *Model) EnterFilterMode() {
	m.viewMode = ViewModeFilter
	m.filterInput.Focus()
	m.filterInput.SetValue(m.nameFilter.GetPattern())
}

// ExitFilterMode exits filter mode
func (m *Model) ExitFilterMode() {
	m.viewMode = ViewModeNormal
	m.filterInput.Blur()
}

// SwitchContext switches to a new Kubernetes context
func (m *Model) SwitchContext(contextName string) tea.Cmd {
	// Show splash screen
	m.viewMode = ViewModeSplash
	m.splash = splash.NewModel(m.logger)
	m.splash.SetSize(m.width, m.height)
	m.ready = false

	return tea.Batch(
		m.splash.Init(),
		func() tea.Msg {
			if err := m.resourceService.SwitchContext(contextName); err != nil {
				return ContextSwitchErrorMsg{Error: err}
			}

			// Clear UI state
			m.resources = []k8s.Resource{}
			m.filteredResources = []k8s.Resource{}
			m.selectedIndex = 0
			m.scrollOffset = 0

			// TODO: Is this the right message to send?
			return ResourceUpdateMsg{}
		},
	)
}

// UpdateFilter updates the resource name filter
func (m *Model) UpdateFilter(pattern string) {
	err := m.nameFilter.SetPattern(pattern)
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

func formatAge(duration time.Duration) string {
	if duration < time.Minute {
		return fmt.Sprintf("%ds", int(duration.Seconds()))
	} else if duration < time.Hour {
		return fmt.Sprintf("%dm", int(duration.Minutes()))
	} else if duration < 24*time.Hour {
		return fmt.Sprintf("%dh", int(duration.Hours()))
	} else {
		return fmt.Sprintf("%dd", int(duration.Hours()/24))
	}
}

func truncate(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen-3] + "..."
}

// GetCurrentModeHelp returns help bindings for the current mode
func (m *Model) GetCurrentModeHelp() help.KeyMap {
	switch m.viewMode {
	case ViewModeNormal:
		return m.normalKeys
	case ViewModeManifest:
		return m.manifestKeys
	case ViewModeVisualize:
		return m.visualizerKeys
	case ViewModeFilter:
		return m.filterKeys
	default:
		return m.normalKeys
	}
}

func formatManifest(obj interface{}) string {
	yamlBytes, err := yaml.Marshal(obj)
	if err != nil {
		return fmt.Sprintf("Error formatting manifest: %v", err)
	}

	yamlContent := string(yamlBytes)

	// Apply syntax highlighting with Chroma
	var highlightedBuf bytes.Buffer

	lexer := lexers.Get("yaml")
	if lexer == nil {
		lexer = lexers.Fallback
	}

	// Use terminal256 formatter for 256-color terminals
	formatter := formatters.Get("terminal256")
	if formatter == nil {
		formatter = formatters.Fallback
	}

	// Use monokai style (good contrast for dark terminals)
	style := styles.Get("monokai")
	if style == nil {
		style = styles.Fallback
	}

	iterator, err := lexer.Tokenise(nil, yamlContent)
	if err != nil {
		// Fall back to non-highlighted if tokenization fails
		yamlContent = string(yamlBytes)
	} else {
		err = formatter.Format(&highlightedBuf, style, iterator)
		if err == nil {
			yamlContent = highlightedBuf.String()
		}
	}

	// Add line numbers
	lines := strings.Split(yamlContent, "\n")
	var numbered strings.Builder
	maxLineNum := len(lines)
	lineNumWidth := len(fmt.Sprintf("%d", maxLineNum))

	// Style for line numbers (muted gray)
	lineNumStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("240"))

	for i, line := range lines {
		lineNum := i + 1
		styledLineNum := lineNumStyle.Render(fmt.Sprintf("%*d", lineNumWidth, lineNum))
		numbered.WriteString(fmt.Sprintf("%s │ %s\n", styledLineNum, line))
	}

	return numbered.String()
}
