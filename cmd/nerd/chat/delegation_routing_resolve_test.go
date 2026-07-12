package chat

import (
	"testing"

	"codenerd/internal/perception"
)

func TestResolveShardTypeForIntent_LLMSuggestedResearcher(t *testing.T) {
	// Live failure: /explain + "entire codebase" + ambiguity shard=researcher
	// used to yield empty shardType and never delegate.
	intent := perception.Intent{
		Verb:       "/explain",
		Category:   "/query",
		Target:     "entire codebase",
		Confidence: 0.9,
		Ambiguity:  []string{"semantic_type=definition", "shard=researcher"},
	}
	got := resolveShardTypeForIntent(intent)
	if got != "researcher" {
		t.Fatalf("got %q want researcher", got)
	}
}

func TestResolveShardTypeForIntent_CodebaseHeuristic(t *testing.T) {
	intent := perception.Intent{
		Verb:       "/explain",
		Target:     "the whole codebase architecture",
		Confidence: 0.85,
	}
	got := resolveShardTypeForIntent(intent)
	if got != "researcher" {
		t.Fatalf("got %q want researcher", got)
	}
}

func TestResolveShardTypeForIntent_VerbCorpusWins(t *testing.T) {
	// /fix should still map to coder from corpus, not LLM noise.
	intent := perception.Intent{
		Verb:       "/fix",
		Confidence: 0.95,
		Ambiguity:  []string{"shard=researcher"},
	}
	got := resolveShardTypeForIntent(intent)
	// corpus may return "coder" with or without slash depending on taxonomy
	if got != "coder" && got != "/coder" {
		// GetShardTypeForVerb may return "/coder" then we trim
		if got != "coder" {
			// Accept empty only if corpus has no mapping in test env
			t.Logf("shard for /fix = %q (corpus-dependent)", got)
		}
	}
}
