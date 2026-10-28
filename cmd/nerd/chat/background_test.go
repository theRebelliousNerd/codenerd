package chat

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"codenerd/internal/config"
	"codenerd/internal/perception"
	"codenerd/internal/types"
)

// background_test.go: read-only delegations run off the turn
// (route_background, routing_arbitration.mg), and "debug yourself" is a
// kernel-laid chain (self_diagnosis.mg). Real kernel, embedded policy.

func TestBackground_ReadOnlyDelegationRunsOffTheTurn(t *testing.T) {
	m := admissionModel(t, config.WorkModeExplicit, nil)
	intent := perception.Intent{Category: "/query", Verb: "/review", Target: "internal/auth", Confidence: 0.95}
	assertRouteIntent(t, m, intent)
	route := m.decideRoute("review internal/auth", intent, "reviewer")
	if route.Kind != RouteDelegate || !route.Background {
		t.Fatalf("route = %s background=%v, want a background delegation", route.Kind, route.Background)
	}
}

func TestBackground_MutationNeverRunsInTheBackground(t *testing.T) {
	m := admissionModel(t, config.WorkModeYolo, nil)
	intent := fixReadme()
	assertRouteIntent(t, m, intent)
	if route := m.decideRoute("fix the typo in README.md", intent, "coder"); route.Background {
		t.Fatalf("a mutation went to the background: %+v", route)
	}
}

func TestBackground_JobIsGroundedWhileRunningAndClearedAfter(t *testing.T) {
	m := admissionModel(t, config.WorkModePropose, nil)
	job := backgroundJob{ID: "bg-1", Shard: "reviewer", Task: "review internal/auth", Started: time.Now()}
	m.background.add(job)
	m.assertBackgroundJob(job, "/running")
	if !strings.Contains(injectedLines(t, m), "background reviewer shard is already working on: review internal/auth") {
		t.Fatalf("a running job is not named to the model:\n%s", injectedLines(t, m))
	}
	if got := m.backgroundFooter(); got != "bg: reviewer" {
		t.Fatalf("footer = %q", got)
	}

	m, _ = m.finishBackground(backgroundDoneMsg{Job: job, Surface: "two findings", Result: "two findings"})
	if strings.Contains(injectedLines(t, m), "review internal/auth") {
		t.Fatal("a finished job is still named as running")
	}
	if m.backgroundFooter() != "" {
		t.Fatal("a finished job is still in the footer")
	}
	last := m.history[len(m.history)-1].Content
	if !strings.Contains(last, "Background reviewer finished") || !strings.Contains(last, "two findings") {
		t.Fatalf("result message: %q", last)
	}
}

func TestBackground_FailureIsReported(t *testing.T) {
	m := admissionModel(t, config.WorkModePropose, nil)
	job := backgroundJob{ID: "bg-2", Shard: "researcher", Task: "find the retry policy", Started: time.Now()}
	m.background.add(job)
	m, _ = m.finishBackground(backgroundDoneMsg{Job: job, Err: errors.New("spawn refused")})
	if last := m.history[len(m.history)-1].Content; !strings.Contains(last, "failed") || !strings.Contains(last, "spawn refused") {
		t.Fatalf("failure message: %q", last)
	}
}

func TestSelfDiagnosis_RoutesToTheBackgroundChain(t *testing.T) {
	m := admissionModel(t, config.WorkModePropose, nil)
	intent := perception.Intent{Category: "/query", Verb: "/self_diagnose", Target: "codenerd", Confidence: 0.9}
	assertRouteIntent(t, m, intent)
	route := m.decideRoute("debug yourself", intent, "reviewer")
	if route.Kind != RouteDelegate || !route.Background || !route.SelfDiagnosis {
		t.Fatalf("route = %+v, want the background self-diagnosis chain", route)
	}
}

func TestSelfDiagnosis_TaskNamesEveryHotProblemWhole(t *testing.T) {
	ws := t.TempDir()
	logs := filepath.Join(ws, ".nerd", "logs")
	if err := os.MkdirAll(logs, 0o755); err != nil {
		t.Fatal(err)
	}
	longTail := strings.Repeat("detail ", 80)
	log := "2026/10/28 06:19:54.100 [ERROR] [session] edit_element failed for fn:chat.Model.render: preimage mismatch " + longTail + "\n" +
		"2026/10/28 06:20:01.200 [WARN] [kernel] slow evaluation 1200ms\n"
	if err := os.WriteFile(filepath.Join(logs, "20261028_061954_problems.log"), []byte(log), 0o644); err != nil {
		t.Fatal(err)
	}
	m := admissionModel(t, config.WorkModePropose, nil)
	m.workspace = ws

	job, ok, reason := m.selfDiagnosisJob("reviewer")
	if !ok {
		t.Fatalf("no diagnosis job: %s", reason)
	}
	if !strings.Contains(job.Task, "edit_element failed") || !strings.Contains(job.Task, strings.TrimSpace(longTail)) {
		t.Errorf("the hot error is missing or cut:\n%s", job.Task)
	}
	if strings.Contains(job.Task, "slow evaluation") {
		t.Errorf("a warning seen once is not hot, yet the task names it:\n%s", job.Task)
	}
	if job.FollowUp != selfDiagnosisFollowUp {
		t.Errorf("the chain has no fix step: %q", job.FollowUp)
	}
	rows, err := m.kernel.Query("self_problem")
	if err != nil || len(rows) != 2 {
		t.Fatalf("the problems log was not projected into the kernel: %d rows, %v", len(rows), err)
	}
}

func TestSelfDiagnosis_NoLogSaysHowToGetOne(t *testing.T) {
	m := admissionModel(t, config.WorkModePropose, nil)
	m.workspace = t.TempDir()
	if _, ok, reason := m.selfDiagnosisJob("reviewer"); ok || !strings.Contains(reason, "/debug-self") {
		t.Fatalf("ok=%v reason=%q", ok, reason)
	}
}

func injectedLines(t *testing.T, m Model) string {
	t.Helper()
	rows, err := m.kernel.Query("injectable_context")
	if err != nil {
		t.Fatal(err)
	}
	var lines []string
	for _, r := range rows {
		if len(r.Args) == 2 {
			lines = append(lines, types.ExtractString(r.Args[1]))
		}
	}
	return strings.Join(lines, "\n")
}
