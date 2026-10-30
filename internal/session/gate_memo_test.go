package session

import (
	"codenerd/internal/evidence"
	"codenerd/internal/perception"
	"codenerd/internal/types"
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// countTestLaunches wraps the test runner and counts the go test processes it
// starts. It swaps a package seam, so its callers do not run in parallel.
func countTestLaunches(t *testing.T) *int {
	t.Helper()
	launches := 0
	real := verifyTestRunner
	verifyTestRunner = func(ctx context.Context, dir string, env []string, name string, args []string) ([]byte, error) {
		if len(args) > 0 && args[0] == "test" {
			launches++
		}
		return real(ctx, dir, env, name, args)
	}
	t.Cleanup(func() { verifyTestRunner = real })
	return &launches
}

// Two gate measurements with no write between them are one measurement: the
// second round reads the first's verdicts and launches no go test. A write
// between them is a new revision, measured again. (Ladder R1-24 measured the
// importer gate twice, 2.5 minutes each, on a tree no one had touched.)
func TestGateTests_AnUnchangedTreeIsMeasuredOnce(t *testing.T) {
	if testing.Short() {
		t.Skip("shells out to the real go toolchain")
	}
	ws := importerModule(t, aAfter)
	result := mutationResult()
	result.WrittenPaths = []string{"a/a.go"}
	result.PreWriteContents = map[string]PreImage{"a/a.go": existed(aBefore)}
	launches := countTestLaunches(t)

	first, _ := gateTests(context.Background(), ws, result, true)
	importers := result.ImporterCheck
	// Own packages, importers, and the pre-turn baseline that attributes
	// the importer failure.
	measured := *launches
	if measured < 2 {
		t.Fatalf("the first gate launched %d go test run(s), want at least 2 (own packages, importers)", measured)
	}
	if first.Verdict() != VerifyPassed || importers.Verdict() != VerifyFailed {
		t.Fatalf("own %s, importers %s: want passed and failed (b pins what a returns)", first.Verdict(), importers.Verdict())
	}

	// The closure asks again, with and without coverage, on the same writes.
	again, _ := gateTests(context.Background(), ws, result, true)
	recheck, _ := gateOwnTests(context.Background(), ws, result, false)
	if *launches != measured {
		t.Fatalf("re-measuring an unchanged tree launched %d more go test run(s), want 0", *launches-measured)
	}
	if again.Verdict() != first.Verdict() || recheck.Verdict() != first.Verdict() || result.ImporterCheck.Verdict() != importers.Verdict() {
		t.Fatalf("a recalled measurement changed its verdicts: own %s/%s, importers %s", again.Verdict(), recheck.Verdict(), result.ImporterCheck.Verdict())
	}

	// A write -- back to what b expects -- is a new revision.
	if err := os.WriteFile(filepath.Join(ws, "a", "a.go"), []byte(aBefore), 0o600); err != nil {
		t.Fatal(err)
	}
	gateTests(context.Background(), ws, result, true)
	if got := *launches - measured; got != 2 {
		t.Fatalf("after a write the gate launched %d go test run(s), want 2 (own packages, importers)", got)
	}
	if result.ImporterCheck.Verdict() != VerifyPassed {
		t.Fatalf("importers after the write = %s, want passed: the old failure was reused", result.ImporterCheck.Verdict())
	}
}

func TestFinalFixTests_WaitsForOtherGates(t *testing.T) {
	for _, gate := range []string{"build", "vet", "pin"} {
		t.Run(gate, func(t *testing.T) {
			result := &ExecutionResult{Intent: perception.Intent{Verb: "/fix"}, WrittenPaths: []string{"a.go"}}
			failed := BuildVerification{Ran: true, Outcome: VerifyFailed, Output: "not ready"}
			switch gate {
			case "build":
				result.BuildCheck = failed
			case "vet":
				result.VetCheck = failed
			case "pin":
				result.PinCheck = failed
			}
			ws := t.TempDir()
			if err := os.WriteFile(filepath.Join(ws, "a.go"), []byte("package a\n"), 0o600); err != nil {
				t.Fatal(err)
			}
			real := verifyTestRunner
			launches := 0
			verifyTestRunner = func(context.Context, string, []string, string, []string) ([]byte, error) { launches++; return nil, nil }
			t.Cleanup(func() { verifyTestRunner = real })
			v, err := finalFixTests(context.Background(), ws, result)
			if err != nil || v.Verdict() == VerifyPassed || launches != 0 {
				t.Fatalf("%s not ready: verdict %s, launches %d, err %v", gate, v.Verdict(), launches, err)
			}
		})
	}
}

func TestFinalFixTests_TimeoutIsNotRepeatedOrPassing(t *testing.T) {
	ws := t.TempDir()
	if err := os.WriteFile(filepath.Join(ws, "go.mod"), []byte("module timeoutprobe\n\ngo 1.22\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(ws, "a.go"), []byte("package timeoutprobe\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	result := &ExecutionResult{Intent: perception.Intent{Verb: "/fix"}, WrittenPaths: []string{"a.go"}}
	own, _ := gateTests(context.Background(), ws, result, true)
	if own.Verdict() != VerifyPassed {
		t.Fatalf("narrow setup: %+v", own)
	}
	real, oldTimeout := verifyTestRunner, testVerifyTimeout
	testVerifyTimeout = time.Millisecond
	launches := 0
	verifyTestRunner = func(ctx context.Context, _ string, _ []string, _ string, args []string) ([]byte, error) {
		launches++
		if args[len(args)-1] != "./..." {
			t.Errorf("repeated narrow setup instead of using current witness: %v", args)
		}
		<-ctx.Done()
		return nil, ctx.Err()
	}
	t.Cleanup(func() { verifyTestRunner = real; testVerifyTimeout = oldTimeout })
	for i := 0; i < 2; i++ {
		v, err := finalFixTests(context.Background(), ws, result)
		if err != nil || v.Verdict() != VerifyIndeterminate {
			t.Fatalf("unfinished final run = %s, err %v", v.Verdict(), err)
		}
	}
	if launches != 1 {
		t.Fatalf("unchanged timed-out suite ran %d times, want once", launches)
	}
}

// scopeQueryKernel injects failures at the policy query, not at the verifier.
// Other executive operations continue through the real kernel when supplied.
type scopeQueryKernel struct {
	types.Kernel
	scopeRows    []types.Fact
	scopeErr     error
	onScopeQuery func()
}

func (k *scopeQueryKernel) Query(predicate string) ([]types.Fact, error) {
	if predicate != "fix_verification_scope" {
		return k.Kernel.Query(predicate)
	}
	if k.onScopeQuery != nil {
		k.onScopeQuery()
	}
	return k.scopeRows, k.scopeErr
}

func scopeRow(phase, scope string) types.Fact {
	return types.Fact{Predicate: "fix_verification_scope", Args: []any{types.MangleAtom(phase), types.MangleAtom(scope)}}
}

func fixPolicyWorkspace(t *testing.T) (string, *ExecutionResult) {
	t.Helper()
	ws := t.TempDir()
	for name, source := range map[string]string{
		"go.mod":    "module fixpolicyprobe\n\ngo 1.22\n",
		"a.go":      "package fixpolicyprobe\nfunc Value() int { return 1 }\n",
		"a_test.go": "package fixpolicyprobe\nimport \"testing\"\nfunc TestValue(t *testing.T) { if Value()!=1 { t.Fatal(\"wrong value\") } }\n",
	} {
		if err := os.WriteFile(filepath.Join(ws, name), []byte(source), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	return ws, &ExecutionResult{Intent: perception.Intent{Verb: "/fix"}, WrittenPaths: []string{"a.go"}}
}

func TestGateMemo_UnavailableSnapshotCannotReadOrStoreAPass(t *testing.T) {
	ws, result := fixPolicyWorkspace(t)
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if key := gateKey(ctx, "own+coverage", ws, result); key != "" {
		t.Fatalf("canceled snapshot produced reusable key %q", key)
	}
	stale := gateRun{v: TestVerification{Ran: true, Outcome: VerifyPassed}}
	result.gateRuns = map[string]gateRun{"": stale, "existing": stale}
	if _, ok := recalledGate(result, ""); ok {
		t.Fatal("empty key recalled a stale passing measurement")
	}
	rememberGate(result, "", TestVerification{Ran: true, Outcome: VerifyFailed}, nil)
	if len(result.gateRuns) != 2 || result.gateRuns[""].v.Verdict() != VerifyPassed {
		t.Fatal("empty key stored a new measurement")
	}
	if run, ok := recalledGate(result, "existing"); !ok || run.v.Verdict() != VerifyPassed {
		t.Fatal("rejecting an empty key damaged a valid witness")
	}
}

func TestFixVerificationScope_FailsClosedOnUnavailableOrAmbiguousPolicy(t *testing.T) {
	queryErr := errors.New("policy query unavailable")
	cases := []struct {
		name   string
		kernel types.Kernel
		want   string
		cause  error
	}{
		{name: "no kernel", want: "no kernel"},
		{name: "query failure", kernel: &scopeQueryKernel{scopeErr: queryErr}, want: "query fix verification scope", cause: queryErr},
		{name: "duplicate phase", kernel: &scopeQueryKernel{scopeRows: []types.Fact{scopeRow("/final", "/full"), scopeRow("/final", "/narrow")}}, want: "ambiguous"},
		{name: "missing phase", kernel: &scopeQueryKernel{scopeRows: []types.Fact{scopeRow("/working", "/narrow")}}, want: "no fix verification scope"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			e := &Executor{kernel: tc.kernel}
			scope, err := e.fixVerificationScope(types.MangleAtom("/final"))
			if err == nil || scope != "" || !strings.Contains(err.Error(), tc.want) {
				t.Fatalf("scope=%q err=%v, want fail-closed %s", scope, err, tc.want)
			}
			if tc.cause != nil && !errors.Is(err, tc.cause) {
				t.Fatalf("query cause was lost: %v", err)
			}
		})
	}
}

func TestConstrainFixVerification_RejectsMissingOrWrongWorkingScope(t *testing.T) {
	queryErr := errors.New("policy unavailable")
	cases := []struct {
		name   string
		kernel types.Kernel
		cause  error
		want   string
	}{
		{name: "missing kernel", want: "no kernel"},
		{name: "query error", kernel: &scopeQueryKernel{scopeErr: queryErr}, cause: queryErr, want: "query fix verification scope"},
		{name: "wrong scope", kernel: &scopeQueryKernel{scopeRows: []types.Fact{scopeRow("/working", "/full")}}, want: "unexpected working fix verification scope"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			e := &Executor{kernel: tc.kernel}
			call := types.ToolCall{Name: "run_tests", Input: map[string]any{"packages": []string{"./a"}}}
			result := &ExecutionResult{Intent: perception.Intent{Verb: "/fix"}, WrittenPaths: []string{"a/a.go"}}
			err := e.constrainFixVerification(&call, result)
			if err == nil || !strings.Contains(err.Error(), tc.want) {
				t.Fatalf("narrow call escaped unavailable policy: %v", err)
			}
			if tc.cause != nil && !errors.Is(err, tc.cause) {
				t.Fatalf("lost query cause: %v", err)
			}
			if got := call.Input["packages"].([]string); len(got) != 1 || got[0] != "./a" {
				t.Fatalf("refused call was rewritten: %v", got)
			}
		})
	}
}

func TestConstrainFixVerification_OmittedPackagesPreserveOtherBuildArguments(t *testing.T) {
	e := roundsExecutor(t)
	e.config.WorkspaceRoot = t.TempDir()
	input := map[string]any{"packages": []any{" ", "null", "undefined"}, "timeout_seconds": 37, "working_dir": "."}
	call := types.ToolCall{Name: "run_build", Input: input}
	result := &ExecutionResult{Intent: perception.Intent{Verb: "/fix"}, WrittenPaths: []string{"a/a.go", "b/b.go", "a/other.go"}}
	if err := e.constrainFixVerification(&call, result); err != nil {
		t.Fatal(err)
	}
	got, ok := call.Input["packages"].([]string)
	if !ok || strings.Join(got, " ") != "./a ./b" {
		t.Fatalf("omitted sentinels did not select edited packages: %v", call.Input["packages"])
	}
	if call.Input["timeout_seconds"] != 37 || call.Input["working_dir"] != "." {
		t.Fatalf("build options lost: %v", call.Input)
	}
	if original, ok := input["packages"].([]any); !ok || len(original) != 3 || original[0] != " " {
		t.Fatalf("caller-owned input mutated: %v", input)
	}
}

func TestFinalFixTests_CanceledInputSnapshotStartsNoTests(t *testing.T) {
	ws, result := fixPolicyWorkspace(t)
	launches := countTestLaunches(t)
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	v, err := finalFixTests(ctx, ws, result)
	if !errors.Is(err, context.Canceled) || !strings.Contains(err.Error(), "snapshot final test inputs") {
		t.Fatalf("snapshot cause not propagated: %v", err)
	}
	if v.Ran || v.Verdict() == VerifyPassed || *launches != 0 || len(result.gateRuns) != 0 {
		t.Fatalf("canceled snapshot supplied evidence or ran tests: %+v, launches %d, memo %v", v, *launches, result.gateRuns)
	}
}

func TestFinalFixTests_ImporterFailureWithholdsTheWholeSuite(t *testing.T) {
	ws := importerModule(t, aAfter)
	result := mutationResult()
	result.Intent.Verb = "/fix"
	result.WrittenPaths = []string{"a/a.go"}
	result.PreWriteContents = map[string]PreImage{"a/a.go": existed(aBefore)}
	real := verifyTestRunner
	fullRuns := 0
	verifyTestRunner = func(ctx context.Context, dir string, env []string, name string, args []string) ([]byte, error) {
		for _, arg := range args {
			if arg == "./..." {
				fullRuns++
			}
		}
		return real(ctx, dir, env, name, args)
	}
	t.Cleanup(func() { verifyTestRunner = real })
	v, err := finalFixTests(context.Background(), ws, result)
	if err != nil {
		t.Fatal(err)
	}
	if result.TestCheck.Verdict() != VerifyPassed || result.ImporterCheck.Verdict() != VerifyFailed {
		t.Fatalf("fixture did not isolate importer failure: own %+v, importer %+v", result.TestCheck, result.ImporterCheck)
	}
	if v.Verdict() != VerifyFailed || v.Result != result.ImporterCheck.Result || v.Output != result.ImporterCheck.Output {
		t.Fatalf("importer failure lost its evidence: %+v", v)
	}
	if fullRuns != 0 {
		t.Fatalf("ran full suite %d times before importer tests passed", fullRuns)
	}
}

func TestFinalFixTests_OutputSnapshotFailureCannotBecomeAPassOrRepeat(t *testing.T) {
	for _, clearMemo := range []bool{false, true} {
		t.Run(fmt.Sprintf("clear_memo_%t", clearMemo), func(t *testing.T) {
			ws, result := fixPolicyWorkspace(t)
			before, err := evidence.Snapshot(context.Background(), ws)
			if err != nil {
				t.Fatal(err)
			}
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			real := verifyTestRunner
			fullRuns := 0
			verifyTestRunner = func(runCtx context.Context, dir string, env []string, name string, args []string) ([]byte, error) {
				out, runErr := real(runCtx, dir, env, name, args)
				if len(args) > 0 && args[len(args)-1] == "./..." {
					fullRuns++
					if clearMemo {
						result.gateRuns = nil
					}
					cancel()
				}
				return out, runErr
			}
			t.Cleanup(func() { verifyTestRunner = real })
			_, err = finalFixTests(ctx, ws, result)
			if !errors.Is(err, context.Canceled) || !strings.Contains(err.Error(), "snapshot final test outputs") {
				t.Fatalf("output snapshot error lost: %v", err)
			}
			memo, ok := result.gateRuns["full|"+ws+"|"+before]
			if !ok || memo.v.Verdict() != VerifyIndeterminate || memo.v.OK || memo.v.Reason != "final test inputs could not be rechecked" {
				t.Fatalf("unrecheckable run cached as success or lost: %+v, found %t", memo.v, ok)
			}
			again, err := finalFixTests(context.Background(), ws, result)
			if err != nil || again.Verdict() != VerifyIndeterminate || again.OK || fullRuns != 1 {
				t.Fatalf("unchanged run repeated or promoted: %+v err %v runs %d", again, err, fullRuns)
			}
		})
	}
}

func TestFinalFixTests_ChangedInputsInvalidateAPassingProcess(t *testing.T) {
	ws, result := fixPolicyWorkspace(t)
	before, err := evidence.Snapshot(context.Background(), ws)
	if err != nil {
		t.Fatal(err)
	}
	real := verifyTestRunner
	fullRuns := 0
	verifyTestRunner = func(ctx context.Context, dir string, env []string, name string, args []string) ([]byte, error) {
		out, runErr := real(ctx, dir, env, name, args)
		if len(args) > 0 && args[len(args)-1] == "./..." {
			fullRuns++
			if runErr != nil {
				t.Errorf("full process did not pass before the external edit: %v", runErr)
			}
			if err := os.WriteFile(filepath.Join(ws, "a.go"), []byte("package fixpolicyprobe\nfunc Value() int { return 2 }\n"), 0o600); err != nil {
				t.Fatal(err)
			}
		}
		return out, runErr
	}
	t.Cleanup(func() { verifyTestRunner = real })
	v, err := finalFixTests(context.Background(), ws, result)
	if err != nil || v.Verdict() != VerifyIndeterminate || v.OK || v.Reason != "workspace changed during final test run" {
		t.Fatalf("stale process pass was accepted: %+v, err %v", v, err)
	}
	if memo, ok := result.gateRuns["full|"+ws+"|"+before]; !ok || memo.v.Verdict() != VerifyIndeterminate {
		t.Fatalf("original revision lost its attempted final run: %+v found %t", memo.v, ok)
	}
	again, err := finalFixTests(context.Background(), ws, result)
	if err != nil || again.Verdict() != VerifyFailed || fullRuns != 1 {
		t.Fatalf("changed input bypassed narrow failure: %+v, err %v, full runs %d", again, err, fullRuns)
	}
}

func TestFinalFixTests_RecreatesAnEmptyMemoAfterTheFinalRun(t *testing.T) {
	ws, result := fixPolicyWorkspace(t)
	real := verifyTestRunner
	fullRuns := 0
	verifyTestRunner = func(ctx context.Context, dir string, env []string, name string, args []string) ([]byte, error) {
		out, err := real(ctx, dir, env, name, args)
		if len(args) > 0 && args[len(args)-1] == "./..." {
			fullRuns++
			result.gateRuns = nil
		}
		return out, err
	}
	t.Cleanup(func() { verifyTestRunner = real })
	first, err := finalFixTests(context.Background(), ws, result)
	if err != nil || first.Verdict() != VerifyPassed || !first.Ran || !first.OK {
		t.Fatalf("final process did not pass: %+v, err %v", first, err)
	}
	if len(result.gateRuns) != 1 {
		t.Fatalf("final evidence was not retained after memo reset: %v", result.gateRuns)
	}
	again, err := finalFixTests(context.Background(), ws, result)
	if err != nil || again.Verdict() != VerifyPassed || again.Result != first.Result || fullRuns != 1 {
		t.Fatalf("unchanged final run was lost or repeated: %+v, err %v, runs %d", again, err, fullRuns)
	}
}

// An unchanged workspace can owe a new written-test obligation. The final
// suite must not run twice, but its memo cannot replace that new narrow check.
func TestGateMemo_NewWrittenTaggedTestInvalidatesNarrowEvidence(t *testing.T) {
	for _, final := range []bool{false, true} {
		name := "working"
		if final {
			name = "final"
		}
		t.Run(name, func(t *testing.T) {
			ws, result := fixPolicyWorkspace(t)
			tagged := "//go:build integration\n\npackage fixpolicyprobe\nimport \"testing\"\nfunc TestTaggedValue(t *testing.T) { if Value()!=2 { t.Fatal(\"tagged obligation must run\") } }\n"
			if err := os.WriteFile(filepath.Join(ws, "integration_test.go"), []byte(tagged), 0o600); err != nil {
				t.Fatal(err)
			}
			fullRuns := 0
			real := verifyTestRunner
			verifyTestRunner = func(ctx context.Context, dir string, env []string, name string, args []string) ([]byte, error) {
				if len(args) > 0 && args[0] == "test" && args[len(args)-1] == "./..." {
					fullRuns++
				}
				return real(ctx, dir, env, name, args)
			}
			t.Cleanup(func() { verifyTestRunner = real })
			measure := func() TestVerification {
				if final {
					v, err := finalFixTests(context.Background(), ws, result)
					if err != nil {
						t.Fatal(err)
					}
					return v
				}
				v, _ := gateOwnTests(context.Background(), ws, result, true)
				return v
			}
			if v := measure(); v.Verdict() != VerifyPassed {
				t.Fatalf("initial untagged measurement: %s: %s", v.Verdict(), v.Output)
			}
			before, err := evidence.Snapshot(context.Background(), ws)
			if err != nil {
				t.Fatal(err)
			}
			result.WrittenPaths = append(result.WrittenPaths, "integration_test.go")
			after, err := evidence.Snapshot(context.Background(), ws)
			if err != nil || after != before {
				t.Fatalf("obligation-only change altered workspace: %v", err)
			}
			if v := measure(); v.Verdict() != VerifyFailed || !strings.Contains(v.Output, "tagged obligation must run") {
				t.Fatalf("new tagged obligation reused a pass: %s: %s", v.Verdict(), v.Output)
			}
			wantFull := 0
			if final {
				wantFull = 1
			}
			if fullRuns != wantFull {
				t.Fatalf("full-suite launches = %d, want %d", fullRuns, wantFull)
			}
		})
	}
}

func TestGateKey_IncludesBaselineIdentity(t *testing.T) {
	ws, result := fixPolicyWorkspace(t)
	ctx := context.Background()
	initial := gateKey(ctx, "own", ws, result)
	for _, tc := range []struct {
		name     string
		preimage PreImage
	}{
		{name: "absent"},
		{name: "empty", preimage: PreImage{Existed: true}},
		{name: "content", preimage: PreImage{Existed: true, Content: "package fixpolicyprobe\n"}},
		{name: "unknown", preimage: PreImage{Unknown: "unreadable"}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			result.PreWriteContents = map[string]PreImage{"a.go": tc.preimage}
			key := gateKey(ctx, "own", ws, result)
			if key == "" || key == initial {
				t.Fatal("changed baseline reused the previous gate identity")
			}
			initial = key
		})
	}
	key := gateKey(ctx, "own", ws, result)
	result.WrittenPaths = append(result.WrittenPaths, "a.go")
	if got := gateKey(ctx, "own", ws, result); got != key {
		t.Fatal("duplicate written path invalidated an unchanged obligation")
	}
}
