# Task 7: Output Parser

## Objective

Parse the agent's output to determine if it's done or extract progress/learnings sections.

## Requirements

1. Detect "DONE DONE DONE!!!" (exact match, entire output)
2. Extract "## Progress" section content
3. Extract "## Learnings" section content
4. Handle malformed output gracefully

## Output Types

```go
type ParseResult struct {
    IsDone    bool
    Progress  string // Empty if done or not found
    Learnings string // Empty if done or not found
    Raw       string // Original output
}
```

## Parsing Rules

1. **Done Detection**:
   - Trim whitespace from output
   - Check if trimmed output equals exactly "DONE DONE DONE!!!"
   - If yes: `IsDone = true`, ignore everything else

2. **Section Extraction**:
   - Find "## Progress" header (case-insensitive)
   - Extract content until next "##" header or end
   - Find "## Learnings" header (case-insensitive)
   - Extract content until next "##" header or end
   - Trim whitespace from extracted content

3. **Malformed Output**:
   - If neither done nor valid sections found, treat entire output as Progress
   - Log warning but don't error

## Interface

```go
func Parse(output string) *ParseResult
```

## Examples

```go
// Done case
Parse("DONE DONE DONE!!!")
// => {IsDone: true, Progress: "", Learnings: "", Raw: "DONE DONE DONE!!!"}

// Normal case
Parse(`
## Progress
Built the user authentication module.
Added tests for login flow.

## Learnings
The codebase uses JWT tokens stored in httpOnly cookies.
Found that the User model is in internal/models/user.go.
`)
// => {IsDone: false, Progress: "Built the user...", Learnings: "The codebase uses..."}

// Malformed (no headers)
Parse("I made some changes to the code.")
// => {IsDone: false, Progress: "I made some changes...", Learnings: ""}
```

## Acceptance Criteria

- [ ] Correctly detects done state (exact match)
- [ ] Extracts Progress section content
- [ ] Extracts Learnings section content
- [ ] Handles output with only Progress (no Learnings)
- [ ] Handles output with only Learnings (no Progress)
- [ ] Handles malformed output gracefully
- [ ] Case-insensitive header matching
- [ ] Trims whitespace appropriately
- [ ] Comprehensive unit tests with edge cases

## Files to Create

- `internal/parser/parser.go`
- `internal/parser/parser_test.go`

## Notes

The parser should be lenient - we don't want to fail an iteration just because the agent formatted output slightly wrong. Log warnings for unexpected formats but extract what we can.
