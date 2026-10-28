package context

import (
	"fmt"
	"math/rand"
	"sort"
	"testing"
	"time"

	"codenerd/internal/core"
)

func randomScored(n int, seed int64) []ScoredFact {
	r := rand.New(rand.NewSource(seed))
	out := make([]ScoredFact, n)
	for i := range out {
		out[i] = ScoredFact{
			Fact:  core.Fact{Predicate: fmt.Sprintf("p%d", r.Intn(20)), Args: []any{fmt.Sprintf("arg%d", r.Intn(n)), int64(r.Intn(5))}},
			Score: float64(r.Intn(10)),
		}
	}
	return out
}

// The cached-key sort orders exactly as the comparator that rendered both
// facts on every comparison did.
func TestSortScoredFactsDescMatchesTheRenderingComparator(t *testing.T) {
	got := randomScored(5000, 7)
	want := append([]ScoredFact(nil), got...)
	sort.SliceStable(want, func(i, j int) bool {
		if want[i].Score != want[j].Score {
			return want[i].Score > want[j].Score
		}
		return want[i].Fact.String() < want[j].Fact.String()
	})
	sortScoredFactsDesc(got)
	for i := range got {
		if got[i].Fact.String() != want[i].Fact.String() || got[i].Score != want[i].Score {
			t.Fatalf("order differs at %d: got %s (%v), want %s (%v)", i, got[i].Fact, got[i].Score, want[i].Fact, want[i].Score)
		}
	}
}

func TestSortScoredFactsDescTiming(t *testing.T) {
	a := randomScored(50000, 11)
	b := append([]ScoredFact(nil), a...)
	start := time.Now()
	sort.SliceStable(b, func(i, j int) bool {
		if b[i].Score != b[j].Score {
			return b[i].Score > b[j].Score
		}
		return b[i].Fact.String() < b[j].Fact.String()
	})
	old := time.Since(start)
	start = time.Now()
	sortScoredFactsDesc(a)
	t.Logf("50000 facts: rendering comparator %v, cached keys %v", old, time.Since(start))
}
