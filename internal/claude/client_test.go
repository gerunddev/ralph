package claude

import (
	"context"
	"os"
	"os/exec"
	"strings"
	"testing"
	"time"
)

// =============================================================================
// Client Tests - NewClient
// =============================================================================

func TestNewClient(t *testing.T) {
	cfg := ClientConfig{
		Model:    "opus",
		MaxTurns: 50,
		Verbose:  true,
	}

	client := NewClient(cfg)

	if client == nil {
		t.Fatal("NewClient() returned nil")
	}
	if client.model != "opus" {
		t.Errorf("client.model = %q, want %q", client.model, "opus")
	}
	if client.maxTurns != 50 {
		t.Errorf("client.maxTurns = %d, want %d", client.maxTurns, 50)
	}
	if !client.verbose {
		t.Error("client.verbose = false, want true")
	}
	if client.commandCreator == nil {
		t.Error("client.commandCreator is nil")
	}
}

func TestNewClient_Defaults(t *testing.T) {
	cfg := ClientConfig{}
	client := NewClient(cfg)

	if client == nil {
		t.Fatal("NewClient() returned nil")
	}
	if client.model != "" {
		t.Errorf("client.model = %q, want empty", client.model)
	}
	if client.maxTurns != 0 {
		t.Errorf("client.maxTurns = %d, want 0", client.maxTurns)
	}
	if client.verbose {
		t.Error("client.verbose = true, want false")
	}
}

// =============================================================================
// Client Tests - Command Arguments
// =============================================================================

// mockCommandCreator creates a mock command that records the arguments
// and writes predefined output to stdout.
func mockCommandCreator(output string) (CommandCreator, *[][]string) {
	var calls [][]string

	creator := func(ctx context.Context, name string, args ...string) *exec.Cmd {
		calls = append(calls, append([]string{name}, args...))
		return exec.CommandContext(ctx, "echo", "-n", output)
	}

	return creator, &calls
}

func TestClient_RunBuildsCorrectArguments(t *testing.T) {
	cfg := ClientConfig{
		Model:    "opus",
		MaxTurns: 25,
		Verbose:  true,
	}
	client := NewClient(cfg)

	// Create a mock that outputs valid JSONL
	output := `{"type":"init","session_id":"test"}
{"type":"result","session_id":"test"}`
	creator, calls := mockCommandCreator(output)
	client.SetCommandCreator(creator)

	ctx := context.Background()
	session, err := client.Run(ctx, "test prompt")
	if err != nil {
		t.Fatalf("Run() returned error: %v", err)
	}

	// Drain events to let the session complete
	for range session.Events() {
	}
	_ = session.Wait()

	// Verify the command was called with correct arguments
	if len(*calls) != 1 {
		t.Fatalf("Expected 1 call, got %d", len(*calls))
	}

	args := (*calls)[0]
	// First element is the command name
	if args[0] != "claude" {
		t.Errorf("Command name = %q, want %q", args[0], "claude")
	}

	// Check for required arguments
	argsStr := strings.Join(args[1:], " ")
	expectedParts := []string{
		"-p",
		"--output-format stream-json",
		"--verbose",
		"--model opus",
		"--max-turns 25",
		"test prompt",
	}

	for _, part := range expectedParts {
		if !strings.Contains(argsStr, part) {
			t.Errorf("Arguments missing %q, got: %v", part, args[1:])
		}
	}
}

func TestClient_RunOmitsOptionalArguments(t *testing.T) {
	// Create client with no optional config
	cfg := ClientConfig{}
	client := NewClient(cfg)

	output := `{"type":"init","session_id":"test"}
{"type":"result","session_id":"test"}`
	creator, calls := mockCommandCreator(output)
	client.SetCommandCreator(creator)

	ctx := context.Background()
	session, err := client.Run(ctx, "test prompt")
	if err != nil {
		t.Fatalf("Run() returned error: %v", err)
	}

	// Drain events
	for range session.Events() {
	}
	_ = session.Wait()

	if len(*calls) != 1 {
		t.Fatalf("Expected 1 call, got %d", len(*calls))
	}

	args := (*calls)[0]
	argsStr := strings.Join(args, " ")

	// These should NOT be present when not configured
	if strings.Contains(argsStr, "--model") {
		t.Error("--model should not be present when model is empty")
	}
	if strings.Contains(argsStr, "--max-turns") {
		t.Error("--max-turns should not be present when maxTurns is 0")
	}
	// Note: --verbose is always required when using --output-format stream-json with -p
	if !strings.Contains(argsStr, "--verbose") {
		t.Error("--verbose should always be present (required for stream-json with -p)")
	}
}

// =============================================================================
// Client Tests - Session Events
// =============================================================================

func TestSession_EventsChannel(t *testing.T) {
	cfg := ClientConfig{}
	client := NewClient(cfg)

	output := `{"type":"init","session_id":"test123"}
{"message":{"id":"msg1","role":"assistant","content":[{"type":"text","text":"Hello!"}]}}
{"type":"result","session_id":"test123","cost_usd":0.01}`

	creator, _ := mockCommandCreator(output)
	client.SetCommandCreator(creator)

	ctx := context.Background()
	session, err := client.Run(ctx, "test")
	if err != nil {
		t.Fatalf("Run() returned error: %v", err)
	}

	var events []StreamEvent
	for event := range session.Events() {
		events = append(events, event)
	}

	if err := session.Wait(); err != nil {
		t.Errorf("Wait() returned error: %v", err)
	}

	if len(events) != 3 {
		t.Fatalf("Expected 3 events, got %d", len(events))
	}

	// Verify event types in order
	expectedTypes := []EventType{EventInit, EventMessage, EventResult}
	for i, expected := range expectedTypes {
		if events[i].Type != expected {
			t.Errorf("events[%d].Type = %v, want %v", i, events[i].Type, expected)
		}
	}

	// Verify init event
	if events[0].Init == nil || events[0].Init.SessionID != "test123" {
		t.Error("Init event not parsed correctly")
	}

	// Verify message event
	if events[1].Message == nil || events[1].Message.Text != "Hello!" {
		t.Error("Message event not parsed correctly")
	}

	// Verify result event
	if events[2].Result == nil || events[2].Result.CostUSD != 0.01 {
		t.Error("Result event not parsed correctly")
	}
}

// =============================================================================
// Client Tests - Context Cancellation
// =============================================================================

func TestSession_Cancel(t *testing.T) {
	cfg := ClientConfig{}
	client := NewClient(cfg)

	// Use a command that runs for a while
	client.SetCommandCreator(func(ctx context.Context, name string, args ...string) *exec.Cmd {
		// Sleep command that will be interrupted
		return exec.CommandContext(ctx, "sleep", "10")
	})

	ctx := context.Background()
	session, err := client.Run(ctx, "test")
	if err != nil {
		t.Fatalf("Run() returned error: %v", err)
	}

	// Cancel immediately
	session.Cancel()

	// Should complete quickly
	select {
	case <-session.Done():
		// Good - session ended
	case <-time.After(2 * time.Second):
		t.Error("Session did not end within timeout after Cancel()")
	}
}

func TestSession_ContextCancellation(t *testing.T) {
	cfg := ClientConfig{}
	client := NewClient(cfg)

	// Use a command that runs for a while
	client.SetCommandCreator(func(ctx context.Context, name string, args ...string) *exec.Cmd {
		return exec.CommandContext(ctx, "sleep", "10")
	})

	ctx, cancel := context.WithCancel(context.Background())
	session, err := client.Run(ctx, "test")
	if err != nil {
		t.Fatalf("Run() returned error: %v", err)
	}

	// Cancel via context
	cancel()

	// Should complete quickly
	select {
	case <-session.Done():
		// Good
	case <-time.After(2 * time.Second):
		t.Error("Session did not end within timeout after context cancellation")
	}
}

// =============================================================================
// Client Tests - Error Handling
// =============================================================================

func TestClient_CommandNotFound(t *testing.T) {
	cfg := ClientConfig{}
	client := NewClient(cfg)

	// Use a command that doesn't exist
	client.SetCommandCreator(func(ctx context.Context, name string, args ...string) *exec.Cmd {
		return exec.CommandContext(ctx, "nonexistent_command_12345")
	})

	ctx := context.Background()
	_, err := client.Run(ctx, "test")

	// Note: The exact error depends on the system
	// It should fail to start
	if err == nil {
		t.Error("Run() should return error for nonexistent command")
	}
}

// =============================================================================
// Client Tests - Done Channel
// =============================================================================

func TestSession_DoneChannel(t *testing.T) {
	cfg := ClientConfig{}
	client := NewClient(cfg)

	output := `{"type":"init","session_id":"test"}`
	creator, _ := mockCommandCreator(output)
	client.SetCommandCreator(creator)

	ctx := context.Background()
	session, err := client.Run(ctx, "test")
	if err != nil {
		t.Fatalf("Run() returned error: %v", err)
	}

	// Drain events
	for range session.Events() {
	}

	// Done should be closed after events are drained and command completes
	select {
	case <-session.Done():
		// Good
	case <-time.After(5 * time.Second):
		t.Error("Done channel not closed within timeout")
	}
}

// =============================================================================
// Integration Tests
// =============================================================================

func hasClaude() bool {
	_, err := exec.LookPath("claude")
	return err == nil
}

func TestIntegration_BasicRun(t *testing.T) {
	if !hasClaude() {
		t.Skip("claude not installed, skipping integration test")
	}

	// Skip if no API key (check common env vars)
	if os.Getenv("ANTHROPIC_API_KEY") == "" && os.Getenv("CLAUDE_API_KEY") == "" {
		t.Skip("No API key found, skipping integration test")
	}

	cfg := ClientConfig{
		Model:    "sonnet",
		MaxTurns: 1,
		Verbose:  true,
	}
	client := NewClient(cfg)

	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	session, err := client.Run(ctx, "Say 'Hello' and nothing else")
	if err != nil {
		t.Fatalf("Run() returned error: %v", err)
	}

	var events []StreamEvent
	for event := range session.Events() {
		events = append(events, event)
		t.Logf("Event: type=%s", event.Type)
	}

	if err := session.Wait(); err != nil {
		t.Logf("Wait() returned error (may be expected): %v", err)
	}

	if len(events) == 0 {
		t.Error("Expected at least one event")
	}

	// Should have at least an init and result event
	hasInit := false
	hasResult := false
	for _, e := range events {
		if e.Type == EventInit {
			hasInit = true
		}
		if e.Type == EventResult {
			hasResult = true
		}
	}

	if !hasInit {
		t.Error("Missing init event")
	}
	if !hasResult {
		t.Error("Missing result event")
	}
}
