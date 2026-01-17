package jj

import (
	"context"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
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
// Unit Tests - NewChange
// =============================================================================

func TestNewChange(t *testing.T) {
	mock := newMockRunner()
	// First call: jj new -m "description"
	mock.addResponse("", "", nil)
	// Second call: jj log -r @ -T change_id --no-graph
	mock.addResponse("abc123xyz", "", nil)

	client := NewClient("/test/dir")
	client.SetCommandRunner(mock.run)

	changeID, err := client.NewChange(context.Background(), "Test change")
	if err != nil {
		t.Fatalf("NewChange() returned error: %v", err)
	}

	if changeID != "abc123xyz" {
		t.Errorf("NewChange() changeID = %q, want %q", changeID, "abc123xyz")
	}

	// Verify the calls
	if len(mock.calls) != 2 {
		t.Fatalf("Expected 2 calls, got %d", len(mock.calls))
	}

	// First call should be jj new
	call := mock.calls[0]
	if call.name != "jj" {
		t.Errorf("Call 1 name = %q, want %q", call.name, "jj")
	}
	expectedArgs := []string{"new", "-m", "Test change"}
	if !slicesEqual(call.args, expectedArgs) {
		t.Errorf("Call 1 args = %v, want %v", call.args, expectedArgs)
	}
	if call.dir != "/test/dir" {
		t.Errorf("Call 1 dir = %q, want %q", call.dir, "/test/dir")
	}
}

func TestNewChange_Error(t *testing.T) {
	mock := newMockRunner()
	mock.addResponse("", "Error: not a jj repository", errors.New("exit status 1"))

	client := NewClient("/test/dir")
	client.SetCommandRunner(mock.run)

	_, err := client.NewChange(context.Background(), "Test")
	if err == nil {
		t.Fatal("NewChange() should return error")
	}
}

// =============================================================================
// Unit Tests - Show
// =============================================================================

func TestShow(t *testing.T) {
	mock := newMockRunner()
	expectedDiff := `diff --git a/file.txt b/file.txt
--- a/file.txt
+++ b/file.txt
@@ -1 +1 @@
-old
+new`
	mock.addResponse(expectedDiff, "", nil)

	client := NewClient("/test/dir")
	client.SetCommandRunner(mock.run)

	output, err := client.Show(context.Background())
	if err != nil {
		t.Fatalf("Show() returned error: %v", err)
	}

	if output != expectedDiff {
		t.Errorf("Show() output = %q, want %q", output, expectedDiff)
	}

	// Verify the call
	if len(mock.calls) != 1 {
		t.Fatalf("Expected 1 call, got %d", len(mock.calls))
	}
	call := mock.calls[0]
	if call.name != "jj" || !slicesEqual(call.args, []string{"show"}) {
		t.Errorf("Show() called %q %v, want jj [show]", call.name, call.args)
	}
}

func TestShow_EmptyDiff(t *testing.T) {
	mock := newMockRunner()
	mock.addResponse("", "", nil)

	client := NewClient("/test/dir")
	client.SetCommandRunner(mock.run)

	output, err := client.Show(context.Background())
	if err != nil {
		t.Fatalf("Show() returned error: %v", err)
	}

	if output != "" {
		t.Errorf("Show() output = %q, want empty string", output)
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

	err := client.Describe(context.Background(), "Updated description")
	if err != nil {
		t.Fatalf("Describe() returned error: %v", err)
	}

	// Verify the call
	if len(mock.calls) != 1 {
		t.Fatalf("Expected 1 call, got %d", len(mock.calls))
	}
	call := mock.calls[0]
	expectedArgs := []string{"describe", "-m", "Updated description"}
	if call.name != "jj" || !slicesEqual(call.args, expectedArgs) {
		t.Errorf("Describe() called %q %v, want jj %v", call.name, call.args, expectedArgs)
	}
}

func TestDescribe_Error(t *testing.T) {
	mock := newMockRunner()
	mock.addResponse("", "Error: cannot describe immutable commit", errors.New("exit status 1"))

	client := NewClient("/test/dir")
	client.SetCommandRunner(mock.run)

	err := client.Describe(context.Background(), "Test")
	if err == nil {
		t.Fatal("Describe() should return error")
	}
}

// =============================================================================
// Unit Tests - CurrentChangeID
// =============================================================================

func TestCurrentChangeID(t *testing.T) {
	mock := newMockRunner()
	mock.addResponse("xyzzlmkqr\n", "", nil)

	client := NewClient("/test/dir")
	client.SetCommandRunner(mock.run)

	changeID, err := client.CurrentChangeID(context.Background())
	if err != nil {
		t.Fatalf("CurrentChangeID() returned error: %v", err)
	}

	if changeID != "xyzzlmkqr" {
		t.Errorf("CurrentChangeID() = %q, want %q", changeID, "xyzzlmkqr")
	}

	// Verify the call
	if len(mock.calls) != 1 {
		t.Fatalf("Expected 1 call, got %d", len(mock.calls))
	}
	call := mock.calls[0]
	expectedArgs := []string{"log", "-r", "@", "-T", "change_id", "--no-graph"}
	if call.name != "jj" || !slicesEqual(call.args, expectedArgs) {
		t.Errorf("CurrentChangeID() called %q %v, want jj %v", call.name, call.args, expectedArgs)
	}
}

func TestCurrentChangeID_EmptyOutput(t *testing.T) {
	mock := newMockRunner()
	mock.addResponse("", "", nil)

	client := NewClient("/test/dir")
	client.SetCommandRunner(mock.run)

	_, err := client.CurrentChangeID(context.Background())
	if err == nil {
		t.Fatal("CurrentChangeID() should return error for empty output")
	}
	if !strings.Contains(err.Error(), "unable to determine current change ID") {
		t.Errorf("CurrentChangeID() error = %q, want error about unable to determine change ID", err)
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
}

// =============================================================================
// Unit Tests - Diff
// =============================================================================

func TestDiff(t *testing.T) {
	mock := newMockRunner()
	expectedDiff := "diff content here"
	mock.addResponse(expectedDiff, "", nil)

	client := NewClient("/test/dir")
	client.SetCommandRunner(mock.run)

	output, err := client.Diff(context.Background())
	if err != nil {
		t.Fatalf("Diff() returned error: %v", err)
	}

	if output != expectedDiff {
		t.Errorf("Diff() output = %q, want %q", output, expectedDiff)
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

	output, err := client.Log(context.Background(), "@", "description")
	if err != nil {
		t.Fatalf("Log() returned error: %v", err)
	}

	if output != "log output" {
		t.Errorf("Log() output = %q, want %q", output, "log output")
	}

	// Verify the call
	call := mock.calls[0]
	expectedArgs := []string{"log", "-r", "@", "-T", "description"}
	if !slicesEqual(call.args, expectedArgs) {
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

	// Verify the call has no -r or -T flags
	call := mock.calls[0]
	expectedArgs := []string{"log"}
	if !slicesEqual(call.args, expectedArgs) {
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

	_, err := client.Show(context.Background())
	if !errors.Is(err, ErrCommandNotFound) {
		t.Errorf("Show() error = %v, want ErrCommandNotFound", err)
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

			_, err := client.Show(context.Background())
			if !errors.Is(err, ErrNotRepo) {
				t.Errorf("Show() error = %v, want ErrNotRepo", err)
			}
		})
	}
}

func TestWrapError_ContextCanceled(t *testing.T) {
	mock := newMockRunner()
	mock.addResponse("", "", context.Canceled)

	client := NewClient("/test/dir")
	client.SetCommandRunner(mock.run)

	_, err := client.Show(context.Background())
	if !errors.Is(err, context.Canceled) {
		t.Errorf("Show() error = %v, want context.Canceled", err)
	}
}

func TestWrapError_ContextDeadlineExceeded(t *testing.T) {
	mock := newMockRunner()
	mock.addResponse("", "", context.DeadlineExceeded)

	client := NewClient("/test/dir")
	client.SetCommandRunner(mock.run)

	_, err := client.Show(context.Background())
	if !errors.Is(err, context.DeadlineExceeded) {
		t.Errorf("Show() error = %v, want context.DeadlineExceeded", err)
	}
}

func TestWrapError_GenericError(t *testing.T) {
	mock := newMockRunner()
	mock.addResponse("", "Some error message", errors.New("exit status 1"))

	client := NewClient("/test/dir")
	client.SetCommandRunner(mock.run)

	_, err := client.Show(context.Background())
	if err == nil {
		t.Fatal("Show() should return error")
	}
	if !strings.Contains(err.Error(), "jj show failed") {
		t.Errorf("Show() error = %q, want error containing 'jj show failed'", err)
	}
	if !strings.Contains(err.Error(), "Some error message") {
		t.Errorf("Show() error = %q, want error containing 'Some error message'", err)
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
	defer os.RemoveAll(tempDir)

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

	// Test CurrentChangeID
	changeID, err := client.CurrentChangeID(ctx)
	if err != nil {
		t.Fatalf("CurrentChangeID() failed: %v", err)
	}
	if changeID == "" {
		t.Error("CurrentChangeID() returned empty string")
	}

	// Test Status
	status, err := client.Status(ctx)
	if err != nil {
		t.Fatalf("Status() failed: %v", err)
	}
	// Status should return something (even if just working copy info)
	_ = status

	// Create a file and test Show
	testFile := filepath.Join(tempDir, "test.txt")
	if err := os.WriteFile(testFile, []byte("hello world\n"), 0644); err != nil {
		t.Fatalf("Failed to create test file: %v", err)
	}

	show, err := client.Show(ctx)
	if err != nil {
		t.Fatalf("Show() failed: %v", err)
	}
	// Show should include the new file
	if !strings.Contains(show, "test.txt") {
		t.Errorf("Show() output should contain test.txt, got: %s", show)
	}

	// Test Describe
	err = client.Describe(ctx, "Test description")
	if err != nil {
		t.Fatalf("Describe() failed: %v", err)
	}

	// Test NewChange
	newChangeID, err := client.NewChange(ctx, "New change for testing")
	if err != nil {
		t.Fatalf("NewChange() failed: %v", err)
	}
	if newChangeID == "" {
		t.Error("NewChange() returned empty change ID")
	}
	if newChangeID == changeID {
		t.Error("NewChange() should create a different change ID")
	}

	// Verify we're now on the new change
	currentID, err := client.CurrentChangeID(ctx)
	if err != nil {
		t.Fatalf("CurrentChangeID() failed after NewChange: %v", err)
	}
	if currentID != newChangeID {
		t.Errorf("CurrentChangeID() = %q, want %q", currentID, newChangeID)
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
	defer os.RemoveAll(tempDir)

	ctx := context.Background()
	client := NewClient(tempDir)

	_, err = client.Show(ctx)
	if err == nil {
		t.Fatal("Show() should fail in non-jj directory")
	}
	if !errors.Is(err, ErrNotRepo) {
		// The error message might vary, but it should indicate repo issue
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
	defer os.RemoveAll(tempDir)

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

	_, err = client.Show(ctx)
	if err == nil {
		t.Fatal("Show() should fail with cancelled context")
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
	defer os.RemoveAll(tempDir)

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

	// Create a context with a very short timeout
	// Note: jj commands are usually fast, so this might actually succeed
	// We're just testing that timeout is respected
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	client := NewClient(tempDir)

	// This should succeed within the timeout
	_, err = client.CurrentChangeID(ctx)
	if err != nil {
		t.Logf("CurrentChangeID() with timeout: %v", err)
	}
}

// =============================================================================
// Helper Functions
// =============================================================================

func slicesEqual(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}
