// Package main is the entry point for the Ralph TUI application.
package main

import (
	"fmt"
	"os"

	"github.com/gerund/ralph/internal/app"
	"github.com/spf13/cobra"
)

func main() {
	if err := run(); err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}
}

func run() error {
	var createPath string

	rootCmd := &cobra.Command{
		Use:   "ralph",
		Short: "Ralph automates Claude Code sessions using plan-based development",
		Long: `Ralph is a TUI application that automates Claude Code sessions using a
plan-based development workflow. It implements the "Ralph Loop" pattern: breaking
a plan into discrete tasks, then iterating each task through developer→reviewer
cycles until complete, with each iteration using a fresh Claude context window.`,
		RunE: func(cmd *cobra.Command, args []string) error {
			return app.Run(app.Options{
				CreateFromPlan: createPath,
			})
		},
	}

	rootCmd.Flags().StringVarP(&createPath, "create", "c", "",
		"Create a new project from the specified plan file")

	// Feedback subcommand
	var projectID, feedbackFile string
	feedbackCmd := &cobra.Command{
		Use:   "feedback",
		Short: "Submit feedback for a project",
		Long: `Submit user feedback for a project. The feedback will be processed
as a new task through the standard Ralph development cycle.`,
		RunE: func(cmd *cobra.Command, args []string) error {
			return app.SubmitFeedback(projectID, feedbackFile)
		},
	}
	feedbackCmd.Flags().StringVarP(&projectID, "project", "p", "", "Project ID (required)")
	feedbackCmd.Flags().StringVarP(&feedbackFile, "file", "f", "", "Path to feedback markdown file (required)")
	_ = feedbackCmd.MarkFlagRequired("project")
	_ = feedbackCmd.MarkFlagRequired("file")
	rootCmd.AddCommand(feedbackCmd)

	// Task subcommand group
	rootCmd.AddCommand(taskCmd())

	return rootCmd.Execute()
}
