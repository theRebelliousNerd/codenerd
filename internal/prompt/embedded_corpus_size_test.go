package prompt

import (
	"testing"
	"unicode/utf8"
)

// maxEmbeddedAtomContentChars is the ceiling on one embedded atom's content.
// An atom is one idea said concisely: a model reads a 1,000-character atom
// closely and skims a 20,000-character converted manual. Oversized material is
// split into sibling atoms (<original-id>/<slug>) or tightened, not shipped.
const maxEmbeddedAtomContentChars = 1500

// oversizeExemptions lists atoms allowed to exceed the ceiling. Keep it short:
// every entry needs a reason that a split or rewrite cannot remove, and the
// test fails when an entry is no longer needed.
var oversizeExemptions = map[string]string{
	// internal/perception/understanding_adapter_test.go pins this atom to carry
	// the canonical schema block plus five worked examples in one content
	// string (it splits on "```json" and expects exactly 7 blocks), and the
	// atom is mandatory, so it cannot be split into several mandatory atoms.
	"perception_understanding": "pinned by the perception contract test: one schema and five examples in a single mandatory atom",
}

func TestEmbeddedCorpus_AtomContentStaysWithinCeiling(t *testing.T) {
	corpus, err := LoadEmbeddedCorpus()
	if err != nil {
		t.Fatalf("load the embedded corpus: %v", err)
	}

	present := make(map[string]bool, len(oversizeExemptions))
	for _, a := range corpus.All() {
		if a == nil {
			continue
		}
		n := utf8.RuneCountInString(a.Content)
		if _, exempt := oversizeExemptions[a.ID]; exempt {
			present[a.ID] = true
			if n <= maxEmbeddedAtomContentChars {
				t.Errorf("atom %q is within the ceiling (%d chars); remove it from oversizeExemptions", a.ID, n)
			}
			continue
		}
		if n > maxEmbeddedAtomContentChars {
			t.Errorf("atom %q content is %d chars, ceiling %d: split it into <id>/<slug> atoms or tighten it", a.ID, n, maxEmbeddedAtomContentChars)
		}
	}
	for id, reason := range oversizeExemptions {
		if !present[id] {
			t.Errorf("oversize exemption %q (%s) names an atom that is not in the embedded corpus", id, reason)
		}
	}
	if len(oversizeExemptions) > 5 {
		t.Errorf("oversizeExemptions has %d entries; the list must stay short", len(oversizeExemptions))
	}
}
