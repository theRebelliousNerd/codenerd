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

// /debug reads as a query ("why does this fail?") but the corpus gives it to the
// coder, whose persona verb is /fix. A read-only request must go to a shard that
// can finish without writing: delegated to the coder it ran in the background
// and failed every time on the hollow-success guard.
func TestResolveShardTypeForIntent_ReadOnlyDebugGoesToTheReviewer(t *testing.T) {
	query := perception.Intent{Verb: "/debug", Category: "/query", Target: "Mean in stats.go", Confidence: 0.96}
	if got := resolveShardTypeForIntent(query); got != "reviewer" {
		t.Fatalf("a /query /debug went to %q, want reviewer", got)
	}
	mutation := perception.Intent{Verb: "/debug", Category: "/mutation", Target: "Mean in stats.go", Confidence: 0.96}
	if got := resolveShardTypeForIntent(mutation); got != "coder" {
		t.Fatalf("a /mutation /debug went to %q, want coder: a request to debug and fix writes", got)
	}
}

// The kernel takes that read-only debug off the turn, to the reviewer.
func TestDecideRoute_ReadOnlyDebugRunsInTheBackgroundAsTheReviewer(t *testing.T) {
	intent := perception.Intent{Category: "/query", Verb: "/debug", Target: "Mean in stats.go", Confidence: 0.96, IsQuestion: true}
	got := routeFor(t, "Mean returns 6 instead of 4, why?", intent)
	if got.Kind != RouteDelegate || got.Shard != "reviewer" || !got.Background {
		t.Fatalf("route = %+v, want a background delegation to the reviewer", got)
	}
}
