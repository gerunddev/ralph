# Task 5: Agent Prompt Builder

## Objective

Create the prompt builder that constructs the full prompt for each iteration from instructions, plan, progress, and learnings.

## Requirements

1. Static instructions section (the agent persona and output format)
2. Dynamic sections: plan content, current progress, current learnings
3. Template-based construction
4. Handle empty progress/learnings gracefully

## Prompt Template

```
# Instructions

You are an experienced software developer working iteratively on a plan.
You can wear many hats: developer, reviewer, architect, security engineer.

## Your Capabilities
- Critically evaluate your own code; don't stop until you're confident it's right
- Find and fix security and performance issues
- Maintain high standards for coding best practices in every language
- Break work into smaller units and determine execution order
- Track your progress and learnings about the codebase

## Output Format

You MUST output one of two things:

### Option A: When you believe you're completely done
Output exactly this and nothing else:
DONE DONE DONE!!!

### Option B: When there's more work to do
Output two sections with these exact headers:

## Progress
[What you've built, completed, current state]

## Learnings
[Insights about the codebase, patterns discovered, approaches that didn't work]

---

# Plan

{{.PlanContent}}

---

# Progress So Far

{{if .Progress}}{{.Progress}}{{else}}No progress yet.{{end}}

---

# Learnings So Far

{{if .Learnings}}{{.Learnings}}{{else}}No learnings yet.{{end}}
```

## Interface

```go
type PromptContext struct {
    PlanContent string
    Progress    string // Empty string if none
    Learnings   string // Empty string if none
}

func BuildPrompt(ctx PromptContext) (string, error)
```

## Acceptance Criteria

- [ ] Renders complete prompt with all sections
- [ ] Handles empty progress (shows "No progress yet.")
- [ ] Handles empty learnings (shows "No learnings yet.")
- [ ] Template errors return clear error messages
- [ ] Instructions are clear and complete
- [ ] Unit tests for various input combinations

## Files to Create

- `internal/agent/prompt.go`
- `internal/agent/prompt_test.go`

## Notes

The instructions should be refined based on testing. Key points:
- Agent must output EXACTLY "DONE DONE DONE!!!" (nothing before or after) when done
- Progress/Learnings sections must have exact headers for parsing
- Agent should understand it will be called repeatedly with updated context
