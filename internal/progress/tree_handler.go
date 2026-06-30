package progress

import (
	"fmt"
	"io"
	"strings"
	"time"
)

// TreeHandler outputs events with tree-like visualization
type TreeHandler struct {
	writer    io.Writer
	depth     int
	timings   []TimingEntry // Track all timings for summary
	rules     []RuleEntry   // Track all rule matches for summary
	scanStart time.Time     // Track overall scan start time
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
		h.handleScanStart(event)
	case EventScanComplete:
		h.handleScanComplete(event)
	case EventEnterDirectory:
		fmt.Fprintf(h.writer, "%s%s%s\n", indent, prefix, event.Path)
		h.depth++
	case EventComponentDetected:
		fmt.Fprintf(h.writer, "%s%sDetected: %s (%s)\n", indent, prefix, event.Name, event.Tech)
	case EventLeaveDirectory:
		h.handleLeaveDirectory(event)
	case EventProgress:
		fmt.Fprintf(h.writer, "%s%sProgress: %d files, %d directories\n", indent, prefix, event.FileCount, event.DirCount)
	case EventFolderFileProcessingStart:
		fmt.Fprintf(h.writer, "%s%sProcessing files in: %s\n", indent, prefix, event.Path)
	case EventFolderFileProcessingEnd:
		h.handleFolderFileProcessingEnd(event, indent)
	case EventScanInitializing:
		h.handleScanInitializing(event, indent, prefix)
	case EventFileWriting:
		fmt.Fprintf(h.writer, "%s%sWriting results to: %s\n", indent, prefix, event.Path)
	case EventFileWritten:
		fmt.Fprintf(h.writer, "%s%sResults written: %s\n", indent, prefix, event.Path)
	case EventInfo, EventGitIgnoreEnter, EventGitIgnoreLeave:
		fmt.Fprintf(h.writer, "%s%s%s\n", indent, prefix, event.Info)
	case EventRuleCheck:
		h.handleRuleCheck(event, indent, prefix)
	case EventRuleResult:
		h.handleRuleResult(event, indent)
	}
}

func (h *TreeHandler) handleScanStart(event Event) {
	h.scanStart = time.Now()
	fmt.Fprintf(h.writer, "Scanning %s...\n", event.Path)
	if event.Info != "" {
		fmt.Fprintf(h.writer, "Excluding: %s\n", event.Info)
	}
	fmt.Fprintln(h.writer)
}

func (h *TreeHandler) handleScanComplete(event Event) {
	msPerKFiles := 0.0
	if event.FileCount > 0 {
		msPerKFiles = (event.Duration.Seconds() * 1000) / (float64(event.FileCount) / 1000)
	}
	fmt.Fprintf(h.writer, "└─ Completed: %d files, %d directories in %.1fs (%.1fms per 1000 files)\n",
		event.FileCount, event.DirCount, event.Duration.Seconds(), msPerKFiles)
	// Print machine-readable CSV data for debug mode.
	h.printMachineReadableTimingData()
	h.printMachineReadableRuleData()
}

func (h *TreeHandler) handleLeaveDirectory(event Event) {
	h.depth--
	if h.depth < 0 {
		h.depth = 0
	}
	if event.Duration <= 0 {
		return
	}
	indent := strings.Repeat("│  ", h.depth)
	h.timings = append(h.timings, TimingEntry{Path: event.Path, Duration: event.Duration, Depth: h.depth})
	seconds := event.Duration.Seconds()
	fmt.Fprintf(h.writer, "%s└─ %s ⏱  %.2fs\n", indent, getTimingIcon(seconds), seconds)
}

func (h *TreeHandler) handleFolderFileProcessingEnd(event Event, indent string) {
	if event.Duration <= 0 {
		return
	}
	h.timings = append(h.timings, TimingEntry{Path: event.Path, Duration: event.Duration, Depth: h.depth})
	seconds := event.Duration.Seconds()
	fmt.Fprintf(h.writer, "%s└─ %s 📁 %.2fs\n", indent, getTimingIcon(seconds), seconds)
}

func (h *TreeHandler) handleScanInitializing(event Event, indent, prefix string) {
	fmt.Fprintf(h.writer, "%s%sInitializing: %s\n", indent, prefix, event.Path)
	if event.Info != "" {
		fmt.Fprintf(h.writer, "%s%sExcluding: %s\n", indent, prefix, event.Info)
	}
}

func (h *TreeHandler) handleRuleCheck(event Event, indent, prefix string) {
	fmt.Fprintf(h.writer, "%s%sChecking rule: %s\n", indent, prefix, event.Tech)
	for _, detail := range event.Details {
		fmt.Fprintf(h.writer, "%s│  %s\n", indent, detail)
	}
}

func (h *TreeHandler) handleRuleResult(event Event, indent string) {
	h.rules = append(h.rules, RuleEntry{
		Tech:    event.Tech,
		Reason:  event.Reason,
		Path:    event.Path,
		Matched: event.Matched,
	})
	if !event.Matched {
		fmt.Fprintf(h.writer, "%s└─ ✗ NOT MATCHED: %s - %s\n", indent, event.Tech, event.Reason)
		return
	}
	if event.Path != "" {
		fmt.Fprintf(h.writer, "%s└─ ✓ MATCHED: %s - %s (in %s)\n", indent, event.Tech, event.Reason, event.Path)
	} else {
		fmt.Fprintf(h.writer, "%s└─ ✓ MATCHED: %s - %s\n", indent, event.Tech, event.Reason)
	}
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

// NullHandler discards all events (for disabled verbose mode)
type NullHandler struct{}

func NewNullHandler() *NullHandler {
	return &NullHandler{}
}

func (h *NullHandler) Handle(event Event) {
	// Do nothing
}
