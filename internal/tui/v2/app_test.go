package tui

import (
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/gerund/ralph/internal/claude"
	"github.com/gerund/ralph/internal/loop"
)

// Helper to update and cast the model
func updateModel(m Model, msg tea.Msg) Model {
	updated, _ := m.Update(msg)
	return updated.(Model)
}

func TestNewModel(t *testing.T) {
	m := NewModel()

	if m.completed {
		t.Error("expected completed to be false initially")
	}

	if m.quitting {
		t.Error("expected quitting to be false initially")
	}

	if m.feedPanel == nil {
		t.Error("expected feedPanel to be initialized")
	}
}

func TestNewModelWithEvents(t *testing.T) {
	events := make(chan loop.Event)
	m := NewModelWithEvents(events)

	if m.events == nil {
		t.Error("expected events channel to be set")
	}

	close(events)
}

func TestModel_Init(t *testing.T) {
	m := NewModel()
	cmd := m.Init()

	// Without events, Init should return nil
	if cmd != nil {
		t.Error("expected nil command when no events channel")
	}

	// With events channel
	events := make(chan loop.Event)
	m2 := NewModelWithEvents(events)
	cmd2 := m2.Init()

	if cmd2 == nil {
		t.Error("expected non-nil command when events channel is set")
	}

	close(events)
}

func TestModel_WindowSizeMsg(t *testing.T) {
	m := NewModel()

	msg := tea.WindowSizeMsg{Width: 100, Height: 40}
	model := updateModel(m, msg)

	if model.width != 100 {
		t.Errorf("expected width 100, got %d", model.width)
	}
	if model.height != 40 {
		t.Errorf("expected height 40, got %d", model.height)
	}
	if !model.initialized {
		t.Error("expected initialized to be true after WindowSizeMsg")
	}
}

func TestModel_QuitKey(t *testing.T) {
	m := NewModel()
	// Initialize first
	m = updateModel(m, tea.WindowSizeMsg{Width: 100, Height: 40})

	msg := tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'q'}}
	updated, cmd := m.Update(msg)
	model := updated.(Model)

	if !model.quitting {
		t.Error("expected quitting to be true after 'q' key")
	}

	// Check that quit command was returned
	if cmd == nil {
		t.Error("expected quit command to be returned")
	}
}

func TestModel_HandleLoopEvent_Started(t *testing.T) {
	events := make(chan loop.Event, 10)
	m := NewModelWithEvents(events)
	m = updateModel(m, tea.WindowSizeMsg{Width: 100, Height: 40})

	// Simulate loop started event
	event := loop.Event{
		Type:      loop.EventStarted,
		Iteration: 0,
		MaxIter:   10,
		Message:   "Loop started",
	}

	m.handleLoopEvent(event)

	if m.status != "Running" {
		t.Errorf("expected status 'Running', got '%s'", m.status)
	}

	output := m.feedPanel.Content()
	if !strings.Contains(output, "Starting execution") {
		t.Errorf("expected output to contain 'Starting execution', got '%s'", output)
	}

	close(events)
}

func TestModel_HandleLoopEvent_Done(t *testing.T) {
	events := make(chan loop.Event, 10)
	m := NewModelWithEvents(events)
	m = updateModel(m, tea.WindowSizeMsg{Width: 100, Height: 40})

	event := loop.Event{
		Type:      loop.EventDone,
		Iteration: 5,
		MaxIter:   10,
		Message:   "Agent completed",
	}

	m.handleLoopEvent(event)

	if !m.completed {
		t.Error("expected completed to be true")
	}

	if m.status != "Completed" {
		t.Errorf("expected status 'Completed', got '%s'", m.status)
	}

	output := m.feedPanel.Content()
	if !strings.Contains(output, "DONE DONE DONE") {
		t.Errorf("expected output to contain 'DONE DONE DONE', got '%s'", output)
	}

	close(events)
}

func TestModel_HandleLoopEvent_MaxIterations(t *testing.T) {
	events := make(chan loop.Event, 10)
	m := NewModelWithEvents(events)
	m = updateModel(m, tea.WindowSizeMsg{Width: 100, Height: 40})

	event := loop.Event{
		Type:      loop.EventMaxIterations,
		Iteration: 10,
		MaxIter:   10,
		Message:   "Reached max iterations",
	}

	m.handleLoopEvent(event)

	if !m.completed {
		t.Error("expected completed to be true")
	}

	if m.status != "Max Iterations" {
		t.Errorf("expected status 'Max Iterations', got '%s'", m.status)
	}

	close(events)
}

func TestModel_HandleLoopEvent_Error(t *testing.T) {
	events := make(chan loop.Event, 10)
	m := NewModelWithEvents(events)
	m = updateModel(m, tea.WindowSizeMsg{Width: 100, Height: 40})

	event := loop.Event{
		Type:      loop.EventError,
		Iteration: 3,
		MaxIter:   10,
		Message:   "Something went wrong",
	}

	m.handleLoopEvent(event)

	output := m.feedPanel.Content()
	if !strings.Contains(output, "ERROR: Something went wrong") {
		t.Errorf("expected output to contain error message, got '%s'", output)
	}

	close(events)
}

func TestModel_HandleLoopEvent_ClaudeOutput(t *testing.T) {
	events := make(chan loop.Event, 10)
	m := NewModelWithEvents(events)
	m = updateModel(m, tea.WindowSizeMsg{Width: 100, Height: 40})

	event := loop.Event{
		Type:      loop.EventClaudeOutput,
		Iteration: 2,
		MaxIter:   5,
		Output:    "This is the final collected output from Claude",
	}

	m.handleLoopEvent(event)

	// EventClaudeOutput is a no-op now (output comes via streaming EventAssistantText events)
	if m.iteration != 2 {
		t.Errorf("expected iteration 2, got %d", m.iteration)
	}
	if m.maxIter != 5 {
		t.Errorf("expected maxIter 5, got %d", m.maxIter)
	}

	close(events)
}

func TestModel_HandleLoopEvent_ClaudeStream_AssistantText(t *testing.T) {
	events := make(chan loop.Event, 10)
	m := NewModelWithEvents(events)
	m = updateModel(m, tea.WindowSizeMsg{Width: 100, Height: 40})

	// Simulate streaming text chunks from Claude
	chunks := []string{"Hello, ", "how are ", "you today?"}

	for _, chunk := range chunks {
		event := loop.Event{
			Type:      loop.EventClaudeStream,
			Iteration: 1,
			MaxIter:   5,
			ClaudeEvent: &claude.StreamEvent{
				Type: claude.EventAssistantText,
				AssistantText: &claude.AssistantTextContent{
					Text: chunk,
				},
			},
		}
		m.handleLoopEvent(event)
	}

	// Verify all chunks appear in feed panel
	outputContent := m.feedPanel.Content()
	expectedOutput := "Hello, how are you today?"
	if outputContent != expectedOutput {
		t.Errorf("expected output '%s', got '%s'", expectedOutput, outputContent)
	}

	close(events)
}

func TestModel_HandleLoopEvent_ClaudeStream_EmptyText(t *testing.T) {
	events := make(chan loop.Event, 10)
	m := NewModelWithEvents(events)
	m = updateModel(m, tea.WindowSizeMsg{Width: 100, Height: 40})

	// Empty text events should be ignored
	event := loop.Event{
		Type:      loop.EventClaudeStream,
		Iteration: 1,
		MaxIter:   5,
		ClaudeEvent: &claude.StreamEvent{
			Type: claude.EventAssistantText,
			AssistantText: &claude.AssistantTextContent{
				Text: "",
			},
		},
	}
	m.handleLoopEvent(event)

	// Output should remain empty
	outputContent := m.feedPanel.Content()
	if outputContent != "" {
		t.Errorf("expected empty output for empty text event, got '%s'", outputContent)
	}

	close(events)
}

func TestModel_HandleLoopEvent_ClaudeStream_NilAssistantText(t *testing.T) {
	events := make(chan loop.Event, 10)
	m := NewModelWithEvents(events)
	m = updateModel(m, tea.WindowSizeMsg{Width: 100, Height: 40})

	// Nil AssistantText should be handled gracefully
	event := loop.Event{
		Type:      loop.EventClaudeStream,
		Iteration: 1,
		MaxIter:   5,
		ClaudeEvent: &claude.StreamEvent{
			Type:          claude.EventAssistantText,
			AssistantText: nil,
		},
	}
	m.handleLoopEvent(event)

	// Output should remain empty
	outputContent := m.feedPanel.Content()
	if outputContent != "" {
		t.Errorf("expected empty output for nil assistant text, got '%s'", outputContent)
	}

	close(events)
}

func TestModel_HandleLoopEvent_PromptBuilt(t *testing.T) {
	events := make(chan loop.Event, 10)
	m := NewModelWithEvents(events)
	m = updateModel(m, tea.WindowSizeMsg{Width: 100, Height: 40})

	event := loop.Event{
		Type:      loop.EventPromptBuilt,
		Iteration: 1,
		MaxIter:   5,
		Prompt:    "Test prompt for Claude",
	}

	m.handleLoopEvent(event)

	// Prompt is shown in feed panel (styled dividers use ─── instead of ---)
	outputContent := m.feedPanel.Content()
	if !strings.Contains(outputContent, "Prompt") {
		t.Errorf("expected feed panel to contain prompt delimiter, got '%s'", outputContent)
	}
	if !strings.Contains(outputContent, "Test prompt for Claude") {
		t.Errorf("expected feed panel to contain the prompt, got '%s'", outputContent)
	}
	if !strings.Contains(outputContent, "Output") {
		t.Errorf("expected feed panel to contain output delimiter, got '%s'", outputContent)
	}

	close(events)
}

func TestModel_SetPromptMsg(t *testing.T) {
	m := NewModel()
	m = updateModel(m, tea.WindowSizeMsg{Width: 100, Height: 40})

	msg := SetPromptMsg{Prompt: "Test prompt content"}
	model := updateModel(m, msg)

	content := model.feedPanel.Content()
	if !strings.Contains(content, "Test prompt content") {
		t.Errorf("expected feed to contain prompt content, got '%s'", content)
	}
	// Styled dividers use ─── instead of ---
	if !strings.Contains(content, "Prompt") {
		t.Errorf("expected feed to contain prompt delimiter, got '%s'", content)
	}
}

func TestModel_SetIterationMsg(t *testing.T) {
	m := NewModel()
	m = updateModel(m, tea.WindowSizeMsg{Width: 100, Height: 40})

	msg := SetIterationMsg{Current: 5, Max: 20}
	model := updateModel(m, msg)

	if model.iteration != 5 {
		t.Errorf("expected iteration 5, got %d", model.iteration)
	}

	if model.maxIter != 20 {
		t.Errorf("expected maxIter 20, got %d", model.maxIter)
	}
}

func TestModel_AppendOutputMsg(t *testing.T) {
	m := NewModel()
	m = updateModel(m, tea.WindowSizeMsg{Width: 100, Height: 40})

	msg := AppendOutputMsg{Text: "Hello "}
	model := updateModel(m, msg)

	msg2 := AppendOutputMsg{Text: "World"}
	model = updateModel(model, msg2)

	content := model.feedPanel.Content()
	if content != "Hello World" {
		t.Errorf("expected output 'Hello World', got '%s'", content)
	}
}

func TestModel_EventsClosedMsg(t *testing.T) {
	m := NewModel()
	m = updateModel(m, tea.WindowSizeMsg{Width: 100, Height: 40})

	msg := EventsClosedMsg{}
	model := updateModel(m, msg)

	if !model.completed {
		t.Error("expected completed to be true after events closed")
	}

	if model.status != "Completed" {
		t.Errorf("expected status 'Completed', got '%s'", model.status)
	}
}

func TestModel_View_NotInitialized(t *testing.T) {
	m := NewModel()

	view := m.View()
	if !strings.Contains(view, "Initializing") {
		t.Errorf("expected 'Initializing' in view, got '%s'", view)
	}
}

func TestModel_View_Quitting(t *testing.T) {
	m := NewModel()
	m.quitting = true

	view := m.View()
	if !strings.Contains(view, "Goodbye") {
		t.Errorf("expected 'Goodbye' in view, got '%s'", view)
	}
}

func TestModel_View_Initialized(t *testing.T) {
	m := NewModel()
	m = updateModel(m, tea.WindowSizeMsg{Width: 100, Height: 40})

	view := m.View()

	// Should contain Status
	if !strings.Contains(view, "Status:") {
		t.Error("expected view to contain 'Status:'")
	}

	// Should contain Feed panel
	if !strings.Contains(view, "Feed") {
		t.Error("expected view to contain 'Feed'")
	}

	// Should contain key hints in header
	if !strings.Contains(view, "quit") {
		t.Error("expected view to contain 'quit' hint")
	}
	if !strings.Contains(view, "scroll") {
		t.Error("expected view to contain 'scroll' hint")
	}
}

func TestHeader_View(t *testing.T) {
	h := NewHeader()
	h.SetIteration(3, 20)
	h.SetStatus("Running")
	h.SetWidth(80)

	view := h.View()

	if !strings.Contains(view, "3/20") {
		t.Error("expected iteration '3/20' in view")
	}

	if !strings.Contains(view, "Running") {
		t.Error("expected 'Running' status in view")
	}

	// Check hints (now just scroll and quit):
	if !strings.Contains(view, "scroll") {
		t.Error("expected 'scroll' hint in header")
	}
	if !strings.Contains(view, "quit") {
		t.Error("expected 'quit' hint in header")
	}
}

func TestScrollablePanel_Content(t *testing.T) {
	p := NewScrollablePanel("Test", false)
	p.SetSize(80, 20)

	// Test SetContent
	p.SetContent("Hello")
	if p.Content() != "Hello" {
		t.Errorf("expected 'Hello', got '%s'", p.Content())
	}

	// Test AppendContent
	p.AppendContent(" World")
	if p.Content() != "Hello World" {
		t.Errorf("expected 'Hello World', got '%s'", p.Content())
	}

	// Test AppendLine
	p.AppendLine("!")
	if p.Content() != "Hello World!\n" {
		t.Errorf("expected 'Hello World!\\n', got '%s'", p.Content())
	}

	// Test Clear
	p.Clear()
	if p.Content() != "" {
		t.Errorf("expected empty content, got '%s'", p.Content())
	}
}

func TestScrollablePanel_View(t *testing.T) {
	p := NewScrollablePanel("Test Panel", true)
	p.SetSize(60, 15)
	p.SetContent("Test content here")

	view := p.View()

	if !strings.Contains(view, "Test Panel") {
		t.Error("expected panel title in view")
	}

	if !strings.Contains(view, "auto-scroll") {
		t.Error("expected auto-scroll indicator when AutoScroll is true")
	}
}

func TestScrollablePanel_Focus(t *testing.T) {
	p := NewScrollablePanel("Test", false)
	p.SetSize(60, 15)

	if p.Focused {
		t.Error("expected panel to not be focused initially")
	}

	p.SetFocused(true)
	if !p.Focused {
		t.Error("expected panel to be focused after SetFocused(true)")
	}

	p.SetFocused(false)
	if p.Focused {
		t.Error("expected panel to not be focused after SetFocused(false)")
	}
}

func TestScrollablePanel_DeferredSync(t *testing.T) {
	// Verify that AppendContent defers viewport sync until View() is called.
	// This is critical for O(n) performance instead of O(n²) during streaming.
	p := NewScrollablePanel("Test", true)
	p.SetSize(80, 20)

	// Append multiple times (simulating streaming chunks)
	p.AppendContent("chunk1 ")
	p.AppendContent("chunk2 ")
	p.AppendContent("chunk3")

	// Content should be accumulated
	if p.Content() != "chunk1 chunk2 chunk3" {
		t.Errorf("expected accumulated content, got '%s'", p.Content())
	}

	// View should show the content (triggering sync)
	view := p.View()
	if !strings.Contains(view, "chunk1") || !strings.Contains(view, "chunk3") {
		t.Errorf("expected view to contain appended content after sync")
	}
}

func TestKeyMap_ShortHelp(t *testing.T) {
	km := DefaultKeyMap()
	help := km.ShortHelp()

	if len(help) == 0 {
		t.Error("expected non-empty short help")
	}

	// Should have 2 bindings: scroll and quit
	if len(help) != 2 {
		t.Errorf("expected 2 short help bindings, got %d", len(help))
	}
}

func TestKeyMap_FullHelp(t *testing.T) {
	km := DefaultKeyMap()
	help := km.FullHelp()

	if len(help) == 0 {
		t.Error("expected non-empty full help")
	}

	// Should have one group with 3 bindings: up, down, quit
	if len(help) != 1 {
		t.Errorf("expected 1 help group, got %d", len(help))
	}
	if len(help[0]) != 3 {
		t.Errorf("expected 3 bindings in help group, got %d", len(help[0]))
	}
}

func TestModel_SetPrompt(t *testing.T) {
	m := NewModel()
	m = updateModel(m, tea.WindowSizeMsg{Width: 100, Height: 40})
	m.SetPrompt("My test prompt")

	content := m.feedPanel.Content()
	if !strings.Contains(content, "My test prompt") {
		t.Errorf("expected feed to contain prompt 'My test prompt', got '%s'", content)
	}
	// Styled dividers use ─── instead of ---
	if !strings.Contains(content, "Prompt") {
		t.Errorf("expected feed to contain prompt delimiter, got '%s'", content)
	}
}

func TestModel_IsCompleted(t *testing.T) {
	m := NewModel()

	if m.IsCompleted() {
		t.Error("expected IsCompleted() to be false initially")
	}

	m.completed = true
	if !m.IsCompleted() {
		t.Error("expected IsCompleted() to be true after setting completed")
	}
}

func TestModel_Error(t *testing.T) {
	m := NewModel()

	if m.Error() != nil {
		t.Error("expected Error() to be nil initially")
	}

	m = updateModel(m, tea.WindowSizeMsg{Width: 100, Height: 40})
	msg := SetErrorMsg{Error: "test error"}
	m = updateModel(m, msg)

	if m.Error() == nil {
		t.Error("expected Error() to be non-nil after SetErrorMsg")
	}
}

func TestModel_ScrollKeys(t *testing.T) {
	m := NewModel()
	m = updateModel(m, tea.WindowSizeMsg{Width: 100, Height: 40})

	// Add some content so we can scroll
	m.feedPanel.SetContent(strings.Repeat("Line\n", 100))

	// Test scroll down (arrow key)
	m = updateModel(m, tea.KeyMsg{Type: tea.KeyDown})

	// Test scroll up (arrow key)
	m = updateModel(m, tea.KeyMsg{Type: tea.KeyUp})

	// Verify scrolling works (no errors)
	if m.feedPanel == nil {
		t.Error("expected feedPanel to still be valid after scrolling")
	}
}

func TestModel_LoopEventMsg(t *testing.T) {
	events := make(chan loop.Event, 10)
	m := NewModelWithEvents(events)
	m = updateModel(m, tea.WindowSizeMsg{Width: 100, Height: 40})

	// Send a LoopEventMsg directly
	loopMsg := LoopEventMsg{
		Event: loop.Event{
			Type:      loop.EventIterationStart,
			Iteration: 1,
			MaxIter:   10,
			Message:   "Starting iteration 1",
		},
	}
	m = updateModel(m, loopMsg)

	if m.iteration != 1 {
		t.Errorf("expected iteration 1, got %d", m.iteration)
	}

	if m.maxIter != 10 {
		t.Errorf("expected maxIter 10, got %d", m.maxIter)
	}

	close(events)
}

func TestModel_SetStatusMsg(t *testing.T) {
	m := NewModel()
	m = updateModel(m, tea.WindowSizeMsg{Width: 100, Height: 40})

	msg := SetStatusMsg{Status: "Custom Status"}
	m = updateModel(m, msg)

	if m.status != "Custom Status" {
		t.Errorf("expected status 'Custom Status', got '%s'", m.status)
	}
}

func TestModel_HandleClaudeEvent_EventMessage_Fallback(t *testing.T) {
	events := make(chan loop.Event, 10)
	m := NewModelWithEvents(events)
	m = updateModel(m, tea.WindowSizeMsg{Width: 100, Height: 40})

	// When streaming hasn't produced any output, EventMessage should be shown as fallback
	event := loop.Event{
		Type:      loop.EventClaudeStream,
		Iteration: 1,
		MaxIter:   5,
		ClaudeEvent: &claude.StreamEvent{
			Type: claude.EventMessage,
			Message: &claude.MessageContent{
				Text: "Complete message from Claude",
			},
		},
	}
	m.handleLoopEvent(event)

	outputContent := m.feedPanel.Content()
	if outputContent != "Complete message from Claude" {
		t.Errorf("expected fallback message to be shown, got '%s'", outputContent)
	}

	close(events)
}

func TestModel_HandleClaudeEvent_EventMessage_NoDuplication(t *testing.T) {
	events := make(chan loop.Event, 10)
	m := NewModelWithEvents(events)
	m = updateModel(m, tea.WindowSizeMsg{Width: 100, Height: 40})

	// First, stream some text
	streamEvent := loop.Event{
		Type:      loop.EventClaudeStream,
		Iteration: 1,
		MaxIter:   5,
		ClaudeEvent: &claude.StreamEvent{
			Type: claude.EventAssistantText,
			AssistantText: &claude.AssistantTextContent{
				Text: "Streamed text",
			},
		},
	}
	m.handleLoopEvent(streamEvent)

	// Now send EventMessage with the same text - it should be skipped
	messageEvent := loop.Event{
		Type:      loop.EventClaudeStream,
		Iteration: 1,
		MaxIter:   5,
		ClaudeEvent: &claude.StreamEvent{
			Type: claude.EventMessage,
			Message: &claude.MessageContent{
				Text: "Streamed text",
			},
		},
	}
	m.handleLoopEvent(messageEvent)

	// Should only contain the streamed text once
	outputContent := m.feedPanel.Content()
	if outputContent != "Streamed text" {
		t.Errorf("expected no duplication, got '%s'", outputContent)
	}

	close(events)
}

func TestModel_HandleClaudeEvent_EventToolUse(t *testing.T) {
	events := make(chan loop.Event, 10)
	m := NewModelWithEvents(events)
	m = updateModel(m, tea.WindowSizeMsg{Width: 100, Height: 40})

	event := loop.Event{
		Type:      loop.EventClaudeStream,
		Iteration: 1,
		MaxIter:   5,
		ClaudeEvent: &claude.StreamEvent{
			Type: claude.EventToolUse,
			ToolUse: &claude.ToolUseContent{
				ID:    "tool_123",
				Name:  "Read",
				Input: []byte(`{"file_path": "/path/to/file.go"}`),
			},
		},
	}
	m.handleLoopEvent(event)

	outputContent := m.feedPanel.Content()
	// Styled tool display uses ▶ icon instead of brackets
	if !strings.Contains(outputContent, "Read") {
		t.Errorf("expected tool use display, got '%s'", outputContent)
	}
	if !strings.Contains(outputContent, "/path/to/file.go") {
		t.Errorf("expected file path in tool use display, got '%s'", outputContent)
	}

	close(events)
}

func TestModel_HandleClaudeEvent_EventToolUse_WithPrecedingText(t *testing.T) {
	events := make(chan loop.Event, 10)
	m := NewModelWithEvents(events)
	m = updateModel(m, tea.WindowSizeMsg{Width: 100, Height: 40})

	// Tool use with preceding text in Message field
	event := loop.Event{
		Type:      loop.EventClaudeStream,
		Iteration: 1,
		MaxIter:   5,
		ClaudeEvent: &claude.StreamEvent{
			Type: claude.EventToolUse,
			Message: &claude.MessageContent{
				Text: "Let me read that file.",
			},
			ToolUse: &claude.ToolUseContent{
				ID:    "tool_456",
				Name:  "Read",
				Input: []byte(`{"file_path": "README.md"}`),
			},
		},
	}
	m.handleLoopEvent(event)

	outputContent := m.feedPanel.Content()
	if !strings.Contains(outputContent, "Let me read that file.") {
		t.Errorf("expected preceding text to be shown, got '%s'", outputContent)
	}
	// Styled tool display uses ▶ icon instead of brackets
	if !strings.Contains(outputContent, "Read") {
		t.Errorf("expected tool use display, got '%s'", outputContent)
	}

	close(events)
}

func TestModel_StreamedBytesReset(t *testing.T) {
	events := make(chan loop.Event, 10)
	m := NewModelWithEvents(events)
	m = updateModel(m, tea.WindowSizeMsg{Width: 100, Height: 40})

	// Stream some text
	streamEvent := loop.Event{
		Type:      loop.EventClaudeStream,
		Iteration: 1,
		MaxIter:   5,
		ClaudeEvent: &claude.StreamEvent{
			Type: claude.EventAssistantText,
			AssistantText: &claude.AssistantTextContent{
				Text: "Some text",
			},
		},
	}
	m.handleLoopEvent(streamEvent)

	// Start a new iteration - should reset streamedBytes
	iterationStartEvent := loop.Event{
		Type:      loop.EventIterationStart,
		Iteration: 2,
		MaxIter:   5,
		Message:   "Starting iteration 2",
	}
	m.handleLoopEvent(iterationStartEvent)

	// Clear output for clean test
	m.feedPanel.Clear()

	// Now EventMessage should be shown as fallback (since streamedBytes reset)
	messageEvent := loop.Event{
		Type:      loop.EventClaudeStream,
		Iteration: 2,
		MaxIter:   5,
		ClaudeEvent: &claude.StreamEvent{
			Type: claude.EventMessage,
			Message: &claude.MessageContent{
				Text: "Fallback message for iteration 2",
			},
		},
	}
	m.handleLoopEvent(messageEvent)

	outputContent := m.feedPanel.Content()
	if outputContent != "Fallback message for iteration 2" {
		t.Errorf("expected fallback message after iteration reset, got '%s'", outputContent)
	}

	close(events)
}

func TestFormatToolUse(t *testing.T) {
	tests := []struct {
		name         string
		tool         *claude.ToolUseContent
		containsName string
		containsPath string
	}{
		{
			name:         "nil tool",
			tool:         nil,
			containsName: "",
			containsPath: "",
		},
		{
			name: "tool with file_path",
			tool: &claude.ToolUseContent{
				Name:  "Read",
				Input: []byte(`{"file_path": "/path/to/file.go"}`),
			},
			containsName: "Read",
			containsPath: "/path/to/file.go",
		},
		{
			name: "tool with command",
			tool: &claude.ToolUseContent{
				Name:  "Bash",
				Input: []byte(`{"command": "go test ./..."}`),
			},
			containsName: "Bash",
			containsPath: "go test ./...",
		},
		{
			name: "tool with no matching params",
			tool: &claude.ToolUseContent{
				Name:  "Custom",
				Input: []byte(`{"foo": "bar"}`),
			},
			containsName: "Custom",
			containsPath: "",
		},
		{
			name: "tool with empty input",
			tool: &claude.ToolUseContent{
				Name:  "Empty",
				Input: []byte{},
			},
			containsName: "Empty",
			containsPath: "",
		},
		{
			name: "tool with long value",
			tool: &claude.ToolUseContent{
				Name:  "Read",
				Input: []byte(`{"file_path": "/this/is/a/very/long/path/that/exceeds/sixty/characters/and/should/be/truncated/file.go"}`),
			},
			containsName: "Read",
			containsPath: "...", // Truncated values end with ...
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := formatToolUse(tt.tool)
			if tt.tool == nil {
				if result != "" {
					t.Errorf("formatToolUse() = %q, expected empty string for nil tool", result)
				}
				return
			}
			if !strings.Contains(result, tt.containsName) {
				t.Errorf("formatToolUse() = %q, expected to contain tool name %q", result, tt.containsName)
			}
			if tt.containsPath != "" && !strings.Contains(result, tt.containsPath) {
				t.Errorf("formatToolUse() = %q, expected to contain %q", result, tt.containsPath)
			}
		})
	}
}

func TestFloatingWindow_Visibility(t *testing.T) {
	fw := NewFloatingWindow("Test Title")
	fw.SetSize(100, 40)

	// Initially hidden
	if fw.IsVisible() {
		t.Error("expected floating window to be hidden initially")
	}

	// Show it
	fw.Show("Test content")
	if !fw.IsVisible() {
		t.Error("expected floating window to be visible after Show()")
	}

	// Hide it
	fw.Hide()
	if fw.IsVisible() {
		t.Error("expected floating window to be hidden after Hide()")
	}
}

func TestFloatingWindow_View(t *testing.T) {
	fw := NewFloatingWindow("Completed")
	fw.SetSize(100, 40)

	// Hidden window returns empty view
	view := fw.View()
	if view != "" {
		t.Errorf("expected empty view when hidden, got '%s'", view)
	}

	// Visible window returns content
	fw.Show("Task completed successfully!")
	view = fw.View()
	if view == "" {
		t.Error("expected non-empty view when visible")
	}
	if !strings.Contains(view, "Completed") {
		t.Error("expected view to contain title 'Completed'")
	}
	if !strings.Contains(view, "close") {
		t.Error("expected view to contain key hints")
	}
}

func TestFloatingWindow_Scrolling(t *testing.T) {
	fw := NewFloatingWindow("Test")
	fw.SetSize(100, 40)

	// Add scrollable content
	longContent := strings.Repeat("Line of content\n", 50)
	fw.Show(longContent)

	// Scroll operations should not panic
	fw.ScrollDown(5)
	fw.ScrollUp(3)

	// Window should still be visible
	if !fw.IsVisible() {
		t.Error("expected floating window to remain visible after scrolling")
	}
}

func TestModel_FloatingWindow_DismissWithEnter(t *testing.T) {
	m := NewModel()
	m = updateModel(m, tea.WindowSizeMsg{Width: 100, Height: 40})

	// Show the floating window
	m.floatingWindow.Show("Test content")
	if !m.floatingWindow.IsVisible() {
		t.Error("expected floating window to be visible")
	}

	// Press Enter to dismiss
	m = updateModel(m, tea.KeyMsg{Type: tea.KeyEnter})

	if m.floatingWindow.IsVisible() {
		t.Error("expected floating window to be dismissed after Enter key")
	}
}

func TestModel_FloatingWindow_DismissWithEsc(t *testing.T) {
	m := NewModel()
	m = updateModel(m, tea.WindowSizeMsg{Width: 100, Height: 40})

	// Show the floating window
	m.floatingWindow.Show("Test content")

	// Press Esc to dismiss
	m = updateModel(m, tea.KeyMsg{Type: tea.KeyEscape})

	if m.floatingWindow.IsVisible() {
		t.Error("expected floating window to be dismissed after Esc key")
	}
}

func TestModel_FloatingWindow_ScrollWhileVisible(t *testing.T) {
	m := NewModel()
	m = updateModel(m, tea.WindowSizeMsg{Width: 100, Height: 40})

	// Add content to feed panel
	m.feedPanel.SetContent(strings.Repeat("Feed line\n", 100))

	// Show the floating window with scrollable content
	longContent := strings.Repeat("Floating content line\n", 50)
	m.floatingWindow.Show(longContent)

	// Scroll down - should scroll floating window, not feed panel
	m = updateModel(m, tea.KeyMsg{Type: tea.KeyDown})

	// Window should still be visible
	if !m.floatingWindow.IsVisible() {
		t.Error("expected floating window to remain visible during scrolling")
	}
}

func TestModel_ShowCompletionWindow_WithProgress(t *testing.T) {
	events := make(chan loop.Event, 10)
	m := NewModelWithEvents(events)
	m = updateModel(m, tea.WindowSizeMsg{Width: 100, Height: 40})

	// Simulate progress being tracked
	outputEvent := loop.Event{
		Type:      loop.EventClaudeOutput,
		Iteration: 3,
		MaxIter:   10,
		Output:    "## Progress\nImplemented the new feature\n\n## Learnings\nFound a useful pattern",
	}
	m.handleLoopEvent(outputEvent)

	// Verify progress was tracked
	if m.lastProgress != "Implemented the new feature" {
		t.Errorf("expected lastProgress to be set, got '%s'", m.lastProgress)
	}

	// Trigger completion
	doneEvent := loop.Event{
		Type:      loop.EventDone,
		Iteration: 3,
		MaxIter:   10,
	}
	m.handleLoopEvent(doneEvent)

	// Floating window should be visible with summary
	if !m.floatingWindow.IsVisible() {
		t.Error("expected floating window to be visible after completion")
	}

	view := m.floatingWindow.View()
	if !strings.Contains(view, "Implemented the new feature") {
		t.Errorf("expected completion window to contain progress summary, got '%s'", view)
	}

	close(events)
}

func TestModel_View_OverlaysFloatingWindow(t *testing.T) {
	m := NewModel()
	m = updateModel(m, tea.WindowSizeMsg{Width: 100, Height: 40})

	// Add content to feed panel
	m.feedPanel.SetContent("Base content from feed panel")

	// View without floating window
	viewWithoutFloat := m.View()

	// Show floating window
	m.floatingWindow.Show("Floating window content here")

	// View with floating window should be different
	viewWithFloat := m.View()

	if viewWithFloat == viewWithoutFloat {
		t.Error("expected view to change when floating window is shown")
	}
}

func TestExtractMainParam(t *testing.T) {
	tests := []struct {
		name     string
		input    []byte
		expected string
	}{
		{
			name:     "empty input",
			input:    []byte{},
			expected: "",
		},
		{
			name:     "invalid json",
			input:    []byte(`not json`),
			expected: "",
		},
		{
			name:     "file_path param",
			input:    []byte(`{"file_path": "/path/file.go"}`),
			expected: "/path/file.go",
		},
		{
			name:     "path param",
			input:    []byte(`{"path": "/some/path"}`),
			expected: "/some/path",
		},
		{
			name:     "command param",
			input:    []byte(`{"command": "ls -la"}`),
			expected: "ls -la",
		},
		{
			name:     "query param",
			input:    []byte(`{"query": "search term"}`),
			expected: "search term",
		},
		{
			name:     "pattern param",
			input:    []byte(`{"pattern": "*.go"}`),
			expected: "*.go",
		},
		{
			name:     "url param",
			input:    []byte(`{"url": "https://example.com"}`),
			expected: "https://example.com",
		},
		{
			name:     "content param",
			input:    []byte(`{"content": "file contents"}`),
			expected: "file contents",
		},
		{
			name:     "non-string value",
			input:    []byte(`{"path": 123}`),
			expected: "",
		},
		{
			name:     "long value truncation",
			input:    []byte(`{"path": "this is a very long string that should be truncated because it exceeds sixty characters"}`),
			expected: "this is a very long string that should be truncated becau...",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := extractMainParam(tt.input)
			if result != tt.expected {
				t.Errorf("extractMainParam() = %q, expected %q", result, tt.expected)
			}
		})
	}
}
