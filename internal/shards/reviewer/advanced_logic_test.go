package reviewer

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/google/mangle/analysis"
	"github.com/google/mangle/ast"
	_ "github.com/google/mangle/builtin" // Register builtins
	"github.com/google/mangle/engine"
	"github.com/google/mangle/factstore"
	"github.com/google/mangle/parse"
)

// TestAdvancedReviewerCapabilities verifies the Mangle logic for advanced insights.
func TestAdvancedReviewerCapabilities(t *testing.T) {
	// 1. Load Policy (reviewer.mg)
	policyBytes, err := os.ReadFile("../../core/defaults/reviewer.mg")
	if err != nil {
		t.Fatalf("Failed to read reviewer.mg: %v", err)
	}

	// Load all modular schema files dynamically
	// Note: schemas.mg is now an index file. We load all schemas_*.mg files.
	schemaPattern := "../../core/defaults/schemas_*.mg"
	schemaFiles, err := filepath.Glob(schemaPattern)
	if err != nil {
		t.Fatalf("Failed to glob schema files: %v", err)
	}
	if len(schemaFiles) == 0 {
		t.Fatalf("No schema files found matching pattern: %s", schemaPattern)
	}

	var schemaBytes []byte
	for _, sf := range schemaFiles {
		data, err := os.ReadFile(sf)
		if err != nil {
			t.Fatalf("Failed to read %s: %v", sf, err)
		}
		schemaBytes = append(schemaBytes, data...)
		schemaBytes = append(schemaBytes, '\n')
	}

	// 2. Define Mock Data
	mockData := `
		# --- Hero Risk Scenario ---
		churn_rate("hero.go", 10).
		complexity_warning("hero.go", "complexFunc").
		git_history("hero.go", "hash1", "Alice", 1, "msg").
		git_history("hero.go", "hash2", "Alice", 2, "msg").
        # Alice is the only author

		# --- Shotgun Surgery Scenario ---
		git_history("shotgun_a.go", "commit99", "Bob", 1, "fix").
		git_history("shotgun_b.go", "commit99", "Bob", 1, "fix").
		git_history("shotgun_a.go", "commit100", "Bob", 1, "fix").
		git_history("shotgun_b.go", "commit100", "Bob", 1, "fix").
		git_history("shotgun_a.go", "commit101", "Bob", 1, "fix").
		git_history("shotgun_b.go", "commit101", "Bob", 1, "fix").
        # No dependency link between them

        # --- Architecture Leakage ---
        # Core -> UI
        # Need file_topology to satisfy rules, although layer() now uses symbol_graph + config
        # We explicitly use paths that match the DEFAULTS in reviewer.mg
        file_topology("internal/core/logic.go", "hash", /go, 1, /false).
        file_topology("cmd/nerd/main.go", "hash", /go, 1, /false).

        # Simulate string_contains virtual predicate for layer detection
        string_contains("internal/core/logic.go", "internal/").
        string_contains("internal/core/logic.go", "core/").
        string_contains("cmd/nerd/main.go", "cmd/").
        
        symbol_graph("core_id", /function, /public, "internal/core/logic.go", "sig").
        symbol_graph("ui_id", /function, /public, "cmd/nerd/main.go", "sig").
        dependency_link("core_id", "ui_id", "cmd/nerd").

        # --- Zombie Test ---
        file_topology("zombie_test.go", "hash", /go, 1, /true).
        symbol_graph("zombie_test_id", /file, /public, "zombie_test.go", "_").
        # No dependency_link 

        # --- Circular Dependency ---
        # cycle_a.go -> cycle_b.go -> cycle_a.go
        file_topology("cycle_a.go", "hash", /go, 1, /false).
        file_topology("cycle_b.go", "hash", /go, 1, /false).
        
        symbol_graph("id_a", /func, /pub, "cycle_a.go", "_").
        symbol_graph("id_b", /func, /pub, "cycle_b.go", "_").
        
        dependency_link("id_a", "id_b", "import/b").
        dependency_link("id_b", "id_a", "import/a").

        # DEBUG: Check if file_topology is matching /true
        Decl zombie_candidate(F, Val).
        zombie_candidate(F, Val) :- file_topology(F, _, _, _, Val).
	`

	// Combine: Schemas + Policy + Mock Data
	fullProgram := string(schemaBytes) + "\n" + string(policyBytes) + "\n" + mockData

	// 3. Parse
	unit, err := parse.Unit(strings.NewReader(fullProgram))
	if err != nil {
		t.Fatalf("Failed to parse program: %v", err)
	}

	// 4. Analyze
	programInfo, err := analysis.AnalyzeOneUnit(unit, nil)
	if err != nil {
		t.Fatalf("Failed to analyze program: %v", err)
	}

	// 5. Evaluate
	store := factstore.NewSimpleInMemoryStore()
	_, err = engine.EvalProgramWithStats(programInfo, store)
	if err != nil {
		t.Fatalf("Failed to evaluate program: %v", err)
	}

	// 6. Verify Findings
	foundHero := false
	foundShotgun := false
	foundLeakage := false
	foundZombie := false
	foundCycle := false

	// Look up the raw_finding predicate symbol
	var rawFindingPred ast.PredicateSym
	for pred := range programInfo.Decls {
		if pred.Symbol == "raw_finding" {
			rawFindingPred = pred
			break
		}
	}

	store.GetFacts(ast.NewQuery(rawFindingPred), func(a ast.Atom) error {
		s := a.String()
		t.Logf("Found: %s", s)

		if strings.Contains(s, "HERO_RISK") && strings.Contains(s, "hero.go") {
			foundHero = true
		}
		if strings.Contains(s, "SHOTGUN_SURGERY") && strings.Contains(s, "shotgun_a.go") {
			foundShotgun = true
		}
		if strings.Contains(s, "LAYER_LEAKAGE") && strings.Contains(s, "internal/core/logic.go") {
			foundLeakage = true
		}
		if strings.Contains(s, "ZOMBIE_TEST") && strings.Contains(s, "zombie_test.go") {
			foundZombie = true
		}
		if strings.Contains(s, "CIRCULAR_DEPENDENCY") && strings.Contains(s, "cycle_a.go") {
			foundCycle = true
		}
		return nil
	})

	if !foundHero {
		t.Error("Failed to detect HERO_RISK")
	}
	if !foundShotgun {
		t.Error("Failed to detect SHOTGUN_SURGERY")
	}
	if !foundLeakage {
		t.Error("Failed to detect LAYER_LEAKAGE")
	}
	if !foundZombie {
		t.Error("Failed to detect ZOMBIE_TEST")
	}
	if !foundCycle {
		t.Error("Failed to detect CIRCULAR_DEPENDENCY")
	}
}
