package context

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"testing"

	"codenerd/internal/core"
)

// Retention is the kernel's decision (TODO-CTX-04A,
// Docs/architecture/context/TODO.md): context_must_retain names the predicates
// the window always carries, and getCoreFacts obeys it. Until 2026-09-25 the
// list was a Go literal the policy could not see or extend.

// The shipped policy never retains less than the constitutional floor. A policy
// edit that drops one of these would take a safety fact out of every prompt.
func TestRetentionPolicy_KeepsTheConstitutionalFloor(t *testing.T) {
	comp := newKernelBackedCompressor(t)
	rows, err := comp.kernel.Query("context_must_retain")
	if err != nil {
		t.Fatalf("query context_must_retain: %v", err)
	}
	derived := map[string]bool{}
	for _, r := range rows {
		derived[strings.TrimPrefix(argString(r.Args[0]), "/")] = true
	}
	for _, p := range constitutionalFloor {
		if !derived[p] {
			t.Errorf("context_must_retain does not retain %s (derived: %v)", p, rows)
		}
	}
}

// A predicate the policy retains is retained, whatever Go thinks: the proof
// that the decision moved into the kernel.
func TestGetCoreFacts_RetainsWhatTheKernelDecides(t *testing.T) {
	comp := newKernelBackedCompressor(t)
	comp.kernel.AppendPolicy(`
Decl ctx_test_invariant(Note) bound [/string].
context_must_retain(/ctx_test_invariant).
`)
	if err := comp.kernel.Assert(fact("ctx_test_invariant", "never evict this")); err != nil {
		t.Fatalf("assert: %v", err)
	}

	comp.mu.Lock()
	got := comp.getCoreFacts()
	comp.mu.Unlock()
	if !containsFact(got, "ctx_test_invariant", "never evict this") {
		t.Fatalf("a predicate context_must_retain names was not retained: %v", got)
	}
	if n := comp.GetSelectionStats().RetentionFloorUsed; n != 0 {
		t.Errorf("RetentionFloorUsed = %d with the policy loaded, want 0", n)
	}

	ctx, err := comp.BuildContext(context.Background())
	if err != nil {
		t.Fatalf("BuildContext: %v", err)
	}
	if !strings.Contains(ctx.CoreFacts, "never evict this") {
		t.Errorf("the retained fact is missing from the constitutional section:\n%s", ctx.CoreFacts)
	}
}

// A kernel that decides nothing gets the floor, and says so once.
func TestGetCoreFacts_WhenTheKernelDecidesNoRetention_ShouldRetainTheFloorAndCountIt(t *testing.T) {
	comp := newKernelBackedCompressor(t)
	comp.SetKernelReader(&fakeReader{
		rows: map[string][]core.Fact{"block_commit": {fact("block_commit", "floor still holds")}},
		errs: map[string]error{"context_must_retain": errors.New("no such predicate")},
	})

	for range 2 {
		comp.mu.Lock()
		got := comp.getCoreFacts()
		comp.mu.Unlock()
		if !containsFact(got, "block_commit", "floor still holds") {
			t.Fatalf("with no retention decision the floor was not retained: %v", got)
		}
	}
	if n := comp.GetSelectionStats().RetentionFloorUsed; n != 2 {
		t.Errorf("RetentionFloorUsed = %d, want 2", n)
	}
	if n := comp.GetMetrics()["retention_floor_used"]; n != 2 {
		t.Errorf("GetMetrics does not expose the retention floor count: %v", n)
	}
}

// The compressor asks through its reader. In production the reader is the
// domain Cortex and the compressor's own kernel is the catch-all shard, which
// does not hold facts other shards own or derive.
func TestGetCoreFacts_ReadsThroughTheKernelReader(t *testing.T) {
	comp := newKernelBackedCompressor(t)
	comp.SetKernelReader(&fakeReader{
		fallback: comp.kernel,
		rows:     map[string][]core.Fact{"block_commit": {fact("block_commit", "derived in another shard")}},
	})

	ctx, err := comp.BuildContext(context.Background())
	if err != nil {
		t.Fatalf("BuildContext: %v", err)
	}
	if !strings.Contains(ctx.CoreFacts, "derived in another shard") {
		t.Errorf("a retained fact only the reader holds did not reach the window:\n%s", ctx.CoreFacts)
	}
}

// The rendered block says who chose and ordered its active facts, exactly
// once (TODO-CTX-06C).
func TestSerializeCompressedContext_SaysWhoOrderedTheActiveBlock(t *testing.T) {
	comp := newKernelBackedCompressor(t)
	// A fact no inclusion rule fires on alone: the Go fallback picks it.
	if err := comp.kernel.Assert(fact("test_state", "/failing")); err != nil {
		t.Fatalf("assert: %v", err)
	}

	fallback, err := comp.GetContextString(context.Background())
	if err != nil {
		t.Fatalf("GetContextString: %v", err)
	}
	if n := strings.Count(fallback, "heuristic_ordered"); n != 1 || strings.Contains(fallback, "ordered by the kernel") {
		t.Errorf("a Go-fallback block carries %d heuristic_ordered records (want 1) or claims kernel order:\n%s", n, fallback)
	}
	if !strings.Contains(fallback, reasonNoKernelFacts) {
		t.Errorf("the heuristic record does not say why the kernel did not decide:\n%s", fallback)
	}

	if err := comp.kernel.AssertString(`modified("auth.go")`); err != nil {
		t.Fatalf("assert modified: %v", err)
	}
	derived, err := comp.GetContextString(context.Background())
	if err != nil {
		t.Fatalf("GetContextString: %v", err)
	}
	if strings.Contains(derived, "heuristic_ordered") || strings.Count(derived, "ordered by the kernel") != 1 {
		t.Errorf("a kernel-selected block is not marked as the kernel's, exactly once:\n%s", derived)
	}
}

type fakeReader struct {
	fallback *core.RealKernel
	rows     map[string][]core.Fact
	errs     map[string]error
}

func (f *fakeReader) Query(predicate string) ([]core.Fact, error) {
	if err := f.errs[predicate]; err != nil {
		return nil, err
	}
	var out []core.Fact
	if f.fallback != nil {
		got, err := f.fallback.Query(predicate)
		if err != nil {
			return nil, err
		}
		out = append(out, got...)
	}
	return append(out, f.rows[predicate]...), nil
}

func containsFact(facts []core.Fact, predicate string, arg string) bool {
	for _, f := range facts {
		if f.Predicate != predicate {
			continue
		}
		for _, a := range f.Args {
			if argString(a) == arg {
				return true
			}
		}
	}
	return false
}

func argString(a any) string {
	if s, ok := a.(string); ok {
		return s
	}
	return strings.TrimSpace(strings.Trim(fmt.Sprint(a), `"`))
}
