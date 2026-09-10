package prompt

import (
	"context"
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"codenerd/internal/jsonl"
	"codenerd/internal/usage"
)

// coUseTestCompiler builds a compiler over a small embedded corpus so that a
// compilation actually yields atoms. A bare compiler selects nothing here, and
// a test that skips when there is nothing to record proves nothing at all.
func coUseTestCompiler(t *testing.T) *JITPromptCompiler {
	t.Helper()

	corpus := NewEmbeddedCorpus([]*PromptAtom{
		{ID: "identity/core", Category: CategoryIdentity, Content: "You are codenerd.",
			Priority: 100, IsMandatory: true, TokenCount: 8},
		{ID: "knowledge/go", Category: CategoryKnowledge, Content: "Go error handling.",
			Priority: 80, IsMandatory: true, TokenCount: 8},
	})

	// The compiler requires a kernel for skeleton selection; the mock returns
	// the selected_atom facts the real Mangle rules would derive.
	kernel := &mockKernel{
		facts: []any{
			Fact{Predicate: "selected_atom", Args: []any{"identity/core", "skeleton", 1.0}},
			Fact{Predicate: "selected_atom", Args: []any{"knowledge/go", "skeleton", 0.9}},
		},
	}

	compiler, err := NewJITPromptCompiler(WithEmbeddedCorpus(corpus), WithKernel(kernel))
	if err != nil {
		t.Fatalf("NewJITPromptCompiler: %v", err)
	}
	t.Cleanup(func() { _ = compiler.Close() })
	return compiler
}

// promptRepoRoot walks up to the module root from the package directory.
func promptRepoRoot(t *testing.T) string {
	t.Helper()
	dir, err := os.Getwd()
	if err != nil {
		t.Fatalf("getwd: %v", err)
	}
	for i := 0; i < 10; i++ {
		if _, err := os.Stat(filepath.Join(dir, "go.mod")); err == nil {
			return dir
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			break
		}
		dir = parent
	}
	t.Fatal("could not locate go.mod")
	return ""
}

// TestCompileRecordsOnCacheHitsToo is the property the Compile wrapper exists
// for. Two of the three success paths return a result selected on an earlier
// call, and those turns used those atoms just as much as the turn that
// compiled them. A recorder that only saw cache misses would produce a sample
// biased toward whichever contexts happen to be novel -- which is a bias
// toward exactly the unusual turns a taxonomy should not be built on.
func TestCompileRecordsOnCacheHitsToo(t *testing.T) {
	compiler := coUseTestCompiler(t)

	rec := NewCoUseRecorder()
	restore := swapCoUse(rec)
	defer restore()

	cc := NewCompilationContext().WithTokenBudget(10000, 1000)

	for i := 0; i < 3; i++ {
		ctx := usage.WithTurnID(context.Background(), "turn-1")
		result, err := compiler.Compile(ctx, cc)
		if err != nil {
			t.Fatalf("compile %d: %v", i, err)
		}
		if result == nil || len(result.IncludedAtoms) == 0 {
			t.Fatalf("compile %d produced no atoms; the fixture is not exercising the recorder", i)
		}
	}

	if got := rec.Report(DefaultCoUseParams(), nil).PendingTurns; got != 1 {
		t.Fatalf("pending turns = %d, want 1", got)
	}

	rec.Settle("turn-1", OutcomeSuccess)
	params := DefaultCoUseParams()
	params.MinSupport = 1
	rep := rec.Report(params, rec.Categories())

	if rep.SuccessSelections != 3 {
		t.Fatalf("selections = %d, want 3 — a cache hit is still a use", rep.SuccessSelections)
	}
	if rep.DistinctAtoms == 0 {
		t.Fatal("no atoms recorded")
	}
}

func TestCompileWithoutATurnIdIsCountedNotHidden(t *testing.T) {
	compiler := coUseTestCompiler(t)

	rec := NewCoUseRecorder()
	restore := swapCoUse(rec)
	defer restore()

	cc := NewCompilationContext().WithTokenBudget(10000, 1000)
	result, err := compiler.Compile(context.Background(), cc)
	if err != nil {
		t.Fatalf("compile: %v", err)
	}
	if result == nil || len(result.IncludedAtoms) == 0 {
		t.Fatal("compile produced no atoms; the fixture is not exercising the recorder")
	}

	// An untagged compilation can never be settled, so it is not evidence --
	// but a large count here means a whole class of compilations is invisible
	// to the analysis, which is a wiring gap rather than an absence of
	// clustering. It must show up as a number.
	rep := rec.Report(DefaultCoUseParams(), nil)
	if rep.UnattributedSelections != 1 {
		t.Fatalf("unattributed = %d, want 1", rep.UnattributedSelections)
	}
	if rep.PendingTurns != 0 {
		t.Fatalf("pending = %d, want 0 — an untagged selection must not sit in the queue forever",
			rep.PendingTurns)
	}
}

func TestObserveAtomsCapturesCategoriesAtSelectionTime(t *testing.T) {
	rec := NewCoUseRecorder()

	atoms := []*PromptAtom{
		{ID: "a1", Category: CategoryIdentity},
		{ID: "b1", Category: CategoryKnowledge},
		{ID: "", Category: CategoryIdentity}, // skipped: no id
		nil,                                  // skipped: nil entry
	}
	rec.ObserveAtoms("t1", atoms)
	rec.Settle("t1", OutcomeSuccess)

	lookup := rec.Categories()
	if got := lookup("a1"); got != string(CategoryIdentity) {
		t.Fatalf("category(a1) = %q, want %q", got, CategoryIdentity)
	}
	if got := lookup("b1"); got != string(CategoryKnowledge) {
		t.Fatalf("category(b1) = %q, want %q", got, CategoryKnowledge)
	}
	if got := lookup("never-seen"); got != "" {
		t.Fatalf("category of an unknown atom = %q, want empty", got)
	}

	params := DefaultCoUseParams()
	params.MinSupport = 1
	params.MinLift = 0
	params.UbiquityThreshold = 2
	rep := rec.Report(params, lookup)
	if rep.DistinctAtoms != 2 {
		t.Fatalf("distinct atoms = %d, want 2 (nil and empty-id entries are skipped)", rep.DistinctAtoms)
	}
}

func TestCategoryCaptureKeepsTheValueFromSelectionTime(t *testing.T) {
	rec := NewCoUseRecorder()

	rec.ObserveAtoms("t1", []*PromptAtom{{ID: "a", Category: CategoryIdentity}})
	rec.Settle("t1", OutcomeSuccess)
	// The same atom recategorized later. The evidence must keep the category
	// the selection was actually made under; a later corpus edit rewriting the
	// history would make two runs incomparable for reasons invisible in either.
	rec.ObserveAtoms("t2", []*PromptAtom{{ID: "a", Category: CategoryKnowledge}})
	rec.Settle("t2", OutcomeSuccess)

	if got := rec.Categories()("a"); got != string(CategoryIdentity) {
		t.Fatalf("category(a) = %q, want the selection-time value %q", got, CategoryIdentity)
	}
}

// ---------------------------------------------------------------------------
// Wiring
// ---------------------------------------------------------------------------

// TestOutcomeIsSettledByTheExecutor checks the other half of the loop. The
// compiler records selections; only the session executor knows whether the turn
// worked. If that call is removed, nothing fails at compile time and nothing
// fails at runtime -- the recorder simply fills with pending turns that are
// never settled, and every report comes back empty with no error anywhere.
func TestOutcomeIsSettledByTheExecutor(t *testing.T) {
	const rel = "internal/session/executor.go"
	path := filepath.Join(promptRepoRoot(t), rel)

	fset := token.NewFileSet()
	file, err := parser.ParseFile(fset, path, nil, 0)
	if err != nil {
		t.Fatalf("parse %s: %v", rel, err)
	}

	found := false
	ast.Inspect(file, func(n ast.Node) bool {
		call, ok := n.(*ast.CallExpr)
		if !ok {
			return true
		}
		sel, ok := call.Fun.(*ast.SelectorExpr)
		if !ok || sel.Sel.Name != "Settle" {
			return true
		}
		// prompt.CoUse().Settle(...)
		inner, ok := sel.X.(*ast.CallExpr)
		if !ok {
			return true
		}
		innerSel, ok := inner.Fun.(*ast.SelectorExpr)
		if !ok || innerSel.Sel.Name != "CoUse" {
			return true
		}
		pkg, ok := innerSel.X.(*ast.Ident)
		if ok && pkg.Name == "prompt" {
			found = true
		}
		return true
	})

	if !found {
		t.Fatalf("%s no longer calls prompt.CoUse().Settle; recorded selections would never "+
			"receive an outcome and every co-use report would silently come back empty", rel)
	}
}

func TestSettleUsesBothOutcomes(t *testing.T) {
	// A settle call that always passes OutcomeSuccess would report a perfect
	// success rate and fold failed turns into the evidence used to recommend
	// atom groupings. Both constants must appear at the call site.
	src, err := os.ReadFile(filepath.Join(promptRepoRoot(t), "internal/session/executor.go"))
	if err != nil {
		t.Fatalf("read executor: %v", err)
	}
	text := string(src)
	for _, want := range []string{"prompt.OutcomeSuccess", "prompt.OutcomeFailure"} {
		if !strings.Contains(text, want) {
			t.Errorf("executor never uses %s; turn outcomes are not being distinguished", want)
		}
	}
}

// ---------------------------------------------------------------------------
// Persistence
// ---------------------------------------------------------------------------

func TestSelectionLogRoundTripsThroughTheSameTallies(t *testing.T) {
	path := filepath.Join(t.TempDir(), "meter", "atom-selections.jsonl")
	log, err := jsonl.Open(path)
	if err != nil {
		t.Fatalf("open selection log: %v", err)
	}

	live := NewCoUseRecorder()
	live.SetLog(log)

	for i := 0; i < 20; i++ {
		turn := fmt.Sprintf("t%d", i)
		if i%2 == 0 {
			live.ObserveAtoms(turn, []*PromptAtom{
				{ID: "a", Category: CategoryIdentity},
				{ID: "b", Category: CategoryIdentity},
			})
			live.Settle(turn, OutcomeSuccess)
			continue
		}
		live.ObserveAtoms(turn, []*PromptAtom{
			{ID: "c", Category: CategoryKnowledge},
			{ID: "d", Category: CategoryKnowledge},
		})
		live.Settle(turn, OutcomeFailure)
	}
	if err := log.Close(); err != nil {
		t.Fatalf("close log: %v", err)
	}

	replayed, truncated, err := LoadSelections(path)
	if err != nil {
		t.Fatalf("LoadSelections: %v", err)
	}
	if truncated != 0 {
		t.Fatalf("truncated = %d, want 0", truncated)
	}

	params := DefaultCoUseParams()
	params.MinLift = 0
	params.UbiquityThreshold = 2

	want := live.Report(params, live.Categories())
	got := replayed.Report(params, replayed.Categories())

	// The replayed report must be identical to the live one. A separate ingest
	// path is how a readout starts quietly disagreeing with the process it is
	// reading, and the disagreement would look like a finding.
	if !reflect.DeepEqual(want, got) {
		t.Fatalf("replayed report differs from the live one\nlive:    %+v\nreplayed: %+v", want, got)
	}
	if got.SuccessSelections != 10 || got.FailureSelections != 10 {
		t.Fatalf("outcomes = %d success / %d failure, want 10/10",
			got.SuccessSelections, got.FailureSelections)
	}
	if cat := replayed.Categories()("a"); cat != string(CategoryIdentity) {
		t.Fatalf("category survived as %q, want %q", cat, CategoryIdentity)
	}
}

func TestSelectionLogIsOnlyWrittenOnSettle(t *testing.T) {
	path := filepath.Join(t.TempDir(), "selections.jsonl")
	log, err := jsonl.Open(path)
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	rec := NewCoUseRecorder()
	rec.SetLog(log)

	// An unsettled selection is not evidence: the whole question is about turns
	// that succeeded. Writing it at Observe would put unknown-outcome data in a
	// log whose reader has no way to tell it apart.
	rec.ObserveAtoms("pending", []*PromptAtom{{ID: "a", Category: CategoryIdentity}})
	_ = log.Close()

	records, _, err := jsonl.Read[SelectionRecord](path)
	if err != nil {
		t.Fatalf("read: %v", err)
	}
	if len(records) != 0 {
		t.Fatalf("an unsettled selection was persisted: %+v", records)
	}
}

func TestDetachedLogStopsWriting(t *testing.T) {
	path := filepath.Join(t.TempDir(), "selections.jsonl")
	log, err := jsonl.Open(path)
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	rec := NewCoUseRecorder()
	rec.SetLog(log)

	rec.ObserveAtoms("t1", []*PromptAtom{{ID: "a"}})
	rec.Settle("t1", OutcomeSuccess)

	rec.SetLog(nil)
	rec.ObserveAtoms("t2", []*PromptAtom{{ID: "b"}})
	rec.Settle("t2", OutcomeSuccess)
	_ = log.Close()

	records, _, err := jsonl.Read[SelectionRecord](path)
	if err != nil {
		t.Fatalf("read: %v", err)
	}
	if len(records) != 1 {
		t.Fatalf("records = %d, want 1 — detaching the log must stop writes", len(records))
	}
}
