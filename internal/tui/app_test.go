package tui

import (
	"testing"
	"time"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/gerund/ralph/internal/config"
	"github.com/gerund/ralph/internal/db"
	"github.com/gerund/ralph/internal/engine"
)

func TestNewCreateModeModel(t *testing.T) {
	database, err := db.New(":memory:")
	if err != nil {
		t.Fatalf("Failed to create database: %v", err)
	}
	defer func() { _ = database.Close() }()

	project := &db.Project{
		ID:     "test-id",
		Name:   "Test Project",
		Status: db.ProjectPending,
	}

	progress := NewTaskProgressModel(project, database, nil)
	model := NewCreateModeModel(progress, project, database, nil)

	// Verify model is initialized
	if model.progress.project != project {
		t.Error("Expected project to be set in progress model")
	}
	if model.project != project {
		t.Error("Expected project to be set in model")
	}
	if model.db != database {
		t.Error("Expected database to be set in model")
	}
}

func TestCreateModeModel_Init(t *testing.T) {
	database, err := db.New(":memory:")
	if err != nil {
		t.Fatalf("Failed to create database: %v", err)
	}
	defer func() { _ = database.Close() }()

	project := &db.Project{
		ID:     "test-id",
		Name:   "Test Project",
		Status: db.ProjectPending,
	}

	progress := NewTaskProgressModel(project, database, nil)
	model := NewCreateModeModel(progress, project, database, nil)

	cmd := model.Init()
	if cmd == nil {
		t.Error("Expected Init to return a command")
	}
}

func TestCreateModeModel_Update_Quit(t *testing.T) {
	database, err := db.New(":memory:")
	if err != nil {
		t.Fatalf("Failed to create database: %v", err)
	}
	defer func() { _ = database.Close() }()

	project := &db.Project{
		ID:     "test-id",
		Name:   "Test Project",
		Status: db.ProjectPending,
	}

	progress := NewTaskProgressModel(project, database, nil)
	model := NewCreateModeModel(progress, project, database, nil)

	// Test 'q' quit
	_, cmd := model.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'q'}})
	if cmd == nil {
		t.Error("Expected quit command after pressing 'q'")
	}

	// Test ctrl+c quit
	model = NewCreateModeModel(progress, project, database, nil) // Reset
	_, cmd = model.Update(tea.KeyMsg{Type: tea.KeyCtrlC})
	if cmd == nil {
		t.Error("Expected quit command after pressing ctrl+c")
	}
}

func TestCreateModeModel_Update_WindowSize(t *testing.T) {
	database, err := db.New(":memory:")
	if err != nil {
		t.Fatalf("Failed to create database: %v", err)
	}
	defer func() { _ = database.Close() }()

	project := &db.Project{
		ID:     "test-id",
		Name:   "Test Project",
		Status: db.ProjectPending,
	}

	progress := NewTaskProgressModel(project, database, nil)
	model := NewCreateModeModel(progress, project, database, nil)

	newModel, _ := model.Update(tea.WindowSizeMsg{Width: 100, Height: 50})
	m := newModel.(CreateModeModel)

	if m.progress.width != 100 {
		t.Errorf("Expected width 100, got %d", m.progress.width)
	}
	if m.progress.height != 50 {
		t.Errorf("Expected height 50, got %d", m.progress.height)
	}
}

func TestCreateModeModel_Update_ForwardsToProgress(t *testing.T) {
	database, err := db.New(":memory:")
	if err != nil {
		t.Fatalf("Failed to create database: %v", err)
	}
	defer func() { _ = database.Close() }()

	project := &db.Project{
		ID:     "test-id",
		Name:   "Test Project",
		Status: db.ProjectPending,
	}

	tasks := []*db.Task{
		{ID: "t1", Title: "Task 1", Status: db.TaskPending},
		{ID: "t2", Title: "Task 2", Status: db.TaskPending},
	}

	progress := NewTaskProgressModel(project, database, nil)
	model := NewCreateModeModel(progress, project, database, nil)

	// Forward TasksLoadedMsg to progress
	newModel, _ := model.Update(TasksLoadedMsg{Tasks: tasks})
	m := newModel.(CreateModeModel)

	if len(m.progress.Tasks()) != 2 {
		t.Errorf("Expected 2 tasks, got %d", len(m.progress.Tasks()))
	}
}

func TestCreateModeModel_View(t *testing.T) {
	database, err := db.New(":memory:")
	if err != nil {
		t.Fatalf("Failed to create database: %v", err)
	}
	defer func() { _ = database.Close() }()

	project := &db.Project{
		ID:     "test-id",
		Name:   "Test Project",
		Status: db.ProjectPending,
	}

	progress := NewTaskProgressModel(project, database, nil)
	model := NewCreateModeModel(progress, project, database, nil)

	view := model.View()
	if view == "" {
		t.Error("Expected non-empty view")
	}
}

func TestNew(t *testing.T) {
	cfg := config.DefaultConfig()
	cfg.ProjectsDir = t.TempDir()
	workDir := "/test/work/dir"

	model := New(cfg, workDir)

	if model.state != ViewProjectList {
		t.Errorf("Expected initial state ViewProjectList, got %d", model.state)
	}
	if model.db != nil {
		t.Error("Database should be nil until project is selected")
	}
	if model.config != cfg {
		t.Error("Config not set correctly")
	}
	if model.workDir != workDir {
		t.Errorf("WorkDir not set correctly: got %s", model.workDir)
	}
}

func TestModel_Init(t *testing.T) {
	cfg := config.DefaultConfig()
	cfg.ProjectsDir = t.TempDir()

	model := New(cfg, "/test")
	cmd := model.Init()

	if cmd == nil {
		t.Error("Expected Init to return a command")
	}
}

func TestModel_Update_WindowSize(t *testing.T) {
	cfg := config.DefaultConfig()
	cfg.ProjectsDir = t.TempDir()

	model := New(cfg, "/test")
	newModel, _ := model.Update(tea.WindowSizeMsg{Width: 120, Height: 40})
	m := newModel.(Model)

	if m.width != 120 {
		t.Errorf("Expected width 120, got %d", m.width)
	}
	if m.height != 40 {
		t.Errorf("Expected height 40, got %d", m.height)
	}
}

func TestModel_Update_Quit(t *testing.T) {
	cfg := config.DefaultConfig()
	cfg.ProjectsDir = t.TempDir()

	model := New(cfg, "/test")

	// Test 'q' quit
	_, cmd := model.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'q'}})
	if cmd == nil {
		t.Error("Expected quit command after pressing 'q'")
	}
}

func TestModel_Update_CtrlC(t *testing.T) {
	cfg := config.DefaultConfig()
	cfg.ProjectsDir = t.TempDir()

	model := New(cfg, "/test")

	_, cmd := model.Update(tea.KeyMsg{Type: tea.KeyCtrlC})
	if cmd == nil {
		t.Error("Expected quit command after pressing ctrl+c")
	}
}

func TestModel_View_ProjectList(t *testing.T) {
	cfg := config.DefaultConfig()
	cfg.ProjectsDir = t.TempDir()

	model := New(cfg, "/test")
	view := model.View()

	// Should show project list view initially (loading state)
	if view == "" {
		t.Error("Expected non-empty view")
	}
}

func TestModel_ViewState_Constants(t *testing.T) {
	// Verify view state constants are distinct
	states := []ViewState{
		ViewProjectList,
		ViewResumeDialog,
		ViewCompletedProject,
		ViewTaskProgress,
		ViewFeedbackPrompt,
		ViewFeedbackInstructions,
		ViewCapturingLearnings,
		ViewCompleted,
	}

	seen := make(map[ViewState]bool)
	for _, state := range states {
		if seen[state] {
			t.Errorf("duplicate view state: %d", state)
		}
		seen[state] = true
	}

	// Verify ViewProjectList is the initial state (0)
	if ViewProjectList != 0 {
		t.Errorf("ViewProjectList should be 0, got %d", ViewProjectList)
	}
}

func TestModel_Update_ProjectSelected(t *testing.T) {
	// Create a project in a temp project directory
	cfg := config.DefaultConfig()
	cfg.ProjectsDir = t.TempDir()
	workDir := t.TempDir()

	projectID := "test-id"

	// Create the project database
	database, err := db.OpenProjectDB(cfg.GetProjectsDir(), projectID)
	if err != nil {
		t.Fatalf("Failed to open project database: %v", err)
	}

	project := &db.Project{
		ID:        projectID,
		Name:      "Test Project",
		PlanText:  "Test plan",
		Status:    db.ProjectPending,
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}
	if err := database.CreateProject(project); err != nil {
		t.Fatalf("Failed to create project: %v", err)
	}

	// Create a pending task so the project is resumable
	task := &db.Task{
		ID:        "task-1",
		ProjectID: project.ID,
		Sequence:  1,
		Title:     "Test Task",
		Status:    db.TaskPending,
	}
	if err := database.CreateTask(task); err != nil {
		t.Fatalf("Failed to create task: %v", err)
	}
	database.Close()

	model := New(cfg, workDir)
	model.width = 100
	model.height = 50

	// Simulate project selection with ProjectInfo
	projectInfo := db.ProjectInfo{
		ID:        projectID,
		Name:      "Test Project",
		Status:    db.ProjectPending,
		UpdatedAt: time.Now(),
		DBPath:    db.ProjectDBPath(cfg.GetProjectsDir(), projectID),
	}

	newModel, cmd := model.Update(ProjectSelectedMsg{ProjectInfo: projectInfo})
	m := newModel.(Model)

	// Engine should be created
	if m.engine == nil {
		t.Error("Expected engine to be created")
	}

	// Database should be opened
	if m.db == nil {
		t.Error("Expected database to be opened")
	}

	// Should have a command (checkProjectState command)
	if cmd == nil {
		t.Error("Expected command after project selection")
	}

	// Clean up
	if m.db != nil {
		m.db.Close()
	}
}

func TestModel_Update_EngineStartedMsg(t *testing.T) {
	cfg := config.DefaultConfig()
	cfg.ProjectsDir = t.TempDir()

	database, err := db.New(":memory:")
	if err != nil {
		t.Fatalf("Failed to create database: %v", err)
	}
	defer database.Close()

	model := New(cfg, "/test")

	// Create a mock engine for testing
	eng, _ := engine.NewEngine(engine.EngineConfig{
		Config:  cfg,
		DB:      database,
		WorkDir: t.TempDir(),
	})

	_, cmd := model.Update(EngineStartedMsg{Engine: eng})

	// Should return nil command (nothing to do)
	if cmd != nil {
		t.Error("Expected nil command for EngineStartedMsg")
	}
}

func TestModel_Update_EngineErrorMsg(t *testing.T) {
	cfg := config.DefaultConfig()
	cfg.ProjectsDir = t.TempDir()

	model := New(cfg, "/test")

	_, cmd := model.Update(EngineErrorMsg{Err: nil})

	// Should return nil command (error handling)
	if cmd != nil {
		t.Error("Expected nil command for EngineErrorMsg")
	}
}

func TestModel_Update_ForwardsToProjectList(t *testing.T) {
	cfg := config.DefaultConfig()
	cfg.ProjectsDir = t.TempDir()

	model := New(cfg, "/test")

	// Simulate key press that should be forwarded to project list
	projects := []db.ProjectInfo{
		{ID: "p1", Name: "Project 1", Status: db.ProjectPending, UpdatedAt: time.Now()},
		{ID: "p2", Name: "Project 2", Status: db.ProjectPending, UpdatedAt: time.Now()},
	}

	// Load projects first
	newModel, _ := model.Update(ProjectsLoadedMsg{Projects: projects})
	m := newModel.(Model)

	// Navigate down
	newModel, _ = m.Update(tea.KeyMsg{Type: tea.KeyDown})
	m = newModel.(Model)

	if m.projectList.Cursor() != 1 {
		t.Errorf("Expected cursor at 1, got %d", m.projectList.Cursor())
	}
}

func TestModel_View_TaskProgress(t *testing.T) {
	database, err := db.New(":memory:")
	if err != nil {
		t.Fatalf("Failed to create database: %v", err)
	}
	defer func() { _ = database.Close() }()

	cfg := config.DefaultConfig()
	cfg.ProjectsDir = t.TempDir()

	model := New(cfg, t.TempDir())

	// Manually set state to task progress
	model.state = ViewTaskProgress
	model.db = database
	project := &db.Project{
		ID:     "test-id",
		Name:   "Test Project",
		Status: db.ProjectPending,
	}
	model.taskProgress = NewTaskProgressModel(project, database, nil)

	view := model.View()
	if view == "" {
		t.Error("Expected non-empty view for task progress state")
	}
}

func TestModel_View_UnknownState(t *testing.T) {
	cfg := config.DefaultConfig()
	cfg.ProjectsDir = t.TempDir()

	model := New(cfg, "/test")

	// Set invalid state
	model.state = ViewState(999)

	view := model.View()
	if view != "Unknown view state\n" {
		t.Errorf("Expected 'Unknown view state', got %q", view)
	}
}

func TestModel_Update_ShowResumeDialogMsg(t *testing.T) {
	cfg := config.DefaultConfig()
	cfg.ProjectsDir = t.TempDir()

	project := &db.Project{
		ID:     "test-id",
		Name:   "Test Project",
		Status: db.ProjectInProgress,
	}

	state := &engine.ProjectState{
		Project:        project,
		CompletedTasks: 2,
		PendingTasks:   1,
		InProgressTask: &db.Task{ID: "t1", Title: "Interrupted Task"},
		NeedsCleanup:   true,
	}

	model := New(cfg, t.TempDir())
	model.width = 100
	model.height = 50

	newModel, cmd := model.Update(ShowResumeDialogMsg{State: state})
	m := newModel.(Model)

	if m.state != ViewResumeDialog {
		t.Errorf("Expected state to be ViewResumeDialog, got %d", m.state)
	}

	// Should return a command to send window size
	if cmd == nil {
		t.Error("Expected command after ShowResumeDialogMsg")
	}
}

func TestModel_Update_ProjectCompletedMsg(t *testing.T) {
	cfg := config.DefaultConfig()
	cfg.ProjectsDir = t.TempDir()
	workDir := t.TempDir()

	database, err := db.New(":memory:")
	if err != nil {
		t.Fatalf("Failed to create database: %v", err)
	}
	defer database.Close()

	project := &db.Project{
		ID:     "test-id",
		Name:   "Test Project",
		Status: db.ProjectCompleted,
	}
	if err := database.CreateProject(project); err != nil {
		t.Fatalf("Failed to create project: %v", err)
	}

	model := New(cfg, workDir)
	model.width = 100
	model.height = 50
	model.db = database

	// Create engine first
	eng, err := engine.NewEngine(engine.EngineConfig{
		Config:  cfg,
		DB:      database,
		WorkDir: workDir,
	})
	if err != nil {
		t.Fatalf("Failed to create engine: %v", err)
	}
	model.engine = eng
	model.currentProject = project

	newModel, cmd := model.Update(ProjectCompletedMsg{Project: project})
	m := newModel.(Model)

	if m.state != ViewCompletedProject {
		t.Errorf("Expected state to be ViewCompletedProject, got %d", m.state)
	}

	if cmd == nil {
		t.Error("Expected command after ProjectCompletedMsg")
	}
}

func TestModel_Update_StartProjectMsg(t *testing.T) {
	cfg := config.DefaultConfig()
	cfg.ProjectsDir = t.TempDir()
	workDir := t.TempDir()

	database, err := db.New(":memory:")
	if err != nil {
		t.Fatalf("Failed to create database: %v", err)
	}
	defer database.Close()

	project := &db.Project{
		ID:     "test-id",
		Name:   "Test Project",
		Status: db.ProjectPending,
	}
	if err := database.CreateProject(project); err != nil {
		t.Fatalf("Failed to create project: %v", err)
	}

	model := New(cfg, workDir)
	model.width = 100
	model.height = 50
	model.db = database

	// Create engine first
	eng, err := engine.NewEngine(engine.EngineConfig{
		Config:  cfg,
		DB:      database,
		WorkDir: workDir,
	})
	if err != nil {
		t.Fatalf("Failed to create engine: %v", err)
	}
	model.engine = eng
	model.currentProject = project

	newModel, cmd := model.Update(StartProjectMsg{Project: project})
	m := newModel.(Model)

	if m.state != ViewTaskProgress {
		t.Errorf("Expected state to be ViewTaskProgress, got %d", m.state)
	}

	if cmd == nil {
		t.Error("Expected command after StartProjectMsg")
	}
}

func TestModel_View_ResumeDialog(t *testing.T) {
	cfg := config.DefaultConfig()
	cfg.ProjectsDir = t.TempDir()

	project := &db.Project{
		ID:     "test-id",
		Name:   "Test Project",
		Status: db.ProjectInProgress,
	}

	state := &engine.ProjectState{
		Project:        project,
		CompletedTasks: 2,
		PendingTasks:   1,
	}

	model := New(cfg, t.TempDir())
	model.state = ViewResumeDialog
	model.resumeDialog = NewResumeModel(state)

	view := model.View()
	if view == "" {
		t.Error("Expected non-empty view for resume dialog")
	}
}

func TestModel_View_CompletedProject(t *testing.T) {
	cfg := config.DefaultConfig()
	cfg.ProjectsDir = t.TempDir()

	project := &db.Project{
		ID:     "test-id",
		Name:   "Test Project",
		Status: db.ProjectCompleted,
	}

	state := &engine.ProjectState{
		Project:        project,
		CompletedTasks: 5,
	}

	model := New(cfg, t.TempDir())
	model.state = ViewCompletedProject
	model.completedProject = NewCompletedProjectModel(project, state)

	view := model.View()
	if view == "" {
		t.Error("Expected non-empty view for completed project")
	}
}
