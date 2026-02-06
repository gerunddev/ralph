// Package claude provides a wrapper for the Claude CLI and handles streaming output.
package claude

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"strconv"
	"sync"
)

// Error types for Claude operations.
var (
	// ErrCommandNotFound is returned when the claude binary is not found in PATH.
	ErrCommandNotFound = errors.New("claude command not found")
	// ErrSessionCanceled is returned when a session is canceled via context.
	ErrSessionCanceled = errors.New("session canceled")
)

// ClientConfig holds configuration for the Claude client.
type ClientConfig struct {
	Model    string
	MaxTurns int
	Verbose  bool     // Enable verbose output from Claude CLI
	EnvVars  []string // Additional environment variables (KEY=VALUE format)
}

// Client wraps the Claude CLI for executing agent sessions.
type Client struct {
	model    string
	maxTurns int
	verbose  bool
	envVars  []string // Additional environment variables

	// CommandRunner allows overriding command creation for testing.
	// When set, it's called to create the exec.Cmd instead of the default.
	commandCreator CommandCreator
}

// CommandCreator is a function type for creating exec.Cmd instances.
// It allows mocking command execution in tests.
type CommandCreator func(ctx context.Context, name string, args ...string) *exec.Cmd

// defaultCommandCreator creates a standard exec.Cmd.
func defaultCommandCreator(ctx context.Context, name string, args ...string) *exec.Cmd {
	return exec.CommandContext(ctx, name, args...)
}

// NewClient creates a new Claude CLI client.
func NewClient(cfg ClientConfig) *Client {
	return &Client{
		model:          cfg.Model,
		maxTurns:       cfg.MaxTurns,
		verbose:        cfg.Verbose,
		envVars:        cfg.EnvVars,
		commandCreator: defaultCommandCreator,
	}
}

// SetCommandCreator sets a custom command creator (for testing).
func (c *Client) SetCommandCreator(creator CommandCreator) {
	c.commandCreator = creator
}

// Session represents an active Claude session.
type Session struct {
	cmd    *exec.Cmd
	stdout io.ReadCloser
	stderr *bytes.Buffer
	parser *Parser

	ctx    context.Context
	events chan StreamEvent
	done   chan struct{}
	err    error
	errMu  sync.Mutex

	cancel context.CancelFunc
}

// Run executes a Claude session with the given prompt.
// It returns a Session handle for streaming events.
func (c *Client) Run(ctx context.Context, prompt string) (*Session, error) {
	// Create a cancelable context
	ctx, cancel := context.WithCancel(ctx)

	// Build the command arguments
	// Note: --verbose is required when using --output-format stream-json with -p (print mode)
	args := []string{
		"-p",
		"--output-format", "stream-json",
		"--verbose",
		"--include-partial-messages", // Stream assistant text as it arrives
	}

	if c.model != "" {
		args = append(args, "--model", c.model)
	}

	if c.maxTurns > 0 {
		args = append(args, "--max-turns", strconv.Itoa(c.maxTurns))
	}

	// Add the prompt as the final argument
	args = append(args, prompt)

	// Create the command
	cmd := c.commandCreator(ctx, "claude", args...)

	// Set additional environment variables if configured
	if len(c.envVars) > 0 {
		cmd.Env = append(os.Environ(), c.envVars...)
	}

	// Set up stdout pipe for streaming
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		cancel()
		return nil, fmt.Errorf("failed to create stdout pipe: %w", err)
	}

	// Capture stderr
	stderr := &bytes.Buffer{}
	cmd.Stderr = stderr

	// Start the command
	if err := cmd.Start(); err != nil {
		cancel()
		// Check for command not found
		var execErr *exec.Error
		if errors.As(err, &execErr) && errors.Is(execErr.Err, exec.ErrNotFound) {
			return nil, ErrCommandNotFound
		}
		return nil, fmt.Errorf("failed to start claude: %w", err)
	}

	session := &Session{
		cmd:    cmd,
		stdout: stdout,
		stderr: stderr,
		parser: NewParser(stdout),
		ctx:    ctx,
		events: make(chan StreamEvent, 1000),
		done:   make(chan struct{}),
		cancel: cancel,
	}

	// Start the event streaming goroutine
	go session.streamEvents()

	return session, nil
}

// streamEvents reads events from the parser and sends them to the events channel.
func (s *Session) streamEvents() {
	defer close(s.done)
	defer close(s.events)

	for {
		event, err := s.parser.Next()
		if err != nil {
			if err == io.EOF {
				// Normal end of stream
				break
			}
			s.setError(fmt.Errorf("parse error: %w", err))
			break
		}

		// Send the event, checking for context cancellation
		select {
		case s.events <- *event:
		case <-s.ctx.Done():
			return
		}
	}

	// Wait for the command to complete
	if err := s.cmd.Wait(); err != nil {
		// Check for context cancellation
		var exitErr *exec.ExitError
		if errors.As(err, &exitErr) {
			// Check if this was due to cancellation
			if s.cmd.ProcessState != nil && !s.cmd.ProcessState.Success() {
				stderrStr := s.stderr.String()
				if stderrStr != "" {
					s.setError(fmt.Errorf("claude exited with error: %s", stderrStr))
				} else {
					s.setError(fmt.Errorf("claude exited with code %d", exitErr.ExitCode()))
				}
			}
		} else {
			s.setError(fmt.Errorf("claude process error: %w", err))
		}
	}
}

// setError sets the session error (thread-safe).
func (s *Session) setError(err error) {
	s.errMu.Lock()
	defer s.errMu.Unlock()
	if s.err == nil {
		s.err = err
	}
}

// Events returns a channel that emits streaming events.
// The channel is closed when the session ends.
func (s *Session) Events() <-chan StreamEvent {
	return s.events
}

// Wait blocks until the session completes and returns any error.
func (s *Session) Wait() error {
	<-s.done
	s.errMu.Lock()
	defer s.errMu.Unlock()
	return s.err
}

// Cancel stops the Claude process.
func (s *Session) Cancel() {
	s.cancel()
}

// Done returns a channel that's closed when the session completes.
func (s *Session) Done() <-chan struct{} {
	return s.done
}

// Err returns the session error, if any.
func (s *Session) Err() error {
	s.errMu.Lock()
	defer s.errMu.Unlock()
	return s.err
}
