package main

import (
	"fmt"
	"io"
	"os"
	"strconv"
	"strings"

	"github.com/gerunddev/ralph/internal/config"
	"github.com/gerunddev/ralph/internal/db"
	"github.com/gerunddev/ralph/internal/log"
	"github.com/spf13/cobra"
)

func taskImportCmd() *cobra.Command {
	var force bool
	var stripMetadata bool

	cmd := &cobra.Command{
		Use:   "import <project-id> <task-sequence> <file>",
		Short: "Import task description from a file",
		Long: `Import task description content from a file or stdin.

The task must exist and not be completed. Metadata comment lines are
automatically stripped if present.

Examples:
  ralph task import abc123 3 task.md          # Import from file
  ralph task import abc123 3 -                # Import from stdin
  cat task.md | ralph task import abc123 3 -  # Pipe from another command`,
		Args: cobra.ExactArgs(3),
		RunE: func(cmd *cobra.Command, args []string) error {
			projectID := args[0]
			sequence, err := strconv.Atoi(args[1])
			if err != nil {
				return fmt.Errorf("invalid task sequence: %s", args[1])
			}
			inputFile := args[2]

			return runTaskImport(projectID, sequence, inputFile, force, stripMetadata)
		},
	}

	cmd.Flags().BoolVarP(&force, "force", "f", false, "Skip confirmation prompt")
	cmd.Flags().BoolVar(&stripMetadata, "strip-metadata", true, "Strip metadata comment lines")

	return cmd
}

func runTaskImport(projectID string, sequence int, inputFile string, force, stripMetadata bool) error {
	cfg, err := config.Load()
	if err != nil {
		return err
	}

	// Read input
	var content []byte
	if inputFile == "-" {
		content, err = io.ReadAll(os.Stdin)
	} else {
		content, err = os.ReadFile(inputFile)
	}
	if err != nil {
		return fmt.Errorf("failed to read input: %w", err)
	}

	// Strip metadata comments if present
	description := string(content)
	if stripMetadata {
		description = stripMetadataComments(description)
	}

	// Open project database
	database, err := db.OpenProjectDB(cfg.GetProjectsDir(), projectID)
	if err != nil {
		return fmt.Errorf("failed to open project %s: %w", projectID, err)
	}
	defer func() {
		if closeErr := database.Close(); closeErr != nil {
			log.Warn("failed to close database", "error", closeErr)
		}
	}()

	// Get task
	task, err := database.GetTaskBySequence(projectID, sequence)
	if err != nil {
		return fmt.Errorf("task %d not found in project %s: %w", sequence, projectID, err)
	}

	// Validate task can be modified
	if task.Status == db.TaskCompleted {
		return fmt.Errorf("cannot modify completed task %d", sequence)
	}

	// Confirmation
	if !force {
		fmt.Printf("Update task %d (%s) in project %s?\n", sequence, task.Title, projectID)
		fmt.Printf("Current description length: %d chars\n", len(task.Description))
		fmt.Printf("New description length: %d chars\n", len(description))
		fmt.Print("Proceed? [y/N]: ")

		var response string
		if _, err := fmt.Scanln(&response); err != nil || (response != "y" && response != "Y") {
			fmt.Println("Import cancelled.")
			return nil
		}
	}

	// Update task
	if err := database.UpdateTaskDescription(task.ID, description); err != nil {
		return fmt.Errorf("failed to update task: %w", err)
	}

	fmt.Printf("Task %d description updated (%d chars)\n", sequence, len(description))
	return nil
}

// stripMetadataComments removes <!-- ... --> comment blocks from the start.
// Handles both single-line comments and multi-line comments.
func stripMetadataComments(content string) string {
	result := content

	for {
		result = strings.TrimLeft(result, " \t\n\r")
		if !strings.HasPrefix(result, "<!--") {
			break
		}

		// Find closing -->
		endIdx := strings.Index(result, "-->")
		if endIdx == -1 {
			// Unclosed comment, stop stripping
			break
		}

		// Remove this comment block
		result = result[endIdx+3:]
	}

	// Trim leading whitespace after all comments removed
	return strings.TrimLeft(result, " \t\n\r")
}
