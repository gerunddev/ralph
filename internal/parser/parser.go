// Package parser parses agent output to extract progress, learnings, and completion state.
package parser

import (
	"strings"

	"github.com/gerunddev/ralph/internal/log"
)

// DoneMarker is the exact string that indicates the agent is done.
const DoneMarker = "DONE DONE DONE!!!"

// Markers for the dual-agent loop.
const (
	DevDoneMarker          = "DEV_DONE DEV_DONE DEV_DONE!!!"
	ReviewerApprovedMarker = "REVIEWER_APPROVED REVIEWER_APPROVED!!!"
	ReviewerFeedbackPrefix = "REVIEWER_FEEDBACK:"
)

// ParseResult holds the result of parsing agent output.
type ParseResult struct {
	IsDone    bool   // True if the agent indicated completion
	Progress  string // Extracted progress content (empty if not found)
	Learnings string // Extracted learnings content (empty if not found)
	Status    string // Extracted status content (empty if not found)
	Raw       string // Original output
}

// AgentParseResult holds the result of parsing agent output.
type AgentParseResult struct {
	Progress  string // Extracted progress content
	Learnings string // Extracted learnings content
	Raw       string // Original output

	// Developer-specific
	DevDone bool // True if developer signaled DEV_DONE

	// Reviewer-specific
	ReviewerApproved bool   // True if reviewer approved
	ReviewerFeedback string // Feedback text if not approved
}

// Parse parses agent output to determine completion state or extract progress/learnings.
//
// The parser is lenient - it will extract what it can from malformed output.
// If the output is "DONE DONE DONE!!!" (trimmed), IsDone is true and other fields are empty.
// Otherwise, it looks for "## Progress" and "## Learnings" headers (case-insensitive).
// If no valid sections are found, the entire output is treated as Progress.
func Parse(output string) *ParseResult {
	result := &ParseResult{
		Raw: output,
	}

	trimmed := strings.TrimSpace(output)

	// Extract sections
	progress, foundProgress := extractSection(output, "## Progress")
	learnings, foundLearnings := extractSection(output, "## Learnings")
	status, foundStatus := extractSection(output, "## Status")

	result.Progress = progress
	result.Learnings = learnings
	result.Status = status

	// Check for done marker in status section first, then fallback to anywhere in output
	if foundStatus && containsDoneMarker(status) {
		result.IsDone = true
	} else if containsDoneMarker(trimmed) {
		// Backwards compatibility: check entire output for done marker
		result.IsDone = true
	}

	// If no recognized sections found, treat entire output as progress (malformed case)
	if !foundProgress && !foundLearnings && !foundStatus {
		if trimmed != "" {
			log.Warn("malformed agent output: no sections found, treating as progress",
				"output_length", len(output))
			result.Progress = trimmed
		}
	}

	return result
}

// extractSection extracts the content of a markdown section.
// It looks for a header like "## Progress" (case-insensitive) and extracts
// content until the next "##" header or end of string.
// Headers inside fenced code blocks are ignored.
// Returns the extracted content (trimmed) and whether the section was found.
func extractSection(output, header string) (string, bool) {
	// Mask code blocks to avoid matching headers inside them
	masked := maskCodeBlocks(output)

	// Case-insensitive search for the header
	lowerMasked := strings.ToLower(masked)
	lowerHeader := strings.ToLower(header)

	headerIdx := strings.Index(lowerMasked, lowerHeader)
	if headerIdx == -1 {
		return "", false
	}

	// Find the start of content (after the header line)
	contentStart := headerIdx + len(header)

	// Skip to end of header line
	newlineIdx := strings.Index(output[contentStart:], "\n")
	if newlineIdx == -1 {
		// Header at end of string, no content
		return "", true
	}
	contentStart += newlineIdx + 1

	// Find the next "##" header (not inside code block)
	remaining := output[contentStart:]
	maskedRemaining := masked[contentStart:]

	nextHeaderIdx := strings.Index(maskedRemaining, "\n##")
	if nextHeaderIdx == -1 {
		// Also check for "##" at start of remaining content
		if strings.HasPrefix(maskedRemaining, "##") {
			return "", true
		}
		// No next header, take everything
		return strings.TrimSpace(remaining), true
	}

	// Extract content up to next header
	content := remaining[:nextHeaderIdx]
	return strings.TrimSpace(content), true
}

// maskCodeBlocks replaces content inside fenced code blocks with spaces.
// This allows header detection to skip headers that appear inside code blocks.
// The returned string has the same length as input, preserving index positions.
func maskCodeBlocks(s string) string {
	result := []byte(s)
	i := 0

	for i < len(s) {
		// Look for opening fence (``` at start of line or start of string)
		if i == 0 || s[i-1] == '\n' {
			if strings.HasPrefix(s[i:], "```") {
				// Skip the opening ``` and any language identifier
				i += 3
				for i < len(s) && s[i] != '\n' {
					i++
				}
				if i < len(s) {
					i++ // skip the newline
				}

				// Find closing fence
				codeStart := i
				for i < len(s) {
					if (i == 0 || s[i-1] == '\n') && strings.HasPrefix(s[i:], "```") {
						// Mask from code start to before closing fence
						for j := codeStart; j < i; j++ {
							if result[j] != '\n' {
								result[j] = ' '
							}
						}
						i += 3 // skip closing ```
						// Skip rest of closing fence line
						for i < len(s) && s[i] != '\n' {
							i++
						}
						break
					}
					i++
				}
				// If no closing fence found, mask to end
				if i >= len(s) {
					for j := codeStart; j < len(s); j++ {
						if result[j] != '\n' {
							result[j] = ' '
						}
					}
				}
				continue
			}
		}
		i++
	}

	return string(result)
}

// containsDoneMarker checks if the input contains the done marker.
// The marker must not be followed by additional '!' characters to avoid
// false positives like "DONE DONE DONE!!!!" being matched.
func containsDoneMarker(s string) bool {
	idx := strings.Index(s, DoneMarker)
	if idx == -1 {
		return false
	}
	// Ensure marker isn't followed by another '!'
	afterIdx := idx + len(DoneMarker)
	if afterIdx < len(s) && s[afterIdx] == '!' {
		return false
	}
	return true
}

// containsMarker checks if input contains a marker, ensuring it's not followed by extra '!'.
func containsMarker(s, marker string) bool {
	idx := strings.Index(s, marker)
	if idx == -1 {
		return false
	}
	afterIdx := idx + len(marker)
	if afterIdx < len(s) && s[afterIdx] == '!' {
		return false
	}
	return true
}

// ParseAgentOutput parses output from a developer or reviewer agent.
// The agentType should be "developer" or "reviewer".
func ParseAgentOutput(output, agentType string) *AgentParseResult {
	result := &AgentParseResult{
		Raw: output,
	}

	// Extract common sections
	progress, foundProgress := extractSection(output, "## Progress")
	learnings, foundLearnings := extractSection(output, "## Learnings")
	status, _ := extractSection(output, "## Status")

	result.Progress = progress
	result.Learnings = learnings

	// If no recognized sections found, treat entire output as progress (malformed case)
	trimmed := strings.TrimSpace(output)
	if !foundProgress && !foundLearnings {
		if trimmed != "" {
			log.Warn("malformed agent output: no sections found, treating as progress",
				"agent_type", agentType, "output_length", len(output))
			result.Progress = trimmed
		}
	}

	switch agentType {
	case "developer":
		// Check for developer done marker in status section first, then anywhere
		if status != "" && containsMarker(status, DevDoneMarker) {
			result.DevDone = true
		} else if containsMarker(trimmed, DevDoneMarker) {
			result.DevDone = true
		}

	case "reviewer":
		// Check for reviewer approved marker in status/verdict section
		verdict, _ := extractSection(output, "### Verdict")
		if verdict != "" && containsMarker(verdict, ReviewerApprovedMarker) {
			result.ReviewerApproved = true
		} else if status != "" && containsMarker(status, ReviewerApprovedMarker) {
			result.ReviewerApproved = true
		} else if containsMarker(trimmed, ReviewerApprovedMarker) {
			result.ReviewerApproved = true
		}

		// Extract reviewer feedback if not approved
		if !result.ReviewerApproved {
			result.ReviewerFeedback = extractReviewerFeedback(output)
		}
	}

	return result
}

// extractReviewerFeedback extracts feedback from reviewer output.
// Looks for REVIEWER_FEEDBACK: prefix or extracts issue sections.
func extractReviewerFeedback(output string) string {
	// Check for explicit REVIEWER_FEEDBACK: prefix
	idx := strings.Index(output, ReviewerFeedbackPrefix)
	if idx != -1 {
		feedbackStart := idx + len(ReviewerFeedbackPrefix)
		// Extract until end of line or next section
		remaining := output[feedbackStart:]
		if newlineIdx := strings.Index(remaining, "\n##"); newlineIdx != -1 {
			return strings.TrimSpace(remaining[:newlineIdx])
		}
		return strings.TrimSpace(remaining)
	}

	// Otherwise, collect issue sections as feedback
	var feedback strings.Builder

	criticalIssues, foundCritical := extractSection(output, "### Critical Issues")
	majorIssues, foundMajor := extractSection(output, "### Major Issues")
	minorIssues, foundMinor := extractSection(output, "### Minor Issues")

	if foundCritical && criticalIssues != "" && strings.ToLower(criticalIssues) != "none" {
		feedback.WriteString("Critical Issues:\n")
		feedback.WriteString(criticalIssues)
		feedback.WriteString("\n\n")
	}
	if foundMajor && majorIssues != "" && strings.ToLower(majorIssues) != "none" {
		feedback.WriteString("Major Issues:\n")
		feedback.WriteString(majorIssues)
		feedback.WriteString("\n\n")
	}
	if foundMinor && minorIssues != "" && strings.ToLower(minorIssues) != "none" {
		feedback.WriteString("Minor Issues:\n")
		feedback.WriteString(minorIssues)
		feedback.WriteString("\n\n")
	}

	if feedback.Len() > 0 {
		return strings.TrimSpace(feedback.String())
	}

	// Fallback: use entire output as feedback if no structured sections found
	return strings.TrimSpace(output)
}
