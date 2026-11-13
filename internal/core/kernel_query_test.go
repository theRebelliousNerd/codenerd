package core

import (
	"context"
	"fmt"
	"os"
	"runtime"
	"testing"
	"time"
	"weak"

	"codeberg.org/TauCeti/mangle-go/analysis"
	"codeberg.org/TauCeti/mangle-go/ast"
	"codeberg.org/TauCeti/mangle-go/factstore"
)

// REMEDIATED: All 6 TEST_GAP items — see kernel_query_gaps_test.go:
//   TestKernelQueryGap_MassiveArity (User Extremes - high arity)
//   TestKernelQueryGap_QueryAll_LargeEDB (User Extremes - large EDB)
//   TestKernelQueryGap_LoadFactsFromFile_LargeFile (User Extremes - large files)
//   TestKernelQueryGap_ParseFactString_DeepNesting (User Extremes - deep nesting)
//   TestKernelQueryGap_Concurrency_ReadWriteStarvation (State Conflicts)
//   TestKernelQueryGap_UpdateSystemFacts_MissingWorkspace (State Conflicts - git context)

func TestKernelQuery_Parse(t *testing.T) {
	t.Parallel()
	k := setupMockKernel(t)
	k.AppendPolicy("Decl foo(Name).")
	k.Evaluate()

	// 1. Parse valid query strings (explicitly WITHOUT final dot)
	err := k.AssertString(`foo("bar")`)
	if err != nil {
		t.Errorf("Failed to parse valid fact string: %v", err)
	}

	// 2. Parse invalid query strings
	err = k.AssertString(`foo(,,)`)
	if err == nil {
		t.Error("Expected error for invalid fact string, got nil")
	}
}

func TestKernelQuery_Execute(t *testing.T) {
	t.Parallel()
	k := setupMockKernel(t)
	k.AppendPolicy(`
	Decl test_parent(Name, Child).
	Decl test_ancestor(Name, Descendant).
	
	test_ancestor(X, Y) :- test_parent(X, Y).
	test_ancestor(X, Z) :- test_parent(X, Y), test_ancestor(Y, Z).
	`)
	k.Evaluate()

	k.Assert(Fact{Predicate: "test_parent", Args: []any{"alice", "bob"}})
	k.Assert(Fact{Predicate: "test_parent", Args: []any{"bob", "charlie"}})

	// 1. Execute complex query (recursive join)
	err := k.Evaluate()
	if err != nil {
		t.Fatalf("Evaluate failed: %v", err)
	}

	results, err := k.Query("test_ancestor")
	if err != nil {
		t.Fatalf("Query test_ancestor failed: %v", err)
	}

	// Expect: alice->bob, bob->charlie, alice->charlie
	if len(results) != 3 {
		t.Errorf("Expected 3 test_ancestor facts, got %d", len(results))
	}
}

// ----------------------------------------------------------------------
// Remediation Tests: Null/Undefined/Empty boundaries
// ----------------------------------------------------------------------

func TestQuery_UninitializedKernel(t *testing.T) {
	k := &RealKernel{
		store:       nil,
		programInfo: nil,
		initialized: false, // Explicitly false
	}
	// Should fail cleanly without panicking on nil pointers
	_, err := k.Query("test_predicate")
	if err == nil {
		t.Error("Expected error querying uninitialized kernel, got nil")
	}
}

func TestQueryAll_ProgramInfoNil(t *testing.T) {
	// Initialize the kernel but leave programInfo nil
	store := factstore.NewSimpleInMemoryStore()
	k := &RealKernel{
		store:       store,
		programInfo: nil,
		initialized: true,
	}
	results, err := k.QueryAll()
	if err != nil {
		t.Fatalf("QueryAll failed with err: %v", err)
	}
	if len(results) != 0 {
		t.Errorf("Expected empty results from nil programInfo, got %d", len(results))
	}
}

func TestParseFactString_Empty(t *testing.T) {
	_, err := ParseFactString("")
	if err == nil {
		t.Error("Expected error parsing empty fact string, got nil")
	}

	_, err = ParseFactString(".")
	if err == nil {
		t.Error("Expected error parsing just a period, got nil")
	}
}

func TestLoadFactsFromFile_Empty(t *testing.T) {
	t.Parallel()
	k := setupMockKernel(t)
	// Create empty file
	file, err := os.CreateTemp("", "empty-*.mg")
	if err != nil {
		t.Fatal(err)
	}
	defer os.Remove(file.Name())

	// Write nothing to the file, size 0
	err = k.LoadFactsFromFile(file.Name())
	if err != nil {
		t.Errorf("Expected nil error for empty file, got: %v", err)
	}
}

func TestQuery_EmptyPredicate(t *testing.T) {
	t.Parallel()
	k := setupMockKernel(t)

	_, err := k.Query("")
	if err == nil {
		t.Error("Expected error querying empty predicate, got nil")
	}
}

// ----------------------------------------------------------------------
// Remediation Tests: Type Coercion boundaries
// ----------------------------------------------------------------------

func TestBaseTermToValue_Fallback(t *testing.T) {
	// A new AST node or boolean type might not be covered by baseTermToValue
	// We simulate this by passing something unhandled.
	val := baseTermToValue(ast.Constant{Type: 999, Symbol: "unknown_type"})
	if val != "unknown_type" {
		t.Errorf("Expected fallback to return 'unknown_type', got %v", val)
	}
}

func TestQueryPatternPreservesMangleLiteralIdentity(t *testing.T) {
	nameAtom, err := parseAtom(`bar(/alice)`)
	if err != nil {
		t.Fatal(err)
	}
	stringAtom, err := parseAtom(`bar("/alice")`)
	if err != nil {
		t.Fatal(err)
	}
	if atomMatchesQueryPattern(nameAtom, stringAtom) {
		t.Fatal("a Mangle name atom matched a quoted string query")
	}
	if atomMatchesQueryPattern(stringAtom, nameAtom) {
		t.Fatal("a quoted string matched a Mangle name query")
	}
	if !atomMatchesQueryPattern(nameAtom, nameAtom) || !atomMatchesQueryPattern(stringAtom, stringAtom) {
		t.Fatal("typed query pattern failed to match an identical Mangle constant")
	}
}

func TestQuery_NumericPrecision(t *testing.T) {
	t.Parallel()
	k := setupMockKernel(t)
	k.AppendPolicy("Decl num_test(Float64).")
	k.AppendPolicy(`num_test(3.14).`)
	k.Evaluate()

	results, err := k.Query("num_test")
	if err != nil {
		t.Fatalf("Query failed: %v", err)
	}
	if len(results) != 1 {
		t.Fatalf("Expected 1 result, got %d", len(results))
	}
	val := results[0].Args[0]
	if _, ok := val.(float64); !ok && fmt.Sprintf("%T", val) != "func() (float64, error)" {
		t.Errorf("Expected float64, got %T: %v", val, val)
	}
}

// ----------------------------------------------------------------------
// Remediation Tests: TOCTOU boundaries
// ----------------------------------------------------------------------

func TestLoadFactsFromFile_TOCTOU(t *testing.T) {
	t.Parallel()
	k := setupMockKernel(t)

	// Create a temporary directory
	dir, err := os.MkdirTemp("", "toctou")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(dir)

	// The file path we will try to read
	filePath := dir + "/missing.mg"

	// Call LoadFactsFromFile. The file does not exist, so it should return an error.
	err = k.LoadFactsFromFile(filePath)
	if err == nil {
		t.Error("Expected error when reading non-existent file, got nil")
	}

	// Create it briefly and remove it before reading
	file, err := os.Create(filePath)
	if err == nil {
		file.Close()
		os.Remove(filePath)
		err = k.LoadFactsFromFile(filePath)
		if err == nil {
			t.Error("Expected error when reading deleted file, got nil")
		}
	}
}

func TestQueryBareVsPatternParity(t *testing.T) {
	t.Parallel()
	k := setupMockKernel(t)
	k.AppendPolicy(`
	Decl parity_item(Name, Value).
	Decl parity_single(Name).
	Decl parity_multi(Name).
	Decl parity_multi(Name, Other).
	`)
	if err := k.Evaluate(); err != nil {
		t.Fatalf("Evaluate failed: %v", err)
	}
	k.Assert(Fact{Predicate: "parity_item", Args: []any{"a", "1"}})
	k.Assert(Fact{Predicate: "parity_item", Args: []any{"b", "2"}})
	k.Assert(Fact{Predicate: "parity_single", Args: []any{"solo"}})
	k.Assert(Fact{Predicate: "parity_multi", Args: []any{"only-one"}})
	k.Assert(Fact{Predicate: "parity_multi", Args: []any{"x", "y"}})
	if err := k.Evaluate(); err != nil {
		t.Fatalf("Evaluate failed: %v", err)
	}
	factSet := func(facts []Fact) map[string]int {
		out := make(map[string]int, len(facts))
		for _, f := range facts {
			out[fmt.Sprintf("%s/%v", f.Predicate, f.Args)]++
		}
		return out
	}
	equalSets := func(a, b []Fact) bool {
		sa, sb := factSet(a), factSet(b)
		if len(sa) != len(sb) {
			return false
		}
		for key, n := range sa {
			if sb[key] != n {
				return false
			}
		}
		return true
	}
	bareItem, err := k.Query("parity_item")
	if err != nil {
		t.Fatalf("bare parity_item query failed: %v", err)
	}
	patternedItem, err := k.Query("parity_item(X, Y)")
	if err != nil {
		t.Fatalf("patterned parity_item query failed: %v", err)
	}
	if !equalSets(bareItem, patternedItem) {
		t.Fatalf("bare vs patterned mismatch for parity_item: bare=%v patterned=%v", bareItem, patternedItem)
	}
	if len(bareItem) != 2 {
		t.Fatalf("expected 2 parity_item facts, got %d", len(bareItem))
	}
	bareSingle, err := k.Query("parity_single")
	if err != nil {
		t.Fatalf("bare parity_single query failed: %v", err)
	}
	patternedSingle, err := k.Query("parity_single(X)")
	if err != nil {
		t.Fatalf("patterned parity_single query failed: %v", err)
	}
	if !equalSets(bareSingle, patternedSingle) {
		t.Fatalf("bare vs patterned mismatch for parity_single: bare=%v patterned=%v", bareSingle, patternedSingle)
	}
	bareMulti, err := k.Query("parity_multi")
	if err != nil {
		t.Fatalf("bare parity_multi query failed: %v", err)
	}
	arityOne, err := k.Query("parity_multi(X)")
	if err != nil {
		t.Fatalf("patterned parity_multi/1 query failed: %v", err)
	}
	arityTwo, err := k.Query("parity_multi(X, Y)")
	if err != nil {
		t.Fatalf("patterned parity_multi/2 query failed: %v", err)
	}
	union := append(append([]Fact{}, arityOne...), arityTwo...)
	if !equalSets(bareMulti, union) {
		t.Fatalf("bare multi-arity must equal union of arities: bare=%v union=%v", bareMulti, union)
	}
	if len(bareMulti) != 2 {
		t.Fatalf("expected 2 parity_multi facts across arities, got %d", len(bareMulti))
	}
	unknownBare, err := k.Query("parity_missing")
	if err != nil {
		t.Fatalf("unknown bare query failed: %v", err)
	}
	unknownPatterned, err := k.Query("parity_missing(X)")
	if err != nil {
		t.Fatalf("unknown patterned query failed: %v", err)
	}
	if len(unknownBare) != 0 || len(unknownPatterned) != 0 {
		t.Fatalf("unknown predicate must return empty in both paths: bare=%d patterned=%d", len(unknownBare), len(unknownPatterned))
	}
	var callbackBare []Fact
	if err := k.QueryCallback("parity_item", func(f Fact) error {
		callbackBare = append(callbackBare, f)
		return nil
	}); err != nil {
		t.Fatalf("QueryCallback bare failed: %v", err)
	}
	if !equalSets(bareItem, callbackBare) {
		t.Fatalf("Query vs QueryCallback bare mismatch: query=%v callback=%v", bareItem, callbackBare)
	}
	k.Assert(Fact{Predicate: "parity_item", Args: []any{"c", "3"}})
	if err := k.Evaluate(); err != nil {
		t.Fatalf("Evaluate after dirty assert failed: %v", err)
	}
	rebare, err := k.Query("parity_item")
	if err != nil {
		t.Fatalf("bare query after rebuild failed: %v", err)
	}
	repatterned, err := k.Query("parity_item(X, Y)")
	if err != nil {
		t.Fatalf("patterned query after rebuild failed: %v", err)
	}
	if !equalSets(rebare, repatterned) {
		t.Fatalf("bare vs patterned mismatch after rebuild: bare=%v patterned=%v", rebare, repatterned)
	}
	if len(rebare) != 3 {
		t.Fatalf("expected 3 parity_item facts after rebuild, got %d", len(rebare))
	}
}

func TestDeclSymsForBareQuery_MatchesFullScan(t *testing.T) {
	t.Parallel()
	k := setupMockKernel(t)
	k.AppendPolicy(`
	Decl idx_item(Name).
	Decl idx_item(Name, Value).
	Decl idx_other(Name).
	`)
	if err := k.Evaluate(); err != nil {
		t.Fatalf("Evaluate failed: %v", err)
	}
	k.Assert(Fact{Predicate: "idx_item", Args: []any{"solo"}})
	k.Assert(Fact{Predicate: "idx_item", Args: []any{"a", "1"}})
	k.Assert(Fact{Predicate: "idx_item", Args: []any{"b", "2"}})
	k.Assert(Fact{Predicate: "idx_other", Args: []any{"other"}})
	if err := k.Evaluate(); err != nil {
		t.Fatalf("Evaluate failed: %v", err)
	}
	info := k.programInfo
	if info == nil {
		t.Fatal("programInfo is nil after Evaluate")
	}
	// Full Decls scan is the behavior the index must preserve.
	scanSyms := func(symbol string) map[ast.PredicateSym]bool {
		out := make(map[ast.PredicateSym]bool)
		for pred := range info.Decls {
			if pred.Symbol == symbol {
				out[pred] = true
			}
		}
		return out
	}
	for _, symbol := range []string{"idx_item", "idx_other"} {
		expected := scanSyms(symbol)
		if len(expected) == 0 {
			t.Fatalf("scan found no Decls for %q", symbol)
		}
		indexed, found := declSymsForBareQuery(info, symbol)
		if !found {
			t.Fatalf("indexed lookup missed symbol %q", symbol)
		}
		if len(indexed) != len(expected) {
			t.Fatalf("symbol %q: indexed %d decls, scan found %d", symbol, len(indexed), len(expected))
		}
		for _, pred := range indexed {
			if !expected[pred] {
				t.Fatalf("symbol %q: indexed decl %s/%d not in scan set", symbol, pred.Symbol, pred.Arity)
			}
		}
		// Second call exercises the pointer-keyed cache entry; set must match.
		cached, found := declSymsForBareQuery(info, symbol)
		if !found || len(cached) != len(expected) {
			t.Fatalf("symbol %q: cached lookup mismatch", symbol)
		}
		for _, pred := range cached {
			if !expected[pred] {
				t.Fatalf("symbol %q: cached decl %s/%d not in scan set", symbol, pred.Symbol, pred.Arity)
			}
		}
	}
	if _, found := declSymsForBareQuery(info, "idx_missing"); found {
		t.Fatal("indexed lookup must miss unknown symbol idx_missing")
	}
	if _, found := declSymsForBareQuery(nil, "idx_item"); found {
		t.Fatal("nil programInfo must miss")
	}
	// End-to-end: bare Query through the index returns the same facts the
	// scanned arities hold (multi-arity symbol unions both arities).
	bare, err := k.Query("idx_item")
	if err != nil {
		t.Fatalf("bare idx_item query failed: %v", err)
	}
	if len(bare) != 3 {
		t.Fatalf("expected 3 idx_item facts via indexed bare query, got %d: %v", len(bare), bare)
	}
	one, err := k.Query("idx_item(X)")
	if err != nil {
		t.Fatalf("patterned idx_item/1 query failed: %v", err)
	}
	two, err := k.Query("idx_item(X, Y)")
	if err != nil {
		t.Fatalf("patterned idx_item/2 query failed: %v", err)
	}
	if len(one)+len(two) != len(bare) {
		t.Fatalf("indexed bare count %d must equal arity union %d+%d", len(bare), len(one), len(two))
	}
}

func BenchmarkBareQueryIndexed(b *testing.B) {
	k, err := NewRealKernel()
	if err != nil {
		b.Fatalf("NewRealKernel failed: %v", err)
	}
	k.AppendPolicy(`
	Decl bench_item(Name).
	Decl bench_item(Name, Value).
	Decl bench_other(Name).
	`)
	if err := k.Evaluate(); err != nil {
		b.Fatalf("Evaluate failed: %v", err)
	}
	k.Assert(Fact{Predicate: "bench_item", Args: []any{"solo"}})
	k.Assert(Fact{Predicate: "bench_item", Args: []any{"a", "1"}})
	if err := k.Evaluate(); err != nil {
		b.Fatalf("Evaluate failed: %v", err)
	}
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		if _, err := k.Query("bench_item"); err != nil {
			b.Fatalf("bare bench_item query failed: %v", err)
		}
	}
}

func TestQueryContextBareUsesDeclIndex(t *testing.T) {
	k, err := NewRealKernel()
	if err != nil {
		t.Fatalf("NewRealKernel failed: %v", err)
	}
	if k == nil || k.programInfo == nil {
		t.Skip("kernel has no programInfo; nothing to index")
	}
	k.sandbox = false
	bareDeclIndexCache.Delete(weak.Make(k.programInfo))
	ctx := context.Background()
	const missing = "definitely_missing_bare_index_probe_xyz"
	first, err := k.QueryContext(ctx, missing)
	if err != nil {
		t.Fatalf("bare QueryContext for missing predicate failed: %v", err)
	}
	if len(first) != 0 {
		t.Fatalf("missing predicate %q returned %d facts, want 0", missing, len(first))
	}
	if _, ok := bareDeclIndexCache.Load(weak.Make(k.programInfo)); !ok {
		t.Fatalf("bare QueryContext did not populate decl index cache; indexed fast path bypassed")
	}
	second, err := k.QueryContext(ctx, missing)
	if err != nil {
		t.Fatalf("second bare QueryContext failed: %v", err)
	}
	if len(second) != 0 {
		t.Fatalf("second query for %q returned %d facts, want 0", missing, len(second))
	}
	if _, ok := bareDeclIndexCache.Load(weak.Make(k.programInfo)); !ok {
		t.Fatalf("decl index cache missing after second bare query")
	}
	var existing string
	for pred := range k.programInfo.Decls {
		existing = pred.Symbol
		break
	}
	if existing != "" && existing != missing {
		res, err := k.QueryContext(ctx, existing)
		if err != nil {
			t.Fatalf("bare QueryContext for existing predicate %q failed: %v", existing, err)
		}
		_ = res
		if _, ok := bareDeclIndexCache.Load(weak.Make(k.programInfo)); !ok {
			t.Fatalf("decl index cache missing after existing-predicate bare query")
		}
	}
}

func TestQueryContextBareSandboxBypassesIndex(t *testing.T) {
	k, err := NewRealKernel()
	if err != nil {
		t.Fatalf("NewRealKernel failed: %v", err)
	}
	if k == nil || k.programInfo == nil {
		t.Skip("kernel has no programInfo; nothing to index")
	}
	k.sandbox = true
	bareDeclIndexCache.Delete(weak.Make(k.programInfo))
	const missing = "definitely_missing_bare_index_probe_xyz"
	res, err := k.QueryContext(context.Background(), missing)
	if err != nil {
		t.Fatalf("sandbox bare QueryContext failed: %v", err)
	}
	if len(res) != 0 {
		t.Fatalf("sandbox missing predicate returned %d facts, want 0", len(res))
	}
	if _, ok := bareDeclIndexCache.Load(weak.Make(k.programInfo)); ok {
		t.Fatalf("sandbox bare QueryContext populated decl index cache; must bypass cache for throwaway programs")
	}
}

// The bare-query index does not outlive its program: a rebuilt or reloaded
// program's entry is removed when the program is collected.
func TestDeclSymsForBareQuery_IndexIsEvictedWithItsProgram(t *testing.T) {
	key := func() weak.Pointer[analysis.ProgramInfo] {
		info := &analysis.ProgramInfo{Decls: map[ast.PredicateSym]*ast.Decl{
			{Symbol: "evicted_with_its_program", Arity: 1}: {},
		}}
		if _, ok := declSymsForBareQuery(info, "evicted_with_its_program"); !ok {
			t.Fatal("the declared predicate was not indexed")
		}
		k := weak.Make(info)
		if _, ok := bareDeclIndexCache.Load(k); !ok {
			t.Fatal("the index was not cached")
		}
		return k
	}()
	for range 50 {
		runtime.GC()
		if _, ok := bareDeclIndexCache.Load(key); !ok {
			return
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Fatal("the index outlived its program")
}
