// Package db provides database connectivity and operations for Ralph.
package db

import (
	"database/sql"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"time"

	_ "github.com/mattn/go-sqlite3"

	"github.com/gerund/ralph/internal/log"
)

// ErrNotFound is returned when a requested record is not found.
var ErrNotFound = errors.New("record not found")

// DB holds the database connection and provides methods for data access.
type DB struct {
	conn *sql.DB
}

// New creates a new database connection.
// If the path is ":memory:", an in-memory database is created.
// Otherwise, the parent directory is created if it doesn't exist.
func New(path string) (*DB, error) {
	// Create parent directory if needed (not for in-memory DB)
	if path != ":memory:" {
		dir := filepath.Dir(path)
		if err := os.MkdirAll(dir, 0755); err != nil {
			return nil, fmt.Errorf("failed to create database directory: %w", err)
		}
	}

	// Open database with foreign keys enabled
	conn, err := sql.Open("sqlite3", path+"?_foreign_keys=on")
	if err != nil {
		return nil, fmt.Errorf("failed to open database: %w", err)
	}

	// Verify connection
	if err := conn.Ping(); err != nil {
		if closeErr := conn.Close(); closeErr != nil {
			log.Warn("failed to close connection after ping failure", "error", closeErr)
		}
		return nil, fmt.Errorf("failed to connect to database: %w", err)
	}

	db := &DB{conn: conn}

	// Run migrations automatically
	if err := db.Migrate(); err != nil {
		if closeErr := conn.Close(); closeErr != nil {
			log.Warn("failed to close connection after migration failure", "error", closeErr)
		}
		return nil, fmt.Errorf("failed to run migrations: %w", err)
	}

	return db, nil
}

// Close closes the database connection.
func (d *DB) Close() error {
	if d.conn != nil {
		return d.conn.Close()
	}
	return nil
}

// =============================================================================
// Project Methods
// =============================================================================

// CreateProject inserts a new project into the database.
func (d *DB) CreateProject(project *Project) error {
	now := time.Now()
	project.CreatedAt = now
	project.UpdatedAt = now
	if project.Status == "" {
		project.Status = ProjectPending
	}

	_, err := d.conn.Exec(`
		INSERT INTO projects (id, name, plan_text, status, user_feedback_state, learnings_state, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?)`,
		project.ID, project.Name, project.PlanText, project.Status,
		project.UserFeedbackState, project.LearningsState, project.CreatedAt, project.UpdatedAt,
	)
	return err
}

// GetProject retrieves a project by ID.
func (d *DB) GetProject(id string) (*Project, error) {
	project := &Project{}
	err := d.conn.QueryRow(`
		SELECT id, name, plan_text, status, user_feedback_state, learnings_state, created_at, updated_at
		FROM projects WHERE id = ?`, id,
	).Scan(
		&project.ID, &project.Name, &project.PlanText, &project.Status,
		&project.UserFeedbackState, &project.LearningsState, &project.CreatedAt, &project.UpdatedAt,
	)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	return project, nil
}

// ListProjects returns all projects ordered by updated_at descending.
func (d *DB) ListProjects() ([]*Project, error) {
	rows, err := d.conn.Query(`
		SELECT id, name, plan_text, status, user_feedback_state, learnings_state, created_at, updated_at
		FROM projects ORDER BY updated_at DESC`)
	if err != nil {
		return nil, err
	}
	defer func() {
		if closeErr := rows.Close(); closeErr != nil {
			log.Warn("failed to close rows", "operation", "ListProjects", "error", closeErr)
		}
	}()

	var projects []*Project
	for rows.Next() {
		p := &Project{}
		if err := rows.Scan(
			&p.ID, &p.Name, &p.PlanText, &p.Status,
			&p.UserFeedbackState, &p.LearningsState, &p.CreatedAt, &p.UpdatedAt,
		); err != nil {
			return nil, err
		}
		projects = append(projects, p)
	}
	return projects, rows.Err()
}

// UpdateProjectStatus updates a project's status and updated_at timestamp.
func (d *DB) UpdateProjectStatus(id string, status ProjectStatus) error {
	result, err := d.conn.Exec(`
		UPDATE projects SET status = ?, updated_at = ? WHERE id = ?`,
		status, time.Now(), id,
	)
	if err != nil {
		return err
	}
	rows, err := result.RowsAffected()
	if err != nil {
		return err
	}
	if rows == 0 {
		return ErrNotFound
	}
	return nil
}

// UpdateProjectFeedbackState updates a project's user feedback state.
func (d *DB) UpdateProjectFeedbackState(id string, state UserFeedbackState) error {
	result, err := d.conn.Exec(`
		UPDATE projects SET user_feedback_state = ?, updated_at = ? WHERE id = ?`,
		state, time.Now(), id,
	)
	if err != nil {
		return err
	}
	rows, err := result.RowsAffected()
	if err != nil {
		return err
	}
	if rows == 0 {
		return ErrNotFound
	}
	return nil
}

// UpdateProjectLearningsState updates a project's learnings state.
func (d *DB) UpdateProjectLearningsState(id string, state LearningsState) error {
	result, err := d.conn.Exec(`
		UPDATE projects SET learnings_state = ?, updated_at = ? WHERE id = ?`,
		state, time.Now(), id,
	)
	if err != nil {
		return err
	}
	rows, err := result.RowsAffected()
	if err != nil {
		return err
	}
	if rows == 0 {
		return ErrNotFound
	}
	return nil
}

// GetMaxTaskSequence returns the highest sequence number for a project's tasks.
// Returns 0 if there are no tasks.
func (d *DB) GetMaxTaskSequence(projectID string) (int, error) {
	var maxSeq sql.NullInt64
	err := d.conn.QueryRow(`
		SELECT MAX(sequence) FROM tasks WHERE project_id = ?`, projectID,
	).Scan(&maxSeq)
	if err != nil {
		return 0, err
	}
	if !maxSeq.Valid {
		return 0, nil
	}
	return int(maxSeq.Int64), nil
}

// HasPendingTasks returns true if there are any pending tasks for the project.
func (d *DB) HasPendingTasks(projectID string) (bool, error) {
	var count int
	err := d.conn.QueryRow(`
		SELECT COUNT(*) FROM tasks WHERE project_id = ? AND status = ?`,
		projectID, TaskPending,
	).Scan(&count)
	if err != nil {
		return false, err
	}
	return count > 0, nil
}

// =============================================================================
// Task Methods
// =============================================================================

// CreateTask inserts a new task into the database.
func (d *DB) CreateTask(task *Task) error {
	now := time.Now()
	task.CreatedAt = now
	task.UpdatedAt = now
	if task.Status == "" {
		task.Status = TaskPending
	}

	_, err := d.conn.Exec(`
		INSERT INTO tasks (id, project_id, sequence, title, description, status, jj_change_id, iteration_count, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		task.ID, task.ProjectID, task.Sequence, task.Title, task.Description,
		task.Status, task.JJChangeID, task.IterationCount,
		task.CreatedAt, task.UpdatedAt,
	)
	return err
}

// CreateTasks inserts multiple tasks in a single transaction.
func (d *DB) CreateTasks(tasks []*Task) error {
	tx, err := d.conn.Begin()
	if err != nil {
		return err
	}
	defer func() {
		if rbErr := tx.Rollback(); rbErr != nil && rbErr != sql.ErrTxDone {
			log.Warn("failed to rollback transaction", "operation", "CreateTasks", "error", rbErr)
		}
	}()

	stmt, err := tx.Prepare(`
		INSERT INTO tasks (id, project_id, sequence, title, description, status, jj_change_id, iteration_count, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`)
	if err != nil {
		return err
	}
	defer func() {
		if closeErr := stmt.Close(); closeErr != nil {
			log.Warn("failed to close statement", "operation", "CreateTasks", "error", closeErr)
		}
	}()

	now := time.Now()
	for _, task := range tasks {
		task.CreatedAt = now
		task.UpdatedAt = now
		if task.Status == "" {
			task.Status = TaskPending
		}

		_, err := stmt.Exec(
			task.ID, task.ProjectID, task.Sequence, task.Title, task.Description,
			task.Status, task.JJChangeID, task.IterationCount,
			task.CreatedAt, task.UpdatedAt,
		)
		if err != nil {
			return err
		}
	}

	return tx.Commit()
}

// GetTask retrieves a task by ID.
func (d *DB) GetTask(id string) (*Task, error) {
	task := &Task{}
	err := d.conn.QueryRow(`
		SELECT id, project_id, sequence, title, description, status, jj_change_id, iteration_count, created_at, updated_at
		FROM tasks WHERE id = ?`, id,
	).Scan(
		&task.ID, &task.ProjectID, &task.Sequence, &task.Title, &task.Description,
		&task.Status, &task.JJChangeID, &task.IterationCount,
		&task.CreatedAt, &task.UpdatedAt,
	)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	return task, nil
}

// GetTasksByProject returns all tasks for a project ordered by sequence.
func (d *DB) GetTasksByProject(projectID string) ([]*Task, error) {
	rows, err := d.conn.Query(`
		SELECT id, project_id, sequence, title, description, status, jj_change_id, iteration_count, created_at, updated_at
		FROM tasks WHERE project_id = ? ORDER BY sequence`, projectID)
	if err != nil {
		return nil, err
	}
	defer func() {
		if closeErr := rows.Close(); closeErr != nil {
			log.Warn("failed to close rows", "operation", "GetTasksByProject", "error", closeErr)
		}
	}()

	var tasks []*Task
	for rows.Next() {
		t := &Task{}
		if err := rows.Scan(
			&t.ID, &t.ProjectID, &t.Sequence, &t.Title, &t.Description,
			&t.Status, &t.JJChangeID, &t.IterationCount,
			&t.CreatedAt, &t.UpdatedAt,
		); err != nil {
			return nil, err
		}
		tasks = append(tasks, t)
	}
	return tasks, rows.Err()
}

// GetNextPendingTask returns the first pending task for a project (by sequence).
func (d *DB) GetNextPendingTask(projectID string) (*Task, error) {
	task := &Task{}
	err := d.conn.QueryRow(`
		SELECT id, project_id, sequence, title, description, status, jj_change_id, iteration_count, created_at, updated_at
		FROM tasks WHERE project_id = ? AND status = ? ORDER BY sequence LIMIT 1`,
		projectID, TaskPending,
	).Scan(
		&task.ID, &task.ProjectID, &task.Sequence, &task.Title, &task.Description,
		&task.Status, &task.JJChangeID, &task.IterationCount,
		&task.CreatedAt, &task.UpdatedAt,
	)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	return task, nil
}

// UpdateTaskStatus updates a task's status and updated_at timestamp.
func (d *DB) UpdateTaskStatus(id string, status TaskStatus) error {
	result, err := d.conn.Exec(`
		UPDATE tasks SET status = ?, updated_at = ? WHERE id = ?`,
		status, time.Now(), id,
	)
	if err != nil {
		return err
	}
	rows, err := result.RowsAffected()
	if err != nil {
		return err
	}
	if rows == 0 {
		return ErrNotFound
	}
	return nil
}

// UpdateTaskJJChangeID updates a task's jj_change_id and updated_at timestamp.
func (d *DB) UpdateTaskJJChangeID(id string, changeID string) error {
	result, err := d.conn.Exec(`
		UPDATE tasks SET jj_change_id = ?, updated_at = ? WHERE id = ?`,
		changeID, time.Now(), id,
	)
	if err != nil {
		return err
	}
	rows, err := result.RowsAffected()
	if err != nil {
		return err
	}
	if rows == 0 {
		return ErrNotFound
	}
	return nil
}

// IncrementTaskIteration increments a task's iteration_count and updated_at timestamp.
func (d *DB) IncrementTaskIteration(id string) error {
	result, err := d.conn.Exec(`
		UPDATE tasks SET iteration_count = iteration_count + 1, updated_at = ? WHERE id = ?`,
		time.Now(), id,
	)
	if err != nil {
		return err
	}
	rows, err := result.RowsAffected()
	if err != nil {
		return err
	}
	if rows == 0 {
		return ErrNotFound
	}
	return nil
}

// =============================================================================
// Session Methods
// =============================================================================

// CreateSession inserts a new session into the database.
func (d *DB) CreateSession(session *Session) error {
	session.CreatedAt = time.Now()
	if session.Status == "" {
		session.Status = SessionRunning
	}

	_, err := d.conn.Exec(`
		INSERT INTO sessions (id, task_id, agent_type, iteration, input_prompt, status, created_at, completed_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?)`,
		session.ID, session.TaskID, session.AgentType, session.Iteration,
		session.InputPrompt, session.Status, session.CreatedAt, session.CompletedAt,
	)
	return err
}

// GetSession retrieves a session by ID.
func (d *DB) GetSession(id string) (*Session, error) {
	session := &Session{}
	err := d.conn.QueryRow(`
		SELECT id, task_id, agent_type, iteration, input_prompt, status, created_at, completed_at
		FROM sessions WHERE id = ?`, id,
	).Scan(
		&session.ID, &session.TaskID, &session.AgentType, &session.Iteration,
		&session.InputPrompt, &session.Status, &session.CreatedAt, &session.CompletedAt,
	)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	return session, nil
}

// GetSessionsByTask returns all sessions for a task ordered by iteration.
func (d *DB) GetSessionsByTask(taskID string) ([]*Session, error) {
	rows, err := d.conn.Query(`
		SELECT id, task_id, agent_type, iteration, input_prompt, status, created_at, completed_at
		FROM sessions WHERE task_id = ? ORDER BY iteration`, taskID)
	if err != nil {
		return nil, err
	}
	defer func() {
		if closeErr := rows.Close(); closeErr != nil {
			log.Warn("failed to close rows", "operation", "GetSessionsByTask", "error", closeErr)
		}
	}()

	var sessions []*Session
	for rows.Next() {
		s := &Session{}
		if err := rows.Scan(
			&s.ID, &s.TaskID, &s.AgentType, &s.Iteration,
			&s.InputPrompt, &s.Status, &s.CreatedAt, &s.CompletedAt,
		); err != nil {
			return nil, err
		}
		sessions = append(sessions, s)
	}
	return sessions, rows.Err()
}

// CompleteSession marks a session as completed with the given status.
func (d *DB) CompleteSession(id string, status SessionStatus) error {
	now := time.Now()
	result, err := d.conn.Exec(`
		UPDATE sessions SET status = ?, completed_at = ? WHERE id = ?`,
		status, now, id,
	)
	if err != nil {
		return err
	}
	rows, err := result.RowsAffected()
	if err != nil {
		return err
	}
	if rows == 0 {
		return ErrNotFound
	}
	return nil
}

// GetLatestSessionForTask returns the most recent session for a task.
func (d *DB) GetLatestSessionForTask(taskID string) (*Session, error) {
	session := &Session{}
	err := d.conn.QueryRow(`
		SELECT id, task_id, agent_type, iteration, input_prompt, status, created_at, completed_at
		FROM sessions WHERE task_id = ? ORDER BY created_at DESC LIMIT 1`, taskID,
	).Scan(
		&session.ID, &session.TaskID, &session.AgentType, &session.Iteration,
		&session.InputPrompt, &session.Status, &session.CreatedAt, &session.CompletedAt,
	)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	return session, nil
}

// =============================================================================
// Message Methods
// =============================================================================

// CreateMessage inserts a new message into the database.
func (d *DB) CreateMessage(message *Message) error {
	message.CreatedAt = time.Now()

	result, err := d.conn.Exec(`
		INSERT INTO messages (session_id, sequence, message_type, content, created_at)
		VALUES (?, ?, ?, ?, ?)`,
		message.SessionID, message.Sequence, message.MessageType,
		message.Content, message.CreatedAt,
	)
	if err != nil {
		return err
	}

	id, err := result.LastInsertId()
	if err != nil {
		return err
	}
	message.ID = id
	return nil
}

// GetMessagesBySession returns all messages for a session ordered by sequence.
func (d *DB) GetMessagesBySession(sessionID string) ([]*Message, error) {
	rows, err := d.conn.Query(`
		SELECT id, session_id, sequence, message_type, content, created_at
		FROM messages WHERE session_id = ? ORDER BY sequence`, sessionID)
	if err != nil {
		return nil, err
	}
	defer func() {
		if closeErr := rows.Close(); closeErr != nil {
			log.Warn("failed to close rows", "operation", "GetMessagesBySession", "error", closeErr)
		}
	}()

	var messages []*Message
	for rows.Next() {
		m := &Message{}
		if err := rows.Scan(
			&m.ID, &m.SessionID, &m.Sequence, &m.MessageType,
			&m.Content, &m.CreatedAt,
		); err != nil {
			return nil, err
		}
		messages = append(messages, m)
	}
	return messages, rows.Err()
}

// =============================================================================
// Feedback Methods
// =============================================================================

// CreateFeedback inserts a new feedback into the database.
func (d *DB) CreateFeedback(feedback *Feedback) error {
	feedback.CreatedAt = time.Now()

	result, err := d.conn.Exec(`
		INSERT INTO feedback (session_id, feedback_type, content, created_at)
		VALUES (?, ?, ?, ?)`,
		feedback.SessionID, feedback.FeedbackType, feedback.Content, feedback.CreatedAt,
	)
	if err != nil {
		return err
	}

	id, err := result.LastInsertId()
	if err != nil {
		return err
	}
	feedback.ID = id
	return nil
}

// GetFeedbackBySession returns all feedback for a session ordered by created_at.
func (d *DB) GetFeedbackBySession(sessionID string) ([]*Feedback, error) {
	rows, err := d.conn.Query(`
		SELECT id, session_id, feedback_type, content, created_at
		FROM feedback WHERE session_id = ? ORDER BY created_at`, sessionID)
	if err != nil {
		return nil, err
	}
	defer func() {
		if closeErr := rows.Close(); closeErr != nil {
			log.Warn("failed to close rows", "operation", "GetFeedbackBySession", "error", closeErr)
		}
	}()

	var feedbacks []*Feedback
	for rows.Next() {
		f := &Feedback{}
		if err := rows.Scan(
			&f.ID, &f.SessionID, &f.FeedbackType, &f.Content, &f.CreatedAt,
		); err != nil {
			return nil, err
		}
		feedbacks = append(feedbacks, f)
	}
	return feedbacks, rows.Err()
}

// GetLatestFeedbackForTask returns the most recent feedback for any session of a task.
func (d *DB) GetLatestFeedbackForTask(taskID string) (*Feedback, error) {
	feedback := &Feedback{}
	err := d.conn.QueryRow(`
		SELECT f.id, f.session_id, f.feedback_type, f.content, f.created_at
		FROM feedback f
		JOIN sessions s ON f.session_id = s.id
		WHERE s.task_id = ?
		ORDER BY f.created_at DESC LIMIT 1`, taskID,
	).Scan(
		&feedback.ID, &feedback.SessionID, &feedback.FeedbackType,
		&feedback.Content, &feedback.CreatedAt,
	)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	return feedback, nil
}

// =============================================================================
// Task Export/Import Methods
// =============================================================================

// GetTaskBySequence retrieves a task by its sequence number within a project.
func (d *DB) GetTaskBySequence(projectID string, sequence int) (*Task, error) {
	task := &Task{}
	err := d.conn.QueryRow(`
		SELECT id, project_id, sequence, title, description, status, jj_change_id, iteration_count, created_at, updated_at
		FROM tasks
		WHERE project_id = ? AND sequence = ?`,
		projectID, sequence,
	).Scan(
		&task.ID, &task.ProjectID, &task.Sequence, &task.Title, &task.Description,
		&task.Status, &task.JJChangeID, &task.IterationCount,
		&task.CreatedAt, &task.UpdatedAt,
	)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	return task, nil
}

// UpdateTaskDescription updates only the description field of a task.
func (d *DB) UpdateTaskDescription(taskID string, description string) error {
	result, err := d.conn.Exec(`
		UPDATE tasks SET description = ?, updated_at = ? WHERE id = ?`,
		description, time.Now(), taskID,
	)
	if err != nil {
		return err
	}
	rows, err := result.RowsAffected()
	if err != nil {
		return err
	}
	if rows == 0 {
		return ErrNotFound
	}
	return nil
}

// =============================================================================
// V2 Plan Methods
// =============================================================================

// CreatePlan inserts a new plan into the database.
func (d *DB) CreatePlan(plan *Plan) error {
	now := time.Now()
	plan.CreatedAt = now
	plan.UpdatedAt = now
	if plan.Status == "" {
		plan.Status = PlanStatusPending
	}

	_, err := d.conn.Exec(`
		INSERT INTO plans (id, origin_path, content, status, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?, ?)`,
		plan.ID, plan.OriginPath, plan.Content, plan.Status, plan.CreatedAt, plan.UpdatedAt,
	)
	return err
}

// GetPlan retrieves a plan by ID.
func (d *DB) GetPlan(id string) (*Plan, error) {
	plan := &Plan{}
	err := d.conn.QueryRow(`
		SELECT id, origin_path, content, status, created_at, updated_at
		FROM plans WHERE id = ?`, id,
	).Scan(
		&plan.ID, &plan.OriginPath, &plan.Content, &plan.Status,
		&plan.CreatedAt, &plan.UpdatedAt,
	)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	return plan, nil
}

// UpdatePlanStatus updates a plan's status and updated_at timestamp.
func (d *DB) UpdatePlanStatus(id string, status PlanStatus) error {
	result, err := d.conn.Exec(`
		UPDATE plans SET status = ?, updated_at = ? WHERE id = ?`,
		status, time.Now(), id,
	)
	if err != nil {
		return err
	}
	rows, err := result.RowsAffected()
	if err != nil {
		return err
	}
	if rows == 0 {
		return ErrNotFound
	}
	return nil
}

// =============================================================================
// V2 Plan Session Methods
// =============================================================================

// CreatePlanSession inserts a new plan session into the database.
func (d *DB) CreatePlanSession(session *PlanSession) error {
	session.CreatedAt = time.Now()
	if session.Status == "" {
		session.Status = PlanSessionRunning
	}
	if session.AgentType == "" {
		session.AgentType = V15AgentDeveloper
	}

	_, err := d.conn.Exec(`
		INSERT INTO plan_sessions (id, plan_id, iteration, input_prompt, final_output, status, agent_type, created_at, completed_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		session.ID, session.PlanID, session.Iteration, session.InputPrompt,
		session.FinalOutput, session.Status, session.AgentType, session.CreatedAt, session.CompletedAt,
	)
	return err
}

// GetPlanSession retrieves a plan session by ID.
func (d *DB) GetPlanSession(id string) (*PlanSession, error) {
	session := &PlanSession{}
	err := d.conn.QueryRow(`
		SELECT id, plan_id, iteration, input_prompt, final_output, status, agent_type, created_at, completed_at
		FROM plan_sessions WHERE id = ?`, id,
	).Scan(
		&session.ID, &session.PlanID, &session.Iteration, &session.InputPrompt,
		&session.FinalOutput, &session.Status, &session.AgentType, &session.CreatedAt, &session.CompletedAt,
	)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	return session, nil
}

// CompletePlanSession marks a plan session as completed with the given status and output.
func (d *DB) CompletePlanSession(id string, status PlanSessionStatus, finalOutput string) error {
	now := time.Now()
	result, err := d.conn.Exec(`
		UPDATE plan_sessions SET status = ?, final_output = ?, completed_at = ? WHERE id = ?`,
		status, finalOutput, now, id,
	)
	if err != nil {
		return err
	}
	rows, err := result.RowsAffected()
	if err != nil {
		return err
	}
	if rows == 0 {
		return ErrNotFound
	}
	return nil
}

// GetPlanSessionsByPlan returns all sessions for a plan ordered by iteration.
func (d *DB) GetPlanSessionsByPlan(planID string) ([]*PlanSession, error) {
	rows, err := d.conn.Query(`
		SELECT id, plan_id, iteration, input_prompt, final_output, status, agent_type, created_at, completed_at
		FROM plan_sessions WHERE plan_id = ? ORDER BY iteration`, planID)
	if err != nil {
		return nil, err
	}
	defer func() {
		if closeErr := rows.Close(); closeErr != nil {
			log.Warn("failed to close rows", "operation", "GetPlanSessionsByPlan", "error", closeErr)
		}
	}()

	var sessions []*PlanSession
	for rows.Next() {
		s := &PlanSession{}
		if err := rows.Scan(
			&s.ID, &s.PlanID, &s.Iteration, &s.InputPrompt,
			&s.FinalOutput, &s.Status, &s.AgentType, &s.CreatedAt, &s.CompletedAt,
		); err != nil {
			return nil, err
		}
		sessions = append(sessions, s)
	}
	return sessions, rows.Err()
}

// GetLatestPlanSession returns the most recent session for a plan.
func (d *DB) GetLatestPlanSession(planID string) (*PlanSession, error) {
	session := &PlanSession{}
	err := d.conn.QueryRow(`
		SELECT id, plan_id, iteration, input_prompt, final_output, status, agent_type, created_at, completed_at
		FROM plan_sessions WHERE plan_id = ? ORDER BY iteration DESC LIMIT 1`, planID,
	).Scan(
		&session.ID, &session.PlanID, &session.Iteration, &session.InputPrompt,
		&session.FinalOutput, &session.Status, &session.AgentType, &session.CreatedAt, &session.CompletedAt,
	)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil // Return nil, not error, when no records exist
	}
	if err != nil {
		return nil, err
	}
	return session, nil
}

// =============================================================================
// V2 Event Methods
// =============================================================================

// CreateEvent inserts a new event into the database.
func (d *DB) CreateEvent(event *Event) error {
	event.CreatedAt = time.Now()

	result, err := d.conn.Exec(`
		INSERT INTO events (session_id, sequence, event_type, raw_json, created_at)
		VALUES (?, ?, ?, ?, ?)`,
		event.SessionID, event.Sequence, event.EventType, event.RawJSON, event.CreatedAt,
	)
	if err != nil {
		return err
	}

	id, err := result.LastInsertId()
	if err != nil {
		return err
	}
	event.ID = id
	return nil
}

// GetEventsBySession returns all events for a session ordered by sequence.
func (d *DB) GetEventsBySession(sessionID string) ([]*Event, error) {
	rows, err := d.conn.Query(`
		SELECT id, session_id, sequence, event_type, raw_json, created_at
		FROM events WHERE session_id = ? ORDER BY sequence`, sessionID)
	if err != nil {
		return nil, err
	}
	defer func() {
		if closeErr := rows.Close(); closeErr != nil {
			log.Warn("failed to close rows", "operation", "GetEventsBySession", "error", closeErr)
		}
	}()

	var events []*Event
	for rows.Next() {
		e := &Event{}
		if err := rows.Scan(
			&e.ID, &e.SessionID, &e.Sequence, &e.EventType,
			&e.RawJSON, &e.CreatedAt,
		); err != nil {
			return nil, err
		}
		events = append(events, e)
	}
	return events, rows.Err()
}

// =============================================================================
// V2 Progress Methods
// =============================================================================

// CreateProgress inserts a new progress record into the database.
func (d *DB) CreateProgress(progress *Progress) error {
	progress.CreatedAt = time.Now()

	result, err := d.conn.Exec(`
		INSERT INTO progress (plan_id, session_id, content, created_at)
		VALUES (?, ?, ?, ?)`,
		progress.PlanID, progress.SessionID, progress.Content, progress.CreatedAt,
	)
	if err != nil {
		return err
	}

	id, err := result.LastInsertId()
	if err != nil {
		return err
	}
	progress.ID = id
	return nil
}

// GetLatestProgress returns the most recent progress for a plan.
func (d *DB) GetLatestProgress(planID string) (*Progress, error) {
	progress := &Progress{}
	err := d.conn.QueryRow(`
		SELECT id, plan_id, session_id, content, created_at
		FROM progress WHERE plan_id = ? ORDER BY created_at DESC LIMIT 1`, planID,
	).Scan(
		&progress.ID, &progress.PlanID, &progress.SessionID,
		&progress.Content, &progress.CreatedAt,
	)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil // Return nil, not error, when no records exist
	}
	if err != nil {
		return nil, err
	}
	return progress, nil
}

// GetProgressHistory returns all progress records for a plan ordered by created_at.
func (d *DB) GetProgressHistory(planID string) ([]*Progress, error) {
	rows, err := d.conn.Query(`
		SELECT id, plan_id, session_id, content, created_at
		FROM progress WHERE plan_id = ? ORDER BY created_at`, planID)
	if err != nil {
		return nil, err
	}
	defer func() {
		if closeErr := rows.Close(); closeErr != nil {
			log.Warn("failed to close rows", "operation", "GetProgressHistory", "error", closeErr)
		}
	}()

	var progressList []*Progress
	for rows.Next() {
		p := &Progress{}
		if err := rows.Scan(
			&p.ID, &p.PlanID, &p.SessionID, &p.Content, &p.CreatedAt,
		); err != nil {
			return nil, err
		}
		progressList = append(progressList, p)
	}
	return progressList, rows.Err()
}

// =============================================================================
// V2 Learnings Methods
// =============================================================================

// CreateLearnings inserts a new learnings record into the database.
func (d *DB) CreateLearnings(learnings *Learnings) error {
	learnings.CreatedAt = time.Now()

	result, err := d.conn.Exec(`
		INSERT INTO learnings (plan_id, session_id, content, created_at)
		VALUES (?, ?, ?, ?)`,
		learnings.PlanID, learnings.SessionID, learnings.Content, learnings.CreatedAt,
	)
	if err != nil {
		return err
	}

	id, err := result.LastInsertId()
	if err != nil {
		return err
	}
	learnings.ID = id
	return nil
}

// GetLatestLearnings returns the most recent learnings for a plan.
func (d *DB) GetLatestLearnings(planID string) (*Learnings, error) {
	learnings := &Learnings{}
	err := d.conn.QueryRow(`
		SELECT id, plan_id, session_id, content, created_at
		FROM learnings WHERE plan_id = ? ORDER BY created_at DESC LIMIT 1`, planID,
	).Scan(
		&learnings.ID, &learnings.PlanID, &learnings.SessionID,
		&learnings.Content, &learnings.CreatedAt,
	)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil // Return nil, not error, when no records exist
	}
	if err != nil {
		return nil, err
	}
	return learnings, nil
}

// GetLearningsHistory returns all learnings records for a plan ordered by created_at.
func (d *DB) GetLearningsHistory(planID string) ([]*Learnings, error) {
	rows, err := d.conn.Query(`
		SELECT id, plan_id, session_id, content, created_at
		FROM learnings WHERE plan_id = ? ORDER BY created_at`, planID)
	if err != nil {
		return nil, err
	}
	defer func() {
		if closeErr := rows.Close(); closeErr != nil {
			log.Warn("failed to close rows", "operation", "GetLearningsHistory", "error", closeErr)
		}
	}()

	var learningsList []*Learnings
	for rows.Next() {
		l := &Learnings{}
		if err := rows.Scan(
			&l.ID, &l.PlanID, &l.SessionID, &l.Content, &l.CreatedAt,
		); err != nil {
			return nil, err
		}
		learningsList = append(learningsList, l)
	}
	return learningsList, rows.Err()
}

// =============================================================================
// V1.5 Reviewer Feedback Methods
// =============================================================================

// CreateReviewerFeedback inserts a new reviewer feedback record into the database.
func (d *DB) CreateReviewerFeedback(feedback *ReviewerFeedback) error {
	feedback.CreatedAt = time.Now()

	result, err := d.conn.Exec(`
		INSERT INTO reviewer_feedback (plan_id, session_id, content, created_at)
		VALUES (?, ?, ?, ?)`,
		feedback.PlanID, feedback.SessionID, feedback.Content, feedback.CreatedAt,
	)
	if err != nil {
		return err
	}

	id, err := result.LastInsertId()
	if err != nil {
		return err
	}
	feedback.ID = id
	return nil
}

// GetLatestReviewerFeedback returns the most recent reviewer feedback for a plan.
func (d *DB) GetLatestReviewerFeedback(planID string) (*ReviewerFeedback, error) {
	feedback := &ReviewerFeedback{}
	err := d.conn.QueryRow(`
		SELECT id, plan_id, session_id, content, created_at
		FROM reviewer_feedback WHERE plan_id = ? ORDER BY created_at DESC LIMIT 1`, planID,
	).Scan(
		&feedback.ID, &feedback.PlanID, &feedback.SessionID,
		&feedback.Content, &feedback.CreatedAt,
	)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil // Return nil, not error, when no records exist
	}
	if err != nil {
		return nil, err
	}
	return feedback, nil
}

// ClearReviewerFeedback removes all reviewer feedback for a plan (used after developer addresses it).
func (d *DB) ClearReviewerFeedback(planID string) error {
	_, err := d.conn.Exec(`DELETE FROM reviewer_feedback WHERE plan_id = ?`, planID)
	return err
}
