# Addendum 03: Task Loop Pause Mode

## Context

By default, Ralph's task loop runs continuously through all tasks without stopping. This is efficient for fully autonomous operation. However, sometimes you want to:

1. Review changes after each task before proceeding
2. Make manual adjustments between tasks
3. Pause mid-run to take a break or investigate issues

This feature adds a "pause mode" that stops after each task completion and waits for user confirmation before continuing. A TUI hotkey allows toggling pause mode on/off during execution.

## Objective

Add pause mode to the task loop with a TUI hotkey for toggling, allowing users to manually approve progression between tasks.

## Acceptance Criteria

- [ ] Default behavior: continuous mode (no pausing between tasks)
- [ ] Pause mode: after each task completes, wait for user confirmation
- [ ] TUI hotkey `p` toggles pause mode on/off
- [ ] Visual indicator shows current mode (continuous vs paused)
- [ ] When paused, show clear prompt: "Task N complete. Press Enter to continue or 'p' to switch to continuous mode"
- [ ] Config option to set default mode: `default_pause_mode: true/false`
- [ ] Works correctly with task failures (still pauses, shows error)
- [ ] Hotkey works even when not paused (to pre-emptively enable pause)

## Implementation Details

### Engine Changes

```go
// internal/engine/task_loop.go

type TaskLoop struct {
    deps       TaskLoopDeps
    project    *db.Project
    tasks      []*db.Task
    current    int
    events     chan TaskLoopEvent

    // Pause mode
    pauseMode    bool
    pauseCh      chan struct{}  // Signals to continue after pause
    pauseModeMu  sync.RWMutex
}

// SetPauseMode enables or disables pause mode
func (tl *TaskLoop) SetPauseMode(enabled bool) {
    tl.pauseModeMu.Lock()
    defer tl.pauseModeMu.Unlock()
    tl.pauseMode = enabled

    // Emit event so TUI can update display
    tl.events <- TaskLoopEvent{
        Type:    TaskEventPauseModeChanged,
        Message: fmt.Sprintf("Pause mode: %v", enabled),
    }
}

// IsPauseMode returns current pause mode state
func (tl *TaskLoop) IsPauseMode() bool {
    tl.pauseModeMu.RLock()
    defer tl.pauseModeMu.RUnlock()
    return tl.pauseMode
}

// Continue signals the loop to proceed after a pause
func (tl *TaskLoop) Continue() {
    select {
    case tl.pauseCh <- struct{}{}:
    default:
        // Not currently paused, ignore
    }
}
```

### Task Loop Event Types

```go
// internal/engine/task_loop.go

const (
    TaskEventStarted         TaskEventType = "started"
    TaskEventTaskBegin       TaskEventType = "task_begin"
    TaskEventTaskEnd         TaskEventType = "task_end"
    TaskEventCompleted       TaskEventType = "completed"
    TaskEventFailed          TaskEventType = "failed"
    TaskEventPaused          TaskEventType = "paused"          // NEW
    TaskEventResumed         TaskEventType = "resumed"         // NEW
    TaskEventPauseModeChanged TaskEventType = "pause_mode_changed"  // NEW
)
```

### Modified Run Loop

```go
func (tl *TaskLoop) Run(ctx context.Context) error {
    tasks, err := tl.deps.DB.GetTasksByProject(tl.project.ID)
    if err != nil {
        return err
    }

    tl.deps.DB.UpdateProjectStatus(tl.project.ID, db.ProjectInProgress)

    var completed, failed int

    for i, task := range tasks {
        // Check for context cancellation
        select {
        case <-ctx.Done():
            return ctx.Err()
        default:
        }

        if task.Status == db.TaskCompleted {
            completed++
            continue
        }

        if task.Status == db.TaskFailed {
            failed++
            continue
        }

        tl.current = i
        tl.emit(TaskEventTaskBegin, task, "Starting task")

        err := tl.processTask(ctx, task)

        if err != nil {
            tl.deps.DB.UpdateTaskStatus(task.ID, db.TaskFailed)
            failed++
            tl.emit(TaskEventFailed, task, err.Error())
        } else {
            completed++
            tl.emit(TaskEventTaskEnd, task, "Task completed")
        }

        // Check if we should pause before next task
        if tl.IsPauseMode() && i < len(tasks)-1 {
            if err := tl.waitForContinue(ctx, task); err != nil {
                return err
            }
        }
    }

    // Update project status
    if failed > 0 {
        tl.deps.DB.UpdateProjectStatus(tl.project.ID, db.ProjectFailed)
    } else {
        tl.deps.DB.UpdateProjectStatus(tl.project.ID, db.ProjectCompleted)
    }

    tl.emit(TaskEventCompleted, nil, fmt.Sprintf("%d completed, %d failed", completed, failed))
    return nil
}

func (tl *TaskLoop) waitForContinue(ctx context.Context, task *db.Task) error {
    tl.emit(TaskEventPaused, task, "Waiting for confirmation to continue")

    select {
    case <-ctx.Done():
        return ctx.Err()
    case <-tl.pauseCh:
        tl.emit(TaskEventResumed, task, "Continuing to next task")
        return nil
    }
}
```

### TUI Changes

```go
// internal/tui/progress.go

type TaskProgressModel struct {
    // ... existing fields ...

    // Pause mode state
    pauseMode    bool
    isPaused     bool    // Currently waiting at a pause point
    taskLoop     *engine.TaskLoop  // Reference to control pause
}

func (m TaskProgressModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
    switch msg := msg.(type) {
    case tea.KeyMsg:
        switch msg.String() {
        case "p":
            // Toggle pause mode
            m.pauseMode = !m.pauseMode
            m.taskLoop.SetPauseMode(m.pauseMode)
            return m, nil

        case "enter", " ":
            // Continue from pause (if paused)
            if m.isPaused {
                m.taskLoop.Continue()
                m.isPaused = false
                return m, nil
            }

        case "q", "ctrl+c":
            return m, tea.Quit
        // ... other keys ...
        }

    case EngineEventMsg:
        switch msg.Event.Type {
        case engine.TaskEventPaused:
            m.isPaused = true
        case engine.TaskEventResumed:
            m.isPaused = false
        case engine.TaskEventPauseModeChanged:
            // Update display
        }
        // ... handle other events ...
    }

    // ... rest of update ...
}
```

### TUI View Updates

```go
// internal/tui/progress.go

func (m TaskProgressModel) renderFooter() string {
    var left, right string

    // Mode indicator
    modeIndicator := ""
    if m.pauseMode {
        modeIndicator = pauseStyle.Render("[PAUSE MODE]") + " "
    }

    if m.isPaused {
        left = modeIndicator + "Task complete. Press Enter to continue, 'p' to switch to continuous"
    } else if m.completed {
        left = "Completed!"
    } else {
        left = modeIndicator + fmt.Sprintf("Iteration %d - %s", m.iteration, m.agentType)
    }

    // Help text
    if m.pauseMode {
        right = "p: continuous mode | Enter: continue | q: quit"
    } else {
        right = "p: pause mode | j/k: scroll | q: quit"
    }

    return lipgloss.JoinHorizontal(
        lipgloss.Top,
        helpStyle.Render(left),
        strings.Repeat(" ", max(0, m.width-lipgloss.Width(left)-lipgloss.Width(right)-4)),
        helpStyle.Render(right),
    )
}
```

### Styles for Pause Indicator

```go
// internal/tui/styles.go

var pauseStyle = lipgloss.NewStyle().
    Foreground(lipgloss.Color("11")).  // Yellow
    Bold(true)

var pausedBorderStyle = lipgloss.NewStyle().
    Border(lipgloss.RoundedBorder()).
    BorderForeground(lipgloss.Color("11"))  // Yellow border when paused
```

### Config Option

```go
// internal/config/config.go

type Config struct {
    // ... existing fields ...

    // DefaultPauseMode sets whether pause mode is enabled by default
    // Default: false (continuous mode)
    DefaultPauseMode bool `json:"default_pause_mode"`
}
```

### Engine Integration

```go
// internal/engine/engine.go

func (e *Engine) createTaskLoop(project *db.Project) *TaskLoop {
    tl := NewTaskLoop(e.deps, project)

    // Apply default pause mode from config
    tl.SetPauseMode(e.config.DefaultPauseMode)

    return tl
}
```

## Files to Modify

- `internal/engine/task_loop.go` - Add pause mode logic
- `internal/tui/progress.go` - Add hotkey handling, pause display
- `internal/tui/styles.go` - Add pause mode styles
- `internal/config/config.go` - Add `DefaultPauseMode` option
- `internal/engine/engine.go` - Apply config default

## Testing Strategy

1. **Unit tests** - Pause/continue signaling, mode toggling
2. **Integration tests** - Full loop with pause at each task
3. **Concurrency tests** - Toggle while running, continue while not paused
4. **TUI tests** - Hotkey handling, view updates

## User Experience

### Continuous Mode (Default)

```
=== Task 1: Setup structure ===
[Developer working...]
[Reviewer checking...]
Approved!
Task completed.

=== Task 2: Add auth ===
[Developer working...]
...
```

### Pause Mode

```
=== Task 1: Setup structure ===
[Developer working...]
[Reviewer checking...]
Approved!
Task completed.

[PAUSE MODE] Task complete. Press Enter to continue, 'p' to switch to continuous
> [user presses Enter]

=== Task 2: Add auth ===
...
```

### Hotkey Flow

1. Running in continuous mode
2. User presses `p` - shows `[PAUSE MODE]` indicator
3. Current task continues to completion
4. After task ends, pauses and shows confirmation prompt
5. User presses `Enter` to continue, or `p` again to disable pause mode
6. If `p` pressed, switches back to continuous and auto-continues

## Notes

- The pause happens AFTER task completion, not during (we don't interrupt Claude)
- Pressing `p` while in the middle of a task queues the pause for after completion
- Consider adding a "pause after current task" vs "pause immediately" distinction
- Could extend with `n` for "next" (continue one task, stay paused) and `c` for "continue all"
- The yellow color scheme makes pause state visually distinct
