// Package loop provides the main execution loop for Ralph.
package loop

import "github.com/gerunddev/ralph/internal/claude"

// EventType represents the type of a loop event.
type EventType string

const (
	// EventStarted is emitted when the loop starts.
	EventStarted EventType = "started"
	// EventIterationStart is emitted at the start of each iteration.
	EventIterationStart EventType = "iteration_start"
	// EventPromptBuilt is emitted when the prompt is built, with full prompt content.
	EventPromptBuilt EventType = "prompt_built"
	// EventClaudeStart is emitted when a Claude session starts.
	EventClaudeStart EventType = "claude_start"
	// EventClaudeStream wraps Claude streaming events.
	EventClaudeStream EventType = "claude_stream"
	// EventClaudeOutput is emitted with the final collected output text.
	EventClaudeOutput EventType = "claude_output"
	// EventClaudeEnd is emitted when a Claude session ends.
	EventClaudeEnd EventType = "claude_end"
	// EventParsed is emitted after output is parsed.
	EventParsed EventType = "parsed"
	// EventIterationEnd is emitted at the end of each iteration.
	EventIterationEnd EventType = "iteration_end"
	// EventDone is emitted when the agent says it's done.
	EventDone EventType = "done"
	// EventMaxIterations is emitted when max iterations is reached.
	EventMaxIterations EventType = "max_iterations"
	// EventError is emitted when an error occurs.
	EventError EventType = "error"

	// EventDeveloperStart is emitted when the developer agent starts.
	EventDeveloperStart EventType = "developer_start"
	// EventDeveloperEnd is emitted when the developer agent ends.
	EventDeveloperEnd EventType = "developer_end"
	// EventDeveloperDone is emitted when the developer signals DEV_DONE.
	EventDeveloperDone EventType = "developer_done"
	// EventReviewerStart is emitted when the reviewer agent starts.
	EventReviewerStart EventType = "reviewer_start"
	// EventReviewerEnd is emitted when the reviewer agent ends.
	EventReviewerEnd EventType = "reviewer_end"
	// EventReviewerApproved is emitted when the reviewer approves.
	EventReviewerApproved EventType = "reviewer_approved"
	// EventReviewerFeedback is emitted when the reviewer provides feedback (rejection).
	EventReviewerFeedback EventType = "reviewer_feedback"
	// EventBothDone is emitted when both developer and reviewer signal done.
	EventBothDone EventType = "both_done"
	// EventContextLimit is emitted when the context window usage exceeds the limit.
	EventContextLimit EventType = "context_limit"
	// EventExtremeModeTriggered is emitted when extreme mode activates +3 iterations.
	EventExtremeModeTriggered EventType = "extreme_mode_triggered"
)

// Event represents an event emitted by the loop.
type Event struct {
	Type        EventType
	Iteration   int
	MaxIter     int
	Message     string
	Prompt      string              // For EventPromptBuilt events (full prompt content)
	Output      string              // For EventClaudeOutput events (final collected output)
	ClaudeEvent *claude.StreamEvent // For EventClaudeStream events
	Error       error
	TeamMode    bool // Whether team mode is active (for EventDeveloperStart)
}

// NewEvent creates a new loop event with the given type and message.
func NewEvent(t EventType, iter, maxIter int, msg string) Event {
	return Event{
		Type:      t,
		Iteration: iter,
		MaxIter:   maxIter,
		Message:   msg,
	}
}

// NewErrorEvent creates a new error event.
func NewErrorEvent(iter, maxIter int, err error) Event {
	return Event{
		Type:      EventError,
		Iteration: iter,
		MaxIter:   maxIter,
		Error:     err,
		Message:   err.Error(),
	}
}

// NewClaudeStreamEvent creates a new Claude stream event wrapper.
func NewClaudeStreamEvent(iter, maxIter int, claudeEvent *claude.StreamEvent) Event {
	return Event{
		Type:        EventClaudeStream,
		Iteration:   iter,
		MaxIter:     maxIter,
		ClaudeEvent: claudeEvent,
	}
}

// NewPromptBuiltEvent creates a new prompt built event with the full prompt content.
func NewPromptBuiltEvent(iter, maxIter int, prompt string) Event {
	return Event{
		Type:      EventPromptBuilt,
		Iteration: iter,
		MaxIter:   maxIter,
		Prompt:    prompt,
		Message:   "Prompt built",
	}
}

// NewClaudeOutputEvent creates a new event with the final collected Claude output.
func NewClaudeOutputEvent(iter, maxIter int, output string) Event {
	return Event{
		Type:      EventClaudeOutput,
		Iteration: iter,
		MaxIter:   maxIter,
		Output:    output,
		Message:   "Claude output collected",
	}
}
