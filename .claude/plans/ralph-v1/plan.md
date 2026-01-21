# Feature: Ralph TUI Application (Continuation)

## Overview

Ralph is a TUI application that automates Claude Code sessions using plan-based development. It implements the "Ralph Loop" pattern: breaking a plan into discrete tasks, then iterating each task through developer->reviewer cycles until complete.

## Completed Work (Tasks 1-3)

### Task 1: Foundation
- Project structure with Go modules
- Cobra CLI skeleton
- Stub files for all packages

### Task 2: Database Layer
- SQLite database with auto-migration
- Models: Project, Task, Session, Message, Feedback
- Full CRUD operations with type-safe status constants
- Comprehensive tests

### Task 3: Configuration System
- Load from `~/.config/ralph/config.json`
- Defaults for all settings
- Path expansion, validation
- Agent prompt loading (custom or embedded defaults)
- Comprehensive tests

## Requirements (Remaining)

### Functional Requirements
1. Execute Claude Code in print mode with stream-json output
2. Parse streaming JSON and store in database
3. Implement developer->reviewer loop until approval
4. Iterate through tasks with jj change per task
5. TUI showing project selection and task progress
6. CLI flag `-c` to create project from plan file
7. User review feedback after all tasks complete
8. Capture learnings to AGENTS.md and README.md
9. Restart from interrupted state

### Non-Functional Requirements
- Use Bubble Tea for TUI
- Use Jujutsu (jj) for version control
- Store all history in SQLite
- Configurable agent prompts

## Architecture

```
cmd/ralph/main.go          - CLI entry point
internal/
  app/app.go               - Application bootstrap
  config/                  - Configuration (DONE)
  db/                      - Database layer (DONE)
  agents/                  - Agent prompts and construction
  claude/                  - Claude CLI wrapper
  jj/                      - Jujutsu CLI wrapper
  engine/                  - Orchestration loops
  tui/                     - Bubble Tea components
```

### Data Flow
1. User launches with plan file or selects existing project
2. Planner agent breaks plan into tasks (stored in DB)
3. For each task:
   a. `jj new` with task description
   b. Developer agent implements
   c. Reviewer agent reviews (`jj show`)
   d. Loop until approved or max iterations
4. User provides final review feedback
5. Capture learnings

## Task Breakdown

| Task | Description | Dependencies | Plan File |
|------|-------------|--------------|-----------|
| 4 | JJ Client | None | [task-04-jj-client.md](tasks/task-04-jj-client.md) |
| 5 | Claude Client & Parser | None | [task-05-claude-client.md](tasks/task-05-claude-client.md) |
| 6 | Agent Prompt Construction | 5 | [task-06-agent-prompts.md](tasks/task-06-agent-prompts.md) |
| 7 | Review Loop | 5, 6 | [task-07-review-loop.md](tasks/task-07-review-loop.md) |
| 8 | Task Loop | 4, 7 | [task-08-task-loop.md](tasks/task-08-task-loop.md) |
| 9 | Engine Orchestration | 8 | [task-09-engine.md](tasks/task-09-engine.md) |
| 10 | TUI: Project Selection | DB | [task-10-tui-project-selection.md](tasks/task-10-tui-project-selection.md) |
| 11 | TUI: Task Progress | 9, 10 | [task-11-tui-task-progress.md](tasks/task-11-tui-task-progress.md) |
| 12 | CLI & App Bootstrap | 9, 10, 11 | [task-12-cli-bootstrap.md](tasks/task-12-cli-bootstrap.md) |
| 13 | User Review Feedback | 12 | [task-13-user-feedback.md](tasks/task-13-user-feedback.md) |
| 14 | Learnings Capture | 13 | [task-14-learnings.md](tasks/task-14-learnings.md) |
| 15 | Restart from Interim State | 12 | [task-15-restart.md](tasks/task-15-restart.md) |

## Suggested Implementation Order

Tasks can be parallelized where dependencies allow:

### Phase 1: Core Infrastructure (Can be parallel)
- **Task 4**: JJ Client - standalone, no deps
- **Task 5**: Claude Client - standalone, no deps

### Phase 2: Agent Layer
- **Task 6**: Agent Prompt Construction - needs Task 5 for context on Claude integration

### Phase 3: Execution Engine
- **Task 7**: Review Loop - core dev->reviewer cycle
- **Task 8**: Task Loop - iterates through project tasks
- **Task 9**: Engine - ties everything together

### Phase 4: User Interface
- **Task 10**: TUI Project Selection - can start after DB is ready
- **Task 11**: TUI Task Progress - needs engine events
- **Task 12**: CLI & Bootstrap - final wiring

### Phase 5: Polish
- **Task 13**: User Feedback - post-completion flow
- **Task 14**: Learnings Capture - documentation
- **Task 15**: Restart - resilience

### Minimum Viable Product (MVP)

For a working MVP, complete Tasks 4-12. Tasks 13-15 can be deferred.

### Phase 6: Post-MVP Addendums

These tasks enhance the MVP after core functionality is complete:

| Task | Description | Dependencies | Plan File |
|------|-------------|--------------|-----------|
| A-01 | Per-Project SQLite Database | 12 | [addendum-01-per-project-database.md](tasks/addendum-01-per-project-database.md) |
| A-02 | Task Content Export/Import CLI | A-01 (or 12) | [addendum-02-task-export-import.md](tasks/addendum-02-task-export-import.md) |
| A-03 | Task Loop Pause Mode | 11, 12 | [addendum-03-pause-mode.md](tasks/addendum-03-pause-mode.md) |

**A-01: Per-Project Database** - Each project gets its own SQLite database file, enabling schema evolution without backwards compatibility concerns. Projects are discovered by scanning the projects directory.

**A-02: Task Export/Import** - CLI commands (`ralph task export`, `ralph task import`) to export task descriptions to files and import modified content back. Enables on-the-fly task plan editing via export -> edit -> import workflow.

**A-03: Pause Mode** - Adds ability to pause the task loop between tasks for manual approval. Default is continuous (no pausing). TUI hotkey `p` toggles pause mode on/off during execution.

## Testability

- Unit tests for each package
- Integration tests for engine loops (use mock Claude responses)
- TUI tests using Bubble Tea test utilities
- In-memory SQLite for all tests

## Security Considerations

- No secrets stored in config (Claude CLI handles auth)
- Validate file paths before reading
- Sanitize user input before shell commands
- No network access (all via CLI tools)

## Deployability

- Single binary distribution
- No external services required
- SQLite database auto-creates
- Config file optional (uses defaults)
