package chat

import (
	"testing"

	"codenerd/internal/articulation"
)

// An exact repeat is one consultation and must not take the cap slot a
// different question needed.
func TestDedupeKnowledgeRequests_AnExactRepeatIsOneConsultation(t *testing.T) {
	m := NewTestModel()
	t.Cleanup(m.shutdownCancel)
	got := m.dedupeKnowledgeRequests([]articulation.KnowledgeRequest{
		{Specialist: "researcher", Query: "What is X?", Purpose: "p"},
		{Specialist: "researcher", Query: "What is X?", Purpose: "p"},
		{Specialist: "researcher", Query: "what is Y?", Purpose: "p"},
	})
	if len(got) != 2 || got[0].Query != "What is X?" || got[1].Query != "what is Y?" {
		t.Fatalf("got %+v", got)
	}
}

// Identity is literal: questions that differ only in case or in whitespace
// inside them are different questions, and both are consulted. Folding them
// lost the second from execution and from the results synthesis reads
// (audit knowledge-consultation-literal-identity).
func TestDedupeKnowledgeRequests_CaseAndSpacingAreMeaning(t *testing.T) {
	m := NewTestModel()
	t.Cleanup(m.shutdownCancel)
	for _, pair := range [][2]articulation.KnowledgeRequest{
		{{Specialist: "researcher", Query: `Explain Go regexp \s.`, Purpose: "Choose a character class."},
			{Specialist: "researcher", Query: `Explain Go regexp \S.`, Purpose: "Choose a character class."}},
		{{Specialist: "researcher", Query: `What is the length of Go string "a b"?`, Purpose: "p"},
			{Specialist: "researcher", Query: `What is the length of Go string "a  b"?`, Purpose: "p"}},
		{{Specialist: "researcher", Query: "q", Purpose: "Use Path"},
			{Specialist: "researcher", Query: "q", Purpose: "use path"}},
	} {
		got := m.dedupeKnowledgeRequests([]articulation.KnowledgeRequest{pair[0], pair[1]})
		if len(got) != 2 || got[0].Query != pair[0].Query || got[1].Query != pair[1].Query || got[1].Purpose != pair[1].Purpose {
			t.Errorf("distinct requests %q / %q folded: %+v", pair[0].Query, pair[1].Query, got)
		}
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
