# Task 7: Review Loop Implementation

## Context

The review loop is the inner loop of Ralph: developer implements -> reviewer reviews -> repeat until approved or max iterations. The stub exists at `internal/engine/review_loop.go`.

## Objective

Implement the developer->reviewer cycle that runs until the reviewer approves or max iterations are reached.

## Acceptance Criteria

- [ ] `NewReviewLoop(deps, task)` creates a loop for a specific task
- [ ] `Run(ctx)` executes the developer->reviewer cycle
- [ ] Developer agent runs first with task context
- [ ] Reviewer agent runs with diff from `jj show`
- [ ] Parse reviewer output for APPROVED or FEEDBACK
- [ ] On feedback, loop back to developer with feedback context
- [ ] Stop on approval, max iterations, or error
- [ ] All sessions stored in database
- [ ] Status updates emitted for TUI display
- [ ] Context cancellation stops gracefully

## Implementation Details

### Dependencies Struct

```go
type ReviewLoopDeps struct {
    DB      *db.DB
    Claude  *claude.Client
    JJ      *jj.Client
    Agents  *agents.Manager
    Config  *config.Config
}
```

### Review Loop Structure

```go
type ReviewLoop struct {
    deps      ReviewLoopDeps
    task      *db.Task
    plan      string
    status    ReviewLoopStatus
    iteration int
    events    chan ReviewLoopEvent
}

type ReviewLoopEvent struct {
    Type      EventType
    Iteration int
    Agent     string   // "developer" or "reviewer"
    Message   string   // Summary or feedback
}

type EventType string

const (
    EventStarted    EventType = "started"
    EventDeveloping EventType = "developing"
    EventReviewing  EventType = "reviewing"
    EventFeedback   EventType = "feedback"
    EventApproved   EventType = "approved"
    EventFailed     EventType = "failed"
)
```

### Main Loop Logic

```go
func (rl *ReviewLoop) Run(ctx context.Context) error {
    rl.status = ReviewLoopStatusRunning
    rl.emit(EventStarted, "Starting review loop")

    var lastFeedback string
    maxIterations := rl.deps.Config.MaxReviewIterations

    for rl.iteration = 1; rl.iteration <= maxIterations; rl.iteration++ {
        // 1. Run developer
        rl.emit(EventDeveloping, fmt.Sprintf("Iteration %d: Developer working", rl.iteration))
        if err := rl.runDeveloper(ctx, lastFeedback); err != nil {
            return rl.fail(err)
        }

        // 2. Get diff
        diff, err := rl.deps.JJ.Show(ctx)
        if err != nil {
            return rl.fail(fmt.Errorf("failed to get diff: %w", err))
        }

        // 3. Run reviewer
        rl.emit(EventReviewing, fmt.Sprintf("Iteration %d: Reviewer checking", rl.iteration))
        result, err := rl.runReviewer(ctx, diff)
        if err != nil {
            return rl.fail(err)
        }

        // 4. Parse result
        if result.Approved {
            rl.status = ReviewLoopStatusApproved
            rl.emit(EventApproved, "Changes approved")
            return nil
        }

        // 5. Loop with feedback
        lastFeedback = result.Feedback
        rl.emit(EventFeedback, result.Feedback)
    }

    rl.status = ReviewLoopStatusEscalated
    return fmt.Errorf("max iterations (%d) reached", maxIterations)
}
```

### Developer Session

```go
func (rl *ReviewLoop) runDeveloper(ctx context.Context, feedback string) error {
    // Create agent with context
    agent, err := rl.deps.Agents.GetDeveloperAgent(ctx, rl.plan, rl.task, feedback)
    if err != nil {
        return err
    }

    // Create session record
    session := &db.Session{
        ID:          uuid.New().String(),
        TaskID:      rl.task.ID,
        AgentType:   db.AgentDeveloper,
        Iteration:   rl.iteration,
        InputPrompt: agent.Prompt,
        Status:      db.SessionRunning,
    }
    if err := rl.deps.DB.CreateSession(session); err != nil {
        return err
    }

    // Run Claude
    claudeSession, err := rl.deps.Claude.Run(ctx, agent.Prompt, "")
    if err != nil {
        rl.deps.DB.CompleteSession(session.ID, db.SessionFailed)
        return err
    }

    // Store all events
    seq := 0
    for event := range claudeSession.Events() {
        msg := &db.Message{
            SessionID:   session.ID,
            Sequence:    seq,
            MessageType: string(event.Type),
            Content:     string(event.Raw),
        }
        rl.deps.DB.CreateMessage(msg)
        seq++
    }

    if err := claudeSession.Wait(); err != nil {
        rl.deps.DB.CompleteSession(session.ID, db.SessionFailed)
        return err
    }

    rl.deps.DB.CompleteSession(session.ID, db.SessionCompleted)
    return nil
}
```

### Parsing Reviewer Output

```go
type ReviewResult struct {
    Approved bool
    Feedback string
}

func parseReviewerOutput(text string) ReviewResult {
    text = strings.TrimSpace(text)

    // Look for APPROVED
    if strings.Contains(text, "APPROVED") {
        return ReviewResult{Approved: true}
    }

    // Look for FEEDBACK:
    if idx := strings.Index(text, "FEEDBACK:"); idx != -1 {
        feedback := strings.TrimSpace(text[idx+9:])
        return ReviewResult{Feedback: feedback}
    }

    // Default to feedback with full text
    return ReviewResult{Feedback: text}
}
```

### Increment Task Iteration

After each loop iteration, update the task:

```go
rl.deps.DB.IncrementTaskIteration(rl.task.ID)
```

## Files to Modify

- `internal/engine/review_loop.go` - Full implementation
- `internal/engine/review_loop_test.go` - Create with tests

## Testing Strategy

1. **Unit tests** - Mock all dependencies (DB, Claude, JJ, Agents)
2. **Happy path** - Developer runs, reviewer approves
3. **Feedback loop** - Developer runs, reviewer gives feedback, developer fixes, reviewer approves
4. **Max iterations** - Loop reaches limit and escalates
5. **Error handling** - Claude failure, JJ failure, etc.

## Dependencies

- `internal/db` - For session/message storage
- `internal/claude` - For running Claude sessions
- `internal/jj` - For getting diffs
- `internal/agents` - For building agent prompts
- `internal/config` - For max iterations setting

## Notes

- The task.IterationCount tracks total developer attempts
- Each developer and reviewer run creates a separate Session record
- All Claude output (messages) is stored for audit/debugging
- The TUI will subscribe to ReviewLoopEvent for display updates
