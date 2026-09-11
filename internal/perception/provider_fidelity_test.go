package perception

import (
	"encoding/json"
	"strings"
	"testing"

	"codenerd/internal/types"
)

// BlockFidelity is a declaration, and a declaration nothing checks is the thing
// it was written to replace. These tests run the real mappers over the same
// interleaved turn the round-trip tests use and hold every surface to what the
// table says about it — in both directions, so a mapper that quietly GAINS a
// capability fails as loudly as one that quietly loses it.
//
// A gained capability failing looks pedantic until you notice what it means: if
// chat completions started replaying signatures, either the table is wrong and
// callers are declining to pay for reasoning they could have replayed, or the
// mapper is now sending a field that endpoint rejects. Both are worth a red
// test, and neither is visible from the passing direction alone.

// fidelityProbe is one surface's mapper reduced to the bytes it would put on
// the wire. Encoding to JSON is deliberate: the fidelity question is about what
// the API receives, and a Go struct with a field the marshaller omits has
// already answered it differently from what the struct suggests.
type fidelityProbe func(t *testing.T, history []types.Message) string

var fidelityProbes = map[Surface]fidelityProbe{
	SurfaceAnthropicMessages: func(t *testing.T, history []types.Message) string {
		t.Helper()
		msgs, err := buildAnthropicMessagesFromHistory(history)
		if err != nil {
			t.Fatalf("buildAnthropicMessagesFromHistory: %v", err)
		}
		return encodeForFidelity(t, msgs)
	},
	SurfaceOpenAIChatCompletions: func(t *testing.T, history []types.Message) string {
		t.Helper()
		msgs, err := MapTypesHistoryToOpenAIMessages("", history)
		if err != nil {
			t.Fatalf("MapTypesHistoryToOpenAIMessages: %v", err)
		}
		return encodeForFidelity(t, msgs)
	},
	SurfaceOpenAIResponses: func(t *testing.T, history []types.Message) string {
		t.Helper()
		// nil side cache: this turn carries its own signed reasoning, so the
		// cache is not what is under test and supplying one would let a
		// mapper that dropped the message's reasoning still look correct.
		return encodeForFidelity(t, metaInputFromHistory("", history, nil))
	},
	SurfaceGeminiContents: func(t *testing.T, history []types.Message) string {
		t.Helper()
		contents, err := geminiContentsFromHistory(history)
		if err != nil {
			t.Fatalf("geminiContentsFromHistory: %v", err)
		}
		return encodeForFidelity(t, contents)
	},
}

func encodeForFidelity(t *testing.T, v any) string {
	t.Helper()
	encoded, err := json.Marshal(v)
	if err != nil {
		t.Fatalf("marshal request: %v", err)
	}
	return string(encoded)
}

// The two prose fragments of the interleaved turn, welded together. This
// substring exists in a request if and only if the surface collapsed the
// position between them, because nothing else in the corpus produces it.
const weldedProse = "I'll check two things.Now the caller."

// The body of the interleaved turn's thinking block. On a surface that cannot
// replay reasoning this string must not reach the wire at all.
const reasoningProse = "Check the config first, then the caller."

func TestEveryDeclaredSurfaceMatchesWhatItsMapperDoes(t *testing.T) {
	for surface, declared := range BlockFidelity {
		probe, ok := fidelityProbes[surface]
		if !ok {
			t.Errorf("BlockFidelity declares %q and no probe exercises it, so the "+
				"declaration is prose again", surface)
			continue
		}
		t.Run(string(surface), func(t *testing.T) {
			request := probe(t, interleavedHistory())

			// Reasoning: the signature is the only thing a provider will
			// accept back, so its presence is the whole question.
			replayed := strings.Contains(request, "sig-abc-123")
			if replayed != declared.ReplaysReasoning {
				t.Errorf("ReplaysReasoning declared %v, mapper does %v\nrequest: %s",
					declared.ReplaysReasoning, replayed, request)
			}

			// Order: the welded prose appears exactly when the position
			// between the two tool calls was lost.
			kept := !strings.Contains(request, weldedProse)
			if kept != declared.KeepsOrder {
				t.Errorf("KeepsOrder declared %v, mapper does %v\nrequest: %s",
					declared.KeepsOrder, kept, request)
			}

			// Tool ids: our id reaches the wire, or the surface pairs on
			// something else and the id never leaves this process.
			carriesIDs := strings.Contains(request, "toolu_A")
			if carriesIDs != declared.CarriesToolIDs {
				t.Errorf("CarriesToolIDs declared %v, mapper does %v\nrequest: %s",
					declared.CarriesToolIDs, carriesIDs, request)
			}

			// A surface that cannot replay reasoning must DROP it, not fold it
			// into the prose. Smuggling is the tempting fix — the words are
			// right there, and the turn reads fine — and it is worse than the
			// loss: reasoning presented as an answer corrupts every structured
			// -output parse downstream, and the model reads its own scratch
			// work as something it told the user.
			if !declared.ReplaysReasoning && strings.Contains(request, reasoningProse) {
				t.Errorf("%s cannot replay reasoning and put it in the prose anyway"+
					"\nrequest: %s", surface, request)
			}
		})
	}
}

// The other direction: a mapper wired into the probe table but missing from
// BlockFidelity is a surface whose limits nobody wrote down. That is how the
// fifth adapter gets added without anyone noticing it drops reasoning.
func TestEveryProbedSurfaceIsDeclared(t *testing.T) {
	for surface := range fidelityProbes {
		if _, ok := BlockFidelity[surface]; !ok {
			t.Errorf("surface %q has a probe and no BlockFidelity entry; its limits "+
				"are undeclared", surface)
		}
	}
}

// Every tool result must reach every surface, whatever else that surface drops.
//
// This is the one thing no provider is allowed to lose. A tool_use with no
// matching result is a turn the endpoint rejects outright on strict providers
// and a turn the model answers from nothing on permissive ones, and it is
// exactly what a mapper written to "just drop what does not fit" produces.
func TestNoSurfaceDropsAToolResult(t *testing.T) {
	for surface, probe := range fidelityProbes {
		t.Run(string(surface), func(t *testing.T) {
			request := probe(t, interleavedHistory())
			for _, want := range []string{"package config", "no matches"} {
				if !strings.Contains(request, want) {
					t.Errorf("%s dropped the tool result %q\nrequest: %s", surface, want, request)
				}
			}
		})
	}
}

// A surface nobody has declared must read as carrying nothing.
//
// Callers index this map directly, so the miss is silent by construction: there
// is no comma-ok to forget, and a typo'd surface name reads as a real answer.
// The zero value being the SAFE answer is what makes that acceptable, and this
// is the test that keeps it true — reordering the struct so that some future
// field's zero value means "yes" would break here rather than in production,
// where it would mean a signature sent to an endpoint that rejects the request.
func TestAnUndeclaredSurfaceCarriesNothing(t *testing.T) {
	missing := BlockFidelity[Surface("a-provider-nobody-has-written-yet")]
	if missing != (Fidelity{}) {
		t.Fatalf("a missing surface answered %+v, not the zero value", missing)
	}
	if missing.ReplaysReasoning {
		t.Error("an undeclared surface claimed it can replay reasoning; the safe " +
			"default is to decline to pay for it, not to send a signature that " +
			"gets the whole request rejected")
	}
	if missing.KeepsOrder || missing.CarriesToolIDs {
		t.Error("an undeclared surface claimed a capability nobody wrote down")
	}
}
