package mangle

import "testing"

// TestGapSurveyUntestedBranches surveys untested branches in the Mangle kernel.
// Step 1 of coverage work: pin known defaults and enumerate probe targets for
// step 2 table-driven regression tests. This test passes; step 2 adds
// fail-before-fix cases per target.
func TestGapSurveyUntestedBranches(t *testing.T) {
	cfg := DefaultConfig()
	cases := []struct {
		name  string
		check func(t *testing.T)
	}{
		{"fact limit positive", func(t *testing.T) {
			t.Helper()
			if cfg.FactLimit <= 0 {
				t.Fatalf("FactLimit = %d, want > 0", cfg.FactLimit)
			}
		}},
		{"derived facts gas limit positive", func(t *testing.T) {
			t.Helper()
			if cfg.DerivedFactsLimit <= 0 {
				t.Fatalf("DerivedFactsLimit = %d, want > 0", cfg.DerivedFactsLimit)
			}
		}},
		{"query timeout positive", func(t *testing.T) {
			t.Helper()
			if cfg.QueryTimeout <= 0 {
				t.Fatalf("QueryTimeout = %d, want > 0", cfg.QueryTimeout)
			}
		}},
		{"gas sentinel defined", func(t *testing.T) {
			t.Helper()
			if ErrDerivedFactsLimitExceeded == nil {
				t.Fatal("ErrDerivedFactsLimitExceeded is nil")
			}
		}},
		{"no-schema sentinel defined", func(t *testing.T) {
			t.Helper()
			if errNoSchemas == nil {
				t.Fatal("errNoSchemas is nil")
			}
		}},
	}
	for _, tc := range cases {
		t.Run(tc.name, tc.check)
	}
}

// Gap targets for step 2 (untested branches/behaviors to cover with
// fail-before-fix table tests):
//   - evalWithGasLimit gas accounting and limit-exceeded path
//   - convertValueToTypedTerm type coercions (int64/float/bool/string/nil)
//   - parseQueryShape / isIdentifier query-shape edge cases
//   - factToAtomLocked predicate validation and arg encoding
//   - insertFactLocked fact-limit warn-once path (maybeWarnFactLimit)
//   - removeFactsLocked / removePredicateLocked / replaceFactsForFileImpl
//   - ReplaceControlFacts control-fact replacement
//   - Query / QueryFacts / GetFactsSeq / PushFact result shaping
//   - canonicalPath normalization
//   - WarmFromPersistence / rebuildProgramLocked fail-closed paths
//   - EvaluateRule stub behavior
