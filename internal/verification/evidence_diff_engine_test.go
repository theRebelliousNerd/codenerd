package verification

import "testing"

// The judge decides a verdict on these diffs, so none may come from a cache
// whose keys are trusted rather than proven.
func TestEvidenceDiffEngine_NeverServesFromCache(t *testing.T) {
	before := evidenceDiffEngine.Stats()
	for i := 0; i < 3; i++ {
		if got := renderDiff("same.go", "a\nb\n", "a\nc\n"); got == "" {
			t.Fatal("renderDiff returned no diff for a real change")
		}
	}
	after := evidenceDiffEngine.Stats()
	if after.Hits != before.Hits {
		t.Fatalf("the evidence diff engine served %d diffs from cache", after.Hits-before.Hits)
	}
	if after.Entries != 0 {
		t.Fatalf("the evidence diff engine retained %d cache entries", after.Entries)
	}
}
