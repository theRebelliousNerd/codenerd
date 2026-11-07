package session

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
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
