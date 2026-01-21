# Ralph

Ralph is an iterative AI development tool that runs Claude Code in automated loops against a plan file. It creates isolated jj (jujutsu) commits for each iteration, tracks progress and learnings across sessions, and stops when the AI declares the work complete or hits the iteration limit.

## Features

- **Single-agent iterative loop**: Claude works through a plan, tracking progress and learnings between iterations
- **Automatic version control**: Each iteration creates an isolated jj commit with AI-distilled commit messages
- **Resume support**: Pick up where you left off - progress and learnings persist across sessions
- **TUI interface**: Real-time visibility into Claude's work with Bubble Tea
- **Task management**: View, export, and modify tasks on-the-fly during execution

## Requirements

- Go 1.22+
- [Jujutsu](https://github.com/martinvonz/jj) (jj) for version control
- Claude Code CLI (`claude` command)

## Installation

```bash
go install github.com/gerund/ralph/cmd/ralph@latest
```

Or build from source:

```bash
git clone https://github.com/gerund/ralph
cd ralph
go build -o ralph ./cmd/ralph
```

## Quick Start

```bash
# Start a new execution from a plan file
ralph plan.md

# Start with custom iteration limit
ralph plan.md --max-iterations 30

# Resume an existing plan by ID
ralph -r abc123
ralph --resume abc123
```

## How It Works

1. **Provide a plan**: Write a markdown file describing what you want to build
2. **Ralph runs iterations**: Each iteration:
   - Builds a prompt with the plan + current progress + learnings
   - Creates a new jj change for isolation
   - Runs Claude Code with the prompt
   - Parses output for progress/learnings or completion marker
   - Distills a commit message using Claude Haiku
   - Commits the changes
3. **Completion**: Loop ends when Claude outputs `DONE DONE DONE!!!` or hits max iterations

### Agent Output Format

Claude must output one of two formats:

**When done:**
```
DONE DONE DONE!!!
```

**When continuing:**
```markdown
## Progress
What's been built, current state, completed items...

## Learnings
Insights about the codebase, patterns discovered, what didn't work...
```

## Commands

### Main Commands

```bash
# Run with a plan file
ralph <plan-file>
ralph plan.md --max-iterations 30

# Resume existing execution
ralph -r <plan-id>
ralph --resume <plan-id>
```

### Task Management

```bash
# List all tasks in a project
ralph task list <project-id>

# Export task description
ralph task export <project-id> <sequence>           # stdout
ralph task export <project-id> <sequence> -o task.md  # file
ralph task export <project-id> 3 --metadata=false      # without headers

# Import/update task description  
ralph task import <project-id> <sequence> <file>
ralph task import <project-id> <sequence> -        # from stdin
ralph task import <project-id> 3 task.md -f        # skip confirmation
```

Task status indicators:
- `[ ]` pending
- `[~]` in progress
- `[x]` completed
- `[!]` failed
- `[^]` escalated

## Configuration

Ralph uses `~/.config/ralph/config.json` (optional):

```json
{
  "projects_dir": "~/.local/share/ralph/projects",
  "max_iterations": 20,
  "claude": {
    "model": "opus",
    "max_turns": 50,
    "verbose": true
  }
}
```

| Option | Default | Description |
|--------|---------|-------------|
| `projects_dir` | `~/.local/share/ralph/projects` | Database storage location |
| `max_iterations` | `15` | Max iterations before stopping |
| `claude.model` | `opus` | Claude model for development |
| `claude.max_turns` | `50` | Max conversation turns per session |
| `claude.verbose` | `true` | Show Claude CLI output |

## Architecture

```
cmd/ralph/
├── main.go           # CLI entry point (cobra)
├── task.go           # Task subcommand group
├── task_list.go      # ralph task list
├── task_export.go    # ralph task export  
└── task_import.go    # ralph task import

internal/
├── app/v2/           # App orchestration (connects loop to TUI)
├── loop/             # Main iteration loop
├── agent/            # Prompt construction
├── parser/           # Output parsing (progress/learnings/done)
├── distill/          # Commit message generation (Haiku)
├── claude/           # Claude Code CLI wrapper
├── jj/               # Jujutsu VCS wrapper
├── db/               # SQLite persistence
├── config/           # Configuration loading
├── tui/v2/           # Bubble Tea TUI
└── log/              # Structured logging
```

### Core Components

**Loop** (`internal/loop/loop.go`)
- Orchestrates the main iteration cycle
- Emits events for TUI consumption
- Handles context cancellation and max iterations

**Agent Prompt** (`internal/agent/prompt.go`)
- Builds prompts with plan + progress + learnings
- Uses Go templates for consistent structure

**Parser** (`internal/parser/parser.go`)
- Extracts `## Progress` and `## Learnings` sections
- Detects `DONE DONE DONE!!!` completion marker
- Handles malformed output gracefully

**Distiller** (`internal/distill/distill.go`)
- Uses Claude Haiku for fast commit message generation
- Follows conventional commit format
- Falls back gracefully on errors

**Claude Client** (`internal/claude/client.go`)
- Wraps the `claude` CLI command
- Streams events via channels
- Handles JSON-LD output parsing

**JJ Client** (`internal/jj/jj.go`)
- Wraps jujutsu commands (`jj new`, `jj commit`, `jj status`)
- Detects repository and command availability

## Data Storage

Ralph stores data in SQLite databases:

- **Plans**: Original plan content and metadata
- **Sessions**: Each Claude invocation with input/output
- **Events**: Raw Claude streaming events
- **Progress**: Accumulated progress snapshots
- **Learnings**: Accumulated learnings snapshots

Default location: `~/.local/share/ralph/projects/ralph-v2.db`

## Development

```bash
# Run tests
go test ./...

# Run tests with coverage
go test ./... -cover

# Lint
golangci-lint run

# Build
go build ./cmd/ralph
```

## TUI Keybindings

| Key | Action |
|-----|--------|
| `q` / `Ctrl+C` | Quit |
| `↑` / `↓` | Scroll output |
| `Tab` | Switch panels |

## License

MIT
