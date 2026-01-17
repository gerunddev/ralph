package engine

import (
	"context"
	"testing"

	"github.com/gerund/ralph/internal/db"
)

func TestDetectProjectState_AllPending(t *testing.T) {
	database, cfg, workDir, cleanup := createEngineDeps(t)
	defer cleanup()

	// Create project with pending tasks
	project := &db.Project{
		ID:       "test-project",
		Name:     "Test Project",
		PlanText: "Test plan",
		Status:   db.ProjectPending,
	}
	if err := database.CreateProject(project); err != nil {
		t.Fatalf("failed to create project: %v", err)
	}

	tasks := []*db.Task{
		{ID: "t1", ProjectID: project.ID, Sequence: 1, Title: "Task 1", Status: db.TaskPending},
		{ID: "t2", ProjectID: project.ID, Sequence: 2, Title: "Task 2", Status: db.TaskPending},
		{ID: "t3", ProjectID: project.ID, Sequence: 3, Title: "Task 3", Status: db.TaskPending},
	}
	if err := database.CreateTasks(tasks); err != nil {
		t.Fatalf("failed to create tasks: %v", err)
	}

	engine, err := NewEngine(EngineConfig{Config: cfg, DB: database, WorkDir: workDir})
	if err != nil {
		t.Fatalf("failed to create engine: %v", err)
	}

	state, err := engine.DetectProjectState(context.Background(), project.ID)
	if err != nil {
		t.Fatalf("DetectProjectState failed: %v", err)
	}

	if state.Project.ID != project.ID {
		t.Errorf("expected project ID %s, got %s", project.ID, state.Project.ID)
	}
	if state.PendingTasks != 3 {
		t.Errorf("expected 3 pending tasks, got %d", state.PendingTasks)
	}
	if state.CompletedTasks != 0 {
		t.Errorf("expected 0 completed tasks, got %d", state.CompletedTasks)
	}
	if state.FailedTasks != 0 {
		t.Errorf("expected 0 failed tasks, got %d", state.FailedTasks)
	}
	if state.InProgressTask != nil {
		t.Error("expected no in-progress task")
	}
	if state.NeedsCleanup {
		t.Error("expected no cleanup needed")
	}
}

func TestDetectProjectState_MixedStatuses(t *testing.T) {
	database, cfg, workDir, cleanup := createEngineDeps(t)
	defer cleanup()

	project := &db.Project{
		ID:       "test-project",
		Name:     "Test Project",
		PlanText: "Test plan",
		Status:   db.ProjectInProgress,
	}
	if err := database.CreateProject(project); err != nil {
		t.Fatalf("failed to create project: %v", err)
	}

	tasks := []*db.Task{
		{ID: "t1", ProjectID: project.ID, Sequence: 1, Title: "Task 1", Status: db.TaskCompleted},
		{ID: "t2", ProjectID: project.ID, Sequence: 2, Title: "Task 2", Status: db.TaskCompleted},
		{ID: "t3", ProjectID: project.ID, Sequence: 3, Title: "Task 3", Status: db.TaskPending},
		{ID: "t4", ProjectID: project.ID, Sequence: 4, Title: "Task 4", Status: db.TaskFailed},
	}
	if err := database.CreateTasks(tasks); err != nil {
		t.Fatalf("failed to create tasks: %v", err)
	}

	engine, err := NewEngine(EngineConfig{Config: cfg, DB: database, WorkDir: workDir})
	if err != nil {
		t.Fatalf("failed to create engine: %v", err)
	}

	state, err := engine.DetectProjectState(context.Background(), project.ID)
	if err != nil {
		t.Fatalf("DetectProjectState failed: %v", err)
	}

	if state.CompletedTasks != 2 {
		t.Errorf("expected 2 completed tasks, got %d", state.CompletedTasks)
	}
	if state.PendingTasks != 1 {
		t.Errorf("expected 1 pending task, got %d", state.PendingTasks)
	}
	if state.FailedTasks != 1 {
		t.Errorf("expected 1 failed task, got %d", state.FailedTasks)
	}
}

func TestDetectProjectState_InProgressTask(t *testing.T) {
	database, cfg, workDir, cleanup := createEngineDeps(t)
	defer cleanup()

	project := &db.Project{
		ID:       "test-project",
		Name:     "Test Project",
		PlanText: "Test plan",
		Status:   db.ProjectInProgress,
	}
	if err := database.CreateProject(project); err != nil {
		t.Fatalf("failed to create project: %v", err)
	}

	tasks := []*db.Task{
		{ID: "t1", ProjectID: project.ID, Sequence: 1, Title: "Task 1", Status: db.TaskCompleted},
		{ID: "t2", ProjectID: project.ID, Sequence: 2, Title: "Task 2", Status: db.TaskInProgress},
		{ID: "t3", ProjectID: project.ID, Sequence: 3, Title: "Task 3", Status: db.TaskPending},
	}
	if err := database.CreateTasks(tasks); err != nil {
		t.Fatalf("failed to create tasks: %v", err)
	}

	engine, err := NewEngine(EngineConfig{Config: cfg, DB: database, WorkDir: workDir})
	if err != nil {
		t.Fatalf("failed to create engine: %v", err)
	}

	state, err := engine.DetectProjectState(context.Background(), project.ID)
	if err != nil {
		t.Fatalf("DetectProjectState failed: %v", err)
	}

	if state.InProgressTask == nil {
		t.Fatal("expected in-progress task")
	}
	if state.InProgressTask.ID != "t2" {
		t.Errorf("expected in-progress task t2, got %s", state.InProgressTask.ID)
	}
	if !state.NeedsCleanup {
		t.Error("expected cleanup needed")
	}
}

func TestDetectProjectState_EscalatedCountsAsFailed(t *testing.T) {
	database, cfg, workDir, cleanup := createEngineDeps(t)
	defer cleanup()

	project := &db.Project{
		ID:       "test-project",
		Name:     "Test Project",
		PlanText: "Test plan",
		Status:   db.ProjectFailed,
	}
	if err := database.CreateProject(project); err != nil {
		t.Fatalf("failed to create project: %v", err)
	}

	tasks := []*db.Task{
		{ID: "t1", ProjectID: project.ID, Sequence: 1, Title: "Task 1", Status: db.TaskEscalated},
		{ID: "t2", ProjectID: project.ID, Sequence: 2, Title: "Task 2", Status: db.TaskFailed},
	}
	if err := database.CreateTasks(tasks); err != nil {
		t.Fatalf("failed to create tasks: %v", err)
	}

	engine, err := NewEngine(EngineConfig{Config: cfg, DB: database, WorkDir: workDir})
	if err != nil {
		t.Fatalf("failed to create engine: %v", err)
	}

	state, err := engine.DetectProjectState(context.Background(), project.ID)
	if err != nil {
		t.Fatalf("DetectProjectState failed: %v", err)
	}

	// Both escalated and failed should be counted as failed
	if state.FailedTasks != 2 {
		t.Errorf("expected 2 failed tasks, got %d", state.FailedTasks)
	}
}

func TestDetectProjectState_NotFound(t *testing.T) {
	database, cfg, workDir, cleanup := createEngineDeps(t)
	defer cleanup()

	engine, err := NewEngine(EngineConfig{Config: cfg, DB: database, WorkDir: workDir})
	if err != nil {
		t.Fatalf("failed to create engine: %v", err)
	}

	_, err = engine.DetectProjectState(context.Background(), "nonexistent-id")
	if err == nil {
		t.Error("expected error for nonexistent project")
	}
}

func TestCleanupForResume_NoCleanupNeeded(t *testing.T) {
	database, cfg, workDir, cleanup := createEngineDeps(t)
	defer cleanup()

	engine, err := NewEngine(EngineConfig{Config: cfg, DB: database, WorkDir: workDir})
	if err != nil {
		t.Fatalf("failed to create engine: %v", err)
	}

	state := &ProjectState{
		NeedsCleanup: false,
	}

	err = engine.CleanupForResume(context.Background(), state)
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestCleanupForResume_ResetsInProgressTask(t *testing.T) {
	database, cfg, workDir, cleanup := createEngineDeps(t)
	defer cleanup()

	project := &db.Project{
		ID:       "test-project",
		Name:     "Test Project",
		PlanText: "Test plan",
		Status:   db.ProjectInProgress,
	}
	if err := database.CreateProject(project); err != nil {
		t.Fatalf("failed to create project: %v", err)
	}

	task := &db.Task{
		ID:        "t1",
		ProjectID: project.ID,
		Sequence:  1,
		Title:     "Task 1",
		Status:    db.TaskInProgress,
	}
	if err := database.CreateTask(task); err != nil {
		t.Fatalf("failed to create task: %v", err)
	}

	engine, err := NewEngine(EngineConfig{Config: cfg, DB: database, WorkDir: workDir})
	if err != nil {
		t.Fatalf("failed to create engine: %v", err)
	}

	state := &ProjectState{
		InProgressTask: task,
		NeedsCleanup:   true,
	}

	err = engine.CleanupForResume(context.Background(), state)
	if err != nil {
		t.Fatalf("CleanupForResume failed: %v", err)
	}

	// Verify task is now pending
	updatedTask, err := database.GetTask(task.ID)
	if err != nil {
		t.Fatalf("failed to get task: %v", err)
	}
	if updatedTask.Status != db.TaskPending {
		t.Errorf("expected task status to be pending, got %s", updatedTask.Status)
	}
}

func TestCleanupForResume_FailsRunningSession(t *testing.T) {
	database, cfg, workDir, cleanup := createEngineDeps(t)
	defer cleanup()

	project := &db.Project{
		ID:       "test-project",
		Name:     "Test Project",
		PlanText: "Test plan",
		Status:   db.ProjectInProgress,
	}
	if err := database.CreateProject(project); err != nil {
		t.Fatalf("failed to create project: %v", err)
	}

	task := &db.Task{
		ID:        "t1",
		ProjectID: project.ID,
		Sequence:  1,
		Title:     "Task 1",
		Status:    db.TaskInProgress,
	}
	if err := database.CreateTask(task); err != nil {
		t.Fatalf("failed to create task: %v", err)
	}

	session := &db.Session{
		ID:        "s1",
		TaskID:    task.ID,
		AgentType: db.AgentDeveloper,
		Iteration: 1,
		Status:    db.SessionRunning,
	}
	if err := database.CreateSession(session); err != nil {
		t.Fatalf("failed to create session: %v", err)
	}

	engine, err := NewEngine(EngineConfig{Config: cfg, DB: database, WorkDir: workDir})
	if err != nil {
		t.Fatalf("failed to create engine: %v", err)
	}

	state := &ProjectState{
		InProgressTask: task,
		LastSession:    session,
		NeedsCleanup:   true,
	}

	err = engine.CleanupForResume(context.Background(), state)
	if err != nil {
		t.Fatalf("CleanupForResume failed: %v", err)
	}

	// Verify session is now failed
	updatedSession, err := database.GetSession(session.ID)
	if err != nil {
		t.Fatalf("failed to get session: %v", err)
	}
	if updatedSession.Status != db.SessionFailed {
		t.Errorf("expected session status to be failed, got %s", updatedSession.Status)
	}
}

func TestResetProject(t *testing.T) {
	database, cfg, workDir, cleanup := createEngineDeps(t)
	defer cleanup()

	project := &db.Project{
		ID:                "test-project",
		Name:              "Test Project",
		PlanText:          "Test plan",
		Status:            db.ProjectCompleted,
		UserFeedbackState: db.FeedbackStateComplete,
		LearningsState:    db.LearningsStateComplete,
	}
	if err := database.CreateProject(project); err != nil {
		t.Fatalf("failed to create project: %v", err)
	}

	tasks := []*db.Task{
		{ID: "t1", ProjectID: project.ID, Sequence: 1, Title: "Task 1", Status: db.TaskCompleted},
		{ID: "t2", ProjectID: project.ID, Sequence: 2, Title: "Task 2", Status: db.TaskFailed},
		{ID: "t3", ProjectID: project.ID, Sequence: 3, Title: "Task 3", Status: db.TaskEscalated},
	}
	if err := database.CreateTasks(tasks); err != nil {
		t.Fatalf("failed to create tasks: %v", err)
	}

	engine, err := NewEngine(EngineConfig{Config: cfg, DB: database, WorkDir: workDir})
	if err != nil {
		t.Fatalf("failed to create engine: %v", err)
	}

	err = engine.ResetProject(context.Background(), project.ID)
	if err != nil {
		t.Fatalf("ResetProject failed: %v", err)
	}

	// Verify all tasks are pending
	updatedTasks, err := database.GetTasksByProject(project.ID)
	if err != nil {
		t.Fatalf("failed to get tasks: %v", err)
	}
	for _, task := range updatedTasks {
		if task.Status != db.TaskPending {
			t.Errorf("expected task %s to be pending, got %s", task.ID, task.Status)
		}
	}

	// Verify project status is reset
	updatedProject, err := database.GetProject(project.ID)
	if err != nil {
		t.Fatalf("failed to get project: %v", err)
	}
	if updatedProject.Status != db.ProjectPending {
		t.Errorf("expected project status to be pending, got %s", updatedProject.Status)
	}
	if updatedProject.UserFeedbackState != db.FeedbackStateNone {
		t.Errorf("expected feedback state to be none, got %s", updatedProject.UserFeedbackState)
	}
	if updatedProject.LearningsState != db.LearningsStateNone {
		t.Errorf("expected learnings state to be none, got %s", updatedProject.LearningsState)
	}
}

func TestRetryFailedTasks(t *testing.T) {
	database, cfg, workDir, cleanup := createEngineDeps(t)
	defer cleanup()

	project := &db.Project{
		ID:       "test-project",
		Name:     "Test Project",
		PlanText: "Test plan",
		Status:   db.ProjectFailed,
	}
	if err := database.CreateProject(project); err != nil {
		t.Fatalf("failed to create project: %v", err)
	}

	tasks := []*db.Task{
		{ID: "t1", ProjectID: project.ID, Sequence: 1, Title: "Task 1", Status: db.TaskCompleted},
		{ID: "t2", ProjectID: project.ID, Sequence: 2, Title: "Task 2", Status: db.TaskFailed},
		{ID: "t3", ProjectID: project.ID, Sequence: 3, Title: "Task 3", Status: db.TaskEscalated},
		{ID: "t4", ProjectID: project.ID, Sequence: 4, Title: "Task 4", Status: db.TaskPending},
	}
	if err := database.CreateTasks(tasks); err != nil {
		t.Fatalf("failed to create tasks: %v", err)
	}

	engine, err := NewEngine(EngineConfig{Config: cfg, DB: database, WorkDir: workDir})
	if err != nil {
		t.Fatalf("failed to create engine: %v", err)
	}

	err = engine.RetryFailedTasks(context.Background(), project.ID)
	if err != nil {
		t.Fatalf("RetryFailedTasks failed: %v", err)
	}

	// Verify failed and escalated tasks are now pending
	task2, _ := database.GetTask("t2")
	if task2.Status != db.TaskPending {
		t.Errorf("expected task t2 to be pending, got %s", task2.Status)
	}

	task3, _ := database.GetTask("t3")
	if task3.Status != db.TaskPending {
		t.Errorf("expected task t3 to be pending, got %s", task3.Status)
	}

	// Verify completed task is still completed
	task1, _ := database.GetTask("t1")
	if task1.Status != db.TaskCompleted {
		t.Errorf("expected task t1 to remain completed, got %s", task1.Status)
	}

	// Verify pending task is still pending
	task4, _ := database.GetTask("t4")
	if task4.Status != db.TaskPending {
		t.Errorf("expected task t4 to remain pending, got %s", task4.Status)
	}

	// Verify project status is reset to pending
	updatedProject, _ := database.GetProject(project.ID)
	if updatedProject.Status != db.ProjectPending {
		t.Errorf("expected project status to be pending, got %s", updatedProject.Status)
	}
}

func TestIsProjectResumable(t *testing.T) {
	database, cfg, workDir, cleanup := createEngineDeps(t)
	defer cleanup()

	engine, err := NewEngine(EngineConfig{Config: cfg, DB: database, WorkDir: workDir})
	if err != nil {
		t.Fatalf("failed to create engine: %v", err)
	}

	t.Run("resumable with pending tasks", func(t *testing.T) {
		project := &db.Project{
			ID:       "resumable-pending",
			Name:     "Test Project",
			PlanText: "Test plan",
			Status:   db.ProjectPending,
		}
		if err := database.CreateProject(project); err != nil {
			t.Fatalf("failed to create project: %v", err)
		}
		task := &db.Task{
			ID:        "t1-rp",
			ProjectID: project.ID,
			Sequence:  1,
			Title:     "Task 1",
			Status:    db.TaskPending,
		}
		if err := database.CreateTask(task); err != nil {
			t.Fatalf("failed to create task: %v", err)
		}

		resumable, err := engine.IsProjectResumable(context.Background(), project.ID)
		if err != nil {
			t.Fatalf("IsProjectResumable failed: %v", err)
		}
		if !resumable {
			t.Error("expected project to be resumable")
		}
	})

	t.Run("resumable with in-progress task", func(t *testing.T) {
		project := &db.Project{
			ID:       "resumable-inprogress",
			Name:     "Test Project",
			PlanText: "Test plan",
			Status:   db.ProjectInProgress,
		}
		if err := database.CreateProject(project); err != nil {
			t.Fatalf("failed to create project: %v", err)
		}
		task := &db.Task{
			ID:        "t1-ri",
			ProjectID: project.ID,
			Sequence:  1,
			Title:     "Task 1",
			Status:    db.TaskInProgress,
		}
		if err := database.CreateTask(task); err != nil {
			t.Fatalf("failed to create task: %v", err)
		}

		resumable, err := engine.IsProjectResumable(context.Background(), project.ID)
		if err != nil {
			t.Fatalf("IsProjectResumable failed: %v", err)
		}
		if !resumable {
			t.Error("expected project to be resumable")
		}
	})

	t.Run("not resumable when completed", func(t *testing.T) {
		project := &db.Project{
			ID:       "not-resumable",
			Name:     "Test Project",
			PlanText: "Test plan",
			Status:   db.ProjectCompleted,
		}
		if err := database.CreateProject(project); err != nil {
			t.Fatalf("failed to create project: %v", err)
		}
		task := &db.Task{
			ID:        "t1-nr",
			ProjectID: project.ID,
			Sequence:  1,
			Title:     "Task 1",
			Status:    db.TaskCompleted,
		}
		if err := database.CreateTask(task); err != nil {
			t.Fatalf("failed to create task: %v", err)
		}

		resumable, err := engine.IsProjectResumable(context.Background(), project.ID)
		if err != nil {
			t.Fatalf("IsProjectResumable failed: %v", err)
		}
		if resumable {
			t.Error("expected project to not be resumable")
		}
	})
}

func TestProjectState_TotalTasks(t *testing.T) {
	state := &ProjectState{
		CompletedTasks: 2,
		PendingTasks:   3,
		FailedTasks:    1,
	}
	if state.TotalTasks() != 6 {
		t.Errorf("expected 6 total tasks, got %d", state.TotalTasks())
	}

	// With in-progress task
	state.InProgressTask = &db.Task{}
	if state.TotalTasks() != 7 {
		t.Errorf("expected 7 total tasks with in-progress, got %d", state.TotalTasks())
	}
}

func TestProjectState_HasInterruptedWork(t *testing.T) {
	state := &ProjectState{}
	if state.HasInterruptedWork() {
		t.Error("expected no interrupted work")
	}

	state.InProgressTask = &db.Task{}
	if !state.HasInterruptedWork() {
		t.Error("expected interrupted work with in-progress task")
	}

	state.InProgressTask = nil
	state.NeedsCleanup = true
	if !state.HasInterruptedWork() {
		t.Error("expected interrupted work with cleanup needed")
	}
}

func TestProjectState_IsComplete(t *testing.T) {
	// Not complete - has pending
	state := &ProjectState{CompletedTasks: 1, PendingTasks: 1}
	if state.IsComplete() {
		t.Error("expected not complete with pending tasks")
	}

	// Not complete - has failed
	state = &ProjectState{CompletedTasks: 1, FailedTasks: 1}
	if state.IsComplete() {
		t.Error("expected not complete with failed tasks")
	}

	// Not complete - has in-progress
	state = &ProjectState{CompletedTasks: 1, InProgressTask: &db.Task{}}
	if state.IsComplete() {
		t.Error("expected not complete with in-progress task")
	}

	// Not complete - no completed tasks
	state = &ProjectState{}
	if state.IsComplete() {
		t.Error("expected not complete with no completed tasks")
	}

	// Complete
	state = &ProjectState{CompletedTasks: 3}
	if !state.IsComplete() {
		t.Error("expected complete")
	}
}

func TestProjectState_HasFailedTasks(t *testing.T) {
	state := &ProjectState{}
	if state.HasFailedTasks() {
		t.Error("expected no failed tasks")
	}

	state.FailedTasks = 1
	if !state.HasFailedTasks() {
		t.Error("expected failed tasks")
	}
}

func TestDetectProjectState_WithRunningSession(t *testing.T) {
	database, cfg, workDir, cleanup := createEngineDeps(t)
	defer cleanup()

	project := &db.Project{
		ID:       "test-project",
		Name:     "Test Project",
		PlanText: "Test plan",
		Status:   db.ProjectInProgress,
	}
	if err := database.CreateProject(project); err != nil {
		t.Fatalf("failed to create project: %v", err)
	}

	task := &db.Task{
		ID:        "t1",
		ProjectID: project.ID,
		Sequence:  1,
		Title:     "Task 1",
		Status:    db.TaskInProgress,
	}
	if err := database.CreateTask(task); err != nil {
		t.Fatalf("failed to create task: %v", err)
	}

	session := &db.Session{
		ID:        "s1",
		TaskID:    task.ID,
		AgentType: db.AgentDeveloper,
		Iteration: 1,
		Status:    db.SessionRunning,
	}
	if err := database.CreateSession(session); err != nil {
		t.Fatalf("failed to create session: %v", err)
	}

	engine, err := NewEngine(EngineConfig{Config: cfg, DB: database, WorkDir: workDir})
	if err != nil {
		t.Fatalf("failed to create engine: %v", err)
	}

	state, err := engine.DetectProjectState(context.Background(), project.ID)
	if err != nil {
		t.Fatalf("DetectProjectState failed: %v", err)
	}

	if state.LastSession == nil {
		t.Fatal("expected last session")
	}
	if state.LastSession.ID != session.ID {
		t.Errorf("expected session ID %s, got %s", session.ID, state.LastSession.ID)
	}
	if !state.NeedsCleanup {
		t.Error("expected cleanup needed for running session")
	}
}

// createEngineDeps is defined in engine_test.go, but we need a local version
// if this file is run independently. Since we're in the same package, we can
// use the one from engine_test.go.

func TestResetProject_NotFound(t *testing.T) {
	database, cfg, workDir, cleanup := createEngineDeps(t)
	defer cleanup()

	engine, err := NewEngine(EngineConfig{Config: cfg, DB: database, WorkDir: workDir})
	if err != nil {
		t.Fatalf("failed to create engine: %v", err)
	}

	err = engine.ResetProject(context.Background(), "nonexistent-id")
	if err == nil {
		t.Error("expected error for nonexistent project")
	}
}

func TestRetryFailedTasks_NoFailedTasks(t *testing.T) {
	database, cfg, workDir, cleanup := createEngineDeps(t)
	defer cleanup()

	project := &db.Project{
		ID:       "test-project",
		Name:     "Test Project",
		PlanText: "Test plan",
		Status:   db.ProjectCompleted,
	}
	if err := database.CreateProject(project); err != nil {
		t.Fatalf("failed to create project: %v", err)
	}

	tasks := []*db.Task{
		{ID: "t1", ProjectID: project.ID, Sequence: 1, Title: "Task 1", Status: db.TaskCompleted},
	}
	if err := database.CreateTasks(tasks); err != nil {
		t.Fatalf("failed to create tasks: %v", err)
	}

	engine, err := NewEngine(EngineConfig{Config: cfg, DB: database, WorkDir: workDir})
	if err != nil {
		t.Fatalf("failed to create engine: %v", err)
	}

	// Should not error even with no failed tasks
	err = engine.RetryFailedTasks(context.Background(), project.ID)
	if err != nil {
		t.Fatalf("RetryFailedTasks failed: %v", err)
	}
}

