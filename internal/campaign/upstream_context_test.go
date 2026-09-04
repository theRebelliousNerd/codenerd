package campaign

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"codenerd/internal/tactile"
)

func writeUpstreamDoc(t *testing.T, ws, rel, body string) string {
	t.Helper()
	full := filepath.Join(ws, rel)
	if err := os.MkdirAll(filepath.Dir(full), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(full, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
	return rel
}

func upstreamTwoPhaseFixture(t *testing.T) (*Orchestrator, *Task) {
	t.Helper()
	ws := t.TempDir()
	rel1 := writeUpstreamDoc(t, ws, ".nerd/campaigns/up/artifacts/a1.md", "FINDINGS-A1: nil-deref at internal/world/world.go:88 with evidence span 42.")
	rel2 := writeUpstreamDoc(t, ws, ".nerd/campaigns/up/artifacts/a2.md", "FINDINGS-A2: executor loop owns worker pool; must Close FileCache on shutdown.")
	o := &Orchestrator{
		workspace: ws,
		campaign: &Campaign{
			ID: "/campaign_up",
			Phases: []Phase{
				{
					ID: "phase_a", Order: 0,
					Tasks: []Task{
						{ID: "task_a1", PhaseID: "phase_a", Type: TaskTypeResearch, Status: TaskCompleted, Order: 0, Description: "Audit world for nil panics", Artifacts: []TaskArtifact{{Type: "/doc", Path: rel1}}},
						{ID: "task_a2", PhaseID: "phase_a", Type: TaskTypeResearch, Status: TaskCompleted, Order: 1, Description: "Audit session lifecycle", Artifacts: []TaskArtifact{{Type: "/doc", Path: rel2}}},
					},
				},
				{
					ID: "phase_b", Order: 1,
					Dependencies: []PhaseDependency{{DependsOnPhaseID: "phase_a", Type: DepHard}},
					Tasks: []Task{
						{ID: "task_b1", PhaseID: "phase_b", Type: TaskTypeDocument, Status: TaskPending, Order: 0, Description: "Document ranked risk report with severity evidence impact and next checks"},
					},
				},
			},
		},
	}
	return o, &o.campaign.Phases[1].Tasks[0]
}

func TestUpstreamArtifactContext_ContainsBoth(t *testing.T) {
	o, task := upstreamTwoPhaseFixture(t)
	section := o.upstreamArtifactContext(task)
	if section == "" {
		t.Fatal("expected non-empty upstream section")
	}
	if !strings.Contains(section, "## Upstream findings (durable artifacts)") {
		t.Fatalf("missing section header: %q", section)
	}
	for _, want := range []string{"task_a1", "task_a2", "FINDINGS-A1", "FINDINGS-A2"} {
		if !strings.Contains(section, want) {
			t.Fatalf("section missing %q: %q", want, section)
		}
	}
}

func TestUpstreamArtifactContext_TruncatesPerCap(t *testing.T) {
	// The fixture artifacts are ~75 bytes, so the cap must sit below that for
	// truncation to be observable.
	old := upstreamPerArtifactCapBytes
	upstreamPerArtifactCapBytes = 40
	defer func() { upstreamPerArtifactCapBytes = old }()
	o, task := upstreamTwoPhaseFixture(t)
	section := o.upstreamArtifactContext(task)
	if !strings.Contains(section, "[truncated") {
		t.Fatalf("expected [truncated marker with 40-byte cap: %q", section)
	}
	if strings.Contains(section, "evidence span 42") {
		t.Fatalf("body beyond the cap should have been cut: %q", section)
	}
}

func TestUpstreamArtifactContext_NoDepsYieldsEmpty(t *testing.T) {
	ws := t.TempDir()
	o := &Orchestrator{
		workspace: ws,
		campaign: &Campaign{
			ID: "/campaign_solo",
			Phases: []Phase{
				{ID: "phase_0", Order: 0, Tasks: []Task{
					{ID: "task_0", PhaseID: "phase_0", Type: TaskTypeResearch, Status: TaskPending, Description: "Solo research"},
				}},
			},
		},
	}
	if got := o.upstreamArtifactContext(&o.campaign.Phases[0].Tasks[0]); got != "" {
		t.Fatalf("expected empty section, got %q", got)
	}
}

func buildHollowVerifyFixture(t *testing.T, withUpstream bool) (*Orchestrator, *Task, string) {
	t.Helper()
	ws := t.TempDir()
	reportRel := "docs/summary_report.md"
	body := "No findings " + strings.Repeat("x ", 144)
	if len(body) > 300 {
		body = body[:300]
	}
	for len(body) < 300 {
		body += "x"
	}
	writeUpstreamDoc(t, ws, reportRel, body)
	o := &Orchestrator{
		workspace: ws,
		executor: &mockExecutor{
			executeFunc: func(ctx context.Context, cmd tactile.Command) (*tactile.ExecutionResult, error) {
				return &tactile.ExecutionResult{Success: true, ExitCode: 0, Stdout: "ok"}, nil
			},
		},
		campaign: &Campaign{ID: "/campaign_verify"},
	}
	if withUpstream {
		rel1 := writeUpstreamDoc(t, ws, ".nerd/campaigns/v/artifacts/a1.md", "Substantive upstream evidence one: defect at internal/world/world.go:88 with anchor and impact analysis detail.")
		rel2 := writeUpstreamDoc(t, ws, ".nerd/campaigns/v/artifacts/a2.md", "Substantive upstream evidence two: lifecycle hazard in internal/session/executor.go with anchor and impact analysis detail.")
		o.campaign.Phases = []Phase{
			{ID: "phase_a", Order: 0, Tasks: []Task{
				{ID: "task_a1", PhaseID: "phase_a", Type: TaskTypeResearch, Status: TaskCompleted, Order: 0, Description: "Audit world", Artifacts: []TaskArtifact{{Type: "/doc", Path: rel1}}},
				{ID: "task_a2", PhaseID: "phase_a", Type: TaskTypeResearch, Status: TaskCompleted, Order: 1, Description: "Audit session", Artifacts: []TaskArtifact{{Type: "/doc", Path: rel2}}},
			}},
			{ID: "phase_b", Order: 1,
				Dependencies: []PhaseDependency{{DependsOnPhaseID: "phase_a", Type: DepHard}},
				Tasks: []Task{
					{ID: "task_v", PhaseID: "phase_b", Type: TaskTypeVerify, Status: TaskPending, Description: "Verify the summary document is short and every item links to file plus symbol", Artifacts: []TaskArtifact{{Type: "/doc", Path: reportRel}}},
				}},
		}
		return o, &o.campaign.Phases[1].Tasks[0], reportRel
	}
	o.campaign.Phases = []Phase{
		{ID: "phase_b", Order: 0, Tasks: []Task{
			{ID: "task_v", PhaseID: "phase_b", Type: TaskTypeVerify, Status: TaskPending, Description: "Verify the summary document is short and every item links to file plus symbol", Artifacts: []TaskArtifact{{Type: "/doc", Path: reportRel}}},
		}},
	}
	return o, &o.campaign.Phases[0].Tasks[0], reportRel
}

func TestUpstreamVerifyTask_HollowFailsWithUpstream(t *testing.T) {
	o, task, reportRel := buildHollowVerifyFixture(t, true)
	_, err := o.executeVerifyTask(context.Background(), task)
	if err == nil {
		t.Fatal("expected hollow-report verification error, got nil")
	}
	if !strings.Contains(err.Error(), reportRel) {
		t.Fatalf("error must name report %q, got %v", reportRel, err)
	}
	if !strings.Contains(err.Error(), "2") {
		t.Fatalf("error must mention 2 upstream artifacts, got %v", err)
	}
}

func TestUpstreamVerifyTask_NoUpstreamKeepsBehavior(t *testing.T) {
	o, task, _ := buildHollowVerifyFixture(t, false)
	res, err := o.executeVerifyTask(context.Background(), task)
	if err != nil {
		t.Fatalf("with no upstream should keep today's result, got error %v", err)
	}
	m, ok := res.(map[string]any)
	if !ok {
		t.Fatalf("expected map result, got %T", res)
	}
	if m["verified"] != true {
		t.Fatalf("expected verified=true, got %#v", m)
	}
}
