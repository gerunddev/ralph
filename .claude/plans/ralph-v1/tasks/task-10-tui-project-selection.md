# Task 10: TUI - Project Selection

## Context

The TUI needs to show a list of existing projects for the user to select from, or indicate they're creating a new project. This is the first screen users see when launching Ralph without flags. Uses Bubble Tea and Lip Gloss from Charm.

## Objective

Implement the project selection view that lists existing projects and allows navigation/selection.

## Acceptance Criteria

- [ ] Display list of projects from database (most recent first)
- [ ] Show project name, status, and last updated timestamp
- [ ] Keyboard navigation (j/k or arrows to move, Enter to select)
- [ ] Visual highlighting of selected project
- [ ] Status indicators (colors/icons for pending/in_progress/completed/failed)
- [ ] Handle empty state (no projects yet)
- [ ] "New Project" option at top (when launched with -c flag)
- [ ] Quit with q or Ctrl+C
- [ ] Responsive to terminal size

## Implementation Details

### Project List Model

```go
type ProjectListModel struct {
    db       *db.DB
    projects []*db.Project
    cursor   int
    width    int
    height   int
    loading  bool
    err      error
}

func NewProjectListModel(database *db.DB) ProjectListModel {
    return ProjectListModel{
        db:      database,
        loading: true,
    }
}
```

### Messages

```go
// ProjectsLoadedMsg is sent when projects are loaded from DB
type ProjectsLoadedMsg struct {
    Projects []*db.Project
    Err      error
}

// ProjectSelectedMsg is sent when user selects a project
type ProjectSelectedMsg struct {
    Project *db.Project
}
```

### Init - Load Projects

```go
func (m ProjectListModel) Init() tea.Cmd {
    return m.loadProjects
}

func (m ProjectListModel) loadProjects() tea.Msg {
    projects, err := m.db.ListProjects()
    return ProjectsLoadedMsg{Projects: projects, Err: err}
}
```

### Update

```go
func (m ProjectListModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
    switch msg := msg.(type) {
    case tea.WindowSizeMsg:
        m.width = msg.Width
        m.height = msg.Height
        return m, nil

    case ProjectsLoadedMsg:
        m.loading = false
        if msg.Err != nil {
            m.err = msg.Err
            return m, nil
        }
        m.projects = msg.Projects
        return m, nil

    case tea.KeyMsg:
        switch msg.String() {
        case "q", "ctrl+c":
            return m, tea.Quit

        case "up", "k":
            if m.cursor > 0 {
                m.cursor--
            }

        case "down", "j":
            if m.cursor < len(m.projects)-1 {
                m.cursor++
            }

        case "enter":
            if len(m.projects) > 0 {
                return m, func() tea.Msg {
                    return ProjectSelectedMsg{Project: m.projects[m.cursor]}
                }
            }
        }
    }

    return m, nil
}
```

### View with Lip Gloss Styling

```go
import "github.com/charmbracelet/lipgloss"

var (
    titleStyle = lipgloss.NewStyle().
        Bold(true).
        Foreground(lipgloss.Color("205")).
        MarginBottom(1)

    selectedStyle = lipgloss.NewStyle().
        Foreground(lipgloss.Color("229")).
        Background(lipgloss.Color("57")).
        Bold(true).
        Padding(0, 1)

    normalStyle = lipgloss.NewStyle().
        Padding(0, 1)

    statusPending = lipgloss.NewStyle().
        Foreground(lipgloss.Color("246"))

    statusInProgress = lipgloss.NewStyle().
        Foreground(lipgloss.Color("214"))

    statusCompleted = lipgloss.NewStyle().
        Foreground(lipgloss.Color("42"))

    statusFailed = lipgloss.NewStyle().
        Foreground(lipgloss.Color("196"))

    helpStyle = lipgloss.NewStyle().
        Foreground(lipgloss.Color("241")).
        MarginTop(1)
)

func (m ProjectListModel) View() string {
    if m.loading {
        return "Loading projects..."
    }

    if m.err != nil {
        return fmt.Sprintf("Error: %v", m.err)
    }

    var s strings.Builder

    s.WriteString(titleStyle.Render("Ralph - Select a Project"))
    s.WriteString("\n\n")

    if len(m.projects) == 0 {
        s.WriteString("No projects yet. Create one with: ralph -c <plan-file>\n")
    } else {
        for i, p := range m.projects {
            var style lipgloss.Style
            if i == m.cursor {
                style = selectedStyle
            } else {
                style = normalStyle
            }

            statusStr := m.formatStatus(p.Status)
            timeStr := p.UpdatedAt.Format("Jan 02 15:04")

            line := fmt.Sprintf("%s  %s  %s", p.Name, statusStr, timeStr)
            s.WriteString(style.Render(line))
            s.WriteString("\n")
        }
    }

    s.WriteString(helpStyle.Render("j/k: navigate • enter: select • q: quit"))

    return s.String()
}

func (m ProjectListModel) formatStatus(status db.ProjectStatus) string {
    switch status {
    case db.ProjectPending:
        return statusPending.Render("[pending]")
    case db.ProjectInProgress:
        return statusInProgress.Render("[in progress]")
    case db.ProjectCompleted:
        return statusCompleted.Render("[completed]")
    case db.ProjectFailed:
        return statusFailed.Render("[failed]")
    default:
        return string(status)
    }
}
```

### Integration with Main Model

The main TUI model switches between views:

```go
type MainModel struct {
    state        ViewState
    projectList  ProjectListModel
    taskProgress TaskProgressModel
    db           *db.DB
    engine       *engine.Engine
}

type ViewState int

const (
    ViewProjectList ViewState = iota
    ViewTaskProgress
)

func (m MainModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
    switch msg := msg.(type) {
    case ProjectSelectedMsg:
        m.state = ViewTaskProgress
        // Initialize engine and start
        return m, m.startProject(msg.Project)
    }

    switch m.state {
    case ViewProjectList:
        var cmd tea.Cmd
        m.projectList, cmd = m.projectList.Update(msg)
        return m, cmd
    case ViewTaskProgress:
        // Handle in Task 11
    }
    return m, nil
}
```

## Files to Modify

- `internal/tui/projects.go` - Create with ProjectListModel
- `internal/tui/styles.go` - Create with shared Lip Gloss styles
- `internal/tui/app.go` - Update to use ProjectListModel
- `internal/tui/projects_test.go` - Create with tests

## Testing Strategy

1. **Model tests** - Test Update logic for key presses
2. **View tests** - Verify rendered output
3. **Empty state** - No projects
4. **Selection** - Correct project returned

## Dependencies

- `internal/db` - For loading projects
- `github.com/charmbracelet/bubbletea` - TUI framework
- `github.com/charmbracelet/lipgloss` - Styling

## Notes

- When launched with `-c flag`, skip this view and go straight to task progress
- The selected project is passed to the engine for resumption
- Consider adding a confirmation before deleting/resetting a failed project
- Status colors follow semantic conventions (green=good, red=bad, yellow=in progress)
