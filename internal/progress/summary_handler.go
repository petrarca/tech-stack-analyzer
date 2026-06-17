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
	resolving   bool   // in the dependency-resolution phase
	resolveInfo string // latest dependency-resolution status (e.g. POM fetch counts)

	// styles
	spinnerStyle lipgloss.Style
	labelStyle   lipgloss.Style
	countStyle   lipgloss.Style
	dimStyle     lipgloss.Style
	doneStyle    lipgloss.Style
	compStyle    lipgloss.Style
}

var spinFrames = []string{"⠋", "⠙", "⠹", "⠸", "⠼", "⠴", "⠦", "⠧", "⠇", "⠏"}

// NewSummaryHandler creates a handler that shows a single updating progress line.
func NewSummaryHandler(writer io.Writer, isTTY bool) *SummaryHandler {
	return &SummaryHandler{
		writer: writer,
		isTTY:  isTTY,
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
		// Begin the dependency-resolution phase: the scan-walk completion line
		// has already printed; switch the live line to resolution status.
		h.resolving = true
		h.resolveInfo = "resolving dependencies"
		h.render()

	case EventResolveProgress:
		h.resolving = true
		h.resolveInfo = "resolving dependencies — " + event.Info
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
	elapsed := time.Since(h.scanStart).Truncate(time.Second)

	// Always show the scan counts (dirs/files/components), then append the
	// resolution status when resolution is in progress -- so the file-walk
	// progress remains visible even while dependencies are being resolved.
	var parts []string
	parts = append(parts, h.countStyle.Render(fmt.Sprintf("%d", h.dirCount))+" "+h.labelStyle.Render("dirs"))
	if h.fileCount > 0 {
		parts = append(parts, h.countStyle.Render(fmt.Sprintf("%d", h.fileCount))+" "+h.labelStyle.Render("files"))
	}
	if h.compCount > 0 {
		parts = append(parts, h.compStyle.Render(fmt.Sprintf("%d", h.compCount))+" "+h.labelStyle.Render("components"))
	}

	line := fmt.Sprintf("  %s  %s  %s",
		spinner,
		strings.Join(parts, h.dimStyle.Render("  ·  ")),
		h.dimStyle.Render(fmt.Sprintf("(%s)", elapsed)),
	)
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
	summary := h.labelStyle.Render("dependencies resolved") + "  " +
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
