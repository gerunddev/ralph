// Package jj provides a wrapper for the Jujutsu (jj) CLI for version control operations.
package jj

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"os/exec"
	"strings"
)

// Error types for jj operations.
var (
	// ErrNotRepo is returned when the working directory is not inside a jj repository.
	ErrNotRepo = errors.New("not a jj repository")
	// ErrCommandNotFound is returned when the jj binary is not found in PATH.
	ErrCommandNotFound = errors.New("jj command not found")
)

// CommandRunner is the function type used to execute commands.
// It can be replaced in tests to mock command execution.
type CommandRunner func(ctx context.Context, dir string, name string, args ...string) (string, string, error)

// defaultCommandRunner executes a command using exec.CommandContext.
func defaultCommandRunner(ctx context.Context, dir string, name string, args ...string) (string, string, error) {
	cmd := exec.CommandContext(ctx, name, args...)
	cmd.Dir = dir

	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	err := cmd.Run()
	return stdout.String(), stderr.String(), err
}

// Client wraps the jj CLI for version control operations.
type Client struct {
	workDir       string
	commandRunner CommandRunner
}

// NewClient creates a new jj CLI client bound to the specified working directory.
func NewClient(workDir string) *Client {
	return &Client{
		workDir:       workDir,
		commandRunner: defaultCommandRunner,
	}
}

// SetCommandRunner allows setting a custom command runner (for testing).
func (c *Client) SetCommandRunner(runner CommandRunner) {
	c.commandRunner = runner
}

// runCommand executes a jj command and returns the output.
func (c *Client) runCommand(ctx context.Context, args ...string) (string, error) {
	stdout, stderr, err := c.commandRunner(ctx, c.workDir, "jj", args...)
	if err != nil {
		return "", c.wrapError(args[0], stderr, err)
	}
	return stdout, nil
}

// wrapError converts exec errors into appropriate jj error types.
func (c *Client) wrapError(subCommand string, stderr string, err error) error {
	// Check for command not found
	var execErr *exec.Error
	if errors.As(err, &execErr) {
		if errors.Is(execErr.Err, exec.ErrNotFound) {
			return ErrCommandNotFound
		}
	}

	// Check for context cancellation
	if errors.Is(err, context.Canceled) {
		return context.Canceled
	}
	if errors.Is(err, context.DeadlineExceeded) {
		return context.DeadlineExceeded
	}

	// Check for not a repository error in stderr
	stderrLower := strings.ToLower(stderr)
	if strings.Contains(stderrLower, "there is no jj repo") ||
		strings.Contains(stderrLower, "not a jj repository") ||
		strings.Contains(stderrLower, "no such repo") {
		return ErrNotRepo
	}

	// Generic error with context
	return fmt.Errorf("jj %s failed: %s: %w", subCommand, strings.TrimSpace(stderr), err)
}

// NewChange creates a new jj change with the given description.
// It returns the change ID of the newly created change.
func (c *Client) NewChange(ctx context.Context, description string) (string, error) {
	_, err := c.runCommand(ctx, "new", "-m", description)
	if err != nil {
		return "", err
	}

	// Get the change ID of the current change (which is the one we just created)
	return c.CurrentChangeID(ctx)
}

// Show returns the diff for the current change.
func (c *Client) Show(ctx context.Context) (string, error) {
	return c.runCommand(ctx, "show")
}

// Describe updates the description of the current change.
func (c *Client) Describe(ctx context.Context, description string) error {
	_, err := c.runCommand(ctx, "describe", "-m", description)
	return err
}

// CurrentChangeID returns the change ID of the current working copy change.
func (c *Client) CurrentChangeID(ctx context.Context) (string, error) {
	// Use jj log with a template to get just the change ID
	// -r @ selects the current working copy change
	// -T 'change_id' outputs only the change ID
	output, err := c.runCommand(ctx, "log", "-r", "@", "-T", "change_id", "--no-graph")
	if err != nil {
		return "", err
	}

	changeID := strings.TrimSpace(output)
	if changeID == "" {
		return "", fmt.Errorf("unable to determine current change ID")
	}

	return changeID, nil
}

// Status returns the status of the working copy.
func (c *Client) Status(ctx context.Context) (string, error) {
	return c.runCommand(ctx, "status")
}

// Diff returns the diff of the current change compared to its parent.
func (c *Client) Diff(ctx context.Context) (string, error) {
	return c.runCommand(ctx, "diff")
}

// Log returns the log output with the specified revset and optional template.
func (c *Client) Log(ctx context.Context, revset string, template string) (string, error) {
	args := []string{"log"}
	if revset != "" {
		args = append(args, "-r", revset)
	}
	if template != "" {
		args = append(args, "-T", template)
	}
	return c.runCommand(ctx, args...)
}
