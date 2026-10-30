package init

import (
	"reflect"
	"strings"
	"testing"
)

func TestAgentNameParts(t *testing.T) {
	tests := []struct {
		name string
		want []string
	}{
		{"mangle-logic-architect", []string{"mangle", "logic", "architect"}},
		{"SecurityAuditor", []string{"security", "auditor"}},
		{"TestArchitect", []string{"test", "architect"}},
		{"MCPIntegrationExpert", []string{"mcp", "integration", "expert"}},
		{"arch_propose_scout_literature", []string{"arch", "propose", "scout", "literature"}},
		{"codenerd-dogfood", []string{"codenerd", "dogfood"}},
		{"GoExpert", []string{"go", "expert"}},
		{"BubbleTeaExpert", []string{"bubble", "tea", "expert"}},
		{"agent.v2 helper", []string{"agent", "v2", "helper"}},
		{"", nil},
		{"---", nil},
	}
	for _, tt := range tests {
		if got := AgentNameParts(tt.name); !reflect.DeepEqual(got, tt.want) {
			t.Errorf("AgentNameParts(%q) = %v, want %v", tt.name, got, tt.want)
		}
	}
}

// The vocabulary is asserted as name constants and joined by exact equality in
// the kernel, so every entry must already be in the form boot asserts.
func TestConceptDomainsAreWellFormed(t *testing.T) {
	seenKey := map[string]bool{}
	seenTerm := map[string]string{}
	for _, c := range ConceptDomains() {
		if c.Key == "" || c.Key != strings.ToLower(c.Key) || strings.ContainsAny(c.Key, " /\t") {
			t.Errorf("concept key %q is not a lower-case single word", c.Key)
		}
		if seenKey[c.Key] {
			t.Errorf("concept key %q repeated", c.Key)
		}
		seenKey[c.Key] = true
		if len(c.Subjects) == 0 {
			t.Errorf("concept %q has no research subjects: an agent holding it would have nothing to know", c.Key)
		}
		for _, term := range c.Terms {
			if term != strings.ToLower(term) || strings.ContainsAny(term, " /\t") {
				t.Errorf("concept %q term %q is not a lower-case single word", c.Key, term)
			}
			if other, dup := seenTerm[term]; dup {
				t.Errorf("term %q names both %q and %q", term, other, c.Key)
			}
			seenTerm[term] = c.Key
		}
		for _, verb := range c.Verbs {
			if !strings.HasPrefix(verb, "/") || verb != strings.ToLower(verb) {
				t.Errorf("concept %q verb %q is not a /lower-case verb", c.Key, verb)
			}
		}
	}
	// A concept key may not also be a technology the vocabulary names: the
	// two would be one domain with two meanings.
	for _, root := range TechnologyImportRoots() {
		if seenKey[root.Technology] {
			t.Errorf("%q is both a technology and a concept domain", root.Technology)
		}
	}
}

func TestKnowledgeSubjects_ConceptDomainContributesItsSubjects(t *testing.T) {
	got := KnowledgeSubjects("", "securityauditor", []string{"security"}, nil, nil)
	want := ConceptSubjects("security")
	if len(want) == 0 || len(got) != len(want) {
		t.Fatalf("subjects = %+v, want one per concept subject %v", got, want)
	}
	for i, s := range got {
		if s.Topic != want[i] || s.Technology != "security" || s.Agent != "securityauditor" {
			t.Errorf("subject %d = %+v, want topic %q for domain security", i, s, want[i])
		}
	}
	// A domain that is neither a concept nor pinned stays a plain subject.
	plain := KnowledgeSubjects("", "a", []string{"unheardof"}, nil, nil)
	if len(plain) != 1 || plain[0].Topic != "unheardof" {
		t.Errorf("plain domain subjects = %+v", plain)
	}
	// A topic the agent already researches is not asked twice.
	dup := KnowledgeSubjects("", "a", []string{"security"}, ConceptSubjects("security")[:1], nil)
	if len(dup) != len(ConceptSubjects("security")) {
		t.Errorf("a concept subject already a topic was repeated: %+v", dup)
	}
}
