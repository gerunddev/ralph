// Package engine provides the main execution engine for Ralph.
package engine

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/google/uuid"

	"github.com/gerund/ralph/internal/agents"
	"github.com/gerund/ralph/internal/claude"
	"github.com/gerund/ralph/internal/config"
	"github.com/gerund/ralph/internal/db"
	"github.com/gerund/ralph/internal/jj"
	"github.com/gerund/ralph/internal/log"
)

// engineEventChannelBufferSize is the buffer size for the engine event channel.
// Large buffer needed to prevent dropping Claude streaming events.
const engineEventChannelBufferSize = 10000

// EngineEventType represents the type of engine event.
type EngineEventType string

const (
	// EngineEventCreatingProject is emitted when the engine starts creating a project.
	EngineEventCreatingProject EngineEventType = "creating_project"
	// EngineEventPlanningTasks is emitted when the planner agent starts.
	EngineEventPlanningTasks EngineEventType = "planning_tasks"
	// EngineEventTasksCreated is emitted when tasks have been created from the plan.
	EngineEventTasksCreated EngineEventType = "tasks_created"
	// EngineEventRunning is emitted when the engine is running tasks.
	EngineEventRunning EngineEventType = "running"
	// EngineEventCompleted is emitted when all tasks are completed.
	EngineEventCompleted EngineEventType = "completed"
	// EngineEventFailed is emitted when the engine fails.
	EngineEventFailed EngineEventType = "failed"
	// EngineEventCapturingLearnings is emitted when learnings capture starts.
	EngineEventCapturingLearnings EngineEventType = "capturing_learnings"
	// EngineEventLearningsCaptured is emitted when learnings have been captured.
	EngineEventLearningsCaptured EngineEventType = "learnings_captured"
)

// EngineEvent represents an event during engine execution.
type EngineEvent struct {
	Type          EngineEventType
	Message       string
	TaskLoopEvent *TaskLoopEvent // Nested events from task loop
}

// EngineConfig holds configuration for creating an Engine.
type EngineConfig struct {
	Config  *config.Config
	DB      *db.DB
	WorkDir string // Working directory for jj client
}

// Engine orchestrates task execution and the developer->reviewer loop.
type Engine struct {
	config *config.Config
	db     *db.DB
	claude *claude.Client
	jj     *jj.Client
	agents *agents.Manager

	events   chan EngineEvent
	project  *db.Project
	taskLoop *TaskLoop
	mu       sync.RWMutex
	stopped  bool
}

// NewEngine creates a new execution engine.
func NewEngine(cfg EngineConfig) (*Engine, error) {
	if cfg.Config == nil {
		return nil, fmt.Errorf("config is required")
	}
	if cfg.DB == nil {
		return nil, fmt.Errorf("database is required")
	}
	if cfg.WorkDir == "" {
		return nil, fmt.Errorf("work directory is required")
	}

	claudeClient := claude.NewClient(claude.ClientConfig{
		Model:    cfg.Config.Claude.Model,
		MaxTurns: cfg.Config.Claude.MaxTurns,
		Verbose:  cfg.Config.Claude.Verbose,
	})

	jjClient := jj.NewClient(cfg.WorkDir)

	agentsManager := agents.NewManager(cfg.Config)

	return &Engine{
		config: cfg.Config,
		db:     cfg.DB,
		claude: claudeClient,
		jj:     jjClient,
		agents: agentsManager,
		events: make(chan EngineEvent, engineEventChannelBufferSize),
	}, nil
}

// Events returns a channel that emits engine events.
// The channel is closed when the engine stops.
func (e *Engine) Events() <-chan EngineEvent {
	return e.events
}

// Project returns the currently loaded project, if any.
func (e *Engine) Project() *db.Project {
	e.mu.RLock()
	defer e.mu.RUnlock()
	return e.project
}

// emit sends an event to the events channel.
func (e *Engine) emit(eventType EngineEventType, message string) {
	e.mu.RLock()
	stopped := e.stopped
	e.mu.RUnlock()

	if stopped {
		return
	}

	event := EngineEvent{
		Type:    eventType,
		Message: message,
	}

	select {
	case e.events <- event:
	default:
		log.Warn("engine event dropped: channel full",
			"event_type", eventType,
			"message", message)
	}
}

// emitWithTaskLoop sends an event that wraps a task loop event.
func (e *Engine) emitWithTaskLoop(taskLoopEvent *TaskLoopEvent) {
	e.mu.RLock()
	stopped := e.stopped
	e.mu.RUnlock()

	if stopped {
		return
	}

	event := EngineEvent{
		Type:          EngineEventRunning,
		TaskLoopEvent: taskLoopEvent,
	}

	select {
	case e.events <- event:
	default:
		log.Warn("engine task loop event dropped: channel full",
			"task_event_type", taskLoopEvent.Type,
			"task_index", taskLoopEvent.TaskIndex,
			"task_title", taskLoopEvent.TaskTitle)
	}
}

// CreateProject reads a plan file, creates a project, and runs the planner to create tasks.
func (e *Engine) CreateProject(ctx context.Context, planPath string) (*db.Project, error) {
	e.emit(EngineEventCreatingProject, "Reading plan file")

	// Read plan file
	planText, err := os.ReadFile(planPath)
	if err != nil {
		return nil, fmt.Errorf("failed to read plan file: %w", err)
	}

	// Create project
	project := &db.Project{
		ID:       uuid.New().String(),
		Name:     filepath.Base(planPath),
		PlanText: string(planText),
		Status:   db.ProjectPending,
	}

	if err := e.db.CreateProject(project); err != nil {
		return nil, fmt.Errorf("failed to create project: %w", err)
	}

	// Run planner to break into tasks
	e.emit(EngineEventPlanningTasks, "Breaking plan into tasks")
	if err := e.planTasks(ctx, project); err != nil {
		return nil, fmt.Errorf("failed to plan tasks: %w", err)
	}

	e.mu.Lock()
	e.project = project
	e.mu.Unlock()

	return project, nil
}

// planTasks runs the planner agent to break down the plan into tasks.
func (e *Engine) planTasks(ctx context.Context, project *db.Project) error {
	// Get planner agent
	agent, err := e.agents.GetPlannerAgent(ctx, project.PlanText)
	if err != nil {
		return fmt.Errorf("failed to get planner agent: %w", err)
	}

	// Run Claude with planner prompt
	session, err := e.claude.Run(ctx, agent.Prompt)
	if err != nil {
		return fmt.Errorf("failed to run planner: %w", err)
	}

	// Collect output
	var output strings.Builder
	for event := range session.Events() {
		if event.Message != nil {
			output.WriteString(event.Message.Text)
		}
		// Also capture final result text
		if event.Result != nil {
			output.WriteString(event.Result.Result)
		}
	}

	if err := session.Wait(); err != nil {
		return fmt.Errorf("planner session failed: %w", err)
	}

	// Parse JSON tasks from output
	tasks, err := parsePlannerOutput(output.String())
	if err != nil {
		return fmt.Errorf("failed to parse planner output: %w", err)
	}

	// Create task records
	dbTasks := make([]*db.Task, len(tasks))
	for i, t := range tasks {
		dbTasks[i] = &db.Task{
			ID:          uuid.New().String(),
			ProjectID:   project.ID,
			Sequence:    t.Sequence,
			Title:       t.Title,
			Description: t.Description,
			Status:      db.TaskPending,
		}
	}

	if err := e.db.CreateTasks(dbTasks); err != nil {
		return fmt.Errorf("failed to create tasks: %w", err)
	}

	e.emit(EngineEventTasksCreated, fmt.Sprintf("Created %d tasks", len(dbTasks)))
	return nil
}

// plannerTask represents a task parsed from planner output.
type plannerTask struct {
	Title       string `json:"title"`
	Description string `json:"description"`
	Sequence    int    `json:"sequence"`
}

// parsePlannerOutput parses the planner's JSON output into tasks.
func parsePlannerOutput(output string) ([]plannerTask, error) {
	// Find JSON array in output
	start := strings.Index(output, "[")
	end := strings.LastIndex(output, "]")
	if start == -1 || end == -1 || start >= end {
		return nil, fmt.Errorf("no JSON array found in planner output")
	}

	var tasks []plannerTask
	if err := json.Unmarshal([]byte(output[start:end+1]), &tasks); err != nil {
		return nil, fmt.Errorf("failed to parse JSON: %w", err)
	}

	if len(tasks) == 0 {
		return nil, fmt.Errorf("planner returned no tasks")
	}

	return tasks, nil
}

// ResumeProject loads an existing project by ID.
func (e *Engine) ResumeProject(ctx context.Context, projectID string) (*db.Project, error) {
	project, err := e.db.GetProject(projectID)
	if err != nil {
		return nil, fmt.Errorf("failed to load project: %w", err)
	}

	e.mu.Lock()
	e.project = project
	e.mu.Unlock()

	return project, nil
}

// Run executes the full workflow for the loaded project.
func (e *Engine) Run(ctx context.Context) error {
	e.mu.RLock()
	project := e.project
	stopped := e.stopped
	e.mu.RUnlock()

	if stopped {
		return fmt.Errorf("engine has been stopped")
	}

	if project == nil {
		return fmt.Errorf("no project loaded")
	}

	e.emit(EngineEventRunning, "Starting task execution")

	// Create task loop
	taskLoop := NewTaskLoop(TaskLoopDeps{
		DB:     e.db,
		Claude: e.claude,
		JJ:     e.jj,
		Agents: e.agents,
		Config: e.config,
	}, project)

	// Store task loop for external access (pause control)
	e.mu.Lock()
	e.taskLoop = taskLoop
	e.mu.Unlock()

	// Forward task loop events
	go func() {
		for event := range taskLoop.Events() {
			e.emitWithTaskLoop(&event)
		}
	}()

	result, err := taskLoop.Run(ctx)

	// Clear task loop reference
	e.mu.Lock()
	e.taskLoop = nil
	e.mu.Unlock()
	if err != nil {
		e.emit(EngineEventFailed, err.Error())
		return err
	}

	e.emit(EngineEventCompleted, fmt.Sprintf("All tasks completed (%d completed, %d failed, %d skipped)",
		result.Completed, result.Failed, result.Skipped))
	return nil
}

// Stop gracefully stops the engine.
func (e *Engine) Stop() error {
	e.mu.Lock()
	defer e.mu.Unlock()

	if e.stopped {
		return nil
	}

	e.stopped = true
	close(e.events)
	return nil
}

// DB returns the engine's database connection.
// This is useful for callers that need to query projects or tasks.
func (e *Engine) DB() *db.DB {
	return e.db
}

// Claude returns the engine's Claude client.
// This is exposed for testing purposes.
func (e *Engine) Claude() *claude.Client {
	return e.claude
}

// JJ returns the engine's JJ client.
// This is exposed for testing purposes.
func (e *Engine) JJ() *jj.Client {
	return e.jj
}

// Agents returns the engine's agent manager.
// This is exposed for testing purposes.
func (e *Engine) Agents() *agents.Manager {
	return e.agents
}

// TaskLoop returns the currently running task loop, or nil if not running.
// This can be used to control pause mode during execution.
func (e *Engine) TaskLoop() *TaskLoop {
	e.mu.RLock()
	defer e.mu.RUnlock()
	return e.taskLoop
}

// LearningsOutput holds the parsed documentation updates from the documenter agent.
type LearningsOutput struct {
	AgentsMD string
	ReadmeMD string
}

// CaptureLearnings runs the documentation agent to capture learnings from the session.
func (e *Engine) CaptureLearnings(ctx context.Context) error {
	e.emit(EngineEventCapturingLearnings, "Analyzing changes for documentation")

	e.mu.RLock()
	project := e.project
	e.mu.RUnlock()

	if project == nil {
		return fmt.Errorf("no project loaded")
	}

	// Get all completed tasks
	tasks, err := e.db.GetTasksByProject(project.ID)
	if err != nil {
		return fmt.Errorf("failed to get tasks: %w", err)
	}

	completedTasks := filterCompleted(tasks)
	if len(completedTasks) == 0 {
		e.emit(EngineEventLearningsCaptured, "No completed tasks to document")
		return nil
	}

	// Get combined diff/changes
	changesSummary, err := e.getAllChanges(ctx)
	if err != nil {
		changesSummary = "Unable to retrieve changes: " + err.Error()
	}

	// Create documenter agent
	agent, err := e.agents.GetDocumenterAgent(ctx, changesSummary, completedTasks)
	if err != nil {
		return fmt.Errorf("failed to create documenter agent: %w", err)
	}

	// Run Claude
	session, err := e.claude.Run(ctx, agent.Prompt)
	if err != nil {
		return fmt.Errorf("failed to run documenter: %w", err)
	}

	// Collect output
	var output strings.Builder
	for event := range session.Events() {
		if event.Message != nil {
			output.WriteString(event.Message.Text)
		}
		if event.Result != nil {
			output.WriteString(event.Result.Result)
		}
	}

	if err := session.Wait(); err != nil {
		return fmt.Errorf("documenter session failed: %w", err)
	}

	// Parse output
	learnings, err := parseLearningsOutput(output.String())
	if err != nil {
		return fmt.Errorf("failed to parse learnings: %w", err)
	}

	// Apply learnings
	if err := e.applyLearnings(learnings); err != nil {
		return fmt.Errorf("failed to apply learnings: %w", err)
	}

	// Commit changes via jj
	if err := e.jj.Describe(ctx, "docs: capture learnings from development session"); err != nil {
		return fmt.Errorf("failed to commit learnings: %w", err)
	}

	// Update project learnings state
	if err := e.db.UpdateProjectLearningsState(project.ID, db.LearningsStateComplete); err != nil {
		return fmt.Errorf("failed to update learnings state: %w", err)
	}

	e.emit(EngineEventLearningsCaptured, "Documentation updated")
	return nil
}

// filterCompleted returns only the completed tasks from the given list.
func filterCompleted(tasks []*db.Task) []*db.Task {
	var completed []*db.Task
	for _, t := range tasks {
		if t.Status == db.TaskCompleted {
			completed = append(completed, t)
		}
	}
	return completed
}

// getAllChanges retrieves the combined changes from the jj history.
func (e *Engine) getAllChanges(ctx context.Context) (string, error) {
	// Get log with patches for all changes
	// Use a revset that shows recent changes (last 20 or so)
	output, err := e.jj.Log(ctx, "..@", "")
	if err != nil {
		return "", err
	}
	return output, nil
}

// parseLearningsOutput parses the documenter's output into structured learnings.
func parseLearningsOutput(output string) (*LearningsOutput, error) {
	learnings := &LearningsOutput{}

	// Find AGENTS.md content
	agentsStart := strings.Index(output, "### AGENTS.md Content")
	readmeStart := strings.Index(output, "### README.md Content")

	if agentsStart != -1 && readmeStart != -1 {
		agentsSection := output[agentsStart:readmeStart]
		learnings.AgentsMD = extractCodeBlock(agentsSection)

		readmeSection := output[readmeStart:]
		learnings.ReadmeMD = extractCodeBlock(readmeSection)
	} else if agentsStart != -1 {
		// Only AGENTS.md content found
		agentsSection := output[agentsStart:]
		learnings.AgentsMD = extractCodeBlock(agentsSection)
	} else if readmeStart != -1 {
		// Only README.md content found
		readmeSection := output[readmeStart:]
		learnings.ReadmeMD = extractCodeBlock(readmeSection)
	}

	return learnings, nil
}

// extractCodeBlock extracts content from a markdown code block.
func extractCodeBlock(text string) string {
	// Look for ```markdown first, then just ```
	start := strings.Index(text, "```markdown")
	if start == -1 {
		start = strings.Index(text, "```")
	}
	if start == -1 {
		return ""
	}

	// Find the newline after the opening fence
	start = strings.Index(text[start:], "\n")
	if start == -1 {
		return ""
	}
	start++ // Move past the newline

	// Find the opening fence position to calculate the offset
	fenceStart := strings.Index(text, "```")
	contentStart := fenceStart + start

	// Find the closing fence
	end := strings.Index(text[contentStart:], "```")
	if end == -1 {
		return ""
	}

	return strings.TrimSpace(text[contentStart : contentStart+end])
}

// applyLearnings writes the learnings to the appropriate files.
func (e *Engine) applyLearnings(learnings *LearningsOutput) error {
	if learnings.AgentsMD != "" {
		if err := appendToFile("AGENTS.md", learnings.AgentsMD); err != nil {
			return fmt.Errorf("failed to update AGENTS.md: %w", err)
		}
	}

	if learnings.ReadmeMD != "" {
		if err := appendToFile("README.md", learnings.ReadmeMD); err != nil {
			return fmt.Errorf("failed to update README.md: %w", err)
		}
	}

	return nil
}

// appendToFile appends content to a file with a session separator.
// Uses named return to ensure close errors are not lost (important for data integrity).
func appendToFile(path string, content string) (err error) {
	f, err := os.OpenFile(path, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		return err
	}
	defer func() {
		closeErr := f.Close()
		if closeErr != nil {
			log.Warn("failed to close file", "path", path, "error", closeErr)
			// If we haven't already returned an error, return the close error
			if err == nil {
				err = closeErr
			}
		}
	}()

	// Add separator with date
	separator := fmt.Sprintf("\n\n---\n\n## Session: %s\n\n", time.Now().Format("2006-01-02"))

	if _, err := f.WriteString(separator + content); err != nil {
		return err
	}

	return nil
}
