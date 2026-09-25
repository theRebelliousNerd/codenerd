package core

import (
	"context"
	"path/filepath"
	"testing"

	"codenerd/internal/store"
	"codenerd/internal/types"
)

// HydrateSessionContext owns three predicates and replaces each whole: a
// hydration with nothing to load leaves none of the previous session's rows
// behind. The three retractions are one predicate set now
// (hydratedSessionPredicates), so this pins the set's membership: dropping a
// predicate from it would leave that one's stale rows in the kernel.
func TestHydrateSessionContext_ShouldReplaceAllThreeOwnedPredicates(t *testing.T) {
	db, err := store.NewLocalStore(filepath.Join(t.TempDir(), "kb.db"))
	if err != nil {
		t.Fatalf("NewLocalStore: %v", err)
	}
	t.Cleanup(func() { _ = db.Close() })

	kernel, err := NewRealKernel()
	if err != nil {
		t.Fatalf("NewRealKernel: %v", err)
	}
	vs := NewVirtualStore(nil)
	vs.SetLocalDB(db)
	vs.SetKernel(kernel)

	stale := []Fact{
		{Predicate: "session_turn", Args: []any{"old_session", int64(1), "stale input", "stale response"}},
		{Predicate: "similar_content", Args: []any{int64(1), "stale"}},
		{Predicate: "reasoning_trace", Args: []any{"old_trace", types.MangleAtom("/coder"), types.MangleAtom("/code"), "old_session", types.MangleAtom("/true"), int64(10)}},
	}
	for _, f := range stale {
		if err := kernel.Assert(f); err != nil {
			t.Fatalf("fixture %s: %v", f.Predicate, err)
		}
		if rows, _ := kernel.Query(f.Predicate); len(rows) != 1 {
			t.Fatalf("fixture %s did not land: %v", f.Predicate, rows)
		}
	}

	if _, err := vs.HydrateSessionContext(context.Background(), "", "", nil); err != nil {
		t.Fatalf("HydrateSessionContext: %v", err)
	}
	for _, f := range stale {
		rows, err := kernel.Query(f.Predicate)
		if err != nil {
			t.Fatalf("Query %s: %v", f.Predicate, err)
		}
		if len(rows) != 0 {
			t.Errorf("%s kept %d stale row(s) across a hydration: %v", f.Predicate, len(rows), rows)
		}
	}
}
