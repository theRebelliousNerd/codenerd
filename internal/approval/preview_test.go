package approval

import "testing"

// A decision preview is recomputed from its exact inputs every time: a cache
// hit that served another input's hunks would show a person one change and
// authorize another (internal/diff TestDiffImporters_AreDisplayOnlyOrDecisionSafe).
func TestPreviewEngine_NeverServesFromCache(t *testing.T) {
	before := previewEngine.Stats()
	for i := 0; i < 3; i++ {
		if _, err := BuildPreview(PreviewInput{Summary: "s", MaxBytes: 1024,
			File: &FileChange{Path: "same.txt", Before: []byte("identical\n"), Exists: true}}); err != nil {
			t.Fatal(err)
		}
	}
	after := previewEngine.Stats()
	if after.Hits != before.Hits {
		t.Fatalf("the decision preview engine served %d diffs from cache", after.Hits-before.Hits)
	}
	if after.Entries != 0 {
		t.Fatalf("the decision preview engine retained %d cache entries", after.Entries)
	}
}
