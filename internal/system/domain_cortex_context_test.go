package system

import (
	"testing"

	"codenerd/internal/core"
	"codenerd/internal/types"
)

// A modified file reaches the context window through the production kernel.
//
// context_relevant(File, /p85) :- modified(File) fires only in the world
// shard, which owns modified. should_include_context(F, P) :-
// context_relevant(F, P) fires in every shard, because another
// context_relevant rule reads shared user_intent, and the derivation map took
// "fires everywhere" to mean "the same everywhere": it read the head from the
// catch-all alone, where modified never is. Measured before the fix
// (Lane B, 2026-09-25): modified(File) had 0 rows in the catch-all and 1 in
// the Cortex, and should_include_context had no row for the file.
func TestDomainCortex_AModifiedFileIsIncludedInContext(t *testing.T) {
	ck, err := NewDomainCortex(t.TempDir())
	if err != nil {
		t.Fatalf("NewDomainCortex: %v", err)
	}
	if err := ck.Assert(core.Fact{Predicate: "modified", Args: []any{"x.go"}}); err != nil {
		t.Fatalf("assert modified: %v", err)
	}

	rows, err := ck.Query("should_include_context")
	if err != nil {
		t.Fatalf("query should_include_context: %v", err)
	}
	// Two rules make a modified file relevant: modification relevance
	// (/p85) and hop-0 reachability, which counts it as focal (/p100). Both
	// derive in the world shard only.
	priorities := map[string]bool{}
	for _, r := range rows {
		if len(r.Args) == 2 && types.ExtractString(r.Args[0]) == "x.go" {
			priorities[types.ExtractString(r.Args[1])] = true
		}
	}
	if len(priorities) == 0 {
		t.Fatalf("should_include_context has no row for the modified file x.go; rows: %v", rows)
	}
	for _, want := range []string{"/p85", "/p100"} {
		if !priorities[want] {
			t.Errorf("should_include_context(x.go, %s) missing; got priorities %v", want, priorities)
		}
	}
}
