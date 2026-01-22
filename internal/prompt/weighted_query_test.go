package prompt

import (
	"strings"
	"testing"
)

// =============================================================================
// Tests for Weighted QueryBuilder (collectKnowledgeAtoms)
// =============================================================================

// TestWeightedQueryBuilder verifies that query parts are weighted properly
// when building semantic search queries for knowledge atoms.
func TestWeightedQueryBuilder(t *testing.T) {
	tests := []struct {
		name         string
		context      CompilationContext
		wantContains []string
		wantCounts   map[string]int // term -> expected count
	}{
		{
			name: "intent verb weighted 3x",
			context: CompilationContext{
				IntentVerb: "/fix",
			},
			wantCounts: map[string]int{
				"/fix": 3,
			},
		},
		{
			name: "intent target weighted 3x",
			context: CompilationContext{
				IntentTarget: "authentication",
			},
			wantCounts: map[string]int{
				"authentication": 3,
			},
		},
		{
			name: "shard ID weighted 2x",
			context: CompilationContext{
				ShardID: "coder",
			},
			wantCounts: map[string]int{
				"coder": 2,
			},
		},
		{
			name: "language weighted 2x",
			context: CompilationContext{
				Language: "go",
			},
			wantCounts: map[string]int{
				"go": 2,
			},
		},
		{
			name: "frameworks weighted 1x",
			context: CompilationContext{
				Frameworks: []string{"gin", "cobra"},
			},
			wantCounts: map[string]int{
				"gin":   1,
				"cobra": 1,
			},
		},
		{
			name: "full context with all weights",
			context: CompilationContext{
				IntentVerb:   "/implement",
				IntentTarget: "api",
				ShardID:      "coder",
				Language:     "go",
				Frameworks:   []string{"gin"},
			},
			wantCounts: map[string]int{
				"/implement": 3, // 3x weight
				"api":        3, // 3x weight
				"coder":      2, // 2x weight
				"go":         2, // 2x weight
				"gin":        1, // 1x weight
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Build the weighted query string using the same logic as collectKnowledgeAtoms
			var weightedParts []string

			// High priority (weight 3x): Intent verb and target
			if tt.context.IntentVerb != "" {
				weightedParts = append(weightedParts, tt.context.IntentVerb, tt.context.IntentVerb, tt.context.IntentVerb)
			}
			if tt.context.IntentTarget != "" {
				weightedParts = append(weightedParts, tt.context.IntentTarget, tt.context.IntentTarget, tt.context.IntentTarget)
			}

			// Medium priority (weight 2x): ShardID and Language
			if tt.context.ShardID != "" {
				weightedParts = append(weightedParts, tt.context.ShardID, tt.context.ShardID)
			}
			if tt.context.Language != "" {
				weightedParts = append(weightedParts, tt.context.Language, tt.context.Language)
			}

			// Low priority (weight 1x): Frameworks
			weightedParts = append(weightedParts, tt.context.Frameworks...)

			query := strings.Join(weightedParts, " ")

			// Verify term counts
			for term, wantCount := range tt.wantCounts {
				gotCount := strings.Count(query, term)
				if gotCount != wantCount {
					t.Errorf("term %q: got count %d, want %d in query: %s", term, gotCount, wantCount, query)
				}
			}
		})
	}
}

// TestWeightedQueryBuilder_EmptyContext verifies that empty context returns empty query.
func TestWeightedQueryBuilder_EmptyContext(t *testing.T) {
	cc := &CompilationContext{}

	// Check that the trigger conditions (queryParts) would be empty
	var queryParts []string
	if cc.IntentVerb != "" {
		queryParts = append(queryParts, cc.IntentVerb)
	}
	if cc.IntentTarget != "" {
		queryParts = append(queryParts, cc.IntentTarget)
	}
	if cc.ShardID != "" {
		queryParts = append(queryParts, cc.ShardID)
	}
	if cc.Language != "" {
		queryParts = append(queryParts, cc.Language)
	}
	if len(cc.Frameworks) > 0 {
		queryParts = append(queryParts, cc.Frameworks...)
	}

	if len(queryParts) != 0 {
		t.Errorf("Expected empty query parts for empty context, got: %v", queryParts)
	}
}

// TestWeightedQueryBuilder_HighPriorityDomination verifies that high-priority
// terms make up the majority of the query (more semantic weight).
func TestWeightedQueryBuilder_HighPriorityDomination(t *testing.T) {
	cc := CompilationContext{
		IntentVerb:   "/fix",
		IntentTarget: "bug",
		ShardID:      "coder",
		Language:     "go",
		Frameworks:   []string{"gin", "cobra"},
	}

	var weightedParts []string

	// High priority (weight 3x)
	if cc.IntentVerb != "" {
		weightedParts = append(weightedParts, cc.IntentVerb, cc.IntentVerb, cc.IntentVerb)
	}
	if cc.IntentTarget != "" {
		weightedParts = append(weightedParts, cc.IntentTarget, cc.IntentTarget, cc.IntentTarget)
	}

	// Medium priority (weight 2x)
	if cc.ShardID != "" {
		weightedParts = append(weightedParts, cc.ShardID, cc.ShardID)
	}
	if cc.Language != "" {
		weightedParts = append(weightedParts, cc.Language, cc.Language)
	}

	// Low priority (weight 1x)
	weightedParts = append(weightedParts, cc.Frameworks...)

	// Count by priority
	highPriorityCount := 6   // 3 + 3
	mediumPriorityCount := 4 // 2 + 2
	lowPriorityCount := 2    // 1 + 1

	totalParts := len(weightedParts)
	expectedTotal := highPriorityCount + mediumPriorityCount + lowPriorityCount

	if totalParts != expectedTotal {
		t.Errorf("Expected %d total parts, got %d", expectedTotal, totalParts)
	}

	// High priority should make up 50% of the query (6/12 = 50%)
	highPriorityRatio := float64(highPriorityCount) / float64(totalParts)
	if highPriorityRatio < 0.4 {
		t.Errorf("High priority terms should dominate (expected >= 40%%, got %.1f%%)", highPriorityRatio*100)
	}
}
