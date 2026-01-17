// Package db provides database connectivity and operations for Ralph.
package db

import (
	"fmt"
	"path/filepath"

	"github.com/google/uuid"
)

// ProjectDBPath returns the database path for a given project ID.
func ProjectDBPath(projectsDir, projectID string) string {
	return filepath.Join(projectsDir, projectID, "ralph.db")
}

// OpenProjectDB opens or creates the database for a specific project.
func OpenProjectDB(projectsDir, projectID string) (*DB, error) {
	dbPath := ProjectDBPath(projectsDir, projectID)
	database, err := New(dbPath)
	if err != nil {
		return nil, fmt.Errorf("failed to open project database: %w", err)
	}
	return database, nil
}

// GenerateProjectID generates a new unique project ID.
// The ID is filesystem-safe (alphanumeric with hyphens).
func GenerateProjectID() string {
	return uuid.New().String()[:8]
}
