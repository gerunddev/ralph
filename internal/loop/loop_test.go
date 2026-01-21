package loop

import (
	"context"
	"encoding/json"
	"os/exec"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/gerund/ralph/internal/claude"
	"github.com/gerund/ralph/internal/db"
	"github.com/gerund/ralph/internal/distill"
	"github.com/gerund/ralph/internal/jj"
)

// setupTestDB creates an in-memory database for testing.
func setupTestDB(t *testing.T) *db.DB {
	t.Helper()
	database, err := db.New(":memory:")
	if err != nil {
		t.Fatalf("failed to create test db: %v", err)
	}
	t.Cleanup(func() {
		if err := database.Close(); err != nil {
			t.Errorf("failed to close db: %v", err)
		}
	})
	return database
}

// createTestPlan creates a test plan in the database.
func createTestPlan(t *testing.T, database *db.DB, content string) *db.Plan {
	t.Helper()
	plan := &db.Plan{
		ID:         uuid.New().String(),
		OriginPath: "/test/plan.md",
		Content:    content,
		Status:     db.PlanStatusPending,
	}
	if err := database.CreatePlan(plan); err != nil {
		t.Fatalf("failed to create test plan: %v", err)
	}
	return plan
}

// mockClaudeCreator creates a command creator that simulates Claude output.
func mockClaudeCreator(output string) claude.CommandCreator {
	return func(ctx context.Context, name string, args ...string) *exec.Cmd {
		// Create a command that outputs the specified JSON
		jsonOutput := createMockClaudeOutput(output)
		return exec.CommandContext(ctx, "echo", jsonOutput)
	}
}

// createMockClaudeOutput creates mock Claude stream-json output.
func createMockClaudeOutput(text string) string {
	// Create init event
	initEvent := map[string]interface{}{
		"type":       "init",
		"session_id": "test-session-123",
		"model":      "test-model",
		"cwd":        "/test",
	}
	initJSON, _ := json.Marshal(initEvent)

	// Create message event with the text
	messageEvent := map[string]interface{}{
		"message": map[string]interface{}{
			"id":          "msg-123",
			"role":        "assistant",
			"model":       "test-model",
			"stop_reason": "end_turn",
			"content": []map[string]interface{}{
				{"type": "text", "text": text},
			},
		},
	}
	messageJSON, _ := json.Marshal(messageEvent)

	// Create result event
	resultEvent := map[string]interface{}{
		"type":       "result",
		"session_id": "test-session-123",
		"result":     text,
		"cost_usd":   0.001,
		"num_turns":  1,
	}
	resultJSON, _ := json.Marshal(resultEvent)

	return string(initJSON) + "\n" + string(messageJSON) + "\n" + string(resultJSON)
}

// mockJJRunner creates a jj command runner that does nothing.
func mockJJRunner() jj.CommandRunner {
	return func(ctx context.Context, dir string, name string, args ...string) (string, string, error) {
		return "", "", nil
	}
}

// mockDistillerCreator creates a command creator for distillation that returns a fixed message.
func mockDistillerCreator(msg string) claude.CommandCreator {
	return func(ctx context.Context, name string, args ...string) *exec.Cmd {
		// Create a simple output that will be parsed as the commit message
		output := createMockClaudeOutput(msg)
		return exec.CommandContext(ctx, "echo", output)
	}
}

func TestLoopBasicIteration(t *testing.T) {
	database := setupTestDB(t)
	plan := createTestPlan(t, database, "Test plan content")

	// Create mock Claude client that outputs progress/learnings
	claudeClient := claude.NewClient(claude.ClientConfig{
		Model:    "test",
		MaxTurns: 1,
	})
	claudeClient.SetCommandCreator(mockClaudeCreator("## Progress\nDid some work\n\n## Learnings\nLearned something"))

	// Create mock distiller
	distillerClient := claude.NewClient(claude.ClientConfig{
		Model:    "haiku",
		MaxTurns: 1,
	})
	distillerClient.SetCommandCreator(mockDistillerCreator("test: update implementation"))
	testDistiller := distill.NewDistiller(distillerClient)

	// Create mock jj client
	jjClient := jj.NewClient("/tmp")
	jjClient.SetCommandRunner(mockJJRunner())

	// Create loop with max 1 iteration (so it stops)
	loop := New(Config{
		PlanID:        plan.ID,
		MaxIterations: 1,
		WorkDir:       "/tmp",
	}, Deps{
		DB:        database,
		Claude:    claudeClient,
		Distiller: testDistiller,
		JJ:        jjClient,
	})

	// Run with timeout
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	// Collect events with proper synchronization
	var events []Event
	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		defer wg.Done()
		for event := range loop.Events() {
			events = append(events, event)
		}
	}()

	// Run the loop
	err := loop.Run(ctx)
	if err != nil {
		t.Fatalf("loop.Run() error: %v", err)
	}

	// Wait for event collection to complete (channel closes when Run returns)
	wg.Wait()

	// Verify events were emitted
	if len(events) == 0 {
		t.Error("expected events to be emitted")
	}

	// Verify we got a max iterations event
	var foundMaxIter bool
	for _, e := range events {
		if e.Type == EventMaxIterations {
			foundMaxIter = true
			break
		}
	}
	if !foundMaxIter {
		t.Error("expected EventMaxIterations event")
	}

	// Verify progress was stored
	progress, err := database.GetLatestProgress(plan.ID)
	if err != nil {
		t.Fatalf("failed to get progress: %v", err)
	}
	if progress == nil {
		t.Error("expected progress to be stored")
	} else if !strings.Contains(progress.Content, "Did some work") {
		t.Errorf("expected progress to contain 'Did some work', got: %s", progress.Content)
	}

	// Verify learnings was stored
	learnings, err := database.GetLatestLearnings(plan.ID)
	if err != nil {
		t.Fatalf("failed to get learnings: %v", err)
	}
	if learnings == nil {
		t.Error("expected learnings to be stored")
	} else if !strings.Contains(learnings.Content, "Learned something") {
		t.Errorf("expected learnings to contain 'Learned something', got: %s", learnings.Content)
	}
}

func TestLoopDoneMarker(t *testing.T) {
	database := setupTestDB(t)
	plan := createTestPlan(t, database, "Test plan content")

	// Create mock Claude client that outputs "DONE DONE DONE!!!"
	claudeClient := claude.NewClient(claude.ClientConfig{
		Model:    "test",
		MaxTurns: 1,
	})
	claudeClient.SetCommandCreator(mockClaudeCreator("DONE DONE DONE!!!"))

	// Create mock distiller
	distillerClient := claude.NewClient(claude.ClientConfig{
		Model:    "haiku",
		MaxTurns: 1,
	})
	distillerClient.SetCommandCreator(mockDistillerCreator("Complete implementation"))
	testDistiller := distill.NewDistiller(distillerClient)

	// Create mock jj client
	jjClient := jj.NewClient("/tmp")
	jjClient.SetCommandRunner(mockJJRunner())

	// Create loop with high max iterations
	loop := New(Config{
		PlanID:        plan.ID,
		MaxIterations: 100,
		WorkDir:       "/tmp",
	}, Deps{
		DB:        database,
		Claude:    claudeClient,
		Distiller: testDistiller,
		JJ:        jjClient,
	})

	// Run with timeout
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	// Collect events with proper synchronization
	var events []Event
	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		defer wg.Done()
		for event := range loop.Events() {
			events = append(events, event)
		}
	}()

	// Run the loop
	err := loop.Run(ctx)
	if err != nil {
		t.Fatalf("loop.Run() error: %v", err)
	}

	// Wait for event collection to complete (channel closes when Run returns)
	wg.Wait()

	// Verify we got a done event
	var foundDone bool
	for _, e := range events {
		if e.Type == EventDone {
			foundDone = true
			break
		}
	}
	if !foundDone {
		t.Error("expected EventDone event")
	}

	// Verify plan was marked complete
	updatedPlan, err := database.GetPlan(plan.ID)
	if err != nil {
		t.Fatalf("failed to get plan: %v", err)
	}
	if updatedPlan.Status != db.PlanStatusCompleted {
		t.Errorf("expected plan status 'completed', got: %s", updatedPlan.Status)
	}

	// Verify iteration is 1 (only ran once)
	if loop.CurrentIteration() != 1 {
		t.Errorf("expected iteration 1, got: %d", loop.CurrentIteration())
	}
}

func TestLoopContextCancellation(t *testing.T) {
	database := setupTestDB(t)
	plan := createTestPlan(t, database, "Test plan content")

	// Create mock Claude client with slow response
	claudeClient := claude.NewClient(claude.ClientConfig{
		Model:    "test",
		MaxTurns: 1,
	})
	claudeClient.SetCommandCreator(func(ctx context.Context, name string, args ...string) *exec.Cmd {
		// Sleep command to simulate slow Claude
		return exec.CommandContext(ctx, "sleep", "10")
	})

	// Create mock distiller (won't be reached)
	distillerClient := claude.NewClient(claude.ClientConfig{Model: "haiku", MaxTurns: 1})
	distillerClient.SetCommandCreator(mockDistillerCreator("test"))
	testDistiller := distill.NewDistiller(distillerClient)

	// Create mock jj client
	jjClient := jj.NewClient("/tmp")
	jjClient.SetCommandRunner(mockJJRunner())

	// Create loop
	loop := New(Config{
		PlanID:        plan.ID,
		MaxIterations: 100,
		WorkDir:       "/tmp",
	}, Deps{
		DB:        database,
		Claude:    claudeClient,
		Distiller: testDistiller,
		JJ:        jjClient,
	})

	// Run with very short timeout
	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()

	// Drain events
	go func() {
		for range loop.Events() {
		}
	}()

	// Run the loop - should return context error
	err := loop.Run(ctx)
	if err == nil {
		t.Error("expected error from context cancellation")
	}
	if err != context.DeadlineExceeded {
		t.Errorf("expected context.DeadlineExceeded, got: %v", err)
	}
}

func TestLoopResume(t *testing.T) {
	database := setupTestDB(t)
	plan := createTestPlan(t, database, "Test plan content")

	// Simulate a previous session at iteration 5
	previousSession := &db.PlanSession{
		ID:          uuid.New().String(),
		PlanID:      plan.ID,
		Iteration:   5,
		InputPrompt: "previous prompt",
		Status:      db.PlanSessionCompleted,
	}
	if err := database.CreatePlanSession(previousSession); err != nil {
		t.Fatalf("failed to create previous session: %v", err)
	}

	// Store previous progress
	previousProgress := &db.Progress{
		PlanID:    plan.ID,
		SessionID: previousSession.ID,
		Content:   "Previous progress content",
	}
	if err := database.CreateProgress(previousProgress); err != nil {
		t.Fatalf("failed to create previous progress: %v", err)
	}

	// Create mock Claude client that outputs done
	claudeClient := claude.NewClient(claude.ClientConfig{
		Model:    "test",
		MaxTurns: 1,
	})
	claudeClient.SetCommandCreator(mockClaudeCreator("DONE DONE DONE!!!"))

	// Create mock distiller
	distillerClient := claude.NewClient(claude.ClientConfig{Model: "haiku", MaxTurns: 1})
	distillerClient.SetCommandCreator(mockDistillerCreator("Complete"))
	testDistiller := distill.NewDistiller(distillerClient)

	// Create mock jj client
	jjClient := jj.NewClient("/tmp")
	jjClient.SetCommandRunner(mockJJRunner())

	// Create loop - should resume from iteration 5
	loop := New(Config{
		PlanID:        plan.ID,
		MaxIterations: 100,
		WorkDir:       "/tmp",
	}, Deps{
		DB:        database,
		Claude:    claudeClient,
		Distiller: testDistiller,
		JJ:        jjClient,
	})

	// Run
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	// Drain events
	go func() {
		for range loop.Events() {
		}
	}()

	err := loop.Run(ctx)
	if err != nil {
		t.Fatalf("loop.Run() error: %v", err)
	}

	// Verify iteration is 6 (resumed from 5, did one more)
	if loop.CurrentIteration() != 6 {
		t.Errorf("expected iteration 6 (resumed from 5), got: %d", loop.CurrentIteration())
	}
}

func TestLoopEventTypes(t *testing.T) {
	database := setupTestDB(t)
	plan := createTestPlan(t, database, "Test plan content")

	// Create mock Claude client
	claudeClient := claude.NewClient(claude.ClientConfig{
		Model:    "test",
		MaxTurns: 1,
	})
	claudeClient.SetCommandCreator(mockClaudeCreator("DONE DONE DONE!!!"))

	// Create mock distiller
	distillerClient := claude.NewClient(claude.ClientConfig{Model: "haiku", MaxTurns: 1})
	distillerClient.SetCommandCreator(mockDistillerCreator("test commit"))
	testDistiller := distill.NewDistiller(distillerClient)

	// Create mock jj client
	jjClient := jj.NewClient("/tmp")
	jjClient.SetCommandRunner(mockJJRunner())

	// Create loop
	loop := New(Config{
		PlanID:        plan.ID,
		MaxIterations: 100,
		WorkDir:       "/tmp",
	}, Deps{
		DB:        database,
		Claude:    claudeClient,
		Distiller: testDistiller,
		JJ:        jjClient,
	})

	// Run
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	// Collect events with proper synchronization
	var events []Event
	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		defer wg.Done()
		for event := range loop.Events() {
			events = append(events, event)
		}
	}()

	err := loop.Run(ctx)
	if err != nil {
		t.Fatalf("loop.Run() error: %v", err)
	}

	// Wait for event collection to complete (channel closes when Run returns)
	wg.Wait()

	// Check we received expected event types
	expectedTypes := map[EventType]bool{
		EventStarted:        false,
		EventIterationStart: false,
		EventJJNew:          false,
		EventPromptBuilt:    false,
		EventClaudeStart:    false,
		EventClaudeEnd:      false,
		EventParsed:         false,
		EventDistilling:     false,
		EventJJCommit:       false,
		EventDone:           false,
	}

	for _, e := range events {
		if _, ok := expectedTypes[e.Type]; ok {
			expectedTypes[e.Type] = true
		}
	}

	for eventType, found := range expectedTypes {
		if !found {
			t.Errorf("expected event type %s was not emitted", eventType)
		}
	}
}

func TestNewEvent(t *testing.T) {
	event := NewEvent(EventStarted, 1, 10, "test message")

	if event.Type != EventStarted {
		t.Errorf("expected type EventStarted, got: %s", event.Type)
	}
	if event.Iteration != 1 {
		t.Errorf("expected iteration 1, got: %d", event.Iteration)
	}
	if event.MaxIter != 10 {
		t.Errorf("expected max iter 10, got: %d", event.MaxIter)
	}
	if event.Message != "test message" {
		t.Errorf("expected message 'test message', got: %s", event.Message)
	}
}

func TestNewErrorEvent(t *testing.T) {
	err := context.Canceled
	event := NewErrorEvent(5, 10, err)

	if event.Type != EventError {
		t.Errorf("expected type EventError, got: %s", event.Type)
	}
	if event.Error != err {
		t.Errorf("expected error to be context.Canceled")
	}
	if event.Iteration != 5 {
		t.Errorf("expected iteration 5, got: %d", event.Iteration)
	}
}

func TestNewClaudeStreamEvent(t *testing.T) {
	claudeEvent := &claude.StreamEvent{
		Type: claude.EventMessage,
	}
	event := NewClaudeStreamEvent(3, 10, claudeEvent)

	if event.Type != EventClaudeStream {
		t.Errorf("expected type EventClaudeStream, got: %s", event.Type)
	}
	if event.ClaudeEvent != claudeEvent {
		t.Error("expected ClaudeEvent to be set")
	}
}

func TestNewPromptBuiltEvent(t *testing.T) {
	prompt := "Test prompt content"
	event := NewPromptBuiltEvent(2, 5, prompt)

	if event.Type != EventPromptBuilt {
		t.Errorf("expected type EventPromptBuilt, got: %s", event.Type)
	}
	if event.Iteration != 2 {
		t.Errorf("expected iteration 2, got: %d", event.Iteration)
	}
	if event.MaxIter != 5 {
		t.Errorf("expected maxIter 5, got: %d", event.MaxIter)
	}
	if event.Prompt != prompt {
		t.Errorf("expected prompt %q, got: %q", prompt, event.Prompt)
	}
	if event.Message != "Prompt built" {
		t.Errorf("expected message 'Prompt built', got: %q", event.Message)
	}
}

func TestNewClaudeOutputEvent(t *testing.T) {
	output := "Test output content"
	event := NewClaudeOutputEvent(3, 10, output)

	if event.Type != EventClaudeOutput {
		t.Errorf("expected type EventClaudeOutput, got: %s", event.Type)
	}
	if event.Iteration != 3 {
		t.Errorf("expected iteration 3, got: %d", event.Iteration)
	}
	if event.MaxIter != 10 {
		t.Errorf("expected maxIter 10, got: %d", event.MaxIter)
	}
	if event.Output != output {
		t.Errorf("expected output %q, got: %q", output, event.Output)
	}
	if event.Message != "Claude output collected" {
		t.Errorf("expected message 'Claude output collected', got: %q", event.Message)
	}
}
