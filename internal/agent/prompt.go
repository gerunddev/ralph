// Package agent provides prompt construction for the Ralph V2 single-agent loop.
package agent

import (
	"bytes"
	"errors"
	"fmt"
	"strings"
	"text/template"
)

// ErrEmptyPlanContent is returned when PlanContent is empty or whitespace-only.
var ErrEmptyPlanContent = errors.New("PlanContent cannot be empty")

// PromptTemplate is the Go template for building the agent prompt.
// It includes static instructions and dynamic sections for plan, progress, and learnings.
const PromptTemplate = `# Instructions

You are an experienced software developer working iteratively on a plan.
You can wear many hats: developer, reviewer, architect, security engineer.

## Your Capabilities
- Critically evaluate your own code; don't stop until you're confident it's right
- Find and fix security and performance issues
- Maintain high standards for coding best practices in every language
- Break work into smaller units and determine execution order
- Track your progress and learnings about the codebase

## Output Format

Always output three sections with these exact headers, separated by horizontal rules:

## Progress
[What you've built, completed, current state]

## Learnings
[Insights about the codebase, patterns discovered, approaches that didn't work]

---

## Status
RUNNING RUNNING RUNNING

When you are completely done (in a review-only session with no file edits, where you find neither remaining work nor code review feedback to address), change the Status section to:

## Status
DONE DONE DONE!!!

If you edited files this cycle, you must do at least one more review cycle before signaling done.

---

# Plan

{{.PlanContent}}

---

# Progress So Far

{{if .Progress}}{{.Progress}}{{else}}No progress yet.{{end}}

---

# Learnings So Far

{{if .Learnings}}{{.Learnings}}{{else}}No learnings yet.{{end}}`

// promptTemplate is the pre-parsed template, initialized once at package load time.
// This avoids re-parsing the template on every call to BuildPrompt.
var promptTemplate = template.Must(template.New("agent-prompt").Parse(PromptTemplate))

// PromptContext holds the dynamic data for building the agent prompt.
type PromptContext struct {
	PlanContent string // The full plan text
	Progress    string // Current progress (empty string if none)
	Learnings   string // Current learnings (empty string if none)
}

// BuildPrompt constructs the full agent prompt from the given context.
// It renders the template with the provided plan, progress, and learnings.
//
// Whitespace-only strings for Progress and Learnings are treated as empty,
// triggering their respective fallback messages.
//
// Returns ErrEmptyPlanContent if PlanContent is empty or whitespace-only.
// Returns an error if template execution fails.
func BuildPrompt(ctx PromptContext) (string, error) {
	// Validate PlanContent is not empty or whitespace-only
	if strings.TrimSpace(ctx.PlanContent) == "" {
		return "", ErrEmptyPlanContent
	}

	// Normalize whitespace-only strings to empty to trigger fallbacks
	if strings.TrimSpace(ctx.Progress) == "" {
		ctx.Progress = ""
	}
	if strings.TrimSpace(ctx.Learnings) == "" {
		ctx.Learnings = ""
	}

	var buf bytes.Buffer
	if err := promptTemplate.Execute(&buf, ctx); err != nil {
		return "", fmt.Errorf("failed to execute prompt template: %w", err)
	}

	return buf.String(), nil
}
