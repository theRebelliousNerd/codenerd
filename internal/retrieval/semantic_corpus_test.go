package retrieval

import (
	"fmt"
	"slices"
	"sort"
	"testing"
)

// Audit fixture 8: a relevant file past the corpus cutoff. The corpus was the
// first N paths in lexicographic order, so a file in a late directory was out
// of reach whatever the task. Spread over directories, every directory has a
// share.
func TestBoundedCorpus_ALateDirectoryIsNotShutOut(t *testing.T) {
	var paths []string
	for i := range 50 {
		paths = append(paths, fmt.Sprintf("a/f%02d.go", i))
	}
	for i := range 50 {
		paths = append(paths, fmt.Sprintf("b/f%02d.go", i))
	}
	paths = append(paths, "z/target.go")
	sort.Strings(paths)

	prefix := paths[:10]
	if slices.Contains(prefix, "z/target.go") {
		t.Fatal("precondition: the lexicographic prefix must miss z/target.go")
	}
	got := boundedCorpus(paths, 10)
	if len(got) != 10 {
		t.Fatalf("picked %d, want 10", len(got))
	}
	if !slices.Contains(got, "z/target.go") {
		t.Fatalf("z/target.go is outside the bounded corpus %v", got)
	}
	if !sort.StringsAreSorted(got) {
		t.Fatalf("corpus must be sorted for a stable sample: %v", got)
	}
	if again := boundedCorpus(paths, 10); !slices.Equal(got, again) {
		t.Fatalf("the sample is not deterministic: %v vs %v", got, again)
	}
}

func TestBoundedCorpus_UnderTheLimitIsUntouched(t *testing.T) {
	paths := []string{"a/x.go", "b/y.go"}
	if got := boundedCorpus(paths, 5); !slices.Equal(got, paths) {
		t.Fatalf("got %v", got)
	}
}
