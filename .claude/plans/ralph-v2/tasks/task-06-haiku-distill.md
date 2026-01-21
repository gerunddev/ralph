# Task 6: Haiku Commit Message Distillation

## Objective

Create a small prompt that asks Claude Haiku to distill a session's output into a concise commit message.

## Requirements

1. Use Haiku model (fast, cheap)
2. Input: The session's final output (progress + learnings or done message)
3. Output: A clean, conventional commit message
4. No mention of Claude, AI, robots, or automation
5. Simple, direct style

## Distillation Prompt

```
You are a commit message writer. Given the following development session output, write a concise git commit message.

Rules:
- Use conventional commit format if appropriate (feat:, fix:, refactor:, etc.)
- Keep the first line under 72 characters
- Be specific about what changed
- Do not mention AI, Claude, automation, or robots
- Write as if a human developer made these changes

Session output:
{{.SessionOutput}}

Respond with only the commit message, nothing else.
```

## Interface

```go
type Distiller struct {
    client *claude.Client // Configured for Haiku
}

func NewDistiller(client *claude.Client) *Distiller

func (d *Distiller) Distill(ctx context.Context, sessionOutput string) (string, error)
```

## Acceptance Criteria

- [ ] Calls Claude with Haiku model
- [ ] Returns clean commit message (trimmed, no extra text)
- [ ] Handles "DONE DONE DONE!!!" output gracefully (e.g., "Complete implementation")
- [ ] Handles empty output gracefully
- [ ] Falls back to generic message on error (e.g., "Update implementation")
- [ ] Unit tests with mocked Claude client

## Files to Create

- `internal/distill/distill.go`
- `internal/distill/distill_test.go`

## Notes

The Haiku model name may vary. Check current Claude CLI documentation. Likely:
- `claude-3-haiku-20240307` or similar

This should be a quick, cheap call - not a full agentic session. Use minimal max_turns (1).

Consider: Should we also include iteration number or plan name in the prompt for better context?
