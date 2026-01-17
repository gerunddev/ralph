// Package engine provides the main execution engine for Ralph.
package engine

import (
	"context"
	"fmt"

	"github.com/gerund/ralph/internal/db"
)

// ProjectState represents the detected state of a project for resume decisions.
type ProjectState struct {
	Project        *db.Project
	CompletedTasks int
	PendingTasks   int
	FailedTasks    int
	InProgressTask *db.Task   // nil if none
	LastSession    *db.Session // nil if none
	NeedsCleanup   bool
}

// DetectProjectState analyzes a project's current state for resume decisions.
func (e *Engine) DetectProjectState(ctx context.Context, projectID string) (*ProjectState, error) {
	project, err := e.db.GetProject(projectID)
	if err != nil {
		return nil, fmt.Errorf("failed to get project: %w", err)
	}

	tasks, err := e.db.GetTasksByProject(projectID)
	if err != nil {
		return nil, fmt.Errorf("failed to get tasks: %w", err)
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
			state.FailedTasks++ // Count as failed
		}
	}

	// Check for running sessions on in-progress task
	if state.InProgressTask != nil {
		session, err := e.db.GetLatestSessionForTask(state.InProgressTask.ID)
		if err == nil {
			state.LastSession = session
			if session.Status == db.SessionRunning {
				state.NeedsCleanup = true
			}
		}
		// Ignore ErrNotFound - just means no session yet
	}

	return state, nil
}

// CleanupForResume prepares a project for resumption by resetting any interrupted state.
func (e *Engine) CleanupForResume(ctx context.Context, state *ProjectState) error {
	if !state.NeedsCleanup {
		return nil
	}

	// 1. Mark in-progress task as pending
	if state.InProgressTask != nil {
		if err := e.db.UpdateTaskStatus(state.InProgressTask.ID, db.TaskPending); err != nil {
			return fmt.Errorf("failed to reset in-progress task: %w", err)
		}
		// Reset iteration count by updating the task
		// Note: We restart the task from scratch rather than continuing mid-iteration
	}

	// 2. Mark running sessions as failed
	if state.LastSession != nil && state.LastSession.Status == db.SessionRunning {
		if err := e.db.CompleteSession(state.LastSession.ID, db.SessionFailed); err != nil {
			return fmt.Errorf("failed to mark session as failed: %w", err)
		}
	}

	// 3. Keep project status as-is - the task loop will update it as needed
	return nil
}

// ResetProject resets all tasks in a project to pending status.
func (e *Engine) ResetProject(ctx context.Context, projectID string) error {
	// 1. Get all tasks
	tasks, err := e.db.GetTasksByProject(projectID)
	if err != nil {
		return fmt.Errorf("failed to get tasks: %w", err)
	}

	// 2. Reset all tasks to pending
	for _, task := range tasks {
		if err := e.db.UpdateTaskStatus(task.ID, db.TaskPending); err != nil {
			return fmt.Errorf("failed to reset task %s: %w", task.ID, err)
		}
		// Note: We keep jj_change_id for reference - changes are already in the repo
	}

	// 3. Reset project status
	if err := e.db.UpdateProjectStatus(projectID, db.ProjectPending); err != nil {
		return fmt.Errorf("failed to reset project status: %w", err)
	}

	// 4. Reset feedback and learnings state
	if err := e.db.UpdateProjectFeedbackState(projectID, db.FeedbackStateNone); err != nil {
		return fmt.Errorf("failed to reset feedback state: %w", err)
	}
	if err := e.db.UpdateProjectLearningsState(projectID, db.LearningsStateNone); err != nil {
		return fmt.Errorf("failed to reset learnings state: %w", err)
	}

	return nil
}

// RetryFailedTasks resets failed tasks to pending so they can be retried.
func (e *Engine) RetryFailedTasks(ctx context.Context, projectID string) error {
	tasks, err := e.db.GetTasksByProject(projectID)
	if err != nil {
		return fmt.Errorf("failed to get tasks: %w", err)
	}

	for _, task := range tasks {
		if task.Status == db.TaskFailed || task.Status == db.TaskEscalated {
			if err := e.db.UpdateTaskStatus(task.ID, db.TaskPending); err != nil {
				return fmt.Errorf("failed to reset failed task %s: %w", task.ID, err)
			}
		}
	}

	// Update project status back to pending if it was failed
	project, err := e.db.GetProject(projectID)
	if err != nil {
		return fmt.Errorf("failed to get project: %w", err)
	}

	if project.Status == db.ProjectFailed || project.Status == db.ProjectCompleted {
		if err := e.db.UpdateProjectStatus(projectID, db.ProjectPending); err != nil {
			return fmt.Errorf("failed to update project status: %w", err)
		}
	}

	return nil
}

// IsProjectResumable returns true if the project has work that can be resumed.
func (e *Engine) IsProjectResumable(ctx context.Context, projectID string) (bool, error) {
	state, err := e.DetectProjectState(ctx, projectID)
	if err != nil {
		return false, err
	}

	// Project is resumable if there are pending tasks or interrupted work
	return state.PendingTasks > 0 || state.InProgressTask != nil, nil
}

// TotalTasks returns the count of all task statuses.
func (s *ProjectState) TotalTasks() int {
	total := s.CompletedTasks + s.PendingTasks + s.FailedTasks
	if s.InProgressTask != nil {
		total++
	}
	return total
}

// HasInterruptedWork returns true if there's work that was interrupted.
func (s *ProjectState) HasInterruptedWork() bool {
	return s.InProgressTask != nil || s.NeedsCleanup
}

// IsComplete returns true if all tasks are completed.
func (s *ProjectState) IsComplete() bool {
	return s.PendingTasks == 0 && s.FailedTasks == 0 && s.InProgressTask == nil && s.CompletedTasks > 0
}

// HasFailedTasks returns true if there are any failed tasks.
func (s *ProjectState) HasFailedTasks() bool {
	return s.FailedTasks > 0
}
