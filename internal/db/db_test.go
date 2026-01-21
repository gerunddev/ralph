package db

import (
	"errors"
	"testing"
	"time"
)

// newTestDB creates a new in-memory database for testing.
func newTestDB(t *testing.T) *DB {
	t.Helper()
	db, err := New(":memory:")
	if err != nil {
		t.Fatalf("failed to create test database: %v", err)
	}
	t.Cleanup(func() {
		if err := db.Close(); err != nil {
			t.Errorf("failed to close test database: %v", err)
		}
	})
	return db
}

// =============================================================================
// Database Connection Tests
// =============================================================================

func TestNew(t *testing.T) {
	db, err := New(":memory:")
	if err != nil {
		t.Fatalf("New() returned error: %v", err)
	}
	defer func() {
		if err := db.Close(); err != nil {
			t.Errorf("Close() returned error: %v", err)
		}
	}()

	if db.conn == nil {
		t.Error("New() returned DB with nil connection")
	}
}

func TestNew_AutoMigrate(t *testing.T) {
	db := newTestDB(t)

	// Verify tables exist by inserting a project
	project := &Project{
		ID:       "test-project",
		Name:     "Test Project",
		PlanText: "Test plan",
	}
	if err := db.CreateProject(project); err != nil {
		t.Errorf("CreateProject() after migration failed: %v", err)
	}
}

func TestClose(t *testing.T) {
	db, err := New(":memory:")
	if err != nil {
		t.Fatalf("New() returned error: %v", err)
	}

	if err := db.Close(); err != nil {
		t.Errorf("Close() returned error: %v", err)
	}

	// Double close should not panic or error
	if err := db.Close(); err != nil {
		t.Errorf("Double Close() returned error: %v", err)
	}
}

// =============================================================================
// Project Tests
// =============================================================================

func TestCreateProject(t *testing.T) {
	db := newTestDB(t)

	project := &Project{
		ID:       "proj-1",
		Name:     "My Project",
		PlanText: "Build something great",
	}

	if err := db.CreateProject(project); err != nil {
		t.Fatalf("CreateProject() returned error: %v", err)
	}

	// Verify timestamps were set
	if project.CreatedAt.IsZero() {
		t.Error("CreateProject() did not set CreatedAt")
	}
	if project.UpdatedAt.IsZero() {
		t.Error("CreateProject() did not set UpdatedAt")
	}

	// Verify default status
	if project.Status != ProjectPending {
		t.Errorf("CreateProject() status = %v, want %v", project.Status, ProjectPending)
	}
}

func TestGetProject(t *testing.T) {
	db := newTestDB(t)

	project := &Project{
		ID:       "proj-1",
		Name:     "My Project",
		PlanText: "Build something great",
		Status:   ProjectInProgress,
	}
	if err := db.CreateProject(project); err != nil {
		t.Fatalf("CreateProject() returned error: %v", err)
	}

	got, err := db.GetProject("proj-1")
	if err != nil {
		t.Fatalf("GetProject() returned error: %v", err)
	}

	if got.ID != project.ID {
		t.Errorf("GetProject().ID = %v, want %v", got.ID, project.ID)
	}
	if got.Name != project.Name {
		t.Errorf("GetProject().Name = %v, want %v", got.Name, project.Name)
	}
	if got.PlanText != project.PlanText {
		t.Errorf("GetProject().PlanText = %v, want %v", got.PlanText, project.PlanText)
	}
	if got.Status != ProjectInProgress {
		t.Errorf("GetProject().Status = %v, want %v", got.Status, ProjectInProgress)
	}
}

func TestGetProject_NotFound(t *testing.T) {
	db := newTestDB(t)

	_, err := db.GetProject("nonexistent")
	if !errors.Is(err, ErrNotFound) {
		t.Errorf("GetProject() error = %v, want ErrNotFound", err)
	}
}

func TestListProjects(t *testing.T) {
	db := newTestDB(t)

	// Create projects with different updated_at times
	proj1 := &Project{ID: "proj-1", Name: "First", PlanText: "Plan 1"}
	proj2 := &Project{ID: "proj-2", Name: "Second", PlanText: "Plan 2"}
	proj3 := &Project{ID: "proj-3", Name: "Third", PlanText: "Plan 3"}

	if err := db.CreateProject(proj1); err != nil {
		t.Fatalf("CreateProject() returned error: %v", err)
	}
	time.Sleep(10 * time.Millisecond)
	if err := db.CreateProject(proj2); err != nil {
		t.Fatalf("CreateProject() returned error: %v", err)
	}
	time.Sleep(10 * time.Millisecond)
	if err := db.CreateProject(proj3); err != nil {
		t.Fatalf("CreateProject() returned error: %v", err)
	}

	projects, err := db.ListProjects()
	if err != nil {
		t.Fatalf("ListProjects() returned error: %v", err)
	}

	if len(projects) != 3 {
		t.Errorf("ListProjects() returned %d projects, want 3", len(projects))
	}

	// Should be ordered by updated_at DESC (newest first)
	if projects[0].ID != "proj-3" {
		t.Errorf("ListProjects()[0].ID = %v, want proj-3", projects[0].ID)
	}
}

func TestListProjects_Empty(t *testing.T) {
	db := newTestDB(t)

	projects, err := db.ListProjects()
	if err != nil {
		t.Fatalf("ListProjects() returned error: %v", err)
	}

	if len(projects) != 0 {
		t.Errorf("ListProjects() returned %d projects, want 0", len(projects))
	}
}

func TestUpdateProjectStatus(t *testing.T) {
	db := newTestDB(t)

	project := &Project{
		ID:       "proj-1",
		Name:     "My Project",
		PlanText: "Plan",
	}
	if err := db.CreateProject(project); err != nil {
		t.Fatalf("CreateProject() returned error: %v", err)
	}

	if err := db.UpdateProjectStatus("proj-1", ProjectCompleted); err != nil {
		t.Fatalf("UpdateProjectStatus() returned error: %v", err)
	}

	got, err := db.GetProject("proj-1")
	if err != nil {
		t.Fatalf("GetProject() returned error: %v", err)
	}
	if got.Status != ProjectCompleted {
		t.Errorf("UpdateProjectStatus() status = %v, want %v", got.Status, ProjectCompleted)
	}
}

func TestUpdateProjectStatus_NotFound(t *testing.T) {
	db := newTestDB(t)

	err := db.UpdateProjectStatus("nonexistent", ProjectCompleted)
	if !errors.Is(err, ErrNotFound) {
		t.Errorf("UpdateProjectStatus() error = %v, want ErrNotFound", err)
	}
}

// =============================================================================
// Task Tests
// =============================================================================

func TestCreateTask(t *testing.T) {
	db := newTestDB(t)

	// Create project first (foreign key)
	project := &Project{ID: "proj-1", Name: "Project", PlanText: "Plan"}
	if err := db.CreateProject(project); err != nil {
		t.Fatalf("CreateProject() returned error: %v", err)
	}

	task := &Task{
		ID:          "task-1",
		ProjectID:   "proj-1",
		Sequence:    1,
		Title:       "First Task",
		Description: "Do something",
	}

	if err := db.CreateTask(task); err != nil {
		t.Fatalf("CreateTask() returned error: %v", err)
	}

	if task.Status != TaskPending {
		t.Errorf("CreateTask() status = %v, want %v", task.Status, TaskPending)
	}
	if task.CreatedAt.IsZero() {
		t.Error("CreateTask() did not set CreatedAt")
	}
}

func TestCreateTask_ForeignKey(t *testing.T) {
	db := newTestDB(t)

	// Try to create task without project
	task := &Task{
		ID:          "task-1",
		ProjectID:   "nonexistent",
		Sequence:    1,
		Title:       "Task",
		Description: "Desc",
	}

	err := db.CreateTask(task)
	if err == nil {
		t.Error("CreateTask() should fail with invalid project_id")
	}
}

func TestCreateTasks_Bulk(t *testing.T) {
	db := newTestDB(t)

	project := &Project{ID: "proj-1", Name: "Project", PlanText: "Plan"}
	if err := db.CreateProject(project); err != nil {
		t.Fatalf("CreateProject() returned error: %v", err)
	}

	tasks := []*Task{
		{ID: "task-1", ProjectID: "proj-1", Sequence: 1, Title: "Task 1", Description: "Desc 1"},
		{ID: "task-2", ProjectID: "proj-1", Sequence: 2, Title: "Task 2", Description: "Desc 2"},
		{ID: "task-3", ProjectID: "proj-1", Sequence: 3, Title: "Task 3", Description: "Desc 3"},
	}

	if err := db.CreateTasks(tasks); err != nil {
		t.Fatalf("CreateTasks() returned error: %v", err)
	}

	got, err := db.GetTasksByProject("proj-1")
	if err != nil {
		t.Fatalf("GetTasksByProject() returned error: %v", err)
	}
	if len(got) != 3 {
		t.Errorf("CreateTasks() created %d tasks, want 3", len(got))
	}
}

func TestGetTask(t *testing.T) {
	db := newTestDB(t)

	project := &Project{ID: "proj-1", Name: "Project", PlanText: "Plan"}
	if err := db.CreateProject(project); err != nil {
		t.Fatalf("CreateProject() returned error: %v", err)
	}

	changeID := "abc123"
	task := &Task{
		ID:          "task-1",
		ProjectID:   "proj-1",
		Sequence:    1,
		Title:       "Task",
		Description: "Desc",
		JJChangeID:  &changeID,
	}
	if err := db.CreateTask(task); err != nil {
		t.Fatalf("CreateTask() returned error: %v", err)
	}

	got, err := db.GetTask("task-1")
	if err != nil {
		t.Fatalf("GetTask() returned error: %v", err)
	}

	if got.ID != "task-1" {
		t.Errorf("GetTask().ID = %v, want task-1", got.ID)
	}
	if got.JJChangeID == nil || *got.JJChangeID != "abc123" {
		t.Errorf("GetTask().JJChangeID = %v, want abc123", got.JJChangeID)
	}
}

func TestGetTask_NotFound(t *testing.T) {
	db := newTestDB(t)

	_, err := db.GetTask("nonexistent")
	if !errors.Is(err, ErrNotFound) {
		t.Errorf("GetTask() error = %v, want ErrNotFound", err)
	}
}

func TestGetTasksByProject(t *testing.T) {
	db := newTestDB(t)

	project := &Project{ID: "proj-1", Name: "Project", PlanText: "Plan"}
	if err := db.CreateProject(project); err != nil {
		t.Fatalf("CreateProject() returned error: %v", err)
	}

	// Create tasks out of order
	if err := db.CreateTask(&Task{ID: "task-3", ProjectID: "proj-1", Sequence: 3, Title: "T3", Description: "D3"}); err != nil {
		t.Fatalf("CreateTask() returned error: %v", err)
	}
	if err := db.CreateTask(&Task{ID: "task-1", ProjectID: "proj-1", Sequence: 1, Title: "T1", Description: "D1"}); err != nil {
		t.Fatalf("CreateTask() returned error: %v", err)
	}
	if err := db.CreateTask(&Task{ID: "task-2", ProjectID: "proj-1", Sequence: 2, Title: "T2", Description: "D2"}); err != nil {
		t.Fatalf("CreateTask() returned error: %v", err)
	}

	tasks, err := db.GetTasksByProject("proj-1")
	if err != nil {
		t.Fatalf("GetTasksByProject() returned error: %v", err)
	}

	if len(tasks) != 3 {
		t.Errorf("GetTasksByProject() returned %d tasks, want 3", len(tasks))
	}

	// Should be ordered by sequence
	if tasks[0].Sequence != 1 || tasks[1].Sequence != 2 || tasks[2].Sequence != 3 {
		t.Error("GetTasksByProject() tasks not ordered by sequence")
	}
}

func TestGetNextPendingTask(t *testing.T) {
	db := newTestDB(t)

	project := &Project{ID: "proj-1", Name: "Project", PlanText: "Plan"}
	if err := db.CreateProject(project); err != nil {
		t.Fatalf("CreateProject() returned error: %v", err)
	}

	if err := db.CreateTask(&Task{ID: "task-1", ProjectID: "proj-1", Sequence: 1, Title: "T1", Description: "D1", Status: TaskCompleted}); err != nil {
		t.Fatalf("CreateTask() returned error: %v", err)
	}
	if err := db.CreateTask(&Task{ID: "task-2", ProjectID: "proj-1", Sequence: 2, Title: "T2", Description: "D2", Status: TaskPending}); err != nil {
		t.Fatalf("CreateTask() returned error: %v", err)
	}
	if err := db.CreateTask(&Task{ID: "task-3", ProjectID: "proj-1", Sequence: 3, Title: "T3", Description: "D3", Status: TaskPending}); err != nil {
		t.Fatalf("CreateTask() returned error: %v", err)
	}

	task, err := db.GetNextPendingTask("proj-1")
	if err != nil {
		t.Fatalf("GetNextPendingTask() returned error: %v", err)
	}

	if task.ID != "task-2" {
		t.Errorf("GetNextPendingTask() returned %v, want task-2", task.ID)
	}
}

func TestGetNextPendingTask_NoneRemaining(t *testing.T) {
	db := newTestDB(t)

	project := &Project{ID: "proj-1", Name: "Project", PlanText: "Plan"}
	if err := db.CreateProject(project); err != nil {
		t.Fatalf("CreateProject() returned error: %v", err)
	}

	if err := db.CreateTask(&Task{ID: "task-1", ProjectID: "proj-1", Sequence: 1, Title: "T1", Description: "D1", Status: TaskCompleted}); err != nil {
		t.Fatalf("CreateTask() returned error: %v", err)
	}

	_, err := db.GetNextPendingTask("proj-1")
	if !errors.Is(err, ErrNotFound) {
		t.Errorf("GetNextPendingTask() error = %v, want ErrNotFound", err)
	}
}

func TestUpdateTaskStatus(t *testing.T) {
	db := newTestDB(t)

	project := &Project{ID: "proj-1", Name: "Project", PlanText: "Plan"}
	if err := db.CreateProject(project); err != nil {
		t.Fatalf("CreateProject() returned error: %v", err)
	}
	if err := db.CreateTask(&Task{ID: "task-1", ProjectID: "proj-1", Sequence: 1, Title: "T", Description: "D"}); err != nil {
		t.Fatalf("CreateTask() returned error: %v", err)
	}

	if err := db.UpdateTaskStatus("task-1", TaskInProgress); err != nil {
		t.Fatalf("UpdateTaskStatus() returned error: %v", err)
	}

	got, err := db.GetTask("task-1")
	if err != nil {
		t.Fatalf("GetTask() returned error: %v", err)
	}
	if got.Status != TaskInProgress {
		t.Errorf("UpdateTaskStatus() status = %v, want %v", got.Status, TaskInProgress)
	}
}

func TestUpdateTaskJJChangeID(t *testing.T) {
	db := newTestDB(t)

	project := &Project{ID: "proj-1", Name: "Project", PlanText: "Plan"}
	if err := db.CreateProject(project); err != nil {
		t.Fatalf("CreateProject() returned error: %v", err)
	}
	if err := db.CreateTask(&Task{ID: "task-1", ProjectID: "proj-1", Sequence: 1, Title: "T", Description: "D"}); err != nil {
		t.Fatalf("CreateTask() returned error: %v", err)
	}

	if err := db.UpdateTaskJJChangeID("task-1", "xyz789"); err != nil {
		t.Fatalf("UpdateTaskJJChangeID() returned error: %v", err)
	}

	got, err := db.GetTask("task-1")
	if err != nil {
		t.Fatalf("GetTask() returned error: %v", err)
	}
	if got.JJChangeID == nil || *got.JJChangeID != "xyz789" {
		t.Errorf("UpdateTaskJJChangeID() jj_change_id = %v, want xyz789", got.JJChangeID)
	}
}

func TestIncrementTaskIteration(t *testing.T) {
	db := newTestDB(t)

	project := &Project{ID: "proj-1", Name: "Project", PlanText: "Plan"}
	if err := db.CreateProject(project); err != nil {
		t.Fatalf("CreateProject() returned error: %v", err)
	}
	if err := db.CreateTask(&Task{ID: "task-1", ProjectID: "proj-1", Sequence: 1, Title: "T", Description: "D"}); err != nil {
		t.Fatalf("CreateTask() returned error: %v", err)
	}

	// Increment twice
	if err := db.IncrementTaskIteration("task-1"); err != nil {
		t.Fatalf("IncrementTaskIteration() returned error: %v", err)
	}
	if err := db.IncrementTaskIteration("task-1"); err != nil {
		t.Fatalf("IncrementTaskIteration() returned error: %v", err)
	}

	got, err := db.GetTask("task-1")
	if err != nil {
		t.Fatalf("GetTask() returned error: %v", err)
	}
	if got.IterationCount != 2 {
		t.Errorf("IncrementTaskIteration() count = %d, want 2", got.IterationCount)
	}
}

// =============================================================================
// Session Tests
// =============================================================================

func TestCreateSession(t *testing.T) {
	db := newTestDB(t)

	project := &Project{ID: "proj-1", Name: "Project", PlanText: "Plan"}
	if err := db.CreateProject(project); err != nil {
		t.Fatalf("CreateProject() returned error: %v", err)
	}
	if err := db.CreateTask(&Task{ID: "task-1", ProjectID: "proj-1", Sequence: 1, Title: "T", Description: "D"}); err != nil {
		t.Fatalf("CreateTask() returned error: %v", err)
	}

	session := &Session{
		ID:          "sess-1",
		TaskID:      "task-1",
		AgentType:   AgentDeveloper,
		Iteration:   1,
		InputPrompt: "Do the thing",
	}

	if err := db.CreateSession(session); err != nil {
		t.Fatalf("CreateSession() returned error: %v", err)
	}

	if session.Status != SessionRunning {
		t.Errorf("CreateSession() status = %v, want %v", session.Status, SessionRunning)
	}
	if session.CreatedAt.IsZero() {
		t.Error("CreateSession() did not set CreatedAt")
	}
}

func TestCreateSession_ForeignKey(t *testing.T) {
	db := newTestDB(t)

	session := &Session{
		ID:          "sess-1",
		TaskID:      "nonexistent",
		AgentType:   AgentDeveloper,
		Iteration:   1,
		InputPrompt: "Prompt",
	}

	err := db.CreateSession(session)
	if err == nil {
		t.Error("CreateSession() should fail with invalid task_id")
	}
}

func TestGetSession(t *testing.T) {
	db := newTestDB(t)

	project := &Project{ID: "proj-1", Name: "Project", PlanText: "Plan"}
	if err := db.CreateProject(project); err != nil {
		t.Fatalf("CreateProject() returned error: %v", err)
	}
	if err := db.CreateTask(&Task{ID: "task-1", ProjectID: "proj-1", Sequence: 1, Title: "T", Description: "D"}); err != nil {
		t.Fatalf("CreateTask() returned error: %v", err)
	}
	if err := db.CreateSession(&Session{ID: "sess-1", TaskID: "task-1", AgentType: AgentReviewer, Iteration: 1, InputPrompt: "Review"}); err != nil {
		t.Fatalf("CreateSession() returned error: %v", err)
	}

	got, err := db.GetSession("sess-1")
	if err != nil {
		t.Fatalf("GetSession() returned error: %v", err)
	}

	if got.AgentType != AgentReviewer {
		t.Errorf("GetSession().AgentType = %v, want %v", got.AgentType, AgentReviewer)
	}
}

func TestGetSessionsByTask(t *testing.T) {
	db := newTestDB(t)

	project := &Project{ID: "proj-1", Name: "Project", PlanText: "Plan"}
	if err := db.CreateProject(project); err != nil {
		t.Fatalf("CreateProject() returned error: %v", err)
	}
	if err := db.CreateTask(&Task{ID: "task-1", ProjectID: "proj-1", Sequence: 1, Title: "T", Description: "D"}); err != nil {
		t.Fatalf("CreateTask() returned error: %v", err)
	}

	// Create sessions in reverse order
	if err := db.CreateSession(&Session{ID: "sess-3", TaskID: "task-1", AgentType: AgentDeveloper, Iteration: 3, InputPrompt: "P3"}); err != nil {
		t.Fatalf("CreateSession() returned error: %v", err)
	}
	if err := db.CreateSession(&Session{ID: "sess-1", TaskID: "task-1", AgentType: AgentDeveloper, Iteration: 1, InputPrompt: "P1"}); err != nil {
		t.Fatalf("CreateSession() returned error: %v", err)
	}
	if err := db.CreateSession(&Session{ID: "sess-2", TaskID: "task-1", AgentType: AgentReviewer, Iteration: 2, InputPrompt: "P2"}); err != nil {
		t.Fatalf("CreateSession() returned error: %v", err)
	}

	sessions, err := db.GetSessionsByTask("task-1")
	if err != nil {
		t.Fatalf("GetSessionsByTask() returned error: %v", err)
	}

	if len(sessions) != 3 {
		t.Errorf("GetSessionsByTask() returned %d sessions, want 3", len(sessions))
	}

	// Should be ordered by iteration
	if sessions[0].Iteration != 1 || sessions[1].Iteration != 2 || sessions[2].Iteration != 3 {
		t.Error("GetSessionsByTask() sessions not ordered by iteration")
	}
}

func TestCompleteSession(t *testing.T) {
	db := newTestDB(t)

	project := &Project{ID: "proj-1", Name: "Project", PlanText: "Plan"}
	if err := db.CreateProject(project); err != nil {
		t.Fatalf("CreateProject() returned error: %v", err)
	}
	if err := db.CreateTask(&Task{ID: "task-1", ProjectID: "proj-1", Sequence: 1, Title: "T", Description: "D"}); err != nil {
		t.Fatalf("CreateTask() returned error: %v", err)
	}
	if err := db.CreateSession(&Session{ID: "sess-1", TaskID: "task-1", AgentType: AgentDeveloper, Iteration: 1, InputPrompt: "P"}); err != nil {
		t.Fatalf("CreateSession() returned error: %v", err)
	}

	if err := db.CompleteSession("sess-1", SessionCompleted); err != nil {
		t.Fatalf("CompleteSession() returned error: %v", err)
	}

	got, err := db.GetSession("sess-1")
	if err != nil {
		t.Fatalf("GetSession() returned error: %v", err)
	}
	if got.Status != SessionCompleted {
		t.Errorf("CompleteSession() status = %v, want %v", got.Status, SessionCompleted)
	}
	if got.CompletedAt == nil {
		t.Error("CompleteSession() did not set CompletedAt")
	}
}

func TestGetLatestSessionForTask(t *testing.T) {
	db := newTestDB(t)

	project := &Project{ID: "proj-1", Name: "Project", PlanText: "Plan"}
	if err := db.CreateProject(project); err != nil {
		t.Fatalf("CreateProject() returned error: %v", err)
	}
	if err := db.CreateTask(&Task{ID: "task-1", ProjectID: "proj-1", Sequence: 1, Title: "T", Description: "D"}); err != nil {
		t.Fatalf("CreateTask() returned error: %v", err)
	}

	if err := db.CreateSession(&Session{ID: "sess-1", TaskID: "task-1", AgentType: AgentDeveloper, Iteration: 1, InputPrompt: "P1"}); err != nil {
		t.Fatalf("CreateSession() returned error: %v", err)
	}
	time.Sleep(10 * time.Millisecond)
	if err := db.CreateSession(&Session{ID: "sess-2", TaskID: "task-1", AgentType: AgentReviewer, Iteration: 2, InputPrompt: "P2"}); err != nil {
		t.Fatalf("CreateSession() returned error: %v", err)
	}

	got, err := db.GetLatestSessionForTask("task-1")
	if err != nil {
		t.Fatalf("GetLatestSessionForTask() returned error: %v", err)
	}

	if got.ID != "sess-2" {
		t.Errorf("GetLatestSessionForTask() returned %v, want sess-2", got.ID)
	}
}

// =============================================================================
// Message Tests
// =============================================================================

func TestCreateMessage(t *testing.T) {
	db := newTestDB(t)

	project := &Project{ID: "proj-1", Name: "Project", PlanText: "Plan"}
	if err := db.CreateProject(project); err != nil {
		t.Fatalf("CreateProject() returned error: %v", err)
	}
	if err := db.CreateTask(&Task{ID: "task-1", ProjectID: "proj-1", Sequence: 1, Title: "T", Description: "D"}); err != nil {
		t.Fatalf("CreateTask() returned error: %v", err)
	}
	if err := db.CreateSession(&Session{ID: "sess-1", TaskID: "task-1", AgentType: AgentDeveloper, Iteration: 1, InputPrompt: "P"}); err != nil {
		t.Fatalf("CreateSession() returned error: %v", err)
	}

	msg := &Message{
		SessionID:   "sess-1",
		Sequence:    1,
		MessageType: "text",
		Content:     `{"type":"text","content":"Hello"}`,
	}

	if err := db.CreateMessage(msg); err != nil {
		t.Fatalf("CreateMessage() returned error: %v", err)
	}

	if msg.ID == 0 {
		t.Error("CreateMessage() did not set ID")
	}
	if msg.CreatedAt.IsZero() {
		t.Error("CreateMessage() did not set CreatedAt")
	}
}

func TestGetMessagesBySession(t *testing.T) {
	db := newTestDB(t)

	project := &Project{ID: "proj-1", Name: "Project", PlanText: "Plan"}
	if err := db.CreateProject(project); err != nil {
		t.Fatalf("CreateProject() returned error: %v", err)
	}
	if err := db.CreateTask(&Task{ID: "task-1", ProjectID: "proj-1", Sequence: 1, Title: "T", Description: "D"}); err != nil {
		t.Fatalf("CreateTask() returned error: %v", err)
	}
	if err := db.CreateSession(&Session{ID: "sess-1", TaskID: "task-1", AgentType: AgentDeveloper, Iteration: 1, InputPrompt: "P"}); err != nil {
		t.Fatalf("CreateSession() returned error: %v", err)
	}

	// Create messages out of order
	if err := db.CreateMessage(&Message{SessionID: "sess-1", Sequence: 3, MessageType: "text", Content: "C3"}); err != nil {
		t.Fatalf("CreateMessage() returned error: %v", err)
	}
	if err := db.CreateMessage(&Message{SessionID: "sess-1", Sequence: 1, MessageType: "text", Content: "C1"}); err != nil {
		t.Fatalf("CreateMessage() returned error: %v", err)
	}
	if err := db.CreateMessage(&Message{SessionID: "sess-1", Sequence: 2, MessageType: "text", Content: "C2"}); err != nil {
		t.Fatalf("CreateMessage() returned error: %v", err)
	}

	messages, err := db.GetMessagesBySession("sess-1")
	if err != nil {
		t.Fatalf("GetMessagesBySession() returned error: %v", err)
	}

	if len(messages) != 3 {
		t.Errorf("GetMessagesBySession() returned %d messages, want 3", len(messages))
	}

	// Should be ordered by sequence
	if messages[0].Sequence != 1 || messages[1].Sequence != 2 || messages[2].Sequence != 3 {
		t.Error("GetMessagesBySession() messages not ordered by sequence")
	}
}

// =============================================================================
// Feedback Tests
// =============================================================================

func TestCreateFeedback(t *testing.T) {
	db := newTestDB(t)

	project := &Project{ID: "proj-1", Name: "Project", PlanText: "Plan"}
	if err := db.CreateProject(project); err != nil {
		t.Fatalf("CreateProject() returned error: %v", err)
	}
	if err := db.CreateTask(&Task{ID: "task-1", ProjectID: "proj-1", Sequence: 1, Title: "T", Description: "D"}); err != nil {
		t.Fatalf("CreateTask() returned error: %v", err)
	}
	if err := db.CreateSession(&Session{ID: "sess-1", TaskID: "task-1", AgentType: AgentReviewer, Iteration: 1, InputPrompt: "P"}); err != nil {
		t.Fatalf("CreateSession() returned error: %v", err)
	}

	content := "Please fix the error handling"
	feedback := &Feedback{
		SessionID:    "sess-1",
		FeedbackType: FeedbackMajor,
		Content:      &content,
	}

	if err := db.CreateFeedback(feedback); err != nil {
		t.Fatalf("CreateFeedback() returned error: %v", err)
	}

	if feedback.ID == 0 {
		t.Error("CreateFeedback() did not set ID")
	}
}

func TestCreateFeedback_NilContent(t *testing.T) {
	db := newTestDB(t)

	project := &Project{ID: "proj-1", Name: "Project", PlanText: "Plan"}
	if err := db.CreateProject(project); err != nil {
		t.Fatalf("CreateProject() returned error: %v", err)
	}
	if err := db.CreateTask(&Task{ID: "task-1", ProjectID: "proj-1", Sequence: 1, Title: "T", Description: "D"}); err != nil {
		t.Fatalf("CreateTask() returned error: %v", err)
	}
	if err := db.CreateSession(&Session{ID: "sess-1", TaskID: "task-1", AgentType: AgentReviewer, Iteration: 1, InputPrompt: "P"}); err != nil {
		t.Fatalf("CreateSession() returned error: %v", err)
	}

	feedback := &Feedback{
		SessionID:    "sess-1",
		FeedbackType: FeedbackApproved,
		Content:      nil, // Approved feedback may have no content
	}

	if err := db.CreateFeedback(feedback); err != nil {
		t.Fatalf("CreateFeedback() with nil content returned error: %v", err)
	}
}

func TestGetFeedbackBySession(t *testing.T) {
	db := newTestDB(t)

	project := &Project{ID: "proj-1", Name: "Project", PlanText: "Plan"}
	if err := db.CreateProject(project); err != nil {
		t.Fatalf("CreateProject() returned error: %v", err)
	}
	if err := db.CreateTask(&Task{ID: "task-1", ProjectID: "proj-1", Sequence: 1, Title: "T", Description: "D"}); err != nil {
		t.Fatalf("CreateTask() returned error: %v", err)
	}
	if err := db.CreateSession(&Session{ID: "sess-1", TaskID: "task-1", AgentType: AgentReviewer, Iteration: 1, InputPrompt: "P"}); err != nil {
		t.Fatalf("CreateSession() returned error: %v", err)
	}

	content1 := "Issue 1"
	content2 := "Issue 2"
	if err := db.CreateFeedback(&Feedback{SessionID: "sess-1", FeedbackType: FeedbackMinor, Content: &content1}); err != nil {
		t.Fatalf("CreateFeedback() returned error: %v", err)
	}
	time.Sleep(10 * time.Millisecond)
	if err := db.CreateFeedback(&Feedback{SessionID: "sess-1", FeedbackType: FeedbackMajor, Content: &content2}); err != nil {
		t.Fatalf("CreateFeedback() returned error: %v", err)
	}

	feedbacks, err := db.GetFeedbackBySession("sess-1")
	if err != nil {
		t.Fatalf("GetFeedbackBySession() returned error: %v", err)
	}

	if len(feedbacks) != 2 {
		t.Errorf("GetFeedbackBySession() returned %d feedbacks, want 2", len(feedbacks))
	}
}

func TestGetLatestFeedbackForTask(t *testing.T) {
	db := newTestDB(t)

	project := &Project{ID: "proj-1", Name: "Project", PlanText: "Plan"}
	if err := db.CreateProject(project); err != nil {
		t.Fatalf("CreateProject() returned error: %v", err)
	}
	if err := db.CreateTask(&Task{ID: "task-1", ProjectID: "proj-1", Sequence: 1, Title: "T", Description: "D"}); err != nil {
		t.Fatalf("CreateTask() returned error: %v", err)
	}

	if err := db.CreateSession(&Session{ID: "sess-1", TaskID: "task-1", AgentType: AgentReviewer, Iteration: 1, InputPrompt: "P1"}); err != nil {
		t.Fatalf("CreateSession() returned error: %v", err)
	}
	if err := db.CreateSession(&Session{ID: "sess-2", TaskID: "task-1", AgentType: AgentReviewer, Iteration: 2, InputPrompt: "P2"}); err != nil {
		t.Fatalf("CreateSession() returned error: %v", err)
	}

	content1 := "Old feedback"
	content2 := "New feedback"
	if err := db.CreateFeedback(&Feedback{SessionID: "sess-1", FeedbackType: FeedbackMinor, Content: &content1}); err != nil {
		t.Fatalf("CreateFeedback() returned error: %v", err)
	}
	time.Sleep(10 * time.Millisecond)
	if err := db.CreateFeedback(&Feedback{SessionID: "sess-2", FeedbackType: FeedbackApproved, Content: &content2}); err != nil {
		t.Fatalf("CreateFeedback() returned error: %v", err)
	}

	feedback, err := db.GetLatestFeedbackForTask("task-1")
	if err != nil {
		t.Fatalf("GetLatestFeedbackForTask() returned error: %v", err)
	}

	if feedback.FeedbackType != FeedbackApproved {
		t.Errorf("GetLatestFeedbackForTask() returned %v, want FeedbackApproved", feedback.FeedbackType)
	}
}

func TestGetLatestFeedbackForTask_NotFound(t *testing.T) {
	db := newTestDB(t)

	project := &Project{ID: "proj-1", Name: "Project", PlanText: "Plan"}
	if err := db.CreateProject(project); err != nil {
		t.Fatalf("CreateProject() returned error: %v", err)
	}
	if err := db.CreateTask(&Task{ID: "task-1", ProjectID: "proj-1", Sequence: 1, Title: "T", Description: "D"}); err != nil {
		t.Fatalf("CreateTask() returned error: %v", err)
	}

	_, err := db.GetLatestFeedbackForTask("task-1")
	if !errors.Is(err, ErrNotFound) {
		t.Errorf("GetLatestFeedbackForTask() error = %v, want ErrNotFound", err)
	}
}

// =============================================================================
// Status Type Tests
// =============================================================================

func TestProjectStatusConstants(t *testing.T) {
	tests := []struct {
		status ProjectStatus
		want   string
	}{
		{ProjectPending, "pending"},
		{ProjectInProgress, "in_progress"},
		{ProjectCompleted, "completed"},
		{ProjectFailed, "failed"},
	}

	for _, tt := range tests {
		if string(tt.status) != tt.want {
			t.Errorf("ProjectStatus %v = %q, want %q", tt.status, string(tt.status), tt.want)
		}
	}
}

func TestTaskStatusConstants(t *testing.T) {
	tests := []struct {
		status TaskStatus
		want   string
	}{
		{TaskPending, "pending"},
		{TaskInProgress, "in_progress"},
		{TaskCompleted, "completed"},
		{TaskFailed, "failed"},
		{TaskEscalated, "escalated"},
	}

	for _, tt := range tests {
		if string(tt.status) != tt.want {
			t.Errorf("TaskStatus %v = %q, want %q", tt.status, string(tt.status), tt.want)
		}
	}
}

func TestSessionStatusConstants(t *testing.T) {
	tests := []struct {
		status SessionStatus
		want   string
	}{
		{SessionRunning, "running"},
		{SessionCompleted, "completed"},
		{SessionFailed, "failed"},
	}

	for _, tt := range tests {
		if string(tt.status) != tt.want {
			t.Errorf("SessionStatus %v = %q, want %q", tt.status, string(tt.status), tt.want)
		}
	}
}

func TestAgentTypeConstants(t *testing.T) {
	tests := []struct {
		agentType AgentType
		want      string
	}{
		{AgentDeveloper, "developer"},
		{AgentReviewer, "reviewer"},
		{AgentPlanner, "planner"},
	}

	for _, tt := range tests {
		if string(tt.agentType) != tt.want {
			t.Errorf("AgentType %v = %q, want %q", tt.agentType, string(tt.agentType), tt.want)
		}
	}
}

func TestFeedbackTypeConstants(t *testing.T) {
	tests := []struct {
		feedbackType FeedbackType
		want         string
	}{
		{FeedbackApproved, "approved"},
		{FeedbackMajor, "major"},
		{FeedbackMinor, "minor"},
		{FeedbackCritical, "critical"},
	}

	for _, tt := range tests {
		if string(tt.feedbackType) != tt.want {
			t.Errorf("FeedbackType %v = %q, want %q", tt.feedbackType, string(tt.feedbackType), tt.want)
		}
	}
}

func TestLearningsStateConstants(t *testing.T) {
	tests := []struct {
		state LearningsState
		want  string
	}{
		{LearningsStateNone, ""},
		{LearningsStateComplete, "complete"},
	}

	for _, tt := range tests {
		if string(tt.state) != tt.want {
			t.Errorf("LearningsState %v = %q, want %q", tt.state, string(tt.state), tt.want)
		}
	}
}

func TestUpdateProjectLearningsState(t *testing.T) {
	db := newTestDB(t)

	project := &Project{
		ID:       "proj-1",
		Name:     "My Project",
		PlanText: "Plan",
	}
	if err := db.CreateProject(project); err != nil {
		t.Fatalf("CreateProject() returned error: %v", err)
	}

	// Verify initial state is empty
	got, err := db.GetProject("proj-1")
	if err != nil {
		t.Fatalf("GetProject() returned error: %v", err)
	}
	if got.LearningsState != LearningsStateNone {
		t.Errorf("Initial LearningsState = %v, want %v", got.LearningsState, LearningsStateNone)
	}

	// Update to complete
	if err := db.UpdateProjectLearningsState("proj-1", LearningsStateComplete); err != nil {
		t.Fatalf("UpdateProjectLearningsState() returned error: %v", err)
	}

	got, err = db.GetProject("proj-1")
	if err != nil {
		t.Fatalf("GetProject() returned error: %v", err)
	}
	if got.LearningsState != LearningsStateComplete {
		t.Errorf("UpdateProjectLearningsState() state = %v, want %v", got.LearningsState, LearningsStateComplete)
	}
}

func TestUpdateProjectLearningsState_NotFound(t *testing.T) {
	db := newTestDB(t)

	err := db.UpdateProjectLearningsState("nonexistent", LearningsStateComplete)
	if !errors.Is(err, ErrNotFound) {
		t.Errorf("UpdateProjectLearningsState() error = %v, want ErrNotFound", err)
	}
}

func TestProjectLearningsStateInListProjects(t *testing.T) {
	db := newTestDB(t)

	project := &Project{
		ID:             "proj-1",
		Name:           "My Project",
		PlanText:       "Plan",
		LearningsState: LearningsStateComplete,
	}
	if err := db.CreateProject(project); err != nil {
		t.Fatalf("CreateProject() returned error: %v", err)
	}

	projects, err := db.ListProjects()
	if err != nil {
		t.Fatalf("ListProjects() returned error: %v", err)
	}

	if len(projects) != 1 {
		t.Fatalf("ListProjects() returned %d projects, want 1", len(projects))
	}

	if projects[0].LearningsState != LearningsStateComplete {
		t.Errorf("ListProjects()[0].LearningsState = %v, want %v", projects[0].LearningsState, LearningsStateComplete)
	}
}

// =============================================================================
// Task Export/Import Method Tests
// =============================================================================

func TestGetTaskBySequence(t *testing.T) {
	db := newTestDB(t)

	project := &Project{ID: "proj-1", Name: "Project", PlanText: "Plan"}
	if err := db.CreateProject(project); err != nil {
		t.Fatalf("CreateProject() returned error: %v", err)
	}

	if err := db.CreateTask(&Task{ID: "task-1", ProjectID: "proj-1", Sequence: 1, Title: "First", Description: "Desc 1"}); err != nil {
		t.Fatalf("CreateTask() returned error: %v", err)
	}
	if err := db.CreateTask(&Task{ID: "task-2", ProjectID: "proj-1", Sequence: 2, Title: "Second", Description: "Desc 2"}); err != nil {
		t.Fatalf("CreateTask() returned error: %v", err)
	}
	if err := db.CreateTask(&Task{ID: "task-3", ProjectID: "proj-1", Sequence: 3, Title: "Third", Description: "Desc 3"}); err != nil {
		t.Fatalf("CreateTask() returned error: %v", err)
	}

	got, err := db.GetTaskBySequence("proj-1", 2)
	if err != nil {
		t.Fatalf("GetTaskBySequence() returned error: %v", err)
	}

	if got.ID != "task-2" {
		t.Errorf("GetTaskBySequence().ID = %v, want task-2", got.ID)
	}
	if got.Title != "Second" {
		t.Errorf("GetTaskBySequence().Title = %v, want Second", got.Title)
	}
	if got.Description != "Desc 2" {
		t.Errorf("GetTaskBySequence().Description = %v, want Desc 2", got.Description)
	}
}

func TestGetTaskBySequence_NotFound(t *testing.T) {
	db := newTestDB(t)

	project := &Project{ID: "proj-1", Name: "Project", PlanText: "Plan"}
	if err := db.CreateProject(project); err != nil {
		t.Fatalf("CreateProject() returned error: %v", err)
	}

	if err := db.CreateTask(&Task{ID: "task-1", ProjectID: "proj-1", Sequence: 1, Title: "First", Description: "Desc"}); err != nil {
		t.Fatalf("CreateTask() returned error: %v", err)
	}

	_, err := db.GetTaskBySequence("proj-1", 99)
	if !errors.Is(err, ErrNotFound) {
		t.Errorf("GetTaskBySequence() error = %v, want ErrNotFound", err)
	}
}

func TestGetTaskBySequence_WrongProject(t *testing.T) {
	db := newTestDB(t)

	project1 := &Project{ID: "proj-1", Name: "Project 1", PlanText: "Plan"}
	project2 := &Project{ID: "proj-2", Name: "Project 2", PlanText: "Plan"}
	if err := db.CreateProject(project1); err != nil {
		t.Fatalf("CreateProject() returned error: %v", err)
	}
	if err := db.CreateProject(project2); err != nil {
		t.Fatalf("CreateProject() returned error: %v", err)
	}

	if err := db.CreateTask(&Task{ID: "task-1", ProjectID: "proj-1", Sequence: 1, Title: "Task", Description: "Desc"}); err != nil {
		t.Fatalf("CreateTask() returned error: %v", err)
	}

	// Task exists in proj-1 but not proj-2
	_, err := db.GetTaskBySequence("proj-2", 1)
	if !errors.Is(err, ErrNotFound) {
		t.Errorf("GetTaskBySequence() error = %v, want ErrNotFound", err)
	}
}

func TestUpdateTaskDescription(t *testing.T) {
	db := newTestDB(t)

	project := &Project{ID: "proj-1", Name: "Project", PlanText: "Plan"}
	if err := db.CreateProject(project); err != nil {
		t.Fatalf("CreateProject() returned error: %v", err)
	}
	if err := db.CreateTask(&Task{ID: "task-1", ProjectID: "proj-1", Sequence: 1, Title: "Task", Description: "Original description"}); err != nil {
		t.Fatalf("CreateTask() returned error: %v", err)
	}

	newDesc := "Updated description with more details"
	if err := db.UpdateTaskDescription("task-1", newDesc); err != nil {
		t.Fatalf("UpdateTaskDescription() returned error: %v", err)
	}

	got, err := db.GetTask("task-1")
	if err != nil {
		t.Fatalf("GetTask() returned error: %v", err)
	}
	if got.Description != newDesc {
		t.Errorf("UpdateTaskDescription() description = %v, want %v", got.Description, newDesc)
	}
}

func TestUpdateTaskDescription_NotFound(t *testing.T) {
	db := newTestDB(t)

	err := db.UpdateTaskDescription("nonexistent", "new description")
	if !errors.Is(err, ErrNotFound) {
		t.Errorf("UpdateTaskDescription() error = %v, want ErrNotFound", err)
	}
}

func TestUpdateTaskDescription_UpdatesTimestamp(t *testing.T) {
	db := newTestDB(t)

	project := &Project{ID: "proj-1", Name: "Project", PlanText: "Plan"}
	if err := db.CreateProject(project); err != nil {
		t.Fatalf("CreateProject() returned error: %v", err)
	}
	if err := db.CreateTask(&Task{ID: "task-1", ProjectID: "proj-1", Sequence: 1, Title: "Task", Description: "Original"}); err != nil {
		t.Fatalf("CreateTask() returned error: %v", err)
	}

	original, err := db.GetTask("task-1")
	if err != nil {
		t.Fatalf("GetTask() returned error: %v", err)
	}
	originalUpdatedAt := original.UpdatedAt

	time.Sleep(10 * time.Millisecond)

	if err := db.UpdateTaskDescription("task-1", "New description"); err != nil {
		t.Fatalf("UpdateTaskDescription() returned error: %v", err)
	}

	updated, err := db.GetTask("task-1")
	if err != nil {
		t.Fatalf("GetTask() returned error: %v", err)
	}
	if !updated.UpdatedAt.After(originalUpdatedAt) {
		t.Error("UpdateTaskDescription() did not update UpdatedAt timestamp")
	}
}
