package agent

import (
	"errors"
	"strings"
	"testing"
)

func TestBuildPrompt_AllFieldsPopulated(t *testing.T) {
	ctx := PromptContext{
		PlanContent: "Build a REST API with authentication",
		Progress:    "Completed user model and database schema",
		Learnings:   "The project uses GORM for database access",
	}

	result, err := BuildPrompt(ctx)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Check instructions section
	if !strings.Contains(result, "# Instructions") {
		t.Error("missing Instructions header")
	}
	if !strings.Contains(result, "experienced software developer") {
		t.Error("missing developer persona")
	}
	if !strings.Contains(result, "DONE DONE DONE!!!") {
		t.Error("missing done marker in instructions")
	}

	// Check plan section
	if !strings.Contains(result, "# Plan") {
		t.Error("missing Plan header")
	}
	if !strings.Contains(result, "Build a REST API with authentication") {
		t.Error("plan content not injected")
	}

	// Check progress section
	if !strings.Contains(result, "# Progress So Far") {
		t.Error("missing Progress So Far header")
	}
	if !strings.Contains(result, "Completed user model and database schema") {
		t.Error("progress content not injected")
	}

	// Check learnings section
	if !strings.Contains(result, "# Learnings So Far") {
		t.Error("missing Learnings So Far header")
	}
	if !strings.Contains(result, "The project uses GORM for database access") {
		t.Error("learnings content not injected")
	}

	// Should NOT contain fallback text
	if strings.Contains(result, "No progress yet.") {
		t.Error("should not show fallback when progress is provided")
	}
	if strings.Contains(result, "No learnings yet.") {
		t.Error("should not show fallback when learnings is provided")
	}
}

func TestBuildPrompt_EmptyProgress(t *testing.T) {
	ctx := PromptContext{
		PlanContent: "Build a CLI tool",
		Progress:    "",
		Learnings:   "Found existing patterns in codebase",
	}

	result, err := BuildPrompt(ctx)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Should show fallback for empty progress
	if !strings.Contains(result, "No progress yet.") {
		t.Error("should show 'No progress yet.' when progress is empty")
	}

	// Learnings should still be shown
	if !strings.Contains(result, "Found existing patterns in codebase") {
		t.Error("learnings should be injected")
	}
	if strings.Contains(result, "No learnings yet.") {
		t.Error("should not show fallback when learnings is provided")
	}
}

func TestBuildPrompt_EmptyLearnings(t *testing.T) {
	ctx := PromptContext{
		PlanContent: "Build a CLI tool",
		Progress:    "Implemented main function",
		Learnings:   "",
	}

	result, err := BuildPrompt(ctx)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Progress should be shown
	if !strings.Contains(result, "Implemented main function") {
		t.Error("progress should be injected")
	}
	if strings.Contains(result, "No progress yet.") {
		t.Error("should not show fallback when progress is provided")
	}

	// Should show fallback for empty learnings
	if !strings.Contains(result, "No learnings yet.") {
		t.Error("should show 'No learnings yet.' when learnings is empty")
	}
}

func TestBuildPrompt_EmptyProgressAndLearnings(t *testing.T) {
	ctx := PromptContext{
		PlanContent: "Build a CLI tool",
		Progress:    "",
		Learnings:   "",
	}

	result, err := BuildPrompt(ctx)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Should show both fallbacks
	if !strings.Contains(result, "No progress yet.") {
		t.Error("should show 'No progress yet.' when progress is empty")
	}
	if !strings.Contains(result, "No learnings yet.") {
		t.Error("should show 'No learnings yet.' when learnings is empty")
	}

	// Plan should still be shown
	if !strings.Contains(result, "Build a CLI tool") {
		t.Error("plan content should be injected")
	}
}

func TestBuildPrompt_EmptyPlan(t *testing.T) {
	ctx := PromptContext{
		PlanContent: "",
		Progress:    "Some progress",
		Learnings:   "Some learnings",
	}

	_, err := BuildPrompt(ctx)
	if err == nil {
		t.Fatal("expected error for empty PlanContent")
	}

	if !errors.Is(err, ErrEmptyPlanContent) {
		t.Errorf("expected ErrEmptyPlanContent, got: %v", err)
	}
}

func TestBuildPrompt_AllEmpty(t *testing.T) {
	ctx := PromptContext{
		PlanContent: "",
		Progress:    "",
		Learnings:   "",
	}

	_, err := BuildPrompt(ctx)
	if err == nil {
		t.Fatal("expected error for empty PlanContent")
	}

	if !errors.Is(err, ErrEmptyPlanContent) {
		t.Errorf("expected ErrEmptyPlanContent, got: %v", err)
	}
}

func TestBuildPrompt_MultilineContent(t *testing.T) {
	ctx := PromptContext{
		PlanContent: `Step 1: Design the API
Step 2: Implement endpoints
Step 3: Write tests`,
		Progress: `- Completed design phase
- Started implementation
- Working on GET /users`,
		Learnings: `The codebase uses:
- Go 1.21
- Standard library only
- No external dependencies`,
	}

	result, err := BuildPrompt(ctx)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Verify multiline content is preserved
	if !strings.Contains(result, "Step 1: Design the API\nStep 2: Implement endpoints") {
		t.Error("multiline plan content not preserved")
	}
	if !strings.Contains(result, "- Completed design phase\n- Started implementation") {
		t.Error("multiline progress content not preserved")
	}
	if !strings.Contains(result, "The codebase uses:\n- Go 1.21") {
		t.Error("multiline learnings content not preserved")
	}
}

func TestBuildPrompt_SpecialCharacters(t *testing.T) {
	ctx := PromptContext{
		PlanContent: "Handle special chars: <html>&quot;'test'</html>",
		Progress:    "Fixed bug with {{template}} syntax",
		Learnings:   "Use `backticks` for code, $variables work too",
	}

	result, err := BuildPrompt(ctx)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// text/template does not escape, so special chars should be preserved
	if !strings.Contains(result, "<html>&quot;'test'</html>") {
		t.Error("HTML special chars in plan not preserved")
	}
	if !strings.Contains(result, "{{template}}") {
		t.Error("template-like syntax in progress not preserved")
	}
	if !strings.Contains(result, "`backticks`") {
		t.Error("backticks in learnings not preserved")
	}
	if !strings.Contains(result, "$variables") {
		t.Error("dollar sign in learnings not preserved")
	}
}

func TestBuildPrompt_InstructionsSectionComplete(t *testing.T) {
	ctx := PromptContext{
		PlanContent: "Test plan",
		Progress:    "",
		Learnings:   "",
	}

	result, err := BuildPrompt(ctx)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Verify all capability bullets are present
	capabilities := []string{
		"Critically evaluate your own code",
		"Find and fix security and performance issues",
		"Maintain high standards for coding best practices",
		"Break work into smaller units",
		"Track your progress and learnings",
	}

	for _, cap := range capabilities {
		if !strings.Contains(result, cap) {
			t.Errorf("missing capability: %s", cap)
		}
	}

	// Verify output format section describes always outputting three sections
	if !strings.Contains(result, "Always output three sections") {
		t.Error("missing instruction to always output sections")
	}
	if !strings.Contains(result, "## Progress") {
		t.Error("missing ## Progress header in output format instructions")
	}
	if !strings.Contains(result, "## Learnings") {
		t.Error("missing ## Learnings header in output format instructions")
	}
	if !strings.Contains(result, "## Status") {
		t.Error("missing ## Status header in output format instructions")
	}
	if !strings.Contains(result, "RUNNING RUNNING RUNNING") {
		t.Error("missing RUNNING marker in output format instructions")
	}
}

func TestBuildPrompt_SectionSeparators(t *testing.T) {
	ctx := PromptContext{
		PlanContent: "Test plan",
		Progress:    "Test progress",
		Learnings:   "Test learnings",
	}

	result, err := BuildPrompt(ctx)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Check for section separators (---)
	separatorCount := strings.Count(result, "\n---\n")
	if separatorCount < 3 {
		t.Errorf("expected at least 3 section separators, got %d", separatorCount)
	}
}

func TestBuildPrompt_OutputFormatExactMarker(t *testing.T) {
	ctx := PromptContext{
		PlanContent: "Test plan",
		Progress:    "",
		Learnings:   "",
	}

	result, err := BuildPrompt(ctx)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// The exact marker that agents should output when done (in Status section)
	if !strings.Contains(result, "DONE DONE DONE!!!") {
		t.Error("missing exact 'DONE DONE DONE!!!' marker")
	}

	// The running marker for normal operation
	if !strings.Contains(result, "RUNNING RUNNING RUNNING") {
		t.Error("missing exact 'RUNNING RUNNING RUNNING' marker")
	}

	// Instructions should mention review-only session requirement
	if !strings.Contains(result, "review-only session") {
		t.Error("missing instruction about review-only session requirement for done marker")
	}
}

func TestPromptContext_DefaultValues(t *testing.T) {
	// Test that zero-value PromptContext returns an error (empty PlanContent)
	var ctx PromptContext

	_, err := BuildPrompt(ctx)
	if err == nil {
		t.Fatal("expected error for zero-value context (empty PlanContent)")
	}

	if !errors.Is(err, ErrEmptyPlanContent) {
		t.Errorf("expected ErrEmptyPlanContent, got: %v", err)
	}
}

func TestPromptTemplate_IsValid(t *testing.T) {
	// Verify the template can be parsed (no syntax errors)
	if PromptTemplate == "" {
		t.Fatal("PromptTemplate should not be empty")
	}

	// Verify required template variables are present
	requiredVars := []string{
		"{{.PlanContent}}",
		"{{.Progress}}",
		"{{.Learnings}}",
	}

	for _, v := range requiredVars {
		if !strings.Contains(PromptTemplate, v) {
			t.Errorf("PromptTemplate missing required variable: %s", v)
		}
	}

	// Verify conditionals for empty values
	if !strings.Contains(PromptTemplate, "{{if .Progress}}") {
		t.Error("PromptTemplate missing conditional for Progress")
	}
	if !strings.Contains(PromptTemplate, "{{if .Learnings}}") {
		t.Error("PromptTemplate missing conditional for Learnings")
	}
}

func TestBuildPrompt_WhitespaceOnlyFields(t *testing.T) {
	// Whitespace-only strings are normalized to empty and trigger fallbacks
	ctx := PromptContext{
		PlanContent: "Test plan",
		Progress:    "   ",
		Learnings:   "\t\n",
	}

	result, err := BuildPrompt(ctx)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Whitespace-only strings should trigger fallbacks
	if !strings.Contains(result, "No progress yet.") {
		t.Error("whitespace-only progress should trigger fallback")
	}
	if !strings.Contains(result, "No learnings yet.") {
		t.Error("whitespace-only learnings should trigger fallback")
	}
}

func TestBuildPrompt_WhitespaceOnlyPlan(t *testing.T) {
	// Whitespace-only PlanContent should be treated as empty and return an error
	ctx := PromptContext{
		PlanContent: "   \t\n  ",
		Progress:    "Some progress",
		Learnings:   "Some learnings",
	}

	_, err := BuildPrompt(ctx)
	if err == nil {
		t.Fatal("expected error for whitespace-only PlanContent")
	}

	if !errors.Is(err, ErrEmptyPlanContent) {
		t.Errorf("expected ErrEmptyPlanContent, got: %v", err)
	}
}

func TestBuildPrompt_LongContent(t *testing.T) {
	// Test with very long content to ensure no truncation
	longPlan := strings.Repeat("This is a long plan. ", 1000)
	longProgress := strings.Repeat("Progress item. ", 500)
	longLearnings := strings.Repeat("Learning discovered. ", 500)

	ctx := PromptContext{
		PlanContent: longPlan,
		Progress:    longProgress,
		Learnings:   longLearnings,
	}

	result, err := BuildPrompt(ctx)
	if err != nil {
		t.Fatalf("unexpected error with long content: %v", err)
	}

	// Verify content is not truncated
	if !strings.Contains(result, longPlan) {
		t.Error("long plan content was truncated")
	}
	if !strings.Contains(result, longProgress) {
		t.Error("long progress content was truncated")
	}
	if !strings.Contains(result, longLearnings) {
		t.Error("long learnings content was truncated")
	}
}

// =============================================================================
// V1.5 Developer Prompt Tests
// =============================================================================

func TestBuildV15DeveloperPrompt_AllFieldsPopulated(t *testing.T) {
	ctx := V15DeveloperContext{
		PlanContent:      "Build a REST API with authentication",
		Progress:         "Completed user model",
		Learnings:        "Uses GORM for database",
		ReviewerFeedback: "",
	}

	result, err := BuildV15DeveloperPrompt(ctx)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Check instructions section
	if !strings.Contains(result, "# Instructions") {
		t.Error("missing Instructions header")
	}
	if !strings.Contains(result, "experienced software developer") {
		t.Error("missing developer persona")
	}

	// Should use DEV_DONE marker, not DONE DONE DONE
	if !strings.Contains(result, "DEV_DONE DEV_DONE DEV_DONE!!!") {
		t.Error("missing DEV_DONE marker in instructions")
	}
	// Should NOT contain old DONE marker in instructions
	if strings.Contains(result, "DONE DONE DONE!!!") {
		t.Error("should use DEV_DONE, not old DONE marker")
	}

	// Check plan section
	if !strings.Contains(result, "# Plan") {
		t.Error("missing Plan header")
	}
	if !strings.Contains(result, "Build a REST API with authentication") {
		t.Error("plan content not injected")
	}

	// Check progress section
	if !strings.Contains(result, "# Progress So Far") {
		t.Error("missing Progress So Far header")
	}
	if !strings.Contains(result, "Completed user model") {
		t.Error("progress content not injected")
	}

	// Check learnings section
	if !strings.Contains(result, "# Learnings So Far") {
		t.Error("missing Learnings So Far header")
	}
	if !strings.Contains(result, "Uses GORM for database") {
		t.Error("learnings content not injected")
	}

	// Should NOT contain reviewer feedback section (empty)
	if strings.Contains(result, "# Reviewer Feedback") {
		t.Error("should not show reviewer feedback section when empty")
	}
}

func TestBuildV15DeveloperPrompt_WithReviewerFeedback(t *testing.T) {
	ctx := V15DeveloperContext{
		PlanContent:      "Build an API",
		Progress:         "In progress",
		Learnings:        "Learning things",
		ReviewerFeedback: "Missing error handling in auth.go:42",
	}

	result, err := BuildV15DeveloperPrompt(ctx)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Should contain reviewer feedback section
	if !strings.Contains(result, "# Reviewer Feedback") {
		t.Error("missing Reviewer Feedback header when feedback provided")
	}
	if !strings.Contains(result, "Missing error handling in auth.go:42") {
		t.Error("reviewer feedback content not injected")
	}
	if !strings.Contains(result, "MUST ADDRESS") {
		t.Error("missing emphasis on addressing feedback")
	}
}

func TestBuildV15DeveloperPrompt_EmptyPlan(t *testing.T) {
	ctx := V15DeveloperContext{
		PlanContent: "",
		Progress:    "Some progress",
		Learnings:   "Some learnings",
	}

	_, err := BuildV15DeveloperPrompt(ctx)
	if err == nil {
		t.Fatal("expected error for empty PlanContent")
	}

	if !errors.Is(err, ErrEmptyPlanContent) {
		t.Errorf("expected ErrEmptyPlanContent, got: %v", err)
	}
}

func TestBuildV15DeveloperPrompt_Fallbacks(t *testing.T) {
	ctx := V15DeveloperContext{
		PlanContent:      "Test plan",
		Progress:         "",
		Learnings:        "",
		ReviewerFeedback: "",
	}

	result, err := BuildV15DeveloperPrompt(ctx)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Should show both fallbacks
	if !strings.Contains(result, "No progress yet.") {
		t.Error("should show progress fallback")
	}
	if !strings.Contains(result, "No learnings yet.") {
		t.Error("should show learnings fallback")
	}
}

func TestBuildV15DeveloperPrompt_WhitespaceNormalized(t *testing.T) {
	ctx := V15DeveloperContext{
		PlanContent:      "Test plan",
		Progress:         "   ",
		Learnings:        "\t\n",
		ReviewerFeedback: "  \n  ",
	}

	result, err := BuildV15DeveloperPrompt(ctx)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Whitespace-only should trigger fallbacks
	if !strings.Contains(result, "No progress yet.") {
		t.Error("whitespace progress should trigger fallback")
	}
	if !strings.Contains(result, "No learnings yet.") {
		t.Error("whitespace learnings should trigger fallback")
	}
	// Whitespace feedback should NOT show section
	if strings.Contains(result, "# Reviewer Feedback") {
		t.Error("whitespace feedback should not show section")
	}
}

// =============================================================================
// V1.5 Reviewer Prompt Tests
// =============================================================================

func TestBuildV15ReviewerPrompt_AllFieldsPopulated(t *testing.T) {
	ctx := V15ReviewerContext{
		PlanContent:      "Build a REST API",
		Progress:         "Completed implementation",
		Learnings:        "Uses Go patterns",
		DiffOutput:       "+ func NewUser() {}\n- func OldUser() {}",
		DeveloperSummary: "Added user creation endpoint",
	}

	result, err := BuildV15ReviewerPrompt(ctx)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Check reviewer-specific instructions
	if !strings.Contains(result, "VERY HARD CRITIC") {
		t.Error("missing very hard critic instruction")
	}
	if !strings.Contains(result, "ONLY approve") {
		t.Error("missing strict approval criteria")
	}

	// Check approval/feedback markers
	if !strings.Contains(result, "REVIEWER_APPROVED REVIEWER_APPROVED!!!") {
		t.Error("missing REVIEWER_APPROVED marker")
	}
	if !strings.Contains(result, "REVIEWER_FEEDBACK:") {
		t.Error("missing REVIEWER_FEEDBACK prefix instruction")
	}

	// Check checklist items
	checklistItems := []string{
		"Correctness",
		"Edge Cases",
		"Error Handling",
		"Security",
		"Performance",
		"Tests",
		"Style",
		"Documentation",
	}
	for _, item := range checklistItems {
		if !strings.Contains(result, item) {
			t.Errorf("missing checklist item: %s", item)
		}
	}

	// Check issue sections instruction
	if !strings.Contains(result, "Critical Issues") {
		t.Error("missing Critical Issues section instruction")
	}
	if !strings.Contains(result, "Major Issues") {
		t.Error("missing Major Issues section instruction")
	}
	if !strings.Contains(result, "Minor Issues") {
		t.Error("missing Minor Issues section instruction")
	}

	// Check diff section
	if !strings.Contains(result, "# Diff to Review") {
		t.Error("missing Diff to Review header")
	}
	if !strings.Contains(result, "+ func NewUser()") {
		t.Error("diff content not injected")
	}
	if !strings.Contains(result, "```diff") {
		t.Error("diff should be in code block")
	}

	// Check developer summary section
	if !strings.Contains(result, "# Developer Summary") {
		t.Error("missing Developer Summary header")
	}
	if !strings.Contains(result, "Added user creation endpoint") {
		t.Error("developer summary not injected")
	}
}

func TestBuildV15ReviewerPrompt_EmptyPlan(t *testing.T) {
	ctx := V15ReviewerContext{
		PlanContent: "",
		Progress:    "Some progress",
		Learnings:   "Some learnings",
		DiffOutput:  "some diff",
	}

	_, err := BuildV15ReviewerPrompt(ctx)
	if err == nil {
		t.Fatal("expected error for empty PlanContent")
	}

	if !errors.Is(err, ErrEmptyPlanContent) {
		t.Errorf("expected ErrEmptyPlanContent, got: %v", err)
	}
}

func TestBuildV15ReviewerPrompt_Fallbacks(t *testing.T) {
	ctx := V15ReviewerContext{
		PlanContent:      "Test plan",
		Progress:         "",
		Learnings:        "",
		DiffOutput:       "",
		DeveloperSummary: "",
	}

	result, err := BuildV15ReviewerPrompt(ctx)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Should show fallbacks
	if !strings.Contains(result, "No progress yet.") {
		t.Error("should show progress fallback")
	}
	if !strings.Contains(result, "No learnings yet.") {
		t.Error("should show learnings fallback")
	}
	if !strings.Contains(result, "No diff available.") {
		t.Error("should show diff fallback")
	}
	if !strings.Contains(result, "No developer summary available.") {
		t.Error("should show developer summary fallback")
	}
}

func TestBuildV15ReviewerPrompt_ZeroIssuesApproval(t *testing.T) {
	ctx := V15ReviewerContext{
		PlanContent: "Test plan",
		Progress:    "Complete",
		Learnings:   "Done",
		DiffOutput:  "some changes",
	}

	result, err := BuildV15ReviewerPrompt(ctx)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Instructions should emphasize zero tolerance for issues
	if !strings.Contains(result, "Zero critical issues") {
		t.Error("missing zero critical issues requirement")
	}
	if !strings.Contains(result, "Zero major issues") {
		t.Error("missing zero major issues requirement")
	}
	if !strings.Contains(result, "Zero minor issues") {
		t.Error("missing zero minor issues requirement")
	}
}

func TestV15DeveloperPromptTemplate_IsValid(t *testing.T) {
	// Verify the template is not empty and contains required variables
	if V15DeveloperPromptTemplate == "" {
		t.Fatal("V15DeveloperPromptTemplate should not be empty")
	}

	requiredVars := []string{
		"{{.PlanContent}}",
		"{{.Progress}}",
		"{{.Learnings}}",
		"{{.ReviewerFeedback}}",
	}

	for _, v := range requiredVars {
		if !strings.Contains(V15DeveloperPromptTemplate, v) {
			t.Errorf("V15DeveloperPromptTemplate missing required variable: %s", v)
		}
	}
}

func TestV15ReviewerPromptTemplate_IsValid(t *testing.T) {
	// Verify the template is not empty and contains required variables
	if V15ReviewerPromptTemplate == "" {
		t.Fatal("V15ReviewerPromptTemplate should not be empty")
	}

	requiredVars := []string{
		"{{.PlanContent}}",
		"{{.Progress}}",
		"{{.Learnings}}",
		"{{.DiffOutput}}",
		"{{.DeveloperSummary}}",
	}

	for _, v := range requiredVars {
		if !strings.Contains(V15ReviewerPromptTemplate, v) {
			t.Errorf("V15ReviewerPromptTemplate missing required variable: %s", v)
		}
	}
}
