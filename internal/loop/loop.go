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

// Config holds configuration for the loop.
type Config struct {
	PlanID          string
	MaxIterations   int
	WorkDir         string // For jj operations
	EventBufferSize int    // Size of event channel buffer (default: 1000)
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

		// Run one iteration
		done, err := l.runIteration(ctx)
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

	// 3. Run jj new
	l.emit(NewEvent(EventJJNew, l.iteration, l.cfg.MaxIterations, "Creating new jj change"))
	if err := l.deps.JJ.New(ctx); err != nil {
		// Log but continue - jj errors shouldn't stop the loop
		log.Warn("jj new failed", "error", err)
		l.emit(NewErrorEvent(l.iteration, l.cfg.MaxIterations,
			fmt.Errorf("jj new failed: %w", err)))
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
	sequence := 0
	for claudeEvent := range claudeSession.Events() {
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
		// Mark session complete with output
		if err := l.deps.DB.CompletePlanSession(sessionID, db.PlanSessionCompleted, finalOutput); err != nil {
			log.Warn("failed to complete session", "error", err)
		}

		// Distill and commit
		l.distillAndCommit(ctx, sessionID, finalOutput)

		return true, nil // Done!
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
	l.emit(NewEvent(EventDistilling, l.iteration, l.cfg.MaxIterations, "Distilling commit message"))

	commitMsg, err := l.deps.Distiller.Distill(ctx, output)
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
