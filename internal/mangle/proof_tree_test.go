package mangle

import (
	"context"
	"strings"
	"testing"
	"time"
)

func TestProofTreeTracer_TraceQuery(t *testing.T) {
	// 1. Setup Engine
	cfg := DefaultConfig()
	engine, err := NewEngine(cfg, nil)
	if err != nil {
		t.Fatalf("Failed to create engine: %v", err)
	}

	// 2. Define Schema & Rules
	// We use a simplified rule for 'impacted' to match the hardcoded logic in proof_tree.go
	schema := `
	Decl dependency_link(File, Dep, Type) descr [mode("-", "-", "-")].
	Decl impacted(File) descr [mode("-")].
	
	impacted(X) :- dependency_link(X, _, _).
	`
	if err := engine.LoadSchemaString(schema); err != nil {
		t.Fatalf("Failed to load schema: %v", err)
	}

	// 3. Add Facts
	err = engine.AddFact("dependency_link", "main.go", "lib.go", "import")
	if err != nil {
		t.Fatalf("Failed to add fact: %v", err)
	}

	// 4. Trace
	tracer := NewProofTreeTracer(engine)
	tracer.IndexRules() // Populate ruleIndex from ProgramInfo for premise discovery
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	trace, err := tracer.TraceQuery(ctx, "impacted(X)")
	if err != nil {
		t.Fatalf("TraceQuery failed: %v", err)
	}

	// 5. Verify Trace
	if len(trace.RootNodes) != 1 {
		t.Fatalf("Expected 1 root node, got %d", len(trace.RootNodes))
	}

	root := trace.RootNodes[0]
	if root.Fact.Predicate != "impacted" {
		t.Errorf("Expected root predicate 'impacted', got '%s'", root.Fact.Predicate)
	}
	if root.Fact.Args[0] != "main.go" {
		t.Errorf("Expected root arg 'main.go', got '%v'", root.Fact.Args[0])
	}

	// Verify Classification
	if root.Source != SourceIDB {
		t.Errorf("Expected SourceIDB, got %v", root.Source)
	}
	if root.RuleName != "impacted" {
		t.Errorf("Expected rule 'impacted', got '%s'", root.RuleName)
	}

	// Verify Children (Premises)
	// findPremises for 'impacted' looks for body predicates from ProgramInfo rules
	if len(root.Children) != 1 {
		t.Fatalf("Expected 1 child (premise), got %d", len(root.Children))
	}

	child := root.Children[0]
	if child.Fact.Predicate != "dependency_link" {
		t.Errorf("Expected child predicate 'dependency_link', got '%s'", child.Fact.Predicate)
	}
	if child.Source != SourceEDB {
		t.Errorf("Expected SourceEDB for dependency_link, got %v", child.Source)
	}
}

func TestProofTreeTracer_RenderASCII(t *testing.T) {
	// Setup a fake derivation tree manually
	root := &DerivationNode{
		Fact:     Fact{Predicate: "impacted", Args: []interface{}{"main.go"}},
		Source:   SourceIDB,
		RuleName: "impacted",
		Children: []*DerivationNode{
			{
				Fact:   Fact{Predicate: "dependency_link", Args: []interface{}{"main.go", "lib.go", "import"}},
				Source: SourceEDB,
			},
		},
	}

	trace := &DerivationTrace{
		Query:     "impacted(X)",
		RootNodes: []*DerivationNode{root},
		Duration:  10 * time.Millisecond,
	}

	ascii := trace.RenderASCII()

	// Basic string containment checks
	if len(ascii) == 0 {
		t.Error("RenderASCII returned empty string")
	}
	// Note: Strings in Mangle output might be quoted like "main.go"
	// proof_tree.go uses Fact.String() which formats args.
	// Strings starting with / are atoms, others are quoted.

	// impacted("main.go"). [IDB:transitive_impact]
	// └── dependency_link("main.go", "lib.go", "import"). [EDB]

	expectedRoot := `impacted("main.go"). [IDB:impacted]`
	if !containsNormalized(ascii, expectedRoot) {
		t.Errorf("ASCII output missing root node pattern. Got:\n%s", ascii)
	}

	expectedChild := `dependency_link("main.go", "lib.go", "import"). [EDB]`
	if !containsNormalized(ascii, expectedChild) {
		t.Errorf("ASCII output missing child node pattern. Got:\n%s", ascii)
	}
}

func containsNormalized(s, substr string) bool {
	// Simple helper to ignore minor whitespace differences if needed
	return strings.Contains(s, substr)
}
