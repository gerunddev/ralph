// Package tui provides the Bubble Tea TUI for Ralph V2 single-agent loop.
package tui

import "github.com/charmbracelet/bubbles/key"

// KeyMap defines the key bindings for the TUI.
type KeyMap struct {
	// Navigation
	Up   key.Binding
	Down key.Binding

	// Actions
	Quit    key.Binding
	Dismiss key.Binding
}

// DefaultKeyMap returns the default key bindings.
func DefaultKeyMap() KeyMap {
	return KeyMap{
		Up: key.NewBinding(
			key.WithKeys("up"),
			key.WithHelp("↑↓", "scroll"),
		),
		Down: key.NewBinding(
			key.WithKeys("down"),
		),
		Quit: key.NewBinding(
			key.WithKeys("q", "ctrl+c"),
			key.WithHelp("q", "quit"),
		),
		Dismiss: key.NewBinding(
			key.WithKeys("enter", "esc"),
			key.WithHelp("Enter/Esc", "close"),
		),
	}
}

// ShortHelp returns the key bindings for the short help view.
func (k KeyMap) ShortHelp() []key.Binding {
	return []key.Binding{k.Up, k.Quit}
}

// FullHelp returns the key bindings for the full help view.
func (k KeyMap) FullHelp() [][]key.Binding {
	return [][]key.Binding{{k.Up, k.Down, k.Quit}}
}
