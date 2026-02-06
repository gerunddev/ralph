package main

import (
	"bytes"
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/gerunddev/ralph/internal/app"
	"github.com/gerunddev/ralph/internal/db"
	"github.com/gerunddev/ralph/internal/jj"
	"github.com/spf13/cobra"
)

// executeCommand is a test helper that executes a cobra command with args.
func executeCommand(root *cobra.Command, args ...string) (output string, err error) {
	buf := new(bytes.Buffer)
	root.SetOut(buf)
	root.SetErr(buf)
	root.SetArgs(args)

	err = root.Execute()
	return buf.String(), err
}

// testFlags holds the parsed flag values for testing.
type testFlags struct {
	planPath      string
	promptStr     string
	resumeID      string
	maxIterations int
	extremeMode   bool
}

// createTestCommand creates a test version of the root command
// that captures flags and includes all validation logic from main.go.
func createTestCommand() (*cobra.Command, *testFlags) {
	flags := &testFlags{}

	rootCmd := &cobra.Command{
		Use:   "ralph [plan-file]",
		Short: "Iterative AI development with a single agent loop",
		Long: `Ralph runs an AI agent iteratively against a plan file.
The agent works until it declares completion or hits max iterations.
Each iteration creates a jj commit with the changes.

Examples:
  ralph plan.md                    # Start new execution from plan file
  ralph plan.md --max-iterations 30  # Start with custom iteration limit
  ralph -r abc123                  # Resume existing plan by ID
  ralph --resume abc123            # Resume existing plan by ID
  ralph -p "Fix the login bug"     # Start execution with inline prompt`,
		Args: cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			// Validate max-iterations is not negative (same as main.go)
			if flags.maxIterations < 0 {
				return errors.New("--max-iterations cannot be negative")
			}

			// Skip jj validation in test command

			// Validate mode selection (same as main.go)
			if flags.resumeID != "" {
				if len(args) > 0 || flags.promptStr != "" {
					return errors.New("cannot specify both --resume and plan file or --prompt")
				}
				// Resume mode - valid
				return nil
			}

			if flags.promptStr != "" {
				if len(args) > 0 {
					return errors.New("cannot specify both plan file and --prompt")
				}
				// Prompt mode - valid
				return nil
			}

			if len(args) == 0 {
				return errors.New("plan file required (or use --resume or --prompt)")
			}

			// Capture the plan path argument
			flags.planPath = args[0]
			return nil
		},
	}

	rootCmd.Flags().StringVarP(&flags.resumeID, "resume", "r", "",
		"Resume execution of an existing plan by ID")
	rootCmd.Flags().StringVarP(&flags.promptStr, "prompt", "p", "",
		"Use inline prompt as the plan instead of a file")
	rootCmd.Flags().IntVar(&flags.maxIterations, "max-iterations", 0,
		"Override max iterations from config")
	rootCmd.Flags().BoolVarP(&flags.extremeMode, "extreme", "x", false,
		"Extreme mode: run +3 iterations after robots think they're done")

	return rootCmd, flags
}

func TestCLI_Help(t *testing.T) {
	cmd, _ := createTestCommand()

	output, err := executeCommand(cmd, "--help")
	if err != nil {
		t.Errorf("Unexpected error: %v", err)
	}

	// Check help contains expected elements
	expectedStrings := []string{
		"ralph",
		"[plan-file]",
		"AI agent iteratively",
		"-r, --resume",
		"--max-iterations",
		"Resume execution of an existing plan by ID",
		"Override max iterations from config",
	}

	for _, expected := range expectedStrings {
		if !strings.Contains(output, expected) {
			t.Errorf("Help should contain %q, got:\n%s", expected, output)
		}
	}
}

func TestCLI_HelpContainsExamples(t *testing.T) {
	cmd, _ := createTestCommand()

	output, _ := executeCommand(cmd, "--help")

	// Check that examples are included
	if !strings.Contains(output, "Examples:") {
		t.Error("Help should contain Examples section")
	}
	if !strings.Contains(output, "ralph plan.md") {
		t.Error("Help should contain example: ralph plan.md")
	}
	if !strings.Contains(output, "ralph -r abc123") {
		t.Error("Help should contain example: ralph -r abc123")
	}
}

func TestCLI_PlanFileArgument(t *testing.T) {
	cmd, flags := createTestCommand()

	_, err := executeCommand(cmd, "/path/to/plan.md")
	if err != nil {
		t.Errorf("Unexpected error: %v", err)
	}

	if flags.planPath != "/path/to/plan.md" {
		t.Errorf("Expected planPath '/path/to/plan.md', got %q", flags.planPath)
	}
}

func TestCLI_PlanFileWithMaxIterations(t *testing.T) {
	cmd, flags := createTestCommand()

	_, err := executeCommand(cmd, "/path/to/plan.md", "--max-iterations", "30")
	if err != nil {
		t.Errorf("Unexpected error: %v", err)
	}

	if flags.planPath != "/path/to/plan.md" {
		t.Errorf("Expected planPath '/path/to/plan.md', got %q", flags.planPath)
	}
	if flags.maxIterations != 30 {
		t.Errorf("Expected maxIterations 30, got %d", flags.maxIterations)
	}
}

func TestCLI_ResumeFlagShort(t *testing.T) {
	cmd, flags := createTestCommand()

	_, err := executeCommand(cmd, "-r", "abc123")
	if err != nil {
		t.Errorf("Unexpected error: %v", err)
	}

	if flags.resumeID != "abc123" {
		t.Errorf("Expected resumeID 'abc123', got %q", flags.resumeID)
	}
}

func TestCLI_ResumeFlagLong(t *testing.T) {
	cmd, flags := createTestCommand()

	_, err := executeCommand(cmd, "--resume", "xyz789")
	if err != nil {
		t.Errorf("Unexpected error: %v", err)
	}

	if flags.resumeID != "xyz789" {
		t.Errorf("Expected resumeID 'xyz789', got %q", flags.resumeID)
	}
}

func TestCLI_ResumeFlagEquals(t *testing.T) {
	cmd, flags := createTestCommand()

	_, err := executeCommand(cmd, "--resume=plan-id-here")
	if err != nil {
		t.Errorf("Unexpected error: %v", err)
	}

	if flags.resumeID != "plan-id-here" {
		t.Errorf("Expected resumeID 'plan-id-here', got %q", flags.resumeID)
	}
}

func TestCLI_ResumeWithMaxIterations(t *testing.T) {
	cmd, flags := createTestCommand()

	_, err := executeCommand(cmd, "-r", "abc123", "--max-iterations", "50")
	if err != nil {
		t.Errorf("Unexpected error: %v", err)
	}

	if flags.resumeID != "abc123" {
		t.Errorf("Expected resumeID 'abc123', got %q", flags.resumeID)
	}
	if flags.maxIterations != 50 {
		t.Errorf("Expected maxIterations 50, got %d", flags.maxIterations)
	}
}

func TestCLI_MaxIterationsFlag(t *testing.T) {
	cmd, flags := createTestCommand()

	_, err := executeCommand(cmd, "plan.md", "--max-iterations", "100")
	if err != nil {
		t.Errorf("Unexpected error: %v", err)
	}

	if flags.maxIterations != 100 {
		t.Errorf("Expected maxIterations 100, got %d", flags.maxIterations)
	}
}

func TestCLI_MaxIterationsZeroDefault(t *testing.T) {
	cmd, flags := createTestCommand()

	_, err := executeCommand(cmd, "plan.md")
	if err != nil {
		t.Errorf("Unexpected error: %v", err)
	}

	if flags.maxIterations != 0 {
		t.Errorf("Expected maxIterations 0 (default), got %d", flags.maxIterations)
	}
}

func TestCLI_InvalidFlag(t *testing.T) {
	cmd, _ := createTestCommand()

	_, err := executeCommand(cmd, "--invalid-flag")
	if err == nil {
		t.Error("Expected error for invalid flag")
	}
}

func TestCLI_ResumeFlagMissingValue(t *testing.T) {
	cmd, _ := createTestCommand()

	_, err := executeCommand(cmd, "-r")
	if err == nil {
		t.Error("Expected error for -r without value")
	}
}

func TestCLI_TooManyArguments(t *testing.T) {
	cmd, _ := createTestCommand()

	_, err := executeCommand(cmd, "plan1.md", "plan2.md")
	if err == nil {
		t.Error("Expected error for too many arguments")
	}
}

func TestCLI_RelativePath(t *testing.T) {
	cmd, flags := createTestCommand()

	_, err := executeCommand(cmd, "./relative/plan.md")
	if err != nil {
		t.Errorf("Unexpected error: %v", err)
	}

	if flags.planPath != "./relative/plan.md" {
		t.Errorf("Expected planPath './relative/plan.md', got %q", flags.planPath)
	}
}

func TestCLI_PathWithSpaces(t *testing.T) {
	cmd, flags := createTestCommand()

	_, err := executeCommand(cmd, "/path/with spaces/plan.md")
	if err != nil {
		t.Errorf("Unexpected error: %v", err)
	}

	if flags.planPath != "/path/with spaces/plan.md" {
		t.Errorf("Expected planPath with spaces, got %q", flags.planPath)
	}
}

func TestCLI_ShortDescription(t *testing.T) {
	cmd, _ := createTestCommand()

	if cmd.Short != "Iterative AI development with a single agent loop" {
		t.Errorf("Unexpected short description: %s", cmd.Short)
	}
}

func TestCLI_UseLine(t *testing.T) {
	cmd, _ := createTestCommand()

	if cmd.Use != "ralph [plan-file]" {
		t.Errorf("Unexpected use line: %s", cmd.Use)
	}
}

// Test that max-iterations accepts only valid integers
func TestCLI_MaxIterationsInvalidValue(t *testing.T) {
	cmd, _ := createTestCommand()

	_, err := executeCommand(cmd, "--max-iterations", "not-a-number")
	if err == nil {
		t.Error("Expected error for non-integer max-iterations")
	}
}

func TestCLI_MaxIterationsNegativeValue(t *testing.T) {
	cmd, _ := createTestCommand()

	// Negative values should be rejected at CLI level
	_, err := executeCommand(cmd, "plan.md", "--max-iterations", "-5")
	if err == nil {
		t.Error("Expected error for negative max-iterations")
	}
	if err != nil && !strings.Contains(err.Error(), "cannot be negative") {
		t.Errorf("Expected 'cannot be negative' error, got: %v", err)
	}
}

// Tests using createTestCommand() to test validation logic
// This ensures tests stay in sync with any changes to validation logic in main.go

func TestValidation_NoPlanFileNoResume(t *testing.T) {
	cmd, _ := createTestCommand()

	_, err := executeCommand(cmd)
	if err == nil {
		t.Error("Expected error when no plan file and no --resume")
	}
	if err != nil && !strings.Contains(err.Error(), "plan file required") {
		t.Errorf("Expected 'plan file required' error, got: %v", err)
	}
}

func TestValidation_PromptFlagWorks(t *testing.T) {
	cmd, flags := createTestCommand()

	_, err := executeCommand(cmd, "-p", "Fix the login bug")
	if err != nil {
		t.Errorf("Unexpected error with valid --prompt: %v", err)
	}

	if flags.promptStr != "Fix the login bug" {
		t.Errorf("Expected promptStr 'Fix the login bug', got %q", flags.promptStr)
	}
}

func TestValidation_PromptFlagLong(t *testing.T) {
	cmd, flags := createTestCommand()

	_, err := executeCommand(cmd, "--prompt", "Add user authentication")
	if err != nil {
		t.Errorf("Unexpected error with valid --prompt: %v", err)
	}

	if flags.promptStr != "Add user authentication" {
		t.Errorf("Expected promptStr 'Add user authentication', got %q", flags.promptStr)
	}
}

func TestValidation_PromptWithMaxIterations(t *testing.T) {
	cmd, flags := createTestCommand()

	_, err := executeCommand(cmd, "-p", "Fix bug", "--max-iterations", "15")
	if err != nil {
		t.Errorf("Unexpected error: %v", err)
	}

	if flags.promptStr != "Fix bug" {
		t.Errorf("Expected promptStr 'Fix bug', got %q", flags.promptStr)
	}
	if flags.maxIterations != 15 {
		t.Errorf("Expected maxIterations 15, got %d", flags.maxIterations)
	}
}

func TestValidation_BothPlanFileAndPrompt(t *testing.T) {
	cmd, _ := createTestCommand()

	_, err := executeCommand(cmd, "plan.md", "-p", "Some prompt")
	if err == nil {
		t.Error("Expected error when both plan file and --prompt are specified")
	}
	if err != nil && !strings.Contains(err.Error(), "cannot specify both") {
		t.Errorf("Expected 'cannot specify both' error, got: %v", err)
	}
}

func TestValidation_BothResumeAndPrompt(t *testing.T) {
	cmd, _ := createTestCommand()

	_, err := executeCommand(cmd, "-r", "abc123", "-p", "Some prompt")
	if err == nil {
		t.Error("Expected error when both --resume and --prompt are specified")
	}
	if err != nil && !strings.Contains(err.Error(), "cannot specify both") {
		t.Errorf("Expected 'cannot specify both' error, got: %v", err)
	}
}

func TestValidation_BothPlanFileAndResume(t *testing.T) {
	cmd, _ := createTestCommand()

	_, err := executeCommand(cmd, "plan.md", "-r", "abc123")
	if err == nil {
		t.Error("Expected error when both plan file and --resume are specified")
	}
	if err != nil && !strings.Contains(err.Error(), "cannot specify both") {
		t.Errorf("Expected 'cannot specify both' error, got: %v", err)
	}
}

func TestValidation_ResumeOnlyWorks(t *testing.T) {
	cmd, _ := createTestCommand()

	_, err := executeCommand(cmd, "-r", "abc123")
	if err != nil {
		t.Errorf("Unexpected error with valid --resume: %v", err)
	}
}

func TestValidation_PlanFileOnlyWorks(t *testing.T) {
	cmd, _ := createTestCommand()

	_, err := executeCommand(cmd, "plan.md")
	if err != nil {
		t.Errorf("Unexpected error with valid plan file: %v", err)
	}
}

// Tests for validateJJRepository function

func TestValidateJJRepository_Success(t *testing.T) {
	// Save original and restore after test
	originalValidator := jjValidator
	defer func() { jjValidator = originalValidator }()

	// Mock jj validator to return success
	jjValidator = func(ctx context.Context, workDir string) error {
		return nil
	}

	err := validateJJRepository(context.Background())
	if err != nil {
		t.Errorf("Expected no error, got: %v", err)
	}
}

func TestValidateJJRepository_NotRepo(t *testing.T) {
	// Save original and restore after test
	originalValidator := jjValidator
	defer func() { jjValidator = originalValidator }()

	// Mock jj validator to return ErrNotRepo
	jjValidator = func(ctx context.Context, workDir string) error {
		return jj.ErrNotRepo
	}

	err := validateJJRepository(context.Background())
	if err == nil {
		t.Error("Expected error for not a jj repository")
	}
	if !strings.Contains(err.Error(), "not a jj repository") {
		t.Errorf("Expected 'not a jj repository' error, got: %v", err)
	}
}

func TestValidateJJRepository_CommandNotFound(t *testing.T) {
	// Save original and restore after test
	originalValidator := jjValidator
	defer func() { jjValidator = originalValidator }()

	// Mock jj validator to return ErrCommandNotFound
	jjValidator = func(ctx context.Context, workDir string) error {
		return jj.ErrCommandNotFound
	}

	err := validateJJRepository(context.Background())
	if err == nil {
		t.Error("Expected error for jj command not found")
	}
	if !strings.Contains(err.Error(), "jj command not found") {
		t.Errorf("Expected 'jj command not found' error, got: %v", err)
	}
}

func TestValidateJJRepository_OtherError(t *testing.T) {
	// Save original and restore after test
	originalValidator := jjValidator
	defer func() { jjValidator = originalValidator }()

	// Mock jj validator to return a generic error
	jjValidator = func(ctx context.Context, workDir string) error {
		return errors.New("some other jj error")
	}

	err := validateJJRepository(context.Background())
	if err == nil {
		t.Error("Expected error for generic jj failure")
	}
	if !strings.Contains(err.Error(), "failed to verify jj repository") {
		t.Errorf("Expected 'failed to verify jj repository' error, got: %v", err)
	}
}

// Tests for runNew function

func TestRunNew_PlanFileNotFound(t *testing.T) {
	// Save original and restore after test
	originalFactory := appFactory
	defer func() { appFactory = originalFactory }()

	// App factory should not even be called
	appFactory = func(cfg app.Config) (App, error) {
		t.Error("appFactory should not be called when plan file doesn't exist")
		return nil, nil
	}

	tempDir := t.TempDir()
	nonExistentPath := filepath.Join(tempDir, "nonexistent.md")

	err := runNew(context.Background(), nonExistentPath, 0, false)
	if err == nil {
		t.Error("Expected error for non-existent plan file")
	}
	if !strings.Contains(err.Error(), "plan file not found") {
		t.Errorf("Expected 'plan file not found' error, got: %v", err)
	}
}

func TestRunNew_PlanFileExists_AppFactoryError(t *testing.T) {
	// Save original and restore after test
	originalFactory := appFactory
	defer func() { appFactory = originalFactory }()

	// Mock app factory to return an error
	appFactory = func(cfg app.Config) (App, error) {
		return nil, errors.New("failed to create app")
	}

	// Create a temporary plan file
	tempDir := t.TempDir()
	planPath := filepath.Join(tempDir, "plan.md")
	err := os.WriteFile(planPath, []byte("# Test Plan"), 0644)
	if err != nil {
		t.Fatalf("Failed to create test plan file: %v", err)
	}

	err = runNew(context.Background(), planPath, 0, false)
	if err == nil {
		t.Error("Expected error from app factory")
	}
	if !strings.Contains(err.Error(), "failed to create app") {
		t.Errorf("Expected 'failed to create app' error, got: %v", err)
	}
}

func TestRunNew_Success(t *testing.T) {
	// Save original and restore after test
	originalFactory := appFactory
	defer func() { appFactory = originalFactory }()

	// Track if Run was called with correct arguments
	var capturedPlanPath string
	var capturedMaxIterations int

	mockApp := &mockAppImpl{
		runFunc: func(ctx context.Context, planPath string) error {
			capturedPlanPath = planPath
			return nil
		},
	}

	appFactory = func(cfg app.Config) (App, error) {
		capturedMaxIterations = cfg.MaxIterationsOverride
		return mockApp, nil
	}

	// Create a temporary plan file
	tempDir := t.TempDir()
	planPath := filepath.Join(tempDir, "plan.md")
	err := os.WriteFile(planPath, []byte("# Test Plan"), 0644)
	if err != nil {
		t.Fatalf("Failed to create test plan file: %v", err)
	}

	err = runNew(context.Background(), planPath, 25, false)
	if err != nil {
		t.Errorf("Unexpected error: %v", err)
	}

	if capturedPlanPath != planPath {
		t.Errorf("Expected plan path %q, got %q", planPath, capturedPlanPath)
	}
	if capturedMaxIterations != 25 {
		t.Errorf("Expected maxIterations 25, got %d", capturedMaxIterations)
	}
}

func TestRunNew_AppRunError(t *testing.T) {
	// Save original and restore after test
	originalFactory := appFactory
	defer func() { appFactory = originalFactory }()

	mockApp := &mockAppImpl{
		runFunc: func(ctx context.Context, planPath string) error {
			return errors.New("execution failed")
		},
	}

	appFactory = func(cfg app.Config) (App, error) {
		return mockApp, nil
	}

	// Create a temporary plan file
	tempDir := t.TempDir()
	planPath := filepath.Join(tempDir, "plan.md")
	err := os.WriteFile(planPath, []byte("# Test Plan"), 0644)
	if err != nil {
		t.Fatalf("Failed to create test plan file: %v", err)
	}

	err = runNew(context.Background(), planPath, 0, false)
	if err == nil {
		t.Error("Expected error from app.Run")
	}
	if !strings.Contains(err.Error(), "execution failed") {
		t.Errorf("Expected 'execution failed' error, got: %v", err)
	}
}

// Tests for runNewWithPrompt function

func TestRunNewWithPrompt_AppFactoryError(t *testing.T) {
	// Save original and restore after test
	originalFactory := appFactory
	defer func() { appFactory = originalFactory }()

	// Mock app factory to return an error
	appFactory = func(cfg app.Config) (App, error) {
		return nil, errors.New("failed to create app")
	}

	err := runNewWithPrompt(context.Background(), "Fix the bug", 0, false)
	if err == nil {
		t.Error("Expected error from app factory")
	}
	if !strings.Contains(err.Error(), "failed to create app") {
		t.Errorf("Expected 'failed to create app' error, got: %v", err)
	}
}

func TestRunNewWithPrompt_Success(t *testing.T) {
	// Save original and restore after test
	originalFactory := appFactory
	defer func() { appFactory = originalFactory }()

	// Track if RunWithPrompt was called with correct arguments
	var capturedPrompt string
	var capturedMaxIterations int

	mockApp := &mockAppImpl{
		runWithPromptFunc: func(ctx context.Context, prompt string) error {
			capturedPrompt = prompt
			return nil
		},
	}

	appFactory = func(cfg app.Config) (App, error) {
		capturedMaxIterations = cfg.MaxIterationsOverride
		return mockApp, nil
	}

	err := runNewWithPrompt(context.Background(), "Fix the login bug", 20, false)
	if err != nil {
		t.Errorf("Unexpected error: %v", err)
	}

	if capturedPrompt != "Fix the login bug" {
		t.Errorf("Expected prompt 'Fix the login bug', got %q", capturedPrompt)
	}
	if capturedMaxIterations != 20 {
		t.Errorf("Expected maxIterations 20, got %d", capturedMaxIterations)
	}
}

func TestRunNewWithPrompt_AppRunError(t *testing.T) {
	// Save original and restore after test
	originalFactory := appFactory
	defer func() { appFactory = originalFactory }()

	mockApp := &mockAppImpl{
		runWithPromptFunc: func(ctx context.Context, prompt string) error {
			return errors.New("execution failed")
		},
	}

	appFactory = func(cfg app.Config) (App, error) {
		return mockApp, nil
	}

	err := runNewWithPrompt(context.Background(), "Fix bug", 0, false)
	if err == nil {
		t.Error("Expected error from app.RunWithPrompt")
	}
	if !strings.Contains(err.Error(), "execution failed") {
		t.Errorf("Expected 'execution failed' error, got: %v", err)
	}
}

// Tests for runResume function

func TestRunResume_AppFactoryError(t *testing.T) {
	// Save original and restore after test
	originalFactory := appFactory
	defer func() { appFactory = originalFactory }()

	// Mock app factory to return an error
	appFactory = func(cfg app.Config) (App, error) {
		return nil, errors.New("failed to create app")
	}

	err := runResume(context.Background(), "plan-123", 0, false)
	if err == nil {
		t.Error("Expected error from app factory")
	}
	if !strings.Contains(err.Error(), "failed to create app") {
		t.Errorf("Expected 'failed to create app' error, got: %v", err)
	}
}

func TestRunResume_Success(t *testing.T) {
	// Save original and restore after test
	originalFactory := appFactory
	defer func() { appFactory = originalFactory }()

	// Track if Resume was called with correct arguments
	var capturedPlanID string
	var capturedMaxIterations int

	mockApp := &mockAppImpl{
		resumeFunc: func(ctx context.Context, planID string) error {
			capturedPlanID = planID
			return nil
		},
	}

	appFactory = func(cfg app.Config) (App, error) {
		capturedMaxIterations = cfg.MaxIterationsOverride
		return mockApp, nil
	}

	err := runResume(context.Background(), "plan-xyz", 42, false)
	if err != nil {
		t.Errorf("Unexpected error: %v", err)
	}

	if capturedPlanID != "plan-xyz" {
		t.Errorf("Expected plan ID 'plan-xyz', got %q", capturedPlanID)
	}
	if capturedMaxIterations != 42 {
		t.Errorf("Expected maxIterations 42, got %d", capturedMaxIterations)
	}
}

func TestRunResume_PlanNotFound(t *testing.T) {
	// Save original and restore after test
	originalFactory := appFactory
	defer func() { appFactory = originalFactory }()

	mockApp := &mockAppImpl{
		resumeFunc: func(ctx context.Context, planID string) error {
			return db.ErrNotFound
		},
	}

	appFactory = func(cfg app.Config) (App, error) {
		return mockApp, nil
	}

	err := runResume(context.Background(), "nonexistent-plan", 0, false)
	if err == nil {
		t.Error("Expected error for plan not found")
	}
	if !strings.Contains(err.Error(), "plan not found") {
		t.Errorf("Expected 'plan not found' error, got: %v", err)
	}
}

func TestRunResume_OtherError(t *testing.T) {
	// Save original and restore after test
	originalFactory := appFactory
	defer func() { appFactory = originalFactory }()

	mockApp := &mockAppImpl{
		resumeFunc: func(ctx context.Context, planID string) error {
			return errors.New("database connection failed")
		},
	}

	appFactory = func(cfg app.Config) (App, error) {
		return mockApp, nil
	}

	err := runResume(context.Background(), "plan-123", 0, false)
	if err == nil {
		t.Error("Expected error from resume")
	}
	if !strings.Contains(err.Error(), "database connection failed") {
		t.Errorf("Expected 'database connection failed' error, got: %v", err)
	}
}

func TestCLI_ExtremeModeFlagShort(t *testing.T) {
	cmd, flags := createTestCommand()
	_, err := executeCommand(cmd, "plan.md", "-x")
	if err != nil {
		t.Errorf("Unexpected error: %v", err)
	}
	if !flags.extremeMode {
		t.Error("Expected extremeMode to be true with -x flag")
	}
}

func TestCLI_ExtremeModeFlagLong(t *testing.T) {
	cmd, flags := createTestCommand()
	_, err := executeCommand(cmd, "plan.md", "--extreme")
	if err != nil {
		t.Errorf("Unexpected error: %v", err)
	}
	if !flags.extremeMode {
		t.Error("Expected extremeMode to be true with --extreme flag")
	}
}

func TestCLI_ExtremeModeDefaultFalse(t *testing.T) {
	cmd, flags := createTestCommand()
	_, err := executeCommand(cmd, "plan.md")
	if err != nil {
		t.Errorf("Unexpected error: %v", err)
	}
	if flags.extremeMode {
		t.Error("Expected extremeMode to be false by default")
	}
}

func TestRunNew_ExtremeModePassedToApp(t *testing.T) {
	originalFactory := appFactory
	defer func() { appFactory = originalFactory }()

	var capturedExtremeMode bool
	mockApp := &mockAppImpl{
		runFunc: func(ctx context.Context, planPath string) error {
			return nil
		},
	}
	appFactory = func(cfg app.Config) (App, error) {
		capturedExtremeMode = cfg.ExtremeMode
		return mockApp, nil
	}

	tempDir := t.TempDir()
	planPath := filepath.Join(tempDir, "plan.md")
	os.WriteFile(planPath, []byte("# Test Plan"), 0644)

	err := runNew(context.Background(), planPath, 0, true)
	if err != nil {
		t.Errorf("Unexpected error: %v", err)
	}
	if !capturedExtremeMode {
		t.Error("Expected ExtremeMode=true to be passed to app.Config")
	}
}

// mockAppImpl is a mock implementation of the App interface for testing
type mockAppImpl struct {
	runFunc           func(ctx context.Context, planPath string) error
	runWithPromptFunc func(ctx context.Context, prompt string) error
	resumeFunc        func(ctx context.Context, planID string) error
}

func (m *mockAppImpl) Run(ctx context.Context, planPath string) error {
	if m.runFunc != nil {
		return m.runFunc(ctx, planPath)
	}
	return nil
}

func (m *mockAppImpl) RunWithPrompt(ctx context.Context, prompt string) error {
	if m.runWithPromptFunc != nil {
		return m.runWithPromptFunc(ctx, prompt)
	}
	return nil
}

func (m *mockAppImpl) Resume(ctx context.Context, planID string) error {
	if m.resumeFunc != nil {
		return m.resumeFunc(ctx, planID)
	}
	return nil
}
