package main

import (
	"strings"
	"testing"

	"codenerd/internal/store"
)

// `nerd memory` printed the sum of every knowledge.db table under the label
// "Vector (Embeddings)". The vector line is the vectors table now, every other
// table is named, and the store's gauges are shown when it measured them.
func TestRenderStoreStats_NamesEachTierAndTheGauges(t *testing.T) {
	out := renderStoreStats(map[string]int64{
		"vectors":                        7,
		"reasoning_traces":               40,
		"prompt_atoms":                   900,
		store.StatVecIndexMissing:        2,
		store.StatReflectionTraceBacklog: 5,
	})
	for _, want := range []string{
		"Vector (Embeddings):   7 entries",
		"reasoning_traces:",
		"prompt_atoms:",
		"ANN drift:             2 embedded rows missing from vec_index",
		"Reflection backlog:    5 traces",
	} {
		if !strings.Contains(out, want) {
			t.Errorf("missing %q in:\n%s", want, out)
		}
	}
	if strings.Contains(out, "954") {
		t.Errorf("the tables were summed into one number:\n%s", out)
	}

	bare := renderStoreStats(map[string]int64{"vectors": 1})
	if strings.Contains(bare, "ANN drift") || strings.Contains(bare, "Reflection backlog") {
		t.Errorf("gauges the store did not measure were printed:\n%s", bare)
	}
}
