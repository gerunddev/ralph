# Ralph

Ralph is a terminal application that automates Claude Code sessions using a plan-based development workflow. It breaks a project plan into discrete tasks and iterates each through developer→reviewer cycles, using fresh Claude context windows for efficient handling of larger projects while maintaining code quality.

## Quick Start

```bash
# Start a new project with a plan file
ralph -c /path/to/plan.md

# Resume or select an existing project
ralph
```

## How It Works

1. **Planning**: Provide a markdown plan file describing your development goals. Ralph breaks this into logical tasks.

2. **Execution**: For each task:
   - Creates a new jujutsu change for isolation
   - Developer agent implements the task
   - Reviewer agent validates the work
   - If issues are found, the developer iterates with structured feedback
   - Task is marked complete when approved

3. **User Review**: After all tasks complete, you can provide additional feedback to refine the work.

4. **Documentation**: Ralph captures learnings and updates project documentation automatically.

## Commands

### Create a New Project

```bash
ralph -c /path/to/plan.md
```

Reads a plan file, creates a new project, and launches the TUI to begin work.

### Select/Resume a Project

```bash
ralph
```

Shows a list of existing projects to resume or continue working on.

### Submit Feedback

```bash
ralph feedback -p <project-id> -f feedback.md
```

Submit feedback for a project while the TUI is not running. Restart the TUI to process the feedback.

### Task Management

```bash
# List all tasks with status
ralph task list <project-id>

# Export a task description
ralph task export <project-id> <sequence>
ralph task export <project-id> 3 -o task.md

# Import/update a task description
ralph task import <project-id> <sequence> <file>
ralph task import <project-id> 3 -    # read from stdin
```

Task status indicators: `[ ]` pending, `[~]` in-progress, `[x]` completed, `[!]` failed, `[^]` escalated

## Configuration

Ralph uses an optional configuration file at `~/.config/ralph/config.json`:

```json
{
  "projects_dir": "~/.local/share/ralph/projects",
  "max_review_iterations": 5,
  "max_task_attempts": 10,
  "default_pause_mode": false,
  "claude": {
    "model": "opus",
    "max_turns": 50,
    "max_budget_usd": 10.0
  },
  "agents": {
    "developer": "/path/to/custom/developer.md",
    "reviewer": "/path/to/custom/reviewer.md",
    "planner": "/path/to/custom/planner.md",
    "documenter": "/path/to/custom/documenter.md"
  }
}
```

| Option | Description |
|--------|-------------|
| `projects_dir` | Where project databases are stored |
| `max_review_iterations` | Maximum developer→reviewer cycles per task |
| `max_task_attempts` | Maximum attempts before failing a task |
| `default_pause_mode` | Pause after each task for manual review |
| `claude.model` | Claude model to use |
| `claude.max_turns` | Maximum conversation turns per session |
| `claude.max_budget_usd` | Budget limit per session |
| `agents.*` | Custom prompt files for each agent type |

## Agents

Ralph uses four specialized agents:

- **Planner**: Breaks the plan into discrete tasks
- **Developer**: Implements each task given the plan and task description
- **Reviewer**: Validates work and provides structured feedback
- **Documenter**: Captures learnings and updates documentation

Custom agent prompts can be configured to customize behavior.

## Requirements

- Go 1.21+
- [Jujutsu](https://github.com/martinvonz/jj) (jj) for version control
- Claude Code CLI

## Project Storage

Each project gets a unique ID and SQLite database stored in the projects directory (default: `~/.local/share/ralph/projects/`). Projects can be resumed from any point, including after interruptions.
