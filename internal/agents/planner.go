// Package agents provides agent orchestration and prompt management for Ralph.
package agents

// DefaultPlannerPrompt is the embedded default prompt for the planner agent.
// It uses Go text/template syntax for variable substitution.
const DefaultPlannerPrompt = `You are a planner agent. Your job is to break down a development plan into discrete tasks.

## Plan to Decompose
{{.PlanText}}

## Your Role

- Read and understand the overall plan
- Break it into discrete, implementable tasks
- Order tasks by dependencies

## Guidelines

1. Each task should be completable in one development session
2. Tasks should have clear acceptance criteria
3. Later tasks can depend on earlier ones
4. Keep tasks focused - one concern per task

## Output Format

Return a JSON array of tasks:

[
  {
    "title": "Short descriptive title",
    "description": "Detailed description of what to implement",
    "sequence": 1
  },
  ...
]

Order tasks by sequence number. Tasks with lower sequence numbers must be completed first.
`

// PlannerContext holds the context data for building a planner agent prompt.
type PlannerContext struct {
	PlanText string // The plan text to be decomposed into tasks
}

// PlannerAgent creates a planner agent with the given plan.
// Deprecated: Use AgentBuilder.Planner instead for proper template rendering.
func PlannerAgent(planText string) *Agent {
	return &Agent{
		Type:   AgentTypePlanner,
		Prompt: DefaultPlannerPrompt,
	}
}

// buildPlannerPrompt renders the planner prompt template with the given context.
func buildPlannerPrompt(basePrompt string, ctx PlannerContext) (string, error) {
	return executeTemplate(basePrompt, ctx)
}
