// Package tui provides the Bubble Tea TUI application for Ralph.
package tui

import (
	tea "github.com/charmbracelet/bubbletea"
)

// TasksModel is the model for the task list view.
type TasksModel struct {
	// TODO: implement
}

// NewTasksModel creates a new task list model.
func NewTasksModel() TasksModel {
	return TasksModel{}
}

// Init implements tea.Model.
func (m TasksModel) Init() tea.Cmd {
	return nil
}

// Update implements tea.Model.
func (m TasksModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	// TODO: implement
	return m, nil
}

// View implements tea.Model.
func (m TasksModel) View() string {
	return "Tasks view - TODO\n"
}
