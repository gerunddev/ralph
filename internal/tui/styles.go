// Package tui provides the Bubble Tea TUI for Ralph.
package tui

import (
	"strings"

	"github.com/charmbracelet/lipgloss"
)

// =============================================================================
// COLOR PALETTE - Monokai Pro with depth-through-intensity
// =============================================================================

var (
	// Base colors
	colorForeground = lipgloss.Color("#fcfcfa")
	colorBackground = lipgloss.Color("#2d2a2e") // For future use

	// Cyan family (Claude output, Read tools)
	colorCyan      = lipgloss.Color("#78dce8") // Primary
	colorCyanLight = lipgloss.Color("#a1eaf8") // Light
	colorCyanDim   = lipgloss.Color("#4b8a94") // Dim

	// Green family (Success, Bash commands, commits)
	colorGreen      = lipgloss.Color("#a9dc76") // Primary
	colorGreenLight = lipgloss.Color("#c4e8a4") // Light
	colorGreenDim   = lipgloss.Color("#6a8a4a") // Dim

	// Yellow family (Write/Edit tools, focus, attention)
	colorYellow      = lipgloss.Color("#ffd866") // Primary
	colorYellowLight = lipgloss.Color("#ffe9a0") // Light
	colorYellowDim   = lipgloss.Color("#9a8340") // Dim

	// Orange family (Running status, misc tools)
	colorOrange      = lipgloss.Color("#fc9867") // Primary
	colorOrangeLight = lipgloss.Color("#fdb899") // Light
	colorOrangeDim   = lipgloss.Color("#9a5e3f") // Dim

	// Red family (Errors only)
	colorRed      = lipgloss.Color("#ff6188") // Primary
	colorRedLight = lipgloss.Color("#ff97ab") // Light
	colorRedDim   = lipgloss.Color("#993a52") // Dim

	// Magenta family (Structure, Reviewing, Search tools)
	colorMagenta      = lipgloss.Color("#ab9df2") // Primary
	colorMagentaLight = lipgloss.Color("#c9bff7") // Light
	colorMagentaDim   = lipgloss.Color("#6e6494") // Dim

	// Neutral family
	colorGray    = lipgloss.Color("#727072")
	colorDimGray = lipgloss.Color("#5b595c")
)

// =============================================================================
// TOOL CATEGORY SYSTEM
// =============================================================================

// ToolCategory represents a category of Claude tools.
type ToolCategory int

const (
	ToolCategoryRead ToolCategory = iota
	ToolCategoryWrite
	ToolCategoryBash
	ToolCategorySearch
	ToolCategoryOther
)

// GetToolCategory returns the category for a tool name.
func GetToolCategory(toolName string) ToolCategory {
	switch toolName {
	case "Read":
		return ToolCategoryRead
	case "Write", "Edit":
		return ToolCategoryWrite
	case "Bash":
		return ToolCategoryBash
	case "Grep", "Glob", "WebSearch", "WebFetch":
		return ToolCategorySearch
	default:
		return ToolCategoryOther
	}
}

// =============================================================================
// PANEL STYLES
// =============================================================================

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
				Foreground(colorDimGray).
				Italic(true)
)

// =============================================================================
// STATUS STYLES
// =============================================================================

// Status indicator styles
var (
	statusRunningStyle = lipgloss.NewStyle().
				Foreground(colorOrange).
				Bold(true)

	statusDevelopingStyle = lipgloss.NewStyle().
				Foreground(colorCyan).
				Bold(true)

	statusReviewingStyle = lipgloss.NewStyle().
				Foreground(colorMagenta).
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

// =============================================================================
// HELP STYLES
// =============================================================================

// Help text styles
var (
	helpKeyStyle = lipgloss.NewStyle().
			Foreground(colorYellow)

	helpDescStyle = lipgloss.NewStyle().
			Foreground(colorGray)

	helpSeparatorStyle = lipgloss.NewStyle().
				Foreground(colorDimGray)
)

// =============================================================================
// ERROR STYLES
// =============================================================================

// Error styles
var (
	errorStyle = lipgloss.NewStyle().
			Foreground(colorRed).
			Bold(true)

	errorMessageStyle = lipgloss.NewStyle().
				Foreground(colorRedLight)
)

// =============================================================================
// FLOATING WINDOW STYLES
// =============================================================================

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

// =============================================================================
// TOOL CALL STYLES (by category)
// =============================================================================

// Tool styles by category
var (
	// Read operations - Cyan family
	toolReadStyle = lipgloss.NewStyle().
			Foreground(colorCyan).
			Bold(true)
	toolReadParamStyle = lipgloss.NewStyle().
				Foreground(colorCyanLight)

	// Write/Edit operations - Yellow family
	toolWriteStyle = lipgloss.NewStyle().
			Foreground(colorYellow).
			Bold(true)
	toolWriteParamStyle = lipgloss.NewStyle().
				Foreground(colorYellowLight)

	// Bash/Command operations - Green family
	toolBashStyle = lipgloss.NewStyle().
			Foreground(colorGreen).
			Bold(true)
	toolBashParamStyle = lipgloss.NewStyle().
				Foreground(colorGreenLight)

	// Search operations - Magenta family
	toolSearchStyle = lipgloss.NewStyle().
			Foreground(colorMagenta).
			Bold(true)
	toolSearchParamStyle = lipgloss.NewStyle().
				Foreground(colorMagentaLight)

	// Other/misc tools - Orange family
	toolOtherStyle = lipgloss.NewStyle().
			Foreground(colorOrange).
			Bold(true)
	toolOtherParamStyle = lipgloss.NewStyle().
				Foreground(colorOrangeLight)

	// Chevron separator and icon for tool calls
	toolChevronStyle = lipgloss.NewStyle().
				Foreground(colorDimGray)
	toolIconStyle = lipgloss.NewStyle().
			Foreground(colorGray)
)

// GetToolStyles returns the name and param styles for a tool category.
func GetToolStyles(category ToolCategory) (nameStyle, paramStyle lipgloss.Style) {
	switch category {
	case ToolCategoryRead:
		return toolReadStyle, toolReadParamStyle
	case ToolCategoryWrite:
		return toolWriteStyle, toolWriteParamStyle
	case ToolCategoryBash:
		return toolBashStyle, toolBashParamStyle
	case ToolCategorySearch:
		return toolSearchStyle, toolSearchParamStyle
	default:
		return toolOtherStyle, toolOtherParamStyle
	}
}

// =============================================================================
// PHASE STYLES (for iteration markers)
// =============================================================================

// Phase-colored text styles for iteration markers
var (
	iterationTextStyle = lipgloss.NewStyle().
				Foreground(colorForeground)

	iterationDashStyle = lipgloss.NewStyle().
				Foreground(colorDimGray)

	iterationBulletStyle = lipgloss.NewStyle().
				Foreground(colorDimGray)

	phaseRunningStyle = lipgloss.NewStyle().
				Foreground(colorOrange)

	phaseDevelopingStyle = lipgloss.NewStyle().
				Foreground(colorCyan)

	phaseReviewingStyle = lipgloss.NewStyle().
				Foreground(colorMagenta)

	phaseCompletedStyle = lipgloss.NewStyle().
				Foreground(colorGreen)

	phaseFailedStyle = lipgloss.NewStyle().
				Foreground(colorRed)
)

// GetPhaseStyle returns the appropriate style for a phase name.
func GetPhaseStyle(phase string) lipgloss.Style {
	switch strings.ToLower(phase) {
	case "running", "in progress":
		return phaseRunningStyle
	case "developing":
		return phaseDevelopingStyle
	case "reviewing":
		return phaseReviewingStyle
	case "completed", "done", "complete":
		return phaseCompletedStyle
	case "failed", "error", "max iterations":
		return phaseFailedStyle
	default:
		return lipgloss.NewStyle().Foreground(colorGray)
	}
}

// =============================================================================
// MESSAGE CONTENT STYLES
// =============================================================================

// Message content styles - for aesthetic formatting of Claude session output
var (
	// Section dividers and markers
	sectionDividerStyle = lipgloss.NewStyle().
				Foreground(colorDimGray)

	doneMarkerStyle = lipgloss.NewStyle().
			Foreground(colorGreen).
			Bold(true)

	// System messages (jj operations, commits, etc.)
	systemMessageStyle = lipgloss.NewStyle().
				Foreground(colorGray).
				Italic(true)
)
