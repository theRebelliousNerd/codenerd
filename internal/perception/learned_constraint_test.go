package perception

import (
	"context"
	"path/filepath"
	"strings"
	"testing"

	storepkg "codenerd/internal/store"
)

// A learned mapping's qualifier survives the durable round trip into the
// exemplar the classifier prompt shows: written through the perception
// adapter, the store closed and reopened, searched, and rendered by
// BuildPrompt. The adapter used to return Constraint "" whatever the row held;
// a mapping with no qualifier must still render empty.
func TestLearnedCorpusStore_SearchCarriesTheQualifierAfterReopen(t *testing.T) {
	const constraint = "only under internal/api, never the generated files"
	engine := &ctxEngine{dims: 4}
	dbPath := filepath.Join(t.TempDir(), "learned.db")
	ctx := context.Background()

	open := func() *LearnedCorpusStore {
		backend, err := storepkg.NewLearnedCorpusStoreWithContext(ctx, dbPath, engine)
		if err != nil {
			t.Fatalf("open backend: %v", err)
		}
		return &LearnedCorpusStore{backend: backend, dimensions: engine.dims, embeddings: map[string][]float32{}}
	}

	first := open()
	for _, entry := range []CorpusEntry{
		{TextContent: "regenerate the client", Verb: "/create", Target: "client", Constraint: constraint, Confidence: 0.9},
		{TextContent: "fix the flaky test", Verb: "/fix", Target: "test", Confidence: 0.9},
	} {
		if err := first.Add(ctx, entry, nil); err != nil {
			t.Fatalf("Add %q: %v", entry.TextContent, err)
		}
	}
	if err := first.Close(); err != nil {
		t.Fatal(err)
	}

	reopened := open()
	t.Cleanup(func() { _ = reopened.Close() })
	matches, err := reopened.Search([]float32{1, 0, 0, 0}, 5)
	if err != nil {
		t.Fatalf("Search after reopen: %v", err)
	}
	got := map[string]string{}
	for _, m := range matches {
		got[m.TextContent] = m.Constraint
	}
	if len(got) != 2 {
		t.Fatalf("matches = %+v, want both mappings", matches)
	}
	if got["regenerate the client"] != constraint {
		t.Errorf("qualified mapping Constraint = %q, want %q", got["regenerate the client"], constraint)
	}
	if got["fix the flaky test"] != "" {
		t.Errorf("unqualified mapping Constraint = %q, want empty", got["fix the flaky test"])
	}

	prompt := (&LLMTransducer{}).BuildPrompt("regenerate the client again", nil, matches, nil, "")
	if !strings.Contains(prompt, "Constraint="+constraint+" ") {
		t.Errorf("the exemplar the prompt shows lost the qualifier:\n%s", prompt)
	}
	if !strings.Contains(prompt, "Verb=/fix, Target=test, Constraint= ") {
		t.Errorf("the unqualified exemplar does not render an empty constraint:\n%s", prompt)
	}
}
