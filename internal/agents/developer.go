// Package agents provides agent orchestration and prompt management for Ralph.
package agents

// DefaultDeveloperPrompt is the embedded default prompt for the developer agent.
// It uses Go text/template syntax for variable substitution.
const DefaultDeveloperPrompt = `You are a developer agent implementing a task.

## Plan Context
{{.Plan}}

## Current Task
**Title:** {{.TaskTitle}}
**Description:** {{.TaskDescription}}

{{if .Feedback}}
## Previous Review Feedback
The reviewer provided this feedback on your last attempt:
{{.Feedback}}

Address all feedback items before completing.
{{end}}

## Guidelines
1. Focus on the specific task requirements
2. Follow existing patterns in the codebase
3. Do NOT run tests, builds, or linting - the reviewer handles that
4. Commit your changes when done

## Output
Summarize what you implemented when complete.
`

// DeveloperContext holds the context data for building a developer agent prompt.
type DeveloperContext struct {
	Plan            string // The full plan text
	TaskTitle       string // Title of the current task
	TaskDescription string // Description of the current task
	Feedback        string // Feedback from previous review (empty if first iteration)
}

// DeveloperAgent creates a developer agent with the given context.
// Deprecated: Use AgentBuilder.Developer instead for proper template rendering.
func DeveloperAgent(taskDescription string, feedback string) *Agent {
	return &Agent{
		Type:   AgentTypeDeveloper,
		Prompt: DefaultDeveloperPrompt,
	}
}

// buildDeveloperPrompt renders the developer prompt template with the given context.
func buildDeveloperPrompt(basePrompt string, ctx DeveloperContext) (string, error) {
	return executeTemplate(basePrompt, ctx)
}
