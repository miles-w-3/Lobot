package modes

import (
	"strings"

	"charm.land/bubbles/v2/textinput"
	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/x/ansi"
	"github.com/miles-w-3/lobot/internal/command"
	"github.com/miles-w-3/lobot/internal/keys"
	"github.com/miles-w-3/lobot/internal/util"
)

// SelectorKind identifies the domain value a selector returns. The selector
// itself only knows about labels and values; Home owns the domain-specific
// consequences of a selection.
type SelectorKind uint8

const (
	SelectorKindNamespace SelectorKind = iota
	SelectorKindContext
	SelectorKindResourceType
)

// SelectorOption is the presentation/value pair shown by SelectorModel.
type SelectorOption struct {
	Label string
	Value string
}

// SelectorResult is returned synchronously to the owning screen. Selector
// navigation is local UI state and never becomes a root-level Bubble Tea
// message.
type SelectorResult struct {
	Kind      SelectorKind
	Value     string
	Accepted  bool
	Cancelled bool
}

const (
	selectorMaxWidth  = 70
	selectorMaxHeight = 22
	selectorMinWidth  = 36
	selectorMinHeight = 8
)

// SelectorModel is a reusable selection overlay. It is deliberately not a
// tea.Model or Screen: the owning screen controls its lifecycle and consumes
// results directly.
type SelectorModel struct {
	kind     SelectorKind
	title    string
	options  []SelectorOption
	filtered []SelectorOption
	current  string
	selected int
	scroll   int
	width    int
	height   int
	input    textinput.Model
	registry *command.Registry[keys.SelectorCmd]
}

func NewSelectorModel(kind SelectorKind, title string, options []SelectorOption, current string) *SelectorModel {
	input := textinput.New()
	input.Prompt = "/ "
	input.Placeholder = "filter choices"
	input.Focus()

	model := &SelectorModel{
		kind:     kind,
		title:    title,
		options:  append([]SelectorOption(nil), options...),
		current:  current,
		input:    input,
		registry: keys.NewSelectorRegistry(),
	}
	model.filterOptions()
	return model
}

// Presentation exposes selector commands to the root-owned help bar and
// command palette without making the selector a root-owned screen.
func (s *SelectorModel) Presentation() command.Presentation {
	if s == nil || s.registry == nil {
		return command.Presentation{}
	}
	return s.registry.Presentation()
}

// Update handles selector messages and returns a local result for accept or
// cancel. Recognized selector keys are consumed here; only unbound input is
// forwarded to the text input.
func (s *SelectorModel) Update(msg tea.Msg) (*SelectorResult, tea.Cmd) {
	if s == nil {
		return nil, nil
	}

	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		s.width = msg.Width
		s.height = msg.Height
		s.input.SetWidth(max(10, s.contentWidth()-4))
		return nil, nil
	case tea.KeyPressMsg:
		if cmd, err := s.registry.Dispatch(msg); err == nil {
			return s.dispatch(cmd)
		}
	}

	before := s.input.Value()
	var cmd tea.Cmd
	s.input, cmd = s.input.Update(msg)
	if s.input.Value() != before {
		s.filterOptions()
	}
	return nil, cmd
}

func (s *SelectorModel) dispatch(cmd keys.SelectorCmd) (*SelectorResult, tea.Cmd) {
	switch cmd {
	case keys.SelectorCmdUp:
		s.move(-1)
	case keys.SelectorCmdDown:
		s.move(1)
	case keys.SelectorCmdPageUp:
		s.move(-s.pageSize())
	case keys.SelectorCmdPageDown:
		s.move(s.pageSize())
	case keys.SelectorCmdAccept:
		if s.selected >= 0 && s.selected < len(s.filtered) {
			return &SelectorResult{
				Kind:     s.kind,
				Value:    s.filtered[s.selected].Value,
				Accepted: true,
			}, nil
		}
	case keys.SelectorCmdCancel:
		return &SelectorResult{Kind: s.kind, Cancelled: true}, nil
	}
	return nil, nil
}

func (s *SelectorModel) move(delta int) {
	if len(s.filtered) == 0 || delta == 0 {
		return
	}

	s.selected += delta
	if s.selected < 0 {
		s.selected = 0
	}
	if s.selected >= len(s.filtered) {
		s.selected = len(s.filtered) - 1
	}
	s.adjustScroll()
}

func (s *SelectorModel) pageSize() int {
	rows := s.listHeight()
	if rows < 1 {
		return 1
	}
	return rows
}

func (s *SelectorModel) filterOptions() {
	query := strings.ToLower(strings.TrimSpace(s.input.Value()))
	s.filtered = s.filtered[:0]
	for _, option := range s.options {
		if query == "" || strings.Contains(strings.ToLower(option.Label), query) || strings.Contains(strings.ToLower(option.Value), query) {
			s.filtered = append(s.filtered, option)
		}
	}

	s.selected = 0
	if query == "" && s.current != "" {
		for index, option := range s.filtered {
			if option.Value == s.current {
				s.selected = index
				break
			}
		}
	}
	if len(s.filtered) == 0 {
		s.selected = 0
	}
	s.scroll = 0
	s.adjustScroll()
}

func (s *SelectorModel) adjustScroll() {
	rows := s.listHeight()
	if rows < 1 {
		rows = 1
	}
	if s.selected < s.scroll {
		s.scroll = s.selected
	}
	if s.selected >= s.scroll+rows {
		s.scroll = s.selected - rows + 1
	}
	if s.scroll < 0 {
		s.scroll = 0
	}
}

func (s *SelectorModel) dimensions() (int, int) {
	width := selectorMaxWidth
	height := selectorMaxHeight
	if s.width > 0 {
		width = min(width, max(selectorMinWidth, s.width-4))
	}
	if s.height > 0 {
		height = min(height, max(selectorMinHeight, s.height-4))
	}
	return width, height
}

func (s *SelectorModel) contentWidth() int {
	width, _ := s.dimensions()
	return max(1, width-6)
}

func (s *SelectorModel) listHeight() int {
	_, height := s.dimensions()
	return max(1, height-7)
}

func (s *SelectorModel) View() string {
	width, height := s.dimensions()
	contentWidth := max(1, width-6)

	title := lipgloss.NewStyle().Bold(true).Foreground(util.ColorText).Render(s.title)
	input := s.input.View()

	var body strings.Builder
	body.WriteString(title)
	body.WriteString("\n\n")
	body.WriteString(input)
	body.WriteString("\n\n")

	if len(s.filtered) == 0 {
		body.WriteString(lipgloss.NewStyle().Foreground(util.ColorMuted).Render("No matching choices"))
	} else {
		rows := s.listHeight()
		end := min(len(s.filtered), s.scroll+rows)
		for index := s.scroll; index < end; index++ {
			option := s.filtered[index]
			line := ansi.Truncate(option.Label, contentWidth-2, "…")
			if index == s.selected {
				line = lipgloss.NewStyle().
					Background(util.ColorPrimary).
					Foreground(lipgloss.Color("#000000")).
					Bold(true).
					Width(contentWidth).
					Render("▶ " + line)
			} else {
				line = lipgloss.NewStyle().Foreground(util.ColorText).Render("  " + line)
			}
			body.WriteString(line)
			if index < end-1 {
				body.WriteByte('\n')
			}
		}
	}

	return lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(util.ColorAccent).
		Padding(1, 2).
		Width(width).
		Height(height).
		Render(body.String())
}
