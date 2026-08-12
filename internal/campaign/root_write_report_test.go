package campaign

import (
	"encoding/json"
	"os"
	"path/filepath"
	"sort"
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

func TestRecordRootBaseline_SnapshotsAndPersists(t *testing.T) {
	ws := t.TempDir()
	nerdDir := t.TempDir()
	for _, name := range []string{"go.mod", "README.md", "main.go"} {
		if err := os.WriteFile(filepath.Join(ws, name), []byte("x"), 0o644); err != nil {
			t.Fatalf("write %s: %v", name, err)
		}
	}
	if err := os.Mkdir(filepath.Join(ws, "internal"), 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	campaign := &Campaign{ID: "campaign_baseline_a"}
	o := &Orchestrator{
		config:   OrchestratorConfig{Workspace: ws},
		campaign: campaign,
		nerdDir:  nerdDir,
		kernel:   &MockKernel{},
	}
	o.recordRootBaseline()
	if o.rootBaseline == nil {
		t.Fatal("rootBaseline nil after first run; expected snapshot")
	}
	for _, want := range []string{"go.mod", "README.md", "main.go"} {
		if !o.rootBaseline[want] {
			t.Errorf("baseline missing %q", want)
		}
	}
	if o.rootBaseline["internal"] {
		t.Error("directory should not be in baseline")
	}
	if len(campaign.RootBaseline) == 0 {
		t.Fatal("campaign.RootBaseline not populated on first run")
	}
	if !sort.StringsAreSorted(campaign.RootBaseline) {
		t.Errorf("RootBaseline not sorted: %v", campaign.RootBaseline)
	}
	if len(campaign.RootBaseline) != len(o.rootBaseline) {
		t.Errorf("persisted slice len %d != baseline map len %d", len(campaign.RootBaseline), len(o.rootBaseline))
	}
	for _, name := range campaign.RootBaseline {
		if !o.rootBaseline[name] {
			t.Errorf("persisted entry %q not in in-memory baseline", name)
		}
	}
	// Verify persistence to disk is stable: file exists and contains sorted baseline.
	path := filepath.Join(nerdDir, "campaigns", campaign.ID+".json")
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read persisted campaign: %v", err)
	}
	var persisted Campaign
	if err := json.Unmarshal(data, &persisted); err != nil {
		t.Fatalf("decode persisted campaign: %v", err)
	}
	if len(persisted.RootBaseline) != len(campaign.RootBaseline) {
		t.Fatalf("persisted file RootBaseline len %d, want %d", len(persisted.RootBaseline), len(campaign.RootBaseline))
	}
	for i := range persisted.RootBaseline {
		if persisted.RootBaseline[i] != campaign.RootBaseline[i] {
			t.Fatalf("persisted file mismatch at %d: %q vs %q", i, persisted.RootBaseline[i], campaign.RootBaseline[i])
		}
	}
}

func TestRecordRootBaseline_RestoresPersistedBaseline(t *testing.T) {
	ws := t.TempDir()
	nerdDir := t.TempDir()
	// Pre-existing file that is part of the original baseline.
	if err := os.WriteFile(filepath.Join(ws, "keep.txt"), []byte("keep"), 0o644); err != nil {
		t.Fatalf("write keep: %v", err)
	}
	if err := os.WriteFile(filepath.Join(ws, "go.mod"), []byte("module x"), 0o644); err != nil {
		t.Fatalf("write go.mod: %v", err)
	}
	// Scratch file created by earlier run, now on disk but absent from persisted baseline.
	if err := os.WriteFile(filepath.Join(ws, "scratch.txt"), []byte("scratch"), 0o644); err != nil {
		t.Fatalf("write scratch: %v", err)
	}
	persistedBaseline := []string{"go.mod", "keep.txt"}
	// Ensure persisted is sorted as recordRootBaseline would have stored it.
	sort.Strings(persistedBaseline)
	campaign := &Campaign{ID: "campaign_baseline_b", RootBaseline: persistedBaseline}
	o := &Orchestrator{
		config:   OrchestratorConfig{Workspace: ws},
		campaign: campaign,
		nerdDir:  nerdDir,
		kernel:   &MockKernel{},
	}
	o.recordRootBaseline()
	// Must restore exactly the persisted set, not re-snapshot which would include scratch.txt.
	if !o.rootBaseline["keep.txt"] || !o.rootBaseline["go.mod"] {
		t.Error("restored baseline missing expected entries")
	}
	if o.rootBaseline["scratch.txt"] {
		t.Error("restored baseline incorrectly includes scratch.txt that was on disk but absent from persisted slice; must NOT re-snapshot")
	}
	if len(o.rootBaseline) != len(persistedBaseline) {
		t.Errorf("restored baseline size %d, want %d", len(o.rootBaseline), len(persistedBaseline))
	}
	// Campaign field must remain unchanged (no overwrite with snapshot).
	if len(campaign.RootBaseline) != len(persistedBaseline) {
		t.Fatalf("campaign.RootBaseline was overwritten")
	}
	for i := range persistedBaseline {
		if campaign.RootBaseline[i] != persistedBaseline[i] {
			t.Errorf("campaign.RootBaseline[%d] = %q, want %q", i, campaign.RootBaseline[i], persistedBaseline[i])
		}
	}
	// Prove sweepability: scratch.txt is absent from baseline and not declared, so sweep must move it.
	o.campaign.Phases = []Phase{{Tasks: []Task{{ID: "/t1", WriteSet: []string{"keep.txt"}}}}}
	// Ensure artifact dir does not yet contain scratch.
	o.sweepUndeclaredRootWrites()
	if _, err := os.Stat(filepath.Join(ws, "scratch.txt")); !os.IsNotExist(err) {
		t.Error("scratch.txt should have been swept (moved) but remains in workspace root")
	}
	if _, err := os.Stat(filepath.Join(ws, ".nerd", "campaigns", shortCampaignID(campaign.ID), "artifacts", "scratch.txt")); err != nil {
		t.Errorf("scratch.txt not preserved in artifacts after sweep: %v", err)
	}
	// Keep file must remain.
	if _, err := os.Stat(filepath.Join(ws, "keep.txt")); err != nil {
		t.Error("keep.txt should not have been swept")
	}
}

func TestRecordRootBaseline_OrderIndependent(t *testing.T) {
	ws := t.TempDir()
	nerdDir := t.TempDir()
	// Persisted baseline in unsorted order.
	unsorted := []string{"zebra.md", "alpha.md", "middle.md"}
	campaign := &Campaign{ID: "campaign_baseline_c", RootBaseline: unsorted}
	o := &Orchestrator{
		config:   OrchestratorConfig{Workspace: ws},
		campaign: campaign,
		nerdDir:  nerdDir,
		kernel:   &MockKernel{},
	}
	// Workspace contains those files plus a stray.
	for _, name := range []string{"alpha.md", "middle.md", "zebra.md", "stray.md"} {
		if err := os.WriteFile(filepath.Join(ws, name), []byte("x"), 0o644); err != nil {
			t.Fatalf("write %s: %v", name, err)
		}
	}
	o.recordRootBaseline()
	// Baseline must contain exactly the 3 persisted files regardless of order, not the stray.
	for _, want := range []string{"alpha.md", "middle.md", "zebra.md"} {
		if !o.rootBaseline[want] {
			t.Errorf("order-independent restore missing %q", want)
		}
	}
	if o.rootBaseline["stray.md"] {
		t.Error("stray.md should not be in restored baseline")
	}
	if len(o.rootBaseline) != 3 {
		t.Errorf("restored baseline len %d, want 3", len(o.rootBaseline))
	}
	// Second orchestrator with same set in different order must yield identical map.
	otherOrder := []string{"middle.md", "zebra.md", "alpha.md"}
	campaign2 := &Campaign{ID: "campaign_baseline_c2", RootBaseline: otherOrder}
	o2 := &Orchestrator{
		config:   OrchestratorConfig{Workspace: ws},
		campaign: campaign2,
		nerdDir:  nerdDir,
		kernel:   &MockKernel{},
	}
	o2.recordRootBaseline()
	if len(o2.rootBaseline) != len(o.rootBaseline) {
		t.Fatalf("order variation changed baseline size")
	}
	for k := range o.rootBaseline {
		if !o2.rootBaseline[k] {
			t.Errorf("order variation missing %q", k)
		}
	}
}


func TestRecordRootBaseline_EmptyNonNilBaselineIsRecorded(t *testing.T) {
	ws := t.TempDir()
	nerdDir := t.TempDir()
	// File that appeared after the empty baseline was recorded.
	if err := os.WriteFile(filepath.Join(ws, "scratch.txt"), []byte("scratch"), 0o644); err != nil {
		t.Fatalf("write scratch: %v", err)
	}
	// Empty but non-nil: this is what JSON `[]` decodes to, distinct from absent (nil).
	campaign := &Campaign{ID: "campaign_baseline_empty", RootBaseline: []string{}}
	o := &Orchestrator{
		config:   OrchestratorConfig{Workspace: ws},
		campaign: campaign,
		nerdDir:  nerdDir,
		kernel:   &MockKernel{},
	}
	o.recordRootBaseline()
	if o.rootBaseline == nil {
		t.Fatal("empty non-nil persisted baseline should restore as empty map, not nil")
	}
	if len(o.rootBaseline) != 0 {
		t.Errorf("restored baseline len %d, want 0; empty baseline must stay empty and not re-snapshot", len(o.rootBaseline))
	}
	if o.rootBaseline["scratch.txt"] {
		t.Error("scratch.txt must not be in restored baseline; empty baseline is recorded and must NOT re-snapshot the current root")
	}
	if campaign.RootBaseline == nil {
		t.Error("campaign.RootBaseline must remain non-nil (empty recorded baseline)")
	}
	if len(campaign.RootBaseline) != 0 {
		t.Errorf("campaign.RootBaseline len %d, want 0; must not be overwritten by snapshot", len(campaign.RootBaseline))
	}
	// Prove sweepability: the scratch file is sweepable because baseline is empty.
	o.campaign.Phases = []Phase{{Tasks: []Task{{ID: "/t1"}}}}
	o.sweepUndeclaredRootWrites()
	if _, err := os.Stat(filepath.Join(ws, "scratch.txt")); !os.IsNotExist(err) {
		t.Error("scratch.txt should have been swept when baseline is empty-but-recorded; re-snapshotting would have made it baseline and unsweepable")
	}
	if _, err := os.Stat(filepath.Join(ws, ".nerd", "campaigns", shortCampaignID(campaign.ID), "artifacts", "scratch.txt")); err != nil {
		t.Errorf("scratch.txt not preserved in artifacts after sweep: %v", err)
	}
}

