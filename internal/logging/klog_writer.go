// Package logging contains adapters for third-party logging streams.
package logging

import (
	"fmt"
	"os"
	"sync"
)

// KlogWriter persists client-go/klog output without coupling it to the TUI.
// It intentionally does not classify or count messages; the file is the
// complete source of truth for that stream.
type KlogWriter struct {
	file *os.File
	mu   sync.Mutex
}

// NewKlogWriter creates or truncates the log at path.
func NewKlogWriter(path string) (*KlogWriter, error) {
	file, err := os.Create(path)
	if err != nil {
		return nil, fmt.Errorf("failed to create klog output: %w", err)
	}
	return &KlogWriter{file: file}, nil
}

// Write implements io.Writer for klog.
func (w *KlogWriter) Write(p []byte) (int, error) {
	w.mu.Lock()
	defer w.mu.Unlock()

	if w.file == nil {
		return 0, os.ErrInvalid
	}
	return w.file.Write(p)
}

// Close closes the underlying log file.
func (w *KlogWriter) Close() error {
	w.mu.Lock()
	defer w.mu.Unlock()

	if w.file == nil {
		return nil
	}
	err := w.file.Close()
	w.file = nil
	return err
}
