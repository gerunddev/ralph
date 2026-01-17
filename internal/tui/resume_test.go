package tui

import (
	"testing"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/gerund/ralph/internal/db"
	"github.com/gerund/ralph/internal/engine"
)

func TestNewResumeModel(t *testing.T) {
	state := &engine.ProjectState{
		Project:        &db.Project{ID: "test-id", Name: "Test Project"},
		CompletedTasks: 2,
		PendingTasks:   3,
		FailedTasks:    1,
	}

	model := NewResumeModel(state)

	if model.state != state {
		t.Error("expected state to be set")
	}
	if model.action != ResumeActionNone {
		t.Errorf("expected action to be None, got %d", model.action)
	}
}

func TestResumeModel_Init(t *testing.T) {
	state := &engine.ProjectState{
		Project: &db.Project{ID: "test-id", Name: "Test Project"},
	}
	model := NewResumeModel(state)

	cmd := model.Init()
	if cmd != nil {
		t.Error("expected nil command from Init")
	}
}

func TestResumeModel_Update_WindowSize(t *testing.T) {
	state := &engine.ProjectState{
		Project: &db.Project{ID: "test-id", Name: "Test Project"},
	}
	model := NewResumeModel(state)

	newModel, _ := model.Update(tea.WindowSizeMsg{Width: 100, Height: 50})

	if newModel.width != 100 {
		t.Errorf("expected width 100, got %d", newModel.width)
	}
	if newModel.height != 50 {
		t.Errorf("expected height 50, got %d", newModel.height)
	}
}

func TestResumeModel_Update_Enter(t *testing.T) {
	state := &engine.ProjectState{
		Project: &db.Project{ID: "test-id", Name: "Test Project"},
	}
	model := NewResumeModel(state)

	newModel, cmd := model.Update(tea.KeyMsg{Type: tea.KeyEnter})

	if newModel.action != ResumeActionResume {
		t.Errorf("expected action Resume, got %d", newModel.action)
	}
	if cmd == nil {
		t.Error("expected command from enter key")
	}

	// Execute command and check message type
	msg := cmd()
	if _, ok := msg.(ResumeConfirmedMsg); !ok {
		t.Errorf("expected ResumeConfirmedMsg, got %T", msg)
	}
}

func TestResumeModel_Update_Y(t *testing.T) {
	state := &engine.ProjectState{
		Project: &db.Project{ID: "test-id", Name: "Test Project"},
	}
	model := NewResumeModel(state)

	newModel, cmd := model.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'y'}})

	if newModel.action != ResumeActionResume {
		t.Errorf("expected action Resume, got %d", newModel.action)
	}
	if cmd == nil {
		t.Error("expected command from 'y' key")
	}
}

func TestResumeModel_Update_Reset(t *testing.T) {
	state := &engine.ProjectState{
		Project: &db.Project{ID: "test-id", Name: "Test Project"},
	}
	model := NewResumeModel(state)

	newModel, cmd := model.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'r'}})

	if newModel.action != ResumeActionReset {
		t.Errorf("expected action Reset, got %d", newModel.action)
	}
	if cmd == nil {
		t.Error("expected command from 'r' key")
	}

	msg := cmd()
	if _, ok := msg.(ResetConfirmedMsg); !ok {
		t.Errorf("expected ResetConfirmedMsg, got %T", msg)
	}
}

func TestResumeModel_Update_Details(t *testing.T) {
	state := &engine.ProjectState{
		Project:     &db.Project{ID: "test-id", Name: "Test Project"},
		LastSession: &db.Session{ID: "s1", AgentType: db.AgentDeveloper},
	}
	model := NewResumeModel(state)

	if model.showDetails {
		t.Error("expected showDetails to be false initially")
	}

	newModel, _ := model.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'d'}})
	if !newModel.showDetails {
		t.Error("expected showDetails to be true after 'd' key")
	}

	newModel, _ = newModel.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'d'}})
	if newModel.showDetails {
		t.Error("expected showDetails to be false after second 'd' key")
	}
}

func TestResumeModel_Update_Quit(t *testing.T) {
	state := &engine.ProjectState{
		Project: &db.Project{ID: "test-id", Name: "Test Project"},
	}
	model := NewResumeModel(state)

	newModel, cmd := model.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'q'}})

	if newModel.action != ResumeActionQuit {
		t.Errorf("expected action Quit, got %d", newModel.action)
	}
	if cmd == nil {
		t.Error("expected quit command")
	}
}

func TestResumeModel_Update_Escape(t *testing.T) {
	state := &engine.ProjectState{
		Project: &db.Project{ID: "test-id", Name: "Test Project"},
	}
	model := NewResumeModel(state)

	newModel, _ := model.Update(tea.KeyMsg{Type: tea.KeyEsc})

	if newModel.action != ResumeActionQuit {
		t.Errorf("expected action Quit, got %d", newModel.action)
	}
}

func TestResumeModel_View(t *testing.T) {
	state := &engine.ProjectState{
		Project:        &db.Project{ID: "test-id", Name: "Test Project", Status: db.ProjectInProgress},
		CompletedTasks: 2,
		PendingTasks:   3,
		FailedTasks:    1,
	}
	model := NewResumeModel(state)

	view := model.View()

	if view == "" {
		t.Error("expected non-empty view")
	}
	if !containsString(view, "Resume Project?") {
		t.Error("expected view to contain title")
	}
	if !containsString(view, "Test Project") {
		t.Error("expected view to contain project name")
	}
	if !containsString(view, "Completed: 2") {
		t.Error("expected view to contain completed count")
	}
	if !containsString(view, "Pending:   3") {
		t.Error("expected view to contain pending count")
	}
	if !containsString(view, "Failed:    1") {
		t.Error("expected view to contain failed count")
	}
}

func TestResumeModel_View_WithInterruptedTask(t *testing.T) {
	state := &engine.ProjectState{
		Project:        &db.Project{ID: "test-id", Name: "Test Project", Status: db.ProjectInProgress},
		CompletedTasks: 2,
		PendingTasks:   1,
		InProgressTask: &db.Task{ID: "t1", Title: "Interrupted Task"},
	}
	model := NewResumeModel(state)

	view := model.View()

	if !containsString(view, "Interrupted task:") {
		t.Error("expected view to show interrupted task")
	}
	if !containsString(view, "Interrupted Task") {
		t.Error("expected view to show interrupted task title")
	}
}

func TestResumeModel_View_WithDetails(t *testing.T) {
	state := &engine.ProjectState{
		Project:     &db.Project{ID: "test-id", Name: "Test Project", Status: db.ProjectInProgress},
		LastSession: &db.Session{ID: "s1", AgentType: db.AgentDeveloper, Iteration: 2, Status: db.SessionRunning},
	}
	model := NewResumeModel(state)
	model.showDetails = true

	view := model.View()

	if !containsString(view, "Last session:") {
		t.Error("expected view to show last session details")
	}
	if !containsString(view, "developer") {
		t.Error("expected view to show agent type")
	}
}

func TestResumeModel_Accessors(t *testing.T) {
	state := &engine.ProjectState{
		Project: &db.Project{ID: "test-id", Name: "Test Project"},
	}
	model := NewResumeModel(state)
	model.action = ResumeActionResume

	if model.State() != state {
		t.Error("State() returned wrong value")
	}
	if model.Action() != ResumeActionResume {
		t.Errorf("Action() returned wrong value: %d", model.Action())
	}
}

func TestResumeModel_FormatProjectStatus(t *testing.T) {
	testCases := []struct {
		status   db.ProjectStatus
		expected string
	}{
		{db.ProjectPending, "pending"},
		{db.ProjectInProgress, "in progress"},
		{db.ProjectCompleted, "completed"},
		{db.ProjectFailed, "failed"},
	}

	for _, tc := range testCases {
		t.Run(string(tc.status), func(t *testing.T) {
			state := &engine.ProjectState{
				Project: &db.Project{ID: "test-id", Name: "Test", Status: tc.status},
			}
			model := NewResumeModel(state)
			result := model.formatProjectStatus()
			if !containsString(result, tc.expected) {
				t.Errorf("expected status to contain %q, got %q", tc.expected, result)
			}
		})
	}
}

// FailedTasksModel tests

func TestNewFailedTasksModel(t *testing.T) {
	tasks := []*db.Task{
		{ID: "t1", Title: "Task 1", Status: db.TaskCompleted},
		{ID: "t2", Title: "Task 2", Status: db.TaskFailed},
		{ID: "t3", Title: "Task 3", Status: db.TaskEscalated},
		{ID: "t4", Title: "Task 4", Status: db.TaskPending},
	}

	model := NewFailedTasksModel("project-id", tasks)

	if len(model.tasks) != 2 {
		t.Errorf("expected 2 failed tasks, got %d", len(model.tasks))
	}
}

func TestFailedTasksModel_Init(t *testing.T) {
	model := NewFailedTasksModel("project-id", nil)
	cmd := model.Init()
	if cmd != nil {
		t.Error("expected nil command from Init")
	}
}

func TestFailedTasksModel_Update_Navigation(t *testing.T) {
	tasks := []*db.Task{
		{ID: "t1", Title: "Task 1", Status: db.TaskFailed},
		{ID: "t2", Title: "Task 2", Status: db.TaskFailed},
		{ID: "t3", Title: "Task 3", Status: db.TaskFailed},
	}
	model := NewFailedTasksModel("project-id", tasks)

	// Navigate down
	newModel, _ := model.Update(tea.KeyMsg{Type: tea.KeyDown})
	if newModel.cursor != 1 {
		t.Errorf("expected cursor at 1, got %d", newModel.cursor)
	}

	newModel, _ = newModel.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'j'}})
	if newModel.cursor != 2 {
		t.Errorf("expected cursor at 2, got %d", newModel.cursor)
	}

	// Can't go past end
	newModel, _ = newModel.Update(tea.KeyMsg{Type: tea.KeyDown})
	if newModel.cursor != 2 {
		t.Errorf("expected cursor to stay at 2, got %d", newModel.cursor)
	}

	// Navigate up
	newModel, _ = newModel.Update(tea.KeyMsg{Type: tea.KeyUp})
	if newModel.cursor != 1 {
		t.Errorf("expected cursor at 1, got %d", newModel.cursor)
	}

	newModel, _ = newModel.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'k'}})
	if newModel.cursor != 0 {
		t.Errorf("expected cursor at 0, got %d", newModel.cursor)
	}

	// Can't go past start
	newModel, _ = newModel.Update(tea.KeyMsg{Type: tea.KeyUp})
	if newModel.cursor != 0 {
		t.Errorf("expected cursor to stay at 0, got %d", newModel.cursor)
	}
}

func TestFailedTasksModel_Update_Retry(t *testing.T) {
	tasks := []*db.Task{
		{ID: "t1", Title: "Task 1", Status: db.TaskFailed},
	}
	model := NewFailedTasksModel("project-id", tasks)

	_, cmd := model.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'r'}})
	if cmd == nil {
		t.Error("expected command from 'r' key")
	}

	msg := cmd()
	if retryMsg, ok := msg.(FailedTasksRetryMsg); !ok {
		t.Errorf("expected FailedTasksRetryMsg, got %T", msg)
	} else if retryMsg.ProjectID != "project-id" {
		t.Errorf("expected project ID 'project-id', got %s", retryMsg.ProjectID)
	}
}

func TestFailedTasksModel_Update_Skip(t *testing.T) {
	tasks := []*db.Task{
		{ID: "t1", Title: "Task 1", Status: db.TaskFailed},
	}
	model := NewFailedTasksModel("project-id", tasks)

	_, cmd := model.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'s'}})
	if cmd == nil {
		t.Error("expected command from 's' key")
	}

	msg := cmd()
	if _, ok := msg.(FailedTasksSkipMsg); !ok {
		t.Errorf("expected FailedTasksSkipMsg, got %T", msg)
	}
}

func TestFailedTasksModel_Update_Abort(t *testing.T) {
	tasks := []*db.Task{
		{ID: "t1", Title: "Task 1", Status: db.TaskFailed},
	}
	model := NewFailedTasksModel("project-id", tasks)

	// Test 'a' key
	_, cmd := model.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'a'}})
	if cmd == nil {
		t.Error("expected command from 'a' key")
	}
	msg := cmd()
	if _, ok := msg.(FailedTasksAbortMsg); !ok {
		t.Errorf("expected FailedTasksAbortMsg, got %T", msg)
	}

	// Test 'q' key
	_, cmd = model.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'q'}})
	if cmd == nil {
		t.Error("expected command from 'q' key")
	}
}

func TestFailedTasksModel_View(t *testing.T) {
	tasks := []*db.Task{
		{ID: "t1", Title: "Failed Task 1", Status: db.TaskFailed},
		{ID: "t2", Title: "Escalated Task", Status: db.TaskEscalated},
	}
	model := NewFailedTasksModel("project-id", tasks)

	view := model.View()

	if !containsString(view, "Some tasks failed:") {
		t.Error("expected view to contain header")
	}
	if !containsString(view, "Failed Task 1") {
		t.Error("expected view to contain first task")
	}
	if !containsString(view, "Escalated Task") {
		t.Error("expected view to contain second task")
	}
}

func TestFailedTasksModel_View_Empty(t *testing.T) {
	model := NewFailedTasksModel("project-id", nil)

	view := model.View()

	if !containsString(view, "No failed tasks") {
		t.Error("expected empty state message")
	}
}

func TestFailedTasksModel_Accessors(t *testing.T) {
	tasks := []*db.Task{
		{ID: "t1", Title: "Task 1", Status: db.TaskFailed},
		{ID: "t2", Title: "Task 2", Status: db.TaskFailed},
	}
	model := NewFailedTasksModel("project-id", tasks)
	model.cursor = 1

	if len(model.Tasks()) != 2 {
		t.Errorf("expected 2 tasks, got %d", len(model.Tasks()))
	}
	if model.Cursor() != 1 {
		t.Errorf("expected cursor at 1, got %d", model.Cursor())
	}
}

// CompletedProjectModel tests

func TestNewCompletedProjectModel(t *testing.T) {
	project := &db.Project{ID: "test-id", Name: "Test Project"}
	state := &engine.ProjectState{CompletedTasks: 5}

	model := NewCompletedProjectModel(project, state)

	if model.project != project {
		t.Error("expected project to be set")
	}
	if model.state != state {
		t.Error("expected state to be set")
	}
}

func TestCompletedProjectModel_Init(t *testing.T) {
	model := NewCompletedProjectModel(nil, nil)
	cmd := model.Init()
	if cmd != nil {
		t.Error("expected nil command from Init")
	}
}

func TestCompletedProjectModel_Update_Reset(t *testing.T) {
	project := &db.Project{ID: "test-id", Name: "Test Project"}
	model := NewCompletedProjectModel(project, nil)

	_, cmd := model.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'r'}})
	if cmd == nil {
		t.Error("expected command from 'r' key")
	}

	msg := cmd()
	if resetMsg, ok := msg.(ResetConfirmedMsg); !ok {
		t.Errorf("expected ResetConfirmedMsg, got %T", msg)
	} else if resetMsg.ProjectID != "test-id" {
		t.Errorf("expected project ID 'test-id', got %s", resetMsg.ProjectID)
	}
}

func TestCompletedProjectModel_Update_Quit(t *testing.T) {
	project := &db.Project{ID: "test-id", Name: "Test Project"}
	model := NewCompletedProjectModel(project, nil)

	// Test 'q' key
	_, cmd := model.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'q'}})
	if cmd == nil {
		t.Error("expected quit command from 'q' key")
	}

	// Test 'esc' key
	_, cmd = model.Update(tea.KeyMsg{Type: tea.KeyEsc})
	if cmd == nil {
		t.Error("expected quit command from 'esc' key")
	}

	// Test 'enter' key
	_, cmd = model.Update(tea.KeyMsg{Type: tea.KeyEnter})
	if cmd == nil {
		t.Error("expected quit command from 'enter' key")
	}
}

func TestCompletedProjectModel_View(t *testing.T) {
	project := &db.Project{ID: "test-id", Name: "Test Project"}
	state := &engine.ProjectState{CompletedTasks: 5}
	model := NewCompletedProjectModel(project, state)

	view := model.View()

	if !containsString(view, "Project Completed") {
		t.Error("expected view to contain title")
	}
	if !containsString(view, "Test Project") {
		t.Error("expected view to contain project name")
	}
	if !containsString(view, "5 tasks completed") {
		t.Error("expected view to contain task count")
	}
}

func TestCompletedProjectModel_Project(t *testing.T) {
	project := &db.Project{ID: "test-id", Name: "Test Project"}
	model := NewCompletedProjectModel(project, nil)

	if model.Project() != project {
		t.Error("Project() returned wrong value")
	}
}

// ResumeAction constants test
func TestResumeAction_Constants(t *testing.T) {
	if ResumeActionNone != 0 {
		t.Errorf("expected ResumeActionNone to be 0, got %d", ResumeActionNone)
	}
	if ResumeActionResume != 1 {
		t.Errorf("expected ResumeActionResume to be 1, got %d", ResumeActionResume)
	}
	if ResumeActionReset != 2 {
		t.Errorf("expected ResumeActionReset to be 2, got %d", ResumeActionReset)
	}
	if ResumeActionQuit != 3 {
		t.Errorf("expected ResumeActionQuit to be 3, got %d", ResumeActionQuit)
	}
}

// Helper function
func containsString(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(s) > 0 && findSubstring(s, substr))
}

func findSubstring(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
