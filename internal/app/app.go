// Package app provides the application orchestration for Ralph.
// It connects the main execution loop to the TUI, handling the full lifecycle.
package app

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sync"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/google/uuid"

	"github.com/gerunddev/ralph/internal/claude"
	"github.com/gerunddev/ralph/internal/config"
	"github.com/gerunddev/ralph/internal/db"
	"github.com/gerunddev/ralph/internal/distill"
	"github.com/gerunddev/ralph/internal/jj"
	"github.com/gerunddev/ralph/internal/log"
	"github.com/gerunddev/ralph/internal/loop"
	"github.com/gerunddev/ralph/internal/tui"
)

// App orchestrates the main execution loop and TUI.
type App struct {
	cfg     *config.Config
	db      *db.DB
	claude  *claude.Client
	distill *distill.Distiller
	jj      *jj.Client
	workDir string

	// plan is set after loading/creating
	plan *db.Plan

	// loop is set after initialization
	loop *loop.Loop

	// For testing: allow injecting mock dependencies
	claudeOverride  *claude.Client
	distillOverride *distill.Distiller
	jjOverride      *jj.Client
}

// Config holds configuration for creating a new App.
type Config struct {
	// WorkDir is the working directory for jj operations.
	// If empty, uses the current working directory.
	WorkDir string

	// MaxIterationsOverride overrides the max_iterations from config.
	// If 0, uses the value from config file.
	MaxIterationsOverride int
}

// New creates a new App.
func New(cfg Config) (*App, error) {
	// Load configuration
	appConfig, err := config.Load()
	if err != nil {
		return nil, fmt.Errorf("failed to load config: %w", err)
	}

	// Determine working directory
	workDir := cfg.WorkDir
	if workDir == "" {
		workDir, err = os.Getwd()
		if err != nil {
			return nil, fmt.Errorf("failed to get working directory: %w", err)
		}
	}

	// Apply max iterations override if specified
	if cfg.MaxIterationsOverride > 0 {
		appConfig.MaxIterations = cfg.MaxIterationsOverride
	}

	app := &App{
		cfg:     appConfig,
		workDir: workDir,
	}

	return app, nil
}

// Run starts execution with a new plan from the given file path.
func (a *App) Run(ctx context.Context, planPath string) error {
	// Initialize dependencies
	if err := a.initDependencies(); err != nil {
		return err
	}
	defer a.cleanup()

	// Create plan from file
	if err := a.createPlanFromFile(planPath); err != nil {
		return err
	}

	return a.runLoop(ctx)
}

// Resume continues execution of an existing plan.
func (a *App) Resume(ctx context.Context, planID string) error {
	// Initialize dependencies
	if err := a.initDependencies(); err != nil {
		return err
	}
	defer a.cleanup()

	// Load existing plan
	if err := a.loadPlan(planID); err != nil {
		return err
	}

	return a.runLoop(ctx)
}

// RunWithPrompt starts execution with a plan from an inline prompt string.
func (a *App) RunWithPrompt(ctx context.Context, prompt string) error {
	// Initialize dependencies
	if err := a.initDependencies(); err != nil {
		return err
	}
	defer a.cleanup()

	// Create plan from prompt string
	if err := a.createPlanFromPrompt(prompt); err != nil {
		return err
	}

	return a.runLoop(ctx)
}

// initDependencies initializes all required dependencies.
func (a *App) initDependencies() error {
	// Create database directory and initialize
	dbDir := a.cfg.GetProjectsDir()
	if err := os.MkdirAll(dbDir, 0755); err != nil {
		return fmt.Errorf("failed to create database directory: %w", err)
	}

	// Use centralized database
	dbPath := filepath.Join(dbDir, "ralph.db")
	database, err := db.New(dbPath)
	if err != nil {
		return fmt.Errorf("failed to open database: %w", err)
	}
	a.db = database

	// Create Claude client (use override if set, for testing)
	if a.claudeOverride != nil {
		a.claude = a.claudeOverride
	} else {
		a.claude = claude.NewClient(claude.ClientConfig{
			Model:    a.cfg.Claude.Model,
			MaxTurns: a.cfg.Claude.MaxTurns,
			Verbose:  a.cfg.Claude.Verbose,
		})
	}

	// Create distiller (use override if set, for testing)
	if a.distillOverride != nil {
		a.distill = a.distillOverride
	} else {
		a.distill = distill.NewDistillerWithDefaults()
	}

	// Create jj client (use override if set, for testing)
	if a.jjOverride != nil {
		a.jj = a.jjOverride
	} else {
		a.jj = jj.NewClient(a.workDir)
	}

	return nil
}

// cleanup releases resources.
func (a *App) cleanup() {
	if a.db != nil {
		if err := a.db.Close(); err != nil {
			log.Warn("failed to close database", "error", err)
		}
	}
}

// createPlanFromFile reads a plan file and creates it in the database.
func (a *App) createPlanFromFile(planPath string) error {
	content, err := os.ReadFile(planPath)
	if err != nil {
		return fmt.Errorf("failed to read plan file: %w", err)
	}

	absPath, err := filepath.Abs(planPath)
	if err != nil {
		absPath = planPath // Use as-is if we can't get absolute path
	}

	plan := &db.Plan{
		ID:         uuid.New().String(),
		OriginPath: absPath,
		Content:    string(content),
		Status:     db.PlanStatusPending,
	}

	if err := a.db.CreatePlan(plan); err != nil {
		return fmt.Errorf("failed to create plan: %w", err)
	}

	a.plan = plan
	return nil
}

// createPlanFromPrompt creates a plan from an inline prompt string.
func (a *App) createPlanFromPrompt(prompt string) error {
	plan := &db.Plan{
		ID:         uuid.New().String(),
		OriginPath: "", // No file origin for inline prompts
		Content:    prompt,
		Status:     db.PlanStatusPending,
	}

	if err := a.db.CreatePlan(plan); err != nil {
		return fmt.Errorf("failed to create plan: %w", err)
	}

	a.plan = plan
	return nil
}

// loadPlan loads an existing plan from the database.
func (a *App) loadPlan(planID string) error {
	plan, err := a.db.GetPlan(planID)
	if err != nil {
		return fmt.Errorf("failed to load plan: %w", err)
	}

	a.plan = plan
	return nil
}

// createLoop creates a new loop instance with the current plan and dependencies.
func (a *App) createLoop() {
	a.loop = loop.New(loop.Config{
		PlanID:        a.plan.ID,
		MaxIterations: a.cfg.MaxIterations,
		WorkDir:       a.workDir,
	}, loop.Deps{
		DB:        a.db,
		Claude:    a.claude,
		Distiller: a.distill,
		JJ:        a.jj,
	})
}

// runLoopHeadless runs the loop without TUI and collects the result.
// The events channel is drained in a background goroutine that exits
// when the loop completes (the loop closes the events channel on completion).
func (a *App) runLoopHeadless(ctx context.Context) *Result {
	a.createLoop()

	// Drain events in background to prevent blocking.
	// This goroutine exits when loop.Run() completes because
	// Loop.Run() closes the events channel via defer close(l.events).
	go func() {
		for range a.loop.Events() {
			// Discard events
		}
	}()

	// Run loop
	loopErr := a.loop.Run(ctx)

	// Get final iteration count
	iterations := a.loop.CurrentIteration()

	// Check if completed successfully
	updatedPlan, _ := a.db.GetPlan(a.plan.ID)
	completed := updatedPlan != nil && updatedPlan.Status == db.PlanStatusCompleted

	return &Result{
		PlanID:     a.plan.ID,
		Completed:  completed,
		Iterations: iterations,
		Error:      loopErr,
	}
}

// runLoop creates and runs the loop with the TUI.
func (a *App) runLoop(ctx context.Context) error {
	// Create cancelable context for the loop
	loopCtx, cancelLoop := context.WithCancel(ctx)
	defer cancelLoop()

	// Create the loop
	a.createLoop()

	// Create TUI with event channel
	model := tui.NewModelWithEvents(a.loop.Events())

	// Set prompt content (truncated for display)
	promptPreview := a.plan.Content
	if len(promptPreview) > 2000 {
		promptPreview = promptPreview[:2000] + "\n\n... (truncated)"
	}
	model.SetPrompt(promptPreview)

	// Create the Bubble Tea program
	p := tea.NewProgram(model, tea.WithAltScreen())

	// Channel for loop completion
	loopDone := make(chan error, 1)
	var wg sync.WaitGroup

	// Start the loop in a goroutine
	wg.Add(1)
	go func() {
		defer wg.Done()
		err := a.loop.Run(loopCtx)
		loopDone <- err
	}()

	// Run the TUI (blocks until quit)
	_, tuiErr := p.Run()

	// Cancel the loop when TUI exits
	cancelLoop()

	// Wait for loop to finish
	wg.Wait()

	// Get loop error (guaranteed to be available after wg.Wait())
	loopErr := <-loopDone

	// Return TUI error first (user quit), then loop error
	if tuiErr != nil {
		return tuiErr
	}

	// Context.Canceled is expected when user quits
	if loopErr != nil && !errors.Is(loopErr, context.Canceled) {
		return loopErr
	}

	return nil
}

// SetClaudeClient allows injecting a mock Claude client for testing.
func (a *App) SetClaudeClient(client *claude.Client) {
	a.claudeOverride = client
}

// SetDistiller allows injecting a mock distiller for testing.
func (a *App) SetDistiller(d *distill.Distiller) {
	a.distillOverride = d
}

// SetJJClient allows injecting a mock jj client for testing.
func (a *App) SetJJClient(client *jj.Client) {
	a.jjOverride = client
}

// PlanID returns the current plan ID, or empty string if not set.
func (a *App) PlanID() string {
	if a.plan != nil {
		return a.plan.ID
	}
	return ""
}

// Result holds the result of a completed execution.
type Result struct {
	PlanID     string
	Completed  bool
	Iterations int
	Error      error
}

// RunHeadless runs the loop without TUI, useful for scripting.
// Returns the result when the loop completes.
func (a *App) RunHeadless(ctx context.Context, planPath string) (*Result, error) {
	// Initialize dependencies
	if err := a.initDependencies(); err != nil {
		return nil, err
	}
	defer a.cleanup()

	// Create plan from file
	if err := a.createPlanFromFile(planPath); err != nil {
		return nil, err
	}

	return a.runLoopHeadless(ctx), nil
}

// ResumeHeadless resumes an existing plan without TUI.
func (a *App) ResumeHeadless(ctx context.Context, planID string) (*Result, error) {
	// Initialize dependencies
	if err := a.initDependencies(); err != nil {
		return nil, err
	}
	defer a.cleanup()

	// Load existing plan
	if err := a.loadPlan(planID); err != nil {
		return nil, err
	}

	return a.runLoopHeadless(ctx), nil
}
