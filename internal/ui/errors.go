package ui

import (
	"fmt"
	"log/slog"
	"os"
	"strings"
	"sync"
	"time"
)

// ErrorTracker tracks errors and writes them to a log file
// It provides a way to monitor errors without cluttering the TUI
type ErrorTracker struct {
	hasErrors  bool
	errorCount int
	logFile    *os.File
	logger     *slog.Logger
	mu         sync.RWMutex
}

// NewErrorTracker creates a new error tracker that writes to error.log
func NewErrorTracker() (*ErrorTracker, error) {
	logFile, err := os.Create("error.log")
	if err != nil {
		return nil, fmt.Errorf("failed to create error.log: %w", err)
	}

	// Create a logger that writes to the error log file
	logger := slog.New(slog.NewTextHandler(logFile, &slog.HandlerOptions{
		Level: slog.LevelWarn,
	}))

	return &ErrorTracker{
		hasErrors:  false,
		errorCount: 0,
		logFile:    logFile,
		logger:     logger,
	}, nil
}

// LogError logs an error to the error.log file and increments the counter
func (e *ErrorTracker) LogError(source, message string, args ...any) {
	e.mu.Lock()
	defer e.mu.Unlock()

	e.hasErrors = true
	e.errorCount++

	// Build args slice with source
	logArgs := append([]any{"source", source, "time", time.Now().Format(time.RFC3339)}, args...)
	e.logger.Error(message, logArgs...)
}

// LogWarning logs a warning to the error.log file (doesn't increment error count)
func (e *ErrorTracker) LogWarning(source, message string, args ...any) {
	e.mu.Lock()
	defer e.mu.Unlock()

	// Warnings are logged but don't set hasErrors flag
	logArgs := append([]any{"source", source, "time", time.Now().Format(time.RFC3339)}, args...)
	e.logger.Warn(message, logArgs...)
}

// HasErrors returns true if any errors have been logged
func (e *ErrorTracker) HasErrors() bool {
	e.mu.RLock()
	defer e.mu.RUnlock()
	return e.hasErrors
}

// GetErrorCount returns the number of errors logged
func (e *ErrorTracker) GetErrorCount() int {
	e.mu.RLock()
	defer e.mu.RUnlock()
	return e.errorCount
}

// ClearErrors resets the error state (useful when user acknowledges errors)
func (e *ErrorTracker) ClearErrors() {
	e.mu.Lock()
	defer e.mu.Unlock()
	e.hasErrors = false
	e.errorCount = 0
}

// Close closes the error log file
func (e *ErrorTracker) Close() error {
	if e.logFile != nil {
		return e.logFile.Close()
	}
	return nil
}

// Write implements io.Writer interface for klog redirection
// This allows client-go's klog output to be captured in error.log
func (e *ErrorTracker) Write(p []byte) (n int, err error) {
	e.mu.Lock()
	defer e.mu.Unlock()

	// Check if this looks like a warning or error (klog format)
	msg := string(p)
	if strings.Contains(msg, "Warning") || strings.Contains(msg, "Error") || strings.Contains(msg, "error") {
		e.hasErrors = true
		e.errorCount++
	}

	// Write to the log file
	return e.logFile.Write(p)
}
