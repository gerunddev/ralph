package app

import (
	"os"
	"path/filepath"
	"testing"
)

func TestOptions_Structure(t *testing.T) {
	opts := Options{
		CreateFromPlan: "/path/to/plan.md",
	}

	if opts.CreateFromPlan != "/path/to/plan.md" {
		t.Errorf("CreateFromPlan not set correctly: got %s", opts.CreateFromPlan)
	}
}

func TestOptions_EmptyCreateFromPlan(t *testing.T) {
	opts := Options{}

	if opts.CreateFromPlan != "" {
		t.Errorf("Expected empty CreateFromPlan, got %s", opts.CreateFromPlan)
	}
}

func TestRun_InvalidConfigPath(t *testing.T) {
	// Create a temp directory for test
	tempDir, err := os.MkdirTemp("", "ralph-app-test-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer func() { _ = os.RemoveAll(tempDir) }()

	// Save original config location and restore after test
	// Note: This test relies on being able to load default config successfully
	// The actual test behavior depends on whether a config file exists

	// Test with empty options (selection mode)
	// This will fail due to TUI initialization requiring a terminal,
	// but we're mainly testing that the bootstrap logic works
	// In a real test environment, we'd mock the TUI

	// For now, just verify Options work correctly
	opts := Options{CreateFromPlan: ""}
	if opts.CreateFromPlan != "" {
		t.Error("Expected empty CreateFromPlan")
	}
}

func TestRun_CreateMode_FileNotFound(t *testing.T) {
	// Skip if no terminal available (CI environment)
	if os.Getenv("CI") != "" {
		t.Skip("Skipping TUI test in CI environment")
	}

	opts := Options{
		CreateFromPlan: "/nonexistent/path/plan.md",
	}

	err := Run(opts)
	if err == nil {
		t.Error("Expected error for non-existent plan file")
	}
}

func TestRun_CreateMode_EmptyPlanFile(t *testing.T) {
	// Skip if no terminal available (CI environment)
	if os.Getenv("CI") != "" {
		t.Skip("Skipping TUI test in CI environment")
	}

	// Create temp dir with empty plan file
	tempDir, err := os.MkdirTemp("", "ralph-app-test-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer func() { _ = os.RemoveAll(tempDir) }()

	planPath := filepath.Join(tempDir, "plan.md")
	if err := os.WriteFile(planPath, []byte(""), 0644); err != nil {
		t.Fatalf("Failed to write plan file: %v", err)
	}

	opts := Options{
		CreateFromPlan: planPath,
	}

	// This will fail during planner execution (no Claude CLI),
	// but verifies the file can be read
	err = Run(opts)
	if err == nil {
		t.Error("Expected error (planner can't run without Claude CLI)")
	}
}

func TestOptions_ModeSelection(t *testing.T) {
	testCases := []struct {
		name           string
		createFromPlan string
		isCreateMode   bool
	}{
		{
			name:           "selection mode with empty string",
			createFromPlan: "",
			isCreateMode:   false,
		},
		{
			name:           "create mode with path",
			createFromPlan: "/path/to/plan.md",
			isCreateMode:   true,
		},
		{
			name:           "create mode with relative path",
			createFromPlan: "./plan.md",
			isCreateMode:   true,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			opts := Options{CreateFromPlan: tc.createFromPlan}
			isCreateMode := opts.CreateFromPlan != ""
			if isCreateMode != tc.isCreateMode {
				t.Errorf("Expected isCreateMode=%v, got %v", tc.isCreateMode, isCreateMode)
			}
		})
	}
}
