// Package tui provides the Bubble Tea TUI for Ralph V2 single-agent loop.
package tui

import "github.com/charmbracelet/lipgloss"

// Monokai Pro color palette
var (
	colorForeground = lipgloss.Color("#fcfcfa")
	colorYellow     = lipgloss.Color("#ffd866")
	colorOrange     = lipgloss.Color("#fc9867")
	colorRed        = lipgloss.Color("#ff6188")
	colorMagenta    = lipgloss.Color("#ab9df2")
	colorGreen      = lipgloss.Color("#a9dc76")
	colorGray       = lipgloss.Color("#727072")
	colorDimGray    = lipgloss.Color("#5b595c")
)

// Panel styles
var (
	// headerStyle is used for the header panel border
	headerStyle = lipgloss.NewStyle().
			BorderStyle(lipgloss.RoundedBorder()).
			BorderForeground(colorDimGray).
			Padding(0, 1)

	// headerLabelStyle is used for labels in the header
	headerLabelStyle = lipgloss.NewStyle().
				Foreground(colorGray)

	// headerValueStyle is used for values in the header
	headerValueStyle = lipgloss.NewStyle().
				Foreground(colorForeground).
				Bold(true)

	// progressBarStyle is the outer border for the progress bar section
	progressBarStyle = lipgloss.NewStyle().
				BorderStyle(lipgloss.RoundedBorder()).
				BorderForeground(colorDimGray).
				Padding(0, 1)

	// progressFillStyle is the filled portion of the progress bar
	progressFillStyle = lipgloss.NewStyle().
				Foreground(colorGreen)

	// progressEmptyStyle is the empty portion of the progress bar
	progressEmptyStyle = lipgloss.NewStyle().
				Foreground(colorDimGray)

	// panelStyle is used for scrollable panels (prompt and output)
	panelStyle = lipgloss.NewStyle().
			BorderStyle(lipgloss.RoundedBorder()).
			BorderForeground(colorDimGray).
			Padding(0, 1)

	// panelTitleStyle is used for panel titles
	panelTitleStyle = lipgloss.NewStyle().
			Foreground(colorMagenta).
			Bold(true)

	// panelFocusedStyle is used for focused panel border
	panelFocusedStyle = lipgloss.NewStyle().
				BorderStyle(lipgloss.RoundedBorder()).
				BorderForeground(colorYellow).
				Padding(0, 1)

	// statusBarStyle is used for the status bar
	statusBarStyle = lipgloss.NewStyle().
			BorderStyle(lipgloss.RoundedBorder()).
			BorderForeground(colorDimGray).
			Padding(0, 1)

	// scrollIndicatorStyle is for scroll indicators
	scrollIndicatorStyle = lipgloss.NewStyle().
				Foreground(colorGray).
				Italic(true)
)

// Status indicator styles
var (
	statusRunningStyle = lipgloss.NewStyle().
				Foreground(colorOrange).
				Bold(true)

	statusCompletedStyle = lipgloss.NewStyle().
				Foreground(colorGreen).
				Bold(true)

	statusFailedStyle = lipgloss.NewStyle().
				Foreground(colorRed).
				Bold(true)

	statusPendingStyle = lipgloss.NewStyle().
				Foreground(colorGray)
)

// Help text styles
var (
	helpKeyStyle = lipgloss.NewStyle().
			Foreground(colorYellow)

	helpDescStyle = lipgloss.NewStyle().
			Foreground(colorGray)

	helpSeparatorStyle = lipgloss.NewStyle().
				Foreground(colorDimGray)
)

// Error styles
var (
	errorStyle = lipgloss.NewStyle().
			Foreground(colorRed).
			Bold(true)

	errorMessageStyle = lipgloss.NewStyle().
				Foreground(colorRed)
)

// Floating window styles
var (
	floatingWindowStyle = lipgloss.NewStyle().
				BorderStyle(lipgloss.DoubleBorder()).
				BorderForeground(colorGreen).
				Padding(0, 1)

	floatingTitleStyle = lipgloss.NewStyle().
				Foreground(colorGreen).
				Bold(true)
)
