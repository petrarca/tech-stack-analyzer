package progress

import (
	"fmt"
	"os"
	"strings"
	"time"
)

// Progress manages scan progress reporting with different handlers
type Progress struct {
	enabled     bool
	handler     Handler
	withTimings bool
	traceRules  bool
	dirTimings  map[string]time.Time // Track directory/folder timing start times
	dirCount    int                  // Count of directories visited
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

func (p *Progress) ScanComplete(files, _ int, duration time.Duration) {
	p.Report(Event{
		Type:      EventScanComplete,
		FileCount: files,
		DirCount:  p.dirCount, // Use tracked directory count instead of passed value
		Duration:  duration,
	})
}

// EnterDirectory reports entering a directory (timing tracked via FolderFileProcessing events)
func (p *Progress) EnterDirectory(path string) {
	p.dirCount++
	p.Report(Event{
		Type: EventEnterDirectory,
		Path: path,
	})
}

// LeaveDirectory reports leaving a directory
func (p *Progress) LeaveDirectory(path string) {
	p.Report(Event{
		Type: EventLeaveDirectory,
		Path: path,
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
		Type: EventFileProcessingStart,
		Path: path,
		Info: info,
	})
}

func (p *Progress) FileProcessingEnd(path string) {
	p.Report(Event{
		Type: EventFileProcessingEnd,
		Path: path,
	})
}

func (p *Progress) FolderFileProcessingStart(path string) {
	if p.withTimings {
		p.dirTimings[path] = time.Now()
	}
	p.Report(Event{
		Type: EventFolderFileProcessingStart,
		Path: path,
	})
}

func (p *Progress) FolderFileProcessingEnd(path string) {
	var duration time.Duration
	if p.withTimings {
		if startTime, ok := p.dirTimings[path]; ok {
			duration = time.Since(startTime)
			delete(p.dirTimings, path)
		}
	}
	p.Report(Event{
		Type:     EventFolderFileProcessingEnd,
		Path:     path,
		Duration: duration,
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
		Info: fmt.Sprintf("GitIgnore context: %s (patterns active)", path),
	})
}

func (p *Progress) GitIgnoreLeave(path string) {
	p.Report(Event{
		Type: EventGitIgnoreLeave,
		Path: path,
		Info: fmt.Sprintf("GitIgnore context: %s (patterns removed)", path),
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
