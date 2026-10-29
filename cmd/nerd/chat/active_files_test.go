package chat

import (
	"context"
	"path/filepath"
	"strings"
	"testing"

	"codenerd/internal/core"
)

// SessionContext.ActiveFiles had two consumers and no producer: the 1-hop
// dependency query here, and queryGraphMemory, which walks graph-memory links
// for the same files. Nothing in the repository ever wrote the field, so
// DependencyContext was empty in every session ever run and both retrieval
// paths behind it were unreachable — not broken, not logged, just never
// entered, because the loop they live in iterates an empty slice.
//
// The fixture uses DELIBERATELY MISMATCHED path forms, because that is the
// shape production has. modified() is asserted by internal/tactile with
// whatever path the tool call carried, which is normally absolute; the world
// scan stores dependency_link workspace-relative through world.CanonicalPath.
// A wire that joins them raw is a map lookup that never hits, and a lookup that
// never hits returns an empty slice — indistinguishable from "this file has no
// dependencies". This test fails against that version.
func TestDependencyContextJoinsAcrossTheTwoPathForms(t *testing.T) {
	m, _ := SetupLiveModel(t)

	const rel = "internal/session/executor.go"
	abs := filepath.Join(m.workspace, filepath.FromSlash(rel))

	// The agent edited the file: tactile records the path it was handed.
	if err := m.kernel.Assert(core.Fact{Predicate: "modified", Args: []any{abs}}); err != nil {
		t.Fatalf("assert modified: %v", err)
	}
	// The world scan recorded the same file's import, canonically.
	if err := m.kernel.Assert(core.Fact{
		Predicate: "dependency_link",
		Args:      []any{rel, "pkg:codenerd/internal/types", "codenerd/internal/types"},
	}); err != nil {
		t.Fatalf("assert dependency_link: %v", err)
	}

	sessionCtx := m.buildSessionContext(context.Background())
	if sessionCtx == nil {
		t.Fatal("buildSessionContext returned nil")
	}

	if len(sessionCtx.ActiveFiles) == 0 {
		t.Fatal("ActiveFiles is empty with a modified() fact asserted; the field " +
			"still has no producer, so everything downstream of it stays dark")
	}
	if len(sessionCtx.DependencyContext) == 0 {
		t.Fatalf("DependencyContext is empty though the active file has a "+
			"dependency_link. ActiveFiles=%v — the two producers disagree about "+
			"path form and the join was made on the raw strings",
			sessionCtx.ActiveFiles)
	}
	joined := strings.Join(sessionCtx.DependencyContext, "\n")
	if !strings.Contains(joined, "codenerd/internal/types") {
		t.Errorf("DependencyContext does not name the import it was derived from:\n%s", joined)
	}
}

// The mismatch runs the other way too, and the fix has to cover both ends or
// half of it is decoration.
//
// world.CanonicalPath is the single definition of a file's identity and its own
// header records that a producer has already drifted from it once. Nothing
// forces every future dependency_link writer to be canonical — internal/world's
// own test fixtures assert that predicate with absolute paths — so the fact
// side is normalized as well. This is the case that makes that line
// load-bearing rather than defensive-looking.
func TestDependencyContextJoinsWhenTheFactSideIsTheAbsoluteOne(t *testing.T) {
	m, _ := SetupLiveModel(t)

	const rel = "internal/prompt/compiler.go"
	abs := filepath.Join(m.workspace, filepath.FromSlash(rel))

	// Reversed from the case above: the focus is canonical, the fact is not.
	if err := m.kernel.Assert(core.Fact{Predicate: "modified", Args: []any{rel}}); err != nil {
		t.Fatalf("assert modified: %v", err)
	}
	if err := m.kernel.Assert(core.Fact{
		Predicate: "dependency_link",
		Args:      []any{abs, "pkg:codenerd/internal/logging", "codenerd/internal/logging"},
	}); err != nil {
		t.Fatalf("assert dependency_link: %v", err)
	}

	sessionCtx := m.buildSessionContext(context.Background())
	if sessionCtx == nil {
		t.Fatal("buildSessionContext returned nil")
	}
	joined := strings.Join(sessionCtx.DependencyContext, "\n")
	if !strings.Contains(joined, "codenerd/internal/logging") {
		t.Errorf("an absolute dependency_link did not join to a canonical active "+
			"file. ActiveFiles=%v DependencyContext=%v",
			sessionCtx.ActiveFiles, sessionCtx.DependencyContext)
	}
}

// And a file the session has NOT touched must not drag its dependencies in.
// The point of the field is focus; a join that matches everything is the same
// as no join, and costs prompt budget to say so.
func TestDependencyContextIgnoresFilesTheSessionDidNotTouch(t *testing.T) {
	m, _ := SetupLiveModel(t)

	if err := m.kernel.Assert(core.Fact{
		Predicate: "modified",
		Args:      []any{filepath.Join(m.workspace, "a.go")},
	}); err != nil {
		t.Fatalf("assert modified: %v", err)
	}
	if err := m.kernel.Assert(core.Fact{
		Predicate: "dependency_link",
		Args:      []any{"b.go", "pkg:net/http", "net/http"},
	}); err != nil {
		t.Fatalf("assert dependency_link: %v", err)
	}

	sessionCtx := m.buildSessionContext(context.Background())
	if sessionCtx == nil {
		t.Fatal("buildSessionContext returned nil")
	}
	for _, dep := range sessionCtx.DependencyContext {
		if strings.Contains(dep, "net/http") {
			t.Errorf("a file the session never touched contributed %q", dep)
		}
	}
}

// With nothing modified, the field stays empty and the query is skipped. A
// session that has written nothing has no focus, and inventing one would put
// the whole dependency graph in front of the model.
func TestNoModifiedFilesMeansNoFocusAndNoDependencyContext(t *testing.T) {
	m, _ := SetupLiveModel(t)

	sessionCtx := m.buildSessionContext(context.Background())
	if sessionCtx == nil {
		t.Fatal("buildSessionContext returned nil")
	}
	if len(sessionCtx.ActiveFiles) != 0 {
		t.Errorf("ActiveFiles = %v with nothing modified", sessionCtx.ActiveFiles)
	}
}
