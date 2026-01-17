package config

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestDefaultConfig(t *testing.T) {
	cfg := DefaultConfig()

	if cfg.DatabasePath != "~/.local/share/ralph/ralph.db" {
		t.Errorf("expected default database path, got %s", cfg.DatabasePath)
	}

	if cfg.ProjectsDir != "~/.local/share/ralph/projects" {
		t.Errorf("expected default projects dir, got %s", cfg.ProjectsDir)
	}

	if cfg.MaxReviewIterations != 5 {
		t.Errorf("expected max_review_iterations=5, got %d", cfg.MaxReviewIterations)
	}

	if cfg.MaxTaskAttempts != 10 {
		t.Errorf("expected max_task_attempts=10, got %d", cfg.MaxTaskAttempts)
	}

	if cfg.DefaultPauseMode != false {
		t.Errorf("expected default_pause_mode=false, got %v", cfg.DefaultPauseMode)
	}

	if cfg.Claude.Model != "opus" {
		t.Errorf("expected model=opus, got %s", cfg.Claude.Model)
	}

	if cfg.Claude.MaxTurns != 50 {
		t.Errorf("expected max_turns=50, got %d", cfg.Claude.MaxTurns)
	}

	if cfg.Claude.MaxBudgetUSD != 10.0 {
		t.Errorf("expected max_budget_usd=10.0, got %f", cfg.Claude.MaxBudgetUSD)
	}

	if cfg.Agents.Developer != "" {
		t.Errorf("expected empty developer path, got %s", cfg.Agents.Developer)
	}

	if cfg.Agents.Reviewer != "" {
		t.Errorf("expected empty reviewer path, got %s", cfg.Agents.Reviewer)
	}

	if cfg.Agents.Planner != "" {
		t.Errorf("expected empty planner path, got %s", cfg.Agents.Planner)
	}
}

func TestLoadFromPath_MissingFile(t *testing.T) {
	// Load from a non-existent path should return defaults.
	cfg, err := LoadFromPath("/nonexistent/path/config.json")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Should have default values with expanded paths.
	if !strings.HasSuffix(cfg.DatabasePath, ".local/share/ralph/ralph.db") {
		t.Errorf("expected expanded default database path, got %s", cfg.DatabasePath)
	}

	if !strings.HasSuffix(cfg.ProjectsDir, ".local/share/ralph/projects") {
		t.Errorf("expected expanded default projects dir, got %s", cfg.ProjectsDir)
	}

	if cfg.MaxReviewIterations != 5 {
		t.Errorf("expected default max_review_iterations, got %d", cfg.MaxReviewIterations)
	}
}

func TestLoadFromPath_ValidFile(t *testing.T) {
	// Create a temp directory and config file.
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "config.json")

	configJSON := `{
		"database_path": "/custom/path/db.sqlite",
		"max_review_iterations": 3,
		"claude": {
			"model": "sonnet",
			"max_turns": 25
		}
	}`

	if err := os.WriteFile(configPath, []byte(configJSON), 0644); err != nil {
		t.Fatalf("failed to write config file: %v", err)
	}

	cfg, err := LoadFromPath(configPath)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Check that file values were applied.
	if cfg.DatabasePath != "/custom/path/db.sqlite" {
		t.Errorf("expected custom database path, got %s", cfg.DatabasePath)
	}

	if cfg.MaxReviewIterations != 3 {
		t.Errorf("expected max_review_iterations=3, got %d", cfg.MaxReviewIterations)
	}

	if cfg.Claude.Model != "sonnet" {
		t.Errorf("expected model=sonnet, got %s", cfg.Claude.Model)
	}

	if cfg.Claude.MaxTurns != 25 {
		t.Errorf("expected max_turns=25, got %d", cfg.Claude.MaxTurns)
	}

	// Check that defaults were preserved for fields not in file.
	if cfg.MaxTaskAttempts != 10 {
		t.Errorf("expected default max_task_attempts=10, got %d", cfg.MaxTaskAttempts)
	}

	if cfg.Claude.MaxBudgetUSD != 10.0 {
		t.Errorf("expected default max_budget_usd=10.0, got %f", cfg.Claude.MaxBudgetUSD)
	}
}

func TestLoadFromPath_PartialConfig(t *testing.T) {
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "config.json")

	// Only set one field.
	configJSON := `{"max_review_iterations": 7}`

	if err := os.WriteFile(configPath, []byte(configJSON), 0644); err != nil {
		t.Fatalf("failed to write config file: %v", err)
	}

	cfg, err := LoadFromPath(configPath)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// The set field should be updated.
	if cfg.MaxReviewIterations != 7 {
		t.Errorf("expected max_review_iterations=7, got %d", cfg.MaxReviewIterations)
	}

	// All other fields should be defaults.
	if cfg.MaxTaskAttempts != 10 {
		t.Errorf("expected default max_task_attempts, got %d", cfg.MaxTaskAttempts)
	}

	if cfg.Claude.Model != "opus" {
		t.Errorf("expected default model, got %s", cfg.Claude.Model)
	}
}

func TestLoadFromPath_InvalidJSON(t *testing.T) {
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "config.json")

	// Write invalid JSON.
	if err := os.WriteFile(configPath, []byte("not valid json"), 0644); err != nil {
		t.Fatalf("failed to write config file: %v", err)
	}

	_, err := LoadFromPath(configPath)
	if err == nil {
		t.Fatal("expected error for invalid JSON")
	}

	if !strings.Contains(err.Error(), "failed to parse config file") {
		t.Errorf("expected parse error, got: %v", err)
	}
}

func TestLoadFromPath_InvalidValues(t *testing.T) {
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "config.json")

	// Invalid: max_review_iterations = 0.
	configJSON := `{"max_review_iterations": 0}`

	if err := os.WriteFile(configPath, []byte(configJSON), 0644); err != nil {
		t.Fatalf("failed to write config file: %v", err)
	}

	_, err := LoadFromPath(configPath)
	if err == nil {
		t.Fatal("expected validation error")
	}

	if !strings.Contains(err.Error(), "max_review_iterations must be >= 1") {
		t.Errorf("expected validation error message, got: %v", err)
	}
}

func TestValidate_AllValid(t *testing.T) {
	cfg := DefaultConfig()
	if err := cfg.Validate(); err != nil {
		t.Errorf("default config should be valid, got error: %v", err)
	}
}

func TestValidate_InvalidMaxReviewIterations(t *testing.T) {
	cfg := DefaultConfig()
	cfg.MaxReviewIterations = 0

	err := cfg.Validate()
	if err == nil {
		t.Fatal("expected validation error")
	}

	if !strings.Contains(err.Error(), "max_review_iterations must be >= 1") {
		t.Errorf("expected specific error message, got: %v", err)
	}
}

func TestValidate_InvalidMaxTaskAttempts(t *testing.T) {
	cfg := DefaultConfig()
	cfg.MaxTaskAttempts = 0

	err := cfg.Validate()
	if err == nil {
		t.Fatal("expected validation error")
	}

	if !strings.Contains(err.Error(), "max_task_attempts must be >= 1") {
		t.Errorf("expected specific error message, got: %v", err)
	}
}

func TestValidate_InvalidClaudeModel(t *testing.T) {
	cfg := DefaultConfig()
	cfg.Claude.Model = ""

	err := cfg.Validate()
	if err == nil {
		t.Fatal("expected validation error")
	}

	if !strings.Contains(err.Error(), "claude.model must be non-empty") {
		t.Errorf("expected specific error message, got: %v", err)
	}
}

func TestValidate_InvalidClaudeMaxTurns(t *testing.T) {
	cfg := DefaultConfig()
	cfg.Claude.MaxTurns = 0

	err := cfg.Validate()
	if err == nil {
		t.Fatal("expected validation error")
	}

	if !strings.Contains(err.Error(), "claude.max_turns must be >= 1") {
		t.Errorf("expected specific error message, got: %v", err)
	}
}

func TestValidate_InvalidClaudeMaxBudget(t *testing.T) {
	cfg := DefaultConfig()
	cfg.Claude.MaxBudgetUSD = 0

	err := cfg.Validate()
	if err == nil {
		t.Fatal("expected validation error")
	}

	if !strings.Contains(err.Error(), "claude.max_budget_usd must be > 0") {
		t.Errorf("expected specific error message, got: %v", err)
	}
}

func TestValidate_MultipleErrors(t *testing.T) {
	cfg := DefaultConfig()
	cfg.MaxReviewIterations = 0
	cfg.MaxTaskAttempts = 0
	cfg.Claude.Model = ""

	err := cfg.Validate()
	if err == nil {
		t.Fatal("expected validation error")
	}

	// Should contain all three error messages.
	errStr := err.Error()
	if !strings.Contains(errStr, "max_review_iterations") {
		t.Error("expected max_review_iterations error")
	}
	if !strings.Contains(errStr, "max_task_attempts") {
		t.Error("expected max_task_attempts error")
	}
	if !strings.Contains(errStr, "claude.model") {
		t.Error("expected claude.model error")
	}
}

func TestValidate_NonexistentAgentFile(t *testing.T) {
	cfg := DefaultConfig()
	cfg.Agents.Developer = "/nonexistent/path/developer.md"

	err := cfg.Validate()
	if err == nil {
		t.Fatal("expected validation error for nonexistent agent file")
	}

	if !strings.Contains(err.Error(), "agents.developer file does not exist") {
		t.Errorf("expected specific error message, got: %v", err)
	}
}

func TestExpandPaths_TildePath(t *testing.T) {
	cfg := &Config{
		DatabasePath: "~/.local/share/ralph/test.db",
	}

	if err := cfg.ExpandPaths(); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	home, _ := os.UserHomeDir()
	expected := filepath.Join(home, ".local/share/ralph/test.db")

	if cfg.DatabasePath != expected {
		t.Errorf("expected %s, got %s", expected, cfg.DatabasePath)
	}
}

func TestExpandPaths_AbsolutePath(t *testing.T) {
	cfg := &Config{
		DatabasePath: "/absolute/path/db.sqlite",
	}

	if err := cfg.ExpandPaths(); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if cfg.DatabasePath != "/absolute/path/db.sqlite" {
		t.Errorf("expected unchanged path, got %s", cfg.DatabasePath)
	}
}

func TestExpandPaths_AgentPaths(t *testing.T) {
	cfg := &Config{
		DatabasePath: "/db/path",
		Agents: AgentConfig{
			Developer: "~/agents/developer.md",
			Reviewer:  "~/agents/reviewer.md",
			Planner:   "~/agents/planner.md",
		},
	}

	if err := cfg.ExpandPaths(); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	home, _ := os.UserHomeDir()

	if cfg.Agents.Developer != filepath.Join(home, "agents/developer.md") {
		t.Errorf("developer path not expanded correctly: %s", cfg.Agents.Developer)
	}

	if cfg.Agents.Reviewer != filepath.Join(home, "agents/reviewer.md") {
		t.Errorf("reviewer path not expanded correctly: %s", cfg.Agents.Reviewer)
	}

	if cfg.Agents.Planner != filepath.Join(home, "agents/planner.md") {
		t.Errorf("planner path not expanded correctly: %s", cfg.Agents.Planner)
	}
}

func TestExpandPaths_Idempotent(t *testing.T) {
	cfg := &Config{
		DatabasePath: "~/.local/share/ralph/test.db",
	}

	// Expand twice.
	if err := cfg.ExpandPaths(); err != nil {
		t.Fatalf("unexpected error on first expand: %v", err)
	}

	firstPath := cfg.DatabasePath

	if err := cfg.ExpandPaths(); err != nil {
		t.Fatalf("unexpected error on second expand: %v", err)
	}

	if cfg.DatabasePath != firstPath {
		t.Errorf("expand should be idempotent, got different results")
	}
}

func TestGetDatabasePath(t *testing.T) {
	cfg := &Config{
		DatabasePath: "/custom/path/db.sqlite",
	}

	if cfg.GetDatabasePath() != "/custom/path/db.sqlite" {
		t.Errorf("GetDatabasePath returned wrong value: %s", cfg.GetDatabasePath())
	}
}

func TestGetProjectsDir(t *testing.T) {
	cfg := &Config{
		ProjectsDir: "/custom/projects",
	}

	if cfg.GetProjectsDir() != "/custom/projects" {
		t.Errorf("GetProjectsDir returned wrong value: %s", cfg.GetProjectsDir())
	}
}

func TestLoadFromPath_ProjectsDir(t *testing.T) {
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "config.json")

	configJSON := `{
		"projects_dir": "~/my-projects"
	}`

	if err := os.WriteFile(configPath, []byte(configJSON), 0644); err != nil {
		t.Fatalf("failed to write config file: %v", err)
	}

	cfg, err := LoadFromPath(configPath)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	home, _ := os.UserHomeDir()
	expected := filepath.Join(home, "my-projects")

	if cfg.ProjectsDir != expected {
		t.Errorf("expected %s, got %s", expected, cfg.ProjectsDir)
	}
}

func TestExpandPaths_ProjectsDir(t *testing.T) {
	cfg := &Config{
		DatabasePath: "/db/path",
		ProjectsDir:  "~/my-projects",
	}

	if err := cfg.ExpandPaths(); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	home, _ := os.UserHomeDir()
	expected := filepath.Join(home, "my-projects")

	if cfg.ProjectsDir != expected {
		t.Errorf("expected %s, got %s", expected, cfg.ProjectsDir)
	}
}

func TestGetAgentPrompt_DefaultDeveloper(t *testing.T) {
	cfg := DefaultConfig()

	prompt, err := cfg.GetAgentPrompt("developer")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Config returns empty string for defaults - caller handles defaults.
	if prompt != "" {
		t.Errorf("expected empty string for default, got %s", prompt)
	}
}

func TestGetAgentPrompt_DefaultReviewer(t *testing.T) {
	cfg := DefaultConfig()

	prompt, err := cfg.GetAgentPrompt("reviewer")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Config returns empty string for defaults - caller handles defaults.
	if prompt != "" {
		t.Errorf("expected empty string for default, got %s", prompt)
	}
}

func TestGetAgentPrompt_DefaultPlanner(t *testing.T) {
	cfg := DefaultConfig()

	prompt, err := cfg.GetAgentPrompt("planner")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Config returns empty string for defaults - caller handles defaults.
	if prompt != "" {
		t.Errorf("expected empty string for default, got %s", prompt)
	}
}

func TestGetAgentPrompt_CustomFile(t *testing.T) {
	tmpDir := t.TempDir()
	promptPath := filepath.Join(tmpDir, "custom_developer.md")

	customPrompt := "This is a custom developer prompt."
	if err := os.WriteFile(promptPath, []byte(customPrompt), 0644); err != nil {
		t.Fatalf("failed to write custom prompt: %v", err)
	}

	cfg := DefaultConfig()
	cfg.Agents.Developer = promptPath

	prompt, err := cfg.GetAgentPrompt("developer")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if prompt != customPrompt {
		t.Errorf("expected custom prompt, got: %s", prompt)
	}
}

func TestGetAgentPrompt_CustomFileNotFound(t *testing.T) {
	cfg := DefaultConfig()
	cfg.Agents.Developer = "/nonexistent/path/developer.md"

	_, err := cfg.GetAgentPrompt("developer")
	if err == nil {
		t.Fatal("expected error for nonexistent custom file")
	}

	if !strings.Contains(err.Error(), "failed to read custom prompt file") {
		t.Errorf("expected specific error message, got: %v", err)
	}
}

func TestGetAgentPrompt_UnknownType(t *testing.T) {
	cfg := DefaultConfig()

	_, err := cfg.GetAgentPrompt("unknown")
	if err == nil {
		t.Fatal("expected error for unknown agent type")
	}

	if !strings.Contains(err.Error(), "unknown agent type") {
		t.Errorf("expected specific error message, got: %v", err)
	}
}

func TestExpandPath_EmptyString(t *testing.T) {
	path, err := expandPath("")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if path != "" {
		t.Errorf("expected empty string, got %s", path)
	}
}

func TestExpandPath_Tilde(t *testing.T) {
	path, err := expandPath("~/test/path")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	home, _ := os.UserHomeDir()
	expected := filepath.Join(home, "test/path")

	if path != expected {
		t.Errorf("expected %s, got %s", expected, path)
	}
}

func TestExpandPath_TildeOnly(t *testing.T) {
	path, err := expandPath("~")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	home, _ := os.UserHomeDir()

	if path != home {
		t.Errorf("expected %s, got %s", home, path)
	}
}

func TestExpandPath_AbsolutePath(t *testing.T) {
	path, err := expandPath("/absolute/path")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if path != "/absolute/path" {
		t.Errorf("expected /absolute/path, got %s", path)
	}
}

func TestExpandPath_RelativePath(t *testing.T) {
	path, err := expandPath("relative/path")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Should be cleaned but stay relative.
	if path != "relative/path" {
		t.Errorf("expected relative/path, got %s", path)
	}
}

func TestLoadFromPath_TildeExpansion(t *testing.T) {
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "config.json")

	configJSON := `{
		"database_path": "~/.ralph/custom.db"
	}`

	if err := os.WriteFile(configPath, []byte(configJSON), 0644); err != nil {
		t.Fatalf("failed to write config file: %v", err)
	}

	cfg, err := LoadFromPath(configPath)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	home, _ := os.UserHomeDir()
	expected := filepath.Join(home, ".ralph/custom.db")

	if cfg.DatabasePath != expected {
		t.Errorf("expected %s, got %s", expected, cfg.DatabasePath)
	}
}

func TestLoadFromPath_AgentWithExistingFile(t *testing.T) {
	tmpDir := t.TempDir()

	// Create a custom agent file.
	agentPath := filepath.Join(tmpDir, "developer.md")
	if err := os.WriteFile(agentPath, []byte("custom prompt"), 0644); err != nil {
		t.Fatalf("failed to write agent file: %v", err)
	}

	configPath := filepath.Join(tmpDir, "config.json")
	configJSON := `{
		"agents": {
			"developer": "` + agentPath + `"
		}
	}`

	if err := os.WriteFile(configPath, []byte(configJSON), 0644); err != nil {
		t.Fatalf("failed to write config file: %v", err)
	}

	cfg, err := LoadFromPath(configPath)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if cfg.Agents.Developer != agentPath {
		t.Errorf("expected %s, got %s", agentPath, cfg.Agents.Developer)
	}
}

func TestLoadFromPath_AgentWithNonexistentFile(t *testing.T) {
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "config.json")

	configJSON := `{
		"agents": {
			"developer": "/nonexistent/developer.md"
		}
	}`

	if err := os.WriteFile(configPath, []byte(configJSON), 0644); err != nil {
		t.Fatalf("failed to write config file: %v", err)
	}

	_, err := LoadFromPath(configPath)
	if err == nil {
		t.Fatal("expected validation error for nonexistent agent file")
	}

	if !strings.Contains(err.Error(), "agents.developer file does not exist") {
		t.Errorf("expected specific error message, got: %v", err)
	}
}

func TestLoadFromPath_DefaultPauseMode(t *testing.T) {
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "config.json")

	// Test with pause mode enabled
	configJSON := `{"default_pause_mode": true}`
	if err := os.WriteFile(configPath, []byte(configJSON), 0644); err != nil {
		t.Fatalf("failed to write config file: %v", err)
	}

	cfg, err := LoadFromPath(configPath)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if !cfg.DefaultPauseMode {
		t.Error("expected default_pause_mode to be true")
	}

	// Test with pause mode disabled (explicit)
	configJSON = `{"default_pause_mode": false}`
	if err := os.WriteFile(configPath, []byte(configJSON), 0644); err != nil {
		t.Fatalf("failed to write config file: %v", err)
	}

	cfg, err = LoadFromPath(configPath)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if cfg.DefaultPauseMode {
		t.Error("expected default_pause_mode to be false")
	}
}

func TestLoadFromPath_DefaultPauseModeNotSet(t *testing.T) {
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "config.json")

	// Config without pause mode setting should use default (false)
	configJSON := `{"max_review_iterations": 7}`
	if err := os.WriteFile(configPath, []byte(configJSON), 0644); err != nil {
		t.Fatalf("failed to write config file: %v", err)
	}

	cfg, err := LoadFromPath(configPath)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if cfg.DefaultPauseMode {
		t.Error("expected default_pause_mode to be false when not set")
	}
}
