// Package tui provides the Bubble Tea TUI application for Ralph.
package tui

import (
	"strings"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/gerund/ralph/internal/db"
)

// LearningsModel shows the learnings capture progress.
type LearningsModel struct {
	project   *db.Project
	status    string
	completed bool
	err       error
	width     int
	height    int
}

// NewLearningsModel creates a new learnings model.
func NewLearningsModel(project *db.Project) LearningsModel {
	return LearningsModel{
		project: project,
		status:  "Capturing learnings...",
	}
}

// Init implements tea.Model.
func (m LearningsModel) Init() tea.Cmd {
	return nil
}

// Update implements tea.Model.
func (m LearningsModel) Update(msg tea.Msg) (LearningsModel, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		return m, nil

	case LearningsCapturedMsg:
		m.completed = true
		if msg.Err != nil {
			m.err = msg.Err
			m.status = "Failed to capture learnings"
		} else {
			m.status = "Learnings captured successfully!"
		}
		return m, nil
	}

	return m, nil
}

// View implements tea.Model.
func (m LearningsModel) View() string {
	var s strings.Builder

	s.WriteString(titleStyle.Render("Capturing Learnings"))
	s.WriteString("\n\n")

	if m.err != nil {
		s.WriteString(statusFailedStyle.Render(m.status))
		s.WriteString("\n")
		s.WriteString(errorStyle.Render(m.err.Error()))
	} else if m.completed {
		s.WriteString(statusCompletedStyle.Render(m.status))
	} else {
		s.WriteString(statusInProgressStyle.Render(m.status))
		s.WriteString("\n\n")
		s.WriteString("Analyzing changes and updating documentation...\n")
		s.WriteString("  - AGENTS.md: Coding patterns and conventions\n")
		s.WriteString("  - README.md: User-facing documentation\n")
	}

	s.WriteString("\n\n")
	s.WriteString(helpStyle.Render("Please wait..."))

	return s.String()
}

// IsCompleted returns whether learnings capture has completed.
func (m LearningsModel) IsCompleted() bool {
	return m.completed
}

// Error returns the error if any.
func (m LearningsModel) Error() error {
	return m.err
}
