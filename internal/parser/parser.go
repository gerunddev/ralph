// Package parser parses agent output to extract progress, learnings, and completion state.
package parser

import (
	"strings"

	"github.com/gerund/ralph/internal/log"
)

// DoneMarker is the exact string that indicates the agent is done.
const DoneMarker = "DONE DONE DONE!!!"

// ParseResult holds the result of parsing agent output.
type ParseResult struct {
	IsDone    bool   // True if the agent indicated completion
	Progress  string // Extracted progress content (empty if not found)
	Learnings string // Extracted learnings content (empty if not found)
	Status    string // Extracted status content (empty if not found)
	Raw       string // Original output
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
