package core

import (
	"strings"
	"testing"
)

// The tool catalog is part of the system prompt, and by the epoch fingerprint's
// own design tool definitions are the FIRST thing in a provider's cacheable
// prefix — the fingerprint hashes tool definitions and then the system prompt,
// in wire order, precisely because a prefix cache is a prefix of the token
// stream.
//
// So a catalog whose section order is decided by Go's map iteration is not a
// cosmetic problem. The same tools produce different bytes on different runs,
// the prefix never matches, and the cache-write premium is paid on every
// request while no read ever earns it back. That is a real cost to save
// sorting half a dozen strings once per catalog build.
//
// It costs a second way, harder to price: two runs of a prompt that differ only
// in map order cannot be diffed, which is exactly what someone needs to do when
// a prompt change makes the agent worse.
func TestBuildToolCatalogIsByteStable(t *testing.T) {
	tr := NewToolRegistry(t.TempDir())

	// Several affinities, so there is a section order to get wrong. One
	// affinity would pass whatever the iteration did.
	for _, tool := range []*Tool{
		// Two affinity groups, several tools in each: the order can go wrong at
		// both levels, and a single tool per group would pass either way.
		{Name: "zebra_check", Command: "zebra", ShardAffinity: "/coder", Description: "reviews"},
		{Name: "alpha_build", Command: "alpha", ShardAffinity: "/coder", Description: "builds"},
		{Name: "mid_probe", Command: "mid", ShardAffinity: "/coder", Description: "tests"},
		{Name: "omni_write", Command: "omni", ShardAffinity: "/all", Description: "everything"},
		{Name: "any_read", Command: "any", ShardAffinity: "/all", Description: "reads"},
	} {
		if err := tr.RegisterToolWithInfo(tool); err != nil {
			t.Fatalf("RegisterToolWithInfo(%s): %v", tool.Name, err)
		}
	}

	first := tr.BuildToolCatalog("/coder")
	if !strings.Contains(first, "alpha_build") {
		t.Fatalf("catalog does not contain the registered tools, so this test proves nothing:\n%s", first)
	}

	// Go reseeds map iteration per range, so repeating the build is what
	// exercises the ordering. A single comparison would pass by luck often
	// enough to be useless.
	for i := 0; i < 50; i++ {
		if got := tr.BuildToolCatalog("/coder"); got != first {
			t.Fatalf("catalog differs between builds on iteration %d.\n"+
				"This text is part of the system prompt and sits in the provider's "+
				"cacheable prefix; bytes that move between runs can never be cached.\n"+
				"first:\n%s\n\ngot:\n%s", i, first, got)
		}
	}
}
