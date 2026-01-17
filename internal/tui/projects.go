// Package tui provides the Bubble Tea TUI application for Ralph.
package tui

import (
	"fmt"
	"strings"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/gerund/ralph/internal/config"
	"github.com/gerund/ralph/internal/db"
)

// ProjectsLoadedMsg is sent when projects are loaded from the database.
type ProjectsLoadedMsg struct {
	Projects []db.ProjectInfo
	Err      error
}

// ProjectSelectedMsg is sent when the user selects a project.
type ProjectSelectedMsg struct {
	ProjectInfo db.ProjectInfo
}

// ProjectListModel is the model for the project selection view.
type ProjectListModel struct {
	cfg      *config.Config
	projects []db.ProjectInfo
	cursor   int
	width    int
	height   int
	loading  bool
	err      error
}

// NewProjectListModel creates a new project list model.
func NewProjectListModel(cfg *config.Config) ProjectListModel {
	return ProjectListModel{
		cfg:     cfg,
		loading: true,
	}
}

// Init implements tea.Model.
func (m ProjectListModel) Init() tea.Cmd {
	return m.loadProjects
}

// loadProjects discovers projects from the projects directory.
func (m ProjectListModel) loadProjects() tea.Msg {
	projects, err := db.DiscoverProjects(m.cfg.GetProjectsDir())
	return ProjectsLoadedMsg{Projects: projects, Err: err}
}

// Update implements tea.Model.
func (m ProjectListModel) Update(msg tea.Msg) (ProjectListModel, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		return m, nil

	case ProjectsLoadedMsg:
		m.loading = false
		if msg.Err != nil {
			m.err = msg.Err
			return m, nil
		}
		m.projects = msg.Projects
		return m, nil

	case tea.KeyMsg:
		// Don't process keys while loading or if there's an error
		if m.loading || m.err != nil {
			return m, nil
		}

		switch msg.String() {
		case "up", "k":
			if m.cursor > 0 {
				m.cursor--
			}

		case "down", "j":
			if m.cursor < len(m.projects)-1 {
				m.cursor++
			}

		case "enter":
			if len(m.projects) > 0 {
				return m, func() tea.Msg {
					return ProjectSelectedMsg{ProjectInfo: m.projects[m.cursor]}
				}
			}

		case "home", "g":
			m.cursor = 0

		case "end", "G":
			if len(m.projects) > 0 {
				m.cursor = len(m.projects) - 1
			}
		}
	}

	return m, nil
}

// View implements tea.Model.
func (m ProjectListModel) View() string {
	if m.loading {
		return "Loading projects...\n"
	}

	if m.err != nil {
		return errorStyle.Render(fmt.Sprintf("Error: %v", m.err)) + "\n"
	}

	var s strings.Builder

	s.WriteString(titleStyle.Render("Ralph - Select a Project"))
	s.WriteString("\n\n")

	if len(m.projects) == 0 {
		s.WriteString(emptyStateStyle.Render("No projects yet. Create one with: ralph -c <plan-file>"))
		s.WriteString("\n")
	} else {
		// Calculate visible items based on terminal height
		// Reserve space for title (2 lines) + help (2 lines) + margins
		visibleHeight := m.height - 6
		if visibleHeight < 3 {
			visibleHeight = 3
		}

		// Determine which items to display (scrolling)
		startIdx := 0
		endIdx := len(m.projects)

		if len(m.projects) > visibleHeight {
			// Keep cursor visible with some context
			halfVisible := visibleHeight / 2
			if m.cursor > halfVisible {
				startIdx = m.cursor - halfVisible
			}
			if startIdx+visibleHeight > len(m.projects) {
				startIdx = len(m.projects) - visibleHeight
			}
			endIdx = startIdx + visibleHeight
		}

		// Show scroll indicator at top if needed
		if startIdx > 0 {
			s.WriteString(helpStyle.Render(fmt.Sprintf("  ... %d more above", startIdx)))
			s.WriteString("\n")
		}

		for i := startIdx; i < endIdx; i++ {
			p := m.projects[i]
			cursor := "  "
			if i == m.cursor {
				cursor = cursorStyle.Render("> ")
			}

			statusStr := m.formatStatus(p.Status)
			timeStr := p.UpdatedAt.Format("Jan 02 15:04")

			var line string
			if i == m.cursor {
				line = selectedStyle.Render(fmt.Sprintf("%s  %s  %s", p.Name, statusStr, timeStr))
			} else {
				line = normalStyle.Render(fmt.Sprintf("%s  %s  %s", p.Name, statusStr, timeStr))
			}

			s.WriteString(cursor + line)
			s.WriteString("\n")
		}

		// Show scroll indicator at bottom if needed
		if endIdx < len(m.projects) {
			s.WriteString(helpStyle.Render(fmt.Sprintf("  ... %d more below", len(m.projects)-endIdx)))
			s.WriteString("\n")
		}
	}

	s.WriteString(helpStyle.Render("j/k: navigate | enter: select | q: quit"))

	return s.String()
}

// formatStatus returns a styled status string.
func (m ProjectListModel) formatStatus(status db.ProjectStatus) string {
	switch status {
	case db.ProjectPending:
		return statusPendingStyle.Render("[pending]")
	case db.ProjectInProgress:
		return statusInProgressStyle.Render("[in progress]")
	case db.ProjectCompleted:
		return statusCompletedStyle.Render("[completed]")
	case db.ProjectFailed:
		return statusFailedStyle.Render("[failed]")
	default:
		return string(status)
	}
}

// Projects returns the list of projects.
func (m ProjectListModel) Projects() []db.ProjectInfo {
	return m.projects
}

// Cursor returns the current cursor position.
func (m ProjectListModel) Cursor() int {
	return m.cursor
}

// IsLoading returns whether the model is loading.
func (m ProjectListModel) IsLoading() bool {
	return m.loading
}

// Error returns the error if any.
func (m ProjectListModel) Error() error {
	return m.err
}

// SelectedProject returns the currently selected project info.
// Returns nil pointer equivalent (zero value) if no projects.
func (m ProjectListModel) SelectedProject() *db.ProjectInfo {
	if len(m.projects) == 0 {
		return nil
	}
	return &m.projects[m.cursor]
}
