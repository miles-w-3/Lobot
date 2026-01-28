package ui

import (
	"log/slog"
	"sort"
	"strings"

	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/miles-w-3/lobot/internal/command"
)

// PaletteSelectedMsg implies a command was selected
type PaletteSelectedMsg struct {
	Entry command.PaletteEntry
}

// PaletteModel is the model for the command palette
type PaletteModel struct {
	input           textinput.Model
	entries         []command.PaletteEntry
	filteredEntries []command.PaletteEntry
	selectedIndex   int
	width           int
	height          int
}

// NewPaletteModel creates a new palette
func NewPaletteModel(width, height int) PaletteModel {
	ti := textinput.New()
	ti.Placeholder = "Search commands..."
	ti.Focus()
	ti.Prompt = " "
	ti.TextStyle = lipgloss.NewStyle().Foreground(ColorText)
	ti.Cursor.Style = lipgloss.NewStyle().Foreground(ColorPrimary)

	return PaletteModel{
		input:  ti,
		width:  width,
		height: height,
	}
}

// SetEntries updates the list of available commands
func (m *PaletteModel) SetEntries(entries []command.PaletteEntry) {
	m.entries = entries
	m.filterEntries()
}

// Reset clears the input and resets selection
func (m *PaletteModel) Reset() {
	// Clear input value but don't unfocus
	m.input.SetValue("")
	m.selectedIndex = 0
}

// filterEntries filters the entries based on input
func (m *PaletteModel) filterEntries() {
	query := strings.ToLower(m.input.Value())

	if query == "" {
		// If empty, show all (or maybe top X, but showing all grouped is fine)
		// Copied to filtered
		// We'll group them later in rendering?
		// Better to sort/group here if we want consistent navigation.
		// For now, just copy.
		m.filteredEntries = make([]command.PaletteEntry, len(m.entries))
		copy(m.filteredEntries, m.entries)
	} else {
		var matches []command.PaletteEntry
		// Simple containment search
		for _, e := range m.entries {
			// Check Description, Key, Group, Searchable
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
				// Check alt keys
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

	// Sort filtered entries by Group then Description
	sort.SliceStable(m.filteredEntries, func(i, j int) bool {
		if m.filteredEntries[i].Group != m.filteredEntries[j].Group {
			return m.filteredEntries[i].Group < m.filteredEntries[j].Group
		}
		return m.filteredEntries[i].Description < m.filteredEntries[j].Description
	})

	m.selectedIndex = 0
}

func (m PaletteModel) Init() tea.Cmd {
	return textinput.Blink
}

func (m PaletteModel) Update(msg tea.Msg) (PaletteModel, tea.Cmd) {
	var cmd tea.Cmd
	logger := slog.Default()
	logger.Debug("Handling command palette update ")
	switch msg := msg.(type) {
	case tea.KeyMsg:
		logger.Debug("Handling command palette key press ", "key", msg.String())
		switch msg.String() {
		case "up", "ctrl+k":
			if m.selectedIndex > 0 {
				m.selectedIndex--
			}
			return m, nil
		case "down", "ctrl+j":
			if m.selectedIndex < len(m.filteredEntries)-1 {
				m.selectedIndex++
			}
			return m, nil
			// Esc treated by parent?
			// Better to handle enter here to select
		case "enter":
			if len(m.filteredEntries) > 0 {
				selected := m.filteredEntries[m.selectedIndex]
				return m, func() tea.Msg {
					return PaletteSelectedMsg{Entry: selected}
				}
			}
		}
	}

	m.input, cmd = m.input.Update(msg)
	// Refilter on input change
	// Optimized: only refilter if text changed?
	// textinput update returns a generic cmd, hard to know if text changed without comparing
	// But textinput state IS updated.

	// Refilter on input change
	m.filterEntries()

	return m, cmd
}

func (m PaletteModel) View() string {
	// Render the dialog box
	// Header
	// Input
	// List

	// Calculate available height for list
	// Box borders + header + input + padding = approx 6-8 lines?

	width := 60
	height := 20
	if m.width < 64 {
		width = m.width - 4
	}
	if m.height < 24 {
		height = m.height - 4
	}

	var s strings.Builder

	// Styles
	boxStyle := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(ColorBorder).
		Width(width).
		Height(height).
		Padding(0, 0) // We'll manage padding inside

	// Header "Commands    esc"
	headerLeft := lipgloss.NewStyle().Bold(true).Foreground(ColorText).Render("Commands")
	headerRight := lipgloss.NewStyle().Foreground(ColorMuted).Render("esc")
	space := width - lipgloss.Width(headerLeft) - lipgloss.Width(headerRight) - 4 // -4 for border/padding?
	if space < 1 {
		space = 1
	}
	s.WriteString(" " + headerLeft + strings.Repeat(" ", space) + headerRight + " ")
	s.WriteString("\n\n")

	// Input "S|earch" - mimic image, or just "Search..."
	// image has highlighted S? "Search" text.
	// We use standard input
	s.WriteString(" " + m.input.View() + "\n")
	s.WriteString("\n")

	// List
	listViewHeight := height - 5 // Approx
	if listViewHeight < 1 {
		listViewHeight = 1
	}

	// Viewport logic roughly:
	// Scroll to keep selectedIndex visible
	// Reuse viewport? Or manual slice?
	// Manual slice for simple control

	startIdx := 0
	endIdx := len(m.filteredEntries)

	// Scroll logic
	if m.selectedIndex >= listViewHeight {
		startIdx = m.selectedIndex - listViewHeight + 1
	}
	if endIdx > startIdx+listViewHeight {
		endIdx = startIdx + listViewHeight
	}

	// Strategy: Render filteredEntries to a slice of strings.
	// If entry i starts a group, pre-pend group header line.

	// Let's try simple calculation
	rows := make([]string, 0)
	selectionRow := 0 // which row index is the selected item?

	currentG := ""
	for i, e := range m.filteredEntries {
		if e.Group != currentG {
			rows = append(rows, "") // spacing
			rows = append(rows, lipgloss.NewStyle().Foreground(ColorSecondary).Bold(true).Render(e.Group))
			currentG = e.Group
		}

		// Item row
		style := lipgloss.NewStyle().Padding(0, 1).Width(width - 2) // -2 borders
		keyStyle := lipgloss.NewStyle().Foreground(ColorMuted)

		if i == m.selectedIndex {
			style = style.Background(ColorPrimary).Foreground(lipgloss.Color("#000000")).Bold(true)
			keyStyle = keyStyle.Foreground(lipgloss.Color("#1a1a1a")) // Darker gray for key on selection
			selectionRow = len(rows)
		} else {
			style = style.Foreground(ColorText)
		}

		// "Description ......... Key"
		desc := e.Description
		key := e.Display
		if key == "" {
			key = e.Key
		}
		// Preprocess Shift? Already done in Binding.Display?
		// e.Display should have it.

		// Spacer
		spaceW := width - 4 - len(desc) - len(key)
		if spaceW < 1 {
			spaceW = 1
		}

		line := desc + strings.Repeat(" ", spaceW) + keyStyle.Render(key)
		rows = append(rows, style.Render(line))
	}

	// Viewport calculation
	// We have `rows`. `selectionRow` points to our interesting part.
	// Viewport height `listViewHeight`.
	// We want `selectionRow` to be visible.

	vpStart := 0
	if selectionRow >= listViewHeight {
		vpStart = selectionRow - listViewHeight + 2 // keep context?
	}

	visibleRows := rows[vpStart:]
	if len(visibleRows) > listViewHeight {
		visibleRows = visibleRows[:listViewHeight]
	}

	for _, r := range visibleRows {
		s.WriteString(" " + r + "\n")
	}

	return boxStyle.Render(s.String())
}
