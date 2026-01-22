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
	// EventAssistantText contains streaming assistant text (partial messages).
	EventAssistantText EventType = "assistant_text"
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
	Type          EventType
	Raw           []byte                // Original JSON for storage
	Init          *InitContent          // For init events
	Message       *MessageContent       // For message events
	AssistantText *AssistantTextContent // For streaming assistant text (partial messages)
	ToolUse       *ToolUseContent       // For tool_use events
	ToolResult    *ToolResultContent
	Result        *ResultContent // For result events
	Error         *ErrorContent
	System        *SystemContent // For system events
}

// InitContent contains initialization information for a session.
type InitContent struct {
	SessionID  string `json:"session_id"`
	Model      string `json:"model"`
	CWD        string `json:"cwd"`
	Tools      int    `json:"tools"`       // Count of tools available
	MCPServers int    `json:"mcp_servers"` // Count of MCP servers
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

// AssistantTextContent contains streaming assistant text from partial messages.
type AssistantTextContent struct {
	Text string `json:"text"` // The text delta/chunk
}

// Usage contains token usage information.
type Usage struct {
	InputTokens  int `json:"input_tokens"`
	OutputTokens int `json:"output_tokens"`
	CacheRead    int `json:"cache_read_input_tokens"`
	CacheCreate  int `json:"cache_creation_input_tokens"`
}

// ContextLimitPercent is the threshold at which to stop a session.
const ContextLimitPercent = 50.0

// DefaultContextWindow is the fallback context window size if model is unknown.
const DefaultContextWindow = 200000

// modelContextEntry represents a model prefix and its context window size.
type modelContextEntry struct {
	prefix      string
	contextSize int
}

// modelContextWindows defines model prefixes and their context window sizes.
// Since no prefix is a substring of another, iteration order doesn't affect matching.
// All current Claude models have 200K context windows.
var modelContextWindows = []modelContextEntry{
	{"claude-3-5-sonnet", 200000},
	{"claude-3-sonnet", 200000},
	{"claude-3-haiku", 200000},
	{"claude-3-opus", 200000},
	{"claude-sonnet-4", 200000},
	{"claude-haiku-4", 200000},
	{"claude-opus-4", 200000},
}

// GetContextWindowForModel returns the context window size for a given model name.
// It matches model name prefixes and returns DefaultContextWindow if unknown.
func GetContextWindowForModel(model string) int {
	for _, entry := range modelContextWindows {
		if len(model) >= len(entry.prefix) && model[:len(entry.prefix)] == entry.prefix {
			return entry.contextSize
		}
	}
	return DefaultContextWindow
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
	SessionID  string          `json:"session_id"`
	Model      string          `json:"model"`
	CWD        string          `json:"cwd"`
	Tools      json.RawMessage `json:"tools"`       // Can be int or array
	MCPServers json.RawMessage `json:"mcp_servers"` // Can be int or array

	// Message event - contains the full message object
	Message *rawMessage `json:"message"`

	// Partial message / streaming text fields (from --include-partial-messages)
	ContentBlockDelta *rawContentBlockDelta `json:"content_block_delta"`

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

// rawContentBlockDelta represents streaming content block delta.
type rawContentBlockDelta struct {
	Type  string `json:"type"`
	Text  string `json:"text"`
	Delta *struct {
		Type string `json:"type"`
		Text string `json:"text"`
	} `json:"delta"`
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
