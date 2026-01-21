# Task 11: CLI with New/Resume Modes

## Objective

Create the CLI entry point using Cobra with support for new plan execution and resume.

## Requirements

1. `ralph /path/to/plan.md` - Start new execution
2. `ralph -r <plan-id>` or `ralph --resume <plan-id>` - Resume existing plan
3. `--max-iterations` flag to override config
4. Validate inputs before starting
5. Clear error messages

## Commands

```bash
# New plan execution
ralph /path/to/plan.md
ralph /path/to/plan.md --max-iterations 30

# Resume existing plan
ralph -r abc123
ralph --resume abc123
ralph -r abc123 --max-iterations 50
```

## Implementation

```go
func main() {
    if err := run(); err != nil {
        fmt.Fprintf(os.Stderr, "error: %v\n", err)
        os.Exit(1)
    }
}

func run() error {
    var resumeID string
    var maxIterations int

    rootCmd := &cobra.Command{
        Use:   "ralph [plan-file]",
        Short: "Iterative AI development with a single agent loop",
        Long: `Ralph runs an AI agent iteratively against a plan file.
The agent works until it declares completion or hits max iterations.
Each iteration creates a jj commit with the changes.`,
        Args: cobra.MaximumNArgs(1),
        RunE: func(cmd *cobra.Command, args []string) error {
            // Load config
            cfg, err := config.Load()
            if err != nil {
                return fmt.Errorf("failed to load config: %w", err)
            }

            // Override max iterations if flag provided
            if maxIterations > 0 {
                cfg.MaxIterations = maxIterations
            }

            // Determine mode
            if resumeID != "" {
                if len(args) > 0 {
                    return fmt.Errorf("cannot specify both plan file and --resume")
                }
                return runResume(cfg, resumeID)
            }

            if len(args) == 0 {
                return fmt.Errorf("plan file required (or use --resume)")
            }

            return runNew(cfg, args[0])
        },
    }

    rootCmd.Flags().StringVarP(&resumeID, "resume", "r", "",
        "Resume execution of an existing plan by ID")
    rootCmd.Flags().IntVar(&maxIterations, "max-iterations", 0,
        "Override max iterations from config")

    return rootCmd.Execute()
}
```

## Validation

Before starting:
1. Config loads successfully
2. max_iterations is valid (> 0)
3. For new: plan file exists and is readable
4. For resume: plan ID exists in database
5. Working directory is a jj repository

## Error Messages

```
error: config file not found: ~/.config/ralph/config.json
error: max_iterations must be set in config
error: plan file not found: /path/to/plan.md
error: plan not found: abc123
error: not a jj repository (run from within a jj repo)
```

## runNew Flow

```go
func runNew(cfg *config.Config, planPath string) error {
    // 1. Read plan file
    // 2. Open database
    // 3. Create plan record
    // 4. Create app
    // 5. Run app
}
```

## runResume Flow

```go
func runResume(cfg *config.Config, planID string) error {
    // 1. Open database
    // 2. Load plan record
    // 3. Verify plan exists and isn't already completed
    // 4. Create app
    // 5. Resume app
}
```

## Acceptance Criteria

- [ ] `ralph plan.md` starts new execution
- [ ] `ralph -r <id>` resumes existing plan
- [ ] `--max-iterations` overrides config
- [ ] Clear error if no plan file and no --resume
- [ ] Clear error if both plan file and --resume
- [ ] Clear error for missing config
- [ ] Clear error for missing plan file
- [ ] Clear error for invalid plan ID
- [ ] Clear error for non-jj directory
- [ ] Help text is useful
- [ ] Exit codes are correct (0 success, 1 error)

## Files to Create/Modify

- `cmd/ralph/main.go` (rewrite for V2)
- `cmd/ralph/main_test.go`

## Notes

Keep it simple. V1 had subcommands (task, feedback). V2 is just the main command with flags.

Consider adding a `ralph list` command later to show all plans in the database, but not for MVP.
