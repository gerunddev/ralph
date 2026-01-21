package claude

import (
	"io"
	"strings"
	"testing"
)

// =============================================================================
// Parser Tests - Init Event
// =============================================================================

func TestParser_InitEvent(t *testing.T) {
	input := `{"type":"init","session_id":"abc123","model":"claude-sonnet-4-20250514","cwd":"/home/user","tools":10,"mcp_servers":2}`

	parser := NewParser(strings.NewReader(input))
	event, err := parser.Next()
	if err != nil {
		t.Fatalf("Next() returned error: %v", err)
	}

	if event.Type != EventInit {
		t.Errorf("event.Type = %v, want %v", event.Type, EventInit)
	}

	if event.Init == nil {
		t.Fatal("event.Init is nil")
	}

	if event.Init.SessionID != "abc123" {
		t.Errorf("Init.SessionID = %q, want %q", event.Init.SessionID, "abc123")
	}
	if event.Init.Model != "claude-sonnet-4-20250514" {
		t.Errorf("Init.Model = %q, want %q", event.Init.Model, "claude-sonnet-4-20250514")
	}
	if event.Init.CWD != "/home/user" {
		t.Errorf("Init.CWD = %q, want %q", event.Init.CWD, "/home/user")
	}
	if event.Init.Tools != 10 {
		t.Errorf("Init.Tools = %d, want %d", event.Init.Tools, 10)
	}
	if event.Init.MCPServers != 2 {
		t.Errorf("Init.MCPServers = %d, want %d", event.Init.MCPServers, 2)
	}

	// Verify raw JSON is preserved
	if string(event.Raw) != input {
		t.Errorf("event.Raw = %q, want %q", string(event.Raw), input)
	}
}

func TestParser_InitEvent_ToolsAsArray(t *testing.T) {
	// Test when tools and mcp_servers are arrays instead of integers
	input := `{"type":"init","session_id":"abc123","model":"claude-sonnet-4-20250514","cwd":"/home/user","tools":["Read","Write","Bash","Grep","Glob"],"mcp_servers":["server1","server2"]}`

	parser := NewParser(strings.NewReader(input))
	event, err := parser.Next()
	if err != nil {
		t.Fatalf("Next() returned error: %v", err)
	}

	if event.Type != EventInit {
		t.Errorf("event.Type = %v, want %v", event.Type, EventInit)
	}

	if event.Init == nil {
		t.Fatal("event.Init is nil")
	}

	// Should count array elements
	if event.Init.Tools != 5 {
		t.Errorf("Init.Tools = %d, want %d (count of array elements)", event.Init.Tools, 5)
	}
	if event.Init.MCPServers != 2 {
		t.Errorf("Init.MCPServers = %d, want %d (count of array elements)", event.Init.MCPServers, 2)
	}
}

// =============================================================================
// Parser Tests - Message Event
// =============================================================================

func TestParser_MessageEvent_TextContent(t *testing.T) {
	input := `{"message":{"id":"msg_123","role":"assistant","model":"claude-sonnet-4-20250514","content":[{"type":"text","text":"Hello, world!"}],"stop_reason":"end_turn","usage":{"input_tokens":100,"output_tokens":50}}}`

	parser := NewParser(strings.NewReader(input))
	event, err := parser.Next()
	if err != nil {
		t.Fatalf("Next() returned error: %v", err)
	}

	if event.Type != EventMessage {
		t.Errorf("event.Type = %v, want %v", event.Type, EventMessage)
	}

	if event.Message == nil {
		t.Fatal("event.Message is nil")
	}

	if event.Message.ID != "msg_123" {
		t.Errorf("Message.ID = %q, want %q", event.Message.ID, "msg_123")
	}
	if event.Message.Role != "assistant" {
		t.Errorf("Message.Role = %q, want %q", event.Message.Role, "assistant")
	}
	if event.Message.Text != "Hello, world!" {
		t.Errorf("Message.Text = %q, want %q", event.Message.Text, "Hello, world!")
	}
	if event.Message.StopReason != "end_turn" {
		t.Errorf("Message.StopReason = %q, want %q", event.Message.StopReason, "end_turn")
	}
	if event.Message.Usage.InputTokens != 100 {
		t.Errorf("Message.Usage.InputTokens = %d, want %d", event.Message.Usage.InputTokens, 100)
	}
	if event.Message.Usage.OutputTokens != 50 {
		t.Errorf("Message.Usage.OutputTokens = %d, want %d", event.Message.Usage.OutputTokens, 50)
	}
}

func TestParser_MessageEvent_MultipleTextBlocks(t *testing.T) {
	input := `{"message":{"id":"msg_123","role":"assistant","content":[{"type":"text","text":"First part"},{"type":"text","text":"Second part"}]}}`

	parser := NewParser(strings.NewReader(input))
	event, err := parser.Next()
	if err != nil {
		t.Fatalf("Next() returned error: %v", err)
	}

	if event.Type != EventMessage {
		t.Errorf("event.Type = %v, want %v", event.Type, EventMessage)
	}

	expectedText := "First part\nSecond part"
	if event.Message.Text != expectedText {
		t.Errorf("Message.Text = %q, want %q", event.Message.Text, expectedText)
	}
}

// =============================================================================
// Parser Tests - Tool Use Event
// =============================================================================

func TestParser_ToolUseEvent(t *testing.T) {
	input := `{"message":{"id":"msg_123","role":"assistant","content":[{"type":"tool_use","id":"tool_456","name":"read_file","input":{"path":"/tmp/test.txt"}}],"stop_reason":"tool_use"}}`

	parser := NewParser(strings.NewReader(input))
	event, err := parser.Next()
	if err != nil {
		t.Fatalf("Next() returned error: %v", err)
	}

	if event.Type != EventToolUse {
		t.Errorf("event.Type = %v, want %v", event.Type, EventToolUse)
	}

	if event.ToolUse == nil {
		t.Fatal("event.ToolUse is nil")
	}

	if event.ToolUse.ID != "tool_456" {
		t.Errorf("ToolUse.ID = %q, want %q", event.ToolUse.ID, "tool_456")
	}
	if event.ToolUse.Name != "read_file" {
		t.Errorf("ToolUse.Name = %q, want %q", event.ToolUse.Name, "read_file")
	}

	// Verify input is preserved as raw JSON
	expectedInput := `{"path":"/tmp/test.txt"}`
	if string(event.ToolUse.Input) != expectedInput {
		t.Errorf("ToolUse.Input = %q, want %q", string(event.ToolUse.Input), expectedInput)
	}

	// Message context should also be set
	if event.Message == nil {
		t.Error("event.Message should be set for tool_use events")
	}
	if event.Message != nil && event.Message.StopReason != "tool_use" {
		t.Errorf("Message.StopReason = %q, want %q", event.Message.StopReason, "tool_use")
	}
}

// =============================================================================
// Parser Tests - Result Event
// =============================================================================

func TestParser_ResultEvent(t *testing.T) {
	input := `{"type":"result","session_id":"abc123","cost_usd":0.0123,"duration_ms":5000,"duration_api_ms":4500,"num_turns":3,"usage":{"input_tokens":1000,"output_tokens":500},"result":"Task completed successfully"}`

	parser := NewParser(strings.NewReader(input))
	event, err := parser.Next()
	if err != nil {
		t.Fatalf("Next() returned error: %v", err)
	}

	if event.Type != EventResult {
		t.Errorf("event.Type = %v, want %v", event.Type, EventResult)
	}

	if event.Result == nil {
		t.Fatal("event.Result is nil")
	}

	if event.Result.SessionID != "abc123" {
		t.Errorf("Result.SessionID = %q, want %q", event.Result.SessionID, "abc123")
	}
	if event.Result.CostUSD != 0.0123 {
		t.Errorf("Result.CostUSD = %f, want %f", event.Result.CostUSD, 0.0123)
	}
	if event.Result.DurationMS != 5000 {
		t.Errorf("Result.DurationMS = %d, want %d", event.Result.DurationMS, 5000)
	}
	if event.Result.DurationAPI != 4500 {
		t.Errorf("Result.DurationAPI = %d, want %d", event.Result.DurationAPI, 4500)
	}
	if event.Result.NumTurns != 3 {
		t.Errorf("Result.NumTurns = %d, want %d", event.Result.NumTurns, 3)
	}
	if event.Result.TotalUsage.InputTokens != 1000 {
		t.Errorf("Result.TotalUsage.InputTokens = %d, want %d", event.Result.TotalUsage.InputTokens, 1000)
	}
	if event.Result.Result != "Task completed successfully" {
		t.Errorf("Result.Result = %q, want %q", event.Result.Result, "Task completed successfully")
	}
}

// =============================================================================
// Parser Tests - Error Event
// =============================================================================

func TestParser_ErrorEvent(t *testing.T) {
	input := `{"type":"error","error":{"code":"rate_limit","message":"Rate limit exceeded"}}`

	parser := NewParser(strings.NewReader(input))
	event, err := parser.Next()
	if err != nil {
		t.Fatalf("Next() returned error: %v", err)
	}

	if event.Type != EventError {
		t.Errorf("event.Type = %v, want %v", event.Type, EventError)
	}

	if event.Error == nil {
		t.Fatal("event.Error is nil")
	}

	if event.Error.Code != "rate_limit" {
		t.Errorf("Error.Code = %q, want %q", event.Error.Code, "rate_limit")
	}
	if event.Error.Message != "Rate limit exceeded" {
		t.Errorf("Error.Message = %q, want %q", event.Error.Message, "Rate limit exceeded")
	}
}

// =============================================================================
// Parser Tests - System Event
// =============================================================================

func TestParser_SystemEvent(t *testing.T) {
	input := `{"type":"system","subtype":"progress"}`

	parser := NewParser(strings.NewReader(input))
	event, err := parser.Next()
	if err != nil {
		t.Fatalf("Next() returned error: %v", err)
	}

	if event.Type != EventSystem {
		t.Errorf("event.Type = %v, want %v", event.Type, EventSystem)
	}

	if event.System == nil {
		t.Fatal("event.System is nil")
	}

	if event.System.SubType != "progress" {
		t.Errorf("System.SubType = %q, want %q", event.System.SubType, "progress")
	}
}

// =============================================================================
// Parser Tests - Multiple Events
// =============================================================================

func TestParser_MultipleEvents(t *testing.T) {
	input := `{"type":"init","session_id":"abc123"}
{"message":{"id":"msg_1","role":"assistant","content":[{"type":"text","text":"Hello"}]}}
{"type":"result","session_id":"abc123","cost_usd":0.01}`

	parser := NewParser(strings.NewReader(input))

	// First event - init
	event1, err := parser.Next()
	if err != nil {
		t.Fatalf("Next() 1 returned error: %v", err)
	}
	if event1.Type != EventInit {
		t.Errorf("event1.Type = %v, want %v", event1.Type, EventInit)
	}

	// Second event - message
	event2, err := parser.Next()
	if err != nil {
		t.Fatalf("Next() 2 returned error: %v", err)
	}
	if event2.Type != EventMessage {
		t.Errorf("event2.Type = %v, want %v", event2.Type, EventMessage)
	}

	// Third event - result
	event3, err := parser.Next()
	if err != nil {
		t.Fatalf("Next() 3 returned error: %v", err)
	}
	if event3.Type != EventResult {
		t.Errorf("event3.Type = %v, want %v", event3.Type, EventResult)
	}

	// Fourth call should return EOF
	_, err = parser.Next()
	if err != io.EOF {
		t.Errorf("Next() 4 returned %v, want io.EOF", err)
	}
}

// =============================================================================
// Parser Tests - Empty Lines
// =============================================================================

func TestParser_SkipsEmptyLines(t *testing.T) {
	input := `{"type":"init","session_id":"abc"}

{"type":"result","session_id":"abc"}
`

	parser := NewParser(strings.NewReader(input))

	// First event
	event1, err := parser.Next()
	if err != nil {
		t.Fatalf("Next() 1 returned error: %v", err)
	}
	if event1.Type != EventInit {
		t.Errorf("event1.Type = %v, want %v", event1.Type, EventInit)
	}

	// Second event (should skip empty line)
	event2, err := parser.Next()
	if err != nil {
		t.Fatalf("Next() 2 returned error: %v", err)
	}
	if event2.Type != EventResult {
		t.Errorf("event2.Type = %v, want %v", event2.Type, EventResult)
	}
}

// =============================================================================
// Parser Tests - Error Handling
// =============================================================================

func TestParser_InvalidJSON(t *testing.T) {
	input := `not valid json`

	parser := NewParser(strings.NewReader(input))
	_, err := parser.Next()
	if err == nil {
		t.Error("Next() should return error for invalid JSON")
	}
}

func TestParser_EmptyStream(t *testing.T) {
	parser := NewParser(strings.NewReader(""))
	_, err := parser.Next()
	if err != io.EOF {
		t.Errorf("Next() = %v, want io.EOF", err)
	}
}

func TestParser_UnknownEventType(t *testing.T) {
	input := `{"type":"unknown_event"}`

	parser := NewParser(strings.NewReader(input))
	event, err := parser.Next()
	if err != nil {
		t.Fatalf("Next() returned error: %v", err)
	}

	// Should still parse and preserve the type
	if event.Type != "unknown_event" {
		t.Errorf("event.Type = %v, want %v", event.Type, "unknown_event")
	}

	// Raw should be preserved
	if string(event.Raw) != input {
		t.Errorf("event.Raw = %q, want %q", string(event.Raw), input)
	}
}

func TestParser_MissingType(t *testing.T) {
	input := `{"some_field":"some_value"}`

	parser := NewParser(strings.NewReader(input))
	event, err := parser.Next()
	if err != nil {
		t.Fatalf("Next() returned error: %v", err)
	}

	// Should get "unknown" type
	if event.Type != "unknown" {
		t.Errorf("event.Type = %v, want %v", event.Type, "unknown")
	}
}

// =============================================================================
// Parser Tests - Streaming Text (content_block_delta)
// =============================================================================

func TestParser_ContentBlockDelta_NestedFormat(t *testing.T) {
	// Format where content_block_delta is a nested object with delta.text
	input := `{"content_block_delta":{"type":"content_block_delta","delta":{"type":"text_delta","text":"Hello, "}}}`

	parser := NewParser(strings.NewReader(input))
	event, err := parser.Next()
	if err != nil {
		t.Fatalf("Next() returned error: %v", err)
	}

	if event.Type != EventAssistantText {
		t.Errorf("event.Type = %v, want %v", event.Type, EventAssistantText)
	}

	if event.AssistantText == nil {
		t.Fatal("event.AssistantText is nil")
	}

	if event.AssistantText.Text != "Hello, " {
		t.Errorf("AssistantText.Text = %q, want %q", event.AssistantText.Text, "Hello, ")
	}
}

func TestParser_ContentBlockDelta_TopLevelType(t *testing.T) {
	// Format where type is at top level and text is in nested content_block_delta
	input := `{"type":"content_block_delta","content_block_delta":{"delta":{"type":"text_delta","text":"world!"}}}`

	parser := NewParser(strings.NewReader(input))
	event, err := parser.Next()
	if err != nil {
		t.Fatalf("Next() returned error: %v", err)
	}

	if event.Type != EventAssistantText {
		t.Errorf("event.Type = %v, want %v", event.Type, EventAssistantText)
	}

	if event.AssistantText == nil {
		t.Fatal("event.AssistantText is nil")
	}

	if event.AssistantText.Text != "world!" {
		t.Errorf("AssistantText.Text = %q, want %q", event.AssistantText.Text, "world!")
	}
}

func TestParser_ContentBlockDelta_DirectText(t *testing.T) {
	// Alternative format where text is directly in content_block_delta
	input := `{"content_block_delta":{"type":"content_block_delta","text":"streaming chunk"}}`

	parser := NewParser(strings.NewReader(input))
	event, err := parser.Next()
	if err != nil {
		t.Fatalf("Next() returned error: %v", err)
	}

	if event.Type != EventAssistantText {
		t.Errorf("event.Type = %v, want %v", event.Type, EventAssistantText)
	}

	if event.AssistantText == nil {
		t.Fatal("event.AssistantText is nil")
	}

	if event.AssistantText.Text != "streaming chunk" {
		t.Errorf("AssistantText.Text = %q, want %q", event.AssistantText.Text, "streaming chunk")
	}
}

func TestParser_ContentBlockDelta_StreamingSequence(t *testing.T) {
	// Test a sequence of streaming events
	input := `{"type":"init","session_id":"abc"}
{"content_block_delta":{"delta":{"text":"Hello, "}}}
{"content_block_delta":{"delta":{"text":"how are "}}}
{"content_block_delta":{"delta":{"text":"you?"}}}
{"type":"result","session_id":"abc"}`

	parser := NewParser(strings.NewReader(input))

	// Init event
	event, err := parser.Next()
	if err != nil {
		t.Fatalf("Next() 1 returned error: %v", err)
	}
	if event.Type != EventInit {
		t.Errorf("event 1 Type = %v, want %v", event.Type, EventInit)
	}

	// First chunk
	event, err = parser.Next()
	if err != nil {
		t.Fatalf("Next() 2 returned error: %v", err)
	}
	if event.Type != EventAssistantText {
		t.Errorf("event 2 Type = %v, want %v", event.Type, EventAssistantText)
	}
	if event.AssistantText.Text != "Hello, " {
		t.Errorf("event 2 Text = %q, want %q", event.AssistantText.Text, "Hello, ")
	}

	// Second chunk
	event, err = parser.Next()
	if err != nil {
		t.Fatalf("Next() 3 returned error: %v", err)
	}
	if event.Type != EventAssistantText {
		t.Errorf("event 3 Type = %v, want %v", event.Type, EventAssistantText)
	}
	if event.AssistantText.Text != "how are " {
		t.Errorf("event 3 Text = %q, want %q", event.AssistantText.Text, "how are ")
	}

	// Third chunk
	event, err = parser.Next()
	if err != nil {
		t.Fatalf("Next() 4 returned error: %v", err)
	}
	if event.Type != EventAssistantText {
		t.Errorf("event 4 Type = %v, want %v", event.Type, EventAssistantText)
	}
	if event.AssistantText.Text != "you?" {
		t.Errorf("event 4 Text = %q, want %q", event.AssistantText.Text, "you?")
	}

	// Result event
	event, err = parser.Next()
	if err != nil {
		t.Fatalf("Next() 5 returned error: %v", err)
	}
	if event.Type != EventResult {
		t.Errorf("event 5 Type = %v, want %v", event.Type, EventResult)
	}
}

// =============================================================================
// Parser Tests - Raw JSON Preservation
// =============================================================================

func TestParser_RawJSONPreserved(t *testing.T) {
	inputs := []string{
		`{"type":"init","session_id":"abc","extra_field":"preserved"}`,
		`{"message":{"id":"msg","role":"assistant","content":[{"type":"text","text":"hi"}],"unknown":"field"}}`,
		`{"type":"result","session_id":"abc","cost_usd":0.01,"custom":true}`,
	}

	for _, input := range inputs {
		parser := NewParser(strings.NewReader(input))
		event, err := parser.Next()
		if err != nil {
			t.Errorf("Next() for %q returned error: %v", input, err)
			continue
		}

		if string(event.Raw) != input {
			t.Errorf("Raw JSON not preserved.\ngot:  %q\nwant: %q", string(event.Raw), input)
		}
	}
}
