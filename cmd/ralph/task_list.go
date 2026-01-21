package main

import (
	"fmt"

	"github.com/gerund/ralph/internal/config"
	"github.com/gerund/ralph/internal/db"
	"github.com/gerund/ralph/internal/log"
	"github.com/spf13/cobra"
)

func taskListCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "list <project-id>",
		Short: "List all tasks in a project",
		Long: `List all tasks in a project with their status and sequence numbers.

Example:
  ralph task list abc123`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			return runTaskList(args[0])
		},
	}
}

func runTaskList(projectID string) error {
	cfg, err := config.Load()
	if err != nil {
		return err
	}

	database, err := db.OpenProjectDB(cfg.GetProjectsDir(), projectID)
	if err != nil {
		return fmt.Errorf("failed to open project %s: %w", projectID, err)
	}
	defer func() {
		if closeErr := database.Close(); closeErr != nil {
			log.Warn("failed to close database", "error", closeErr)
		}
	}()

	tasks, err := database.GetTasksByProject(projectID)
	if err != nil {
		return err
	}

	if len(tasks) == 0 {
		fmt.Printf("No tasks in project %s\n", projectID)
		return nil
	}

	fmt.Printf("Tasks in project %s:\n\n", projectID)
	for _, task := range tasks {
		icon := statusIcon(task.Status)
		fmt.Printf("  %s %d. %s\n", icon, task.Sequence, task.Title)
	}

	return nil
}

func statusIcon(status db.TaskStatus) string {
	switch status {
	case db.TaskCompleted:
		return "[x]"
	case db.TaskInProgress:
		return "[~]"
	case db.TaskFailed:
		return "[!]"
	case db.TaskEscalated:
		return "[^]"
	default:
		return "[ ]"
	}
}
