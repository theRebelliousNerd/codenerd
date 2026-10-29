package core

import (
	"errors"
	"fmt"
	"path/filepath"
	"sort"
	"strings"
	"testing"

	"codenerd/internal/store"

	"codeberg.org/TauCeti/mangle-go/ast"
)

// Differential regression tests for Query vs QueryCallback on external
// predicates (B2). Contract, from queryExternalVirtualStore: for a declared
// external predicate queried with a pattern and a wired VirtualStore, live
// results supersede stored eval-cache rows; a provider error surfaces with
// zero callbacks made; Query and QueryCallback return identical fact sets.
//
// Every case below drives the real kernel/VirtualStore boundary: a booted
// RealKernel with the query_learned/2 external declaration, a VirtualStore
// over a real temp-file LocalStore holding the scripted live rows, and stored
// eval-cache rows planted through the kernel's own Assert path.

// diffBoot boots a kernel with the query_learned/2 external declaration,
// wires a VirtualStore over a fresh LocalStore, and returns both. Stored
// cache rows are planted per-case via diffPlantStored.
func diffBoot(t *testing.T) (*RealKernel, *store.LocalStore) {
	t.Helper()
	k := setupMockKernel(t)
	found := false
	for pred, d := range k.programInfo.Decls {
		if pred.Symbol == "query_learned" && pred.Arity == 2 && d.IsExternal() {
			found = true
			break
		}
	}
	if !found {
		k.AppendPolicy(`Decl query_learned(Predicate, Args) descr [external(), mode('-', '-')] bound [/string, /string].`)
		if err := k.Evaluate(); err != nil {
			t.Fatalf("Evaluate external decl: %v", err)
		}
	}
	ls, err := store.NewLocalStore(filepath.Join(t.TempDir(), "know.db"))
	if err != nil {
		t.Fatalf("NewLocalStore: %v", err)
	}
	t.Cleanup(func() { _ = ls.Close() })
	k.SetVirtualStore(&VirtualStore{localDB: ls})
	return k, ls
}

// diffPlantStored plants stored eval-cache rows for the external through the
// kernel's own Assert path.
func diffPlantStored(t *testing.T, k *RealKernel, rows ...Fact) {
	t.Helper()
	for _, f := range rows {
		if err := k.Assert(f); err != nil {
			t.Fatalf("Assert stored row %+v: %v", f, err)
		}
	}
}

// diffStoreLive scripts live provider rows into cold storage.
func diffStoreLive(t *testing.T, ls *store.LocalStore, pred string, args ...any) {
	t.Helper()
	if err := ls.StoreFact(pred, args, "fact", 1); err != nil {
		t.Fatalf("StoreFact %s: %v", pred, err)
	}
}

// diffCollect gathers QueryCallback's emitted facts.
func diffCollect(t *testing.T, k *RealKernel, pattern string) ([]Fact, error) {
	t.Helper()
	var out []Fact
	err := k.QueryCallback(pattern, func(f Fact) error {
		out = append(out, f)
		return nil
	})
	return out, err
}

func diffKey(f Fact) string {
	var sb strings.Builder
	sb.WriteString(f.Predicate)
	for _, a := range f.Args {
		fmt.Fprintf(&sb, "|%v", a)
	}
	return sb.String()
}

// diffAssertSameSet fails unless Query and QueryCallback yield identical sets.
func diffAssertSameSet(t *testing.T, k *RealKernel, pattern string) []Fact {
	t.Helper()
	sliceRows, err := k.Query(pattern)
	if err != nil {
		t.Fatalf("Query(%q): %v", pattern, err)
	}
	cbRows, err := diffCollect(t, k, pattern)
	if err != nil {
		t.Fatalf("QueryCallback(%q): %v", pattern, err)
	}
	a := make([]string, 0, len(sliceRows))
	for _, f := range sliceRows {
		a = append(a, diffKey(f))
	}
	b := make([]string, 0, len(cbRows))
	for _, f := range cbRows {
		b = append(b, diffKey(f))
	}
	sort.Strings(a)
	sort.Strings(b)
	if strings.Join(a, "\n") != strings.Join(b, "\n") {
		t.Fatalf("Query vs QueryCallback mismatch for %q:\nQuery=%v\nCallback=%v", pattern, a, b)
	}
	return sliceRows
}

func diffFirstArgs(rows []Fact) []string {
	var out []string
	for _, f := range rows {
		if len(f.Args) > 0 {
			out = append(out, fmt.Sprintf("%v", f.Args[0]))
		}
	}
	return out
}

// Conflicting cached rows must never escape: both APIs return the live set.
func TestQueryDifferential_ConflictingCacheSuperseded(t *testing.T) {
	k, ls := diffBoot(t)
	diffPlantStored(t, k, Fact{Predicate: "query_learned", Args: []any{"stale_pred", "stale-marker"}})
	diffStoreLive(t, ls, "live_pred", "a")

	rows := diffAssertSameSet(t, k, "query_learned(P, A)")
	if len(rows) != 1 {
		t.Fatalf("expected exactly the 1 live row, got %d: %v", len(rows), rows)
	}
	for _, a := range diffFirstArgs(rows) {
		if strings.Contains(a, "stale_pred") {
			t.Fatalf("stale cache row escaped into results: %v", rows)
		}
	}
}

// Matching cached rows must not duplicate: one live row in, one row out.
func TestQueryDifferential_MatchingCacheNoDuplicates(t *testing.T) {
	k, ls := diffBoot(t)
	diffStoreLive(t, ls, "live_pred", "a")
	live, err := k.Query("query_learned(P, A)")
	if err != nil {
		t.Fatalf("Query live: %v", err)
	}
	if len(live) == 0 {
		t.Fatal("expected live rows to plant as cache")
	}
	// Plant the live encoding back as stored cache: self-calibrating match.
	diffPlantStored(t, k, live...)

	rows := diffAssertSameSet(t, k, "query_learned(P, A)")
	if len(rows) != len(live) {
		t.Fatalf("expected %d rows (no duplicates), got %d", len(live), len(rows))
	}
}

// Empty live results with stored rows present: both APIs answer empty.
func TestQueryDifferential_EmptyLiveStaysEmpty(t *testing.T) {
	k, _ := diffBoot(t)
	diffPlantStored(t, k, Fact{Predicate: "query_learned", Args: []any{"stale_pred", "stale-marker"}})

	rows := diffAssertSameSet(t, k, "query_learned(P, A)")
	if len(rows) != 0 {
		t.Fatalf("expected empty results with empty live provider, got %v", rows)
	}
}

// Provider failure: both APIs error, and the callback fires zero times.
func TestQueryDifferential_ProviderErrorZeroCallbacks(t *testing.T) {
	k := setupMockKernel(t)
	k.SetVirtualStore(&VirtualStore{localDB: nil})
	diffPlantStored(t, k, Fact{Predicate: "query_learned", Args: []any{"stale_pred", "stale-marker"}})

	if _, err := k.Query("query_learned(P, A)"); err == nil {
		t.Fatal("Query with failing provider: expected error, got nil")
	}
	calls := 0
	err := k.QueryCallback("query_learned(P, A)", func(f Fact) error {
		calls++
		return nil
	})
	if err == nil {
		t.Fatal("QueryCallback with failing provider: expected error, got nil")
	}
	if calls != 0 {
		t.Fatalf("provider failure emitted %d callbacks before erroring; want 0", calls)
	}
}

// Bound inputs filter live rows identically through both APIs.
func TestQueryDifferential_BoundInputs(t *testing.T) {
	k, ls := diffBoot(t)
	diffStoreLive(t, ls, "wanted_pred", "a")
	diffStoreLive(t, ls, "other_pred", "b")

	rows := diffAssertSameSet(t, k, `query_learned("wanted_pred", A)`)
	if len(rows) != 1 {
		t.Fatalf("expected 1 bound row, got %d: %v", len(rows), rows)
	}
	if got := diffFirstArgs(rows); len(got) != 1 || !strings.Contains(got[0], "wanted_pred") {
		t.Fatalf("bound result first arg = %v, want wanted_pred", got)
	}

	empty := diffAssertSameSet(t, k, `query_learned("missing_pred", A)`)
	if len(empty) != 0 {
		t.Fatalf("expected empty bound miss, got %v", empty)
	}
}

// Ordinary non-external facts are untouched by the external fast path.
func TestQueryDifferential_NonExternalUnchanged(t *testing.T) {
	k := setupMockKernel(t)
	k.AppendPolicy("Decl diff_plain(X).")
	if err := k.Evaluate(); err != nil {
		t.Fatalf("Evaluate: %v", err)
	}
	for _, v := range []string{"a", "b", "c"} {
		if err := k.Assert(Fact{Predicate: "diff_plain", Args: []any{v}}); err != nil {
			t.Fatalf("Assert: %v", err)
		}
	}
	rows := diffAssertSameSet(t, k, "diff_plain(X)")
	if len(rows) != 3 {
		t.Fatalf("expected 3 stored rows, got %d", len(rows))
	}
}

// Early termination must stop on live data, never on a stale cache row.
// Pre-fix, QueryCallback streamed stored rows first, so a stop-after-first
// consumer received the stale row before the live provider was consulted.
func TestQueryDifferential_EarlyStopSeesLive(t *testing.T) {
	k, ls := diffBoot(t)
	diffPlantStored(t, k, Fact{Predicate: "query_learned", Args: []any{"stale_pred", "stale-marker"}})
	diffStoreLive(t, ls, "live_pred", "a")

	stop := errors.New("stop after first")
	var got []Fact
	err := k.QueryCallback("query_learned(P, A)", func(f Fact) error {
		got = append(got, f)
		return stop
	})
	if !errors.Is(err, stop) {
		t.Fatalf("expected stop error, got %v", err)
	}
	if len(got) != 1 {
		t.Fatalf("expected exactly 1 row before stop, got %d", len(got))
	}
	if first := diffFirstArgs(got)[0]; !strings.Contains(first, "live_pred") {
		t.Fatalf("early stop received stale row %q, want live_pred", first)
	}
}

// The decl guard: without an external declaration the helper must not engage.
func TestQueryDifferential_DeclIsExternal(t *testing.T) {
	k, _ := diffBoot(t)
	d, ok := k.programInfo.Decls[ast.PredicateSym{Symbol: "query_learned", Arity: 2}]
	if !ok {
		t.Fatal("query_learned/2 not declared")
	}
	if !d.IsExternal() {
		t.Fatal("query_learned/2 declared but not external")
	}
}
