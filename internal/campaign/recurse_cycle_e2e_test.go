package campaign

import (
	"context"
	"slices"
	"strings"
	"sync"
	"testing"

	"codenerd/internal/core"
	"codenerd/internal/gates"
	"codenerd/internal/tactile"
	"codenerd/internal/types"
)

// tracingKernel records the derivations the loop asks the kernel for, at the
// moment it asks. Ratchet inputs are retracted once the cycle is judged
// (recursePolicy.endCycle), so a query after the run cannot see
// recurse_ratchet; the query that decides the cycle can.
type tracingKernel struct {
	*core.RealKernel
	trace *cycleTrace
}

func (t *tracingKernel) Query(predicate string) ([]core.Fact, error) {
	rows, err := t.RealKernel.Query(predicate)
	if err != nil || t.trace == nil {
		return rows, err
	}
	switch predicate {
	case "recurse_next_node":
		t.trace.addNextNodes(rows)
	case "recurse_next":
		t.trace.addNextFindings(rows)
	case "recurse_ratchet":
		t.trace.addRatchet(t.RealKernel, rows)
	}
	return rows, nil
}

func (t *tracingKernel) peek(predicate string) ([]core.Fact, error) {
	return t.RealKernel.Query(predicate)
}

type cycleTrace struct {
	mu           sync.Mutex
	nextNodes    []string
	nextFindings []string
	ratchets     []ratchetSnap
}

type ratchetSnap struct {
	cycle    int
	verdict  string
	targetID string
	target   string
	changed  string
	worse    []string
}

func (tr *cycleTrace) addNextNodes(rows []core.Fact) {
	tr.mu.Lock()
	defer tr.mu.Unlock()
	for _, f := range rows {
		if len(f.Args) > 0 {
			tr.nextNodes = append(tr.nextNodes, types.ExtractString(f.Args[0]))
		}
	}
}

func (tr *cycleTrace) addNextFindings(rows []core.Fact) {
	tr.mu.Lock()
	defer tr.mu.Unlock()
	for _, f := range rows {
		if len(f.Args) > 0 {
			tr.nextFindings = append(tr.nextFindings, types.ExtractString(f.Args[0]))
		}
	}
}

func (tr *cycleTrace) addRatchet(k *core.RealKernel, rows []core.Fact) {
	targets, _ := k.Query("recurse_ratchet_target")
	changed, _ := k.Query("recurse_ratchet_changed")
	worse, _ := k.Query("recurse_gate_worse")
	tr.mu.Lock()
	defer tr.mu.Unlock()
	for _, row := range rows {
		if len(row.Args) < 2 {
			continue
		}
		cycle := argInt(row.Args[0])
		snap := ratchetSnap{cycle: cycle, verdict: types.ExtractString(row.Args[1])}
		for _, f := range targets {
			if len(f.Args) >= 3 && argInt(f.Args[0]) == cycle {
				snap.targetID = types.ExtractString(f.Args[1])
				snap.target = types.ExtractString(f.Args[2])
			}
		}
		for _, f := range changed {
			if len(f.Args) >= 2 && argInt(f.Args[0]) == cycle {
				snap.changed = types.ExtractString(f.Args[1])
			}
		}
		for _, f := range worse {
			if len(f.Args) >= 2 && argInt(f.Args[0]) == cycle {
				snap.worse = append(snap.worse, types.ExtractString(f.Args[1]))
			}
		}
		tr.ratchets = append(tr.ratchets, snap)
	}
}

// recordingExecutor is the real executor plus the argv it was given, so the
// test can see that acceptance ran the finding's check and not a stand-in.
type recordingExecutor struct {
	inner tactile.Executor
	ran   []tactile.Command
}

func (r *recordingExecutor) Execute(ctx context.Context, cmd tactile.Command) (*tactile.ExecutionResult, error) {
	r.ran = append(r.ran, cmd)
	return r.inner.Execute(ctx, cmd)
}
func (r *recordingExecutor) Capabilities() tactile.ExecutorCapabilities {
	return r.inner.Capabilities()
}
func (r *recordingExecutor) Validate(cmd tactile.Command) error { return r.inner.Validate(cmd) }

// One pass of the recurse loop over a real two-package module whose
// dependency's test fails. The executor is the same seam the other recurse
// tests use. For the fix it applies the scripted edit and then runs that
// attempt's campaign acceptance (the production witness) with a real
// `go test`, and every step below is a fact the kernel held, not only the
// tree at the end.
//
// breakWeb is the second scenario: the edit makes the dependency's test pass
// and breaks the importer's build. The witness still passes; the ratchet
// derives revert and the tree returns to the planted failure.
func TestRecurseCycle_EndToEndOneCycle(t *testing.T) {
	t.Run("keep", func(t *testing.T) { runRecurseEndToEnd(t, false) })
	t.Run("revert", func(t *testing.T) { runRecurseEndToEnd(t, true) })
}

func runRecurseEndToEnd(t *testing.T, breakWeb bool) {
	t.Helper()
	root := recurseFixture(t, nil)
	real, err := core.NewRealKernelWithWorkspace(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	tk := &tracingKernel{RealKernel: real, trace: &cycleTrace{}}

	var findingID string
	var fixCycle int
	var progress strings.Builder
	res, err := RunRecurseCycles(context.Background(), RecurseCycleConfig{
		Workspace: root,
		Kernel:    tk,
		Passes:    1,
		Progress:  &progress,
		Execute: func(ctx context.Context, a RecurseAttempt) error {
			if a.Angle != "" {
				return nil
			}
			if findingID != "" {
				t.Errorf("step picked: a second fix attempt %s on %s; the planted failure should be one finding", a.Finding.ID, a.Node.ID)
				return nil
			}
			findingID, fixCycle = a.Finding.ID, a.Cycle
			assertRecurseAttemptFacts(t, tk, root, a)
			if breakWeb {
				write(t, root, "store/store.go", fixedStore)
				write(t, root, "web/web.go", "package web\n\nfunc Page() int { return undefinedThing }\n")
			} else {
				write(t, root, "store/store.go", fixedStore)
			}
			assertAttemptAcceptancePassed(t, ctx, tk, root, a)
			return nil
		},
	})
	t.Logf("progress:\n%s", progress.String())
	if err != nil {
		t.Fatalf("RunRecurseCycles: %v", err)
	}
	if findingID == "" {
		t.Fatal("step measured: the loop made no fix attempt; go test did not surface the planted failure")
	}

	tk.trace.mu.Lock()
	snaps := append([]ratchetSnap(nil), tk.trace.ratchets...)
	tk.trace.mu.Unlock()
	var fix *ratchetSnap
	for i := range snaps {
		if snaps[i].targetID == findingID {
			fix = &snaps[i]
			break
		}
	}
	if fix == nil {
		t.Fatalf("step ratchet: no recurse_ratchet row for finding %s (cycle %d); snaps %+v", findingID, fixCycle, snaps)
	}
	if fix.cycle != fixCycle {
		t.Fatalf("step ratchet: verdict cycle %d, attempt cycle %d", fix.cycle, fixCycle)
	}
	if fix.target != "/resolved" {
		t.Fatalf("step ratchet: target %s, want /resolved (the scripted edit cleared the finding)", fix.target)
	}
	if fix.changed != "/yes" {
		t.Fatalf("step ratchet: changed %s, want /yes", fix.changed)
	}

	if breakWeb {
		if fix.verdict != ratchetRevert {
			t.Fatalf("step ratchet: verdict %s, want %s; worse %v", fix.verdict, ratchetRevert, fix.worse)
		}
		if !slices.ContainsFunc(fix.worse, func(g string) bool { return strings.Contains(g, "go:build") }) {
			t.Fatalf("step ratchet: worse gates %v, want the importer's build", fix.worse)
		}
		rows, qerr := tk.peek("recurse_attempt")
		if qerr != nil {
			t.Fatal(qerr)
		}
		if !attemptFact(rows, findingID, "store", fixCycle, outcomeReverted, "go:build") {
			t.Fatalf("step ratchet: recurse_attempt does not record the revert: %s", formatFacts(rows))
		}
		if res.Kept != 0 {
			t.Fatalf("step disk: kept %d attempts after a revert verdict", res.Kept)
		}
		if mustRead(t, root, "store/store.go") != recurseModule["store/store.go"] || mustRead(t, root, "web/web.go") != recurseModule["web/web.go"] {
			t.Fatal("step disk: the reverted attempt left a write behind")
		}
		return
	}

	if fix.verdict != ratchetKeep {
		t.Fatalf("step ratchet: verdict %s, want %s; worse %v", fix.verdict, ratchetKeep, fix.worse)
	}
	if len(fix.worse) != 0 {
		t.Fatalf("step ratchet: a kept cycle still has a worse gate: %v", fix.worse)
	}
	rows, qerr := tk.peek("recurse_node_kept")
	if qerr != nil {
		t.Fatal(qerr)
	}
	if !nodeKeptFact(rows, "store", fixCycle) {
		t.Fatalf("step ratchet: recurse_node_kept does not record the keep: %s", formatFacts(rows))
	}
	if res.Kept != 1 {
		t.Fatalf("step disk: kept = %d, want the one fix; result %+v", res.Kept, res)
	}
	if mustRead(t, root, "store/store.go") != fixedStore {
		t.Fatal("step disk: the kept edit is not the file on disk")
	}
	if mustRead(t, root, "web/web.go") != recurseModule["web/web.go"] {
		t.Fatal("step disk: the importer changed")
	}
	if subj := headSubject(t, root); !strings.Contains(subj, "recurse: store:") {
		t.Fatalf("step disk: head commit = %q", subj)
	}
}

// assertRecurseAttemptFacts checks the facts held while the fix attempt is
// in flight: the real test measured the failure, it is the dependency's, the
// sweep is on that dependency, and the kernel picked this finding.
func assertRecurseAttemptFacts(t *testing.T, tk *tracingKernel, root string, a RecurseAttempt) {
	t.Helper()
	if a.Node.ID != "store" || a.Finding.Node != "store" || a.Finding.Target != "store::TestGet" || a.Finding.Gate != "go:test" {
		t.Fatalf("step attributed: attempt node %s finding %+v, want store's TestGet from go:test", a.Node.ID, a.Finding)
	}
	if !strings.Contains(a.Evidence, "Get() != 2") || !slices.Contains(a.Check, "go") || !slices.Contains(a.Check, "test") || !slices.Contains(a.Check, "./store") {
		t.Fatalf("step measured: the attempt does not carry go test's output and argv: check %v", a.Check)
	}
	set, err := gates.Detect(root)
	if err != nil {
		t.Fatal(err)
	}
	var testGate gates.Gate
	foundGate := false
	for _, g := range set.Gates {
		if g.ID == "go:test" {
			testGate, foundGate = g, true
		}
	}
	if !foundGate {
		t.Fatal("step measured: the workspace has no go:test gate")
	}
	if !slices.Equal(a.OKExitCodes, testGate.OKExitCodes) {
		t.Fatalf("step acceptance-wired: attempt OK exits %v, go:test declares %v", a.OKExitCodes, testGate.OKExitCodes)
	}

	tk.trace.mu.Lock()
	nextNodes := append([]string(nil), tk.trace.nextNodes...)
	picked := append([]string(nil), tk.trace.nextFindings...)
	tk.trace.mu.Unlock()
	storeAt := slices.Index(nextNodes, "store")
	webAt := slices.Index(nextNodes, "web")
	if storeAt < 0 || (webAt >= 0 && storeAt > webAt) || (len(nextNodes) > 0 && nextNodes[0] != "store") {
		t.Fatalf("step sweep: next-node derivations %v, want store (the dependency) before web", nextNodes)
	}
	if !slices.Contains(picked, a.Finding.ID) {
		t.Fatalf("step picked: recurse_next never named %s; picks %v", a.Finding.ID, picked)
	}

	visit, err := tk.peek("recurse_visit")
	if err != nil {
		t.Fatal(err)
	}
	if len(visit) != 1 || types.ExtractString(visit[0].Args[0]) != "store" {
		t.Fatalf("step sweep: recurse_visit = %s, want store", formatFacts(visit))
	}
	deps, err := tk.peek("subsystem_depends")
	if err != nil {
		t.Fatal(err)
	}
	if !dependsFact(deps, "web", "store") {
		t.Fatalf("step sweep: subsystem_depends has no web→store edge: %s", formatFacts(deps))
	}
	visited, err := tk.peek("recurse_node_visited")
	if err != nil {
		t.Fatal(err)
	}
	for _, f := range visited {
		if len(f.Args) >= 2 && argInt(f.Args[0]) == a.Pass {
			t.Fatalf("step sweep: pass %d already visited %s before the dependency's attempt finished", a.Pass, types.ExtractString(f.Args[1]))
		}
	}
	next, err := tk.peek("recurse_next_node")
	if err != nil {
		t.Fatal(err)
	}
	if len(next) != 1 || types.ExtractString(next[0].Args[0]) != "store" {
		t.Fatalf("step sweep: recurse_next_node during the attempt = %s, want store", formatFacts(next))
	}

	findings, err := tk.peek("recurse_finding")
	if err != nil {
		t.Fatal(err)
	}
	var matched bool
	for _, f := range findings {
		if len(f.Args) < 5 || types.ExtractString(f.Args[0]) != a.Finding.ID {
			continue
		}
		matched = true
		node := types.ExtractString(f.Args[1])
		gate := types.ExtractString(f.Args[2])
		kind := types.ExtractString(f.Args[3])
		target := types.ExtractString(f.Args[4])
		if node != "store" || gate != "go:test" || kind != "/test" || target != "store::TestGet" {
			t.Fatalf("step attributed: recurse_finding(%s) = node %s gate %s kind %s target %s", a.Finding.ID, node, gate, kind, target)
		}
	}
	if !matched {
		t.Fatalf("step attributed: recurse_finding has no row for %s: %s", a.Finding.ID, formatFacts(findings))
	}
}

// assertAttemptAcceptancePassed builds the attempt's campaign and runs its
// witness. The phase is already the model's "done" — the same shape as the
// acceptance tests — after the scripted edit, so the check is what judges it.
func assertAttemptAcceptancePassed(t *testing.T, ctx context.Context, tk *tracingKernel, root string, a RecurseAttempt) {
	t.Helper()
	camp := RecurseAttemptCampaign(root, a)
	if camp.Acceptance == nil {
		t.Fatal("step acceptance-wired: the fix attempt declares no witness")
	}
	if strings.Join(camp.Acceptance.Command, "\x00") != strings.Join(a.Check, "\x00") {
		t.Fatalf("step acceptance-wired: witness %v, check %v", camp.Acceptance.Command, a.Check)
	}
	if !slices.Equal(camp.Acceptance.OKExitCodes, a.OKExitCodes) {
		t.Fatalf("step acceptance-wired: witness exits %v, attempt exits %v", camp.Acceptance.OKExitCodes, a.OKExitCodes)
	}
	camp.Phases[0].Status = PhaseCompleted
	camp.Phases[0].Tasks[0].Status = TaskCompleted
	camp.CompletedPhases, camp.CompletedTasks = 1, 1

	exec := &recordingExecutor{inner: tactile.NewDirectExecutor()}
	orch, err := NewOrchestrator(OrchestratorConfig{
		Workspace:    root,
		Kernel:       tk,
		LLMClient:    &MockLLMClient{},
		TaskExecutor: &MockTaskExecutor{},
		Executor:     exec,
		VirtualStore: &core.VirtualStore{},
		EventChan:    make(chan OrchestratorEvent, 64),
	})
	if err != nil {
		t.Fatalf("step acceptance-passed: NewOrchestrator: %v", err)
	}
	orch.campaign = camp
	if err := tk.LoadFacts(camp.ToFacts()); err != nil {
		t.Fatalf("step acceptance-wired: load campaign facts: %v", err)
	}
	declared, err := tk.peek("campaign_acceptance")
	if err != nil {
		t.Fatal(err)
	}
	if !acceptanceCommandFact(declared, camp.ID, a.Check) {
		t.Fatalf("step acceptance-wired: campaign_acceptance = %s", formatFacts(declared))
	}

	outcome, err := orch.settleAcceptance(ctx)
	if err != nil || outcome != acceptanceSatisfied {
		t.Fatalf("step acceptance-passed: outcome %v, err %v", outcome, err)
	}
	if len(exec.ran) != 1 {
		t.Fatalf("step acceptance-passed: the witness ran %d times", len(exec.ran))
	}
	got := append([]string{exec.ran[0].Binary}, exec.ran[0].Arguments...)
	if strings.Join(got, "\x00") != strings.Join(a.Check, "\x00") || exec.ran[0].WorkingDirectory != root {
		t.Fatalf("step acceptance-passed: ran %q in %q, check %q", got, exec.ran[0].WorkingDirectory, a.Check)
	}
	if len(camp.Acceptance.Rounds) != 1 || !camp.Acceptance.Rounds[0].Passed || camp.Acceptance.Rounds[0].ExitCode != 0 {
		t.Fatalf("step acceptance-passed: round %+v", camp.Acceptance.Rounds)
	}
	if !orch.acceptanceDerived("campaign_accepted") {
		t.Fatal("step acceptance-passed: campaign_accepted is not derived")
	}
	results, err := tk.peek("campaign_acceptance_result")
	if err != nil {
		t.Fatal(err)
	}
	verdict := ""
	for _, f := range results {
		if len(f.Args) >= 3 && types.ExtractString(f.Args[0]) == camp.ID {
			verdict = types.ExtractString(f.Args[2])
		}
	}
	if !strings.Contains(verdict, "pass") {
		t.Fatalf("step acceptance-passed: campaign_acceptance_result verdict %q, facts %s", verdict, formatFacts(results))
	}
}

// pytest's detector declares exits 0 and 5 (internal/gates/detect.go). The Go
// cycle's go:test gate declares none, so this is the loop that shows a
// non-empty list traveling: the attempt is built from the python:pytest
// result, and the attempt and its campaign witness both carry the detector's
// codes. The package's test fails; an interpreter that cannot import pytest
// fails the same gate, and that result carries the same codes.
func TestRecurseCycle_PytestOKExitsTravelWithTheCheck(t *testing.T) {
	root := gitFixture(t, map[string]string{
		".gitignore":       ".nerd/\n__pycache__/\n*.pyc\n.pytest_cache/\n",
		"pytest.ini":       "[pytest]\n",
		"pkg/test_calc.py": "def test_value():\n    assert 1 == 2\n",
	}).root
	set, err := gates.Detect(root)
	if err != nil {
		t.Fatal(err)
	}
	for _, u := range set.Unavailable {
		if u.Gate.ID == "python:pytest" {
			t.Skip(u.Reason)
		}
	}
	var py gates.Gate
	for _, g := range set.Gates {
		if g.ID == "python:pytest" {
			py = g
		}
	}
	if py.ID == "" || !slices.Equal(py.OKExitCodes, []int{0, 5}) {
		t.Fatalf("python:pytest OK exits = %v, want [0 5]", py.OKExitCodes)
	}
	wantCheck := py.ForNode("pkg")

	var carried [][]int
	_, err = runRecurse(t, context.Background(), root, 1, onlyFixes(func(ctx context.Context, a RecurseAttempt) error {
		if a.Finding.Gate != "python:pytest" {
			return nil
		}
		if a.Evidence == "" || strings.Join(a.Check, "\x00") != strings.Join(wantCheck, "\x00") {
			t.Fatalf("step measured: check %v evidence %q, detected %v", a.Check, a.Evidence, wantCheck)
		}
		carried = append(carried, append([]int(nil), a.OKExitCodes...))
		camp := RecurseAttemptCampaign(root, a)
		if camp.Acceptance == nil || !slices.Equal(camp.Acceptance.OKExitCodes, []int{0, 5}) {
			t.Fatalf("step acceptance-wired: witness exits %v, want [0 5] (check %v)", acceptanceCodes(camp), a.Check)
		}
		if len(a.OKExitCodes) > 1 {
			a.OKExitCodes[1] = 9
		}
		if !slices.Equal(camp.Acceptance.OKExitCodes, []int{0, 5}) {
			t.Fatalf("step acceptance-wired: the witness aliases the attempt's exits: %v", camp.Acceptance.OKExitCodes)
		}
		return nil
	}))
	if err != nil {
		t.Fatal(err)
	}
	if len(carried) == 0 {
		t.Fatal("step measured: the loop made no python:pytest attempt")
	}
	for _, codes := range carried {
		if !slices.Equal(codes, []int{0, 5}) {
			t.Fatalf("step acceptance-wired: attempt OK exits %v, pytest declares [0 5]", codes)
		}
	}
}

func acceptanceCodes(c *Campaign) []int {
	if c == nil || c.Acceptance == nil {
		return nil
	}
	return c.Acceptance.OKExitCodes
}

func dependsFact(rows []core.Fact, node, dep string) bool {
	for _, f := range rows {
		if len(f.Args) >= 2 && types.ExtractString(f.Args[0]) == node && types.ExtractString(f.Args[1]) == dep {
			return true
		}
	}
	return false
}

func acceptanceCommandFact(rows []core.Fact, campaignID string, check []string) bool {
	want := strings.Join(check, " ")
	for _, f := range rows {
		if len(f.Args) >= 2 && types.ExtractString(f.Args[0]) == campaignID && types.ExtractString(f.Args[1]) == want {
			return true
		}
	}
	return false
}

func nodeKeptFact(rows []core.Fact, node string, cycle int) bool {
	for _, f := range rows {
		if len(f.Args) >= 2 && types.ExtractString(f.Args[0]) == node && argInt(f.Args[1]) == cycle {
			return true
		}
	}
	return false
}

func attemptFact(rows []core.Fact, finding, node string, cycle int, outcome, signaturePart string) bool {
	for _, f := range rows {
		if len(f.Args) < 5 {
			continue
		}
		if types.ExtractString(f.Args[0]) != finding || types.ExtractString(f.Args[1]) != node {
			continue
		}
		if argInt(f.Args[2]) != cycle || types.ExtractString(f.Args[3]) != outcome {
			continue
		}
		if strings.Contains(types.ExtractString(f.Args[4]), signaturePart) {
			return true
		}
	}
	return false
}

func formatFacts(rows []core.Fact) string {
	var b strings.Builder
	for _, f := range rows {
		if b.Len() > 0 {
			b.WriteString("; ")
		}
		b.WriteString(f.Predicate)
		b.WriteByte('(')
		for i, a := range f.Args {
			if i > 0 {
				b.WriteString(", ")
			}
			b.WriteString(types.ExtractString(a))
		}
		b.WriteByte(')')
	}
	if b.Len() == 0 {
		return "(none)"
	}
	return b.String()
}
