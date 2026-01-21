# Addendum 02: Task Content Export/Import CLI

## Context

During Ralph execution, you may want to modify task plans on the fly - perhaps to add clarification, change scope, or fix issues discovered during development. Currently, task content is stored in the database with no easy way to edit it.

This feature adds CLI commands to:
1. Export a task's content (description) to a file
2. Import modified content back into the task

This enables a simple workflow: `export -> edit in your editor -> import`

## Objective

Add CLI commands for exporting and importing task content, enabling on-the-fly task plan modifications.

## Acceptance Criteria

- [ ] `ralph task export <project-id> <task-sequence>` exports task description to stdout or file
- [ ] `ralph task import <project-id> <task-sequence> <file>` imports content from file
- [ ] Export includes task metadata (title, status) as comments/header
- [ ] Import validates task exists and is not completed
- [ ] Support for `--output <file>` flag on export
- [ ] Support for `-` to read from stdin on import
- [ ] Clear error messages for invalid project/task IDs
- [ ] Confirmation prompt before overwriting (with `--force` to skip)

## Implementation Details

### CLI Commands

```go
// cmd/ralph/main.go

func main() {
    rootCmd := &cobra.Command{...}

    // Add task subcommand group
    taskCmd := &cobra.Command{
        Use:   "task",
        Short: "Task management commands",
    }

    taskCmd.AddCommand(taskExportCmd())
    taskCmd.AddCommand(taskImportCmd())

    rootCmd.AddCommand(taskCmd)
}
```

### Export Command

```go
// cmd/ralph/task_export.go

func taskExportCmd() *cobra.Command {
    var outputFile string
    var includeMetadata bool

    cmd := &cobra.Command{
        Use:   "export <project-id> <task-sequence>",
        Short: "Export task description to a file",
        Long: `Export a task's description content to stdout or a file.

The exported content can be edited and re-imported using 'ralph task import'.

Examples:
  ralph task export abc123 3              # Export task 3 to stdout
  ralph task export abc123 3 -o task.md   # Export to file
  ralph task export abc123 3 | vim -      # Pipe to editor`,
        Args: cobra.ExactArgs(2),
        RunE: func(cmd *cobra.Command, args []string) error {
            projectID := args[0]
            sequence, err := strconv.Atoi(args[1])
            if err != nil {
                return fmt.Errorf("invalid task sequence: %s", args[1])
            }

            return runTaskExport(projectID, sequence, outputFile, includeMetadata)
        },
    }

    cmd.Flags().StringVarP(&outputFile, "output", "o", "", "Output file (default: stdout)")
    cmd.Flags().BoolVar(&includeMetadata, "metadata", true, "Include task metadata as header comments")

    return cmd
}

func runTaskExport(projectID string, sequence int, outputFile string, includeMetadata bool) error {
    cfg, err := config.Load()
    if err != nil {
        return err
    }

    // Open project database
    database, err := db.OpenProjectDB(cfg.GetProjectsDir(), projectID)
    if err != nil {
        return fmt.Errorf("failed to open project %s: %w", projectID, err)
    }
    defer database.Close()

    // Get task
    task, err := database.GetTaskBySequence(projectID, sequence)
    if err != nil {
        return fmt.Errorf("task %d not found in project %s: %w", sequence, projectID, err)
    }

    // Build content
    var content strings.Builder

    if includeMetadata {
        content.WriteString(fmt.Sprintf("<!-- Task: %s -->\n", task.Title))
        content.WriteString(fmt.Sprintf("<!-- Project: %s -->\n", projectID))
        content.WriteString(fmt.Sprintf("<!-- Sequence: %d -->\n", task.Sequence))
        content.WriteString(fmt.Sprintf("<!-- Status: %s -->\n", task.Status))
        content.WriteString("<!-- Edit below, then import with: ralph task import %s %d <file> -->\n\n", projectID, sequence)
    }

    content.WriteString(task.Description)

    // Output
    if outputFile == "" {
        fmt.Print(content.String())
        return nil
    }

    return os.WriteFile(outputFile, []byte(content.String()), 0644)
}
```

### Import Command

```go
// cmd/ralph/task_import.go

func taskImportCmd() *cobra.Command {
    var force bool
    var stripMetadata bool

    cmd := &cobra.Command{
        Use:   "import <project-id> <task-sequence> <file>",
        Short: "Import task description from a file",
        Long: `Import task description content from a file or stdin.

The task must exist and not be completed. Metadata comment lines are
automatically stripped if present.

Examples:
  ralph task import abc123 3 task.md          # Import from file
  ralph task import abc123 3 -                # Import from stdin
  cat task.md | ralph task import abc123 3 -  # Pipe from another command`,
        Args: cobra.ExactArgs(3),
        RunE: func(cmd *cobra.Command, args []string) error {
            projectID := args[0]
            sequence, err := strconv.Atoi(args[1])
            if err != nil {
                return fmt.Errorf("invalid task sequence: %s", args[1])
            }
            inputFile := args[2]

            return runTaskImport(projectID, sequence, inputFile, force, stripMetadata)
        },
    }

    cmd.Flags().BoolVarP(&force, "force", "f", false, "Skip confirmation prompt")
    cmd.Flags().BoolVar(&stripMetadata, "strip-metadata", true, "Strip metadata comment lines")

    return cmd
}

func runTaskImport(projectID string, sequence int, inputFile string, force, stripMetadata bool) error {
    cfg, err := config.Load()
    if err != nil {
        return err
    }

    // Read input
    var content []byte
    if inputFile == "-" {
        content, err = io.ReadAll(os.Stdin)
    } else {
        content, err = os.ReadFile(inputFile)
    }
    if err != nil {
        return fmt.Errorf("failed to read input: %w", err)
    }

    // Strip metadata comments if present
    description := string(content)
    if stripMetadata {
        description = stripMetadataComments(description)
    }

    // Open project database
    database, err := db.OpenProjectDB(cfg.GetProjectsDir(), projectID)
    if err != nil {
        return fmt.Errorf("failed to open project %s: %w", projectID, err)
    }
    defer database.Close()

    // Get task
    task, err := database.GetTaskBySequence(projectID, sequence)
    if err != nil {
        return fmt.Errorf("task %d not found in project %s: %w", sequence, projectID, err)
    }

    // Validate task can be modified
    if task.Status == db.TaskCompleted {
        return fmt.Errorf("cannot modify completed task %d", sequence)
    }

    // Confirmation
    if !force {
        fmt.Printf("Update task %d (%s) in project %s?\n", sequence, task.Title, projectID)
        fmt.Printf("Current description length: %d chars\n", len(task.Description))
        fmt.Printf("New description length: %d chars\n", len(description))
        fmt.Print("Proceed? [y/N]: ")

        var response string
        fmt.Scanln(&response)
        if response != "y" && response != "Y" {
            return fmt.Errorf("import cancelled")
        }
    }

    // Update task
    if err := database.UpdateTaskDescription(task.ID, description); err != nil {
        return fmt.Errorf("failed to update task: %w", err)
    }

    fmt.Printf("Task %d description updated (%d chars)\n", sequence, len(description))
    return nil
}

// stripMetadataComments removes <!-- ... --> comment lines from the start
func stripMetadataComments(content string) string {
    lines := strings.Split(content, "\n")

    // Find first non-comment, non-empty line
    startIdx := 0
    for i, line := range lines {
        trimmed := strings.TrimSpace(line)
        if trimmed == "" {
            continue
        }
        if strings.HasPrefix(trimmed, "<!--") && strings.HasSuffix(trimmed, "-->") {
            startIdx = i + 1
            continue
        }
        break
    }

    // Skip leading empty lines after comments
    for startIdx < len(lines) && strings.TrimSpace(lines[startIdx]) == "" {
        startIdx++
    }

    if startIdx >= len(lines) {
        return ""
    }

    return strings.Join(lines[startIdx:], "\n")
}
```

### Database Methods

```go
// internal/db/db.go

// GetTaskBySequence retrieves a task by its sequence number within a project
func (d *DB) GetTaskBySequence(projectID string, sequence int) (*Task, error) {
    var task Task
    err := d.db.QueryRow(`
        SELECT id, project_id, sequence, title, description, status,
               jj_change_id, iteration_count, created_at, updated_at
        FROM tasks
        WHERE project_id = ? AND sequence = ?
    `, projectID, sequence).Scan(
        &task.ID, &task.ProjectID, &task.Sequence, &task.Title,
        &task.Description, &task.Status, &task.JJChangeID,
        &task.IterationCount, &task.CreatedAt, &task.UpdatedAt,
    )
    if err != nil {
        return nil, err
    }
    return &task, nil
}

// UpdateTaskDescription updates only the description field of a task
func (d *DB) UpdateTaskDescription(taskID string, description string) error {
    _, err := d.db.Exec(`
        UPDATE tasks SET description = ?, updated_at = ? WHERE id = ?
    `, description, time.Now(), taskID)
    return err
}
```

### List Tasks Command (Bonus)

```go
// cmd/ralph/task_list.go

func taskListCmd() *cobra.Command {
    return &cobra.Command{
        Use:   "list <project-id>",
        Short: "List all tasks in a project",
        Args:  cobra.ExactArgs(1),
        RunE: func(cmd *cobra.Command, args []string) error {
            return runTaskList(args[0])
        },
    }
}

func runTaskList(projectID string) error {
    cfg, err := config.Load()
    if err != nil {
        return err
    }

    database, err := db.OpenProjectDB(cfg.GetProjectsDir(), projectID)
    if err != nil {
        return err
    }
    defer database.Close()

    tasks, err := database.GetTasksByProject(projectID)
    if err != nil {
        return err
    }

    fmt.Printf("Tasks in project %s:\n\n", projectID)
    for _, task := range tasks {
        status := statusIcon(task.Status)
        fmt.Printf("  %s %d. %s\n", status, task.Sequence, task.Title)
    }

    return nil
}
```

## Files to Create/Modify

- `cmd/ralph/task.go` - Create: task subcommand group
- `cmd/ralph/task_export.go` - Create: export command
- `cmd/ralph/task_import.go` - Create: import command
- `cmd/ralph/task_list.go` - Create: list command (optional)
- `internal/db/db.go` - Add: `GetTaskBySequence`, `UpdateTaskDescription`

## Testing Strategy

1. **Unit tests** - Metadata stripping, content processing
2. **Integration tests** - Export then import roundtrip
3. **Edge cases** - Empty content, completed task, invalid IDs
4. **CLI tests** - Flag parsing, stdin handling

## Dependencies

- Depends on: addendum-01-per-project-database (for `OpenProjectDB`)
- Or can work with current single-database approach (just use existing `db.New`)

## Example Workflow

```bash
# List tasks to find the one to edit
$ ralph task list abc123
Tasks in project abc123:

  ● 1. Setup project structure
  ◐ 2. Implement auth module      # Currently in progress
  ○ 3. Add user endpoints
  ○ 4. Write tests

# Export task 3 for editing
$ ralph task export abc123 3 -o task3.md

# Edit in your favorite editor
$ vim task3.md

# Import the changes
$ ralph task import abc123 3 task3.md
Update task 3 (Add user endpoints) in project abc123?
Current description length: 450 chars
New description length: 892 chars
Proceed? [y/N]: y
Task 3 description updated (892 chars)

# Or do it all in one flow with pipe
$ ralph task export abc123 3 | vim - | ralph task import abc123 3 -
```

## Notes

- The metadata header makes it easy to identify which task the file belongs to
- Stripping metadata on import means you can safely export->edit->import without accumulating headers
- Consider adding `ralph task show <project> <seq>` as an alias for export to stdout
- Could extend to support exporting/importing multiple tasks at once
- JSON export format could be an option for programmatic use
