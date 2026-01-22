package distill

import (
	"context"
	"fmt"
	"os/exec"
	"strings"
	"testing"
	"time"

	"github.com/gerund/ralph/internal/claude"
)

// =============================================================================
// Distiller Creation Tests
// =============================================================================

func TestNewDistiller(t *testing.T) {
	client := claude.NewClient(claude.ClientConfig{
		Model:    "haiku",
		MaxTurns: 1,
	})

	d := NewDistiller(client)

	if d == nil {
		t.Fatal("NewDistiller() returned nil")
	}
	if d.client == nil {
		t.Error("NewDistiller().client is nil")
	}
}

func TestNewDistillerWithDefaults(t *testing.T) {
	d := NewDistillerWithDefaults()

	if d == nil {
		t.Fatal("NewDistillerWithDefaults() returned nil")
	}
	if d.client == nil {
		t.Error("NewDistillerWithDefaults().client is nil")
	}
}

// =============================================================================
// Distill Tests - Special Cases
// =============================================================================

func TestDistill_EmptyOutput(t *testing.T) {
	d := newMockedDistiller("")

	tests := []struct {
		name   string
		output string
		diff   string
	}{
		{"empty string and empty diff", "", ""},
		{"whitespace only", "   ", "   "},
		{"newlines only", "\n\n", "\n\n"},
		{"tabs and spaces", "\t  \n  \t", "\t  \n  \t"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx := context.Background()
			msg, err := d.Distill(ctx, tt.output, tt.diff)

			if err != nil {
				t.Errorf("Distill() returned error: %v", err)
			}
			if msg != FallbackMessage {
				t.Errorf("Distill(%q, %q) = %q, want %q", tt.output, tt.diff, msg, FallbackMessage)
			}
		})
	}
}

func TestDistill_DoneMarker(t *testing.T) {
	d := newMockedDistiller("")

	tests := []struct {
		name  string
		input string
		want  string
	}{
		{"exact done marker", "DONE DONE DONE!!!", "Complete implementation"},
		{"done with leading space", "  DONE DONE DONE!!!", "Complete implementation"},
		{"done with trailing space", "DONE DONE DONE!!!  ", "Complete implementation"},
		{"done with both spaces", "  DONE DONE DONE!!!  ", "Complete implementation"},
		{"done with surrounding text", "Some intro.\n\nDONE DONE DONE!!!\n\nTrailing notes.", "Complete implementation"},
		{"done with prefix text", "prefix DONE DONE DONE!!!", "Complete implementation"},
		{"done with suffix text", "DONE DONE DONE!!! extra", "Complete implementation"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx := context.Background()
			msg, err := d.Distill(ctx, tt.input, "some diff")

			if err != nil {
				t.Errorf("Distill() returned error: %v", err)
			}
			if msg != tt.want {
				t.Errorf("Distill(%q) = %q, want %q", tt.input, msg, tt.want)
			}
		})
	}
}

// =============================================================================
// Distill Tests - Normal Output
// =============================================================================

func TestDistill_NormalOutput(t *testing.T) {
	// Mock Claude returning a commit message
	d := newMockedDistiller("feat: add user authentication")

	ctx := context.Background()
	sessionOutput := `## Progress
- Added login form component
- Implemented JWT token validation

## Learnings
- React hooks work well for form state`

	diff := `diff --git a/login.go b/login.go
+func Login(username, password string) error {
+    // JWT validation logic
+}`

	msg, err := d.Distill(ctx, sessionOutput, diff)
	if err != nil {
		t.Errorf("Distill() returned error: %v", err)
	}

	if msg != "feat: add user authentication" {
		t.Errorf("Distill() = %q, want %q", msg, "feat: add user authentication")
	}
}

func TestDistill_MultilineResponse(t *testing.T) {
	// Mock Claude returning a multi-line message (only first line should be used)
	d := newMockedDistiller("fix: resolve database connection issue\n\nThis fixes the timeout problem with pooling.")

	ctx := context.Background()
	msg, err := d.Distill(ctx, "Fixed the database issue", "diff --git a/db.go b/db.go")

	if err != nil {
		t.Errorf("Distill() returned error: %v", err)
	}

	// Should only return the first line
	if msg != "fix: resolve database connection issue" {
		t.Errorf("Distill() = %q, want first line only", msg)
	}
}

func TestDistill_ResponseWithWhitespace(t *testing.T) {
	// Mock Claude returning a message with extra whitespace
	d := newMockedDistiller("  refactor: clean up error handling  \n")

	ctx := context.Background()
	msg, err := d.Distill(ctx, "Cleaned up the code", "diff --git a/main.go b/main.go")

	if err != nil {
		t.Errorf("Distill() returned error: %v", err)
	}

	if msg != "refactor: clean up error handling" {
		t.Errorf("Distill() = %q, want trimmed message", msg)
	}
}

func TestDistill_EmptyResponse(t *testing.T) {
	// Mock Claude returning empty response
	d := newMockedDistiller("")

	ctx := context.Background()
	msg, err := d.Distill(ctx, "Some progress was made", "diff --git a/file.go b/file.go")

	if err != nil {
		t.Errorf("Distill() returned error: %v", err)
	}

	if msg != FallbackMessage {
		t.Errorf("Distill() = %q, want %q", msg, FallbackMessage)
	}
}

// =============================================================================
// Distill Tests - Error Handling
// =============================================================================

func TestDistill_ContextCancellation(t *testing.T) {
	// Create a distiller with a slow response
	d := newMockedDistillerWithSlowResponse("test message", 5*time.Second)

	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()

	msg, err := d.Distill(ctx, "Some work done", "diff --git a/file.go b/file.go")

	// Should return fallback message on timeout/cancellation
	if msg != FallbackMessage && msg != "test message" {
		t.Errorf("Distill() = %q, expected fallback or partial message", msg)
	}
	// Error is expected but not required (context cancellation may or may not propagate)
	_ = err
}

// =============================================================================
// cleanMessage Tests
// =============================================================================

func TestCleanMessage(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  string
	}{
		{"simple message", "feat: add feature", "feat: add feature"},
		{"leading whitespace", "  feat: add feature", "feat: add feature"},
		{"trailing whitespace", "feat: add feature  ", "feat: add feature"},
		{"both whitespace", "  feat: add feature  ", "feat: add feature"},
		{"multiline takes first", "feat: add feature\n\nmore details", "feat: add feature"},
		{"empty lines before content", "\n\nfeat: add feature", "feat: add feature"},
		{"empty string", "", ""},
		{"whitespace only", "   ", ""},
		{"newlines only", "\n\n\n", ""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := cleanMessage(tt.input)
			if got != tt.want {
				t.Errorf("cleanMessage(%q) = %q, want %q", tt.input, got, tt.want)
			}
		})
	}
}

// =============================================================================
// Test Helpers
// =============================================================================

// newMockedDistiller creates a Distiller with a mocked Claude client.
// The mock will produce no output (useful for testing special case handling).
func newMockedDistiller(output string) *Distiller {
	client := claude.NewClient(claude.ClientConfig{
		Model:    "haiku",
		MaxTurns: 1,
	})

	// Set up mock that returns valid JSONL with the given message
	jsonl := buildMockJSONL(output)
	client.SetCommandCreator(func(ctx context.Context, name string, args ...string) *exec.Cmd {
		return exec.CommandContext(ctx, "echo", "-n", jsonl)
	})

	return NewDistiller(client)
}

// newMockedDistillerWithSlowResponse creates a Distiller with a delayed response.
func newMockedDistillerWithSlowResponse(response string, delay time.Duration) *Distiller {
	client := claude.NewClient(claude.ClientConfig{
		Model:    "haiku",
		MaxTurns: 1,
	})

	// Build the JSON output
	jsonl := buildMockJSONL(response)

	// Format delay as seconds with one decimal place
	delaySeconds := fmt.Sprintf("%.1f", delay.Seconds())

	// Escape single quotes in jsonl for shell
	escapedJSONL := strings.ReplaceAll(jsonl, "'", "'\"'\"'")

	client.SetCommandCreator(func(ctx context.Context, name string, args ...string) *exec.Cmd {
		// Use bash to sleep then echo the JSON output
		return exec.CommandContext(ctx, "bash", "-c",
			fmt.Sprintf("sleep %s && printf '%%s' '%s'", delaySeconds, escapedJSONL))
	})

	return NewDistiller(client)
}

// buildMockJSONL creates valid Claude stream-JSON output with a message.
func buildMockJSONL(messageText string) string {
	if messageText == "" {
		// Return just init and result events (no message)
		return `{"type":"init","session_id":"test-session"}
{"type":"result","session_id":"test-session","cost_usd":0.001}`
	}

	// Escape the message text for JSON
	escapedText := strings.ReplaceAll(messageText, `"`, `\"`)
	escapedText = strings.ReplaceAll(escapedText, "\n", `\n`)

	return `{"type":"init","session_id":"test-session"}
{"message":{"id":"msg1","role":"assistant","content":[{"type":"text","text":"` + escapedText + `"}]}}
{"type":"result","session_id":"test-session","cost_usd":0.001}`
}
