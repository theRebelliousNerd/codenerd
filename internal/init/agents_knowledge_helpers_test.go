package init

import (
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
		// Inherited and placeholder atoms must NOT count toward coverage.
		{Concept: "inherited:foo", Content: "rust ownership borrow"},
		{Concept: "domain/rust-ownership", Content: "rustexpert domain knowledge (1 of 1): rust ownership. When working on rust ownership, check the repository's own docs and code for project-specific rules."},
		{Concept: "agent_identity", Content: "You are rustexpert, a specialist agent for rust ownership."},
	}
	got := filterTopicsNeedingResearch(existing, []string{"go concurrency", "rust ownership"}, 2)
	// "go concurrency" has 2 genuine atoms (covered); "rust ownership" only has
	// inherited/placeholder atoms (not counted) so it still needs research.
	if len(got) != 1 || got[0] != "rust ownership" {
		t.Errorf("expected only 'rust ownership' to need research, got %v", got)
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
