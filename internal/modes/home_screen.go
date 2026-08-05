package modes

import (
	"context"
	"fmt"
	"log/slog"
	"sort"
	"strings"
	"time"

	"charm.land/bubbles/v2/table"
	"charm.land/bubbles/v2/textinput"
	"charm.land/bubbles/v2/viewport"
	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/x/ansi"
	"github.com/miles-w-3/lobot/internal/command"
	"github.com/miles-w-3/lobot/internal/filters"
	"github.com/miles-w-3/lobot/internal/k8s"
	"github.com/miles-w-3/lobot/internal/keys"
	"github.com/miles-w-3/lobot/internal/util"
)

const (
	favoriteTypesBoxWidth     = 16
	favoriteTypesContentWidth = favoriteTypesBoxWidth - 4 // border + padding
)

type homeSelectorKind uint8

const (
	homeSelectorNamespace homeSelectorKind = iota
	homeSelectorContext
	homeSelectorResourceType
)

// HomeScreen is the resource-list screen shown after startup. It owns the
// resource table, its filters, and the commands that operate on that list.
type HomeScreen struct {
	log             *slog.Logger
	resourceService *k8s.ResourceService

	trackedTypes      []*k8s.TrackedType
	currentType       int
	resources         []k8s.TrackedObject
	filteredResources []k8s.TrackedObject

	table          table.Model
	registry       *command.Registry[keys.HomeCmd]
	filterRegistry *command.Registry[keys.FilterCmd]
	filterInput    textinput.Model
	filtering      bool

	favoriteTypesViewport viewport.Model
	showingFavoriteTypes  bool

	namespaceFilter *filters.NamespaceFilter
	nameFilter      *filters.ResourceNameFilter

	selector              *SelectorModel
	selectorKind          homeSelectorKind
	selectorLoading       bool
	selectorRequestID     uint64
	selectorResourceTypes map[string]*k8s.TrackedType

	selectedIndex int
	width         int
	height        int
	filterError   error
}

var _ Screen = (*HomeScreen)(nil)

// NewHomeScreen constructs the resource-list screen.
func NewHomeScreen(resourceService *k8s.ResourceService, log *slog.Logger) *HomeScreen {
	if log == nil {
		log = slog.Default()
	}

	filterInput := textinput.New()
	filterInput.Placeholder = "Search resource name..."
	filterInput.CharLimit = 100
	filterInput.Blur()

	// TODO: This is WIP for favorite types
	favoriteTypesViewport := viewport.New(
		viewport.WithWidth(favoriteTypesContentWidth),
		viewport.WithHeight(1),
	)
	favoriteTypesViewport.SetContent("Test\nTest2\nTest3\n")

	s := &HomeScreen{
		log:                   log.With("component", "home"),
		resourceService:       resourceService,
		trackedTypes:          k8s.DefaultResourceTypes(),
		table:                 newHomeTable(),
		registry:              keys.NewHomeRegistry(),
		filterRegistry:        keys.NewFilterRegistry(),
		filterInput:           filterInput,
		favoriteTypesViewport: favoriteTypesViewport,
		namespaceFilter:       filters.NewNamespaceFilter(),
		nameFilter:            filters.NewResourceNameFilter(),
	}
	s.updateResources()
	return s
}

func newHomeTable() table.Model {
	t := table.New(table.WithFocused(true))
	styles := table.DefaultStyles()
	styles.Header = styles.Header.
		BorderStyle(lipgloss.NormalBorder()).
		BorderForeground(util.ColorPrimary).
		BorderBottom(true).
		Bold(true)
	styles.Selected = styles.Selected.
		Foreground(util.ColorText).
		Background(util.ColorSecondary).
		Bold(true)
	t.SetStyles(styles)
	return t
}

// View renders only the home content. The root owns the shared help bar and
// terminal view settings.
func (s *HomeScreen) View() string {
	content := s.render()
	if s.width <= 0 || s.height <= 0 {
		return content
	}
	if s.selector != nil {
		return overlayCenter(content, s.selector.View(), s.width, s.height)
	}
	if s.selectorLoading {
		loading := lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(util.ColorAccent).
			Padding(1, 2).
			Render("Loading choices…")
		return overlayCenter(content, loading, s.width, s.height)
	}
	return content
}

// Update handles home messages. Key presses are dispatched through the home
// registry; the root has already consumed global key bindings before this
// method is called.
func (s *HomeScreen) Update(msg tea.Msg) tea.Cmd {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		s.updateSize(msg)
		if s.selector != nil {
			_, cmd := s.selector.Update(msg)
			return cmd
		}
		return nil
	case ResourceUpdateMsg:
		s.updateResources()
		return nil
	case HomeActivatedMsg:
		s.selector = nil
		s.selectorKind = homeSelectorNamespace
		s.selectorLoading = false
		s.selectorResourceTypes = nil
		s.updateResources()
		return nil
	case NamespaceOptionsReadyMsg:
		return s.openNamespaceSelector(msg)
	case ContextOptionsReadyMsg:
		return s.openContextSelector(msg)
	case ResourceTypeOptionsReadyMsg:
		return s.openResourceTypeSelector(msg)
	case ContextSwitchStartedMsg:
		s.resetForContext()
		return nil
	case tea.KeyPressMsg:
		return s.updateKey(msg)
	case tea.MouseWheelMsg:
		return s.updateMouseWheel(msg)
	case tea.MouseClickMsg:
		return s.updateMouseClick(msg)
	}

	if s.selector != nil {
		_, cmd := s.selector.Update(msg)
		return cmd
	}
	if s.selectorLoading {
		return nil
	}
	if s.filtering {
		var cmd tea.Cmd
		s.filterInput, cmd = s.filterInput.Update(msg)
		return cmd
	}

	return nil
}

func (s *HomeScreen) updateKey(msg tea.KeyPressMsg) tea.Cmd {
	if s.selector != nil {
		result, cmd := s.selector.Update(msg)
		if result == nil {
			return cmd
		}
		return tea.Batch(cmd, s.finishSelector(*result))
	}
	if s.selectorLoading {
		return nil
	}
	if s.filtering {
		if cmd, err := s.filterRegistry.Dispatch(msg); err == nil {
			return s.dispatchFilterCommand(cmd)
		}

		var cmd tea.Cmd
		s.filterInput, cmd = s.filterInput.Update(msg)
		return cmd
	}

	if cmd, err := s.registry.Dispatch(msg); err == nil {
		return s.dispatchCommand(cmd)
	}

	// Unbound key presses are intentionally ignored. The table's default key
	// map is not used here because application shortcuts must come from the
	// registry rather than from a component's implicit bindings.
	return nil
}

func (s *HomeScreen) updateMouseWheel(msg tea.MouseWheelMsg) tea.Cmd {
	if s.selector != nil || s.selectorLoading {
		return nil
	}
	switch msg.Button {
	case tea.MouseWheelUp:
		s.moveSelection(-1)
	case tea.MouseWheelDown:
		s.moveSelection(1)
	}
	return nil
}

func (s *HomeScreen) updateMouseClick(msg tea.MouseClickMsg) tea.Cmd {
	if s.selector != nil || s.selectorLoading || msg.Button != tea.MouseLeft || len(s.filteredResources) == 0 {
		return nil
	}

	// The status line and table border occupy the first few rows. The filter
	// bar adds two more rows while it is visible.
	row := msg.Y - 3
	if s.filtering {
		row -= 2
	}
	if row < 0 {
		return nil
	}

	index := row
	if index >= 0 && index < len(s.filteredResources) {
		s.setSelection(index)
	}
	return nil
}

func (s *HomeScreen) dispatchCommand(cmd keys.HomeCmd) tea.Cmd {
	switch cmd {
	case keys.HomeCmdMoveUp:
		s.moveSelection(-1)
	case keys.HomeCmdMoveDown:
		s.moveSelection(1)
	case keys.HomeCmdPageUp:
		s.moveSelection(-s.pageSize())
	case keys.HomeCmdPageDown:
		s.moveSelection(s.pageSize())
	case keys.HomeCmdHome:
		s.setSelection(0)
	case keys.HomeCmdEnd:
		s.setSelection(len(s.filteredResources) - 1)
	case keys.HomeCmdNextType:
		s.nextResourceType(1)
	case keys.HomeCmdPrevType:
		s.nextResourceType(-1)
	case keys.HomeCmdFilter:
		s.enterFilter()
	case keys.HomeCmdOpenManifest:
		return s.openManifest()
	case keys.HomeCmdEdit:
		return s.editSelectedResource()
	case keys.HomeCmdRefresh:
		return s.refreshCurrentType()
	case keys.HomeCmdVisualize:
		return s.visualizeSelected()
	case keys.HomeCmdOpenUtilization:
		return func() tea.Msg { return UtilizationRequestedMsg{} }
	case keys.HomeCmdOpenNamespaceSelector:
		return s.requestNamespaceSelector()
	case keys.HomeCmdOpenResourceTypeSelector:
		return s.requestResourceTypeSelector()
	case keys.HomeCmdOpenContextSelector:
		return s.requestContextSelector()
	case keys.HomeCmdToggleFavorites:
		s.showingFavoriteTypes = !s.showingFavoriteTypes
		s.layoutComponents()
	}

	return nil
}

func (s *HomeScreen) dispatchFilterCommand(cmd keys.FilterCmd) tea.Cmd {
	switch cmd {
	case keys.FilterCmdAccept:
		if s.applyNameFilter(s.filterInput.Value()) {
			s.exitFilter()
		}
	case keys.FilterCmdCancel:
		s.exitFilter()
	case keys.FilterCmdClear:
		s.filterInput.SetValue("")
	}
	return nil
}

func (s *HomeScreen) enterFilter() {
	s.filtering = true
	s.filterInput.SetValue(s.nameFilter.GetPattern())
	s.filterInput.Focus()
	s.layoutComponents()
}

func (s *HomeScreen) exitFilter() {
	s.filtering = false
	s.filterInput.Blur()
	s.layoutComponents()
}

func (s *HomeScreen) applyNameFilter(pattern string) bool {
	previous := s.nameFilter.GetPattern()
	if err := s.nameFilter.SetPattern(pattern); err != nil {
		// ResourceNameFilter records the pattern before compiling a regular
		// expression. Restore the last valid pattern so future cache updates
		// cannot observe a nil compiled expression.
		_ = s.nameFilter.SetPattern(previous)
		s.filterError = err
		return false
	}
	s.filterError = nil
	s.updateResources()
	return true
}

func (s *HomeScreen) selectedResource() k8s.TrackedObject {
	if s.selectedIndex < 0 || s.selectedIndex >= len(s.filteredResources) {
		return nil
	}
	return s.filteredResources[s.selectedIndex]
}

func (s *HomeScreen) openManifest() tea.Cmd {
	resource := s.selectedResource()
	if resource == nil || resource.GetRaw() == nil {
		return nil
	}
	return func() tea.Msg {
		return ManifestRequestedMsg{Resource: resource}
	}
}

func (s *HomeScreen) editSelectedResource() tea.Cmd {
	resource := s.selectedResource()
	if resource == nil {
		return nil
	}
	return func() tea.Msg {
		return ManifestEditRequestedMsg{Resource: resource}
	}
}

func (s *HomeScreen) visualizeSelected() tea.Cmd {
	resource := s.selectedResource()
	if resource == nil {
		return nil
	}
	return func() tea.Msg {
		return VisualizerRequestedMsg{Resource: resource}
	}
}

func (s *HomeScreen) requestNamespaceSelector() tea.Cmd {
	if s.selector != nil || s.selectorLoading {
		return nil
	}

	s.selectorLoading = true
	s.selectorRequestID++
	requestID := s.selectorRequestID
	fallback := s.namespaceOptionsFromResources()
	current := s.namespaceFilter.GetPattern()
	service := s.resourceService
	return func() tea.Msg {
		options := fallback
		if service != nil {
			if namespaces, err := service.GetAllNamespaces(context.Background()); err == nil && len(namespaces) > 0 {
				options = namespaces
			}
		}
		return NamespaceOptionsReadyMsg{RequestID: requestID, Options: options, Current: current}
	}
}

func (s *HomeScreen) requestContextSelector() tea.Cmd {
	if s.selector != nil || s.selectorLoading {
		return nil
	}

	s.selectorLoading = true
	s.selectorRequestID++
	requestID := s.selectorRequestID
	service := s.resourceService
	fallback := []string{"default"}
	current := "default"
	if service != nil && service.GetClient() != nil {
		if name := service.GetCurrentContext(); name != "" {
			current = name
			fallback = []string{name}
		}
	}
	return func() tea.Msg {
		options := fallback
		if service != nil {
			if contexts, selected, err := service.GetAvailableContexts(); err == nil && len(contexts) > 0 {
				options = contexts
				if selected != "" {
					current = selected
				}
			}
		}
		return ContextOptionsReadyMsg{RequestID: requestID, Options: options, Current: current}
	}
}

func (s *HomeScreen) requestResourceTypeSelector() tea.Cmd {
	if s.selector != nil || s.selectorLoading {
		return nil
	}

	s.selectorLoading = true
	s.selectorRequestID++
	requestID := s.selectorRequestID
	service := s.resourceService
	return func() tea.Msg {
		if service == nil {
			return ResourceTypeOptionsReadyMsg{RequestID: requestID, Types: k8s.DefaultResourceTypes()}
		}
		types, err := service.GetAllResourceTypes()
		if err != nil {
			return ResourceTypeOptionsReadyMsg{RequestID: requestID, Types: k8s.DefaultResourceTypes(), Error: err}
		}
		return ResourceTypeOptionsReadyMsg{RequestID: requestID, Types: types}
	}
}

func (s *HomeScreen) openNamespaceSelector(msg NamespaceOptionsReadyMsg) tea.Cmd {
	if !s.selectorLoading || msg.RequestID != s.selectorRequestID {
		return nil
	}
	s.selectorLoading = false
	options := []SelectorOption{{Label: "<all>", Value: "<all>"}}
	for _, namespace := range sortedStrings(msg.Options) {
		if namespace == "" || namespace == "<all>" {
			continue
		}
		options = append(options, SelectorOption{Label: namespace, Value: namespace})
	}
	current := msg.Current
	if current == "" {
		current = "<all>"
	}
	s.selectorKind = homeSelectorNamespace
	s.selector = NewSelectorModel("Select Namespace", options, current)
	_, _ = s.selector.Update(tea.WindowSizeMsg{Width: s.width, Height: s.height})
	return nil
}

func (s *HomeScreen) openContextSelector(msg ContextOptionsReadyMsg) tea.Cmd {
	if !s.selectorLoading || msg.RequestID != s.selectorRequestID {
		return nil
	}
	s.selectorLoading = false
	options := make([]SelectorOption, 0, len(msg.Options))
	for _, contextName := range sortedStrings(msg.Options) {
		if contextName == "" {
			continue
		}
		options = append(options, SelectorOption{Label: contextName, Value: contextName})
	}
	s.selectorKind = homeSelectorContext
	s.selector = NewSelectorModel("Select Cluster Context", options, msg.Current)
	_, _ = s.selector.Update(tea.WindowSizeMsg{Width: s.width, Height: s.height})
	return nil
}

func (s *HomeScreen) openResourceTypeSelector(msg ResourceTypeOptionsReadyMsg) tea.Cmd {
	if !s.selectorLoading || msg.RequestID != s.selectorRequestID {
		return nil
	}
	s.selectorLoading = false
	if msg.Error != nil {
		s.log.Warn("resource type discovery failed; using defaults", "error", msg.Error)
	}

	s.selectorResourceTypes = make(map[string]*k8s.TrackedType, len(msg.Types))
	options := make([]SelectorOption, 0, len(msg.Types))
	for _, resourceType := range msg.Types {
		if resourceType == nil || resourceType.DisplayName == "" {
			continue
		}
		s.selectorResourceTypes[resourceType.DisplayName] = resourceType
		options = append(options, SelectorOption{
			Label: resourceType.DisplayName,
			Value: resourceType.DisplayName,
		})
	}
	s.selectorKind = homeSelectorResourceType
	s.selector = NewSelectorModel("Select Resource Type", options, s.currentResourceType().DisplayName)
	_, _ = s.selector.Update(tea.WindowSizeMsg{Width: s.width, Height: s.height})
	return nil
}

func (s *HomeScreen) finishSelector(result SelectorResult) tea.Cmd {
	kind := s.selectorKind
	resourceTypes := s.selectorResourceTypes
	s.selector = nil
	s.selectorResourceTypes = nil
	if result.Cancelled || !result.Accepted {
		return nil
	}

	switch kind {
	case homeSelectorNamespace:
		if result.Value == "<all>" {
			_ = s.namespaceFilter.SetPattern("")
		} else {
			_ = s.namespaceFilter.SetPattern(result.Value)
		}
		s.updateResources()
	case homeSelectorContext:
		return func() tea.Msg {
			return ContextSwitchRequestedMsg{ContextName: result.Value}
		}
	case homeSelectorResourceType:
		return s.selectResourceType(resourceTypes[result.Value])
	}
	return nil
}

func (s *HomeScreen) selectResourceType(resourceType *k8s.TrackedType) tea.Cmd {
	if resourceType == nil {
		return nil
	}

	for index, trackedType := range s.trackedTypes {
		if trackedType.GVR == resourceType.GVR {
			s.currentType = index
			s.selectedIndex = 0
			s.updateResources()
			return nil
		}
	}

	s.trackedTypes = append(s.trackedTypes, resourceType)
	s.currentType = len(s.trackedTypes) - 1
	s.selectedIndex = 0
	s.updateResources()
	if s.resourceService == nil {
		return nil
	}

	service := s.resourceService
	return func() tea.Msg {
		if err := service.StartInformer(resourceType); err != nil {
			return ErrorMsg{Error: err}
		}
		return ResourceUpdateMsg{}
	}
}

func (s *HomeScreen) resetForContext() {
	s.selector = nil
	s.selectorKind = homeSelectorNamespace
	s.selectorLoading = false
	s.selectorResourceTypes = nil
	s.filtering = false
	s.filterInput.Blur()
	_ = s.namespaceFilter.SetPattern("")
	_ = s.nameFilter.SetPattern("")
	s.filterError = nil
	s.trackedTypes = k8s.DefaultResourceTypes()
	s.currentType = 0
	s.selectedIndex = 0
	s.table.SetRows(nil)
	s.updateResources()
}

func (s *HomeScreen) namespaceOptionsFromResources() []string {
	seen := make(map[string]struct{})
	for _, resource := range s.resources {
		if resource != nil && resource.GetNamespace() != "" {
			seen[resource.GetNamespace()] = struct{}{}
		}
	}
	options := make([]string, 0, len(seen))
	for namespace := range seen {
		options = append(options, namespace)
	}
	return options
}

func sortedStrings(values []string) []string {
	copyValues := append([]string(nil), values...)
	sort.Strings(copyValues)
	return copyValues
}

func (s *HomeScreen) refreshCurrentType() tea.Cmd {
	if s.resourceService == nil {
		return nil
	}

	resourceType := s.currentResourceType()
	return func() tea.Msg {
		if err := s.resourceService.StartInformer(resourceType); err != nil {
			s.log.Error("failed to refresh resource type", "type", resourceType.DisplayName, "error", err)
			return ErrorMsg{Error: err}
		}
		return nil
	}
}

func (s *HomeScreen) nextResourceType(step int) {
	if len(s.trackedTypes) == 0 {
		return
	}

	s.currentType = (s.currentType + step) % len(s.trackedTypes)
	if s.currentType < 0 {
		s.currentType += len(s.trackedTypes)
	}
	s.selectedIndex = 0
	s.updateResources()
}

func (s *HomeScreen) moveSelection(delta int) {
	if len(s.filteredResources) == 0 || delta == 0 {
		return
	}

	s.setSelection(s.selectedIndex + delta)
}

func (s *HomeScreen) setSelection(index int) {
	if len(s.filteredResources) == 0 {
		s.selectedIndex = 0
		return
	}

	if index < 0 {
		index = 0
	}
	if index >= len(s.filteredResources) {
		index = len(s.filteredResources) - 1
	}
	s.selectedIndex = index
	s.table.SetCursor(index)
}

func (s *HomeScreen) pageSize() int {
	if height := s.table.Height(); height > 0 {
		return height
	}
	return 10
}

func (s *HomeScreen) currentResourceType() *k8s.TrackedType {
	if s.currentType >= 0 && s.currentType < len(s.trackedTypes) {
		return s.trackedTypes[s.currentType]
	}
	return k8s.PodResource
}

func (s *HomeScreen) selectedResourceKey() string {
	if s.selectedIndex < 0 || s.selectedIndex >= len(s.filteredResources) {
		return ""
	}
	resource := s.filteredResources[s.selectedIndex]
	return resourceKey(resource)
}

func resourceKey(resource k8s.TrackedObject) string {
	if resource == nil {
		return ""
	}
	return fmt.Sprintf("%s/%s/%s", resource.GetKind(), resource.GetNamespace(), resource.GetName())
}

func (s *HomeScreen) updateResources() {
	selectedKey := s.selectedResourceKey()
	resourceType := s.currentResourceType()

	if s.resourceService == nil {
		s.resources = nil
		s.filteredResources = nil
		// Bubbles renders existing rows during SetColumns. Clear them first
		// whenever the resource type changes so old row widths cannot be
		// indexed against the new column layout.
		s.table.SetRows(nil)
		s.table.SetColumns(resourceType.Columns)
		s.selectedIndex = 0
		return
	}

	s.resources = s.resourceService.GetResources(resourceType.GVR)
	s.filteredResources = s.namespaceFilter.FilterResources(s.resources)
	s.filteredResources = s.nameFilter.FilterResources(s.filteredResources)

	// SetColumns updates the viewport immediately, so never leave rows from
	// the previous resource type in the table while changing column schemas.
	s.table.SetRows(nil)
	s.table.SetColumns(resourceType.Columns)
	rows := make([]table.Row, 0, len(s.filteredResources))
	for _, resource := range s.filteredResources {
		rows = append(rows, resourceType.RowBinder(resource))
	}
	s.table.SetRows(rows)

	if len(s.filteredResources) == 0 {
		s.selectedIndex = 0
		return
	}

	if selectedKey != "" {
		for index, resource := range s.filteredResources {
			if resourceKey(resource) == selectedKey {
				s.selectedIndex = index
				break
			}
		}
	}
	s.setSelection(s.selectedIndex)
}

func (s *HomeScreen) updateSize(msg tea.WindowSizeMsg) {
	s.width = msg.Width
	s.height = msg.Height
	s.layoutComponents()
}

func (s *HomeScreen) mainContentHeight() int {
	// Reserve one row for the status line and two rows for the outer border.
	return max(1, s.height-1-2)
}

func (s *HomeScreen) resourceContentWidth() int {
	width := max(1, s.width-4) // outer border on both sides
	if s.showingFavoriteTypes {
		width = max(1, width-favoriteTypesBoxWidth)
	}
	return width
}

func (s *HomeScreen) layoutComponents() {
	mainHeight := s.mainContentHeight()
	resourceWidth := s.resourceContentWidth()

	filterHeight := 0
	if s.filtering {
		filterHeight = 3 // top border, input row, bottom border
	}
	statusHeight := 1
	tableHeight := max(1, mainHeight-filterHeight-statusHeight)

	s.table.SetHeight(tableHeight)
	s.table.SetWidth(resourceWidth)

	filterLabelWidth := lipgloss.Width("Resource name filter: ")
	s.filterInput.SetWidth(max(10, resourceWidth-filterLabelWidth-4))

	if s.showingFavoriteTypes {
		s.favoriteTypesViewport.SetWidth(favoriteTypesContentWidth)
		s.favoriteTypesViewport.SetHeight(max(1, mainHeight-2))
	}
}

// IsTyping reports whether Home's active submode owns text input. Selector
// loading is not typing; global commands remain available until the field is
// actually displayed.
func (s *HomeScreen) IsTyping() bool {
	return s.filtering || s.selector != nil
}

// CommandPresentation exposes the registry for the currently active Home
// submode. It contains metadata only; HomeScreen still dispatches commands.
func (s *HomeScreen) CommandPresentation() command.Presentation {
	if s.selector != nil {
		return s.selector.Presentation()
	}
	if s.filtering {
		return s.filterRegistry.Presentation()
	}
	return s.registry.Presentation()
}

func (s *HomeScreen) render() string {
	if s.width <= 0 || s.height <= 0 {
		return s.renderStatusLine()
	}

	statusLine := s.renderStatusLine()
	mainHeight := max(1, s.height-lipgloss.Height(statusLine))
	mainContent := s.renderMainContent()
	bordered := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(util.ColorPrimary).
		Width(max(1, s.width-2)).
		Height(mainHeight).
		Render(mainContent)

	return lipgloss.JoinVertical(lipgloss.Left, statusLine, bordered)
}

func (s *HomeScreen) renderMainContent() string {
	sections := make([]string, 0, 3)
	if s.filtering {
		sections = append(sections, s.renderFilterBar())
	}
	sections = append(sections, s.renderResourceTable())
	sections = append(sections, s.renderStatusBar())

	resourceContent := lipgloss.JoinVertical(lipgloss.Left, sections...)
	if !s.showingFavoriteTypes {
		return resourceContent
	}

	return lipgloss.JoinHorizontal(
		lipgloss.Top,
		s.renderFavoriteTypesContent(),
		resourceContent,
	)
}

func (s *HomeScreen) renderFavoriteTypesContent() string {
	return lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(util.ColorBorder).
		Padding(0, 1).
		Width(favoriteTypesBoxWidth).
		Height(s.mainContentHeight()).
		Render(s.favoriteTypesViewport.View())
}

func (s *HomeScreen) renderStatusLine() string {
	clusterName := "<Unknown Cluster>"
	if s.resourceService != nil {
		clusterName = s.resourceService.GetClusterName()
	}

	left := lipgloss.NewStyle().
		Foreground(util.ColorPrimary).
		Bold(true).
		Render(fmt.Sprintf("▶ %s", clusterName))

	resourceType := s.currentResourceType()
	badge := lipgloss.NewStyle().
		Foreground(util.ColorSecondary).
		Background(lipgloss.Color("#1a1a1a")).
		Bold(true).
		Padding(0, 1).
		Render(fmt.Sprintf("● %s", resourceType.DisplayName))

	metadata := "watch: live"
	if s.resourceService != nil {
		lastUpdate := s.resourceService.GetLastUpdateTime(resourceType.GVR)
		if !lastUpdate.IsZero() {
			metadata = fmt.Sprintf("updated %s • %s", formatRelativeTime(lastUpdate), metadata)
		}
	}
	right := badge + " • " + lipgloss.NewStyle().Foreground(util.ColorMuted).Render(metadata)

	spacing := s.width - ansi.StringWidth(ansi.Strip(left)) - ansi.StringWidth(ansi.Strip(right)) - 4
	if spacing < 1 {
		spacing = 1
	}
	return lipgloss.NewStyle().
		Padding(0, 1).
		Render(left + strings.Repeat(" ", spacing) + right)
}

func (s *HomeScreen) renderResourceTable() string {
	if s.resourceService == nil {
		return lipgloss.NewStyle().Foreground(util.ColorMuted).Render("Resource service unavailable")
	}
	if !s.resourceService.IsResourceReady(s.currentResourceType().GVR) {
		return lipgloss.NewStyle().Foreground(util.ColorMuted).Render("Loading resources…")
	}
	if len(s.filteredResources) == 0 {
		return lipgloss.NewStyle().Foreground(util.ColorMuted).Render("No resources found")
	}
	return s.table.View()
}

func (s *HomeScreen) renderStatusBar() string {
	count := lipgloss.NewStyle().Foreground(util.ColorPrimary).Bold(true)
	countInfo := fmt.Sprintf("%d resources", len(s.resources))
	if len(s.filteredResources) != len(s.resources) {
		shown := lipgloss.NewStyle().Foreground(util.ColorSecondary).Bold(true)
		countInfo += fmt.Sprintf(" • %s", shown.Render(fmt.Sprintf("%d shown", len(s.filteredResources))))
	}

	parts := []string{count.Render(countInfo)}
	var activeFilters []string
	if pattern := s.namespaceFilter.GetPattern(); pattern != "" {
		activeFilters = append(activeFilters, "ns:"+pattern)
	}
	if pattern := s.nameFilter.GetPattern(); pattern != "" {
		activeFilters = append(activeFilters, "name:"+pattern)
	}
	if len(activeFilters) > 0 {
		parts = append(parts, lipgloss.NewStyle().Foreground(util.ColorAccent).Render("filters: "+strings.Join(activeFilters, ", ")))
	}
	if s.filterError != nil {
		parts = append(parts, lipgloss.NewStyle().Foreground(util.ColorDanger).Render("filter error: "+s.filterError.Error()))
	}
	return lipgloss.NewStyle().
		Foreground(util.ColorMuted).
		Padding(0, 1).
		Width(s.resourceContentWidth()).
		Render(strings.Join(parts, "  "))
}

func (s *HomeScreen) renderFilterBar() string {
	return lipgloss.NewStyle().
		Foreground(util.ColorSecondary).
		BorderStyle(lipgloss.RoundedBorder()).
		BorderForeground(util.ColorBorder).
		Padding(0, 1).
		Width(s.resourceContentWidth()).
		Render("Resource name filter: " + s.filterInput.View())
}

func formatRelativeTime(t time.Time) string {
	if t.IsZero() {
		return "never"
	}

	d := time.Since(t)
	switch {
	case d < time.Second:
		return "just now"
	case d < time.Minute:
		return fmt.Sprintf("%ds ago", int(d.Seconds()))
	case d < time.Hour:
		return fmt.Sprintf("%dm ago", int(d.Minutes()))
	case d < 24*time.Hour:
		return fmt.Sprintf("%dh ago", int(d.Hours()))
	default:
		return fmt.Sprintf("%dd ago", int(d.Hours()/24))
	}
}
