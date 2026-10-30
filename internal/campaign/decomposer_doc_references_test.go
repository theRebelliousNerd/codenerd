package campaign

import (
	"os"
	"path/filepath"
	"testing"

	"codenerd/internal/core"
	"codenerd/internal/types"
)

// A document the goal's topics reach pulls in the pages it links to. The
// references were never asserted, so relevance stopped at the first page.
func TestDocReferenceFacts_FollowRelativeLinksInsideTheSet(t *testing.T) {
	dir := t.TempDir()
	write := func(rel, body string) FileMetadata {
		path := filepath.Join(dir, rel)
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
		return FileMetadata{Path: path, SizeBytes: int64(len(body))}
	}
	spec := write("spec.md", "See [the api](api/api.md#auth), [outside](../elsewhere.md), [web](https://x.test/a.md), [self](spec.md).\n")
	api := write("api/api.md", "Back to [the spec](../spec.md).\n")
	orphan := write("notes.md", "nothing\n")

	facts := docReferenceFacts([]FileMetadata{spec, api, orphan})
	got := map[string]bool{}
	for _, f := range facts {
		got[filepath.Base(f.Args[0].(string))+">"+filepath.Base(f.Args[1].(string))] = true
	}
	if !got["spec.md>api.md"] || !got["api.md>spec.md"] || len(got) != 2 {
		t.Errorf("edges = %v, want spec>api and api>spec only (not outside the set, the web, or itself)", got)
	}
}

func TestSelectionPolicy_RelevanceFlowsAcrossDocReferences(t *testing.T) {
	k, err := core.NewRealKernel()
	if err != nil {
		t.Fatal(err)
	}
	facts := []core.Fact{
		{Predicate: "doc_metadata", Args: []any{"c1", "spec.md", "/spec", int64(1), int64(1)}},
		{Predicate: "doc_metadata", Args: []any{"c1", "api.md", "/api_doc", int64(1), int64(1)}},
		{Predicate: "doc_metadata", Args: []any{"c1", "notes.md", "/readme", int64(1), int64(1)}},
		{Predicate: "doc_tag", Args: []any{"spec.md", "/auth"}},
		{Predicate: "goal_topic", Args: []any{"c1", "/auth"}},
		{Predicate: "doc_reference", Args: []any{"spec.md", "api.md"}},
	}
	if err := k.AssertBatch(facts); err != nil {
		t.Fatal(err)
	}
	rows, err := k.Query("is_relevant")
	if err != nil {
		t.Fatal(err)
	}
	got := map[string]bool{}
	for _, f := range rows {
		got[types.ExtractString(f.Args[0])] = true
	}
	if !got["spec.md"] || !got["api.md"] || got["notes.md"] {
		t.Errorf("relevant = %v, want spec (tagged) and api (linked from it), not notes", got)
	}
}
