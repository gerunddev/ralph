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
	PlanID    string
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

// SetPlanID sets the plan ID.
func (h *Header) SetPlanID(id string) {
	h.PlanID = id
}

// View renders the header.
func (h Header) View() string {
	// Get border size (Width() sets width including padding but excluding border)
	borderH := headerStyle.GetHorizontalBorderSize()

	// styleWidth is what we pass to Width() - includes padding but not border
	styleWidth := h.width - borderH
	if styleWidth < 40 {
		styleWidth = 40
	}

	// Build content: Iteration | Status: <status> | ↑↓:scroll  q:quit
	iterStr := "---"
	if h.MaxIter > 0 {
		iterStr = fmt.Sprintf("%d/%d", h.Iteration, h.MaxIter)
	} else if h.Iteration > 0 {
		// Extreme mode: MaxIter=0 means "X"
		iterStr = fmt.Sprintf("%d/X", h.Iteration)
	}

	iterSection := headerValueStyle.Render(iterStr)

	statusSection := lipgloss.JoinHorizontal(lipgloss.Center,
		headerLabelStyle.Render("Status: "),
		h.renderStatus(),
	)

	separator := headerLabelStyle.Render("  |  ")

	// Key hints inline after status
	hints := h.renderKeyHints()

	content := iterSection + separator + statusSection + separator + hints

	// Add plan ID after key hints if set
	if h.PlanID != "" {
		// Truncate UUID to first 8 chars for display
		shortID := h.PlanID
		if len(shortID) > 8 {
			shortID = shortID[:8]
		}
		content += separator + helpDescStyle.Render(shortID)
	}

	// Apply style with explicit width and safety cap
	style := headerStyle.Width(styleWidth).MaxHeight(3)
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
