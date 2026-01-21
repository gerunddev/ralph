# Task 3: Claude Client with Streaming

## Objective

Create a Claude CLI wrapper that runs sessions with `--output-format stream-json --verbose` and streams events.

## Requirements

1. Execute `claude -p --output-format stream-json --verbose <prompt>`
2. Support optional flags: `--model`, `--max-turns`
3. Stream events through a channel as they arrive
4. Parse each JSON line into typed events
5. Capture final result
6. Handle errors and cancellation gracefully

## Claude CLI Flags

```bash
claude -p \
  --output-format stream-json \
  --verbose \
  [--model <model>] \
  [--max-turns <n>] \
  "<prompt>"
```

## Event Types (from stream-json)

Key event types to handle:
- `system` - System messages
- `assistant` - Assistant text chunks
- `tool_use` - Tool invocations
- `tool_result` - Tool results
- `result` - Final result
- `error` - Errors

## Interface

```go
type ClientConfig struct {
    Model    string
    MaxTurns int
    Verbose  bool // Should always be true for V2
}

type Client struct { ... }

func NewClient(cfg ClientConfig) *Client

type Session struct { ... }

func (c *Client) Run(ctx context.Context, prompt string) (*Session, error)
func (s *Session) Events() <-chan StreamEvent
func (s *Session) Wait() error
func (s *Session) Cancel()

type StreamEvent struct {
    Type    EventType
    Raw     json.RawMessage // Original JSON for storage
    // Parsed fields for common types
    Message *MessageEvent
    Tool    *ToolEvent
    Result  *ResultEvent
    Error   *ErrorEvent
}
```

## Acceptance Criteria

- [ ] Executes claude CLI with correct flags
- [ ] Streams events as they arrive (no buffering until complete)
- [ ] Each event includes raw JSON for database storage
- [ ] Parses common event types into structured data
- [ ] Context cancellation stops the process
- [ ] Returns appropriate errors for CLI not found, non-zero exit
- [ ] Mock-friendly design for testing (CommandCreator pattern from V1)
- [ ] Unit tests with mocked command execution

## Files to Create/Modify

- `internal/claude/client.go` (adapt from V1)
- `internal/claude/parser.go` (adapt from V1)
- `internal/claude/types.go` (adapt from V1)
- `internal/claude/client_test.go`
- `internal/claude/parser_test.go`

## Notes

V1's claude client is very similar. Main changes:
- Always use `--verbose` flag
- Ensure raw JSON is preserved in events for storage
- Simplify if any V1 complexity isn't needed
