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
	EventFileProcessingStart
	EventFileProcessingEnd
	EventFolderFileProcessingStart
	EventFolderFileProcessingEnd
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

// Progress manages scan progress reporting with different handlers
type Progress struct {
	enabled     bool
	handler     Handler
	withTimings bool
	traceRules  bool
	dirTimings  map[string]time.Time // Track directory/folder timing start times
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

// EnterDirectory reports entering a directory (timing tracked via FolderFileProcessing events)
func (p *Progress) EnterDirectory(path string) {
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

// SimpleHandler outputs events as simple lines (no tree)
type SimpleHandler struct {
	writer    io.Writer
	timings   []TimingEntry // Track all timings for summary
	rules     []RuleEntry   // Track all rule matches for summary
	scanStart time.Time     // Track overall scan start time
}

func NewSimpleHandler(writer io.Writer) *SimpleHandler {
	return &SimpleHandler{
		writer:  writer,
		timings: make([]TimingEntry, 0),
		rules:   make([]RuleEntry, 0),
	}
}

func (h *SimpleHandler) Handle(event Event) {
	switch event.Type {
	case EventScanStart:
		h.scanStart = time.Now()
		fmt.Fprintf(h.writer, "[SCAN] Starting: %s\n", event.Path)
		if event.Info != "" {
			fmt.Fprintf(h.writer, "[SCAN] Excluding: %s\n", event.Info)
		}

	case EventScanComplete:
		totalScanTime := time.Since(h.scanStart)
		fmt.Fprintf(h.writer, "[SCAN] Completed: %d files, %d directories in %.1fs\n",
			event.FileCount, event.DirCount, event.Duration.Seconds())

		// Print concise summaries for verbose mode
		h.printConciseTimingSummary(totalScanTime)
		h.printConciseRuleSummary()

	case EventEnterDirectory:
		fmt.Fprintf(h.writer, "[DIR]  Entering: %s\n", event.Path)

	case EventLeaveDirectory:
		// Show timing if duration is set and track it
		if event.Duration > 0 {
			h.timings = append(h.timings, TimingEntry{
				Path:     event.Path,
				Duration: event.Duration,
				Depth:    0,
			})
			seconds := event.Duration.Seconds()
			fmt.Fprintf(h.writer, "[TIME] %s: %s %.2fs\n", event.Path, getTimingIcon(seconds), seconds)
		}

	case EventComponentDetected:
		fmt.Fprintf(h.writer, "[COMP] Detected: %s (%s) at %s\n",
			event.Name, event.Tech, event.Path)

	case EventFileProcessingStart:
		fmt.Fprintf(h.writer, "[FILE] Processing: %s (%s)\n", event.Path, event.Info)

	case EventFileProcessingEnd:
		// File processing completed - no output needed for timing

	case EventFolderFileProcessingStart:
		// Start timing for folder file processing
		fmt.Fprintf(h.writer, "[FOLDER] Starting file processing: %s\n", event.Path)

	case EventFolderFileProcessingEnd:
		// Track timing for individual folder file processing
		if event.Duration > 0 {
			h.timings = append(h.timings, TimingEntry{
				Path:     event.Path,
				Duration: event.Duration,
				Depth:    0,
			})
			seconds := event.Duration.Seconds()
			fmt.Fprintf(h.writer, "[FOLDER] %s: %s %.2fs\n", event.Path, getTimingIcon(seconds), seconds)
		}

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
		// Track rule matches for summary
		h.rules = append(h.rules, RuleEntry{
			Tech:    event.Tech,
			Reason:  event.Reason,
			Path:    event.Path,
			Matched: event.Matched,
		})

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
	writer    io.Writer
	depth     int
	timings   []TimingEntry // Track all timings for summary
	rules     []RuleEntry   // Track all rule matches for summary
	scanStart time.Time     // Track overall scan start time
}

// TimingEntry represents a directory timing for analysis
type TimingEntry struct {
	Path     string
	Duration time.Duration
	Depth    int
}

// RuleEntry represents a rule match for analysis
type RuleEntry struct {
	Tech    string
	Reason  string
	Path    string
	Matched bool
}

// getTimingIcon returns the appropriate icon for a duration
func getTimingIcon(seconds float64) string {
	if seconds >= 10.0 {
		return "🔴" // Slow
	} else if seconds >= 1.0 {
		return "🟡" // Medium
	}
	return "🟢" // Fast
}

// shortenPath shortens a path for display if it's too long
func shortenPath(path string, maxLen int) string {
	if len(path) <= maxLen {
		return path
	}
	parts := strings.Split(path, "/")
	if len(parts) > 3 {
		return "..." + "/" + strings.Join(parts[len(parts)-2:], "/")
	}
	return path
}

// sortTimingsByDuration sorts timings by duration descending (partial sort for top N)
func sortTimingsByDuration(timings []TimingEntry, topN int) []TimingEntry {
	sorted := make([]TimingEntry, len(timings))
	copy(sorted, timings)
	for i := 0; i < len(sorted)-1 && i < topN; i++ {
		for j := 0; j < len(sorted)-i-1; j++ {
			if sorted[j].Duration < sorted[j+1].Duration {
				sorted[j], sorted[j+1] = sorted[j+1], sorted[j]
			}
		}
	}
	return sorted
}

func NewTreeHandler(writer io.Writer) *TreeHandler {
	return &TreeHandler{
		writer:  writer,
		depth:   0,
		timings: make([]TimingEntry, 0),
		rules:   make([]RuleEntry, 0),
	}
}

func (h *TreeHandler) Handle(event Event) {
	indent := strings.Repeat("│  ", h.depth)
	prefix := "├─ "

	switch event.Type {
	case EventScanStart:
		h.scanStart = time.Now()
		fmt.Fprintf(h.writer, "Scanning %s...\n", event.Path)
		if event.Info != "" {
			fmt.Fprintf(h.writer, "Excluding: %s\n", event.Info)
		}
		fmt.Fprintln(h.writer)

	case EventScanComplete:
		fmt.Fprintf(h.writer, "└─ Completed: %d files, %d directories in %.1fs\n",
			event.FileCount, event.DirCount, event.Duration.Seconds())

		// Print machine-readable CSV data for debug mode
		h.printMachineReadableTimingData()
		h.printMachineReadableRuleData()

	case EventEnterDirectory:
		fmt.Fprintf(h.writer, "%s%s%s\n", indent, prefix, event.Path)
		h.depth++

	case EventComponentDetected:
		fmt.Fprintf(h.writer, "%s%sDetected: %s (%s)\n", indent, prefix, event.Name, event.Tech)

	case EventLeaveDirectory:
		h.depth--
		if h.depth < 0 {
			h.depth = 0
		}
		// Show timing if duration is set and track it
		if event.Duration > 0 {
			indent := strings.Repeat("│  ", h.depth)

			// Track timing for summary - use cumulative time if available
			duration := event.Duration
			// For now, use the event duration since cumulative timing needs progress system access
			h.timings = append(h.timings, TimingEntry{
				Path:     event.Path,
				Duration: duration,
				Depth:    h.depth,
			})

			seconds := duration.Seconds()
			fmt.Fprintf(h.writer, "%s└─ %s ⏱  %.2fs\n", indent, getTimingIcon(seconds), seconds)
		}

	case EventProgress:
		fmt.Fprintf(h.writer, "%s%sProgress: %d files, %d directories\n",
			indent, prefix, event.FileCount, event.DirCount)

	case EventFolderFileProcessingStart:
		// Start timing for folder file processing (TreeHandler)
		fmt.Fprintf(h.writer, "%s%sProcessing files in: %s\n", indent, prefix, event.Path)

	case EventFolderFileProcessingEnd:
		// Track timing for individual folder file processing (TreeHandler)
		if event.Duration > 0 {
			h.timings = append(h.timings, TimingEntry{
				Path:     event.Path,
				Duration: event.Duration,
				Depth:    h.depth,
			})
			seconds := event.Duration.Seconds()
			fmt.Fprintf(h.writer, "%s└─ %s 📁 %.2fs\n", indent, getTimingIcon(seconds), seconds)
		}

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
		// Track rule matches for CSV output
		h.rules = append(h.rules, RuleEntry{
			Tech:    event.Tech,
			Reason:  event.Reason,
			Path:    event.Path,
			Matched: event.Matched,
		})

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

// printConciseTimingSummary provides human-readable timing summary for SimpleHandler
func (h *SimpleHandler) printConciseTimingSummary(totalScanTime time.Duration) {
	if len(h.timings) == 0 {
		return
	}

	// Calculate statistics
	var totalDirTime time.Duration
	slowCount := 0
	var slowest TimingEntry

	for _, timing := range h.timings {
		totalDirTime += timing.Duration
		seconds := timing.Duration.Seconds()
		if seconds >= 10.0 {
			slowCount++
		}
		if timing.Duration > slowest.Duration {
			slowest = timing
		}
	}

	avgTime := totalDirTime.Seconds() / float64(len(h.timings))

	fmt.Fprintf(h.writer, "\n📊 TIMING SUMMARY\n")
	fmt.Fprintf(h.writer, "   • Total directories: %d\n", len(h.timings))
	fmt.Fprintf(h.writer, "   • Average per directory: %.3fs\n", avgTime)

	if slowCount > 0 {
		fmt.Fprintf(h.writer, "   • ⚠️  Slow directories (≥10s): %d\n", slowCount)
	} else {
		fmt.Fprintf(h.writer, "   • ✅ All directories processed quickly\n")
	}

	if slowest.Duration > 0 {
		// Shorten path for display
		displayPath := slowest.Path
		if len(displayPath) > 50 {
			parts := strings.Split(displayPath, "/")
			if len(parts) > 2 {
				displayPath = ".../" + strings.Join(parts[len(parts)-2:], "/")
			}
		}
		fmt.Fprintf(h.writer, "   • Slowest: %s (%.2fs)\n", displayPath, slowest.Duration.Seconds())
	}

	fmt.Fprintln(h.writer)
}

// printConciseRuleSummary provides human-readable rule summary for SimpleHandler
func (h *SimpleHandler) printConciseRuleSummary() {
	if len(h.rules) == 0 {
		return
	}

	matchedCount := 0
	techCount := make(map[string]bool)

	for _, rule := range h.rules {
		if rule.Matched {
			matchedCount++
			techCount[rule.Tech] = true
		}
	}

	fmt.Fprintf(h.writer, "\n🔍 RULE SUMMARY\n")
	fmt.Fprintf(h.writer, "   • Total rules checked: %d\n", len(h.rules))
	fmt.Fprintf(h.writer, "   • Technologies matched: %d\n", len(techCount))
	fmt.Fprintf(h.writer, "   • Successful matches: %d\n", matchedCount)

	if matchedCount > 0 {
		fmt.Fprintf(h.writer, "   • ✅ Detection successful\n")
	} else {
		fmt.Fprintf(h.writer, "   • ⚠️  No technologies detected\n")
	}

	fmt.Fprintln(h.writer)
}

// printMachineReadableTimingData outputs top 10 slowest directories for TreeHandler
func (h *TreeHandler) printMachineReadableTimingData() {
	if len(h.timings) == 0 {
		return
	}

	sortedTimings := sortTimingsByDuration(h.timings, 10)

	fmt.Fprintln(h.writer)
	fmt.Fprintf(h.writer, "🐌 TOP 10 SLOWEST DIRECTORIES\n")
	fmt.Fprintf(h.writer, "═══════════════════════════════════════\n")

	maxShow := len(sortedTimings)
	if maxShow > 10 {
		maxShow = 10
	}

	for i := 0; i < maxShow; i++ {
		timing := sortedTimings[i]
		seconds := timing.Duration.Seconds()
		fmt.Fprintf(h.writer, " %s %2d. %-45s %6.2fs\n", getTimingIcon(seconds), i+1, shortenPath(timing.Path, 60), seconds)
	}

	fmt.Fprintln(h.writer)
}

// printMachineReadableRuleData outputs rule summary for TreeHandler
func (h *TreeHandler) printMachineReadableRuleData() {
	if len(h.rules) == 0 {
		return
	}

	// Count matches and group by technology
	matchedCount := 0
	techMatches := make(map[string]int)

	for _, rule := range h.rules {
		if rule.Matched {
			matchedCount++
			techMatches[rule.Tech]++
		}
	}

	fmt.Fprintf(h.writer, "🔍 RULE ANALYSIS\n")
	fmt.Fprintf(h.writer, "═══════════════════════════════════════\n")
	fmt.Fprintf(h.writer, " Total rules checked: %d\n", len(h.rules))
	fmt.Fprintf(h.writer, " Successful matches: %d\n", matchedCount)
	fmt.Fprintf(h.writer, " Technologies detected: %d\n", len(techMatches))

	if len(techMatches) > 0 {
		fmt.Fprintln(h.writer)
		fmt.Fprintf(h.writer, " Detected technologies:\n")
		for tech, count := range techMatches {
			fmt.Fprintf(h.writer, "   • %s (%d matches)\n", tech, count)
		}
	}

	fmt.Fprintln(h.writer)
}
