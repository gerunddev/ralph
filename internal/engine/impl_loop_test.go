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

// Helper to create test dependencies
func createTestDeps(t *testing.T) (ImplLoopDeps, func()) {
	// Create in-memory database
	database, err := db.New(":memory:")
	if err != nil {
		t.Fatalf("failed to create test database: %v", err)
	}

	// Create a test project and task
	project := &db.Project{
		ID:       "test-project",
		Name:     "Test Project",
		PlanText: "Test plan",
		Status:   db.ProjectInProgress,
	}
	if err := database.CreateProject(project); err != nil {
		t.Fatalf("failed to create test project: %v", err)
	}

	cfg := config.DefaultConfig()

	deps := ImplLoopDeps{
		DB:     database,
		Claude: claude.NewClient(claude.ClientConfig{Model: "test"}),
		JJ:     jj.NewClient("/tmp/test"),
		Agents: agents.NewManager(cfg),
		Config: cfg,
	}

	cleanup := func() {
		database.Close()
	}

	return deps, cleanup
}

// Helper to create a test task
func createTestTask(t *testing.T, database *db.DB, projectID string) *db.Task {
	task := &db.Task{
		ID:          "test-task",
		ProjectID:   projectID,
		Sequence:    1,
		Title:       "Test Task",
		Description: "Test task description",
		Status:      db.TaskPending,
	}
	if err := database.CreateTask(task); err != nil {
		t.Fatalf("failed to create test task: %v", err)
	}
	return task
}

func TestNewImplLoop(t *testing.T) {
	deps, cleanup := createTestDeps(t)
	defer cleanup()

	task := createTestTask(t, deps.DB, "test-project")
	plan := "Test plan"

	il := NewImplLoop(deps, task, plan)

	if il == nil {
		t.Fatal("NewImplLoop returned nil")
	}
	if il.Status() != ImplLoopStatusPending {
		t.Errorf("expected status %s, got %s", ImplLoopStatusPending, il.Status())
	}
	if il.task != task {
		t.Error("task not set correctly")
	}
	if il.plan != plan {
		t.Error("plan not set correctly")
	}
}

func TestParseReviewerOutput_Approved(t *testing.T) {
	testCases := []struct {
		name     string
		input    string
		approved bool
	}{
		{"simple approved", "APPROVED", true},
		{"approved with text", "The code looks good. APPROVED", true},
		{"approved lowercase", "approved", true},
		{"approved mixed case", "Approved", true},
		{"not approved", "NOT APPROVED - needs work", false},
		{"not yet approved", "NOT YET APPROVED", false},
		{"cannot be approved", "This CANNOT BE APPROVED until fixed", false},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			result := parseReviewerOutput(tc.input)
			if result.Approved != tc.approved {
				t.Errorf("expected approved=%v, got %v", tc.approved, result.Approved)
			}
		})
	}
}

func TestParseReviewerOutput_Feedback(t *testing.T) {
	testCases := []struct {
		name             string
		input            string
		expectedFeedback string
	}{
		{
			"feedback with prefix",
			"FEEDBACK: Please add error handling",
			"Please add error handling",
		},
		{
			"feedback lowercase prefix",
			"feedback: needs tests",
			"needs tests",
		},
		{
			"feedback mixed case",
			"Feedback: update the docs",
			"update the docs",
		},
		{
			"no prefix defaults to full text",
			"Please fix the bug on line 42",
			"Please fix the bug on line 42",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			result := parseReviewerOutput(tc.input)
			if result.Approved {
				t.Error("expected approved=false")
			}
			if result.Feedback != tc.expectedFeedback {
				t.Errorf("expected feedback %q, got %q", tc.expectedFeedback, result.Feedback)
			}
		})
	}
}

func TestImplLoop_Events(t *testing.T) {
	deps, cleanup := createTestDeps(t)
	defer cleanup()

	task := createTestTask(t, deps.DB, "test-project")
	il := NewImplLoop(deps, task, "Test plan")

	events := il.Events()
	if events == nil {
		t.Fatal("Events() returned nil channel")
	}
}

func TestImplLoop_StatusUpdates(t *testing.T) {
	deps, cleanup := createTestDeps(t)
	defer cleanup()

	task := createTestTask(t, deps.DB, "test-project")
	il := NewImplLoop(deps, task, "Test plan")

	// Initial status
	if il.Status() != ImplLoopStatusPending {
		t.Errorf("expected initial status %s, got %s", ImplLoopStatusPending, il.Status())
	}

	// Test setStatus
	il.setStatus(ImplLoopStatusRunning)
	if il.Status() != ImplLoopStatusRunning {
		t.Errorf("expected status %s after setStatus, got %s", ImplLoopStatusRunning, il.Status())
	}
}

func TestImplLoop_EmitEvents(t *testing.T) {
	deps, cleanup := createTestDeps(t)
	defer cleanup()

	task := createTestTask(t, deps.DB, "test-project")
	il := NewImplLoop(deps, task, "Test plan")

	// Start consuming events
	go func() {
		il.iteration = 1
		il.emit(EventStarted, "test message")
	}()

	// Read the event
	select {
	case event := <-il.Events():
		if event.Type != EventStarted {
			t.Errorf("expected event type %s, got %s", EventStarted, event.Type)
		}
		if event.Message != "test message" {
			t.Errorf("expected message %q, got %q", "test message", event.Message)
		}
		if event.Iteration != 1 {
			t.Errorf("expected iteration 1, got %d", event.Iteration)
		}
	case <-time.After(100 * time.Millisecond):
		t.Error("timeout waiting for event")
	}
}

func TestImplLoop_Fail(t *testing.T) {
	deps, cleanup := createTestDeps(t)
	defer cleanup()

	task := createTestTask(t, deps.DB, "test-project")
	il := NewImplLoop(deps, task, "Test plan")

	// Drain events in background
	go func() {
		for range il.Events() {
		}
	}()

	testErr := errors.New("test error")
	err := il.fail(testErr)

	if err != testErr {
		t.Errorf("expected error %v, got %v", testErr, err)
	}
	if il.Status() != ImplLoopStatusFailed {
		t.Errorf("expected status %s, got %s", ImplLoopStatusFailed, il.Status())
	}
}

func TestImplLoop_ContextCancellation(t *testing.T) {
	deps, cleanup := createTestDeps(t)
	defer cleanup()

	task := createTestTask(t, deps.DB, "test-project")
	il := NewImplLoop(deps, task, "Test plan")

	// Cancel context immediately
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	// Drain events
	go func() {
		for range il.Events() {
		}
	}()

	err := il.Run(ctx)
	if !errors.Is(err, context.Canceled) {
		t.Errorf("expected context.Canceled error, got %v", err)
	}
	if il.Status() != ImplLoopStatusFailed {
		t.Errorf("expected status %s, got %s", ImplLoopStatusFailed, il.Status())
	}
}

func TestReviewResult_Structure(t *testing.T) {
	// Test that ReviewResult has the expected fields
	result := ReviewResult{
		Approved: true,
		Feedback: "test feedback",
	}

	if !result.Approved {
		t.Error("expected Approved to be true")
	}
	if result.Feedback != "test feedback" {
		t.Errorf("expected Feedback %q, got %q", "test feedback", result.Feedback)
	}
}

func TestEventTypes(t *testing.T) {
	// Ensure all event types are distinct
	eventTypes := []EventType{
		EventStarted,
		EventDeveloping,
		EventReviewing,
		EventFeedback,
		EventApproved,
		EventFailed,
	}

	seen := make(map[EventType]bool)
	for _, et := range eventTypes {
		if seen[et] {
			t.Errorf("duplicate event type: %s", et)
		}
		seen[et] = true
	}
}

func TestImplLoopStatus_Values(t *testing.T) {
	// Ensure all status values are distinct
	statuses := []ImplLoopStatus{
		ImplLoopStatusPending,
		ImplLoopStatusRunning,
		ImplLoopStatusApproved,
		ImplLoopStatusFailed,
		ImplLoopStatusEscalated,
	}

	seen := make(map[ImplLoopStatus]bool)
	for _, s := range statuses {
		if seen[s] {
			t.Errorf("duplicate status: %s", s)
		}
		seen[s] = true
	}
}

func TestImplLoopDeps_Structure(t *testing.T) {
	// Verify that ImplLoopDeps has all expected fields
	deps := ImplLoopDeps{
		DB:     nil,
		Claude: nil,
		JJ:     nil,
		Agents: nil,
		Config: nil,
	}

	// Just verifying the struct compiles with these fields
	_ = deps
}

func TestImplLoopEvent_Structure(t *testing.T) {
	event := ImplLoopEvent{
		Type:      EventStarted,
		Iteration: 1,
		Message:   "test message",
	}

	if event.Type != EventStarted {
		t.Error("Type field not set correctly")
	}
	if event.Iteration != 1 {
		t.Error("Iteration field not set correctly")
	}
	if event.Message != "test message" {
		t.Error("Message field not set correctly")
	}
}

func TestParseReviewerOutput_EdgeCases(t *testing.T) {
	testCases := []struct {
		name     string
		input    string
		approved bool
		feedback string
	}{
		{
			"empty string",
			"",
			false,
			"",
		},
		{
			"whitespace only",
			"   \n\t  ",
			false,
			"",
		},
		{
			"approved with newlines",
			"Code looks good.\n\nAPPROVED",
			true,
			"",
		},
		{
			"feedback with multiple lines",
			"FEEDBACK: Line 1\nLine 2\nLine 3",
			false,
			"Line 1\nLine 2\nLine 3",
		},
		{
			"approved at start",
			"APPROVED - great work!",
			true,
			"",
		},
		{
			"approved in middle",
			"The implementation is APPROVED for merge.",
			true,
			"",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			result := parseReviewerOutput(tc.input)
			if result.Approved != tc.approved {
				t.Errorf("expected approved=%v, got %v", tc.approved, result.Approved)
			}
			if !tc.approved && result.Feedback != tc.feedback {
				t.Errorf("expected feedback %q, got %q", tc.feedback, result.Feedback)
			}
		})
	}
}

func TestImplLoop_ConcurrentStatusAccess(t *testing.T) {
	deps, cleanup := createTestDeps(t)
	defer cleanup()

	task := createTestTask(t, deps.DB, "test-project")
	il := NewImplLoop(deps, task, "Test plan")

	// Test concurrent access to status
	done := make(chan bool)
	go func() {
		for i := 0; i < 100; i++ {
			il.setStatus(ImplLoopStatusRunning)
		}
		done <- true
	}()
	go func() {
		for i := 0; i < 100; i++ {
			_ = il.Status()
		}
		done <- true
	}()

	<-done
	<-done
	// If we get here without a race condition, the test passes
}
