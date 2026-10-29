// Package defaults provides embedded default resources for codeNERD.
// This file embeds the predicate corpus database for schema validation.
package defaults

import "embed"

// PredicateCorpusDB contains the pre-built predicate corpus database.
// This database is built by running: go run ./cmd/tools/predicate_corpus_builder
//
// The corpus contains:
// - All declared predicates from schemas.mg with their types and signatures
// - IDB predicates derived from policy.mg
// - Error patterns for repair guidance
// - Predicate examples from mangle-programming skill resources
//
// Usage:
//
//	data, err := PredicateCorpusDB.ReadFile("predicate_corpus.db")
//	if err != nil {
//	    // Handle missing corpus (development mode)
//	}
//
// NOTE: The embed pattern uses a wildcard to allow the build to succeed
// even when the corpus database hasn't been generated yet.
//
//go:embed predicate_corpus.db*
var PredicateCorpusDB embed.FS

// PredicateCorpusAvailable returns true if the embedded corpus is available.
// During development, the corpus may not exist until built.
func PredicateCorpusAvailable() bool {
	entries, err := PredicateCorpusDB.ReadDir(".")
	if err != nil {
		return false
	}
	for _, e := range entries {
		if e.Name() == "predicate_corpus.db" {
			return true
		}
	}
	return false
}
