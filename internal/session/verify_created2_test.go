package session

import (
	"strings"
	"testing"

	"codenerd/internal/core"
	"codenerd/internal/types"
)

// These tests exercise the test-obligation rule
//
//	missing_test_for(File) :- created_source(File), !test_coverage(File).
//
// declared in internal/core/defaults/policy/coder_safety.mg, against a real
// kernel loading the real policy corpus, so what fires here is what ships.
//
// They assert the facts a turn would have produced and then call
// checkHollowSuccess, rather than driving executeToolCall. An earlier version
// of this file drove the full tool path; it needs a tool registry and workspace
// setup these tests do not have, so three tests failed with "tool write_file
// not found in any registry" and three others passed VACUOUSLY -- they
// discarded the same error with `_, _ =`, so no fact was ever asserted and the
// obligation check trivially succeeded. A test that passes because nothing
// happened is worse than one that fails.
func newObligationExec(t *testing.T) *Executor {
	t.Helper()
	k, err := core.NewRealKernel()
	if err != nil {
		t.Fatalf("NewRealKernel: %v", err)
	}
	e := NewExecutor(k, &MockVirtualStore{}, &MockLLMClient{}, &MockJITCompiler{}, &MockConfigFactory{}, &MockTransducer{})
	e.kernel = k
	e.config.WorkspaceRoot = t.TempDir()
	return e
}

// mutationResult builds the ExecutionResult shape a completed write-oriented
// turn produces, so checkHollowSuccess reaches the obligation check instead of
// failing earlier for want of a write.
func mutationResult() *ExecutionResult {
	res := &ExecutionResult{ToolCallsExecuted: 1, SuccessfulToolCalls: 1, SuccessfulWriteTools: 1}
	res.Intent.Verb = "/create"
	res.Intent.Category = "/mutation"
	return res
}

func assertCreatedSource(t *testing.T, e *Executor, path string) {
	t.Helper()
	fact := types.Fact{
		Predicate: "created_source",
		Args:      []any{types.MangleString(path)},
	}
	if err := e.kernel.Assert(fact); err != nil {
		t.Fatalf("assert created_source(%q): %v", path, err)
	}
	e.perTurnCreatedSourceFacts = append(e.perTurnCreatedSourceFacts, fact)
}

func assertTestFileFor(t *testing.T, e *Executor, testPath, sourcePath string) {
	t.Helper()
	if err := e.kernel.Assert(types.Fact{
		Predicate: "test_file_for",
		Args:      []any{types.MangleString(testPath), types.MangleString(sourcePath)},
	}); err != nil {
		t.Fatalf("assert test_file_for(%q, %q): %v", testPath, sourcePath, err)
	}
}

// A turn that created source and no test owes a test, and the message must name
// the file so the operator knows which one.
func TestVerify_CreateNoTestFails(t *testing.T) {
	e := newObligationExec(t)
	assertCreatedSource(t, e, "pkg/foo.go")

	err := e.checkHollowSuccess(mutationResult())
	if err == nil {
		t.Fatal("expected an obligation failure for created source with no test")
	}
	if !isHollowSuccessError(err) {
		t.Fatalf("expected a hollow-success error, got %T: %v", err, err)
	}
	if !strings.Contains(err.Error(), "foo.go") {
		t.Fatalf("error must name the uncovered file, got: %v", err)
	}
	if !strings.Contains(strings.ToLower(err.Error()), "test") {
		t.Fatalf("error must mention the missing test, got: %v", err)
	}
}

// Source and its test written in the same turn satisfy the rule without waiting
// for a world rescan, because the turn asserts the same test_file_for pairing
// the scanner emits.
func TestVerify_CreateWithTestPasses(t *testing.T) {
	e := newObligationExec(t)
	assertCreatedSource(t, e, "pkg/foo.go")
	assertTestFileFor(t, e, "pkg/foo_test.go", "pkg/foo.go")

	if err := e.checkHollowSuccess(mutationResult()); err != nil {
		t.Fatalf("source plus its test must satisfy the obligation, got: %v", err)
	}
}

// Editing an existing file is not creating new code, so nothing is asserted and
// nothing is owed.
func TestVerify_EditNoObligation(t *testing.T) {
	e := newObligationExec(t)

	if err := e.checkHollowSuccess(mutationResult()); err != nil {
		t.Fatalf("an edit records no created_source and must raise no obligation, got: %v", err)
	}
}

// Only Go source raises an obligation.
func TestVerify_NonGoNoObligation(t *testing.T) {
	e := newObligationExec(t)

	facts, err := e.kernel.Query("created_source")
	if err != nil {
		t.Fatalf("query created_source: %v", err)
	}
	if len(facts) != 0 {
		t.Fatalf("no created_source expected before the turn, got %v", facts)
	}
	if err := e.checkHollowSuccess(mutationResult()); err != nil {
		t.Fatalf("a non-Go write must raise no obligation, got: %v", err)
	}
}

// The obligation must not leak into the next turn. A created_source left behind
// would fail every later turn forever, which is worse than the defect it guards.
func TestVerify_StaleNotLeak(t *testing.T) {
	e := newObligationExec(t)
	assertCreatedSource(t, e, "pkg/foo.go")

	if err := e.checkHollowSuccess(mutationResult()); err == nil {
		t.Fatal("expected the first turn to fail its obligation")
	}

	facts, err := e.kernel.Query("created_source")
	if err != nil {
		t.Fatalf("query created_source after check: %v", err)
	}
	if len(facts) != 0 {
		t.Fatalf("created_source must be retracted after the check, got %v", facts)
	}

	// The next turn starts clean and must not inherit the previous obligation.
	if err := e.checkHollowSuccess(mutationResult()); err != nil {
		t.Fatalf("a later turn must not inherit the obligation, got: %v", err)
	}
}

// A created_source argument must be stored as a string, not a Mangle name
// constant. A path beginning with a slash would otherwise be stored as a name
// and join with nothing.
func TestVerify_MangleStringType(t *testing.T) {
	e := newObligationExec(t)
	assertCreatedSource(t, e, "/pkg/foo.go")

	facts, err := e.kernel.Query("created_source")
	if err != nil {
		t.Fatalf("query created_source: %v", err)
	}
	if len(facts) == 0 {
		t.Fatal("expected a created_source fact; a zero-result query cannot prove the term type")
	}
	if len(facts[0].Args) == 0 {
		t.Fatalf("created_source fact has no arguments: %v", facts[0])
	}
	got, ok := facts[0].Args[0].(string)
	if !ok {
		t.Fatalf("created_source argument must round-trip as a string, got %T (%v)", facts[0].Args[0], facts[0].Args[0])
	}
	if got != "/pkg/foo.go" {
		t.Fatalf("created_source argument = %q, want %q", got, "/pkg/foo.go")
	}
}

// With no kernel there is nothing to derive from, so the check is skipped rather
// than failing the turn.
func TestVerify_NilKernelSkips(t *testing.T) {
	e := NewExecutor(nil, &MockVirtualStore{}, &MockLLMClient{}, &MockJITCompiler{}, &MockConfigFactory{}, &MockTransducer{})
	e.config.WorkspaceRoot = t.TempDir()

	if err := e.checkHollowSuccess(mutationResult()); err != nil {
		t.Fatalf("a nil kernel must skip the obligation check, got: %v", err)
	}
}
