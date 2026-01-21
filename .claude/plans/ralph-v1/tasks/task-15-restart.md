# Task 15: Restart from Interim State

## Context

Ralph should be able to resume interrupted projects. If the app crashes or is killed mid-task, restarting should pick up from where it left off. The database already tracks task status.

## Objective

Implement reliable restart capability that detects the current state and resumes appropriately.

## Acceptance Criteria

- [ ] Detect in-progress tasks and mark as pending for retry
- [ ] Resume from first pending task
- [ ] Clean up partial sessions on restart
- [ ] Handle interrupted review loops (reset iteration)
- [ ] User confirmation before resuming
- [ ] Show what was completed vs what remains
- [ ] Option to reset entire project

## Implementation Details

### Project State Detection

```go
// internal/engine/resume.go

type ProjectState struct {
    Project         *db.Project
    CompletedTasks  int
    PendingTasks    int
    FailedTasks     int
    InProgressTask  *db.Task  // nil if none
    LastSession     *db.Session
    NeedsCleanup    bool
}

func (e *Engine) DetectProjectState(ctx context.Context, projectID string) (*ProjectState, error) {
    project, err := e.db.GetProject(projectID)
    if err != nil {
        return nil, err
    }

    tasks, err := e.db.GetTasksByProject(projectID)
    if err != nil {
        return nil, err
    }

    state := &ProjectState{
        Project: project,
    }

    for _, task := range tasks {
        switch task.Status {
        case db.TaskCompleted:
            state.CompletedTasks++
        case db.TaskPending:
            state.PendingTasks++
        case db.TaskFailed:
            state.FailedTasks++
        case db.TaskInProgress:
            state.InProgressTask = task
            state.NeedsCleanup = true
        case db.TaskEscalated:
            state.FailedTasks++  // Count as failed
        }
    }

    // Check for running sessions
    if state.InProgressTask != nil {
        session, err := e.db.GetLatestSessionForTask(state.InProgressTask.ID)
        if err == nil {
            state.LastSession = session
            if session.Status == db.SessionRunning {
                state.NeedsCleanup = true
            }
        }
    }

    return state, nil
}
```

### Cleanup on Resume

```go
func (e *Engine) CleanupForResume(ctx context.Context, state *ProjectState) error {
    if !state.NeedsCleanup {
        return nil
    }

    // 1. Mark in-progress task as pending
    if state.InProgressTask != nil {
        if err := e.db.UpdateTaskStatus(state.InProgressTask.ID, db.TaskPending); err != nil {
            return err
        }

        // Reset iteration count to retry from scratch
        // (or we could preserve it and continue from last iteration)
    }

    // 2. Mark running sessions as failed
    if state.LastSession != nil && state.LastSession.Status == db.SessionRunning {
        if err := e.db.CompleteSession(state.LastSession.ID, db.SessionFailed); err != nil {
            return err
        }
    }

    // 3. Update project status if needed
    if state.Project.Status == db.ProjectInProgress {
        // Keep as in_progress, will be updated by task loop
    }

    return nil
}
```

### Resume Confirmation View

```go
// internal/tui/resume.go

type ResumeModel struct {
    state     *engine.ProjectState
    confirmed bool
    reset     bool
}

func NewResumeModel(state *engine.ProjectState) ResumeModel {
    return ResumeModel{state: state}
}

func (m ResumeModel) View() string {
    var s strings.Builder

    s.WriteString(titleStyle.Render("Resume Project?"))
    s.WriteString("\n\n")

    s.WriteString(fmt.Sprintf("Project: %s\n", m.state.Project.Name))
    s.WriteString(fmt.Sprintf("Status: %s\n\n", m.state.Project.Status))

    s.WriteString("Progress:\n")
    s.WriteString(fmt.Sprintf("  Completed: %d tasks\n", m.state.CompletedTasks))
    s.WriteString(fmt.Sprintf("  Pending: %d tasks\n", m.state.PendingTasks))
    s.WriteString(fmt.Sprintf("  Failed: %d tasks\n", m.state.FailedTasks))

    if m.state.InProgressTask != nil {
        s.WriteString(fmt.Sprintf("\nInterrupted task: %s\n", m.state.InProgressTask.Title))
        s.WriteString("This task will be restarted from the beginning.\n")
    }

    s.WriteString("\n")
    s.WriteString(helpStyle.Render("enter: resume • r: reset all • q: quit"))

    return s.String()
}

func (m ResumeModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
    switch msg := msg.(type) {
    case tea.KeyMsg:
        switch msg.String() {
        case "enter":
            m.confirmed = true
            return m, func() tea.Msg { return ResumeConfirmedMsg{} }

        case "r":
            m.reset = true
            return m, func() tea.Msg { return ResetConfirmedMsg{} }

        case "q":
            return m, tea.Quit
        }
    }
    return m, nil
}
```

### Reset Project

```go
func (e *Engine) ResetProject(ctx context.Context, projectID string) error {
    // 1. Get all tasks
    tasks, err := e.db.GetTasksByProject(projectID)
    if err != nil {
        return err
    }

    // 2. Reset all tasks to pending
    for _, task := range tasks {
        if err := e.db.UpdateTaskStatus(task.ID, db.TaskPending); err != nil {
            return err
        }
        // Clear jj change ID since we're starting fresh
        // (or we could keep it for reference)
    }

    // 3. Reset project status
    if err := e.db.UpdateProjectStatus(projectID, db.ProjectPending); err != nil {
        return err
    }

    // 4. Optionally clear sessions/messages for clean slate
    // This could be a config option

    return nil
}
```

### Integration in Selection Mode

```go
func (m MainModel) handleProjectSelected(project *db.Project) tea.Cmd {
    return func() tea.Msg {
        ctx := context.Background()

        // Detect state
        state, err := m.engine.DetectProjectState(ctx, project.ID)
        if err != nil {
            return ErrorMsg{Err: err}
        }

        // If needs cleanup or has interrupted work, show resume dialog
        if state.NeedsCleanup || state.InProgressTask != nil {
            return ShowResumeDialogMsg{State: state}
        }

        // If completed, maybe show summary instead
        if state.PendingTasks == 0 && state.CompletedTasks > 0 {
            return ProjectCompletedMsg{Project: project}
        }

        // Otherwise start normally
        return StartProjectMsg{Project: project}
    }
}
```

### Failed Task Handling

```go
type FailedTasksModel struct {
    tasks  []*db.Task
    cursor int
}

func (m FailedTasksModel) View() string {
    var s strings.Builder

    s.WriteString(warnStyle.Render("Some tasks failed:"))
    s.WriteString("\n\n")

    for i, task := range m.tasks {
        prefix := "  "
        if i == m.cursor {
            prefix = "> "
        }
        s.WriteString(fmt.Sprintf("%s%s\n", prefix, task.Title))
    }

    s.WriteString("\n")
    s.WriteString(helpStyle.Render("r: retry selected • s: skip failed • a: abort"))

    return s.String()
}
```

## Files to Modify

- `internal/engine/resume.go` - Create with state detection
- `internal/engine/engine.go` - Add cleanup and reset methods
- `internal/tui/resume.go` - Create with resume dialog
- `internal/tui/app.go` - Integrate resume flow

## Testing Strategy

1. **State detection** - Various project states
2. **Cleanup** - In-progress task reset
3. **Resume flow** - Full restart scenario
4. **Reset** - Clean slate functionality

## Dependencies

- `internal/db` - State queries
- `internal/engine` - Core methods
- `internal/tui` - Resume UI

## Notes

- The database is the source of truth for state
- In-progress tasks are reset to pending on resume
- Failed tasks stay failed unless explicitly retried
- Consider adding "resume from specific task" option
- JJ changes are preserved - we continue from current state
- Sessions are marked failed on interrupt (for audit trail)
