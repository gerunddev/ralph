package db

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestProjectDBPath(t *testing.T) {
	tests := []struct {
		name       string
		projectsDir string
		projectID  string
		want       string
	}{
		{
			name:       "basic path",
			projectsDir: "/home/user/.local/share/ralph/projects",
			projectID:  "abc123",
			want:       "/home/user/.local/share/ralph/projects/abc123/ralph.db",
		},
		{
			name:       "short id",
			projectsDir: "/data/projects",
			projectID:  "a1b2c3d4",
			want:       "/data/projects/a1b2c3d4/ralph.db",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := ProjectDBPath(tt.projectsDir, tt.projectID)
			if got != tt.want {
				t.Errorf("ProjectDBPath() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestOpenProjectDB(t *testing.T) {
	tmpDir := t.TempDir()

	projectID := "test-proj"
	database, err := OpenProjectDB(tmpDir, projectID)
	if err != nil {
		t.Fatalf("OpenProjectDB() returned error: %v", err)
	}
	defer database.Close()

	// Verify database file was created
	expectedPath := filepath.Join(tmpDir, projectID, "ralph.db")
	if _, err := os.Stat(expectedPath); os.IsNotExist(err) {
		t.Errorf("Database file was not created at %s", expectedPath)
	}

	// Verify we can create a project in the database
	project := &Project{
		ID:       "proj-1",
		Name:     "Test Project",
		PlanText: "Test plan",
	}
	if err := database.CreateProject(project); err != nil {
		t.Errorf("Failed to create project in database: %v", err)
	}
}

func TestOpenProjectDB_MultipleTimes(t *testing.T) {
	tmpDir := t.TempDir()
	projectID := "multi-open"

	// Open and create a project
	db1, err := OpenProjectDB(tmpDir, projectID)
	if err != nil {
		t.Fatalf("First OpenProjectDB() returned error: %v", err)
	}

	project := &Project{
		ID:       "proj-1",
		Name:     "Test Project",
		PlanText: "Test plan",
	}
	if err := db1.CreateProject(project); err != nil {
		t.Fatalf("Failed to create project: %v", err)
	}
	db1.Close()

	// Open again and verify project exists
	db2, err := OpenProjectDB(tmpDir, projectID)
	if err != nil {
		t.Fatalf("Second OpenProjectDB() returned error: %v", err)
	}
	defer db2.Close()

	got, err := db2.GetProject("proj-1")
	if err != nil {
		t.Fatalf("Failed to get project: %v", err)
	}

	if got.Name != "Test Project" {
		t.Errorf("Project name = %v, want %v", got.Name, "Test Project")
	}
}

func TestGenerateProjectID(t *testing.T) {
	id1 := GenerateProjectID()
	id2 := GenerateProjectID()

	// IDs should be 8 characters long
	if len(id1) != 8 {
		t.Errorf("GenerateProjectID() length = %d, want 8", len(id1))
	}

	// IDs should be different
	if id1 == id2 {
		t.Error("GenerateProjectID() returned same ID twice")
	}

	// IDs should be filesystem-safe (alphanumeric and hyphens)
	for _, c := range id1 {
		if !((c >= 'a' && c <= 'z') || (c >= '0' && c <= '9') || c == '-') {
			t.Errorf("GenerateProjectID() contains unsafe character: %c", c)
		}
	}
}

func TestGenerateProjectID_Uniqueness(t *testing.T) {
	seen := make(map[string]bool)
	for i := 0; i < 100; i++ {
		id := GenerateProjectID()
		if seen[id] {
			t.Errorf("GenerateProjectID() returned duplicate ID: %s", id)
		}
		seen[id] = true
	}
}

func TestOpenProjectDB_CreatesDirIfMissing(t *testing.T) {
	tmpDir := t.TempDir()
	projectID := "new-proj"

	// Project directory should not exist yet
	projectDir := filepath.Join(tmpDir, projectID)
	if _, err := os.Stat(projectDir); !os.IsNotExist(err) {
		t.Fatal("Project directory should not exist before OpenProjectDB")
	}

	database, err := OpenProjectDB(tmpDir, projectID)
	if err != nil {
		t.Fatalf("OpenProjectDB() returned error: %v", err)
	}
	defer database.Close()

	// Project directory should now exist
	if _, err := os.Stat(projectDir); os.IsNotExist(err) {
		t.Error("Project directory was not created")
	}
}

func TestProjectDBPath_Portability(t *testing.T) {
	// Test that the path uses proper separators
	path := ProjectDBPath("/base", "id")

	// Should not contain double slashes
	if strings.Contains(path, "//") {
		t.Errorf("Path contains double slashes: %s", path)
	}
}
