# Task 5: Claude Client & Stream Parser

## Context

Ralph executes Claude Code in print mode with streaming JSON output. Each line of output is a JSON object that needs to be parsed and stored in the database for history. The stub files exist at `internal/claude/client.go` and `internal/claude/parser.go`.

## Objective

Implement a Claude CLI wrapper that executes `claude -p` with stream-json output and a parser that processes the JSONL stream.

## Acceptance Criteria

- [ ] `NewClient(config)` creates a client with Claude configuration
- [ ] `Run(ctx, prompt, systemPrompt)` executes Claude and returns a `Session` handle
- [ ] Session provides a channel or reader for streaming events
- [ ] Parser reads JSONL and emits typed `StreamEvent` structs
- [ ] Handle message events with text content
- [ ] Handle tool_use events (Claude calling tools)
- [ ] Handle tool_result events (tool responses)
- [ ] Handle system events (init, result, etc.)
- [ ] Context cancellation stops the Claude process
- [ ] All streamed JSON is preserved for database storage
- [ ] Unit tests with mock command execution
- [ ] Error handling for command failures, parse errors

## Implementation Details

### Client Structure

```go
type Client struct {
    model        string
    maxTurns     int
    maxBudgetUSD float64
}

type ClientConfig struct {
    Model        string
    MaxTurns     int
    MaxBudgetUSD float64
}

func NewClient(cfg ClientConfig) *Client {
    return &Client{
        model:        cfg.Model,
        maxTurns:     cfg.MaxTurns,
        maxBudgetUSD: cfg.MaxBudgetUSD,
    }
}
```

### Command Construction

```bash
claude -p \
  --output-format stream-json \
  --model <model> \
  --max-turns <turns> \
  --max-budget-usd <budget> \
  --system-prompt "<system>" \
  "<prompt>"
```

### Session Handle

```go
type Session struct {
    cmd       *exec.Cmd
    stdout    io.ReadCloser
    stderr    bytes.Buffer
    parser    *Parser
    done      chan struct{}
    err       error
}

func (s *Session) Events() <-chan StreamEvent
func (s *Session) Wait() error
func (s *Session) Cancel()
```

### Stream Event Types

Based on Claude's stream-json output:

```go
type EventType string

const (
    EventInit       EventType = "init"
    EventMessage    EventType = "message"
    EventToolUse    EventType = "tool_use"
    EventToolResult EventType = "tool_result"
    EventResult     EventType = "result"
    EventError      EventType = "error"
)

type StreamEvent struct {
    Type       EventType
    Raw        []byte           // Original JSON for storage
    Message    *MessageContent  // For message events
    ToolUse    *ToolUseContent  // For tool_use events
    ToolResult *ToolResultContent
    Error      *ErrorContent
}

type MessageContent struct {
    ID         string
    Role       string
    Model      string
    Text       string           // Extracted text content
    StopReason string
    Usage      Usage
}

type Usage struct {
    InputTokens  int
    OutputTokens int
}
```

### Parser Implementation

```go
type Parser struct {
    scanner *bufio.Scanner
}

func NewParser(r io.Reader) *Parser {
    return &Parser{scanner: bufio.NewScanner(r)}
}

func (p *Parser) Next() (*StreamEvent, error) {
    if !p.scanner.Scan() {
        if err := p.scanner.Err(); err != nil {
            return nil, err
        }
        return nil, io.EOF
    }

    line := p.scanner.Bytes()
    // Parse JSON and construct StreamEvent
}
```

### JSON Structure

Each line is a JSON object with various shapes:

```json
{"type": "init", "session_id": "...", ...}
{"message": {"id": "msg_...", "role": "assistant", "content": [{"type": "text", "text": "..."}], ...}}
{"type": "result", "session_id": "...", "cost_usd": 0.01, ...}
```

## Files to Modify

- `internal/claude/client.go` - Full implementation
- `internal/claude/parser.go` - Full implementation
- `internal/claude/types.go` - Create for event type definitions
- `internal/claude/client_test.go` - Create with tests
- `internal/claude/parser_test.go` - Create with tests

## Testing Strategy

1. **Parser tests** - Feed known JSONL strings and verify parsed events
2. **Client tests** - Mock exec.Command to return predefined output
3. **Error tests** - Malformed JSON, command failure, context cancellation

## Dependencies

- `internal/config` - For Claude configuration values

## Notes

- The README says "I want to use claude code in print mode, verbose, and with the output as streamed json"
- All JSON objects should be stored in the database as `Message` records
- The final "result" event contains cost/usage information
- Use `--include-partial-messages` if we want streaming text updates (may not be needed for v1)
