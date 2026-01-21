# Task 14: Learnings Capture

## Context

After project completion, Ralph should capture learnings into AGENTS.md (for future Claude sessions) and update README.md with functional changes. This matches README requirement for steps 5-6.

## Objective

Implement the learnings capture phase that updates project documentation based on what was built.

## Acceptance Criteria

- [ ] After final user feedback, prompt for learnings capture
- [ ] Run a documentation agent to analyze changes
- [ ] Generate/update AGENTS.md with patterns learned
- [ ] Generate/update README.md with feature documentation
- [ ] User can review and approve changes
- [ ] Changes committed via jj

## Implementation Details

### Documentation Agent

Create a new agent type for documentation:

```go
// internal/agents/documenter.go

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
\`\`\`markdown
[content to append to AGENTS.md]
\`\`\`

### README.md Content
\`\`\`markdown
[content to append to README.md]
\`\`\`

Be concise but comprehensive.
`

type DocumenterContext struct {
    ChangesSummary string
    Tasks          []*db.Task
}

func DocumenterAgent(ctx DocumenterContext) *Agent {
    prompt, _ := executeTemplate(DefaultDocumenterPrompt, ctx)
    return &Agent{
        Type:   AgentTypeDocumenter,
        Prompt: prompt,
    }
}
```

### Engine Method

```go
func (e *Engine) CaptureLearnings(ctx context.Context) error {
    e.emit(EngineEventCapturingLearnings, "Analyzing changes for documentation")

    // Get all completed tasks
    tasks, err := e.db.GetTasksByProject(e.project.ID)
    if err != nil {
        return err
    }

    completedTasks := filterCompleted(tasks)

    // Get combined diff
    changesSummary, err := e.getAllChanges(ctx)
    if err != nil {
        changesSummary = "Unable to retrieve changes"
    }

    // Create documenter agent
    agent := agents.DocumenterAgent(agents.DocumenterContext{
        ChangesSummary: changesSummary,
        Tasks:          completedTasks,
    })

    // Run Claude
    session, err := e.claude.Run(ctx, agent.Prompt, "")
    if err != nil {
        return err
    }

    // Collect output
    var output strings.Builder
    for event := range session.Events() {
        if event.Message != nil {
            output.WriteString(event.Message.Text)
        }
    }

    if err := session.Wait(); err != nil {
        return err
    }

    // Parse output
    learnings, err := parseLearningsOutput(output.String())
    if err != nil {
        return fmt.Errorf("failed to parse learnings: %w", err)
    }

    // Apply learnings
    if err := e.applyLearnings(learnings); err != nil {
        return err
    }

    // Commit changes
    if err := e.jj.Describe(ctx, "docs: capture learnings from development session"); err != nil {
        return err
    }

    e.emit(EngineEventLearningsCaptured, "Documentation updated")
    return nil
}
```

### Parse Learnings Output

```go
type LearningsOutput struct {
    AgentsMD  string
    ReadmeMD  string
}

func parseLearningsOutput(output string) (*LearningsOutput, error) {
    learnings := &LearningsOutput{}

    // Find AGENTS.md content
    agentsStart := strings.Index(output, "### AGENTS.md Content")
    readmeStart := strings.Index(output, "### README.md Content")

    if agentsStart != -1 && readmeStart != -1 {
        agentsSection := output[agentsStart:readmeStart]
        learnings.AgentsMD = extractCodeBlock(agentsSection)

        readmeSection := output[readmeStart:]
        learnings.ReadmeMD = extractCodeBlock(readmeSection)
    }

    return learnings, nil
}

func extractCodeBlock(text string) string {
    start := strings.Index(text, "```markdown")
    if start == -1 {
        start = strings.Index(text, "```")
    }
    if start == -1 {
        return ""
    }

    start = strings.Index(text[start:], "\n") + start + 1
    end := strings.Index(text[start:], "```")
    if end == -1 {
        return ""
    }

    return strings.TrimSpace(text[start : start+end])
}
```

### Apply Learnings

```go
func (e *Engine) applyLearnings(learnings *LearningsOutput) error {
    if learnings.AgentsMD != "" {
        if err := appendToFile("AGENTS.md", learnings.AgentsMD); err != nil {
            return fmt.Errorf("failed to update AGENTS.md: %w", err)
        }
    }

    if learnings.ReadmeMD != "" {
        if err := appendToFile("README.md", learnings.ReadmeMD); err != nil {
            return fmt.Errorf("failed to update README.md: %w", err)
        }
    }

    return nil
}

func appendToFile(path string, content string) error {
    f, err := os.OpenFile(path, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
    if err != nil {
        return err
    }
    defer f.Close()

    // Add separator
    separator := fmt.Sprintf("\n\n---\n\n## Session: %s\n\n", time.Now().Format("2006-01-02"))

    if _, err := f.WriteString(separator + content); err != nil {
        return err
    }

    return nil
}
```

### Get All Changes

```go
func (e *Engine) getAllChanges(ctx context.Context) (string, error) {
    // Get diff from the first task's change to current
    // This requires knowing the base revision

    // Simple approach: jj log with patch
    output, err := e.jj.Log(ctx, "--patch")
    if err != nil {
        return "", err
    }

    return output, nil
}
```

### TUI Integration

Add learnings phase to the flow:

```go
type ViewState int

const (
    ViewProjectList ViewState = iota
    ViewTaskProgress
    ViewUserFeedback
    ViewCapturingLearnings
    ViewCompleted
)

func (m MainModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
    switch msg := msg.(type) {
    case UserFeedbackSkippedMsg, FeedbackCycleCompleteMsg:
        // Move to learnings capture
        if m.userFeedback.skipped || !hasMoreFeedback {
            m.state = ViewCapturingLearnings
            return m, m.captureLearnings()
        }

    case LearningsCapturedMsg:
        m.state = ViewCompleted
        return m, nil
    }
    // ...
}
```

## Files to Modify

- `internal/agents/documenter.go` - Create with documenter agent
- `internal/engine/engine.go` - Add CaptureLearnings method
- `internal/jj/jj.go` - Add Log method if needed
- `internal/tui/app.go` - Add learnings phase

## Testing Strategy

1. **Parse tests** - Learnings output parsing
2. **File tests** - Append to file operations
3. **Integration** - Full learnings flow

## Dependencies

- `internal/agents` - Documenter agent
- `internal/claude` - Run documentation session
- `internal/jj` - Get change history

## Notes

- AGENTS.md is for future Claude sessions in this codebase
- README.md updates should be user-facing documentation
- Consider making this step optional via config
- Content is appended, not overwritten
- Each session adds a dated section
