package modes

import (
	"log/slog"
	"sort"
	"strings"

	"charm.land/bubbles/v2/textinput"
	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	"github.com/miles-w-3/lobot/internal/command"
	"github.com/miles-w-3/lobot/internal/keys"
	"github.com/miles-w-3/lobot/internal/util"
)

// Constants for palette dimensions - improve in future
const (
	paletteMaxWidth  = 80
	paletteMaxHeight = 22
	paletteMinWidth  = 40
	paletteMinHeight = 10
)

// PaletteModel is the model for the command palette
type PaletteModel struct {
	input           textinput.Model
	entries         []command.PaletteEntry
	filteredEntries []command.PaletteEntry
	selectedIndex   int
	scrollOffset    int
	width           int
	height          int
	registry        *command.Registry[keys.PaletteCmd]
}

// NewPaletteModel creates a new palette model
func NewPaletteModel(width, height int, isDark bool) PaletteModel {
	ti := textinput.New()
	ti.Placeholder = ""
	ti.Focus()
	ti.Prompt = ""
	ti.SetWidth(paletteMaxWidth - 4)

	m := PaletteModel{
		input:    ti,
		width:    width,
		height:   height,
		registry: keys.NewPaletteRegistry(),
	}
	m.SetTheme(isDark)
	return m
}

// SetTheme applies background-aware text input styles.
func (m *PaletteModel) SetTheme(isDark bool) {
	styles := textinput.DefaultStyles(isDark)
	styles.Focused.Text = lipgloss.NewStyle().Foreground(util.ColorText)
	styles.Blurred.Text = lipgloss.NewStyle().Foreground(util.ColorText)
	styles.Cursor.Color = util.ColorPrimary
	m.input.SetStyles(styles)
}

// SetSize updates the terminal dimensions used to constrain the palette.
func (m *PaletteModel) SetSize(width, height int) {
	m.width = width
	m.height = height
}

// SetEntries updates the list of available commands
func (m *PaletteModel) SetEntries(entries []command.PaletteEntry) {
	m.entries = entries
	m.filterEntries()
}

// Reset clears the input and resets selection
func (m *PaletteModel) Reset() {
	m.input.SetValue("")
	m.selectedIndex = 0
	m.scrollOffset = 0
}

// getPaletteDimensions calculates the actual palette dimensions based on terminal size
func (m *PaletteModel) getPaletteDimensions() (width, height int) {
	// Start with max dimensions
	width = paletteMaxWidth
	height = paletteMaxHeight

	// Constrain to terminal size with some padding
	maxAvailWidth := m.width - 4
	maxAvailHeight := m.height - 4

	if width > maxAvailWidth {
		width = maxAvailWidth
	}
	if height > maxAvailHeight {
		height = maxAvailHeight
	}

	// Ensure minimum dimensions
	if width < paletteMinWidth {
		width = paletteMinWidth
	}
	if height < paletteMinHeight {
		height = paletteMinHeight
	}

	return width, height
}

// filterEntries filters the entries based on input and sorts by group then description
func (m *PaletteModel) filterEntries() {
	query := strings.ToLower(m.input.Value())

	if query == "" {
		// Show all entries when no search query
		m.filteredEntries = make([]command.PaletteEntry, len(m.entries))
		copy(m.filteredEntries, m.entries)
	} else {
		var matches []command.PaletteEntry
		for _, e := range m.entries {
			// Search in description, group, key, alt keys, and searchable terms
			hit := strings.Contains(strings.ToLower(e.Description), query) ||
				strings.Contains(strings.ToLower(e.Group), query) ||
				strings.Contains(strings.ToLower(e.Key), query)

			if !hit {
				for _, alias := range e.Searchable {
					if strings.Contains(strings.ToLower(alias), query) {
						hit = true
						break
					}
				}
			}

			if !hit {
				for _, k := range e.AltKeys {
					if strings.Contains(strings.ToLower(k), query) {
						hit = true
						break
					}
				}
			}

			if hit {
				matches = append(matches, e)
			}
		}
		m.filteredEntries = matches
	}

	// Sort by group then description
	sort.SliceStable(m.filteredEntries, func(i, j int) bool {
		if m.filteredEntries[i].Group != m.filteredEntries[j].Group {
			return m.filteredEntries[i].Group < m.filteredEntries[j].Group
		}
		return m.filteredEntries[i].Description < m.filteredEntries[j].Description
	})

	// Preserve selection when possible, only reset if out of bounds
	if m.selectedIndex >= len(m.filteredEntries) {
		m.selectedIndex = 0
		m.scrollOffset = 0
	}
}

// Init initializes the palette model
func (m PaletteModel) Init() tea.Cmd {
	return textinput.Blink
}

// Update handles messages and updates the palette model
func (m PaletteModel) Update(msg tea.Msg) (PaletteModel, tea.Cmd) {
	var cmd tea.Cmd
	logger := slog.Default()
	logger.Debug("Handling command palette update")

	switch msg := msg.(type) {
	case tea.KeyPressMsg:
		logger.Debug("Handling command palette key press", "key", msg.String())

		// Check palette registry for commands
		if paletteCmd, err := m.registry.Dispatch(msg); err == nil {
			switch paletteCmd {
			case keys.PaletteCmdUp:
				if m.selectedIndex > 0 {
					m.selectedIndex--
					m.adjustScrollOffset()
				}
				return m, nil
			case keys.PaletteCmdDown:
				if m.selectedIndex < len(m.filteredEntries)-1 {
					m.selectedIndex++
					m.adjustScrollOffset()
				}
				return m, nil
			case keys.PaletteCmdEnter:
				if len(m.filteredEntries) > 0 && m.selectedIndex >= 0 && m.selectedIndex < len(m.filteredEntries) {
					selected := m.filteredEntries[m.selectedIndex]
					return m, func() tea.Msg {
						return PaletteSelectedMsg{Entry: selected}
					}
				}
				return m, nil
			case keys.PaletteCmdBack:
				return m, func() tea.Msg {
					return PaletteBackMsg{}
				}
			}
		}
	}

	// Update the input component
	m.input, cmd = m.input.Update(msg)

	// Refilter on input change
	m.filterEntries()

	return m, cmd
}

// getItemIndex returns the visual item index for a given entry index
// This accounts for group headers that appear before the entry
func (m *PaletteModel) getItemIndex(entryIdx int) int {
	if entryIdx >= len(m.filteredEntries) {
		return entryIdx
	}

	itemIdx := 0
	currentGroup := ""

	for i := 0; i <= entryIdx && i < len(m.filteredEntries); i++ {
		entry := m.filteredEntries[i]
		if entry.Group != currentGroup {
			itemIdx++ // Count the group header
			currentGroup = entry.Group
		}
		if i < entryIdx {
			itemIdx++ // Count the entry itself (but not for the target entry)
		}
	}

	return itemIdx
}

// adjustScrollOffset ensures the selected item is visible in the viewport
func (m *PaletteModel) adjustScrollOffset() {
	_, height := m.getPaletteDimensions()

	// Calculate available space for list items
	listHeight := height - 6
	if listHeight < 1 {
		listHeight = 1
	}

	// Convert selected entry index to item index (including headers)
	selectedItemIdx := m.getItemIndex(m.selectedIndex)

	// Scroll up if selection is above viewport
	if selectedItemIdx < m.scrollOffset {
		m.scrollOffset = selectedItemIdx
	}

	// Scroll down if selection is below viewport
	if selectedItemIdx >= m.scrollOffset+listHeight {
		m.scrollOffset = selectedItemIdx - listHeight + 1
	}

	// Ensure scroll offset stays valid
	if m.scrollOffset < 0 {
		m.scrollOffset = 0
	}
}

// View renders the palette
func (m PaletteModel) View() string {
	width, height := m.getPaletteDimensions()

	// Main container style with border
	containerStyle := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(util.ColorPrimary).
		Width(width).
		Height(height).
		Padding(0, 1)

	var content strings.Builder

	// Header: title on the left and the registry-backed close key on the right.
	headerLeft := lipgloss.NewStyle().
		Bold(true).
		Foreground(util.ColorText).
		Render("Commands")

	backKey := ""
	if entry, ok := m.registry.EntryForCommand(keys.PaletteCmdBack); ok {
		backKey = entry.Display
	}
	headerRight := lipgloss.NewStyle().
		Foreground(util.ColorMuted).
		Render(backKey)

	// Calculate spacing between header elements
	headerSpace := width - lipgloss.Width(headerLeft) - lipgloss.Width(headerRight) - 4
	if headerSpace < 1 {
		headerSpace = 1
	}

	content.WriteString(headerLeft)
	content.WriteString(strings.Repeat(" ", headerSpace))
	content.WriteString(headerRight)
	content.WriteString("\n\n")

	// Search input
	content.WriteString(m.input.View())
	content.WriteString("\n\n")

	// Calculate available height for the list
	listHeight := height - 6 // Header (3) + search (1) + padding
	if listHeight < 1 {
		listHeight = 1
	}

	// Render the list of commands
	if len(m.filteredEntries) == 0 {
		noResults := lipgloss.NewStyle().
			Foreground(util.ColorMuted).
			Render("No matching commands")
		content.WriteString(noResults)
	} else {
		m.renderCommandList(&content, width-4, listHeight)
	}

	return containerStyle.Render(content.String())
}

// renderCommandList renders the scrollable list of commands with group headers
func (m *PaletteModel) renderCommandList(content *strings.Builder, width, height int) {
	// Styles
	groupStyle := lipgloss.NewStyle().
		Bold(true).
		Foreground(util.ColorSecondary)

	selectedRowStyle := lipgloss.NewStyle().
		Background(util.ColorPrimary).
		Foreground(lipgloss.Color("#000000")).
		Bold(true).
		Width(width)

	normalStyle := lipgloss.NewStyle().
		Foreground(util.ColorText)

	keyStyle := lipgloss.NewStyle().
		Foreground(util.ColorMuted)

	// Build list with group headers
	type listItem struct {
		isGroup    bool
		groupName  string
		entry      *command.PaletteEntry
		entryIndex int
	}

	var items []listItem
	currentGroup := ""

	for i, entry := range m.filteredEntries {
		// Add group header if group changes
		if entry.Group != currentGroup {
			items = append(items, listItem{
				isGroup:   true,
				groupName: entry.Group,
			})
			currentGroup = entry.Group
		}

		items = append(items, listItem{
			isGroup:    false,
			entry:      &m.filteredEntries[i],
			entryIndex: i,
		})
	}

	// Calculate visible range
	visibleStart := m.scrollOffset
	visibleEnd := visibleStart + height
	if visibleEnd > len(items) {
		visibleEnd = len(items)
	}
	if visibleStart > len(items) {
		visibleStart = len(items)
	}

	// Render visible items
	for i := visibleStart; i < visibleEnd; i++ {
		if i >= len(items) {
			break
		}

		item := items[i]

		if item.isGroup {
			// Render group header with padding (except for first group)
			if i > 0 {
				content.WriteString("\n")
			}
			content.WriteString(groupStyle.Render(item.groupName))
			content.WriteString("\n")
		} else {
			// Render command entry
			entry := item.entry
			isSelected := item.entryIndex == m.selectedIndex

			// Get display key
			displayKey := entry.Display
			if displayKey == "" {
				displayKey = entry.Key
			}

			// Calculate spacing between description and key
			descWidth := len(entry.Description)
			keyWidth := len(displayKey)
			spaceWidth := width - descWidth - keyWidth - 2
			if spaceWidth < 1 {
				spaceWidth = 1
			}

			// Render line with full-width background for selected items
			if isSelected {
				// Build the full line content
				lineContent := entry.Description + strings.Repeat(" ", spaceWidth) + displayKey
				// Apply full-width background style
				line := selectedRowStyle.Render(lineContent)
				content.WriteString(line)
			} else {
				line := normalStyle.Render(entry.Description) +
					strings.Repeat(" ", spaceWidth) +
					keyStyle.Render(displayKey)
				content.WriteString(line)
			}
			content.WriteString("\n")
		}
	}
}
