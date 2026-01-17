package tui

import (
	"errors"
	"strings"
	"testing"
	"time"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/gerund/ralph/internal/db"
	"github.com/gerund/ralph/internal/engine"
)

func TestNewTaskProgressModel(t *testing.T) {
	database, err := db.New(":memory:")
	if err != nil {
		t.Fatalf("Failed to create database: %v", err)
	}
	defer database.Close()

	project := &db.Project{
		ID:   "p1",
		Name: "Test Project",
	}

	events := make(chan engine.EngineEvent)
	m := NewTaskProgressModel(project, database, events)

	if m.project != project {
		t.Error("Expected project to be set")
	}
	if m.db != database {
		t.Error("Expected database to be set")
	}
	if !m.autoScroll {
		t.Error("Expected autoScroll to be true initially")
	}
	if m.completed {
		t.Error("Expected completed to be false initially")
	}
}

func TestNewTaskProgressModelWithError(t *testing.T) {
	project := &db.Project{
		ID:   "p1",
		Name: "Test Project",
	}

	testErr := errors.New("test error")
	m := NewTaskProgressModelWithError(project, testErr)

	if m.project != project {
		t.Error("Expected project to be set")
	}
	if m.err != testErr {
		t.Error("Expected error to be set")
	}
	if !m.autoScroll {
		t.Error("Expected autoScroll to be true initially")
	}
	if m.db != nil {
		t.Error("Expected database to be nil")
	}
	if m.engineEvents != nil {
		t.Error("Expected engineEvents to be nil")
	}
}

func TestTaskProgressModel_Init(t *testing.T) {
	database, err := db.New(":memory:")
	if err != nil {
		t.Fatalf("Failed to create database: %v", err)
	}
	defer database.Close()

	project := &db.Project{
		ID:   "p1",
		Name: "Test Project",
	}

	events := make(chan engine.EngineEvent, 1)
	m := NewTaskProgressModel(project, database, events)

	cmd := m.Init()
	if cmd == nil {
		t.Fatal("Expected Init to return a command")
	}
}

func TestTaskProgressModel_Update_WindowSize(t *testing.T) {
	database, err := db.New(":memory:")
	if err != nil {
		t.Fatalf("Failed to create database: %v", err)
	}
	defer database.Close()

	project := &db.Project{
		ID:   "p1",
		Name: "Test Project",
	}

	events := make(chan engine.EngineEvent)
	m := NewTaskProgressModel(project, database, events)
	m, _ = m.Update(tea.WindowSizeMsg{Width: 100, Height: 50})

	if m.width != 100 {
		t.Errorf("Expected width 100, got %d", m.width)
	}
	if m.height != 50 {
		t.Errorf("Expected height 50, got %d", m.height)
	}
}

func TestTaskProgressModel_Update_TasksLoaded(t *testing.T) {
	database, err := db.New(":memory:")
	if err != nil {
		t.Fatalf("Failed to create database: %v", err)
	}
	defer database.Close()

	project := &db.Project{
		ID:   "p1",
		Name: "Test Project",
	}

	tasks := []*db.Task{
		{ID: "t1", Title: "Task 1", Status: db.TaskPending},
		{ID: "t2", Title: "Task 2", Status: db.TaskPending},
	}

	events := make(chan engine.EngineEvent)
	m := NewTaskProgressModel(project, database, events)
	m, _ = m.Update(TasksLoadedMsg{Tasks: tasks})

	if len(m.Tasks()) != 2 {
		t.Errorf("Expected 2 tasks, got %d", len(m.Tasks()))
	}
}

func TestTaskProgressModel_Update_TasksLoadedError(t *testing.T) {
	database, err := db.New(":memory:")
	if err != nil {
		t.Fatalf("Failed to create database: %v", err)
	}
	defer database.Close()

	project := &db.Project{
		ID:   "p1",
		Name: "Test Project",
	}

	events := make(chan engine.EngineEvent)
	m := NewTaskProgressModel(project, database, events)
	m, _ = m.Update(TasksLoadedMsg{Err: db.ErrNotFound})

	if m.Error() == nil {
		t.Error("Expected error to be set")
	}
}

func TestTaskProgressModel_Update_ScrollNavigation(t *testing.T) {
	database, err := db.New(":memory:")
	if err != nil {
		t.Fatalf("Failed to create database: %v", err)
	}
	defer database.Close()

	project := &db.Project{
		ID:   "p1",
		Name: "Test Project",
	}

	events := make(chan engine.EngineEvent)
	m := NewTaskProgressModel(project, database, events)
	m, _ = m.Update(tea.WindowSizeMsg{Width: 100, Height: 50})

	// Initial state should have autoScroll enabled
	if !m.autoScroll {
		t.Error("Expected autoScroll to be true initially")
	}

	// Test scroll up with 'k' - should disable autoScroll
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'k'}})
	if m.autoScroll {
		t.Error("Expected autoScroll to be false after pressing k")
	}

	// Test 'G' to go to bottom (re-enable autoScroll)
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'G'}})
	if !m.autoScroll {
		t.Error("Expected autoScroll to be true after pressing G")
	}

	// Test 'g' goes to top - should disable autoScroll
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'g'}})
	if m.autoScroll {
		t.Error("Expected autoScroll to be false after pressing g")
	}

	// Test pgup - should disable autoScroll
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyPgUp})
	if m.autoScroll {
		t.Error("Expected autoScroll to be false after pgup")
	}
}

func TestTaskProgressModel_HandleEngineEvent_Completed(t *testing.T) {
	database, err := db.New(":memory:")
	if err != nil {
		t.Fatalf("Failed to create database: %v", err)
	}
	defer database.Close()

	project := &db.Project{
		ID:   "p1",
		Name: "Test Project",
	}

	events := make(chan engine.EngineEvent, 1)
	m := NewTaskProgressModel(project, database, events)
	m, _ = m.Update(tea.WindowSizeMsg{Width: 100, Height: 50})

	// Simulate engine event
	m.handleEngineEvent(engine.EngineEvent{
		Type:    engine.EngineEventCompleted,
		Message: "All done",
	})

	if !m.IsCompleted() {
		t.Error("Expected completed to be true")
	}
	if !strings.Contains(m.Output(), "completed") {
		t.Error("Expected output to contain 'completed'")
	}
}

func TestTaskProgressModel_Update_EngineEventsClosed(t *testing.T) {
	database, err := db.New(":memory:")
	if err != nil {
		t.Fatalf("Failed to create database: %v", err)
	}
	defer database.Close()

	project := &db.Project{
		ID:   "p1",
		Name: "Test Project",
	}

	events := make(chan engine.EngineEvent)
	m := NewTaskProgressModel(project, database, events)
	m, _ = m.Update(tea.WindowSizeMsg{Width: 100, Height: 50})

	// Simulate channel closed message
	m, cmd := m.Update(EngineEventsClosedMsg{})

	if !m.IsCompleted() {
		t.Error("Expected completed to be true after channel closed")
	}
	if !strings.Contains(m.Output(), "finished") {
		t.Error("Expected output to contain 'finished'")
	}
	if cmd != nil {
		t.Error("Expected no command after channel closed")
	}
}

func TestTaskProgressModel_Update_EngineEventsClosed_WithError(t *testing.T) {
	project := &db.Project{
		ID:   "p1",
		Name: "Test Project",
	}

	// Create model with pre-existing error
	m := NewTaskProgressModelWithError(project, errors.New("existing error"))
	m, _ = m.Update(tea.WindowSizeMsg{Width: 100, Height: 50})

	// Simulate channel closed message - should not overwrite error state
	m, _ = m.Update(EngineEventsClosedMsg{})

	// Should not be marked completed because there was an error
	if m.IsCompleted() {
		t.Error("Expected completed to be false when error exists")
	}
	if m.Error() == nil {
		t.Error("Expected error to still be set")
	}
}

func TestTaskProgressModel_HandleEngineEvent_Failed(t *testing.T) {
	database, err := db.New(":memory:")
	if err != nil {
		t.Fatalf("Failed to create database: %v", err)
	}
	defer database.Close()

	project := &db.Project{
		ID:   "p1",
		Name: "Test Project",
	}

	events := make(chan engine.EngineEvent, 1)
	m := NewTaskProgressModel(project, database, events)
	m, _ = m.Update(tea.WindowSizeMsg{Width: 100, Height: 50})

	// Simulate engine event
	m.handleEngineEvent(engine.EngineEvent{
		Type:    engine.EngineEventFailed,
		Message: "Something went wrong",
	})

	if m.Error() == nil {
		t.Error("Expected error to be set")
	}
	if !strings.Contains(m.Output(), "Failed") {
		t.Error("Expected output to contain 'Failed'")
	}
}

func TestTaskProgressModel_HandleTaskLoopEvent(t *testing.T) {
	database, err := db.New(":memory:")
	if err != nil {
		t.Fatalf("Failed to create database: %v", err)
	}
	defer database.Close()

	project := &db.Project{
		ID:   "p1",
		Name: "Test Project",
	}

	tasks := []*db.Task{
		{ID: "t1", Title: "Task 1", Status: db.TaskPending},
		{ID: "t2", Title: "Task 2", Status: db.TaskPending},
	}

	events := make(chan engine.EngineEvent)
	m := NewTaskProgressModel(project, database, events)
	m, _ = m.Update(TasksLoadedMsg{Tasks: tasks})
	m, _ = m.Update(tea.WindowSizeMsg{Width: 100, Height: 50})

	// Simulate task begin
	m.handleTaskLoopEvent(engine.TaskLoopEvent{
		Type:      engine.TaskEventTaskBegin,
		TaskIndex: 0,
		TaskTitle: "Task 1",
	})

	if m.CurrentIndex() != 0 {
		t.Errorf("Expected currentIdx 0, got %d", m.CurrentIndex())
	}
	if m.tasks[0].Status != db.TaskInProgress {
		t.Error("Expected task status to be in_progress")
	}

	// Simulate task end
	m.handleTaskLoopEvent(engine.TaskLoopEvent{
		Type:      engine.TaskEventTaskEnd,
		TaskIndex: 0,
		TaskTitle: "Task 1",
	})

	if m.tasks[0].Status != db.TaskCompleted {
		t.Error("Expected task status to be completed")
	}
}

func TestTaskProgressModel_HandleImplEvent(t *testing.T) {
	database, err := db.New(":memory:")
	if err != nil {
		t.Fatalf("Failed to create database: %v", err)
	}
	defer database.Close()

	project := &db.Project{
		ID:   "p1",
		Name: "Test Project",
	}

	events := make(chan engine.EngineEvent)
	m := NewTaskProgressModel(project, database, events)
	m, _ = m.Update(tea.WindowSizeMsg{Width: 100, Height: 50})

	// Simulate developing event
	m.handleImplEvent(engine.ImplLoopEvent{
		Type:      engine.EventDeveloping,
		Iteration: 1,
	})

	if m.AgentType() != "developer" {
		t.Errorf("Expected agentType 'developer', got %s", m.AgentType())
	}
	if m.Iteration() != 1 {
		t.Errorf("Expected iteration 1, got %d", m.Iteration())
	}

	// Simulate reviewing event
	m.handleImplEvent(engine.ImplLoopEvent{
		Type:      engine.EventReviewing,
		Iteration: 1,
	})

	if m.AgentType() != "reviewer" {
		t.Errorf("Expected agentType 'reviewer', got %s", m.AgentType())
	}

	// Simulate approved event
	m.handleImplEvent(engine.ImplLoopEvent{
		Type: engine.EventApproved,
	})

	if !strings.Contains(m.Output(), "Approved") {
		t.Error("Expected output to contain 'Approved'")
	}
}

func TestTaskProgressModel_View_Empty(t *testing.T) {
	database, err := db.New(":memory:")
	if err != nil {
		t.Fatalf("Failed to create database: %v", err)
	}
	defer database.Close()

	project := &db.Project{
		ID:   "p1",
		Name: "Test Project",
	}

	events := make(chan engine.EngineEvent)
	m := NewTaskProgressModel(project, database, events)
	m, _ = m.Update(tea.WindowSizeMsg{Width: 100, Height: 50})

	view := m.View()

	// Check header
	if !strings.Contains(view, "Ralph") {
		t.Error("Expected view to contain 'Ralph'")
	}
	if !strings.Contains(view, "Test Project") {
		t.Error("Expected view to contain project name")
	}

	// Check empty state
	if !strings.Contains(view, "No tasks yet") {
		t.Error("Expected view to contain empty state message")
	}

	// Check footer
	if !strings.Contains(view, "j/k") || !strings.Contains(view, "scroll") {
		t.Error("Expected view to contain help text")
	}
}

func TestTaskProgressModel_View_WithTasks(t *testing.T) {
	database, err := db.New(":memory:")
	if err != nil {
		t.Fatalf("Failed to create database: %v", err)
	}
	defer database.Close()

	project := &db.Project{
		ID:   "p1",
		Name: "Test Project",
	}

	tasks := []*db.Task{
		{ID: "t1", Title: "First Task", Status: db.TaskCompleted},
		{ID: "t2", Title: "Second Task", Status: db.TaskInProgress},
		{ID: "t3", Title: "Third Task", Status: db.TaskPending},
	}

	events := make(chan engine.EngineEvent)
	m := NewTaskProgressModel(project, database, events)
	m, _ = m.Update(TasksLoadedMsg{Tasks: tasks})
	m, _ = m.Update(tea.WindowSizeMsg{Width: 100, Height: 50})

	view := m.View()

	// Check progress count
	if !strings.Contains(view, "1/3") {
		t.Error("Expected view to contain progress '1/3'")
	}

	// Check task titles appear
	if !strings.Contains(view, "First Task") {
		t.Error("Expected view to contain 'First Task'")
	}
	if !strings.Contains(view, "Second Task") {
		t.Error("Expected view to contain 'Second Task'")
	}
	if !strings.Contains(view, "Third Task") {
		t.Error("Expected view to contain 'Third Task'")
	}
}

func TestTaskProgressModel_View_Error(t *testing.T) {
	database, err := db.New(":memory:")
	if err != nil {
		t.Fatalf("Failed to create database: %v", err)
	}
	defer database.Close()

	project := &db.Project{
		ID:   "p1",
		Name: "Test Project",
	}

	events := make(chan engine.EngineEvent)
	m := NewTaskProgressModel(project, database, events)
	m, _ = m.Update(TasksLoadedMsg{Err: db.ErrNotFound})
	m, _ = m.Update(tea.WindowSizeMsg{Width: 100, Height: 50})

	view := m.View()

	if !strings.Contains(view, "Error") {
		t.Error("Expected view to contain 'Error'")
	}
	if !strings.Contains(view, "quit") {
		t.Error("Expected view to contain quit instruction")
	}
}

func TestTaskProgressModel_View_Completed(t *testing.T) {
	database, err := db.New(":memory:")
	if err != nil {
		t.Fatalf("Failed to create database: %v", err)
	}
	defer database.Close()

	project := &db.Project{
		ID:   "p1",
		Name: "Test Project",
	}

	events := make(chan engine.EngineEvent)
	m := NewTaskProgressModel(project, database, events)
	m, _ = m.Update(tea.WindowSizeMsg{Width: 100, Height: 50})

	m.handleEngineEvent(engine.EngineEvent{
		Type:    engine.EngineEventCompleted,
		Message: "All done",
	})

	view := m.View()

	if !strings.Contains(view, "Completed") {
		t.Error("Expected view to contain 'Completed'")
	}
}

func TestTaskProgressModel_CountCompleted(t *testing.T) {
	database, err := db.New(":memory:")
	if err != nil {
		t.Fatalf("Failed to create database: %v", err)
	}
	defer database.Close()

	project := &db.Project{
		ID:   "p1",
		Name: "Test Project",
	}

	tasks := []*db.Task{
		{ID: "t1", Title: "Task 1", Status: db.TaskCompleted},
		{ID: "t2", Title: "Task 2", Status: db.TaskInProgress},
		{ID: "t3", Title: "Task 3", Status: db.TaskCompleted},
		{ID: "t4", Title: "Task 4", Status: db.TaskPending},
	}

	events := make(chan engine.EngineEvent)
	m := NewTaskProgressModel(project, database, events)
	m, _ = m.Update(TasksLoadedMsg{Tasks: tasks})

	count := m.countCompleted()
	if count != 2 {
		t.Errorf("Expected 2 completed tasks, got %d", count)
	}
}

func TestTaskProgressModel_TaskStatusIcon(t *testing.T) {
	database, err := db.New(":memory:")
	if err != nil {
		t.Fatalf("Failed to create database: %v", err)
	}
	defer database.Close()

	project := &db.Project{
		ID:   "p1",
		Name: "Test Project",
	}

	events := make(chan engine.EngineEvent)
	m := NewTaskProgressModel(project, database, events)

	tests := []struct {
		status   db.TaskStatus
		expected string
	}{
		{db.TaskPending, "○"},
		{db.TaskInProgress, "◐"},
		{db.TaskCompleted, "●"},
		{db.TaskFailed, "✗"},
		{db.TaskEscalated, "!"},
	}

	for _, tt := range tests {
		icon := m.taskStatusIcon(tt.status)
		if !strings.Contains(icon, tt.expected) {
			t.Errorf("Expected status icon to contain %s for %s, got %s", tt.expected, tt.status, icon)
		}
	}
}

func TestTruncate(t *testing.T) {
	tests := []struct {
		input    string
		length   int
		expected string
	}{
		{"short", 10, "short"},
		{"exactly10c", 10, "exactly10c"},
		{"this is a longer string", 10, "this is..."},
		{"ab", 5, "ab"},
		{"abc", 3, "abc"},
		{"abcd", 3, "abc"}, // length <= 3, so no ellipsis
		// Unicode test cases - CJK characters are 2-width each
		{"日本語テスト", 10, "日本語..."},         // 6 width + 3 ellipsis = 9 (テ would make 11)
		{"こんにちは", 10, "こんにちは"},           // Exactly 10 width
		{"hello世界", 10, "hello世界"},       // 5 + 4 = 9 width, fits
		{"hello世界test", 10, "hello世..."}, // 5 + 2 + 3 = 10 width
	}

	for _, tt := range tests {
		result := truncate(tt.input, tt.length)
		if result != tt.expected {
			t.Errorf("truncate(%q, %d) = %q, expected %q", tt.input, tt.length, result, tt.expected)
		}
	}
}

func TestTaskProgressModel_NilProject(t *testing.T) {
	events := make(chan engine.EngineEvent)
	m := NewTaskProgressModel(nil, nil, events)

	view := m.View()
	if !strings.Contains(view, "Unknown") {
		t.Error("Expected view to handle nil project")
	}
}

func TestTaskProgressModel_Update_EngineEvent(t *testing.T) {
	database, err := db.New(":memory:")
	if err != nil {
		t.Fatalf("Failed to create database: %v", err)
	}
	defer database.Close()

	project := &db.Project{
		ID:   "p1",
		Name: "Test Project",
	}

	events := make(chan engine.EngineEvent, 1)
	m := NewTaskProgressModel(project, database, events)
	m, _ = m.Update(tea.WindowSizeMsg{Width: 100, Height: 50})

	// Send an event and update
	_, cmd := m.Update(EngineEventMsg{
		Event: engine.EngineEvent{
			Type:    engine.EngineEventRunning,
			Message: "Running",
		},
	})

	// Should return a command to listen for more events
	if cmd == nil {
		t.Error("Expected command to continue listening")
	}
}

func TestTaskProgressModel_Accessors(t *testing.T) {
	database, err := db.New(":memory:")
	if err != nil {
		t.Fatalf("Failed to create database: %v", err)
	}
	defer database.Close()

	now := time.Now()
	project := &db.Project{
		ID:        "p1",
		Name:      "Test Project",
		UpdatedAt: now,
	}

	tasks := []*db.Task{
		{ID: "t1", Title: "Task 1", Status: db.TaskPending},
	}

	events := make(chan engine.EngineEvent)
	m := NewTaskProgressModel(project, database, events)
	m, _ = m.Update(TasksLoadedMsg{Tasks: tasks})

	if m.Project() != project {
		t.Error("Expected Project() to return project")
	}
	if len(m.Tasks()) != 1 {
		t.Error("Expected Tasks() to return tasks")
	}
	if m.CurrentIndex() != 0 {
		t.Error("Expected CurrentIndex() to return 0")
	}
	if m.Iteration() != 0 {
		t.Error("Expected Iteration() to return 0")
	}
	if m.AgentType() != "" {
		t.Error("Expected AgentType() to return empty string")
	}
	if m.IsCompleted() {
		t.Error("Expected IsCompleted() to return false")
	}
	if m.Error() != nil {
		t.Error("Expected Error() to return nil")
	}
}

func TestTaskProgressModel_PauseModeInitialState(t *testing.T) {
	database, err := db.New(":memory:")
	if err != nil {
		t.Fatalf("Failed to create database: %v", err)
	}
	defer database.Close()

	project := &db.Project{
		ID:   "p1",
		Name: "Test Project",
	}

	events := make(chan engine.EngineEvent)
	m := NewTaskProgressModel(project, database, events)

	// Initially pause mode should be disabled
	if m.IsPauseMode() {
		t.Error("Expected pause mode to be disabled initially")
	}
	if m.IsPaused() {
		t.Error("Expected not paused initially")
	}
}

func TestTaskProgressModel_PauseModeToggle(t *testing.T) {
	database, err := db.New(":memory:")
	if err != nil {
		t.Fatalf("Failed to create database: %v", err)
	}
	defer database.Close()

	project := &db.Project{
		ID:   "p1",
		Name: "Test Project",
	}

	events := make(chan engine.EngineEvent)
	m := NewTaskProgressModel(project, database, events)
	m, _ = m.Update(tea.WindowSizeMsg{Width: 100, Height: 50})

	// Press 'p' to toggle pause mode
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'p'}})

	if !m.IsPauseMode() {
		t.Error("Expected pause mode to be enabled after pressing 'p'")
	}

	// Press 'p' again to toggle off
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'p'}})

	if m.IsPauseMode() {
		t.Error("Expected pause mode to be disabled after pressing 'p' again")
	}
}

func TestTaskProgressModel_HandlePausedEvent(t *testing.T) {
	database, err := db.New(":memory:")
	if err != nil {
		t.Fatalf("Failed to create database: %v", err)
	}
	defer database.Close()

	project := &db.Project{
		ID:   "p1",
		Name: "Test Project",
	}

	events := make(chan engine.EngineEvent)
	m := NewTaskProgressModel(project, database, events)
	m, _ = m.Update(tea.WindowSizeMsg{Width: 100, Height: 50})

	// Simulate paused event
	m.handleTaskLoopEvent(engine.TaskLoopEvent{
		Type:      engine.TaskEventPaused,
		TaskIndex: 0,
		TaskTitle: "Task 1",
		Message:   "Waiting for confirmation",
	})

	if !m.IsPaused() {
		t.Error("Expected isPaused to be true after paused event")
	}
	if !strings.Contains(m.Output(), "PAUSED") {
		t.Error("Expected output to contain PAUSED")
	}
}

func TestTaskProgressModel_HandleResumedEvent(t *testing.T) {
	database, err := db.New(":memory:")
	if err != nil {
		t.Fatalf("Failed to create database: %v", err)
	}
	defer database.Close()

	project := &db.Project{
		ID:   "p1",
		Name: "Test Project",
	}

	events := make(chan engine.EngineEvent)
	m := NewTaskProgressModel(project, database, events)
	m, _ = m.Update(tea.WindowSizeMsg{Width: 100, Height: 50})

	// First set paused state
	m.handleTaskLoopEvent(engine.TaskLoopEvent{
		Type:    engine.TaskEventPaused,
		Message: "Waiting",
	})

	// Then resume
	m.handleTaskLoopEvent(engine.TaskLoopEvent{
		Type:    engine.TaskEventResumed,
		Message: "Continuing",
	})

	if m.IsPaused() {
		t.Error("Expected isPaused to be false after resumed event")
	}
	if !strings.Contains(m.Output(), "Resuming") {
		t.Error("Expected output to contain 'Resuming'")
	}
}

func TestTaskProgressModel_HandlePauseModeChangedEvent(t *testing.T) {
	database, err := db.New(":memory:")
	if err != nil {
		t.Fatalf("Failed to create database: %v", err)
	}
	defer database.Close()

	project := &db.Project{
		ID:   "p1",
		Name: "Test Project",
	}

	events := make(chan engine.EngineEvent)
	m := NewTaskProgressModel(project, database, events)
	m, _ = m.Update(tea.WindowSizeMsg{Width: 100, Height: 50})

	// Simulate pause mode enabled event
	pauseEnabled := true
	m.handleTaskLoopEvent(engine.TaskLoopEvent{
		Type:             engine.TaskEventPauseModeChanged,
		Message:          "Pause mode: true",
		PauseModeEnabled: &pauseEnabled,
	})

	if !m.IsPauseMode() {
		t.Error("Expected pauseMode to be true after event")
	}
	if !strings.Contains(m.Output(), "Pause mode enabled") {
		t.Error("Expected output to contain 'Pause mode enabled'")
	}

	// Simulate pause mode disabled event
	pauseDisabled := false
	m.handleTaskLoopEvent(engine.TaskLoopEvent{
		Type:             engine.TaskEventPauseModeChanged,
		Message:          "Pause mode: false",
		PauseModeEnabled: &pauseDisabled,
	})

	if m.IsPauseMode() {
		t.Error("Expected pauseMode to be false after event")
	}
	if !strings.Contains(m.Output(), "Continuous mode enabled") {
		t.Error("Expected output to contain 'Continuous mode enabled'")
	}
}

func TestTaskProgressModel_Footer_PauseMode(t *testing.T) {
	database, err := db.New(":memory:")
	if err != nil {
		t.Fatalf("Failed to create database: %v", err)
	}
	defer database.Close()

	project := &db.Project{
		ID:   "p1",
		Name: "Test Project",
	}

	events := make(chan engine.EngineEvent)
	m := NewTaskProgressModel(project, database, events)
	m, _ = m.Update(tea.WindowSizeMsg{Width: 100, Height: 50})

	// Enable pause mode
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'p'}})

	view := m.View()

	// Check that pause mode indicator is shown
	if !strings.Contains(view, "PAUSE") {
		t.Error("Expected view to contain PAUSE indicator when pause mode enabled")
	}
	// Check help text shows continuous option
	if !strings.Contains(view, "p: continuous") {
		t.Error("Expected help text to show 'p: continuous' when in pause mode")
	}
}

func TestTaskProgressModel_Footer_WhenPaused(t *testing.T) {
	database, err := db.New(":memory:")
	if err != nil {
		t.Fatalf("Failed to create database: %v", err)
	}
	defer database.Close()

	project := &db.Project{
		ID:   "p1",
		Name: "Test Project",
	}

	events := make(chan engine.EngineEvent)
	m := NewTaskProgressModel(project, database, events)
	m, _ = m.Update(tea.WindowSizeMsg{Width: 100, Height: 50})

	// Enable pause mode and simulate paused state
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'p'}})
	m.handleTaskLoopEvent(engine.TaskLoopEvent{
		Type:    engine.TaskEventPaused,
		Message: "Waiting",
	})

	view := m.View()

	// Check that paused prompt is shown
	if !strings.Contains(view, "Task complete") {
		t.Error("Expected view to contain 'Task complete' when paused")
	}
	if !strings.Contains(view, "Enter to continue") {
		t.Error("Expected view to mention Enter to continue")
	}
}

func TestTaskProgressModel_PauseAccessors(t *testing.T) {
	database, err := db.New(":memory:")
	if err != nil {
		t.Fatalf("Failed to create database: %v", err)
	}
	defer database.Close()

	project := &db.Project{
		ID:   "p1",
		Name: "Test Project",
	}

	events := make(chan engine.EngineEvent)
	m := NewTaskProgressModel(project, database, events)

	// Test initial values
	if m.IsPauseMode() {
		t.Error("Expected IsPauseMode() to return false initially")
	}
	if m.IsPaused() {
		t.Error("Expected IsPaused() to return false initially")
	}
}
