# Ralph

Ralph is an iterative AI development tool that runs [Claude Code](https://docs.anthropic.com/en/docs/claude-code) in automated loops against a plan file. It orchestrates a developer → reviewer agent cycle using [Jujutsu](https://github.com/martinvonz/jj) for change tracking, and stops when both agents approve the work or the iteration limit is reached.

## Opinionated Choices

Ralph makes deliberate choices that reflect a specific vision of AI-assisted development:

| Choice | What It Means |
|--------|---------------|
| **[Jujutsu](https://github.com/martinvonz/jj) for version control** | Not git. Ralph requires jj and uses its change-based model for tracking cumulative diffs across iterations. |
| **[Claude](https://www.anthropic.com/claude) only** | Uses Claude Code CLI exclusively. [Opus](https://www.anthropic.com/claude/opus) is the default model for development work. |
| **Developer → Reviewer loop** | Two-agent architecture: a developer agent writes code, then a reviewer agent inspects changes. Work isn't complete until both agree. |
| **Plan-driven execution** | All work derives from a markdown plan file. Progress and learnings accumulate across iterations. |

## Requirements

- [Go](https://go.dev/) 1.22+
- [Jujutsu](https://github.com/martinvonz/jj) (jj) for version control
- [Claude Code CLI](https://docs.anthropic.com/en/docs/claude-code) (`claude` command)

## Installation

```bash
go install github.com/gerunddev/ralph/cmd/ralph@latest
```

Or build from source:

```bash
git clone https://github.com/gerunddev/ralph
cd ralph
go build -o ralph ./cmd/ralph
```

## Usage

```bash
# Run with a plan file
ralph plan.md

# Run with custom iteration limit
ralph plan.md --max-iterations 30

# Run with an inline prompt
ralph --prompt "Add a logout button to the navbar"

# Resume an existing execution
ralph --resume <plan-id>
```

## How It Works

1. **Write a plan**: Create a markdown file describing what you want to build
2. **Ralph iterates**: Each iteration runs a developer → reviewer cycle:
   - Developer agent works on the plan, tracking progress and learnings
   - Reviewer agent inspects the cumulative diff, either approving or providing feedback
3. **Completion**: Loop ends when both agents approve or max iterations reached

Progress and learnings persist to a local SQLite database, so you can resume interrupted sessions.

## Configuration

Ralph uses `~/.config/ralph/config.json` (optional):

```json
{
  "projects_dir": "~/.local/share/ralph/projects",
  "max_iterations": 20,
  "claude": {
    "model": "opus",
    "max_turns": 50
  }
}
```

| Option | Default | Description |
|--------|---------|-------------|
| `projects_dir` | `~/.local/share/ralph/projects` | Where to store project data |
| `max_iterations` | `15` | Max iterations before stopping |
| `claude.model` | `opus` | Claude model for development |
| `claude.max_turns` | `50` | Max turns per agent session |

## TUI Keybindings

| Key | Action |
|-----|--------|
| `q` | Quit |
| `↑` / `↓` | Scroll output |

## License

MIT
