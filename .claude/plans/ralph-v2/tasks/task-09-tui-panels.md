# Task 9: TUI Panels and Layout

## Objective

Create Bubble Tea TUI components for displaying execution status: plan ID, iteration progress, current prompt, and Claude streaming output.

## Requirements

1. **Header Panel**: Plan ID, iteration counter (e.g., "3/20"), status
2. **Progress Bar**: Visual iteration progress
3. **Prompt Panel**: Current prompt being sent (scrollable)
4. **Output Panel**: Claude streaming output (auto-scroll, scrollable)
5. **Status Bar**: Current action, errors, key hints
6. Responsive layout that uses terminal size
7. Keyboard navigation (scroll panels, quit)

## Layout

```
+------------------------------------------------------------------+
| Plan: abc123...  | Iteration: 3/20  | Status: Running            |
+------------------------------------------------------------------+
| [=========>                                              ] 15%   |
+------------------------------------------------------------------+
| Current Prompt                                                    |
| ---------------------------------------------------------------- |
| # Instructions                                                   |
| You are an experienced software developer...                     |
| ...                                                              |
|                                                         [scroll] |
+------------------------------------------------------------------+
| Claude Output                                                     |
| ---------------------------------------------------------------- |
| I'll start by examining the codebase structure...                |
| > Reading file: internal/config/config.go                        |
| ...                                                              |
|                                                    [auto-scroll] |
+------------------------------------------------------------------+
| Running Claude session... | q: quit | j/k: scroll                |
+------------------------------------------------------------------+
```

## Components

```go
// Header shows plan info and iteration
type Header struct {
    PlanID    string
    Iteration int
    MaxIter   int
    Status    string
}

// ProgressBar shows iteration progress
type ProgressBar struct {
    Current int
    Total   int
}

// ScrollablePanel is a generic scrollable text panel
type ScrollablePanel struct {
    Title      string
    Content    string
    AutoScroll bool
    // scroll position, viewport size, etc.
}

// StatusBar shows current action and key hints
type StatusBar struct {
    Message string
    Error   string
}
```

## Styling (Lipgloss)

- Use subtle colors (not garish)
- Clear visual hierarchy
- Borders between sections
- Highlighted status for errors/completion

## Key Bindings

- `q` / `Ctrl+C`: Quit
- `j` / `Down`: Scroll down in focused panel
- `k` / `Up`: Scroll up in focused panel
- `Tab`: Switch focus between panels
- `g`: Go to top
- `G`: Go to bottom

## Interface

```go
type Model struct {
    header      Header
    progressBar ProgressBar
    promptPanel ScrollablePanel
    outputPanel ScrollablePanel
    statusBar   StatusBar
    // ...
}

func NewModel() Model
func (m Model) Init() tea.Cmd
func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd)
func (m Model) View() string

// Messages for updating state
type SetPlanMsg struct { ... }
type SetIterationMsg struct { ... }
type AppendOutputMsg struct { ... }
type SetPromptMsg struct { ... }
type SetStatusMsg struct { ... }
type SetErrorMsg struct { ... }
```

## Acceptance Criteria

- [ ] All panels render correctly
- [ ] Layout adapts to terminal size
- [ ] Scrolling works in both panels
- [ ] Auto-scroll follows new output
- [ ] Progress bar updates smoothly
- [ ] Status bar shows current action
- [ ] Error display is visible but not overwhelming
- [ ] Key bindings work as expected
- [ ] Clean quit without artifacts
- [ ] Unit tests for panel rendering

## Files to Create

- `internal/tui/app.go`
- `internal/tui/header.go`
- `internal/tui/progress.go`
- `internal/tui/panel.go`
- `internal/tui/status.go`
- `internal/tui/styles.go`
- `internal/tui/keys.go`
- `internal/tui/app_test.go`

## Notes

V1 has TUI code but it's structured differently (project selection, task list). This is a simpler single-screen design focused on watching one execution.

Consider using `bubbles` library components:
- `viewport` for scrollable panels
- `progress` for progress bar
