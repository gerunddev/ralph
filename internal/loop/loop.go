// Package loop provides the main execution loop for Ralph V2.
package loop

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"sync"

	"github.com/google/uuid"

	"github.com/gerund/ralph/internal/agent"
	"github.com/gerund/ralph/internal/claude"
	"github.com/gerund/ralph/internal/db"
	"github.com/gerund/ralph/internal/distill"
	"github.com/gerund/ralph/internal/jj"
	"github.com/gerund/ralph/internal/log"
	"github.com/gerund/ralph/internal/parser"
)

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
	WorkDir         string // For jj operations
	EventBufferSize int    // Size of event channel buffer (default: 1000)
	UseV15          bool   // Use V1.5 dual-agent loop (developer + reviewer)
}

// Deps holds dependencies for the loop.
type Deps struct {
	DB        *db.DB
	Claude    *claude.Client // Main model for development
	Distiller *distill.Distiller
	JJ        *jj.Client
}

// Loop orchestrates the main execution loop for Ralph V2.
type Loop struct {
	cfg  Config
	deps Deps

	events      chan Event
	eventsMu    sync.Mutex
	iterationMu sync.RWMutex
	iteration   int

	// For tracking state
	plan *db.Plan
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

	// Emit started event
	l.emit(NewEvent(EventStarted, l.iteration, l.cfg.MaxIterations, "Loop started"))

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

		// Check max iterations
		if currentIter > l.cfg.MaxIterations {
			l.emit(NewEvent(EventMaxIterations, l.iteration-1, l.cfg.MaxIterations,
				fmt.Sprintf("Reached max iterations (%d)", l.cfg.MaxIterations)))
			return nil
		}

		// Run one iteration (choose loop version based on config)
		var done bool
		var err error
		if l.cfg.UseV15 {
			done, err = l.runV15Iteration(ctx)
		} else {
			done, err = l.runIteration(ctx)
		}
		if err != nil {
			if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
				return err
			}
			// Log error but continue - be resilient
			log.Error("iteration error", "iteration", l.iteration, "error", err)
			l.emit(NewErrorEvent(l.iteration, l.cfg.MaxIterations, err))
			continue
		}

		if done {
			// Agent said "DONE DONE DONE!!!"
			if err := l.deps.DB.UpdatePlanStatus(l.cfg.PlanID, db.PlanStatusCompleted); err != nil {
				log.Warn("failed to mark plan complete", "error", err)
			}
			l.emit(NewEvent(EventDone, l.iteration, l.cfg.MaxIterations, "Agent completed"))
			return nil
		}
	}
}

// runIteration runs a single iteration of the loop.
// Returns (done, error) where done indicates the agent said "DONE DONE DONE!!!".
func (l *Loop) runIteration(ctx context.Context) (bool, error) {
	l.emit(NewEvent(EventIterationStart, l.iteration, l.cfg.MaxIterations,
		fmt.Sprintf("Starting iteration %d", l.iteration)))

	// 1. Load latest progress and learnings
	progress, err := l.deps.DB.GetLatestProgress(l.cfg.PlanID)
	if err != nil {
		return false, fmt.Errorf("failed to get latest progress: %w", err)
	}

	learnings, err := l.deps.DB.GetLatestLearnings(l.cfg.PlanID)
	if err != nil {
		return false, fmt.Errorf("failed to get latest learnings: %w", err)
	}

	// 2. Build prompt
	var progressContent, learningsContent string
	if progress != nil {
		progressContent = progress.Content
	}
	if learnings != nil {
		learningsContent = learnings.Content
	}

	prompt, err := agent.BuildPrompt(agent.PromptContext{
		PlanContent: l.plan.Content,
		Progress:    progressContent,
		Learnings:   learningsContent,
	})
	if err != nil {
		return false, fmt.Errorf("failed to build prompt: %w", err)
	}

	// Emit prompt built event with full prompt content
	l.emit(NewPromptBuiltEvent(l.iteration, l.cfg.MaxIterations, prompt))

	// 3. Run jj new only if current change has content
	isEmpty, err := l.deps.JJ.IsEmpty(ctx)
	if err != nil {
		log.Warn("failed to check if change is empty", "error", err)
		// Assume not empty on error - safer to create new change
		isEmpty = false
	}

	if !isEmpty {
		l.emit(NewEvent(EventJJNew, l.iteration, l.cfg.MaxIterations, "Creating new jj change"))
		if err := l.deps.JJ.New(ctx); err != nil {
			// Log but continue - jj errors shouldn't stop the loop
			log.Warn("jj new failed", "error", err)
			l.emit(NewErrorEvent(l.iteration, l.cfg.MaxIterations,
				fmt.Errorf("jj new failed: %w", err)))
		}
	} else {
		l.emit(NewEvent(EventJJNew, l.iteration, l.cfg.MaxIterations, "Skipping jj new (current change is empty)"))
	}

	// 4. Create session in DB
	sessionID := uuid.New().String()
	session := &db.PlanSession{
		ID:          sessionID,
		PlanID:      l.cfg.PlanID,
		Iteration:   l.iteration,
		InputPrompt: prompt,
		Status:      db.PlanSessionRunning,
	}
	if err := l.deps.DB.CreatePlanSession(session); err != nil {
		return false, fmt.Errorf("failed to create session: %w", err)
	}

	// 5. Run Claude session
	l.emit(NewEvent(EventClaudeStart, l.iteration, l.cfg.MaxIterations, "Starting Claude session"))

	claudeSession, err := l.deps.Claude.Run(ctx, prompt)
	if err != nil {
		// Mark session as failed
		if dbErr := l.deps.DB.CompletePlanSession(sessionID, db.PlanSessionFailed, ""); dbErr != nil {
			log.Warn("failed to mark session as failed", "error", dbErr)
		}
		return false, fmt.Errorf("failed to start Claude: %w", err)
	}

	// Stream events and collect output
	var outputBuilder strings.Builder
	var sessionHasEdits bool
	sequence := 0
	for claudeEvent := range claudeSession.Events() {
		// Track if any edit tools are used
		if claudeEvent.Type == claude.EventToolUse && claudeEvent.ToolUse != nil {
			if isEditTool(claudeEvent.ToolUse.Name) {
				sessionHasEdits = true
			}
		}
		// Emit to our event channel
		eventCopy := claudeEvent
		l.emit(NewClaudeStreamEvent(l.iteration, l.cfg.MaxIterations, &eventCopy))

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

		// Collect text from assistant text streaming events and message events
		if claudeEvent.Type == claude.EventAssistantText && claudeEvent.AssistantText != nil {
			outputBuilder.WriteString(claudeEvent.AssistantText.Text)
		} else if claudeEvent.Type == claude.EventMessage && claudeEvent.Message != nil {
			// Also collect from complete message events (fallback if streaming not available)
			outputBuilder.WriteString(claudeEvent.Message.Text)
		}
	}

	// Wait for Claude to complete
	if err := claudeSession.Wait(); err != nil {
		log.Warn("Claude session error", "error", err)
		// Don't fail the iteration - we might still have useful output
	}

	// Get final output
	finalOutput := outputBuilder.String()

	// Emit final collected output for display
	l.emit(NewClaudeOutputEvent(l.iteration, l.cfg.MaxIterations, finalOutput))

	l.emit(NewEvent(EventClaudeEnd, l.iteration, l.cfg.MaxIterations, "Claude session ended"))

	// 6. Parse output
	l.emit(NewEvent(EventParsed, l.iteration, l.cfg.MaxIterations, "Parsing output"))
	parseResult := parser.Parse(finalOutput)

	// 7. Handle completion or store progress/learnings
	if parseResult.IsDone {
		if sessionHasEdits {
			// DONE marker rejected because session contained file edits.
			// The agent should have done a review-only pass before saying DONE.
			log.Info("ignoring DONE marker because session contained file edits",
				"iteration", l.iteration)
			l.emit(NewEvent(EventParsed, l.iteration, l.cfg.MaxIterations,
				"DONE ignored - session had edits, will continue iteration"))

			// Sanitize the DONE marker from parsed sections before saving
			parseResult.Progress = sanitizeDoneMarker(parseResult.Progress)
			parseResult.Learnings = sanitizeDoneMarker(parseResult.Learnings)
			parseResult.Status = sanitizeDoneMarker(parseResult.Status)
			parseResult.IsDone = false // Reset so logic falls through correctly
			// Fall through to store sanitized progress/learnings and continue
		} else {
			// Accept DONE - this was a true review-only session
			// Mark session complete with output
			if err := l.deps.DB.CompletePlanSession(sessionID, db.PlanSessionCompleted, finalOutput); err != nil {
				log.Warn("failed to complete session", "error", err)
			}

			// Distill and commit
			l.distillAndCommit(ctx, sessionID, finalOutput)

			return true, nil // Done!
		}
	}

	// Store progress if present
	if parseResult.Progress != "" {
		progressRecord := &db.Progress{
			PlanID:    l.cfg.PlanID,
			SessionID: sessionID,
			Content:   parseResult.Progress,
		}
		if err := l.deps.DB.CreateProgress(progressRecord); err != nil {
			log.Warn("failed to store progress", "error", err)
		}
	}

	// Store learnings if present
	if parseResult.Learnings != "" {
		learningsRecord := &db.Learnings{
			PlanID:    l.cfg.PlanID,
			SessionID: sessionID,
			Content:   parseResult.Learnings,
		}
		if err := l.deps.DB.CreateLearnings(learningsRecord); err != nil {
			log.Warn("failed to store learnings", "error", err)
		}
	}

	// 8. Mark session complete before distillation (so session is recorded even if distill hangs)
	if err := l.deps.DB.CompletePlanSession(sessionID, db.PlanSessionCompleted, finalOutput); err != nil {
		log.Warn("failed to complete session", "error", err)
	}

	// 9. Distill commit message and commit
	l.distillAndCommit(ctx, sessionID, finalOutput)

	l.emit(NewEvent(EventIterationEnd, l.iteration, l.cfg.MaxIterations,
		fmt.Sprintf("Completed iteration %d", l.iteration)))

	return false, nil
}

// distillAndCommit distills a commit message and runs jj commit.
func (l *Loop) distillAndCommit(ctx context.Context, sessionID, output string) {
	// Get the diff for context
	diff, err := l.deps.JJ.Show(ctx)
	if err != nil {
		log.Warn("failed to get diff for distillation", "error", err)
		diff = "" // Continue without diff
	}

	l.emit(NewEvent(EventDistilling, l.iteration, l.cfg.MaxIterations, "Distilling commit message"))

	commitMsg, err := l.deps.Distiller.Distill(ctx, output, diff)
	if err != nil {
		log.Warn("distillation failed, using fallback", "error", err)
		// commitMsg already contains fallback from Distill
	}

	l.emit(NewEvent(EventJJCommit, l.iteration, l.cfg.MaxIterations,
		fmt.Sprintf("Committing: %s", commitMsg)))

	if err := l.deps.JJ.Commit(ctx, commitMsg); err != nil {
		log.Warn("jj commit failed", "error", err)
		l.emit(NewErrorEvent(l.iteration, l.cfg.MaxIterations,
			fmt.Errorf("jj commit failed: %w", err)))
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

// =============================================================================
// V1.5 Dual-Agent Loop
// =============================================================================

// runV15Iteration runs a single V1.5 iteration with developer and optional reviewer.
// Returns (done, error) where done indicates both developer and reviewer approved.
func (l *Loop) runV15Iteration(ctx context.Context) (bool, error) {
	l.emit(NewEvent(EventIterationStart, l.iteration, l.cfg.MaxIterations,
		fmt.Sprintf("Starting V1.5 iteration %d", l.iteration)))

	// 1. Load state
	progress, learnings, feedback, err := l.loadV15State()
	if err != nil {
		return false, err
	}

	// 2. Run developer agent
	l.emit(NewEvent(EventDeveloperStart, l.iteration, l.cfg.MaxIterations, "Starting developer agent"))

	devOutput, devHadEdits, devSessionID, err := l.runV15Developer(ctx, progress, learnings, feedback)
	if err != nil {
		return false, fmt.Errorf("developer agent failed: %w", err)
	}

	l.emit(NewEvent(EventDeveloperEnd, l.iteration, l.cfg.MaxIterations, "Developer agent ended"))

	// 3. Parse developer output
	devResult := parser.ParseV15Output(devOutput, "developer")

	// 4. Store developer progress/learnings
	l.storeV15ProgressLearnings(devSessionID, devResult.Progress, devResult.Learnings)

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
		l.distillAndCommit(ctx, devSessionID, devOutput)
		l.emit(NewEvent(EventIterationEnd, l.iteration, l.cfg.MaxIterations,
			fmt.Sprintf("Developer iteration %d complete, continuing", l.iteration)))
		return false, nil
	}

	// Developer done without edits - run reviewer
	l.emit(NewEvent(EventDeveloperDone, l.iteration, l.cfg.MaxIterations,
		"Developer signaled DEV_DONE, triggering reviewer"))

	// 7. Get diff for reviewer
	diff, err := l.deps.JJ.Show(ctx)
	if err != nil {
		log.Warn("failed to get diff for reviewer", "error", err)
		diff = ""
	}

	// 8. Run reviewer agent
	l.emit(NewEvent(EventReviewerStart, l.iteration, l.cfg.MaxIterations, "Starting reviewer agent"))

	reviewOutput, reviewSessionID, err := l.runV15Reviewer(ctx, progress, learnings, diff, devOutput)
	if err != nil {
		return false, fmt.Errorf("reviewer agent failed: %w", err)
	}

	l.emit(NewEvent(EventReviewerEnd, l.iteration, l.cfg.MaxIterations, "Reviewer agent ended"))

	// 9. Parse reviewer output
	reviewResult := parser.ParseV15Output(reviewOutput, "reviewer")

	// 10. Store reviewer progress/learnings
	l.storeV15ProgressLearnings(reviewSessionID, reviewResult.Progress, reviewResult.Learnings)

	// 11. Check reviewer verdict
	if reviewResult.ReviewerApproved {
		// BOTH voted DONE - complete!
		l.emit(NewEvent(EventReviewerApproved, l.iteration, l.cfg.MaxIterations,
			"Reviewer approved - implementation complete"))
		l.emit(NewEvent(EventBothDone, l.iteration, l.cfg.MaxIterations,
			"Both developer and reviewer approved"))

		if err := l.deps.DB.UpdatePlanStatus(l.cfg.PlanID, db.PlanStatusCompleted); err != nil {
			log.Warn("failed to mark plan complete", "error", err)
		}

		l.distillAndCommit(ctx, reviewSessionID, reviewOutput)
		return true, nil
	}

	// 12. Reviewer has feedback - store for next iteration
	l.emit(NewEvent(EventReviewerFeedback, l.iteration, l.cfg.MaxIterations,
		fmt.Sprintf("Reviewer feedback: %s", truncateString(reviewResult.ReviewerFeedback, 100))))

	if err := l.storeReviewerFeedback(reviewSessionID, reviewResult.ReviewerFeedback); err != nil {
		log.Warn("failed to store reviewer feedback", "error", err)
	}

	l.distillAndCommit(ctx, reviewSessionID, reviewOutput)

	l.emit(NewEvent(EventIterationEnd, l.iteration, l.cfg.MaxIterations,
		fmt.Sprintf("V1.5 iteration %d complete with reviewer feedback", l.iteration)))

	return false, nil
}

// loadV15State loads progress, learnings, and last reviewer feedback.
func (l *Loop) loadV15State() (progress, learnings, feedback string, err error) {
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

// runV15Developer runs the developer agent and returns output, whether edits occurred, and session ID.
func (l *Loop) runV15Developer(ctx context.Context, progress, learnings, feedback string) (output string, hadEdits bool, sessionID string, err error) {
	// Build developer prompt
	prompt, err := agent.BuildV15DeveloperPrompt(agent.V15DeveloperContext{
		PlanContent:      l.plan.Content,
		Progress:         progress,
		Learnings:        learnings,
		ReviewerFeedback: feedback,
	})
	if err != nil {
		return "", false, "", fmt.Errorf("failed to build developer prompt: %w", err)
	}

	l.emit(NewPromptBuiltEvent(l.iteration, l.cfg.MaxIterations, prompt))

	// Run jj new only if current change has content
	isEmpty, err := l.deps.JJ.IsEmpty(ctx)
	if err != nil {
		log.Warn("failed to check if change is empty", "error", err)
		isEmpty = false
	}

	if !isEmpty {
		l.emit(NewEvent(EventJJNew, l.iteration, l.cfg.MaxIterations, "Creating new jj change"))
		if err := l.deps.JJ.New(ctx); err != nil {
			log.Warn("jj new failed", "error", err)
		}
	}

	// Create session in DB
	sessionID = uuid.New().String()
	session := &db.PlanSession{
		ID:          sessionID,
		PlanID:      l.cfg.PlanID,
		Iteration:   l.iteration,
		InputPrompt: prompt,
		Status:      db.PlanSessionRunning,
		AgentType:   db.V15AgentDeveloper,
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

// runV15Reviewer runs the reviewer agent and returns output and session ID.
func (l *Loop) runV15Reviewer(ctx context.Context, progress, learnings, diff, devSummary string) (output string, sessionID string, err error) {
	// Build reviewer prompt
	prompt, err := agent.BuildV15ReviewerPrompt(agent.V15ReviewerContext{
		PlanContent:      l.plan.Content,
		Progress:         progress,
		Learnings:        learnings,
		DiffOutput:       diff,
		DeveloperSummary: devSummary,
	})
	if err != nil {
		return "", "", fmt.Errorf("failed to build reviewer prompt: %w", err)
	}

	l.emit(NewPromptBuiltEvent(l.iteration, l.cfg.MaxIterations, prompt))

	// Create session in DB
	sessionID = uuid.New().String()
	session := &db.PlanSession{
		ID:          sessionID,
		PlanID:      l.cfg.PlanID,
		Iteration:   l.iteration,
		InputPrompt: prompt,
		Status:      db.PlanSessionRunning,
		AgentType:   db.V15AgentReviewer,
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
	l.emit(NewEvent(EventClaudeStart, l.iteration, l.cfg.MaxIterations, "Starting Claude session"))

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
	for claudeEvent := range claudeSession.Events() {
		// Track if any edit tools are used
		if claudeEvent.Type == claude.EventToolUse && claudeEvent.ToolUse != nil {
			if isEditTool(claudeEvent.ToolUse.Name) {
				hadEdits = true
			}
		}

		// Emit to our event channel
		eventCopy := claudeEvent
		l.emit(NewClaudeStreamEvent(l.iteration, l.cfg.MaxIterations, &eventCopy))

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
	l.emit(NewClaudeOutputEvent(l.iteration, l.cfg.MaxIterations, output))
	l.emit(NewEvent(EventClaudeEnd, l.iteration, l.cfg.MaxIterations, "Claude session ended"))

	// Mark session complete
	if err := l.deps.DB.CompletePlanSession(sessionID, db.PlanSessionCompleted, output); err != nil {
		log.Warn("failed to complete session", "error", err)
	}

	return output, hadEdits, nil
}

// storeV15ProgressLearnings stores progress and learnings from a V1.5 agent session.
func (l *Loop) storeV15ProgressLearnings(sessionID, progress, learnings string) {
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
