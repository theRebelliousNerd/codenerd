package campaign

import (
	"testing"

	"codenerd/internal/core"
	"codenerd/internal/store"
	"codenerd/internal/types"
)

// The decomposer seeds the gatherer's knowledge-graph edges and the standing
// strategy (the goal as vision, reusable patterns and architecture hints as
// patterns) so policy/intelligence.mg can judge them; before this nothing
// asserted either predicate.
func TestSeedIntelligenceFacts_AssertsKnowledgeLinksAndStrategy(t *testing.T) {
	k, err := core.NewRealKernel()
	if err != nil {
		t.Fatal(err)
	}
	d := &Decomposer{kernel: k}
	d.seedIntelligenceFacts("camp1", "  keep core stable ", &IntelligenceReport{
		KnowledgeLinks:    []store.KnowledgeLink{{EntityA: "core.go", Relation: "documented_by", EntityB: "core.md", Weight: 0.8}},
		ReusablePatterns:  []string{"table tests"},
		ArchitectureHints: []string{"layered"},
	})

	links, err := k.Query("intelligence_knowledge_link")
	if err != nil || len(links) != 1 || types.ExtractString(links[0].Args[3]) != "80" {
		t.Fatalf("a 0.8 link weight must reach the kernel as 80: %v %v", links, err)
	}
	know, err := k.Query("intelligence_strategic_knowledge")
	if err != nil {
		t.Fatal(err)
	}
	got := map[string]string{}
	for _, f := range know {
		got[types.ExtractString(f.Args[2])] = types.ExtractString(f.Args[1]) + ":" + types.ExtractString(f.Args[3])
	}
	want := map[string]string{"keep core stable": "/vision:100", "table tests": "/pattern:70", "layered": "/pattern:60"}
	for content, v := range want {
		if got[content] != v {
			t.Errorf("strategic knowledge %q = %q, want %q (all: %v)", content, got[content], v, got)
		}
	}
}
