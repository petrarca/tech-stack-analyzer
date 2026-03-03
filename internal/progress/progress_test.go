package progress

import (
	"bytes"
	"strings"
	"testing"
	"time"
)

func TestSimpleHandler(t *testing.T) {
	tests := []struct {
		name     string
		event    Event
		expected string
	}{
		{
			name: "scan start",
			event: Event{
				Type: EventScanStart,
				Path: "/path/to/project",
				Info: "node_modules, vendor",
			},
			expected: "[SCAN] Starting: /path/to/project\n[SCAN] Excluding: node_modules, vendor\n",
		},
		{
			name: "enter directory",
			event: Event{
				Type: EventEnterDirectory,
				Path: "/backend",
			},
			expected: "[DIR]  Entering: /backend\n",
		},
		{
			name: "component detected",
			event: Event{
				Type: EventComponentDetected,
				Name: "backend",
				Tech: "nodejs",
				Path: "/backend",
			},
			expected: "[COMP] Detected: backend (nodejs) at /backend\n",
		},
		{
			name: "file processing",
			event: Event{
				Type: EventFileProcessingStart,
				Path: "/package.json",
				Info: "15 dependencies",
			},
			expected: "[FILE] Processing: /package.json (15 dependencies)\n",
		},
		{
			name: "skipped",
			event: Event{
				Type:   EventSkipped,
				Path:   "/node_modules",
				Reason: "excluded",
			},
			expected: "[SKIP] Excluding: /node_modules (excluded)\n",
		},
		{
			name: "progress",
			event: Event{
				Type:      EventProgress,
				FileCount: 500,
				DirCount:  45,
			},
			expected: "[PROG] Progress: 500 files, 45 directories\n",
		},
		{
			name: "scan complete",
			event: Event{
				Type:      EventScanComplete,
				FileCount: 3247,
				DirCount:  412,
				Duration:  2345 * time.Millisecond,
			},
			expected: "[SCAN] Completed: 3247 files, 412 directories in 2.3s (722.2ms per 1000 files)\n",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			buf := &bytes.Buffer{}
			handler := NewSimpleHandler(buf)
			handler.Handle(tt.event)

			if buf.String() != tt.expected {
				t.Errorf("Expected:\n%s\nGot:\n%s", tt.expected, buf.String())
			}
		})
	}
}

func TestTreeHandler(t *testing.T) {
	buf := &bytes.Buffer{}
	handler := NewTreeHandler(buf)

	// Simulate a scan sequence
	handler.Handle(Event{Type: EventScanStart, Path: "/project"})
	handler.Handle(Event{Type: EventEnterDirectory, Path: "/"})
	handler.Handle(Event{Type: EventComponentDetected, Name: "main", Tech: "nodejs", Path: "/"})
	handler.Handle(Event{Type: EventEnterDirectory, Path: "/backend"})
	handler.Handle(Event{Type: EventComponentDetected, Name: "backend", Tech: "nodejs", Path: "/backend"})
	handler.Handle(Event{Type: EventLeaveDirectory, Path: "/backend"})
	handler.Handle(Event{Type: EventLeaveDirectory, Path: "/"})
	handler.Handle(Event{Type: EventScanComplete, FileCount: 100, DirCount: 10, Duration: time.Second})

	output := buf.String()

	// Check that output contains expected elements
	expectedParts := []string{
		"Scanning /project",
		"├─ /",
		"├─ Detected: main (nodejs)",
		"│  ├─ /backend",
		"│  ├─ Detected: backend (nodejs)",
		"└─ Completed: 100 files, 10 directories",
	}

	for _, part := range expectedParts {
		if !strings.Contains(output, part) {
			t.Errorf("Expected output to contain: %s\nGot:\n%s", part, output)
		}
	}
}

func TestProgressReporter(t *testing.T) {
	t.Run("enabled reporter calls handler", func(t *testing.T) {
		buf := &bytes.Buffer{}
		handler := NewSimpleHandler(buf)
		progress := New(true, handler)

		progress.EnterDirectory("/test")

		if buf.Len() == 0 {
			t.Error("Expected handler to be called when enabled")
		}
	})

	t.Run("disabled reporter does not call handler", func(t *testing.T) {
		buf := &bytes.Buffer{}
		handler := NewSimpleHandler(buf)
		progress := New(false, handler)

		progress.EnterDirectory("/test")

		if buf.Len() > 0 {
			t.Error("Expected handler not to be called when disabled")
		}
	})
}

func TestConvenienceMethods(t *testing.T) {
	buf := &bytes.Buffer{}
	handler := NewSimpleHandler(buf)
	progress := New(true, handler)

	progress.ScanStart("/project", []string{"node_modules", "vendor"})
	progress.EnterDirectory("/backend")
	progress.ComponentDetected("backend", "nodejs", "/backend")
	progress.FileProcessing("/package.json", "15 deps")
	progress.Skipped("/node_modules", "excluded")
	progress.ProgressUpdate(500, 45)
	progress.ScanComplete(3247, 412, 2*time.Second)

	output := buf.String()

	// Verify all methods produced output
	expectedLines := 8 // scan start (2 lines) + 6 other events
	actualLines := strings.Count(output, "\n")

	if actualLines != expectedLines {
		t.Errorf("Expected %d lines, got %d\nOutput:\n%s", expectedLines, actualLines, output)
	}
}

func BenchmarkSimpleHandler(b *testing.B) {
	buf := &bytes.Buffer{}
	handler := NewSimpleHandler(buf)
	event := Event{
		Type: EventEnterDirectory,
		Path: "/some/path",
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		handler.Handle(event)
	}
}

func BenchmarkProgressReporter(b *testing.B) {
	buf := &bytes.Buffer{}
	handler := NewSimpleHandler(buf)
	progress := New(true, handler)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		progress.EnterDirectory("/some/path")
	}
}

func BenchmarkProgressReporterDisabled(b *testing.B) {
	buf := &bytes.Buffer{}
	handler := NewSimpleHandler(buf)
	progress := New(false, handler)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		progress.EnterDirectory("/some/path")
	}
}
