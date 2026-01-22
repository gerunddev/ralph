// Package tui provides the Bubble Tea TUI for Ralph.
package tui

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/lipgloss"
)

// Header displays iteration status and key hints.
type Header struct {
	Iteration int
	MaxIter   int
	Status    string
	width     int
}

// NewHeader creates a new header component.
func NewHeader() Header {
	return Header{
		Status: "Pending",
	}
}

// SetIteration sets the current iteration and max.
func (h *Header) SetIteration(current, max int) {
	h.Iteration = current
	h.MaxIter = max
}

// SetStatus sets the status text.
func (h *Header) SetStatus(status string) {
	h.Status = status
}

// SetWidth sets the component width.
func (h *Header) SetWidth(w int) {
	h.width = w
}

// View renders the header.
func (h Header) View() string {
	// Border takes 2 chars, padding takes 2 chars (1 each side)
	// But Width() in lipgloss includes padding, so we subtract border only for outer width
	// and subtract padding for inner content calculation
	outerWidth := h.width - 2 // Subtract border (1 char each side)
	if outerWidth < 40 {
		outerWidth = 40
	}
	innerWidth := outerWidth - 2 // Subtract padding (1 char each side)

	// Left side: Iteration + Status
	iterStr := "---"
	if h.MaxIter > 0 {
		iterStr = fmt.Sprintf("%d/%d", h.Iteration, h.MaxIter)
	}

	iterSection := headerValueStyle.Render(iterStr)

	statusSection := lipgloss.JoinHorizontal(lipgloss.Center,
		headerLabelStyle.Render("Status: "),
		h.renderStatus(),
	)

	separator := headerLabelStyle.Render("  |  ")
	leftContent := iterSection + separator + statusSection

	// Right side: Key hints
	hints := h.renderKeyHints()

	// Calculate spacing to push hints to the right
	leftWidth := lipgloss.Width(leftContent)
	hintsWidth := lipgloss.Width(hints)
	spacing := innerWidth - leftWidth - hintsWidth
	if spacing < 1 {
		spacing = 1
	}

	content := leftContent + strings.Repeat(" ", spacing) + hints

	style := headerStyle.Width(outerWidth)
	return style.Render(content)
}

// renderStatus renders the status with appropriate styling.
func (h Header) renderStatus() string {
	status := h.Status
	if status == "" {
		status = "Pending"
	}

	switch strings.ToLower(status) {
	case "running", "in progress":
		return statusRunningStyle.Render(status)
	case "developing":
		return statusDevelopingStyle.Render(status)
	case "reviewing":
		return statusReviewingStyle.Render(status)
	case "completed", "done", "complete":
		return statusCompletedStyle.Render(status)
	case "failed", "error":
		return statusFailedStyle.Render(status)
	default:
		return statusPendingStyle.Render(status)
	}
}

// renderKeyHints renders the key binding hints.
func (h Header) renderKeyHints() string {
	parts := []string{
		h.renderHint("↑↓", "scroll"),
		h.renderHint("q", "quit"),
	}
	return strings.Join(parts, helpSeparatorStyle.Render("  "))
}

// renderHint renders a single key hint.
func (h Header) renderHint(key, desc string) string {
	return helpKeyStyle.Render(key) + helpDescStyle.Render(":"+desc)
}
