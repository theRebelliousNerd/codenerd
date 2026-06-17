package world

import (
	"testing"
)

func TestWorldPredicateSet(t *testing.T) {
	set := WorldPredicateSet()

	if set == nil {
		t.Fatal("WorldPredicateSet() returned nil, expected a map")
	}

	if len(set) != len(WorldPredicates) {
		t.Errorf("WorldPredicateSet() returned map of length %d, expected %d", len(set), len(WorldPredicates))
	}

	for _, p := range WorldPredicates {
		if _, ok := set[p]; !ok {
			t.Errorf("WorldPredicateSet() missing expected key %q", p)
		}
	}
}
