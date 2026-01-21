# Task 11: TUI - Task Progress View

## Context

Once a project is running, the TUI shows task progress: which task is current, dev/review iteration status, and streaming Claude output. This is the main view during execution.

## Objective

Implement the task progress view that shows real-time updates as Ralph processes tasks.

## Acceptance Criteria

- [ ] Display task list with status indicators
- [ ] Highlight current task being processed
- [ ] Show current iteration (developer/reviewer)
- [ ] Display streaming Claude output in a viewport
- [ ] Progress bar or fraction (3/10 tasks)
- [ ] Auto-scroll output, with ability to pause/scroll manually
- [ ] Handle completion (show summary)
- [ ] Handle errors gracefully
- [ ] Keyboard shortcuts (q to quit, scroll controls)

## Implementation Details

### Task Progress Model

```go
type TaskProgressModel struct {
    project      *db.Project
    tasks        []*db.Task
    currentIdx   int
    iteration    int
    agentType    string          // "developer" or "reviewer"
    output       strings.Builder
    viewport     viewport.Model
    width        int
    height       int
    completed    bool
    err          error

    // Engine event channel
    engineEvents <-chan engine.EngineEvent
}

func NewTaskProgressModel(project *db.Project, events <-chan engine.EngineEvent) TaskProgressModel {
    return TaskProgressModel{
        project:      project,
        engineEvents: events,
    }
}
```

### Custom Messages

```go
// EngineEventMsg wraps engine events for Bubble Tea
type EngineEventMsg struct {
    Event engine.EngineEvent
}

// TasksLoadedMsg signals tasks are ready
type TasksLoadedMsg struct {
    Tasks []*db.Task
}
```

### Init - Start Listening

```go
func (m TaskProgressModel) Init() tea.Cmd {
    return tea.Batch(
        m.loadTasks,
        m.listenForEvents,
    )
}

func (m TaskProgressModel) listenForEvents() tea.Msg {
    event, ok := <-m.engineEvents
    if !ok {
        return nil  // Channel closed
    }
    return EngineEventMsg{Event: event}
}
```

### Update - Handle Events

```go
func (m TaskProgressModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
    var cmds []tea.Cmd

    switch msg := msg.(type) {
    case tea.WindowSizeMsg:
        m.width = msg.Width
        m.height = msg.Height
        m.viewport.Width = msg.Width
        m.viewport.Height = msg.Height - 10  // Reserve space for header/footer
        return m, nil

    case TasksLoadedMsg:
        m.tasks = msg.Tasks
        return m, nil

    case EngineEventMsg:
        m.handleEngineEvent(msg.Event)
        // Continue listening
        cmds = append(cmds, m.listenForEvents)

    case tea.KeyMsg:
        switch msg.String() {
        case "q", "ctrl+c":
            return m, tea.Quit
        case "up", "k":
            m.viewport.LineUp(1)
        case "down", "j":
            m.viewport.LineDown(1)
        case "pgup":
            m.viewport.ViewUp()
        case "pgdown":
            m.viewport.ViewDown()
        case "g":
            m.viewport.GotoTop()
        case "G":
            m.viewport.GotoBottom()
        }
    }

    // Update viewport
    var cmd tea.Cmd
    m.viewport, cmd = m.viewport.Update(msg)
    cmds = append(cmds, cmd)

    return m, tea.Batch(cmds...)
}
```

### Handle Engine Events

```go
func (m *TaskProgressModel) handleEngineEvent(event engine.EngineEvent) {
    switch event.Type {
    case engine.EngineEventTasksCreated:
        m.output.WriteString(fmt.Sprintf("Created %s\n", event.Message))

    case engine.EngineEventRunning:
        if event.TaskLoopEvent != nil {
            m.handleTaskLoopEvent(*event.TaskLoopEvent)
        }

    case engine.EngineEventCompleted:
        m.completed = true
        m.output.WriteString("\n--- All tasks completed! ---\n")

    case engine.EngineEventFailed:
        m.err = fmt.Errorf(event.Message)
        m.output.WriteString(fmt.Sprintf("\n--- Failed: %s ---\n", event.Message))
    }

    m.viewport.SetContent(m.output.String())
    m.viewport.GotoBottom()
}

func (m *TaskProgressModel) handleTaskLoopEvent(event engine.TaskLoopEvent) {
    switch event.Type {
    case engine.TaskEventTaskBegin:
        m.currentIdx = event.TaskIndex
        m.output.WriteString(fmt.Sprintf("\n=== Task %d: %s ===\n", event.TaskIndex+1, event.TaskTitle))

    case engine.TaskEventTaskEnd:
        m.output.WriteString(fmt.Sprintf("Task completed.\n"))

    case engine.TaskEventFailed:
        m.output.WriteString(fmt.Sprintf("Task failed: %s\n", event.Message))
    }

    // Handle nested review events
    if event.ReviewEvent != nil {
        m.handleReviewEvent(*event.ReviewEvent)
    }
}

func (m *TaskProgressModel) handleReviewEvent(event engine.ReviewLoopEvent) {
    switch event.Type {
    case engine.EventDeveloping:
        m.agentType = "developer"
        m.iteration = event.Iteration
        m.output.WriteString(fmt.Sprintf("  [Iteration %d] Developer working...\n", event.Iteration))

    case engine.EventReviewing:
        m.agentType = "reviewer"
        m.output.WriteString(fmt.Sprintf("  [Iteration %d] Reviewer checking...\n", event.Iteration))

    case engine.EventFeedback:
        m.output.WriteString(fmt.Sprintf("  Feedback: %s\n", truncate(event.Message, 100)))

    case engine.EventApproved:
        m.output.WriteString("  Approved!\n")
    }
}
```

### View - Multi-Panel Layout

```go
func (m TaskProgressModel) View() string {
    if m.err != nil && !m.completed {
        return fmt.Sprintf("Error: %v\n\nPress q to quit", m.err)
    }

    var s strings.Builder

    // Header
    s.WriteString(m.renderHeader())
    s.WriteString("\n")

    // Task list sidebar + output viewport
    s.WriteString(m.renderBody())
    s.WriteString("\n")

    // Footer
    s.WriteString(m.renderFooter())

    return s.String()
}

func (m TaskProgressModel) renderHeader() string {
    title := titleStyle.Render(fmt.Sprintf("Ralph - %s", m.project.Name))

    progress := ""
    if len(m.tasks) > 0 {
        completed := m.countCompleted()
        progress = fmt.Sprintf(" [%d/%d tasks]", completed, len(m.tasks))
    }

    return lipgloss.JoinHorizontal(lipgloss.Top, title, progress)
}

func (m TaskProgressModel) renderBody() string {
    // Left: Task list
    taskList := m.renderTaskList()

    // Right: Output viewport
    outputView := m.viewport.View()

    // Join horizontally
    return lipgloss.JoinHorizontal(
        lipgloss.Top,
        taskListStyle.Render(taskList),
        outputStyle.Render(outputView),
    )
}

func (m TaskProgressModel) renderTaskList() string {
    var s strings.Builder

    for i, task := range m.tasks {
        prefix := "  "
        if i == m.currentIdx {
            prefix = "> "
        }

        status := m.taskStatusIcon(task.Status)
        line := fmt.Sprintf("%s%s %s", prefix, status, truncate(task.Title, 20))
        s.WriteString(line)
        s.WriteString("\n")
    }

    return s.String()
}

func (m TaskProgressModel) taskStatusIcon(status db.TaskStatus) string {
    switch status {
    case db.TaskPending:
        return statusPending.Render("○")
    case db.TaskInProgress:
        return statusInProgress.Render("◐")
    case db.TaskCompleted:
        return statusCompleted.Render("●")
    case db.TaskFailed:
        return statusFailed.Render("✗")
    case db.TaskEscalated:
        return statusFailed.Render("!")
    default:
        return "?"
    }
}

func (m TaskProgressModel) renderFooter() string {
    var left, right string

    if m.completed {
        left = "Completed!"
    } else {
        left = fmt.Sprintf("Iteration %d - %s", m.iteration, m.agentType)
    }

    right = "j/k: scroll • q: quit"

    return lipgloss.JoinHorizontal(
        lipgloss.Top,
        helpStyle.Render(left),
        strings.Repeat(" ", max(0, m.width-len(left)-len(right)-4)),
        helpStyle.Render(right),
    )
}
```

### Viewport Setup

Use Bubble Tea's viewport component:

```go
import "github.com/charmbracelet/bubbles/viewport"

func (m *TaskProgressModel) initViewport() {
    m.viewport = viewport.New(m.width-30, m.height-10)
    m.viewport.SetContent(m.output.String())
}
```

## Files to Modify

- `internal/tui/progress.go` - Create with TaskProgressModel
- `internal/tui/app.go` - Integrate with main model
- `internal/tui/styles.go` - Add additional styles
- `internal/tui/progress_test.go` - Create with tests

## Add Dependencies

Update go.mod:
```
github.com/charmbracelet/bubbles v0.20.0
```

## Testing Strategy

1. **Model tests** - Event handling, state transitions
2. **View tests** - Rendered output correctness
3. **Event flow** - Mock engine events
4. **Scrolling** - Viewport behavior

## Dependencies

- `internal/engine` - For event types
- `internal/db` - For task/project models
- `github.com/charmbracelet/bubbles/viewport` - Scrollable output

## Notes

- The viewport should auto-scroll to bottom when new content arrives
- User can pause auto-scroll by scrolling up
- Consider adding a spinner for active operations
- Output should be formatted nicely (syntax highlighting for code?)
- May want to show Claude's streaming text in real-time (requires forwarding text events)
