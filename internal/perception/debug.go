package perception

import "context"

// DebugTaxonomy exposes the internal classification logic for verification tools.
// It returns the selected verb, category, confidence, and shard type.
// Uses a background context for semantic classification.
func DebugTaxonomy(input string) (string, string, float64, string) {
	return matchVerbFromCorpus(context.Background(), input)
}

// DebugTaxonomyWithContext exposes the internal classification logic with context support.
// Use this variant when you have a context to propagate (e.g., for cancellation).
func DebugTaxonomyWithContext(ctx context.Context, input string) (string, string, float64, string) {
	return matchVerbFromCorpus(ctx, input)
}
