package init

import (
	"testing"

	"codenerd/internal/orient"
	"codenerd/internal/types"
)

// A paragraph two different files carry verbatim is one row of the agent's
// knowledge; the tracked file's copy is the one kept.
func TestEcosystemKnowledgeByAgent_SharedParagraphIsOneRow(t *testing.T) {
	shared := "Always run the focused package tests before handoff."
	byID := map[string]orient.Source{
		"a-local": {ID: "a-local", Tool: "claude", Path: "notes/local.md", Tracked: false, Body: shared + "\n\nlocal only"},
		"b-repo":  {ID: "b-repo", Tool: "codex", Path: ".codex/skills/x/SKILL.md", Tracked: true, Body: "repo only\n\n" + shared},
	}
	knowledge := []types.Fact{
		{Predicate: "orient_agent_knowledge", Args: []any{"Reviewer", "a-local"}},
		{Predicate: "orient_agent_knowledge", Args: []any{"Reviewer", "b-repo"}},
	}
	got := ecosystemKnowledgeByAgent(knowledge, byID, func(s string) string { return s }, 0)["Reviewer"]

	var sharedRows []ecosystemAtom
	contents := map[string]bool{}
	for _, atom := range got {
		contents[atom.Content] = true
		if atom.Content == shared {
			sharedRows = append(sharedRows, atom)
		}
	}
	if len(sharedRows) != 1 {
		t.Fatalf("shared paragraph held %d times, want once: %+v", len(sharedRows), got)
	}
	if sharedRows[0].Path != ".codex/skills/x/SKILL.md" {
		t.Fatalf("shared paragraph kept from %s, want the tracked file", sharedRows[0].Path)
	}
	if !contents["local only"] || !contents["repo only"] {
		t.Fatalf("a paragraph only one file holds was dropped: %+v", got)
	}
}
