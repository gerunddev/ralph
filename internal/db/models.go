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

// PlanStatus represents the status of a plan.
type PlanStatus string

const (
	PlanStatusPending   PlanStatus = "pending"
	PlanStatusRunning   PlanStatus = "running"
	PlanStatusCompleted PlanStatus = "completed"
	PlanStatusFailed    PlanStatus = "failed"
	PlanStatusStopped   PlanStatus = "stopped"
)

// PlanSessionStatus represents the status of a plan session.
type PlanSessionStatus string

const (
	PlanSessionRunning   PlanSessionStatus = "running"
	PlanSessionCompleted PlanSessionStatus = "completed"
	PlanSessionFailed    PlanSessionStatus = "failed"
)

// LoopAgentType represents the type of agent in a loop session.
type LoopAgentType string

const (
	LoopAgentDeveloper LoopAgentType = "developer"
	LoopAgentReviewer  LoopAgentType = "reviewer"
)

// Plan represents a plan to be executed.
type Plan struct {
	ID           string
	OriginPath   string
	Content      string
	Status       PlanStatus
	BaseChangeID string // jj change ID captured at plan start, used for cumulative reviewer diffs
	CreatedAt    time.Time
	UpdatedAt    time.Time
}

// PlanSession represents a Claude session linked to a plan.
type PlanSession struct {
	ID          string
	PlanID      string
	Iteration   int
	InputPrompt string
	FinalOutput string
	Status      PlanSessionStatus
	AgentType   LoopAgentType // "developer" or "reviewer"
	CreatedAt   time.Time
	CompletedAt *time.Time
}

// Event represents a stream event from Claude.
type Event struct {
	ID        int64
	SessionID string
	Sequence  int
	EventType string
	RawJSON   string
	CreatedAt time.Time
}

// Progress represents a progress snapshot.
type Progress struct {
	ID        int64
	PlanID    string
	SessionID string
	Content   string
	CreatedAt time.Time
}

// Learnings represents a learnings snapshot.
type Learnings struct {
	ID        int64
	PlanID    string
	SessionID string
	Content   string
	CreatedAt time.Time
}

// ReviewerFeedback represents feedback from a reviewer rejection.
type ReviewerFeedback struct {
	ID        int64
	PlanID    string
	SessionID string // The reviewer session that generated the feedback
	Content   string
	CreatedAt time.Time
}
