// Package engine provides the main execution engine for Ralph.
package engine

import (
	"context"
	"fmt"
	"strings"
	"sync"

	"github.com/google/uuid"

	"github.com/gerund/ralph/internal/agents"
	"github.com/gerund/ralph/internal/claude"
	"github.com/gerund/ralph/internal/config"
	"github.com/gerund/ralph/internal/db"
	"github.com/gerund/ralph/internal/jj"
	"github.com/gerund/ralph/internal/log"
)

// ImplLoopDeps holds the dependencies for the implementation loop.
type ImplLoopDeps struct {
	DB     *db.DB
	Claude *claude.Client
	JJ     *jj.Client
	Agents *agents.Manager
	Config *config.Config
}

// ImplLoop handles the developer->reviewer iteration for a single task.
type ImplLoop struct {
	deps      ImplLoopDeps
	task      *db.Task
	plan      string
	status    ImplLoopStatus
	iteration int
	events    chan ImplLoopEvent
	mu        sync.RWMutex
}

// ImplLoopEvent represents an event during the implementation loop execution.
type ImplLoopEvent struct {
	Type      EventType
	Iteration int
	Message   string // Summary or feedback
}

// EventType represents the type of implementation loop event.
type EventType string

const (
	// EventStarted is emitted when the implementation loop starts.
	EventStarted EventType = "started"
	// EventDeveloping is emitted when the developer agent starts working.
	EventDeveloping EventType = "developing"
	// EventReviewing is emitted when the reviewer agent starts checking.
	EventReviewing EventType = "reviewing"
	// EventFeedback is emitted when the reviewer provides feedback.
	EventFeedback EventType = "feedback"
	// EventApproved is emitted when the reviewer approves the changes.
	EventApproved EventType = "approved"
	// EventFailed is emitted when the implementation loop fails.
	EventFailed EventType = "failed"
)

// ImplLoopStatus represents the current status of the implementation loop.
type ImplLoopStatus string

const (
	ImplLoopStatusPending   ImplLoopStatus = "pending"
	ImplLoopStatusRunning   ImplLoopStatus = "running"
	ImplLoopStatusApproved  ImplLoopStatus = "approved"
	ImplLoopStatusFailed    ImplLoopStatus = "failed"
	ImplLoopStatusEscalated ImplLoopStatus = "escalated"
)

// ReviewResult represents the parsed output from a reviewer.
type ReviewResult struct {
	Approved bool
	Feedback string
}

// NewImplLoop creates a new implementation loop for a task.
func NewImplLoop(deps ImplLoopDeps, task *db.Task, plan string) *ImplLoop {
	return &ImplLoop{
		deps:   deps,
		task:   task,
		plan:   plan,
		status: ImplLoopStatusPending,
		events: make(chan ImplLoopEvent, 100),
	}
}

// Events returns a channel that emits implementation loop events.
// The channel is closed when the implementation loop completes.
func (il *ImplLoop) Events() <-chan ImplLoopEvent {
	return il.events
}

// Status returns the current status of the implementation loop.
func (il *ImplLoop) Status() ImplLoopStatus {
	il.mu.RLock()
	defer il.mu.RUnlock()
	return il.status
}

// setStatus sets the implementation loop status (thread-safe).
func (il *ImplLoop) setStatus(status ImplLoopStatus) {
	il.mu.Lock()
	defer il.mu.Unlock()
	il.status = status
}

// emit sends an event to the events channel.
func (il *ImplLoop) emit(eventType EventType, message string) {
	event := ImplLoopEvent{
		Type:      eventType,
		Iteration: il.iteration,
		Message:   message,
	}

	select {
	case il.events <- event:
	default:
		log.Warn("impl loop event dropped: channel full",
			"event_type", eventType,
			"iteration", il.iteration,
			"message", message)
	}
}

// fail marks the implementation loop as failed and emits a failure event.
func (il *ImplLoop) fail(err error) error {
	il.setStatus(ImplLoopStatusFailed)
	il.emit(EventFailed, err.Error())
	return err
}

// Run executes the developer->reviewer loop until approval or max iterations.
func (il *ImplLoop) Run(ctx context.Context) error {
	defer close(il.events)

	il.setStatus(ImplLoopStatusRunning)
	il.emit(EventStarted, "Starting implementation loop")

	var lastFeedback string
	maxIterations := il.deps.Config.MaxReviewIterations

	for il.iteration = 1; il.iteration <= maxIterations; il.iteration++ {
		// Check for context cancellation
		select {
		case <-ctx.Done():
			return il.fail(ctx.Err())
		default:
		}

		// 1. Run developer
		il.emit(EventDeveloping, fmt.Sprintf("Iteration %d: Developer working", il.iteration))
		if err := il.runDeveloper(ctx, lastFeedback); err != nil {
			return il.fail(err)
		}

		// Check for context cancellation after developer
		select {
		case <-ctx.Done():
			return il.fail(ctx.Err())
		default:
		}

		// 2. Get diff
		diff, err := il.deps.JJ.Show(ctx)
		if err != nil {
			return il.fail(fmt.Errorf("failed to get diff: %w", err))
		}

		// 3. Run reviewer
		il.emit(EventReviewing, fmt.Sprintf("Iteration %d: Reviewer checking", il.iteration))
		result, err := il.runReviewer(ctx, diff)
		if err != nil {
			return il.fail(err)
		}

		// 4. Parse result
		if result.Approved {
			il.setStatus(ImplLoopStatusApproved)
			il.emit(EventApproved, "Changes approved")
			return nil
		}

		// 5. Loop with feedback
		lastFeedback = result.Feedback
		il.emit(EventFeedback, result.Feedback)

		// Increment task iteration count in database.
		// Errors are intentionally ignored: iteration tracking is for metrics only,
		// not critical to the implementation loop's correctness.
		_ = il.deps.DB.IncrementTaskIteration(il.task.ID)
	}

	il.setStatus(ImplLoopStatusEscalated)
	return fmt.Errorf("max iterations (%d) reached", maxIterations)
}

// runDeveloper runs the developer agent with the given feedback context.
func (il *ImplLoop) runDeveloper(ctx context.Context, feedback string) error {
	// Create agent with context
	agent, err := il.deps.Agents.GetDeveloperAgent(ctx, il.plan, il.task, feedback)
	if err != nil {
		return fmt.Errorf("failed to get developer agent: %w", err)
	}

	// Create session record
	session := &db.Session{
		ID:          uuid.New().String(),
		TaskID:      il.task.ID,
		AgentType:   db.AgentDeveloper,
		Iteration:   il.iteration,
		InputPrompt: agent.Prompt,
		Status:      db.SessionRunning,
	}
	if err := il.deps.DB.CreateSession(session); err != nil {
		return fmt.Errorf("failed to create session record: %w", err)
	}

	// Run Claude
	claudeSession, err := il.deps.Claude.Run(ctx, agent.Prompt, "")
	if err != nil {
		// Best-effort status update on failure path.
		_ = il.deps.DB.CompleteSession(session.ID, db.SessionFailed)
		return fmt.Errorf("failed to run claude: %w", err)
	}

	// Store all events. Message storage errors are intentionally ignored:
	// losing individual messages doesn't affect the implementation loop's correctness.
	seq := 0
	for event := range claudeSession.Events() {
		msg := &db.Message{
			SessionID:   session.ID,
			Sequence:    seq,
			MessageType: string(event.Type),
			Content:     string(event.Raw),
		}
		_ = il.deps.DB.CreateMessage(msg)
		seq++
	}

	if err := claudeSession.Wait(); err != nil {
		// Best-effort status update on failure path.
		_ = il.deps.DB.CompleteSession(session.ID, db.SessionFailed)
		return fmt.Errorf("claude session failed: %w", err)
	}

	// Best-effort status update; session ran successfully regardless of DB state.
	_ = il.deps.DB.CompleteSession(session.ID, db.SessionCompleted)
	return nil
}

// runReviewer runs the reviewer agent with the diff output.
func (il *ImplLoop) runReviewer(ctx context.Context, diff string) (*ReviewResult, error) {
	// Create agent with context
	agent, err := il.deps.Agents.GetReviewerAgent(ctx, il.plan, il.task, diff)
	if err != nil {
		return nil, fmt.Errorf("failed to get reviewer agent: %w", err)
	}

	// Create session record
	session := &db.Session{
		ID:          uuid.New().String(),
		TaskID:      il.task.ID,
		AgentType:   db.AgentReviewer,
		Iteration:   il.iteration,
		InputPrompt: agent.Prompt,
		Status:      db.SessionRunning,
	}
	if err := il.deps.DB.CreateSession(session); err != nil {
		return nil, fmt.Errorf("failed to create session record: %w", err)
	}

	// Run Claude
	claudeSession, err := il.deps.Claude.Run(ctx, agent.Prompt, "")
	if err != nil {
		// Best-effort status update on failure path.
		_ = il.deps.DB.CompleteSession(session.ID, db.SessionFailed)
		return nil, fmt.Errorf("failed to run claude: %w", err)
	}

	// Store all events and collect text output for parsing.
	// Message storage errors are intentionally ignored: losing individual messages
	// doesn't affect the implementation loop's correctness.
	var textOutput strings.Builder
	seq := 0
	for event := range claudeSession.Events() {
		msg := &db.Message{
			SessionID:   session.ID,
			Sequence:    seq,
			MessageType: string(event.Type),
			Content:     string(event.Raw),
		}
		_ = il.deps.DB.CreateMessage(msg)
		seq++

		// Collect text from message events for parsing
		if event.Type == claude.EventMessage && event.Message != nil {
			textOutput.WriteString(event.Message.Text)
		}
		// Also capture final result text
		if event.Type == claude.EventResult && event.Result != nil {
			textOutput.WriteString(event.Result.Result)
		}
	}

	if err := claudeSession.Wait(); err != nil {
		// Best-effort status update on failure path.
		_ = il.deps.DB.CompleteSession(session.ID, db.SessionFailed)
		return nil, fmt.Errorf("claude session failed: %w", err)
	}

	// Best-effort status update; session ran successfully regardless of DB state.
	_ = il.deps.DB.CompleteSession(session.ID, db.SessionCompleted)

	// Parse the reviewer output
	result := parseReviewerOutput(textOutput.String())

	// Store feedback in database if not approved.
	// Errors are intentionally ignored: feedback storage is for tracking only.
	if !result.Approved && result.Feedback != "" {
		feedbackContent := result.Feedback
		feedback := &db.Feedback{
			SessionID:    session.ID,
			FeedbackType: db.FeedbackMajor, // Default to major feedback
			Content:      &feedbackContent,
		}
		_ = il.deps.DB.CreateFeedback(feedback)
	}

	return &result, nil
}

// parseReviewerOutput parses the reviewer's text output for APPROVED or FEEDBACK.
func parseReviewerOutput(text string) ReviewResult {
	text = strings.TrimSpace(text)

	// Look for APPROVED (case-insensitive check, but commonly uppercase)
	if strings.Contains(strings.ToUpper(text), "APPROVED") {
		// Make sure it's not "NOT APPROVED" or similar negation
		upperText := strings.ToUpper(text)
		if !strings.Contains(upperText, "NOT APPROVED") &&
			!strings.Contains(upperText, "NOT YET APPROVED") &&
			!strings.Contains(upperText, "CANNOT BE APPROVED") {
			return ReviewResult{Approved: true}
		}
	}

	// Look for FEEDBACK:
	if idx := strings.Index(strings.ToUpper(text), "FEEDBACK:"); idx != -1 {
		// Extract everything after "FEEDBACK:"
		feedback := strings.TrimSpace(text[idx+9:])
		return ReviewResult{Feedback: feedback}
	}

	// Default to feedback with full text
	// This handles cases where reviewer didn't use the expected format
	return ReviewResult{Feedback: text}
}
