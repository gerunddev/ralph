package main

import (
	"fmt"
	"os"
	"strconv"
	"strings"

	"github.com/gerund/ralph/internal/config"
	"github.com/gerund/ralph/internal/db"
	"github.com/gerund/ralph/internal/log"
	"github.com/spf13/cobra"
)

// filePermissions is the default permission for exported files.
const filePermissions = 0644

func taskExportCmd() *cobra.Command {
	var outputFile string
	var includeMetadata bool

	cmd := &cobra.Command{
		Use:   "export <project-id> <task-sequence>",
		Short: "Export task description to a file",
		Long: `Export a task's description content to stdout or a file.

The exported content can be edited and re-imported using 'ralph task import'.

Examples:
  ralph task export abc123 3              # Export task 3 to stdout
  ralph task export abc123 3 -o task.md   # Export to file
  ralph task export abc123 3 | vim -      # Pipe to editor`,
		Args: cobra.ExactArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			projectID := args[0]
			sequence, err := strconv.Atoi(args[1])
			if err != nil {
				return fmt.Errorf("invalid task sequence: %s", args[1])
			}

			return runTaskExport(projectID, sequence, outputFile, includeMetadata)
		},
	}

	cmd.Flags().StringVarP(&outputFile, "output", "o", "", "Output file (default: stdout)")
	cmd.Flags().BoolVar(&includeMetadata, "metadata", true, "Include task metadata as header comments")

	return cmd
}

func runTaskExport(projectID string, sequence int, outputFile string, includeMetadata bool) error {
	cfg, err := config.Load()
	if err != nil {
		return err
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

	// Build content
	var content strings.Builder

	if includeMetadata {
		content.WriteString(fmt.Sprintf("<!-- Task: %s -->\n", task.Title))
		content.WriteString(fmt.Sprintf("<!-- Project: %s -->\n", projectID))
		content.WriteString(fmt.Sprintf("<!-- Sequence: %d -->\n", task.Sequence))
		content.WriteString(fmt.Sprintf("<!-- Status: %s -->\n", task.Status))
		content.WriteString(fmt.Sprintf("<!-- Edit below, then import with: ralph task import %s %d <file> -->\n\n", projectID, sequence))
	}

	content.WriteString(task.Description)

	// Output
	if outputFile == "" {
		fmt.Print(content.String())
		return nil
	}

	return os.WriteFile(outputFile, []byte(content.String()), filePermissions)
}
