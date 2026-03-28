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
	writer     io.Writer
	mu         sync.Mutex
	scanStart  time.Time
	dirCount   int
	fileCount  int
	compCount  int
	spinIdx    int
	lastRender time.Time
	isTTY      bool

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
	elapsed := time.Since(h.scanStart).Truncate(time.Second)

	spinner := h.spinnerStyle.Render(spinFrames[h.spinIdx])

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

	// \r moves to line start; \033[2K erases the entire line (avoids ANSI-length padding issues)
	fmt.Fprintf(h.writer, "\r\033[2K%s", line)
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
