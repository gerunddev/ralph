// Package agents provides agent orchestration and prompt management for Ralph.
package agents

import "github.com/gerund/ralph/internal/db"

// AgentTypeDocumenter identifies the documenter agent type.
const AgentTypeDocumenter AgentType = "documenter"

// DefaultDocumenterPrompt is the embedded default prompt for the documenter agent.
// It uses Go text/template syntax for variable substitution.
const DefaultDocumenterPrompt = `You are a documentation agent. Your job is to capture learnings from the development session.

## Changes Made
The following changes were made during this session:
{{.ChangesSummary}}

## Tasks Completed
{{range .Tasks}}
- {{.Title}}: {{.Description}}
{{end}}

## Your Tasks

1. **AGENTS.md Updates**: Review the code patterns established and document them for future Claude sessions. Focus on:
   - Coding conventions used
   - Architecture patterns
   - Testing approaches
   - Things to avoid

2. **README.md Updates**: Document user-facing changes:
   - New features added
   - Usage examples
   - Configuration changes

## Output Format

Provide two sections:

### AGENTS.md Content
` + "```markdown" + `
[content to append to AGENTS.md]
` + "```" + `

### README.md Content
` + "```markdown" + `
[content to append to README.md]
` + "```" + `

Be concise but comprehensive.
`

// DocumenterContext holds the context data for building a documenter agent prompt.
type DocumenterContext struct {
	ChangesSummary string     // Combined diff/changes from the session
	Tasks          []*db.Task // List of completed tasks
}

// Documenter creates a documenter agent with the given context.
func (b *AgentBuilder) Documenter(ctx DocumenterContext) (*Agent, error) {
	basePrompt, err := b.config.GetAgentPrompt("documenter")
	if err != nil {
		return nil, err
	}

	// Use default if no custom prompt configured.
	if basePrompt == "" {
		basePrompt = DefaultDocumenterPrompt
	}

	prompt, err := buildDocumenterPrompt(basePrompt, ctx)
	if err != nil {
		return nil, err
	}

	return &Agent{Type: AgentTypeDocumenter, Prompt: prompt}, nil
}

// buildDocumenterPrompt renders the documenter prompt template with the given context.
func buildDocumenterPrompt(basePrompt string, ctx DocumenterContext) (string, error) {
	return executeTemplate(basePrompt, ctx)
}
