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
// Unit Tests - IsEmpty
// =============================================================================

func TestIsEmpty_Empty(t *testing.T) {
	mock := newMockRunner()
	// jj diff returns empty output when there are no changes
	mock.addResponse("", "", nil)

	client := NewClient("/test/dir")
	client.SetCommandRunner(mock.run)

	isEmpty, err := client.IsEmpty(context.Background())
	if err != nil {
		t.Fatalf("IsEmpty() returned error: %v", err)
	}
	if !isEmpty {
		t.Error("IsEmpty() = false, want true for empty diff")
	}

	// Verify the call
	if len(mock.calls) != 1 {
		t.Fatalf("Expected 1 call, got %d", len(mock.calls))
	}

	call := mock.calls[0]
	expectedArgs := []string{"diff"}
	if !slices.Equal(call.args, expectedArgs) {
		t.Errorf("IsEmpty() called with args %v, want %v", call.args, expectedArgs)
	}
}

func TestIsEmpty_NotEmpty(t *testing.T) {
	mock := newMockRunner()
	// jj diff returns actual diff content when there are changes
	mock.addResponse("diff --git a/file.go b/file.go\n--- a/file.go\n+++ b/file.go\n@@ -1,3 +1,10 @@\n+new line", "", nil)

	client := NewClient("/test/dir")
	client.SetCommandRunner(mock.run)

	isEmpty, err := client.IsEmpty(context.Background())
	if err != nil {
		t.Fatalf("IsEmpty() returned error: %v", err)
	}
	if isEmpty {
		t.Error("IsEmpty() = true, want false for non-empty diff")
	}
}

func TestIsEmpty_WhitespaceOnly(t *testing.T) {
	mock := newMockRunner()
	// jj diff might return whitespace only (edge case)
	mock.addResponse("   \n\t  ", "", nil)

	client := NewClient("/test/dir")
	client.SetCommandRunner(mock.run)

	isEmpty, err := client.IsEmpty(context.Background())
	if err != nil {
		t.Fatalf("IsEmpty() returned error: %v", err)
	}
	if !isEmpty {
		t.Error("IsEmpty() = false, want true for whitespace-only output")
	}
}

func TestIsEmpty_Error(t *testing.T) {
	mock := newMockRunner()
	mock.addResponse("", "Error: failed", errors.New("exit status 1"))

	client := NewClient("/test/dir")
	client.SetCommandRunner(mock.run)

	_, err := client.IsEmpty(context.Background())
	if err == nil {
		t.Fatal("IsEmpty() should return error")
	}
}

func TestIsEmpty_NotRepo(t *testing.T) {
	mock := newMockRunner()
	mock.addResponse("", "Error: There is no jj repo in \"/test\"", errors.New("exit status 1"))

	client := NewClient("/test/dir")
	client.SetCommandRunner(mock.run)

	_, err := client.IsEmpty(context.Background())
	if !errors.Is(err, ErrNotRepo) {
		t.Errorf("IsEmpty() error = %v, want ErrNotRepo", err)
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

	_, err := client.Status(context.Background())
	if !errors.Is(err, ErrCommandNotFound) {
		t.Errorf("Status() error = %v, want ErrCommandNotFound", err)
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

			_, err := client.Status(context.Background())
			if !errors.Is(err, ErrNotRepo) {
				t.Errorf("Status() error = %v, want ErrNotRepo", err)
			}
		})
	}
}

func TestWrapError_ContextCanceled(t *testing.T) {
	mock := newMockRunner()
	mock.addResponse("", "", context.Canceled)

	client := NewClient("/test/dir")
	client.SetCommandRunner(mock.run)

	_, err := client.Status(context.Background())
	if !errors.Is(err, context.Canceled) {
		t.Errorf("Status() error = %v, want context.Canceled", err)
	}
}

func TestWrapError_ContextDeadlineExceeded(t *testing.T) {
	mock := newMockRunner()
	mock.addResponse("", "", context.DeadlineExceeded)

	client := NewClient("/test/dir")
	client.SetCommandRunner(mock.run)

	_, err := client.Status(context.Background())
	if !errors.Is(err, context.DeadlineExceeded) {
		t.Errorf("Status() error = %v, want context.DeadlineExceeded", err)
	}
}

func TestWrapError_GenericError(t *testing.T) {
	mock := newMockRunner()
	mock.addResponse("", "Some error message", errors.New("exit status 1"))

	client := NewClient("/test/dir")
	client.SetCommandRunner(mock.run)

	_, err := client.Status(context.Background())
	if err == nil {
		t.Fatal("Status() should return error")
	}
	if !strings.Contains(err.Error(), "jj status failed") {
		t.Errorf("Status() error = %q, want error containing 'jj status failed'", err)
	}
	if !strings.Contains(err.Error(), "Some error message") {
		t.Errorf("Status() error = %q, want error containing 'Some error message'", err)
	}
}

// =============================================================================
// Unit Tests - Diff
// =============================================================================

func TestDiff(t *testing.T) {
	mock := newMockRunner()
	mock.addResponse("diff output", "", nil)

	client := NewClient("/test/dir")
	client.SetCommandRunner(mock.run)

	output, err := client.Diff(context.Background(), "abc123", "@")
	if err != nil {
		t.Fatalf("Diff() error = %v", err)
	}

	if output != "diff output" {
		t.Errorf("Diff() output = %q, want %q", output, "diff output")
	}

	// Verify command was called correctly
	if len(mock.calls) != 1 {
		t.Fatalf("expected 1 call, got %d", len(mock.calls))
	}

	call := mock.calls[0]
	if call.name != "jj" {
		t.Errorf("command name = %q, want 'jj'", call.name)
	}
	expectedArgs := []string{"diff", "--from", "abc123", "--to", "@"}
	if !slices.Equal(call.args, expectedArgs) {
		t.Errorf("command args = %v, want %v", call.args, expectedArgs)
	}
}

func TestDiff_NoFrom(t *testing.T) {
	mock := newMockRunner()
	mock.addResponse("diff output", "", nil)

	client := NewClient("/test/dir")
	client.SetCommandRunner(mock.run)

	_, err := client.Diff(context.Background(), "", "@")
	if err != nil {
		t.Fatalf("Diff() error = %v", err)
	}

	// Verify --from was not included
	call := mock.calls[0]
	expectedArgs := []string{"diff", "--to", "@"}
	if !slices.Equal(call.args, expectedArgs) {
		t.Errorf("command args = %v, want %v", call.args, expectedArgs)
	}
}

func TestDiff_NoTo(t *testing.T) {
	mock := newMockRunner()
	mock.addResponse("diff output", "", nil)

	client := NewClient("/test/dir")
	client.SetCommandRunner(mock.run)

	_, err := client.Diff(context.Background(), "abc123", "")
	if err != nil {
		t.Fatalf("Diff() error = %v", err)
	}

	// Verify --to was not included
	call := mock.calls[0]
	expectedArgs := []string{"diff", "--from", "abc123"}
	if !slices.Equal(call.args, expectedArgs) {
		t.Errorf("command args = %v, want %v", call.args, expectedArgs)
	}
}

func TestDiff_Error(t *testing.T) {
	mock := newMockRunner()
	mock.addResponse("", "error message", errors.New("exit status 1"))

	client := NewClient("/test/dir")
	client.SetCommandRunner(mock.run)

	_, err := client.Diff(context.Background(), "abc123", "@")
	if err == nil {
		t.Fatal("Diff() should return error")
	}
}

// =============================================================================
// Unit Tests - GetCurrentChangeID
// =============================================================================

func TestGetCurrentChangeID(t *testing.T) {
	mock := newMockRunner()
	mock.addResponse("abc123def456\n", "", nil)

	client := NewClient("/test/dir")
	client.SetCommandRunner(mock.run)

	changeID, err := client.GetCurrentChangeID(context.Background())
	if err != nil {
		t.Fatalf("GetCurrentChangeID() error = %v", err)
	}

	if changeID != "abc123def456" {
		t.Errorf("GetCurrentChangeID() = %q, want %q", changeID, "abc123def456")
	}

	// Verify command was called correctly
	if len(mock.calls) != 1 {
		t.Fatalf("expected 1 call, got %d", len(mock.calls))
	}

	call := mock.calls[0]
	if call.name != "jj" {
		t.Errorf("command name = %q, want 'jj'", call.name)
	}
	expectedArgs := []string{"log", "-r", "@", "-T", "change_id", "--no-graph"}
	if !slices.Equal(call.args, expectedArgs) {
		t.Errorf("command args = %v, want %v", call.args, expectedArgs)
	}
}

func TestGetCurrentChangeID_Error(t *testing.T) {
	mock := newMockRunner()
	mock.addResponse("", "error message", errors.New("exit status 1"))

	client := NewClient("/test/dir")
	client.SetCommandRunner(mock.run)

	_, err := client.GetCurrentChangeID(context.Background())
	if err == nil {
		t.Fatal("GetCurrentChangeID() should return error")
	}
}

// =============================================================================
// Unit Tests - GetParentChangeID
// =============================================================================

func TestGetParentChangeID(t *testing.T) {
	mock := newMockRunner()
	mock.addResponse("parent123def456\n", "", nil)

	client := NewClient("/test/dir")
	client.SetCommandRunner(mock.run)

	changeID, err := client.GetParentChangeID(context.Background())
	if err != nil {
		t.Fatalf("GetParentChangeID() error = %v", err)
	}

	if changeID != "parent123def456" {
		t.Errorf("GetParentChangeID() = %q, want %q", changeID, "parent123def456")
	}

	// Verify command was called correctly
	if len(mock.calls) != 1 {
		t.Fatalf("expected 1 call, got %d", len(mock.calls))
	}

	call := mock.calls[0]
	expectedArgs := []string{"log", "-r", "@-", "-T", "change_id", "--no-graph"}
	if !slices.Equal(call.args, expectedArgs) {
		t.Errorf("GetParentChangeID() args = %v, want %v", call.args, expectedArgs)
	}
}

func TestGetParentChangeID_RootCommit(t *testing.T) {
	mock := newMockRunner()
	mock.addResponse("", "Error: cannot resolve @- root", errors.New("exit status 1"))

	client := NewClient("/test/dir")
	client.SetCommandRunner(mock.run)

	changeID, err := client.GetParentChangeID(context.Background())
	if err != nil {
		t.Fatalf("GetParentChangeID() should not return error for root commit, got: %v", err)
	}

	if changeID != "" {
		t.Errorf("GetParentChangeID() = %q, want empty string for root commit", changeID)
	}
}

func TestGetParentChangeID_Error(t *testing.T) {
	mock := newMockRunner()
	mock.addResponse("", "some other error", errors.New("exit status 1"))

	client := NewClient("/test/dir")
	client.SetCommandRunner(mock.run)

	_, err := client.GetParentChangeID(context.Background())
	if err == nil {
		t.Fatal("GetParentChangeID() should return error for non-root errors")
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

	// Test Show
	showOutput, err := client.Show(ctx)
	if err != nil {
		t.Fatalf("Show() failed: %v", err)
	}
	if showOutput == "" {
		t.Error("Show() should return non-empty output after file creation")
	}

	// Test Diff
	diffOutput, err := client.Diff(ctx, "", "")
	if err != nil {
		t.Fatalf("Diff() failed: %v", err)
	}
	_ = diffOutput

	// Test Log
	logOutput, err := client.Log(ctx, "", "")
	if err != nil {
		t.Fatalf("Log() failed: %v", err)
	}
	// Log should return non-empty output
	if logOutput == "" {
		t.Error("Log() should return non-empty output")
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

	_, err = client.Status(ctx)
	if err == nil {
		t.Fatal("Status() should fail in non-jj directory")
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

	_, err = client.Status(ctx)
	if err == nil {
		t.Fatal("Status() should fail with cancelled context")
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

	// Status should succeed within the timeout
	_, err = client.Status(ctx)
	if err != nil {
		t.Logf("Status() with timeout: %v", err)
	}
}
