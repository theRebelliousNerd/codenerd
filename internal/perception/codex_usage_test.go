package perception

import (
	"context"
	"os"
	"testing"

	"codenerd/internal/types"
	"codenerd/internal/usage"
)

// The Codex CLI bills the user's subscription and reports every completed
// turn's token accounting on its --json stdout. A codex-backed call used to
// reach the broker with no usage at all, so its receipt read "unreported".
func TestCodexExec_ReportsTheCLIsOwnTokenAccounting(t *testing.T) {
	dir := installFakeCLI(t, "codex")
	t.Setenv("PATH", prependPath(dir, os.Getenv("PATH")))
	t.Setenv("CODEX_TEST_MODE", "usage")
	t.Setenv("CODEX_TEST_PAYLOAD", "answer")

	rec := &usageRecorder{}
	ctx := usage.WithObserver(context.Background(), rec)
	if _, err := codexTestClient().CompleteWithSystem(ctx, "sys", "user"); err != nil {
		t.Fatalf("CompleteWithSystem: %v", err)
	}
	u := rec.last
	if u.InputTokens != 1000 || u.OutputTokens != 50 || u.CachedContentTokens != 800 || u.ThinkingTokens != 20 {
		t.Fatalf("reported %+v, want input 1000 (cache read 800 inside it), output 50 (reasoning 20 inside it)", u)
	}
	if !u.Has(types.UsageTokens|types.UsageCacheRead|types.UsageReasoning) || u.Has(types.UsageCacheWrite) {
		t.Errorf("known = %v: the categories the CLI emitted are known, cache write is not", u.Known)
	}
}

func TestCodexExecUsage_SumsTurnsAndLeavesAMissingBlockUnreported(t *testing.T) {
	stream := `{"type":"turn.completed","usage":{"input_tokens":10,"output_tokens":2}}` + "\n" +
		"not json\n" +
		`{"type":"turn.completed","usage":{"input_tokens":5,"output_tokens":1,"cached_input_tokens":3}}` + "\n"
	u, ok := codexExecUsage(stream)
	if !ok || u.InputTokens != 15 || u.OutputTokens != 3 {
		t.Fatalf("usage = %+v ok=%v, want the two turns summed", u, ok)
	}
	if u.Has(types.UsageCacheRead) {
		t.Error("cache read known after summing a turn that did not report it")
	}
	if _, ok := codexExecUsage(`{"type":"turn.started"}` + "\n" + `{"type":"error","message":"boom"}`); ok {
		t.Error("a stream with no completed turn must stay unreported, not free")
	}
}
