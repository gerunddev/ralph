# Task 8: Main Loop Orchestration

## Objective

Create the main execution loop that ties together all components: prompt building, jj, Claude sessions, parsing, and database storage.

## Requirements

1. Accept plan (new or resumed) and config
2. Loop until "DONE DONE DONE!!!" or max_iterations
3. Each iteration:
   - Build prompt with latest progress/learnings
   - Run `jj new`
   - Create session in DB
   - Run Claude, stream events, store them
   - Parse output
   - Store progress/learnings (if not done)
   - Distill commit message via Haiku
   - Run `jj commit`
4. Emit events for TUI consumption
5. Handle errors gracefully (log, continue)
6. Support context cancellation

## Event Types

```go
type LoopEventType string

const (
    LoopEventStarted       LoopEventType = "started"
    LoopEventIterationStart LoopEventType = "iteration_start"
    LoopEventJJNew         LoopEventType = "jj_new"
    LoopEventClaudeStart   LoopEventType = "claude_start"
    LoopEventClaudeStream  LoopEventType = "claude_stream"  // Wraps claude events
    LoopEventClaudeEnd     LoopEventType = "claude_end"
    LoopEventParsed        LoopEventType = "parsed"
    LoopEventDistilling    LoopEventType = "distilling"
    LoopEventJJCommit      LoopEventType = "jj_commit"
    LoopEventIterationEnd  LoopEventType = "iteration_end"
    LoopEventDone          LoopEventType = "done"          // Agent said done
    LoopEventMaxIterations LoopEventType = "max_iterations"
    LoopEventError         LoopEventType = "error"
)

type LoopEvent struct {
    Type        LoopEventType
    Iteration   int
    MaxIter     int
    Message     string
    ClaudeEvent *claude.StreamEvent // For claude_stream events
    Error       error
}
```

## Interface

```go
type LoopConfig struct {
    PlanID        string
    MaxIterations int
    WorkDir       string // For jj operations
}

type LoopDeps struct {
    DB       *db.DB
    Claude   *claude.Client   // Main model
    Haiku    *claude.Client   // For distillation
    JJ       *jj.Client
}

type Loop struct { ... }

func NewLoop(cfg LoopConfig, deps LoopDeps) *Loop

func (l *Loop) Run(ctx context.Context) error
func (l *Loop) Events() <-chan LoopEvent

// For resume support
func (l *Loop) CurrentIteration() int
```

## Execution Flow (per iteration)

```
1. Increment iteration counter
2. Load latest progress/learnings from DB
3. Build prompt
4. Emit LoopEventIterationStart
5. Run jj new
   - On error: log, emit error event, continue
6. Create session record (status: running)
7. Run Claude session
   - Stream events to channel AND store in DB
   - On error: log, mark session failed, continue
8. Parse output
   - If "DONE DONE DONE!!!":
     - Mark session complete
     - Mark plan complete
     - Emit LoopEventDone
     - Return
   - Else:
     - Store progress in DB
     - Store learnings in DB
9. Distill commit message
   - On error: use fallback message
10. Run jj commit
    - On error: log, continue
11. Mark session complete
12. Emit LoopEventIterationEnd
13. Check max_iterations
    - If reached: emit LoopEventMaxIterations, return
14. Next iteration
```

## Acceptance Criteria

- [ ] Loops correctly until done or max iterations
- [ ] Builds prompt with latest progress/learnings
- [ ] Stores all session data (input, events, output)
- [ ] Stores versioned progress/learnings
- [ ] Emits events for all major steps
- [ ] Continues on recoverable errors
- [ ] Context cancellation stops cleanly
- [ ] Resume works (starts at correct iteration with correct state)
- [ ] Unit tests with mocked dependencies

## Files to Create

- `internal/loop/loop.go`
- `internal/loop/events.go`
- `internal/loop/loop_test.go`

## Notes

Error handling philosophy: Be resilient. If jj fails, still run Claude. If distillation fails, use a generic message. If event storage fails, log but continue. Only stop on context cancellation or max iterations.

The Loop doesn't know about TUI - it just emits events. TUI subscribes and renders.
