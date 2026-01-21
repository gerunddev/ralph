# Task 12: CLI Flags and App Bootstrap

## Context

The CLI needs `-c` flag to create a project from a plan file. The app bootstrap wires together config, database, engine, and TUI. Currently `cmd/ralph/main.go` has a stub and `internal/app/app.go` is empty.

## Objective

Implement CLI flags and the application bootstrap that ties all components together.

## Acceptance Criteria

- [ ] `-c <path>` or `--create <path>` flag creates project from plan file
- [ ] Without flags, show project selection TUI
- [ ] Load configuration from standard location
- [ ] Initialize database connection
- [ ] Create engine with all dependencies
- [ ] Start TUI with appropriate initial view
- [ ] Handle startup errors gracefully
- [ ] Clean shutdown on exit

## Implementation Details

### CLI Structure

```go
// cmd/ralph/main.go

func main() {
    if err := run(); err != nil {
        fmt.Fprintf(os.Stderr, "error: %v\n", err)
        os.Exit(1)
    }
}

func run() error {
    var createPath string

    rootCmd := &cobra.Command{
        Use:   "ralph",
        Short: "Ralph automates Claude Code sessions using plan-based development",
        Long: `Ralph is a TUI application that automates Claude Code sessions using a
plan-based development workflow. It implements the "Ralph Loop" pattern: breaking
a plan into discrete tasks, then iterating each task through developer→reviewer
cycles until complete.`,
        RunE: func(cmd *cobra.Command, args []string) error {
            return app.Run(app.Options{
                CreateFromPlan: createPath,
            })
        },
    }

    rootCmd.Flags().StringVarP(&createPath, "create", "c", "",
        "Create a new project from the specified plan file")

    return rootCmd.Execute()
}
```

### App Options

```go
// internal/app/app.go

type Options struct {
    CreateFromPlan string  // Path to plan file, empty for selection mode
}
```

### App Bootstrap

```go
func Run(opts Options) error {
    // 1. Load configuration
    cfg, err := config.Load()
    if err != nil {
        return fmt.Errorf("failed to load config: %w", err)
    }

    // 2. Open database
    database, err := db.New(cfg.GetDatabasePath())
    if err != nil {
        return fmt.Errorf("failed to open database: %w", err)
    }
    defer database.Close()

    // 3. Get working directory
    workDir, err := os.Getwd()
    if err != nil {
        return fmt.Errorf("failed to get working directory: %w", err)
    }

    // 4. Create engine
    eng, err := engine.NewEngine(engine.EngineConfig{
        Config:  cfg,
        DB:      database,
        WorkDir: workDir,
    })
    if err != nil {
        return fmt.Errorf("failed to create engine: %w", err)
    }
    defer eng.Stop()

    // 5. Handle create mode vs selection mode
    if opts.CreateFromPlan != "" {
        return runCreateMode(eng, opts.CreateFromPlan)
    }

    return runSelectionMode(database, eng)
}
```

### Create Mode

```go
func runCreateMode(eng *engine.Engine, planPath string) error {
    ctx, cancel := context.WithCancel(context.Background())
    defer cancel()

    // Create project
    project, err := eng.CreateProject(ctx, planPath)
    if err != nil {
        return fmt.Errorf("failed to create project: %w", err)
    }

    // Start TUI with progress view
    model := tui.NewProgressModel(project, eng.Events())
    p := tea.NewProgram(model, tea.WithAltScreen())

    // Run engine in background
    errCh := make(chan error, 1)
    go func() {
        errCh <- eng.Run(ctx)
    }()

    // Run TUI (blocks until quit)
    if _, err := p.Run(); err != nil {
        cancel()  // Stop engine
        return err
    }

    // Check for engine error
    select {
    case err := <-errCh:
        return err
    default:
        return nil
    }
}
```

### Selection Mode

```go
func runSelectionMode(database *db.DB, eng *engine.Engine) error {
    // Start TUI with project list
    model := tui.NewMainModel(database, eng)
    p := tea.NewProgram(model, tea.WithAltScreen())

    _, err := p.Run()
    return err
}
```

### Main TUI Model

```go
// internal/tui/app.go

type MainModel struct {
    state         ViewState
    projectList   ProjectListModel
    taskProgress  *TaskProgressModel
    db            *db.DB
    engine        *engine.Engine
    width, height int
}

type ViewState int

const (
    ViewProjectList ViewState = iota
    ViewTaskProgress
)

func NewMainModel(database *db.DB, eng *engine.Engine) MainModel {
    return MainModel{
        state:       ViewProjectList,
        projectList: NewProjectListModel(database),
        db:          database,
        engine:      eng,
    }
}

func (m MainModel) Init() tea.Cmd {
    return m.projectList.Init()
}

func (m MainModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
    switch msg := msg.(type) {
    case tea.WindowSizeMsg:
        m.width = msg.Width
        m.height = msg.Height
        // Forward to current view
        if m.state == ViewProjectList {
            var cmd tea.Cmd
            m.projectList, cmd = m.projectList.Update(msg).(ProjectListModel)
            return m, cmd
        }

    case ProjectSelectedMsg:
        return m, m.startProject(msg.Project)
    }

    // Route to current view
    switch m.state {
    case ViewProjectList:
        model, cmd := m.projectList.Update(msg)
        m.projectList = model.(ProjectListModel)
        return m, cmd

    case ViewTaskProgress:
        if m.taskProgress != nil {
            model, cmd := m.taskProgress.Update(msg)
            tp := model.(TaskProgressModel)
            m.taskProgress = &tp
            return m, cmd
        }
    }

    return m, nil
}

func (m *MainModel) startProject(project *db.Project) tea.Cmd {
    return func() tea.Msg {
        ctx := context.Background()

        // Resume or start project
        if project.Status == db.ProjectPending {
            // Already have tasks, just run
        }

        if _, err := m.engine.ResumeProject(ctx, project.ID); err != nil {
            return ErrorMsg{Err: err}
        }

        // Switch to progress view
        m.state = ViewTaskProgress
        progress := NewTaskProgressModel(project, m.engine.Events())
        m.taskProgress = &progress

        // Start engine in background
        go m.engine.Run(ctx)

        return nil
    }
}

func (m MainModel) View() string {
    switch m.state {
    case ViewProjectList:
        return m.projectList.View()
    case ViewTaskProgress:
        if m.taskProgress != nil {
            return m.taskProgress.View()
        }
    }
    return "Loading..."
}
```

### Error Handling

```go
type ErrorMsg struct {
    Err error
}

func (m MainModel) handleError(err error) tea.Cmd {
    // Could show error modal or quit
    return tea.Quit
}
```

### Alt Screen Mode

Use `tea.WithAltScreen()` for full terminal takeover.

## Files to Modify

- `cmd/ralph/main.go` - Add -c flag
- `internal/app/app.go` - Full bootstrap implementation
- `internal/tui/app.go` - MainModel implementation

## Testing Strategy

1. **CLI tests** - Flag parsing
2. **Bootstrap tests** - Mock dependencies
3. **Integration** - End-to-end with test database

## Dependencies

- All internal packages
- `github.com/spf13/cobra` - CLI framework

## Notes

- Alt screen gives a cleaner experience
- Consider adding `--verbose` flag for debug output
- Config errors should be user-friendly (suggest creating config file)
- Database is created automatically if it doesn't exist
