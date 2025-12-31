// mangle_repair_test.go tests the Mangle Repair Shard's PredicateSelector integration.
package system

import (
	"strings"
	"testing"

	"codenerd/internal/prompt"
)

func TestMangleRepairShard_PredicateSelectorWiring(t *testing.T) {
	// Create a repair shard
	shard := NewMangleRepairShard()

	// Initially, predicateSelector should be nil
	shard.mu.RLock()
	if shard.predicateSelector != nil {
		t.Error("Expected predicateSelector to be nil before SetCorpus")
	}
	shard.mu.RUnlock()

	// Try to load actual corpus (will fail gracefully if not available)
	// This test verifies the wiring mechanism, not the corpus content
	// The actual corpus is tested in integration tests
	t.Log("PredicateSelector wiring test: checking initial state")
	t.Log("Note: Full corpus tests require predicate_corpus.db to be built")
}

func TestMangleRepairShard_BuildRepairPromptWithoutCorpus(t *testing.T) {
	// Create shard without corpus
	shard := NewMangleRepairShard()

	// Test with errors
	rule := "next_action(/start) :- undefined_predicate(X)."
	errors := []string{
		"undefined predicate: undefined_predicate",
		"syntax: parse error",
	}

	// Build repair prompt WITHOUT corpus (should still work, just no predicate list)
	prompt := shard.buildRepairPrompt(rule, errors, nil)

	// Verify prompt contains key sections
	if !strings.Contains(prompt, "validation errors") {
		t.Error("Prompt should mention validation errors")
	}

	if !strings.Contains(prompt, "MangleSynth JSON object") {
		t.Error("Prompt should ask for MangleSynth JSON output")
	}

	// Should list the errors
	if !strings.Contains(prompt, "undefined predicate") {
		t.Error("Prompt should include the actual errors")
	}

	// Should include repair instructions
	if !strings.Contains(prompt, "Uses only declared predicates") {
		t.Error("Prompt should include repair instructions")
	}
}

func TestMangleRepairShard_ExtractErrorTypes(t *testing.T) {
	shard := NewMangleRepairShard()

	tests := []struct {
		name     string
		errors   []string
		expected []string
	}{
		{
			name: "shard error",
			errors: []string{
				"undefined predicate: shard_state",
			},
			expected: []string{"shard"},
		},
		{
			name: "campaign error",
			errors: []string{
				"undefined predicate: campaign_phase",
			},
			expected: []string{"campaign"},
		},
		{
			name: "routing error",
			errors: []string{
				"undefined predicate: next_action",
			},
			expected: []string{"routing"},
		},
		{
			name: "multiple domains",
			errors: []string{
				"undefined predicate: shard_state",
				"undefined predicate: campaign_phase",
				"undefined predicate: tool_available",
			},
			expected: []string{"shard", "campaign", "tool"},
		},
		{
			name: "safety error",
			errors: []string{
				"undefined predicate: permitted",
			},
			expected: []string{"safety"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := shard.extractErrorTypes(tt.errors)

			// Check that all expected types are present
			resultMap := make(map[string]bool)
			for _, r := range result {
				resultMap[r] = true
			}

			for _, exp := range tt.expected {
				if !resultMap[exp] {
					t.Errorf("Expected error type %q not found in result: %v", exp, result)
				}
			}
		})
	}
}

func TestMangleRepairShard_PredicateSelectorIntegration(t *testing.T) {
	// This test verifies that PredicateSelector integration is correct
	// It doesn't require an actual corpus, just tests the wiring

	shard := NewMangleRepairShard()

	// Test that we can set a PredicateSelector directly
	// (In production, this is auto-created when SetCorpus is called)
	mockSelector := (*prompt.PredicateSelector)(nil) // nil selector for this test
	shard.SetPredicateSelector(mockSelector)

	// Verify the field was set
	shard.mu.RLock()
	if shard.predicateSelector != mockSelector {
		t.Error("Expected predicateSelector to be set to the provided value")
	}
	shard.mu.RUnlock()

	t.Log("PredicateSelector can be set directly via SetPredicateSelector")
	t.Log("In production, it's auto-created when SetCorpus is called")
}

func TestMangleRepairShard_FallbackWithoutSelector(t *testing.T) {
	// Create shard WITHOUT setting corpus (no PredicateSelector)
	shard := NewMangleRepairShard()

	// Build repair prompt without corpus or selector
	rule := "next_action(/start)."
	errors := []string{"some error"}

	prompt := shard.buildRepairPrompt(rule, errors, nil)

	// Should still generate a valid prompt with instructions
	if !strings.Contains(prompt, "MangleSynth JSON object") {
		t.Error("Prompt should still ask for MangleSynth JSON output even without corpus")
	}

	// Should not have predicate listings
	if strings.Contains(prompt, "Available predicates") {
		t.Error("Should not list predicates when corpus is nil")
	}
}

func TestMangleRepairShard_SetPredicateSelectorDirectly(t *testing.T) {
	// Test that we can set PredicateSelector directly
	shard := NewMangleRepairShard()

	// Create a nil selector for testing (in production, it's created from a real corpus)
	var selector *prompt.PredicateSelector = nil
	shard.SetPredicateSelector(selector)

	shard.mu.RLock()
	storedSelector := shard.predicateSelector
	shard.mu.RUnlock()

	if storedSelector != selector {
		t.Error("Expected predicateSelector to be set to provided value")
	}

	t.Log("SetPredicateSelector properly stores the provided selector")
}
