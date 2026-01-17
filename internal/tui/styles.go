// Package tui provides the Bubble Tea TUI application for Ralph.
package tui

import "github.com/charmbracelet/lipgloss"

// Monokai Pro color palette
var (
	colorBackground = lipgloss.Color("#2d2a2e")
	colorForeground = lipgloss.Color("#fcfcfa")
	colorYellow     = lipgloss.Color("#ffd866")
	colorOrange     = lipgloss.Color("#fc9867")
	colorRed        = lipgloss.Color("#ff6188")
	colorMagenta    = lipgloss.Color("#ab9df2")
	colorGreen      = lipgloss.Color("#a9dc76")
	colorGray       = lipgloss.Color("#727072")
	colorDimGray    = lipgloss.Color("#5b595c")
)

// Title styles
var (
	titleStyle = lipgloss.NewStyle().
		Bold(true).
		Foreground(colorMagenta).
		MarginBottom(1)
)

// List item styles
var (
	selectedStyle = lipgloss.NewStyle().
			Foreground(colorBackground).
			Background(colorYellow).
			Bold(true).
			Padding(0, 1)

	normalStyle = lipgloss.NewStyle().
			Foreground(colorForeground).
			Padding(0, 1)

	cursorStyle = lipgloss.NewStyle().
			Foreground(colorYellow).
			Bold(true)
)

// Status indicator styles
var (
	statusPendingStyle = lipgloss.NewStyle().
				Foreground(colorGray)

	statusInProgressStyle = lipgloss.NewStyle().
				Foreground(colorOrange)

	statusCompletedStyle = lipgloss.NewStyle().
				Foreground(colorGreen)

	statusFailedStyle = lipgloss.NewStyle().
				Foreground(colorRed)
)

// Help and informational styles
var (
	helpStyle = lipgloss.NewStyle().
			Foreground(colorDimGray).
			MarginTop(1)

	errorStyle = lipgloss.NewStyle().
			Foreground(colorRed).
			Bold(true)

	emptyStateStyle = lipgloss.NewStyle().
			Foreground(colorGray).
			Italic(true)
)

// Task progress view styles
var (
	taskListStyle = lipgloss.NewStyle().
			Width(26).
			BorderStyle(lipgloss.RoundedBorder()).
			BorderForeground(colorDimGray).
			Padding(0, 1)

	outputStyle = lipgloss.NewStyle().
			BorderStyle(lipgloss.RoundedBorder()).
			BorderForeground(colorDimGray).
			Padding(0, 1)
)

// Pause mode styles
var pauseStyle = lipgloss.NewStyle().
	Foreground(colorYellow).
	Bold(true)
