package perception

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"reflect"
	"strings"
	"testing"
	"time"

	"codenerd/internal/config"
	"codenerd/internal/types"
)

func codexTestClient() *CodexCLIClient {
	skillOff := false
	return NewCodexCLIClient(&config.CodexCLIConfig{Model: "gpt-test", Timeout: 30, SkillEnabled: &skillOff})
}

// End to end through a fake binary: exec - with the prompt on stdin (never
// in argv), the read-only/shell-disabled fail-closed flags, and the answer
// read back from the --output-last-message file.
func TestCodexExec_ArgvContractAndCompletion(t *testing.T) {
	dir := installFakeCLI(t, "codex")
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
	stdin := readLog(t, dir, "stdin.log")
	for _, want := range []string{"<system_instructions>", "sys-prompt", "user-prompt"} {
		if !strings.Contains(stdin, want) {
			t.Errorf("stdin lacks %q: %q", want, stdin)
		}
	}
}

// With the repo skill present, the prompt is prefixed with the skill
// invocation so codex runs under codeNERD's instructions.
func TestCodexExec_SkillPrefix(t *testing.T) {
	dir := installFakeCLI(t, "codex")
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
	dir := installFakeCLI(t, "codex")
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
	dir := installFakeCLI(t, "codex")
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
	dir := installFakeCLI(t, "codex")
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
	dir := installFakeCLI(t, "codex")
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
	dir := installFakeCLI(t, "codex")
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
	dir := installFakeCLI(t, "codex")
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
