package main

import "github.com/spf13/cobra"

// taskCmd creates the task subcommand group.
func taskCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "task",
		Short: "Task management commands",
		Long: `Task management commands for viewing, exporting, and importing task content.

These commands allow you to modify task plans on the fly during Ralph execution.`,
	}

	cmd.AddCommand(taskListCmd())
	cmd.AddCommand(taskExportCmd())
	cmd.AddCommand(taskImportCmd())

	return cmd
}
