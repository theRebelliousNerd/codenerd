package init

import (
	"strings"
	"testing"

	"codenerd/internal/store"
)

func TestBuildAtomHashSet_WhenAtoms_ShouldHashByConceptAndContent(t *testing.T) {
	atoms := []store.KnowledgeAtom{
		{Concept: "go:concurrency", Content: "goroutines and channels"},
		{Concept: "go:errors", Content: "wrap with %w"},
	}
	set := buildAtomHashSet(atoms)
	if len(set) != 2 {
		t.Fatalf("expected 2 distinct hashes, got %d", len(set))
	}
	// The hash of an identical concept+content must collide with the set entry.
	h := computeAtomHash("go:concurrency", "goroutines and channels")
	if !set[h] {
		t.Errorf("hash set is missing the expected atom hash %q", h)
	}
	if set[computeAtomHash("missing", "nope")] {
		t.Error("hash set should not contain an unrelated atom hash")
	}
}

func TestParseResearchResult_WhenSectionedContent_ShouldChunkByParagraph(t *testing.T) {
	init := &Initializer{}
	long := strings.Repeat("Detailed knowledge about the topic. ", 5) // > 50 chars
	content := long + "\n\n" + "tiny" + "\n\n" + long
	atoms := init.parseResearchResult("go", content)
	if len(atoms) != 2 {
		t.Fatalf("expected one atom per substantive paragraph, got %d", len(atoms))
	}
	// The short "tiny" section (< 50 chars) must be skipped, and no atom
	// claims to be a summary: the old ":summary" atom was the first 500
	// characters of the text, not a summary of it.
	for _, a := range atoms {
		if a.Content == "tiny" {
			t.Error("sections under 50 chars should be skipped")
		}
		if strings.HasSuffix(a.Concept, ":summary") {
			t.Errorf("a head cut was stored as a summary: %s", a.Concept)
		}
		if strings.TrimSpace(a.Content) != strings.TrimSpace(long) {
			t.Errorf("paragraph was not stored whole: %q", a.Content)
		}
	}
}

func TestParseResearchResult_WhenLongSection_ShouldKeepItWhole(t *testing.T) {
	init := &Initializer{}
	huge := strings.Repeat("x", 3000)
	atoms := init.parseResearchResult("topic", huge)
	if len(atoms) != 1 {
		t.Fatalf("expected one atom, got %d", len(atoms))
	}
	if len(atoms[0].Content) != 3000 {
		t.Errorf("section of len 3000 was cut to %d", len(atoms[0].Content))
	}
}

func TestFilterTopicsNeedingResearch_WhenNoAtoms_ShouldResearchAll(t *testing.T) {
	topics := []string{"go concurrency", "rust ownership"}
	got := filterTopicsNeedingResearch(nil, topics, 2)
	if len(got) != len(topics) {
		t.Errorf("with no existing atoms all topics need research, got %v", got)
	}
}

func TestFilterTopicsNeedingResearch_WhenCovered_ShouldSkip(t *testing.T) {
	existing := []store.KnowledgeAtom{
		{Concept: "go-concurrency:section_0", Content: "go concurrency goroutines"},
		{Concept: "go-concurrency:section_1", Content: "go concurrency channels"},
		// Inherited and identity atoms must NOT count toward coverage.
		{Concept: "inherited:foo", Content: "rust ownership borrow"},
		{Concept: "agent_identity", Content: "rust ownership move"},
	}
	got := filterTopicsNeedingResearch(existing, []string{"go concurrency", "rust ownership"}, 2)
	// "go concurrency" has 2 genuine atoms (covered); "rust ownership" only has
	// inherited/identity atoms (not counted) so it still needs research.
	if len(got) != 1 || got[0] != "rust ownership" {
		t.Errorf("expected only 'rust ownership' to need research, got %v", got)
	}
}

func TestConvertStoreAtomsToInitAtoms_ShouldCopyFields(t *testing.T) {
	src := []store.KnowledgeAtom{{Concept: "c", Content: "body", Confidence: 0.7}}
	got := convertStoreAtomsToInitAtoms(src)
	if len(got) != 1 {
		t.Fatalf("expected 1 atom, got %d", len(got))
	}
	a := got[0]
	if a.Concept != "c" || a.Content != "body" || a.Title != "c" || a.Confidence != 0.7 {
		t.Errorf("field copy mismatch: %+v", a)
	}
}

func TestGenerateBaseKnowledgeAtoms_ShouldIncludeIdentityAndTopics(t *testing.T) {
	init := &Initializer{}
	agent := RecommendedAgent{
		Name:        "go-expert",
		Description: "Go specialist",
		Reason:      "project is Go",
		Topics:      []string{"goroutines", "generics"},
	}
	atoms := init.generateBaseKnowledgeAtoms(agent)
	var hasIdentity, hasMission bool
	for _, a := range atoms {
		switch a.Concept {
		case "agent_identity":
			hasIdentity = true
			if !strings.Contains(a.Content, "go-expert") {
				t.Errorf("identity atom missing agent name: %q", a.Content)
			}
		case "agent_mission":
			hasMission = true
		}
	}
	if !hasIdentity || !hasMission {
		t.Errorf("expected identity and mission atoms, got %d atoms", len(atoms))
	}
	// At least one atom per expertise topic should be present.
	if len(atoms) < 2+len(agent.Topics) {
		t.Errorf("expected >= %d atoms (identity+mission+topics), got %d", 2+len(agent.Topics), len(atoms))
	}
}

func TestComputeDocHash_ShouldBeDeterministicAnd16Chars(t *testing.T) {
	a := computeDocHash("hello world")
	b := computeDocHash("hello world")
	if a != b {
		t.Error("computeDocHash should be deterministic for identical input")
	}
	if len(a) != 16 {
		t.Errorf("computeDocHash length=%d, want 16", len(a))
	}
	if computeDocHash("different") == a {
		t.Error("computeDocHash should differ for different input")
	}
}
