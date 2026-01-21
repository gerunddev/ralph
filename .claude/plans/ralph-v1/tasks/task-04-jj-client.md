# Task 4: JJ Client Implementation

## Context

Ralph uses Jujutsu (jj) for version control. Each task gets its own jj change, and the reviewer sees diffs via `jj show`. The stub file exists at `internal/jj/jj.go`.

## Objective

Implement a fully functional Jujutsu CLI wrapper that executes jj commands and returns their output.

## Acceptance Criteria

- [ ] `NewClient(workDir string)` creates client bound to a working directory
- [ ] `NewChange(ctx, description)` runs `jj new -m "description"` and returns change ID
- [ ] `Show(ctx)` runs `jj show` and returns the diff output
- [ ] `Describe(ctx, description)` runs `jj describe -m "description"`
- [ ] `CurrentChangeID(ctx)` returns the current change ID
- [ ] All methods handle errors appropriately (command not found, not a jj repo, etc.)
- [ ] Context cancellation stops running commands
- [ ] Unit tests with mock exec (don't require actual jj installation)
- [ ] Integration test (skipped if jj not installed) that creates a temp repo

## Implementation Details

### Client Structure

```go
type Client struct {
    workDir string
}

func NewClient(workDir string) *Client {
    return &Client{workDir: workDir}
}
```

### Command Execution Pattern

Use `exec.CommandContext` for all jj commands:

```go
func (c *Client) runCommand(ctx context.Context, args ...string) (string, error) {
    cmd := exec.CommandContext(ctx, "jj", args...)
    cmd.Dir = c.workDir

    var stdout, stderr bytes.Buffer
    cmd.Stdout = &stdout
    cmd.Stderr = &stderr

    if err := cmd.Run(); err != nil {
        return "", fmt.Errorf("jj %s failed: %s: %w", args[0], stderr.String(), err)
    }

    return stdout.String(), nil
}
```

### Getting Change ID

After `jj new`, parse the output or run `jj log -r @ -T 'change_id'` to get the change ID.

### Error Types

Create specific error types:
- `ErrNotRepo` - not inside a jj repository
- `ErrCommandNotFound` - jj binary not found

## Files to Modify

- `internal/jj/jj.go` - Full implementation
- `internal/jj/jj_test.go` - Create with unit tests

## Testing Strategy

1. **Unit tests** - Mock `exec.Command` using a test helper that replaces the command runner
2. **Integration test** - Skip if `jj` not in PATH; create temp directory, `jj git init`, run operations

## Dependencies

None - this is a leaf package.

## Notes

- The README mentions `jj new` should create a change with "a brief description of the task (a single line of text)"
- We'll use this to track which jj change corresponds to which Ralph task
- The `jj show` output goes to the reviewer agent for code review
