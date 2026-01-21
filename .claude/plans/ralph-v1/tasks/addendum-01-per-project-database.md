# Addendum 01: Per-Project SQLite Database

## Context

Currently Ralph uses a single global SQLite database (`~/.local/share/ralph/ralph.db`) for all projects. This creates backwards compatibility challenges as the schema evolves. By giving each project its own database file, we can:

1. Iterate on schema without migration concerns for old projects
2. Keep project data isolated and portable
3. Delete a project by simply removing its database file
4. Archive/share projects as self-contained files

## Objective

Change Ralph to create a new SQLite database file for each project, stored in a project-specific directory.

## Acceptance Criteria

- [ ] Each new project creates a database at `~/.local/share/ralph/projects/<project-id>/ralph.db`
- [ ] Project selection UI scans the projects directory for existing databases
- [ ] Global database is no longer used (or is only for global settings/index)
- [ ] Old single-database behavior is deprecated/removed
- [ ] Database path is deterministic from project ID
- [ ] Config can specify a custom base projects directory
- [ ] Empty/corrupt project databases are handled gracefully

## Implementation Details

### Directory Structure

```
~/.local/share/ralph/
├── config.json             # Global config (optional)
└── projects/
    ├── abc123/
    │   └── ralph.db        # Project-specific database
    ├── def456/
    │   └── ralph.db
    └── ...
```

### Config Changes

```go
// internal/config/config.go

type Config struct {
    // Existing fields...

    // ProjectsDir is the base directory for project databases
    // Default: ~/.local/share/ralph/projects
    ProjectsDir string `json:"projects_dir"`
}

func Defaults() *Config {
    return &Config{
        // ...existing...
        ProjectsDir: "~/.local/share/ralph/projects",
    }
}
```

### Database Path Resolution

```go
// internal/db/project_db.go

// ProjectDBPath returns the database path for a given project ID
func ProjectDBPath(projectsDir, projectID string) string {
    return filepath.Join(projectsDir, projectID, "ralph.db")
}

// OpenProjectDB opens or creates the database for a specific project
func OpenProjectDB(projectsDir, projectID string) (*DB, error) {
    dbPath := ProjectDBPath(projectsDir, projectID)
    return New(dbPath)
}
```

### Project Discovery

```go
// internal/db/discovery.go

// DiscoverProjects scans the projects directory for existing databases
// and returns basic project info without opening each database
func DiscoverProjects(projectsDir string) ([]ProjectInfo, error) {
    entries, err := os.ReadDir(projectsDir)
    if err != nil {
        if os.IsNotExist(err) {
            return nil, nil  // No projects yet
        }
        return nil, err
    }

    var projects []ProjectInfo
    for _, entry := range entries {
        if !entry.IsDir() {
            continue
        }

        dbPath := filepath.Join(projectsDir, entry.Name(), "ralph.db")
        if _, err := os.Stat(dbPath); err == nil {
            // Database exists, open briefly to get project info
            info, err := getProjectInfo(dbPath)
            if err != nil {
                // Log warning but continue
                continue
            }
            projects = append(projects, info)
        }
    }

    return projects, nil
}

type ProjectInfo struct {
    ID        string
    Name      string
    Status    ProjectStatus
    UpdatedAt time.Time
    DBPath    string
}
```

### App Bootstrap Changes

```go
// internal/app/app.go

func Run(opts Options) error {
    cfg, err := config.Load()
    if err != nil {
        return fmt.Errorf("failed to load config: %w", err)
    }

    // Ensure projects directory exists
    projectsDir := cfg.GetProjectsDir()
    if err := os.MkdirAll(projectsDir, 0755); err != nil {
        return fmt.Errorf("failed to create projects directory: %w", err)
    }

    if opts.CreateFromPlan != "" {
        return runCreateMode(cfg, opts.CreateFromPlan)
    }

    return runSelectionMode(cfg)
}

func runCreateMode(cfg *config.Config, planPath string) error {
    // Generate project ID
    projectID := generateProjectID()

    // Create project-specific database
    database, err := db.OpenProjectDB(cfg.GetProjectsDir(), projectID)
    if err != nil {
        return fmt.Errorf("failed to create project database: %w", err)
    }
    defer database.Close()

    // Continue with engine creation...
}

func runSelectionMode(cfg *config.Config) error {
    // Discover existing projects
    projects, err := db.DiscoverProjects(cfg.GetProjectsDir())
    if err != nil {
        return fmt.Errorf("failed to discover projects: %w", err)
    }

    // Show TUI with discovered projects
    model := tui.NewProjectListModel(projects, cfg)
    // ...
}
```

### TUI Changes

The project selection TUI needs to:
1. Display discovered projects (from scanning)
2. When a project is selected, open its specific database
3. Handle database open errors gracefully

```go
// internal/tui/project_list.go

type ProjectListModel struct {
    projects []db.ProjectInfo  // Changed from []*db.Project
    cfg      *config.Config
    // ...
}

// When project is selected:
func (m *ProjectListModel) openProject(info db.ProjectInfo) tea.Cmd {
    return func() tea.Msg {
        database, err := db.OpenProjectDB(m.cfg.GetProjectsDir(), info.ID)
        if err != nil {
            return ErrorMsg{Err: err}
        }

        project, err := database.GetProject(info.ID)
        if err != nil {
            database.Close()
            return ErrorMsg{Err: err}
        }

        return ProjectOpenedMsg{
            Project:  project,
            Database: database,
        }
    }
}
```

### ID Generation

```go
// internal/db/id.go

import "github.com/google/uuid"

func GenerateProjectID() string {
    return uuid.New().String()[:8]  // Short 8-char ID
}
```

## Files to Modify

- `internal/config/config.go` - Add `ProjectsDir` field
- `internal/db/project_db.go` - Create new file for project DB helpers
- `internal/db/discovery.go` - Create new file for project discovery
- `internal/app/app.go` - Update bootstrap to use per-project DBs
- `internal/tui/project_list.go` - Update to use discovered projects
- `internal/tui/app.go` - Handle database lifecycle per project
- `cmd/ralph/main.go` - No changes expected

## Testing Strategy

1. **Unit tests** - Path generation, discovery logic
2. **Integration tests** - Create multiple projects, verify isolation
3. **Edge cases** - Corrupt database, missing directory, permissions
4. **Migration** - Optional: tool to migrate from single DB to per-project

## Dependencies

- Existing `internal/db` package
- `github.com/google/uuid` for ID generation (or use existing ID scheme)

## Migration Notes

If there are existing users with the old single-database approach:
1. The old `DatabasePath` config field could be deprecated
2. Optionally provide a migration command to split existing projects
3. Or simply document that old projects won't appear in the new UI

## Notes

- Project ID should be filesystem-safe (no special characters)
- Consider using creation timestamp in ID for natural sorting
- Database connections should be short-lived (open when needed, close when done)
- The TUI should show helpful message when no projects exist
