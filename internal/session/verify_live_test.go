package session

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"codenerd/internal/sessionlog"
)

// The turn's gates run after the model's last answer; each run shows in the
// live view as a verify call that starts and ends with its outcome.
func TestVerifyTests_ReportsTheGateRunLive(t *testing.T) {
	if _, err := exec.LookPath("go"); err != nil {
		t.Skip("no Go toolchain on PATH")
	}
	dir := t.TempDir()
	for name, body := range map[string]string{
		"go.mod":      "module livegate\n\ngo 1.22\n",
		"ok_test.go":  "package livegate\n\nimport \"testing\"\n\nfunc TestOK(t *testing.T) {}\n",
		"livegate.go": "package livegate\n",
	} {
		if err := os.WriteFile(filepath.Join(dir, name), []byte(body), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	var live []sessionlog.LiveEvent
	ctx := sessionlog.WithAgent(sessionlog.WithLiveProgress(context.Background(), func(_ context.Context, ev sessionlog.LiveEvent) {
		live = append(live, ev)
	}), "coder", "gate")
	if v := verifyTests(ctx, dir, []string{"./..."}); v.Verdict() != VerifyPassed {
		t.Fatalf("probe module tests: %+v", v)
	}
	if len(live) != 2 || live[0].Kind != "tool_started" || live[1].Kind != sessionlog.EventToolResult ||
		live[0].Tool != "verify" || live[0].CallID != live[1].CallID || live[1].Status != "PASS" || live[1].Failed {
		t.Fatalf("live gate events = %+v", live)
	}
	if want := "go test -json ./..."; live[0].Text != want {
		t.Fatalf("gate subject = %q, want %q", live[0].Text, want)
	}
}

// The pinning gate re-runs the turn's tests once per change and once per
// condition, which can take minutes; the live view showed "preparing next
// action" throughout. Each round now shows as a verify call that ends with
// its outcome.
func TestVerifyPinning_ReportsEachRoundLive(t *testing.T) {
	if testing.Short() {
		t.Skip("shells out to the real go toolchain")
	}
	if _, err := exec.LookPath("go"); err != nil {
		t.Skip("no Go toolchain on PATH")
	}
	ws, result := pinTurn(t)
	var live []sessionlog.LiveEvent
	ctx := sessionlog.WithAgent(sessionlog.WithLiveProgress(context.Background(), func(_ context.Context, ev sessionlog.LiveEvent) {
		live = append(live, ev)
	}), "coder", "pin")
	if v := verifyPinning(ctx, ws, result, true); v.Verdict() != VerifyFailed {
		t.Fatalf("fixture verdict = %s (%s), want failed", v.Verdict(), v.Reason)
	}
	if len(live) != 4 {
		t.Fatalf("live events = %+v, want a started and a finished call for the change round and the condition round", live)
	}
	for i := 0; i < 4; i += 2 {
		start, end := live[i], live[i+1]
		if start.Kind != "tool_started" || end.Kind != sessionlog.EventToolResult || start.Tool != "verify" || start.CallID != end.CallID {
			t.Fatalf("round %d events = %+v, %+v", i/2, start, end)
		}
	}
	if !strings.Contains(live[0].Text, "change(s) taken out one at a time") || live[1].Status != "FAIL" || !live[1].Failed {
		t.Fatalf("change round = %+v -> %+v, want it to name the round and end FAIL", live[0], live[1])
	}
	if !strings.Contains(live[2].Text, "condition(s) forced one at a time (advisory)") || live[3].Failed {
		t.Fatalf("condition round = %+v -> %+v, want an advisory round that does not fail", live[2], live[3])
	}
}
