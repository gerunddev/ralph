// Package db provides database connectivity and operations for Ralph.
package db

import (
	"database/sql"
	"os"
	"path/filepath"
	"time"

	"github.com/gerunddev/ralph/internal/log"
)

// ProjectInfo contains basic information about a discovered project.
// This is used for displaying projects in the selection UI without
// keeping database connections open.
type ProjectInfo struct {
	ID        string
	Name      string
	Status    ProjectStatus
	UpdatedAt time.Time
	DBPath    string
}

// DiscoverProjects scans the projects directory for existing databases
// and returns basic project info without keeping connections open.
func DiscoverProjects(projectsDir string) ([]ProjectInfo, error) {
	entries, err := os.ReadDir(projectsDir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil // No projects yet
		}
		return nil, err
	}

	var projects []ProjectInfo
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}

		dbPath := filepath.Join(projectsDir, entry.Name(), "ralph.db")
		if _, err := os.Stat(dbPath); err != nil {
			// Database doesn't exist in this directory
			continue
		}

		// Open briefly to get project info
		info, err := getProjectInfo(dbPath, entry.Name())
		if err != nil {
			// Log warning but continue - corrupt databases are skipped
			continue
		}
		projects = append(projects, info)
	}

	// Sort by UpdatedAt descending (most recent first)
	sortProjectsByUpdatedAt(projects)

	return projects, nil
}

// getProjectInfo opens a database briefly to retrieve basic project info.
func getProjectInfo(dbPath, dirName string) (ProjectInfo, error) {
	// Open database directly without auto-migrate (read-only query)
	// Use busy_timeout to handle concurrent access gracefully
	conn, err := sql.Open("sqlite", dbPath+"?_pragma=foreign_keys(1)&_pragma=busy_timeout(5000)")
	if err != nil {
		return ProjectInfo{}, err
	}
	defer func() {
		if closeErr := conn.Close(); closeErr != nil {
			log.Warn("failed to close database connection", "path", dbPath, "error", closeErr)
		}
	}()

	// Query for project info - there should be exactly one project per database
	var info ProjectInfo
	info.DBPath = dbPath

	err = conn.QueryRow(`
		SELECT id, name, status, updated_at
		FROM projects
		ORDER BY updated_at DESC
		LIMIT 1
	`).Scan(&info.ID, &info.Name, &info.Status, &info.UpdatedAt)
	if err != nil {
		// If table doesn't exist or no projects, use directory name as fallback
		if err == sql.ErrNoRows {
			info.ID = dirName
			info.Name = dirName
			info.Status = ProjectPending
			info.UpdatedAt = time.Time{}
			return info, nil
		}
		return ProjectInfo{}, err
	}

	return info, nil
}

// sortProjectsByUpdatedAt sorts projects by UpdatedAt in descending order.
func sortProjectsByUpdatedAt(projects []ProjectInfo) {
	// Simple bubble sort is fine for small lists
	for i := 0; i < len(projects)-1; i++ {
		for j := 0; j < len(projects)-i-1; j++ {
			if projects[j].UpdatedAt.Before(projects[j+1].UpdatedAt) {
				projects[j], projects[j+1] = projects[j+1], projects[j]
			}
		}
	}
}
