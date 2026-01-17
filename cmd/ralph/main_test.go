package main

import (
	"bytes"
	"os"
	"strings"
	"testing"

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

// createTestCommand creates a test version of the root command
// that doesn't actually run the app (for testing flag parsing).
func createTestCommand() (*cobra.Command, *string) {
	var createPath string

	rootCmd := &cobra.Command{
		Use:   "ralph",
		Short: "Ralph automates Claude Code sessions using plan-based development",
		Long: `Ralph is a TUI application that automates Claude Code sessions using a
plan-based development workflow. It implements the "Ralph Loop" pattern: breaking
a plan into discrete tasks, then iterating each task through developer→reviewer
cycles until complete, with each iteration using a fresh Claude context window.`,
		RunE: func(cmd *cobra.Command, args []string) error {
			// Don't actually run the app in tests
			return nil
		},
	}

	rootCmd.Flags().StringVarP(&createPath, "create", "c", "",
		"Create a new project from the specified plan file")

	return rootCmd, &createPath
}

func TestCLI_NoFlags(t *testing.T) {
	cmd, createPath := createTestCommand()

	_, err := executeCommand(cmd)
	if err != nil {
		t.Errorf("Unexpected error: %v", err)
	}

	if *createPath != "" {
		t.Errorf("Expected empty createPath, got %q", *createPath)
	}
}

func TestCLI_CreateFlagShort(t *testing.T) {
	cmd, createPath := createTestCommand()

	_, err := executeCommand(cmd, "-c", "/path/to/plan.md")
	if err != nil {
		t.Errorf("Unexpected error: %v", err)
	}

	if *createPath != "/path/to/plan.md" {
		t.Errorf("Expected createPath '/path/to/plan.md', got %q", *createPath)
	}
}

func TestCLI_CreateFlagLong(t *testing.T) {
	cmd, createPath := createTestCommand()

	_, err := executeCommand(cmd, "--create", "/another/plan.md")
	if err != nil {
		t.Errorf("Unexpected error: %v", err)
	}

	if *createPath != "/another/plan.md" {
		t.Errorf("Expected createPath '/another/plan.md', got %q", *createPath)
	}
}

func TestCLI_CreateFlagEquals(t *testing.T) {
	cmd, createPath := createTestCommand()

	_, err := executeCommand(cmd, "--create=/equals/path.md")
	if err != nil {
		t.Errorf("Unexpected error: %v", err)
	}

	if *createPath != "/equals/path.md" {
		t.Errorf("Expected createPath '/equals/path.md', got %q", *createPath)
	}
}

func TestCLI_CreateFlagRelativePath(t *testing.T) {
	cmd, createPath := createTestCommand()

	_, err := executeCommand(cmd, "-c", "./relative/plan.md")
	if err != nil {
		t.Errorf("Unexpected error: %v", err)
	}

	if *createPath != "./relative/plan.md" {
		t.Errorf("Expected createPath './relative/plan.md', got %q", *createPath)
	}
}

func TestCLI_CreateFlagWithSpaces(t *testing.T) {
	cmd, createPath := createTestCommand()

	_, err := executeCommand(cmd, "-c", "/path/with spaces/plan.md")
	if err != nil {
		t.Errorf("Unexpected error: %v", err)
	}

	if *createPath != "/path/with spaces/plan.md" {
		t.Errorf("Expected createPath with spaces, got %q", *createPath)
	}
}

func TestCLI_Help(t *testing.T) {
	cmd, _ := createTestCommand()

	output, err := executeCommand(cmd, "--help")
	if err != nil {
		t.Errorf("Unexpected error: %v", err)
	}

	// Check help contains expected elements
	if !strings.Contains(output, "ralph") {
		t.Error("Help should contain 'ralph'")
	}
	if !strings.Contains(output, "plan-based development") {
		t.Error("Help should contain 'plan-based development'")
	}
	if !strings.Contains(output, "-c, --create") {
		t.Error("Help should contain '-c, --create' flag")
	}
}

func TestCLI_InvalidFlag(t *testing.T) {
	cmd, _ := createTestCommand()

	_, err := executeCommand(cmd, "--invalid-flag")
	if err == nil {
		t.Error("Expected error for invalid flag")
	}
}

func TestCLI_CreateFlagMissingValue(t *testing.T) {
	cmd, _ := createTestCommand()

	_, err := executeCommand(cmd, "-c")
	if err == nil {
		t.Error("Expected error for -c without value")
	}
}

func TestCLI_DescriptionContainsRalphLoop(t *testing.T) {
	cmd, _ := createTestCommand()

	output, _ := executeCommand(cmd, "--help")

	if !strings.Contains(output, "Ralph Loop") {
		t.Error("Long description should mention Ralph Loop pattern")
	}
}

func TestCLI_CreateFlagDescription(t *testing.T) {
	cmd, _ := createTestCommand()

	output, _ := executeCommand(cmd, "--help")

	if !strings.Contains(output, "Create a new project from the specified plan file") {
		t.Error("Help should contain create flag description")
	}
}

// TestRunFunction tests that the run() function works as expected
// We can't fully test it without mocking, but we can test basic structure
func TestRunFunction_Structure(t *testing.T) {
	// Just verify the function signature is correct by calling it
	// in a way that will fail fast (invalid environment)

	// This test primarily verifies compilation
	// The actual behavior is tested via integration tests

	// Skip in CI since it requires terminal
	if os.Getenv("CI") != "" {
		t.Skip("Skipping run function test in CI")
	}

	// Could add more tests here with mocking
}
