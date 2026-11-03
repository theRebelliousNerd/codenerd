package campaign

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"codenerd/internal/config"
	"codenerd/internal/core"
	"codenerd/internal/deliberation"
	"codenerd/internal/gates"
	"codenerd/internal/observation"
	"codenerd/internal/types"
	"codenerd/internal/usage"
)

// Audit 1: Docs/audits/recurse-collapsed-finding-invocation-identity.md
// When a collapsed multi-path node (e.g. a+b) runs a node-scoped gate once per
// path, each finding retains the producing invocation's Result (Argv, ExitCode,
// Output) rather than inheriting a sibling path's command.
func TestRecurseCollapsedFindingInvocationIdentity(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	node := SubsystemNode{ID: "a+b", Paths: []string{"a", "b"}, Languages: []string{"python"}}
	g := gates.Gate{ID: "py:check", Kind: gates.Lint, Scope: gates.ScopeNode, Language: "python"}

	resA := gates.Result{
		Gate:     g,
		Node:     "a",
		Argv:     []string{"pycheck", "a"},
		ExitCode: 1,
		Output:   "a/mod.py:1:1: error in a\n",
	}
	resB := gates.Result{
		Gate:     g,
		Node:     "b",
		Argv:     []string{"pycheck", "b"},
		ExitCode: 2,
		Output:   "b/mod.py:2:1: error in b\n",
	}
	fsA := gates.FindingsAll(root, resA)
	fsB := gates.FindingsAll(root, resB)
	if len(fsA) != 1 || len(fsB) != 1 {
		t.Fatalf("findings = %d, %d; want 1, 1", len(fsA), len(fsB))
	}
	fsA[0].Node = node.ID
	fsB[0].Node = node.ID

	run := gateRun{
		result:      resA,
		invocations: []gates.Result{resA, resB},
		byFinding: map[string]gates.Result{
			fsA[0].ID: resA,
			fsB[0].ID: resB,
		},
		findingPath: map[string]string{
			fsA[0].ID: "a",
			fsB[0].ID: "b",
		},
		allFindingIDs: map[string]bool{
			fsA[0].ID: true,
			fsB[0].ID: true,
		},
		findings: []gates.Finding{fsA[0], fsB[0]},
	}
	run.result.Output += "\n" + resB.Output

	r := &recurseRun{
		root:  root,
		state: map[string]gateRun{gateKey(g.ID, node.ID): run},
	}

	srcA := r.sourceOf(fsA[0], node)
	if !slices.Equal(srcA.result.Argv, []string{"pycheck", "a"}) || srcA.result.ExitCode != 1 {
		t.Fatalf("finding A source = %+v; want pycheck a (exit 1)", srcA.result)
	}
	srcB := r.sourceOf(fsB[0], node)
	if !slices.Equal(srcB.result.Argv, []string{"pycheck", "b"}) || srcB.result.ExitCode != 2 || !strings.Contains(srcB.result.Output, "error in b") {
		t.Fatalf("finding B source = %+v; want pycheck b (exit 2) and b's output", srcB.result)
	}
}

// Audit 2: Docs/audits/recurse-coverage-gate-identity.md
// When a node has multiple node-scoped test gates reporting coverage, every
// gate's coverage is projected into the ratchet so a coverage increase on the
// first gate cannot hide a coverage regression on a later gate.
func TestRecurseCoverageGateIdentity(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, "example_pkg"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "example_pkg", "mod.py"), []byte("def f():\n    return 1\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "example_pkg", "test_mod.py"), []byte("def test_f():\n    assert True\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	node := SubsystemNode{ID: "example_pkg", Paths: []string{"example_pkg"}, Languages: []string{"python"}}
	gateA := gates.Gate{ID: "nerd.md:a-unit", Kind: gates.Test, Scope: gates.ScopeNode, Language: "python"}
	gateB := gates.Gate{ID: "nerd.md:z-integration", Kind: gates.Test, Scope: gates.ScopeNode, Language: "python"}
	own := []gates.Gate{gateA, gateB}

	r := &recurseRun{root: root}
	beforeRuns := map[string]gateRun{
		gateKey(gateA.ID, node.ID): {result: gates.Result{Gate: gateA, Passed: true, Output: "TOTAL 100 50 50%\n"}},
		gateKey(gateB.ID, node.ID): {result: gates.Result{Gate: gateB, Passed: true, Output: "TOTAL 100 20 80%\n"}},
	}
	// Case 1: Gate A rises 50% -> 60%, Gate B drops 80% -> 40%. Must revert!
	afterRegressedRuns := map[string]gateRun{
		gateKey(gateA.ID, node.ID): {result: gates.Result{Gate: gateA, Passed: true, Output: "TOTAL 100 40 60%\n"}},
		gateKey(gateB.ID, node.ID): {result: gates.Result{Gate: gateB, Passed: true, Output: "TOTAL 100 60 40%\n"}},
	}

	beforeMetrics := r.metrics(node, own, beforeRuns)
	afterRegressedMetrics := r.metrics(node, own, afterRegressedRuns)
	if beforeMetrics["coverage:nerd.md:a-unit"] != 5000 || beforeMetrics["coverage:nerd.md:z-integration"] != 8000 {
		t.Fatalf("beforeMetrics = %v", beforeMetrics)
	}
	if afterRegressedMetrics["coverage:nerd.md:a-unit"] != 6000 || afterRegressedMetrics["coverage:nerd.md:z-integration"] != 4000 {
		t.Fatalf("afterRegressedMetrics = %v", afterRegressedMetrics)
	}

	p := newRecursePolicy(t)
	passGates := []ratchetGate{
		{Gate: gateKey(gateA.ID, node.ID), Before: verdictPass, After: verdictPass},
		{Gate: gateKey(gateB.ID, node.ID), Before: verdictPass, After: verdictPass},
	}
	verdict, err := p.ratchet(ratchetInput{
		Cycle:   1,
		Improve: "harden",
		Changed: true,
		Tested:  true,
		Gates:   passGates,
		Before:  beforeMetrics,
		After:   afterRegressedMetrics,
	})
	if err != nil || verdict != ratchetRevert {
		t.Fatalf("secondary coverage regression verdict = %q, %v; want %q", verdict, err, ratchetRevert)
	}
	if err := p.endCycle(); err != nil {
		t.Fatal(err)
	}

	// Case 2: Gate A stays 50% -> 50%, Gate B rises 80% -> 85%. Must keep!
	afterImprovedRuns := map[string]gateRun{
		gateKey(gateA.ID, node.ID): {result: gates.Result{Gate: gateA, Passed: true, Output: "TOTAL 100 50 50%\n"}},
		gateKey(gateB.ID, node.ID): {result: gates.Result{Gate: gateB, Passed: true, Output: "TOTAL 100 15 85%\n"}},
	}
	afterImprovedMetrics := r.metrics(node, own, afterImprovedRuns)
	verdict, err = p.ratchet(ratchetInput{
		Cycle:   2,
		Improve: "harden",
		Changed: true,
		Tested:  true,
		Gates:   passGates,
		Before:  beforeMetrics,
		After:   afterImprovedMetrics,
	})
	if err != nil || verdict != ratchetKeep {
		t.Fatalf("secondary coverage improvement verdict = %q, %v; want %q", verdict, err, ratchetKeep)
	}
	if err := p.endCycle(); err != nil {
		t.Fatal(err)
	}

	// Case 3: Gate A rises 50% -> 60%, Gate B's coverage report disappears. Must revert!
	afterMissingRuns := map[string]gateRun{
		gateKey(gateA.ID, node.ID): {result: gates.Result{Gate: gateA, Passed: true, Output: "TOTAL 100 40 60%\n"}},
		gateKey(gateB.ID, node.ID): {result: gates.Result{Gate: gateB, Passed: true, Output: "passed without coverage summary\n"}},
	}
	afterMissingMetrics := r.metrics(node, own, afterMissingRuns)
	verdict, err = p.ratchet(ratchetInput{
		Cycle:   3,
		Improve: "harden",
		Changed: true,
		Tested:  true,
		Gates:   passGates,
		Before:  beforeMetrics,
		After:   afterMissingMetrics,
	})
	if err != nil || verdict != ratchetRevert {
		t.Fatalf("missing secondary coverage report verdict = %q, %v; want %q", verdict, err, ratchetRevert)
	}
}

// Audit 3: Docs/audits/recurse-finding-census-completeness.md
// MaxFindingsPerRun (50) bounds work selection without blinding the ratchet to
// a 50 -> 51 finding increase or a target finding pushed past position 50.
func TestRecurseFindingCensusCompleteness(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	g := gates.Gate{ID: "nerd.md:lint", Kind: gates.Lint, Scope: gates.ScopeAll}

	var lines50, lines51 []string
	for i := 1; i <= 50; i++ {
		lines50 = append(lines50, fmt.Sprintf("pkg/file_%02d.go:%d:1: lint issue %d", i, i, i))
	}
	// In the 51-finding output, prepend a new diagnostic at file_00.go so the
	// original 50th diagnostic (file_50.go) is pushed to position 51.
	lines51 = append(lines51, "pkg/file_00.go:1:1: lint issue 0")
	lines51 = append(lines51, lines50...)

	res50 := gates.Result{Gate: g, ExitCode: 1, Output: strings.Join(lines50, "\n") + "\n"}
	res51 := gates.Result{Gate: g, ExitCode: 1, Output: strings.Join(lines51, "\n") + "\n"}

	capped50 := gates.Findings(root, res50)
	capped51 := gates.Findings(root, res51)
	all50 := gates.FindingsAll(root, res50)
	all51 := gates.FindingsAll(root, res51)

	if len(capped50) != gates.MaxFindingsPerRun || len(capped51) != gates.MaxFindingsPerRun {
		t.Fatalf("capped findings = %d, %d; want %d", len(capped50), len(capped51), gates.MaxFindingsPerRun)
	}
	if len(all50) != 50 || len(all51) != 51 {
		t.Fatalf("all findings = %d, %d; want 50, 51", len(all50), len(all51))
	}

	target50 := all50[49]
	if hasFinding(capped51, target50.ID) {
		t.Fatal("target50 should be displaced from the capped first-50 slice in res51")
	}

	ids50 := map[string]bool{}
	for _, f := range all50 {
		ids50[f.ID] = true
	}
	ids51 := map[string]bool{}
	for _, f := range all51 {
		ids51[f.ID] = true
	}
	runBefore := gateRun{result: res50, findings: capped50, allFindingIDs: ids50}
	runAfter := gateRun{result: res51, findings: capped51, allFindingIDs: ids51}

	if runBefore.totalFindings() != 50 || runAfter.totalFindings() != 51 {
		t.Fatalf("totalFindings = %d -> %d; want 50 -> 51", runBefore.totalFindings(), runAfter.totalFindings())
	}
	if !runAfter.hasFinding(target50.ID) {
		t.Fatal("runAfter.hasFinding(target50) = false; want true even when displaced past position 50")
	}

	p := newRecursePolicy(t)
	// Witness A: 50 -> 51 on a secondary gate vetoes keep even when target A resolved.
	verdict, err := p.ratchet(ratchetInput{
		Cycle:    1,
		TargetID: "target-a",
		TargetOK: true,
		Changed:  true,
		Gates: []ratchetGate{
			{Gate: "nerd.md:lint@", Before: verdictFail, After: verdictFail, BeforeCount: runBefore.totalFindings(), AfterCount: runAfter.totalFindings()},
		},
	})
	if err != nil || verdict != ratchetRevert {
		t.Fatalf("50 -> 51 finding increase verdict = %q, %v; want %q", verdict, err, ratchetRevert)
	}
}

// Audit 4: Docs/audits/recurse-no-tests-evidence-continuity.md
// A test runner exit code in NoTestsExitCodes (e.g. pytest exit 5) is a process
// pass, not a test failure, but it cannot supply Tested=true or resolve a prior
// failing test target.
func TestRecurseNoTestsEvidenceContinuity(t *testing.T) {
	t.Parallel()
	pyGate := gates.Gate{
		ID:               "py:test",
		Kind:             gates.Test,
		Scope:            gates.ScopeNode,
		Language:         "python",
		OKExitCodes:      []int{0, 5},
		NoTestsExitCodes: []int{5},
	}
	beforeFail := gateRun{
		result:   gates.Result{Gate: pyGate, Node: "pkg", ExitCode: 1, Passed: false, Output: "FAILED pkg/test_widget.py::test_x\n"},
		findings: []gates.Finding{{ID: "f-widget", Gate: "py:test", Kind: gates.Test, Node: "pkg", Target: "pkg/test_widget.py::test_x"}},
		findingPath: map[string]string{
			"f-widget": "pkg",
		},
		allFindingIDs: map[string]bool{
			"f-widget": true,
		},
	}
	afterNoTests := gateRun{
		result:      gates.Result{Gate: pyGate, Node: "pkg", ExitCode: 5, Passed: true, Output: "collected 0 items\n"},
		invocations: []gates.Result{{Gate: pyGate, Node: "pkg", ExitCode: 5, Passed: true}},
	}

	if afterNoTests.verdict() != verdictPass {
		t.Fatalf("verdict = %q, want %q (not a fake test failure)", afterNoTests.verdict(), verdictPass)
	}
	if !afterNoTests.noTests() || !afterNoTests.noTestsForTarget(beforeFail, "f-widget") {
		t.Fatal("expected noTests to be true for pytest exit 5")
	}

	// Neither Tested nor TargetOK may become true from an empty test collection.
	tested := afterNoTests.result.Gate.Kind == gates.Test && !afterNoTests.result.Unverified() && !afterNoTests.noTests()
	targetOK := !afterNoTests.result.Unverified() && !afterNoTests.noTestsForTarget(beforeFail, "f-widget") && !afterNoTests.hasFinding("f-widget")
	if tested || targetOK {
		t.Fatalf("empty test collection set tested=%v targetOK=%v; want both false", tested, targetOK)
	}
}

// Audit 5: Docs/audits/recurse-root-crate-diagnostic-ownership.md
// A root Cargo crate ("." with language "rust") owns its descendant .rs source
// files (such as "src/main.rs") when no more-specific nested crate matches,
// while unknown or location-free targets remain unattributed.
func TestRecurseRootCrateDiagnosticOwnership(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, "src"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(root, "crates", "sub", "src"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "Cargo.toml"), []byte("[package]\nname = \"rootcrate\"\nversion = \"0.1.0\"\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "src", "main.rs"), []byte("fn main() { let _ = x; }\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "crates", "sub", "Cargo.toml"), []byte("[package]\nname = \"subcrate\"\nversion = \"0.1.0\"\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "crates", "sub", "src", "lib.rs"), []byte("pub fn sub() {}\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	nodes, err := DeriveWorkspaceDAG(context.Background(), root)
	if err != nil {
		t.Fatal(err)
	}

	rustOut := "error[E0425]: cannot find value `x` in this scope\n --> src/main.rs:1:21\n"
	fs := gates.Findings(root, gates.Result{
		Gate:     gates.Gate{ID: "rust:build", Kind: gates.Build, Scope: gates.ScopeAll, Language: "rust"},
		ExitCode: 101,
		Output:   rustOut,
	})
	if len(fs) != 1 || fs[0].Target != "src/main.rs" {
		t.Fatalf("findings = %+v", fs)
	}
	if got := attributeNodeInRoot(root, fs[0].Target, nodes); got != "." {
		t.Fatalf("attributeNodeInRoot(src/main.rs) = %q, want \".\"", got)
	}
	if got := attributeNodeInRoot(root, "crates/sub/src/lib.rs", nodes); got != "crates/sub" {
		t.Fatalf("attributeNodeInRoot(crates/sub/src/lib.rs) = %q, want \"crates/sub\"", got)
	}
	if got := attributeNodeInRoot(root, "src/missing.rs", nodes); got != RecurseUnattributedNodeID {
		t.Fatalf("non-existent rust file attributed to %q, want %q", got, RecurseUnattributedNodeID)
	}
	if got := attributeNodeInRoot(root, gates.UnattributedTarget+"::tests::it_works", nodes); got != RecurseUnattributedNodeID {
		t.Fatalf("location-free libtest attributed to %q, want %q", got, RecurseUnattributedNodeID)
	}
}

// Audit 6: Docs/audits/recurse-workspace-test-witness.md
// When a node has no node-scoped test gate and all applicable tests are
// workspace-scoped (e.g. commands.test in nerd.md), improvement attempts run
// the workspace test gate in after-measurement so a valid improvement can be
// certified and kept; if no test gate exists at all, improve skips early.
func TestRecurseWorkspaceTestWitnessForImprovements(t *testing.T) {
	t.Parallel()
	root := recurseFixture(t, map[string]string{
		"nerd.md":         "---\nschema: nerd/v1\ncommands:\n  test: go test ./...\n---\n",
		"store/store.go":  "package store\n\nfunc Get() int { return 2 }\n\nfunc dead() int {\n\treturn 9\n}\n",
		"web/web_test.go": "package web\n\nimport \"testing\"\n\nfunc TestPage(t *testing.T) {\n\tif Page() != 2 {\n\t\tt.Fatal(\"Page\")\n\t}\n}\n",
	})

	res, err := runRecurse(t, context.Background(), root, 3, func(ctx context.Context, a RecurseAttempt) error {
		if a.Angle == "simplify" && a.Node.ID == "store" {
			write(t, root, "store/store.go", fixedStore)
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	if res.Improved != 1 {
		t.Fatalf("workspace-scoped test gate did not certify simplify improvement: %+v", res)
	}
}

// Audit 7: Docs/audits/recurse-values-continuation-ownership.md
// Two sequential mission runs in the same session that both stop on /values
// persist their questions under mission ownership ("retained_for_owner") so
// session restoration via Deliberator.Pending() does not fail with
// "multiple pending deliberations in session".
func TestRecurseMissionValuesContinuationOwnership(t *testing.T) {
	const model = "mission-values-owner-fixture"
	usage.RegisterPrice(model, usage.Price{InputPerMTok: 1, OutputPerMTok: 1})

	root := t.TempDir()
	k, err := core.NewRealKernelWithWorkspace(root)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "note.txt"), []byte("v1"), 0o600); err != nil {
		t.Fatal(err)
	}

	e := config.DefaultExplainConfig()
	explainOpts := core.ExplainOptions{MaxProofs: e.MaxProofs, MaxDepth: e.MaxDepth, MaxNodes: e.MaxNodes, MaxEdges: e.MaxEdges, MaxBytes: e.MaxBytes}

	runShard := func(ctx context.Context, role string, frame deliberation.Frame) (string, error) {
		usage.RecordFromContext(ctx, usage.Report{Model: model, Usage: types.UsageMetadata{InputTokens: 1, Known: types.UsageTokens}})
		if role == deliberation.Proposer {
			b, err := json.Marshal(deliberation.Proposal{Candidates: []deliberation.Candidate{
				{ID: "a", Brief: "Keep existing contract", SuccessTest: "tests pass", Evidence: []string{"note.txt"}},
				{ID: "b", Brief: "Introduce new contract", SuccessTest: "tests pass", Evidence: []string{"note.txt"}},
			}})
			return string(b), err
		}
		b, err := json.Marshal(deliberation.Challenge{
			Recommendation: "a",
			Question:       "Keep existing contract or introduce new contract?",
			Assessments: []deliberation.Assessment{
				{CandidateID: "a", Success: true, Items: []deliberation.Item{{Key: "pref", Severity: "critical", Kind: "values"}}},
				{CandidateID: "b", Success: true, Items: []deliberation.Item{{Key: "pref", Severity: "critical", Kind: "values"}}},
			},
		})
		return string(b), err
	}

	sharedService := &deliberation.Service{
		Workspace: root,
		Session:   "shared-tui-session",
		Kernel:    k,
		Config:    config.DefaultRoutingConfig(),
		Explain:   explainOpts,
		RunShard:  runShard,
	}

	var missionIDs []string
	for i := 0; i < 2; i++ {
		m := &RecurseMission{
			Text:              fmt.Sprintf("Mission %d", i+1),
			Limits:            config.DefaultMissionConfig(),
			QuiesceBackground: func(context.Context) (func(), error) { return func() {}, nil },
			Deliberator:       sharedService,
		}
		ctx, cancel, err := m.begin(context.Background(), root)
		if err != nil {
			t.Fatal(err)
		}
		a := RecurseAttempt{Cycle: 1, Node: SubsystemNode{ID: ".", Paths: []string{"."}}}
		prepErr := m.prepare(ctx, &a)
		if prepErr != errMissionStopped || m.State.Status != "/values" {
			cancel()
			t.Fatalf("mission %d prepare = %v, status = %s; want /values stop", i+1, prepErr, m.State.Status)
		}
		if err := m.end(ctx, prepErr); err != nil {
			cancel()
			t.Fatal(err)
		}
		cancel()
		missionIDs = append(missionIDs, m.State.ID)
	}

	// Reloading the session's conversational clarification must succeed with (nil, nil).
	if pending, err := sharedService.Pending(); err != nil || pending != nil {
		t.Fatalf("sharedService.Pending() after 2 mission /values stops = %+v, %v; want nil, nil", pending, err)
	}
	// Both mission questions remain recoverable by their respective mission owners.
	for _, id := range missionIDs {
		th, err := sharedService.PendingForOwner("mission", id)
		if err != nil || th == nil || th.OwnerDisposition != "retained_for_owner" {
			t.Fatalf("PendingForOwner(%s) = %+v, %v", id, th, err)
		}
	}
	allMissionPending, err := sharedService.PendingMissionThreads()
	if err != nil || len(allMissionPending) != 2 {
		t.Fatalf("PendingMissionThreads = %d, %v; want 2", len(allMissionPending), err)
	}
	_ = observation.FileWrite{}
}

// Audit 8: Docs/audits/recurse-cargo-inherited-dependency-closure.md
// Cargo workspace-inherited dependencies (`z.workspace = true`, `z = { workspace = true }`,
// renamed `package = "z"`, dev/build/target dependencies, and two-hop chains)
// produce the same local dependency edges and focused closure as direct `path`
// dependencies, without falsely attaching `[workspace.dependencies]` entries
// to a root package or members that never inherit them.
func TestRecurseCargoInheritedDependencyClosure(t *testing.T) {
	t.Parallel()

	t.Run("virtual_workspace_inherited_dependency_and_focus_closure", func(t *testing.T) {
		root := t.TempDir()
		writeTree(t, root, map[string]string{
			"Cargo.toml":               "[workspace]\nmembers = [\"crates/a\", \"crates/z\", \"crates/unused\"]\nresolver = \"2\"\n\n[workspace.dependencies]\nz = { path = \"crates/z\" }\nunused = { path = \"crates/unused\" }\n",
			"crates/a/Cargo.toml":      "[package]\nname = \"a\"\nversion = \"0.1.0\"\nedition = \"2021\"\n\n[dependencies]\nz.workspace = true\n",
			"crates/a/src/lib.rs":      "pub fn a() {}\n",
			"crates/z/Cargo.toml":      "[package]\nname = \"z\"\nversion = \"0.1.0\"\nedition = \"2021\"\n",
			"crates/z/src/lib.rs":      "pub fn z() {}\n",
			"crates/unused/Cargo.toml": "[package]\nname = \"unused\"\nversion = \"0.1.0\"\nedition = \"2021\"\n",
			"crates/unused/src/lib.rs": "pub fn u() {}\n",
		})

		nodes, err := DeriveWorkspaceDAG(context.Background(), root)
		if err != nil {
			t.Fatal(err)
		}
		focused, err := FilterDAG(nodes, []string{"crates/a"})
		if err != nil {
			t.Fatal(err)
		}
		var focusedIDs []string
		for _, n := range focused {
			focusedIDs = append(focusedIDs, n.ID)
		}
		if !slices.Contains(focusedIDs, "crates/z") {
			t.Fatalf("FilterDAG(crates/a) omitted workspace-inherited prerequisite crates/z: %v", focusedIDs)
		}
		if slices.Contains(focusedIDs, "crates/unused") {
			t.Fatalf("FilterDAG(crates/a) included uninherited workspace dependency crates/unused: %v", focusedIDs)
		}

		ids := sweepOrder(t, root)
		if !before(ids, "crates/z", "crates/a") {
			t.Fatalf("crates/z must sweep before dependent crates/a: %v", ids)
		}
	})

	t.Run("root_package_and_two_hop_renamed_and_target_inherited_dependencies", func(t *testing.T) {
		root := t.TempDir()
		writeTree(t, root, map[string]string{
			"Cargo.toml":             "[package]\nname = \"root-pkg\"\nversion = \"0.1.0\"\n\n[workspace]\nmembers = [\"crates/app\", \"crates/mid\", \"crates/base\"]\n\n[workspace.dependencies]\nmid_ws = { package = \"mid\", path = \"crates/mid\" }\nbase = { path = \"crates/base\" }\n",
			"src/lib.rs":             "pub fn root() {}\n",
			"crates/app/Cargo.toml":  "[package]\nname = \"app\"\nversion = \"0.1.0\"\n\n[dependencies]\nmy_mid = { package = \"mid\", workspace = true }\n",
			"crates/app/src/lib.rs":  "pub fn app() {}\n",
			"crates/mid/Cargo.toml":  "[package]\nname = \"mid\"\nversion = \"0.1.0\"\n\n[target.'cfg(unix)'.dev-dependencies]\nbase = { workspace = true }\n",
			"crates/mid/src/lib.rs":  "pub fn mid() {}\n",
			"crates/base/Cargo.toml": "[package]\nname = \"base\"\nversion = \"0.1.0\"\n",
			"crates/base/src/lib.rs": "pub fn base() {}\n",
		})

		nodes, err := DeriveWorkspaceDAG(context.Background(), root)
		if err != nil {
			t.Fatal(err)
		}
		byID := map[string]SubsystemNode{}
		for _, n := range nodes {
			byID[n.ID] = n
		}
		if len(byID["."].DependsOn) != 0 {
			t.Fatalf("root package '.' falsely inherited [workspace.dependencies] as its own dependencies: %v", byID["."].DependsOn)
		}

		focused, err := FilterDAG(nodes, []string{"crates/app"})
		if err != nil {
			t.Fatal(err)
		}
		var focusedIDs []string
		for _, n := range focused {
			focusedIDs = append(focusedIDs, n.ID)
		}
		if !slices.Contains(focusedIDs, "crates/mid") || !slices.Contains(focusedIDs, "crates/base") {
			t.Fatalf("two-hop workspace-inherited closure for crates/app incomplete: %v", focusedIDs)
		}

		ids := sweepOrder(t, root)
		if !before(ids, "crates/base", "crates/mid") || !before(ids, "crates/mid", "crates/app") {
			t.Fatalf("expected crates/base -> crates/mid -> crates/app order, got %v", ids)
		}
	})
}
