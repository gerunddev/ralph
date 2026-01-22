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

	"github.com/gerunddev/ralph/internal/claude"
	"github.com/gerunddev/ralph/internal/db"
	"github.com/gerunddev/ralph/internal/distill"
	"github.com/gerunddev/ralph/internal/jj"
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

// mockJJRunner creates a jj command runner that handles common commands.
// It returns non-empty diff (so IsEmpty returns false), and a sample diff for show.
func mockJJRunner() jj.CommandRunner {
	return func(ctx context.Context, dir string, name string, args ...string) (string, string, error) {
		if len(args) >= 1 && args[0] == "diff" {
			// Return non-empty diff so jj new is called
			return "diff --git a/file.go b/file.go\n+// new code", "", nil
		}
		if len(args) >= 1 && args[0] == "show" {
			// Return a sample diff for distillation
			return "diff --git a/file.go b/file.go\n+// new code", "", nil
		}
		return "", "", nil
	}
}

// mockJJRunnerEmpty creates a jj command runner where IsEmpty returns true.
func mockJJRunnerEmpty() jj.CommandRunner {
	return func(ctx context.Context, dir string, name string, args ...string) (string, string, error) {
		if len(args) >= 1 && args[0] == "show" {
			return "", "", nil
		}
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

	// Track calls to differentiate developer vs reviewer
	callCount := 0

	// Create mock Claude client with DEV_DONE + REVIEWER_APPROVED sequence
	claudeClient := claude.NewClient(claude.ClientConfig{
		Model:    "test",
		MaxTurns: 1,
	})
	claudeClient.SetCommandCreator(func(ctx context.Context, name string, args ...string) *exec.Cmd {
		callCount++
		var output string
		if callCount == 1 {
			output = "## Progress\nCompleted\n\n## Status\nDEV_DONE DEV_DONE DEV_DONE!!!"
		} else {
			output = "## Progress\nReviewed\n\n### Critical Issues\nNone\n\n### Major Issues\nNone\n\n### Minor Issues\nNone\n\n### Verdict\nREVIEWER_APPROVED REVIEWER_APPROVED!!!"
		}
		jsonOutput := createMockClaudeOutput(output)
		return exec.CommandContext(ctx, "echo", jsonOutput)
	})

	// Create mock distiller
	distillerClient := claude.NewClient(claude.ClientConfig{
		Model:    "haiku",
		MaxTurns: 1,
	})
	distillerClient.SetCommandCreator(mockDistillerCreator("Complete implementation"))
	testDistiller := distill.NewDistiller(distillerClient)

	// Create mock jj client (empty so DEV_DONE is accepted)
	jjClient := jj.NewClient("/tmp")
	jjClient.SetCommandRunner(mockJJRunnerEmpty())

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

	// Track calls to differentiate developer vs reviewer
	callCount := 0

	// Create mock Claude client with DEV_DONE + REVIEWER_APPROVED sequence
	claudeClient := claude.NewClient(claude.ClientConfig{
		Model:    "test",
		MaxTurns: 1,
	})
	claudeClient.SetCommandCreator(func(ctx context.Context, name string, args ...string) *exec.Cmd {
		callCount++
		var output string
		if callCount == 1 {
			output = "## Progress\nCompleted\n\n## Status\nDEV_DONE DEV_DONE DEV_DONE!!!"
		} else {
			output = "## Progress\nReviewed\n\n### Critical Issues\nNone\n\n### Major Issues\nNone\n\n### Minor Issues\nNone\n\n### Verdict\nREVIEWER_APPROVED REVIEWER_APPROVED!!!"
		}
		jsonOutput := createMockClaudeOutput(output)
		return exec.CommandContext(ctx, "echo", jsonOutput)
	})

	// Create mock distiller
	distillerClient := claude.NewClient(claude.ClientConfig{Model: "haiku", MaxTurns: 1})
	distillerClient.SetCommandCreator(mockDistillerCreator("Complete"))
	testDistiller := distill.NewDistiller(distillerClient)

	// Create mock jj client (empty so DEV_DONE is accepted)
	jjClient := jj.NewClient("/tmp")
	jjClient.SetCommandRunner(mockJJRunnerEmpty())

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

	// Track calls to differentiate developer vs reviewer
	callCount := 0

	// Create mock Claude client that returns DEV_DONE then REVIEWER_APPROVED
	claudeClient := claude.NewClient(claude.ClientConfig{
		Model:    "test",
		MaxTurns: 1,
	})
	claudeClient.SetCommandCreator(func(ctx context.Context, name string, args ...string) *exec.Cmd {
		callCount++
		var output string
		if callCount == 1 {
			output = "## Progress\nCompleted\n\n## Learnings\nLearned\n\n## Status\nDEV_DONE DEV_DONE DEV_DONE!!!"
		} else {
			output = "## Progress\nReviewed\n\n## Learnings\nGood\n\n### Critical Issues\nNone\n\n### Major Issues\nNone\n\n### Minor Issues\nNone\n\n### Verdict\nREVIEWER_APPROVED REVIEWER_APPROVED!!!"
		}
		jsonOutput := createMockClaudeOutput(output)
		return exec.CommandContext(ctx, "echo", jsonOutput)
	})

	// Create mock distiller
	distillerClient := claude.NewClient(claude.ClientConfig{Model: "haiku", MaxTurns: 1})
	distillerClient.SetCommandCreator(mockDistillerCreator("test commit"))
	testDistiller := distill.NewDistiller(distillerClient)

	// Create mock jj client with diff so distilling phase runs and emits EventDistilling
	jjClient := jj.NewClient("/tmp")
	jjClient.SetCommandRunner(mockJJRunnerWithDiff("basechange123", "+func test() {}"))

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

	// Check we received expected event types for dual-agent loop
	expectedTypes := map[EventType]bool{
		EventStarted:          false,
		EventIterationStart:   false,
		EventDeveloperStart:   false,
		EventPromptBuilt:      false,
		EventClaudeStart:      false,
		EventClaudeEnd:        false,
		EventDeveloperEnd:     false,
		EventDeveloperDone:    false,
		EventReviewerStart:    false,
		EventReviewerEnd:      false,
		EventReviewerApproved: false,
		EventBothDone:         false,
		EventDistilling:       false,
		EventDone:             false,
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

func TestLoopSkipsJJNewWhenEmpty(t *testing.T) {
	database := setupTestDB(t)
	plan := createTestPlan(t, database, "Test plan content")

	// Track calls to differentiate developer vs reviewer
	callCount := 0

	// Create mock Claude client with DEV_DONE + REVIEWER_APPROVED sequence
	claudeClient := claude.NewClient(claude.ClientConfig{
		Model:    "test",
		MaxTurns: 1,
	})
	claudeClient.SetCommandCreator(func(ctx context.Context, name string, args ...string) *exec.Cmd {
		callCount++
		var output string
		if callCount == 1 {
			output = "## Progress\nCompleted\n\n## Status\nDEV_DONE DEV_DONE DEV_DONE!!!"
		} else {
			output = "## Progress\nReviewed\n\n### Critical Issues\nNone\n\n### Major Issues\nNone\n\n### Minor Issues\nNone\n\n### Verdict\nREVIEWER_APPROVED REVIEWER_APPROVED!!!"
		}
		jsonOutput := createMockClaudeOutput(output)
		return exec.CommandContext(ctx, "echo", jsonOutput)
	})

	// Create mock distiller
	distillerClient := claude.NewClient(claude.ClientConfig{Model: "haiku", MaxTurns: 1})
	distillerClient.SetCommandCreator(mockDistillerCreator("test commit"))
	testDistiller := distill.NewDistiller(distillerClient)

	// Create mock jj client that returns empty diff (IsEmpty = true)
	jjClient := jj.NewClient("/tmp")
	var jjNewCalled bool
	jjClient.SetCommandRunner(func(ctx context.Context, dir string, name string, args ...string) (string, string, error) {
		if len(args) >= 1 && args[0] == "new" {
			jjNewCalled = true
		}
		// Return empty diff so IsEmpty returns true
		return "", "", nil
	})

	// Create loop
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

	// Run
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	// Collect events
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

	wg.Wait()

	// jj new should NOT have been called because diff was empty
	if jjNewCalled {
		t.Error("jj new was called when change was empty - should have been skipped")
	}

	// Verify we got the "skipping" event message
	var foundSkipMessage bool
	for _, e := range events {
		if e.Type == EventJJNew && strings.Contains(e.Message, "Skipping") {
			foundSkipMessage = true
			break
		}
	}
	if !foundSkipMessage {
		t.Error("expected EventJJNew with 'Skipping' message when change is empty")
	}
}

func TestLoopCallsJJNewWhenNotEmpty(t *testing.T) {
	database := setupTestDB(t)
	plan := createTestPlan(t, database, "Test plan content")

	// Create mock Claude client (developer output - no DEV_DONE since we just want one iteration)
	claudeClient := claude.NewClient(claude.ClientConfig{
		Model:    "test",
		MaxTurns: 1,
	})
	claudeClient.SetCommandCreator(mockClaudeCreator("## Progress\nWorking\n\n## Status\nRUNNING RUNNING RUNNING"))

	// Create mock distiller
	distillerClient := claude.NewClient(claude.ClientConfig{Model: "haiku", MaxTurns: 1})
	distillerClient.SetCommandCreator(mockDistillerCreator("test commit"))
	testDistiller := distill.NewDistiller(distillerClient)

	// Create mock jj client that returns non-empty diff
	jjClient := jj.NewClient("/tmp")
	var jjNewCalled bool
	jjClient.SetCommandRunner(func(ctx context.Context, dir string, name string, args ...string) (string, string, error) {
		if len(args) >= 1 && args[0] == "new" {
			jjNewCalled = true
		}
		if len(args) >= 1 && args[0] == "diff" {
			// Return non-empty diff
			return "diff --git a/file.go b/file.go\n+// new code", "", nil
		}
		if len(args) >= 1 && args[0] == "show" {
			return "diff --git a/file.go b/file.go", "", nil
		}
		return "", "", nil
	})

	// Create loop
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

	// Run
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	// Collect events
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

	wg.Wait()

	// jj new SHOULD have been called because diff was not empty
	if !jjNewCalled {
		t.Error("jj new was NOT called when change was not empty - should have been called")
	}

	// Verify we got the "Creating" event message (not "Skipping")
	var foundCreateMessage bool
	for _, e := range events {
		if e.Type == EventJJNew && strings.Contains(e.Message, "Creating") {
			foundCreateMessage = true
			break
		}
	}
	if !foundCreateMessage {
		t.Error("expected EventJJNew with 'Creating' message when change is not empty")
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

// createMockClaudeOutputWithToolUse creates mock Claude stream-json output that includes tool use events.
func createMockClaudeOutputWithToolUse(text string, toolNames ...string) string {
	var lines []string

	// Create init event
	initEvent := map[string]interface{}{
		"type":       "init",
		"session_id": "test-session-123",
		"model":      "test-model",
		"cwd":        "/test",
	}
	initJSON, _ := json.Marshal(initEvent)
	lines = append(lines, string(initJSON))

	// Create tool_use events for each tool
	for i, toolName := range toolNames {
		toolUseEvent := map[string]interface{}{
			"message": map[string]interface{}{
				"id":          "msg-tool-" + string(rune('0'+i)),
				"role":        "assistant",
				"model":       "test-model",
				"stop_reason": "tool_use",
				"content": []map[string]interface{}{
					{
						"type":  "tool_use",
						"id":    "tool-" + string(rune('0'+i)),
						"name":  toolName,
						"input": map[string]interface{}{},
					},
				},
			},
		}
		toolJSON, _ := json.Marshal(toolUseEvent)
		lines = append(lines, string(toolJSON))
	}

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
	lines = append(lines, string(messageJSON))

	// Create result event
	resultEvent := map[string]interface{}{
		"type":       "result",
		"session_id": "test-session-123",
		"result":     text,
		"cost_usd":   0.001,
		"num_turns":  1,
	}
	resultJSON, _ := json.Marshal(resultEvent)
	lines = append(lines, string(resultJSON))

	return strings.Join(lines, "\n")
}

// mockClaudeCreatorWithToolUse creates a command creator that simulates Claude output with tool use.
func mockClaudeCreatorWithToolUse(output string, toolNames ...string) claude.CommandCreator {
	return func(ctx context.Context, name string, args ...string) *exec.Cmd {
		jsonOutput := createMockClaudeOutputWithToolUse(output, toolNames...)
		return exec.CommandContext(ctx, "echo", jsonOutput)
	}
}

func TestLoopDoneMarkerIgnoredWithEdits(t *testing.T) {
	database := setupTestDB(t)
	plan := createTestPlan(t, database, "Test plan content")

	// Create mock Claude client that outputs DEV_DONE but also uses an Edit tool
	claudeClient := claude.NewClient(claude.ClientConfig{
		Model:    "test",
		MaxTurns: 1,
	})
	// Simulate using Edit tool then saying DEV_DONE - should be ignored
	claudeClient.SetCommandCreator(mockClaudeCreatorWithToolUse(
		"## Progress\nMade edits\n\n## Status\nDEV_DONE DEV_DONE DEV_DONE!!!",
		"Edit",
	))

	// Create mock distiller
	distillerClient := claude.NewClient(claude.ClientConfig{
		Model:    "haiku",
		MaxTurns: 1,
	})
	distillerClient.SetCommandCreator(mockDistillerCreator("test commit"))
	testDistiller := distill.NewDistiller(distillerClient)

	// Create mock jj client
	jjClient := jj.NewClient("/tmp")
	jjClient.SetCommandRunner(mockJJRunner())

	// Create loop with max 2 iterations - if DONE is accepted, we'd only run 1
	// If DONE is ignored (correctly), we should hit max iterations
	loop := New(Config{
		PlanID:        plan.ID,
		MaxIterations: 2,
		WorkDir:       "/tmp",
	}, Deps{
		DB:        database,
		Claude:    claudeClient,
		Distiller: testDistiller,
		JJ:        jjClient,
	})

	// Run with timeout
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
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

	// Wait for event collection to complete
	wg.Wait()

	// Should NOT have EventDone since DONE was rejected
	var foundDone bool
	for _, e := range events {
		if e.Type == EventDone {
			foundDone = true
			break
		}
	}
	if foundDone {
		t.Error("expected EventDone NOT to be emitted when session had edits")
	}

	// Should have EventMaxIterations since loop continued after rejecting DONE
	var foundMaxIter bool
	for _, e := range events {
		if e.Type == EventMaxIterations {
			foundMaxIter = true
			break
		}
	}
	if !foundMaxIter {
		t.Error("expected EventMaxIterations event when DONE is rejected due to edits")
	}

	// Plan should NOT be marked complete
	updatedPlan, err := database.GetPlan(plan.ID)
	if err != nil {
		t.Fatalf("failed to get plan: %v", err)
	}
	if updatedPlan.Status == db.PlanStatusCompleted {
		t.Error("expected plan NOT to be marked complete when DONE was rejected")
	}
}

func TestLoopDoneMarkerAcceptedWithoutEdits(t *testing.T) {
	database := setupTestDB(t)
	plan := createTestPlan(t, database, "Test plan content")

	// Track calls to differentiate developer vs reviewer
	callCount := 0

	// Create mock Claude client - developer uses Read (not edit) and signals DEV_DONE,
	// then reviewer approves
	claudeClient := claude.NewClient(claude.ClientConfig{
		Model:    "test",
		MaxTurns: 1,
	})
	claudeClient.SetCommandCreator(func(ctx context.Context, name string, args ...string) *exec.Cmd {
		callCount++
		var output string
		if callCount == 1 {
			// Developer: uses Read tool (not an edit) and signals DEV_DONE
			output = "## Progress\nReviewed code\n\n## Status\nDEV_DONE DEV_DONE DEV_DONE!!!"
		} else {
			// Reviewer: approves
			output = "## Progress\nReviewed\n\n### Critical Issues\nNone\n\n### Major Issues\nNone\n\n### Minor Issues\nNone\n\n### Verdict\nREVIEWER_APPROVED REVIEWER_APPROVED!!!"
		}
		jsonOutput := createMockClaudeOutputWithToolUse(output, "Read")
		return exec.CommandContext(ctx, "echo", jsonOutput)
	})

	// Create mock distiller
	distillerClient := claude.NewClient(claude.ClientConfig{
		Model:    "haiku",
		MaxTurns: 1,
	})
	distillerClient.SetCommandCreator(mockDistillerCreator("Complete implementation"))
	testDistiller := distill.NewDistiller(distillerClient)

	// Create mock jj client (empty so DEV_DONE is accepted)
	jjClient := jj.NewClient("/tmp")
	jjClient.SetCommandRunner(mockJJRunnerEmpty())

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

	// Collect events
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

	wg.Wait()

	// Should have EventDone since DEV_DONE was accepted and reviewer approved
	var foundDone bool
	for _, e := range events {
		if e.Type == EventDone {
			foundDone = true
			break
		}
	}
	if !foundDone {
		t.Error("expected EventDone to be emitted when session had no edits and reviewer approved")
	}

	// Plan should be marked complete
	updatedPlan, err := database.GetPlan(plan.ID)
	if err != nil {
		t.Fatalf("failed to get plan: %v", err)
	}
	if updatedPlan.Status != db.PlanStatusCompleted {
		t.Errorf("expected plan status 'completed', got: %s", updatedPlan.Status)
	}

	// Iteration should be 1 (only ran once)
	if loop.CurrentIteration() != 1 {
		t.Errorf("expected iteration 1, got: %d", loop.CurrentIteration())
	}
}

func TestDevDoneMarkerSanitizedFromProgress(t *testing.T) {
	database := setupTestDB(t)
	plan := createTestPlan(t, database, "Test plan content")

	// Create mock Claude client that has DEV_DONE marker in progress section and uses Edit
	claudeClient := claude.NewClient(claude.ClientConfig{
		Model:    "test",
		MaxTurns: 1,
	})
	// Progress contains DEV_DONE marker, and we use Edit tool (so DEV_DONE is ignored)
	claudeClient.SetCommandCreator(mockClaudeCreatorWithToolUse(
		"## Progress\nDEV_DONE DEV_DONE DEV_DONE!!! - completed work\n\n## Learnings\nLearned about DEV_DONE DEV_DONE DEV_DONE!!! marker\n\n## Status\nDEV_DONE DEV_DONE DEV_DONE!!!",
		"Write", // Using Write tool (an edit tool)
	))

	// Create mock distiller
	distillerClient := claude.NewClient(claude.ClientConfig{
		Model:    "haiku",
		MaxTurns: 1,
	})
	distillerClient.SetCommandCreator(mockDistillerCreator("test commit"))
	testDistiller := distill.NewDistiller(distillerClient)

	// Create mock jj client
	jjClient := jj.NewClient("/tmp")
	jjClient.SetCommandRunner(mockJJRunner())

	// Create loop with max 1 iteration
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

	// Drain events
	go func() {
		for range loop.Events() {
		}
	}()

	// Run the loop
	err := loop.Run(ctx)
	if err != nil {
		t.Fatalf("loop.Run() error: %v", err)
	}

	// Verify progress was stored WITHOUT the DEV_DONE marker
	progress, err := database.GetLatestProgress(plan.ID)
	if err != nil {
		t.Fatalf("failed to get progress: %v", err)
	}
	if progress == nil {
		t.Fatal("expected progress to be stored")
	}
	if strings.Contains(progress.Content, "DEV_DONE DEV_DONE DEV_DONE!!!") {
		t.Errorf("expected DEV_DONE marker to be sanitized from progress, got: %s", progress.Content)
	}
	if !strings.Contains(progress.Content, "completed work") {
		t.Errorf("expected progress to contain 'completed work', got: %s", progress.Content)
	}

	// Verify learnings was stored WITHOUT the DEV_DONE marker
	learnings, err := database.GetLatestLearnings(plan.ID)
	if err != nil {
		t.Fatalf("failed to get learnings: %v", err)
	}
	if learnings == nil {
		t.Fatal("expected learnings to be stored")
	}
	if strings.Contains(learnings.Content, "DEV_DONE DEV_DONE DEV_DONE!!!") {
		t.Errorf("expected DEV_DONE marker to be sanitized from learnings, got: %s", learnings.Content)
	}
	if !strings.Contains(learnings.Content, "Learned about") {
		t.Errorf("expected learnings to contain 'Learned about', got: %s", learnings.Content)
	}
}

func TestIsEditTool(t *testing.T) {
	editTools := []string{"Edit", "Write", "NotebookEdit"}
	nonEditTools := []string{"Read", "Bash", "Glob", "Grep", "Task", "WebFetch", ""}

	for _, tool := range editTools {
		if !isEditTool(tool) {
			t.Errorf("expected %q to be classified as an edit tool", tool)
		}
	}

	for _, tool := range nonEditTools {
		if isEditTool(tool) {
			t.Errorf("expected %q to NOT be classified as an edit tool", tool)
		}
	}
}

func TestSanitizeDoneMarker(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{"DONE DONE DONE!!!", ""},
		{"Some text DONE DONE DONE!!! more text", "Some text  more text"},
		{"No marker here", "No marker here"},
		{"DONE DONE DONE!!! at start", " at start"},
		{"at end DONE DONE DONE!!!", "at end "},
		{"Multiple DONE DONE DONE!!! markers DONE DONE DONE!!!", "Multiple  markers "},
	}

	for _, tc := range tests {
		result := sanitizeDoneMarker(tc.input)
		if result != tc.expected {
			t.Errorf("sanitizeDoneMarker(%q) = %q, expected %q", tc.input, result, tc.expected)
		}
	}
}

func TestSanitizeDevDoneMarker(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{"DEV_DONE DEV_DONE DEV_DONE!!!", ""},
		{"Some text DEV_DONE DEV_DONE DEV_DONE!!! more text", "Some text  more text"},
		{"No marker here", "No marker here"},
		{"DEV_DONE DEV_DONE DEV_DONE!!! at start", " at start"},
		{"at end DEV_DONE DEV_DONE DEV_DONE!!!", "at end "},
	}

	for _, tc := range tests {
		result := sanitizeDevDoneMarker(tc.input)
		if result != tc.expected {
			t.Errorf("sanitizeDevDoneMarker(%q) = %q, expected %q", tc.input, result, tc.expected)
		}
	}
}

// =============================================================================
// Dual-Agent Loop Tests
// =============================================================================

func TestLoop_DeveloperOnlyIteration(t *testing.T) {
	database := setupTestDB(t)
	plan := createTestPlan(t, database, "Test plan content")

	// Create mock Claude client that outputs developer progress (no DEV_DONE)
	claudeClient := claude.NewClient(claude.ClientConfig{
		Model:    "test",
		MaxTurns: 1,
	})
	claudeClient.SetCommandCreator(mockClaudeCreator("## Progress\nWorking on feature\n\n## Learnings\nFound pattern\n\n## Status\nRUNNING RUNNING RUNNING"))

	// Create mock distiller
	distillerClient := claude.NewClient(claude.ClientConfig{Model: "haiku", MaxTurns: 1})
	distillerClient.SetCommandCreator(mockDistillerCreator("wip: working on feature"))
	testDistiller := distill.NewDistiller(distillerClient)

	// Create mock jj client
	jjClient := jj.NewClient("/tmp")
	jjClient.SetCommandRunner(mockJJRunner())

	// Create loop with max 1 iteration
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

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

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
	wg.Wait()

	// Should have developer events but NOT reviewer events (no DEV_DONE)
	var foundDevStart, foundDevEnd, foundReviewerStart bool
	for _, e := range events {
		if e.Type == EventDeveloperStart {
			foundDevStart = true
		}
		if e.Type == EventDeveloperEnd {
			foundDevEnd = true
		}
		if e.Type == EventReviewerStart {
			foundReviewerStart = true
		}
	}

	if !foundDevStart {
		t.Error("expected EventDeveloperStart")
	}
	if !foundDevEnd {
		t.Error("expected EventDeveloperEnd")
	}
	if foundReviewerStart {
		t.Error("reviewer should NOT start when developer didn't signal DEV_DONE")
	}

	// Verify progress was stored
	progress, err := database.GetLatestProgress(plan.ID)
	if err != nil {
		t.Fatalf("failed to get progress: %v", err)
	}
	if progress == nil || !strings.Contains(progress.Content, "Working on feature") {
		t.Errorf("expected progress to contain 'Working on feature'")
	}
}

func TestLoop_DeveloperDoneTriggersReviewer(t *testing.T) {
	database := setupTestDB(t)
	plan := createTestPlan(t, database, "Test plan content")

	// Track which prompts are being built to differentiate developer vs reviewer calls
	callCount := 0

	// Create mock Claude client with call tracking
	claudeClient := claude.NewClient(claude.ClientConfig{
		Model:    "test",
		MaxTurns: 1,
	})
	claudeClient.SetCommandCreator(func(ctx context.Context, name string, args ...string) *exec.Cmd {
		callCount++
		var output string
		if callCount == 1 {
			// First call is developer - signal DEV_DONE
			output = "## Progress\nCompleted work\n\n## Learnings\nLearned stuff\n\n## Status\nDEV_DONE DEV_DONE DEV_DONE!!!"
		} else {
			// Second call is reviewer - approve
			output = "## Progress\nReviewed code\n\n## Learnings\nCode looks good\n\n### Critical Issues\nNone\n\n### Major Issues\nNone\n\n### Minor Issues\nNone\n\n### Verdict\nREVIEWER_APPROVED REVIEWER_APPROVED!!!"
		}
		jsonOutput := createMockClaudeOutput(output)
		return exec.CommandContext(ctx, "echo", jsonOutput)
	})

	// Create mock distiller
	distillerClient := claude.NewClient(claude.ClientConfig{Model: "haiku", MaxTurns: 1})
	distillerClient.SetCommandCreator(mockDistillerCreator("Complete feature"))
	testDistiller := distill.NewDistiller(distillerClient)

	// Create mock jj client - return empty diff so developer can signal done
	jjClient := jj.NewClient("/tmp")
	jjClient.SetCommandRunner(mockJJRunnerEmpty())

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

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

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
	wg.Wait()

	// Should have both developer and reviewer events
	var foundDevDone, foundReviewerStart, foundReviewerApproved, foundBothDone bool
	for _, e := range events {
		if e.Type == EventDeveloperDone {
			foundDevDone = true
		}
		if e.Type == EventReviewerStart {
			foundReviewerStart = true
		}
		if e.Type == EventReviewerApproved {
			foundReviewerApproved = true
		}
		if e.Type == EventBothDone {
			foundBothDone = true
		}
	}

	if !foundDevDone {
		t.Error("expected EventDeveloperDone")
	}
	if !foundReviewerStart {
		t.Error("expected EventReviewerStart when developer signaled DEV_DONE")
	}
	if !foundReviewerApproved {
		t.Error("expected EventReviewerApproved")
	}
	if !foundBothDone {
		t.Error("expected EventBothDone")
	}

	// Plan should be completed
	updatedPlan, err := database.GetPlan(plan.ID)
	if err != nil {
		t.Fatalf("failed to get plan: %v", err)
	}
	if updatedPlan.Status != db.PlanStatusCompleted {
		t.Errorf("expected plan status 'completed', got: %s", updatedPlan.Status)
	}
}

func TestLoop_DevDoneIgnoredWithEdits(t *testing.T) {
	database := setupTestDB(t)
	plan := createTestPlan(t, database, "Test plan content")

	// Create mock Claude client that outputs DEV_DONE but also uses Edit tool
	claudeClient := claude.NewClient(claude.ClientConfig{
		Model:    "test",
		MaxTurns: 1,
	})
	claudeClient.SetCommandCreator(mockClaudeCreatorWithToolUse(
		"## Progress\nMade edits\n\n## Status\nDEV_DONE DEV_DONE DEV_DONE!!!",
		"Edit",
	))

	// Create mock distiller
	distillerClient := claude.NewClient(claude.ClientConfig{Model: "haiku", MaxTurns: 1})
	distillerClient.SetCommandCreator(mockDistillerCreator("wip"))
	testDistiller := distill.NewDistiller(distillerClient)

	// Create mock jj client
	jjClient := jj.NewClient("/tmp")
	jjClient.SetCommandRunner(mockJJRunner())

	// Create loop with max 2 iterations
	loop := New(Config{
		PlanID:        plan.ID,
		MaxIterations: 2,
		WorkDir:       "/tmp",
	}, Deps{
		DB:        database,
		Claude:    claudeClient,
		Distiller: testDistiller,
		JJ:        jjClient,
	})

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

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
	wg.Wait()

	// Should NOT trigger reviewer since DEV_DONE was ignored due to edits
	var foundReviewerStart bool
	for _, e := range events {
		if e.Type == EventReviewerStart {
			foundReviewerStart = true
		}
	}

	if foundReviewerStart {
		t.Error("reviewer should NOT start when DEV_DONE is ignored due to edits")
	}

	// Should hit max iterations
	var foundMaxIter bool
	for _, e := range events {
		if e.Type == EventMaxIterations {
			foundMaxIter = true
		}
	}
	if !foundMaxIter {
		t.Error("expected EventMaxIterations when DEV_DONE is ignored")
	}
}

func TestLoop_ReviewerRejects(t *testing.T) {
	database := setupTestDB(t)
	plan := createTestPlan(t, database, "Test plan content")

	callCount := 0

	// Create mock Claude client
	claudeClient := claude.NewClient(claude.ClientConfig{
		Model:    "test",
		MaxTurns: 1,
	})
	claudeClient.SetCommandCreator(func(ctx context.Context, name string, args ...string) *exec.Cmd {
		callCount++
		var output string
		if callCount == 1 {
			// Developer signals done
			output = "## Progress\nCompleted work\n\n## Status\nDEV_DONE DEV_DONE DEV_DONE!!!"
		} else if callCount == 2 {
			// Reviewer rejects
			output = "## Progress\nReviewed code\n\n### Critical Issues\nNone\n\n### Major Issues\n- Missing error handling in auth.go:42\n\n### Minor Issues\nNone\n\n### Verdict\nREVIEWER_FEEDBACK: Fix the error handling issue"
		} else {
			// Subsequent developer calls (max iterations will stop it)
			output = "## Progress\nFixed issues\n\n## Status\nRUNNING RUNNING RUNNING"
		}
		jsonOutput := createMockClaudeOutput(output)
		return exec.CommandContext(ctx, "echo", jsonOutput)
	})

	// Create mock distiller
	distillerClient := claude.NewClient(claude.ClientConfig{Model: "haiku", MaxTurns: 1})
	distillerClient.SetCommandCreator(mockDistillerCreator("wip"))
	testDistiller := distill.NewDistiller(distillerClient)

	// Create mock jj client
	jjClient := jj.NewClient("/tmp")
	jjClient.SetCommandRunner(mockJJRunnerEmpty())

	// Create loop with max 1 iteration so feedback persists
	// (iteration 2 would clear feedback after developer sees it)
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

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

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
	wg.Wait()

	// Should have reviewer feedback event
	var foundReviewerFeedback bool
	for _, e := range events {
		if e.Type == EventReviewerFeedback {
			foundReviewerFeedback = true
		}
	}
	if !foundReviewerFeedback {
		t.Error("expected EventReviewerFeedback")
	}

	// Should NOT have both done (reviewer rejected)
	var foundBothDone bool
	for _, e := range events {
		if e.Type == EventBothDone {
			foundBothDone = true
		}
	}
	if foundBothDone {
		t.Error("should NOT have EventBothDone when reviewer rejects")
	}

	// Feedback should be stored
	feedback, err := database.GetLatestReviewerFeedback(plan.ID)
	if err != nil {
		t.Fatalf("failed to get feedback: %v", err)
	}
	if feedback == nil {
		t.Error("expected reviewer feedback to be stored")
	} else if !strings.Contains(feedback.Content, "error handling") {
		t.Errorf("expected feedback to contain 'error handling', got: %s", feedback.Content)
	}
}

func TestLoop_FeedbackIncludedInNextIteration(t *testing.T) {
	database := setupTestDB(t)
	plan := createTestPlan(t, database, "Test plan content")

	// Pre-store some reviewer feedback
	session := &db.PlanSession{
		ID:          uuid.New().String(),
		PlanID:      plan.ID,
		Iteration:   0,
		InputPrompt: "previous",
		Status:      db.PlanSessionCompleted,
		AgentType:   db.LoopAgentReviewer,
	}
	if err := database.CreatePlanSession(session); err != nil {
		t.Fatalf("failed to create session: %v", err)
	}

	feedback := &db.ReviewerFeedback{
		PlanID:    plan.ID,
		SessionID: session.ID,
		Content:   "Fix the security vulnerability in auth.go",
	}
	if err := database.CreateReviewerFeedback(feedback); err != nil {
		t.Fatalf("failed to create feedback: %v", err)
	}

	// Track the prompt passed to Claude
	var capturedPrompt string

	// Create mock Claude client
	claudeClient := claude.NewClient(claude.ClientConfig{
		Model:    "test",
		MaxTurns: 1,
	})
	claudeClient.SetCommandCreator(func(ctx context.Context, name string, args ...string) *exec.Cmd {
		// Capture the prompt from args (claude-code passes prompt via args)
		for i, arg := range args {
			if arg == "-p" && i+1 < len(args) {
				capturedPrompt = args[i+1]
			}
		}
		jsonOutput := createMockClaudeOutput("## Progress\nFixed security issue\n\n## Status\nRUNNING RUNNING RUNNING")
		return exec.CommandContext(ctx, "echo", jsonOutput)
	})

	// Create mock distiller
	distillerClient := claude.NewClient(claude.ClientConfig{Model: "haiku", MaxTurns: 1})
	distillerClient.SetCommandCreator(mockDistillerCreator("fix security"))
	testDistiller := distill.NewDistiller(distillerClient)

	// Create mock jj client
	jjClient := jj.NewClient("/tmp")
	jjClient.SetCommandRunner(mockJJRunner())

	// Create loop with max 1 iteration
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

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	// Capture prompt from events
	var promptFromEvent string
	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		defer wg.Done()
		for event := range loop.Events() {
			if event.Type == EventPromptBuilt && event.Prompt != "" {
				promptFromEvent = event.Prompt
			}
		}
	}()

	err := loop.Run(ctx)
	if err != nil {
		t.Fatalf("loop.Run() error: %v", err)
	}
	wg.Wait()

	// The prompt should include the reviewer feedback
	promptToCheck := promptFromEvent
	if promptToCheck == "" {
		promptToCheck = capturedPrompt
	}

	if promptToCheck != "" && !strings.Contains(promptToCheck, "security vulnerability") {
		t.Errorf("expected developer prompt to include reviewer feedback about 'security vulnerability'")
	}

	// Feedback should be cleared after developer sees it
	remainingFeedback, err := database.GetLatestReviewerFeedback(plan.ID)
	if err != nil {
		t.Fatalf("failed to get feedback: %v", err)
	}
	if remainingFeedback != nil {
		t.Error("expected reviewer feedback to be cleared after developer sees it")
	}
}

// mockJJRunnerWithDiff creates a jj command runner that returns a proper base change ID
// and cumulative diff. This properly simulates the reviewer diff flow.
func mockJJRunnerWithDiff(baseChangeID, diffContent string) jj.CommandRunner {
	return func(ctx context.Context, dir string, name string, args ...string) (string, string, error) {
		if len(args) >= 1 && args[0] == "log" {
			// Check if this is GetParentChangeID call (log -r @- -T change_id --no-graph)
			for i, arg := range args {
				if arg == "-r" && i+1 < len(args) && args[i+1] == "@-" {
					return baseChangeID + "\n", "", nil
				}
			}
			return "", "", nil
		}
		if len(args) >= 1 && args[0] == "diff" {
			// Check if this is a cumulative diff (diff --from X --to @)
			for i, arg := range args {
				if arg == "--from" && i+1 < len(args) && args[i+1] == baseChangeID {
					return diffContent, "", nil
				}
			}
			// Plain jj diff (no args) - used by distillAndCommit to get current change diff
			// Return the diff content to trigger distillation
			if len(args) == 1 {
				return diffContent, "", nil
			}
			return "", "", nil
		}
		if len(args) >= 1 && args[0] == "show" {
			return diffContent, "", nil
		}
		return "", "", nil
	}
}

func TestLoop_ReviewerReceivesCumulativeDiff(t *testing.T) {
	database := setupTestDB(t)
	plan := createTestPlan(t, database, "Test plan content")

	// Define the expected diff content
	expectedDiff := "diff --git a/main.go b/main.go\n+func newFeature() {\n+    // implementation\n+}"
	baseChangeID := "testbase123"

	callCount := 0
	var reviewerPrompt string

	// Create mock Claude client that captures the reviewer prompt
	claudeClient := claude.NewClient(claude.ClientConfig{
		Model:    "test",
		MaxTurns: 1,
	})
	claudeClient.SetCommandCreator(func(ctx context.Context, name string, args ...string) *exec.Cmd {
		callCount++
		var output string
		if callCount == 1 {
			// Developer - signal DEV_DONE
			output = "## Progress\nDone\n\n## Status\nDEV_DONE DEV_DONE DEV_DONE!!!"
		} else {
			// Reviewer - capture prompt from args and approve
			for i, arg := range args {
				if arg == "-p" && i+1 < len(args) {
					reviewerPrompt = args[i+1]
				}
			}
			output = "## Progress\nReviewed\n\n### Critical Issues\nNone\n\n### Major Issues\nNone\n\n### Minor Issues\nNone\n\n### Verdict\nREVIEWER_APPROVED REVIEWER_APPROVED!!!"
		}
		jsonOutput := createMockClaudeOutput(output)
		return exec.CommandContext(ctx, "echo", jsonOutput)
	})

	// Create mock distiller
	distillerClient := claude.NewClient(claude.ClientConfig{Model: "haiku", MaxTurns: 1})
	distillerClient.SetCommandCreator(mockDistillerCreator("done"))
	testDistiller := distill.NewDistiller(distillerClient)

	// Create mock jj client that returns a proper diff
	jjClient := jj.NewClient("/tmp")
	jjClient.SetCommandRunner(mockJJRunnerWithDiff(baseChangeID, expectedDiff))

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

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	// Also capture prompt from events
	var promptFromEvent string
	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		defer wg.Done()
		for event := range loop.Events() {
			// Capture the second PromptBuilt event (reviewer prompt)
			if event.Type == EventPromptBuilt && event.Prompt != "" {
				if strings.Contains(event.Prompt, "VERY HARD CRITIC") {
					promptFromEvent = event.Prompt
				}
			}
		}
	}()

	err := loop.Run(ctx)
	if err != nil {
		t.Fatalf("loop.Run() error: %v", err)
	}
	wg.Wait()

	// Verify the baseChangeID was persisted
	updatedPlan, err := database.GetPlan(plan.ID)
	if err != nil {
		t.Fatalf("failed to get plan: %v", err)
	}
	if updatedPlan.BaseChangeID != baseChangeID {
		t.Errorf("expected BaseChangeID=%q to be persisted, got %q", baseChangeID, updatedPlan.BaseChangeID)
	}

	// Use the prompt from event (more reliable than capturing from Claude args)
	promptToCheck := promptFromEvent
	if promptToCheck == "" {
		promptToCheck = reviewerPrompt
	}

	// Verify the diff was included in the reviewer prompt
	if promptToCheck == "" {
		t.Fatal("failed to capture reviewer prompt")
	}

	if !strings.Contains(promptToCheck, "# Diff to Review") {
		t.Error("reviewer prompt missing '# Diff to Review' header")
	}

	if strings.Contains(promptToCheck, "No diff available") {
		t.Error("reviewer prompt should NOT show 'No diff available' when diff is provided")
	}

	if !strings.Contains(promptToCheck, "newFeature") {
		t.Errorf("reviewer prompt should contain the diff content 'newFeature', got prompt:\n%s", truncateString(promptToCheck, 500))
	}
}

func TestLoop_AgentTypeStoredInSession(t *testing.T) {
	database := setupTestDB(t)
	plan := createTestPlan(t, database, "Test plan content")

	callCount := 0

	// Create mock Claude client
	claudeClient := claude.NewClient(claude.ClientConfig{
		Model:    "test",
		MaxTurns: 1,
	})
	claudeClient.SetCommandCreator(func(ctx context.Context, name string, args ...string) *exec.Cmd {
		callCount++
		var output string
		if callCount == 1 {
			output = "## Progress\nDone\n\n## Status\nDEV_DONE DEV_DONE DEV_DONE!!!"
		} else {
			output = "### Verdict\nREVIEWER_APPROVED REVIEWER_APPROVED!!!"
		}
		jsonOutput := createMockClaudeOutput(output)
		return exec.CommandContext(ctx, "echo", jsonOutput)
	})

	// Create mock distiller
	distillerClient := claude.NewClient(claude.ClientConfig{Model: "haiku", MaxTurns: 1})
	distillerClient.SetCommandCreator(mockDistillerCreator("done"))
	testDistiller := distill.NewDistiller(distillerClient)

	// Create mock jj client
	jjClient := jj.NewClient("/tmp")
	jjClient.SetCommandRunner(mockJJRunnerEmpty())

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

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	go func() {
		for range loop.Events() {
		}
	}()

	err := loop.Run(ctx)
	if err != nil {
		t.Fatalf("loop.Run() error: %v", err)
	}

	// Verify sessions were created with correct agent types
	sessions, err := database.GetPlanSessionsByPlan(plan.ID)
	if err != nil {
		t.Fatalf("failed to get sessions: %v", err)
	}

	if len(sessions) < 2 {
		t.Fatalf("expected at least 2 sessions (developer + reviewer), got %d", len(sessions))
	}

	var foundDeveloper, foundReviewer bool
	for _, s := range sessions {
		if s.AgentType == db.LoopAgentDeveloper {
			foundDeveloper = true
		}
		if s.AgentType == db.LoopAgentReviewer {
			foundReviewer = true
		}
	}

	if !foundDeveloper {
		t.Error("expected developer session to be stored")
	}
	if !foundReviewer {
		t.Error("expected reviewer session to be stored")
	}
}

func TestTruncateDiff(t *testing.T) {
	tests := []struct {
		name      string
		input     string
		wantTrunc bool
	}{
		{
			name:      "small diff unchanged",
			input:     "small diff content",
			wantTrunc: false,
		},
		{
			name:      "exact limit unchanged",
			input:     strings.Repeat("x", maxDiffBytes),
			wantTrunc: false,
		},
		{
			name:      "large diff truncated",
			input:     strings.Repeat("x", 200*1024), // 200KB
			wantTrunc: true,
		},
		{
			name:      "truncates at line boundary",
			input:     strings.Repeat("line\n", maxDiffBytes/5+1000), // Many lines exceeding limit
			wantTrunc: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := truncateDiff(tt.input)
			if tt.wantTrunc {
				if len(result) >= len(tt.input) {
					t.Errorf("expected truncation, got len %d >= %d", len(result), len(tt.input))
				}
				if !strings.Contains(result, "DIFF TRUNCATED") {
					t.Error("expected truncation message")
				}
			} else {
				if result != tt.input {
					t.Error("expected unchanged diff")
				}
			}
		})
	}
}
