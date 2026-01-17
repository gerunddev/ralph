// Package tui provides the Bubble Tea TUI application for Ralph.
package tui

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/mattn/go-runewidth"

	"github.com/gerund/ralph/internal/db"
	"github.com/gerund/ralph/internal/engine"
)

// EngineEventMsg wraps engine events for Bubble Tea.
type EngineEventMsg struct {
	Event engine.EngineEvent
}

// TasksLoadedMsg signals tasks are ready.
type TasksLoadedMsg struct {
	Tasks []*db.Task
	Err   error
}

// EngineStartedMsg signals the engine has started.
type EngineStartedMsg struct {
	Engine *engine.Engine
}

// EngineErrorMsg signals an engine error.
type EngineErrorMsg struct {
	Err error
}

// EngineEventsClosedMsg signals the engine event channel has closed.
type EngineEventsClosedMsg struct{}

// TaskProgressModel is the model for the task progress view.
type TaskProgressModel struct {
	project    *db.Project
	tasks      []*db.Task
	currentIdx int
	iteration  int
	agentType  string
	output     strings.Builder
	viewport   viewport.Model
	width      int
	height     int
	completed  bool
	err        error
	autoScroll bool

	// Engine event channel
	engineEvents <-chan engine.EngineEvent
	db           *db.DB

	// Pause mode state
	pauseMode bool // Whether pause mode is enabled
	isPaused  bool // Whether currently waiting at a pause point
	engine    *engine.Engine
}

// NewTaskProgressModel creates a new task progress model.
func NewTaskProgressModel(project *db.Project, database *db.DB, events <-chan engine.EngineEvent) TaskProgressModel {
	vp := viewport.New(80, 20)
	return TaskProgressModel{
		project:      project,
		db:           database,
		engineEvents: events,
		viewport:     vp,
		autoScroll:   true,
	}
}

// SetEngine sets the engine reference for pause mode control.
// This must be called after the engine is started but before updates.
func (m *TaskProgressModel) SetEngine(eng *engine.Engine) {
	m.engine = eng
	// Initialize pause mode from engine's task loop if available
	if eng != nil {
		if tl := eng.TaskLoop(); tl != nil {
			m.pauseMode = tl.IsPauseMode()
		}
	}
}

// NewTaskProgressModelWithError creates a task progress model in an error state.
func NewTaskProgressModelWithError(project *db.Project, err error) TaskProgressModel {
	vp := viewport.New(80, 20)
	return TaskProgressModel{
		project:    project,
		viewport:   vp,
		autoScroll: true,
		err:        err,
	}
}

// Init implements tea.Model.
func (m TaskProgressModel) Init() tea.Cmd {
	return tea.Batch(
		m.loadTasks,
		m.listenForEvents,
	)
}

// loadTasks fetches tasks from the database.
func (m TaskProgressModel) loadTasks() tea.Msg {
	if m.db == nil || m.project == nil {
		return TasksLoadedMsg{Err: fmt.Errorf("database or project not initialized")}
	}
	tasks, err := m.db.GetTasksByProject(m.project.ID)
	return TasksLoadedMsg{Tasks: tasks, Err: err}
}

// listenForEvents listens for engine events.
func (m TaskProgressModel) listenForEvents() tea.Msg {
	if m.engineEvents == nil {
		return nil
	}
	event, ok := <-m.engineEvents
	if !ok {
		return EngineEventsClosedMsg{}
	}
	return EngineEventMsg{Event: event}
}

// Update implements tea.Model.
func (m TaskProgressModel) Update(msg tea.Msg) (TaskProgressModel, tea.Cmd) {
	var cmds []tea.Cmd

	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		// Reserve space for header (3 lines) + footer (2 lines)
		viewportHeight := msg.Height - 5
		if viewportHeight < 3 {
			viewportHeight = 3
		}
		// Task list takes ~25 chars, rest for output
		taskListWidth := 28
		viewportWidth := msg.Width - taskListWidth - 4
		if viewportWidth < 20 {
			viewportWidth = 20
		}
		m.viewport.Width = viewportWidth
		m.viewport.Height = viewportHeight
		return m, nil

	case TasksLoadedMsg:
		if msg.Err != nil {
			m.err = msg.Err
			return m, nil
		}
		m.tasks = msg.Tasks
		return m, nil

	case EngineEventMsg:
		m.handleEngineEvent(msg.Event)
		// Continue listening
		cmds = append(cmds, m.listenForEvents)

	case EngineEventsClosedMsg:
		// Engine event channel closed - mark as completed if not already in error state
		if m.err == nil && !m.completed {
			m.completed = true
			m.output.WriteString("\n--- Engine finished ---\n")
			m.viewport.SetContent(m.output.String())
			if m.autoScroll {
				m.viewport.GotoBottom()
			}
		}
		return m, nil

	case tea.KeyMsg:
		switch msg.String() {
		case "q", "ctrl+c":
			// Don't quit here - let the main model handle it
			return m, nil
		case "p":
			// Toggle pause mode
			m.pauseMode = !m.pauseMode
			if m.engine != nil {
				if tl := m.engine.TaskLoop(); tl != nil {
					tl.SetPauseMode(m.pauseMode)
				}
			}
			return m, nil
		case "enter", " ":
			// Continue from pause (if paused)
			if m.isPaused {
				if m.engine != nil {
					if tl := m.engine.TaskLoop(); tl != nil {
						tl.Continue()
					}
				}
				m.isPaused = false
				return m, nil
			}
		case "up", "k":
			m.viewport.LineUp(1)
			m.autoScroll = false
		case "down", "j":
			m.viewport.LineDown(1)
			// Re-enable auto-scroll if at bottom
			if m.viewport.AtBottom() {
				m.autoScroll = true
			}
		case "pgup":
			m.viewport.ViewUp()
			m.autoScroll = false
		case "pgdown":
			m.viewport.ViewDown()
			if m.viewport.AtBottom() {
				m.autoScroll = true
			}
		case "g", "home":
			m.viewport.GotoTop()
			m.autoScroll = false
		case "G", "end":
			m.viewport.GotoBottom()
			m.autoScroll = true
		}
	}

	// Update viewport
	var cmd tea.Cmd
	m.viewport, cmd = m.viewport.Update(msg)
	cmds = append(cmds, cmd)

	return m, tea.Batch(cmds...)
}

// handleEngineEvent processes an engine event.
func (m *TaskProgressModel) handleEngineEvent(event engine.EngineEvent) {
	switch event.Type {
	case engine.EngineEventCreatingProject:
		m.output.WriteString(fmt.Sprintf("Creating project: %s\n", event.Message))

	case engine.EngineEventPlanningTasks:
		m.output.WriteString(fmt.Sprintf("Planning: %s\n", event.Message))

	case engine.EngineEventTasksCreated:
		m.output.WriteString(fmt.Sprintf("Created tasks: %s\n", event.Message))

	case engine.EngineEventRunning:
		if event.TaskLoopEvent != nil {
			m.handleTaskLoopEvent(*event.TaskLoopEvent)
		}

	case engine.EngineEventCompleted:
		m.completed = true
		m.output.WriteString("\n--- All tasks completed! ---\n")

	case engine.EngineEventFailed:
		m.err = fmt.Errorf("%s", event.Message)
		m.output.WriteString(fmt.Sprintf("\n--- Failed: %s ---\n", event.Message))
	}

	m.viewport.SetContent(m.output.String())
	if m.autoScroll {
		m.viewport.GotoBottom()
	}
}

// handleTaskLoopEvent processes a task loop event.
func (m *TaskProgressModel) handleTaskLoopEvent(event engine.TaskLoopEvent) {
	switch event.Type {
	case engine.TaskEventStarted:
		m.output.WriteString(fmt.Sprintf("\n%s\n", event.Message))

	case engine.TaskEventTaskBegin:
		m.currentIdx = event.TaskIndex
		m.output.WriteString(fmt.Sprintf("\n=== Task %d: %s ===\n", event.TaskIndex+1, event.TaskTitle))
		// Update task status in our local list
		if event.TaskIndex < len(m.tasks) {
			m.tasks[event.TaskIndex].Status = db.TaskInProgress
		}

	case engine.TaskEventTaskEnd:
		m.output.WriteString("Task completed.\n")
		// Update task status in our local list
		if event.TaskIndex < len(m.tasks) {
			m.tasks[event.TaskIndex].Status = db.TaskCompleted
		}

	case engine.TaskEventFailed:
		m.output.WriteString(fmt.Sprintf("Task failed: %s\n", event.Message))
		// Update task status in our local list
		if event.TaskIndex < len(m.tasks) {
			m.tasks[event.TaskIndex].Status = db.TaskFailed
		}

	case engine.TaskEventCompleted:
		m.output.WriteString(fmt.Sprintf("\n%s\n", event.Message))

	case engine.TaskEventProgress:
		// Handle nested implementation loop events
		if event.ImplEvent != nil {
			m.handleImplEvent(*event.ImplEvent)
		}

	case engine.TaskEventPaused:
		m.isPaused = true
		m.output.WriteString("\n[PAUSED] Press Enter to continue, 'p' to switch to continuous mode\n")

	case engine.TaskEventResumed:
		m.isPaused = false
		m.output.WriteString("Resuming...\n")

	case engine.TaskEventPauseModeChanged:
		// Use dedicated field for pause mode state
		if event.PauseModeEnabled != nil {
			m.pauseMode = *event.PauseModeEnabled
		}
		if m.pauseMode {
			m.output.WriteString("Pause mode enabled - will pause after current task\n")
		} else {
			m.output.WriteString("Continuous mode enabled\n")
		}
	}
}

// handleImplEvent processes an implementation loop event.
func (m *TaskProgressModel) handleImplEvent(event engine.ImplLoopEvent) {
	switch event.Type {
	case engine.EventStarted:
		m.output.WriteString("  Starting implementation loop...\n")

	case engine.EventDeveloping:
		m.agentType = "developer"
		m.iteration = event.Iteration
		m.output.WriteString(fmt.Sprintf("  [Iteration %d] Developer working...\n", event.Iteration))

	case engine.EventReviewing:
		m.agentType = "reviewer"
		m.output.WriteString(fmt.Sprintf("  [Iteration %d] Reviewer checking...\n", event.Iteration))

	case engine.EventFeedback:
		m.output.WriteString(fmt.Sprintf("  Feedback: %s\n", truncate(event.Message, 100)))

	case engine.EventApproved:
		m.output.WriteString("  Approved!\n")

	case engine.EventFailed:
		m.output.WriteString(fmt.Sprintf("  Failed: %s\n", truncate(event.Message, 100)))
	}
}

// View implements tea.Model.
func (m TaskProgressModel) View() string {
	if m.err != nil && !m.completed {
		return fmt.Sprintf("Error: %v\n\nPress q to quit", m.err)
	}

	var s strings.Builder

	// Header
	s.WriteString(m.renderHeader())
	s.WriteString("\n")

	// Task list sidebar + output viewport
	s.WriteString(m.renderBody())
	s.WriteString("\n")

	// Footer
	s.WriteString(m.renderFooter())

	return s.String()
}

// renderHeader renders the header section.
func (m TaskProgressModel) renderHeader() string {
	name := "Unknown"
	if m.project != nil {
		name = m.project.Name
	}
	title := titleStyle.Render(fmt.Sprintf("Ralph - %s", name))

	progress := ""
	if len(m.tasks) > 0 {
		completed := m.countCompleted()
		progress = fmt.Sprintf(" [%d/%d tasks]", completed, len(m.tasks))
	}

	return lipgloss.JoinHorizontal(lipgloss.Top, title, progress)
}

// renderBody renders the main body with task list and output.
func (m TaskProgressModel) renderBody() string {
	// Left: Task list
	taskList := m.renderTaskList()

	// Right: Output viewport
	outputView := m.viewport.View()

	// Join horizontally
	return lipgloss.JoinHorizontal(
		lipgloss.Top,
		taskListStyle.Render(taskList),
		outputStyle.Render(outputView),
	)
}

// renderTaskList renders the task list sidebar.
func (m TaskProgressModel) renderTaskList() string {
	var s strings.Builder

	if len(m.tasks) == 0 {
		s.WriteString(emptyStateStyle.Render("No tasks yet"))
		return s.String()
	}

	for i, task := range m.tasks {
		prefix := "  "
		if i == m.currentIdx {
			prefix = "> "
		}

		status := m.taskStatusIcon(task.Status)
		line := fmt.Sprintf("%s%s %s", prefix, status, truncate(task.Title, 20))
		s.WriteString(line)
		s.WriteString("\n")
	}

	return s.String()
}

// taskStatusIcon returns a styled status icon for a task status.
func (m TaskProgressModel) taskStatusIcon(status db.TaskStatus) string {
	switch status {
	case db.TaskPending:
		return statusPendingStyle.Render("○")
	case db.TaskInProgress:
		return statusInProgressStyle.Render("◐")
	case db.TaskCompleted:
		return statusCompletedStyle.Render("●")
	case db.TaskFailed:
		return statusFailedStyle.Render("✗")
	case db.TaskEscalated:
		return statusFailedStyle.Render("!")
	default:
		return "?"
	}
}

// renderFooter renders the footer section.
func (m TaskProgressModel) renderFooter() string {
	var left, right string

	// Mode indicator
	modeIndicator := ""
	if m.pauseMode {
		modeIndicator = pauseStyle.Render("[PAUSE]") + " "
	}

	if m.isPaused {
		left = modeIndicator + "Task complete. Press Enter to continue, 'p' to switch to continuous"
	} else if m.completed {
		left = statusCompletedStyle.Render("Completed!")
	} else if m.err != nil {
		left = statusFailedStyle.Render("Failed")
	} else if m.iteration > 0 {
		left = modeIndicator + fmt.Sprintf("Iteration %d - %s", m.iteration, m.agentType)
	} else {
		left = modeIndicator + "Starting..."
	}

	// Help text varies based on pause mode
	if m.pauseMode {
		right = "p: continuous | Enter: continue | j/k: scroll | q: quit"
	} else {
		right = "p: pause mode | j/k: scroll | g/G: top/bottom | q: quit"
	}

	// Calculate spacing using display width for proper Unicode handling
	leftWidth := lipgloss.Width(left)
	rightWidth := lipgloss.Width(right)
	spacing := m.width - leftWidth - rightWidth - 4
	if spacing < 1 {
		spacing = 1
	}

	return lipgloss.JoinHorizontal(
		lipgloss.Top,
		helpStyle.Render(left),
		strings.Repeat(" ", spacing),
		helpStyle.Render(right),
	)
}

// countCompleted returns the number of completed tasks.
func (m TaskProgressModel) countCompleted() int {
	count := 0
	for _, task := range m.tasks {
		if task.Status == db.TaskCompleted {
			count++
		}
	}
	return count
}

// truncate truncates a string to the given display width.
// It properly handles Unicode characters by using rune width calculations.
func truncate(s string, maxWidth int) string {
	if runewidth.StringWidth(s) <= maxWidth {
		return s
	}
	if maxWidth <= 3 {
		return runewidth.Truncate(s, maxWidth, "")
	}
	return runewidth.Truncate(s, maxWidth, "...")
}

// Project returns the current project.
func (m TaskProgressModel) Project() *db.Project {
	return m.project
}

// Tasks returns the list of tasks.
func (m TaskProgressModel) Tasks() []*db.Task {
	return m.tasks
}

// CurrentIndex returns the current task index.
func (m TaskProgressModel) CurrentIndex() int {
	return m.currentIdx
}

// Iteration returns the current iteration.
func (m TaskProgressModel) Iteration() int {
	return m.iteration
}

// AgentType returns the current agent type.
func (m TaskProgressModel) AgentType() string {
	return m.agentType
}

// IsCompleted returns whether the engine has completed.
func (m TaskProgressModel) IsCompleted() bool {
	return m.completed
}

// Error returns the error if any.
func (m TaskProgressModel) Error() error {
	return m.err
}

// Output returns the current output content.
func (m TaskProgressModel) Output() string {
	return m.output.String()
}

// IsPauseMode returns whether pause mode is enabled.
func (m TaskProgressModel) IsPauseMode() bool {
	return m.pauseMode
}

// IsPaused returns whether the loop is currently paused waiting for continuation.
func (m TaskProgressModel) IsPaused() bool {
	return m.isPaused
}
