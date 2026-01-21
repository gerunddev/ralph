package jj

import (
	"context"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strings"
	"testing"
	"time"
)

// =============================================================================
// Mock Command Runner
// =============================================================================

// mockCall records a single command invocation.
type mockCall struct {
	dir  string
	name string
	args []string
}

// mockCommandRunner is a test helper that records calls and returns predefined responses.
type mockCommandRunner struct {
	calls     []mockCall
	responses []mockResponse
	callIndex int
}

type mockResponse struct {
	stdout string
	stderr string
	err    error
}

func newMockRunner() *mockCommandRunner {
	return &mockCommandRunner{
		calls:     make([]mockCall, 0),
		responses: make([]mockResponse, 0),
	}
}

func (m *mockCommandRunner) addResponse(stdout, stderr string, err error) {
	m.responses = append(m.responses, mockResponse{stdout: stdout, stderr: stderr, err: err})
}

func (m *mockCommandRunner) run(ctx context.Context, dir string, name string, args ...string) (string, string, error) {
	m.calls = append(m.calls, mockCall{dir: dir, name: name, args: args})

	if m.callIndex >= len(m.responses) {
		return "", "", errors.New("no mock response configured")
	}

	resp := m.responses[m.callIndex]
	m.callIndex++
	return resp.stdout, resp.stderr, resp.err
}

// =============================================================================
// Unit Tests - NewClient
// =============================================================================

func TestNewClient(t *testing.T) {
	client := NewClient("/some/path")

	if client == nil {
		t.Fatal("NewClient() returned nil")
	}
	if client.workDir != "/some/path" {
		t.Errorf("NewClient() workDir = %q, want %q", client.workDir, "/some/path")
	}
	if client.commandRunner == nil {
		t.Error("NewClient() commandRunner is nil")
	}
}

func TestNewClient_EmptyPath(t *testing.T) {
	client := NewClient("")

	if client == nil {
		t.Fatal("NewClient() returned nil")
	}
	if client.workDir != "" {
		t.Errorf("NewClient() workDir = %q, want empty string", client.workDir)
	}
}

// =============================================================================
// Unit Tests - New
// =============================================================================

func TestNew(t *testing.T) {
	mock := newMockRunner()
	mock.addResponse("", "", nil)

	client := NewClient("/test/dir")
	client.SetCommandRunner(mock.run)

	err := client.New(context.Background())
	if err != nil {
		t.Fatalf("New() returned error: %v", err)
	}

	// Verify the call
	if len(mock.calls) != 1 {
		t.Fatalf("Expected 1 call, got %d", len(mock.calls))
	}

	call := mock.calls[0]
	if call.name != "jj" {
		t.Errorf("Call name = %q, want %q", call.name, "jj")
	}
	expectedArgs := []string{"new"}
	if !slices.Equal(call.args, expectedArgs) {
		t.Errorf("Call args = %v, want %v", call.args, expectedArgs)
	}
	if call.dir != "/test/dir" {
		t.Errorf("Call dir = %q, want %q", call.dir, "/test/dir")
	}
}

func TestNew_Error(t *testing.T) {
	mock := newMockRunner()
	mock.addResponse("", "Error: not a jj repository", errors.New("exit status 1"))

	client := NewClient("/test/dir")
	client.SetCommandRunner(mock.run)

	err := client.New(context.Background())
	if err == nil {
		t.Fatal("New() should return error")
	}
}

func TestNew_NotRepo(t *testing.T) {
	mock := newMockRunner()
	mock.addResponse("", "Error: There is no jj repo in \"/test\"", errors.New("exit status 1"))

	client := NewClient("/test/dir")
	client.SetCommandRunner(mock.run)

	err := client.New(context.Background())
	if !errors.Is(err, ErrNotRepo) {
		t.Errorf("New() error = %v, want ErrNotRepo", err)
	}
}

// =============================================================================
// Unit Tests - NewChange
// =============================================================================

func TestNewChange(t *testing.T) {
	mock := newMockRunner()
	// First call: jj new --no-edit -m "description"
	mock.addResponse("", "", nil)
	// Second call: jj log -r @ -T change_id --no-graph
	mock.addResponse("abc123def456", "", nil)

	client := NewClient("/test/dir")
	client.SetCommandRunner(mock.run)

	changeID, err := client.NewChange(context.Background(), "Test description")
	if err != nil {
		t.Fatalf("NewChange() returned error: %v", err)
	}

	if changeID != "abc123def456" {
		t.Errorf("NewChange() changeID = %q, want %q", changeID, "abc123def456")
	}

	// Verify the first call (new)
	if len(mock.calls) != 2 {
		t.Fatalf("Expected 2 calls, got %d", len(mock.calls))
	}

	newCall := mock.calls[0]
	expectedNewArgs := []string{"new", "--no-edit", "-m", "Test description"}
	if !slices.Equal(newCall.args, expectedNewArgs) {
		t.Errorf("NewChange() new call args = %v, want %v", newCall.args, expectedNewArgs)
	}

	// Verify the second call (log)
	logCall := mock.calls[1]
	expectedLogArgs := []string{"log", "-r", "@", "-T", "change_id", "--no-graph"}
	if !slices.Equal(logCall.args, expectedLogArgs) {
		t.Errorf("NewChange() log call args = %v, want %v", logCall.args, expectedLogArgs)
	}
}

func TestNewChange_EmptyDescription(t *testing.T) {
	mock := newMockRunner()
	// First call: jj new --no-edit (no -m flag)
	mock.addResponse("", "", nil)
	// Second call: jj log -r @ -T change_id --no-graph
	mock.addResponse("abc123", "", nil)

	client := NewClient("/test/dir")
	client.SetCommandRunner(mock.run)

	changeID, err := client.NewChange(context.Background(), "")
	if err != nil {
		t.Fatalf("NewChange() returned error: %v", err)
	}

	if changeID != "abc123" {
		t.Errorf("NewChange() changeID = %q, want %q", changeID, "abc123")
	}

	// Verify the new command has no -m flag
	newCall := mock.calls[0]
	expectedNewArgs := []string{"new", "--no-edit"}
	if !slices.Equal(newCall.args, expectedNewArgs) {
		t.Errorf("NewChange() new call args = %v, want %v", newCall.args, expectedNewArgs)
	}
}

func TestNewChange_Error(t *testing.T) {
	mock := newMockRunner()
	mock.addResponse("", "Error: failed", errors.New("exit status 1"))

	client := NewClient("/test/dir")
	client.SetCommandRunner(mock.run)

	_, err := client.NewChange(context.Background(), "Test")
	if err == nil {
		t.Fatal("NewChange() should return error")
	}
}

// =============================================================================
// Unit Tests - Describe
// =============================================================================

func TestDescribe(t *testing.T) {
	mock := newMockRunner()
	mock.addResponse("", "", nil)

	client := NewClient("/test/dir")
	client.SetCommandRunner(mock.run)

	err := client.Describe(context.Background(), "New description")
	if err != nil {
		t.Fatalf("Describe() returned error: %v", err)
	}

	// Verify the call
	if len(mock.calls) != 1 {
		t.Fatalf("Expected 1 call, got %d", len(mock.calls))
	}

	call := mock.calls[0]
	expectedArgs := []string{"describe", "-m", "New description"}
	if !slices.Equal(call.args, expectedArgs) {
		t.Errorf("Describe() called with args %v, want %v", call.args, expectedArgs)
	}
}

func TestDescribe_EmptyMessage(t *testing.T) {
	mock := newMockRunner()

	client := NewClient("/test/dir")
	client.SetCommandRunner(mock.run)

	err := client.Describe(context.Background(), "")
	if err == nil {
		t.Fatal("Describe() should return error for empty message")
	}
	if !strings.Contains(err.Error(), "empty") {
		t.Errorf("Describe() error = %q, want error about empty description", err)
	}

	// Should not have made any calls
	if len(mock.calls) != 0 {
		t.Errorf("Expected 0 calls, got %d", len(mock.calls))
	}
}

func TestDescribe_Error(t *testing.T) {
	mock := newMockRunner()
	mock.addResponse("", "Error: describe failed", errors.New("exit status 1"))

	client := NewClient("/test/dir")
	client.SetCommandRunner(mock.run)

	err := client.Describe(context.Background(), "Test")
	if err == nil {
		t.Fatal("Describe() should return error")
	}
}

// =============================================================================
// Unit Tests - Show
// =============================================================================

func TestShow(t *testing.T) {
	mock := newMockRunner()
	expectedOutput := "diff --git a/file.txt b/file.txt\n+new line"
	mock.addResponse(expectedOutput, "", nil)

	client := NewClient("/test/dir")
	client.SetCommandRunner(mock.run)

	output, err := client.Show(context.Background())
	if err != nil {
		t.Fatalf("Show() returned error: %v", err)
	}

	if output != expectedOutput {
		t.Errorf("Show() output = %q, want %q", output, expectedOutput)
	}

	// Verify the call
	if len(mock.calls) != 1 {
		t.Fatalf("Expected 1 call, got %d", len(mock.calls))
	}

	call := mock.calls[0]
	expectedArgs := []string{"show"}
	if !slices.Equal(call.args, expectedArgs) {
		t.Errorf("Show() called with args %v, want %v", call.args, expectedArgs)
	}
}

func TestShow_Error(t *testing.T) {
	mock := newMockRunner()
	mock.addResponse("", "Error: show failed", errors.New("exit status 1"))

	client := NewClient("/test/dir")
	client.SetCommandRunner(mock.run)

	_, err := client.Show(context.Background())
	if err == nil {
		t.Fatal("Show() should return error")
	}
}

// =============================================================================
// Unit Tests - Commit
// =============================================================================

func TestCommit(t *testing.T) {
	mock := newMockRunner()
	mock.addResponse("", "", nil)

	client := NewClient("/test/dir")
	client.SetCommandRunner(mock.run)

	err := client.Commit(context.Background(), "Test commit message")
	if err != nil {
		t.Fatalf("Commit() returned error: %v", err)
	}

	// Verify the call
	if len(mock.calls) != 1 {
		t.Fatalf("Expected 1 call, got %d", len(mock.calls))
	}

	call := mock.calls[0]
	if call.name != "jj" {
		t.Errorf("Call name = %q, want %q", call.name, "jj")
	}
	expectedArgs := []string{"commit", "-m", "Test commit message"}
	if !slices.Equal(call.args, expectedArgs) {
		t.Errorf("Call args = %v, want %v", call.args, expectedArgs)
	}
}

func TestCommit_Error(t *testing.T) {
	mock := newMockRunner()
	mock.addResponse("", "Error: commit failed", errors.New("exit status 1"))

	client := NewClient("/test/dir")
	client.SetCommandRunner(mock.run)

	err := client.Commit(context.Background(), "Test")
	if err == nil {
		t.Fatal("Commit() should return error")
	}
}

func TestCommit_EmptyMessage(t *testing.T) {
	mock := newMockRunner()

	client := NewClient("/test/dir")
	client.SetCommandRunner(mock.run)

	err := client.Commit(context.Background(), "")
	if err == nil {
		t.Fatal("Commit() should return error for empty message")
	}
	if !strings.Contains(err.Error(), "empty") {
		t.Errorf("Commit() error = %q, want error about empty message", err)
	}

	// Should not have made any calls
	if len(mock.calls) != 0 {
		t.Errorf("Expected 0 calls, got %d", len(mock.calls))
	}
}

func TestCommit_WhitespaceOnlyMessage(t *testing.T) {
	mock := newMockRunner()

	client := NewClient("/test/dir")
	client.SetCommandRunner(mock.run)

	err := client.Commit(context.Background(), "   \n\t  ")
	if err == nil {
		t.Fatal("Commit() should return error for whitespace-only message")
	}
	if !strings.Contains(err.Error(), "empty") {
		t.Errorf("Commit() error = %q, want error about empty message", err)
	}
}

func TestCommit_NotRepo(t *testing.T) {
	mock := newMockRunner()
	mock.addResponse("", "Error: There is no jj repo in \"/test\"", errors.New("exit status 1"))

	client := NewClient("/test/dir")
	client.SetCommandRunner(mock.run)

	err := client.Commit(context.Background(), "Test")
	if !errors.Is(err, ErrNotRepo) {
		t.Errorf("Commit() error = %v, want ErrNotRepo", err)
	}
}

// =============================================================================
// Unit Tests - Sanitize Message
// =============================================================================

func TestSanitizeMessage(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name:     "normal message",
			input:    "Fix bug in parser",
			expected: "Fix bug in parser",
		},
		{
			name:     "message with leading/trailing whitespace",
			input:    "  Fix bug  ",
			expected: "Fix bug",
		},
		{
			name:     "message with null bytes",
			input:    "Fix bug\x00 in parser",
			expected: "Fix bug in parser",
		},
		{
			name:     "message with newlines",
			input:    "Fix bug\n\nWith details",
			expected: "Fix bug\n\nWith details",
		},
		{
			name:     "message with quotes",
			input:    `Fix "important" bug`,
			expected: `Fix "important" bug`,
		},
		{
			name:     "message with special chars",
			input:    "Fix bug ($variable)",
			expected: "Fix bug ($variable)",
		},
		{
			name:     "empty string",
			input:    "",
			expected: "",
		},
		{
			name:     "only whitespace",
			input:    "   \t\n  ",
			expected: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := sanitizeMessage(tt.input)
			if result != tt.expected {
				t.Errorf("sanitizeMessage(%q) = %q, want %q", tt.input, result, tt.expected)
			}
		})
	}
}

// =============================================================================
// Unit Tests - Status
// =============================================================================

func TestStatus(t *testing.T) {
	mock := newMockRunner()
	expectedOutput := "Working copy changes:\nM file.txt"
	mock.addResponse(expectedOutput, "", nil)

	client := NewClient("/test/dir")
	client.SetCommandRunner(mock.run)

	output, err := client.Status(context.Background())
	if err != nil {
		t.Fatalf("Status() returned error: %v", err)
	}

	if output != expectedOutput {
		t.Errorf("Status() output = %q, want %q", output, expectedOutput)
	}

	// Verify the call
	call := mock.calls[0]
	expectedArgs := []string{"status"}
	if !slices.Equal(call.args, expectedArgs) {
		t.Errorf("Status() called with args %v, want %v", call.args, expectedArgs)
	}
}

// =============================================================================
// Unit Tests - Log
// =============================================================================

func TestLog(t *testing.T) {
	mock := newMockRunner()
	mock.addResponse("log output", "", nil)

	client := NewClient("/test/dir")
	client.SetCommandRunner(mock.run)

	output, err := client.Log(context.Background(), "@", "")
	if err != nil {
		t.Fatalf("Log() returned error: %v", err)
	}

	if output != "log output" {
		t.Errorf("Log() output = %q, want %q", output, "log output")
	}

	// Verify the call
	call := mock.calls[0]
	expectedArgs := []string{"log", "-r", "@"}
	if !slices.Equal(call.args, expectedArgs) {
		t.Errorf("Log() called with args %v, want %v", call.args, expectedArgs)
	}
}

func TestLog_NoRevset(t *testing.T) {
	mock := newMockRunner()
	mock.addResponse("log output", "", nil)

	client := NewClient("/test/dir")
	client.SetCommandRunner(mock.run)

	_, err := client.Log(context.Background(), "", "")
	if err != nil {
		t.Fatalf("Log() returned error: %v", err)
	}

	// Verify the call has no -r flag
	call := mock.calls[0]
	expectedArgs := []string{"log"}
	if !slices.Equal(call.args, expectedArgs) {
		t.Errorf("Log() called with args %v, want %v", call.args, expectedArgs)
	}
}

func TestLog_WithTemplate(t *testing.T) {
	mock := newMockRunner()
	mock.addResponse("change_id123", "", nil)

	client := NewClient("/test/dir")
	client.SetCommandRunner(mock.run)

	output, err := client.Log(context.Background(), "@", "change_id")
	if err != nil {
		t.Fatalf("Log() returned error: %v", err)
	}

	if output != "change_id123" {
		t.Errorf("Log() output = %q, want %q", output, "change_id123")
	}

	// Verify the call includes both -r and -T flags
	call := mock.calls[0]
	expectedArgs := []string{"log", "-r", "@", "-T", "change_id"}
	if !slices.Equal(call.args, expectedArgs) {
		t.Errorf("Log() called with args %v, want %v", call.args, expectedArgs)
	}
}

// =============================================================================
// Unit Tests - Error Handling
// =============================================================================

func TestWrapError_CommandNotFound(t *testing.T) {
	mock := newMockRunner()
	mock.addResponse("", "", &exec.Error{Name: "jj", Err: exec.ErrNotFound})

	client := NewClient("/test/dir")
	client.SetCommandRunner(mock.run)

	err := client.New(context.Background())
	if !errors.Is(err, ErrCommandNotFound) {
		t.Errorf("New() error = %v, want ErrCommandNotFound", err)
	}
}

func TestWrapError_NotRepo(t *testing.T) {
	testCases := []string{
		"Error: There is no jj repo in \"/path\"",
		"error: not a jj repository",
		"Error: No such repo found",
	}

	for _, stderr := range testCases {
		t.Run(stderr, func(t *testing.T) {
			mock := newMockRunner()
			mock.addResponse("", stderr, errors.New("exit status 1"))

			client := NewClient("/test/dir")
			client.SetCommandRunner(mock.run)

			err := client.New(context.Background())
			if !errors.Is(err, ErrNotRepo) {
				t.Errorf("New() error = %v, want ErrNotRepo", err)
			}
		})
	}
}

func TestWrapError_ContextCanceled(t *testing.T) {
	mock := newMockRunner()
	mock.addResponse("", "", context.Canceled)

	client := NewClient("/test/dir")
	client.SetCommandRunner(mock.run)

	err := client.New(context.Background())
	if !errors.Is(err, context.Canceled) {
		t.Errorf("New() error = %v, want context.Canceled", err)
	}
}

func TestWrapError_ContextDeadlineExceeded(t *testing.T) {
	mock := newMockRunner()
	mock.addResponse("", "", context.DeadlineExceeded)

	client := NewClient("/test/dir")
	client.SetCommandRunner(mock.run)

	err := client.New(context.Background())
	if !errors.Is(err, context.DeadlineExceeded) {
		t.Errorf("New() error = %v, want context.DeadlineExceeded", err)
	}
}

func TestWrapError_GenericError(t *testing.T) {
	mock := newMockRunner()
	mock.addResponse("", "Some error message", errors.New("exit status 1"))

	client := NewClient("/test/dir")
	client.SetCommandRunner(mock.run)

	err := client.New(context.Background())
	if err == nil {
		t.Fatal("New() should return error")
	}
	if !strings.Contains(err.Error(), "jj new failed") {
		t.Errorf("New() error = %q, want error containing 'jj new failed'", err)
	}
	if !strings.Contains(err.Error(), "Some error message") {
		t.Errorf("New() error = %q, want error containing 'Some error message'", err)
	}
}

// =============================================================================
// Integration Tests
// =============================================================================

// hasJJ checks if the jj command is available in PATH.
func hasJJ() bool {
	_, err := exec.LookPath("jj")
	return err == nil
}

func TestIntegration_BasicWorkflow(t *testing.T) {
	if !hasJJ() {
		t.Skip("jj not installed, skipping integration test")
	}

	// Create a temporary directory
	tempDir, err := os.MkdirTemp("", "jj-test-*")
	if err != nil {
		t.Fatalf("Failed to create temp directory: %v", err)
	}
	t.Cleanup(func() { _ = os.RemoveAll(tempDir) })

	// Initialize a git repo first (jj needs a backend)
	gitInit := exec.Command("git", "init")
	gitInit.Dir = tempDir
	if err := gitInit.Run(); err != nil {
		t.Fatalf("Failed to initialize git repo: %v", err)
	}

	// Configure git user for the repo
	gitConfig1 := exec.Command("git", "config", "user.email", "test@example.com")
	gitConfig1.Dir = tempDir
	if err := gitConfig1.Run(); err != nil {
		t.Fatalf("Failed to configure git email: %v", err)
	}

	gitConfig2 := exec.Command("git", "config", "user.name", "Test User")
	gitConfig2.Dir = tempDir
	if err := gitConfig2.Run(); err != nil {
		t.Fatalf("Failed to configure git name: %v", err)
	}

	// Initialize jj
	jjInit := exec.Command("jj", "git", "init", "--colocate")
	jjInit.Dir = tempDir
	if output, err := jjInit.CombinedOutput(); err != nil {
		t.Fatalf("Failed to initialize jj repo: %v\nOutput: %s", err, output)
	}

	ctx := context.Background()
	client := NewClient(tempDir)

	// Test Status
	status, err := client.Status(ctx)
	if err != nil {
		t.Fatalf("Status() failed: %v", err)
	}
	_ = status // Status should return something

	// Create a file
	testFile := filepath.Join(tempDir, "test.txt")
	if err := os.WriteFile(testFile, []byte("hello world\n"), 0644); err != nil {
		t.Fatalf("Failed to create test file: %v", err)
	}

	// Test Commit
	err = client.Commit(ctx, "Add test file")
	if err != nil {
		t.Fatalf("Commit() failed: %v", err)
	}

	// Test New
	err = client.New(ctx)
	if err != nil {
		t.Fatalf("New() failed: %v", err)
	}

	// Create another file
	testFile2 := filepath.Join(tempDir, "test2.txt")
	if err := os.WriteFile(testFile2, []byte("second file\n"), 0644); err != nil {
		t.Fatalf("Failed to create second test file: %v", err)
	}

	// Test another Commit
	err = client.Commit(ctx, "Add second test file")
	if err != nil {
		t.Fatalf("Second Commit() failed: %v", err)
	}

	// Test Log
	logOutput, err := client.Log(ctx, "", "")
	if err != nil {
		t.Fatalf("Log() failed: %v", err)
	}
	// Log should contain our commit messages
	if !strings.Contains(logOutput, "test file") {
		t.Errorf("Log() output should contain commit messages, got: %s", logOutput)
	}
}

func TestIntegration_NotRepo(t *testing.T) {
	if !hasJJ() {
		t.Skip("jj not installed, skipping integration test")
	}

	// Create a temporary directory that is NOT a jj repo
	tempDir, err := os.MkdirTemp("", "jj-test-notrepo-*")
	if err != nil {
		t.Fatalf("Failed to create temp directory: %v", err)
	}
	t.Cleanup(func() { _ = os.RemoveAll(tempDir) })

	ctx := context.Background()
	client := NewClient(tempDir)

	err = client.New(ctx)
	if err == nil {
		t.Fatal("New() should fail in non-jj directory")
	}
	if !errors.Is(err, ErrNotRepo) {
		t.Logf("Error (expected ErrNotRepo or similar): %v", err)
	}
}

func TestIntegration_ContextCancellation(t *testing.T) {
	if !hasJJ() {
		t.Skip("jj not installed, skipping integration test")
	}

	// Create a temporary directory
	tempDir, err := os.MkdirTemp("", "jj-test-cancel-*")
	if err != nil {
		t.Fatalf("Failed to create temp directory: %v", err)
	}
	t.Cleanup(func() { _ = os.RemoveAll(tempDir) })

	// Initialize git and jj
	gitInit := exec.Command("git", "init")
	gitInit.Dir = tempDir
	if err := gitInit.Run(); err != nil {
		t.Fatalf("Failed to initialize git repo: %v", err)
	}

	jjInit := exec.Command("jj", "git", "init", "--colocate")
	jjInit.Dir = tempDir
	if err := jjInit.Run(); err != nil {
		t.Fatalf("Failed to initialize jj repo: %v", err)
	}

	// Create a context that's already cancelled
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	client := NewClient(tempDir)

	err = client.New(ctx)
	if err == nil {
		t.Fatal("New() should fail with cancelled context")
	}
	// The error might be context.Canceled or a wrapped error
	if !errors.Is(err, context.Canceled) {
		t.Logf("Error (expected context.Canceled): %v", err)
	}
}

func TestIntegration_ContextTimeout(t *testing.T) {
	if !hasJJ() {
		t.Skip("jj not installed, skipping integration test")
	}

	// Create a temporary directory
	tempDir, err := os.MkdirTemp("", "jj-test-timeout-*")
	if err != nil {
		t.Fatalf("Failed to create temp directory: %v", err)
	}
	t.Cleanup(func() { _ = os.RemoveAll(tempDir) })

	// Initialize git and jj
	gitInit := exec.Command("git", "init")
	gitInit.Dir = tempDir
	if err := gitInit.Run(); err != nil {
		t.Fatalf("Failed to initialize git repo: %v", err)
	}

	jjInit := exec.Command("jj", "git", "init", "--colocate")
	jjInit.Dir = tempDir
	if err := jjInit.Run(); err != nil {
		t.Fatalf("Failed to initialize jj repo: %v", err)
	}

	// Create a context with a reasonable timeout
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	client := NewClient(tempDir)

	// This should succeed within the timeout
	err = client.New(ctx)
	if err != nil {
		t.Logf("New() with timeout: %v", err)
	}
}
