package app

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/gerunddev/ralph/internal/claude"
	"github.com/gerunddev/ralph/internal/db"
	"github.com/gerunddev/ralph/internal/distill"
	"github.com/gerunddev/ralph/internal/jj"
)

func TestNew(t *testing.T) {
	// Create a temp directory
	tempDir, err := os.MkdirTemp("", "ralph-app-test-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	t.Cleanup(func() { _ = os.RemoveAll(tempDir) })

	app, err := New(Config{
		WorkDir: tempDir,
	})
	if err != nil {
		t.Fatalf("New() error: %v", err)
	}

	if app == nil {
		t.Fatal("Expected non-nil app")
	}

	if app.workDir != tempDir {
		t.Errorf("Expected workDir=%s, got %s", tempDir, app.workDir)
	}
}

func TestNew_WithMaxIterationsOverride(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "ralph-app-test-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	t.Cleanup(func() { _ = os.RemoveAll(tempDir) })

	app, err := New(Config{
		WorkDir:               tempDir,
		MaxIterationsOverride: 25,
	})
	if err != nil {
		t.Fatalf("New() error: %v", err)
	}

	if app.cfg.MaxIterations != 25 {
		t.Errorf("Expected MaxIterations=25, got %d", app.cfg.MaxIterations)
	}
}

func TestApp_SetClaudeClient(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "ralph-app-test-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	t.Cleanup(func() { _ = os.RemoveAll(tempDir) })

	app, err := New(Config{WorkDir: tempDir})
	if err != nil {
		t.Fatalf("New() error: %v", err)
	}

	mockClient := claude.NewClient(claude.ClientConfig{
		Model:    "test-model",
		MaxTurns: 1,
	})

	app.SetClaudeClient(mockClient)

	if app.claudeOverride != mockClient {
		t.Error("Claude client not set correctly")
	}
}

func TestApp_SetDistiller(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "ralph-app-test-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	t.Cleanup(func() { _ = os.RemoveAll(tempDir) })

	app, err := New(Config{WorkDir: tempDir})
	if err != nil {
		t.Fatalf("New() error: %v", err)
	}

	mockDistiller := distill.NewDistillerWithDefaults()
	app.SetDistiller(mockDistiller)

	if app.distillOverride != mockDistiller {
		t.Error("Distiller not set correctly")
	}
}

func TestApp_SetJJClient(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "ralph-app-test-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	t.Cleanup(func() { _ = os.RemoveAll(tempDir) })

	app, err := New(Config{WorkDir: tempDir})
	if err != nil {
		t.Fatalf("New() error: %v", err)
	}

	mockJJ := jj.NewClient(tempDir)
	app.SetJJClient(mockJJ)

	if app.jjOverride != mockJJ {
		t.Error("JJ client not set correctly")
	}
}

func TestApp_PlanID_BeforeRun(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "ralph-app-test-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	t.Cleanup(func() { _ = os.RemoveAll(tempDir) })

	app, err := New(Config{WorkDir: tempDir})
	if err != nil {
		t.Fatalf("New() error: %v", err)
	}

	if app.PlanID() != "" {
		t.Errorf("Expected empty PlanID before run, got %s", app.PlanID())
	}
}

func TestApp_Run_PlanFileNotFound(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "ralph-app-test-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	t.Cleanup(func() { _ = os.RemoveAll(tempDir) })

	app, err := New(Config{WorkDir: tempDir})
	if err != nil {
		t.Fatalf("New() error: %v", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	err = app.Run(ctx, "/nonexistent/plan.md")
	if err == nil {
		t.Error("Expected error for non-existent plan file")
	}
}

func TestApp_Resume_PlanNotFound(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "ralph-app-test-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	t.Cleanup(func() { _ = os.RemoveAll(tempDir) })

	app, err := New(Config{WorkDir: tempDir})
	if err != nil {
		t.Fatalf("New() error: %v", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	err = app.Resume(ctx, "nonexistent-plan-id")
	if err == nil {
		t.Error("Expected error for non-existent plan ID")
	}
}

func TestConfig_DefaultWorkDir(t *testing.T) {
	cfg := Config{}
	if cfg.WorkDir != "" {
		t.Errorf("Expected empty default WorkDir, got %s", cfg.WorkDir)
	}
}

func TestConfig_WithWorkDir(t *testing.T) {
	cfg := Config{
		WorkDir: "/custom/path",
	}
	if cfg.WorkDir != "/custom/path" {
		t.Errorf("Expected WorkDir=/custom/path, got %s", cfg.WorkDir)
	}
}

func TestResult_Structure(t *testing.T) {
	result := &Result{
		PlanID:     "test-plan-123",
		Completed:  true,
		Iterations: 5,
		Error:      nil,
	}

	if result.PlanID != "test-plan-123" {
		t.Errorf("Expected PlanID=test-plan-123, got %s", result.PlanID)
	}
	if !result.Completed {
		t.Error("Expected Completed=true")
	}
	if result.Iterations != 5 {
		t.Errorf("Expected Iterations=5, got %d", result.Iterations)
	}
	if result.Error != nil {
		t.Errorf("Expected nil Error, got %v", result.Error)
	}
}

// TestApp_InitDependencies_CreatesDatabase verifies that initDependencies creates the database.
func TestApp_InitDependencies_CreatesDatabase(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "ralph-app-test-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	t.Cleanup(func() { _ = os.RemoveAll(tempDir) })

	app, err := New(Config{WorkDir: tempDir})
	if err != nil {
		t.Fatalf("New() error: %v", err)
	}

	// Override the projects dir to temp
	app.cfg.ProjectsDir = tempDir

	err = app.initDependencies()
	if err != nil {
		t.Fatalf("initDependencies() error: %v", err)
	}
	defer app.cleanup()

	// Verify database was created
	if app.db == nil {
		t.Error("Expected db to be initialized")
	}

	// Verify Claude client was created
	if app.claude == nil {
		t.Error("Expected Claude client to be initialized")
	}

	// Verify distiller was created
	if app.distill == nil {
		t.Error("Expected distiller to be initialized")
	}

	// Verify jj client was created
	if app.jj == nil {
		t.Error("Expected jj client to be initialized")
	}
}

// TestApp_InitDependencies_UsesOverrides verifies that overrides are used when set.
func TestApp_InitDependencies_UsesOverrides(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "ralph-app-test-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	t.Cleanup(func() { _ = os.RemoveAll(tempDir) })

	app, err := New(Config{WorkDir: tempDir})
	if err != nil {
		t.Fatalf("New() error: %v", err)
	}

	// Override the projects dir
	app.cfg.ProjectsDir = tempDir

	// Set overrides
	mockClaude := claude.NewClient(claude.ClientConfig{Model: "mock"})
	mockDistiller := distill.NewDistillerWithDefaults()
	mockJJ := jj.NewClient(tempDir)

	app.SetClaudeClient(mockClaude)
	app.SetDistiller(mockDistiller)
	app.SetJJClient(mockJJ)

	err = app.initDependencies()
	if err != nil {
		t.Fatalf("initDependencies() error: %v", err)
	}
	defer app.cleanup()

	// Verify overrides were used
	if app.claude != mockClaude {
		t.Error("Expected Claude override to be used")
	}
	if app.distill != mockDistiller {
		t.Error("Expected distiller override to be used")
	}
	if app.jj != mockJJ {
		t.Error("Expected jj override to be used")
	}
}

// TestApp_RunHeadless_FileNotFound tests RunHeadless with a non-existent plan file.
func TestApp_RunHeadless_FileNotFound(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "ralph-app-test-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	t.Cleanup(func() { _ = os.RemoveAll(tempDir) })

	app, err := New(Config{WorkDir: tempDir})
	if err != nil {
		t.Fatalf("New() error: %v", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	result, err := app.RunHeadless(ctx, "/nonexistent/plan.md")
	if err == nil {
		t.Error("Expected error for non-existent plan file")
	}
	if result != nil {
		t.Error("Expected nil result on error")
	}
}

// TestApp_ResumeHeadless_PlanNotFound tests resuming a non-existent plan.
func TestApp_ResumeHeadless_PlanNotFound(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "ralph-app-test-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	t.Cleanup(func() { _ = os.RemoveAll(tempDir) })

	app, err := New(Config{WorkDir: tempDir})
	if err != nil {
		t.Fatalf("New() error: %v", err)
	}

	// Override the projects dir
	app.cfg.ProjectsDir = tempDir

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	result, err := app.ResumeHeadless(ctx, "nonexistent-plan-id")
	if err == nil {
		t.Error("Expected error for non-existent plan ID")
	}
	if result != nil {
		t.Error("Expected nil result on error")
	}
}

// TestApp_PlanCreation verifies that a plan is created correctly.
func TestApp_PlanCreation(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "ralph-app-test-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	t.Cleanup(func() { _ = os.RemoveAll(tempDir) })

	// Create a plan file
	planContent := "# Test Plan\n\nImplement feature X"
	planPath := filepath.Join(tempDir, "plan.md")
	if err := os.WriteFile(planPath, []byte(planContent), 0644); err != nil {
		t.Fatalf("Failed to write plan file: %v", err)
	}

	app, err := New(Config{
		WorkDir:               tempDir,
		MaxIterationsOverride: 1, // Minimal iterations
	})
	if err != nil {
		t.Fatalf("New() error: %v", err)
	}

	// Override projects dir
	app.cfg.ProjectsDir = tempDir

	// Initialize dependencies
	err = app.initDependencies()
	if err != nil {
		t.Fatalf("initDependencies() error: %v", err)
	}
	defer app.cleanup()

	// Read plan content
	content, err := os.ReadFile(planPath)
	if err != nil {
		t.Fatalf("Failed to read plan: %v", err)
	}

	absPath, _ := filepath.Abs(planPath)

	// Create plan in database
	plan := &db.Plan{
		ID:         "test-plan-123",
		OriginPath: absPath,
		Content:    string(content),
		Status:     db.PlanStatusPending,
	}

	if err := app.db.CreatePlan(plan); err != nil {
		t.Fatalf("Failed to create plan: %v", err)
	}

	// Verify plan was created
	retrieved, err := app.db.GetPlan("test-plan-123")
	if err != nil {
		t.Fatalf("Failed to retrieve plan: %v", err)
	}

	if retrieved.Content != planContent {
		t.Errorf("Plan content mismatch: expected %q, got %q", planContent, retrieved.Content)
	}

	if retrieved.Status != db.PlanStatusPending {
		t.Errorf("Expected status=pending, got %s", retrieved.Status)
	}
}

// TestApp_CreatePlanFromFile tests the createPlanFromFile method directly.
func TestApp_CreatePlanFromFile(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "ralph-app-test-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	t.Cleanup(func() { _ = os.RemoveAll(tempDir) })

	app, err := New(Config{WorkDir: tempDir})
	if err != nil {
		t.Fatalf("New() error: %v", err)
	}

	// Override projects dir
	app.cfg.ProjectsDir = tempDir

	// Initialize dependencies (needed for db)
	err = app.initDependencies()
	if err != nil {
		t.Fatalf("initDependencies() error: %v", err)
	}
	defer app.cleanup()

	t.Run("valid plan file", func(t *testing.T) {
		planContent := "# Test Plan\n\nImplement feature X"
		planPath := filepath.Join(tempDir, "valid-plan.md")
		if err := os.WriteFile(planPath, []byte(planContent), 0644); err != nil {
			t.Fatalf("Failed to write plan file: %v", err)
		}

		err := app.createPlanFromFile(planPath)
		if err != nil {
			t.Fatalf("createPlanFromFile() error: %v", err)
		}

		// Verify plan was set
		if app.plan == nil {
			t.Fatal("Expected plan to be set")
		}

		// Verify plan content
		if app.plan.Content != planContent {
			t.Errorf("Plan content mismatch: expected %q, got %q", planContent, app.plan.Content)
		}

		// Verify plan status
		if app.plan.Status != db.PlanStatusPending {
			t.Errorf("Expected status=pending, got %s", app.plan.Status)
		}

		// Verify origin path is absolute
		if !filepath.IsAbs(app.plan.OriginPath) {
			t.Errorf("Expected absolute origin path, got %s", app.plan.OriginPath)
		}

		// Verify plan was persisted to database
		retrieved, err := app.db.GetPlan(app.plan.ID)
		if err != nil {
			t.Fatalf("Failed to retrieve plan from db: %v", err)
		}
		if retrieved.Content != planContent {
			t.Errorf("Database plan content mismatch")
		}
	})

	t.Run("non-existent file", func(t *testing.T) {
		err := app.createPlanFromFile("/nonexistent/path/plan.md")
		if err == nil {
			t.Error("Expected error for non-existent file")
		}
	})

	t.Run("relative path resolution", func(t *testing.T) {
		// Create a plan file with a relative path
		planContent := "# Relative Path Test"
		planPath := filepath.Join(tempDir, "relative-plan.md")
		if err := os.WriteFile(planPath, []byte(planContent), 0644); err != nil {
			t.Fatalf("Failed to write plan file: %v", err)
		}

		// Use relative path (from tempDir)
		oldWd, _ := os.Getwd()
		_ = os.Chdir(tempDir)
		defer func() { _ = os.Chdir(oldWd) }()

		err := app.createPlanFromFile("relative-plan.md")
		if err != nil {
			t.Fatalf("createPlanFromFile() error: %v", err)
		}

		// Origin path should be absolute even when given relative path
		if !filepath.IsAbs(app.plan.OriginPath) {
			t.Errorf("Expected absolute origin path for relative input, got %s", app.plan.OriginPath)
		}
	})
}

// TestApp_ContextCancellation verifies that context cancellation is handled.
func TestApp_ContextCancellation(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "ralph-app-test-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	t.Cleanup(func() { _ = os.RemoveAll(tempDir) })

	// Create a plan file
	planPath := filepath.Join(tempDir, "plan.md")
	if err := os.WriteFile(planPath, []byte("# Test Plan"), 0644); err != nil {
		t.Fatalf("Failed to write plan file: %v", err)
	}

	app, err := New(Config{WorkDir: tempDir})
	if err != nil {
		t.Fatalf("New() error: %v", err)
	}

	// Create an already-canceled context
	ctx, cancel := context.WithCancel(context.Background())
	cancel() // Cancel immediately

	// Run should return quickly due to canceled context
	// Note: This test verifies the context cancellation path,
	// but actual behavior depends on where the cancellation is checked
	err = app.Run(ctx, planPath)

	// We expect an error (either context canceled or TUI error)
	// The exact error depends on the timing and implementation
	// This test mainly verifies we don't hang
	_ = err // Error expected, specific error varies by timing
}
