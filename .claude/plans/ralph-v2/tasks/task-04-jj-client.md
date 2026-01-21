# Task 4: JJ Client

## Objective

Create a Jujutsu (jj) CLI wrapper for version control operations.

## Requirements

1. `jj new` - Create new change before each iteration
2. `jj commit -m "<message>"` - Commit after each iteration
3. Work in a specified directory
4. Handle errors gracefully (not a repo, command not found)

## Interface

```go
type Client struct { ... }

func NewClient(workDir string) *Client

func (c *Client) New(ctx context.Context) error
func (c *Client) Commit(ctx context.Context, message string) error

// Optional helpers (may be useful)
func (c *Client) Status(ctx context.Context) (string, error)
func (c *Client) Log(ctx context.Context, revset string) (string, error)
```

## Error Types

```go
var (
    ErrNotRepo         = errors.New("not a jj repository")
    ErrCommandNotFound = errors.New("jj command not found")
)
```

## Acceptance Criteria

- [ ] `New()` runs `jj new` in work directory
- [ ] `Commit()` runs `jj commit -m` with sanitized message
- [ ] Detects and returns `ErrNotRepo` appropriately
- [ ] Detects and returns `ErrCommandNotFound` appropriately
- [ ] Context cancellation works
- [ ] Mock-friendly design (CommandRunner pattern from V1)
- [ ] Unit tests with mocked command execution

## Files to Create/Modify

- `internal/jj/jj.go` (simplify from V1)
- `internal/jj/jj_test.go`

## Notes

V1's jj client has more methods (Show, Describe, Diff, CurrentChangeID). V2 only needs `New` and `Commit`. Can keep others if useful for debugging but not required.

Important: Sanitize commit messages to prevent shell injection. Remove or escape problematic characters.
