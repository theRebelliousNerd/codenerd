package system

import (
	"context"
	"strings"
	"testing"

	"codenerd/internal/mangle"
)

// The repair shard's fallback prompt -- used when no JIT prompt can be
// compiled -- must teach the syntax the parser accepts. It wrote negation as
// "positive(X), not negative(X)", which Mangle does not parse, so a repair made
// under the fallback failed on its own advice. The example is taken from the
// prompt and parsed with the production parser.
func TestMangleRepairFallbackPromptTeachesParsableNegation(t *testing.T) {
	prompt := NewMangleRepairShard().getSystemPrompt(context.Background(), []string{"negation of unbound variable"})

	const lead = "its variables must be bound first: "
	i := strings.Index(prompt, lead)
	if i < 0 {
		t.Fatalf("the fallback prompt no longer shows a negation example:\n%s", prompt)
	}
	example := strings.TrimSpace(strings.SplitN(prompt[i+len(lead):], "\n", 2)[0])
	rule := "repaired(X) :- " + example + ".\n"
	program := "Decl positive(X) bound [/string].\nDecl negative(X) bound [/string].\nDecl repaired(X) bound [/string].\n" + rule
	if _, err := mangle.ParseUnit(strings.NewReader(program)); err != nil {
		t.Fatalf("the fallback prompt's negation example does not parse (%q): %v", example, err)
	}
}
