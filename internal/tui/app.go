// Package tui provides the Bubble Tea TUI application for Ralph.
package tui

import (
	"context"
	"fmt"
	"strings"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/gerund/ralph/internal/config"
	"github.com/gerund/ralph/internal/db"
	"github.com/gerund/ralph/internal/engine"
	"github.com/gerund/ralph/internal/log"
)

// createModeState represents the state of create mode.
type createModeState int

const (
	createModeProgress createModeState = iota
	createModeFeedbackPrompt
	createModeFeedbackInstructions
	createModeCapturingLearnings
	createModeCompleted
)

// CreateModeModel wraps TaskProgressModel for create mode (ralph -c).
// It handles quit behavior, feedback flow, and serves as the top-level model.
type CreateModeModel struct {
	state                createModeState
	progress             TaskProgressModel
	feedbackPrompt       FeedbackPromptModel
	feedbackInstructions FeedbackInstructionsModel
	learnings            LearningsModel
	project              *db.Project
	db                   *db.DB
	engine               *engine.Engine
	width                int
	height               int
}

// NewCreateModeModel creates a new model for create mode.
func NewCreateModeModel(progress TaskProgressModel, project *db.Project, database *db.DB, eng *engine.Engine) CreateModeModel {
	return CreateModeModel{
		state:    createModeProgress,
		progress: progress,
		project:  project,
		db:       database,
		engine:   eng,
	}
}

// Init implements tea.Model.
func (m CreateModeModel) Init() tea.Cmd {
	return m.progress.Init()
}

// Update implements tea.Model.
func (m CreateModeModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "q", "ctrl+c":
			return m, tea.Quit
		}

	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		switch m.state {
		case createModeProgress:
			m.progress, _ = m.progress.Update(msg)
		case createModeFeedbackPrompt:
			m.feedbackPrompt, _ = m.feedbackPrompt.Update(msg)
		case createModeFeedbackInstructions:
			m.feedbackInstructions, _ = m.feedbackInstructions.Update(msg)
		case createModeCapturingLearnings:
			m.learnings, _ = m.learnings.Update(msg)
		}
		return m, nil

	case EngineEventsClosedMsg:
		// Forward to task progress to update its state
		m.progress, _ = m.progress.Update(msg)

		// Engine event channel closed - check if we should show feedback prompt
		if m.progress.IsCompleted() && m.project != nil {
			return m, m.checkFeedbackState()
		}
		return m, nil

	case FeedbackStateCheckedMsg:
		if msg.Err != nil {
			log.Warn("failed to check feedback state", "error", msg.Err)
			return m, nil
		}

		switch msg.State {
		case db.FeedbackStateNone, db.FeedbackStateProvided:
			// Don't reset PROVIDED state here - wait until user makes a choice
			// This preserves state if user quits mid-prompt
			m.feedbackPrompt = NewFeedbackPromptModel(m.project.ID)
			m.state = createModeFeedbackPrompt
			return m, func() tea.Msg {
				return tea.WindowSizeMsg{Width: m.width, Height: m.height}
			}

		case db.FeedbackStatePending:
			m.feedbackInstructions = NewFeedbackInstructionsModel(m.project.ID)
			m.state = createModeFeedbackInstructions
			return m, func() tea.Msg {
				return tea.WindowSizeMsg{Width: m.width, Height: m.height}
			}
		}
		return m, nil

	case FeedbackChoiceMsg:
		if msg.Complete {
			if m.project != nil && m.db != nil {
				// Reset PROVIDED state and set to COMPLETE
				if err := m.db.UpdateProjectFeedbackState(m.project.ID, db.FeedbackStateComplete); err != nil {
					log.Warn("failed to update feedback state to complete", "error", err)
				}
			}
			// Check if we need to capture learnings
			return m, m.checkLearningsState()
		}
		if m.project != nil && m.db != nil {
			if err := m.db.UpdateProjectFeedbackState(m.project.ID, db.FeedbackStatePending); err != nil {
				log.Warn("failed to update feedback state to pending", "error", err)
			}
		}
		m.feedbackInstructions = NewFeedbackInstructionsModel(m.project.ID)
		m.state = createModeFeedbackInstructions
		return m, func() tea.Msg {
			return tea.WindowSizeMsg{Width: m.width, Height: m.height}
		}

	case LearningsStateCheckedMsg:
		if msg.Err != nil {
			log.Warn("failed to check learnings state", "error", msg.Err)
			// Still complete the project
			if m.project != nil && m.db != nil {
				if err := m.db.UpdateProjectStatus(m.project.ID, db.ProjectCompleted); err != nil {
					log.Warn("failed to update project status to completed", "error", err)
				}
			}
			return m, tea.Quit
		}

		if msg.State == db.LearningsStateComplete {
			// Learnings already captured, mark project complete and quit
			if m.project != nil && m.db != nil {
				if err := m.db.UpdateProjectStatus(m.project.ID, db.ProjectCompleted); err != nil {
					log.Warn("failed to update project status to completed", "error", err)
				}
			}
			return m, tea.Quit
		}

		// Start learnings capture
		m.learnings = NewLearningsModel(m.project)
		m.state = createModeCapturingLearnings
		return m, tea.Batch(
			func() tea.Msg {
				return tea.WindowSizeMsg{Width: m.width, Height: m.height}
			},
			m.captureLearnings(),
		)

	case LearningsCapturedMsg:
		if msg.Err != nil {
			log.Warn("failed to capture learnings", "error", msg.Err)
		}
		// Mark project as completed
		if m.project != nil && m.db != nil {
			if err := m.db.UpdateProjectStatus(m.project.ID, db.ProjectCompleted); err != nil {
				log.Warn("failed to update project status to completed", "error", err)
			}
		}
		m.state = createModeCompleted
		return m, nil
	}

	// Delegate to current view
	switch m.state {
	case createModeProgress:
		var cmd tea.Cmd
		m.progress, cmd = m.progress.Update(msg)
		return m, cmd

	case createModeFeedbackPrompt:
		var cmd tea.Cmd
		m.feedbackPrompt, cmd = m.feedbackPrompt.Update(msg)
		return m, cmd

	case createModeFeedbackInstructions:
		var cmd tea.Cmd
		m.feedbackInstructions, cmd = m.feedbackInstructions.Update(msg)
		return m, cmd

	case createModeCapturingLearnings:
		var cmd tea.Cmd
		m.learnings, cmd = m.learnings.Update(msg)
		return m, cmd

	case createModeCompleted:
		// Handle quit on any key
		if _, ok := msg.(tea.KeyMsg); ok {
			return m, tea.Quit
		}
	}

	return m, nil
}

// checkFeedbackState checks the project's feedback state and returns a message.
func (m CreateModeModel) checkFeedbackState() tea.Cmd {
	return func() tea.Msg {
		if m.project == nil || m.db == nil {
			return FeedbackStateCheckedMsg{Err: nil}
		}

		project, err := m.db.GetProject(m.project.ID)
		if err != nil {
			return FeedbackStateCheckedMsg{Err: err}
		}

		return FeedbackStateCheckedMsg{State: project.UserFeedbackState}
	}
}

// checkLearningsState checks the project's learnings state and returns a message.
func (m CreateModeModel) checkLearningsState() tea.Cmd {
	return func() tea.Msg {
		if m.project == nil || m.db == nil {
			return LearningsStateCheckedMsg{Err: nil}
		}

		project, err := m.db.GetProject(m.project.ID)
		if err != nil {
			return LearningsStateCheckedMsg{Err: err}
		}

		return LearningsStateCheckedMsg{State: project.LearningsState}
	}
}

// captureLearnings starts the learnings capture process.
func (m CreateModeModel) captureLearnings() tea.Cmd {
	return func() tea.Msg {
		if m.engine == nil {
			return LearningsCapturedMsg{Err: nil}
		}

		ctx := context.Background()
		err := m.engine.CaptureLearnings(ctx)
		return LearningsCapturedMsg{Err: err}
	}
}

// View implements tea.Model.
func (m CreateModeModel) View() string {
	switch m.state {
	case createModeProgress:
		return m.progress.View()
	case createModeFeedbackPrompt:
		return m.feedbackPrompt.View()
	case createModeFeedbackInstructions:
		return m.feedbackInstructions.View()
	case createModeCapturingLearnings:
		return m.learnings.View()
	case createModeCompleted:
		return m.renderCompleted()
	default:
		return m.progress.View()
	}
}

// renderCompleted renders the completion screen.
func (m CreateModeModel) renderCompleted() string {
	var s strings.Builder

	s.WriteString(titleStyle.Render("Project Complete!"))
	s.WriteString("\n\n")

	s.WriteString("All tasks have been completed and learnings have been captured.\n\n")

	if m.project != nil {
		s.WriteString(fmt.Sprintf("Project: %s\n", m.project.Name))
	}

	s.WriteString("\n")
	s.WriteString(helpStyle.Render("Press any key to exit"))

	return s.String()
}

// ViewState represents which view is currently active.
type ViewState int

const (
	// ViewProjectList shows the project selection screen.
	ViewProjectList ViewState = iota
	// ViewResumeDialog shows the resume confirmation dialog.
	ViewResumeDialog
	// ViewCompletedProject shows a completed project with reset option.
	ViewCompletedProject
	// ViewTaskProgress shows the task execution progress.
	ViewTaskProgress
	// ViewFeedbackPrompt shows the feedback prompt after all tasks complete.
	ViewFeedbackPrompt
	// ViewFeedbackInstructions shows instructions for submitting feedback via CLI.
	ViewFeedbackInstructions
	// ViewCapturingLearnings shows the learnings capture progress.
	ViewCapturingLearnings
	// ViewCompleted shows the final completion screen.
	ViewCompleted
)

// Model is the main Bubble Tea model for the Ralph TUI.
type Model struct {
	state                ViewState
	projectList          ProjectListModel
	resumeDialog         ResumeModel
	completedProject     CompletedProjectModel
	taskProgress         TaskProgressModel
	feedbackPrompt       FeedbackPromptModel
	feedbackInstructions FeedbackInstructionsModel
	learnings            LearningsModel
	db                   *db.DB // Per-project database, opened when project is selected
	config               *config.Config
	workDir              string
	engine               *engine.Engine
	currentProject       *db.Project
	currentProjectInfo   *db.ProjectInfo // Info from discovery
	width                int
	height               int
}

// New creates a new TUI model with the given config and work directory.
func New(cfg *config.Config, workDir string) Model {
	return Model{
		state:       ViewProjectList,
		projectList: NewProjectListModel(cfg),
		config:      cfg,
		workDir:     workDir,
	}
}

// Init implements tea.Model.
func (m Model) Init() tea.Cmd {
	// Initialize the project list
	return m.projectList.Init()
}

// Update implements tea.Model.
func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		// Forward to current view
		switch m.state {
		case ViewProjectList:
			m.projectList, _ = m.projectList.Update(msg)
		case ViewResumeDialog:
			m.resumeDialog, _ = m.resumeDialog.Update(msg)
		case ViewCompletedProject:
			m.completedProject, _ = m.completedProject.Update(msg)
		case ViewTaskProgress:
			m.taskProgress, _ = m.taskProgress.Update(msg)
		case ViewFeedbackPrompt:
			m.feedbackPrompt, _ = m.feedbackPrompt.Update(msg)
		case ViewFeedbackInstructions:
			m.feedbackInstructions, _ = m.feedbackInstructions.Update(msg)
		case ViewCapturingLearnings:
			m.learnings, _ = m.learnings.Update(msg)
		}
		return m, nil

	case tea.KeyMsg:
		switch msg.String() {
		case "q", "ctrl+c":
			// Stop engine if running
			if m.engine != nil {
				if err := m.engine.Stop(); err != nil {
					log.Warn("failed to stop engine on quit", "error", err)
				}
			}
			// Close project database if open
			if m.db != nil {
				if err := m.db.Close(); err != nil {
					log.Warn("failed to close database on quit", "error", err)
				}
			}
			return m, tea.Quit
		}

	case ProjectSelectedMsg:
		m.currentProjectInfo = &msg.ProjectInfo

		// Open project-specific database
		database, err := db.OpenProjectDB(m.config.GetProjectsDir(), msg.ProjectInfo.ID)
		if err != nil {
			// Show error in task progress view
			m.taskProgress = NewTaskProgressModelWithError(nil, fmt.Errorf("failed to open project database: %w", err))
			m.state = ViewTaskProgress
			return m, func() tea.Msg {
				return tea.WindowSizeMsg{Width: m.width, Height: m.height}
			}
		}
		m.db = database

		// Load full project from database
		project, err := database.GetProject(msg.ProjectInfo.ID)
		if err != nil {
			if closeErr := m.db.Close(); closeErr != nil {
				log.Warn("failed to close database after project load failure", "error", closeErr)
			}
			m.db = nil
			m.taskProgress = NewTaskProgressModelWithError(nil, fmt.Errorf("failed to load project: %w", err))
			m.state = ViewTaskProgress
			return m, func() tea.Msg {
				return tea.WindowSizeMsg{Width: m.width, Height: m.height}
			}
		}
		m.currentProject = project

		// Create engine for the selected project
		eng, err := engine.NewEngine(engine.EngineConfig{
			Config:  m.config,
			DB:      m.db,
			WorkDir: m.workDir,
		})
		if err != nil {
			// Transition to progress view with error
			m.taskProgress = NewTaskProgressModelWithError(project, err)
			m.state = ViewTaskProgress
			return m, func() tea.Msg {
				return tea.WindowSizeMsg{Width: m.width, Height: m.height}
			}
		}
		m.engine = eng

		// Check project state to decide whether to show resume dialog
		return m, m.checkProjectState(project)

	case ShowResumeDialogMsg:
		// Show resume dialog for interrupted project
		m.resumeDialog = NewResumeModel(msg.State)
		m.state = ViewResumeDialog
		return m, func() tea.Msg {
			return tea.WindowSizeMsg{Width: m.width, Height: m.height}
		}

	case ProjectCompletedMsg:
		// Show completed project view
		state, err := m.engine.DetectProjectState(context.Background(), msg.Project.ID)
		if err != nil {
			state = &engine.ProjectState{Project: msg.Project}
		}
		m.completedProject = NewCompletedProjectModel(msg.Project, state)
		m.state = ViewCompletedProject
		return m, func() tea.Msg {
			return tea.WindowSizeMsg{Width: m.width, Height: m.height}
		}

	case ResumeConfirmedMsg:
		// User confirmed resume - cleanup and start
		return m, m.cleanupAndStart(msg.ProjectID)

	case ResetConfirmedMsg:
		// User confirmed reset - reset project and start
		return m, m.resetAndStart(msg.ProjectID)

	case StartProjectMsg:
		// Start the project - create progress model and run engine
		m.taskProgress = NewTaskProgressModel(msg.Project, m.db, m.engine.Events())
		m.state = ViewTaskProgress

		return m, tea.Batch(
			m.taskProgress.Init(),
			func() tea.Msg {
				return tea.WindowSizeMsg{Width: m.width, Height: m.height}
			},
			m.startEngine(msg.Project),
		)

	case EngineStartedMsg:
		// Engine started, nothing to do
		return m, nil

	case EngineErrorMsg:
		// Engine failed to start
		return m, nil

	case EngineEventsClosedMsg:
		// Forward to task progress to update its state
		m.taskProgress, _ = m.taskProgress.Update(msg)

		// Engine event channel closed - check if we should show feedback prompt
		if m.taskProgress.IsCompleted() && m.currentProject != nil {
			return m, m.checkFeedbackState()
		}
		return m, nil

	case FeedbackStateCheckedMsg:
		// Handle feedback state check result
		if msg.Err != nil {
			log.Warn("failed to check feedback state", "error", msg.Err)
			return m, nil
		}

		switch msg.State {
		case db.FeedbackStateNone, db.FeedbackStateProvided:
			// Don't reset PROVIDED state here - wait until user makes a choice
			// This preserves state if user quits mid-prompt
			// Show feedback prompt
			m.feedbackPrompt = NewFeedbackPromptModel(m.currentProject.ID)
			m.state = ViewFeedbackPrompt
			return m, func() tea.Msg {
				return tea.WindowSizeMsg{Width: m.width, Height: m.height}
			}

		case db.FeedbackStateComplete:
			// Already marked as complete, nothing to do
			return m, nil

		case db.FeedbackStatePending:
			// User selected "provide feedback" but hasn't submitted yet
			// Show instructions again
			m.feedbackInstructions = NewFeedbackInstructionsModel(m.currentProject.ID)
			m.state = ViewFeedbackInstructions
			return m, func() tea.Msg {
				return tea.WindowSizeMsg{Width: m.width, Height: m.height}
			}
		}
		return m, nil

	case FeedbackChoiceMsg:
		if msg.Complete {
			// User chose to mark review as complete
			// This also resets any PROVIDED state from a previously submitted feedback
			if m.currentProject != nil {
				if err := m.db.UpdateProjectFeedbackState(m.currentProject.ID, db.FeedbackStateComplete); err != nil {
					log.Warn("failed to update feedback state to complete", "error", err)
				}
			}
			// Check if we need to capture learnings
			return m, m.checkLearningsState()
		}
		// User chose to provide feedback
		// This resets any PROVIDED state to PENDING for the next feedback submission
		if m.currentProject != nil {
			if err := m.db.UpdateProjectFeedbackState(m.currentProject.ID, db.FeedbackStatePending); err != nil {
				log.Warn("failed to update feedback state to pending", "error", err)
			}
		}
		// Show feedback instructions
		m.feedbackInstructions = NewFeedbackInstructionsModel(m.currentProject.ID)
		m.state = ViewFeedbackInstructions
		return m, func() tea.Msg {
			return tea.WindowSizeMsg{Width: m.width, Height: m.height}
		}

	case LearningsStateCheckedMsg:
		if msg.Err != nil {
			log.Warn("failed to check learnings state", "error", msg.Err)
			// Still complete the project
			if m.currentProject != nil {
				if err := m.db.UpdateProjectStatus(m.currentProject.ID, db.ProjectCompleted); err != nil {
					log.Warn("failed to update project status to completed", "error", err)
				}
			}
			return m, tea.Quit
		}

		if msg.State == db.LearningsStateComplete {
			// Learnings already captured, mark project complete and quit
			if m.currentProject != nil {
				if err := m.db.UpdateProjectStatus(m.currentProject.ID, db.ProjectCompleted); err != nil {
					log.Warn("failed to update project status to completed", "error", err)
				}
			}
			return m, tea.Quit
		}

		// Start learnings capture
		m.learnings = NewLearningsModel(m.currentProject)
		m.state = ViewCapturingLearnings
		return m, tea.Batch(
			func() tea.Msg {
				return tea.WindowSizeMsg{Width: m.width, Height: m.height}
			},
			m.captureLearnings(),
		)

	case LearningsCapturedMsg:
		if msg.Err != nil {
			log.Warn("failed to capture learnings", "error", msg.Err)
		}
		// Mark project as completed
		if m.currentProject != nil {
			if err := m.db.UpdateProjectStatus(m.currentProject.ID, db.ProjectCompleted); err != nil {
				log.Warn("failed to update project status to completed", "error", err)
			}
		}
		m.state = ViewCompleted
		return m, nil
	}

	// Delegate to current view
	switch m.state {
	case ViewProjectList:
		var cmd tea.Cmd
		m.projectList, cmd = m.projectList.Update(msg)
		return m, cmd

	case ViewResumeDialog:
		var cmd tea.Cmd
		m.resumeDialog, cmd = m.resumeDialog.Update(msg)
		return m, cmd

	case ViewCompletedProject:
		var cmd tea.Cmd
		m.completedProject, cmd = m.completedProject.Update(msg)
		return m, cmd

	case ViewTaskProgress:
		var cmd tea.Cmd
		m.taskProgress, cmd = m.taskProgress.Update(msg)
		return m, cmd

	case ViewFeedbackPrompt:
		var cmd tea.Cmd
		m.feedbackPrompt, cmd = m.feedbackPrompt.Update(msg)
		return m, cmd

	case ViewFeedbackInstructions:
		var cmd tea.Cmd
		m.feedbackInstructions, cmd = m.feedbackInstructions.Update(msg)
		return m, cmd

	case ViewCapturingLearnings:
		var cmd tea.Cmd
		m.learnings, cmd = m.learnings.Update(msg)
		return m, cmd

	case ViewCompleted:
		// Handle quit on any key
		if _, ok := msg.(tea.KeyMsg); ok {
			return m, tea.Quit
		}
	}

	return m, nil
}

// FeedbackStateCheckedMsg is sent after checking the project's feedback state.
type FeedbackStateCheckedMsg struct {
	State db.UserFeedbackState
	Err   error
}

// LearningsStateCheckedMsg is sent after checking the project's learnings state.
type LearningsStateCheckedMsg struct {
	State db.LearningsState
	Err   error
}

// LearningsCapturingMsg is sent when learnings capture starts.
type LearningsCapturingMsg struct{}

// LearningsCapturedMsg is sent when learnings have been captured.
type LearningsCapturedMsg struct {
	Err error
}

// checkFeedbackState checks the project's feedback state and returns a message.
func (m Model) checkFeedbackState() tea.Cmd {
	return func() tea.Msg {
		if m.currentProject == nil {
			return FeedbackStateCheckedMsg{Err: nil}
		}

		// Refresh project from database to get latest state
		project, err := m.db.GetProject(m.currentProject.ID)
		if err != nil {
			return FeedbackStateCheckedMsg{Err: err}
		}

		return FeedbackStateCheckedMsg{State: project.UserFeedbackState}
	}
}

// checkLearningsState checks the project's learnings state and returns a message.
func (m Model) checkLearningsState() tea.Cmd {
	return func() tea.Msg {
		if m.currentProject == nil {
			return LearningsStateCheckedMsg{Err: nil}
		}

		// Refresh project from database to get latest state
		project, err := m.db.GetProject(m.currentProject.ID)
		if err != nil {
			return LearningsStateCheckedMsg{Err: err}
		}

		return LearningsStateCheckedMsg{State: project.LearningsState}
	}
}

// captureLearnings starts the learnings capture process.
func (m Model) captureLearnings() tea.Cmd {
	return func() tea.Msg {
		if m.engine == nil {
			return LearningsCapturedMsg{Err: nil}
		}

		ctx := context.Background()
		err := m.engine.CaptureLearnings(ctx)
		return LearningsCapturedMsg{Err: err}
	}
}

// startEngine starts the engine for the given project.
func (m Model) startEngine(project *db.Project) tea.Cmd {
	return func() tea.Msg {
		if m.engine == nil {
			return EngineErrorMsg{Err: nil}
		}

		// Resume project and run engine in a goroutine
		// Errors are communicated via the engine's event channel as EngineEventFailed events
		go func() {
			ctx := context.Background()
			if _, err := m.engine.ResumeProject(ctx, project.ID); err != nil {
				// ResumeProject failure - engine should emit EngineEventFailed
				// but ensure we stop cleanly
				_ = m.engine.Stop()
				return
			}
			// Run blocks until completion or error
			// Engine emits EngineEventCompleted or EngineEventFailed
			_ = m.engine.Run(ctx)
		}()

		return EngineStartedMsg{Engine: m.engine}
	}
}

// View implements tea.Model.
func (m Model) View() string {
	switch m.state {
	case ViewProjectList:
		return m.projectList.View()
	case ViewResumeDialog:
		return m.resumeDialog.View()
	case ViewCompletedProject:
		return m.completedProject.View()
	case ViewTaskProgress:
		return m.taskProgress.View()
	case ViewFeedbackPrompt:
		return m.feedbackPrompt.View()
	case ViewFeedbackInstructions:
		return m.feedbackInstructions.View()
	case ViewCapturingLearnings:
		return m.learnings.View()
	case ViewCompleted:
		return m.renderCompleted()
	default:
		return "Unknown view state\n"
	}
}

// renderCompleted renders the completion screen.
func (m Model) renderCompleted() string {
	var s strings.Builder

	s.WriteString(titleStyle.Render("Project Complete!"))
	s.WriteString("\n\n")

	s.WriteString("All tasks have been completed and learnings have been captured.\n\n")

	if m.currentProject != nil {
		s.WriteString(fmt.Sprintf("Project: %s\n", m.currentProject.Name))
	}

	s.WriteString("\n")
	s.WriteString(helpStyle.Render("Press any key to exit"))

	return s.String()
}

// checkProjectState checks the project state and determines the next action.
func (m Model) checkProjectState(project *db.Project) tea.Cmd {
	return func() tea.Msg {
		if m.engine == nil {
			return EngineErrorMsg{Err: fmt.Errorf("engine not initialized")}
		}

		ctx := context.Background()
		state, err := m.engine.DetectProjectState(ctx, project.ID)
		if err != nil {
			return EngineErrorMsg{Err: err}
		}

		// If project has interrupted work or needs cleanup, show resume dialog
		if state.HasInterruptedWork() || state.NeedsCleanup {
			return ShowResumeDialogMsg{State: state}
		}

		// If project is complete with no pending tasks, show completed view
		if state.IsComplete() {
			return ProjectCompletedMsg{Project: project}
		}

		// Otherwise start normally (this includes failed projects with failed tasks)
		return StartProjectMsg{Project: project}
	}
}

// StartProjectMsg signals to start a project directly.
type StartProjectMsg struct {
	Project *db.Project
}

// cleanupAndStart cleans up interrupted state and starts the engine.
func (m Model) cleanupAndStart(projectID string) tea.Cmd {
	return func() tea.Msg {
		if m.engine == nil {
			return EngineErrorMsg{Err: fmt.Errorf("engine not initialized")}
		}

		ctx := context.Background()

		// Detect state for cleanup
		state, err := m.engine.DetectProjectState(ctx, projectID)
		if err != nil {
			return EngineErrorMsg{Err: err}
		}

		// Cleanup interrupted state
		if err := m.engine.CleanupForResume(ctx, state); err != nil {
			return EngineErrorMsg{Err: err}
		}

		// Get fresh project
		project, err := m.db.GetProject(projectID)
		if err != nil {
			return EngineErrorMsg{Err: err}
		}

		return StartProjectMsg{Project: project}
	}
}

// resetAndStart resets the project and starts from scratch.
func (m Model) resetAndStart(projectID string) tea.Cmd {
	return func() tea.Msg {
		if m.engine == nil {
			return EngineErrorMsg{Err: fmt.Errorf("engine not initialized")}
		}

		ctx := context.Background()

		// Reset project to initial state
		if err := m.engine.ResetProject(ctx, projectID); err != nil {
			return EngineErrorMsg{Err: err}
		}

		// Get fresh project
		project, err := m.db.GetProject(projectID)
		if err != nil {
			return EngineErrorMsg{Err: err}
		}

		return StartProjectMsg{Project: project}
	}
}

// Run starts the TUI application with the given config and work directory.
func Run(cfg *config.Config, workDir string) error {
	p := tea.NewProgram(New(cfg, workDir), tea.WithAltScreen())
	_, err := p.Run()
	return err
}
