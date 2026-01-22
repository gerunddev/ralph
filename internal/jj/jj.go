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

// sanitizeMessage removes or escapes characters that could cause issues in commit messages.
// This prevents potential issues with shell interpretation or jj's message parsing.
func sanitizeMessage(message string) string {
	// Replace null bytes (could terminate strings early)
	message = strings.ReplaceAll(message, "\x00", "")

	// Trim leading/trailing whitespace
	message = strings.TrimSpace(message)

	return message
}

// New creates a new jj change (working copy commit).
// This is typically called before starting a new iteration of work.
func (c *Client) New(ctx context.Context) error {
	_, err := c.runCommand(ctx, "new")
	return err
}

// NewChange creates a new jj change with a description and returns the change ID.
// This is used when starting a new task to create a dedicated change for it.
func (c *Client) NewChange(ctx context.Context, description string) (string, error) {
	sanitized := sanitizeMessage(description)
	args := []string{"new", "--no-edit"}
	if sanitized != "" {
		args = append(args, "-m", sanitized)
	}
	_, err := c.runCommand(ctx, args...)
	if err != nil {
		return "", err
	}

	// Get the current change ID
	output, err := c.runCommand(ctx, "log", "-r", "@", "-T", "change_id", "--no-graph")
	if err != nil {
		return "", fmt.Errorf("failed to get change ID: %w", err)
	}
	return strings.TrimSpace(output), nil
}

// Describe sets or updates the description of the current change.
func (c *Client) Describe(ctx context.Context, message string) error {
	sanitized := sanitizeMessage(message)
	if sanitized == "" {
		return errors.New("description cannot be empty")
	}
	_, err := c.runCommand(ctx, "describe", "-m", sanitized)
	return err
}

// Show returns the diff of the current change.
// This shows the changes in the working copy compared to the parent.
func (c *Client) Show(ctx context.Context) (string, error) {
	return c.runCommand(ctx, "show")
}

// Commit commits the current change with the given message.
// The message is sanitized to prevent issues with special characters.
func (c *Client) Commit(ctx context.Context, message string) error {
	sanitized := sanitizeMessage(message)
	if sanitized == "" {
		return errors.New("commit message cannot be empty")
	}
	_, err := c.runCommand(ctx, "commit", "-m", sanitized)
	return err
}

// Status returns the status of the working copy.
// This is a helper method useful for debugging.
func (c *Client) Status(ctx context.Context) (string, error) {
	return c.runCommand(ctx, "status")
}

// Log returns the log output with the specified revset and template.
// If revset is empty, the default revset is used.
// If template is empty, the default template is used.
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

// IsEmpty returns true if the current change has no file modifications.
// It uses `jj diff` which returns empty output when there are no changes.
func (c *Client) IsEmpty(ctx context.Context) (bool, error) {
	output, err := c.runCommand(ctx, "diff")
	if err != nil {
		return false, err
	}
	return strings.TrimSpace(output) == "", nil
}

// Diff returns the diff between two revisions.
// If from is empty, it diffs from the parent of 'to'.
// If to is empty, it defaults to "@" (current change).
func (c *Client) Diff(ctx context.Context, from, to string) (string, error) {
	args := []string{"diff"}
	if from != "" {
		args = append(args, "--from", from)
	}
	if to != "" {
		args = append(args, "--to", to)
	}
	return c.runCommand(ctx, args...)
}

// GetCurrentChangeID returns the change ID of the current revision (@).
func (c *Client) GetCurrentChangeID(ctx context.Context) (string, error) {
	output, err := c.runCommand(ctx, "log", "-r", "@", "-T", "change_id", "--no-graph")
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(output), nil
}

// GetParentChangeID returns the change ID of the parent of the current revision (@-).
// Returns empty string if there is no parent (root commit).
func (c *Client) GetParentChangeID(ctx context.Context) (string, error) {
	output, err := c.runCommand(ctx, "log", "-r", "@-", "-T", "change_id", "--no-graph")
	if err != nil {
		// Check if it's a root commit error (no parent)
		if strings.Contains(err.Error(), "root") || strings.Contains(err.Error(), "empty") {
			return "", nil
		}
		return "", err
	}
	return strings.TrimSpace(output), nil
}
