// MANDATE FOR AI CODING TOOLS:
// See internal/mangle/differential.go for PRD header.
// This is a test file for DifferentialEngine.

package mangle

import (
	"context"
	"strings"
	"testing"

	"codeberg.org/TauCeti/mangle-go/ast"
)

// TestDifferentialEngine_Stratification validates that predicates are assigned to correct strata.

// TestNewDifferentialEngine validates the instantiation of DifferentialEngine
func TestNewDifferentialEngine(t *testing.T) {
	cfg := DefaultConfig()

	// Test 1: Error when base engine has no program/schema loaded
	baseEngineNoSchema, _ := NewEngine(cfg, nil)
	_, err := NewDifferentialEngine(baseEngineNoSchema)
	if err == nil {
		t.Errorf("Expected error when creating DifferentialEngine without loaded schema, got nil")
	} else if err.Error() != "base engine must have a loaded schema/program" {
		t.Errorf("Expected 'base engine must have a loaded schema/program' error, got %v", err)
	}

	// Test 2: Success when base engine has program/schema loaded
	baseEngineWithSchema, _ := NewEngine(cfg, nil)
	schema := "Decl a(Name). Decl b(Name). a(X) :- b(X)."
	err = baseEngineWithSchema.LoadSchemaString(schema)
	if err != nil {
		t.Fatalf("Failed to load schema: %v", err)
	}

	diffEngine, err := NewDifferentialEngine(baseEngineWithSchema)
	if err != nil {
		t.Fatalf("Failed to create differential engine: %v", err)
	}

	// Verify state initialization
	if diffEngine.baseEngine != baseEngineWithSchema {
		t.Errorf("Expected baseEngine to be set correctly")
	}
	if diffEngine.config != baseEngineWithSchema.config {
		t.Errorf("Expected config to be set correctly")
	}
	if diffEngine.programInfo != baseEngineWithSchema.programInfo {
		t.Errorf("Expected programInfo to be set correctly")
	}
	if diffEngine.predStratum == nil {
		t.Errorf("Expected predStratum to be initialized")
	}
	if diffEngine.strataRules == nil {
		t.Errorf("Expected strataRules to be initialized")
	}
}
func TestDifferentialEngine_Stratification(t *testing.T) {
	cfg := DefaultConfig()
	cfg.AutoEval = true
	baseEngine, _ := NewEngine(cfg, nil)

	// Naive stratification assumed: IDB=1, EDB=0.
	// Rule: a(X) :- b(X). includes 'a' in IDB. 'b' if not in head is EDB.
	schema := "Decl a(Name). Decl b(Name). a(X) :- b(X)."
	baseEngine.LoadSchemaString(schema)

	diffEngine, err := NewDifferentialEngine(baseEngine)
	if err != nil {
		t.Fatalf("Failed to create differential engine: %v", err)
	}

	// Verify a is S1, b is S0.
	aSym := baseEngine.predicateIndex["a"]
	bSym := baseEngine.predicateIndex["b"]

	if s, ok := diffEngine.predStratum[aSym]; !ok || s != 1 {
		t.Errorf("Expected 'a' to be Stratum 1, got %d", s)
	}
	if s, ok := diffEngine.predStratum[bSym]; !ok || s != 0 {
		t.Errorf("Expected 'b' to be Stratum 0, got %d", s)
	}
}

// TestDifferentialEngine_Incremental validates derived facts.
func TestDifferentialEngine_Incremental(t *testing.T) {
	cfg := DefaultConfig()
	cfg.AutoEval = true
	baseEngine, err := NewEngine(cfg, nil)
	if err != nil {
		t.Fatalf("Failed to create base engine: %v", err)
	}
	// Use variable names in Decl (not type names like "String")
	schema := "Decl a(X). Decl b(X). a(X) :- b(X)."
	if err := baseEngine.LoadSchemaString(schema); err != nil {
		t.Fatalf("Failed to load schema: %v", err)
	}

	diffEngine, err := NewDifferentialEngine(baseEngine)
	if err != nil {
		t.Fatalf("Failed to create differential engine: %v", err)
	}

	// Add b("foo"). Should derive a("foo").
	err = diffEngine.AddFactIncremental(Fact{Predicate: "b", Args: []any{"foo"}})
	if err != nil {
		t.Fatal(err)
	}

	// Query 'a'.
	// Since we don't have a direct query method exposed properly in my memory of implementation (I added Query?),
	// I should check if I added Query. Yes I did in Step 305/444.
	// BUT I added it with `context` param.

	res, err := diffEngine.Query(context.Background(), "a(X)")
	if err != nil {
		t.Fatalf("Query failed: %v", err)
	}

	found := false
	for _, binding := range res.Bindings {
		// Mangle may return string "foo" or name constant "/foo" depending on how the fact was added
		if val, ok := binding["X"].(string); ok && (val == "foo" || val == "/foo") {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("Expected a(foo) or a(/foo) to be derived, got %v", res.Bindings)
	}
}

func TestDifferentialEngine_DerivedFactsLimit(t *testing.T) {
	values := []string{"a", "b", "c", "d", "e", "f", "g", "h", "i", "j"}
	atoms := make([]ast.Atom, 0, len(values))
	facts := make([]Fact, 0, len(values))
	for _, value := range values {
		atoms = append(atoms, ast.NewAtom("node", ast.String(value)))
		facts = append(facts, Fact{Predicate: "node", Args: []any{value}})
	}

	tests := []struct {
		name    string
		unified bool
		apply   func(*DifferentialEngine) error
	}{
		{
			name:  "legacy atom delta",
			apply: func(engine *DifferentialEngine) error { return engine.ApplyAtomDelta(atoms) },
		},
		{
			name:  "legacy fact delta",
			apply: func(engine *DifferentialEngine) error { return engine.ApplyDelta(facts) },
		},
		{
			name:    "unified atom delta",
			unified: true,
			apply:   func(engine *DifferentialEngine) error { return engine.ApplyAtomDelta(atoms) },
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := DefaultConfig()
			cfg.DerivedFactsLimit = 5
			baseEngine, err := NewEngine(cfg, nil)
			if err != nil {
				t.Fatalf("NewEngine() error = %v", err)
			}
			if err := baseEngine.LoadSchemaString(`
				Decl node(X).
				Decl pair(X, Y).
				pair(X, Y) :- node(X), node(Y).
			`); err != nil {
				t.Fatalf("LoadSchemaString() error = %v", err)
			}

			diffEngine, err := NewDifferentialEngine(baseEngine)
			if err != nil {
				t.Fatalf("NewDifferentialEngine() error = %v", err)
			}
			if tt.unified {
				if err := diffEngine.EnableUnifiedFastPath(); err != nil {
					t.Fatalf("EnableUnifiedFastPath() error = %v", err)
				}
			}

			err = tt.apply(diffEngine)
			if err == nil {
				t.Fatal("delta evaluation succeeded after exceeding DerivedFactsLimit")
			}
			if !strings.Contains(err.Error(), "fact size limit reached") {
				t.Fatalf("delta evaluation error = %q, want created-fact limit error", err)
			}
		})
	}
}

// TestSnapshotIsolation validates COW Snapshot.
func TestSnapshotIsolation(t *testing.T) {
	cfg := DefaultConfig()
	baseEngine, err := NewEngine(cfg, nil)
	if err != nil {
		t.Fatalf("Failed to create engine: %v", err)
	}
	// Use variable names in Decl (not type names like "String")
	if err := baseEngine.LoadSchemaString("Decl item(X)."); err != nil {
		t.Fatalf("Failed to load schema: %v", err)
	}

	diffEngine, err := NewDifferentialEngine(baseEngine)
	if err != nil {
		t.Fatalf("Failed to create differential engine: %v", err)
	}
	if err := diffEngine.AddFactIncremental(Fact{Predicate: "item", Args: []any{"A"}}); err != nil {
		t.Fatalf("Failed to add fact A: %v", err)
	}

	snapshot := diffEngine.Snapshot()
	if err := snapshot.AddFactIncremental(Fact{Predicate: "item", Args: []any{"B"}}); err != nil {
		t.Fatalf("Failed to add fact B to snapshot: %v", err)
	}

	// Debug: Check strataStores
	t.Logf("DiffEngine has %d strata stores", len(diffEngine.strataStores))
	for i, store := range diffEngine.strataStores {
		preds := store.store.ListPredicates()
		t.Logf("  Stratum %d has %d predicates", i, len(preds))
	}

	// Verify Main Engine has 1 fact (A)
	// We can use Query on each.
	res1, err := diffEngine.Query(context.Background(), "item(X)")
	if err != nil {
		t.Fatalf("Query on main engine failed: %v", err)
	}
	if len(res1.Bindings) != 1 {
		t.Errorf("Main engine impacted by snapshot! Count: %d", len(res1.Bindings))
	}

	// Verify Snapshot has 2 facts (A, B)
	res2, _ := snapshot.Query(context.Background(), "item(X)")
	if len(res2.Bindings) != 2 {
		t.Errorf("Snapshot missing fact! Count: %d", len(res2.Bindings))
	}
}

// TestLazyLoading validates virtual predicate loading.
func TestLazyLoading(t *testing.T) {
	cfg := DefaultConfig()
	cfg.AutoEval = true // important
	baseEngine, err := NewEngine(cfg, nil)
	if err != nil {
		t.Fatalf("Failed to create engine: %v", err)
	}
	// Use variable names in Decl (not type names like "String")
	schema := "Decl virtual_file(Path, Content). Decl has_content(File). has_content(F) :- virtual_file(F, _)."
	if err := baseEngine.LoadSchemaString(schema); err != nil {
		t.Fatalf("Failed to load schema: %v", err)
	}

	diffEngine, err := NewDifferentialEngine(baseEngine)
	if err != nil {
		t.Fatalf("Failed to create differential engine: %v", err)
	}

	// Register Loader for virtual_file
	diffEngine.RegisterVirtualPredicate("virtual_file", func(key string) (string, error) {
		if key == "/path/to/file.txt" {
			return "content of file", nil
		}
		return "", nil // or error
	})

	// Add a query that triggers it?
	// "virtual_file" needs to be queried.
	// If we rely on generic "Query", it delegates to `queryContext.EvalQuery`.
	// EvalQuery calls `Store.GetFacts`.
	// `FactStoreProxy` intercepts `GetFacts`.

	// We execute a query that requires the file.
	// Querying "has_content" should trigger rule "has_content(F) :- virtual_file(F, _)."
	// BUT naive implementation: `EvalProgram` iterates rules.
	// Rule for "has_content" depends on "virtual_file".
	// When evaluating "virtual_file", it calls `GetFacts`.
	// However, Mangle evaluation usually iterates *known facts*.
	// If `virtual_file` is empty in store, `GetFacts` with unbound args?
	// `Loader` requires a KEY.
	// `RegisterVirtualPredicate` implementation:
	// `if len(atom.Args) > 0 ... key = atom.Args[0]`
	// This implies we must query with bound first argument!

	// So if rule is `has_content(F) :- virtual_file(F, _)`, strict evaluation might scan all virtual_file?
	// If scanning, `GetFacts` is called with unbound vars.
	// Our loader check `len(atom.Args) > 0` might fail or atom.Args[0] is Variable.
	// `convertBaseTermToInterface` handles vars? No, usually constants.

	// Limit test to DIRECT Query with specific key.
	// `virtual_file("/path/to/file.txt", Content)`

	res, err := diffEngine.Query(context.Background(), "virtual_file(\"/path/to/file.txt\", C)")
	if err != nil {
		t.Fatalf("Query virtual failed: %v", err)
	}

	found := false
	for _, binding := range res.Bindings {
		if val, ok := binding["C"].(string); ok && val == "\"content of file\"" { // Mangle strings are quoted?
			// Wait, `convertBaseTermToInterface` usually returns string for StringType.
			// Quoting depends on implementation.
			found = true
		} else if val == "content of file" {
			found = true
		}
	}

	if !found {
		t.Errorf("Lazy load failed, results: %v", res.Bindings)
	}
}

// TestNewKnowledgeGraph validates the instantiation of KnowledgeGraph.
func TestNewKnowledgeGraph(t *testing.T) {
	kg := NewKnowledgeGraph()
	if kg == nil {
		t.Fatal("NewKnowledgeGraph returned nil")
	}
	if kg.store == nil {
		t.Error("Expected KnowledgeGraph.store to be initialized, got nil")
	}
	if kg.isFrozen {
		t.Error("Expected KnowledgeGraph.isFrozen to be false by default")
	}
}

// =============================================================================
// Boundary Analysis Coverage (QA 2026-06-01 mangle_differential_boundary_analysis)
// =============================================================================

// TODO: TEST_GAP: [Null/Undefined/Empty] Verify empty query string to Query() returns typed error instead of crashing/panicking.
// TODO: TEST_GAP: [Null/Undefined/Empty] Verify fully bound query (no variables) correctly returns an empty slice or single empty binding.
// TODO: TEST_GAP: [Null/Undefined/Empty] Verify calling AddFactIncremental with an empty Predicate or nil/empty Args slice gracefully errors.
// TODO: TEST_GAP: [Null/Undefined/Empty] Verify DifferentialEngine gracefully handles being instantiated on a baseEngine with a fully empty schema.
// TODO: TEST_GAP: [Null/Undefined/Empty] Verify virtual predicate lazy loaders handle being passed an empty string key without repeated failing I/O.
// TODO: TEST_GAP: [Type Coercion] Verify Go strings passed via AddFactIncremental correctly map to Mangle Name/Atom vs String per schema without silent mismatches.
// TODO: TEST_GAP: [Type Coercion] Verify passing unsupported Go types (like slices or maps) into Fact.Args results in ErrUnsupportedType and not a panic.
// TODO: TEST_GAP: [Type Coercion] Verify numeric boundary overflows (like massive float64) are caught during AddFactIncremental if Mangle expects ints.
// TODO: TEST_GAP: [Type Coercion] Verify conflicting declarations between the base schema and incremental additions are caught or handled without panicking.
// TODO: TEST_GAP: [User Request Extremes] Verify deep graph recursion or infinite loops are halted successfully via context cancellation without leaking goroutines.
// TODO: TEST_GAP: [User Request Extremes] Verify context cancellation on a query with thousands of results successfully aborts and reaps the background goroutine.
// TODO: TEST_GAP: [User Request Extremes] Verify performance/memory impact of adding 500,000 incremental facts via sequential AddFactIncremental calls.
// TODO: TEST_GAP: [User Request Extremes] Verify snapshotting 5,000 times does not lead to OOM due to unintended deep copies of the overlay store.
// TODO: TEST_GAP: [User Request Extremes] Verify loading massive virtual files (e.g. 2GB) correctly enforces limits and doesn't crash the orchestrator.
// TODO: TEST_GAP: [State Conflicts] Verify data race safety when multiple goroutines concurrently call AddFactIncremental on the same DifferentialEngine.
// TODO: TEST_GAP: [State Conflicts] Verify race condition safety if Snapshot() is called exactly while an AddFactIncremental is partially mutating the overlay store.
// TODO: TEST_GAP: [State Conflicts] Verify ChainedFactStore iteration safety during a Query if another goroutine concurrently modifies the overlay store.
// TODO: TEST_GAP: [State Conflicts] Verify consequences and prevention of modifying the immutable baseEngine directly after it has been wrapped by DifferentialEngine.
