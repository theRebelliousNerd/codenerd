package prompt

import (
	"strings"
	"testing"
)

func TestEmbeddedPromptsUseDecisionSummaries(t *testing.T) {
	corpus, err := LoadEmbeddedCorpus()
	if err != nil {
		t.Fatal(err)
	}
	seenSummary := false
	for _, atom := range corpus.All() {
		for _, retired := range []string{`"reasoning_trace":`, `"chain_of_thought":`, "your actual step-by-step reasoning", "Your reasoning_trace must demonstrate"} {
			if strings.Contains(atom.Content, retired) {
				t.Errorf("atom %s still requests retired analysis output %q", atom.ID, retired)
			}
		}
		if atom.ID == "protocol/decision_summary/requirements" {
			seenSummary = true
			if !strings.Contains(atom.Content, "one or two sentences") || !strings.Contains(atom.Content, "no minimum word count") || !strings.Contains(atom.Content, "observable") && !strings.Contains(atom.Content, "tool results") {
				t.Error("decision summary must request a brief explanation grounded in evidence without a word minimum")
			}
		}
	}
	if !seenSummary {
		t.Fatal("decision summary contract is absent from the embedded corpus")
	}
}
