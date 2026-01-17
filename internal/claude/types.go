// Package claude provides a wrapper for the Claude CLI and handles streaming output.
package claude

import "encoding/json"

// EventType represents the type of a stream event.
type EventType string

const (
	// EventInit is sent at the start of a session.
	EventInit EventType = "init"
	// EventMessage contains assistant message content.
	EventMessage EventType = "message"
	// EventToolUse indicates Claude is calling a tool.
	EventToolUse EventType = "tool_use"
	// EventToolResult contains the result of a tool call.
	EventToolResult EventType = "tool_result"
	// EventResult is sent at the end of a session with final status.
	EventResult EventType = "result"
	// EventError indicates an error occurred.
	EventError EventType = "error"
	// EventSystem is for system-level events.
	EventSystem EventType = "system"
)

// StreamEvent represents a parsed event from Claude's stream-JSON output.
type StreamEvent struct {
	Type       EventType
	Raw        []byte          // Original JSON for storage
	Init       *InitContent    // For init events
	Message    *MessageContent // For message events
	ToolUse    *ToolUseContent // For tool_use events
	ToolResult *ToolResultContent
	Result     *ResultContent // For result events
	Error      *ErrorContent
	System     *SystemContent // For system events
}

// InitContent contains initialization information for a session.
type InitContent struct {
	SessionID  string `json:"session_id"`
	Model      string `json:"model"`
	CWD        string `json:"cwd"`
	Tools      int    `json:"tools"`
	MCPServers int    `json:"mcp_servers"`
}

// MessageContent contains the content of an assistant message.
type MessageContent struct {
	ID         string `json:"id"`
	Role       string `json:"role"`
	Model      string `json:"model"`
	Text       string // Extracted text content
	StopReason string `json:"stop_reason"`
	Usage      Usage  `json:"usage"`
}

// Usage contains token usage information.
type Usage struct {
	InputTokens  int `json:"input_tokens"`
	OutputTokens int `json:"output_tokens"`
	CacheRead    int `json:"cache_read_input_tokens"`
	CacheCreate  int `json:"cache_creation_input_tokens"`
}

// ToolUseContent contains information about a tool being called.
type ToolUseContent struct {
	ID    string          `json:"id"`
	Name  string          `json:"name"`
	Input json.RawMessage `json:"input"`
}

// ToolResultContent contains the result of a tool execution.
type ToolResultContent struct {
	ToolUseID string `json:"tool_use_id"`
	Content   string `json:"content"`
	IsError   bool   `json:"is_error"`
}

// ResultContent contains the final result of a session.
type ResultContent struct {
	SessionID   string  `json:"session_id"`
	CostUSD     float64 `json:"cost_usd"`
	DurationMS  int64   `json:"duration_ms"`
	DurationAPI int64   `json:"duration_api_ms"`
	NumTurns    int     `json:"num_turns"`
	TotalUsage  Usage   `json:"usage"`
	Result      string  `json:"result"`
	SubAgent    bool    `json:"is_sub_agent"`
}

// ErrorContent contains error information.
type ErrorContent struct {
	Code    string `json:"code"`
	Message string `json:"message"`
}

// SystemContent contains system-level event information.
type SystemContent struct {
	SubType string `json:"subtype"`
	Message string `json:"message"`
}

// rawEvent is used for initial JSON parsing to determine event type.
type rawEvent struct {
	// Top-level type field (for init, result, error, system events)
	Type string `json:"type"`

	// Init event fields
	SessionID  string `json:"session_id"`
	Model      string `json:"model"`
	CWD        string `json:"cwd"`
	Tools      int    `json:"tools"`
	MCPServers int    `json:"mcp_servers"`

	// Message event - contains the full message object
	Message *rawMessage `json:"message"`

	// Result event fields
	CostUSD     float64 `json:"cost_usd"`
	DurationMS  int64   `json:"duration_ms"`
	DurationAPI int64   `json:"duration_api_ms"`
	NumTurns    int     `json:"num_turns"`
	TotalUsage  *Usage  `json:"usage"`
	Result      string  `json:"result"`
	SubAgent    bool    `json:"is_sub_agent"`

	// Error event fields
	Error *ErrorContent `json:"error"`

	// System event fields
	SubType string `json:"subtype"`
}

// rawMessage represents the message object in Claude's output.
type rawMessage struct {
	ID         string       `json:"id"`
	Role       string       `json:"role"`
	Model      string       `json:"model"`
	StopReason string       `json:"stop_reason"`
	Usage      Usage        `json:"usage"`
	Content    []rawContent `json:"content"`
}

// rawContent represents a content block within a message.
type rawContent struct {
	Type  string          `json:"type"`
	Text  string          `json:"text"`  // For text content
	ID    string          `json:"id"`    // For tool_use
	Name  string          `json:"name"`  // For tool_use
	Input json.RawMessage `json:"input"` // For tool_use
}
