// Package agents provides agent orchestration and prompt management for Ralph.
package agents

// DefaultReviewerPrompt is the embedded default prompt for the reviewer agent.
// It uses Go text/template syntax for variable substitution.
const DefaultReviewerPrompt = `You are a reviewer agent. Review the code changes for this task.

## Plan Context
{{.Plan}}

## Task Being Reviewed
**Title:** {{.TaskTitle}}
**Description:** {{.TaskDescription}}

## Changes to Review
` + "```diff" + `
{{.DiffOutput}}
` + "```" + `

## Review Process
1. Check if changes fulfill task requirements
2. Run tests and linting
3. Check for bugs, edge cases, security issues
4. Verify code style matches the project

## Output Format
End your review with exactly one of:
- APPROVED - if changes are complete and correct
- FEEDBACK: <detailed feedback> - if changes need work

Be specific about what needs to change.
`

// ReviewerContext holds the context data for building a reviewer agent prompt.
type ReviewerContext struct {
	Plan            string // The full plan text
	TaskTitle       string // Title of the task being reviewed
	TaskDescription string // Description of the task being reviewed
	DiffOutput      string // Diff output from jj show
}

// ReviewerAgent creates a reviewer agent with the given context.
// Deprecated: Use AgentBuilder.Reviewer instead for proper template rendering.
func ReviewerAgent(taskDescription string, diffOutput string) *Agent {
	return &Agent{
		Type:   AgentTypeReviewer,
		Prompt: DefaultReviewerPrompt,
	}
}

// buildReviewerPrompt renders the reviewer prompt template with the given context.
func buildReviewerPrompt(basePrompt string, ctx ReviewerContext) (string, error) {
	return executeTemplate(basePrompt, ctx)
}
