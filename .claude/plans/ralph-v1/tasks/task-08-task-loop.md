# Task 8: Task Loop Implementation

## Context

The task loop iterates through all pending tasks in a project. For each task, it creates a jj change and runs the review loop. The stub exists at `internal/engine/task_loop.go`.

## Objective

Implement the outer loop that processes tasks sequentially, creating version control changes and running the review loop for each.

## Acceptance Criteria

- [ ] `NewTaskLoop(deps, project)` creates a loop for a project
- [ ] `Run(ctx)` processes all pending tasks in sequence order
- [ ] For each task: create jj change, run review loop
- [ ] Update task status (pending -> in_progress -> completed/failed)
- [ ] Store jj change ID on task record
- [ ] Emit events for TUI updates
- [ ] Handle task failures (mark failed, optionally continue)
- [ ] Return summary of completed/failed tasks
- [ ] Context cancellation stops gracefully

## Implementation Details

### Dependencies Struct

```go
type TaskLoopDeps struct {
    DB      *db.DB
    Claude  *claude.Client
    JJ      *jj.Client
    Agents  *agents.Manager
    Config  *config.Config
}
```

### Task Loop Structure

```go
type TaskLoop struct {
    deps       TaskLoopDeps
    project    *db.Project
    tasks      []*db.Task
    current    int
    events     chan TaskLoopEvent
}

type TaskLoopEvent struct {
    Type        TaskEventType
    TaskIndex   int
    TaskTitle   string
    Message     string
    ReviewEvent *ReviewLoopEvent  // Nested events from review loop
}

type TaskEventType string

const (
    TaskEventStarted     TaskEventType = "started"
    TaskEventTaskBegin   TaskEventType = "task_begin"
    TaskEventTaskEnd     TaskEventType = "task_end"
    TaskEventCompleted   TaskEventType = "completed"
    TaskEventFailed      TaskEventType = "failed"
)
```

### Main Loop Logic

```go
func (tl *TaskLoop) Run(ctx context.Context) error {
    // Load pending tasks
    tasks, err := tl.deps.DB.GetTasksByProject(tl.project.ID)
    if err != nil {
        return err
    }

    // Update project status
    tl.deps.DB.UpdateProjectStatus(tl.project.ID, db.ProjectInProgress)

    var completed, failed int

    for i, task := range tasks {
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

        if err := tl.processTask(ctx, task); err != nil {
            // Mark task failed
            tl.deps.DB.UpdateTaskStatus(task.ID, db.TaskFailed)
            failed++
            tl.emit(TaskEventFailed, task, err.Error())

            // Check if we should continue
            if tl.deps.Config.MaxTaskAttempts > 0 {
                continue  // Try next task
            }
            return err  // Fail fast
        }

        completed++
        tl.emit(TaskEventTaskEnd, task, "Task completed")
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
```

### Process Single Task

```go
func (tl *TaskLoop) processTask(ctx context.Context, task *db.Task) error {
    // 1. Update status to in_progress
    if err := tl.deps.DB.UpdateTaskStatus(task.ID, db.TaskInProgress); err != nil {
        return err
    }

    // 2. Create jj change
    changeID, err := tl.deps.JJ.NewChange(ctx, task.Title)
    if err != nil {
        return fmt.Errorf("failed to create jj change: %w", err)
    }

    // 3. Store change ID
    if err := tl.deps.DB.UpdateTaskJJChangeID(task.ID, changeID); err != nil {
        return err
    }

    // 4. Run review loop
    reviewLoop := NewReviewLoop(ReviewLoopDeps{
        DB:      tl.deps.DB,
        Claude:  tl.deps.Claude,
        JJ:      tl.deps.JJ,
        Agents:  tl.deps.Agents,
        Config:  tl.deps.Config,
    }, task, tl.project.PlanText)

    // Forward review events
    go func() {
        for event := range reviewLoop.Events() {
            tl.events <- TaskLoopEvent{
                Type:        TaskEventTaskBegin,  // Reuse for review updates
                TaskIndex:   tl.current,
                TaskTitle:   task.Title,
                ReviewEvent: &event,
            }
        }
    }()

    if err := reviewLoop.Run(ctx); err != nil {
        // Check if it's an escalation (max iterations) vs error
        if reviewLoop.Status() == ReviewLoopStatusEscalated {
            tl.deps.DB.UpdateTaskStatus(task.ID, db.TaskEscalated)
            return err
        }
        return err
    }

    // 5. Mark completed
    return tl.deps.DB.UpdateTaskStatus(task.ID, db.TaskCompleted)
}
```

### Current Task Accessor

```go
func (tl *TaskLoop) CurrentTask() *db.Task {
    if tl.current < len(tl.tasks) {
        return tl.tasks[tl.current]
    }
    return nil
}

func (tl *TaskLoop) Progress() (current, total int) {
    return tl.current + 1, len(tl.tasks)
}
```

### Event Channel

```go
func (tl *TaskLoop) Events() <-chan TaskLoopEvent {
    return tl.events
}
```

## Files to Modify

- `internal/engine/task_loop.go` - Full implementation
- `internal/engine/task_loop_test.go` - Create with tests

## Testing Strategy

1. **Unit tests** - Mock all dependencies
2. **Empty project** - No tasks to process
3. **All succeed** - Multiple tasks, all complete
4. **One fails** - Task fails, others continue
5. **Resume** - Some tasks already completed, start from pending
6. **Cancellation** - Context cancelled mid-task

## Dependencies

- `internal/db` - For task status updates
- `internal/jj` - For creating changes
- `internal/engine/review_loop` - For task processing
- `internal/agents` - For agent construction
- `internal/config` - For settings

## Notes

- The task loop should be resumable - if Ralph restarts, it picks up from the first pending task
- Tasks are processed in sequence order (from DB)
- Each task gets its own jj change, making rollback easy
- The TUI subscribes to TaskLoopEvent for progress display
