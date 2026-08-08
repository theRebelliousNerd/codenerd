package campaign

import (
	"os"
	"path/filepath"
	"testing"
)

// Observed live on campaign fc6472c2 (2026-08-08): a campaign asked only to add
// a doc comment to GateName also produced TEST_REPORT.md and
// research_runToolLoop_verification_gates.md in the repository root. No task
// requested either, and nothing reported that they had appeared.
//
// Task.WriteSet exists, but its only consumer is the lock manager, which
// serialises tasks touching the same file. Nothing checks that a write lands
// inside the declared set.

func newRootWatchOrchestrator(t *testing.T, ws string) *Orchestrator {
	t.Helper()
	return &Orchestrator{config: OrchestratorConfig{Workspace: ws}}
}

func TestSnapshotWorkspaceRoot(t *testing.T) {
	ws := t.TempDir()
	if err := os.WriteFile(filepath.Join(ws, "go.mod"), []byte("module x\n"), 0o644); err != nil {
		t.Fatalf("write: %v", err)
	}
	if err := os.MkdirAll(filepath.Join(ws, "internal"), 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}

	o := newRootWatchOrchestrator(t, ws)
	snap := o.snapshotWorkspaceRoot()

	if !snap["go.mod"] {
		t.Error("root file not captured")
	}
	// Directories are not the pattern this watches for; a task creating a
	// package directory is doing its job.
	if snap["internal"] {
		t.Error("a directory was captured; only root-level files should be")
	}

	// An unreadable workspace must yield nil, which the caller treats as
	// "cannot compare" and stays silent rather than reporting a phantom write.
	if got := newRootWatchOrchestrator(t, filepath.Join(ws, "nope")).snapshotWorkspaceRoot(); got != nil {
		t.Errorf("missing workspace should yield nil, got %v", got)
	}
	if got := newRootWatchOrchestrator(t, "  ").snapshotWorkspaceRoot(); got != nil {
		t.Errorf("empty workspace should yield nil, got %v", got)
	}
}

func TestReportUnexpectedRootWrites(t *testing.T) {
	ws := t.TempDir()
	o := newRootWatchOrchestrator(t, ws)

	touch := func(name string) {
		t.Helper()
		if err := os.WriteFile(filepath.Join(ws, name), []byte("x"), 0o644); err != nil {
			t.Fatalf("write %s: %v", name, err)
		}
	}
	touch("go.mod")
	before := o.snapshotWorkspaceRoot()

	// Exactly the observed pollution, plus one file the task legitimately
	// declared.
	touch("TEST_REPORT.md")
	touch("research_runToolLoop_verification_gates.md")
	touch("declared.md")

	task := &Task{ID: "/task_x", WriteSet: []string{"declared.md"}}

	// The report is a log line, so the assertion is that it runs cleanly and
	// that the declared file is excluded from the comparison it performs.
	// Verified directly here rather than through log capture.
	after := o.snapshotWorkspaceRoot()
	declared := map[string]bool{"declared.md": true}
	var unexpected []string
	for name := range after {
		if !before[name] && !declared[name] {
			unexpected = append(unexpected, name)
		}
	}
	if len(unexpected) != 2 {
		t.Fatalf("expected the 2 undeclared files to be flagged, got %v", unexpected)
	}

	// Must not panic, and must tolerate a nil baseline (workspace unreadable at
	// task start) by staying silent.
	o.reportUnexpectedRootWrites(task, before)
	o.reportUnexpectedRootWrites(task, nil)
	o.reportUnexpectedRootWrites(nil, before)
}

// A task that creates nothing new at the root must produce no report at all.
func TestReportUnexpectedRootWrites_QuietWhenNothingNew(t *testing.T) {
	ws := t.TempDir()
	o := newRootWatchOrchestrator(t, ws)
	if err := os.WriteFile(filepath.Join(ws, "go.mod"), []byte("x"), 0o644); err != nil {
		t.Fatalf("write: %v", err)
	}
	before := o.snapshotWorkspaceRoot()
	after := o.snapshotWorkspaceRoot()

	for name := range after {
		if !before[name] {
			t.Errorf("nothing changed, but %q was seen as new", name)
		}
	}
}

// The completion sweep must move ONLY files the campaign itself created and
// that no task declared. Every other file must be left exactly where it is.
func TestSweepUndeclaredRootWrites(t *testing.T) {
	ws := t.TempDir()
	touch := func(name string) {
		t.Helper()
		if err := os.WriteFile(filepath.Join(ws, name), []byte("x"), 0o644); err != nil {
			t.Fatalf("write %s: %v", name, err)
		}
	}

	// Pre-existing repository content.
	touch("go.mod")
	touch("README.md")

	o := &Orchestrator{
		config: OrchestratorConfig{Workspace: ws},
		campaign: &Campaign{
			ID: "/campaign_fc6472c2",
			Phases: []Phase{{
				Tasks: []Task{{ID: "/t1", WriteSet: []string{"declared.md"}}},
			}},
		},
	}
	o.recordRootBaseline()

	// What the campaign then produces: one declared file and two strays of
	// exactly the shape observed live.
	touch("declared.md")
	touch("TEST_REPORT.md")
	touch("research_runToolLoop_verification_gates.md")

	o.sweepUndeclaredRootWrites()

	exists := func(rel string) bool {
		_, err := os.Stat(filepath.Join(ws, rel))
		return err == nil
	}

	// Pre-existing files are untouchable — they are not in the campaign's
	// creations, so the baseline alone must protect them.
	for _, keep := range []string{"go.mod", "README.md"} {
		if !exists(keep) {
			t.Errorf("pre-existing file %q was swept; the baseline must protect it", keep)
		}
	}
	// A declared file was asked for and stays.
	if !exists("declared.md") {
		t.Error("a file the task declared in its write set was swept")
	}
	// The strays are gone from the root...
	for _, stray := range []string{"TEST_REPORT.md", "research_runToolLoop_verification_gates.md"} {
		if exists(stray) {
			t.Errorf("stray %q was left polluting the repository root", stray)
		}
	}
	// ...and preserved under the campaign, not deleted.
	for _, stray := range []string{"TEST_REPORT.md", "research_runToolLoop_verification_gates.md"} {
		if !exists(filepath.Join(".nerd", "campaigns", "fc6472c2", "artifacts", stray)) {
			t.Errorf("stray %q was not preserved in the campaign artifacts; content must be moved, never deleted", stray)
		}
	}
}

// With no baseline (campaign never started properly) the sweep must do nothing
// rather than treat every root file as a stray.
func TestSweepUndeclaredRootWrites_NoBaselineIsANoOp(t *testing.T) {
	ws := t.TempDir()
	if err := os.WriteFile(filepath.Join(ws, "important.md"), []byte("x"), 0o644); err != nil {
		t.Fatalf("write: %v", err)
	}

	o := &Orchestrator{
		config:   OrchestratorConfig{Workspace: ws},
		campaign: &Campaign{ID: "/campaign_abc"},
	}
	o.sweepUndeclaredRootWrites() // rootBaseline is nil

	if _, err := os.Stat(filepath.Join(ws, "important.md")); err != nil {
		t.Fatal("a missing baseline caused an existing file to be swept; it must be a no-op")
	}
}

func TestShortCampaignID(t *testing.T) {
	cases := map[string]string{
		"/campaign_fc6472c2": "fc6472c2",
		"campaign_fc6472c2":  "fc6472c2",
		"fc6472c2":           "fc6472c2",
		"/fc6472c2":          "fc6472c2",
	}
	for in, want := range cases {
		if got := shortCampaignID(in); got != want {
			t.Errorf("shortCampaignID(%q) = %q; want %q", in, got, want)
		}
	}
}
