package tui

import (
	"os"
	"strings"
	"testing"
	"time"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/gerund/ralph/internal/config"
	"github.com/gerund/ralph/internal/db"
)

// testConfig creates a test config with a temp projects directory.
func testConfig(t *testing.T) *config.Config {
	t.Helper()
	tmpDir := t.TempDir()
	cfg := config.DefaultConfig()
	cfg.ProjectsDir = tmpDir
	return cfg
}

// createTestProject creates a test project in the projects directory.
func createTestProject(t *testing.T, cfg *config.Config, id, name string, status db.ProjectStatus) {
	t.Helper()
	database, err := db.OpenProjectDB(cfg.GetProjectsDir(), id)
	if err != nil {
		t.Fatalf("Failed to create project database: %v", err)
	}
	defer database.Close()

	project := &db.Project{
		ID:       id,
		Name:     name,
		PlanText: "Test plan",
		Status:   status,
	}
	if err := database.CreateProject(project); err != nil {
		t.Fatalf("Failed to create project: %v", err)
	}
}

func TestNewProjectListModel(t *testing.T) {
	cfg := testConfig(t)

	m := NewProjectListModel(cfg)

	if !m.IsLoading() {
		t.Error("Expected model to be loading initially")
	}
	if m.cursor != 0 {
		t.Errorf("Expected cursor to be 0, got %d", m.cursor)
	}
	if len(m.Projects()) != 0 {
		t.Errorf("Expected no projects initially, got %d", len(m.Projects()))
	}
}

func TestProjectListModel_Init(t *testing.T) {
	cfg := testConfig(t)

	m := NewProjectListModel(cfg)
	cmd := m.Init()

	if cmd == nil {
		t.Fatal("Expected Init to return a command")
	}

	// Execute the command to get the message
	msg := cmd()
	loadedMsg, ok := msg.(ProjectsLoadedMsg)
	if !ok {
		t.Fatalf("Expected ProjectsLoadedMsg, got %T", msg)
	}

	if loadedMsg.Err != nil {
		t.Errorf("Expected no error, got %v", loadedMsg.Err)
	}
}

func TestProjectListModel_Update_ProjectsLoaded(t *testing.T) {
	cfg := testConfig(t)

	// Create test projects using ProjectInfo
	projects := []db.ProjectInfo{
		{ID: "p1", Name: "Project 1", Status: db.ProjectPending, UpdatedAt: time.Now()},
		{ID: "p2", Name: "Project 2", Status: db.ProjectInProgress, UpdatedAt: time.Now()},
	}

	m := NewProjectListModel(cfg)
	m, _ = m.Update(ProjectsLoadedMsg{Projects: projects})

	if m.IsLoading() {
		t.Error("Expected model to not be loading after ProjectsLoadedMsg")
	}
	if len(m.Projects()) != 2 {
		t.Errorf("Expected 2 projects, got %d", len(m.Projects()))
	}
}

func TestProjectListModel_Update_ProjectsLoadedError(t *testing.T) {
	cfg := testConfig(t)

	m := NewProjectListModel(cfg)
	testErr := db.ErrNotFound
	m, _ = m.Update(ProjectsLoadedMsg{Err: testErr})

	if m.IsLoading() {
		t.Error("Expected model to not be loading after error")
	}
	if m.Error() == nil {
		t.Error("Expected error to be set")
	}
}

func TestProjectListModel_Update_Navigation(t *testing.T) {
	cfg := testConfig(t)

	projects := []db.ProjectInfo{
		{ID: "p1", Name: "Project 1", Status: db.ProjectPending, UpdatedAt: time.Now()},
		{ID: "p2", Name: "Project 2", Status: db.ProjectInProgress, UpdatedAt: time.Now()},
		{ID: "p3", Name: "Project 3", Status: db.ProjectCompleted, UpdatedAt: time.Now()},
	}

	m := NewProjectListModel(cfg)
	m, _ = m.Update(ProjectsLoadedMsg{Projects: projects})

	// Test initial cursor position
	if m.Cursor() != 0 {
		t.Errorf("Expected cursor at 0, got %d", m.Cursor())
	}

	// Test down navigation with 'j'
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'j'}})
	if m.Cursor() != 1 {
		t.Errorf("Expected cursor at 1 after 'j', got %d", m.Cursor())
	}

	// Test down navigation with arrow key
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyDown})
	if m.Cursor() != 2 {
		t.Errorf("Expected cursor at 2 after down arrow, got %d", m.Cursor())
	}

	// Test cursor doesn't go past end
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyDown})
	if m.Cursor() != 2 {
		t.Errorf("Expected cursor to stay at 2, got %d", m.Cursor())
	}

	// Test up navigation with 'k'
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'k'}})
	if m.Cursor() != 1 {
		t.Errorf("Expected cursor at 1 after 'k', got %d", m.Cursor())
	}

	// Test up navigation with arrow key
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyUp})
	if m.Cursor() != 0 {
		t.Errorf("Expected cursor at 0 after up arrow, got %d", m.Cursor())
	}

	// Test cursor doesn't go below 0
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyUp})
	if m.Cursor() != 0 {
		t.Errorf("Expected cursor to stay at 0, got %d", m.Cursor())
	}
}

func TestProjectListModel_Update_HomeEnd(t *testing.T) {
	cfg := testConfig(t)

	projects := []db.ProjectInfo{
		{ID: "p1", Name: "Project 1", Status: db.ProjectPending, UpdatedAt: time.Now()},
		{ID: "p2", Name: "Project 2", Status: db.ProjectInProgress, UpdatedAt: time.Now()},
		{ID: "p3", Name: "Project 3", Status: db.ProjectCompleted, UpdatedAt: time.Now()},
	}

	m := NewProjectListModel(cfg)
	m, _ = m.Update(ProjectsLoadedMsg{Projects: projects})

	// Test 'G' goes to end
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'G'}})
	if m.Cursor() != 2 {
		t.Errorf("Expected cursor at 2 after 'G', got %d", m.Cursor())
	}

	// Test 'g' goes to beginning
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'g'}})
	if m.Cursor() != 0 {
		t.Errorf("Expected cursor at 0 after 'g', got %d", m.Cursor())
	}
}

func TestProjectListModel_Update_Selection(t *testing.T) {
	cfg := testConfig(t)

	projects := []db.ProjectInfo{
		{ID: "p1", Name: "Project 1", Status: db.ProjectPending, UpdatedAt: time.Now()},
		{ID: "p2", Name: "Project 2", Status: db.ProjectInProgress, UpdatedAt: time.Now()},
	}

	m := NewProjectListModel(cfg)
	m, _ = m.Update(ProjectsLoadedMsg{Projects: projects})

	// Move to second project
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyDown})

	// Press enter
	_, cmd := m.Update(tea.KeyMsg{Type: tea.KeyEnter})

	if cmd == nil {
		t.Fatal("Expected command after pressing enter")
	}

	// Execute command
	msg := cmd()
	selectedMsg, ok := msg.(ProjectSelectedMsg)
	if !ok {
		t.Fatalf("Expected ProjectSelectedMsg, got %T", msg)
	}

	if selectedMsg.ProjectInfo.ID != "p2" {
		t.Errorf("Expected project p2, got %s", selectedMsg.ProjectInfo.ID)
	}
}

func TestProjectListModel_Update_EnterWithNoProjects(t *testing.T) {
	cfg := testConfig(t)

	m := NewProjectListModel(cfg)
	m, _ = m.Update(ProjectsLoadedMsg{Projects: []db.ProjectInfo{}})

	// Press enter with no projects
	_, cmd := m.Update(tea.KeyMsg{Type: tea.KeyEnter})

	if cmd != nil {
		t.Error("Expected no command when pressing enter with no projects")
	}
}

func TestProjectListModel_Update_WindowSize(t *testing.T) {
	cfg := testConfig(t)

	m := NewProjectListModel(cfg)
	m, _ = m.Update(tea.WindowSizeMsg{Width: 100, Height: 50})

	if m.width != 100 {
		t.Errorf("Expected width 100, got %d", m.width)
	}
	if m.height != 50 {
		t.Errorf("Expected height 50, got %d", m.height)
	}
}

func TestProjectListModel_View_Loading(t *testing.T) {
	cfg := testConfig(t)

	m := NewProjectListModel(cfg)
	view := m.View()

	if !strings.Contains(view, "Loading") {
		t.Error("Expected view to contain 'Loading' while loading")
	}
}

func TestProjectListModel_View_Error(t *testing.T) {
	cfg := testConfig(t)

	m := NewProjectListModel(cfg)
	m, _ = m.Update(ProjectsLoadedMsg{Err: db.ErrNotFound})

	view := m.View()
	if !strings.Contains(view, "Error") {
		t.Error("Expected view to contain 'Error' when there's an error")
	}
}

func TestProjectListModel_View_EmptyState(t *testing.T) {
	cfg := testConfig(t)

	m := NewProjectListModel(cfg)
	m, _ = m.Update(ProjectsLoadedMsg{Projects: []db.ProjectInfo{}})

	view := m.View()
	if !strings.Contains(view, "No projects yet") {
		t.Error("Expected view to show empty state message")
	}
}

func TestProjectListModel_View_WithProjects(t *testing.T) {
	cfg := testConfig(t)

	now := time.Date(2024, 1, 15, 10, 30, 0, 0, time.UTC)
	projects := []db.ProjectInfo{
		{ID: "p1", Name: "Test Project", Status: db.ProjectPending, UpdatedAt: now},
	}

	m := NewProjectListModel(cfg)
	m, _ = m.Update(ProjectsLoadedMsg{Projects: projects})

	view := m.View()

	// Check title
	if !strings.Contains(view, "Ralph") {
		t.Error("Expected view to contain title")
	}

	// Check project name
	if !strings.Contains(view, "Test Project") {
		t.Error("Expected view to contain project name")
	}

	// Check status
	if !strings.Contains(view, "pending") {
		t.Error("Expected view to contain status")
	}

	// Check help text
	if !strings.Contains(view, "j/k") || !strings.Contains(view, "navigate") {
		t.Error("Expected view to contain help text")
	}
}

func TestProjectListModel_View_StatusColors(t *testing.T) {
	cfg := testConfig(t)

	now := time.Now()
	projects := []db.ProjectInfo{
		{ID: "p1", Name: "Pending", Status: db.ProjectPending, UpdatedAt: now},
		{ID: "p2", Name: "In Progress", Status: db.ProjectInProgress, UpdatedAt: now},
		{ID: "p3", Name: "Completed", Status: db.ProjectCompleted, UpdatedAt: now},
		{ID: "p4", Name: "Failed", Status: db.ProjectFailed, UpdatedAt: now},
	}

	m := NewProjectListModel(cfg)
	m, _ = m.Update(ProjectsLoadedMsg{Projects: projects})
	m, _ = m.Update(tea.WindowSizeMsg{Width: 80, Height: 30})

	view := m.View()

	// Just verify all statuses appear (colors are handled by lipgloss)
	if !strings.Contains(view, "pending") {
		t.Error("Expected view to contain pending status")
	}
	if !strings.Contains(view, "in progress") {
		t.Error("Expected view to contain in progress status")
	}
	if !strings.Contains(view, "completed") {
		t.Error("Expected view to contain completed status")
	}
	if !strings.Contains(view, "failed") {
		t.Error("Expected view to contain failed status")
	}
}

func TestProjectListModel_SelectedProject(t *testing.T) {
	cfg := testConfig(t)

	projects := []db.ProjectInfo{
		{ID: "p1", Name: "Project 1", Status: db.ProjectPending, UpdatedAt: time.Now()},
		{ID: "p2", Name: "Project 2", Status: db.ProjectInProgress, UpdatedAt: time.Now()},
	}

	m := NewProjectListModel(cfg)

	// No projects loaded - should return nil
	if m.SelectedProject() != nil {
		t.Error("Expected nil when no projects")
	}

	// Load projects
	m, _ = m.Update(ProjectsLoadedMsg{Projects: projects})

	selected := m.SelectedProject()
	if selected == nil {
		t.Fatal("Expected selected project")
	}
	if selected.ID != "p1" {
		t.Errorf("Expected p1, got %s", selected.ID)
	}

	// Move cursor
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyDown})
	selected = m.SelectedProject()
	if selected.ID != "p2" {
		t.Errorf("Expected p2 after moving cursor, got %s", selected.ID)
	}
}

func TestProjectListModel_NavigationWhileLoading(t *testing.T) {
	cfg := testConfig(t)

	m := NewProjectListModel(cfg)

	// Try to navigate while loading
	initialCursor := m.Cursor()
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyDown})

	// Cursor should not change
	if m.Cursor() != initialCursor {
		t.Error("Cursor should not change while loading")
	}
}

func TestProjectListModel_DiscoverProjects(t *testing.T) {
	cfg := testConfig(t)

	// Create test projects
	createTestProject(t, cfg, "proj-1", "First Project", db.ProjectPending)
	time.Sleep(10 * time.Millisecond)
	createTestProject(t, cfg, "proj-2", "Second Project", db.ProjectInProgress)

	// Create model and load projects
	m := NewProjectListModel(cfg)
	cmd := m.Init()
	msg := cmd()

	loadedMsg, ok := msg.(ProjectsLoadedMsg)
	if !ok {
		t.Fatalf("Expected ProjectsLoadedMsg, got %T", msg)
	}

	if loadedMsg.Err != nil {
		t.Fatalf("Unexpected error: %v", loadedMsg.Err)
	}

	if len(loadedMsg.Projects) != 2 {
		t.Errorf("Expected 2 projects, got %d", len(loadedMsg.Projects))
	}

	// Projects should be sorted by UpdatedAt descending
	if loadedMsg.Projects[0].ID != "proj-2" {
		t.Errorf("Expected most recent project first, got %s", loadedMsg.Projects[0].ID)
	}
}

func TestProjectListModel_DiscoverEmptyDir(t *testing.T) {
	tmpDir := t.TempDir()
	cfg := config.DefaultConfig()
	cfg.ProjectsDir = tmpDir

	// Ensure directory exists
	if err := os.MkdirAll(tmpDir, 0755); err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}

	m := NewProjectListModel(cfg)
	cmd := m.Init()
	msg := cmd()

	loadedMsg, ok := msg.(ProjectsLoadedMsg)
	if !ok {
		t.Fatalf("Expected ProjectsLoadedMsg, got %T", msg)
	}

	if loadedMsg.Err != nil {
		t.Fatalf("Unexpected error: %v", loadedMsg.Err)
	}

	if len(loadedMsg.Projects) != 0 {
		t.Errorf("Expected 0 projects, got %d", len(loadedMsg.Projects))
	}
}
