package parser

import (
	"strings"
	"testing"
)

// =============================================================================
// Done Detection Tests
// =============================================================================

func TestParse_DoneExact(t *testing.T) {
	result := Parse("DONE DONE DONE!!!")

	if !result.IsDone {
		t.Error("IsDone should be true for exact done marker")
	}
	// When just the done marker is present, it gets treated as malformed
	// and placed in Progress for backwards compatibility
	if result.Raw != "DONE DONE DONE!!!" {
		t.Errorf("Raw should preserve original, got %q", result.Raw)
	}
}

func TestParse_DoneWithWhitespace(t *testing.T) {
	tests := []string{
		"  DONE DONE DONE!!!  ",
		"\nDONE DONE DONE!!!\n",
		"\t DONE DONE DONE!!! \t",
		"\n\n  DONE DONE DONE!!!  \n\n",
	}

	for _, input := range tests {
		result := Parse(input)
		if !result.IsDone {
			t.Errorf("IsDone should be true for %q", input)
		}
	}
}

func TestParse_NotDone_PartialMarker(t *testing.T) {
	tests := []string{
		"DONE DONE DONE",     // Missing exclamation marks
		"DONE DONE DONE!",    // Wrong number of exclamation marks
		"DONE DONE DONE!!!!", // Extra exclamation mark
		"done done done!!!",  // Lowercase
	}

	for _, input := range tests {
		result := Parse(input)
		if result.IsDone {
			t.Errorf("IsDone should be false for %q", input)
		}
	}
}

func TestParse_DoneWithSurroundingContent(t *testing.T) {
	// Done marker should be detected even when surrounded by other content
	tests := []string{
		"DONE DONE DONE!!! extra",
		"prefix DONE DONE DONE!!!",
		"Some intro text.\n\nDONE DONE DONE!!!",
		"Based on my analysis, I'm confident.\n\nDONE DONE DONE!!!",
		"DONE DONE DONE!!!\n\nSome trailing notes.",
	}

	for _, input := range tests {
		result := Parse(input)
		if !result.IsDone {
			t.Errorf("IsDone should be true for %q", input)
		}
	}
}

// =============================================================================
// Section Extraction Tests
// =============================================================================

func TestParse_BothSections(t *testing.T) {
	input := `## Progress
Built the user authentication module.
Added tests for login flow.

## Learnings
The codebase uses JWT tokens stored in httpOnly cookies.
Found that the User model is in internal/models/user.go.`

	result := Parse(input)

	if result.IsDone {
		t.Error("IsDone should be false")
	}

	expectedProgress := "Built the user authentication module.\nAdded tests for login flow."
	if result.Progress != expectedProgress {
		t.Errorf("Progress = %q, want %q", result.Progress, expectedProgress)
	}

	expectedLearnings := "The codebase uses JWT tokens stored in httpOnly cookies.\nFound that the User model is in internal/models/user.go."
	if result.Learnings != expectedLearnings {
		t.Errorf("Learnings = %q, want %q", result.Learnings, expectedLearnings)
	}
}

func TestParse_ProgressOnly(t *testing.T) {
	input := `## Progress
Made some changes to the database layer.
Fixed the connection pooling issue.`

	result := Parse(input)

	if result.IsDone {
		t.Error("IsDone should be false")
	}

	expectedProgress := "Made some changes to the database layer.\nFixed the connection pooling issue."
	if result.Progress != expectedProgress {
		t.Errorf("Progress = %q, want %q", result.Progress, expectedProgress)
	}

	if result.Learnings != "" {
		t.Errorf("Learnings should be empty, got %q", result.Learnings)
	}
}

func TestParse_LearningsOnly(t *testing.T) {
	input := `## Learnings
The config system uses viper.
Tests are in *_test.go files next to the source.`

	result := Parse(input)

	if result.IsDone {
		t.Error("IsDone should be false")
	}

	if result.Progress != "" {
		t.Errorf("Progress should be empty, got %q", result.Progress)
	}

	expectedLearnings := "The config system uses viper.\nTests are in *_test.go files next to the source."
	if result.Learnings != expectedLearnings {
		t.Errorf("Learnings = %q, want %q", result.Learnings, expectedLearnings)
	}
}

func TestParse_CaseInsensitiveHeaders(t *testing.T) {
	tests := []struct {
		name  string
		input string
		wantP string
		wantL string
	}{
		{
			name:  "lowercase",
			input: "## progress\nSome progress\n\n## learnings\nSome learnings",
			wantP: "Some progress",
			wantL: "Some learnings",
		},
		{
			name:  "uppercase",
			input: "## PROGRESS\nSome progress\n\n## LEARNINGS\nSome learnings",
			wantP: "Some progress",
			wantL: "Some learnings",
		},
		{
			name:  "mixed case",
			input: "## PrOgReSs\nSome progress\n\n## LeArNiNgS\nSome learnings",
			wantP: "Some progress",
			wantL: "Some learnings",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := Parse(tt.input)
			if result.Progress != tt.wantP {
				t.Errorf("Progress = %q, want %q", result.Progress, tt.wantP)
			}
			if result.Learnings != tt.wantL {
				t.Errorf("Learnings = %q, want %q", result.Learnings, tt.wantL)
			}
		})
	}
}

func TestParse_SectionsWithOtherHeaders(t *testing.T) {
	input := `## Progress
Did some work.

## Other Section
This should be ignored.

## Learnings
Learned something.

## Another Section
Also ignored.`

	result := Parse(input)

	if result.Progress != "Did some work." {
		t.Errorf("Progress = %q, want %q", result.Progress, "Did some work.")
	}

	if result.Learnings != "Learned something." {
		t.Errorf("Learnings = %q, want %q", result.Learnings, "Learned something.")
	}
}

func TestParse_ReversedSections(t *testing.T) {
	// Learnings before Progress
	input := `## Learnings
Found a bug in the auth module.

## Progress
Fixed the login flow.`

	result := Parse(input)

	if result.Progress != "Fixed the login flow." {
		t.Errorf("Progress = %q, want %q", result.Progress, "Fixed the login flow.")
	}

	if result.Learnings != "Found a bug in the auth module." {
		t.Errorf("Learnings = %q, want %q", result.Learnings, "Found a bug in the auth module.")
	}
}

// =============================================================================
// Malformed Output Tests
// =============================================================================

func TestParse_NoHeaders(t *testing.T) {
	input := "I made some changes to the code."

	result := Parse(input)

	if result.IsDone {
		t.Error("IsDone should be false")
	}

	// Entire output should be treated as progress
	if result.Progress != input {
		t.Errorf("Progress = %q, want %q", result.Progress, input)
	}

	if result.Learnings != "" {
		t.Errorf("Learnings should be empty, got %q", result.Learnings)
	}
}

func TestParse_NoHeadersMultiline(t *testing.T) {
	input := `I worked on the feature.
It's coming along nicely.
Still need to add tests.`

	result := Parse(input)

	if result.Progress != input {
		t.Errorf("Progress = %q, want %q", result.Progress, input)
	}
}

func TestParse_EmptyOutput(t *testing.T) {
	result := Parse("")

	if result.IsDone {
		t.Error("IsDone should be false for empty output")
	}
	if result.Progress != "" {
		t.Errorf("Progress should be empty, got %q", result.Progress)
	}
	if result.Learnings != "" {
		t.Errorf("Learnings should be empty, got %q", result.Learnings)
	}
	if result.Raw != "" {
		t.Errorf("Raw should be empty, got %q", result.Raw)
	}
}

func TestParse_WhitespaceOnlyOutput(t *testing.T) {
	tests := []string{
		"   ",
		"\n\n\n",
		"\t\t",
		" \n \t ",
	}

	for _, input := range tests {
		result := Parse(input)
		if result.IsDone {
			t.Errorf("IsDone should be false for whitespace: %q", input)
		}
		if result.Progress != "" {
			t.Errorf("Progress should be empty for whitespace: %q, got %q", input, result.Progress)
		}
		if result.Raw != input {
			t.Errorf("Raw should preserve original whitespace")
		}
	}
}

func TestParse_EmptySections(t *testing.T) {
	input := `## Progress

## Learnings
`

	result := Parse(input)

	if result.Progress != "" {
		t.Errorf("Progress should be empty, got %q", result.Progress)
	}
	if result.Learnings != "" {
		t.Errorf("Learnings should be empty, got %q", result.Learnings)
	}
}

func TestParse_HeaderOnlyNoNewline(t *testing.T) {
	input := "## Progress"

	result := Parse(input)

	// Section found but no content
	if result.Progress != "" {
		t.Errorf("Progress should be empty, got %q", result.Progress)
	}
}

// =============================================================================
// Edge Cases
// =============================================================================

func TestParse_PrefixContentBeforeSections(t *testing.T) {
	input := `Some intro text here that should be ignored.

## Progress
Did the work.

## Learnings
Learned things.`

	result := Parse(input)

	// Progress should only contain content after the header
	if result.Progress != "Did the work." {
		t.Errorf("Progress = %q, want %q", result.Progress, "Did the work.")
	}
	if result.Learnings != "Learned things." {
		t.Errorf("Learnings = %q, want %q", result.Learnings, "Learned things.")
	}
}

func TestParse_MultilineContentInSections(t *testing.T) {
	input := `## Progress
Line 1 of progress.
Line 2 of progress.

A paragraph break.

More progress content.

## Learnings
Single line learning.`

	result := Parse(input)

	expectedProgress := `Line 1 of progress.
Line 2 of progress.

A paragraph break.

More progress content.`

	if result.Progress != expectedProgress {
		t.Errorf("Progress = %q, want %q", result.Progress, expectedProgress)
	}

	if result.Learnings != "Single line learning." {
		t.Errorf("Learnings = %q, want %q", result.Learnings, "Single line learning.")
	}
}

func TestParse_NestedCodeBlocks(t *testing.T) {
	input := `## Progress
Updated the parser:
` + "```go" + `
func Parse(s string) {
    // code here
}
` + "```" + `

## Learnings
The existing code style.`

	result := Parse(input)

	if !strings.Contains(result.Progress, "func Parse") {
		t.Errorf("Progress should contain code block content, got %q", result.Progress)
	}
}

func TestParse_SpecialCharacters(t *testing.T) {
	input := `## Progress
Added emoji support!
Fixed bug with <brackets> and &ampersands.

## Learnings
The codebase uses "quotes" and 'apostrophes'.`

	result := Parse(input)

	if !strings.Contains(result.Progress, "emoji") {
		t.Errorf("Progress should handle special chars: %q", result.Progress)
	}
	if !strings.Contains(result.Learnings, "quotes") {
		t.Errorf("Learnings should handle special chars: %q", result.Learnings)
	}
}

func TestParse_RawPreserved(t *testing.T) {
	input := "  ## Progress\nSome content\n  "

	result := Parse(input)

	if result.Raw != input {
		t.Errorf("Raw should be preserved exactly, got %q", result.Raw)
	}
}

func TestParse_HashInContent(t *testing.T) {
	// Single # shouldn't be treated as section header
	input := `## Progress
Added support for # comments in config files.
Also # hashtags work now.

## Learnings
Python uses # for comments.`

	result := Parse(input)

	if !strings.Contains(result.Progress, "# comments") {
		t.Errorf("Progress should preserve # in content: %q", result.Progress)
	}
}

func TestParse_DuplicateSectionHeaders(t *testing.T) {
	// First occurrence should win
	input := `## Progress
First progress section.

## Progress
Second progress section (should be ignored).

## Learnings
Learnings content.`

	result := Parse(input)

	// First progress section content
	if result.Progress != "First progress section." {
		t.Errorf("Progress = %q, want first section content", result.Progress)
	}
}

func TestParse_HeaderInsideCodeBlock(t *testing.T) {
	// ## Progress inside a code block should NOT be treated as a section header
	input := `## Learnings
Here's how markdown headers work:
` + "```markdown" + `
## Progress
This is a markdown header example
` + "```" + `
The actual Progress section is below.

## Progress
Real progress content.`

	result := Parse(input)

	// The ## Progress inside the code block should be ignored
	if result.Progress != "Real progress content." {
		t.Errorf("Progress = %q, want %q", result.Progress, "Real progress content.")
	}

	// Learnings should include the code block
	if !strings.Contains(result.Learnings, "Here's how markdown headers work:") {
		t.Errorf("Learnings should contain intro text, got %q", result.Learnings)
	}
	if !strings.Contains(result.Learnings, "```markdown") {
		t.Errorf("Learnings should contain code fence, got %q", result.Learnings)
	}
}

func TestParse_HeaderOnlyInCodeBlock(t *testing.T) {
	// If the only "## Progress" is inside a code block, it should not be found
	input := `## Learnings
Example of headers:
` + "```" + `
## Progress
Not a real section
` + "```" + `
That's all.`

	result := Parse(input)

	// Progress should be empty (the only ## Progress is inside code block)
	if result.Progress != "" {
		t.Errorf("Progress should be empty (header is in code block), got %q", result.Progress)
	}

	// Learnings should have all the content
	if !strings.Contains(result.Learnings, "Example of headers:") {
		t.Errorf("Learnings should contain content, got %q", result.Learnings)
	}
}

func TestParse_MultipleCodeBlocks(t *testing.T) {
	input := `## Progress
First code block:
` + "```go" + `
// ## Learnings - not a header
func foo() {}
` + "```" + `

Second code block:
` + "```python" + `
## Progress - also not a header
def bar(): pass
` + "```" + `

Real work done.

## Learnings
Actual learnings here.`

	result := Parse(input)

	// Progress should include both code blocks
	if !strings.Contains(result.Progress, "func foo()") {
		t.Errorf("Progress should contain first code block, got %q", result.Progress)
	}
	if !strings.Contains(result.Progress, "def bar()") {
		t.Errorf("Progress should contain second code block, got %q", result.Progress)
	}
	if !strings.Contains(result.Progress, "Real work done.") {
		t.Errorf("Progress should contain trailing content, got %q", result.Progress)
	}

	if result.Learnings != "Actual learnings here." {
		t.Errorf("Learnings = %q, want %q", result.Learnings, "Actual learnings here.")
	}
}

func TestParse_UnclosedCodeBlock(t *testing.T) {
	// Unclosed code block - everything after opening fence is treated as code
	input := `## Progress
Started work.
` + "```" + `
## Learnings
This looks like a header but is inside unclosed code block.`

	result := Parse(input)

	// Learnings header is inside unclosed code block, so not found
	if result.Learnings != "" {
		t.Errorf("Learnings should be empty (inside unclosed code block), got %q", result.Learnings)
	}

	// Progress should be found and have content
	if !strings.Contains(result.Progress, "Started work.") {
		t.Errorf("Progress should contain 'Started work.', got %q", result.Progress)
	}
}

// =============================================================================
// Status Section Tests
// =============================================================================

func TestParse_StatusSectionRunning(t *testing.T) {
	input := `## Progress
Built the feature.

## Learnings
Found existing patterns.

## Status
RUNNING RUNNING RUNNING`

	result := Parse(input)

	if result.IsDone {
		t.Error("IsDone should be false when status is RUNNING")
	}
	if result.Progress != "Built the feature." {
		t.Errorf("Progress = %q, want %q", result.Progress, "Built the feature.")
	}
	if result.Learnings != "Found existing patterns." {
		t.Errorf("Learnings = %q, want %q", result.Learnings, "Found existing patterns.")
	}
	if result.Status != "RUNNING RUNNING RUNNING" {
		t.Errorf("Status = %q, want %q", result.Status, "RUNNING RUNNING RUNNING")
	}
}

func TestParse_StatusSectionDone(t *testing.T) {
	input := `## Progress
Completed all tasks.

## Learnings
Everything works as expected.

## Status
DONE DONE DONE!!!`

	result := Parse(input)

	if !result.IsDone {
		t.Error("IsDone should be true when status contains done marker")
	}
	if result.Progress != "Completed all tasks." {
		t.Errorf("Progress = %q, want %q", result.Progress, "Completed all tasks.")
	}
	if result.Learnings != "Everything works as expected." {
		t.Errorf("Learnings = %q, want %q", result.Learnings, "Everything works as expected.")
	}
	if result.Status != "DONE DONE DONE!!!" {
		t.Errorf("Status = %q, want %q", result.Status, "DONE DONE DONE!!!")
	}
}

func TestParse_AllThreeSections(t *testing.T) {
	input := `## Progress
Made progress on the task.

## Learnings
Learned about the codebase structure.

## Status
RUNNING RUNNING RUNNING`

	result := Parse(input)

	if result.IsDone {
		t.Error("IsDone should be false")
	}
	if result.Progress != "Made progress on the task." {
		t.Errorf("Progress = %q, want %q", result.Progress, "Made progress on the task.")
	}
	if result.Learnings != "Learned about the codebase structure." {
		t.Errorf("Learnings = %q, want %q", result.Learnings, "Learned about the codebase structure.")
	}
	if result.Status != "RUNNING RUNNING RUNNING" {
		t.Errorf("Status = %q, want %q", result.Status, "RUNNING RUNNING RUNNING")
	}
}

func TestParse_StatusCaseInsensitive(t *testing.T) {
	input := `## progress
Some progress.

## learnings
Some learnings.

## status
RUNNING RUNNING RUNNING`

	result := Parse(input)

	if result.Progress != "Some progress." {
		t.Errorf("Progress = %q, want %q", result.Progress, "Some progress.")
	}
	if result.Learnings != "Some learnings." {
		t.Errorf("Learnings = %q, want %q", result.Learnings, "Some learnings.")
	}
	if result.Status != "RUNNING RUNNING RUNNING" {
		t.Errorf("Status = %q, want %q", result.Status, "RUNNING RUNNING RUNNING")
	}
}

func TestParse_BackwardsCompatibility_DoneWithoutStatusSection(t *testing.T) {
	// Old format: done marker anywhere in output should still work
	input := `## Progress
Everything is complete.

## Learnings
No issues found.

DONE DONE DONE!!!`

	result := Parse(input)

	if !result.IsDone {
		t.Error("IsDone should be true for backwards compatibility")
	}
}

func TestParse_StatusSectionEmptyDoesNotTriggerDone(t *testing.T) {
	input := `## Progress
Work in progress.

## Status
`

	result := Parse(input)

	if result.IsDone {
		t.Error("IsDone should be false when status section is empty")
	}
}
