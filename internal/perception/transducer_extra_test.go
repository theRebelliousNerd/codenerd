package perception

import (
	"testing"

	"codenerd/internal/config"
	"codenerd/internal/logging"
)

func TestInitPerceptionLayer(t *testing.T) {
	// Should do nothing / not panic when called with nil or mock
	err := InitPerceptionLayer(nil, &config.UserConfig{})
	if err != nil {
		t.Errorf("Expected error when InitPerceptionLayer is called with nil kernel")
	}

	// ClosePerceptionLayer takes no args
	ClosePerceptionLayer()
}

func TestSeedFallbackSemanticFacts(t *testing.T) {
	seedFallbackSemanticFacts("", nil)
}

// seedFallbackSemanticFacts injects low-confidence semantic_match facts from regex candidates.
// NOTE: production must NOT call this before ClassifyInput — the engine Clear()
// inside wipes pre-seeded facts before scoring. Fallback seeding now happens
// inside ClassifyInputWithMatches, after Clear. Lives in the test file because
// transducer_extra_test.go is its only caller.
// when the SemanticClassifier fails or returns no matches. This ensures the Mangle inference
// rules always have some semantic signal to work with, even in degraded mode.
func seedFallbackSemanticFacts(input string, candidates []VerbEntry) {
	if SharedTaxonomy == nil || SharedTaxonomy.engine == nil {
		return
	}

	// Seed low-confidence semantic_match facts for top regex candidates
	for rank, cand := range candidates {
		if rank >= 5 {
			break // Only top 5 candidates
		}
		// Assert semantic_match(UserInput, CanonicalSentence, Verb, Target, Rank, Similarity)
		err := SharedTaxonomy.engine.AddFact("semantic_match",
			input,     // UserInput
			"",        // CanonicalSentence (empty for fallback)
			cand.Verb, // Verb
			"",        // Target (empty for fallback)
			rank+1,    // Rank (1-indexed)
			int64(50), // Similarity: int64, since the /number Decl rejects float64
		)
		if err != nil {
			logging.PerceptionDebug("Failed to seed fallback semantic_match for %s: %v", cand.Verb, err)
		}
	}

	if len(candidates) > 0 {
		logging.PerceptionDebug("Seeded %d fallback semantic_match facts", min(len(candidates), 5))
	}
}
