package perception

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"runtime"
	"strings"
	"testing"
	"time"

	"codenerd/internal/config"
	"codenerd/internal/types"
)

// writeFakeCodexExec installs an executable named "codex" into a fresh temp
// dir and returns the dir; tests prepend it to PATH. Both a POSIX shell
// script and a Windows .cmd are written (following writeFakeCodex) so the
// pins run on either OS. Unlike the probe fake, this one honors the real
// transport protocol: the response goes to the --output-last-message file,
// not stdout. Behavior is driven by CODEX_TEST_MODE:
//
//	ok          write $CODEX_TEST_PAYLOAD to the out file, exit 0
//	rate_limit  "rate limit reached" on stderr, exit 1
//	fallback    succeed iff "--model fb-model" is in argv, else rate-limit fail
//	empty_out   exit 0 with an empty out file + a JSONL agent_message on stdout
//
// Every invocation appends its argv (one arg per line) plus a --- separator
// to args.log, the stdin bytes to stdin.log, one line to calls.log, and a
// copy of the --output-schema file (when passed) to schema_copy.json.
func writeFakeCodexExec(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()

	sh := "#!/usr/bin/env sh\n" +
		"d=\"$(dirname \"$0\")\"\n" +
		"printf '%s\\n' \"$@\" >> \"$d/args.log\"\n" +
		"echo '---' >> \"$d/args.log\"\n" +
		"cat > \"$d/stdin.log\"\n" +
		"echo called >> \"$d/calls.log\"\n" +
		"out=\"\"\n" +
		"schema=\"\"\n" +
		"prev=\"\"\n" +
		"for a in \"$@\"; do\n" +
		"  if [ \"$prev\" = \"--output-last-message\" ]; then out=\"$a\"; fi\n" +
		"  if [ \"$prev\" = \"--output-schema\" ]; then schema=\"$a\"; fi\n" +
		"  prev=\"$a\"\n" +
		"done\n" +
		"if [ -n \"$schema\" ] && [ -f \"$schema\" ]; then cp \"$schema\" \"$d/schema_copy.json\"; fi\n" +
		"case \"$CODEX_TEST_MODE\" in\n" +
		"  rate_limit)\n" +
		"    echo \"rate limit reached\" 1>&2\n" +
		"    exit 1\n" +
		"    ;;\n" +
		"  fallback)\n" +
		"    for a in \"$@\"; do\n" +
		"      if [ \"$a\" = \"fb-model\" ]; then\n" +
		"        printf '%s' \"$CODEX_TEST_PAYLOAD\" > \"$out\"\n" +
		"        exit 0\n" +
		"      fi\n" +
		"    done\n" +
		"    echo \"rate limit reached\" 1>&2\n" +
		"    exit 1\n" +
		"    ;;\n" +
		"  empty_out)\n" +
		"    : > \"$out\"\n" +
		"    printf '{\"type\":\"item.completed\",\"item\":{\"type\":\"agent_message\",\"text\":\"  recovered  \"}}\\n'\n" +
		"    exit 0\n" +
		"    ;;\n" +
		"esac\n" +
		"printf '%s' \"$CODEX_TEST_PAYLOAD\" > \"$out\"\n" +
		"exit 0\n"
	if err := os.WriteFile(filepath.Join(dir, "codex"), []byte(sh), 0o755); err != nil {
		t.Fatalf("write fake codex: %v", err)
	}

	cmd := "@echo off\r\n" +
		"setlocal\r\n" +
		"set \"d=%~dp0\"\r\n" +
		"set \"out=\"\r\n" +
		"set \"schema=\"\r\n" +
		"set \"prev=\"\r\n" +
		":argloop\r\n" +
		"if \"%~1\"==\"\" goto argsdone\r\n" +
		"@echo(%~1>> \"%d%args.log\"\r\n" +
		"if \"%prev%\"==\"--output-last-message\" set \"out=%~1\"\r\n" +
		"if \"%prev%\"==\"--output-schema\" set \"schema=%~1\"\r\n" +
		"set \"prev=%~1\"\r\n" +
		"shift\r\n" +
		"goto argloop\r\n" +
		":argsdone\r\n" +
		"@echo --->> \"%d%args.log\"\r\n" +
		"more > \"%d%stdin.log\"\r\n" +
		"@echo called>> \"%d%calls.log\"\r\n" +
		"if defined schema copy \"%schema%\" \"%d%schema_copy.json\" >nul\r\n" +
		"if \"%CODEX_TEST_MODE%\"==\"rate_limit\" (\r\n" +
		"  >&2 echo rate limit reached\r\n" +
		"  exit /b 1\r\n" +
		")\r\n" +
		"if \"%CODEX_TEST_MODE%\"==\"fallback\" (\r\n" +
		"  for %%a in (%*) do if \"%%a\"==\"fb-model\" goto fbsuccess\r\n" +
		"  >&2 echo rate limit reached\r\n" +
		"  exit /b 1\r\n" +
		"  :fbsuccess\r\n" +
		"  echo %CODEX_TEST_PAYLOAD%> \"%out%\"\r\n" +
		"  exit /b 0\r\n" +
		")\r\n" +
		"if \"%CODEX_TEST_MODE%\"==\"empty_out\" (\r\n" +
		"  type nul > \"%out%\"\r\n" +
		"  echo {\"type\":\"item.completed\",\"item\":{\"type\":\"agent_message\",\"text\":\"  recovered  \"}}\r\n" +
		"  exit /b 0\r\n" +
		")\r\n" +
		"echo %CODEX_TEST_PAYLOAD%> \"%out%\"\r\n" +
		"exit /b 0\r\n"
	if err := os.WriteFile(filepath.Join(dir, "codex.cmd"), []byte(cmd), 0o755); err != nil {
		t.Fatalf("write fake codex.cmd: %v", err)
	}
	return dir
}

func codexTestClient() *CodexCLIClient {
	skillOff := false
	return NewCodexCLIClient(&config.CodexCLIConfig{Model: "gpt-test", Timeout: 30, SkillEnabled: &skillOff})
}

// End to end through a fake binary: exec - with the prompt on stdin (never
// in argv), the read-only/shell-disabled fail-closed flags, and the answer
// read back from the --output-last-message file.
func TestCodexExec_ArgvContractAndCompletion(t *testing.T) {
	dir := writeFakeCodexExec(t)
	t.Setenv("PATH", prependPath(dir, os.Getenv("PATH")))
	t.Setenv("CODEX_TEST_MODE", "ok")
	t.Setenv("CODEX_TEST_PAYLOAD", "  codex says hi  ")

	resp, err := codexTestClient().CompleteWithSystem(context.Background(), "sys-prompt", "user-prompt")
	if err != nil {
		t.Fatalf("CompleteWithSystem: %v", err)
	}
	if resp != "codex says hi" {
		t.Errorf("response = %q, want trimmed out-file content", resp)
	}

	lines := argLines(t, dir)
	joined := strings.Join(lines, "\n")
	for _, want := range []string{"exec", "-", "--sandbox", "read-only", "--color", "never",
		"--output-last-message", "--json", "--disable", "shell_tool"} {
		found := false
		for _, l := range lines {
			if strings.TrimSpace(l) == want {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("argv lacks %q:\n%s", want, joined)
		}
	}
	assertArgPair(t, lines, "--model", "gpt-test")
	if strings.Contains(joined, "user-prompt") {
		t.Fatalf("prompt leaked into argv (injection surface):\n%s", joined)
	}
	// Stdin content is asserted on unix only (see the claude fake).
	if runtime.GOOS != "windows" {
		stdin := readLog(t, dir, "stdin.log")
		for _, want := range []string{"<system_instructions>", "sys-prompt", "user-prompt"} {
			if !strings.Contains(stdin, want) {
				t.Errorf("stdin lacks %q: %q", want, stdin)
			}
		}
	}
}

// With the repo skill present, the prompt is prefixed with the skill
// invocation so codex runs under codeNERD's instructions.
func TestCodexExec_SkillPrefix(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("stdin content asserted on unix only (see the claude fake)")
	}
	dir := writeFakeCodexExec(t)
	t.Setenv("PATH", prependPath(dir, os.Getenv("PATH")))
	t.Setenv("CODEX_TEST_MODE", "ok")
	t.Setenv("CODEX_TEST_PAYLOAD", "ok")

	client := NewCodexCLIClient(&config.CodexCLIConfig{Model: "gpt-test", Timeout: 30})
	if !client.SkillAvailable() {
		t.Skip("repo skill not present in this checkout")
	}
	if _, err := client.CompleteWithSystem(context.Background(), "sys", "user"); err != nil {
		t.Fatalf("CompleteWithSystem: %v", err)
	}
	stdin := readLog(t, dir, "stdin.log")
	if !strings.HasPrefix(stdin, "$"+config.DefaultCodexExecSkillName+"\n\n") {
		t.Errorf("stdin lacks the $skill prefix:\n%s", stdin)
	}
}

// Schema mode writes the schema to a temp file, passes it via
// --output-schema, and returns the constrained answer.
func TestCodexExec_SchemaFileProtocol(t *testing.T) {
	dir := writeFakeCodexExec(t)
	t.Setenv("PATH", prependPath(dir, os.Getenv("PATH")))
	t.Setenv("CODEX_TEST_MODE", "ok")
	t.Setenv("CODEX_TEST_PAYLOAD", `{"surface_response":"hi"}`)

	schema := `{"type":"object","properties":{"surface_response":{"type":"string"}}}`
	resp, err := codexTestClient().CompleteWithSchema(context.Background(), "sys", "user", schema)
	if err != nil {
		t.Fatalf("CompleteWithSchema: %v", err)
	}
	if resp != `{"surface_response":"hi"}` {
		t.Errorf("response = %q, want the constrained answer", resp)
	}
	lines := argLines(t, dir)
	found := false
	for _, l := range lines {
		if strings.TrimSpace(l) == "--output-schema" {
			found = true
		}
	}
	if !found {
		t.Fatalf("argv lacks --output-schema:\n%s", strings.Join(lines, "\n"))
	}
	// The client re-encodes with indentation; compare semantically.
	var got, want map[string]any
	if err := json.Unmarshal([]byte(readLog(t, dir, "schema_copy.json")), &got); err != nil {
		t.Fatalf("schema file is not JSON: %v", err)
	}
	if err := json.Unmarshal([]byte(schema), &want); err != nil {
		t.Fatalf("test schema is not JSON: %v", err)
	}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("schema file = %v, want %v", got, want)
	}
}

// A rate-limited primary fails over to the fallback model exactly once;
// without a fallback the typed RateLimitError surfaces for errors.As.
func TestCodexExec_FallbackOnRateLimit(t *testing.T) {
	dir := writeFakeCodexExec(t)
	t.Setenv("PATH", prependPath(dir, os.Getenv("PATH")))
	t.Setenv("CODEX_TEST_MODE", "fallback")
	t.Setenv("CODEX_TEST_PAYLOAD", "fb ok")

	skillOff := false
	client := NewCodexCLIClient(&config.CodexCLIConfig{Model: "primary", FallbackModel: "fb-model", Timeout: 30, SkillEnabled: &skillOff})
	resp, err := client.CompleteWithSystem(context.Background(), "sys", "user")
	if err != nil {
		t.Fatalf("CompleteWithSystem with fallback: %v", err)
	}
	if resp != "fb ok" {
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

func TestCodexExec_RateLimitErrorWithoutFallback(t *testing.T) {
	dir := writeFakeCodexExec(t)
	t.Setenv("PATH", prependPath(dir, os.Getenv("PATH")))
	t.Setenv("CODEX_TEST_MODE", "rate_limit")

	_, err := codexTestClient().CompleteWithSystem(context.Background(), "sys", "user")
	var rlErr *RateLimitError
	if !errors.As(err, &rlErr) {
		t.Fatalf("err = %v (%T), want *RateLimitError", err, err)
	}
	if rlErr.Provider != "codex-cli" {
		t.Errorf("provider = %q, want codex-cli", rlErr.Provider)
	}
}

// An empty out file is recovered from the stdout JSONL agent message
// instead of failing the turn.
func TestCodexExec_EmptyOutFileRecoveredFromStdout(t *testing.T) {
	dir := writeFakeCodexExec(t)
	t.Setenv("PATH", prependPath(dir, os.Getenv("PATH")))
	t.Setenv("CODEX_TEST_MODE", "empty_out")

	resp, err := codexTestClient().CompleteWithSystem(context.Background(), "sys", "user")
	if err != nil {
		t.Fatalf("CompleteWithSystem: %v", err)
	}
	if resp != "recovered" {
		t.Errorf("response = %q, want the JSONL fallback message", resp)
	}
}

// The per-shard reasoning hint reaches the wire as a TOML-quoted -c
// override, exactly as the unit contract pins it.
func TestCodexExec_ReasoningEffortReachesWire(t *testing.T) {
	dir := writeFakeCodexExec(t)
	t.Setenv("PATH", prependPath(dir, os.Getenv("PATH")))
	t.Setenv("CODEX_TEST_MODE", "ok")
	t.Setenv("CODEX_TEST_PAYLOAD", "ok")

	skillOff := false
	client := NewCodexCLIClient(&config.CodexCLIConfig{
		Model: "gpt-test", Timeout: 30, SkillEnabled: &skillOff,
		ReasoningEffortHighReasoning: "xhigh",
	})
	ctx := types.WithModelCapability(context.Background(), types.CapabilityHighReasoning)
	if _, err := client.CompleteWithSystem(ctx, "sys", "user"); err != nil {
		t.Fatalf("CompleteWithSystem: %v", err)
	}
	lines := argLines(t, dir)
	found := false
	for i, l := range lines {
		if strings.TrimSpace(l) == "-c" && i+1 < len(lines) &&
			strings.TrimSpace(lines[i+1]) == `model_reasoning_effort="xhigh"` {
			found = true
		}
	}
	if !found {
		t.Errorf("argv lacks the quoted reasoning override:\n%s", strings.Join(lines, "\n"))
	}
}

// A cancelled context fails fast instead of running the CLI to completion.
func TestCodexExec_CancelledContextFailsFast(t *testing.T) {
	dir := writeFakeCodexExec(t)
	t.Setenv("PATH", prependPath(dir, os.Getenv("PATH")))
	t.Setenv("CODEX_TEST_MODE", "ok")
	t.Setenv("CODEX_TEST_PAYLOAD", "ok")

	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	start := time.Now()
	_, err := codexTestClient().CompleteWithSystem(ctx, "sys", "user")
	elapsed := time.Since(start)
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("err = %v, want wrapped context.Canceled", err)
	}
	if elapsed > 10*time.Second {
		t.Errorf("cancelled call took %v, want fast failure", elapsed)
	}
}
