package main

import (
	"strings"
	"testing"
)

func TestTaskCmd_SubcommandGroup(t *testing.T) {
	cmd := taskCmd()

	if cmd.Use != "task" {
		t.Errorf("taskCmd().Use = %q, want %q", cmd.Use, "task")
	}

	// Verify subcommands exist
	subcommands := cmd.Commands()
	if len(subcommands) != 3 {
		t.Errorf("taskCmd() has %d subcommands, want 3", len(subcommands))
	}

	subNames := make(map[string]bool)
	for _, sub := range subcommands {
		subNames[sub.Use] = true
	}

	expected := []string{"list <project-id>", "export <project-id> <task-sequence>", "import <project-id> <task-sequence> <file>"}
	for _, e := range expected {
		if !subNames[e] {
			t.Errorf("taskCmd() missing subcommand %q", e)
		}
	}
}

func TestTaskExportCmd_Flags(t *testing.T) {
	cmd := taskExportCmd()

	// Check flags exist
	outputFlag := cmd.Flags().Lookup("output")
	if outputFlag == nil {
		t.Fatal("export command missing 'output' flag")
	}
	if outputFlag.Shorthand != "o" {
		t.Errorf("output flag shorthand = %q, want %q", outputFlag.Shorthand, "o")
	}

	metadataFlag := cmd.Flags().Lookup("metadata")
	if metadataFlag == nil {
		t.Fatal("export command missing 'metadata' flag")
	}
	if metadataFlag.DefValue != "true" {
		t.Errorf("metadata flag default = %q, want %q", metadataFlag.DefValue, "true")
	}
}

func TestTaskImportCmd_Flags(t *testing.T) {
	cmd := taskImportCmd()

	// Check flags exist
	forceFlag := cmd.Flags().Lookup("force")
	if forceFlag == nil {
		t.Fatal("import command missing 'force' flag")
	}
	if forceFlag.Shorthand != "f" {
		t.Errorf("force flag shorthand = %q, want %q", forceFlag.Shorthand, "f")
	}

	stripFlag := cmd.Flags().Lookup("strip-metadata")
	if stripFlag == nil {
		t.Fatal("import command missing 'strip-metadata' flag")
	}
	if stripFlag.DefValue != "true" {
		t.Errorf("strip-metadata flag default = %q, want %q", stripFlag.DefValue, "true")
	}
}

func TestTaskListCmd_Args(t *testing.T) {
	cmd := taskListCmd()

	if cmd.Use != "list <project-id>" {
		t.Errorf("taskListCmd().Use = %q, want %q", cmd.Use, "list <project-id>")
	}
}

func TestStripMetadataComments_SingleLine(t *testing.T) {
	input := `<!-- Task: Test Task -->
<!-- Project: proj-1 -->
<!-- Sequence: 1 -->

This is the actual content.`

	got := stripMetadataComments(input)
	want := "This is the actual content."

	if got != want {
		t.Errorf("stripMetadataComments() = %q, want %q", got, want)
	}
}

func TestStripMetadataComments_MultiLine(t *testing.T) {
	input := `<!--
This is a multi-line
comment block
-->

Actual content here.`

	got := stripMetadataComments(input)
	want := "Actual content here."

	if got != want {
		t.Errorf("stripMetadataComments() = %q, want %q", got, want)
	}
}

func TestStripMetadataComments_MixedComments(t *testing.T) {
	input := `<!-- Single line -->
<!--
Multi-line
comment
-->
<!-- Another single -->

Content starts here.`

	got := stripMetadataComments(input)
	want := "Content starts here."

	if got != want {
		t.Errorf("stripMetadataComments() = %q, want %q", got, want)
	}
}

func TestStripMetadataComments_NoComments(t *testing.T) {
	input := "Just plain content without any comments."

	got := stripMetadataComments(input)
	want := input

	if got != want {
		t.Errorf("stripMetadataComments() = %q, want %q", got, want)
	}
}

func TestStripMetadataComments_EmptyInput(t *testing.T) {
	got := stripMetadataComments("")
	if got != "" {
		t.Errorf("stripMetadataComments(\"\") = %q, want empty string", got)
	}
}

func TestStripMetadataComments_OnlyComments(t *testing.T) {
	input := `<!-- Comment 1 -->
<!-- Comment 2 -->`

	got := stripMetadataComments(input)
	if got != "" {
		t.Errorf("stripMetadataComments(only comments) = %q, want empty string", got)
	}
}

func TestStripMetadataComments_CommentInMiddle(t *testing.T) {
	// Comments in the middle of content should NOT be stripped
	input := `Content before.
<!-- This comment is in the middle -->
Content after.`

	got := stripMetadataComments(input)

	// The leading comment should be preserved since content comes first
	if !strings.HasPrefix(got, "Content before.") {
		t.Errorf("stripMetadataComments() should preserve content before middle comments, got %q", got)
	}
	if !strings.Contains(got, "<!-- This comment is in the middle -->") {
		t.Errorf("stripMetadataComments() should preserve middle comments, got %q", got)
	}
}

func TestStripMetadataComments_UnclosedComment(t *testing.T) {
	input := `<!-- Unclosed comment
This should be preserved since comment is unclosed`

	got := stripMetadataComments(input)
	// Unclosed comment should stop stripping and preserve content
	if !strings.Contains(got, "<!--") {
		t.Errorf("stripMetadataComments() should preserve unclosed comment, got %q", got)
	}
}

func TestStripMetadataComments_LeadingWhitespace(t *testing.T) {
	input := `

   <!-- Comment with leading whitespace -->

Actual content.`

	got := stripMetadataComments(input)
	want := "Actual content."

	if got != want {
		t.Errorf("stripMetadataComments() = %q, want %q", got, want)
	}
}

func TestStatusIcon(t *testing.T) {
	tests := []struct {
		status string
		want   string
	}{
		{"completed", "[x]"},
		{"in_progress", "[~]"},
		{"failed", "[!]"},
		{"escalated", "[^]"},
		{"pending", "[ ]"},
		{"unknown", "[ ]"},
	}

	for _, tt := range tests {
		t.Run(tt.status, func(t *testing.T) {
			// Import the db package status types indirectly through the function
			var got string
			switch tt.status {
			case "completed":
				got = statusIcon("completed")
			case "in_progress":
				got = statusIcon("in_progress")
			case "failed":
				got = statusIcon("failed")
			case "escalated":
				got = statusIcon("escalated")
			default:
				got = statusIcon("pending")
			}

			if got != tt.want {
				t.Errorf("statusIcon(%q) = %q, want %q", tt.status, got, tt.want)
			}
		})
	}
}
