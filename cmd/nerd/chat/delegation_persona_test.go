package chat

import (
	"strings"
	"testing"

	"codenerd/internal/perception"
	"codenerd/internal/types"
)

// A persona runs its persona_verb, and a write-oriented verb ends a turn that
// wrote nothing in the hollow-success guard. An admitted /query is therefore
// never handed to a persona whose verb writes, whichever source named the
// candidate (policy/delegation.mg delegation_persona).

func personaCandidates() map[string]perception.Intent {
	// /explain's corpus shard is /none, so its candidate is the one
	// perception derived (shard=...). /debug's is the coder from the corpus.
	const file = "internal/stats/stats.go"
	explain := func(shard string) perception.Intent {
		return perception.Intent{
			Category: "/query", Verb: "/explain", Target: file, ScopeLevel: "file", ScopeFile: file,
			Confidence: 0.9, IsQuestion: true, Ambiguity: []string{"semantic_type=definition", shard},
		}
	}
	return map[string]perception.Intent{
		"derived shard=coder (explain + git default-table witness)": explain("shard=coder"),
		"derived shard=/coder": explain("shard=/coder"),
		"corpus /debug": {
			Category: "/query", Verb: "/debug", Target: file, Confidence: 0.96, IsQuestion: true,
		},
	}
}

func TestDecideRoute_AQueryNeverGoesToAWritingPersona(t *testing.T) {
	t.Parallel()
	for name, intent := range personaCandidates() {
		t.Run(name, func(t *testing.T) {
			if got := resolveShardTypeForIntent(intent); got != "coder" {
				t.Fatalf("candidate = %q, want coder: the fixture must reach the coder before the kernel decides", got)
			}
			got := routeFor(t, "why does this fail?", intent)
			if got.Kind != RouteDelegate || got.Shard != "reviewer" || !got.Background || got.Verify {
				t.Fatalf("route = %+v, want a background, unverified delegation to the reviewer", got)
			}
		})
	}
}

// The persona the kernel picks must itself run a verb that finishes without a
// write, or the redirect only moves the failure.
func TestDelegationPersona_TheReaderRunsAReadVerb(t *testing.T) {
	t.Parallel()
	m := newRoundtripModel(t)
	verbs, err := m.kernel.Query("persona_verb")
	if err != nil {
		t.Fatal(err)
	}
	var reviewerVerb string
	for _, f := range verbs {
		if len(f.Args) == 2 && strings.HasSuffix(types.ExtractString(f.Args[0]), "reviewer") {
			reviewerVerb = types.ExtractString(f.Args[1])
		}
	}
	if reviewerVerb == "" {
		t.Fatal("persona_verb has no reviewer row")
	}
	writes, err := m.kernel.Query("write_oriented_intent")
	if err != nil {
		t.Fatal(err)
	}
	for _, f := range writes {
		if len(f.Args) == 1 && types.ExtractString(f.Args[0]) == reviewerVerb {
			t.Fatalf("the reviewer's verb %s is write-oriented", reviewerVerb)
		}
	}
}

// Controls: authorised mutations keep the coder and their verification; a
// query a read-only persona already takes is unchanged.
func TestDecideRoute_PersonaControls(t *testing.T) {
	t.Parallel()
	t.Run("a mutation /debug stays with the coder under verification", func(t *testing.T) {
		intent := perception.Intent{Category: "/mutation", Verb: "/debug", Target: "Mean in stats.go", Confidence: 0.96}
		got := routeFor(t, "fix Mean", intent)
		if got.Kind != RouteDelegate || got.Shard != "coder" || got.Background || !got.Verify {
			t.Fatalf("route = %+v, want a verified foreground delegation to the coder", got)
		}
	})
	t.Run("a query suggested to the researcher stays with the researcher", func(t *testing.T) {
		intent := perception.Intent{Category: "/query", Verb: "/explain", Target: "entire codebase", Confidence: 0.9,
			IsQuestion: true, ScopeLevel: "file", ScopeFile: "main.go", Ambiguity: []string{"shard=researcher"}}
		got := routeFor(t, "teach me the codebase", intent)
		if got.Kind != RouteDelegate || got.Shard != "researcher" || !got.Background {
			t.Fatalf("route = %+v, want a background delegation to the researcher", got)
		}
	})
	t.Run("a candidate below the confidence gate does not spawn", func(t *testing.T) {
		intent := perception.Intent{Category: "/query", Verb: "/debug", Target: "stats.go", Confidence: 0.05, IsQuestion: true}
		if got := routeFor(t, "why?", intent); got.Kind == RouteDelegate {
			t.Fatalf("route = %+v, want no delegation below the gate", got)
		}
	})
	t.Run("a query with no evidence need and no candidate answers directly", func(t *testing.T) {
		intent := perception.Intent{Category: "/query", Verb: "/explain", Target: "what is a goroutine", Confidence: 0.9, IsQuestion: true}
		if got := routeFor(t, "what is a goroutine?", intent); got.Kind == RouteDelegate {
			t.Fatalf("route = %+v, want no delegation", got)
		}
	})
}
