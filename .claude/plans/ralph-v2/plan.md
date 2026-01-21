# Feature: Ralph V2 - Simplified Single-Agent Loop

## Overview

Ralph V2 is a simplified rewrite that replaces the multi-agent (planner→developer→reviewer) architecture with a single experienced developer agent that iterates until completion. The agent receives the plan, its accumulated progress, and learnings each iteration, then either declares "DONE DONE DONE!!!" or outputs updated progress and learnings.

## Requirements

### Functional Requirements

1. Run `ralph /path/to/plan.md` to start a new execution
2. Run `ralph -r <plan-id>` or `ralph --resume <plan-id>` to resume
3. Single agent prompted iteratively with: instructions + plan + progress + learnings
4. Agent outputs either:
   - "DONE DONE DONE!!!" (exactly this, nothing else) when complete
   - Two sections: Progress updates AND Learnings about codebase/problem
5. Each iteration: `jj new` → Claude session → Haiku distills commit message → `jj commit`
6. Store everything: input prompts, raw stream events, final output, progress, learnings
7. All data versioned and queryable over time
8. Max iterations from config (required), CLI can override
9. On errors: log and continue to next iteration
10. Feature-rich TUI showing plan ID, iteration, prompt, streaming output, progress bar

### Non-Functional Requirements

- Centralized SQLite database at `~/.config/ralph/ralph.db`
- Config file at `~/.config/ralph/config.json` (max_iterations required)
- Use Bubble Tea for TUI
- Use Jujutsu (jj) for version control
- Claude CLI with `--output-format stream-json --verbose`

## Architecture

```
cmd/ralph/main.go           - CLI entry point (cobra)
internal/
  config/config.go          - Configuration loading
  db/
    db.go                   - Database connection & migrations
    models.go               - Data models
  claude/
    client.go               - Claude CLI wrapper
    parser.go               - Stream JSON parser
    types.go                - Event types
  jj/jj.go                  - Jujutsu CLI wrapper
  agent/prompt.go           - Agent prompt construction
  distill/distill.go        - Haiku commit message distillation
  loop/loop.go              - Main iteration loop
  tui/
    app.go                  - Bubble Tea application
    panels.go               - UI panel components
    styles.go               - Lipgloss styles
```

### Data Model

```
plans
  - id (UUID, PK)
  - origin_path (TEXT) - file path for metadata
  - content (TEXT) - full plan text
  - status (TEXT) - pending/running/completed/failed
  - created_at (TIMESTAMP)
  - updated_at (TIMESTAMP)

sessions
  - id (UUID, PK)
  - plan_id (UUID, FK)
  - iteration (INT)
  - input_prompt (TEXT)
  - final_output (TEXT)
  - status (TEXT) - running/completed/failed
  - created_at (TIMESTAMP)
  - completed_at (TIMESTAMP)

events
  - id (INT, PK, autoincrement)
  - session_id (UUID, FK)
  - sequence (INT)
  - event_type (TEXT)
  - raw_json (TEXT)
  - created_at (TIMESTAMP)

progress
  - id (INT, PK, autoincrement)
  - plan_id (UUID, FK)
  - session_id (UUID, FK)
  - content (TEXT)
  - created_at (TIMESTAMP)

learnings
  - id (INT, PK, autoincrement)
  - plan_id (UUID, FK)
  - session_id (UUID, FK)
  - content (TEXT)
  - created_at (TIMESTAMP)
```

### Agent Prompt Structure

```
# Instructions

You are an experienced software developer working iteratively on a plan.
You can wear many hats: developer, reviewer, architect, security engineer.

## Your Capabilities
- Critically evaluate your own code; don't stop until you're confident it's right
- Find and fix security and performance issues
- Maintain high standards for coding best practices in every language
- Break work into smaller units and determine execution order
- Track your progress and learnings about the codebase

## Output Format

You MUST output one of two things:

### Option A: When you believe you're completely done
Output exactly this and nothing else:
DONE DONE DONE!!!

### Option B: When there's more work to do
Output two sections with these exact headers:

## Progress
[What you've built, completed, current state]

## Learnings
[Insights about the codebase, patterns discovered, approaches that didn't work]

---

# Plan
{plan_content}

---

# Progress So Far
{latest_progress or "No progress yet."}

---

# Learnings So Far
{latest_learnings or "No learnings yet."}
```

### Execution Flow

```
1. Parse CLI args (new plan path OR resume plan-id)
2. Load/create plan in DB
3. Load latest progress & learnings (if resuming)
4. Start TUI
5. Loop until "DONE DONE DONE!!!" or max_iterations:
   a. Build prompt (instructions + plan + progress + learnings)
   b. Run `jj new`
   c. Run Claude session (stream to TUI, store events)
   d. Parse output:
      - If "DONE DONE DONE!!!": mark complete, exit loop
      - Else: extract Progress/Learnings sections, store in DB
   e. Call Haiku to distill commit message
   f. Run `jj commit -m "<message>"`
   g. Increment iteration
6. Final status display
```

## Testability

- Unit tests for each package
- Mock Claude client for loop tests
- Mock jj client for loop tests
- In-memory SQLite for all tests
- Table-driven tests for prompt parsing
- TUI tests using Bubble Tea test utilities

## Security Considerations

- No secrets in config (Claude CLI handles auth)
- Validate file paths before reading plan files
- Sanitize commit messages before passing to jj
- No network access except via Claude CLI

## Deployability

- Single binary distribution
- Auto-create `~/.config/ralph/` directory
- Auto-migrate database schema
- Clear error on missing required config

## Tasks

See `tasks/` directory for detailed task breakdowns.

### Task Overview

| Task | Description | Dependencies |
|------|-------------|--------------|
| 1 | Config system | None |
| 2 | Database schema & migrations | None |
| 3 | Claude client with streaming | None |
| 4 | JJ client | None |
| 5 | Agent prompt builder | None |
| 6 | Haiku distillation | 3 |
| 7 | Output parser (progress/learnings/done) | None |
| 8 | Main loop orchestration | 1-7 |
| 9 | TUI panels and layout | None |
| 10 | TUI integration with loop | 8, 9 |
| 11 | CLI with new/resume modes | 8, 10 |

### Suggested Implementation Order

**Phase 1: Foundation (parallel)**
- Task 1: Config
- Task 2: Database
- Task 3: Claude client
- Task 4: JJ client

**Phase 2: Logic (parallel after Phase 1)**
- Task 5: Agent prompt
- Task 6: Haiku distillation
- Task 7: Output parser

**Phase 3: Core Loop**
- Task 8: Main loop (depends on 1-7)

**Phase 4: UI**
- Task 9: TUI panels
- Task 10: TUI integration

**Phase 5: CLI**
- Task 11: CLI wiring

### Minimum Viable Product

Tasks 1-8, 11 with minimal stdout output instead of TUI. TUI (9-10) can be added after.
