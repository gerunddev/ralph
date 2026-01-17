// Package config provides configuration loading and validation for Ralph.
package config

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// Standard config file location.
const defaultConfigPath = "~/.config/ralph/config.json"

// Config holds all Ralph configuration settings.
type Config struct {
	DatabasePath        string       `json:"database_path"`         // Deprecated: Use ProjectsDir instead
	ProjectsDir         string       `json:"projects_dir"`          // Base directory for per-project databases
	MaxReviewIterations int          `json:"max_review_iterations"`
	MaxTaskAttempts     int          `json:"max_task_attempts"`
	DefaultPauseMode    bool         `json:"default_pause_mode"`    // Whether to pause between tasks by default
	Claude              ClaudeConfig `json:"claude"`
	Agents              AgentConfig  `json:"agents"`

	// expandedPaths tracks whether ExpandPaths has been called.
	expandedPaths bool
}

// ClaudeConfig holds Claude-specific configuration.
type ClaudeConfig struct {
	Model        string  `json:"model"`
	MaxTurns     int     `json:"max_turns"`
	MaxBudgetUSD float64 `json:"max_budget_usd"`
}

// AgentConfig holds paths to custom agent prompts.
type AgentConfig struct {
	Developer  string `json:"developer"`
	Reviewer   string `json:"reviewer"`
	Planner    string `json:"planner"`
	Documenter string `json:"documenter"`
}

// DefaultConfig returns the default configuration.
func DefaultConfig() *Config {
	return &Config{
		DatabasePath:        "~/.local/share/ralph/ralph.db", // Deprecated
		ProjectsDir:         "~/.local/share/ralph/projects",
		MaxReviewIterations: 5,
		MaxTaskAttempts:     10,
		Claude: ClaudeConfig{
			Model:        "opus",
			MaxTurns:     50,
			MaxBudgetUSD: 10.0,
		},
		Agents: AgentConfig{},
	}
}

// Load reads config from the standard location (~/.config/ralph/config.json),
// falling back to defaults if the file doesn't exist.
// Missing fields use default values (not zero values).
func Load() (*Config, error) {
	configPath, err := expandPath(defaultConfigPath)
	if err != nil {
		return nil, fmt.Errorf("failed to expand config path: %w", err)
	}
	return LoadFromPath(configPath)
}

// LoadFromPath reads config from a specific path.
// If the file doesn't exist, returns default config.
// If the file exists but is invalid, returns an error.
func LoadFromPath(path string) (*Config, error) {
	// Start with default config.
	cfg := DefaultConfig()

	// Check if config file exists.
	if _, err := os.Stat(path); os.IsNotExist(err) {
		// No config file - use all defaults.
		if err := cfg.ExpandPaths(); err != nil {
			return nil, fmt.Errorf("failed to expand paths: %w", err)
		}
		return cfg, nil
	}

	// Read the config file.
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("failed to read config file: %w", err)
	}

	// Parse JSON into a temporary struct for merging.
	var fileCfg fileConfig
	if err := json.Unmarshal(data, &fileCfg); err != nil {
		return nil, fmt.Errorf("failed to parse config file: %w", err)
	}

	// Merge file values over defaults (only non-zero values).
	mergeConfig(cfg, &fileCfg)

	// Expand paths.
	if err := cfg.ExpandPaths(); err != nil {
		return nil, fmt.Errorf("failed to expand paths: %w", err)
	}

	// Validate the merged config.
	if err := cfg.Validate(); err != nil {
		return nil, fmt.Errorf("invalid configuration: %w", err)
	}

	return cfg, nil
}

// fileConfig is used for parsing JSON with pointer fields to detect what was set.
type fileConfig struct {
	DatabasePath        *string           `json:"database_path"`
	ProjectsDir         *string           `json:"projects_dir"`
	MaxReviewIterations *int              `json:"max_review_iterations"`
	MaxTaskAttempts     *int              `json:"max_task_attempts"`
	DefaultPauseMode    *bool             `json:"default_pause_mode"`
	Claude              *fileClaudeConfig `json:"claude"`
	Agents              *fileAgentConfig  `json:"agents"`
}

type fileClaudeConfig struct {
	Model        *string  `json:"model"`
	MaxTurns     *int     `json:"max_turns"`
	MaxBudgetUSD *float64 `json:"max_budget_usd"`
}

type fileAgentConfig struct {
	Developer  *string `json:"developer"`
	Reviewer   *string `json:"reviewer"`
	Planner    *string `json:"planner"`
	Documenter *string `json:"documenter"`
}

// mergeConfig merges file config values into the default config.
// Only non-nil values from the file config are applied.
func mergeConfig(cfg *Config, fileCfg *fileConfig) {
	if fileCfg.DatabasePath != nil {
		cfg.DatabasePath = *fileCfg.DatabasePath
	}
	if fileCfg.ProjectsDir != nil {
		cfg.ProjectsDir = *fileCfg.ProjectsDir
	}
	if fileCfg.MaxReviewIterations != nil {
		cfg.MaxReviewIterations = *fileCfg.MaxReviewIterations
	}
	if fileCfg.MaxTaskAttempts != nil {
		cfg.MaxTaskAttempts = *fileCfg.MaxTaskAttempts
	}
	if fileCfg.DefaultPauseMode != nil {
		cfg.DefaultPauseMode = *fileCfg.DefaultPauseMode
	}

	if fileCfg.Claude != nil {
		if fileCfg.Claude.Model != nil {
			cfg.Claude.Model = *fileCfg.Claude.Model
		}
		if fileCfg.Claude.MaxTurns != nil {
			cfg.Claude.MaxTurns = *fileCfg.Claude.MaxTurns
		}
		if fileCfg.Claude.MaxBudgetUSD != nil {
			cfg.Claude.MaxBudgetUSD = *fileCfg.Claude.MaxBudgetUSD
		}
	}

	if fileCfg.Agents != nil {
		if fileCfg.Agents.Developer != nil {
			cfg.Agents.Developer = *fileCfg.Agents.Developer
		}
		if fileCfg.Agents.Reviewer != nil {
			cfg.Agents.Reviewer = *fileCfg.Agents.Reviewer
		}
		if fileCfg.Agents.Planner != nil {
			cfg.Agents.Planner = *fileCfg.Agents.Planner
		}
		if fileCfg.Agents.Documenter != nil {
			cfg.Agents.Documenter = *fileCfg.Agents.Documenter
		}
	}
}

// Validate checks that all config values are valid.
func (c *Config) Validate() error {
	var errs []error

	if c.MaxReviewIterations < 1 {
		errs = append(errs, errors.New("max_review_iterations must be >= 1"))
	}

	if c.MaxTaskAttempts < 1 {
		errs = append(errs, errors.New("max_task_attempts must be >= 1"))
	}

	if c.Claude.Model == "" {
		errs = append(errs, errors.New("claude.model must be non-empty"))
	}

	if c.Claude.MaxTurns < 1 {
		errs = append(errs, errors.New("claude.max_turns must be >= 1"))
	}

	if c.Claude.MaxBudgetUSD <= 0 {
		errs = append(errs, errors.New("claude.max_budget_usd must be > 0"))
	}

	// Validate agent prompt paths if set.
	if c.Agents.Developer != "" {
		if _, err := os.Stat(c.Agents.Developer); os.IsNotExist(err) {
			errs = append(errs, fmt.Errorf("agents.developer file does not exist: %s", c.Agents.Developer))
		}
	}

	if c.Agents.Reviewer != "" {
		if _, err := os.Stat(c.Agents.Reviewer); os.IsNotExist(err) {
			errs = append(errs, fmt.Errorf("agents.reviewer file does not exist: %s", c.Agents.Reviewer))
		}
	}

	if c.Agents.Planner != "" {
		if _, err := os.Stat(c.Agents.Planner); os.IsNotExist(err) {
			errs = append(errs, fmt.Errorf("agents.planner file does not exist: %s", c.Agents.Planner))
		}
	}

	if c.Agents.Documenter != "" {
		if _, err := os.Stat(c.Agents.Documenter); os.IsNotExist(err) {
			errs = append(errs, fmt.Errorf("agents.documenter file does not exist: %s", c.Agents.Documenter))
		}
	}

	if len(errs) > 0 {
		return errors.Join(errs...)
	}

	return nil
}

// ExpandPaths expands ~ to home directory in all path fields.
func (c *Config) ExpandPaths() error {
	if c.expandedPaths {
		return nil
	}

	var err error

	c.DatabasePath, err = expandPath(c.DatabasePath)
	if err != nil {
		return fmt.Errorf("failed to expand database_path: %w", err)
	}

	c.ProjectsDir, err = expandPath(c.ProjectsDir)
	if err != nil {
		return fmt.Errorf("failed to expand projects_dir: %w", err)
	}

	if c.Agents.Developer != "" {
		c.Agents.Developer, err = expandPath(c.Agents.Developer)
		if err != nil {
			return fmt.Errorf("failed to expand agents.developer: %w", err)
		}
	}

	if c.Agents.Reviewer != "" {
		c.Agents.Reviewer, err = expandPath(c.Agents.Reviewer)
		if err != nil {
			return fmt.Errorf("failed to expand agents.reviewer: %w", err)
		}
	}

	if c.Agents.Planner != "" {
		c.Agents.Planner, err = expandPath(c.Agents.Planner)
		if err != nil {
			return fmt.Errorf("failed to expand agents.planner: %w", err)
		}
	}

	if c.Agents.Documenter != "" {
		c.Agents.Documenter, err = expandPath(c.Agents.Documenter)
		if err != nil {
			return fmt.Errorf("failed to expand agents.documenter: %w", err)
		}
	}

	c.expandedPaths = true
	return nil
}

// GetDatabasePath returns the expanded database path.
// Deprecated: Use GetProjectsDir and per-project databases instead.
func (c *Config) GetDatabasePath() string {
	return c.DatabasePath
}

// GetProjectsDir returns the expanded projects directory path.
func (c *Config) GetProjectsDir() string {
	return c.ProjectsDir
}

// GetAgentPrompt returns the prompt for an agent type.
// If a custom path is set, reads from file. Otherwise returns empty string,
// signaling that the caller should use the embedded default.
func (c *Config) GetAgentPrompt(agentType string) (string, error) {
	var customPath string

	switch agentType {
	case "developer":
		customPath = c.Agents.Developer
	case "reviewer":
		customPath = c.Agents.Reviewer
	case "planner":
		customPath = c.Agents.Planner
	case "documenter":
		customPath = c.Agents.Documenter
	default:
		return "", fmt.Errorf("unknown agent type: %s", agentType)
	}

	// If custom path is set, read from file.
	if customPath != "" {
		data, err := os.ReadFile(customPath)
		if err != nil {
			return "", fmt.Errorf("failed to read custom prompt file: %w", err)
		}
		return string(data), nil
	}

	// Return empty string - caller should use embedded default.
	return "", nil
}

// expandPath expands ~ to the user's home directory.
// It also handles relative paths by making them absolute.
func expandPath(path string) (string, error) {
	if path == "" {
		return "", nil
	}

	// Expand ~
	if strings.HasPrefix(path, "~") {
		home, err := os.UserHomeDir()
		if err != nil {
			return "", fmt.Errorf("failed to get home directory: %w", err)
		}
		path = filepath.Join(home, path[1:])
	}

	// Clean the path.
	return filepath.Clean(path), nil
}
