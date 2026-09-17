package perception

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"codenerd/internal/config"
)

func claudeTestClient() *ClaudeCodeCLIClient {
	return NewClaudeCodeCLIClient(&config.ClaudeCLIConfig{Model: "sonnet-test", Timeout: 30})
}

func readLog(t *testing.T, dir, name string) string {
	t.Helper()
	b, err := os.ReadFile(filepath.Join(dir, name))
	if err != nil {
		t.Fatalf("read %s: %v", name, err)
	}
	return string(b)
}

// argLines returns the argv of the LAST invocation (blocks are --- separated).
func argLines(t *testing.T, dir string) []string {
	t.Helper()
	blocks := strings.Split(strings.TrimSpace(readLog(t, dir, "args.log")), "---")
	for i := len(blocks) - 1; i >= 0; i-- {
		if last := strings.TrimSpace(blocks[i]); last != "" {
			return strings.Split(last, "\n")
		}
	}
	t.Fatal("no argv captured")
	return nil
}

func assertArgPair(t *testing.T, lines []string, flag, want string) {
	t.Helper()
	for i, l := range lines {
		if strings.TrimSpace(l) == flag {
			if i+1 >= len(lines) {
				t.Fatalf("flag %s is last with no value", flag)
			}
			if got := strings.TrimSpace(lines[i+1]); got != want {
				t.Fatalf("flag %s value = %q, want %q", flag, got, want)
			}
			return
		}
	}
	t.Fatalf("flag %s not found in argv:\n%s", flag, strings.Join(lines, "\n"))
}

const claudeOKPayload = `{"type":"result","subtype":"success","result":"  hello cli  ","usage":{"input_tokens":5,"output_tokens":7}}`

// End to end through a fake binary: the prompt travels on stdin (never in
// argv — the injection property), -p selects print mode, tools stay
// disabled, and the JSON result parses.
func TestClaudeCLI_ArgvContractAndCompletion(t *testing.T) {
	dir := installFakeCLI(t, "claude")
	t.Setenv("PATH", prependPath(dir, os.Getenv("PATH")))
	t.Setenv("CLAUDE_TEST_MODE", "ok")
	t.Setenv("CLAUDE_TEST_PAYLOAD", claudeOKPayload)

	resp, err := claudeTestClient().CompleteWithSystem(context.Background(), "sys-prompt", "user-prompt")
	if err != nil {
		t.Fatalf("CompleteWithSystem: %v", err)
	}
	if resp != "hello cli" {
		t.Errorf("response = %q, want trimmed result", resp)
	}

	lines := argLines(t, dir)
	foundP := false
	for _, l := range lines {
		if strings.TrimSpace(l) == "-p" {
			foundP = true
		}
		if strings.Contains(l, "user-prompt") {
			t.Fatalf("prompt leaked into argv (injection surface):\n%s", strings.Join(lines, "\n"))
		}
	}
	if !foundP {
		t.Errorf("argv lacks -p (print mode is required):\n%s", strings.Join(lines, "\n"))
	}
	assertArgPair(t, lines, "--max-turns", "1")
	assertArgPair(t, lines, "--model", "sonnet-test")
	assertArgPair(t, lines, "--output-format", "json")
	assertArgPair(t, lines, "--system-prompt", "sys-prompt")
	// --tools "" : the value line must be present and empty.
	toolsOK := false
	for i, l := range lines {
		if strings.TrimSpace(l) == "--tools" && i+1 < len(lines) && strings.TrimSpace(lines[i+1]) == "" {
			toolsOK = true
		}
	}
	if !toolsOK {
		t.Errorf("argv lacks --tools with an empty value (tools must be disabled):\n%s", strings.Join(lines, "\n"))
	}
	if stdin := readLog(t, dir, "stdin.log"); stdin != "user-prompt" {
		t.Errorf("stdin = %q, want the prompt", stdin)
	}
}

// Schema mode bumps max-turns to 3, passes the schema through, keeps tools
// enabled for the internal schema call, and returns structured_output raw.
func TestClaudeCLI_SchemaMode(t *testing.T) {
	dir := installFakeCLI(t, "claude")
	t.Setenv("PATH", prependPath(dir, os.Getenv("PATH")))
	t.Setenv("CLAUDE_TEST_MODE", "ok")
	t.Setenv("CLAUDE_TEST_PAYLOAD", `{"type":"result","subtype":"success","result":{},"structured_output":{"surface_response":"hi"}}`)

	schema := `{"type":"object"}`
	resp, err := claudeTestClient().CompleteWithSchema(context.Background(), "sys", "user", schema)
	if err != nil {
		t.Fatalf("CompleteWithSchema: %v", err)
	}
	if resp != `{"surface_response":"hi"}` {
		t.Errorf("response = %q, want raw structured_output", resp)
	}

	lines := argLines(t, dir)
	assertArgPair(t, lines, "--max-turns", "3")
	assertArgPair(t, lines, "--json-schema", schema)
	for _, l := range lines {
		if strings.TrimSpace(l) == "--tools" {
			t.Fatalf("schema mode must not pass --tools (the schema call needs it):\n%s", strings.Join(lines, "\n"))
		}
	}
}

// A rate-limited primary fails over to the fallback model exactly once;
// without a fallback the typed RateLimitError surfaces for errors.As.
func TestClaudeCLI_FallbackOnRateLimit(t *testing.T) {
	dir := installFakeCLI(t, "claude")
	t.Setenv("PATH", prependPath(dir, os.Getenv("PATH")))
	t.Setenv("CLAUDE_TEST_MODE", "fallback")
	t.Setenv("CLAUDE_TEST_PAYLOAD", claudeOKPayload)

	client := NewClaudeCodeCLIClient(&config.ClaudeCLIConfig{Model: "primary", FallbackModel: "fb-model", Timeout: 30})
	resp, err := client.CompleteWithSystem(context.Background(), "sys", "user")
	if err != nil {
		t.Fatalf("CompleteWithSystem with fallback: %v", err)
	}
	if resp != "hello cli" {
		t.Errorf("response = %q, want fallback result", resp)
	}
	calls := strings.Count(strings.TrimSpace(readLog(t, dir, "calls.log")), "\n") + 1
	if calls != 2 {
		t.Errorf("invocations = %d, want 2 (primary + fallback)", calls)
	}
	argsLog := readLog(t, dir, "args.log")
	if !strings.Contains(argsLog, "primary") || !strings.Contains(argsLog, "fb-model") {
		t.Errorf("want one call per model in:\n%s", argsLog)
	}
}

func TestClaudeCLI_RateLimitErrorWithoutFallback(t *testing.T) {
	dir := installFakeCLI(t, "claude")
	t.Setenv("PATH", prependPath(dir, os.Getenv("PATH")))
	t.Setenv("CLAUDE_TEST_MODE", "rate_limit")

	_, err := claudeTestClient().CompleteWithSystem(context.Background(), "sys", "user")
	var rlErr *RateLimitError
	if !errors.As(err, &rlErr) {
		t.Fatalf("err = %v (%T), want *RateLimitError", err, err)
	}
	if rlErr.Provider != "claude-cli" {
		t.Errorf("provider = %q, want claude-cli", rlErr.Provider)
	}
}

// A single stream line larger than bufio's 64KB default must still parse:
// the pooled 1MB scanner matches every other stream parser here.
func TestClaudeCLI_StreamingLongLine(t *testing.T) {
	dir := installFakeCLI(t, "claude")
	t.Setenv("PATH", prependPath(dir, os.Getenv("PATH")))
	t.Setenv("CLAUDE_TEST_MODE", "stream_long")

	var sb strings.Builder
	err := claudeTestClient().CompleteStreaming(context.Background(), "sys", "user", func(chunk StreamChunk) error {
		sb.WriteString(chunk.Text)
		sb.WriteString(chunk.Content)
		return nil
	})
	if err != nil {
		t.Fatalf("CompleteStreaming: %v", err)
	}
	if sb.Len() != 102400 {
		t.Errorf("streamed bytes = %d, want 102400", sb.Len())
	}
}

// A cancelled context fails fast instead of running the CLI to completion.
func TestClaudeCLI_CancelledContextFailsFast(t *testing.T) {
	dir := installFakeCLI(t, "claude")
	t.Setenv("PATH", prependPath(dir, os.Getenv("PATH")))
	t.Setenv("CLAUDE_TEST_MODE", "ok")
	t.Setenv("CLAUDE_TEST_PAYLOAD", claudeOKPayload)

	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	start := time.Now()
	_, err := claudeTestClient().CompleteWithSystem(ctx, "sys", "user")
	elapsed := time.Since(start)
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("err = %v, want wrapped context.Canceled", err)
	}
	if elapsed > 10*time.Second {
		t.Errorf("cancelled call took %v, want fast failure", elapsed)
	}
}

// An empty system prompt gets codeNERD's default like every API engine,
// rather than leaving the CLI on its own agentic instructions.
func TestClaudeCLI_EmptySystemGetsDefault(t *testing.T) {
	dir := installFakeCLI(t, "claude")
	t.Setenv("PATH", prependPath(dir, os.Getenv("PATH")))
	t.Setenv("CLAUDE_TEST_MODE", "ok")
	t.Setenv("CLAUDE_TEST_PAYLOAD", claudeOKPayload)

	if _, err := claudeTestClient().Complete(context.Background(), "hi"); err != nil {
		t.Fatalf("Complete: %v", err)
	}
	argsLog := readLog(t, dir, "args.log")
	if !strings.Contains(argsLog, "--system-prompt") {
		t.Fatalf("argv lacks --system-prompt:\n%s", argsLog)
	}
	if !strings.Contains(argsLog, defaultSystemPrompt) {
		t.Error("argv --system-prompt is not codeNERD's default")
	}
}
