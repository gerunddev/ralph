// Package claude provides a wrapper for the Claude CLI and handles streaming output.
package claude

import (
	"bufio"
	"encoding/json"
	"fmt"
	"io"
)

// Parser parses Claude's stream-JSON output format.
type Parser struct {
	scanner *bufio.Scanner
}

// NewParser creates a new stream-JSON parser.
func NewParser(r io.Reader) *Parser {
	scanner := bufio.NewScanner(r)
	// Set a larger buffer for potentially large JSON lines
	const maxScannerBuffer = 10 * 1024 * 1024 // 10MB
	buf := make([]byte, 0, 64*1024)
	scanner.Buffer(buf, maxScannerBuffer)

	return &Parser{
		scanner: scanner,
	}
}

// Next returns the next event from the stream.
// Returns io.EOF when the stream is exhausted.
func (p *Parser) Next() (*StreamEvent, error) {
	if !p.scanner.Scan() {
		if err := p.scanner.Err(); err != nil {
			return nil, fmt.Errorf("scanner error: %w", err)
		}
		return nil, io.EOF
	}

	line := p.scanner.Bytes()
	if len(line) == 0 {
		// Skip empty lines
		return p.Next()
	}

	return p.parseLine(line)
}

// parseLine parses a single JSON line into a StreamEvent.
func (p *Parser) parseLine(line []byte) (*StreamEvent, error) {
	// First pass: determine the event type
	var raw rawEvent
	if err := json.Unmarshal(line, &raw); err != nil {
		return nil, fmt.Errorf("failed to parse JSON: %w", err)
	}

	event := &StreamEvent{
		Raw: append([]byte(nil), line...), // Copy the raw bytes
	}

	// Determine event type based on content
	switch {
	case raw.Message != nil:
		// This is a message event - check for tool_use content
		return p.parseMessageEvent(event, &raw)

	case raw.Type == "init":
		event.Type = EventInit
		event.Init = &InitContent{
			SessionID:  raw.SessionID,
			Model:      raw.Model,
			CWD:        raw.CWD,
			Tools:      raw.Tools,
			MCPServers: raw.MCPServers,
		}

	case raw.Type == "result":
		event.Type = EventResult
		usage := Usage{}
		if raw.TotalUsage != nil {
			usage = *raw.TotalUsage
		}
		event.Result = &ResultContent{
			SessionID:   raw.SessionID,
			CostUSD:     raw.CostUSD,
			DurationMS:  raw.DurationMS,
			DurationAPI: raw.DurationAPI,
			NumTurns:    raw.NumTurns,
			TotalUsage:  usage,
			Result:      raw.Result,
			SubAgent:    raw.SubAgent,
		}

	case raw.Type == "error" || raw.Error != nil:
		event.Type = EventError
		if raw.Error != nil {
			event.Error = raw.Error
		} else {
			event.Error = &ErrorContent{
				Message: "unknown error",
			}
		}

	case raw.Type == "system":
		event.Type = EventSystem
		event.System = &SystemContent{
			SubType: raw.SubType,
		}

	default:
		// Unknown event type - still preserve raw data
		event.Type = EventType(raw.Type)
		if event.Type == "" {
			event.Type = "unknown"
		}
	}

	return event, nil
}

// parseMessageEvent handles message events which may contain text or tool_use content.
func (p *Parser) parseMessageEvent(event *StreamEvent, raw *rawEvent) (*StreamEvent, error) {
	msg := raw.Message

	// Check for tool_use or tool_result content blocks
	var textParts []string
	var toolUse *ToolUseContent
	var toolResult *ToolResultContent

	for _, content := range msg.Content {
		switch content.Type {
		case "text":
			textParts = append(textParts, content.Text)
		case "tool_use":
			toolUse = &ToolUseContent{
				ID:    content.ID,
				Name:  content.Name,
				Input: content.Input,
			}
		case "tool_result":
			toolResult = &ToolResultContent{
				ToolUseID: content.ID,
				Content:   content.Text,
			}
		}
	}

	// If there's a tool_use, report it as a tool_use event
	if toolUse != nil {
		event.Type = EventToolUse
		event.ToolUse = toolUse
		// Also include the message context
		event.Message = &MessageContent{
			ID:         msg.ID,
			Role:       msg.Role,
			Model:      msg.Model,
			StopReason: msg.StopReason,
			Usage:      msg.Usage,
		}
		return event, nil
	}

	// If there's a tool_result, report it as a tool_result event
	if toolResult != nil {
		event.Type = EventToolResult
		event.ToolResult = toolResult
		return event, nil
	}

	// Otherwise it's a text message
	event.Type = EventMessage
	text := ""
	if len(textParts) > 0 {
		text = textParts[0]
		for i := 1; i < len(textParts); i++ {
			text += "\n" + textParts[i]
		}
	}

	event.Message = &MessageContent{
		ID:         msg.ID,
		Role:       msg.Role,
		Model:      msg.Model,
		Text:       text,
		StopReason: msg.StopReason,
		Usage:      msg.Usage,
	}

	return event, nil
}
