package logging

import (
	"os"
	"path/filepath"
	"sync"
	"testing"
)

func TestKlogWriterWritesAndCloses(t *testing.T) {
	path := filepath.Join(t.TempDir(), "klog.log")
	writer, err := NewKlogWriter(path)
	if err != nil {
		t.Fatalf("NewKlogWriter() error = %v", err)
	}

	want := []byte("warning: request throttled\n")
	if got, err := writer.Write(want); err != nil {
		t.Fatalf("Write() error = %v", err)
	} else if got != len(want) {
		t.Fatalf("Write() bytes = %d, want %d", got, len(want))
	}

	if err := writer.Close(); err != nil {
		t.Fatalf("Close() error = %v", err)
	}

	got, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("ReadFile() error = %v", err)
	}
	if string(got) != string(want) {
		t.Fatalf("file contents = %q, want %q", got, want)
	}
}

func TestKlogWriterSerializesWrites(t *testing.T) {
	path := filepath.Join(t.TempDir(), "klog.log")
	writer, err := NewKlogWriter(path)
	if err != nil {
		t.Fatalf("NewKlogWriter() error = %v", err)
	}
	defer writer.Close()

	const writers = 8
	const writesPerWriter = 20
	var wg sync.WaitGroup
	for i := 0; i < writers; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for j := 0; j < writesPerWriter; j++ {
				if _, err := writer.Write([]byte("entry\n")); err != nil {
					t.Errorf("Write() error = %v", err)
					return
				}
			}
		}()
	}
	wg.Wait()
}
