// Package main is the entry point for the Ralph CLI application.
package main

import (
	"context"
	"errors"
	"fmt"
	"os"

	"github.com/gerunddev/ralph/internal/app"
	"github.com/gerunddev/ralph/internal/db"
	"github.com/gerunddev/ralph/internal/jj"
	"github.com/spf13/cobra"
)

// jjValidator is the function used to validate jj repository.
// It can be replaced in tests to mock jj validation.
var jjValidator = defaultJJValidator

// appFactory is the function used to create a new app.App.
// It can be replaced in tests to mock app creation.
var appFactory = defaultAppFactory

// defaultJJValidator is the production jj validation implementation.
func defaultJJValidator(ctx context.Context, workDir string) error {
	jjClient := jj.NewClient(workDir)
	_, err := jjClient.Status(ctx)
	return err
}

// defaultAppFactory is the production app factory implementation.
func defaultAppFactory(cfg app.Config) (App, error) {
	return app.New(cfg)
}

// App interface defines the methods needed from app.App for testing.
type App interface {
	Run(ctx context.Context, planPath string) error
	RunWithPrompt(ctx context.Context, prompt string) error
	Resume(ctx context.Context, planID string) error
}

func main() {
	if err := run(); err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}
}

func run() error {
	var resumeID string
	var maxIterations int
	var promptStr string
	var extremeMode bool
	var teamMode bool

	rootCmd := &cobra.Command{
		Use:   "ralph [plan-file]",
		Short: "Iterative AI development with a single agent loop",
		Long: `Ralph runs an AI agent iteratively against a plan file.
The agent works until it declares completion or hits max iterations.

Examples:
  ralph plan.md                    # Start new execution from plan file
  ralph plan.md --max-iterations 30  # Start with custom iteration limit
  ralph -r abc123                  # Resume existing plan by ID
  ralph --resume abc123            # Resume existing plan by ID
  ralph -p "Fix the login bug"     # Start execution with inline prompt`,
		Args: cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx := context.Background()

			// Validate max-iterations is not negative
			if maxIterations < 0 {
				return fmt.Errorf("--max-iterations cannot be negative")
			}

			// Validate working directory is a jj repository
			if err := validateJJRepository(ctx); err != nil {
				return err
			}

			// Determine mode
			if resumeID != "" {
				if len(args) > 0 || promptStr != "" {
					return fmt.Errorf("cannot specify both --resume and plan file or --prompt")
				}
				return runResume(ctx, resumeID, maxIterations, extremeMode, teamMode)
			}

			if promptStr != "" {
				if len(args) > 0 {
					return fmt.Errorf("cannot specify both plan file and --prompt")
				}
				return runNewWithPrompt(ctx, promptStr, maxIterations, extremeMode, teamMode)
			}

			if len(args) == 0 {
				return fmt.Errorf("plan file required (or use --resume or --prompt)")
			}

			return runNew(ctx, args[0], maxIterations, extremeMode, teamMode)
		},
	}

	rootCmd.Flags().StringVarP(&resumeID, "resume", "r", "",
		"Resume execution of an existing plan by ID")
	rootCmd.Flags().StringVarP(&promptStr, "prompt", "p", "",
		"Use inline prompt as the plan instead of a file")
	rootCmd.Flags().IntVar(&maxIterations, "max-iterations", 0,
		"Override max iterations from config")
	rootCmd.Flags().BoolVarP(&extremeMode, "extreme", "x", false,
		"Extreme mode: run +3 iterations after robots think they're done")
	rootCmd.Flags().BoolVarP(&teamMode, "team", "t", false,
		"Enable agent teams for parallel development")

	// Add subcommands
	rootCmd.AddCommand(taskCmd())

	return rootCmd.Execute()
}

// validateJJRepository checks that we're inside a jj repository.
func validateJJRepository(ctx context.Context) error {
	workDir, err := os.Getwd()
	if err != nil {
		return fmt.Errorf("failed to get working directory: %w", err)
	}

	err = jjValidator(ctx, workDir)
	if errors.Is(err, jj.ErrNotRepo) {
		return fmt.Errorf("not a jj repository (run from within a jj repo)")
	}
	if errors.Is(err, jj.ErrCommandNotFound) {
		return fmt.Errorf("jj command not found (install jujutsu: https://github.com/martinvonz/jj)")
	}
	if err != nil {
		return fmt.Errorf("failed to verify jj repository: %w", err)
	}
	return nil
}

// runNew starts execution with a new plan from the given file path.
func runNew(ctx context.Context, planPath string, maxIterations int, extremeMode, teamMode bool) error {
	// Validate plan file exists
	if _, err := os.Stat(planPath); os.IsNotExist(err) {
		return fmt.Errorf("plan file not found: %s", planPath)
	}

	// Create app
	app, err := appFactory(app.Config{
		MaxIterationsOverride: maxIterations,
		ExtremeMode:           extremeMode,
		TeamMode:              teamMode,
	})
	if err != nil {
		return err
	}

	// Run with new plan
	return app.Run(ctx, planPath)
}

// runNewWithPrompt starts execution with a plan from an inline prompt string.
func runNewWithPrompt(ctx context.Context, prompt string, maxIterations int, extremeMode, teamMode bool) error {
	// Create app
	app, err := appFactory(app.Config{
		MaxIterationsOverride: maxIterations,
		ExtremeMode:           extremeMode,
		TeamMode:              teamMode,
	})
	if err != nil {
		return err
	}

	// Run with inline prompt
	return app.RunWithPrompt(ctx, prompt)
}

// runResume continues execution of an existing plan.
func runResume(ctx context.Context, planID string, maxIterations int, extremeMode, teamMode bool) error {
	// Create app first to access database
	app, err := appFactory(app.Config{
		MaxIterationsOverride: maxIterations,
		ExtremeMode:           extremeMode,
		TeamMode:              teamMode,
	})
	if err != nil {
		return err
	}

	// Resume execution
	err = app.Resume(ctx, planID)
	if err != nil {
		// Check if the error is due to plan not found
		if errors.Is(err, db.ErrNotFound) {
			return fmt.Errorf("plan not found: %s", planID)
		}
		return err
	}

	return nil
}
