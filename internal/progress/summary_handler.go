package progress

import (
	"fmt"
	"io"
	"strings"
	"sync"
	"time"

	"github.com/charmbracelet/lipgloss"
)

// SummaryHandler provides minimal default progress output: a single updating
// line with spinner, file/directory/component counts, and elapsed time.
// Used when neither --verbose nor --debug is set.
type SummaryHandler struct {
	writer      io.Writer
	mu          sync.Mutex
	scanStart   time.Time
	dirCount    int
	fileCount   int
	compCount   int
	spinIdx     int
	lastRender  time.Time
	isTTY       bool
	resolving   bool    // in the dependency-resolution phase
	resolveInfo string  // latest dependency-resolution status (e.g. POM fetch counts)
	phaseLabel  string  // noun for the resolution phase ("dependencies" by default; "currency" for currency runs)
	resolveFrac float64 // 0..1 completion fraction for a progress bar; <0 means "unknown" (no bar)

	// styles
	spinnerStyle lipgloss.Style
	labelStyle   lipgloss.Style
	countStyle   lipgloss.Style
	dimStyle     lipgloss.Style
	doneStyle    lipgloss.Style
	compStyle    lipgloss.Style
}

var spinFrames = []string{"⠋", "⠙", "⠹", "⠸", "⠼", "⠴", "⠦", "⠧", "⠇", "⠏"}

// SetPhaseLabel overrides the noun used for the resolution phase (default
// "dependencies"). Currency runs set "currency" so the spinner and completion
// line read naturally ("resolving currency", "currency resolved").
func (h *SummaryHandler) SetPhaseLabel(label string) {
	h.mu.Lock()
	defer h.mu.Unlock()
	if label != "" {
		h.phaseLabel = label
	}
}

// SetResolveFraction sets the completion fraction (0..1) for a progress bar on
// the resolution line. Pass a negative value to disable the bar (the default;
// dependency resolution has no known total). Callers that know a denominator
// (e.g. currency, with N/total) set this each progress tick.
func (h *SummaryHandler) SetResolveFraction(frac float64) {
	h.mu.Lock()
	defer h.mu.Unlock()
	h.resolveFrac = frac
}

// renderBar draws a fixed-width bar like "[████████░░░░] 62%" for frac in 0..1.
func renderBar(frac float64, width int) string {
	if frac < 0 {
		frac = 0
	}
	if frac > 1 {
		frac = 1
	}
	filled := int(frac*float64(width) + 0.5)
	if filled > width {
		filled = width
	}
	return fmt.Sprintf("[%s%s] %3.0f%%",
		strings.Repeat("█", filled),
		strings.Repeat("░", width-filled),
		frac*100)
}

// NewSummaryHandler creates a handler that shows a single updating progress line.
func NewSummaryHandler(writer io.Writer, isTTY bool) *SummaryHandler {
	return &SummaryHandler{
		writer:      writer,
		isTTY:       isTTY,
		phaseLabel:  "dependencies",
		resolveFrac: -1, // no bar unless a fraction is provided
		spinnerStyle: lipgloss.NewStyle().
			Foreground(lipgloss.Color("205")),
		labelStyle: lipgloss.NewStyle().
			Foreground(lipgloss.Color("252")),
		countStyle: lipgloss.NewStyle().
			Foreground(lipgloss.Color("86")).
			Bold(true),
		dimStyle: lipgloss.NewStyle().
			Foreground(lipgloss.Color("240")),
		doneStyle: lipgloss.NewStyle().
			Foreground(lipgloss.Color("82")).
			Bold(true),
		compStyle: lipgloss.NewStyle().
			Foreground(lipgloss.Color("214")).
			Bold(true),
	}
}

func (h *SummaryHandler) Handle(event Event) {
	h.mu.Lock()
	defer h.mu.Unlock()

	switch event.Type {
	case EventScanStart:
		h.scanStart = time.Now()
		h.lastRender = time.Now()

	case EventEnterDirectory:
		h.dirCount++
		h.throttledRender()

	case EventFileProcessingStart:
		h.fileCount++
		h.throttledRender()

	case EventComponentDetected:
		h.compCount++
		h.throttledRender()

	case EventInfo:
		// Generic info folds into the live line rather than scrolling.
		h.resolveInfo = event.Info
		h.render()

	case EventResolveStart:
		// Begin the resolution phase: the scan-walk completion line has already
		// printed; switch the live line to resolution status.
		h.resolving = true
		h.resolveInfo = "resolving " + h.phaseLabel
		h.render()

	case EventResolveProgress:
		h.resolving = true
		h.resolveInfo = "resolving " + h.phaseLabel + " — " + event.Info
		h.render()

	case EventResolveComplete:
		h.renderResolveComplete(event)

	case EventScanComplete:
		h.renderComplete(event)
	}
}

// throttledRender updates the progress line at most every 80ms to avoid flicker.
func (h *SummaryHandler) throttledRender() {
	now := time.Now()
	if now.Sub(h.lastRender) < 80*time.Millisecond {
		return
	}
	h.lastRender = now
	h.render()
}

func (h *SummaryHandler) render() {
	if !h.isTTY {
		return // no progress line on non-TTY (piped output)
	}

	h.spinIdx = (h.spinIdx + 1) % len(spinFrames)
	spinner := h.spinnerStyle.Render(spinFrames[h.spinIdx])
	if h.scanStart.IsZero() {
		h.scanStart = time.Now() // resolution-only run (e.g. currency): seed the clock
	}
	elapsed := time.Since(h.scanStart).Truncate(time.Second)

	// Show the scan counts (dirs/files/components) when a scan ran, then append
	// the resolution status. For a resolution-only run (no scan walked), omit the
	// zero scan counts entirely so the line reads "spinner  resolving ...".
	var parts []string
	if h.dirCount > 0 || h.fileCount > 0 || h.compCount > 0 {
		parts = append(parts, h.countStyle.Render(fmt.Sprintf("%d", h.dirCount))+" "+h.labelStyle.Render("dirs"))
		if h.fileCount > 0 {
			parts = append(parts, h.countStyle.Render(fmt.Sprintf("%d", h.fileCount))+" "+h.labelStyle.Render("files"))
		}
		if h.compCount > 0 {
			parts = append(parts, h.compStyle.Render(fmt.Sprintf("%d", h.compCount))+" "+h.labelStyle.Render("components"))
		}
	}

	// Optional progress bar (only when a fraction was provided, e.g. currency).
	bar := ""
	if h.resolving && h.resolveFrac >= 0 {
		bar = h.countStyle.Render(renderBar(h.resolveFrac, 20)) + "  "
	}

	var line string
	if len(parts) > 0 {
		line = fmt.Sprintf("  %s  %s%s  %s", spinner, bar,
			strings.Join(parts, h.dimStyle.Render("  ·  ")),
			h.dimStyle.Render(fmt.Sprintf("(%s)", elapsed)))
	} else {
		line = fmt.Sprintf("  %s  %s%s", spinner, bar,
			h.dimStyle.Render(fmt.Sprintf("(%s)", elapsed)))
	}
	if h.resolving && h.resolveInfo != "" {
		line += h.dimStyle.Render("  ·  ") + h.labelStyle.Render(h.resolveInfo)
	}

	// \r moves to line start; \033[2K erases the entire line (avoids ANSI-length padding issues)
	fmt.Fprintf(h.writer, "\r\033[2K%s", line)
}

// renderResolveComplete prints the final line for the dependency-resolution
// phase (a checkmark + the resolution metrics + elapsed time).
func (h *SummaryHandler) renderResolveComplete(event Event) {
	h.resolving = false
	check := h.doneStyle.Render("✓")
	summary := h.labelStyle.Render(h.phaseLabel+" resolved") + "  " +
		h.dimStyle.Render(event.Info) + "  " +
		h.dimStyle.Render(fmt.Sprintf("(%s)", event.Duration.Truncate(100*time.Millisecond)))
	if h.isTTY {
		fmt.Fprintf(h.writer, "\r\033[2K")
	}
	fmt.Fprintf(h.writer, "  %s  %s\n", check, summary)
}

func (h *SummaryHandler) renderComplete(event Event) {
	elapsed := event.Duration.Truncate(100 * time.Millisecond)

	parts := []string{
		h.countStyle.Render(fmt.Sprintf("%d", event.FileCount)) + " " + h.labelStyle.Render("files"),
		h.countStyle.Render(fmt.Sprintf("%d", event.DirCount)) + " " + h.labelStyle.Render("dirs"),
	}
	if h.compCount > 0 {
		parts = append(parts, h.compStyle.Render(fmt.Sprintf("%d", h.compCount))+" "+h.labelStyle.Render("components"))
	}

	check := h.doneStyle.Render("✓")
	summary := strings.Join(parts, h.dimStyle.Render(", ")) + "  " + h.dimStyle.Render(fmt.Sprintf("(%s)", elapsed))

	if h.isTTY {
		// Erase the progress line before printing final summary
		fmt.Fprintf(h.writer, "\r\033[2K")
	}
	fmt.Fprintf(h.writer, "  %s  %s\n", check, summary)
}
