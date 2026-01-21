package config

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestLoadFromPath_MissingFile(t *testing.T) {
	// Missing file should return default config (not an error)
	cfg, err := LoadFromPath("/nonexistent/path/config.json")
	if err != nil {
		t.Fatalf("expected default config for missing file, got error: %v", err)
	}

	// Check defaults
	if cfg.MaxIterations != 15 {
		t.Errorf("expected default max_iterations=15, got %d", cfg.MaxIterations)
	}
}

func TestLoadFromPath_ValidMinimalConfig(t *testing.T) {
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "config.json")

	// Minimal valid config with just max_iterations.
	configJSON := `{"max_iterations": 20}`

	if err := os.WriteFile(configPath, []byte(configJSON), 0644); err != nil {
		t.Fatalf("failed to write config file: %v", err)
	}

	cfg, err := LoadFromPath(configPath)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if cfg.MaxIterations != 20 {
		t.Errorf("expected max_iterations=20, got %d", cfg.MaxIterations)
	}

	// Check defaults were applied for other fields
	if cfg.Claude.Model != "opus" {
		t.Errorf("expected default model=opus, got %s", cfg.Claude.Model)
	}

	if cfg.Claude.MaxTurns != 50 {
		t.Errorf("expected default max_turns=50, got %d", cfg.Claude.MaxTurns)
	}

	if cfg.Claude.Verbose != true {
		t.Errorf("expected verbose=true (default), got %v", cfg.Claude.Verbose)
	}
}

func TestLoadFromPath_FullConfig(t *testing.T) {
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "config.json")

	configJSON := `{
		"max_iterations": 15,
		"claude": {
			"model": "claude-sonnet-4-20250514",
			"max_turns": 100,
			"verbose": false
		}
	}`

	if err := os.WriteFile(configPath, []byte(configJSON), 0644); err != nil {
		t.Fatalf("failed to write config file: %v", err)
	}

	cfg, err := LoadFromPath(configPath)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if cfg.MaxIterations != 15 {
		t.Errorf("expected max_iterations=15, got %d", cfg.MaxIterations)
	}

	if cfg.Claude.Model != "claude-sonnet-4-20250514" {
		t.Errorf("expected model=claude-sonnet-4-20250514, got %s", cfg.Claude.Model)
	}

	if cfg.Claude.MaxTurns != 100 {
		t.Errorf("expected max_turns=100, got %d", cfg.Claude.MaxTurns)
	}

	if cfg.Claude.Verbose != false {
		t.Errorf("expected verbose=false, got %v", cfg.Claude.Verbose)
	}
}

func TestLoadFromPath_EmptyConfig(t *testing.T) {
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "config.json")

	// Empty config should use all defaults
	if err := os.WriteFile(configPath, []byte("{}"), 0644); err != nil {
		t.Fatalf("failed to write config file: %v", err)
	}

	cfg, err := LoadFromPath(configPath)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// All defaults should apply
	if cfg.MaxIterations != 15 {
		t.Errorf("expected default max_iterations=15, got %d", cfg.MaxIterations)
	}

	if cfg.Claude.Model != "opus" {
		t.Errorf("expected default model=opus, got %s", cfg.Claude.Model)
	}
}

func TestLoadFromPath_InvalidJSON(t *testing.T) {
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "config.json")

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

func TestLoadFromPath_PartialClaudeConfig(t *testing.T) {
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "config.json")

	// Only set model, should use defaults for max_turns and verbose.
	configJSON := `{
		"claude": {
			"model": "sonnet"
		}
	}`

	if err := os.WriteFile(configPath, []byte(configJSON), 0644); err != nil {
		t.Fatalf("failed to write config file: %v", err)
	}

	cfg, err := LoadFromPath(configPath)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if cfg.Claude.Model != "sonnet" {
		t.Errorf("expected model=sonnet, got %s", cfg.Claude.Model)
	}

	// Should use defaults for unset fields.
	if cfg.Claude.MaxTurns != 50 {
		t.Errorf("expected default max_turns=50, got %d", cfg.Claude.MaxTurns)
	}

	if cfg.Claude.Verbose != true {
		t.Errorf("expected default verbose=true, got %v", cfg.Claude.Verbose)
	}
}

func TestLoadFromPath_VerboseExplicitlyFalse(t *testing.T) {
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "config.json")

	configJSON := `{
		"claude": {
			"verbose": false
		}
	}`

	if err := os.WriteFile(configPath, []byte(configJSON), 0644); err != nil {
		t.Fatalf("failed to write config file: %v", err)
	}

	cfg, err := LoadFromPath(configPath)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if cfg.Claude.Verbose != false {
		t.Errorf("expected verbose=false, got %v", cfg.Claude.Verbose)
	}
}

func TestLoadFromPath_ZeroMaxIterations(t *testing.T) {
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "config.json")

	// Explicitly set to 0 should trigger validation error
	configJSON := `{"max_iterations": 0}`

	if err := os.WriteFile(configPath, []byte(configJSON), 0644); err != nil {
		t.Fatalf("failed to write config file: %v", err)
	}

	_, err := LoadFromPath(configPath)
	if err == nil {
		t.Fatal("expected validation error for max_iterations=0")
	}

	if !strings.Contains(err.Error(), "max_iterations must be >= 1") {
		t.Errorf("expected validation error message, got: %v", err)
	}
}

func TestLoadFromPath_NegativeMaxIterations(t *testing.T) {
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "config.json")

	configJSON := `{"max_iterations": -5}`

	if err := os.WriteFile(configPath, []byte(configJSON), 0644); err != nil {
		t.Fatalf("failed to write config file: %v", err)
	}

	_, err := LoadFromPath(configPath)
	if err == nil {
		t.Fatal("expected validation error for negative max_iterations")
	}

	if !strings.Contains(err.Error(), "max_iterations must be >= 1") {
		t.Errorf("expected validation error message, got: %v", err)
	}
}

func TestLoadFromPath_ZeroMaxTurns(t *testing.T) {
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "config.json")

	// max_turns=0 should trigger validation error
	configJSON := `{
		"claude": {
			"max_turns": 0
		}
	}`

	if err := os.WriteFile(configPath, []byte(configJSON), 0644); err != nil {
		t.Fatalf("failed to write config file: %v", err)
	}

	_, err := LoadFromPath(configPath)
	if err == nil {
		t.Fatal("expected validation error for max_turns=0")
	}

	if !strings.Contains(err.Error(), "claude.max_turns must be >= 1") {
		t.Errorf("expected validation error message, got: %v", err)
	}
}

func TestLoadFromPath_NegativeMaxTurns(t *testing.T) {
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "config.json")

	configJSON := `{
		"claude": {
			"max_turns": -1
		}
	}`

	if err := os.WriteFile(configPath, []byte(configJSON), 0644); err != nil {
		t.Fatalf("failed to write config file: %v", err)
	}

	_, err := LoadFromPath(configPath)
	if err == nil {
		t.Fatal("expected validation error for negative max_turns")
	}

	if !strings.Contains(err.Error(), "claude.max_turns must be >= 1") {
		t.Errorf("expected validation error message, got: %v", err)
	}
}

func TestValidate_Valid(t *testing.T) {
	cfg := &Config{
		MaxIterations:   10,
		MaxTaskAttempts: 5,
		Claude: ClaudeConfig{
			Model:    "sonnet",
			MaxTurns: 50,
			Verbose:  true,
		},
	}

	if err := cfg.Validate(); err != nil {
		t.Errorf("expected valid config, got error: %v", err)
	}
}

func TestValidate_InvalidMaxIterations(t *testing.T) {
	cfg := DefaultConfig()
	cfg.MaxIterations = 0

	err := cfg.Validate()
	if err == nil {
		t.Fatal("expected validation error")
	}

	if !strings.Contains(err.Error(), "max_iterations must be >= 1") {
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

func TestValidate_InvalidMaxTurns(t *testing.T) {
	cfg := DefaultConfig()
	cfg.Claude.MaxTurns = 0

	err := cfg.Validate()
	if err == nil {
		t.Fatal("expected validation error for zero max_turns")
	}

	if !strings.Contains(err.Error(), "claude.max_turns must be >= 1") {
		t.Errorf("expected specific error message, got: %v", err)
	}
}

func TestValidate_EmptyModel(t *testing.T) {
	cfg := DefaultConfig()
	cfg.Claude.Model = ""

	err := cfg.Validate()
	if err == nil {
		t.Fatal("expected validation error for empty model")
	}

	if !strings.Contains(err.Error(), "claude.model must be non-empty") {
		t.Errorf("expected specific error message, got: %v", err)
	}
}

func TestValidate_MultipleErrors(t *testing.T) {
	cfg := &Config{
		MaxIterations:   0,
		MaxTaskAttempts: 0,
		Claude: ClaudeConfig{
			Model:    "",
			MaxTurns: 0,
		},
	}

	err := cfg.Validate()
	if err == nil {
		t.Fatal("expected validation error")
	}

	errStr := err.Error()
	if !strings.Contains(errStr, "max_iterations") {
		t.Error("expected max_iterations error")
	}
	if !strings.Contains(errStr, "max_task_attempts") {
		t.Error("expected max_task_attempts error")
	}
	if !strings.Contains(errStr, "max_turns") {
		t.Error("expected max_turns error")
	}
	if !strings.Contains(errStr, "model") {
		t.Error("expected model error")
	}
}

func TestDefaultConfig(t *testing.T) {
	cfg := DefaultConfig()

	if cfg.MaxIterations != 15 {
		t.Errorf("expected default max_iterations=15, got %d", cfg.MaxIterations)
	}

	if cfg.MaxReviewIterations != 15 {
		t.Errorf("expected default max_review_iterations=15, got %d", cfg.MaxReviewIterations)
	}

	if cfg.MaxTaskAttempts != 10 {
		t.Errorf("expected default max_task_attempts=10, got %d", cfg.MaxTaskAttempts)
	}

	if cfg.Claude.Model != "opus" {
		t.Errorf("expected default model=opus, got %s", cfg.Claude.Model)
	}

	if cfg.Claude.MaxTurns != 50 {
		t.Errorf("expected default max_turns=50, got %d", cfg.Claude.MaxTurns)
	}

	if cfg.Claude.Verbose != true {
		t.Errorf("expected default verbose=true, got %v", cfg.Claude.Verbose)
	}
}

func TestGetProjectsDir(t *testing.T) {
	cfg := DefaultConfig()
	if err := cfg.ExpandPaths(); err != nil {
		t.Fatalf("failed to expand paths: %v", err)
	}

	projectsDir := cfg.GetProjectsDir()
	if projectsDir == "" {
		t.Error("expected non-empty projects dir")
	}

	// Should not contain ~ (should be expanded)
	if strings.Contains(projectsDir, "~") {
		t.Errorf("expected ~ to be expanded, got: %s", projectsDir)
	}

	// Should contain the default path components
	if !strings.Contains(projectsDir, "ralph") || !strings.Contains(projectsDir, "projects") {
		t.Errorf("expected projects dir to contain 'ralph/projects', got: %s", projectsDir)
	}
}

func TestGetAgentPrompt_NoCustomPath(t *testing.T) {
	cfg := DefaultConfig()

	// With no custom path set, should return empty string
	prompt, err := cfg.GetAgentPrompt("developer")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if prompt != "" {
		t.Errorf("expected empty prompt for no custom path, got: %s", prompt)
	}
}

func TestGetAgentPrompt_UnknownType(t *testing.T) {
	cfg := DefaultConfig()

	_, err := cfg.GetAgentPrompt("unknown")
	if err == nil {
		t.Fatal("expected error for unknown agent type")
	}

	if !strings.Contains(err.Error(), "unknown agent type") {
		t.Errorf("expected unknown agent type error, got: %v", err)
	}
}

func TestGetAgentPrompt_CustomPath(t *testing.T) {
	tmpDir := t.TempDir()
	promptPath := filepath.Join(tmpDir, "custom_developer.md")

	customPrompt := "Custom developer prompt content"
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
		t.Errorf("expected '%s', got '%s'", customPrompt, prompt)
	}
}

func TestEnsureConfigDir(t *testing.T) {
	// This test verifies EnsureConfigDir creates the directory.
	// We can't easily test the actual ~/.config/ralph without side effects,
	// so we just verify it returns a valid path.
	dir, err := EnsureConfigDir()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if dir == "" {
		t.Error("expected non-empty directory path")
	}

	// Verify the directory exists.
	info, err := os.Stat(dir)
	if err != nil {
		t.Fatalf("config directory should exist: %v", err)
	}

	if !info.IsDir() {
		t.Error("expected directory, not file")
	}
}

func TestGetConfigPath(t *testing.T) {
	path, err := GetConfigPath()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if path == "" {
		t.Error("expected non-empty path")
	}

	// Should end with config.json.
	if !strings.HasSuffix(path, "config.json") {
		t.Errorf("expected path to end with config.json, got: %s", path)
	}

	// Should contain ralph.
	if !strings.Contains(path, "ralph") {
		t.Errorf("expected path to contain 'ralph', got: %s", path)
	}

	// Should not contain ~.
	if strings.Contains(path, "~") {
		t.Errorf("expected ~ to be expanded, got: %s", path)
	}
}

func TestExpandPath_Empty(t *testing.T) {
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

	if path != "relative/path" {
		t.Errorf("expected relative/path, got %s", path)
	}
}

func TestExpandPath_CleansDotDot(t *testing.T) {
	path, err := expandPath("/foo/bar/../baz")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if path != "/foo/baz" {
		t.Errorf("expected /foo/baz, got %s", path)
	}
}

func TestMaxIterationsAndMaxReviewIterationsSync(t *testing.T) {
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "config.json")

	// Test max_iterations sets max_review_iterations
	configJSON := `{"max_iterations": 8}`

	if err := os.WriteFile(configPath, []byte(configJSON), 0644); err != nil {
		t.Fatalf("failed to write config file: %v", err)
	}

	cfg, err := LoadFromPath(configPath)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if cfg.MaxIterations != 8 {
		t.Errorf("expected max_iterations=8, got %d", cfg.MaxIterations)
	}

	if cfg.MaxReviewIterations != 8 {
		t.Errorf("expected max_review_iterations=8 (synced from max_iterations), got %d", cfg.MaxReviewIterations)
	}
}

func TestMaxReviewIterationsSetsMaxIterations(t *testing.T) {
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "config.json")

	// Test max_review_iterations sets max_iterations (backward compat)
	configJSON := `{"max_review_iterations": 12}`

	if err := os.WriteFile(configPath, []byte(configJSON), 0644); err != nil {
		t.Fatalf("failed to write config file: %v", err)
	}

	cfg, err := LoadFromPath(configPath)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if cfg.MaxReviewIterations != 12 {
		t.Errorf("expected max_review_iterations=12, got %d", cfg.MaxReviewIterations)
	}

	if cfg.MaxIterations != 12 {
		t.Errorf("expected max_iterations=12 (synced from max_review_iterations), got %d", cfg.MaxIterations)
	}
}

func TestExpandPaths_OnlyOnce(t *testing.T) {
	cfg := DefaultConfig()

	// First call
	if err := cfg.ExpandPaths(); err != nil {
		t.Fatalf("unexpected error on first expand: %v", err)
	}

	// Verify first expansion worked
	if strings.Contains(cfg.ProjectsDir, "~") {
		t.Errorf("expected ~ to be expanded on first call, got %s", cfg.ProjectsDir)
	}

	// Change the unexpanded value and call again
	cfg.ProjectsDir = "~/different/path"

	// Second call should be a no-op (expandedPaths flag prevents re-expansion)
	if err := cfg.ExpandPaths(); err != nil {
		t.Fatalf("unexpected error on second expand: %v", err)
	}

	// The ~ should NOT be expanded since ExpandPaths is a no-op after first call
	if cfg.ProjectsDir != "~/different/path" {
		t.Errorf("expected ProjectsDir to remain ~/different/path (unexpanded), got %s", cfg.ProjectsDir)
	}
}
