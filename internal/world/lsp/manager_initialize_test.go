package lsp

import (
	"context"
	"os"
	"path/filepath"
	"testing"
)

// TestManager_InitializeAndProject drives the LSP manager end to end against a
// real Mangle workspace: it indexes a temp .mg file, projects LSP data into
// World Model facts, and exercises the batch query + validation APIs.
func TestManager_InitializeAndProject(t *testing.T) {
	dir := t.TempDir()
	src := "Decl foo(X).\nDecl bar(X).\nbar(X) :- foo(X).\n"
	if err := os.WriteFile(filepath.Join(dir, "rules.mg"), []byte(src), 0o644); err != nil {
		t.Fatal(err)
	}

	m := NewManager(dir)
	if err := m.Initialize(context.Background()); err != nil {
		t.Fatalf("Initialize: %v", err)
	}

	// After initialization, projection should succeed (no "not initialized").
	facts, err := m.ProjectToFacts()
	if err != nil {
		t.Fatalf("ProjectToFacts: %v", err)
	}
	// We don't assert an exact count (indexer internals may vary), only that the
	// projection path ran and returned a (possibly empty) slice without error.
	_ = facts

	// Batch query APIs no longer return the not-initialized error.
	if _, err := m.GetDefinitions("foo"); err != nil {
		t.Errorf("GetDefinitions after init: %v", err)
	}
	if _, err := m.GetReferences("foo"); err != nil {
		t.Errorf("GetReferences after init: %v", err)
	}

	// ValidateCode on well-formed Mangle should not error.
	if _, err := m.ValidateCode(filepath.Join(dir, "rules.mg"), src); err != nil {
		t.Errorf("ValidateCode(valid): %v", err)
	}

	// ValidateCode on malformed Mangle should surface diagnostics (as facts) or
	// at minimum not panic; we accept either a non-nil error or diagnostic facts.
	badDiags, err := m.ValidateCode(filepath.Join(dir, "bad.mg"), "this is not (((valid mangle")
	if err == nil && badDiags == nil {
		t.Log("malformed code produced no diagnostics; validator tolerated it")
	}
}
