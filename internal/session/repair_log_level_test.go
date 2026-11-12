package session

import (
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"testing"

	"codenerd/internal/logging"
	"codenerd/internal/types"
)

const sessionOwnProcessEnv = "CODENERD_SESSION_OWN_PROCESS"

// inOwnSessionProcess runs the calling test alone in a child test process,
// for a test that reads back what process-wide logging wrote; it reports true
// in the parent, which then returns.
func inOwnSessionProcess(t *testing.T) bool {
	t.Helper()
	if os.Getenv(sessionOwnProcessEnv) == t.Name() {
		return false
	}
	cmd := exec.Command(os.Args[0], "-test.run=^"+regexp.QuoteMeta(t.Name())+"$", "-test.count=1", "-test.v")
	cmd.Env = append(os.Environ(), sessionOwnProcessEnv+"="+t.Name())
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("%s in its own process: %v\n%s", t.Name(), err, out)
	}
	if !strings.Contains(string(out), "--- PASS: "+t.Name()) {
		t.Fatalf("%s did not run in its own process:\n%s", t.Name(), out)
	}
	return true
}

// A turn whose edits fail their tests, and whose repair gives up, is a
// verdict on the workspace that the turn reports itself: its gate runs and
// repair rounds are logged, but not as WARN or ERROR, which is the problems
// log debug-self reads as codeNERD's own faults. Live 2026-11-12: a failed
// self-repair fix turn's "Repair attempt # (tests)" and "test verification
// FAILED" lines became the next self-repair's targets.
func TestRepairLoop_AWorkspaceVerdictStaysOutOfTheProblemsLog(t *testing.T) {
	if inOwnSessionProcess(t) {
		return
	}
	h := newRepairHarness(t, func(e *Executor) { e.config.RepairMaxAttempts = 1 })
	logWs := t.TempDir()
	if err := os.MkdirAll(filepath.Join(logWs, ".nerd"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(logWs, ".nerd", "config.json"), []byte(`{"logging":{"debug_mode":true,"level":"debug"}}`), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := logging.Initialize(logWs); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		logging.CloseAll()
		logging.ApplyConfig(logging.Config{DebugMode: false})
		logging.ClearInjectedConfig()
	})
	testPath := filepath.Join(h.ws, "main_test.go")
	h.initial = func() *types.LLMToolResponse {
		return &types.LLMToolResponse{Text: "writing", ToolCalls: []types.ToolCall{
			h.writeCall("c1", "write_file", filepath.Join(h.ws, "main.go"), repairMainGo),
			h.writeCall("c2", "write_file", testPath, repairTestBroken),
		}}
	}
	h.onRepair = func(int, []types.Message) *types.LLMToolResponse {
		return &types.LLMToolResponse{Text: "trying", ToolCalls: []types.ToolCall{
			h.writeCall("r1", "write_file", testPath, strings.Replace(repairTestBroken, "broken", "still broken", 1)),
		}}
	}
	if _, err := h.drive(t, "fix the probe test"); err == nil {
		t.Fatal("the repair converged; the test needs it to give up")
	}
	logging.CloseAll()
	read := func(glob string) string {
		matches, _ := filepath.Glob(filepath.Join(logWs, ".nerd", "logs", glob))
		var b strings.Builder
		for _, p := range matches {
			data, _ := os.ReadFile(p)
			b.Write(data)
		}
		return b.String()
	}
	problems := read("*_problems.log")
	for _, line := range []string{"Repair attempt", "test verification FAILED", "Edits broke the tests", "gave up after"} {
		if strings.Contains(problems, line) {
			t.Errorf("the workspace verdict %q is in the problems log:\n%s", line, problems)
		}
	}
	session := read("*_session.log")
	for _, line := range []string{"Repair attempt 1 (tests)", "test verification FAILED"} {
		if !strings.Contains(session, line) {
			t.Errorf("%q is not in the session log at all", line)
		}
	}
}
