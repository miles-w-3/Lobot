package main

import (
	"context"
	"flag"
	"fmt"
	"log/slog"
	"os"
	"os/signal"
	"syscall"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/miles-w-3/lobot/internal/k8s"
	"github.com/miles-w-3/lobot/internal/ui"
	"k8s.io/klog/v2"
)

func main() {
	// Initialize slog to write to out.log (overwrites on each run)
	logFile, err := os.Create("out.log")
	if err != nil {
		fmt.Fprintf(os.Stderr, "Failed to create log file: %v\n", err)
		os.Exit(1)
	}
	defer logFile.Close()

	logger := slog.New(slog.NewTextHandler(logFile, &slog.HandlerOptions{
		Level: slog.LevelDebug,
	}))
	slog.SetDefault(logger)

	slog.Info("Lobot starting")

	if err := run(); err != nil {
		slog.Error("Application error", "error", err)
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}

	slog.Info("Lobot exiting")
}

func run() error {
	logger := slog.Default()

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

	// Create error tracker for logging errors to error.log
	errorTracker, err := ui.NewErrorTracker()
	if err != nil {
		return fmt.Errorf("failed to create error tracker: %w", err)
	}
	defer errorTracker.Close()

	// Redirect klog (client-go) output to our error tracker
	// We must also disable logging to stderr which klog does by default
	// This prevents "Throttling request" messages from breaking the TUI
	klog.SetOutput(errorTracker)

	// Configure klog flags to avoid writing to stderr
	// We need to use the flag package to set these as klog uses flags
	// Note: We ignore errors here as flags might be already parsed/set
	// Set stderrthreshold to FATAL so only fatal errors go to stderr (if any)
	// Set logtostderr to false
	// We use direct flag manipulation for klog
	// Make sure to import "flag"
	fs := flag.NewFlagSet("", flag.ContinueOnError)
	klog.InitFlags(fs)
	fs.Set("logtostderr", "false")
	fs.Set("stderrthreshold", "FATAL") // Only FATAL logs go to stderr
	fs.Set("alsologtostderr", "false")

	// Initialize Kubernetes client
	client, err := k8s.NewClient(logger)
	if err != nil {
		return fmt.Errorf("failed to create Kubernetes client: %w", err)
	}

	// Create ResourceService
	resourceService, err := k8s.NewResourceService(ctx, client, logger)
	if err != nil {
		return fmt.Errorf("failed to create resource service: %w", err)
	}
	defer resourceService.Close()

	// Create UI model
	model := ui.NewModel(resourceService, logger, errorTracker)

	// Create Bubbletea program
	p := tea.NewProgram(
		model,
		tea.WithAltScreen(),
		tea.WithMouseCellMotion(),
	)

	// Forward messages into the model and UI
	processUpdateCallback := func(update k8s.ServiceUpdate) {
		logger.Debug("Processing update callback", "type", update.Type, "context", update.Context)
		switch update.Type {
		case k8s.ServiceUpdateResources:
			p.Send(ui.ResourceUpdateMsg{})
		case k8s.ServiceUpdateReady:
			logger.Info("System ready", "context", update.Context)
			p.Send(ui.ReadyMsg{})
		case k8s.ServiceUpdateError:
			logger.Error("Resource service error", "error", update.Error)
			// Send error to UI instead of just logging
			p.Send(ui.ErrorMsg{Error: update.Error})
		}
	}

	// Start resource service initialization in background after program starts
	// This ensures the UI is running and can receive/display errors
	go func() {
		resourceService.FinalizeConfiguration(processUpdateCallback)
	}()

	// Run the program
	if _, err := p.Run(); err != nil {
		return fmt.Errorf("error running program: %w", err)
	}

	return nil
}
