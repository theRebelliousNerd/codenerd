package prompt

import (
	"testing"

	"codenerd/internal/store"
)

// The same text held under two concepts, or by two sources, enters a compile
// once: the first copy, from the source searched first. Distinct texts all
// enter.
func TestAppendKnowledgePromptAtoms_TheSameBodyEntersOnce(t *testing.T) {
	t.Parallel()
	seen := map[string]struct{}{}
	page := "how the importers of mangle-go call the analysis package"
	first := []store.KnowledgeAtom{
		{Concept: "paragraph path=.claude/agents/a.md", Content: "one paragraph", Confidence: 0.8},
		{Concept: "paragraph path=.grok/agents/a.md", Content: "one paragraph", Confidence: 0.8},
		{Concept: "repo/importers", Content: page, Confidence: 0.9},
	}
	second := []store.KnowledgeAtom{
		{Concept: "repo/importers", Content: page + "\n", Confidence: 0.7},
		{Concept: "another", Content: "a different page", Confidence: 0.7},
	}
	var out []*PromptAtom
	out = appendKnowledgePromptAtoms(out, seen, first, "", expertSourceName("mangleexpert"), "mangleexpert")
	out = appendKnowledgePromptAtoms(out, seen, second, "", expertSourceName("mangle-logic-architect"), "mangle-logic-architect")
	if len(out) != 3 {
		var ids []string
		for _, a := range out {
			ids = append(ids, a.KnowledgeConcept)
		}
		t.Fatalf("got %d rows %v, want 3: one paragraph, the page once, the other page", len(out), ids)
	}
	if out[1].KnowledgeExpert != "mangleexpert" {
		t.Errorf("the page's kept copy is %s's, want the first source's", out[1].KnowledgeExpert)
	}
}
