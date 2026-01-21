# Task 13: User Review Feedback Loop

## Context

After all tasks complete, Ralph should prompt the user for final review feedback. The feedback mechanism uses a CLI command to submit feedback files, with the TUI tracking feedback state at the project level.

## Objective

Implement project-level feedback state tracking, a CLI command for submitting feedback, and TUI views for prompting and instructing users.

## Acceptance Criteria

- [ ] Add `user_feedback_state` column to projects table (states: complete, pending, provided)
- [ ] After all tasks complete, TUI shows feedback prompt asking if user wants to mark review as complete
- [ ] "Yes" (complete) sets state to COMPLETE and marks project done
- [ ] "No" (provide feedback) sets state to PENDING and shows CLI instructions
- [ ] CLI command `ralph feedback -p PROJECT_ID -f /path/to/feedback.md` creates feedback task and sets state to PROVIDED
- [ ] When TUI reopens with PROVIDED state, runs feedback task then resets to prompt again
- [ ] Feedback tasks are normal tasks with sequence N+1

## Implementation Details

### 1. Database Layer

#### New Type (`internal/db/models.go`)

Add after existing status types:

```go
// UserFeedbackState represents the state of user feedback for a project.
type UserFeedbackState string

const (
    FeedbackStateNone     UserFeedbackState = ""        // Initial state, not yet prompted
    FeedbackStatePending  UserFeedbackState = "pending"  // User wants to provide feedback via CLI
    FeedbackStateProvided UserFeedbackState = "provided" // Feedback submitted, task created
    FeedbackStateComplete UserFeedbackState = "complete" // User marked review as complete
)
```

#### Update Project Struct (`internal/db/models.go`)

```go
type Project struct {
    ID                string
    Name              string
    PlanText          string
    Status            ProjectStatus
    UserFeedbackState UserFeedbackState  // NEW FIELD
    CreatedAt         time.Time
    UpdatedAt         time.Time
}
```

#### Migration (`internal/db/migrations.go`)

Add column to projects table schema and handle ALTER TABLE for existing databases:

```sql
ALTER TABLE projects ADD COLUMN user_feedback_state TEXT NOT NULL DEFAULT '';
```

#### New DB Methods (`internal/db/db.go`)

```go
// UpdateProjectFeedbackState updates a project's user feedback state.
func (d *DB) UpdateProjectFeedbackState(id string, state UserFeedbackState) error

// GetMaxTaskSequence returns the highest sequence number for a project's tasks.
func (d *DB) GetMaxTaskSequence(projectID string) (int, error)

// HasPendingTasks returns true if there are any pending tasks for the project.
func (d *DB) HasPendingTasks(projectID string) (bool, error)
```

Also update `CreateProject`, `GetProject`, `ListProjects` to include `user_feedback_state` field.

### 2. CLI Command (`cmd/ralph/main.go`)

Add feedback subcommand:

```go
var projectID, feedbackFile string
feedbackCmd := &cobra.Command{
    Use:   "feedback",
    Short: "Submit feedback for a project",
    Long: `Submit user feedback for a project. The feedback will be processed
as a new task through the standard Ralph development cycle.`,
    RunE: func(cmd *cobra.Command, args []string) error {
        return app.SubmitFeedback(projectID, feedbackFile)
    },
}
feedbackCmd.Flags().StringVarP(&projectID, "project", "p", "", "Project ID (required)")
feedbackCmd.Flags().StringVarP(&feedbackFile, "file", "f", "", "Path to feedback markdown file (required)")
feedbackCmd.MarkFlagRequired("project")
feedbackCmd.MarkFlagRequired("file")

rootCmd.AddCommand(feedbackCmd)
```

### 3. Feedback Submission (`internal/app/app.go`)

```go
func SubmitFeedback(projectID, feedbackFile string) error {
    // 1. Load config and open database
    // 2. Verify project exists and state is PENDING
    // 3. Read feedback file content
    // 4. Get max task sequence, create task with sequence = max + 1
    // 5. Update project feedback state to PROVIDED
    // 6. Print success message with TUI restart instructions
}
```

### 4. TUI Views (`internal/tui/feedback.go`)

#### FeedbackPromptModel

Shows after all tasks complete when feedback state is None:
- Question: "Mark review as complete?"
- Two options: "Yes" / "No, I want to provide feedback"
- Arrow keys to select, Enter to confirm
- Sends `FeedbackChoiceMsg{Complete: bool}`

#### FeedbackInstructionsModel

Shows after user selects "No, I want to provide feedback":
- Displays CLI command: `ralph feedback -p PROJECT_ID -f /path/to/feedback.md`
- Instructions to close TUI, create feedback file, run CLI, restart TUI
- Press q or Enter to exit

### 5. TUI State Machine (`internal/tui/app.go`)

```go
type appState int

const (
    stateTaskList appState = iota
    stateTaskRunning
    stateFeedbackPrompt
    stateFeedbackInstructions
    stateCompleted
)
```

State transitions:
1. All tasks complete → check `UserFeedbackState`
2. If None → show `FeedbackPromptModel`
3. If user chooses "Yes" (complete) → set COMPLETE → mark project completed
4. If user chooses "No" (provide feedback) → set PENDING → show `FeedbackInstructionsModel` → exit
5. If PROVIDED (TUI reopened after CLI) → run feedback task → reset to None → show prompt again

## Files to Modify

| File | Change |
|------|--------|
| `internal/db/models.go` | Add `UserFeedbackState` type, update `Project` struct |
| `internal/db/migrations.go` | Add `user_feedback_state` column |
| `internal/db/db.go` | Update project methods, add new feedback methods |
| `cmd/ralph/main.go` | Add `feedback` subcommand |
| `internal/app/app.go` | Add `SubmitFeedback()` function |
| `internal/tui/feedback.go` | Create with `FeedbackPromptModel`, `FeedbackInstructionsModel` |
| `internal/tui/app.go` | Add feedback flow to state machine |

## Testing Strategy

1. **DB tests** - New methods for feedback state and max sequence
2. **CLI tests** - Feedback command with valid/invalid inputs
3. **TUI tests** - State transitions for feedback flow
4. **Integration** - Full cycle: tasks complete → prompt → CLI → TUI reopens → feedback task → prompt again

## Dependencies

- `github.com/spf13/cobra` - Already used for CLI

## Notes

- Feedback state is project-level, not task-level
- Feedback tasks are normal tasks with sequence N+1
- Users can provide multiple rounds of feedback (loop until they mark review complete)
- Empty feedback file should be rejected
- CLI validates project exists and is in PENDING state before accepting feedback
- Marking review "complete" without providing feedback is valid (user is satisfied)
