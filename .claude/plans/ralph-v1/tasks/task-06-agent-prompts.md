# Task 6: Agent Prompt Construction

## Context

Ralph uses three agent types (developer, reviewer, planner) with customizable prompts. The stub files exist with default prompts, but the prompt construction with context injection is incomplete. The config package already handles loading custom prompts from files.

## Objective

Implement agent prompt construction that injects task context, plan text, feedback, and diff output into agent prompts.

## Acceptance Criteria

- [ ] `DeveloperAgent(plan, task, feedback)` builds prompt with all context
- [ ] `ReviewerAgent(plan, task, diffOutput)` builds prompt with diff for review
- [ ] `PlannerAgent(planText)` builds prompt for task decomposition
- [ ] Agent manager loads prompts from config (custom or default)
- [ ] Prompts use clear section markers for context injection
- [ ] Unit tests verify context is properly injected
- [ ] Prompt templates support variable substitution

## Implementation Details

### Enhanced Default Prompts

```go
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
```

### Agent Builder Pattern

```go
type AgentBuilder struct {
    config *config.Config
}

func NewAgentBuilder(cfg *config.Config) *AgentBuilder {
    return &AgentBuilder{config: cfg}
}

func (b *AgentBuilder) Developer(ctx DeveloperContext) (*Agent, error) {
    basePrompt, err := b.config.GetAgentPrompt("developer")
    if err != nil {
        return nil, err
    }

    prompt, err := executeTemplate(basePrompt, ctx)
    if err != nil {
        return nil, err
    }

    return &Agent{Type: AgentTypeDeveloper, Prompt: prompt}, nil
}

type DeveloperContext struct {
    Plan            string
    TaskTitle       string
    TaskDescription string
    Feedback        string  // Empty if first iteration
}
```

### Template Execution

Use Go's `text/template` for variable substitution:

```go
func executeTemplate(tmpl string, data interface{}) (string, error) {
    t, err := template.New("prompt").Parse(tmpl)
    if err != nil {
        return "", fmt.Errorf("failed to parse template: %w", err)
    }

    var buf bytes.Buffer
    if err := t.Execute(&buf, data); err != nil {
        return "", fmt.Errorf("failed to execute template: %w", err)
    }

    return buf.String(), nil
}
```

### Manager Integration

```go
type Manager struct {
    builder *AgentBuilder
}

func NewManager(cfg *config.Config) *Manager {
    return &Manager{
        builder: NewAgentBuilder(cfg),
    }
}

func (m *Manager) GetDeveloperAgent(ctx context.Context, plan string, task *db.Task, feedback string) (*Agent, error) {
    return m.builder.Developer(DeveloperContext{
        Plan:            plan,
        TaskTitle:       task.Title,
        TaskDescription: task.Description,
        Feedback:        feedback,
    })
}
```

### Reviewer Context

```go
type ReviewerContext struct {
    Plan            string
    TaskTitle       string
    TaskDescription string
    DiffOutput      string  // From jj show
}
```

The reviewer prompt includes the diff so it can review the actual changes:

```go
const DefaultReviewerPrompt = `You are a reviewer agent. Review the code changes for this task.

## Plan Context
{{.Plan}}

## Task Being Reviewed
**Title:** {{.TaskTitle}}
**Description:** {{.TaskDescription}}

## Changes to Review
\`\`\`diff
{{.DiffOutput}}
\`\`\`

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
```

## Files to Modify

- `internal/agents/agents.go` - Manager and builder
- `internal/agents/developer.go` - Enhanced prompt and context
- `internal/agents/reviewer.go` - Enhanced prompt and context
- `internal/agents/planner.go` - Enhanced prompt
- `internal/agents/template.go` - Create for template utilities
- `internal/agents/agents_test.go` - Create with tests

## Testing Strategy

1. **Template tests** - Verify context injection
2. **Edge cases** - Empty feedback, missing optional fields
3. **Custom prompt loading** - Verify config integration

## Dependencies

- `internal/config` - For loading custom prompts
- `internal/db` - For Task model

## Notes

- Custom prompts from config should also support template variables
- The planner needs to return JSON for task parsing (existing format is good)
- Reviewer must output APPROVED or FEEDBACK: for parsing in Task 7
