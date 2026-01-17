// Package db provides database connectivity and operations for Ralph.
package db

import "time"

// ProjectStatus represents the status of a project.
type ProjectStatus string

const (
	ProjectPending    ProjectStatus = "pending"
	ProjectInProgress ProjectStatus = "in_progress"
	ProjectCompleted  ProjectStatus = "completed"
	ProjectFailed     ProjectStatus = "failed"
)

// TaskStatus represents the status of a task.
type TaskStatus string

const (
	TaskPending    TaskStatus = "pending"
	TaskInProgress TaskStatus = "in_progress"
	TaskCompleted  TaskStatus = "completed"
	TaskFailed     TaskStatus = "failed"
	TaskEscalated  TaskStatus = "escalated"
)

// SessionStatus represents the status of a session.
type SessionStatus string

const (
	SessionRunning   SessionStatus = "running"
	SessionCompleted SessionStatus = "completed"
	SessionFailed    SessionStatus = "failed"
)

// AgentType represents the type of agent running a session.
type AgentType string

const (
	AgentDeveloper AgentType = "developer"
	AgentReviewer  AgentType = "reviewer"
	AgentPlanner   AgentType = "planner"
)

// FeedbackType represents the type of feedback from a reviewer.
type FeedbackType string

const (
	FeedbackApproved FeedbackType = "approved"
	FeedbackMajor    FeedbackType = "major"
	FeedbackMinor    FeedbackType = "minor"
	FeedbackCritical FeedbackType = "critical"
)

// UserFeedbackState represents the state of user feedback for a project.
type UserFeedbackState string

const (
	FeedbackStateNone     UserFeedbackState = ""         // Initial state, not yet prompted
	FeedbackStatePending  UserFeedbackState = "pending"  // User wants to provide feedback via CLI
	FeedbackStateProvided UserFeedbackState = "provided" // Feedback submitted, task created
	FeedbackStateComplete UserFeedbackState = "complete" // User marked review as complete
)

// LearningsState represents the state of learnings capture for a project.
type LearningsState string

const (
	LearningsStateNone     LearningsState = ""         // Initial state, not yet captured
	LearningsStateComplete LearningsState = "complete" // Learnings have been captured
)

// Project represents a Ralph project with its plan and status.
type Project struct {
	ID                string
	Name              string
	PlanText          string
	Status            ProjectStatus
	UserFeedbackState UserFeedbackState
	LearningsState    LearningsState
	CreatedAt         time.Time
	UpdatedAt         time.Time
}

// Task represents a single task within a project.
type Task struct {
	ID             string
	ProjectID      string
	Sequence       int
	Title          string
	Description    string
	Status         TaskStatus
	JJChangeID     *string // nullable
	IterationCount int
	CreatedAt      time.Time
	UpdatedAt      time.Time
}

// Session represents a Claude agent session for a task.
type Session struct {
	ID          string
	TaskID      string
	AgentType   AgentType
	Iteration   int
	InputPrompt string
	Status      SessionStatus
	CreatedAt   time.Time
	CompletedAt *time.Time // nullable
}

// Message represents a single message in a session's streaming history.
type Message struct {
	ID          int64
	SessionID   string
	Sequence    int
	MessageType string // from Claude stream-json types
	Content     string // Full JSON blob
	CreatedAt   time.Time
}

// Feedback represents review feedback for a session.
type Feedback struct {
	ID           int64
	SessionID    string
	FeedbackType FeedbackType
	Content      *string // nullable
	CreatedAt    time.Time
}
