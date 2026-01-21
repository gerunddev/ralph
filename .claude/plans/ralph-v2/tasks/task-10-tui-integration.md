# Task 10: TUI Integration with Loop

## Objective

Connect the main execution loop to the TUI, translating loop events into TUI updates and handling the full application lifecycle.

## Requirements

1. Start TUI and loop concurrently
2. Translate LoopEvents into TUI messages
3. Stream Claude output to TUI in real-time
4. Handle completion (done or max iterations)
5. Handle quit (cancel loop gracefully)
6. Show final status before exit

## Architecture

```
                    +-------------+
                    |     CLI     |
                    +------+------+
                           |
                    +------v------+
                    |     App     |
                    +------+------+
                           |
          +----------------+----------------+
          |                                 |
   +------v------+                  +-------v------+
   |    Loop     |   events chan   |     TUI      |
   |  (goroutine)|  ------------->  |  (main)     |
   +-------------+                  +--------------+
```

## Event Translation

```go
// Translate loop events to TUI messages
func translateEvent(event loop.LoopEvent) tea.Msg {
    switch event.Type {
    case loop.LoopEventStarted:
        return SetStatusMsg{Message: "Starting execution..."}

    case loop.LoopEventIterationStart:
        return SetIterationMsg{
            Current: event.Iteration,
            Max:     event.MaxIter,
        }

    case loop.LoopEventClaudeStream:
        if event.ClaudeEvent.Message != nil {
            return AppendOutputMsg{Text: event.ClaudeEvent.Message.Text}
        }
        // Handle tool use display
        if event.ClaudeEvent.Tool != nil {
            return AppendOutputMsg{Text: formatToolUse(event.ClaudeEvent.Tool)}
        }

    case loop.LoopEventParsed:
        return SetStatusMsg{Message: "Parsing output..."}

    case loop.LoopEventDone:
        return CompletedMsg{Success: true, Message: "Agent completed!"}

    case loop.LoopEventMaxIterations:
        return CompletedMsg{Success: false, Message: "Max iterations reached"}

    case loop.LoopEventError:
        return SetErrorMsg{Error: event.Error.Error()}

    // ... etc
    }
}
```

## App Lifecycle

```go
type App struct {
    config *config.Config
    db     *db.DB
    loop   *loop.Loop
    tui    *tui.Model
}

func (a *App) Run(ctx context.Context, planPath string) error {
    // 1. Load or create plan
    // 2. Create loop
    // 3. Start loop in goroutine
    // 4. Start TUI (blocks)
    // 5. On TUI quit, cancel loop context
    // 6. Wait for loop to finish
    // 7. Return
}

func (a *App) Resume(ctx context.Context, planID string) error {
    // Similar but loads existing plan
}
```

## TUI Message Bridge

```go
// Subscribe to loop events and send to TUI
func (a *App) bridgeEvents(ctx context.Context, p *tea.Program) {
    for {
        select {
        case <-ctx.Done():
            return
        case event, ok := <-a.loop.Events():
            if !ok {
                return
            }
            msg := translateEvent(event)
            if msg != nil {
                p.Send(msg)
            }
        }
    }
}
```

## Prompt Display

When a new iteration starts:
1. Clear/reset output panel
2. Set prompt panel content to current prompt
3. Update iteration counter

## Completion Handling

When loop completes:
1. Show final status (success/max iterations/error)
2. Keep TUI open for user to review
3. User presses `q` to exit
4. Or auto-exit after timeout?

## Acceptance Criteria

- [ ] TUI starts and shows initial state
- [ ] Iteration counter updates each iteration
- [ ] Prompt panel shows current prompt
- [ ] Claude output streams in real-time
- [ ] Tool use is formatted readably
- [ ] Progress bar advances
- [ ] Errors display in status bar
- [ ] `q` cancels loop and exits cleanly
- [ ] Completion state is clear
- [ ] No race conditions between loop and TUI
- [ ] Integration tests

## Files to Create/Modify

- `internal/app/app.go` (new for V2)
- `internal/app/bridge.go`
- `internal/tui/messages.go` (TUI-specific messages)
- `internal/app/app_test.go`

## Notes

The tricky part is coordinating the loop goroutine with the TUI main thread. Use context cancellation for clean shutdown.

Consider: Should we show the full prompt or a truncated version? Full prompt might be very long. Maybe show first N lines with "..." and allow scrolling to see full.
