// Package agent provides prompt construction for Ralph.
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

## Version Control

This project uses Jujutsu (jj) for version control, NOT git. Even if there is a colocated .git directory, you MUST use jj commands instead of git commands:
- Use ` + "`jj diff`" + ` instead of ` + "`git diff`" + `
- Use ` + "`jj show`" + ` instead of ` + "`git show`" + `
- Use ` + "`jj log`" + ` instead of ` + "`git log`" + `
- Use ` + "`jj status`" + ` instead of ` + "`git status`" + `

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

// DeveloperContext holds context for developer agent prompts.
type DeveloperContext struct {
	PlanContent      string // The full plan text
	Progress         string // Current progress (empty string if none)
	Learnings        string // Current learnings (empty string if none)
	ReviewerFeedback string // Feedback from last review rejection (empty if none)
}

// ReviewerContext holds context for reviewer agent prompts.
type ReviewerContext struct {
	PlanContent      string // The full plan text
	Progress         string // Current progress (empty string if none)
	Learnings        string // Current learnings (empty string if none)
	DiffOutput       string // Output from jj show (the changes to review)
	DeveloperSummary string // Developer's output text for context
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

// DeveloperPromptTemplate is the template for developer agent prompts.
const DeveloperPromptTemplate = `# Instructions

You are an experienced software developer working iteratively on a plan.
You can wear many hats: developer, reviewer, architect, security engineer.

## Version Control

This project uses Jujutsu (jj) for version control, NOT git. Even if there is a colocated .git directory, you MUST use jj commands instead of git commands:
- Use ` + "`jj diff`" + ` instead of ` + "`git diff`" + `
- Use ` + "`jj show`" + ` instead of ` + "`git show`" + `
- Use ` + "`jj log`" + ` instead of ` + "`git log`" + `
- Use ` + "`jj status`" + ` instead of ` + "`git status`" + `

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
DEV_DONE DEV_DONE DEV_DONE!!!

If you edited files this cycle, you must do at least one more review cycle before signaling done.

---

# Plan

{{.PlanContent}}

---

# Progress So Far

{{if .Progress}}{{.Progress}}{{else}}No progress yet.{{end}}

---

# Learnings So Far

{{if .Learnings}}{{.Learnings}}{{else}}No learnings yet.{{end}}
{{if .ReviewerFeedback}}
---

# Reviewer Feedback (from last review - MUST ADDRESS)

The reviewer rejected your previous work. You MUST address all the following issues:

{{.ReviewerFeedback}}
{{end}}`

// ReviewerPromptTemplate is the template for reviewer agent prompts.
const ReviewerPromptTemplate = `# Instructions

You are a VERY HARD CRITIC code reviewer.

## Important

The diff section below shows the cumulative changes made during this development session. If the diff appears incomplete, incorrect, or you need to understand the context of changes better, you MAY use jj commands to examine the history:
- ` + "`jj log`" + ` to see the commit history
- ` + "`jj show <change-id>`" + ` to see the diff for a specific change
- ` + "`jj diff --from <change-id> --to <change-id>`" + ` to see changes between specific points

If the diff section shows "No code changes to review" then no code was modified and you should approve based on the developer's analysis in the Developer Summary section.

You will ONLY approve code that meets ALL of the following criteria:
- Zero critical issues (security vulnerabilities, crashes, data loss, race conditions)
- Zero major issues (bugs, incorrect logic, missing error handling, broken functionality)
- Zero minor issues (style violations, unclear naming, missing comments, dead code)

Your job is to be EXTREMELY thorough. If you miss an issue, it goes to production.

## Review Checklist

For EACH file in the diff, check:
1. **Correctness** - Does the code do what it's supposed to? Does it match the plan?
2. **Edge Cases** - Are all edge cases handled? Empty inputs, nil values, boundary conditions?
3. **Error Handling** - Are all errors handled appropriately? No swallowed errors?
4. **Security** - Any injection risks? Improper input validation? Sensitive data exposure?
5. **Performance** - Any N+1 queries? Unnecessary allocations? Unbounded loops?
6. **Tests** - Are there tests? Do they cover the happy path AND edge cases?
7. **Style** - Consistent with the codebase? Clear naming? Appropriate comments?
8. **Documentation** - Are public APIs documented? Complex logic explained?

## Output Format

Always output three sections with these exact headers:

## Progress
[Summary of what you reviewed]

## Learnings
[Patterns you noticed, potential systemic issues]

---

Then output your review findings:

### Critical Issues
[List each critical issue with file:line reference, or "None"]

### Major Issues
[List each major issue with file:line reference, or "None"]

### Minor Issues
[List each minor issue with file:line reference, or "None"]

### Verdict

If ALL issue lists above are exactly "None":
REVIEWER_APPROVED REVIEWER_APPROVED!!!

Otherwise:
REVIEWER_FEEDBACK: [Summarize what needs to be fixed]

---

# Plan (for context)

{{.PlanContent}}

---

# Progress So Far

{{if .Progress}}{{.Progress}}{{else}}No progress yet.{{end}}

---

# Learnings So Far

{{if .Learnings}}{{.Learnings}}{{else}}No learnings yet.{{end}}

---

# Developer Summary

{{if .DeveloperSummary}}{{.DeveloperSummary}}{{else}}No developer summary available.{{end}}

---

# Diff to Review

{{if .DiffOutput}}` + "```diff" + `
{{.DiffOutput}}
` + "```" + `{{else}}No code changes to review. The developer completed analysis/investigation without modifying any files. Review the Developer Summary section above to verify the developer's conclusions are sound.{{end}}`

// developerTemplate is the pre-parsed developer template.
var developerTemplate = template.Must(template.New("developer-prompt").Parse(DeveloperPromptTemplate))

// reviewerTemplate is the pre-parsed reviewer template.
var reviewerTemplate = template.Must(template.New("reviewer-prompt").Parse(ReviewerPromptTemplate))

// BuildDeveloperPrompt constructs the developer agent prompt.
func BuildDeveloperPrompt(ctx DeveloperContext) (string, error) {
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
	if strings.TrimSpace(ctx.ReviewerFeedback) == "" {
		ctx.ReviewerFeedback = ""
	}

	var buf bytes.Buffer
	if err := developerTemplate.Execute(&buf, ctx); err != nil {
		return "", fmt.Errorf("failed to execute developer prompt template: %w", err)
	}

	return buf.String(), nil
}

// BuildReviewerPrompt constructs the reviewer agent prompt.
func BuildReviewerPrompt(ctx ReviewerContext) (string, error) {
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
	if strings.TrimSpace(ctx.DiffOutput) == "" {
		ctx.DiffOutput = ""
	}
	if strings.TrimSpace(ctx.DeveloperSummary) == "" {
		ctx.DeveloperSummary = ""
	}

	var buf bytes.Buffer
	if err := reviewerTemplate.Execute(&buf, ctx); err != nil {
		return "", fmt.Errorf("failed to execute reviewer prompt template: %w", err)
	}

	return buf.String(), nil
}
