# Task 1: Configuration System

## Objective

Create a configuration system that loads settings from `~/.config/ralph/config.json` with required fields and sensible defaults.

## Requirements

1. Config file location: `~/.config/ralph/config.json`
2. Required field: `max_iterations` (error if missing)
3. Optional fields with defaults:
   - `claude.model` (default: empty, uses Claude CLI default)
   - `claude.max_turns` (default: 0, unlimited)
   - `claude.verbose` (default: true)
4. Create config directory if it doesn't exist
5. Clear error messages for missing/invalid config

## Config Schema

```json
{
  "max_iterations": 20,
  "claude": {
    "model": "claude-sonnet-4-20250514",
    "max_turns": 0,
    "verbose": true
  }
}
```

## Interface

```go
type Config struct {
    MaxIterations int
    Claude        ClaudeConfig
}

type ClaudeConfig struct {
    Model    string
    MaxTurns int
    Verbose  bool
}

func Load() (*Config, error)
func (c *Config) Validate() error
```

## Acceptance Criteria

- [ ] Loads config from `~/.config/ralph/config.json`
- [ ] Returns clear error if file doesn't exist
- [ ] Returns clear error if `max_iterations` is missing or <= 0
- [ ] Applies defaults for optional fields
- [ ] Expands `~` in path correctly
- [ ] Unit tests for all scenarios

## Files to Create/Modify

- `internal/config/config.go` (rewrite from V1)
- `internal/config/config_test.go`

## Notes

V1 has a config system but it's more complex (agent prompts, project paths). This is a simplified version focused on the new requirements.
