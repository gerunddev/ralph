# Ralph

Ralph is an iterative AI development tool that runs [Claude Code](https://docs.anthropic.com/en/docs/claude-code) in automated loops against a plan. It orchestrates a developer → reviewer agent cycle using [Jujutsu](https://github.com/martinvonz/jj) for change tracking, and stops when both agents approve the work or the iteration limit is reached.

## Opinionated Choices

| Choice | What It Means |
|--------|---------------|
| **[Jujutsu](https://github.com/martinvonz/jj) for version control** | Not git. Ralph requires jj and uses its change-based model for tracking cumulative diffs across iterations. |
| **[Claude](https://www.anthropic.com/claude) only** | Uses Claude Code CLI exclusively. [Opus](https://www.anthropic.com/claude/opus) is the default model. |
| **Developer → Reviewer loop** | Two-agent architecture: a developer agent writes code, then a reviewer agent inspects the cumulative diff. Work isn't complete until both agree. |
| **Plan-driven execution** | All work derives from a markdown plan file (or inline prompt). Progress and learnings accumulate across iterations. |

## Requirements

- [Go](https://go.dev/) 1.22+
- [Jujutsu](https://github.com/martinvonz/jj) (jj) for version control
- [Claude Code CLI](https://docs.anthropic.com/en/docs/claude-code) (`claude` command)

## Installation

```bash
go install github.com/gerunddev/ralph@latest
```

Or build from source:

```bash
git clone https://github.com/gerunddev/ralph
cd ralph
go build -o ralph .
```

## Usage

```bash
# Run with a plan file
ralph plan.md

# Run with custom iteration limit
ralph plan.md --max-iterations 30

# Run with an inline prompt
ralph -p "Add a logout button to the navbar"

# Resume an existing execution
ralph -r <plan-id>

# Extreme mode: keep going +3 iterations after agents think they're done
ralph plan.md --extreme
```

### CLI Flags

| Flag | Short | Description |
|------|-------|-------------|
| `--resume <id>` | `-r` | Resume execution of an existing plan by ID |
| `--prompt <text>` | `-p` | Use inline prompt as the plan instead of a file |
| `--max-iterations <N>` | | Override max iterations from config |
| `--extreme` | `-x` | Extreme mode: +3 iterations after agents agree |

### Task Management

While Ralph is running, you can modify task plans on the fly using the `task` subcommand:

```bash
# List tasks in a project
ralph task list <project-id>

# Export a task description for editing
ralph task export <project-id> <task-sequence>
ralph task export <project-id> <task-sequence> -o task.md
ralph task export <project-id> <task-sequence> --metadata=false  # omit metadata comments

# Import an edited task description
ralph task import <project-id> <task-sequence> task.md
ralph task import <project-id> <task-sequence> -    # from stdin
ralph task import <project-id> <task-sequence> task.md -f  # skip confirmation
ralph task import <project-id> <task-sequence> task.md --strip-metadata=false  # keep metadata comments
```

## How It Works

1. **Write a plan**: Create a markdown file describing what you want to build (or pass an inline prompt with `-p`)
2. **Ralph iterates**: Each iteration runs a developer → reviewer cycle:
   - **Developer agent** works on the plan, tracking progress and learnings across iterations
   - If the developer signals done *without making file edits*, the **reviewer agent** inspects the cumulative jj diff from the start of the session
   - If the developer made file edits, it must do at least one more review cycle before signaling done
   - All changes happen directly in the current jj change — no `jj new`, `jj commit`, or `jj describe`
3. **Completion**: Loop ends when both agents approve or max iterations is reached (a normal termination, not an error)

### Resilience

- If a Claude session hits **50% context window usage**, that session is stopped and the loop continues with a fresh session on the next iteration. Progress and learnings carry over.
- Diffs larger than **256KB** are automatically truncated before being sent to the reviewer, preventing context window exhaustion on large changesets.
- Progress and learnings persist to a local **SQLite database**, so you can resume interrupted sessions with `ralph -r <plan-id>`.

### Extreme Mode

With `--extreme` / `-x`, Ralph doesn't stop when both agents first agree. Instead, it triggers +3 additional iterations, pushing the agents to find more issues or improvements. The iteration counter displays as `N/X` until extreme mode triggers, then shows the actual new max.

## TUI

Ralph runs in a full-screen terminal UI built with [Bubble Tea](https://github.com/charmbracelet/bubbletea). It shows:

- A header with iteration count, status, and the plan ID
- A scrollable feed of developer and reviewer output, including streamed Claude text and tool calls
- A floating summary window on completion or when the iteration limit is reached

### Status Indicators

| Status | Meaning |
|--------|---------|
| Pending | Initial state before the loop starts |
| Running | Iteration starting (between agent phases) |
| Developing | Developer agent is active |
| Reviewing | Reviewer agent is inspecting the diff |
| Completed | Both agents approved the work |
| Stopped | Max iterations reached |

### Keybindings

| Key | Action |
|-----|--------|
| `↑` / `↓` | Scroll output |
| `q` / `Ctrl+C` | Quit |
| `Enter` / `Esc` | Dismiss floating window |

## Configuration

Ralph uses `~/.config/ralph/config.json` (optional):

```json
{
  "projects_dir": "~/.local/share/ralph/projects",
  "max_iterations": 15,
  "max_task_attempts": 10,
  "claude": {
    "model": "opus",
    "max_turns": 50,
    "verbose": true
  },
  "agents": {
    "developer": "/path/to/custom-developer-prompt.md",
    "reviewer": "/path/to/custom-reviewer-prompt.md",
    "planner": "/path/to/custom-planner-prompt.md",
    "documenter": "/path/to/custom-documenter-prompt.md"
  }
}
```

| Option | Default | Description |
|--------|---------|-------------|
| `projects_dir` | `~/.local/share/ralph/projects` | Where to store project databases |
| `max_iterations` | `15` | Max iterations before stopping |
| `max_task_attempts` | `10` | Max attempts per task before failing |
| `claude.model` | `opus` | Claude model for development |
| `claude.max_turns` | `50` | Max turns per Claude session |
| `claude.verbose` | `true` | Enable verbose Claude CLI output |
| `agents.developer` | *(built-in)* | Path to custom developer agent prompt |
| `agents.reviewer` | *(built-in)* | Path to custom reviewer agent prompt |
| `agents.planner` | *(built-in)* | Path to custom planner agent prompt |
| `agents.documenter` | *(built-in)* | Path to custom documenter agent prompt |

## License

MIT
