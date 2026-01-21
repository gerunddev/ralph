# Task 2: Database Schema & Migrations

## Objective

Create a centralized SQLite database at `~/.config/ralph/ralph.db` with tables for plans, sessions, events, progress, and learnings.

## Requirements

1. Database location: `~/.config/ralph/ralph.db`
2. Auto-create directory and database file
3. Auto-run migrations on startup
4. Tables: plans, sessions, events, progress, learnings
5. Foreign key constraints enabled
6. All timestamps in UTC

## Schema

```sql
CREATE TABLE plans (
    id TEXT PRIMARY KEY,
    origin_path TEXT NOT NULL,
    content TEXT NOT NULL,
    status TEXT NOT NULL DEFAULT 'pending',
    created_at TIMESTAMP NOT NULL,
    updated_at TIMESTAMP NOT NULL
);

CREATE TABLE sessions (
    id TEXT PRIMARY KEY,
    plan_id TEXT NOT NULL REFERENCES plans(id),
    iteration INTEGER NOT NULL,
    input_prompt TEXT NOT NULL,
    final_output TEXT,
    status TEXT NOT NULL DEFAULT 'running',
    created_at TIMESTAMP NOT NULL,
    completed_at TIMESTAMP
);

CREATE TABLE events (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    session_id TEXT NOT NULL REFERENCES sessions(id),
    sequence INTEGER NOT NULL,
    event_type TEXT NOT NULL,
    raw_json TEXT NOT NULL,
    created_at TIMESTAMP NOT NULL
);

CREATE TABLE progress (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    plan_id TEXT NOT NULL REFERENCES plans(id),
    session_id TEXT NOT NULL REFERENCES sessions(id),
    content TEXT NOT NULL,
    created_at TIMESTAMP NOT NULL
);

CREATE TABLE learnings (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    plan_id TEXT NOT NULL REFERENCES plans(id),
    session_id TEXT NOT NULL REFERENCES sessions(id),
    content TEXT NOT NULL,
    created_at TIMESTAMP NOT NULL
);

-- Indexes for common queries
CREATE INDEX idx_sessions_plan_id ON sessions(plan_id);
CREATE INDEX idx_events_session_id ON events(session_id);
CREATE INDEX idx_progress_plan_id ON progress(plan_id);
CREATE INDEX idx_learnings_plan_id ON learnings(plan_id);
```

## Status Values

- **Plan status**: `pending`, `running`, `completed`, `failed`
- **Session status**: `running`, `completed`, `failed`

## Interface

```go
type DB struct { ... }

func New(path string) (*DB, error)
func (d *DB) Close() error

// Plans
func (d *DB) CreatePlan(plan *Plan) error
func (d *DB) GetPlan(id string) (*Plan, error)
func (d *DB) UpdatePlanStatus(id string, status PlanStatus) error

// Sessions
func (d *DB) CreateSession(session *Session) error
func (d *DB) GetSession(id string) (*Session, error)
func (d *DB) CompleteSession(id string, status SessionStatus, finalOutput string) error
func (d *DB) GetSessionsByPlan(planID string) ([]*Session, error)
func (d *DB) GetLatestSession(planID string) (*Session, error)

// Events
func (d *DB) CreateEvent(event *Event) error
func (d *DB) GetEventsBySession(sessionID string) ([]*Event, error)

// Progress
func (d *DB) CreateProgress(progress *Progress) error
func (d *DB) GetLatestProgress(planID string) (*Progress, error)
func (d *DB) GetProgressHistory(planID string) ([]*Progress, error)

// Learnings
func (d *DB) CreateLearnings(learnings *Learnings) error
func (d *DB) GetLatestLearnings(planID string) (*Learnings, error)
func (d *DB) GetLearningsHistory(planID string) ([]*Learnings, error)
```

## Acceptance Criteria

- [ ] Creates database at `~/.config/ralph/ralph.db`
- [ ] Auto-creates directory if missing
- [ ] Runs migrations on New()
- [ ] All CRUD operations work correctly
- [ ] Foreign keys enforced
- [ ] GetLatest* returns nil (not error) when no records exist
- [ ] In-memory database support for tests (`:memory:`)
- [ ] Comprehensive unit tests

## Files to Create/Modify

- `internal/db/db.go` (rewrite from V1)
- `internal/db/models.go` (rewrite from V1)
- `internal/db/migrations.go` (rewrite from V1)
- `internal/db/db_test.go`

## Notes

This is a simplification of V1's database. V1 had projects, tasks, messages, feedback. V2 has plans, sessions, events, progress, learnings.
