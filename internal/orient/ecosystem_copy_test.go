package orient

import (
	"testing"

	"codenerd/internal/types"
)

func knowledgeOf(out *Outcome, agent string) []string {
	var ids []string
	for _, f := range out.OrientAgentKnowledge {
		if len(f.Args) == 2 && types.ExtractString(f.Args[0]) == agent {
			ids = append(ids, types.ExtractString(f.Args[1]))
		}
	}
	return ids
}

// One memory note mirrored into three harness trees (same bytes, three
// paths) is one knowledge source: the tracked copy, then the earliest stable
// order. Before, same-digest sources were never paired, every copy was a
// /unique winner, and the agent's KB held each paragraph three times.
func TestPolicy_MirroredCopiesAreOneKnowledgeSource(t *testing.T) {
	facts := sourceFacts("rev", "claude", "subagent", "reviewer", ".claude/agents/reviewer.md", "yes", "d-def", "reviewer", 20, 0, "reviewer")
	facts = append(facts, sourceFacts("m-agents", "agents", "memory", "reviewer", ".agents/memory/reviewer.md", "no", "d-note", "reviewer-note", 851, 1, "reviewer")...)
	facts = append(facts, sourceFacts("m-claude", "claude", "memory", "reviewer", ".claude/agent-memory/reviewer.md", "yes", "d-note", "reviewer-note", 851, 2, "reviewer")...)
	facts = append(facts, sourceFacts("m-grok", "grok", "memory", "reviewer", ".grok/memory/reviewer.md", "yes", "d-note", "reviewer-note", 851, 3, "reviewer")...)
	out := evalPolicy(t, facts)

	got := map[string]bool{}
	for _, id := range knowledgeOf(out, "reviewer") {
		got[id] = true
	}
	if !got["rev"] || !got["m-claude"] || got["m-agents"] || got["m-grok"] || len(got) != 2 {
		t.Fatalf("reviewer knowledge = %v, want rev and the tracked, earliest copy m-claude only", knowledgeOf(out, "reviewer"))
	}
}

// Copies that are all untracked fold to the earliest stable order.
func TestPolicy_UntrackedCopiesFoldToTheEarliest(t *testing.T) {
	facts := sourceFacts("rev", "claude", "subagent", "reviewer", ".claude/agents/reviewer.md", "yes", "d-def", "reviewer", 20, 0, "reviewer")
	facts = append(facts, sourceFacts("m1", "agents", "memory", "reviewer", ".agents/memory/reviewer.md", "no", "d-note", "reviewer-note", 40, 1, "reviewer")...)
	facts = append(facts, sourceFacts("m2", "claude", "memory", "reviewer", ".claude/agent-memory/reviewer.md", "no", "d-note", "reviewer-note", 40, 2, "reviewer")...)
	out := evalPolicy(t, facts)
	got := map[string]bool{}
	for _, id := range knowledgeOf(out, "reviewer") {
		got[id] = true
	}
	if !got["m1"] || got["m2"] {
		t.Fatalf("reviewer knowledge = %v, want m1 and not m2", knowledgeOf(out, "reviewer"))
	}
}
