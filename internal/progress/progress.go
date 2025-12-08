package progress

import (
	"fmt"
	"io"
	"os"
	"strings"
	"time"
)

// Event types that the scanner reports
type EventType int

const (
	EventScanStart EventType = iota
	EventScanComplete
	EventEnterDirectory
	EventLeaveDirectory
	EventComponentDetected
	EventFileProcessing
	EventSkipped
	EventProgress
	EventScanInitializing
	EventFileWriting
	EventFileWritten
	EventInfo
	EventRuleCheck
	EventRuleResult
	EventGitIgnoreEnter
	EventGitIgnoreLeave
)

// Event represents something that happened during scanning
type Event struct {
	Type      EventType
	Path      string
	Name      string
	Tech      string
	Info      string
	Reason    string
	FileCount int
	DirCount  int
	Duration  time.Duration
	Timestamp time.Time // For timing calculations
	Matched   bool      // For rule matching results
	Details   []string  // For detailed rule check information
}

// Reporter is the interface the scanner uses to report events
type Reporter interface {
	Report(event Event)
}

// Handler processes events and produces output
type Handler interface {
	Handle(event Event)
}

// Progress is the centralized verbose system
type Progress struct {
	enabled     bool
	handler     Handler
	withTimings bool
	traceRules  bool
	dirTimings  map[string]time.Time // Track directory entry times
}

// New creates a new progress reporter
func New(enabled bool, handler Handler) *Progress {
	if handler == nil {
		handler = NewSimpleHandler(os.Stderr)
	}
	return &Progress{
		enabled:     enabled,
		handler:     handler,
		withTimings: false,
		traceRules:  false,
		dirTimings:  make(map[string]time.Time),
	}
}

// EnableTimings enables timing information in progress output
func (p *Progress) EnableTimings() {
	p.withTimings = true
}

// EnableRuleTracing enables detailed rule matching information
func (p *Progress) EnableRuleTracing() {
	p.traceRules = true
}

// Report sends an event to the handler (only if enabled)
func (p *Progress) Report(event Event) {
	if !p.enabled {
		return
	}
	p.handler.Handle(event)
}

// Convenience methods for the scanner to report events

func (p *Progress) ScanStart(path string, excludePatterns []string) {
	p.Report(Event{
		Type: EventScanStart,
		Path: path,
		Info: strings.Join(excludePatterns, ", "),
	})
}

func (p *Progress) ScanComplete(files, dirs int, duration time.Duration) {
	p.Report(Event{
		Type:      EventScanComplete,
		FileCount: files,
		DirCount:  dirs,
		Duration:  duration,
	})
}

func (p *Progress) EnterDirectory(path string) {
	if p.withTimings {
		p.dirTimings[path] = time.Now()
	}
	p.Report(Event{
		Type:      EventEnterDirectory,
		Path:      path,
		Timestamp: time.Now(),
	})
}

func (p *Progress) LeaveDirectory(path string) {
	var duration time.Duration
	if p.withTimings {
		if startTime, ok := p.dirTimings[path]; ok {
			duration = time.Since(startTime)
			delete(p.dirTimings, path)
		}
	}
	p.Report(Event{
		Type:     EventLeaveDirectory,
		Path:     path,
		Duration: duration,
	})
}

func (p *Progress) ComponentDetected(name, tech, path string) {
	p.Report(Event{
		Type: EventComponentDetected,
		Name: name,
		Tech: tech,
		Path: path,
	})
}

func (p *Progress) FileProcessing(path, info string) {
	p.Report(Event{
		Type: EventFileProcessing,
		Path: path,
		Info: info,
	})
}

func (p *Progress) Skipped(path, reason string) {
	p.Report(Event{
		Type:   EventSkipped,
		Path:   path,
		Reason: reason,
	})
}

func (p *Progress) ProgressUpdate(files, dirs int) {
	p.Report(Event{
		Type:      EventProgress,
		FileCount: files,
		DirCount:  dirs,
	})
}

func (p *Progress) ScanInitializing(path string, excludePatterns []string) {
	p.Report(Event{
		Type: EventScanInitializing,
		Path: path,
		Info: strings.Join(excludePatterns, ", "),
	})
}

func (p *Progress) FileWriting(path string) {
	p.Report(Event{
		Type: EventFileWriting,
		Path: path,
	})
}

func (p *Progress) FileWritten(path string) {
	p.Report(Event{
		Type: EventFileWritten,
		Path: path,
	})
}

func (p *Progress) Info(message string) {
	p.Report(Event{
		Type: EventInfo,
		Info: message,
	})
}

func (p *Progress) GitIgnoreEnter(path string) {
	p.Report(Event{
		Type: EventGitIgnoreEnter,
		Path: path,
		Info: fmt.Sprintf("📁 GitIgnore context: %s (patterns active)", path),
	})
}

func (p *Progress) GitIgnoreLeave(path string) {
	p.Report(Event{
		Type: EventGitIgnoreLeave,
		Path: path,
		Info: fmt.Sprintf("📤 GitIgnore context: %s (patterns removed)", path),
	})
}

func (p *Progress) RuleCheck(tech string, details []string) {
	if !p.traceRules {
		return
	}
	p.Report(Event{
		Type:    EventRuleCheck,
		Tech:    tech,
		Details: details,
	})
}

func (p *Progress) RuleResult(tech string, matched bool, reason string) {
	if !p.traceRules {
		return
	}
	// Only report matches, skip non-matches to avoid noise
	if !matched {
		return
	}
	p.Report(Event{
		Type:    EventRuleResult,
		Tech:    tech,
		Matched: matched,
		Reason:  reason,
	})
}

func (p *Progress) RuleResultWithPath(tech string, matched bool, reason string, path string) {
	if !p.traceRules {
		return
	}
	// Only report matches, skip non-matches to avoid noise
	if !matched {
		return
	}
	p.Report(Event{
		Type:    EventRuleResult,
		Tech:    tech,
		Matched: matched,
		Reason:  reason,
		Path:    path,
	})
}

// SimpleHandler outputs events as simple lines (no tree)
type SimpleHandler struct {
	writer io.Writer
}

func NewSimpleHandler(writer io.Writer) *SimpleHandler {
	return &SimpleHandler{writer: writer}
}

func (h *SimpleHandler) Handle(event Event) {
	switch event.Type {
	case EventScanStart:
		fmt.Fprintf(h.writer, "[SCAN] Starting: %s\n", event.Path)
		if event.Info != "" {
			fmt.Fprintf(h.writer, "[SCAN] Excluding: %s\n", event.Info)
		}

	case EventScanComplete:
		fmt.Fprintf(h.writer, "[SCAN] Completed: %d files, %d directories in %.1fs\n",
			event.FileCount, event.DirCount, event.Duration.Seconds())

	case EventEnterDirectory:
		fmt.Fprintf(h.writer, "[DIR]  Entering: %s\n", event.Path)

	case EventLeaveDirectory:
		// Show timing if duration is set
		if event.Duration > 0 {
			fmt.Fprintf(h.writer, "[TIME] %s: %.2fs\n", event.Path, event.Duration.Seconds())
		}

	case EventComponentDetected:
		fmt.Fprintf(h.writer, "[COMP] Detected: %s (%s) at %s\n",
			event.Name, event.Tech, event.Path)

	case EventFileProcessing:
		fmt.Fprintf(h.writer, "[FILE] Parsing: %s (%s)\n", event.Path, event.Info)

	case EventSkipped:
		fmt.Fprintf(h.writer, "[SKIP] Excluding: %s (%s)\n", event.Path, event.Reason)

	case EventProgress:
		fmt.Fprintf(h.writer, "[PROG] Progress: %d files, %d directories\n",
			event.FileCount, event.DirCount)

	case EventScanInitializing:
		fmt.Fprintf(h.writer, "[INIT] Initializing scanner: %s\n", event.Path)
		if event.Info != "" {
			fmt.Fprintf(h.writer, "[INIT] Excluding: %s\n", event.Info)
		}

	case EventFileWriting:
		fmt.Fprintf(h.writer, "[OUT]  Writing results to: %s\n", event.Path)

	case EventFileWritten:
		fmt.Fprintf(h.writer, "[OUT]  Results written: %s\n", event.Path)

	case EventInfo:
		fmt.Fprintf(h.writer, "[INFO] %s\n", event.Info)

	case EventGitIgnoreEnter:
		fmt.Fprintf(h.writer, "[GIT]  %s\n", event.Info)

	case EventGitIgnoreLeave:
		fmt.Fprintf(h.writer, "[GIT]  %s\n", event.Info)

	case EventRuleCheck:
		fmt.Fprintf(h.writer, "[RULE] Checking: %s\n", event.Tech)
		for _, detail := range event.Details {
			fmt.Fprintf(h.writer, "       %s\n", detail)
		}

	case EventRuleResult:
		if event.Matched {
			if event.Path != "" {
				fmt.Fprintf(h.writer, "[RULE] ✓ MATCHED: %s - %s (in %s)\n", event.Tech, event.Reason, event.Path)
			} else {
				fmt.Fprintf(h.writer, "[RULE] ✓ MATCHED: %s - %s\n", event.Tech, event.Reason)
			}
		} else {
			fmt.Fprintf(h.writer, "[RULE] ✗ NOT MATCHED: %s - %s\n", event.Tech, event.Reason)
		}
	}
}

// TreeHandler outputs events with tree-like visualization
type TreeHandler struct {
	writer io.Writer
	depth  int
}

func NewTreeHandler(writer io.Writer) *TreeHandler {
	return &TreeHandler{
		writer: writer,
		depth:  0,
	}
}

func (h *TreeHandler) Handle(event Event) {
	indent := strings.Repeat("│  ", h.depth)
	prefix := "├─ "

	switch event.Type {
	case EventScanStart:
		fmt.Fprintf(h.writer, "Scanning %s...\n", event.Path)
		if event.Info != "" {
			fmt.Fprintf(h.writer, "Excluding: %s\n", event.Info)
		}
		fmt.Fprintln(h.writer)

	case EventScanComplete:
		fmt.Fprintf(h.writer, "└─ Completed: %d files, %d directories in %.1fs\n",
			event.FileCount, event.DirCount, event.Duration.Seconds())

	case EventEnterDirectory:
		fmt.Fprintf(h.writer, "%s%s%s\n", indent, prefix, event.Path)
		h.depth++

	case EventLeaveDirectory:
		h.depth--
		if h.depth < 0 {
			h.depth = 0
		}
		// Show timing if duration is set
		if event.Duration > 0 {
			indent := strings.Repeat("│  ", h.depth)
			fmt.Fprintf(h.writer, "%s└─ ⏱  %.2fs\n", indent, event.Duration.Seconds())
		}

	case EventComponentDetected:
		fmt.Fprintf(h.writer, "%s%sDetected: %s (%s)\n",
			indent, prefix, event.Name, event.Tech)

	case EventFileProcessing:
		fmt.Fprintf(h.writer, "%s%sParsing: %s (%s)\n",
			indent, prefix, event.Path, event.Info)

	case EventSkipped:
		fmt.Fprintf(h.writer, "%s%sSkipping: %s (%s)\n",
			indent, prefix, event.Path, event.Reason)

	case EventProgress:
		fmt.Fprintf(h.writer, "%s%sProgress: %d files, %d directories\n",
			indent, prefix, event.FileCount, event.DirCount)

	case EventScanInitializing:
		fmt.Fprintf(h.writer, "%s%sInitializing: %s\n", indent, prefix, event.Path)
		if event.Info != "" {
			fmt.Fprintf(h.writer, "%s%sExcluding: %s\n", indent, prefix, event.Info)
		}

	case EventFileWriting:
		fmt.Fprintf(h.writer, "%s%sWriting results to: %s\n", indent, prefix, event.Path)

	case EventFileWritten:
		fmt.Fprintf(h.writer, "%s%sResults written: %s\n", indent, prefix, event.Path)

	case EventInfo:
		fmt.Fprintf(h.writer, "%s%s%s\n", indent, prefix, event.Info)

	case EventGitIgnoreEnter:
		fmt.Fprintf(h.writer, "%s%s%s\n", indent, prefix, event.Info)

	case EventGitIgnoreLeave:
		fmt.Fprintf(h.writer, "%s%s%s\n", indent, prefix, event.Info)

	case EventRuleCheck:
		fmt.Fprintf(h.writer, "%s%sChecking rule: %s\n", indent, prefix, event.Tech)
		for _, detail := range event.Details {
			fmt.Fprintf(h.writer, "%s│  %s\n", indent, detail)
		}

	case EventRuleResult:
		if event.Matched {
			if event.Path != "" {
				fmt.Fprintf(h.writer, "%s└─ ✓ MATCHED: %s - %s (in %s)\n", indent, event.Tech, event.Reason, event.Path)
			} else {
				fmt.Fprintf(h.writer, "%s└─ ✓ MATCHED: %s - %s\n", indent, event.Tech, event.Reason)
			}
		} else {
			fmt.Fprintf(h.writer, "%s└─ ✗ NOT MATCHED: %s - %s\n", indent, event.Tech, event.Reason)
		}
	}
}

// NullHandler discards all events (for disabled verbose mode)
type NullHandler struct{}

func NewNullHandler() *NullHandler {
	return &NullHandler{}
}

func (h *NullHandler) Handle(event Event) {
	// Do nothing
}
