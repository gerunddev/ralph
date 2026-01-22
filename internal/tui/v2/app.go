// Package tui provides the Bubble Tea TUI for Ralph.
package tui

import (
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/key"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"github.com/gerund/ralph/internal/claude"
	"github.com/gerund/ralph/internal/loop"
	"github.com/gerund/ralph/internal/parser"
)

// Model is the main Bubble Tea model for the Ralph TUI.
type Model struct {
	header         Header
	feedPanel      *ScrollablePanel
	floatingWindow FloatingWindow

	keys KeyMap

	// Event channel from the loop
	events <-chan loop.Event

	// State
	iteration   int
	maxIter     int
	status      string
	completed   bool
	err         error
	quitting    bool
	initialized bool

	// Event tracking
	eventSeq      int
	startTime     time.Time
	streamedBytes int // Track bytes received via EventAssistantText for fallback detection

	// Progress tracking for completion summary
	lastProgress  string
	lastLearnings string

	width  int
	height int
}

// NewModel creates a new TUI model.
func NewModel() Model {
	feedPanel := NewScrollablePanel("Feed", true)
	floatingWindow := NewFloatingWindow("✓ Completed")
	return Model{
		header:         NewHeader(),
		feedPanel:      &feedPanel,
		floatingWindow: floatingWindow,
		keys:           DefaultKeyMap(),
		startTime:      time.Now(),
	}
}

// NewModelWithEvents creates a new TUI model with an event channel.
func NewModelWithEvents(events <-chan loop.Event) Model {
	m := NewModel()
	m.events = events
	return m
}

// Init implements tea.Model.
func (m Model) Init() tea.Cmd {
	return tea.Batch(
		m.listenForEvents(),
	)
}

// listenForEvents returns a command that listens for loop events.
func (m Model) listenForEvents() tea.Cmd {
	if m.events == nil {
		return nil
	}
	return func() tea.Msg {
		event, ok := <-m.events
		if !ok {
			return EventsClosedMsg{}
		}
		return LoopEventMsg{Event: event}
	}
}

// LoopEventMsg wraps a loop event for Bubble Tea.
type LoopEventMsg struct {
	Event loop.Event
}

// EventsClosedMsg signals that the event channel has closed.
type EventsClosedMsg struct{}

// SetIterationMsg sets the iteration information.
type SetIterationMsg struct {
	Current int
	Max     int
}

// SetPromptMsg sets the current prompt.
type SetPromptMsg struct {
	Prompt string
}

// AppendOutputMsg appends to the output panel.
type AppendOutputMsg struct {
	Text string
}

// SetStatusMsg sets the status message.
type SetStatusMsg struct {
	Status string
}

// SetErrorMsg sets an error message.
type SetErrorMsg struct {
	Error string
}

// Update implements tea.Model.
func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmds []tea.Cmd

	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		m.updateLayout()
		m.initialized = true
		return m, nil

	case tea.KeyMsg:
		// Handle quit first
		if key.Matches(msg, m.keys.Quit) {
			m.quitting = true
			return m, tea.Quit
		}

		// Handle floating window dismiss
		if m.floatingWindow.IsVisible() {
			if key.Matches(msg, m.keys.Dismiss) {
				m.floatingWindow.Hide()
				return m, nil
			}
			// Handle floating window scrolling
			return m.handleFloatingScroll(msg)
		}

		// Handle scrolling
		return m.handleScroll(msg)

	case LoopEventMsg:
		m.handleLoopEvent(msg.Event)
		cmds = append(cmds, m.listenForEvents())

	case EventsClosedMsg:
		// Event channel closed
		if !m.completed && m.err == nil {
			m.completed = true
			m.status = "Completed"
			m.header.SetStatus("Completed")
			finishMsg := sectionDividerStyle.Render("─── Execution finished ───")
			m.feedPanel.AppendLine(fmt.Sprintf("\n%s", finishMsg))
		}
		return m, nil

	case SetIterationMsg:
		m.iteration = msg.Current
		m.maxIter = msg.Max
		m.header.SetIteration(msg.Current, msg.Max)

	case SetPromptMsg:
		promptHeader := sectionDividerStyle.Render("─── Prompt ───")
		m.feedPanel.AppendLine(fmt.Sprintf("\n%s", promptHeader))
		m.feedPanel.AppendContent(msg.Prompt)
		outputHeader := sectionDividerStyle.Render("─── Output ───")
		m.feedPanel.AppendLine(fmt.Sprintf("\n%s", outputHeader))

	case AppendOutputMsg:
		m.feedPanel.AppendContent(msg.Text)

	case SetStatusMsg:
		m.status = msg.Status
		m.header.SetStatus(msg.Status)

	case SetErrorMsg:
		m.err = fmt.Errorf("%s", msg.Error)
		errorMsg := errorStyle.Render(fmt.Sprintf("✗ ERROR: %s", msg.Error))
		m.feedPanel.AppendLine(errorMsg)
	}

	return m, tea.Batch(cmds...)
}

// handleScroll handles scroll key events.
func (m Model) handleScroll(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch {
	case key.Matches(msg, m.keys.Up):
		m.feedPanel.ScrollUp(1)
	case key.Matches(msg, m.keys.Down):
		m.feedPanel.ScrollDown(1)
	}

	return m, nil
}

// handleFloatingScroll handles scroll key events when floating window is visible.
func (m Model) handleFloatingScroll(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch {
	case key.Matches(msg, m.keys.Up):
		m.floatingWindow.ScrollUp(1)
	case key.Matches(msg, m.keys.Down):
		m.floatingWindow.ScrollDown(1)
	}

	return m, nil
}

// handleLoopEvent processes a loop event.
func (m *Model) handleLoopEvent(event loop.Event) {
	// Update iteration info
	if event.MaxIter > 0 {
		m.iteration = event.Iteration
		m.maxIter = event.MaxIter
		m.header.SetIteration(event.Iteration, event.MaxIter)
	}

	switch event.Type {
	case loop.EventStarted:
		m.status = "Running"
		m.header.SetStatus("Running")
		m.feedPanel.AppendLine("Starting execution...")

	case loop.EventIterationStart:
		m.streamedBytes = 0 // Reset streaming tracker for new iteration
		m.status = "Running"
		m.header.SetStatus("Running")
		iterMarker := iterationMarkerStyle.Render(fmt.Sprintf("━━━ Iteration %d/%d ━━━", event.Iteration, event.MaxIter))
		m.feedPanel.AppendLine(fmt.Sprintf("\n%s", iterMarker))

	case loop.EventJJNew:
		m.feedPanel.AppendLine(systemMessageStyle.Render("Creating new jj change..."))

	case loop.EventPromptBuilt:
		promptHeader := sectionDividerStyle.Render("─── Prompt ───")
		m.feedPanel.AppendLine(fmt.Sprintf("\n%s", promptHeader))
		m.feedPanel.AppendContent(event.Prompt)
		outputHeader := sectionDividerStyle.Render("─── Output ───")
		m.feedPanel.AppendLine(fmt.Sprintf("\n%s", outputHeader))

	case loop.EventClaudeStart:
		// No-op

	case loop.EventClaudeStream:
		// Handle streaming Claude output (only assistant text is displayed)
		if event.ClaudeEvent != nil {
			m.handleClaudeEvent(event.ClaudeEvent)
		}

	case loop.EventClaudeOutput:
		// Parse and track progress/learnings for completion summary
		if event.Output != "" {
			parseResult := parser.Parse(event.Output)
			if parseResult.Progress != "" {
				m.lastProgress = parseResult.Progress
			}
			if parseResult.Learnings != "" {
				m.lastLearnings = parseResult.Learnings
			}
		}

	case loop.EventClaudeEnd:
		// No-op

	case loop.EventParsed:
		// No-op

	case loop.EventDistilling:
		m.feedPanel.AppendLine(systemMessageStyle.Render("Distilling commit message..."))

	case loop.EventJJCommit:
		commitMsg := systemMessageStyle.Render(fmt.Sprintf("Committing: %s", event.Message))
		m.feedPanel.AppendLine(commitMsg)

	case loop.EventIterationEnd:
		m.feedPanel.AppendLine(systemMessageStyle.Render("Iteration complete"))

	case loop.EventDeveloperStart:
		m.status = "Developing"
		m.header.SetStatus("Developing")

	case loop.EventDeveloperEnd:
		// Status will be updated by reviewer start or done event

	case loop.EventReviewerStart:
		m.status = "Reviewing"
		m.header.SetStatus("Reviewing")

	case loop.EventReviewerEnd:
		// Status will be updated by next event

	case loop.EventDone:
		m.completed = true
		m.status = "Completed"
		m.header.SetStatus("Completed")
		doneMsg := doneMarkerStyle.Render("✓ DONE DONE DONE!!!")
		m.feedPanel.AppendLine(fmt.Sprintf("\n%s", doneMsg))
		// Show completion floating window with summary
		m.showCompletionWindow()

	case loop.EventMaxIterations:
		m.completed = true
		m.status = "Max Iterations"
		m.header.SetStatus("Max Iterations")
		maxIterMsg := statusFailedStyle.Render(fmt.Sprintf("⚠ %s", event.Message))
		m.feedPanel.AppendLine(fmt.Sprintf("\n%s", maxIterMsg))

	case loop.EventError:
		errorMsg := errorStyle.Render(fmt.Sprintf("✗ ERROR: %s", event.Message))
		m.feedPanel.AppendLine(errorMsg)
	}
}

// handleClaudeEvent processes a Claude stream event.
// Only assistant text is displayed to the screen - all events are still stored in the database.
func (m *Model) handleClaudeEvent(event *claude.StreamEvent) {
	m.eventSeq++

	switch event.Type {
	case claude.EventAssistantText:
		// Streaming text - display inline and track
		if event.AssistantText != nil && event.AssistantText.Text != "" {
			m.feedPanel.AppendContent(event.AssistantText.Text)
			m.streamedBytes += len(event.AssistantText.Text)
		}

	case claude.EventMessage:
		// Complete message - FALLBACK: show if streaming produced nothing
		if event.Message != nil && event.Message.Text != "" {
			if m.streamedBytes == 0 {
				// Streaming didn't work, show the complete message
				m.feedPanel.AppendContent(event.Message.Text)
			}
			// If streaming worked, this is duplicate - skip
		}

	case claude.EventToolUse:
		// Show any text that preceded the tool call (often not streamed!)
		if event.Message != nil && event.Message.Text != "" {
			m.feedPanel.AppendContent(event.Message.Text)
		}
		// Tool call - show condensed format
		if event.ToolUse != nil {
			toolLine := formatToolUse(event.ToolUse)
			m.feedPanel.AppendLine(toolLine)
		}

	case claude.EventError:
		// Always show errors with styled formatting
		if event.Error != nil {
			errorMsg := errorStyle.Render(fmt.Sprintf("✗ [%s]: %s", event.Error.Code, event.Error.Message))
			m.feedPanel.AppendLine(fmt.Sprintf("\n%s", errorMsg))
		}
	}
}

// formatToolUse formats a tool use event for display with styled output.
func formatToolUse(tool *claude.ToolUseContent) string {
	if tool == nil {
		return ""
	}
	// Extract first string param value for context
	param := extractMainParam(tool.Input)

	// Build styled tool call: ▶ ToolName param
	icon := toolBracketStyle.Render("▶")
	name := toolNameStyle.Render(tool.Name)

	if param != "" {
		styledParam := toolParamStyle.Render(param)
		return fmt.Sprintf("\n%s %s %s", icon, name, styledParam)
	}
	return fmt.Sprintf("\n%s %s", icon, name)
}

// extractMainParam extracts the first meaningful string param from tool input JSON.
func extractMainParam(input json.RawMessage) string {
	if len(input) == 0 {
		return ""
	}
	var params map[string]interface{}
	if err := json.Unmarshal(input, &params); err != nil {
		return ""
	}
	// Common param names to look for
	for _, key := range []string{"path", "file_path", "command", "query", "pattern", "url", "content"} {
		if v, ok := params[key]; ok {
			if s, ok := v.(string); ok {
				// Truncate long values
				if len(s) > 60 {
					return s[:57] + "..."
				}
				return s
			}
		}
	}
	return ""
}

// updateLayout updates component sizes based on window size.
func (m *Model) updateLayout() {
	m.header.SetWidth(m.width)

	// Header: ~2 lines only
	reservedHeight := 2
	availableHeight := m.height - reservedHeight
	if availableHeight < 10 {
		availableHeight = 10
	}

	// Single feed panel gets ALL available height
	m.feedPanel.SetSize(m.width, availableHeight)
	m.feedPanel.SetFocused(true)

	// Update floating window size
	m.floatingWindow.SetSize(m.width, m.height)
}

// showCompletionWindow displays the floating window with a completion summary.
func (m *Model) showCompletionWindow() {
	var summary strings.Builder

	// Calculate duration
	duration := time.Since(m.startTime)
	durationStr := formatDuration(duration)

	summary.WriteString(fmt.Sprintf("Completed in %d iteration(s) (%s)\n\n", m.iteration, durationStr))

	if m.lastProgress != "" {
		summary.WriteString("## Summary\n")
		summary.WriteString(m.lastProgress)
	} else {
		summary.WriteString("Task completed successfully.")
	}

	m.floatingWindow.Show(summary.String())
}

// formatDuration formats a duration in a human-readable way.
func formatDuration(d time.Duration) string {
	if d < time.Minute {
		return fmt.Sprintf("%ds", int(d.Seconds()))
	}
	if d < time.Hour {
		mins := int(d.Minutes())
		secs := int(d.Seconds()) % 60
		if secs == 0 {
			return fmt.Sprintf("%dm", mins)
		}
		return fmt.Sprintf("%dm %ds", mins, secs)
	}
	hours := int(d.Hours())
	mins := int(d.Minutes()) % 60
	return fmt.Sprintf("%dh %dm", hours, mins)
}

// View implements tea.Model.
func (m Model) View() string {
	if m.quitting {
		return "Goodbye!\n"
	}

	if !m.initialized {
		return "Initializing..."
	}

	var s strings.Builder

	// Header (iter + status + hints)
	s.WriteString(m.header.View())
	s.WriteString("\n")

	// Feed panel (single panel - ALL content)
	s.WriteString(m.feedPanel.View())

	baseView := lipgloss.NewStyle().MaxWidth(m.width).Render(s.String())

	// Overlay floating window if visible
	if m.floatingWindow.IsVisible() {
		return m.overlayFloatingWindow(baseView)
	}

	return baseView
}

// overlayFloatingWindow renders the floating window on top of the base view.
func (m Model) overlayFloatingWindow(baseView string) string {
	floatingView := m.floatingWindow.View()
	if floatingView == "" {
		return baseView
	}

	// Split both views into lines
	baseLines := strings.Split(baseView, "\n")
	floatLines := strings.Split(floatingView, "\n")

	// Ensure base view has enough lines
	for len(baseLines) < m.height {
		baseLines = append(baseLines, "")
	}

	// Overlay float lines onto base lines (skip empty leading lines from centering)
	floatStartLine := 0
	for i, line := range floatLines {
		if strings.TrimSpace(line) != "" {
			floatStartLine = i
			break
		}
	}

	// Calculate vertical center offset
	floatContentLines := floatLines[floatStartLine:]
	verticalOffset := (m.height - len(floatContentLines)) / 2
	if verticalOffset < 0 {
		verticalOffset = 0
	}

	// Overlay the floating window
	for i, floatLine := range floatContentLines {
		targetLine := verticalOffset + i
		if targetLine < len(baseLines) && strings.TrimSpace(floatLine) != "" {
			baseLines[targetLine] = floatLine
		}
	}

	return strings.Join(baseLines, "\n")
}

// SetEvents sets the event channel for the model.
// This allows setting events after model creation.
func (m *Model) SetEvents(events <-chan loop.Event) {
	m.events = events
}

// SetPrompt sets the prompt content.
func (m *Model) SetPrompt(prompt string) {
	promptHeader := sectionDividerStyle.Render("─── Prompt ───")
	m.feedPanel.AppendLine(fmt.Sprintf("\n%s", promptHeader))
	m.feedPanel.AppendContent(prompt)
	outputHeader := sectionDividerStyle.Render("─── Output ───")
	m.feedPanel.AppendLine(fmt.Sprintf("\n%s", outputHeader))
}

// IsCompleted returns whether the execution has completed.
func (m Model) IsCompleted() bool {
	return m.completed
}

// Error returns any error that occurred.
func (m Model) Error() error {
	return m.err
}

// Run starts the TUI application with the given event channel.
func Run(events <-chan loop.Event) error {
	m := NewModelWithEvents(events)
	p := tea.NewProgram(m, tea.WithAltScreen())
	_, err := p.Run()
	return err
}
