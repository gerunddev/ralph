// Package agents provides agent orchestration and prompt management for Ralph.
package agents

import (
	"context"
	"fmt"

	"github.com/gerund/ralph/internal/config"
	"github.com/gerund/ralph/internal/db"
)

// AgentType represents the type of agent.
type AgentType string

const (
	AgentTypeDeveloper AgentType = "developer"
	AgentTypeReviewer  AgentType = "reviewer"
	AgentTypePlanner   AgentType = "planner"
)

// Agent represents an agent that can execute tasks.
type Agent struct {
	Type   AgentType
	Prompt string
}

// AgentBuilder constructs agents with proper prompt templating.
type AgentBuilder struct {
	config *config.Config
}

// NewAgentBuilder creates a new agent builder with the given config.
func NewAgentBuilder(cfg *config.Config) *AgentBuilder {
	return &AgentBuilder{config: cfg}
}

// Developer creates a developer agent with the given context.
func (b *AgentBuilder) Developer(ctx DeveloperContext) (*Agent, error) {
	basePrompt, err := b.config.GetAgentPrompt("developer")
	if err != nil {
		return nil, fmt.Errorf("failed to get developer prompt: %w", err)
	}

	// Use default if no custom prompt configured.
	if basePrompt == "" {
		basePrompt = DefaultDeveloperPrompt
	}

	prompt, err := buildDeveloperPrompt(basePrompt, ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to build developer prompt: %w", err)
	}

	return &Agent{Type: AgentTypeDeveloper, Prompt: prompt}, nil
}

// Reviewer creates a reviewer agent with the given context.
func (b *AgentBuilder) Reviewer(ctx ReviewerContext) (*Agent, error) {
	basePrompt, err := b.config.GetAgentPrompt("reviewer")
	if err != nil {
		return nil, fmt.Errorf("failed to get reviewer prompt: %w", err)
	}

	// Use default if no custom prompt configured.
	if basePrompt == "" {
		basePrompt = DefaultReviewerPrompt
	}

	prompt, err := buildReviewerPrompt(basePrompt, ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to build reviewer prompt: %w", err)
	}

	return &Agent{Type: AgentTypeReviewer, Prompt: prompt}, nil
}

// Planner creates a planner agent with the given context.
func (b *AgentBuilder) Planner(ctx PlannerContext) (*Agent, error) {
	basePrompt, err := b.config.GetAgentPrompt("planner")
	if err != nil {
		return nil, fmt.Errorf("failed to get planner prompt: %w", err)
	}

	// Use default if no custom prompt configured.
	if basePrompt == "" {
		basePrompt = DefaultPlannerPrompt
	}

	prompt, err := buildPlannerPrompt(basePrompt, ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to build planner prompt: %w", err)
	}

	return &Agent{Type: AgentTypePlanner, Prompt: prompt}, nil
}

// Manager manages agent creation and execution.
type Manager struct {
	builder *AgentBuilder
	config  *config.Config
}

// NewManager creates a new agent manager with the given config.
func NewManager(cfg *config.Config) *Manager {
	return &Manager{
		builder: NewAgentBuilder(cfg),
		config:  cfg,
	}
}

// GetDeveloperAgent returns a developer agent configured for the given task.
func (m *Manager) GetDeveloperAgent(ctx context.Context, plan string, task *db.Task, feedback string) (*Agent, error) {
	return m.builder.Developer(DeveloperContext{
		Plan:            plan,
		TaskTitle:       task.Title,
		TaskDescription: task.Description,
		Feedback:        feedback,
	})
}

// GetReviewerAgent returns a reviewer agent configured for reviewing the given task.
func (m *Manager) GetReviewerAgent(ctx context.Context, plan string, task *db.Task, diffOutput string) (*Agent, error) {
	return m.builder.Reviewer(ReviewerContext{
		Plan:            plan,
		TaskTitle:       task.Title,
		TaskDescription: task.Description,
		DiffOutput:      diffOutput,
	})
}

// GetPlannerAgent returns a planner agent configured for decomposing the given plan.
func (m *Manager) GetPlannerAgent(ctx context.Context, planText string) (*Agent, error) {
	return m.builder.Planner(PlannerContext{
		PlanText: planText,
	})
}

// GetDocumenterAgent returns a documenter agent configured for capturing learnings.
func (m *Manager) GetDocumenterAgent(ctx context.Context, changesSummary string, tasks []*db.Task) (*Agent, error) {
	return m.builder.Documenter(DocumenterContext{
		ChangesSummary: changesSummary,
		Tasks:          tasks,
	})
}

// GetAgent returns the agent for the given type with no context.
// Deprecated: Use specific Get*Agent methods instead.
func (m *Manager) GetAgent(ctx context.Context, agentType AgentType) (*Agent, error) {
	switch agentType {
	case AgentTypeDeveloper:
		return m.builder.Developer(DeveloperContext{})
	case AgentTypeReviewer:
		return m.builder.Reviewer(ReviewerContext{})
	case AgentTypePlanner:
		return m.builder.Planner(PlannerContext{})
	default:
		return nil, fmt.Errorf("unknown agent type: %s", agentType)
	}
}

// Config returns the manager's configuration.
func (m *Manager) Config() *config.Config {
	return m.config
}
