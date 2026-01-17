package agents

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/gerund/ralph/internal/config"
	"github.com/gerund/ralph/internal/db"
)

func TestExecuteTemplate_Basic(t *testing.T) {
	tmpl := "Hello, {{.Name}}!"
	data := struct{ Name string }{"World"}

	result, err := executeTemplate(tmpl, data)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if result != "Hello, World!" {
		t.Errorf("expected 'Hello, World!', got '%s'", result)
	}
}

func TestExecuteTemplate_MultipleFields(t *testing.T) {
	tmpl := "{{.Title}} by {{.Author}}"
	data := struct {
		Title  string
		Author string
	}{"The Book", "Jane Doe"}

	result, err := executeTemplate(tmpl, data)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if result != "The Book by Jane Doe" {
		t.Errorf("expected 'The Book by Jane Doe', got '%s'", result)
	}
}

func TestExecuteTemplate_Conditional(t *testing.T) {
	tmpl := "{{if .Visible}}Shown{{end}}"

	// Test with true
	data1 := struct{ Visible bool }{true}
	result1, err := executeTemplate(tmpl, data1)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result1 != "Shown" {
		t.Errorf("expected 'Shown', got '%s'", result1)
	}

	// Test with false
	data2 := struct{ Visible bool }{false}
	result2, err := executeTemplate(tmpl, data2)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result2 != "" {
		t.Errorf("expected empty string, got '%s'", result2)
	}
}

func TestExecuteTemplate_ConditionalString(t *testing.T) {
	tmpl := "Start{{if .Text}} - {{.Text}}{{end}} - End"

	// Test with non-empty string
	data1 := struct{ Text string }{"middle"}
	result1, err := executeTemplate(tmpl, data1)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result1 != "Start - middle - End" {
		t.Errorf("expected 'Start - middle - End', got '%s'", result1)
	}

	// Test with empty string
	data2 := struct{ Text string }{""}
	result2, err := executeTemplate(tmpl, data2)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result2 != "Start - End" {
		t.Errorf("expected 'Start - End', got '%s'", result2)
	}
}

func TestExecuteTemplate_InvalidTemplate(t *testing.T) {
	tmpl := "{{.Invalid"

	_, err := executeTemplate(tmpl, struct{}{})
	if err == nil {
		t.Fatal("expected error for invalid template")
	}

	if !strings.Contains(err.Error(), "failed to parse template") {
		t.Errorf("expected parse error, got: %v", err)
	}
}

func TestExecuteTemplate_MissingField(t *testing.T) {
	tmpl := "{{.Missing}}"
	data := struct{}{}

	_, err := executeTemplate(tmpl, data)
	if err == nil {
		t.Fatal("expected error for missing field")
	}

	if !strings.Contains(err.Error(), "failed to execute template") {
		t.Errorf("expected execute error, got: %v", err)
	}
}

func TestBuildDeveloperPrompt_Basic(t *testing.T) {
	ctx := DeveloperContext{
		Plan:            "Build a CLI tool",
		TaskTitle:       "Task 1",
		TaskDescription: "Implement the main function",
		Feedback:        "",
	}

	result, err := buildDeveloperPrompt(DefaultDeveloperPrompt, ctx)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Check that context values are injected
	if !strings.Contains(result, "Build a CLI tool") {
		t.Error("plan not injected into prompt")
	}
	if !strings.Contains(result, "Task 1") {
		t.Error("task title not injected into prompt")
	}
	if !strings.Contains(result, "Implement the main function") {
		t.Error("task description not injected into prompt")
	}

	// Feedback section should not appear when empty
	if strings.Contains(result, "Previous Review Feedback") {
		t.Error("feedback section should not appear when feedback is empty")
	}
}

func TestBuildDeveloperPrompt_WithFeedback(t *testing.T) {
	ctx := DeveloperContext{
		Plan:            "Build a CLI tool",
		TaskTitle:       "Task 1",
		TaskDescription: "Implement the main function",
		Feedback:        "Please add error handling",
	}

	result, err := buildDeveloperPrompt(DefaultDeveloperPrompt, ctx)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Check that feedback is included
	if !strings.Contains(result, "Previous Review Feedback") {
		t.Error("feedback section header not found")
	}
	if !strings.Contains(result, "Please add error handling") {
		t.Error("feedback content not injected into prompt")
	}
	if !strings.Contains(result, "Address all feedback items") {
		t.Error("feedback instruction not found")
	}
}

func TestBuildReviewerPrompt_Basic(t *testing.T) {
	ctx := ReviewerContext{
		Plan:            "Build a CLI tool",
		TaskTitle:       "Task 1",
		TaskDescription: "Implement the main function",
		DiffOutput:      "+func main() {\n+\tfmt.Println(\"Hello\")\n+}",
	}

	result, err := buildReviewerPrompt(DefaultReviewerPrompt, ctx)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Check that context values are injected
	if !strings.Contains(result, "Build a CLI tool") {
		t.Error("plan not injected into prompt")
	}
	if !strings.Contains(result, "Task 1") {
		t.Error("task title not injected into prompt")
	}
	if !strings.Contains(result, "Implement the main function") {
		t.Error("task description not injected into prompt")
	}
	if !strings.Contains(result, "func main()") {
		t.Error("diff output not injected into prompt")
	}
	if !strings.Contains(result, "APPROVED") {
		t.Error("APPROVED output format not found")
	}
	if !strings.Contains(result, "FEEDBACK:") {
		t.Error("FEEDBACK output format not found")
	}
}

func TestBuildPlannerPrompt_Basic(t *testing.T) {
	ctx := PlannerContext{
		PlanText: "Create a REST API with user authentication",
	}

	result, err := buildPlannerPrompt(DefaultPlannerPrompt, ctx)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Check that context values are injected
	if !strings.Contains(result, "Create a REST API with user authentication") {
		t.Error("plan text not injected into prompt")
	}
	if !strings.Contains(result, "JSON array of tasks") {
		t.Error("JSON output format instruction not found")
	}
	if !strings.Contains(result, "sequence") {
		t.Error("sequence field not mentioned in output format")
	}
}

func TestAgentBuilder_Developer(t *testing.T) {
	cfg := config.DefaultConfig()
	builder := NewAgentBuilder(cfg)

	ctx := DeveloperContext{
		Plan:            "Test plan",
		TaskTitle:       "Test task",
		TaskDescription: "Test description",
		Feedback:        "",
	}

	agent, err := builder.Developer(ctx)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if agent.Type != AgentTypeDeveloper {
		t.Errorf("expected developer type, got %s", agent.Type)
	}

	if !strings.Contains(agent.Prompt, "Test plan") {
		t.Error("plan not injected into agent prompt")
	}
	if !strings.Contains(agent.Prompt, "Test task") {
		t.Error("task title not injected into agent prompt")
	}
}

func TestAgentBuilder_Reviewer(t *testing.T) {
	cfg := config.DefaultConfig()
	builder := NewAgentBuilder(cfg)

	ctx := ReviewerContext{
		Plan:            "Test plan",
		TaskTitle:       "Test task",
		TaskDescription: "Test description",
		DiffOutput:      "+new line",
	}

	agent, err := builder.Reviewer(ctx)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if agent.Type != AgentTypeReviewer {
		t.Errorf("expected reviewer type, got %s", agent.Type)
	}

	if !strings.Contains(agent.Prompt, "+new line") {
		t.Error("diff output not injected into agent prompt")
	}
}

func TestAgentBuilder_Planner(t *testing.T) {
	cfg := config.DefaultConfig()
	builder := NewAgentBuilder(cfg)

	ctx := PlannerContext{
		PlanText: "Test plan to decompose",
	}

	agent, err := builder.Planner(ctx)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if agent.Type != AgentTypePlanner {
		t.Errorf("expected planner type, got %s", agent.Type)
	}

	if !strings.Contains(agent.Prompt, "Test plan to decompose") {
		t.Error("plan text not injected into agent prompt")
	}
}

func TestAgentBuilder_CustomPrompt(t *testing.T) {
	// Create a temp directory and custom prompt file
	tmpDir := t.TempDir()
	promptPath := filepath.Join(tmpDir, "custom_developer.md")

	customPrompt := "Custom developer prompt: {{.Plan}} - {{.TaskTitle}}"
	if err := os.WriteFile(promptPath, []byte(customPrompt), 0644); err != nil {
		t.Fatalf("failed to write custom prompt: %v", err)
	}

	cfg := config.DefaultConfig()
	cfg.Agents.Developer = promptPath

	builder := NewAgentBuilder(cfg)

	ctx := DeveloperContext{
		Plan:            "My Plan",
		TaskTitle:       "My Task",
		TaskDescription: "Description",
		Feedback:        "",
	}

	agent, err := builder.Developer(ctx)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	expected := "Custom developer prompt: My Plan - My Task"
	if agent.Prompt != expected {
		t.Errorf("expected '%s', got '%s'", expected, agent.Prompt)
	}
}

func TestNewManager(t *testing.T) {
	cfg := config.DefaultConfig()
	manager := NewManager(cfg)

	if manager == nil {
		t.Fatal("expected non-nil manager")
	}

	if manager.builder == nil {
		t.Error("expected non-nil builder")
	}

	if manager.config != cfg {
		t.Error("expected config to be stored")
	}
}

func TestManager_GetDeveloperAgent(t *testing.T) {
	cfg := config.DefaultConfig()
	manager := NewManager(cfg)

	task := &db.Task{
		Title:       "Implement feature X",
		Description: "Add feature X to the codebase",
	}

	agent, err := manager.GetDeveloperAgent(context.Background(), "Main plan", task, "")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if agent.Type != AgentTypeDeveloper {
		t.Errorf("expected developer type, got %s", agent.Type)
	}

	if !strings.Contains(agent.Prompt, "Main plan") {
		t.Error("plan not found in prompt")
	}
	if !strings.Contains(agent.Prompt, "Implement feature X") {
		t.Error("task title not found in prompt")
	}
	if !strings.Contains(agent.Prompt, "Add feature X to the codebase") {
		t.Error("task description not found in prompt")
	}
}

func TestManager_GetDeveloperAgent_WithFeedback(t *testing.T) {
	cfg := config.DefaultConfig()
	manager := NewManager(cfg)

	task := &db.Task{
		Title:       "Implement feature X",
		Description: "Add feature X to the codebase",
	}

	feedback := "Please handle the error case"
	agent, err := manager.GetDeveloperAgent(context.Background(), "Main plan", task, feedback)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if !strings.Contains(agent.Prompt, "Please handle the error case") {
		t.Error("feedback not found in prompt")
	}
	if !strings.Contains(agent.Prompt, "Previous Review Feedback") {
		t.Error("feedback section not found in prompt")
	}
}

func TestManager_GetReviewerAgent(t *testing.T) {
	cfg := config.DefaultConfig()
	manager := NewManager(cfg)

	task := &db.Task{
		Title:       "Implement feature X",
		Description: "Add feature X to the codebase",
	}

	diffOutput := `diff --git a/main.go b/main.go
+func newFeature() {
+    // implementation
+}`

	agent, err := manager.GetReviewerAgent(context.Background(), "Main plan", task, diffOutput)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if agent.Type != AgentTypeReviewer {
		t.Errorf("expected reviewer type, got %s", agent.Type)
	}

	if !strings.Contains(agent.Prompt, "Main plan") {
		t.Error("plan not found in prompt")
	}
	if !strings.Contains(agent.Prompt, "func newFeature()") {
		t.Error("diff output not found in prompt")
	}
}

func TestManager_GetPlannerAgent(t *testing.T) {
	cfg := config.DefaultConfig()
	manager := NewManager(cfg)

	planText := "Build a web application with user authentication"
	agent, err := manager.GetPlannerAgent(context.Background(), planText)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if agent.Type != AgentTypePlanner {
		t.Errorf("expected planner type, got %s", agent.Type)
	}

	if !strings.Contains(agent.Prompt, planText) {
		t.Error("plan text not found in prompt")
	}
}

func TestManager_GetAgent_Developer(t *testing.T) {
	cfg := config.DefaultConfig()
	manager := NewManager(cfg)

	agent, err := manager.GetAgent(context.Background(), AgentTypeDeveloper)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if agent.Type != AgentTypeDeveloper {
		t.Errorf("expected developer type, got %s", agent.Type)
	}
}

func TestManager_GetAgent_Reviewer(t *testing.T) {
	cfg := config.DefaultConfig()
	manager := NewManager(cfg)

	agent, err := manager.GetAgent(context.Background(), AgentTypeReviewer)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if agent.Type != AgentTypeReviewer {
		t.Errorf("expected reviewer type, got %s", agent.Type)
	}
}

func TestManager_GetAgent_Planner(t *testing.T) {
	cfg := config.DefaultConfig()
	manager := NewManager(cfg)

	agent, err := manager.GetAgent(context.Background(), AgentTypePlanner)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if agent.Type != AgentTypePlanner {
		t.Errorf("expected planner type, got %s", agent.Type)
	}
}

func TestManager_GetAgent_Unknown(t *testing.T) {
	cfg := config.DefaultConfig()
	manager := NewManager(cfg)

	_, err := manager.GetAgent(context.Background(), AgentType("unknown"))
	if err == nil {
		t.Fatal("expected error for unknown agent type")
	}

	if !strings.Contains(err.Error(), "unknown agent type") {
		t.Errorf("expected unknown agent type error, got: %v", err)
	}
}

func TestManager_Config(t *testing.T) {
	cfg := config.DefaultConfig()
	manager := NewManager(cfg)

	if manager.Config() != cfg {
		t.Error("Config() should return the stored config")
	}
}

func TestDefaultDeveloperPrompt_HasRequiredSections(t *testing.T) {
	// Verify the default prompt has all required template variables
	if !strings.Contains(DefaultDeveloperPrompt, "{{.Plan}}") {
		t.Error("DefaultDeveloperPrompt missing {{.Plan}}")
	}
	if !strings.Contains(DefaultDeveloperPrompt, "{{.TaskTitle}}") {
		t.Error("DefaultDeveloperPrompt missing {{.TaskTitle}}")
	}
	if !strings.Contains(DefaultDeveloperPrompt, "{{.TaskDescription}}") {
		t.Error("DefaultDeveloperPrompt missing {{.TaskDescription}}")
	}
	if !strings.Contains(DefaultDeveloperPrompt, "{{.Feedback}}") {
		t.Error("DefaultDeveloperPrompt missing {{.Feedback}}")
	}
	if !strings.Contains(DefaultDeveloperPrompt, "{{if .Feedback}}") {
		t.Error("DefaultDeveloperPrompt missing conditional for Feedback")
	}
}

func TestDefaultReviewerPrompt_HasRequiredSections(t *testing.T) {
	// Verify the default prompt has all required template variables
	if !strings.Contains(DefaultReviewerPrompt, "{{.Plan}}") {
		t.Error("DefaultReviewerPrompt missing {{.Plan}}")
	}
	if !strings.Contains(DefaultReviewerPrompt, "{{.TaskTitle}}") {
		t.Error("DefaultReviewerPrompt missing {{.TaskTitle}}")
	}
	if !strings.Contains(DefaultReviewerPrompt, "{{.TaskDescription}}") {
		t.Error("DefaultReviewerPrompt missing {{.TaskDescription}}")
	}
	if !strings.Contains(DefaultReviewerPrompt, "{{.DiffOutput}}") {
		t.Error("DefaultReviewerPrompt missing {{.DiffOutput}}")
	}
	// Check for output format markers
	if !strings.Contains(DefaultReviewerPrompt, "APPROVED") {
		t.Error("DefaultReviewerPrompt missing APPROVED marker")
	}
	if !strings.Contains(DefaultReviewerPrompt, "FEEDBACK:") {
		t.Error("DefaultReviewerPrompt missing FEEDBACK: marker")
	}
}

func TestDefaultPlannerPrompt_HasRequiredSections(t *testing.T) {
	// Verify the default prompt has all required template variables
	if !strings.Contains(DefaultPlannerPrompt, "{{.PlanText}}") {
		t.Error("DefaultPlannerPrompt missing {{.PlanText}}")
	}
	// Check for JSON output format instructions
	if !strings.Contains(DefaultPlannerPrompt, "JSON") {
		t.Error("DefaultPlannerPrompt missing JSON instruction")
	}
	if !strings.Contains(DefaultPlannerPrompt, "title") {
		t.Error("DefaultPlannerPrompt missing title field in output format")
	}
	if !strings.Contains(DefaultPlannerPrompt, "description") {
		t.Error("DefaultPlannerPrompt missing description field in output format")
	}
	if !strings.Contains(DefaultPlannerPrompt, "sequence") {
		t.Error("DefaultPlannerPrompt missing sequence field in output format")
	}
}

func TestDeveloperContext_EmptyFields(t *testing.T) {
	// Test that empty fields don't cause template errors
	ctx := DeveloperContext{}

	result, err := buildDeveloperPrompt(DefaultDeveloperPrompt, ctx)
	if err != nil {
		t.Fatalf("unexpected error with empty context: %v", err)
	}

	// Should still produce valid output
	if result == "" {
		t.Error("expected non-empty result")
	}

	// Feedback section should not appear
	if strings.Contains(result, "Previous Review Feedback") {
		t.Error("feedback section should not appear with empty feedback")
	}
}

func TestReviewerContext_EmptyFields(t *testing.T) {
	// Test that empty fields don't cause template errors
	ctx := ReviewerContext{}

	result, err := buildReviewerPrompt(DefaultReviewerPrompt, ctx)
	if err != nil {
		t.Fatalf("unexpected error with empty context: %v", err)
	}

	// Should still produce valid output
	if result == "" {
		t.Error("expected non-empty result")
	}
}

func TestPlannerContext_EmptyFields(t *testing.T) {
	// Test that empty fields don't cause template errors
	ctx := PlannerContext{}

	result, err := buildPlannerPrompt(DefaultPlannerPrompt, ctx)
	if err != nil {
		t.Fatalf("unexpected error with empty context: %v", err)
	}

	// Should still produce valid output
	if result == "" {
		t.Error("expected non-empty result")
	}
}

func TestDeprecatedDeveloperAgent(t *testing.T) {
	// Test the deprecated function still works
	agent := DeveloperAgent("description", "feedback")

	if agent.Type != AgentTypeDeveloper {
		t.Errorf("expected developer type, got %s", agent.Type)
	}

	// Returns the default prompt (without templating)
	if agent.Prompt != DefaultDeveloperPrompt {
		t.Error("expected default developer prompt")
	}
}

func TestDeprecatedReviewerAgent(t *testing.T) {
	// Test the deprecated function still works
	agent := ReviewerAgent("description", "diff")

	if agent.Type != AgentTypeReviewer {
		t.Errorf("expected reviewer type, got %s", agent.Type)
	}

	// Returns the default prompt (without templating)
	if agent.Prompt != DefaultReviewerPrompt {
		t.Error("expected default reviewer prompt")
	}
}

func TestDeprecatedPlannerAgent(t *testing.T) {
	// Test the deprecated function still works
	agent := PlannerAgent("plan text")

	if agent.Type != AgentTypePlanner {
		t.Errorf("expected planner type, got %s", agent.Type)
	}

	// Returns the default prompt (without templating)
	if agent.Prompt != DefaultPlannerPrompt {
		t.Error("expected default planner prompt")
	}
}

func TestAgentTypes(t *testing.T) {
	// Verify agent type constants
	if AgentTypeDeveloper != "developer" {
		t.Errorf("expected 'developer', got '%s'", AgentTypeDeveloper)
	}
	if AgentTypeReviewer != "reviewer" {
		t.Errorf("expected 'reviewer', got '%s'", AgentTypeReviewer)
	}
	if AgentTypePlanner != "planner" {
		t.Errorf("expected 'planner', got '%s'", AgentTypePlanner)
	}
}

func TestCustomPromptWithTemplateVariables(t *testing.T) {
	// Create a temp directory and custom prompt file with template variables
	tmpDir := t.TempDir()
	promptPath := filepath.Join(tmpDir, "custom_reviewer.md")

	customPrompt := `Review for task: {{.TaskTitle}}

Changes:
{{.DiffOutput}}

Plan context: {{.Plan}}

{{if .TaskDescription}}
Details: {{.TaskDescription}}
{{end}}`

	if err := os.WriteFile(promptPath, []byte(customPrompt), 0644); err != nil {
		t.Fatalf("failed to write custom prompt: %v", err)
	}

	cfg := config.DefaultConfig()
	cfg.Agents.Reviewer = promptPath

	builder := NewAgentBuilder(cfg)

	ctx := ReviewerContext{
		Plan:            "Build an API",
		TaskTitle:       "Add endpoint",
		TaskDescription: "Create GET /users endpoint",
		DiffOutput:      "+route.GET('/users')",
	}

	agent, err := builder.Reviewer(ctx)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if !strings.Contains(agent.Prompt, "Review for task: Add endpoint") {
		t.Error("task title not properly substituted")
	}
	if !strings.Contains(agent.Prompt, "+route.GET('/users')") {
		t.Error("diff output not properly substituted")
	}
	if !strings.Contains(agent.Prompt, "Plan context: Build an API") {
		t.Error("plan not properly substituted")
	}
	if !strings.Contains(agent.Prompt, "Details: Create GET /users endpoint") {
		t.Error("task description not properly substituted")
	}
}

func TestPromptWithSpecialCharacters(t *testing.T) {
	ctx := DeveloperContext{
		Plan:            "Build a CLI with flags like --help and --version",
		TaskTitle:       "Add `main` function",
		TaskDescription: "Handle special chars: <>&\"'",
		Feedback:        "",
	}

	result, err := buildDeveloperPrompt(DefaultDeveloperPrompt, ctx)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// text/template does not escape by default, so special chars should be preserved
	if !strings.Contains(result, "--help") {
		t.Error("special characters in plan not preserved")
	}
	if !strings.Contains(result, "`main`") {
		t.Error("backticks in title not preserved")
	}
	if !strings.Contains(result, "<>&\"'") {
		t.Error("special HTML chars in description not preserved")
	}
}

func TestPromptWithMultilineContent(t *testing.T) {
	ctx := DeveloperContext{
		Plan: `Line 1
Line 2
Line 3`,
		TaskTitle:       "Multi-line task",
		TaskDescription: "Description\nwith\nnewlines",
		Feedback:        "",
	}

	result, err := buildDeveloperPrompt(DefaultDeveloperPrompt, ctx)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if !strings.Contains(result, "Line 1\nLine 2\nLine 3") {
		t.Error("multiline plan content not preserved")
	}
	if !strings.Contains(result, "Description\nwith\nnewlines") {
		t.Error("multiline description not preserved")
	}
}
