package campaign

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func writeAssaultFixture(t *testing.T, ws, slug string, results []assaultResult, batches []string) {
	t.Helper()
	dir := filepath.Join(ws, ".nerd", "campaigns", slug, "assault")
	for _, sub := range []string{"batches", "results", "logs", "triage"} {
		if err := os.MkdirAll(filepath.Join(dir, sub), 0o755); err != nil {
			t.Fatalf("mkdir %s: %v", sub, err)
		}
	}

	targets := assaultTargetsFile{
		CampaignID: "/" + slug,
		CreatedAt:  time.Now().UTC(),
		Scope:      AssaultScopeSubsystem,
		Targets:    []string{"internal/alpha", "internal/beta"},
	}
	data, _ := json.MarshalIndent(targets, "", "  ")
	if err := os.WriteFile(filepath.Join(dir, "targets.json"), data, 0o644); err != nil {
		t.Fatalf("write targets: %v", err)
	}

	for _, b := range batches {
		bf := assaultBatchFile{CampaignID: targets.CampaignID, BatchID: b, Targets: targets.Targets}
		bd, _ := json.MarshalIndent(bf, "", "  ")
		if err := os.WriteFile(filepath.Join(dir, "batches", b+".json"), bd, 0o644); err != nil {
			t.Fatalf("write batch: %v", err)
		}
	}

	byBatch := map[string][]assaultResult{}
	for _, r := range results {
		byBatch[r.BatchID] = append(byBatch[r.BatchID], r)
	}
	for batch, rs := range byBatch {
		path := filepath.Join(dir, "results", batch+".jsonl")
		for _, r := range rs {
			if err := appendJSONL(path, r); err != nil {
				t.Fatalf("append result: %v", err)
			}
		}
	}
}

func TestBuildAssaultSummary_ShouldAggregateStagesAndTargets(t *testing.T) {
	ws := t.TempDir()
	slug := "campaign_assault_report"
	now := time.Now().UTC()

	writeAssaultFixture(t, ws, slug, []assaultResult{
		{BatchID: "batch_0000", Target: "internal/alpha", Stage: AssaultStageGoTest, ExitCode: 0, DurationMs: 1200, StartedAt: now},
		{BatchID: "batch_0000", Target: "internal/beta", Stage: AssaultStageGoTest, ExitCode: 1, DurationMs: 8000, StartedAt: now, LogPath: "logs/batch_0000/beta.log"},
		{BatchID: "batch_0000", Target: "internal/beta", Stage: AssaultStageGoVet, ExitCode: 2, DurationMs: 300, StartedAt: now, Error: "vet complained"},
		{BatchID: "batch_0001", Target: "internal/alpha", Stage: AssaultStageGoTest, Killed: true, KillReason: "timeout", DurationMs: 900000, StartedAt: now},
	}, []string{"batch_0000", "batch_0001"})

	summary, err := BuildAssaultSummary(ws, "/"+slug)
	if err != nil {
		t.Fatalf("BuildAssaultSummary: %v", err)
	}

	if summary.TotalRuns != 4 {
		t.Errorf("TotalRuns = %d, want 4", summary.TotalRuns)
	}
	if summary.Passed != 1 {
		t.Errorf("Passed = %d, want 1", summary.Passed)
	}
	if summary.Failed != 3 {
		t.Errorf("Failed = %d, want 3 (two non-zero exits plus the killed run)", summary.Failed)
	}
	if summary.Killed != 1 {
		t.Errorf("Killed = %d, want 1", summary.Killed)
	}
	if summary.Targets != 2 {
		t.Errorf("Targets = %d, want 2", summary.Targets)
	}
	if summary.Batches != 2 || summary.BatchesRun != 2 {
		t.Errorf("batches run/total = %d/%d, want 2/2", summary.BatchesRun, summary.Batches)
	}
	if summary.Incomplete {
		t.Error("a run where every batch produced results must not be marked incomplete")
	}

	if len(summary.Worst) == 0 || summary.Worst[0].Target != "internal/beta" {
		t.Fatalf("worst offender should be internal/beta (2 failures), got %+v", summary.Worst)
	}
	if len(summary.Worst[0].FailedStage) != 2 {
		t.Errorf("beta failed two distinct stages, got %v", summary.Worst[0].FailedStage)
	}

	stageByKind := map[AssaultStageKind]AssaultStageSummary{}
	for _, st := range summary.Stages {
		stageByKind[st.Stage] = st
	}
	goTest := stageByKind[AssaultStageGoTest]
	if goTest.Runs != 3 || goTest.Failed != 2 {
		t.Errorf("go_test summary = %+v, want 3 runs / 2 failed", goTest)
	}
	if goTest.SlowTarget != "internal/alpha" {
		t.Errorf("slowest go_test target = %q, want internal/alpha (the 900s killed run)", goTest.SlowTarget)
	}
}

// A summary over batches that never ran must say so. "0 failures" from work
// that was never executed reads exactly like a clean sweep.
func TestBuildAssaultSummary_WhenBatchesNeverRan_ShouldReportIncomplete(t *testing.T) {
	ws := t.TempDir()
	slug := "campaign_assault_partial"

	writeAssaultFixture(t, ws, slug, []assaultResult{
		{BatchID: "batch_0000", Target: "internal/alpha", Stage: AssaultStageGoTest, ExitCode: 0, DurationMs: 10},
	}, []string{"batch_0000", "batch_0001", "batch_0002"})

	summary, err := BuildAssaultSummary(ws, "/"+slug)
	if err != nil {
		t.Fatalf("BuildAssaultSummary: %v", err)
	}
	if !summary.Incomplete {
		t.Fatal("2 of 3 batches produced no results and the summary did not flag it")
	}

	md := RenderAssaultSummaryMarkdown(summary)
	if !strings.Contains(md, "Partial run") {
		t.Errorf("the rendered report must warn about the partial run:\n%s", md)
	}
}

func TestExportAssaultSummary_ShouldWriteJSONAndMarkdown(t *testing.T) {
	ws := t.TempDir()
	slug := "campaign_assault_export"

	writeAssaultFixture(t, ws, slug, []assaultResult{
		{BatchID: "batch_0000", Target: "internal/alpha", Stage: AssaultStageGoTest, ExitCode: 1, DurationMs: 500, Error: "boom"},
	}, []string{"batch_0000"})

	triage := assaultTriageOutput{
		Summary:          "total_results=1 success=0 failures=1",
		RecommendedTasks: []assaultRemediationTask{{Type: "/shard_task", Description: "fix alpha"}},
	}
	td, _ := json.MarshalIndent(triage, "", "  ")
	triagePath := filepath.Join(ws, ".nerd", "campaigns", slug, "assault", "triage", "latest.json")
	if err := os.WriteFile(triagePath, td, 0o644); err != nil {
		t.Fatalf("write triage: %v", err)
	}

	orch := journalOpsOrchestrator(t, ws)
	orch.campaign = &Campaign{ID: "/" + slug, Type: CampaignTypeAdversarialAssault, Title: "assault"}

	path, summary, err := orch.ExportAssaultSummary()
	if err != nil {
		t.Fatalf("ExportAssaultSummary: %v", err)
	}
	if summary.RemediationTasks != 1 {
		t.Errorf("RemediationTasks = %d, want 1", summary.RemediationTasks)
	}
	if summary.TriageSummary == "" {
		t.Error("triage summary was not carried into the report")
	}

	md, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read markdown report: %v", err)
	}
	for _, want := range []string{"# Assault summary", "## Totals", "internal/alpha", "## Triage"} {
		if !strings.Contains(string(md), want) {
			t.Errorf("report is missing %q:\n%s", want, md)
		}
	}

	jsonPath := filepath.Join(filepath.Dir(path), "summary.json")
	data, err := os.ReadFile(jsonPath)
	if err != nil {
		t.Fatalf("read json report: %v", err)
	}
	var roundTrip AssaultSummary
	if err := json.Unmarshal(data, &roundTrip); err != nil {
		t.Fatalf("summary.json is not valid JSON: %v", err)
	}
	if roundTrip.CampaignID != summary.CampaignID {
		t.Errorf("round-tripped campaign id = %q, want %q", roundTrip.CampaignID, summary.CampaignID)
	}
}

func TestBuildAssaultSummary_WhenNoEvidence_ShouldError(t *testing.T) {
	if _, err := BuildAssaultSummary(t.TempDir(), "/campaign_missing"); err == nil {
		t.Fatal("expected an error for a campaign with no assault directory")
	}
}
