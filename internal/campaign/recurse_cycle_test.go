package campaign

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"codenerd/internal/core"
	"codenerd/internal/gates"
)

// The fixture every end-to-end recurse test starts from: a Go module in a git
// repository, one node red (store's test wants 2, Get returns 1), one node
// that imports it (web), .nerd ignored like a real workspace.
var recurseModule = map[string]string{
	"go.mod":              "module example.com/m\n\ngo 1.21\n",
	".gitignore":          ".nerd/\n",
	"store/store.go":      "package store\n\nfunc Get() int { return 1 }\n",
	"store/store_test.go": "package store\n\nimport \"testing\"\n\nfunc TestGet(t *testing.T) {\n\tif Get() != 2 {\n\t\tt.Fatal(\"Get() != 2\")\n\t}\n}\n",
	"web/web.go":          "package web\n\nimport \"example.com/m/store\"\n\nfunc Page() int { return store.Get() }\n",
}

const fixedStore = "package store\n\nfunc Get() int { return 2 }\n"

func recurseFixture(t *testing.T, extra map[string]string) string {
	t.Helper()
	if _, err := exec.LookPath("go"); err != nil {
		t.Skip("no go toolchain")
	}
	files := map[string]string{}
	for k, v := range recurseModule {
		files[k] = v
	}
	for k, v := range extra {
		files[k] = v
	}
	return gitFixture(t, files).root
}

func runRecurse(t *testing.T, ctx context.Context, root string, passes int, execute RecurseExecutor) (*RecurseCycleResult, error) {
	t.Helper()
	k, err := core.NewRealKernelWithWorkspace(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	var progress strings.Builder
	res, err := RunRecurseCycles(ctx, RecurseCycleConfig{
		Workspace: root, Kernel: k, Execute: execute, Passes: passes, Progress: &progress,
	})
	t.Logf("progress:\n%s", progress.String())
	return res, err
}

func journalOf(t *testing.T, root string) []recurseRecord {
	t.Helper()
	recs, err := readRecurseJournal(root)
	if err != nil {
		t.Fatal(err)
	}
	return recs
}

// ratchets are the fix attempts' verdicts; improvement verdicts are
// improvements(recs).
func ratchets(recs []recurseRecord) []recurseRecord {
	var out []recurseRecord
	for _, r := range recs {
		if r.Step == stepRatchet && r.Angle == "" {
			out = append(out, r)
		}
	}
	return out
}

func improvements(recs []recurseRecord) []recurseRecord {
	var out []recurseRecord
	for _, r := range recs {
		if r.Step == stepRatchet && r.Angle != "" {
			out = append(out, r)
		}
	}
	return out
}

// onlyFixes runs fn for fix attempts and leaves improvement attempts
// untouched, so they revert as changing nothing.
func onlyFixes(fn RecurseExecutor) RecurseExecutor {
	return func(ctx context.Context, a RecurseAttempt) error {
		if a.Angle != "" {
			return nil
		}
		return fn(ctx, a)
	}
}

func write(t *testing.T, root, rel, body string) {
	t.Helper()
	writeTree(t, root, map[string]string{rel: body})
}

func mustRead(t *testing.T, root, rel string) string {
	t.Helper()
	got, _ := readFile(t, root, rel)
	return got
}

func headSubject(t *testing.T, root string) string {
	t.Helper()
	return gitOut(t, recurseGit{root: root}, "log", "-1", "--format=%s%n%b")
}

// A fixing attempt is kept and committed on the loop's branch, with the
// trailers a resumed run reads.
func TestRecurseCycles_AFixIsKeptAndCommitted(t *testing.T) {
	root := recurseFixture(t, nil)
	var attempts []RecurseAttempt
	res, err := runRecurse(t, context.Background(), root, 1, onlyFixes(func(ctx context.Context, a RecurseAttempt) error {
		attempts = append(attempts, a)
		write(t, root, "store/store.go", fixedStore)
		return nil
	}))
	if err != nil {
		t.Fatal(err)
	}
	if len(attempts) != 1 || attempts[0].Node.ID != "store" || attempts[0].Finding.Target != "store::TestGet" {
		t.Fatalf("attempts = %+v", attempts)
	}
	a := attempts[0]
	if !strings.Contains(a.Evidence, "Get() != 2") || !slices.Contains(a.Check, "./store") {
		t.Fatalf("the attempt carries the failing output and the check that witnesses the fix: %+v", a)
	}
	if res.Kept != 1 || res.Improved != 0 || res.Passes != 1 {
		t.Fatalf("result = %+v", res)
	}
	if got := gitOut(t, recurseGit{root: root}, "rev-parse", "--abbrev-ref", "HEAD"); got != DefaultRecurseBranch {
		t.Fatalf("branch = %q", got)
	}
	if subj := headSubject(t, root); !strings.Contains(subj, "recurse: store:") || !strings.Contains(subj, recurseCycleTrailer+": 1") {
		t.Fatalf("head commit = %q", subj)
	}
	r := ratchets(journalOf(t, root))
	if len(r) != 1 || r[0].Outcome != outcomeKept || r[0].Commit == "" {
		t.Fatalf("journal ratchets = %+v", r)
	}
}

// Fixing the target while breaking another gate is worse, not better: the
// whole attempt is reverted, the other node's file included.
func TestRecurseCycles_AFixThatBreaksAnotherGateIsReverted(t *testing.T) {
	root := recurseFixture(t, nil)
	res, err := runRecurse(t, context.Background(), root, 1, onlyFixes(func(ctx context.Context, a RecurseAttempt) error {
		write(t, root, "store/store.go", fixedStore)
		write(t, root, "web/web.go", "package web\n\nfunc Page() int { return undefinedThing }\n")
		return nil
	}))
	if err != nil {
		t.Fatal(err)
	}
	if res.Kept != 0 || len(ratchets(journalOf(t, root))) != 1 {
		t.Fatalf("result = %+v", res)
	}
	if mustRead(t, root, "store/store.go") != recurseModule["store/store.go"] || mustRead(t, root, "web/web.go") != recurseModule["web/web.go"] {
		t.Fatal("a reverted attempt leaves no write behind")
	}
	r := ratchets(journalOf(t, root))
	if len(r) != 1 || r[0].Outcome != outcomeReverted || !strings.Contains(r[0].Signature, "go:build") {
		t.Fatalf("the ratchet records which gate got worse: %+v", r)
	}
}

// An attempt that does nothing is reverted; when the next pass's attempt ends
// the same way, the finding stalls and the third pass leaves it alone.
func TestRecurseCycles_ANoOpAttemptStallsItsFinding(t *testing.T) {
	root := recurseFixture(t, nil)
	calls := 0
	res, err := runRecurse(t, context.Background(), root, 3, onlyFixes(func(ctx context.Context, a RecurseAttempt) error {
		calls++
		return nil
	}))
	if err != nil {
		t.Fatal(err)
	}
	if calls != 2 || len(ratchets(journalOf(t, root))) != 2 || res.Passes != 3 {
		t.Fatalf("calls = %d, result = %+v", calls, res)
	}
	if len(res.Stalled) != 1 {
		t.Fatalf("stalled = %v", res.Stalled)
	}
}

// An attempt that writes a path nerd.md forbids is refused -- reverted and
// never retried -- even when it fixed its target.
func TestRecurseCycles_AForbiddenWriteIsRefused(t *testing.T) {
	root := recurseFixture(t, map[string]string{
		"nerd.md": "---\nschema: nerd/v1\nforbid:\n  - match: web/web.go\n    reason: the loop must not edit it\n---\n",
	})
	calls := 0
	res, err := runRecurse(t, context.Background(), root, 2, onlyFixes(func(ctx context.Context, a RecurseAttempt) error {
		calls++
		write(t, root, "store/store.go", fixedStore)
		write(t, root, "web/web.go", recurseModule["web/web.go"]+"\n// touched\n")
		return nil
	}))
	if err != nil {
		t.Fatal(err)
	}
	if calls != 1 || res.Refused != 1 || len(res.Refusals) != 1 {
		t.Fatalf("calls = %d, result = %+v", calls, res)
	}
	if mustRead(t, root, "web/web.go") != recurseModule["web/web.go"] {
		t.Fatal("a refused attempt is reverted")
	}
	r := ratchets(journalOf(t, root))
	if len(r) != 1 || r[0].Outcome != outcomeRefused || !strings.Contains(r[0].Detail, "web/web.go") {
		t.Fatalf("journal = %+v", r)
	}
}

// Stopping the loop mid-attempt puts the tree back before it returns.
func TestRecurseCycles_StoppedMidAttemptRevertsBeforeReturning(t *testing.T) {
	root := recurseFixture(t, nil)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	_, err := runRecurse(t, ctx, root, 0, onlyFixes(func(ctx context.Context, a RecurseAttempt) error {
		write(t, root, "store/store.go", fixedStore)
		write(t, root, "store/half_written.go", "package store\n")
		cancel()
		return ctx.Err()
	}))
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("err = %v, want context.Canceled", err)
	}
	if mustRead(t, root, "store/store.go") != recurseModule["store/store.go"] {
		t.Fatal("an interrupted attempt's edit must be reverted")
	}
	if _, ok := readFile(t, root, "store/half_written.go"); ok {
		t.Fatal("an interrupted attempt's new file must be removed")
	}
	if _, err := os.Stat(inFlightPath(root)); !os.IsNotExist(err) {
		t.Fatalf("the in-flight record is cleared once the tree is back: %v", err)
	}
}

// A run killed mid-attempt (no chance to revert) is settled on the next
// start: the attempt's writes are put back, the owner's are not touched, the
// cycle count continues, and the interrupted pass resumes where it was.
func TestRecurseCycles_ResumeSettlesAKilledAttempt(t *testing.T) {
	root := recurseFixture(t, nil)
	g := recurseGit{root: root}
	if err := g.prepare(context.Background(), DefaultRecurseBranch); err != nil {
		t.Fatal(err)
	}
	write(t, root, "owner_notes.txt", "mine\n")
	// The killed run's footprint: a journal through an attempt, the in-flight
	// record, and the attempt's half-done writes.
	j, err := openRecurseJournal(root)
	if err != nil {
		t.Fatal(err)
	}
	for _, rec := range []recurseRecord{
		{Step: stepPassStart, Pass: 0},
		{Step: stepVisit, Pass: 0, Node: "store"},
		{Step: stepAttempt, Pass: 0, Cycle: 1, Node: "store", Finding: "dead-finding"},
	} {
		if err := j.append(rec); err != nil {
			t.Fatal(err)
		}
	}
	if err := j.close(); err != nil {
		t.Fatal(err)
	}
	if err := writeInFlight(root, inFlight{Cycle: 1, Pass: 0, Node: "store", Finding: "dead-finding", UntrackedBefore: []string{"owner_notes.txt"}}); err != nil {
		t.Fatal(err)
	}
	write(t, root, "web/web.go", "package web\n// half an edit\n")
	write(t, root, "web/scratch.go", "package web\n")

	var cycles []int
	res, err := runRecurse(t, context.Background(), root, 1, onlyFixes(func(ctx context.Context, a RecurseAttempt) error {
		cycles = append(cycles, a.Cycle)
		write(t, root, "store/store.go", fixedStore)
		return nil
	}))
	if err != nil {
		t.Fatal(err)
	}
	if mustRead(t, root, "web/web.go") != recurseModule["web/web.go"] {
		t.Fatal("the killed attempt's edit is reverted on restart")
	}
	if _, ok := readFile(t, root, "web/scratch.go"); ok {
		t.Fatal("the killed attempt's new file is removed on restart")
	}
	if mustRead(t, root, "owner_notes.txt") != "mine\n" {
		t.Fatal("the owner's untracked file is not the loop's to remove")
	}
	if !slices.Equal(cycles, []int{2}) || res.Kept != 1 {
		t.Fatalf("the cycle count continues past the killed one: cycles %v, result %+v", cycles, res)
	}
	r := ratchets(journalOf(t, root))
	if len(r) != 2 || r[0].Cycle != 1 || r[0].Outcome != outcomeReverted || r[1].Cycle != 2 || r[1].Outcome != outcomeKept {
		t.Fatalf("journal ratchets = %+v", r)
	}
}

// A run killed after its commit landed but before the journal heard of it is
// settled as kept: the change is neither lost nor repeated.
func TestRecurseCycles_ResumeFindsAKeptCommitTheJournalMissed(t *testing.T) {
	root := recurseFixture(t, nil)
	g := recurseGit{root: root}
	ctx := context.Background()
	if err := g.prepare(ctx, DefaultRecurseBranch); err != nil {
		t.Fatal(err)
	}
	write(t, root, "store/store.go", fixedStore)
	hash, err := g.commit(ctx, []string{"store/store.go"}, "recurse: store: fix", 1, "the-finding")
	if err != nil {
		t.Fatal(err)
	}
	if err := writeInFlight(root, inFlight{Cycle: 1, Pass: 0, Node: "store", Finding: "the-finding"}); err != nil {
		t.Fatal(err)
	}
	calls := 0
	if _, err := runRecurse(t, ctx, root, 1, onlyFixes(func(ctx context.Context, a RecurseAttempt) error {
		calls++
		return nil
	})); err != nil {
		t.Fatal(err)
	}
	if mustRead(t, root, "store/store.go") != fixedStore {
		t.Fatal("the kept commit's change survives the restart")
	}
	if calls != 0 {
		t.Fatalf("the tree is green after the kept fix; nothing to attempt, got %d attempts", calls)
	}
	r := ratchets(journalOf(t, root))
	if len(r) == 0 || r[0].Cycle != 1 || r[0].Outcome != outcomeKept || r[0].Commit != hash {
		t.Fatalf("journal ratchets = %+v", r)
	}
}

// A workspace in another language gets the same loop: the gates are the
// workspace's, not Go's.
func TestRecurseCycles_ANonGoWorkspaceUsesItsDeclaredGates(t *testing.T) {
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("no git")
	}
	// A gate every platform can run: a Go program the workspace declares as
	// its test, reading a data file -- the point is that nothing here is
	// detected from go.mod.
	checker := "package main\n\nimport (\n\t\"os\"\n\t\"fmt\"\n)\n\nfunc main() {\n\tb, _ := os.ReadFile(os.Args[1] + \"/version.txt\")\n\tif string(b) != \"2\\n\" {\n\t\tfmt.Println(os.Args[1] + \"/version.txt:1: want 2, got \" + string(b))\n\t\tos.Exit(1)\n\t}\n}\n"
	g := gitFixture(t, map[string]string{
		".gitignore":          ".nerd/\n",
		"tools/check/main.go": checker,
		"svc/__init__.py":     "",
		"svc/version.txt":     "1\n",
		"nerd.md":             "---\nschema: nerd/v1\ngates:\n  - id: version\n    kind: test\n    run: go run ./tools/check/main.go {node}\n    scope: node\n---\n",
	})
	root := g.root
	var targets []string
	res, err := runRecurse(t, context.Background(), root, 1, onlyFixes(func(ctx context.Context, a RecurseAttempt) error {
		targets = append(targets, a.Node.ID+" "+a.Finding.Target)
		write(t, root, "svc/version.txt", "2\n")
		return nil
	}))
	if err != nil {
		t.Fatal(err)
	}
	if res.Kept != 1 || len(targets) != 1 || targets[0] != "svc svc/version.txt" {
		t.Fatalf("targets %v, result %+v", targets, res)
	}
}

func TestRecurseCycles_RefusesADirtyCheckout(t *testing.T) {
	root := recurseFixture(t, nil)
	write(t, root, "web/web.go", "package web\n// the owner's uncommitted work\n")
	_, err := runRecurse(t, context.Background(), root, 1, func(ctx context.Context, a RecurseAttempt) error {
		t.Fatal("no attempt may run over the owner's uncommitted work")
		return nil
	})
	if err == nil || !strings.Contains(err.Error(), "uncommitted") {
		t.Fatalf("err = %v", err)
	}
	if mustRead(t, root, "web/web.go") != "package web\n// the owner's uncommitted work\n" {
		t.Fatal("the owner's work is untouched")
	}
}

func TestInFlight_RoundTrips(t *testing.T) {
	root := t.TempDir()
	want := inFlight{Cycle: 3, Pass: 1, Node: "n", Finding: "f", UntrackedBefore: []string{"a", "b"}}
	if err := writeInFlight(root, want); err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(filepath.Join(root, ".nerd", "recurse", "inflight.json"))
	if err != nil {
		t.Fatal(err)
	}
	var got inFlight
	if err := json.Unmarshal(data, &got); err != nil {
		t.Fatal(err)
	}
	if got.Cycle != 3 || got.Node != "n" || !slices.Equal(got.UntrackedBefore, want.UntrackedBefore) {
		t.Fatalf("round trip = %+v", got)
	}
}

// Every visit, red or green, ends with an improvement attempt from the pass's
// angle. One that adds a passing test is kept; one that moves nothing is
// reverted. Cross-cutting nodes have no metrics of their own and are skipped.
func TestRecurseCycles_EveryVisitImprovesAndKeepsOnlyMeasuredGains(t *testing.T) {
	root := recurseFixture(t, nil)
	var seen []string
	res, err := runRecurse(t, context.Background(), root, 1, func(ctx context.Context, a RecurseAttempt) error {
		if a.Angle == "" {
			write(t, root, "store/store.go", fixedStore)
			return nil
		}
		seen = append(seen, a.Node.ID+":"+a.Angle)
		if a.Metrics[gates.MetricTests] < 1 || a.Metrics[gates.MetricLines] < 1 {
			t.Errorf("an improvement is handed the numbers it must move: %v", a.Metrics)
		}
		switch a.Node.ID {
		case "store":
			write(t, root, "store/more_test.go", "package store\n\nimport \"testing\"\n\nfunc TestGetIsStable(t *testing.T) {\n\tif Get() != Get() {\n\t\tt.Fatal(\"unstable\")\n\t}\n}\n")
		case "web":
			write(t, root, "web/web.go", recurseModule["web/web.go"]+"\n// a comment moves no metric\n")
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	if want := []string{"store:stabilize", "web:stabilize"}; !slices.Equal(seen, want) {
		t.Fatalf("improvement attempts = %v, want %v", seen, want)
	}
	if res.Kept != 2 || res.Improved != 1 {
		t.Fatalf("the fix and the store improvement are kept: %+v", res)
	}
	imp := improvements(journalOf(t, root))
	if len(imp) != 2 || imp[0].Outcome != outcomeKept || !strings.Contains(imp[0].Metrics, "tests 1->2") || imp[1].Outcome != outcomeReverted {
		t.Fatalf("improvement verdicts = %+v", imp)
	}
	if mustRead(t, root, "web/web.go") != recurseModule["web/web.go"] {
		t.Fatal("an improvement that moved nothing is reverted")
	}
	if subj := headSubject(t, root); !strings.Contains(subj, "recurse: store: stabilize") || !strings.Contains(subj, recurseFindingTrailer+": improve:stabilize") {
		t.Fatalf("head commit = %q", subj)
	}
}

// The simplify pass keeps a change that drops a node's lines at equal
// behaviour, and reverts one that gets there by deleting tests.
func TestRecurseCycles_SimplifyKeepsFewerLinesButNeverFewerTests(t *testing.T) {
	root := recurseFixture(t, map[string]string{
		"store/store.go":  "package store\n\nfunc Get() int { return 2 }\n\nfunc unused() int {\n\treturn 3\n}\n",
		"web/web_test.go": "package web\n\nimport \"testing\"\n\nfunc TestPage(t *testing.T) {\n\tif Page() != 2 {\n\t\tt.Fatal(\"Page\")\n\t}\n}\n",
	})
	res, err := runRecurse(t, context.Background(), root, 3, func(ctx context.Context, a RecurseAttempt) error {
		if a.Angle != "simplify" {
			return nil
		}
		switch a.Node.ID {
		case "store":
			write(t, root, "store/store.go", fixedStore)
		case "web":
			write(t, root, "web/web.go", "package web\n\nfunc Page() int { return 2 }\n")
			if err := os.Remove(filepath.Join(root, "web", "web_test.go")); err != nil {
				t.Fatal(err)
			}
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	if res.Improved != 1 {
		t.Fatalf("only the store simplification is kept: %+v", res)
	}
	if mustRead(t, root, "store/store.go") != fixedStore {
		t.Fatal("the dead code is gone")
	}
	if _, ok := readFile(t, root, "web/web_test.go"); !ok {
		t.Fatal("a simplification that deletes a test is reverted, test and all")
	}
	var simplify []recurseRecord
	for _, r := range improvements(journalOf(t, root)) {
		if r.Angle == "simplify" {
			simplify = append(simplify, r)
		}
	}
	if len(simplify) != 2 || simplify[0].Outcome != outcomeKept || simplify[1].Outcome != outcomeReverted || !strings.Contains(simplify[1].Metrics, "tests 2->1") {
		t.Fatalf("simplify verdicts = %+v", simplify)
	}
}
