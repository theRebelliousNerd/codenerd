package policy

import (
	"context"
	"os"
	"testing"
	"time"

	"codenerd/internal/mangle"

	"go.uber.org/goleak"
)

// TestTDDLogic_Golden enforces strict correctness of TDD Mangle policies.
// It loads scenarios from testdata/, runs them against the engine,
// and compares the resulting IDB (derived facts) with a golden file.
func TestTDDLogic_Golden(t *testing.T) {
	// 1. Setup Leaks Trap (Testudo Pattern)
	defer goleak.VerifyNone(t)

	// Scenarios to test
	scenarios := []struct {
		name        string
		policyFiles []string
		schemaFiles []string
		edbFile     string
		goldenFile  string
	}{
		{
			name:        "TDD Loop Transitions",
			policyFiles: []string{"tdd_logic.mg", "tdd_loop.mg"},
			schemaFiles: []string{
				"../schemas_world.mg",     // diagnostic
				"../schemas_testing.mg",   // test_state
				"../schemas_execution.mg", // next_action
				"../schemas_intent.mg",    // user_intent
			},
			edbFile:    "testdata/tdd_loop.edb",
			goldenFile: "testdata/tdd_loop.golden",
		},
	}

	for _, sc := range scenarios {
		t.Run(sc.name, func(t *testing.T) {
			// 2. Setup Engine
			cfg := mangle.DefaultConfig()
			eng, err := mangle.NewEngine(cfg, nil)
			if err != nil {
				t.Fatalf("Failed to create engine: %v", err)
			}
			defer eng.Close() // Ensure resources are cleaned up

			// 3. Load Schemas
			for _, sf := range sc.schemaFiles {
				schemaContent, err := os.ReadFile(sf)
				if err != nil {
					t.Fatalf("Failed to read schema %s: %v", sf, err)
				}
				if err := eng.LoadSchemaString(string(schemaContent)); err != nil {
					t.Fatalf("Failed to load schema %s: %v", sf, err)
				}
			}

			// 4. Load Logic
			for _, pf := range sc.policyFiles {
				policyContent, err := os.ReadFile(pf)
				if err != nil {
					t.Fatalf("Failed to read policy %s: %v", pf, err)
				}
				if err := eng.LoadSchemaString(string(policyContent)); err != nil {
					t.Fatalf("Failed to load policy %s: %v", pf, err)
				}
			}

			// 5. Load EDB (Facts)
			edbContent, err := os.ReadFile(sc.edbFile)
			if err != nil {
				t.Fatalf("Failed to read EDB %s: %v", sc.edbFile, err)
			}
			loadEDBViaEngine(t, eng, string(edbContent))

			// 6. Verify against Golden File
			// We use a context with timeout to bound the test execution time.
			// While Engine.GetFacts doesn't take a context, this protects the test runner.
			ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
			defer cancel()

			// Liveness check: Ensure engine is responsive
			// We query for a known predicate to ensure the engine is up and running.
			// If this hangs, the test timeout (external to this function) or the context check below handles it?
			// Since GetFacts is blocking and doesn't take context, we can't truly cancel it.
			// However, we assert no error occurred.
			if _, err = eng.GetFacts("block_commit"); err != nil {
				t.Fatalf("Engine failed liveness check (GetFacts): %v", err)
			}

			// Check if we timed out during the setup/liveness
			select {
			case <-ctx.Done():
				t.Fatalf("Test execution timed out: %v", ctx.Err())
			default:
			}

			checkGolden(t, eng, sc.goldenFile)
		})
	}
}
