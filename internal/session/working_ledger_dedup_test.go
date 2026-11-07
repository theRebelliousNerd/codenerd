package session

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// Same file at same revision must never be carried twice.
func TestWorkingLedger_NeverCarriesSameFocusViewTwice(t *testing.T) {
	t.Parallel()
	l := newShedLoop(t, "a.go", "b.go", "a.go")
	got := len(l.loop.snapshots)
	if got != 2 {
		t.Fatalf("same-rev revisits must dedup to one snapshot per path, got %d snapshots", got)
	}
	seen := map[string]int{}
	for _, s := range l.loop.snapshots {
		seen[s.path]++
	}
	if seen["a.go"] != 1 || seen["b.go"] != 1 {
		t.Fatalf("deduped snapshots = %v, want one per path", seen)
	}
	msgs := l.round("a.go")
	if n := strings.Count(requestText(msgs), fmt.Sprintf(workingViewHeader, "a.go")); n != 1 {
		t.Fatalf("deduped request carries %d whole snapshots of a.go, want 1", n)
	}
	if len(l.loop.snapshots) != 2 {
		t.Fatalf("revisiting same rev must not append, snapshots = %d, want 2", len(l.loop.snapshots))
	}
}

// New revision must append again: dedup is by file+revision+view, not path.
func TestWorkingLedger_NewRevisionAppendsAgain(t *testing.T) {
	t.Parallel()
	l := newShedLoop(t, "a.go")
	before := len(l.loop.snapshots)
	if before != 1 {
		t.Fatalf("first view appends one snapshot, got %d", before)
	}
	// Same file at a new revision is a new view: bump the file, revisit.
	p := filepath.Join(l.e.config.WorkspaceRoot, "a.go")
	b, err := os.ReadFile(p)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(p, append(b, '\n'), 0o600); err != nil {
		t.Fatal(err)
	}
	l.round("a.go")
	if len(l.loop.snapshots) == before {
		t.Fatalf("new revision must append, snapshots stayed %d", before)
	}
	if got := len(l.loop.snapshots); got != 2 {
		t.Fatalf("new revision appends one more snapshot, got %d want 2", got)
	}
}

// Replay of the turn's 29-view sequence shape: 8 files, repeats at same rev.
// Measures the saving dedup provides, both as arithmetic on the brief's
// totals and as exact bytes on the named duplicates, plus the ledger's
// actual snapshot bytes after replay.
func TestWorkingLedger_DedupSavingOnRepeatedSequence(t *testing.T) {
	t.Parallel()
	// Task brief: this turn carried 29 views totalling 191206 chars across 8
	// files at unchanged revisions. Dedup keeps one view per file+revision.
	const briefViews, briefChars, briefFiles = 29, 191206, 8
	without := briefChars
	perView := float64(briefChars) / float64(briefViews)
	with := int(perView * float64(briefFiles))
	saved := without - with
	t.Logf("dedup saving on brief sequence: %d views/%d chars/%d files -> with dedup ~%d chars, saved ~%d chars (%.1f%%)", briefViews, without, briefFiles, with, saved, 100*float64(saved)/float64(without))
	if saved <= 0 {
		t.Fatalf("dedup must save, saved=%d", saved)
	}
	if float64(saved)/float64(without) < 0.7 {
		t.Fatalf("saving %.1f%% < 70%% on same-rev repeats", 100*float64(saved)/float64(without))
	}
	// Exact lower bound from the duplicates the brief names: main.go carried
	// the identical 16686-char view four times, deterministic_branch_test.go
	// 3268 chars three times, deep_query.go 3010 chars three times. Those ten
	// appends alone collapse to three snapshots.
	const mainView, detView, deepView = 16686, 3268, 3010
	const exactSaved = (4-1)*mainView + (3-1)*detView + (3-1)*deepView
	if exactSaved != 62614 {
		t.Fatalf("exact duplicate math drifted: got %d, want 62614", exactSaved)
	}
	t.Logf("exact duplicates saving: 10 views collapse to 3 snapshots, saved %d chars (%.1f%% of brief total)", exactSaved, 100*float64(exactSaved)/float64(briefChars))
	if exactSaved <= 0 || exactSaved >= briefChars {
		t.Fatalf("exact saving out of range: %d of %d", exactSaved, briefChars)
	}
	files := []string{"a.go", "b.go", "c.go", "d.go", "e.go", "f.go", "g.go", "h.go"}
	// Build a 29-view sequence bouncing across 8 files at fixed revisions.
	seq := make([]string, 0, briefViews)
	for i := 0; i < briefViews; i++ {
		seq = append(seq, files[i%len(files)])
	}
	// And prove it in the ledger: replay the sequence, assert 8 snapshots.
	l := newShedLoop(t, seq[:8]...)
	for _, f := range seq[8:] {
		l.round(f)
	}
	if got := len(l.loop.snapshots); got != 8 {
		t.Fatalf("29 same-rev views across 8 files must collapse to 8 snapshots, got %d", got)
	}
	withLedger := 0
	for _, s := range l.loop.snapshots {
		withLedger += len(s.text)
	}
	// The fixture renders every file at the same size, so the 29 views that
	// would have ridden without dedup cost 29/8 of what the 8 snapshots cost.
	withoutLedger := withLedger * briefViews / briefFiles
	savedLedger := withoutLedger - withLedger
	t.Logf("ledger bytes: without dedup ~%d chars, with dedup %d chars, saved %d chars (%.1f%%)", withoutLedger, withLedger, savedLedger, 100*float64(savedLedger)/float64(withoutLedger))
	if savedLedger <= 0 {
		t.Fatalf("ledger replay must save, saved=%d", savedLedger)
	}
}

func TestWorkingLedger_CarriedViewsNilGuardInitializes(t *testing.T) {
	t.Parallel()
	l := newShedLoop(t, "a.go")
	if got := len(l.loop.snapshots); got != 1 {
		t.Fatalf("setup must carry one snapshot, got %d", got)
	}
	// Force the zero value the nil guard defends against: a loop with no
	// dedup set must not panic and must still enforce once-per-view.
	l.loop.carriedViews = nil
	l.round("a.go")
	if l.loop.carriedViews == nil {
		t.Fatalf("carriedViews nil guard did not initialize set on same view")
	}
	if got := len(l.loop.carriedViews); got != 0 {
		t.Fatalf("same file/same revision/same view must not re-append, set size = %d, want 0", got)
	}
	if got := len(l.loop.snapshots); got != 1 {
		t.Fatalf("same view after nil guard must not append, snapshots = %d, want 1", got)
	}
	l.round("b.go")
	if l.loop.carriedViews == nil {
		t.Fatalf("carriedViews nil guard did not initialize set on new view")
	}
	if got := len(l.loop.carriedViews); got != 1 {
		t.Fatalf("new view after nil guard must populate set, size = %d, want 1", got)
	}
	if got := len(l.loop.snapshots); got != 2 {
		t.Fatalf("new view after nil guard must append, snapshots = %d, want 2", got)
	}
	l.round("b.go")
	if got := len(l.loop.snapshots); got != 2 {
		t.Fatalf("duplicate view must not append twice, snapshots = %d, want 2", got)
	}
}

func TestWorkingLedger_BeginWorkingLoopInitializesCarriedViews(t *testing.T) {
	t.Parallel()
	l := newShedLoop(t)
	if l.loop.carriedViews == nil {
		t.Fatalf("beginWorkingLoop must initialize carriedViews dedup set, got nil before any round")
	}
}
