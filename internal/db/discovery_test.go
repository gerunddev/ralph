package db

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestDiscoverProjects_EmptyDir(t *testing.T) {
	tmpDir := t.TempDir()

	projects, err := DiscoverProjects(tmpDir)
	if err != nil {
		t.Fatalf("DiscoverProjects() returned error: %v", err)
	}

	if len(projects) != 0 {
		t.Errorf("DiscoverProjects() returned %d projects, want 0", len(projects))
	}
}

func TestDiscoverProjects_NonexistentDir(t *testing.T) {
	projects, err := DiscoverProjects("/nonexistent/path")
	if err != nil {
		t.Fatalf("DiscoverProjects() returned error: %v", err)
	}

	if projects != nil && len(projects) != 0 {
		t.Errorf("DiscoverProjects() returned non-empty result for nonexistent dir")
	}
}

func TestDiscoverProjects_SingleProject(t *testing.T) {
	tmpDir := t.TempDir()

	// Create a project database
	projectID := "proj-1"
	database, err := OpenProjectDB(tmpDir, projectID)
	if err != nil {
		t.Fatalf("Failed to create project database: %v", err)
	}

	project := &Project{
		ID:       projectID,
		Name:     "Test Project",
		PlanText: "Test plan",
		Status:   ProjectInProgress,
	}
	if err := database.CreateProject(project); err != nil {
		t.Fatalf("Failed to create project: %v", err)
	}
	database.Close()

	// Discover projects
	projects, err := DiscoverProjects(tmpDir)
	if err != nil {
		t.Fatalf("DiscoverProjects() returned error: %v", err)
	}

	if len(projects) != 1 {
		t.Fatalf("DiscoverProjects() returned %d projects, want 1", len(projects))
	}

	if projects[0].ID != projectID {
		t.Errorf("Project ID = %v, want %v", projects[0].ID, projectID)
	}

	if projects[0].Name != "Test Project" {
		t.Errorf("Project Name = %v, want %v", projects[0].Name, "Test Project")
	}

	if projects[0].Status != ProjectInProgress {
		t.Errorf("Project Status = %v, want %v", projects[0].Status, ProjectInProgress)
	}
}

func TestDiscoverProjects_MultipleProjects(t *testing.T) {
	tmpDir := t.TempDir()

	// Create multiple projects with different timestamps
	projectIDs := []string{"proj-a", "proj-b", "proj-c"}

	for i, id := range projectIDs {
		database, err := OpenProjectDB(tmpDir, id)
		if err != nil {
			t.Fatalf("Failed to create project database: %v", err)
		}

		project := &Project{
			ID:       id,
			Name:     "Project " + id,
			PlanText: "Plan " + id,
			Status:   ProjectPending,
		}
		if err := database.CreateProject(project); err != nil {
			t.Fatalf("Failed to create project: %v", err)
		}
		database.Close()

		// Small delay to ensure different timestamps
		if i < len(projectIDs)-1 {
			time.Sleep(10 * time.Millisecond)
		}
	}

	// Discover projects
	projects, err := DiscoverProjects(tmpDir)
	if err != nil {
		t.Fatalf("DiscoverProjects() returned error: %v", err)
	}

	if len(projects) != 3 {
		t.Fatalf("DiscoverProjects() returned %d projects, want 3", len(projects))
	}

	// Should be sorted by UpdatedAt descending (newest first)
	// proj-c was created last, so should be first
	if projects[0].ID != "proj-c" {
		t.Errorf("First project = %v, want proj-c (most recent)", projects[0].ID)
	}
}

func TestDiscoverProjects_SkipsNonProjectDirs(t *testing.T) {
	tmpDir := t.TempDir()

	// Create a valid project
	validID := "valid-proj"
	database, err := OpenProjectDB(tmpDir, validID)
	if err != nil {
		t.Fatalf("Failed to create project database: %v", err)
	}
	project := &Project{
		ID:       validID,
		Name:     "Valid Project",
		PlanText: "Plan",
	}
	database.CreateProject(project)
	database.Close()

	// Create a directory without a database
	emptyDir := filepath.Join(tmpDir, "empty-dir")
	if err := os.MkdirAll(emptyDir, 0755); err != nil {
		t.Fatalf("Failed to create empty directory: %v", err)
	}

	// Create a file (not a directory)
	filePath := filepath.Join(tmpDir, "not-a-dir")
	if err := os.WriteFile(filePath, []byte("test"), 0644); err != nil {
		t.Fatalf("Failed to create file: %v", err)
	}

	// Discover projects
	projects, err := DiscoverProjects(tmpDir)
	if err != nil {
		t.Fatalf("DiscoverProjects() returned error: %v", err)
	}

	// Should only find the valid project
	if len(projects) != 1 {
		t.Fatalf("DiscoverProjects() returned %d projects, want 1", len(projects))
	}

	if projects[0].ID != validID {
		t.Errorf("Project ID = %v, want %v", projects[0].ID, validID)
	}
}

func TestDiscoverProjects_SkipsCorruptDB(t *testing.T) {
	tmpDir := t.TempDir()

	// Create a valid project
	validID := "valid-proj"
	database, err := OpenProjectDB(tmpDir, validID)
	if err != nil {
		t.Fatalf("Failed to create project database: %v", err)
	}
	project := &Project{
		ID:       validID,
		Name:     "Valid Project",
		PlanText: "Plan",
	}
	database.CreateProject(project)
	database.Close()

	// Create a corrupt database
	corruptDir := filepath.Join(tmpDir, "corrupt-proj")
	if err := os.MkdirAll(corruptDir, 0755); err != nil {
		t.Fatalf("Failed to create corrupt directory: %v", err)
	}
	corruptPath := filepath.Join(corruptDir, "ralph.db")
	if err := os.WriteFile(corruptPath, []byte("not a sqlite database"), 0644); err != nil {
		t.Fatalf("Failed to create corrupt database: %v", err)
	}

	// Discover projects - should skip corrupt and return valid
	projects, err := DiscoverProjects(tmpDir)
	if err != nil {
		t.Fatalf("DiscoverProjects() returned error: %v", err)
	}

	// Should only find the valid project
	if len(projects) != 1 {
		t.Fatalf("DiscoverProjects() returned %d projects, want 1", len(projects))
	}

	if projects[0].ID != validID {
		t.Errorf("Project ID = %v, want %v", projects[0].ID, validID)
	}
}

func TestProjectInfo_DBPath(t *testing.T) {
	tmpDir := t.TempDir()

	// Create a project
	projectID := "test-proj"
	database, err := OpenProjectDB(tmpDir, projectID)
	if err != nil {
		t.Fatalf("Failed to create project database: %v", err)
	}
	project := &Project{
		ID:       projectID,
		Name:     "Test Project",
		PlanText: "Plan",
	}
	database.CreateProject(project)
	database.Close()

	// Discover and check DBPath is set
	projects, err := DiscoverProjects(tmpDir)
	if err != nil {
		t.Fatalf("DiscoverProjects() returned error: %v", err)
	}

	if len(projects) != 1 {
		t.Fatalf("Expected 1 project, got %d", len(projects))
	}

	expectedPath := filepath.Join(tmpDir, projectID, "ralph.db")
	if projects[0].DBPath != expectedPath {
		t.Errorf("DBPath = %v, want %v", projects[0].DBPath, expectedPath)
	}
}

func TestSortProjectsByUpdatedAt(t *testing.T) {
	now := time.Now()
	projects := []ProjectInfo{
		{ID: "old", UpdatedAt: now.Add(-2 * time.Hour)},
		{ID: "newest", UpdatedAt: now},
		{ID: "middle", UpdatedAt: now.Add(-1 * time.Hour)},
	}

	sortProjectsByUpdatedAt(projects)

	if projects[0].ID != "newest" {
		t.Errorf("First project = %v, want newest", projects[0].ID)
	}
	if projects[1].ID != "middle" {
		t.Errorf("Second project = %v, want middle", projects[1].ID)
	}
	if projects[2].ID != "old" {
		t.Errorf("Third project = %v, want old", projects[2].ID)
	}
}
