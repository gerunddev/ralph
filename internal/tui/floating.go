// Package tui provides the Bubble Tea TUI for Ralph.
package tui

import (
	"strings"

	"github.com/charmbracelet/bubbles/viewport"
	"github.com/charmbracelet/lipgloss"
)

// FloatingWindow is a centered modal overlay for displaying completion summaries.
type FloatingWindow struct {
	Title    string
	Content  string
	viewport viewport.Model
	visible  bool
	width    int
	height   int
}

// NewFloatingWindow creates a new floating window.
func NewFloatingWindow(title string) FloatingWindow {
	vp := viewport.New(60, 10)
	return FloatingWindow{
		Title:    title,
		viewport: vp,
	}
}

// SetSize sets the available screen size for centering calculations.
func (f *FloatingWindow) SetSize(width, height int) {
	f.width = width
	f.height = height

	// Calculate window dimensions (60% of screen, with min/max constraints)
	windowWidth := width * 60 / 100
	if windowWidth < 40 {
		windowWidth = 40
	}
	if windowWidth > 100 {
		windowWidth = 100
	}

	windowHeight := height * 60 / 100
	if windowHeight < 10 {
		windowHeight = 10
	}
	if windowHeight > 30 {
		windowHeight = 30
	}

	// Get frame size from style
	frameH, frameV := floatingWindowStyle.GetFrameSize()

	// Title takes 1 line
	titleHeight := 1

	// Viewport is inside the window
	f.viewport.Width = windowWidth - frameH
	f.viewport.Height = windowHeight - frameV - titleHeight
}

// Show displays the floating window with the given content.
func (f *FloatingWindow) Show(content string) {
	f.Content = content
	f.viewport.SetContent(content)
	f.viewport.GotoTop()
	f.visible = true
}

// Hide hides the floating window.
func (f *FloatingWindow) Hide() {
	f.visible = false
}

// IsVisible returns whether the window is visible.
func (f *FloatingWindow) IsVisible() bool {
	return f.visible
}

// ScrollUp scrolls the content up.
func (f *FloatingWindow) ScrollUp(n int) {
	f.viewport.LineUp(n)
}

// ScrollDown scrolls the content down.
func (f *FloatingWindow) ScrollDown(n int) {
	f.viewport.LineDown(n)
}

// View renders the floating window centered on screen.
// Returns empty string if not visible.
func (f FloatingWindow) View() string {
	if !f.visible {
		return ""
	}

	// Calculate window dimensions
	windowWidth := f.width * 60 / 100
	if windowWidth < 40 {
		windowWidth = 40
	}
	if windowWidth > 100 {
		windowWidth = 100
	}

	windowHeight := f.height * 60 / 100
	if windowHeight < 10 {
		windowHeight = 10
	}
	if windowHeight > 30 {
		windowHeight = 30
	}

	// Get frame size dynamically
	frameH, _ := floatingWindowStyle.GetFrameSize()
	contentWidth := windowWidth - frameH

	// Title line
	title := floatingTitleStyle.Render(f.Title)

	// Key hints for the floating window
	hints := helpKeyStyle.Render("↑↓") + helpDescStyle.Render(":scroll") +
		helpSeparatorStyle.Render("  ") +
		helpKeyStyle.Render("Enter/Esc") + helpDescStyle.Render(":close")

	// Title with hints right-aligned
	titleWidth := lipgloss.Width(title)
	hintsWidth := lipgloss.Width(hints)
	spacing := contentWidth - titleWidth - hintsWidth
	if spacing < 1 {
		spacing = 1
	}
	titleLine := title + strings.Repeat(" ", spacing) + hints

	// Viewport content
	viewportContent := f.viewport.View()

	// Combine title and viewport
	content := titleLine + "\n" + viewportContent

	// Apply style with safety cap
	windowStyle := floatingWindowStyle.Width(contentWidth).MaxHeight(windowHeight)
	window := windowStyle.Render(content)

	// Calculate centering offsets
	windowRenderedWidth := lipgloss.Width(window)
	windowRenderedHeight := lipgloss.Height(window)

	horizontalPadding := (f.width - windowRenderedWidth) / 2
	verticalPadding := (f.height - windowRenderedHeight) / 2

	if horizontalPadding < 0 {
		horizontalPadding = 0
	}
	if verticalPadding < 0 {
		verticalPadding = 0
	}

	// Build the centered view
	var result strings.Builder

	// Vertical padding (top)
	for i := 0; i < verticalPadding; i++ {
		result.WriteString("\n")
	}

	// Add horizontal padding to each line
	lines := strings.Split(window, "\n")
	padding := strings.Repeat(" ", horizontalPadding)
	for i, line := range lines {
		result.WriteString(padding)
		result.WriteString(line)
		if i < len(lines)-1 {
			result.WriteString("\n")
		}
	}

	return result.String()
}
