// Package tui provides the Bubble Tea TUI application for Ralph.
package tui

import (
	"fmt"
	"strings"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/gerund/ralph/internal/db"
	"github.com/gerund/ralph/internal/engine"
)

// ResumeAction represents the user's choice in the resume dialog.
type ResumeAction int

const (
	// ResumeActionNone indicates no action has been taken yet.
	ResumeActionNone ResumeAction = iota
	// ResumeActionResume indicates the user chose to resume the project.
	ResumeActionResume
	// ResumeActionReset indicates the user chose to reset the project.
	ResumeActionReset
	// ResumeActionQuit indicates the user chose to quit.
	ResumeActionQuit
)

// ShowResumeDialogMsg is sent when a resume dialog should be shown.
type ShowResumeDialogMsg struct {
	State *engine.ProjectState
}

// ResumeConfirmedMsg is sent when the user confirms resuming.
type ResumeConfirmedMsg struct {
	ProjectID string
}

// ResetConfirmedMsg is sent when the user confirms resetting.
type ResetConfirmedMsg struct {
	ProjectID string
}

// ProjectCompletedMsg is sent when selecting a completed project.
type ProjectCompletedMsg struct {
	Project *db.Project
}

// ResumeModel is the model for the resume confirmation dialog.
type ResumeModel struct {
	state       *engine.ProjectState
	action      ResumeAction
	width       int
	height      int
	showDetails bool
}

// NewResumeModel creates a new resume model with the given project state.
func NewResumeModel(state *engine.ProjectState) ResumeModel {
	return ResumeModel{
		state:  state,
		action: ResumeActionNone,
	}
}

// Init implements tea.Model.
func (m ResumeModel) Init() tea.Cmd {
	return nil
}

// Update implements tea.Model.
func (m ResumeModel) Update(msg tea.Msg) (ResumeModel, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		return m, nil

	case tea.KeyMsg:
		switch msg.String() {
		case "enter", "y":
			m.action = ResumeActionResume
			return m, func() tea.Msg {
				return ResumeConfirmedMsg{ProjectID: m.state.Project.ID}
			}

		case "r":
			m.action = ResumeActionReset
			return m, func() tea.Msg {
				return ResetConfirmedMsg{ProjectID: m.state.Project.ID}
			}

		case "d":
			m.showDetails = !m.showDetails
			return m, nil

		case "q", "esc":
			m.action = ResumeActionQuit
			return m, tea.Quit
		}
	}
	return m, nil
}

// View implements tea.Model.
func (m ResumeModel) View() string {
	var s strings.Builder

	s.WriteString(titleStyle.Render("Resume Project?"))
	s.WriteString("\n\n")

	// Project info
	if m.state.Project != nil {
		s.WriteString(fmt.Sprintf("Project: %s\n", m.state.Project.Name))
		s.WriteString(fmt.Sprintf("Status:  %s\n\n", m.formatProjectStatus()))
	}

	// Progress summary
	s.WriteString("Progress:\n")
	s.WriteString(fmt.Sprintf("  %s Completed: %d tasks\n", statusCompletedStyle.Render("●"), m.state.CompletedTasks))
	s.WriteString(fmt.Sprintf("  %s Pending:   %d tasks\n", statusPendingStyle.Render("○"), m.state.PendingTasks))
	if m.state.FailedTasks > 0 {
		s.WriteString(fmt.Sprintf("  %s Failed:    %d tasks\n", statusFailedStyle.Render("✗"), m.state.FailedTasks))
	}

	// Interrupted task info
	if m.state.InProgressTask != nil {
		s.WriteString("\n")
		s.WriteString(statusInProgressStyle.Render("Interrupted task:"))
		s.WriteString(fmt.Sprintf(" %s\n", m.state.InProgressTask.Title))
		s.WriteString("This task will be restarted from the beginning.\n")
	}

	// Show details if toggled
	if m.showDetails && m.state.LastSession != nil {
		s.WriteString("\n")
		s.WriteString("Last session:\n")
		s.WriteString(fmt.Sprintf("  Agent: %s\n", m.state.LastSession.AgentType))
		s.WriteString(fmt.Sprintf("  Iteration: %d\n", m.state.LastSession.Iteration))
		s.WriteString(fmt.Sprintf("  Status: %s\n", m.state.LastSession.Status))
	}

	s.WriteString("\n")
	s.WriteString(helpStyle.Render("enter: resume | r: reset all | d: details | q: quit"))

	return s.String()
}

// formatProjectStatus returns a styled string for the project status.
func (m ResumeModel) formatProjectStatus() string {
	if m.state.Project == nil {
		return "unknown"
	}
	switch m.state.Project.Status {
	case db.ProjectPending:
		return statusPendingStyle.Render("pending")
	case db.ProjectInProgress:
		return statusInProgressStyle.Render("in progress")
	case db.ProjectCompleted:
		return statusCompletedStyle.Render("completed")
	case db.ProjectFailed:
		return statusFailedStyle.Render("failed")
	default:
		return string(m.state.Project.Status)
	}
}

// State returns the project state.
func (m ResumeModel) State() *engine.ProjectState {
	return m.state
}

// Action returns the selected action.
func (m ResumeModel) Action() ResumeAction {
	return m.action
}

// FailedTasksModel is the model for displaying and handling failed tasks.
type FailedTasksModel struct {
	projectID string
	tasks     []*db.Task
	cursor    int
	width     int
	height    int
}

// NewFailedTasksModel creates a new failed tasks model.
func NewFailedTasksModel(projectID string, tasks []*db.Task) FailedTasksModel {
	// Filter to only failed/escalated tasks
	var failedTasks []*db.Task
	for _, t := range tasks {
		if t.Status == db.TaskFailed || t.Status == db.TaskEscalated {
			failedTasks = append(failedTasks, t)
		}
	}
	return FailedTasksModel{
		projectID: projectID,
		tasks:     failedTasks,
	}
}

// FailedTasksRetryMsg is sent when the user chooses to retry failed tasks.
type FailedTasksRetryMsg struct {
	ProjectID string
}

// FailedTasksSkipMsg is sent when the user chooses to skip failed tasks.
type FailedTasksSkipMsg struct {
	ProjectID string
}

// FailedTasksAbortMsg is sent when the user chooses to abort.
type FailedTasksAbortMsg struct{}

// Init implements tea.Model.
func (m FailedTasksModel) Init() tea.Cmd {
	return nil
}

// Update implements tea.Model.
func (m FailedTasksModel) Update(msg tea.Msg) (FailedTasksModel, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		return m, nil

	case tea.KeyMsg:
		switch msg.String() {
		case "up", "k":
			if m.cursor > 0 {
				m.cursor--
			}
		case "down", "j":
			if m.cursor < len(m.tasks)-1 {
				m.cursor++
			}
		case "r":
			return m, func() tea.Msg {
				return FailedTasksRetryMsg{ProjectID: m.projectID}
			}
		case "s":
			return m, func() tea.Msg {
				return FailedTasksSkipMsg{ProjectID: m.projectID}
			}
		case "a", "q":
			return m, func() tea.Msg {
				return FailedTasksAbortMsg{}
			}
		}
	}
	return m, nil
}

// View implements tea.Model.
func (m FailedTasksModel) View() string {
	var s strings.Builder

	s.WriteString(errorStyle.Render("Some tasks failed:"))
	s.WriteString("\n\n")

	if len(m.tasks) == 0 {
		s.WriteString(emptyStateStyle.Render("No failed tasks"))
		s.WriteString("\n")
	} else {
		for i, task := range m.tasks {
			prefix := "  "
			if i == m.cursor {
				prefix = cursorStyle.Render("> ")
			}
			statusIcon := statusFailedStyle.Render("✗")
			if task.Status == db.TaskEscalated {
				statusIcon = statusFailedStyle.Render("!")
			}
			s.WriteString(fmt.Sprintf("%s%s %s\n", prefix, statusIcon, task.Title))
		}
	}

	s.WriteString("\n")
	s.WriteString(helpStyle.Render("r: retry all | s: skip failed | a: abort"))

	return s.String()
}

// Tasks returns the list of failed tasks.
func (m FailedTasksModel) Tasks() []*db.Task {
	return m.tasks
}

// Cursor returns the current cursor position.
func (m FailedTasksModel) Cursor() int {
	return m.cursor
}

// CompletedProjectModel is the model for displaying a completed project.
type CompletedProjectModel struct {
	project *db.Project
	state   *engine.ProjectState
	width   int
	height  int
}

// NewCompletedProjectModel creates a new completed project model.
func NewCompletedProjectModel(project *db.Project, state *engine.ProjectState) CompletedProjectModel {
	return CompletedProjectModel{
		project: project,
		state:   state,
	}
}

// Init implements tea.Model.
func (m CompletedProjectModel) Init() tea.Cmd {
	return nil
}

// Update implements tea.Model.
func (m CompletedProjectModel) Update(msg tea.Msg) (CompletedProjectModel, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		return m, nil

	case tea.KeyMsg:
		switch msg.String() {
		case "r":
			// Reset project and start over
			return m, func() tea.Msg {
				return ResetConfirmedMsg{ProjectID: m.project.ID}
			}
		case "q", "esc", "enter":
			return m, tea.Quit
		}
	}
	return m, nil
}

// View implements tea.Model.
func (m CompletedProjectModel) View() string {
	var s strings.Builder

	s.WriteString(titleStyle.Render("Project Completed"))
	s.WriteString("\n\n")

	if m.project != nil {
		s.WriteString(fmt.Sprintf("Project: %s\n", m.project.Name))
	}

	if m.state != nil {
		s.WriteString(fmt.Sprintf("\n%s All %d tasks completed.\n", statusCompletedStyle.Render("●"), m.state.CompletedTasks))
	}

	s.WriteString("\n")
	s.WriteString(helpStyle.Render("r: reset and run again | enter/q: quit"))

	return s.String()
}

// Project returns the project.
func (m CompletedProjectModel) Project() *db.Project {
	return m.project
}
