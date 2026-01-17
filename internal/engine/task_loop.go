// Package engine provides the main execution engine for Ralph.
package engine

import (
	"context"
	"fmt"
	"sync"

	"github.com/gerund/ralph/internal/agents"
	"github.com/gerund/ralph/internal/claude"
	"github.com/gerund/ralph/internal/config"
	"github.com/gerund/ralph/internal/db"
	"github.com/gerund/ralph/internal/jj"
	"github.com/gerund/ralph/internal/log"
)

// eventChannelBufferSize is the buffer size for the task loop event channel.
// A larger buffer helps prevent dropped events when the TUI is slow to consume.
const eventChannelBufferSize = 250

// TaskLoopDeps holds the dependencies for the task loop.
type TaskLoopDeps struct {
	DB     *db.DB
	Claude *claude.Client
	JJ     *jj.Client
	Agents *agents.Manager
	Config *config.Config
}

// TaskLoop handles the iteration through all tasks in a project.
type TaskLoop struct {
	deps    TaskLoopDeps
	project *db.Project
	tasks   []*db.Task
	current int
	events  chan TaskLoopEvent
	mu      sync.RWMutex

	// Pause mode support
	pauseMode   bool
	pauseCh     chan struct{} // Signals to continue after pause
	pauseModeMu sync.RWMutex
}

// TaskLoopEvent represents an event during task loop execution.
type TaskLoopEvent struct {
	Type             TaskEventType
	TaskIndex        int
	TaskTitle        string
	Message          string
	ImplEvent        *ImplLoopEvent // Nested events from implementation loop
	PauseModeEnabled *bool          // For TaskEventPauseModeChanged events
}

// TaskEventType represents the type of task loop event.
type TaskEventType string

const (
	// TaskEventStarted is emitted when the task loop begins.
	TaskEventStarted TaskEventType = "started"
	// TaskEventTaskBegin is emitted when a task starts processing.
	TaskEventTaskBegin TaskEventType = "task_begin"
	// TaskEventTaskEnd is emitted when a task completes successfully.
	TaskEventTaskEnd TaskEventType = "task_end"
	// TaskEventCompleted is emitted when all tasks are processed.
	TaskEventCompleted TaskEventType = "completed"
	// TaskEventFailed is emitted when a task fails.
	TaskEventFailed TaskEventType = "failed"
	// TaskEventProgress is emitted for nested implementation loop events.
	TaskEventProgress TaskEventType = "progress"
	// TaskEventPaused is emitted when the loop pauses after task completion.
	TaskEventPaused TaskEventType = "paused"
	// TaskEventResumed is emitted when the loop continues after a pause.
	TaskEventResumed TaskEventType = "resumed"
	// TaskEventPauseModeChanged is emitted when pause mode is toggled.
	TaskEventPauseModeChanged TaskEventType = "pause_mode_changed"
)

// TaskLoopResult summarizes the outcome of the task loop.
type TaskLoopResult struct {
	Completed int
	Failed    int
	Skipped   int
}

// NewTaskLoop creates a new task loop for a project.
func NewTaskLoop(deps TaskLoopDeps, project *db.Project) *TaskLoop {
	tl := &TaskLoop{
		deps:    deps,
		project: project,
		events:  make(chan TaskLoopEvent, eventChannelBufferSize),
		pauseCh: make(chan struct{}, 1),
	}

	// Apply default pause mode from config
	if deps.Config != nil {
		tl.pauseMode = deps.Config.DefaultPauseMode
	}

	return tl
}

// Events returns a channel that emits task loop events.
// The channel is closed when the task loop completes.
func (tl *TaskLoop) Events() <-chan TaskLoopEvent {
	return tl.events
}

// CurrentTask returns the currently executing task.
func (tl *TaskLoop) CurrentTask() *db.Task {
	tl.mu.RLock()
	defer tl.mu.RUnlock()
	if tl.current < len(tl.tasks) {
		return tl.tasks[tl.current]
	}
	return nil
}

// Progress returns the current task index (1-indexed) and total task count.
// For example, when processing the first of 3 tasks, returns (1, 3).
func (tl *TaskLoop) Progress() (current, total int) {
	tl.mu.RLock()
	defer tl.mu.RUnlock()
	return tl.current + 1, len(tl.tasks)
}

// SetCurrentForTesting sets the current task index. This is only intended for
// testing concurrent access to Progress() and CurrentTask().
func (tl *TaskLoop) SetCurrentForTesting(index int) {
	tl.mu.Lock()
	defer tl.mu.Unlock()
	tl.current = index
}

// SetTasksForTesting sets the tasks slice. This is only intended for testing.
func (tl *TaskLoop) SetTasksForTesting(tasks []*db.Task) {
	tl.mu.Lock()
	defer tl.mu.Unlock()
	tl.tasks = tasks
}

// SetPauseMode enables or disables pause mode.
// When enabled, the loop will pause after each task completion
// and wait for Continue() to be called.
func (tl *TaskLoop) SetPauseMode(enabled bool) {
	tl.pauseModeMu.Lock()
	defer tl.pauseModeMu.Unlock()
	tl.pauseMode = enabled

	// Emit event so TUI can update display
	event := TaskLoopEvent{
		Type:             TaskEventPauseModeChanged,
		Message:          fmt.Sprintf("Pause mode: %v", enabled),
		PauseModeEnabled: &enabled,
	}
	select {
	case tl.events <- event:
	default:
		log.Warn("task loop event dropped: channel full",
			"event_type", TaskEventPauseModeChanged,
			"pause_mode_enabled", enabled)
	}
}

// IsPauseMode returns current pause mode state.
func (tl *TaskLoop) IsPauseMode() bool {
	tl.pauseModeMu.RLock()
	defer tl.pauseModeMu.RUnlock()
	return tl.pauseMode
}

// Continue signals the loop to proceed after a pause.
// If not currently paused, this is a no-op.
func (tl *TaskLoop) Continue() {
	select {
	case tl.pauseCh <- struct{}{}:
	default:
		// Not currently paused or already signaled, ignore
	}
}

// emit sends an event to the events channel.
func (tl *TaskLoop) emit(eventType TaskEventType, task *db.Task, message string) {
	var title string
	var index int
	if task != nil {
		title = task.Title
		tl.mu.RLock()
		index = tl.current
		tl.mu.RUnlock()
	}

	event := TaskLoopEvent{
		Type:      eventType,
		TaskIndex: index,
		TaskTitle: title,
		Message:   message,
	}

	select {
	case tl.events <- event:
	default:
		log.Warn("task loop event dropped: channel full",
			"event_type", eventType,
			"task_index", index,
			"task_title", title,
			"message", message)
	}
}

// emitWithImpl sends an event that wraps an implementation loop event.
func (tl *TaskLoop) emitWithImpl(task *db.Task, implEvent *ImplLoopEvent) {
	var title string
	if task != nil {
		title = task.Title
	}

	tl.mu.RLock()
	index := tl.current
	tl.mu.RUnlock()

	event := TaskLoopEvent{
		Type:      TaskEventProgress,
		TaskIndex: index,
		TaskTitle: title,
		ImplEvent: implEvent,
	}

	select {
	case tl.events <- event:
	default:
		log.Warn("task loop impl event dropped: channel full",
			"event_type", TaskEventProgress,
			"task_index", index,
			"task_title", title,
			"impl_event_type", implEvent.Type)
	}
}

// Run executes all pending tasks in sequence.
func (tl *TaskLoop) Run(ctx context.Context) (*TaskLoopResult, error) {
	defer close(tl.events)

	// Load tasks for the project
	tasks, err := tl.deps.DB.GetTasksByProject(tl.project.ID)
	if err != nil {
		return nil, fmt.Errorf("failed to load tasks: %w", err)
	}
	tl.tasks = tasks

	// If no tasks, we're done
	if len(tasks) == 0 {
		tl.emit(TaskEventCompleted, nil, "No tasks to process")
		return &TaskLoopResult{}, nil
	}

	// Update project status to in_progress
	if err := tl.deps.DB.UpdateProjectStatus(tl.project.ID, db.ProjectInProgress); err != nil {
		return nil, fmt.Errorf("failed to update project status: %w", err)
	}

	tl.emit(TaskEventStarted, nil, fmt.Sprintf("Processing %d tasks", len(tasks)))

	result := &TaskLoopResult{}

	for i, task := range tasks {
		// Check for context cancellation
		select {
		case <-ctx.Done():
			return result, ctx.Err()
		default:
		}

		tl.mu.Lock()
		tl.current = i
		tl.mu.Unlock()

		// Skip already completed tasks
		if task.Status == db.TaskCompleted {
			result.Completed++
			continue
		}

		// Skip already failed tasks
		if task.Status == db.TaskFailed {
			result.Failed++
			continue
		}

		// Skip escalated tasks
		if task.Status == db.TaskEscalated {
			result.Skipped++
			continue
		}

		tl.emit(TaskEventTaskBegin, task, "Starting task")

		if err := tl.processTask(ctx, task); err != nil {
			// Mark task as failed
			if dbErr := tl.deps.DB.UpdateTaskStatus(task.ID, db.TaskFailed); dbErr != nil {
				log.Warn("failed to update task status to failed", "task_id", task.ID, "error", dbErr)
			}
			result.Failed++
			tl.emit(TaskEventFailed, task, err.Error())

			// Check if we should continue to next task
			// MaxTaskAttempts > 0 means we try to continue despite failures
			if tl.deps.Config.MaxTaskAttempts > 0 {
				// Check if we should pause before next task (even on failure)
				if tl.IsPauseMode() && i < len(tasks)-1 {
					if err := tl.waitForContinue(ctx, task); err != nil {
						return result, err
					}
				}
				continue // Try next task
			}

			// Fail fast - update project status and return
			if dbErr := tl.deps.DB.UpdateProjectStatus(tl.project.ID, db.ProjectFailed); dbErr != nil {
				log.Warn("failed to update project status to failed", "project_id", tl.project.ID, "error", dbErr)
			}
			return result, err
		}

		result.Completed++
		tl.emit(TaskEventTaskEnd, task, "Task completed")

		// Check if we should pause before next task
		if tl.IsPauseMode() && i < len(tasks)-1 {
			if err := tl.waitForContinue(ctx, task); err != nil {
				return result, err
			}
		}
	}

	// Update project status based on results
	if result.Failed > 0 {
		if err := tl.deps.DB.UpdateProjectStatus(tl.project.ID, db.ProjectFailed); err != nil {
			log.Warn("failed to update project status to failed", "project_id", tl.project.ID, "error", err)
		}
	} else {
		if err := tl.deps.DB.UpdateProjectStatus(tl.project.ID, db.ProjectCompleted); err != nil {
			log.Warn("failed to update project status to completed", "project_id", tl.project.ID, "error", err)
		}
	}

	tl.emit(TaskEventCompleted, nil, fmt.Sprintf("%d completed, %d failed", result.Completed, result.Failed))
	return result, nil
}

// waitForContinue blocks until Continue() is called or context is cancelled.
// It emits paused/resumed events for TUI feedback.
func (tl *TaskLoop) waitForContinue(ctx context.Context, task *db.Task) error {
	tl.emit(TaskEventPaused, task, "Waiting for confirmation to continue")

	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-tl.pauseCh:
		tl.emit(TaskEventResumed, task, "Continuing to next task")
		return nil
	}
}

// processTask handles the execution of a single task.
func (tl *TaskLoop) processTask(ctx context.Context, task *db.Task) error {
	// 1. Update status to in_progress
	if err := tl.deps.DB.UpdateTaskStatus(task.ID, db.TaskInProgress); err != nil {
		return fmt.Errorf("failed to update task status: %w", err)
	}

	// 2. Create jj change for this task
	changeID, err := tl.deps.JJ.NewChange(ctx, task.Title)
	if err != nil {
		return fmt.Errorf("failed to create jj change: %w", err)
	}

	// 3. Store the change ID on the task
	if err := tl.deps.DB.UpdateTaskJJChangeID(task.ID, changeID); err != nil {
		return fmt.Errorf("failed to store jj change ID: %w", err)
	}

	// 4. Create and run the implementation loop
	implLoop := NewImplLoop(ImplLoopDeps{
		DB:     tl.deps.DB,
		Claude: tl.deps.Claude,
		JJ:     tl.deps.JJ,
		Agents: tl.deps.Agents,
		Config: tl.deps.Config,
	}, task, tl.project.PlanText)

	// Forward implementation loop events to our event channel
	go func() {
		for event := range implLoop.Events() {
			tl.emitWithImpl(task, &event)
		}
	}()

	// Run the implementation loop
	if err := implLoop.Run(ctx); err != nil {
		// Check if it's an escalation (max iterations reached)
		if implLoop.Status() == ImplLoopStatusEscalated {
			if dbErr := tl.deps.DB.UpdateTaskStatus(task.ID, db.TaskEscalated); dbErr != nil {
				log.Warn("failed to update task status to escalated", "task_id", task.ID, "error", dbErr)
			}
			return fmt.Errorf("task escalated: %w", err)
		}
		return err
	}

	// 5. Mark task as completed
	if err := tl.deps.DB.UpdateTaskStatus(task.ID, db.TaskCompleted); err != nil {
		return fmt.Errorf("failed to mark task completed: %w", err)
	}

	return nil
}
