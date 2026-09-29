package session

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"codenerd/internal/evidence"
	"codenerd/internal/types"
)

// The /build gate charges a turn only for a failure its write set can have
// caused. go build ./... stays whole-workspace; these tests are the
// attribution. Each one compiles a throwaway module, the way the other
// session gate tests do.

func TestBuildAttribution_OwnCompileErrorIsRedThenRepaired(t *testing.T) {
	if testing.Short() {
		t.Skip("compiles a throwaway package")
	}
	h := newRepairHarness(t, nil)
	if err := os.WriteFile(filepath.Join(h.ws, "main.go"), []byte(repairMainGo), 0o600); err != nil {
		t.Fatal(err)
	}
	sawRed := false
	h.initial = func() *types.LLMToolResponse {
		return &types.LLMToolResponse{Text: "writing", ToolCalls: []types.ToolCall{
			h.writeCall("c1", "write_file", filepath.Join(h.ws, "main.go"), repairMainTypeBroken),
		}}
	}
	h.onRepair = func(_ int, _ []types.Message) *types.LLMToolResponse {
		// The first repair call is not always calls==1: the initial
		// generation can share this path. The gate is what matters, and it
		// is still asserted when the loop asks.
		facts, err := h.executor.kernel.Query("turn_gate")
		if err != nil {
			t.Fatalf("query turn_gate: %v", err)
		}
		for _, fact := range facts {
			if len(fact.Args) >= 3 && types.ExtractString(fact.Args[1]) == "/build" && types.ExtractString(fact.Args[2]) == "/failing" {
				sawRed = true
			}
		}
		foreign, err := h.executor.kernel.Query("turn_build_failure_foreign")
		if err != nil {
			t.Fatalf("query foreign: %v", err)
		}
		if len(foreign) != 0 {
			t.Fatalf("the turn's own compile error was called foreign: %v", foreign)
		}
		return &types.LLMToolResponse{Text: "fixed", ToolCalls: []types.ToolCall{
			h.writeCall("r1", "write_file", filepath.Join(h.ws, "main.go"), repairMainTypeFixed),
		}}
	}
	result, err := h.drive(t, "fix make the package build")
	if err != nil {
		t.Fatalf("ProcessWithIntent: %v", err)
	}
	if !sawRed {
		t.Fatal("repair ran without turn_gate(/build, /failing) for the turn's own compile error")
	}
	if !result.BuildCheck.OK || result.BuildCheck.Verdict() != VerifyPassed {
		t.Fatalf("BuildCheck = %+v, want passed after repair", result.BuildCheck)
	}
	if result.BuildCheck.Repair == nil || !result.BuildCheck.Repair.Passed {
		t.Fatalf("build repair record = %+v, want passed", result.BuildCheck.Repair)
	}
}

func TestBuildAttribution_ForeignPackageIsNotCharged(t *testing.T) {
	if testing.Short() {
		t.Skip("compiles a throwaway package")
	}
	ws := t.TempDir()
	mustWrite(t, filepath.Join(ws, "go.mod"), "module example.com/buildgate\n\ngo 1.21\n")
	mustWrite(t, filepath.Join(ws, "mine", "mine.go"), "package mine\n\nfunc F() int { return 1 }\n")
	mustWrite(t, filepath.Join(ws, "other", "other.go"), "package other\n\nfunc G() int { return missing }\n")

	e := NewExecutor(realKernel(t), nil, nil, nil, nil, nil)
	e.config.WorkspaceRoot = ws
	e.config.EnableSafetyGate = false
	e.config.VerifyBuildAfterEdits = true
	e.config.VerifyTestsAfterEdits = false
	result := &ExecutionResult{
		SuccessfulToolCalls:  1,
		SuccessfulWriteTools: 1,
		WrittenPaths:         []string{"mine/mine.go"},
		roundsRan:            map[string]bool{"/build": true},
	}
	result.Intent.Verb = "/fix"
	result.Intent.Category = "/mutation"

	// A nil repair client: if the failure is charged, this errors. A foreign
	// failure must not start the loop.
	ctx := context.Background()
	if _, _, err := e.verifyAndRepairBuild(ctx, nil, "", nil, nil, nil, nil, result); err != nil {
		t.Fatalf("foreign build was charged: %v", err)
	}
	turn := result.turnAtom()
	if pass, fail := e.derivedGate(turn, "/build"); pass || fail {
		t.Fatalf("turn_gate(/build) = pass %v fail %v, want no verdict", pass, fail)
	}
	pkgs := e.foreignBuildPackages(turn)
	if !slices.Contains(pkgs, "example.com/buildgate/other") {
		t.Fatalf("foreign packages = %v, want example.com/buildgate/other", pkgs)
	}
	unknown, err := e.turnRows("turn_build_graph_unknown", turn)
	if err != nil {
		t.Fatal(err)
	}
	if len(unknown) != 0 {
		t.Fatal("the import graph was missing; the foreign package was not a real attribution")
	}

	before, err := evidence.Snapshot(ctx, ws)
	if err != nil {
		t.Fatal(err)
	}
	if err := e.closeChangeEvidence(ctx, result, before); err != nil {
		t.Fatalf("closeChangeEvidence charged a foreign build: %v", err)
	}
	if err := e.checkHollowSuccess(result); err != nil {
		t.Fatalf("checkHollowSuccess: %v", err)
	}
	if result.TurnOutcome != types.MangleAtom("/unverified") {
		t.Fatalf("TurnOutcome = %s, want /unverified", result.TurnOutcome)
	}
	if !slices.Contains(result.MissingEvidence, "/build_failure_foreign") {
		t.Fatalf("missing evidence = %v, want /build_failure_foreign", result.MissingEvidence)
	}
	if slices.Contains(result.MissingEvidence, "/build_not_green") {
		t.Fatalf("/build_not_green must not accompany the foreign atom, got %v", result.MissingEvidence)
	}
	result.Response = "did the work"
	e.appendEvidenceSummary(result)
	const want = "the build is broken outside this turn: example.com/buildgate/other"
	if !strings.Contains(result.Response, want) {
		t.Fatalf("response = %q, want it to contain %q", result.Response, want)
	}
}

func TestBuildAttribution_TransitiveImporterIsRed(t *testing.T) {
	if testing.Short() {
		t.Skip("compiles a throwaway package")
	}
	ws := t.TempDir()
	mustWrite(t, filepath.Join(ws, "go.mod"), "module example.com/buildgate\n\ngo 1.21\n")
	// The turn changed a.F to return string. b still compiles. c imports
	// only b, and b.G's new type does not fit c.H. c is not in the write
	// set and does not import a, but it depends on a through b.
	mustWrite(t, filepath.Join(ws, "a", "a.go"), "package a\n\nfunc F() string { return \"x\" }\n")
	mustWrite(t, filepath.Join(ws, "b", "b.go"), "package b\n\nimport \"example.com/buildgate/a\"\n\nfunc G() string { return a.F() }\n")
	mustWrite(t, filepath.Join(ws, "c", "c.go"), "package c\n\nimport \"example.com/buildgate/b\"\n\nfunc H() int { return b.G() }\n")

	e := NewExecutor(realKernel(t), nil, nil, nil, nil, nil)
	e.config.WorkspaceRoot = ws
	e.config.EnableSafetyGate = false
	e.config.VerifyBuildAfterEdits = true
	result := &ExecutionResult{
		SuccessfulWriteTools: 1,
		WrittenPaths:         []string{"a/a.go"},
	}
	probe := &buildGateProbe{e: e, result: result}
	ctx := inTurnWorkingLoop(t, e)
	_, _, err := e.verifyAndRepairBuild(ctx, probe, "system", nil, nil, nil, nil, result)
	if err == nil {
		t.Fatal("attributed build failure returned no error; the repair did not start")
	}
	if strings.Contains(err.Error(), "broken outside this turn") {
		t.Fatalf("transitive importer was called foreign: %v", err)
	}
	if probe.calls == 0 {
		t.Fatalf("repair did not start: %v", err)
	}
	if !probe.red {
		t.Fatal("turn_gate(/build, /failing) did not hold when the repair started")
	}
	if len(probe.foreign) != 0 {
		t.Fatalf("foreign packages = %v, want none", probe.foreign)
	}
	if probe.graphUnknown {
		t.Fatal("the import graph was missing; the transitive break was fail-closed, not derived")
	}
	if !probe.depends {
		t.Fatal("example.com/buildgate/c did not derive as depending on a package the turn wrote")
	}
}

// buildGateProbe is the repair client for an attributed failure. It reads
// the gate the loop is about to repair, then stops the attempt.
type buildGateProbe struct {
	e            *Executor
	result       *ExecutionResult
	calls        int
	red          bool
	graphUnknown bool
	depends      bool
	foreign      []string
}

func (p *buildGateProbe) CompleteWithToolResults(context.Context, string, []types.Message, []types.ToolDefinition) (*types.LLMToolResponse, error) {
	p.calls++
	turn := p.result.turnAtom()
	_, p.red = p.e.derivedGate(turn, "/build")
	p.foreign = p.e.foreignBuildPackages(turn)
	unknown, err := p.e.turnRows("turn_build_graph_unknown", turn)
	if err != nil {
		return nil, err
	}
	p.graphUnknown = len(unknown) != 0
	deps, err := p.e.turnRows("turn_build_depends_on_written", turn)
	if err != nil {
		return nil, err
	}
	for _, fact := range deps {
		if len(fact.Args) >= 2 && types.ExtractString(fact.Args[1]) == "example.com/buildgate/c" {
			p.depends = true
		}
	}
	return nil, errStopBuildRepair
}

var errStopBuildRepair = errors.New("stop after observing the build gate")
