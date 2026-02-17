package command

import (
	"fmt"
	"log/slog"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

type PaletteEntry struct {
	Key         string   // Primary key (e.g., "ctrl+n")
	AltKeys     []string // Alternate keys
	Display     string   // Display string (e.g., "↑" for "up")
	Description string   // Human-readable description
	Group       string   // Group title for categorization
	Searchable  []string // Additional search terms
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

var DefaultHelpStyles = HelpStyles{
	ShortKey: lipgloss.NewStyle().
		Foreground(lipgloss.Color("#06bf88")). // Hardcoded ColorPrimary for now, or use ui.ColorPrimary if import cycle allows (unlikely, better command package is independent)
		Bold(true),
	ShortDesc: lipgloss.NewStyle().
		Foreground(lipgloss.AdaptiveColor{Light: "#B2B2B2", Dark: "#626262"}),
	ShortSeparator: lipgloss.NewStyle().
		Foreground(lipgloss.AdaptiveColor{Light: "#DDDADA", Dark: "#3C3C3C"}).
		Inline(true),
	FullKey: lipgloss.NewStyle().
		Foreground(lipgloss.AdaptiveColor{Light: "#909090", Dark: "#626262"}).
		Bold(true),
	FullDesc: lipgloss.NewStyle().
		Foreground(lipgloss.AdaptiveColor{Light: "#B2B2B2", Dark: "#4A4A4A"}),
	FullSeparator: lipgloss.NewStyle().
		Foreground(lipgloss.AdaptiveColor{Light: "#DDDADA", Dark: "#3C3C3C"}),
	FullGroup: lipgloss.NewStyle().
		Foreground(lipgloss.AdaptiveColor{Light: "#FFFFFF", Dark: "#FFFFFF"}).
		Bold(true).
		Underline(true),
	GroupSeparator: lipgloss.NewStyle().
		Foreground(lipgloss.AdaptiveColor{Light: "#DDDADA", Dark: "#3C3C3C"}),
}

type HelpConfig struct {
	Width  int
	Styles HelpStyles
}

func NewHelpConfig() HelpConfig {
	return HelpConfig{
		Width:  80,
		Styles: DefaultHelpStyles,
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
	groups     []*CommandGroup[T]
	byKey      map[string]T
	helpConfig HelpConfig
	shortHelp  string
	fullHelp   string
}

// RegistryInterface defines the common interface for all command registries
type RegistryInterface interface {
	ShortView() string
	FullView() string
	PaletteEntries() []PaletteEntry
}

func NewRegistry[T comparable]() *Registry[T] {
	return &Registry[T]{
		groups:     make([]*CommandGroup[T], 0),
		byKey:      make(map[string]T),
		helpConfig: NewHelpConfig(),
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

func (r *Registry[T]) Dispatch(msg tea.KeyMsg) (T, error) {
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

func (r *Registry[T]) WithConfig(cfg HelpConfig) *Registry[T] {
	r.helpConfig = cfg
	r.shortHelp = ""
	r.fullHelp = ""
	return r
}

func (r *Registry[T]) ShortView() string {
	if r.shortHelp != "" {
		return r.shortHelp
	}

	var items []string

	for _, group := range r.groups {
		if len(group.Commands) == 0 {
			continue
		}
		if group.Description != "" {
			keys := collectGroupKeys(group)
			items = append(items, r.helpConfig.Styles.ShortKey.Render(keys)+" "+r.helpConfig.Styles.ShortDesc.Render(group.Description))
		} else {
			for _, b := range group.Commands {
				items = append(items, r.helpConfig.Styles.ShortKey.Render(b.Display)+" "+r.helpConfig.Styles.ShortDesc.Render(b.Description))
			}
		}
	}

	r.shortHelp = strings.Join(items, r.helpConfig.Styles.ShortSeparator.Render(" • "))
	return r.shortHelp
}

func (r *Registry[T]) FullView() string {
	if r.fullHelp != "" {
		return r.fullHelp
	}

	var lines []string

	for _, group := range r.groups {
		if len(group.Commands) == 0 {
			continue
		}

		if group.Description != "" {
			keys := collectGroupKeys(group)
			line := r.helpConfig.Styles.FullKey.Render(keys) + "  " +
				r.helpConfig.Styles.FullDesc.Render(group.Description)
			groupHeader := r.helpConfig.Styles.FullGroup.Render(group.Title + ":")
			lines = append(lines, groupHeader)
			lines = append(lines, "  "+line)
		} else {
			groupHeader := r.helpConfig.Styles.FullGroup.Render(group.Title + ":")
			lines = append(lines, groupHeader)
			for _, b := range group.Commands {
				allKeys := append([]string{b.Key}, b.AltKeys...)
				keyStr := strings.Join(allKeys, ", ")
				line := "  " +
					r.helpConfig.Styles.FullKey.Render(keyStr) + "  " +
					r.helpConfig.Styles.FullDesc.Render(b.Description)
				lines = append(lines, line)
			}
		}
	}

	r.fullHelp = strings.Join(lines, "\n")
	return r.fullHelp
}

func collectGroupKeys[T comparable](group *CommandGroup[T]) string {
	var keys []string
	for _, b := range group.Commands {
		keys = append(keys, b.Display)
	}
	return strings.Join(keys, "|")
}

func (r *Registry[T]) PaletteEntries() []PaletteEntry {
	var entries []PaletteEntry
	seen := make(map[string]bool)

	for _, group := range r.groups {
		// Skip unified command groups (groups with a unified description)
		// These represent multiple key bindings for a single action (e.g., "navigate")
		// and should not appear as individual palette entries
		if group.Description != "" {
			continue
		}

		for _, binding := range group.Commands {
			if seen[binding.Key] {
				continue
			}
			seen[binding.Key] = true

			entries = append(entries, PaletteEntry{
				Key:         binding.Key,
				AltKeys:     binding.AltKeys,
				Display:     binding.Display,
				Description: binding.Description,
				Group:       group.Title,
				Searchable:  binding.Searchable,
			})
		}
	}

	return entries
}
