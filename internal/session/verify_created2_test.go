package session

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"codenerd/internal/core"
	"codenerd/internal/types"
)

// These tests exercise the test-obligation rule
//
//	turn_missing_test(Turn, File) :- turn_created_source(Turn, File), !turn_test_coverage(Turn, File).
//
// declared in internal/core/defaults/policy/coder_safety.mg, against a real
// kernel loading the real policy corpus, so what fires here is what ships.
//
// They record what a turn would have created and then call checkHollowSuccess,
// rather than driving executeToolCall. An earlier version of this file drove
// the full tool path; it needs a tool registry and workspace setup these tests
// do not have, so three tests failed with "tool write_file not found in any
// registry" and three others passed VACUOUSLY -- they discarded the same error
// with `_, _ =`, so no fact was ever asserted and the obligation check
// trivially succeeded. A test that passes because nothing happened is worse
// than one that fails.
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
	res := &ExecutionResult{ToolCallsExecuted: 1, SuccessfulToolCalls: 1, WriteToolEffects: 1}
	res.Intent.Verb = "/create"
	res.Intent.Category = "/mutation"
	return res
}

// recordCreatedSource records a Go source the turn created, as
// recordGoFileCreations does from inside the tool loop.
func recordCreatedSource(e *Executor, path string) {
	writeObligationFile(e, path, "package foo\n")
	e.recordGoFileCreations(map[string]bool{path: false}, nil)
}

func writeObligationFile(e *Executor, path, content string) {
	ws := e.config.WorkspaceRoot
	var phys string
	if filepath.IsAbs(path) {
		phys = filepath.FromSlash(path)
	} else {
		phys = filepath.Join(ws, filepath.FromSlash(path))
	}
	if err := os.MkdirAll(filepath.Dir(phys), 0o755); err != nil {
		panic(err)
	}
	if err := os.WriteFile(phys, []byte(content), 0o644); err != nil {
		panic(err)
	}
}

func removeWorkspaceFile(e *Executor, path string) {
	ws := e.config.WorkspaceRoot
	var phys string
	if filepath.IsAbs(path) {
		phys = filepath.FromSlash(path)
	} else {
		phys = filepath.Join(ws, filepath.FromSlash(path))
	}
	if err := os.Remove(phys); err != nil {
		panic(err)
	}
}

// assertWorldTestPairing is the world scanner's test_file_for fact: a test the
// workspace already pairs with the source.
func assertWorldTestPairing(t *testing.T, e *Executor, testPath, sourcePath string) types.Fact {
	t.Helper()
	fact := types.Fact{
		Predicate: "test_file_for",
		Args:      []any{types.MangleString(testPath), types.MangleString(sourcePath)},
	}
	if err := e.kernel.Assert(fact); err != nil {
		t.Fatalf("assert test_file_for(%q, %q): %v", testPath, sourcePath, err)
	}
	return fact
}

// A turn that created source and no test owes a test, and the message must name
// the file so the operator knows which one.
func TestVerify_CreateNoTestFails(t *testing.T) {
	t.Parallel()
	e := newObligationExec(t)
	recordCreatedSource(e, "pkg/foo.go")

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
// for a world rescan: the turn's own record pairs them.
func TestVerify_CreateWithTestPasses(t *testing.T) {
	t.Parallel()
	e := newObligationExec(t)
	writeObligationFile(e, "pkg/foo.go", "package foo\n")
	writeObligationFile(e, "pkg/foo_test.go", "package foo\n")
	e.recordGoFileCreations(map[string]bool{"pkg/foo.go": false, "pkg/foo_test.go": false}, nil)

	if err := e.checkHollowSuccess(mutationResult()); err != nil {
		t.Fatalf("source plus its test must satisfy the obligation, got: %v", err)
	}
}

// A created source the world already pairs with a test -- the scanner's
// test_file_for -- owes nothing more.
func TestVerify_CreateCoveredByAWorldTestPasses(t *testing.T) {
	t.Parallel()
	e := newObligationExec(t)
	recordCreatedSource(e, "pkg/foo.go")
	assertWorldTestPairing(t, e, "pkg/foo_test.go", "pkg/foo.go")

	if err := e.checkHollowSuccess(mutationResult()); err != nil {
		t.Fatalf("a source the world pairs with a test must satisfy the obligation, got: %v", err)
	}
}

// Editing an existing file is not creating new code, so nothing is recorded and
// nothing is owed.
func TestVerify_EditNoObligation(t *testing.T) {
	t.Parallel()
	e := newObligationExec(t)
	e.recordGoFileCreations(map[string]bool{"pkg/foo.go": true}, nil)

	if err := e.checkHollowSuccess(mutationResult()); err != nil {
		t.Fatalf("an edit records no created source and must raise no obligation, got: %v", err)
	}
}

// Only Go source raises an obligation.
func TestVerify_NonGoNoObligation(t *testing.T) {
	t.Parallel()
	e := newObligationExec(t)
	e.recordGoFileCreations(map[string]bool{"docs/notes.md": false}, nil)

	if len(e.turnCreatedSources) != 0 {
		t.Fatalf("a markdown file is not Go source: %v", e.turnCreatedSources)
	}
	if err := e.checkHollowSuccess(mutationResult()); err != nil {
		t.Fatalf("a non-Go write must raise no obligation, got: %v", err)
	}
}

// The obligation must not leak into the next turn. A created source left behind
// would fail every later turn forever, which is worse than the defect it guards.
func TestVerify_StaleNotLeak(t *testing.T) {
	t.Parallel()
	e := newObligationExec(t)
	recordCreatedSource(e, "pkg/foo.go")

	if err := e.checkHollowSuccess(mutationResult()); err == nil {
		t.Fatal("expected the first turn to fail its obligation")
	}

	facts, err := e.kernel.Query("turn_created_source")
	if err != nil {
		t.Fatalf("query turn_created_source after check: %v", err)
	}
	if len(facts) != 0 || len(e.turnCreatedSources) != 0 {
		t.Fatalf("the turn's created sources must be cleared after the check, got %v and %v", facts, e.turnCreatedSources)
	}

	// The next turn starts clean and must not inherit the previous obligation.
	if err := e.checkHollowSuccess(mutationResult()); err != nil {
		t.Fatalf("a later turn must not inherit the obligation, got: %v", err)
	}
}

// A source created then deleted in the same turn owes nothing at end of turn:
// the file is gone, so there is no new code to test.
func TestVerify_CreatedThenDeletedOwesNothing(t *testing.T) {
	e := newObligationExec(t)
	recordCreatedSource(e, "pkg/foo.go")
	removeWorkspaceFile(e, "pkg/foo.go")

	if err := e.checkHollowSuccess(mutationResult()); err != nil {
		t.Fatalf("a created-then-deleted source must owe nothing, got: %v", err)
	}
}

// Deleting the source discharges the pair even when its test survives: the
// surviving test covers nothing that still exists.
func TestVerify_SourceDeletedTestSurvivesOwesNothing(t *testing.T) {
	e := newObligationExec(t)
	writeObligationFile(e, "pkg/foo.go", "package foo\n")
	writeObligationFile(e, "pkg/foo_test.go", "package foo\n")
	e.recordGoFileCreations(map[string]bool{"pkg/foo.go": false, "pkg/foo_test.go": false}, nil)
	removeWorkspaceFile(e, "pkg/foo.go")

	if err := e.checkHollowSuccess(mutationResult()); err != nil {
		t.Fatalf("a deleted source must owe nothing even when its test survives, got: %v", err)
	}
}

// A covering test deleted in the same turn cannot discharge a surviving
// source: the source still exists and still owes exactly what it owed.
func TestVerify_DeletedTestDoesNotCoverSurvivor(t *testing.T) {
	e := newObligationExec(t)
	writeObligationFile(e, "pkg/foo.go", "package foo\n")
	writeObligationFile(e, "pkg/foo_test.go", "package foo\n")
	e.recordGoFileCreations(map[string]bool{"pkg/foo.go": false, "pkg/foo_test.go": false}, nil)
	removeWorkspaceFile(e, "pkg/foo_test.go")

	err := e.checkHollowSuccess(mutationResult())
	if err == nil {
		t.Fatal("a source whose only covering test was deleted must still owe a test")
	}
	if !isHollowSuccessError(err) {
		t.Fatalf("expected a hollow-success error, got %T: %v", err, err)
	}
}

// A turn_created_source path must be stored as a string, not a Mangle name
// constant. A path beginning with a slash would otherwise be stored as a name
// and join with nothing.
func TestVerify_MangleStringType(t *testing.T) {
	t.Parallel()
	e := newObligationExec(t)
	abs := filepath.Join(e.config.WorkspaceRoot, "pkg", "foo.go")
	writeObligationFile(e, abs, "package foo\n")
	e.recordGoFileCreations(map[string]bool{abs: false}, nil)
	e.assertTurnEvidence(testTurn, "/create", mutationResult())
	defer e.cleanupTurnFacts()

	facts, err := e.kernel.Query("turn_created_source")
	if err != nil {
		t.Fatalf("query turn_created_source: %v", err)
	}
	if len(facts) == 0 {
		t.Fatal("expected a turn_created_source fact; a zero-result query cannot prove the term type")
	}
	if len(facts[0].Args) < 2 {
		t.Fatalf("turn_created_source fact lacks its path: %v", facts[0])
	}
	got, ok := facts[0].Args[1].(string)
	if !ok {
		t.Fatalf("turn_created_source path must round-trip as a string, got %T (%v)", facts[0].Args[1], facts[0].Args[1])
	}
	if got != abs {
		t.Fatalf("turn_created_source path = %q, want %q", got, abs)
	}
}

// An empty canonical path prunes: there is nothing to keep, and asserting an
// empty turn_created_source fact would be noise.
func TestVerify_CreatedFileExists_EmptyPrunes(t *testing.T) {
	e := newObligationExec(t)
	if e.createdFileExists("") {
		t.Fatal("empty canonical path must not exist")
	}
	if e.createdFileExists("   ") {
		t.Fatal("whitespace-only canonical path must not exist")
	}
}

// When the workspace cannot be resolved the existence cannot be determined, so
// a relative path is kept. This is the conservative fallback that preserves
// the previous behaviour instead of pruning on an uncertain signal.
func TestVerify_CreatedFileExists_UnresolvableWorkspaceKeeps(t *testing.T) {
	e := newObligationExec(t)
	e.config.WorkspaceRoot = string([]byte{0})
	if !e.createdFileExists("pkg/foo.go") {
		t.Fatal("relative path with unresolvable workspace must be kept")
	}
}

// A stat failure other than NotExist keeps the file. A path under a regular
// file fails with ENOTDIR, which IsNotExist reports as false, so the
// obligation is conservatively retained rather than pruned on an error.
func TestVerify_CreatedFileExists_StatErrorKeeps(t *testing.T) {
	e := newObligationExec(t)
	writeObligationFile(e, "blocker", "not a dir\n")
	if !e.createdFileExists("blocker/sub/file.go") {
		t.Fatal("ENOTDIR stat error must keep the file")
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
