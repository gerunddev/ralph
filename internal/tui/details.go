// Package tui provides the Bubble Tea TUI application for Ralph.
package tui

import (
	tea "github.com/charmbracelet/bubbletea"
)

// DetailsModel is the model for the session details view.
type DetailsModel struct {
	// TODO: implement
}

// NewDetailsModel creates a new session details model.
func NewDetailsModel() DetailsModel {
	return DetailsModel{}
}

// Init implements tea.Model.
func (m DetailsModel) Init() tea.Cmd {
	return nil
}

// Update implements tea.Model.
func (m DetailsModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	// TODO: implement
	return m, nil
}

// View implements tea.Model.
func (m DetailsModel) View() string {
	return "Session details view - TODO\n"
}
