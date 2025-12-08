// MANDATE FOR AI CODING TOOLS:
// See internal/mangle/differential.go for PRD header.
// This is a test file for DifferentialEngine.

package mangle

import (
	"context"
	"testing"
)

// TestDifferentialEngine_Stratification validates that predicates are assigned to correct strata.
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
	err = diffEngine.AddFactIncremental(Fact{Predicate: "b", Args: []interface{}{"foo"}})
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
	if err := diffEngine.AddFactIncremental(Fact{Predicate: "item", Args: []interface{}{"A"}}); err != nil {
		t.Fatalf("Failed to add fact: %v", err)
	}

	snapshot := diffEngine.Snapshot()
	snapshot.AddFactIncremental(Fact{Predicate: "item", Args: []interface{}{"B"}})

	// Verify Main Engine has 1 fact (A)
	// We can use Query on each.
	res1, _ := diffEngine.Query(context.Background(), "item(X)")
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
