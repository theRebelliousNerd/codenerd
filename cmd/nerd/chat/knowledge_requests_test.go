package chat

import (
	"testing"

	"codenerd/internal/articulation"
)

// A repeated ask is one consultation and must not take the cap slot a
// different question needed.
func TestDedupeKnowledgeRequests_ARepeatedAskIsOneConsultation(t *testing.T) {
	m := NewTestModel()
	t.Cleanup(m.shutdownCancel)
	got := m.dedupeKnowledgeRequests([]articulation.KnowledgeRequest{
		{Specialist: "researcher", Query: "What is X?", Purpose: "p"},
		{Specialist: "researcher", Query: "  what is   x? ", Purpose: "P"},
		{Specialist: "researcher", Query: "what is Y?", Purpose: "p"},
	})
	if len(got) != 2 || got[0].Query != "What is X?" || got[1].Query != "what is Y?" {
		t.Fatalf("got %+v", got)
	}
	if len(got) > maxKnowledgeRequestsPerTurn {
		t.Fatalf("the different question would have been capped out: %d", len(got))
	}
}

// "_any_specialist" resolves before the comparison, so it folds into the
// specialist it resolves to.
func TestDedupeKnowledgeRequests_AnySpecialistIsResolvedFirst(t *testing.T) {
	m := NewTestModel()
	t.Cleanup(m.shutdownCancel)
	got := m.dedupeKnowledgeRequests([]articulation.KnowledgeRequest{
		{Specialist: "_any_specialist", Query: "explain the mangle predicate"},
		{Specialist: "mangleexpert", Query: "explain the mangle predicate"},
	})
	if len(got) != 1 || got[0].Specialist != "mangleexpert" {
		t.Fatalf("got %+v", got)
	}
}

func TestMatchSpecialistForQuery_IsOrderedAndWordAware(t *testing.T) {
	m := NewTestModel()
	t.Cleanup(m.shutdownCancel)
	// "build" contains "ui"; "category" contains "go". Neither names a topic.
	if got := m.matchSpecialistForQuery("how do I build the category index"); got != "researcher" {
		t.Fatalf("substring match routed to %q", got)
	}
	// A query matching two specialists resolves the same way every time.
	for range 50 {
		if got := m.matchSpecialistForQuery("write a test for the mangle rule"); got != "mangleexpert" {
			t.Fatalf("routed to %q; the table order says mangleexpert", got)
		}
	}
}
