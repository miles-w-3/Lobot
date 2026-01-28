package command

import (
	"fmt"
	"log/slog"
	"strings"
)

type CommandGroup[T comparable] struct {
	Title       string
	Description string
	Commands    []*CommandBinding[T] // Keep for iteration during Build
}

func (g *CommandGroup[T]) Add(key string, cmd T) *CommandBinding[T] {
	display := key
	switch key {
	case "up":
		display = "↑"
	case "down":
		display = "↓"
	case "left":
		display = "←"
	case "right":
		display = "→"
	}

	if strings.HasPrefix(key, "shift+") && len(key) == 7 {
		display = strings.ToUpper(key[6:])
	}
	binding := &CommandBinding[T]{
		Key:         key,
		Display:     display,
		Command:     cmd,
		Group:       g,
		Description: "",
	}
	log := slog.Default()
	log.Debug("Adding command", "group", g.Title, "key", key, "cmd", cmd)
	g.Commands = append(g.Commands, binding)
	return binding
}

// BuildBindings returns the key→command mapping for this group.
// Returns an error if there are duplicate keys within the group.
func (g *CommandGroup[T]) BuildBindings() (map[string]T, error) {
	bindings := make(map[string]T)
	for _, cmd := range g.Commands {
		if _, exists := bindings[cmd.Key]; exists {
			return nil, fmt.Errorf("duplicate key in group %q: %s", g.Title, cmd.Key)
		}
		bindings[cmd.Key] = cmd.Command
		for _, alt := range cmd.AltKeys {
			if _, exists := bindings[alt]; exists {
				return nil, fmt.Errorf("duplicate alt key in group %q: %s", g.Title, alt)
			}
			bindings[alt] = cmd.Command
		}
	}
	return bindings, nil
}
