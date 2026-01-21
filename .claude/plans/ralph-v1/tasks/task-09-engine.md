# Task 9: Engine Orchestration

## Context

The engine is the top-level orchestrator that ties together project selection, task planning, task execution, and user feedback. The stub exists at `internal/engine/engine.go`.

## Objective

Implement the main engine that orchestrates the complete Ralph workflow from project creation through task completion.

## Acceptance Criteria

- [ ] `NewEngine(config)` creates engine with configuration
- [ ] `CreateProject(planPath)` reads plan file, creates project, runs planner
- [ ] `ResumeProject(projectID)` resumes an existing project
- [ ] `Run(ctx, project)` executes the full workflow
- [ ] Planner agent parses plan into tasks
- [ ] Task loop runs all tasks
- [ ] Engine emits events for TUI updates
- [ ] Graceful shutdown on context cancellation
- [ ] Error handling at each stage

## Implementation Details

### Engine Structure

```go
type Engine struct {
    config  *config.Config
    db      *db.DB
    claude  *claude.Client
    jj      *jj.Client
    agents  *agents.Manager

    events  chan EngineEvent
    project *db.Project
}

type EngineConfig struct {
    Config  *config.Config
    DB      *db.DB
    WorkDir string  // For jj client
}

func NewEngine(cfg EngineConfig) (*Engine, error) {
    claude := claude.NewClient(claude.ClientConfig{
        Model:        cfg.Config.Claude.Model,
        MaxTurns:     cfg.Config.Claude.MaxTurns,
        MaxBudgetUSD: cfg.Config.Claude.MaxBudgetUSD,
    })

    jj := jj.NewClient(cfg.WorkDir)

    agents := agents.NewManager(cfg.Config)

    return &Engine{
        config:  cfg.Config,
        db:      cfg.DB,
        claude:  claude,
        jj:      jj,
        agents:  agents,
        events:  make(chan EngineEvent, 100),
    }, nil
}
```

### Engine Events

```go
type EngineEventType string

const (
    EngineEventCreatingProject EngineEventType = "creating_project"
    EngineEventPlanningTasks   EngineEventType = "planning_tasks"
    EngineEventTasksCreated    EngineEventType = "tasks_created"
    EngineEventRunning         EngineEventType = "running"
    EngineEventCompleted       EngineEventType = "completed"
    EngineEventFailed          EngineEventType = "failed"
)

type EngineEvent struct {
    Type          EngineEventType
    Message       string
    TaskLoopEvent *TaskLoopEvent  // Nested events
}

func (e *Engine) Events() <-chan EngineEvent {
    return e.events
}
```

### Create Project from Plan

```go
func (e *Engine) CreateProject(ctx context.Context, planPath string) (*db.Project, error) {
    e.emit(EngineEventCreatingProject, "Reading plan file")

    // Read plan file
    planText, err := os.ReadFile(planPath)
    if err != nil {
        return nil, fmt.Errorf("failed to read plan file: %w", err)
    }

    // Create project
    project := &db.Project{
        ID:       uuid.New().String(),
        Name:     filepath.Base(planPath),
        PlanText: string(planText),
        Status:   db.ProjectPending,
    }

    if err := e.db.CreateProject(project); err != nil {
        return nil, fmt.Errorf("failed to create project: %w", err)
    }

    // Run planner to break into tasks
    e.emit(EngineEventPlanningTasks, "Breaking plan into tasks")
    if err := e.planTasks(ctx, project); err != nil {
        return nil, fmt.Errorf("failed to plan tasks: %w", err)
    }

    e.project = project
    return project, nil
}
```

### Plan Tasks with Planner Agent

```go
func (e *Engine) planTasks(ctx context.Context, project *db.Project) error {
    agent := agents.PlannerAgent(project.PlanText)

    // Run Claude with planner prompt
    session, err := e.claude.Run(ctx, agent.Prompt, "")
    if err != nil {
        return err
    }

    // Collect output
    var output strings.Builder
    for event := range session.Events() {
        if event.Message != nil {
            output.WriteString(event.Message.Text)
        }
    }

    if err := session.Wait(); err != nil {
        return err
    }

    // Parse JSON tasks from output
    tasks, err := parsePlannerOutput(output.String())
    if err != nil {
        return fmt.Errorf("failed to parse planner output: %w", err)
    }

    // Create task records
    dbTasks := make([]*db.Task, len(tasks))
    for i, t := range tasks {
        dbTasks[i] = &db.Task{
            ID:          uuid.New().String(),
            ProjectID:   project.ID,
            Sequence:    t.Sequence,
            Title:       t.Title,
            Description: t.Description,
            Status:      db.TaskPending,
        }
    }

    if err := e.db.CreateTasks(dbTasks); err != nil {
        return err
    }

    e.emit(EngineEventTasksCreated, fmt.Sprintf("Created %d tasks", len(dbTasks)))
    return nil
}

type plannerTask struct {
    Title       string `json:"title"`
    Description string `json:"description"`
    Sequence    int    `json:"sequence"`
}

func parsePlannerOutput(output string) ([]plannerTask, error) {
    // Find JSON array in output
    start := strings.Index(output, "[")
    end := strings.LastIndex(output, "]")
    if start == -1 || end == -1 {
        return nil, fmt.Errorf("no JSON array found in planner output")
    }

    var tasks []plannerTask
    if err := json.Unmarshal([]byte(output[start:end+1]), &tasks); err != nil {
        return nil, err
    }

    return tasks, nil
}
```

### Resume Project

```go
func (e *Engine) ResumeProject(ctx context.Context, projectID string) (*db.Project, error) {
    project, err := e.db.GetProject(projectID)
    if err != nil {
        return nil, err
    }

    e.project = project
    return project, nil
}
```

### Run Main Workflow

```go
func (e *Engine) Run(ctx context.Context) error {
    if e.project == nil {
        return fmt.Errorf("no project loaded")
    }

    e.emit(EngineEventRunning, "Starting task execution")

    // Create task loop
    taskLoop := NewTaskLoop(TaskLoopDeps{
        DB:      e.db,
        Claude:  e.claude,
        JJ:      e.jj,
        Agents:  e.agents,
        Config:  e.config,
    }, e.project)

    // Forward task loop events
    go func() {
        for event := range taskLoop.Events() {
            e.events <- EngineEvent{
                Type:          EngineEventRunning,
                TaskLoopEvent: &event,
            }
        }
    }()

    if err := taskLoop.Run(ctx); err != nil {
        e.emit(EngineEventFailed, err.Error())
        return err
    }

    e.emit(EngineEventCompleted, "All tasks completed")
    return nil
}
```

### Graceful Shutdown

```go
func (e *Engine) Stop() error {
    close(e.events)
    return nil
}
```

## Files to Modify

- `internal/engine/engine.go` - Full implementation
- `internal/engine/engine_test.go` - Create with tests

## Testing Strategy

1. **Unit tests** - Mock all dependencies
2. **Create project** - Read plan, run planner, create tasks
3. **Resume project** - Load existing project
4. **Run workflow** - Complete task loop
5. **Error handling** - Various failure scenarios

## Dependencies

- `internal/config` - Configuration
- `internal/db` - Database operations
- `internal/claude` - Claude client
- `internal/jj` - JJ client
- `internal/agents` - Agent management
- `internal/engine/task_loop` - Task execution

## Notes

- The engine is the main entry point for the app
- It owns all the dependency instances
- The TUI will hold an Engine instance and react to its events
- CreateProject vs ResumeProject determines the startup flow
