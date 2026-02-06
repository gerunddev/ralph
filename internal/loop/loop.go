// Package loop provides the main execution loop for Ralph.
package loop

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"sync"

	"github.com/google/uuid"

	"github.com/gerunddev/ralph/internal/agent"
	"github.com/gerunddev/ralph/internal/claude"
	"github.com/gerunddev/ralph/internal/db"
	"github.com/gerunddev/ralph/internal/jj"
	"github.com/gerunddev/ralph/internal/log"
	"github.com/gerunddev/ralph/internal/parser"
)

// maxDiffBytes is the maximum size of diff to include in reviewer prompt.
// Large diffs can exhaust the context window before the reviewer even starts.
// 256KB is ~64k tokens, leaving ~136k tokens for the prompt and response
// with 200k context window models.
const maxDiffBytes = 256 * 1024

// truncateDiff limits diff size to prevent context window exhaustion.
// Returns the original diff if under limit, otherwise truncates with a message.
func truncateDiff(diff string) string {
	if len(diff) <= maxDiffBytes {
		return diff
	}

	truncated := diff[:maxDiffBytes]
	// Try to truncate at a line boundary for cleaner output
	if lastNewline := strings.LastIndex(truncated, "\n"); lastNewline > maxDiffBytes/2 {
		truncated = truncated[:lastNewline]
	}

	return truncated + "\n\n... [DIFF TRUNCATED - " +
		fmt.Sprintf("%d", len(diff)-len(truncated)) +
		" bytes omitted. Review may be incomplete for large changes.]"
}

// editToolNames contains tool names that indicate file modifications.
var editToolNames = map[string]bool{
	"Edit":         true,
	"Write":        true,
	"NotebookEdit": true,
}

// isEditTool returns true if the tool name indicates a file editing operation.
func isEditTool(name string) bool {
	return editToolNames[name]
}

// sanitizeDoneMarker removes the DONE marker from text.
// Used when DONE is rejected due to edits but we still need to save progress/learnings.
func sanitizeDoneMarker(s string) string {
	return strings.ReplaceAll(s, parser.DoneMarker, "")
}

// sanitizeDevDoneMarker removes the DEV_DONE marker from text.
func sanitizeDevDoneMarker(s string) string {
	return strings.ReplaceAll(s, parser.DevDoneMarker, "")
}

// Config holds configuration for the loop.
type Config struct {
	PlanID          string
	MaxIterations   int
	ExtremeMode     bool   // Enable extreme mode (+3 iterations after both done)
	WorkDir         string // For jj operations
	EventBufferSize int    // Size of event channel buffer (default: 1000)
}

// Deps holds dependencies for the loop.
type Deps struct {
	DB     *db.DB
	Claude *claude.Client // Main model for development
	JJ     *jj.Client
}

// Loop orchestrates the main execution loop for Ralph.
type Loop struct {
	cfg  Config
	deps Deps

	events      chan Event
	eventsMu    sync.Mutex
	iterationMu sync.RWMutex
	iteration   int

	// For tracking state
	plan         *db.Plan
	baseChangeID string // jj change ID at the start of the loop, used for reviewer diffs

	// Extreme mode state
	extremeModeTriggered bool // Whether +3 has been triggered
}

// New creates a new Loop with the given configuration and dependencies.
func New(cfg Config, deps Deps) *Loop {
	bufferSize := cfg.EventBufferSize
	if bufferSize <= 0 {
		bufferSize = 10000 // Default buffer size - needs to be large for Claude streaming events
	}
	return &Loop{
		cfg:    cfg,
		deps:   deps,
		events: make(chan Event, bufferSize),
	}
}

// Events returns the channel for receiving loop events.
// The channel is closed when the loop completes.
func (l *Loop) Events() <-chan Event {
	return l.events
}

// effectiveMaxIter returns the max iterations to use in events.
// In extreme mode before trigger, returns 0 (signals "X" to UI).
func (l *Loop) effectiveMaxIter() int {
	if l.cfg.ExtremeMode && !l.extremeModeTriggered {
		return 0
	}
	return l.cfg.MaxIterations
}

// CurrentIteration returns the current iteration number.
// This method is safe to call concurrently.
func (l *Loop) CurrentIteration() int {
	l.iterationMu.RLock()
	defer l.iterationMu.RUnlock()
	return l.iteration
}

// Run executes the main loop until completion, max iterations, or cancellation.
func (l *Loop) Run(ctx context.Context) error {
	defer close(l.events)

	// Load the plan
	plan, err := l.deps.DB.GetPlan(l.cfg.PlanID)
	if err != nil {
		return fmt.Errorf("failed to load plan: %w", err)
	}
	l.plan = plan

	// Determine starting iteration (for resume support)
	latestSession, err := l.deps.DB.GetLatestPlanSession(l.cfg.PlanID)
	if err != nil {
		return fmt.Errorf("failed to get latest session: %w", err)
	}
	if latestSession != nil {
		l.iterationMu.Lock()
		l.iteration = latestSession.Iteration
		l.iterationMu.Unlock()
	}

	// Update plan status to running
	if err := l.deps.DB.UpdatePlanStatus(l.cfg.PlanID, db.PlanStatusRunning); err != nil {
		log.Warn("failed to update plan status", "error", err)
	}

	// Load or capture base change ID for reviewer diffs.
	// On first run, we capture @- (parent of current working copy) and persist it.
	// On resume, we use the persisted value so cumulative diffs include all changes since plan start.
	if plan.BaseChangeID != "" {
		// Resume case: use the persisted base change ID
		l.baseChangeID = plan.BaseChangeID
		log.Debug("using persisted base change ID for reviewer diffs", "changeID", plan.BaseChangeID)
	} else {
		// First run: capture and persist the parent change ID
		baseChangeID, err := l.deps.JJ.GetParentChangeID(ctx)
		if err != nil {
			log.Warn("failed to get parent change ID", "error", err)
		} else if baseChangeID != "" {
			l.baseChangeID = baseChangeID
			// Persist so we have it on resume
			if err := l.deps.DB.UpdatePlanBaseChangeID(l.cfg.PlanID, baseChangeID); err != nil {
				log.Warn("failed to persist base change ID", "error", err)
			} else {
				log.Debug("captured and persisted parent change ID for reviewer diffs", "changeID", baseChangeID)
			}
		} else {
			log.Debug("no parent change ID (root commit), will use jj show fallback")
		}
	}

	// Emit started event
	l.emit(NewEvent(EventStarted, l.iteration, l.effectiveMaxIter(), "Loop started"))

	// Main loop
	for {
		// Check for context cancellation
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}

		// Increment iteration
		l.iterationMu.Lock()
		l.iteration++
		currentIter := l.iteration
		l.iterationMu.Unlock()

		// Check max iterations (skip in extreme mode until triggered)
		if !l.cfg.ExtremeMode || l.extremeModeTriggered {
			if currentIter > l.cfg.MaxIterations {
				l.emit(NewEvent(EventMaxIterations, l.iteration-1, l.effectiveMaxIter(),
					fmt.Sprintf("Reached max iterations (%d)", l.cfg.MaxIterations)))
				return nil
			}
		}

		// Run one iteration
		done, err := l.runIteration(ctx)
		if err != nil {
			if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
				return err
			}
			// Log error but continue - be resilient
			log.Error("iteration error", "iteration", l.iteration, "error", err)
			l.emit(NewErrorEvent(l.iteration, l.effectiveMaxIter(), err))
			continue
		}

		if done {
			if l.cfg.ExtremeMode {
				if !l.extremeModeTriggered {
					// First "both done" in extreme mode - trigger +3
					l.extremeModeTriggered = true
					l.cfg.MaxIterations = currentIter + 3
					l.emit(NewEvent(EventExtremeModeTriggered, l.iteration, l.effectiveMaxIter(),
						fmt.Sprintf("+3 iterations (max now %d)", l.cfg.MaxIterations)))
					continue
				}
				// Already triggered - ignore done, keep going
				continue
			}
			// Normal mode - exit
			if err := l.deps.DB.UpdatePlanStatus(l.cfg.PlanID, db.PlanStatusCompleted); err != nil {
				log.Warn("failed to mark plan complete", "error", err)
			}
			l.emit(NewEvent(EventDone, l.iteration, l.effectiveMaxIter(), "Agent completed"))
			return nil
		}
	}
}

// emit sends an event to the events channel if it's not full.
func (l *Loop) emit(event Event) {
	l.eventsMu.Lock()
	defer l.eventsMu.Unlock()

	select {
	case l.events <- event:
	default:
		// Channel full, log and drop
		log.Warn("event channel full, dropping event", "type", event.Type)
	}
}

// runIteration runs a single iteration with developer and reviewer.
// Returns (done, error) where done indicates both developer and reviewer approved.
func (l *Loop) runIteration(ctx context.Context) (bool, error) {
	l.emit(NewEvent(EventIterationStart, l.iteration, l.effectiveMaxIter(),
		fmt.Sprintf("Starting iteration %d", l.iteration)))

	// 1. Load state
	progress, learnings, feedback, err := l.loadState()
	if err != nil {
		return false, err
	}

	// 2. Run developer agent
	l.emit(NewEvent(EventDeveloperStart, l.iteration, l.effectiveMaxIter(), "Starting developer agent"))

	devOutput, devHadEdits, devSessionID, err := l.runDeveloper(ctx, progress, learnings, feedback)
	if err != nil {
		return false, fmt.Errorf("developer agent failed: %w", err)
	}

	l.emit(NewEvent(EventDeveloperEnd, l.iteration, l.effectiveMaxIter(), "Developer agent ended"))

	// 3. Parse developer output
	devResult := parser.ParseAgentOutput(devOutput, "developer")

	// 4. Store developer progress/learnings
	l.storeProgressLearnings(devSessionID, devResult.Progress, devResult.Learnings)

	// 5. Clear any previous reviewer feedback (developer has now seen and addressed it)
	if feedback != "" {
		if err := l.deps.DB.ClearReviewerFeedback(l.cfg.PlanID); err != nil {
			log.Warn("failed to clear reviewer feedback", "error", err)
		}
	}

	// 6. Check developer done status
	if !devResult.DevDone || devHadEdits {
		// Developer still working (or had edits), commit and continue
		if devHadEdits && devResult.DevDone {
			log.Info("ignoring DEV_DONE because session contained file edits",
				"iteration", l.iteration)
		}
		l.emit(NewEvent(EventIterationEnd, l.iteration, l.effectiveMaxIter(),
			fmt.Sprintf("Developer iteration %d complete, continuing", l.iteration)))
		return false, nil
	}

	// Developer done without edits - run reviewer
	l.emit(NewEvent(EventDeveloperDone, l.iteration, l.effectiveMaxIter(),
		"Developer signaled DEV_DONE, triggering reviewer"))

	// 7. Get diff for reviewer - use cumulative diff from base change, not just current change
	var diff string
	if l.baseChangeID != "" {
		log.Debug("getting cumulative diff for reviewer", "baseChangeID", l.baseChangeID)
		diff, err = l.deps.JJ.Diff(ctx, l.baseChangeID, "@")
		if err != nil {
			log.Warn("failed to get cumulative diff for reviewer", "error", err)
			diff = ""
		} else if strings.TrimSpace(diff) == "" {
			log.Warn("cumulative diff is empty despite having baseChangeID",
				"baseChangeID", l.baseChangeID,
				"hint", "changes may have been squashed, rebased, or committed before loop started")
			// Set a note for the reviewer about this edge case
			diff = "[Note: Cumulative diff from base change " + l.baseChangeID +
				" is empty. This may occur if changes were squashed/rebased, or if all work " +
				"was committed before the loop captured the base change. Review the Developer " +
				"Summary section for context on what was accomplished.]"
		} else {
			log.Debug("got cumulative diff for reviewer", "diffLen", len(diff), "diffPreview", truncateString(diff, 200))
		}
	} else {
		// Fallback to jj show if we don't have a base change ID.
		// This happens when starting from a root commit (no parent to diff against).
		// LIMITATION: This only shows the current change's diff, not cumulative work
		// across multiple changes. If developer created multiple changes before DEV_DONE,
		// earlier changes will not be included in this review.
		log.Warn("no baseChangeID available, falling back to jj show (single change only)",
			"limitation", "review will only include current change, not cumulative session work")
		diff, err = l.deps.JJ.Show(ctx)
		if err != nil {
			log.Warn("failed to get diff for reviewer", "error", err)
			diff = ""
		} else if strings.TrimSpace(diff) != "" {
			// Prepend a note about the limitation
			diff = "[Note: This diff shows only the current jj change. If work spanned " +
				"multiple changes, earlier changes are not included in this review.]\n\n" + diff
		}
	}

	// Truncate large diffs to prevent context window exhaustion
	if len(diff) > maxDiffBytes {
		log.Warn("diff exceeds size limit, truncating",
			"originalSize", len(diff),
			"maxSize", maxDiffBytes)
		diff = truncateDiff(diff)
	}

	// 8. Run reviewer agent
	l.emit(NewEvent(EventReviewerStart, l.iteration, l.effectiveMaxIter(), "Starting reviewer agent"))

	reviewOutput, reviewSessionID, err := l.runReviewer(ctx, progress, learnings, diff, devOutput)
	if err != nil {
		return false, fmt.Errorf("reviewer agent failed: %w", err)
	}

	l.emit(NewEvent(EventReviewerEnd, l.iteration, l.effectiveMaxIter(), "Reviewer agent ended"))

	// 9. Parse reviewer output
	reviewResult := parser.ParseAgentOutput(reviewOutput, "reviewer")

	// 10. Store reviewer progress/learnings
	l.storeProgressLearnings(reviewSessionID, reviewResult.Progress, reviewResult.Learnings)

	// 11. Check reviewer verdict
	if reviewResult.ReviewerApproved {
		// BOTH voted DONE - complete!
		l.emit(NewEvent(EventReviewerApproved, l.iteration, l.effectiveMaxIter(),
			"Reviewer approved - implementation complete"))
		l.emit(NewEvent(EventBothDone, l.iteration, l.effectiveMaxIter(),
			"Both developer and reviewer approved"))

		return true, nil
	}

	// 12. Reviewer has feedback - store for next iteration
	l.emit(NewEvent(EventReviewerFeedback, l.iteration, l.effectiveMaxIter(),
		fmt.Sprintf("Reviewer feedback: %s", truncateString(reviewResult.ReviewerFeedback, 100))))

	if err := l.storeReviewerFeedback(reviewSessionID, reviewResult.ReviewerFeedback); err != nil {
		log.Warn("failed to store reviewer feedback", "error", err)
	}

	l.emit(NewEvent(EventIterationEnd, l.iteration, l.effectiveMaxIter(),
		fmt.Sprintf("iteration %d complete with reviewer feedback", l.iteration)))

	return false, nil
}

// loadState loads progress, learnings, and reviewer feedback.
func (l *Loop) loadState() (progress, learnings, feedback string, err error) {
	progressRecord, err := l.deps.DB.GetLatestProgress(l.cfg.PlanID)
	if err != nil {
		return "", "", "", fmt.Errorf("failed to get latest progress: %w", err)
	}
	if progressRecord != nil {
		progress = progressRecord.Content
	}

	learningsRecord, err := l.deps.DB.GetLatestLearnings(l.cfg.PlanID)
	if err != nil {
		return "", "", "", fmt.Errorf("failed to get latest learnings: %w", err)
	}
	if learningsRecord != nil {
		learnings = learningsRecord.Content
	}

	feedbackRecord, err := l.deps.DB.GetLatestReviewerFeedback(l.cfg.PlanID)
	if err != nil {
		return "", "", "", fmt.Errorf("failed to get latest reviewer feedback: %w", err)
	}
	if feedbackRecord != nil {
		feedback = feedbackRecord.Content
	}

	return progress, learnings, feedback, nil
}

// runDeveloper runs the developer agent and returns output, whether edits occurred, and session ID.
func (l *Loop) runDeveloper(ctx context.Context, progress, learnings, feedback string) (output string, hadEdits bool, sessionID string, err error) {
	// Build developer prompt
	prompt, err := agent.BuildDeveloperPrompt(agent.DeveloperContext{
		PlanContent:      l.plan.Content,
		Progress:         progress,
		Learnings:        learnings,
		ReviewerFeedback: feedback,
	})
	if err != nil {
		return "", false, "", fmt.Errorf("failed to build developer prompt: %w", err)
	}

	l.emit(NewPromptBuiltEvent(l.iteration, l.effectiveMaxIter(), prompt))

	// Create session in DB
	sessionID = uuid.New().String()
	session := &db.PlanSession{
		ID:          sessionID,
		PlanID:      l.cfg.PlanID,
		Iteration:   l.iteration,
		InputPrompt: prompt,
		Status:      db.PlanSessionRunning,
		AgentType:   db.LoopAgentDeveloper,
	}
	if err := l.deps.DB.CreatePlanSession(session); err != nil {
		return "", false, "", fmt.Errorf("failed to create developer session: %w", err)
	}

	// Run Claude session
	output, hadEdits, err = l.runClaudeSession(ctx, sessionID, prompt)
	if err != nil {
		return "", false, sessionID, err
	}

	return output, hadEdits, sessionID, nil
}

// runReviewer runs the reviewer agent and returns output and session ID.
func (l *Loop) runReviewer(ctx context.Context, progress, learnings, diff, devSummary string) (output string, sessionID string, err error) {
	// Build reviewer prompt
	prompt, err := agent.BuildReviewerPrompt(agent.ReviewerContext{
		PlanContent:      l.plan.Content,
		Progress:         progress,
		Learnings:        learnings,
		DiffOutput:       diff,
		DeveloperSummary: devSummary,
	})
	if err != nil {
		return "", "", fmt.Errorf("failed to build reviewer prompt: %w", err)
	}

	l.emit(NewPromptBuiltEvent(l.iteration, l.effectiveMaxIter(), prompt))

	// Create session in DB
	sessionID = uuid.New().String()
	session := &db.PlanSession{
		ID:          sessionID,
		PlanID:      l.cfg.PlanID,
		Iteration:   l.iteration,
		InputPrompt: prompt,
		Status:      db.PlanSessionRunning,
		AgentType:   db.LoopAgentReviewer,
	}
	if err := l.deps.DB.CreatePlanSession(session); err != nil {
		return "", "", fmt.Errorf("failed to create reviewer session: %w", err)
	}

	// Run Claude session (reviewer doesn't track edits)
	output, _, err = l.runClaudeSession(ctx, sessionID, prompt)
	if err != nil {
		return "", sessionID, err
	}

	return output, sessionID, nil
}

// runClaudeSession runs a Claude session and returns the output and whether edits occurred.
func (l *Loop) runClaudeSession(ctx context.Context, sessionID, prompt string) (output string, hadEdits bool, err error) {
	l.emit(NewEvent(EventClaudeStart, l.iteration, l.effectiveMaxIter(), "Starting Claude session"))

	claudeSession, err := l.deps.Claude.Run(ctx, prompt)
	if err != nil {
		if dbErr := l.deps.DB.CompletePlanSession(sessionID, db.PlanSessionFailed, ""); dbErr != nil {
			log.Warn("failed to mark session as failed", "error", dbErr)
		}
		return "", false, fmt.Errorf("failed to start Claude: %w", err)
	}

	// Stream events and collect output
	var outputBuilder strings.Builder
	sequence := 0

	// Context window tracking
	maxContext := claude.DefaultContextWindow
	contextLimitReached := false

	for claudeEvent := range claudeSession.Events() {
		// Get max context from init event
		if claudeEvent.Type == claude.EventInit && claudeEvent.Init != nil {
			maxContext = claude.GetContextWindowForModel(claudeEvent.Init.Model)
			log.Debug("context window determined", "model", claudeEvent.Init.Model, "maxContext", maxContext)
		}

		// Track token usage from message events and check context limit
		if !contextLimitReached && claudeEvent.Type == claude.EventMessage && claudeEvent.Message != nil {
			totalTokens := claudeEvent.Message.Usage.InputTokens + claudeEvent.Message.Usage.OutputTokens
			percentage := float64(totalTokens) / float64(maxContext) * 100.0

			if percentage >= claude.ContextLimitPercent {
				contextLimitReached = true
				log.Info("context limit reached, stopping session",
					"percentage", fmt.Sprintf("%.1f%%", percentage),
					"totalTokens", totalTokens,
					"maxContext", maxContext)
				l.emit(NewEvent(EventContextLimit, l.iteration, l.effectiveMaxIter(),
					fmt.Sprintf("Context limit reached: %.1f%% (%d/%d tokens)", percentage, totalTokens, maxContext)))
				claudeSession.Cancel()
				// Continue to drain remaining events from the channel
			}
		}

		// Track if any edit tools are used
		if claudeEvent.Type == claude.EventToolUse && claudeEvent.ToolUse != nil {
			if isEditTool(claudeEvent.ToolUse.Name) {
				hadEdits = true
			}
		}

		// Emit to our event channel
		eventCopy := claudeEvent
		l.emit(NewClaudeStreamEvent(l.iteration, l.effectiveMaxIter(), &eventCopy))

		// Store event in DB
		dbEvent := &db.Event{
			SessionID: sessionID,
			Sequence:  sequence,
			EventType: string(claudeEvent.Type),
			RawJSON:   string(claudeEvent.Raw),
		}
		if err := l.deps.DB.CreateEvent(dbEvent); err != nil {
			log.Warn("failed to store event", "error", err)
		}
		sequence++

		// Collect text
		if claudeEvent.Type == claude.EventAssistantText && claudeEvent.AssistantText != nil {
			outputBuilder.WriteString(claudeEvent.AssistantText.Text)
		} else if claudeEvent.Type == claude.EventMessage && claudeEvent.Message != nil {
			outputBuilder.WriteString(claudeEvent.Message.Text)
		}
	}

	if err := claudeSession.Wait(); err != nil {
		log.Warn("Claude session error", "error", err)
	}

	output = outputBuilder.String()
	l.emit(NewClaudeOutputEvent(l.iteration, l.effectiveMaxIter(), output))
	l.emit(NewEvent(EventClaudeEnd, l.iteration, l.effectiveMaxIter(), "Claude session ended"))

	// Mark session complete
	if err := l.deps.DB.CompletePlanSession(sessionID, db.PlanSessionCompleted, output); err != nil {
		log.Warn("failed to complete session", "error", err)
	}

	return output, hadEdits, nil
}

// storeProgressLearnings stores progress and learnings from an agent session.
func (l *Loop) storeProgressLearnings(sessionID, progress, learnings string) {
	if progress != "" {
		// Sanitize any done markers
		progress = sanitizeDevDoneMarker(sanitizeDoneMarker(progress))
		progressRecord := &db.Progress{
			PlanID:    l.cfg.PlanID,
			SessionID: sessionID,
			Content:   progress,
		}
		if err := l.deps.DB.CreateProgress(progressRecord); err != nil {
			log.Warn("failed to store progress", "error", err)
		}
	}

	if learnings != "" {
		// Sanitize any done markers
		learnings = sanitizeDevDoneMarker(sanitizeDoneMarker(learnings))
		learningsRecord := &db.Learnings{
			PlanID:    l.cfg.PlanID,
			SessionID: sessionID,
			Content:   learnings,
		}
		if err := l.deps.DB.CreateLearnings(learningsRecord); err != nil {
			log.Warn("failed to store learnings", "error", err)
		}
	}
}

// storeReviewerFeedback stores feedback from a reviewer rejection.
func (l *Loop) storeReviewerFeedback(sessionID, feedback string) error {
	if feedback == "" {
		return nil
	}

	feedbackRecord := &db.ReviewerFeedback{
		PlanID:    l.cfg.PlanID,
		SessionID: sessionID,
		Content:   feedback,
	}
	return l.deps.DB.CreateReviewerFeedback(feedbackRecord)
}

// truncateString truncates a string to maxLen, adding "..." if truncated.
func truncateString(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen-3] + "..."
}
