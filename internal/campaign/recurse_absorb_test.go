package campaign

import (
	"context"
	"io"
	"slices"
	"strings"
	"sync"
	"testing"

	"codenerd/internal/core"
	"codenerd/internal/gates"
)

// lineHook is a progress writer that calls onLine for each line the loop
// writes, in the loop's own goroutine order, and keeps the text.
type lineHook struct {
	mu     sync.Mutex
	text   strings.Builder
	onLine func(line string)
}

func (h *lineHook) Write(p []byte) (int, error) {
	h.mu.Lock()
	h.text.Write(p)
	h.mu.Unlock()
	if h.onLine != nil {
		for line := range strings.SplitSeq(strings.TrimRight(string(p), "\n"), "\n") {
			h.onLine(line)
		}
	}
	return len(p), nil
}

func (h *lineHook) String() string {
	h.mu.Lock()
	defer h.mu.Unlock()
	return h.text.String()
}

// runCycles runs the loop over root with cfg adjusted by tune.
func runCycles(t *testing.T, ctx context.Context, root string, passes int, progress io.Writer, execute attemptFn, tune func(*RecurseCycleConfig)) (*RecurseCycleResult, error) {
	t.Helper()
	k, err := core.NewRealKernelWithWorkspace(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	cfg := RecurseCycleConfig{Workspace: root, Kernel: k, Execute: ownWrites(root, execute), Passes: passes, Progress: progress}
	if tune != nil {
		tune(&cfg)
	}
	return RunRecurseCycles(ctx, cfg)
}

func stepsOf(recs []recurseRecord, step string) []recurseRecord {
	var out []recurseRecord
	for _, r := range recs {
		if r.Step == step {
			out = append(out, r)
		}
	}
	return out
}

func indexOfStep(recs []recurseRecord, step string) int {
	return slices.IndexFunc(recs, func(r recurseRecord) bool { return r.Step == step })
}

func isAncestor(g recurseGit, commit string) bool {
	_, err := g.run(context.Background(), nil, "merge-base", "--is-ancestor", commit, "HEAD")
	return err == nil
}

// A base commit that lands after the run started is merged at the next
// boundary, after the attempt in flight was judged and committed; the sweep
// graph is derived again, so a package the base added is visited in the pass
// that was already running.
func TestRecurseCycles_AbsorbsABaseCommitLandedAfterStart(t *testing.T) {
	root := recurseFixture(t, nil)
	g := recurseGit{root: root}
	var tip string
	progress := &lineHook{}
	res, err := runCycles(t, context.Background(), root, 1, progress, func(ctx context.Context, a RecurseAttempt) error {
		if a.Angle == "" {
			write(t, root, "store/store.go", fixedStore)
		}
		if tip == "" {
			tip = landOnBase(t, g, "main", map[string]string{"extra/extra.go": "package extra\n\nfunc One() int { return 1 }\n"}, "base adds the extra package")
		}
		return nil
	}, nil)
	t.Logf("progress:\n%s", progress)
	if err != nil {
		t.Fatalf("RunRecurseCycles: %v", err)
	}
	if res.Kept == 0 {
		t.Fatal("the fix was not kept")
	}
	if !isAncestor(g, tip) {
		t.Fatal("the base commit that landed during the run is not in the recurse branch")
	}
	if got := mustRead(t, root, "extra/extra.go"); !strings.Contains(got, "One()") {
		t.Fatalf("the base's file is not in the tree: %q", got)
	}
	recs := journalOf(t, root)
	base := stepsOf(recs, stepBase)
	if len(base) != 1 || base[0].Base != "main" || !strings.Contains(base[0].Detail, baseFromStart) {
		t.Fatalf("the ledger records the base the run started from: %+v", base)
	}
	absorbs := stepsOf(recs, stepAbsorb)
	if len(absorbs) != 1 || absorbs[0].Commit != tip || absorbs[0].Base != "main" || !slices.Contains(absorbs[0].Nodes, "extra") {
		t.Fatalf("absorb records = %+v; want one for %s touching node extra", absorbs, tip)
	}
	if first := indexOfStep(recs, stepRatchet); first < 0 || indexOfStep(recs, stepAbsorb) < first {
		t.Fatal("the absorb happened before the attempt in flight was judged")
	}
	visited := false
	for _, v := range stepsOf(recs, stepVisit) {
		visited = visited || v.Node == "extra"
	}
	if !visited {
		t.Fatal("the package the base added was not visited: the sweep graph was not derived again after the absorb")
	}
	if !strings.Contains(progress.String(), "absorbed base") {
		t.Fatalf("the absorb is not in the log:\n%s", progress)
	}
	if st, _ := ReadRecurseStatus(root); st.Base.Branch != "main" || st.Base.LastAbsorbed != tip || st.Base.Absorbs != 1 {
		t.Fatalf("status = %+v; want the base, the absorbed commit and one absorb", st.Base)
	}
}

// A conflicting base commit leaves the tree as it was and is recorded; the run
// goes on, does not retry the same base commit at every boundary, and merges
// once the base has moved to something that fits.
func TestRecurseCycles_ConflictingBaseIsRecordedAndRetriedWhenBaseMoves(t *testing.T) {
	root := recurseFixture(t, nil)
	g := recurseGit{root: root}
	var conflicting, fitting string
	progress := &lineHook{}
	progress.onLine = func(line string) {
		if !strings.HasPrefix(line, "recurse pass 1:") || fitting != "" {
			return
		}
		recs := journalOf(t, root)
		if n := len(stepsOf(recs, stepAbsorbConflict)); n != 1 {
			t.Errorf("at the second pass the conflict is recorded once, not retried at every boundary: %d records", n)
		}
		if isAncestor(g, conflicting) {
			t.Error("the conflicting base commit is in the tree")
		}
		if status := gitOut(t, g, "status", "--porcelain", "--untracked-files=no"); status != "" {
			t.Errorf("the conflicted merge left changes behind: %q", status)
		}
		if got := mustRead(t, root, "store/store.go"); got != fixedStore {
			t.Errorf("store.go = %q after the conflict", got)
		}
		// The base moves on to the same fix the loop kept: it now merges.
		fitting = landOnBase(t, g, "main", map[string]string{"store/store.go": fixedStore}, "base lands the same fix")
	}
	res, err := runCycles(t, context.Background(), root, 2, progress, func(ctx context.Context, a RecurseAttempt) error {
		if a.Angle == "" {
			write(t, root, "store/store.go", fixedStore)
		}
		if conflicting == "" {
			conflicting = landOnBase(t, g, "main", map[string]string{"store/store.go": "package store\n\nfunc Get() int { return 3 }\n"}, "base fixes it differently")
		}
		return nil
	}, nil)
	t.Logf("progress:\n%s", progress)
	if err != nil {
		t.Fatalf("RunRecurseCycles: %v", err)
	}
	if res.Passes != 2 {
		t.Fatalf("the run stopped after %d passes: a conflict must not end it", res.Passes)
	}
	recs := journalOf(t, root)
	conflicts := stepsOf(recs, stepAbsorbConflict)
	if len(conflicts) != 1 || conflicts[0].Commit != conflicting || !strings.Contains(conflicts[0].Detail, "store/store.go") {
		t.Fatalf("conflict records = %+v; want one for %s naming store/store.go", conflicts, conflicting)
	}
	absorbs := stepsOf(recs, stepAbsorb)
	if len(absorbs) != 1 || absorbs[0].Commit != fitting {
		t.Fatalf("absorb records = %+v; want one for the retry at %s", absorbs, fitting)
	}
	if !isAncestor(g, fitting) || !isAncestor(g, conflicting) {
		t.Fatal("after the retry the base's history is in the branch")
	}
	if got := mustRead(t, root, "store/store.go"); got != fixedStore {
		t.Fatalf("store.go = %q", got)
	}
	if !strings.Contains(progress.String(), "WARN") {
		t.Fatalf("a conflict is a WARN in the log:\n%s", progress)
	}
	if st, _ := ReadRecurseStatus(root); len(st.Base.Conflicts) != 1 || !strings.Contains(st.String(), "Merge conflicts: 1") {
		t.Fatalf("status does not show the conflict:\n%s", st)
	}
}

// newAbsorbRun is a run positioned between visits, over the fixture module,
// without the loop: the pieces absorbed() acts on.
func newAbsorbRun(t *testing.T) *recurseRun {
	t.Helper()
	root := recurseFixture(t, nil)
	r := &recurseRun{
		cfg: RecurseCycleConfig{}, root: root, branch: DefaultRecurseBranch, git: recurseGit{root: root},
		policy: *newRecursePolicy(t), out: io.Discard, state: map[string]gateRun{}, seenThisPass: map[string]bool{},
		base: &recurseBaseState{name: "main"},
	}
	journal, err := openRecurseJournal(root)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = journal.close() })
	r.journal = journal
	return r
}

func stallTwice(t *testing.T, p *recursePolicy, finding, node string, cycle int) {
	t.Helper()
	for c := cycle; c < cycle+2; c++ {
		if err := p.attempt(finding, node, c, outcomeReverted, "same failure"); err != nil {
			t.Fatal(err)
		}
	}
}

func stalledNow(t *testing.T, p *recursePolicy) []string {
	t.Helper()
	got, err := p.stalled()
	if err != nil {
		t.Fatal(err)
	}
	return got
}

// After an absorb what was derived from the tree is gone, and the findings
// stalled on nodes whose files changed are retried; others stay stalled. The
// ledger carries the nodes, so a resumed run lifts the same stalls.
func TestRecurseAbsorb_InvalidatesDerivedStateAndLiftsStallsOnChangedNodes(t *testing.T) {
	r := newAbsorbRun(t)
	ctx := context.Background()
	stallTwice(t, &r.policy, "store-finding", "store", 1)
	stallTwice(t, &r.policy, "web-finding", "web", 3)
	if got := stalledNow(t, &r.policy); !slices.Equal(got, []string{"store-finding", "web-finding"}) {
		t.Fatalf("stalled before the absorb = %v", got)
	}
	r.cycle = 4
	r.state["g@store"] = gateRun{snapshot: "old"}
	r.gateBaseline = &gates.Snapshot{ID: "old"}
	r.efGroups = map[string]gates.FindingGroup{"x": {}}
	r.cyclesSinceFull = 5

	res := absorbResult{Outcome: absorbMerged, Commits: 2, BaseTip: "b4se", Before: "b3f0re", After: "af7er", Changed: []string{"store/store.go", "docs/notes.txt"}}
	if err := r.absorbed(ctx, 0, res); err != nil {
		t.Fatal(err)
	}
	if len(r.state) != 0 || r.gateBaseline != nil || r.efGroups != nil || r.cyclesSinceFull != 0 {
		t.Fatalf("derived state survived the absorb: state %d, baseline %v, groups %v, since full %d", len(r.state), r.gateBaseline, r.efGroups, r.cyclesSinceFull)
	}
	if got := stalledNow(t, &r.policy); !slices.Equal(got, []string{"web-finding"}) {
		t.Fatalf("stalled after the absorb = %v; want only the finding on the node the merge did not touch", got)
	}
	if r.cycle != 5 {
		t.Fatalf("cycle = %d: the absorb takes the next cycle number, above every attempt", r.cycle)
	}
	recs, err := readRecurseJournal(r.root)
	if err != nil {
		t.Fatal(err)
	}
	absorbs := stepsOf(recs, stepAbsorb)
	if len(absorbs) != 1 || !slices.Equal(absorbs[0].Nodes, []string{"store"}) || absorbs[0].Cycle != 5 {
		t.Fatalf("absorb records = %+v", absorbs)
	}

	// A resumed run replays the ledger: the same stall is lifted.
	history := []recurseRecord{
		{Step: stepRatchet, Finding: "store-finding", Node: "store", Cycle: 1, Outcome: outcomeReverted, Signature: "same failure"},
		{Step: stepRatchet, Finding: "store-finding", Node: "store", Cycle: 2, Outcome: outcomeReverted, Signature: "same failure"},
		{Step: stepRatchet, Finding: "web-finding", Node: "web", Cycle: 3, Outcome: outcomeReverted, Signature: "same failure"},
		{Step: stepRatchet, Finding: "web-finding", Node: "web", Cycle: 4, Outcome: outcomeReverted, Signature: "same failure"},
	}
	replay := func(extra ...recurseRecord) []string {
		t.Helper()
		resumed := &recurseRun{policy: *newRecursePolicy(t)}
		if _, _, err := resumed.resume(append(slices.Clone(history), extra...)); err != nil {
			t.Fatal(err)
		}
		return stalledNow(t, &resumed.policy)
	}
	if got := replay(); !slices.Equal(got, []string{"store-finding", "web-finding"}) {
		t.Fatalf("without the absorb record both stay stalled: %v", got)
	}
	if got := replay(absorbs...); !slices.Equal(got, []string{"web-finding"}) {
		t.Fatalf("resume does not replay the absorb: stalled %v", got)
	}
}

// A go.mod change touches every node; a path no node holds touches none.
func TestRecurseAbsorb_TouchedNodes(t *testing.T) {
	r := newAbsorbRun(t)
	ctx := context.Background()
	if got := r.touchedNodes(ctx, []string{"README.md", "docs/a.txt"}); len(got) != 0 {
		t.Fatalf("prose touches no node: %v", got)
	}
	all := r.touchedNodes(ctx, []string{"go.mod"})
	if !slices.Contains(all, "store") || !slices.Contains(all, "web") {
		t.Fatalf("go.mod touches every node: %v", all)
	}
	if got := r.touchedNodes(ctx, []string{"web/web.go"}); !slices.Equal(got, []string{"web"}) {
		t.Fatalf("web/web.go touches web: %v", got)
	}
}

// The base a run records is the base a resumed run keeps; the flag replaces it.
func TestRecurseAbsorb_StartBaseRecordsAndResumeKeepsIt(t *testing.T) {
	r := newAbsorbRun(t)
	ctx := context.Background()
	gitOut(t, r.git, "branch", "release")
	r.cfg.BaseBranch = "release"
	if err := r.startBase(ctx, nil, "main"); err != nil {
		t.Fatal(err)
	}
	if r.base == nil || r.base.name != "release" || r.base.source != baseFromFlag {
		t.Fatalf("base = %+v", r.base)
	}
	recs, _ := readRecurseJournal(r.root)
	if got := recordedBase(recs); got != "release" {
		t.Fatalf("the ledger records %q", got)
	}
	r.cfg.BaseBranch, r.base = "", nil
	if err := r.startBase(ctx, recs, "main"); err != nil {
		t.Fatal(err)
	}
	if r.base == nil || r.base.name != "release" || r.base.source != baseFromLedger {
		t.Fatalf("a resumed run keeps the recorded base: %+v", r.base)
	}
	r.cfg.BaseBranch, r.base = "main", nil
	if err := r.startBase(ctx, recs, "main"); err != nil || r.base.name != "main" {
		t.Fatalf("the flag replaces the recorded base: %+v, %v", r.base, err)
	}
}

// A mission run is bounded and its evidence is bound to the tree it measured:
// it does not absorb.
func TestRecurseAbsorb_MissionRunsDoNotAbsorb(t *testing.T) {
	r := newAbsorbRun(t)
	r.cfg.Mission = &RecurseMission{}
	r.base = nil
	if err := r.startBase(context.Background(), nil, "main"); err != nil || r.base != nil {
		t.Fatalf("base = %+v, %v; a mission keeps the tree it measured", r.base, err)
	}
	r.cfg.SelfUpdate = &RecurseSelfUpdate{}
	r.startSelf()
	if r.self != nil {
		t.Fatal("a mission run must not restart onto a rebuilt binary")
	}
}
