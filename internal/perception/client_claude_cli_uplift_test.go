package perception

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"

	"codenerd/internal/config"
)

// writeFakeClaude installs an executable named "claude" into a fresh temp dir
// and returns the dir; tests prepend it to PATH. Both a POSIX shell script
// and a Windows .cmd are written (following writeFakeCodex) so the pins run
// on either OS. Behavior is driven by CLAUDE_TEST_MODE:
//
//	ok          print $CLAUDE_TEST_PAYLOAD to stdout, exit 0
//	rate_limit  "rate limit reached" on stderr, exit 1
//	fallback    succeed iff "--model fb-model" is in argv, else rate-limit fail
//	stream_long emit one 100KB text chunk line, then a done chunk (sh only)
//
// Every invocation appends its argv (one arg per line) plus a --- separator
// to args.log, the stdin bytes to stdin.log, and one line to calls.log.
func writeFakeClaude(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()

	sh := "#!/usr/bin/env sh\n" +
		"d=\"$(dirname \"$0\")\"\n" +
		"printf '%s\\n' \"$@\" >> \"$d/args.log\"\n" +
		"echo '---' >> \"$d/args.log\"\n" +
		"cat > \"$d/stdin.log\"\n" +
		"echo called >> \"$d/calls.log\"\n" +
		"case \"$CLAUDE_TEST_MODE\" in\n" +
		"  rate_limit)\n" +
		"    echo \"rate limit reached, try later\" 1>&2\n" +
		"    exit 1\n" +
		"    ;;\n" +
		"  fallback)\n" +
		"    for a in \"$@\"; do\n" +
		"      if [ \"$a\" = \"fb-model\" ]; then\n" +
		"        printf '%s' \"$CLAUDE_TEST_PAYLOAD\"\n" +
		"        exit 0\n" +
		"      fi\n" +
		"    done\n" +
		"    echo \"rate limit reached\" 1>&2\n" +
		"    exit 1\n" +
		"    ;;\n" +
		"  stream_long)\n" +
		"    printf '{\"type\":\"text\",\"text\":\"'\n" +
		"    head -c 102400 /dev/zero | tr '\\0' 'A'\n" +
		"    printf '\"}\\n'\n" +
		"    printf '{\"type\":\"done\",\"done\":true}\\n'\n" +
		"    exit 0\n" +
		"    ;;\n" +
		"esac\n" +
		"printf '%s' \"$CLAUDE_TEST_PAYLOAD\"\n" +
		"exit 0\n"
	if err := os.WriteFile(filepath.Join(dir, "claude"), []byte(sh), 0o755); err != nil {
		t.Fatalf("write fake claude: %v", err)
	}

	cmd := "@echo off\r\n" +
		"setlocal\r\n" +
		"set \"d=%~dp0\"\r\n" +
		":argloop\r\n" +
		"if \"%~1\"==\"\" goto argsdone\r\n" +
		"@echo(%~1>> \"%d%args.log\"\r\n" +
		"shift\r\n" +
		"goto argloop\r\n" +
		":argsdone\r\n" +
		"@echo --->> \"%d%args.log\"\r\n" +
		"more > \"%d%stdin.log\"\r\n" +
		"@echo called>> \"%d%calls.log\"\r\n" +
		"if \"%CLAUDE_TEST_MODE%\"==\"rate_limit\" (\r\n" +
		"  >&2 echo rate limit reached, try later\r\n" +
		"  exit /b 1\r\n" +
		")\r\n" +
		"if \"%CLAUDE_TEST_MODE%\"==\"fallback\" (\r\n" +
		"  for %%a in (%*) do if \"%%a\"==\"fb-model\" goto fbsuccess\r\n" +
		"  >&2 echo rate limit reached\r\n" +
		"  exit /b 1\r\n" +
		"  :fbsuccess\r\n" +
		"  echo %CLAUDE_TEST_PAYLOAD%\r\n" +
		"  exit /b 0\r\n" +
		")\r\n" +
		"echo %CLAUDE_TEST_PAYLOAD%\r\n" +
		"exit /b 0\r\n"
	if err := os.WriteFile(filepath.Join(dir, "claude.cmd"), []byte(cmd), 0o755); err != nil {
		t.Fatalf("write fake claude.cmd: %v", err)
	}
	return dir
}

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
	dir := writeFakeClaude(t)
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
	// Stdin content is asserted on unix only: sh `cat` is exact, while the
	// .cmd stdin capture exists only so the pipe drains.
	if runtime.GOOS != "windows" {
		if stdin := readLog(t, dir, "stdin.log"); stdin != "user-prompt" {
			t.Errorf("stdin = %q, want the prompt", stdin)
		}
	}
}

// Schema mode bumps max-turns to 3, passes the schema through, keeps tools
// enabled for the internal schema call, and returns structured_output raw.
func TestClaudeCLI_SchemaMode(t *testing.T) {
	dir := writeFakeClaude(t)
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
	dir := writeFakeClaude(t)
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
	dir := writeFakeClaude(t)
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
	if runtime.GOOS == "windows" {
		t.Skip("cmd.exe cannot emit a 100KB line; the scanner is platform-independent")
	}
	dir := writeFakeClaude(t)
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
	dir := writeFakeClaude(t)
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
	dir := writeFakeClaude(t)
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
