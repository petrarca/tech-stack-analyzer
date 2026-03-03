package progress

import (
	"fmt"
	"io"
	"strings"
	"time"
)

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
		msPerKFiles := 0.0
		if event.FileCount > 0 {
			msPerKFiles = (event.Duration.Seconds() * 1000) / (float64(event.FileCount) / 1000)
		}
		fmt.Fprintf(h.writer, "[SCAN] Completed: %d files, %d directories in %.1fs (%.1fms per 1000 files)\n",
			event.FileCount, event.DirCount, event.Duration.Seconds(), msPerKFiles)

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
