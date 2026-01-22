// Package distill provides commit message distillation using Claude Haiku.
package distill

import (
	"context"
	"fmt"
	"strings"

	"github.com/gerund/ralph/internal/claude"
	"github.com/gerund/ralph/internal/parser"
)

// DefaultModel is the default model to use for distillation.
// Haiku is fast and cheap, ideal for simple text transformation.
const DefaultModel = "haiku"

// FallbackMessage is returned when distillation fails or produces no output.
const FallbackMessage = "wip: work in progress"

// distillationPrompt is the prompt template for commit message generation.
const distillationPrompt = `You are a commit message writer. Given the following development session and code diff, write a concise git commit message.

Rules:
- Use conventional commit format (feat:, fix:, refactor:, chore:, docs:, test:)
- Keep the first line under 72 characters
- Be specific about what changed based on the DIFF, not the session output
- Focus on the "what" and "why", not the "how"
- Do not mention AI, Claude, automation, or robots

Session context:
%s

Code diff:
%s

Respond with only the commit message, nothing else.`

// Distiller generates commit messages from session output using Claude Haiku.
type Distiller struct {
	client *claude.Client
}

// NewDistiller creates a new Distiller with a pre-configured Claude client.
// The client should be configured for the Haiku model with maxTurns=1.
func NewDistiller(client *claude.Client) *Distiller {
	return &Distiller{
		client: client,
	}
}

// NewDistillerWithDefaults creates a new Distiller with default Haiku configuration.
func NewDistillerWithDefaults() *Distiller {
	client := claude.NewClient(claude.ClientConfig{
		Model:    DefaultModel,
		MaxTurns: 1,
		Verbose:  false,
	})
	return NewDistiller(client)
}

// Distill takes session output and diff, returns a concise commit message.
// It handles special cases:
// - Empty output returns a generic message
// - "DONE DONE DONE!!!" returns "Complete implementation"
// - Errors return a fallback message
func (d *Distiller) Distill(ctx context.Context, sessionOutput string, diff string) (string, error) {
	// Handle empty output and empty diff
	if strings.TrimSpace(sessionOutput) == "" && strings.TrimSpace(diff) == "" {
		return FallbackMessage, nil
	}

	// Handle the "done" marker specially - use contains logic to match parser behavior
	if containsDoneMarker(sessionOutput) {
		return "Complete implementation", nil
	}

	// Build the prompt
	prompt := fmt.Sprintf(distillationPrompt, sessionOutput, diff)

	// Run Claude session
	session, err := d.client.Run(ctx, prompt)
	if err != nil {
		// Return fallback on error, but also return the error for logging
		return FallbackMessage, fmt.Errorf("failed to start distillation: %w", err)
	}

	// Collect all text from message events
	var output strings.Builder
	for event := range session.Events() {
		if event.Type == claude.EventMessage && event.Message != nil {
			output.WriteString(event.Message.Text)
		}
	}

	// Wait for session to complete
	if err := session.Wait(); err != nil {
		// Return whatever we got plus the error
		if output.Len() > 0 {
			return cleanMessage(output.String()), fmt.Errorf("session error: %w", err)
		}
		return FallbackMessage, fmt.Errorf("session error: %w", err)
	}

	// Clean and return the message
	result := cleanMessage(output.String())
	if result == "" {
		return FallbackMessage, nil
	}

	return result, nil
}

// cleanMessage normalizes a commit message by:
// - Trimming whitespace
// - Taking only the first line if multiple lines
func cleanMessage(msg string) string {
	msg = strings.TrimSpace(msg)
	if msg == "" {
		return ""
	}

	// Take only the first non-empty line (the summary line)
	lines := strings.Split(msg, "\n")
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line != "" {
			return line
		}
	}

	return ""
}

// containsDoneMarker checks if the input contains the done marker.
// The marker must not be followed by additional '!' characters to avoid
// false positives like "DONE DONE DONE!!!!" being matched.
func containsDoneMarker(s string) bool {
	idx := strings.Index(s, parser.DoneMarker)
	if idx == -1 {
		return false
	}
	// Ensure marker isn't followed by another '!'
	afterIdx := idx + len(parser.DoneMarker)
	if afterIdx < len(s) && s[afterIdx] == '!' {
		return false
	}
	return true
}
