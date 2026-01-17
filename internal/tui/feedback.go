// Package tui provides the Bubble Tea TUI application for Ralph.
package tui

import (
	"fmt"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
)

// FeedbackChoiceMsg is sent when the user makes a feedback choice.
type FeedbackChoiceMsg struct {
	Complete bool // true = mark review as complete, false = provide feedback
}

// FeedbackPromptModel shows after all tasks complete when feedback state is None.
// It prompts the user to mark the review as complete or provide feedback.
type FeedbackPromptModel struct {
	projectID string
	cursor    int // 0 = Yes, 1 = No
	width     int
	height    int
}

// NewFeedbackPromptModel creates a new feedback prompt model.
func NewFeedbackPromptModel(projectID string) FeedbackPromptModel {
	return FeedbackPromptModel{
		projectID: projectID,
		cursor:    0,
	}
}

// Init implements tea.Model.
func (m FeedbackPromptModel) Init() tea.Cmd {
	return nil
}

// Update implements tea.Model.
func (m FeedbackPromptModel) Update(msg tea.Msg) (FeedbackPromptModel, tea.Cmd) {
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
			if m.cursor < 1 {
				m.cursor++
			}
		case "enter":
			return m, func() tea.Msg {
				return FeedbackChoiceMsg{Complete: m.cursor == 0}
			}
		}
	}

	return m, nil
}

// View implements tea.Model.
func (m FeedbackPromptModel) View() string {
	var s strings.Builder

	s.WriteString(titleStyle.Render("All tasks completed!"))
	s.WriteString("\n\n")
	s.WriteString("Mark review as complete?\n\n")

	options := []string{
		"Yes, mark as complete",
		"No, I want to provide feedback",
	}

	for i, opt := range options {
		cursor := "  "
		if i == m.cursor {
			cursor = cursorStyle.Render("> ")
		}

		var line string
		if i == m.cursor {
			line = selectedStyle.Render(opt)
		} else {
			line = normalStyle.Render(opt)
		}

		s.WriteString(cursor + line)
		s.WriteString("\n")
	}

	s.WriteString("\n")
	s.WriteString(helpStyle.Render("j/k: select | enter: confirm | q: quit"))

	return s.String()
}

// ProjectID returns the project ID.
func (m FeedbackPromptModel) ProjectID() string {
	return m.projectID
}

// FeedbackInstructionsModel shows after user selects "No, I want to provide feedback".
// It displays CLI instructions for submitting feedback.
type FeedbackInstructionsModel struct {
	projectID string
	width     int
	height    int
}

// NewFeedbackInstructionsModel creates a new feedback instructions model.
func NewFeedbackInstructionsModel(projectID string) FeedbackInstructionsModel {
	return FeedbackInstructionsModel{
		projectID: projectID,
	}
}

// Init implements tea.Model.
func (m FeedbackInstructionsModel) Init() tea.Cmd {
	return nil
}

// Update implements tea.Model.
func (m FeedbackInstructionsModel) Update(msg tea.Msg) (FeedbackInstructionsModel, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		return m, nil

	case tea.KeyMsg:
		switch msg.String() {
		case "q", "ctrl+c", "enter":
			return m, tea.Quit
		}
	}

	return m, nil
}

// View implements tea.Model.
func (m FeedbackInstructionsModel) View() string {
	var s strings.Builder

	s.WriteString(titleStyle.Render("Provide Feedback"))
	s.WriteString("\n\n")

	s.WriteString("To submit feedback, follow these steps:\n\n")

	s.WriteString("1. Close this TUI (press Enter or q)\n\n")

	s.WriteString("2. Create a markdown file with your feedback\n\n")

	s.WriteString("3. Run the following command:\n\n")

	// Show the CLI command
	cmd := fmt.Sprintf("   ralph feedback -p %s -f /path/to/feedback.md", m.projectID)
	s.WriteString(statusInProgressStyle.Render(cmd))
	s.WriteString("\n\n")

	s.WriteString("4. Restart the TUI to process your feedback:\n\n")
	s.WriteString(statusInProgressStyle.Render("   ralph"))
	s.WriteString("\n\n")

	s.WriteString(helpStyle.Render("Press Enter or q to exit"))

	return s.String()
}

// ProjectID returns the project ID.
func (m FeedbackInstructionsModel) ProjectID() string {
	return m.projectID
}
