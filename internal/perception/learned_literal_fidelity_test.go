package perception

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"codenerd/internal/mangle"
)

// A learned example whose text holds an escaped quotation mark lost the
// escape on its way to the engine: the parser decoded \" to " and the
// normalizer pasted the decoded text back between quotes, so the hot-loaded
// source was malformed and a legitimate correction was refused, or read as
// different text (audit learned-example-literal-fidelity). The fact now
// goes through the Mangle parser into typed values and back out through
// Mangle's own quoting.
func TestNormalizeLearnedFact_KeepsEveryLiteralExact(t *testing.T) {
	t.Parallel()
	for _, c := range []struct{ name, pattern, constraint string }{
		{"escaped quotes", `say \"hello\" twice`, `keep \"as is\"`},
		{"backslash", `path C:\\temp\\x`, ``},
		{"comma and parens", `a, b (c), d`, `e, f`},
		{"unicode", `é ü — 漢`, `ok`},
		{"newline escape", `two\nlines`, `x`},
		{"slash-leading string", `/not a name`, `/also not`},
	} {
		fact := `learned_exemplar("` + c.pattern + `", /explain, "target", "` + c.constraint + `", 0.90).`
		want, err := mangle.ParseGroundFacts(fact)
		if err != nil {
			t.Fatalf("%s: the fixture is not valid Mangle: %v", c.name, err)
		}
		norm, err := NormalizeLearnedFact(fact)
		if err != nil {
			t.Fatalf("%s: NormalizeLearnedFact: %v", c.name, err)
		}
		got, err := mangle.ParseGroundFacts(norm)
		if err != nil {
			t.Fatalf("%s: normalized source %q does not parse: %v", c.name, norm, err)
		}
		if len(got) != 1 || len(got[0].Args) != 5 {
			t.Fatalf("%s: normalized source %q is not one five-argument fact", c.name, norm)
		}
		for _, i := range []int{0, 1, 2, 3} {
			if got[0].Args[i] != want[0].Args[i] {
				t.Errorf("%s: argument %d = %#v, want %#v (source %q)", c.name, i, got[0].Args[i], want[0].Args[i], norm)
			}
		}
		pattern, _, _, constraint, _, err := ParseLearnedFact(fact)
		if err != nil || pattern != want[0].Args[0] || constraint != want[0].Args[3] {
			t.Errorf("%s: ParseLearnedFact = %q / %q (%v), want the decoded literals", c.name, pattern, constraint, err)
		}
	}
}

// Anything but one ground learned_exemplar of the expected types is refused.
func TestParseLearnedFact_RefusesWhatIsNotOneTypedExemplar(t *testing.T) {
	t.Parallel()
	for _, fact := range []string{
		`learned_exemplar("a", X, "t", "c", 0.9).`,
		`learned_exemplar("a", "explain", "t", "c", 0.9).`,
		`learned_exemplar(/a, /explain, "t", "c", 0.9).`,
		`learned_exemplar("a", /explain, "t", "c", 0.9). learned_exemplar("b", /fix, "t", "c", 0.9).`,
		`learned_exemplar("a", /explain, "t", "c", 0.9) :- other().`,
		`learned_exemplar("a, /explain, "t", "c", 0.9).`,
		`learned_exemplar("a", /explain, "t", "c", "high").`,
	} {
		if _, _, _, _, _, err := ParseLearnedFact(fact); err == nil {
			t.Errorf("%q accepted", fact)
		}
	}
}

// End to end: a correction with quotes in it is persisted, hot-loaded and
// written to the taxonomy file with its text intact.
func TestPersistLearnedFact_KeepsQuotedText(t *testing.T) {
	eng := testTaxonomyEngine(t)
	root := t.TempDir()
	eng.SetWorkspace(root)
	fact := `learned_exemplar("say \"hello\" back", /explain, "", "", 0.95).`
	if _, err := eng.PersistLearnedFact(context.Background(), fact); err != nil {
		t.Fatalf("PersistLearnedFact refused a valid correction: %v", err)
	}
	raw, err := os.ReadFile(filepath.Join(root, ".nerd", "mangle", "learned_taxonomy.mg"))
	if err != nil {
		t.Fatal(err)
	}
	facts, err := mangle.ParseGroundFacts(strings.TrimSpace(string(raw)))
	if err != nil {
		t.Fatalf("the taxonomy file does not parse: %v\n%s", err, raw)
	}
	if len(facts) != 1 || facts[0].Args[0] != `say "hello" back` {
		t.Fatalf("taxonomy file holds %v, want the text say \"hello\" back", facts)
	}
}
