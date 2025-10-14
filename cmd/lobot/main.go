package main

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"syscall"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/miles-w-3/lobot/internal/k8s"
	"github.com/miles-w-3/lobot/internal/ui"
)

func main() {
	if err := run(); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
}

func run() error {
	// Create context for graceful shutdown
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Handle interrupt signals
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, os.Interrupt, syscall.SIGTERM)
	go func() {
		<-sigChan
		cancel()
	}()

	// Initialize Kubernetes client
	client, err := k8s.NewClient()
	if err != nil {
		return fmt.Errorf("failed to create Kubernetes client: %w", err)
	}

	// Create informer manager
	informer, err := k8s.NewInformerManager(client)
	if err != nil {
		return fmt.Errorf("failed to create informer manager: %w", err)
	}
	defer informer.Stop()

	// Create UI model
	model := ui.NewModel(client, informer)

	// Create Bubbletea program
	p := tea.NewProgram(
		model,
		tea.WithAltScreen(),
		tea.WithMouseCellMotion(),
	)

	// Set up callback for resource updates
	informer.SetUpdateCallback(func() {
		p.Send(ui.ResourceUpdateMsg{})
	})

	// Start informers in background
	go func() {
		// Start with default resource types
		resourceTypes := k8s.DefaultResourceTypes()

		// Start informer for the first resource type (Pods)
		if len(resourceTypes) > 0 {
			if err := informer.StartInformer(ctx, resourceTypes[0]); err != nil {
				fmt.Fprintf(os.Stderr, "Failed to start informer for %s: %v\n",
					resourceTypes[0].DisplayName, err)
				return
			}
		}

		// Start remaining informers
		for i := 1; i < len(resourceTypes); i++ {
			if err := informer.StartInformer(ctx, resourceTypes[i]); err != nil {
				fmt.Fprintf(os.Stderr, "Failed to start informer for %s: %v\n",
					resourceTypes[i].DisplayName, err)
			}
		}

		// Mark model as ready and trigger initial update
		p.Send(ui.ReadyMsg{})
		p.Send(ui.ResourceUpdateMsg{})
	}()

	// Run the program
	if _, err := p.Run(); err != nil {
		return fmt.Errorf("error running program: %w", err)
	}

	return nil
}
