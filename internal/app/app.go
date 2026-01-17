// Package app provides the main application orchestration for Ralph.
package app

import (
	"context"
	"errors"
	"fmt"
	"os"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/google/uuid"

	"github.com/gerund/ralph/internal/config"
	"github.com/gerund/ralph/internal/db"
	"github.com/gerund/ralph/internal/engine"
	"github.com/gerund/ralph/internal/log"
	"github.com/gerund/ralph/internal/tui"
)

// Options configures how Ralph starts.
type Options struct {
	// CreateFromPlan is the path to a plan file. If set, creates a new project
	// from the plan and starts execution. If empty, shows project selection.
	CreateFromPlan string
}

// Run starts the Ralph application with the given options.
func Run(opts Options) error {
	// 1. Load configuration
	cfg, err := config.Load()
	if err != nil {
		return fmt.Errorf("failed to load config: %w", err)
	}

	// 2. Ensure projects directory exists
	projectsDir := cfg.GetProjectsDir()
	if err := os.MkdirAll(projectsDir, 0755); err != nil {
		return fmt.Errorf("failed to create projects directory: %w", err)
	}

	// 3. Get working directory
	workDir, err := os.Getwd()
	if err != nil {
		return fmt.Errorf("failed to get working directory: %w", err)
	}

	// 4. Handle create mode vs selection mode
	if opts.CreateFromPlan != "" {
		return runCreateMode(cfg, workDir, opts.CreateFromPlan)
	}

	return runSelectionMode(cfg, workDir)
}

// runCreateMode creates a project from a plan file and starts execution.
func runCreateMode(cfg *config.Config, workDir, planPath string) error {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Generate a new project ID
	projectID := db.GenerateProjectID()

	// Create project-specific database
	database, err := db.OpenProjectDB(cfg.GetProjectsDir(), projectID)
	if err != nil {
		return fmt.Errorf("failed to create project database: %w", err)
	}
	defer func() {
		if closeErr := database.Close(); closeErr != nil {
			log.Warn("failed to close database", "error", closeErr)
		}
	}()

	// Create engine with the project-specific database
	eng, err := engine.NewEngine(engine.EngineConfig{
		Config:  cfg,
		DB:      database,
		WorkDir: workDir,
	})
	if err != nil {
		return fmt.Errorf("failed to create engine: %w", err)
	}
	defer func() {
		if stopErr := eng.Stop(); stopErr != nil {
			log.Warn("failed to stop engine", "error", stopErr)
		}
	}()

	// Create project from plan file
	project, err := eng.CreateProject(ctx, planPath)
	if err != nil {
		return fmt.Errorf("failed to create project: %w", err)
	}

	// Create task progress model for the new project
	model := tui.NewTaskProgressModel(project, database, eng.Events())
	p := tea.NewProgram(
		tui.NewCreateModeModel(model, project, database, eng),
		tea.WithAltScreen(),
	)

	// Run engine in background
	errCh := make(chan error, 1)
	go func() {
		errCh <- eng.Run(ctx)
	}()

	// Run TUI (blocks until quit)
	if _, err := p.Run(); err != nil {
		cancel() // Stop engine
		return err
	}

	// Check for engine error
	select {
	case err := <-errCh:
		return err
	default:
		return nil
	}
}

// runSelectionMode starts the TUI with project selection.
func runSelectionMode(cfg *config.Config, workDir string) error {
	p := tea.NewProgram(
		tui.New(cfg, workDir),
		tea.WithAltScreen(),
	)
	_, err := p.Run()
	return err
}

// SubmitFeedback submits user feedback for a project via CLI.
// The feedback content is read from the specified file and creates a new task.
func SubmitFeedback(projectID, feedbackFile string) error {
	// 1. Load config and open project-specific database
	cfg, err := config.Load()
	if err != nil {
		return fmt.Errorf("failed to load config: %w", err)
	}

	database, err := db.OpenProjectDB(cfg.GetProjectsDir(), projectID)
	if err != nil {
		return fmt.Errorf("failed to open project database: %w", err)
	}
	defer func() {
		if closeErr := database.Close(); closeErr != nil {
			log.Warn("failed to close database", "error", closeErr)
		}
	}()

	// 2. Verify project exists and state is PENDING
	project, err := database.GetProject(projectID)
	if err != nil {
		if errors.Is(err, db.ErrNotFound) {
			return fmt.Errorf("project not found: %s", projectID)
		}
		return fmt.Errorf("failed to get project: %w", err)
	}

	if project.UserFeedbackState != db.FeedbackStatePending {
		return fmt.Errorf("project is not in pending feedback state (current state: %q)", project.UserFeedbackState)
	}

	// 3. Read feedback file content
	content, err := os.ReadFile(feedbackFile)
	if err != nil {
		return fmt.Errorf("failed to read feedback file: %w", err)
	}

	feedbackContent := strings.TrimSpace(string(content))
	if feedbackContent == "" {
		return fmt.Errorf("feedback file is empty")
	}

	// 4. Get max task sequence, create task with sequence = max + 1
	maxSeq, err := database.GetMaxTaskSequence(projectID)
	if err != nil {
		return fmt.Errorf("failed to get max task sequence: %w", err)
	}

	task := &db.Task{
		ID:          uuid.New().String(),
		ProjectID:   projectID,
		Sequence:    maxSeq + 1,
		Title:       "User Feedback",
		Description: feedbackContent,
		Status:      db.TaskPending,
	}

	if err := database.CreateTask(task); err != nil {
		return fmt.Errorf("failed to create feedback task: %w", err)
	}

	// 5. Update project feedback state to PROVIDED
	if err := database.UpdateProjectFeedbackState(projectID, db.FeedbackStateProvided); err != nil {
		return fmt.Errorf("failed to update feedback state: %w", err)
	}

	// 6. Print success message with TUI restart instructions
	fmt.Println("Feedback submitted successfully!")
	fmt.Println()
	fmt.Printf("Task created: %s (sequence %d)\n", task.Title, task.Sequence)
	fmt.Println()
	fmt.Println("To process the feedback, restart the TUI:")
	fmt.Println("  ralph")
	fmt.Println()
	fmt.Println("Then select the project to continue.")

	return nil
}
