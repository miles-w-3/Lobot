package command

import (
	"fmt"
	"log/slog"
	"strings"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
)

type PaletteEntry struct {
	Key         string   // Primary key (e.g., "ctrl+n")
	AltKeys     []string // Alternate keys
	Display     string   // Display string (e.g., "↑" for "up")
	Description string   // Human-readable description
	Group       string   // Group title for categorization
	Searchable  []string // Additional search terms
}

// CommandGroupPresentation is the immutable command metadata needed to render
// short and full help. It contains no dispatch state or styling.
type CommandGroupPresentation struct {
	Title       string
	Description string
	Commands    []PaletteEntry
}

// Presentation is a snapshot of the commands a screen can dispatch. The root
// owns rendering this metadata as help; screens only expose the snapshot.
type Presentation struct {
	Groups         []CommandGroupPresentation
	PaletteEntries []PaletteEntry
}

func (p Presentation) Empty() bool {
	return len(p.Groups) == 0 && len(p.PaletteEntries) == 0
}

func (p Presentation) ShortView(config HelpConfig) string {
	var items []string
	for _, group := range p.Groups {
		if len(group.Commands) == 0 {
			continue
		}
		if group.Description != "" {
			keys := presentationGroupKeys(group)
			items = append(items,
				config.Styles.ShortKey.Render(keys)+" "+
					config.Styles.ShortDesc.Render(group.Description),
			)
			continue
		}
		for _, command := range group.Commands {
			items = append(items,
				config.Styles.ShortKey.Render(command.Display)+" "+
					config.Styles.ShortDesc.Render(command.Description),
			)
		}
	}
	return strings.Join(items, config.Styles.ShortSeparator.Render(" • "))
}

func (p Presentation) FullView(config HelpConfig) string {
	var lines []string
	for _, group := range p.Groups {
		if len(group.Commands) == 0 {
			continue
		}

		groupHeader := config.Styles.FullGroup.Render(group.Title + ":")
		lines = append(lines, groupHeader)
		if group.Description != "" {
			keys := presentationGroupKeys(group)
			line := config.Styles.FullKey.Render(keys) + "  " +
				config.Styles.FullDesc.Render(group.Description)
			lines = append(lines, "  "+line)
			continue
		}

		for _, command := range group.Commands {
			allKeys := append([]string{command.Key}, command.AltKeys...)
			line := "  " +
				config.Styles.FullKey.Render(strings.Join(allKeys, ", ")) + "  " +
				config.Styles.FullDesc.Render(command.Description)
			lines = append(lines, line)
		}
	}
	return strings.Join(lines, "\n")
}

func presentationGroupKeys(group CommandGroupPresentation) string {
	keys := make([]string, 0, len(group.Commands))
	for _, command := range group.Commands {
		keys = append(keys, command.Display)
	}
	return strings.Join(keys, "|")
}

type HelpStyles struct {
	ShortKey       lipgloss.Style
	ShortDesc      lipgloss.Style
	ShortSeparator lipgloss.Style
	FullKey        lipgloss.Style
	FullDesc       lipgloss.Style
	FullSeparator  lipgloss.Style
	FullGroup      lipgloss.Style
	GroupSeparator lipgloss.Style
}

// DefaultHelpStyles returns help styles for the detected terminal background.
func DefaultHelpStyles(isDark bool) HelpStyles {
	lightDark := lipgloss.LightDark(isDark)
	return HelpStyles{
		ShortKey: lipgloss.NewStyle().
			Foreground(lipgloss.Color("#06bf88")).
			Bold(true),
		ShortDesc: lipgloss.NewStyle().
			Foreground(lightDark(lipgloss.Color("#B2B2B2"), lipgloss.Color("#626262"))),
		ShortSeparator: lipgloss.NewStyle().
			Foreground(lightDark(lipgloss.Color("#DDDADA"), lipgloss.Color("#3C3C3C"))).
			Inline(true),
		FullKey: lipgloss.NewStyle().
			Foreground(lightDark(lipgloss.Color("#087F5B"), lipgloss.Color("#5EE6B0"))).
			Bold(true),
		FullDesc: lipgloss.NewStyle().
			Foreground(lightDark(lipgloss.Color("#344054"), lipgloss.Color("#D0D5DD"))),
		FullSeparator: lipgloss.NewStyle().
			Foreground(lightDark(lipgloss.Color("#98A2B3"), lipgloss.Color("#7C8799"))),
		FullGroup: lipgloss.NewStyle().
			Foreground(lightDark(lipgloss.Color("#101828"), lipgloss.Color("#F2F4F7"))).
			Bold(true).
			Underline(true),
		GroupSeparator: lipgloss.NewStyle().
			Foreground(lightDark(lipgloss.Color("#98A2B3"), lipgloss.Color("#7C8799"))),
	}
}

type HelpConfig struct {
	Width  int
	Styles HelpStyles
}

func NewHelpConfig() HelpConfig {
	return HelpConfig{
		Width:  80,
		Styles: DefaultHelpStyles(true),
	}
}

func (c *HelpConfig) WithWidth(width int) *HelpConfig {
	c.Width = width
	return c
}

func (c *HelpConfig) WithStyles(styles HelpStyles) *HelpConfig {
	c.Styles = styles
	return c
}

type Registry[T comparable] struct {
	groups []*CommandGroup[T]
	byKey  map[string]T
}

func NewRegistry[T comparable]() *Registry[T] {
	return &Registry[T]{
		groups: make([]*CommandGroup[T], 0),
		byKey:  make(map[string]T),
	}
}

func (r *Registry[T]) NewCommandGroup(title string) *CommandGroup[T] {
	g := &CommandGroup[T]{
		Title: title,
	}
	r.groups = append(r.groups, g)
	return g
}

func (r *Registry[T]) NewUnifiedCommandGroup(title, description string) *CommandGroup[T] {
	g := &CommandGroup[T]{
		Title:       title,
		Description: description,
	}
	r.groups = append(r.groups, g)
	return g
}

func (r *Registry[T]) Build() error {
	r.byKey = make(map[string]T)
	log := slog.Default()
	log.Debug("Building registry", "groups", len(r.groups))
	for _, group := range r.groups {
		groupBindings, err := group.BuildBindings()
		if err != nil {
			return err
		}
		for key, cmd := range groupBindings {
			if _, exists := r.byKey[key]; exists {
				return fmt.Errorf("conflicting key binding across groups: %s", key)
			}
			r.byKey[key] = cmd
		}
	}
	return nil
}

func (r *Registry[T]) Dispatch(msg tea.KeyPressMsg) (T, error) {
	keyStr := msg.String()
	log := slog.Default()
	log.Debug("Dispatching key", "key", keyStr)
	log.Debug("Bindings", "bindings", r.byKey)
	if cmd, ok := r.byKey[keyStr]; ok {
		return cmd, nil
	}

	var zero T
	return zero, fmt.Errorf("no binding found for key")
}

func (r *Registry[T]) DispatchString(keyStr string) (T, error) {
	log := slog.Default()
	log.Debug("Dispatching string key", "key", keyStr)
	if cmd, ok := r.byKey[keyStr]; ok {
		return cmd, nil
	}
	var zero T
	return zero, fmt.Errorf("no binding found for key")
}

// KeysForCommand returns the primary key followed by all alternates.
func (r *Registry[T]) KeysForCommand(cmd T) []string {
	entry, ok := r.EntryForCommand(cmd)
	if !ok {
		return nil
	}
	return append([]string{entry.Key}, entry.AltKeys...)
}

// EntryForCommand returns the registry-backed palette entry for a command.
func (r *Registry[T]) EntryForCommand(cmd T) (PaletteEntry, bool) {
	for _, group := range r.groups {
		for _, binding := range group.Commands {
			if binding.Command == cmd {
				return PaletteEntry{
					Key:         binding.Key,
					AltKeys:     append([]string(nil), binding.AltKeys...),
					Display:     binding.Display,
					Description: binding.Description,
					Group:       group.Title,
					Searchable:  append([]string(nil), binding.Searchable...),
				}, true
			}
		}
	}
	return PaletteEntry{}, false
}

// Presentation returns a detached snapshot of this registry's command
// metadata. The snapshot intentionally excludes dispatch state and styles.
func (r *Registry[T]) Presentation() Presentation {
	presentation := Presentation{}
	for _, group := range r.groups {
		if len(group.Commands) == 0 {
			continue
		}

		groupPresentation := CommandGroupPresentation{
			Title:       group.Title,
			Description: group.Description,
			Commands:    make([]PaletteEntry, 0, len(group.Commands)),
		}
		for _, binding := range group.Commands {
			groupPresentation.Commands = append(
				groupPresentation.Commands,
				paletteEntry(binding, group.Title),
			)
		}
		presentation.Groups = append(presentation.Groups, groupPresentation)
	}
	presentation.PaletteEntries = r.paletteEntries()
	return presentation
}

func paletteEntry[T comparable](binding *CommandBinding[T], group string) PaletteEntry {
	return PaletteEntry{
		Key:         binding.Key,
		AltKeys:     append([]string(nil), binding.AltKeys...),
		Display:     binding.Display,
		Description: binding.Description,
		Group:       group,
		Searchable:  append([]string(nil), binding.Searchable...),
	}
}

func (r *Registry[T]) paletteEntries() []PaletteEntry {
	var entries []PaletteEntry
	seen := make(map[string]bool)

	for _, group := range r.groups {
		// Unified groups describe a single action across several keys and are
		// intentionally represented in help rather than as palette entries.
		if group.Description != "" {
			continue
		}

		for _, binding := range group.Commands {
			if seen[binding.Key] {
				continue
			}
			seen[binding.Key] = true
			entries = append(entries, paletteEntry(binding, group.Title))
		}
	}

	return entries
}
