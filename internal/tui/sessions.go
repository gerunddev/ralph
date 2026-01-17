// Package tui provides the Bubble Tea TUI application for Ralph.
package tui

import (
	tea "github.com/charmbracelet/bubbletea"
)

// SessionsModel is the model for the session list view.
type SessionsModel struct {
	// TODO: implement
}

// NewSessionsModel creates a new session list model.
func NewSessionsModel() SessionsModel {
	return SessionsModel{}
}

// Init implements tea.Model.
func (m SessionsModel) Init() tea.Cmd {
	return nil
}

// Update implements tea.Model.
func (m SessionsModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	// TODO: implement
	return m, nil
}

// View implements tea.Model.
func (m SessionsModel) View() string {
	return "Sessions view - TODO\n"
}
