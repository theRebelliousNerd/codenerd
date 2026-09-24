package campaign

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"codenerd/internal/config"
	"codenerd/internal/tactile"
	toolscore "codenerd/internal/tools/core"
)

func writeEvidenceDoc(t *testing.T, ws, rel, body string) string {
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

// evidenceOrchestrator is an orchestrator over c whose kernel is the shipped
// corpus holding the campaign section's thresholds (edit changes them) and
// the campaign's own rows, as NewOrchestrator leaves it.
func evidenceOrchestrator(t *testing.T, ws string, c *Campaign, edit func(*config.CampaignConfig)) *Orchestrator {
	t.Helper()
	k := policyKernel(t, edit)
	if err := k.LoadFacts(c.ToFacts()); err != nil {
		t.Fatalf("load campaign facts: %v", err)
	}
	return &Orchestrator{workspace: ws, campaign: c, kernel: k, policy: testPolicy(edit)}
}

// A body whose head and tail are distinct markers: the tail is present only
// when the whole text is.
func markedBody(head, tail string, filler int) string {
	return "# " + head + "\n\n" + strings.Repeat("evidence line with internal/world/world.go:88 cited\n", filler) + tail + "\n"
}

// Three phases in a chain. The task under test (c1, phase c) depends on c0,
// and its brief names the Docs file a2 wrote two phases up; phase b is the one
// its phase depends on directly, phase a is transitive only.
func evidenceChainFixture(t *testing.T) (*Orchestrator, *Task, map[string]string) {
	t.Helper()
	ws := t.TempDir()
	bodies := map[string]string{
		"a1": markedBody("A1-HEAD", "A1-TAIL", 20),
		"a2": markedBody("A2-HEAD", "A2-TAIL", 20),
		"b1": markedBody("B1-HEAD", "B1-TAIL", 20),
		"c0": markedBody("C0-HEAD", "C0-TAIL", 20),
	}
	relA1 := writeEvidenceDoc(t, ws, ".nerd/campaigns/ev/artifacts/a1.md", bodies["a1"])
	relA2 := writeEvidenceDoc(t, ws, "Docs/spec/a2-principles.md", bodies["a2"])
	relB1 := writeEvidenceDoc(t, ws, ".nerd/campaigns/ev/artifacts/b1.md", bodies["b1"])
	relC0 := writeEvidenceDoc(t, ws, ".nerd/campaigns/ev/artifacts/c0.md", bodies["c0"])
	c := &Campaign{
		ID: "/campaign_ev",
		Phases: []Phase{
			{ID: "/phase_a", Order: 0, Status: PhaseCompleted, Tasks: []Task{
				{ID: "/task_a1", PhaseID: "/phase_a", Type: TaskTypeResearch, Status: TaskCompleted, Order: 0, Description: "Audit the world model", Artifacts: []TaskArtifact{{Type: "/doc", Path: relA1}}},
				{ID: "/task_a2", PhaseID: "/phase_a", Type: TaskTypeFileCreate, Status: TaskCompleted, Order: 1, Description: "Create the principles doc", Artifacts: []TaskArtifact{{Type: "/source_file", Path: relA2}}},
			}},
			{ID: "/phase_b", Order: 1, Status: PhaseCompleted,
				Dependencies: []PhaseDependency{{DependsOnPhaseID: "/phase_a", Type: DepHard}},
				Tasks: []Task{
					{ID: "/task_b1", PhaseID: "/phase_b", Type: TaskTypeResearch, Status: TaskCompleted, Order: 0, Description: "Map the wiring", Artifacts: []TaskArtifact{{Type: "/doc", Path: relB1}}},
				}},
			{ID: "/phase_c", Order: 2, Status: PhaseInProgress,
				Dependencies: []PhaseDependency{{DependsOnPhaseID: "/phase_b", Type: DepHard}},
				Tasks: []Task{
					{ID: "/task_c0", PhaseID: "/phase_c", Type: TaskTypeResearch, Status: TaskCompleted, Order: 0, Description: "Derive the gaps", Artifacts: []TaskArtifact{{Type: "/doc", Path: relC0}}},
					{ID: "/task_c1", PhaseID: "/phase_c", Type: TaskTypeDocument, Status: TaskPending, Order: 1, DependsOn: []string{"/task_c0"},
						Description: "Write the gap report using A2-PRINCIPLES.md and the gaps"},
				}},
		},
	}
	o := evidenceOrchestrator(t, ws, c, nil)
	paths := map[string]string{"a1": relA1, "a2": relA2, "b1": relB1, "c0": relC0}
	return o, &c.Phases[2].Tasks[1], paths
}

func TestTaskEvidence_DerivedByDependencyNameAndDistance(t *testing.T) {
	o, task, paths := evidenceChainFixture(t)
	section, err := o.taskContextSection(context.Background(), task)
	if err != nil {
		t.Fatal(err)
	}
	modes, err := o.askTaskEvidence(task.ID)
	if err != nil {
		t.Fatal(err)
	}
	want := map[string]string{
		paths["c0"]: evidenceInline, // a declared dependency
		paths["a2"]: evidenceInline, // the brief names its file, two phases up
		paths["b1"]: evidenceDigest, // the phase this phase depends on
		paths["a1"]: evidenceHandle, // transitive only
	}
	for path, mode := range want {
		if modes[path] != mode {
			t.Errorf("task_evidence for %s = %q, want %q (all: %v)", path, modes[path], mode, modes)
		}
	}
	if len(modes) != len(want) {
		t.Errorf("derived %d evidence rows, want %d: %v", len(modes), len(want), modes)
	}
	// Inline is whole: head and tail.
	for _, m := range []string{"C0-HEAD", "C0-TAIL", "A2-HEAD", "A2-TAIL"} {
		if !strings.Contains(section, m) {
			t.Errorf("an inlined artifact lost %s: %q", m, section)
		}
	}
	// A digest carries its outline and a handle, not the body; a handle line
	// carries only the handle.
	if strings.Contains(section, "B1-TAIL") || !strings.Contains(section, "B1-HEAD") {
		t.Errorf("the digest of b1 should list its heading and not its body: %q", section)
	}
	if strings.Contains(section, "A1-HEAD") || strings.Contains(section, "A1-TAIL") {
		t.Errorf("a1 is transitive-only and should be a handle line: %q", section)
	}
	if n := strings.Count(section, "handle=obs:sa:"); n != 2 {
		t.Errorf("want one recall handle for the digest and one for the handle line, got %d: %q", n, section)
	}
	if strings.Contains(section, "[truncated") {
		t.Errorf("nothing is cut: %q", section)
	}
	// Order: inline first, then digests, then handles.
	iInline, iDigest, iHandle := strings.Index(section, "C0-HEAD"), strings.Index(section, "B1-HEAD"), strings.Index(section, paths["a1"])
	if iInline < 0 || iDigest < iInline || iHandle < iDigest {
		t.Errorf("sections out of order (inline %d, digest %d, handle %d): %q", iInline, iDigest, iHandle, section)
	}
}

// An artifact the policy would inline but that is over
// campaign.upstream_inline_max_bytes is digested, never cut, and its whole
// text is one recall_context away.
func TestTaskEvidence_OverTheInlineCeilingIsADigestTheModelCanRecall(t *testing.T) {
	ws := t.TempDir()
	big := markedBody("BIG-HEAD", "BIG-TAIL", 400)
	rel := writeEvidenceDoc(t, ws, ".nerd/campaigns/big/artifacts/up.md", big)
	c := &Campaign{ID: "/campaign_big", Phases: []Phase{{ID: "/phase_0", Order: 0, Tasks: []Task{
		{ID: "/task_up", PhaseID: "/phase_0", Type: TaskTypeResearch, Status: TaskCompleted, Order: 0, Description: "Audit", Artifacts: []TaskArtifact{{Type: "/doc", Path: rel}}},
		{ID: "/task_down", PhaseID: "/phase_0", Type: TaskTypeDocument, Status: TaskPending, Order: 1, DependsOn: []string{"/task_up"}, Description: "Report"},
	}}}}
	o := evidenceOrchestrator(t, ws, c, func(cc *config.CampaignConfig) { cc.UpstreamInlineMaxBytes = len(big) - 1 })
	section, err := o.taskContextSection(context.Background(), &c.Phases[0].Tasks[1])
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(section, "BIG-TAIL") || strings.Contains(section, "[truncated") {
		t.Fatalf("an over-ceiling artifact must be a digest, never a cut body: %q", section)
	}
	i := strings.Index(section, "handle=obs:sa:")
	if i < 0 {
		t.Fatalf("the digest carries no recall handle: %q", section)
	}
	handle := strings.Fields(section[i+len("handle="):])[0]
	got, err := toolscore.RecallContextTool().Execute(context.Background(), map[string]any{"id": handle, "offset": 400, "limit": 10})
	if err != nil {
		t.Fatalf("recall_context %s: %v", handle, err)
	}
	if !strings.Contains(got, "BIG-TAIL") {
		t.Fatalf("recall_context did not return the digested artifact's tail: %q", got)
	}
}

// What a task writes is not its evidence, even when its brief names it and a
// completed task declared it first.
func TestTaskEvidence_ATasksOwnOutputIsNotItsEvidence(t *testing.T) {
	ws := t.TempDir()
	rel := writeEvidenceDoc(t, ws, "Docs/spec/OPEN-QUESTIONS.md", markedBody("OQ-HEAD", "OQ-TAIL", 5))
	c := &Campaign{ID: "/campaign_own", Phases: []Phase{{ID: "/phase_0", Order: 0, Tasks: []Task{
		{ID: "/task_first", PhaseID: "/phase_0", Type: TaskTypeFileCreate, Status: TaskCompleted, Order: 0, Description: "Create the questions", Artifacts: []TaskArtifact{{Type: "/source_file", Path: rel}}},
		{ID: "/task_again", PhaseID: "/phase_0", Type: TaskTypeFileModify, Status: TaskPending, Order: 1,
			Description: "Rewrite OPEN-QUESTIONS.md one question per file", WriteSet: []string{filepath.ToSlash(filepath.Join(ws, "docs", "spec", "open-questions.md"))}},
	}}}}
	o := evidenceOrchestrator(t, ws, c, nil)
	modes, err := func() (map[string]string, error) {
		if _, err := o.taskContextSection(context.Background(), &c.Phases[0].Tasks[1]); err != nil {
			return nil, err
		}
		return o.askTaskEvidence("/task_again")
	}()
	if err != nil {
		t.Fatal(err)
	}
	if mode, ok := modes[rel]; ok {
		t.Fatalf("the task's own write target was handed to it as %s evidence", mode)
	}
}

func TestTaskEvidence_ATaskOutsideTheCampaignHasNone(t *testing.T) {
	ws := t.TempDir()
	c := &Campaign{ID: "/campaign_solo", Phases: []Phase{{ID: "/phase_0", Order: 0, Tasks: []Task{
		{ID: "/task_0", PhaseID: "/phase_0", Type: TaskTypeResearch, Status: TaskPending, Description: "Solo research"},
	}}}}
	o := evidenceOrchestrator(t, ws, c, nil)
	for _, task := range []*Task{&c.Phases[0].Tasks[0], {ID: "/task_elsewhere", Description: "not planned"}} {
		got, err := o.taskContextSection(context.Background(), task)
		if err != nil || got != "" {
			t.Fatalf("%s: want no section, got %q (err %v)", task.ID, got, err)
		}
	}
}

func TestTaskEvidence_CyclicPhasesTerminateWithoutSelfEvidence(t *testing.T) {
	ws := t.TempDir()
	relA := writeEvidenceDoc(t, ws, ".nerd/campaigns/cyc/artifacts/a.md", markedBody("CYC-A", "CYC-A-END", 3))
	relB := writeEvidenceDoc(t, ws, ".nerd/campaigns/cyc/artifacts/b.md", markedBody("CYC-B", "CYC-B-END", 3))
	c := &Campaign{ID: "/campaign_cycle", Phases: []Phase{
		{ID: "/phase_a", Order: 0, Dependencies: []PhaseDependency{{DependsOnPhaseID: "/phase_b", Type: DepHard}}, Tasks: []Task{
			{ID: "/task_a1", PhaseID: "/phase_a", Type: TaskTypeResearch, Status: TaskCompleted, Order: 0, Description: "Research A", Artifacts: []TaskArtifact{{Type: "/doc", Path: relA}}},
		}},
		{ID: "/phase_b", Order: 1, Dependencies: []PhaseDependency{{DependsOnPhaseID: "/phase_a", Type: DepHard}}, Tasks: []Task{
			{ID: "/task_b1", PhaseID: "/phase_b", Type: TaskTypeResearch, Status: TaskCompleted, Order: 0, Description: "Research B", Artifacts: []TaskArtifact{{Type: "/doc", Path: relB}}},
			{ID: "/task_b2", PhaseID: "/phase_b", Type: TaskTypeDocument, Status: TaskPending, Order: 1, Description: "Document follow-up"},
		}},
	}}
	o := evidenceOrchestrator(t, ws, c, nil)
	done := make(chan error, 1)
	go func() {
		_, err := o.taskContextSection(context.Background(), &c.Phases[1].Tasks[1])
		done <- err
	}()
	select {
	case err := <-done:
		if err != nil {
			t.Fatal(err)
		}
	case <-time.After(30 * time.Second):
		t.Fatal("the evidence derivation hung on cyclic phase dependencies")
	}
	modes, err := o.askTaskEvidence("/task_b2")
	if err != nil {
		t.Fatal(err)
	}
	if modes[relA] != evidenceDigest || modes[relB] != evidenceDigest || len(modes) != 2 {
		t.Fatalf("want both artifacts digested once (a direct phase dependency and an earlier same-phase task), got %v", modes)
	}
}

// hollowVerifyFixture: phase a researched; phase b's /document task wrote the
// report the /verify task depends on, reportBytes long.
func hollowVerifyFixture(t *testing.T, reportBody string) (*Orchestrator, *Task, string) {
	t.Helper()
	ws := t.TempDir()
	relA1 := writeEvidenceDoc(t, ws, ".nerd/campaigns/v/artifacts/a1.md", markedBody("UP-1", "UP-1-END", 10))
	relA2 := writeEvidenceDoc(t, ws, ".nerd/campaigns/v/artifacts/a2.md", markedBody("UP-2", "UP-2-END", 10))
	report := writeEvidenceDoc(t, ws, "docs/summary_report.md", reportBody)
	c := &Campaign{ID: "/campaign_verify", Phases: []Phase{
		{ID: "/phase_a", Order: 0, Status: PhaseCompleted, Tasks: []Task{
			{ID: "/task_a1", PhaseID: "/phase_a", Type: TaskTypeResearch, Status: TaskCompleted, Order: 0, Description: "Audit world", Artifacts: []TaskArtifact{{Type: "/doc", Path: relA1}}},
			{ID: "/task_a2", PhaseID: "/phase_a", Type: TaskTypeResearch, Status: TaskCompleted, Order: 1, Description: "Audit session", Artifacts: []TaskArtifact{{Type: "/doc", Path: relA2}}},
		}},
		{ID: "/phase_b", Order: 1, Dependencies: []PhaseDependency{{DependsOnPhaseID: "/phase_a", Type: DepHard}}, Tasks: []Task{
			{ID: "/task_report", PhaseID: "/phase_b", Type: TaskTypeDocument, Status: TaskCompleted, Order: 0, Description: "Write the summary report", Artifacts: []TaskArtifact{{Type: "/doc", Path: report}}},
			{ID: "/task_v", PhaseID: "/phase_b", Type: TaskTypeVerify, Status: TaskPending, Order: 1, DependsOn: []string{"/task_report"}, Description: "Verify the summary document is short and every item links to file plus symbol"},
		}},
	}}
	o := evidenceOrchestrator(t, ws, c, nil)
	return o, &c.Phases[1].Tasks[1], report
}

func TestVerifyHollowReport_AReportUnderTheFloorWhileItsProducerWasOwedEvidenceFails(t *testing.T) {
	o, task, report := hollowVerifyFixture(t, "No findings. "+strings.Repeat("x", 280))
	_, err := o.executeVerifyTask(context.Background(), task)
	if err == nil {
		t.Fatal("expected the hollow report to fail the verify task")
	}
	for _, want := range []string{report, "owed 2 upstream artifacts", "is 293 bytes"} {
		if !strings.Contains(err.Error(), want) {
			t.Fatalf("error must say %q, got %v", want, err)
		}
	}
}

// Words are not the measure: a report at the floor passes the check whatever
// it says, and the reviewer judges what it says.
func TestVerifyHollowReport_AReportAtTheFloorPassesWhateverItSays(t *testing.T) {
	o, task, _ := hollowVerifyFixture(t, "No findings. "+strings.Repeat("x", 1024))
	if err := o.checkVerifyHollowReport(task); err != nil {
		t.Fatalf("a report at the floor is not hollow: %v", err)
	}
}

// A decomposer's /verify holds its own persisted review as its artifact. Its
// previous clean review ("No findings") is not a report it checks: the old
// phrase matcher failed the retry of every clean review.
func TestVerifyHollowReport_TheVerifysOwnCleanReviewIsNotItsReport(t *testing.T) {
	ws := t.TempDir()
	relA := writeEvidenceDoc(t, ws, ".nerd/campaigns/r/artifacts/a.md", markedBody("UP", "UP-END", 10))
	own := writeEvidenceDoc(t, ws, ".nerd/campaigns/r/artifacts/task_v.md", "# Verify\n\nNo findings.\n")
	c := &Campaign{ID: "/campaign_review", Phases: []Phase{
		{ID: "/phase_a", Order: 0, Tasks: []Task{
			{ID: "/task_a", PhaseID: "/phase_a", Type: TaskTypeResearch, Status: TaskCompleted, Order: 0, Description: "Audit", Artifacts: []TaskArtifact{{Type: "/doc", Path: relA}}},
		}},
		{ID: "/phase_b", Order: 1, Dependencies: []PhaseDependency{{DependsOnPhaseID: "/phase_a", Type: DepHard}}, Tasks: []Task{
			{ID: "/task_v", PhaseID: "/phase_b", Type: TaskTypeVerify, Status: TaskInProgress, Order: 0, Description: "Verify the audit", Artifacts: []TaskArtifact{{Type: "/doc", Path: own}}},
		}},
	}}
	o := evidenceOrchestrator(t, ws, c, nil)
	if err := o.checkVerifyHollowReport(&c.Phases[1].Tasks[0]); err != nil {
		t.Fatalf("the verify's own clean review failed it as hollow: %v", err)
	}
}

func TestVerifyHollowReport_NoUpstreamKeepsTheBuildRoute(t *testing.T) {
	ws := t.TempDir()
	c := &Campaign{ID: "/campaign_build", Phases: []Phase{{ID: "/phase_b", Order: 0, Tasks: []Task{
		{ID: "/task_code", PhaseID: "/phase_b", Type: TaskTypeFileModify, Status: TaskCompleted, Description: "change the scanner", WriteSet: []string{"internal/world/scan.go"}},
		{ID: "/task_v", PhaseID: "/phase_b", Type: TaskTypeVerify, Status: TaskPending, Order: 1, Description: "Verify the scanner builds"},
	}}}}
	o := evidenceOrchestrator(t, ws, c, nil)
	o.executor = &mockExecutor{executeFunc: func(ctx context.Context, cmd tactile.Command) (*tactile.ExecutionResult, error) {
		return &tactile.ExecutionResult{Success: true, ExitCode: 0, Stdout: "ok"}, nil
	}}
	res, err := o.executeVerifyTask(context.Background(), &c.Phases[0].Tasks[1])
	if err != nil {
		t.Fatalf("a verify with nothing upstream must keep its build route: %v", err)
	}
	if m, ok := res.(map[string]any); !ok || m["verified"] != true {
		t.Fatalf("expected verified=true, got %#v", res)
	}
}

func TestBriefNames(t *testing.T) {
	for _, tc := range []struct {
		brief, path string
		want        bool
	}{
		{"Create 01-VISION.md per the standard", "Docs/architecture/features/01-VISION.md", true},
		{"using 05/06-CAPABILITY-SPEC.md seams", "Docs/architecture/features/06-CAPABILITY-SPEC.md", true},
		{"using 05/06-CAPABILITY-SPEC.md seams", "Docs/architecture/features/05-CAPABILITY-SPEC.md", false},
		{"Repair from .nerd/campaigns/7b/artifacts/task_4_6.md now", ".nerd/campaigns/7b/artifacts/task_4_6.md", true},
		{"edit data2.md", "Docs/a2.md", false},
		{"read the README.md.", "Docs/README.md", true},
		{"read README.markdown", "Docs/README.md", false},
		{"no extension named here: TODO", "Docs/TODO", false},
	} {
		if got := briefNames(tc.brief, tc.path); got != tc.want {
			t.Errorf("briefNames(%q, %q) = %v, want %v", tc.brief, tc.path, got, tc.want)
		}
	}
}
