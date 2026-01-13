package policy

import (
	"fmt"
	"os"
	"strings"
	"testing"

	"codenerd/internal/mangle"

	"github.com/google/mangle/ast"
	"github.com/google/mangle/parse"
	"go.uber.org/goleak"
)

// TestLogic_Golden enforces strict correctness of Mangle policies.
// It loads scenarios from testdata/, runs them against the engine,
// and compares the resulting IDB (derived facts) with a golden file.
func TestLogic_Golden(t *testing.T) {
	scenarios := []struct {
		name        string
		policyFiles []string // Changed to support multiple policy/logic files
		schemaFiles []string // Changed to support multiple schema files
		edbFile     string
		goldenFile  string
	}{
		{
			name:        "Honeypot Detection",
			policyFiles: []string{"browser_honeypot.mg"},
			schemaFiles: []string{"../schemas_browser.mg"},
			edbFile:     "testdata/honeypot.edb",
			goldenFile:  "testdata/honeypot.golden",
		},
		{
			name:        "JIT Logic Context Matching",
			policyFiles: []string{"jit_logic.mg"},
			schemaFiles: []string{"../schemas_prompts.mg"},
			edbFile:     "testdata/jit_logic.edb",
			goldenFile:  "testdata/jit_logic.golden",
		},
		{
			name:        "Campaign Orchestration",
			policyFiles: []string{"campaign_core.mg", "campaign_context.mg", "campaign_planning.mg", "campaign_tasks.mg"},
			schemaFiles: []string{"../schemas_campaign.mg", "../schemas_shards.mg", "../schemas_safety.mg", "../schemas_intent.mg", "../schemas_analysis.mg", "../schemas_execution.mg"},
			edbFile:     "testdata/campaign.edb",
			goldenFile:  "testdata/campaign.golden",
		},
		{
			name:        "CodeDOM Safety",
			policyFiles: []string{"codedom_safety.mg"},
			schemaFiles: []string{"../schemas_codedom.mg", "../schemas_codedom_polyglot.mg"},
			edbFile:     "testdata/codedom_safety.edb",
			goldenFile:  "testdata/codedom_safety.golden",
		},
		{
			name:        "Git Safety / Chesterton's Fence",
			policyFiles: []string{"git_safety.mg"},
			schemaFiles: []string{"../schemas_shards.mg", "../schemas_safety.mg", "../schemas_intent.mg"},
			edbFile:     "testdata/git_safety.edb",
			goldenFile:  "testdata/git_safety.golden",
		},
		{
			name:        "TDD Repair Loop",
			policyFiles: []string{"tdd_loop.mg"},
			schemaFiles: []string{"../schemas_execution.mg", "../schemas_intent.mg"},
			edbFile:     "testdata/tdd_loop.edb",
			goldenFile:  "testdata/tdd_loop.golden",
		},
		{
			name:        "TDD Loop Logic",
			policyFiles: []string{"tdd_logic.mg"},
			schemaFiles: []string{"../schemas_shards.mg", "../schemas_analysis.mg", "../schemas_world.mg"},
			edbFile:     "testdata/tdd_logic.edb",
			goldenFile:  "testdata/tdd_logic.golden",
		},
	}

	for _, sc := range scenarios {
		t.Run(sc.name, func(t *testing.T) {
			// 1. Setup Leaks Trap (Testudo Pattern)
			defer goleak.VerifyNone(t)

			// 2. Setup Engine
			cfg := mangle.DefaultConfig()
			eng, err := mangle.NewEngine(cfg, nil)
			if err != nil {
				t.Fatalf("Failed to create engine: %v", err)
			}
			defer eng.Close() // Ensure resources are cleaned up

			// 2. Load Logic (Schema + Policy)
			for _, sf := range sc.schemaFiles {
				schemaContent, err := os.ReadFile(sf)
				if err != nil {
					t.Fatalf("Failed to read schema %s: %v", sf, err)
				}
				if err := eng.LoadSchemaString(string(schemaContent)); err != nil {
					t.Fatalf("Failed to load schema %s: %v", sf, err)
				}
			}

			for _, pf := range sc.policyFiles {
				policyContent, err := os.ReadFile(pf)
				if err != nil {
					t.Fatalf("Failed to read policy %s: %v", pf, err)
				}
				if err := eng.LoadSchemaString(string(policyContent)); err != nil {
					t.Fatalf("Failed to load policy %s: %v", pf, err)
				}
			}

			// 3. Load EDB (Facts)
			edbContent, err := os.ReadFile(sc.edbFile)
			if err != nil {
				t.Fatalf("Failed to read EDB %s: %v", sc.edbFile, err)
			}
			loadEDBViaEngine(t, eng, string(edbContent))

			// 4. Verify against Golden File
			checkGolden(t, eng, sc.goldenFile)
		})
	}
}

// TestTypeCanary ensures that Atoms and Strings are strictly distinct.
// This prevents the "String-for-Atom" hallucination.
func TestTypeCanary(t *testing.T) {
	defer goleak.VerifyNone(t)

	cfg := mangle.DefaultConfig()
	eng, err := mangle.NewEngine(cfg, nil)
	if err != nil {
		t.Fatalf("NewEngine: %v", err)
	}
	defer eng.Close()

	// Declare schema for user predicate to avoid "not declared" error
	schema := `Decl user(Type).`
	if err := eng.LoadSchemaString(schema); err != nil {
		t.Fatalf("LoadSchemaString: %v", err)
	}

	// Add facts: user(/alice) and user("alice")
	// Note: Engine.AddFact takes interface{} args.
	// Strings starting with "/" are treated as Atoms (names) by the wrapper.
	// Regular strings are strings.

	// Add Atom: /alice
	if err := eng.AddFact("user", "/alice"); err != nil {
		t.Fatalf("AddFact(/alice): %v", err)
	}

	// Add String: "alice bob" (space ensures it's not identifier-promoted)
	if err := eng.AddFact("user", "alice bob"); err != nil {
		t.Fatalf("AddFact(alice bob): %v", err)
	}

	// Now we expect user(/alice) and user("alice bob") to be distinct.
	facts, err := eng.GetFacts("user")
	if err != nil {
		t.Fatalf("GetFacts: %v", err)
	}

	if len(facts) != 2 {
		t.Errorf("Expected 2 facts, got %d: %v", len(facts), facts)
	}

	// Test a non-identifier string to prove they DON'T join when types differ.
	schema2 := `
	Decl q(X).
	Decl r(X).
	Decl p(X).
	p(X) :- q(X), r(X).
	`
	eng3, err := mangle.NewEngine(cfg, nil)
	if err != nil {
		t.Fatalf("NewEngine: %v", err)
	}
	defer eng3.Close()
	if err := eng3.LoadSchemaString(schema2); err != nil {
		t.Fatalf("LoadSchemaString: %v", err)
	}

	if err := eng3.AddFact("q", "/url"); err != nil {
		t.Fatalf("AddFact(q): %v", err)
	}
	if err := eng3.AddFact("r", "http://url"); err != nil {
		t.Fatalf("AddFact(r): %v", err)
	}

	// They should not join.
	facts, err = eng3.GetFacts("p")
	if err != nil {
		// GetFacts might error if predicate not found, which acts as empty set here.
		// But if it's a real error, we should fail.
		// However, GetFacts returns facts for predicate. If predicate p exists (declared), it returns empty list if no facts.
		// If p was not declared, it might error. We declared p in schema2.
		t.Fatalf("GetFacts(p): %v", err)
	}
	if len(facts) != 0 {
		t.Errorf("Type Canary Failed: Joined Atom(/url) with String(\"http://url\") unexpectedly.")
	}
}

// --- Helpers ---

func loadEDBViaEngine(t *testing.T, eng *mangle.Engine, content string) {
	lines := strings.Split(content, "\n")
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}

		// Use Mangle's parser to parse the fact string into an Atom
		// parse.Atom returns an ast.Atom
		parsedAtom, err := parse.Atom(line)
		if err != nil {
			t.Fatalf("Failed to parse fact '%s': %v", line, err)
		}

		// Convert ast.Atom args to interface{} for Engine.AddFact
		// We pass ast.BaseTerm directly to avoid ambiguity in the engine wrapper
		// (e.g. strings starting with "/" being auto-converted to Atoms)
		args := make([]interface{}, len(parsedAtom.Args))
		for i, arg := range parsedAtom.Args {
			args[i] = arg
		}

		if err := eng.AddFact(parsedAtom.Predicate.Symbol, args...); err != nil {
			t.Fatalf("Failed to add fact '%s': %v", line, err)
		}
	}
}

func checkGolden(t *testing.T, eng *mangle.Engine, goldenPath string) {
	expectedContent, err := os.ReadFile(goldenPath)
	if err != nil {
		t.Fatalf("Failed to read golden file %s: %v", goldenPath, err)
	}

	lines := strings.Split(string(expectedContent), "\n")
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}

		expectedAtom, err := parse.Atom(line)
		if err != nil {
			t.Fatalf("Failed to parse golden line '%s': %v", line, err)
		}

		// Fetch facts from engine
		facts, err := eng.GetFacts(expectedAtom.Predicate.Symbol)
		if err != nil {
			// If predicate not found, it means 0 facts, which is a mismatch if we expected some.
			t.Errorf("Missing expected fact (predicate not found): %s", line)
			continue
		}

		found := false
		for _, f := range facts {
			if factMatchesAtom(f, expectedAtom) {
				found = true
				break
			}
		}

		if !found {
			t.Errorf("Missing expected fact: %s", line)
		}
	}
}

// convertTermToInterface converts AST terms back to Go types for feeding into the Engine wrapper.
func convertTermToInterface(term ast.BaseTerm) interface{} {
	switch c := term.(type) {
	case ast.Constant:
		switch c.Type {
		case ast.NameType:
			// Handle existing / prefix to avoid double-slash
			if strings.HasPrefix(c.Symbol, "/") {
				return c.Symbol
			}
			return "/" + c.Symbol
		case ast.StringType:
			return c.Symbol
		case ast.NumberType:
			return c.NumValue
		default:
			return c.Symbol
		}
	default:
		return fmt.Sprintf("%v", term)
	}
}

func factMatchesAtom(f mangle.Fact, a ast.Atom) bool {
	if f.Predicate != a.Predicate.Symbol {
		return false
	}
	if len(f.Args) != len(a.Args) {
		return false
	}
	for i, arg := range f.Args {
		expected := a.Args[i]

		// Convert actual arg to string/int for comparison
		// The Engine wrapper stores args as int64, string, etc.

		switch e := expected.(type) {
		case ast.Constant:
			if e.Type == ast.NameType {
				// Expecting Atom. Engine stores as string.
				// Engine might store as "/name" or "name" depending on how it was inserted/normalized.
				// But typically the wrapper outputs string with / prefix if it's an atom.
				s, ok := arg.(string)
				if !ok {
					// Check MangleAtom alias if it exists in scope, or just fail
					return false
				}
				// Compare. expected.Symbol is "name". arg should be "/name" or "name".
				// Standardize on "/name"
				if s != "/"+e.Symbol && s != e.Symbol {
					return false
				}
			} else if e.Type == ast.StringType {
				s, ok := arg.(string)
				if !ok {
					return false
				}
				if s != e.Symbol {
					return false
				}
			} else if e.Type == ast.NumberType {
				n, ok := arg.(int64)
				if !ok {
					// Try int
					if nInt, ok := arg.(int); ok {
						n = int64(nInt)
					} else {
						return false
					}
				}
				if n != e.NumValue {
					return false
				}
			}
		default:
			// Variables/Wildcards in golden files are not supported by this simple matcher
			return false
		}
	}
	return true
}
