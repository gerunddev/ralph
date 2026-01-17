package engine

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/gerund/ralph/internal/agents"
	"github.com/gerund/ralph/internal/claude"
	"github.com/gerund/ralph/internal/config"
	"github.com/gerund/ralph/internal/db"
	"github.com/gerund/ralph/internal/jj"
)

// Helper to create test dependencies for TaskLoop
func createTaskLoopDeps(t *testing.T) (TaskLoopDeps, *db.Project, func()) {
	// Create in-memory database
	database, err := db.New(":memory:")
	if err != nil {
		t.Fatalf("failed to create test database: %v", err)
	}

	// Create a test project
	project := &db.Project{
		ID:       "test-project",
		Name:     "Test Project",
		PlanText: "Test plan for the project",
		Status:   db.ProjectPending,
	}
	if err := database.CreateProject(project); err != nil {
		t.Fatalf("failed to create test project: %v", err)
	}

	cfg := config.DefaultConfig()

	deps := TaskLoopDeps{
		DB:     database,
		Claude: claude.NewClient(claude.ClientConfig{Model: "test"}),
		JJ:     jj.NewClient("/tmp/test"),
		Agents: agents.NewManager(cfg),
		Config: cfg,
	}

	cleanup := func() {
		database.Close()
	}

	return deps, project, cleanup
}

// Helper to create test tasks for a project
func createTestTasks(t *testing.T, database *db.DB, projectID string, count int) []*db.Task {
	tasks := make([]*db.Task, count)
	for i := 0; i < count; i++ {
		tasks[i] = &db.Task{
			ID:          "task-" + string(rune('a'+i)),
			ProjectID:   projectID,
			Sequence:    i + 1,
			Title:       "Task " + string(rune('A'+i)),
			Description: "Description for task " + string(rune('A'+i)),
			Status:      db.TaskPending,
		}
	}
	if err := database.CreateTasks(tasks); err != nil {
		t.Fatalf("failed to create test tasks: %v", err)
	}
	return tasks
}

func TestNewTaskLoop(t *testing.T) {
	deps, project, cleanup := createTaskLoopDeps(t)
	defer cleanup()

	tl := NewTaskLoop(deps, project)

	if tl == nil {
		t.Fatal("NewTaskLoop returned nil")
	}
	if tl.project != project {
		t.Error("project not set correctly")
	}
	if tl.events == nil {
		t.Error("events channel not initialized")
	}
}

func TestTaskLoop_EmptyProject(t *testing.T) {
	deps, project, cleanup := createTaskLoopDeps(t)
	defer cleanup()

	tl := NewTaskLoop(deps, project)

	// Drain events
	go func() {
		for range tl.Events() {
		}
	}()

	result, err := tl.Run(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if result.Completed != 0 {
		t.Errorf("expected 0 completed, got %d", result.Completed)
	}
	if result.Failed != 0 {
		t.Errorf("expected 0 failed, got %d", result.Failed)
	}
}

func TestTaskLoop_Events(t *testing.T) {
	deps, project, cleanup := createTaskLoopDeps(t)
	defer cleanup()

	tl := NewTaskLoop(deps, project)

	events := tl.Events()
	if events == nil {
		t.Fatal("Events() returned nil channel")
	}
}

func TestTaskLoop_CurrentTask_Empty(t *testing.T) {
	deps, project, cleanup := createTaskLoopDeps(t)
	defer cleanup()

	tl := NewTaskLoop(deps, project)

	// Before running, no tasks are loaded
	if tl.CurrentTask() != nil {
		t.Error("expected CurrentTask to return nil before run")
	}
}

func TestTaskLoop_Progress(t *testing.T) {
	deps, project, cleanup := createTaskLoopDeps(t)
	defer cleanup()

	tl := NewTaskLoop(deps, project)

	// Before running
	current, total := tl.Progress()
	if current != 1 || total != 0 {
		t.Errorf("expected (1, 0) before run, got (%d, %d)", current, total)
	}
}

func TestTaskLoop_SkipsCompletedTasks(t *testing.T) {
	deps, project, cleanup := createTaskLoopDeps(t)
	defer cleanup()

	// Create only completed tasks - these will be counted without running impl loop
	tasks := []*db.Task{
		{
			ID:        "task-1",
			ProjectID: project.ID,
			Sequence:  1,
			Title:     "Completed Task",
			Status:    db.TaskCompleted,
		},
	}
	if err := deps.DB.CreateTasks(tasks); err != nil {
		t.Fatalf("failed to create tasks: %v", err)
	}

	tl := NewTaskLoop(deps, project)

	// Drain events
	go func() {
		for range tl.Events() {
		}
	}()

	result, err := tl.Run(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// The completed task should be counted
	if result.Completed != 1 {
		t.Errorf("expected 1 completed (from skip), got %d", result.Completed)
	}
}

func TestTaskLoop_SkipsFailedTasks(t *testing.T) {
	deps, project, cleanup := createTaskLoopDeps(t)
	defer cleanup()

	// Create tasks with some already failed
	tasks := []*db.Task{
		{
			ID:        "task-1",
			ProjectID: project.ID,
			Sequence:  1,
			Title:     "Failed Task",
			Status:    db.TaskFailed,
		},
	}
	if err := deps.DB.CreateTasks(tasks); err != nil {
		t.Fatalf("failed to create tasks: %v", err)
	}

	tl := NewTaskLoop(deps, project)

	// Drain events
	go func() {
		for range tl.Events() {
		}
	}()

	result, err := tl.Run(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// The failed task should be counted
	if result.Failed != 1 {
		t.Errorf("expected 1 failed (from skip), got %d", result.Failed)
	}
}

func TestTaskLoop_SkipsEscalatedTasks(t *testing.T) {
	deps, project, cleanup := createTaskLoopDeps(t)
	defer cleanup()

	// Create tasks with some already escalated
	tasks := []*db.Task{
		{
			ID:        "task-1",
			ProjectID: project.ID,
			Sequence:  1,
			Title:     "Escalated Task",
			Status:    db.TaskEscalated,
		},
	}
	if err := deps.DB.CreateTasks(tasks); err != nil {
		t.Fatalf("failed to create tasks: %v", err)
	}

	tl := NewTaskLoop(deps, project)

	// Drain events
	go func() {
		for range tl.Events() {
		}
	}()

	result, err := tl.Run(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// The escalated task should be counted as skipped
	if result.Skipped != 1 {
		t.Errorf("expected 1 skipped, got %d", result.Skipped)
	}
}

func TestTaskLoop_ContextCancellation(t *testing.T) {
	deps, project, cleanup := createTaskLoopDeps(t)
	defer cleanup()

	// Create a pending task
	tasks := []*db.Task{
		{
			ID:        "task-1",
			ProjectID: project.ID,
			Sequence:  1,
			Title:     "Pending Task",
			Status:    db.TaskPending,
		},
	}
	if err := deps.DB.CreateTasks(tasks); err != nil {
		t.Fatalf("failed to create tasks: %v", err)
	}

	tl := NewTaskLoop(deps, project)

	// Drain events
	go func() {
		for range tl.Events() {
		}
	}()

	ctx, cancel := context.WithCancel(context.Background())
	cancel() // Cancel immediately

	_, err := tl.Run(ctx)
	if !errors.Is(err, context.Canceled) {
		t.Errorf("expected context.Canceled error, got %v", err)
	}
}

func TestTaskLoop_EmitEvents(t *testing.T) {
	deps, project, cleanup := createTaskLoopDeps(t)
	defer cleanup()

	tl := NewTaskLoop(deps, project)

	// Start a goroutine to emit a test event
	go func() {
		task := &db.Task{Title: "Test Task"}
		tl.emit(TaskEventStarted, task, "test message")
	}()

	// Read the event
	select {
	case event := <-tl.Events():
		if event.Type != TaskEventStarted {
			t.Errorf("expected event type %s, got %s", TaskEventStarted, event.Type)
		}
		if event.Message != "test message" {
			t.Errorf("expected message %q, got %q", "test message", event.Message)
		}
	case <-time.After(100 * time.Millisecond):
		t.Error("timeout waiting for event")
	}
}

func TestTaskLoop_EmitWithImpl(t *testing.T) {
	deps, project, cleanup := createTaskLoopDeps(t)
	defer cleanup()

	tl := NewTaskLoop(deps, project)

	task := &db.Task{Title: "Test Task"}
	implEvent := &ImplLoopEvent{
		Type:      EventDeveloping,
		Iteration: 1,
		Message:   "developer working",
	}

	// Start a goroutine to emit the event
	go func() {
		tl.emitWithImpl(task, implEvent)
	}()

	// Read the event
	select {
	case event := <-tl.Events():
		if event.Type != TaskEventProgress {
			t.Errorf("expected event type %s, got %s", TaskEventProgress, event.Type)
		}
		if event.ImplEvent == nil {
			t.Error("expected ImplEvent to be set")
		} else if event.ImplEvent.Type != EventDeveloping {
			t.Errorf("expected nested event type %s, got %s", EventDeveloping, event.ImplEvent.Type)
		}
	case <-time.After(100 * time.Millisecond):
		t.Error("timeout waiting for event")
	}
}

func TestTaskLoopResult_Structure(t *testing.T) {
	result := TaskLoopResult{
		Completed: 5,
		Failed:    2,
		Skipped:   1,
	}

	if result.Completed != 5 {
		t.Errorf("expected Completed 5, got %d", result.Completed)
	}
	if result.Failed != 2 {
		t.Errorf("expected Failed 2, got %d", result.Failed)
	}
	if result.Skipped != 1 {
		t.Errorf("expected Skipped 1, got %d", result.Skipped)
	}
}

func TestTaskEventTypes(t *testing.T) {
	// Ensure all event types are distinct
	eventTypes := []TaskEventType{
		TaskEventStarted,
		TaskEventTaskBegin,
		TaskEventTaskEnd,
		TaskEventCompleted,
		TaskEventFailed,
		TaskEventProgress,
	}

	seen := make(map[TaskEventType]bool)
	for _, et := range eventTypes {
		if seen[et] {
			t.Errorf("duplicate event type: %s", et)
		}
		seen[et] = true
	}
}

func TestTaskLoopDeps_Structure(t *testing.T) {
	// Verify that TaskLoopDeps has all expected fields
	deps := TaskLoopDeps{
		DB:     nil,
		Claude: nil,
		JJ:     nil,
		Agents: nil,
		Config: nil,
	}

	// Just verifying the struct compiles with these fields
	_ = deps
}

func TestTaskLoopEvent_Structure(t *testing.T) {
	implEvent := &ImplLoopEvent{Type: EventStarted}
	event := TaskLoopEvent{
		Type:      TaskEventTaskBegin,
		TaskIndex: 2,
		TaskTitle: "Test Task",
		Message:   "test message",
		ImplEvent: implEvent,
	}

	if event.Type != TaskEventTaskBegin {
		t.Error("Type field not set correctly")
	}
	if event.TaskIndex != 2 {
		t.Error("TaskIndex field not set correctly")
	}
	if event.TaskTitle != "Test Task" {
		t.Error("TaskTitle field not set correctly")
	}
	if event.Message != "test message" {
		t.Error("Message field not set correctly")
	}
	if event.ImplEvent != implEvent {
		t.Error("ImplEvent field not set correctly")
	}
}

func TestTaskLoop_ConcurrentProgressAccess(t *testing.T) {
	deps, project, cleanup := createTaskLoopDeps(t)
	defer cleanup()

	tl := NewTaskLoop(deps, project)

	// Test concurrent access to progress
	done := make(chan bool)
	go func() {
		for i := 0; i < 100; i++ {
			tl.SetCurrentForTesting(i)
		}
		done <- true
	}()
	go func() {
		for i := 0; i < 100; i++ {
			_, _ = tl.Progress()
		}
		done <- true
	}()

	<-done
	<-done
	// If we get here without a race condition, the test passes
}

func TestTaskLoop_ConcurrentCurrentTaskAccess(t *testing.T) {
	deps, project, cleanup := createTaskLoopDeps(t)
	defer cleanup()

	// Create some tasks
	createTestTasks(t, deps.DB, project.ID, 3)

	tl := NewTaskLoop(deps, project)
	// Load tasks via the test helper
	tasks, _ := deps.DB.GetTasksByProject(project.ID)
	tl.SetTasksForTesting(tasks)

	// Test concurrent access to CurrentTask
	done := make(chan bool)
	go func() {
		for i := 0; i < 100; i++ {
			tl.SetCurrentForTesting(i % len(tasks))
		}
		done <- true
	}()
	go func() {
		for i := 0; i < 100; i++ {
			_ = tl.CurrentTask()
		}
		done <- true
	}()

	<-done
	<-done
	// If we get here without a race condition, the test passes
}

func TestTaskLoop_MixedTaskStatuses(t *testing.T) {
	deps, project, cleanup := createTaskLoopDeps(t)
	defer cleanup()

	// Create tasks with mixed statuses
	tasks := []*db.Task{
		{
			ID:        "task-1",
			ProjectID: project.ID,
			Sequence:  1,
			Title:     "Completed Task",
			Status:    db.TaskCompleted,
		},
		{
			ID:        "task-2",
			ProjectID: project.ID,
			Sequence:  2,
			Title:     "Failed Task",
			Status:    db.TaskFailed,
		},
		{
			ID:        "task-3",
			ProjectID: project.ID,
			Sequence:  3,
			Title:     "Escalated Task",
			Status:    db.TaskEscalated,
		},
	}
	if err := deps.DB.CreateTasks(tasks); err != nil {
		t.Fatalf("failed to create tasks: %v", err)
	}

	tl := NewTaskLoop(deps, project)

	// Drain events
	go func() {
		for range tl.Events() {
		}
	}()

	result, err := tl.Run(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if result.Completed != 1 {
		t.Errorf("expected 1 completed, got %d", result.Completed)
	}
	if result.Failed != 1 {
		t.Errorf("expected 1 failed, got %d", result.Failed)
	}
	if result.Skipped != 1 {
		t.Errorf("expected 1 skipped, got %d", result.Skipped)
	}
}

func TestTaskLoop_ProjectStatusUpdated(t *testing.T) {
	deps, project, cleanup := createTaskLoopDeps(t)
	defer cleanup()

	// Create a completed task (no pending work)
	tasks := []*db.Task{
		{
			ID:        "task-1",
			ProjectID: project.ID,
			Sequence:  1,
			Title:     "Completed Task",
			Status:    db.TaskCompleted,
		},
	}
	if err := deps.DB.CreateTasks(tasks); err != nil {
		t.Fatalf("failed to create tasks: %v", err)
	}

	tl := NewTaskLoop(deps, project)

	// Drain events
	go func() {
		for range tl.Events() {
		}
	}()

	_, err := tl.Run(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Check project status was updated
	updatedProject, err := deps.DB.GetProject(project.ID)
	if err != nil {
		t.Fatalf("failed to get project: %v", err)
	}
	if updatedProject.Status != db.ProjectCompleted {
		t.Errorf("expected project status %s, got %s", db.ProjectCompleted, updatedProject.Status)
	}
}

func TestTaskLoop_ProjectStatusFailedOnFailedTasks(t *testing.T) {
	deps, project, cleanup := createTaskLoopDeps(t)
	defer cleanup()

	// Create a failed task
	tasks := []*db.Task{
		{
			ID:        "task-1",
			ProjectID: project.ID,
			Sequence:  1,
			Title:     "Failed Task",
			Status:    db.TaskFailed,
		},
	}
	if err := deps.DB.CreateTasks(tasks); err != nil {
		t.Fatalf("failed to create tasks: %v", err)
	}

	tl := NewTaskLoop(deps, project)

	// Drain events
	go func() {
		for range tl.Events() {
		}
	}()

	_, err := tl.Run(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Check project status was updated to failed
	updatedProject, err := deps.DB.GetProject(project.ID)
	if err != nil {
		t.Fatalf("failed to get project: %v", err)
	}
	if updatedProject.Status != db.ProjectFailed {
		t.Errorf("expected project status %s, got %s", db.ProjectFailed, updatedProject.Status)
	}
}

func TestTaskLoop_EventChannelClosed(t *testing.T) {
	deps, project, cleanup := createTaskLoopDeps(t)
	defer cleanup()

	tl := NewTaskLoop(deps, project)

	done := make(chan bool)
	go func() {
		for range tl.Events() {
			// Drain events
		}
		done <- true
	}()

	_, err := tl.Run(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Wait for channel to close
	select {
	case <-done:
		// Success - channel was closed
	case <-time.After(time.Second):
		t.Error("timeout waiting for events channel to close")
	}
}

func TestTaskLoop_EmitNilTask(t *testing.T) {
	deps, project, cleanup := createTaskLoopDeps(t)
	defer cleanup()

	tl := NewTaskLoop(deps, project)

	// Emit with nil task should not panic
	go func() {
		tl.emit(TaskEventCompleted, nil, "All done")
	}()

	select {
	case event := <-tl.Events():
		if event.TaskTitle != "" {
			t.Errorf("expected empty task title, got %q", event.TaskTitle)
		}
	case <-time.After(100 * time.Millisecond):
		t.Error("timeout waiting for event")
	}
}

func TestTaskLoop_PauseModeDefaults(t *testing.T) {
	deps, project, cleanup := createTaskLoopDeps(t)
	defer cleanup()

	// Default config has pause mode disabled
	tl := NewTaskLoop(deps, project)

	if tl.IsPauseMode() {
		t.Error("expected pause mode to be disabled by default")
	}
}

func TestTaskLoop_PauseModeFromConfig(t *testing.T) {
	deps, project, cleanup := createTaskLoopDeps(t)
	defer cleanup()

	// Enable default pause mode in config
	deps.Config.DefaultPauseMode = true

	tl := NewTaskLoop(deps, project)

	if !tl.IsPauseMode() {
		t.Error("expected pause mode to be enabled from config")
	}
}

func TestTaskLoop_SetPauseMode(t *testing.T) {
	deps, project, cleanup := createTaskLoopDeps(t)
	defer cleanup()

	tl := NewTaskLoop(deps, project)

	// Initially disabled
	if tl.IsPauseMode() {
		t.Error("expected pause mode to be disabled initially")
	}

	// Enable pause mode
	go func() {
		// Drain the event that will be emitted
		<-tl.Events()
	}()

	tl.SetPauseMode(true)
	if !tl.IsPauseMode() {
		t.Error("expected pause mode to be enabled after SetPauseMode(true)")
	}

	// Disable pause mode
	go func() {
		// Drain the event
		<-tl.Events()
	}()

	tl.SetPauseMode(false)
	if tl.IsPauseMode() {
		t.Error("expected pause mode to be disabled after SetPauseMode(false)")
	}
}

func TestTaskLoop_SetPauseModeEmitsEvent(t *testing.T) {
	deps, project, cleanup := createTaskLoopDeps(t)
	defer cleanup()

	tl := NewTaskLoop(deps, project)

	// Set pause mode and check for event
	go func() {
		tl.SetPauseMode(true)
	}()

	select {
	case event := <-tl.Events():
		if event.Type != TaskEventPauseModeChanged {
			t.Errorf("expected event type %s, got %s", TaskEventPauseModeChanged, event.Type)
		}
		if event.Message != "Pause mode: true" {
			t.Errorf("expected message 'Pause mode: true', got %q", event.Message)
		}
		if event.PauseModeEnabled == nil {
			t.Error("expected PauseModeEnabled field to be set")
		} else if !*event.PauseModeEnabled {
			t.Error("expected PauseModeEnabled to be true")
		}
	case <-time.After(100 * time.Millisecond):
		t.Error("timeout waiting for pause mode event")
	}
}

func TestTaskLoop_ContinueWhenNotPaused(t *testing.T) {
	deps, project, cleanup := createTaskLoopDeps(t)
	defer cleanup()

	tl := NewTaskLoop(deps, project)

	// Continue when not paused should be a no-op
	tl.Continue()
	// No panic means success
}

func TestTaskLoop_ContinueSignalsPauseCh(t *testing.T) {
	deps, project, cleanup := createTaskLoopDeps(t)
	defer cleanup()

	tl := NewTaskLoop(deps, project)

	// Access the pauseCh for testing
	go func() {
		time.Sleep(10 * time.Millisecond)
		tl.Continue()
	}()

	select {
	case <-tl.pauseCh:
		// Success
	case <-time.After(100 * time.Millisecond):
		t.Error("timeout waiting for continue signal")
	}
}

func TestTaskLoop_PauseModeEventTypes(t *testing.T) {
	// Ensure all pause-related event types are distinct
	eventTypes := []TaskEventType{
		TaskEventPaused,
		TaskEventResumed,
		TaskEventPauseModeChanged,
	}

	seen := make(map[TaskEventType]bool)
	for _, et := range eventTypes {
		if seen[et] {
			t.Errorf("duplicate event type: %s", et)
		}
		seen[et] = true

		// Also ensure they don't conflict with existing types
		if et == TaskEventStarted || et == TaskEventTaskBegin || et == TaskEventTaskEnd ||
			et == TaskEventCompleted || et == TaskEventFailed || et == TaskEventProgress {
			t.Errorf("pause event type %s conflicts with existing event type", et)
		}
	}
}

func TestTaskLoop_ConcurrentPauseModeAccess(t *testing.T) {
	deps, project, cleanup := createTaskLoopDeps(t)
	defer cleanup()

	tl := NewTaskLoop(deps, project)

	// Drain events
	go func() {
		for range tl.Events() {
		}
	}()

	// Test concurrent access to pause mode
	done := make(chan bool)
	go func() {
		for i := 0; i < 100; i++ {
			tl.SetPauseMode(i%2 == 0)
		}
		done <- true
	}()
	go func() {
		for i := 0; i < 100; i++ {
			_ = tl.IsPauseMode()
		}
		done <- true
	}()

	<-done
	<-done
	// If we get here without a race condition, the test passes
}
