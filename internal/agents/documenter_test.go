package agents

import (
	"strings"
	"testing"

	"github.com/gerund/ralph/internal/config"
	"github.com/gerund/ralph/internal/db"
)

func TestDefaultDocumenterPrompt(t *testing.T) {
	if DefaultDocumenterPrompt == "" {
		t.Error("DefaultDocumenterPrompt should not be empty")
	}

	// Check that the prompt contains key sections
	expectedPhrases := []string{
		"documentation agent",
		"Changes Made",
		"Tasks Completed",
		"AGENTS.md",
		"README.md",
	}

	for _, phrase := range expectedPhrases {
		if !strings.Contains(DefaultDocumenterPrompt, phrase) {
			t.Errorf("DefaultDocumenterPrompt should contain %q", phrase)
		}
	}
}

func TestAgentBuilder_Documenter(t *testing.T) {
	cfg := config.DefaultConfig()
	builder := NewAgentBuilder(cfg)

	tasks := []*db.Task{
		{Title: "Task 1", Description: "Do task 1"},
		{Title: "Task 2", Description: "Do task 2"},
	}

	ctx := DocumenterContext{
		ChangesSummary: "Added new feature X",
		Tasks:          tasks,
	}

	agent, err := builder.Documenter(ctx)
	if err != nil {
		t.Fatalf("Documenter() failed: %v", err)
	}

	if agent == nil {
		t.Fatal("Documenter() returned nil agent")
	}

	if agent.Type != AgentTypeDocumenter {
		t.Errorf("expected agent type %s, got %s", AgentTypeDocumenter, agent.Type)
	}

	if agent.Prompt == "" {
		t.Error("agent prompt should not be empty")
	}

	// Verify context was templated into the prompt
	if !strings.Contains(agent.Prompt, "Added new feature X") {
		t.Error("prompt should contain changes summary")
	}
	if !strings.Contains(agent.Prompt, "Task 1") {
		t.Error("prompt should contain task title")
	}
	if !strings.Contains(agent.Prompt, "Do task 1") {
		t.Error("prompt should contain task description")
	}
}

func TestAgentBuilder_Documenter_EmptyContext(t *testing.T) {
	cfg := config.DefaultConfig()
	builder := NewAgentBuilder(cfg)

	ctx := DocumenterContext{
		ChangesSummary: "",
		Tasks:          nil,
	}

	agent, err := builder.Documenter(ctx)
	if err != nil {
		t.Fatalf("Documenter() failed: %v", err)
	}

	if agent == nil {
		t.Fatal("Documenter() returned nil agent")
	}

	// Should still produce a valid prompt
	if agent.Prompt == "" {
		t.Error("agent prompt should not be empty even with empty context")
	}
}

func TestBuildDocumenterPrompt(t *testing.T) {
	template := "Changes: {{.ChangesSummary}}\n{{range .Tasks}}- {{.Title}}\n{{end}}"

	tasks := []*db.Task{
		{Title: "Task A"},
		{Title: "Task B"},
	}

	ctx := DocumenterContext{
		ChangesSummary: "Summary",
		Tasks:          tasks,
	}

	result, err := buildDocumenterPrompt(template, ctx)
	if err != nil {
		t.Fatalf("buildDocumenterPrompt() failed: %v", err)
	}

	if !strings.Contains(result, "Changes: Summary") {
		t.Error("result should contain changes summary")
	}
	if !strings.Contains(result, "- Task A") {
		t.Error("result should contain Task A")
	}
	if !strings.Contains(result, "- Task B") {
		t.Error("result should contain Task B")
	}
}

func TestBuildDocumenterPrompt_InvalidTemplate(t *testing.T) {
	template := "{{.InvalidField}"

	ctx := DocumenterContext{}

	_, err := buildDocumenterPrompt(template, ctx)
	if err == nil {
		t.Error("expected error for invalid template")
	}
}

func TestDocumenterContext_Structure(t *testing.T) {
	tasks := []*db.Task{{Title: "Test"}}

	ctx := DocumenterContext{
		ChangesSummary: "changes",
		Tasks:          tasks,
	}

	if ctx.ChangesSummary != "changes" {
		t.Error("ChangesSummary field not set correctly")
	}
	if len(ctx.Tasks) != 1 {
		t.Error("Tasks field not set correctly")
	}
}

func TestAgentTypeDocumenter(t *testing.T) {
	if AgentTypeDocumenter != "documenter" {
		t.Errorf("expected AgentTypeDocumenter to be 'documenter', got %s", AgentTypeDocumenter)
	}
}

func TestManager_GetDocumenterAgent(t *testing.T) {
	cfg := config.DefaultConfig()
	manager := NewManager(cfg)

	tasks := []*db.Task{
		{Title: "Task", Description: "Desc"},
	}

	agent, err := manager.GetDocumenterAgent(nil, "changes summary", tasks)
	if err != nil {
		t.Fatalf("GetDocumenterAgent() failed: %v", err)
	}

	if agent == nil {
		t.Fatal("GetDocumenterAgent() returned nil")
	}

	if agent.Type != AgentTypeDocumenter {
		t.Errorf("expected type %s, got %s", AgentTypeDocumenter, agent.Type)
	}
}
