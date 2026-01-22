// Package db provides database connectivity and operations for Ralph.
package db

import "github.com/gerund/ralph/internal/log"

// schema is the SQL schema for the Ralph database.
const schema = `
-- Projects table
CREATE TABLE IF NOT EXISTS projects (
    id TEXT PRIMARY KEY,
    name TEXT NOT NULL,
    plan_text TEXT NOT NULL,
    status TEXT NOT NULL DEFAULT 'pending',
    user_feedback_state TEXT NOT NULL DEFAULT '',
    learnings_state TEXT NOT NULL DEFAULT '',
    created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
    updated_at DATETIME DEFAULT CURRENT_TIMESTAMP
);

-- Tasks table
CREATE TABLE IF NOT EXISTS tasks (
    id TEXT PRIMARY KEY,
    project_id TEXT NOT NULL,
    sequence INTEGER NOT NULL,
    title TEXT NOT NULL,
    description TEXT NOT NULL,
    status TEXT NOT NULL DEFAULT 'pending',
    jj_change_id TEXT,
    iteration_count INTEGER DEFAULT 0,
    created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
    updated_at DATETIME DEFAULT CURRENT_TIMESTAMP,
    FOREIGN KEY (project_id) REFERENCES projects(id)
);

-- Agent sessions table
CREATE TABLE IF NOT EXISTS sessions (
    id TEXT PRIMARY KEY,
    task_id TEXT NOT NULL,
    agent_type TEXT NOT NULL,
    iteration INTEGER NOT NULL,
    input_prompt TEXT NOT NULL,
    status TEXT NOT NULL DEFAULT 'running',
    created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
    completed_at DATETIME,
    FOREIGN KEY (task_id) REFERENCES tasks(id)
);

-- Session messages (streaming history)
CREATE TABLE IF NOT EXISTS messages (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    session_id TEXT NOT NULL,
    sequence INTEGER NOT NULL,
    message_type TEXT NOT NULL,
    content TEXT NOT NULL,
    created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
    FOREIGN KEY (session_id) REFERENCES sessions(id)
);

-- Review feedback
CREATE TABLE IF NOT EXISTS feedback (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    session_id TEXT NOT NULL,
    feedback_type TEXT NOT NULL,
    content TEXT,
    created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
    FOREIGN KEY (session_id) REFERENCES sessions(id)
);

-- Indexes for common queries
CREATE INDEX IF NOT EXISTS idx_tasks_project ON tasks(project_id);
CREATE INDEX IF NOT EXISTS idx_tasks_status ON tasks(status);
CREATE INDEX IF NOT EXISTS idx_sessions_task ON sessions(task_id);
CREATE INDEX IF NOT EXISTS idx_messages_session ON messages(session_id);
CREATE INDEX IF NOT EXISTS idx_feedback_session ON feedback(session_id);

-- Plans table
CREATE TABLE IF NOT EXISTS plans (
    id TEXT PRIMARY KEY,
    origin_path TEXT NOT NULL,
    content TEXT NOT NULL,
    status TEXT NOT NULL DEFAULT 'pending',
    base_change_id TEXT NOT NULL DEFAULT '',
    created_at DATETIME NOT NULL,
    updated_at DATETIME NOT NULL
);

-- Plan sessions table
CREATE TABLE IF NOT EXISTS plan_sessions (
    id TEXT PRIMARY KEY,
    plan_id TEXT NOT NULL,
    iteration INTEGER NOT NULL,
    input_prompt TEXT NOT NULL,
    final_output TEXT,
    status TEXT NOT NULL DEFAULT 'running',
    agent_type TEXT NOT NULL DEFAULT 'developer',
    created_at DATETIME NOT NULL,
    completed_at DATETIME,
    FOREIGN KEY (plan_id) REFERENCES plans(id)
);

-- Events table (stream events from Claude)
CREATE TABLE IF NOT EXISTS events (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    session_id TEXT NOT NULL,
    sequence INTEGER NOT NULL,
    event_type TEXT NOT NULL,
    raw_json TEXT NOT NULL,
    created_at DATETIME NOT NULL,
    FOREIGN KEY (session_id) REFERENCES plan_sessions(id)
);

-- Progress tracking table
CREATE TABLE IF NOT EXISTS progress (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    plan_id TEXT NOT NULL,
    session_id TEXT NOT NULL,
    content TEXT NOT NULL,
    created_at DATETIME NOT NULL,
    FOREIGN KEY (plan_id) REFERENCES plans(id),
    FOREIGN KEY (session_id) REFERENCES plan_sessions(id)
);

-- Learnings tracking table
CREATE TABLE IF NOT EXISTS learnings (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    plan_id TEXT NOT NULL,
    session_id TEXT NOT NULL,
    content TEXT NOT NULL,
    created_at DATETIME NOT NULL,
    FOREIGN KEY (plan_id) REFERENCES plans(id),
    FOREIGN KEY (session_id) REFERENCES plan_sessions(id)
);

-- Reviewer feedback table
CREATE TABLE IF NOT EXISTS reviewer_feedback (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    plan_id TEXT NOT NULL,
    session_id TEXT NOT NULL,
    content TEXT NOT NULL,
    created_at DATETIME NOT NULL,
    FOREIGN KEY (plan_id) REFERENCES plans(id),
    FOREIGN KEY (session_id) REFERENCES plan_sessions(id)
);

-- Plan-related indexes
CREATE INDEX IF NOT EXISTS idx_plan_sessions_plan ON plan_sessions(plan_id);
CREATE INDEX IF NOT EXISTS idx_events_session ON events(session_id);
CREATE INDEX IF NOT EXISTS idx_progress_plan ON progress(plan_id);
CREATE INDEX IF NOT EXISTS idx_learnings_plan ON learnings(plan_id);
CREATE INDEX IF NOT EXISTS idx_reviewer_feedback_plan ON reviewer_feedback(plan_id);
`

// Migrate runs all database migrations to ensure the schema is up to date.
func (d *DB) Migrate() error {
	// Create tables if they don't exist
	if _, err := d.conn.Exec(schema); err != nil {
		return err
	}

	// Run incremental migrations for existing databases
	return d.runMigrations()
}

// runMigrations applies incremental schema changes for existing databases.
func (d *DB) runMigrations() error {
	// Migration: Add user_feedback_state column to projects table
	if exists, err := d.columnExists("projects", "user_feedback_state"); err != nil {
		return err
	} else if !exists {
		if _, err := d.conn.Exec(`
			ALTER TABLE projects ADD COLUMN user_feedback_state TEXT NOT NULL DEFAULT '';
		`); err != nil {
			return err
		}
	}

	// Migration: Add learnings_state column to projects table
	if exists, err := d.columnExists("projects", "learnings_state"); err != nil {
		return err
	} else if !exists {
		if _, err := d.conn.Exec(`
			ALTER TABLE projects ADD COLUMN learnings_state TEXT NOT NULL DEFAULT '';
		`); err != nil {
			return err
		}
	}

	// Migration: Add agent_type column to plan_sessions table
	if exists, err := d.columnExists("plan_sessions", "agent_type"); err != nil {
		return err
	} else if !exists {
		if _, err := d.conn.Exec(`
			ALTER TABLE plan_sessions ADD COLUMN agent_type TEXT NOT NULL DEFAULT 'developer';
		`); err != nil {
			return err
		}
	}

	// Migration: Add base_change_id column to plans table for cumulative reviewer diffs
	if exists, err := d.columnExists("plans", "base_change_id"); err != nil {
		return err
	} else if !exists {
		if _, err := d.conn.Exec(`
			ALTER TABLE plans ADD COLUMN base_change_id TEXT NOT NULL DEFAULT '';
		`); err != nil {
			return err
		}
	}

	return nil
}

// columnExists checks if a column exists in the specified table.
func (d *DB) columnExists(table, column string) (bool, error) {
	rows, err := d.conn.Query("PRAGMA table_info(" + table + ")")
	if err != nil {
		return false, err
	}
	defer func() {
		if closeErr := rows.Close(); closeErr != nil {
			log.Warn("failed to close rows", "operation", "columnExists", "error", closeErr)
		}
	}()

	for rows.Next() {
		var cid int
		var name, ctype string
		var notnull, pk int
		var dfltValue interface{}
		if err := rows.Scan(&cid, &name, &ctype, &notnull, &dfltValue, &pk); err != nil {
			return false, err
		}
		if name == column {
			return true, nil
		}
	}
	return false, rows.Err()
}
