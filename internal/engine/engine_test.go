package engine

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/gerund/ralph/internal/config"
	"github.com/gerund/ralph/internal/db"
)

// Helper to create test dependencies for Engine
func createEngineDeps(t *testing.T) (*db.DB, *config.Config, string, func()) {
	// Create in-memory database
	database, err := db.New(":memory:")
	if err != nil {
		t.Fatalf("failed to create test database: %v", err)
	}

	cfg := config.DefaultConfig()

	// Create temp directory for work dir
	workDir, err := os.MkdirTemp("", "ralph-engine-test-*")
	if err != nil {
		database.Close()
		t.Fatalf("failed to create temp dir: %v", err)
	}

	cleanup := func() {
		database.Close()
		os.RemoveAll(workDir)
	}

	return database, cfg, workDir, cleanup
}

func TestNewEngine(t *testing.T) {
	database, cfg, workDir, cleanup := createEngineDeps(t)
	defer cleanup()

	engine, err := NewEngine(EngineConfig{
		Config:  cfg,
		DB:      database,
		WorkDir: workDir,
	})

	if err != nil {
		t.Fatalf("NewEngine failed: %v", err)
	}
	if engine == nil {
		t.Fatal("NewEngine returned nil")
	}
	if engine.config != cfg {
		t.Error("config not set correctly")
	}
	if engine.db != database {
		t.Error("database not set correctly")
	}
	if engine.claude == nil {
		t.Error("claude client not created")
	}
	if engine.jj == nil {
		t.Error("jj client not created")
	}
	if engine.agents == nil {
		t.Error("agents manager not created")
	}
	if engine.events == nil {
		t.Error("events channel not created")
	}
}

func TestNewEngine_MissingConfig(t *testing.T) {
	database, _, workDir, cleanup := createEngineDeps(t)
	defer cleanup()

	_, err := NewEngine(EngineConfig{
		Config:  nil,
		DB:      database,
		WorkDir: workDir,
	})

	if err == nil {
		t.Error("expected error for missing config")
	}
}

func TestNewEngine_MissingDB(t *testing.T) {
	_, cfg, workDir, cleanup := createEngineDeps(t)
	defer cleanup()

	_, err := NewEngine(EngineConfig{
		Config:  cfg,
		DB:      nil,
		WorkDir: workDir,
	})

	if err == nil {
		t.Error("expected error for missing database")
	}
}

func TestNewEngine_MissingWorkDir(t *testing.T) {
	database, cfg, _, cleanup := createEngineDeps(t)
	defer cleanup()

	_, err := NewEngine(EngineConfig{
		Config:  cfg,
		DB:      database,
		WorkDir: "",
	})

	if err == nil {
		t.Error("expected error for missing work directory")
	}
}

func TestEngine_Events(t *testing.T) {
	database, cfg, workDir, cleanup := createEngineDeps(t)
	defer cleanup()

	engine, err := NewEngine(EngineConfig{
		Config:  cfg,
		DB:      database,
		WorkDir: workDir,
	})
	if err != nil {
		t.Fatalf("NewEngine failed: %v", err)
	}

	events := engine.Events()
	if events == nil {
		t.Error("Events() returned nil channel")
	}
}

func TestEngine_Project_Initial(t *testing.T) {
	database, cfg, workDir, cleanup := createEngineDeps(t)
	defer cleanup()

	engine, err := NewEngine(EngineConfig{
		Config:  cfg,
		DB:      database,
		WorkDir: workDir,
	})
	if err != nil {
		t.Fatalf("NewEngine failed: %v", err)
	}

	// No project loaded initially
	if engine.Project() != nil {
		t.Error("expected nil project initially")
	}
}

func TestEngine_ResumeProject(t *testing.T) {
	database, cfg, workDir, cleanup := createEngineDeps(t)
	defer cleanup()

	// Create a project in the database
	project := &db.Project{
		ID:       "test-project-id",
		Name:     "Test Project",
		PlanText: "Test plan content",
		Status:   db.ProjectPending,
	}
	if err := database.CreateProject(project); err != nil {
		t.Fatalf("failed to create project: %v", err)
	}

	engine, err := NewEngine(EngineConfig{
		Config:  cfg,
		DB:      database,
		WorkDir: workDir,
	})
	if err != nil {
		t.Fatalf("NewEngine failed: %v", err)
	}

	// Resume the project
	resumed, err := engine.ResumeProject(context.Background(), "test-project-id")
	if err != nil {
		t.Fatalf("ResumeProject failed: %v", err)
	}

	if resumed.ID != project.ID {
		t.Errorf("expected project ID %s, got %s", project.ID, resumed.ID)
	}
	if resumed.Name != project.Name {
		t.Errorf("expected project name %s, got %s", project.Name, resumed.Name)
	}

	// Verify project is now loaded
	loadedProject := engine.Project()
	if loadedProject == nil {
		t.Error("expected project to be loaded")
	}
	if loadedProject.ID != project.ID {
		t.Error("loaded project ID doesn't match")
	}
}

func TestEngine_ResumeProject_NotFound(t *testing.T) {
	database, cfg, workDir, cleanup := createEngineDeps(t)
	defer cleanup()

	engine, err := NewEngine(EngineConfig{
		Config:  cfg,
		DB:      database,
		WorkDir: workDir,
	})
	if err != nil {
		t.Fatalf("NewEngine failed: %v", err)
	}

	_, err = engine.ResumeProject(context.Background(), "non-existent-id")
	if err == nil {
		t.Error("expected error for non-existent project")
	}
}

func TestEngine_Run_NoProject(t *testing.T) {
	database, cfg, workDir, cleanup := createEngineDeps(t)
	defer cleanup()

	engine, err := NewEngine(EngineConfig{
		Config:  cfg,
		DB:      database,
		WorkDir: workDir,
	})
	if err != nil {
		t.Fatalf("NewEngine failed: %v", err)
	}

	// Drain events
	go func() {
		for range engine.Events() {
		}
	}()

	err = engine.Run(context.Background())
	if err == nil {
		t.Error("expected error when running without project")
	}
}

func TestEngine_Run_Stopped(t *testing.T) {
	database, cfg, workDir, cleanup := createEngineDeps(t)
	defer cleanup()

	engine, err := NewEngine(EngineConfig{
		Config:  cfg,
		DB:      database,
		WorkDir: workDir,
	})
	if err != nil {
		t.Fatalf("NewEngine failed: %v", err)
	}

	// Stop the engine
	if err := engine.Stop(); err != nil {
		t.Fatalf("Stop failed: %v", err)
	}

	// Try to run
	err = engine.Run(context.Background())
	if err == nil {
		t.Error("expected error when running after stop")
	}
}

func TestEngine_Stop(t *testing.T) {
	database, cfg, workDir, cleanup := createEngineDeps(t)
	defer cleanup()

	engine, err := NewEngine(EngineConfig{
		Config:  cfg,
		DB:      database,
		WorkDir: workDir,
	})
	if err != nil {
		t.Fatalf("NewEngine failed: %v", err)
	}

	// Stop should succeed
	err = engine.Stop()
	if err != nil {
		t.Errorf("Stop failed: %v", err)
	}

	// Double stop should also succeed
	err = engine.Stop()
	if err != nil {
		t.Errorf("Double Stop failed: %v", err)
	}
}

func TestEngine_Stop_ClosesEvents(t *testing.T) {
	database, cfg, workDir, cleanup := createEngineDeps(t)
	defer cleanup()

	engine, err := NewEngine(EngineConfig{
		Config:  cfg,
		DB:      database,
		WorkDir: workDir,
	})
	if err != nil {
		t.Fatalf("NewEngine failed: %v", err)
	}

	// Wait for events channel to close
	done := make(chan bool)
	go func() {
		for range engine.Events() {
		}
		done <- true
	}()

	// Stop the engine
	engine.Stop()

	// Events channel should close
	select {
	case <-done:
		// Success
	case <-time.After(time.Second):
		t.Error("timeout waiting for events channel to close")
	}
}

func TestEngine_Emit(t *testing.T) {
	database, cfg, workDir, cleanup := createEngineDeps(t)
	defer cleanup()

	engine, err := NewEngine(EngineConfig{
		Config:  cfg,
		DB:      database,
		WorkDir: workDir,
	})
	if err != nil {
		t.Fatalf("NewEngine failed: %v", err)
	}

	// Emit an event
	go func() {
		engine.emit(EngineEventRunning, "test message")
	}()

	// Read the event
	select {
	case event := <-engine.Events():
		if event.Type != EngineEventRunning {
			t.Errorf("expected event type %s, got %s", EngineEventRunning, event.Type)
		}
		if event.Message != "test message" {
			t.Errorf("expected message %q, got %q", "test message", event.Message)
		}
	case <-time.After(100 * time.Millisecond):
		t.Error("timeout waiting for event")
	}
}

func TestEngine_Emit_AfterStop(t *testing.T) {
	database, cfg, workDir, cleanup := createEngineDeps(t)
	defer cleanup()

	engine, err := NewEngine(EngineConfig{
		Config:  cfg,
		DB:      database,
		WorkDir: workDir,
	})
	if err != nil {
		t.Fatalf("NewEngine failed: %v", err)
	}

	engine.Stop()

	// Emitting after stop should not panic
	engine.emit(EngineEventRunning, "test message")
}

func TestEngine_EmitWithTaskLoop(t *testing.T) {
	database, cfg, workDir, cleanup := createEngineDeps(t)
	defer cleanup()

	engine, err := NewEngine(EngineConfig{
		Config:  cfg,
		DB:      database,
		WorkDir: workDir,
	})
	if err != nil {
		t.Fatalf("NewEngine failed: %v", err)
	}

	taskLoopEvent := &TaskLoopEvent{
		Type:      TaskEventTaskBegin,
		TaskIndex: 0,
		TaskTitle: "Test Task",
		Message:   "task message",
	}

	// Emit an event
	go func() {
		engine.emitWithTaskLoop(taskLoopEvent)
	}()

	// Read the event
	select {
	case event := <-engine.Events():
		if event.Type != EngineEventRunning {
			t.Errorf("expected event type %s, got %s", EngineEventRunning, event.Type)
		}
		if event.TaskLoopEvent == nil {
			t.Error("expected TaskLoopEvent to be set")
		}
		if event.TaskLoopEvent.Type != TaskEventTaskBegin {
			t.Errorf("expected nested event type %s, got %s", TaskEventTaskBegin, event.TaskLoopEvent.Type)
		}
	case <-time.After(100 * time.Millisecond):
		t.Error("timeout waiting for event")
	}
}

func TestEngine_Accessors(t *testing.T) {
	database, cfg, workDir, cleanup := createEngineDeps(t)
	defer cleanup()

	engine, err := NewEngine(EngineConfig{
		Config:  cfg,
		DB:      database,
		WorkDir: workDir,
	})
	if err != nil {
		t.Fatalf("NewEngine failed: %v", err)
	}

	if engine.DB() != database {
		t.Error("DB() returned wrong value")
	}
	if engine.Claude() == nil {
		t.Error("Claude() returned nil")
	}
	if engine.JJ() == nil {
		t.Error("JJ() returned nil")
	}
	if engine.Agents() == nil {
		t.Error("Agents() returned nil")
	}
}

func TestParsePlannerOutput(t *testing.T) {
	testCases := []struct {
		name          string
		input         string
		expectedCount int
		expectError   bool
	}{
		{
			name: "valid JSON array",
			input: `Some text before
[
  {"title": "Task 1", "description": "Do task 1", "sequence": 1},
  {"title": "Task 2", "description": "Do task 2", "sequence": 2}
]
Some text after`,
			expectedCount: 2,
			expectError:   false,
		},
		{
			name:          "no JSON array",
			input:         "This is just some text without JSON",
			expectedCount: 0,
			expectError:   true,
		},
		{
			name:          "empty array",
			input:         "[]",
			expectedCount: 0,
			expectError:   true,
		},
		{
			name:          "malformed JSON",
			input:         "[{broken json}]",
			expectedCount: 0,
			expectError:   true,
		},
		{
			name:          "single task",
			input:         `[{"title": "Only Task", "description": "The only task", "sequence": 1}]`,
			expectedCount: 1,
			expectError:   false,
		},
		{
			name:          "only opening bracket",
			input:         "[",
			expectedCount: 0,
			expectError:   true,
		},
		{
			name:          "only closing bracket",
			input:         "]",
			expectedCount: 0,
			expectError:   true,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			tasks, err := parsePlannerOutput(tc.input)

			if tc.expectError {
				if err == nil {
					t.Error("expected error but got none")
				}
			} else {
				if err != nil {
					t.Errorf("unexpected error: %v", err)
				}
				if len(tasks) != tc.expectedCount {
					t.Errorf("expected %d tasks, got %d", tc.expectedCount, len(tasks))
				}
			}
		})
	}
}

func TestParsePlannerOutput_TaskFields(t *testing.T) {
	input := `[{"title": "Test Title", "description": "Test Description", "sequence": 42}]`

	tasks, err := parsePlannerOutput(input)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(tasks) != 1 {
		t.Fatalf("expected 1 task, got %d", len(tasks))
	}

	task := tasks[0]
	if task.Title != "Test Title" {
		t.Errorf("expected title %q, got %q", "Test Title", task.Title)
	}
	if task.Description != "Test Description" {
		t.Errorf("expected description %q, got %q", "Test Description", task.Description)
	}
	if task.Sequence != 42 {
		t.Errorf("expected sequence %d, got %d", 42, task.Sequence)
	}
}

func TestEngineEventTypes(t *testing.T) {
	// Ensure all event types are distinct
	eventTypes := []EngineEventType{
		EngineEventCreatingProject,
		EngineEventPlanningTasks,
		EngineEventTasksCreated,
		EngineEventRunning,
		EngineEventCompleted,
		EngineEventFailed,
	}

	seen := make(map[EngineEventType]bool)
	for _, et := range eventTypes {
		if seen[et] {
			t.Errorf("duplicate event type: %s", et)
		}
		seen[et] = true
	}
}

func TestEngineEvent_Structure(t *testing.T) {
	taskLoopEvent := &TaskLoopEvent{Type: TaskEventStarted}
	event := EngineEvent{
		Type:          EngineEventRunning,
		Message:       "test message",
		TaskLoopEvent: taskLoopEvent,
	}

	if event.Type != EngineEventRunning {
		t.Error("Type field not set correctly")
	}
	if event.Message != "test message" {
		t.Error("Message field not set correctly")
	}
	if event.TaskLoopEvent != taskLoopEvent {
		t.Error("TaskLoopEvent field not set correctly")
	}
}

func TestEngineConfig_Structure(t *testing.T) {
	cfg := EngineConfig{
		Config:  config.DefaultConfig(),
		DB:      nil,
		WorkDir: "/test/path",
	}

	if cfg.Config == nil {
		t.Error("Config field not set")
	}
	if cfg.WorkDir != "/test/path" {
		t.Errorf("WorkDir field not set correctly: %s", cfg.WorkDir)
	}
}

func TestEngine_ConcurrentProjectAccess(t *testing.T) {
	database, cfg, workDir, cleanup := createEngineDeps(t)
	defer cleanup()

	engine, err := NewEngine(EngineConfig{
		Config:  cfg,
		DB:      database,
		WorkDir: workDir,
	})
	if err != nil {
		t.Fatalf("NewEngine failed: %v", err)
	}

	// Test concurrent access to Project()
	done := make(chan bool)
	go func() {
		for i := 0; i < 100; i++ {
			_ = engine.Project()
		}
		done <- true
	}()
	go func() {
		for i := 0; i < 100; i++ {
			_ = engine.Project()
		}
		done <- true
	}()

	<-done
	<-done
	// If we get here without a race condition, the test passes
}

func TestEngine_CreateProject_FileNotFound(t *testing.T) {
	database, cfg, workDir, cleanup := createEngineDeps(t)
	defer cleanup()

	engine, err := NewEngine(EngineConfig{
		Config:  cfg,
		DB:      database,
		WorkDir: workDir,
	})
	if err != nil {
		t.Fatalf("NewEngine failed: %v", err)
	}

	// Drain events
	go func() {
		for range engine.Events() {
		}
	}()

	_, err = engine.CreateProject(context.Background(), "/nonexistent/path/plan.md")
	if err == nil {
		t.Error("expected error for non-existent file")
	}
}

func TestEngine_CreateProject_EmitCreatingProject(t *testing.T) {
	database, cfg, workDir, cleanup := createEngineDeps(t)
	defer cleanup()

	// Create a temp plan file
	planPath := filepath.Join(workDir, "plan.md")
	if err := os.WriteFile(planPath, []byte("# Test Plan\nDo something"), 0644); err != nil {
		t.Fatalf("failed to write plan file: %v", err)
	}

	engine, err := NewEngine(EngineConfig{
		Config:  cfg,
		DB:      database,
		WorkDir: workDir,
	})
	if err != nil {
		t.Fatalf("NewEngine failed: %v", err)
	}

	// Collect first event
	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()

	eventReceived := make(chan EngineEvent, 10)
	go func() {
		for event := range engine.Events() {
			eventReceived <- event
		}
	}()

	// CreateProject will fail on planner (no Claude CLI), but should emit creating event first
	engine.CreateProject(ctx, planPath)

	// Should have received at least the creating project event
	select {
	case event := <-eventReceived:
		if event.Type != EngineEventCreatingProject {
			t.Errorf("expected first event to be %s, got %s", EngineEventCreatingProject, event.Type)
		}
	case <-time.After(100 * time.Millisecond):
		t.Error("timeout waiting for creating project event")
	}
}

func TestPlannerTask_Structure(t *testing.T) {
	task := plannerTask{
		Title:       "Test Task",
		Description: "Task description",
		Sequence:    1,
	}

	if task.Title != "Test Task" {
		t.Error("Title field not set correctly")
	}
	if task.Description != "Task description" {
		t.Error("Description field not set correctly")
	}
	if task.Sequence != 1 {
		t.Error("Sequence field not set correctly")
	}
}

// =============================================================================
// Learnings Capture Tests
// =============================================================================

func TestParseLearningsOutput_BothSections(t *testing.T) {
	input := `Some intro text

### AGENTS.md Content
` + "```markdown" + `
## Coding Conventions
- Use camelCase for variables
` + "```" + `

### README.md Content
` + "```markdown" + `
## New Features
- Added learnings capture
` + "```" + `

Some trailing text`

	learnings, err := parseLearningsOutput(input)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if learnings.AgentsMD == "" {
		t.Error("expected AgentsMD to be populated")
	}
	if learnings.ReadmeMD == "" {
		t.Error("expected ReadmeMD to be populated")
	}

	// Check content (trimmed)
	if !contains(learnings.AgentsMD, "Coding Conventions") {
		t.Errorf("AgentsMD content incorrect: %q", learnings.AgentsMD)
	}
	if !contains(learnings.ReadmeMD, "New Features") {
		t.Errorf("ReadmeMD content incorrect: %q", learnings.ReadmeMD)
	}
}

func TestParseLearningsOutput_OnlyAgents(t *testing.T) {
	input := `### AGENTS.md Content
` + "```markdown" + `
## Patterns
- Pattern A
` + "```" + ``

	learnings, err := parseLearningsOutput(input)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if learnings.AgentsMD == "" {
		t.Error("expected AgentsMD to be populated")
	}
	if learnings.ReadmeMD != "" {
		t.Errorf("expected empty ReadmeMD, got: %q", learnings.ReadmeMD)
	}
}

func TestParseLearningsOutput_OnlyReadme(t *testing.T) {
	input := `### README.md Content
` + "```markdown" + `
## Documentation
- Doc section
` + "```" + ``

	learnings, err := parseLearningsOutput(input)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if learnings.AgentsMD != "" {
		t.Errorf("expected empty AgentsMD, got: %q", learnings.AgentsMD)
	}
	if learnings.ReadmeMD == "" {
		t.Error("expected ReadmeMD to be populated")
	}
}

func TestParseLearningsOutput_NoSections(t *testing.T) {
	input := `Just some random text without any proper sections`

	learnings, err := parseLearningsOutput(input)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if learnings.AgentsMD != "" {
		t.Errorf("expected empty AgentsMD, got: %q", learnings.AgentsMD)
	}
	if learnings.ReadmeMD != "" {
		t.Errorf("expected empty ReadmeMD, got: %q", learnings.ReadmeMD)
	}
}

func TestExtractCodeBlock(t *testing.T) {
	testCases := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name:     "markdown fence",
			input:    "text\n```markdown\ncontent here\n```\nmore",
			expected: "content here",
		},
		{
			name:     "plain fence",
			input:    "text\n```\nplain content\n```\nmore",
			expected: "plain content",
		},
		{
			name:     "no fence",
			input:    "just text without fence",
			expected: "",
		},
		{
			name:     "unclosed fence",
			input:    "```markdown\nunclosed",
			expected: "",
		},
		{
			name:     "empty block",
			input:    "```markdown\n```",
			expected: "",
		},
		{
			name:     "multiline content",
			input:    "```markdown\nline1\nline2\nline3\n```",
			expected: "line1\nline2\nline3",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			result := extractCodeBlock(tc.input)
			if result != tc.expected {
				t.Errorf("expected %q, got %q", tc.expected, result)
			}
		})
	}
}

func TestFilterCompleted(t *testing.T) {
	tasks := []*db.Task{
		{ID: "1", Status: db.TaskCompleted},
		{ID: "2", Status: db.TaskPending},
		{ID: "3", Status: db.TaskCompleted},
		{ID: "4", Status: db.TaskFailed},
		{ID: "5", Status: db.TaskInProgress},
	}

	completed := filterCompleted(tasks)

	if len(completed) != 2 {
		t.Errorf("expected 2 completed tasks, got %d", len(completed))
	}

	// Check the correct tasks are included
	ids := make(map[string]bool)
	for _, t := range completed {
		ids[t.ID] = true
	}

	if !ids["1"] || !ids["3"] {
		t.Error("expected tasks 1 and 3 to be in completed list")
	}
}

func TestFilterCompleted_Empty(t *testing.T) {
	tasks := []*db.Task{}
	completed := filterCompleted(tasks)

	if len(completed) != 0 {
		t.Errorf("expected 0 completed tasks, got %d", len(completed))
	}
}

func TestFilterCompleted_NoneCompleted(t *testing.T) {
	tasks := []*db.Task{
		{ID: "1", Status: db.TaskPending},
		{ID: "2", Status: db.TaskFailed},
	}

	completed := filterCompleted(tasks)

	if len(completed) != 0 {
		t.Errorf("expected 0 completed tasks, got %d", len(completed))
	}
}

func TestLearningsOutput_Structure(t *testing.T) {
	output := LearningsOutput{
		AgentsMD: "agents content",
		ReadmeMD: "readme content",
	}

	if output.AgentsMD != "agents content" {
		t.Error("AgentsMD field not set correctly")
	}
	if output.ReadmeMD != "readme content" {
		t.Error("ReadmeMD field not set correctly")
	}
}

func TestEngineEventTypes_IncludesLearnings(t *testing.T) {
	// Ensure learnings event types are distinct
	eventTypes := []EngineEventType{
		EngineEventCreatingProject,
		EngineEventPlanningTasks,
		EngineEventTasksCreated,
		EngineEventRunning,
		EngineEventCompleted,
		EngineEventFailed,
		EngineEventCapturingLearnings,
		EngineEventLearningsCaptured,
	}

	seen := make(map[EngineEventType]bool)
	for _, et := range eventTypes {
		if seen[et] {
			t.Errorf("duplicate event type: %s", et)
		}
		seen[et] = true
	}

	// Ensure we have the learnings event types
	if !seen[EngineEventCapturingLearnings] {
		t.Error("missing EngineEventCapturingLearnings")
	}
	if !seen[EngineEventLearningsCaptured] {
		t.Error("missing EngineEventLearningsCaptured")
	}
}

func TestEngine_CaptureLearnings_NoProject(t *testing.T) {
	database, cfg, workDir, cleanup := createEngineDeps(t)
	defer cleanup()

	engine, err := NewEngine(EngineConfig{
		Config:  cfg,
		DB:      database,
		WorkDir: workDir,
	})
	if err != nil {
		t.Fatalf("NewEngine failed: %v", err)
	}

	// Drain events
	go func() {
		for range engine.Events() {
		}
	}()

	err = engine.CaptureLearnings(context.Background())
	if err == nil {
		t.Error("expected error when capturing learnings without project")
	}
}

// Helper function for string contains check
func contains(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(s) > 0 && containsAt(s, substr, 0))
}

func containsAt(s, substr string, start int) bool {
	for i := start; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
