package perception

import (
	"context"
	"testing"
)

// context_token/1 carries the words of the user's input, one fact per token.
// ClassifyInput builds them with strings.FieldsSeq + a punctuation trim and
// passes each as a bare Go string, and the Decl in schemas_intent.mg says
// bound [/string] — which makes internal/mangle's convertValueToTypedTerm take
// the ast.StringType branch and explicitly skip the identifier auto-atomizer.
// Every token is therefore a STRING constant.
//
// policy/taxonomy_qualifiers.mg used to match the multi-token phrases as NAME
// constants (context_token(/what), context_token(/is)) on the strength of a
// comment claiming single words "are often atomized if they are identifiers".
// They are not, for this predicate. A name never unifies with a string, so all
// fifteen multi-token interrogative/modal/existence rules derived nothing while
// the single-token rule sitting between them worked — the failure mode this
// whole Decl audit exists to find, and one that no build or analysis error
// could ever surface.
//
// interrogative_type/modal_type/existence_pattern all hold their phrase in a
// /string slot too ("what is", "can you"), so the quoted form is also the only
// one that matches the table it joins against.
func TestContextTokenMultiWordPhrasesDerive(t *testing.T) {
	te, err := NewTaxonomyEngine()
	if err != nil {
		t.Fatalf("NewTaxonomyEngine() error = %v", err)
	}
	te.engine.Clear()

	// Exactly the shape ClassifyInput produces, for an input that contains
	// "what is", "why is", "how do i", "how can i" and "can you".
	for _, tok := range []string{"what", "is", "why", "how", "do", "can", "i", "you", "the", "parser"} {
		if err := te.engine.AddFact("context_token", tok); err != nil {
			t.Fatalf("AddFact(context_token, %q) error = %v", tok, err)
		}
	}

	ctx := context.Background()
	got := func(query string) map[string]bool {
		t.Helper()
		res, qErr := te.engine.Query(ctx, query)
		if qErr != nil {
			t.Fatalf("Query(%s) error = %v", query, qErr)
		}
		out := map[string]bool{}
		for _, b := range res.Bindings {
			if w, ok := b["W"].(string); ok {
				out[w] = true
			}
		}
		return out
	}

	interrogatives := got("detected_interrogative(W, S, V, P)")
	for _, want := range []string{
		"what", "why", "how", // single token — these always worked
		"what is", "why is", "how do i", "how can i", // multi token — these did not
	} {
		if !interrogatives[want] {
			t.Errorf("detected_interrogative is missing %q; got %v", want, interrogatives)
		}
	}

	modals := got("detected_modal(W, M, T, P)")
	for _, want := range []string{"can", "can you"} {
		if !modals[want] {
			t.Errorf("detected_modal is missing %q; got %v", want, modals)
		}
	}
}
