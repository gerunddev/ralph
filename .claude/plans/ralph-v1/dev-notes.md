# Development Notes - Task 2: Database Layer

## Implementation Decisions

### 1. Type-Safe Status Constants
Instead of using raw strings for status fields, I created typed constants (e.g., `ProjectStatus`, `TaskStatus`). This provides:
- Compile-time checking
- IDE autocomplete
- Self-documenting code

### 2. Nullable Fields
Used pointers for nullable database fields:
- `JJChangeID *string` - tasks may not have a jj change yet
- `CompletedAt *time.Time` - sessions may not be complete
- `Content *string` - approved feedback may have no content

### 3. Error Handling
Created a custom `ErrNotFound` sentinel error instead of exposing `sql.ErrNoRows`. This:
- Provides a clean API
- Allows callers to use `errors.Is(err, db.ErrNotFound)`
- Keeps SQL implementation details internal

### 4. Auto-Migration
The `New()` function automatically runs migrations. This simplifies the API - callers don't need to remember to call `Migrate()` separately.

### 5. Foreign Key Enforcement
Enabled via connection string parameter `?_foreign_keys=on`. This is SQLite-specific - foreign keys are off by default in SQLite.

## Areas for Review Attention

1. **Timestamp handling**: The code uses `time.Now()` directly. In a more testable design, this could be injected. Current approach is pragmatic for v1.

2. **Transaction handling in CreateTasks**: Uses `defer tx.Rollback()` pattern. The rollback is a no-op if commit succeeds.

3. **Connection string for in-memory DB**: The code appends `?_foreign_keys=on` which works for both file paths and `:memory:`, but reviewer should verify this is correct.

## Known Limitations

1. No versioned migrations - schema is created fresh each time. Fine for v1, but would need migration versioning for production use.

2. No connection pooling configuration - uses default `sql.DB` settings.

3. `GetLatestFeedbackForTask` does a JOIN - might want an index on `sessions.task_id` (already have `idx_sessions_task`).

## Test Coverage

The tests cover:
- All CRUD operations
- Foreign key constraints
- Ordering guarantees
- ErrNotFound cases
- Nullable field handling
- Status type constants

Tests use `:memory:` SQLite for speed.

---

# Development Notes - Task 3: Configuration System

## Implementation Decisions

### 1. Pointer-Based Merge Strategy
Used a parallel `fileConfig` struct with pointer fields to distinguish between:
- Field explicitly set to zero value (e.g., `"max_turns": 0`)
- Field not present in JSON at all

This allows partial config files to merge properly with defaults - only explicitly set values override.

### 2. Path Expansion Idempotency
The `expandedPaths` boolean flag ensures `ExpandPaths()` is idempotent. Calling it multiple times won't break paths (e.g., expanding `~/` twice would be wrong).

### 3. Agent Prompt Loading
The `GetAgentPrompt()` function returns embedded defaults from the `agents` package when no custom path is set. This creates a dependency on the agents package, but keeps the config as the single source of truth for "which prompt should I use?".

### 4. Validation Order
Validation happens AFTER path expansion. This ensures that when we check if an agent file exists, we're checking the expanded path (e.g., `/Users/foo/agents/developer.md` not `~/agents/developer.md`).

### 5. Error Aggregation
Used `errors.Join()` (Go 1.20+) to collect all validation errors and return them together. This gives users a complete picture of what's wrong rather than fixing one error at a time.

## Files Created

1. `internal/config/config.go` - Full implementation with:
   - `Load()` - load from standard location
   - `LoadFromPath()` - load from specific path
   - `Validate()` - validate all config values
   - `ExpandPaths()` - expand `~` in paths
   - `GetDatabasePath()` - return expanded db path
   - `GetAgentPrompt()` - return prompt for agent type

2. `internal/config/config_test.go` - 33 tests covering:
   - Default config values
   - Loading from missing file
   - Loading from valid file
   - Partial config merging
   - Invalid JSON handling
   - All validation rules
   - Path expansion
   - Agent prompt loading (default and custom)

3. `internal/agents/planner.go` - Added planner agent with default prompt (was missing from codebase)

## Areas for Review Attention

1. **Circular dependency prevention**: The config package imports agents for default prompts. This is a one-way dependency. The agents package should NOT import config.

2. **Home directory errors**: If `os.UserHomeDir()` fails (rare edge case), path expansion errors out. This is intentional - running without a home directory is unusual.

3. **Relative path handling**: Relative paths are cleaned but NOT made absolute. This matches the spec but might be surprising if cwd changes.

## Test Coverage

All acceptance criteria are covered:
- [x] Config loads from default location
- [x] Missing config file uses all defaults
- [x] Partial config file merges with defaults
- [x] Path expansion works for `~`
- [x] Validation catches invalid values
- [x] Custom agent prompt paths work
- [x] Unit tests cover all cases (33 tests)

---

# Development Notes - Task 4: JJ Client Implementation

## Implementation Decisions

### 1. Mockable Command Runner
Used a `CommandRunner` function type that can be replaced in tests via `SetCommandRunner()`. This allows comprehensive unit testing without requiring jj to be installed. The mock records all calls and returns configurable responses.

### 2. Error Handling Strategy
Created sentinel errors that are detected through:
- `ErrCommandNotFound` - via `exec.Error` type assertion checking for `exec.ErrNotFound`
- `ErrNotRepo` - via stderr message parsing (case-insensitive)
- Context errors (`context.Canceled`, `context.DeadlineExceeded`) - passed through directly

The `wrapError()` function handles all error type detection and wrapping.

### 3. Change ID Retrieval
After `jj new`, we call `jj log -r @ -T change_id --no-graph` to get the change ID. This is more reliable than parsing `jj new` output, which varies between jj versions.

### 4. Extra Methods
Added `Status()`, `Diff()`, and `Log()` methods beyond the minimum requirements. These are likely to be useful for the review workflow and they follow the same pattern.

## Files Modified

1. `internal/jj/jj.go` - Full implementation with:
   - `NewClient(workDir)` - creates client bound to working directory
   - `NewChange(ctx, description)` - creates new change, returns change ID
   - `Show(ctx)` - returns diff output
   - `Describe(ctx, description)` - updates change description
   - `CurrentChangeID(ctx)` - returns current change ID
   - `Status(ctx)` - returns working copy status
   - `Diff(ctx)` - returns diff of current change
   - `Log(ctx, revset, template)` - returns log output

2. `internal/jj/jj_test.go` - 23 tests covering:
   - Unit tests with mock command runner
   - Integration tests (skip if jj not installed)
   - Error handling (command not found, not a repo, context cancellation)

## Areas for Review Attention

1. **Stderr parsing for ErrNotRepo**: The exact error messages may vary by jj version. Tested with jj 0.37.0. The code checks for multiple possible messages case-insensitively.

2. **Integration test setup**: Tests initialize both git and jj (`jj git init --colocate`) since jj requires a backend. Also configures git user name/email to avoid errors.

3. **Context handling**: The command runner uses `exec.CommandContext` which should kill the process on context cancellation, but behavior may vary by OS.

## Test Coverage

All acceptance criteria are covered:
- [x] `NewClient(workDir)` creates client bound to a working directory
- [x] `NewChange(ctx, description)` runs `jj new -m "description"` and returns change ID
- [x] `Show(ctx)` runs `jj show` and returns the diff output
- [x] `Describe(ctx, description)` runs `jj describe -m "description"`
- [x] `CurrentChangeID(ctx)` returns the current change ID
- [x] All methods handle errors appropriately
- [x] Context cancellation stops running commands
- [x] Unit tests with mock exec
- [x] Integration test that creates a temp repo (skipped if jj not installed)

---

# Development Notes - Task 9: Engine Orchestration

## Implementation Decisions

### 1. Engine Owns All Dependencies
The Engine struct owns instances of all major components (Claude client, JJ client, Agents manager) rather than accepting them as parameters. This simplifies the API for callers - they just need to provide configuration and the engine handles component instantiation.

### 2. Thread-Safe Project Access
The project field is protected by a mutex (`sync.RWMutex`) because:
- `Project()` getter may be called from the TUI while the engine is running
- `CreateProject` and `ResumeProject` set the project from different contexts
- Events are emitted asynchronously which needs to check the stopped flag

### 3. Event Channel Buffering
Used a buffer size of 100 for the engine events channel. This is smaller than the task loop's 250 because:
- Engine events are less frequent (project-level vs task-level)
- The engine just wraps task loop events, so the task loop buffer handles most backpressure

### 4. Graceful Stop with Double-Stop Safety
`Stop()` is idempotent - calling it multiple times is safe. The stopped flag prevents:
- Double-closing the events channel (which would panic)
- Emitting events after stop

### 5. Planner Output Parsing
The `parsePlannerOutput` function finds JSON array brackets in the output. This handles cases where the planner includes explanatory text around the JSON. It also validates that the returned array is non-empty.

### 6. Exposed Accessors for Testing
Added `DB()`, `Claude()`, `JJ()`, and `Agents()` methods so tests can verify the engine was constructed correctly and potentially mock components.

## Files Modified

1. `internal/engine/engine.go` - Full implementation with:
   - `NewEngine(cfg)` - creates engine with configuration
   - `CreateProject(ctx, planPath)` - reads plan file, creates project, runs planner
   - `ResumeProject(ctx, projectID)` - resumes existing project
   - `Run(ctx)` - executes full workflow with task loop
   - `Stop()` - graceful shutdown
   - `Events()` - returns event channel
   - `Project()` - returns currently loaded project

2. `internal/engine/engine_test.go` - Created with tests covering:
   - Engine construction and validation
   - Event emission
   - Project loading and resumption
   - Planner output parsing
   - Stop behavior and idempotency
   - Concurrent access safety

## Areas for Review Attention

1. **Planner JSON Parsing**: The parser finds the first `[` and last `]` in output. This could fail if the planner's explanatory text contains brackets. Consider more robust JSON extraction if this becomes a problem.

2. **Event Forwarding Goroutine**: In `Run()`, a goroutine forwards task loop events. It continues until the task loop's event channel closes, which happens when `TaskLoop.Run()` completes.

3. **Context Propagation**: The context is passed through to `TaskLoop.Run()` but events emitted after stop are silently dropped. This is intentional to avoid panic on closed channel.

4. **CreateProject Partial Failure**: If planning tasks fails after the project is created in the database, the project remains in the database with status "pending" but no tasks. This is logged but not cleaned up - consider adding cleanup or status update in a future iteration.

## Test Coverage

All acceptance criteria are covered:
- [x] `NewEngine(config)` creates engine with configuration
- [x] `CreateProject(planPath)` reads plan file, creates project, runs planner
- [x] `ResumeProject(projectID)` resumes an existing project
- [x] `Run(ctx, project)` executes the full workflow
- [x] Planner agent parses plan into tasks
- [x] Task loop runs all tasks
- [x] Engine emits events for TUI updates
- [x] Graceful shutdown on context cancellation
- [x] Error handling at each stage

Note: Full integration testing of `CreateProject` and `Run` requires the Claude CLI, which is mocked at a lower level. The tests focus on structure and error handling rather than end-to-end behavior.

---

# Development Notes - Task 14: Learnings Capture

## Implementation Summary

Implemented the learnings capture phase that updates project documentation (AGENTS.md and README.md) based on what was built during the development session.

## Key Design Decision: Learnings State Tracking

Added a `learnings_state` field to the Project model, similar to `user_feedback_state`. This enables:
- Resuming projects that have all tasks done but haven't captured learnings yet
- Tracking overall project completion state (tasks complete + feedback given + learnings captured)
- Idempotent learnings capture (won't re-capture if already done)

## Implementation Decisions

### 1. Append-Only Documentation
Content is appended to AGENTS.md and README.md with session separators (date-stamped), rather than overwritten. This preserves history and allows multiple development sessions to contribute learnings.

### 2. Graceful Error Handling
If learnings capture fails:
- The error is logged
- The project still marks as complete
- The TUI transitions to the completed state
This ensures a failed documentation step doesn't block project completion.

### 3. Two-Phase State Check
After feedback is marked complete, the system checks learnings state:
- If already `complete`, skip directly to project completion
- If not captured, run the documenter agent

### 4. Documenter Agent Design
The documenter agent receives:
- Combined changes summary from `jj log`
- List of completed tasks with titles and descriptions

It outputs two markdown code blocks that are parsed and appended to the respective files.

## Files Modified/Created

### Database (`internal/db/`)
- **models.go**: Added `LearningsState` type with constants, added field to Project
- **migrations.go**: Added schema column and migration
- **db.go**: Updated CRUD operations, added `UpdateProjectLearningsState`
- **db_test.go**: Added tests for learnings state

### Agents (`internal/agents/`)
- **documenter.go**: New file - documenter agent definition and prompt
- **documenter_test.go**: New file - tests for documenter agent
- **agents.go**: Added `GetDocumenterAgent` method to Manager

### Engine (`internal/engine/`)
- **engine.go**: Added learnings capture methods and event types
- **engine_test.go**: Added tests for learnings parsing and filtering

### TUI (`internal/tui/`)
- **learnings.go**: New file - LearningsModel for capture progress view
- **learnings_test.go**: New file - tests for LearningsModel
- **app.go**: Added learnings flow to both CreateModeModel and Model

### App (`internal/app/`)
- **app.go**: Updated `runCreateMode` to pass engine to TUI

## Areas for Review Attention

1. **extractCodeBlock function**: The parsing logic for extracting markdown code blocks from Claude's output may need refinement based on actual documenter output formats.

2. **jj Log revset**: Uses `..@` to get recent history - verify this works correctly across different jj repository states.

3. **File creation**: `appendToFile` creates files if they don't exist. Verify this is desired behavior for projects without existing AGENTS.md/README.md.

4. **Error messages in TUI**: Error states show in the learnings view but auto-advance to completion. May want to give user more control here.

5. **Context propagation**: `CaptureLearnings` uses a fresh `context.Background()` from TUI commands. Consider if this should be more configurable.

## Test Coverage

Added tests for:
- Database: `LearningsState` constants, `UpdateProjectLearningsState`, learnings state in `ListProjects`
- Engine: `parseLearningsOutput`, `extractCodeBlock`, `filterCompleted`, event types, `CaptureLearnings` error cases
- Agents: `Documenter` agent creation, template building, `GetDocumenterAgent`
- TUI: `LearningsModel` states, views, updates, and lifecycle methods

---

# Development Notes - Task 15: Restart from Interim State

## Implementation Summary

Implemented the restart/resume capability for Ralph projects that were interrupted mid-execution.

## Files Created

- `internal/engine/resume.go` - Project state detection, cleanup, and reset methods
- `internal/engine/resume_test.go` - Comprehensive tests for resume functionality
- `internal/tui/resume.go` - Resume dialog, failed tasks view, and completed project view models
- `internal/tui/resume_test.go` - Tests for resume UI components

## Files Modified

- `internal/tui/app.go` - Integrated resume flow into the main TUI model
- `internal/tui/app_test.go` - Updated and added tests for resume flow

## Key Design Decisions

### 1. State Detection via ProjectState Struct
The `ProjectState` struct captures the full state of a project including:
- Completed/pending/failed task counts
- In-progress task (if any)
- Last session info
- Whether cleanup is needed

Helper methods like `HasInterruptedWork()`, `IsComplete()`, `HasFailedTasks()` provide semantic access to the state.

### 2. Cleanup Flow
When resuming an interrupted project:
- In-progress task is reset to pending status
- Running sessions are marked as failed
- Project status is preserved (task loop will update as needed)

This ensures a clean restart without losing the audit trail.

### 3. User Confirmation via Resume Dialog
Users see a resume dialog showing:
- Project name and current status
- Task progress (completed/pending/failed counts)
- Interrupted task info (if any)
- Options: resume, reset all, view details, quit

### 4. Completed Project Handling
When selecting a fully completed project:
- Shows a completion summary
- Allows user to reset and run again
- Clean way to re-run a project from scratch

### 5. View State Changes
Added two new view states to handle the resume flow:
- `ViewResumeDialog` - Shows resume confirmation
- `ViewCompletedProject` - Shows completed project with reset option

### 6. Project Selection Flow Change
The flow now goes:
1. User selects project
2. Engine is created
3. `checkProjectState()` determines next action
4. Based on state: show resume dialog, show completed view, or start directly

## Implementation Details

### Engine Methods (resume.go)
- `DetectProjectState(ctx, projectID)` - Analyzes project state
- `CleanupForResume(ctx, state)` - Resets interrupted state
- `ResetProject(ctx, projectID)` - Full project reset
- `RetryFailedTasks(ctx, projectID)` - Reset only failed tasks
- `IsProjectResumable(ctx, projectID)` - Quick resumability check

### TUI Models (resume.go)
- `ResumeModel` - Resume confirmation dialog
- `FailedTasksModel` - Display/handle failed tasks (for future use)
- `CompletedProjectModel` - Completed project view

### New Message Types
- `ShowResumeDialogMsg` - Trigger resume dialog
- `ResumeConfirmedMsg` - User confirmed resume
- `ResetConfirmedMsg` - User confirmed reset
- `ProjectCompletedMsg` - Project is completed
- `StartProjectMsg` - Start project execution

## Areas for Review Attention

1. **IsProjectResumable**: The `RetryFailedTasks` method is implemented but not fully integrated into the TUI flow. The FailedTasksModel is available for future enhancement.

2. **Iteration Count Reset**: On resume, tasks restart from scratch (iteration 0). An alternative design could preserve the iteration count and continue from where it left off.

3. **JJ State**: The code preserves `jj_change_id` on tasks during reset. The actual jj repo state is separate - cleanup of jj state might be needed for full reset.

4. **Test Coverage**: Added tests for all new functionality. Updated existing tests that relied on immediate state transitions.

## Test Coverage

Added comprehensive tests for:
- State detection (all task status combinations)
- Cleanup flow (in-progress tasks, running sessions)
- Reset project (all tasks, project status, feedback/learnings state)
- Retry failed tasks
- ProjectState helper methods
- TUI resume dialog (all key handlers, view rendering)
- TUI completed project view
- Integration with main Model flow

---

# Development Notes - Addendum 03: Task Loop Pause Mode

## Implementation Summary

Added pause mode functionality to the task loop, allowing users to manually approve progression between tasks. The feature includes:

1. Default behavior: continuous mode (no pausing)
2. Pause mode: after each task completes, waits for user confirmation
3. TUI hotkey `p` toggles pause mode on/off
4. Visual indicator shows current mode
5. Config option `default_pause_mode` to set default

## Files Modified

### Config (`internal/config/config.go`)
- Added `DefaultPauseMode bool` field to Config struct
- Added to fileConfig for JSON parsing
- Added to mergeConfig function
- Default value: false (continuous mode)

### Engine - Task Loop (`internal/engine/task_loop.go`)
- Added fields to TaskLoop struct:
  - `pauseMode bool` - Whether pause mode is enabled
  - `pauseCh chan struct{}` - Channel to signal continuation after pause
  - `pauseModeMu sync.RWMutex` - Mutex for thread-safe pause mode access
- Added new event types:
  - `TaskEventPaused` - Emitted when loop pauses
  - `TaskEventResumed` - Emitted when loop continues after pause
  - `TaskEventPauseModeChanged` - Emitted when pause mode toggled
- Added methods:
  - `SetPauseMode(enabled bool)` - Toggle pause mode
  - `IsPauseMode() bool` - Check current pause mode state
  - `Continue()` - Signal loop to proceed after pause
  - `waitForContinue(ctx, task)` - Internal helper to block until Continue() or context cancellation
- Modified `Run()` to pause after task completion when pause mode enabled
- Modified `NewTaskLoop()` to initialize pauseCh and apply config default

### Engine (`internal/engine/engine.go`)
- Added `taskLoop *TaskLoop` field to Engine struct
- Added `TaskLoop() *TaskLoop` getter method for TUI access
- Modified `Run()` to store/clear task loop reference

### TUI Progress Model (`internal/tui/progress.go`)
- Added fields:
  - `pauseMode bool` - Local pause mode state
  - `isPaused bool` - Whether currently waiting at pause point
  - `engine *engine.Engine` - Reference to engine for task loop control
- Added `SetEngine(eng *engine.Engine)` method
- Updated `Update()` to handle:
  - `p` key to toggle pause mode
  - `enter`/space to continue from pause
- Updated `handleTaskLoopEvent()` to handle new event types
- Updated `renderFooter()` to show:
  - `[PAUSE]` indicator when pause mode enabled
  - Context-appropriate help text
- Added accessor methods: `IsPauseMode()`, `IsPaused()`

### TUI Styles (`internal/tui/styles.go`)
- Added `pauseStyle` - Yellow bold for pause indicator
- Added `pausedBorderStyle` - Yellow border (available for future use)

## Key Design Decisions

### 1. Thread-Safe Pause Mode Access
Used a separate `pauseModeMu` mutex for pause mode state, independent of the main `mu` mutex. This prevents potential deadlocks when SetPauseMode is called from TUI while the task loop is holding its own mutex.

### 2. Buffered Pause Channel
The `pauseCh` is a buffered channel with size 1. This allows:
- Continue() to be called even when not paused (no blocking)
- Multiple Continue() calls are collapsed into one signal
- The select with default in Continue() makes it non-blocking

### 3. Pause After Completion, Not During
The pause happens AFTER task completion, not during task execution. This ensures:
- Claude sessions are not interrupted mid-execution
- Clean task boundaries
- Users can review the completed work before proceeding

### 4. TUI-Engine Communication
The TUI needs a reference to the engine to access the TaskLoop for pause control. This is set via `SetEngine()` after the model is created. The TUI updates its local pause state from events, keeping the engine as source of truth.

### 5. Pause on Both Success and Failure
When pause mode is enabled, the loop pauses after both successful and failed task completions (when continuing to next task). This allows users to review failures before deciding to proceed.

## Test Coverage

### Config Tests
- `TestDefaultConfig` - Updated to check default_pause_mode = false
- `TestLoadFromPath_DefaultPauseMode` - Loading pause mode from config
- `TestLoadFromPath_DefaultPauseModeNotSet` - Default when not specified

### Task Loop Tests
- `TestTaskLoop_PauseModeDefaults` - Default pause mode is disabled
- `TestTaskLoop_PauseModeFromConfig` - Pause mode from config
- `TestTaskLoop_SetPauseMode` - Toggle pause mode on/off
- `TestTaskLoop_SetPauseModeEmitsEvent` - Event emission on toggle
- `TestTaskLoop_ContinueWhenNotPaused` - Continue() is no-op when not paused
- `TestTaskLoop_ContinueSignalsPauseCh` - Continue() signals the channel
- `TestTaskLoop_PauseModeEventTypes` - Event types are distinct
- `TestTaskLoop_ConcurrentPauseModeAccess` - Race condition safety

### TUI Tests
- `TestTaskProgressModel_PauseModeInitialState` - Initial state is not paused
- `TestTaskProgressModel_PauseModeToggle` - Toggle with 'p' key
- `TestTaskProgressModel_HandlePausedEvent` - isPaused set on event
- `TestTaskProgressModel_HandleResumedEvent` - isPaused cleared on event
- `TestTaskProgressModel_HandlePauseModeChangedEvent` - pauseMode updated from event
- `TestTaskProgressModel_Footer_PauseMode` - PAUSE indicator shown
- `TestTaskProgressModel_Footer_WhenPaused` - Paused prompt shown
- `TestTaskProgressModel_PauseAccessors` - IsPauseMode(), IsPaused() methods

## Areas for Review Attention

1. **Engine Reference in TUI**: The TUI stores a reference to the engine, which it uses to access the TaskLoop. This creates a coupling between TUI and engine. Alternatively, we could pass the TaskLoop directly or use a callback pattern.

2. **Pause Mode Message Parsing**: The `handleTaskLoopEvent` parses "Pause mode: true/false" from the event message. This is fragile - consider using a more structured event type or adding a boolean field to TaskLoopEvent.

3. **Continue from TUI While Not Paused**: If user presses Enter when not paused, it's a no-op. Consider providing feedback that they're not in a paused state.

4. **Integration with Main App**: The SetEngine() call needs to be made from the main app flow. The implementation provides the method but the caller needs to ensure it's called at the right time.
