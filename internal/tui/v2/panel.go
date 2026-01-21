// Package tui provides the Bubble Tea TUI for Ralph V2 single-agent loop.
package tui

import (
	"strings"

	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

// ScrollablePanel is a generic scrollable text panel.
type ScrollablePanel struct {
	Title      string
	viewport   viewport.Model
	content    strings.Builder
	AutoScroll bool
	Focused    bool
	width      int
	height     int
	dirty      bool // content changed since last viewport sync
}

// NewScrollablePanel creates a new scrollable panel.
func NewScrollablePanel(title string, autoScroll bool) ScrollablePanel {
	vp := viewport.New(80, 10)
	return ScrollablePanel{
		Title:      title,
		viewport:   vp,
		AutoScroll: autoScroll,
	}
}

// SetSize sets the panel dimensions.
func (p *ScrollablePanel) SetSize(width, height int) {
	p.width = width
	p.height = height

	// Account for title line and borders
	viewportWidth := width - 4
	viewportHeight := height - 4 // title + borders
	if viewportWidth < 10 {
		viewportWidth = 10
	}
	if viewportHeight < 3 {
		viewportHeight = 3
	}

	p.viewport.Width = viewportWidth
	p.viewport.Height = viewportHeight
}

// SetContent replaces the entire content.
func (p *ScrollablePanel) SetContent(content string) {
	p.content.Reset()
	p.content.WriteString(content)
	p.viewport.SetContent(content)
	if p.AutoScroll {
		p.viewport.GotoBottom()
	}
}

// AppendContent adds content to the end.
// This is O(1) - viewport sync is deferred until View() is called.
func (p *ScrollablePanel) AppendContent(content string) {
	p.content.WriteString(content)
	p.dirty = true
}

// AppendLine adds a line of content with a newline.
// This is O(1) - viewport sync is deferred until View() is called.
func (p *ScrollablePanel) AppendLine(line string) {
	p.content.WriteString(line)
	p.content.WriteString("\n")
	p.dirty = true
}

// Clear clears all content.
func (p *ScrollablePanel) Clear() {
	p.content.Reset()
	p.viewport.SetContent("")
	p.dirty = false
}

// Content returns the current content.
func (p *ScrollablePanel) Content() string {
	return p.content.String()
}

// SetFocused sets the focus state.
func (p *ScrollablePanel) SetFocused(focused bool) {
	p.Focused = focused
}

// Update handles messages for the panel.
func (p *ScrollablePanel) Update(msg tea.Msg) tea.Cmd {
	if !p.Focused {
		return nil
	}

	var cmd tea.Cmd
	p.viewport, cmd = p.viewport.Update(msg)

	// Track if user scrolled away from bottom
	if _, ok := msg.(tea.KeyMsg); ok {
		if !p.viewport.AtBottom() {
			p.AutoScroll = false
		} else {
			p.AutoScroll = true
		}
	}

	return cmd
}

// ScrollUp scrolls up by n lines.
func (p *ScrollablePanel) ScrollUp(n int) {
	p.viewport.LineUp(n)
	p.AutoScroll = false
}

// ScrollDown scrolls down by n lines.
func (p *ScrollablePanel) ScrollDown(n int) {
	p.viewport.LineDown(n)
	if p.viewport.AtBottom() {
		p.AutoScroll = true
	}
}

// PageUp scrolls up by one page.
func (p *ScrollablePanel) PageUp() {
	p.viewport.ViewUp()
	p.AutoScroll = false
}

// PageDown scrolls down by one page.
func (p *ScrollablePanel) PageDown() {
	p.viewport.ViewDown()
	if p.viewport.AtBottom() {
		p.AutoScroll = true
	}
}

// GotoTop scrolls to the top.
func (p *ScrollablePanel) GotoTop() {
	p.viewport.GotoTop()
	p.AutoScroll = false
}

// GotoBottom scrolls to the bottom.
func (p *ScrollablePanel) GotoBottom() {
	p.viewport.GotoBottom()
	p.AutoScroll = true
}

// syncViewport updates the viewport with accumulated content changes.
// This batches multiple AppendContent/AppendLine calls into a single
// viewport update, converting O(n²) streaming behavior to O(n).
func (p *ScrollablePanel) syncViewport() {
	if !p.dirty {
		return
	}
	p.viewport.SetContent(p.content.String())
	if p.AutoScroll {
		p.viewport.GotoBottom()
	}
	p.dirty = false
}

// View renders the panel.
func (p *ScrollablePanel) View() string {
	// Sync any pending content changes before rendering
	p.syncViewport()
	contentWidth := p.width - 2 // Account for border
	if contentWidth < 10 {
		contentWidth = 10
	}

	// Title line
	title := panelTitleStyle.Render(p.Title)

	// Scroll indicator
	scrollIndicator := ""
	if p.AutoScroll {
		scrollIndicator = scrollIndicatorStyle.Render("[auto-scroll]")
	} else {
		scrollIndicator = scrollIndicatorStyle.Render("[scroll]")
	}

	// Title with scroll indicator right-aligned
	titleWidth := lipgloss.Width(title)
	indicatorWidth := lipgloss.Width(scrollIndicator)
	spacing := contentWidth - titleWidth - indicatorWidth - 2
	if spacing < 1 {
		spacing = 1
	}
	titleLine := title + strings.Repeat(" ", spacing) + scrollIndicator

	// Viewport content
	viewportContent := p.viewport.View()

	// Combine
	content := titleLine + "\n" + viewportContent

	// Apply border style based on focus
	var style lipgloss.Style
	if p.Focused {
		style = panelFocusedStyle.Width(contentWidth)
	} else {
		style = panelStyle.Width(contentWidth)
	}

	return style.Render(content)
}

// AtBottom returns whether the viewport is at the bottom.
func (p *ScrollablePanel) AtBottom() bool {
	return p.viewport.AtBottom()
}

// AtTop returns whether the viewport is at the top.
func (p *ScrollablePanel) AtTop() bool {
	return p.viewport.AtTop()
}
