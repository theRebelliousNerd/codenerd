package core

import (
	"context"
	"fmt"
	"testing"

	"codenerd/internal/tools"
)

type fakeRecall struct {
	query         string
	offset, limit int
}

func (f *fakeRecall) Recall(_ context.Context, id string, offset, limit int) (string, error) {
	return fmt.Sprintf("recall %s %d %d", id, offset, limit), nil
}

func (f *fakeRecall) Search(_ context.Context, query string, offset, limit int) (string, error) {
	f.query, f.offset, f.limit = query, offset, limit
	return "found", nil
}

// A search answers records, not characters. Observed 2026-09-11: a model
// under the commit regime searched with limit 20000, the page size it had
// just used for a body, and was refused; it meant the store's maximum.
func TestRecallContextSearchClampsARecordLimit(t *testing.T) {
	recall := &fakeRecall{}
	ctx := tools.WithContextRecall(context.Background(), recall)
	out, err := RecallContextTool().Execute(ctx, map[string]any{"query": "tryLoadLearned", "limit": float64(20000)})
	if err != nil || out != "found" {
		t.Fatalf("search with an oversized limit: out=%q err=%v", out, err)
	}
	if recall.limit != maxContextSearchRecords || recall.query != "tryLoadLearned" {
		t.Fatalf("search received limit=%d query=%q, want the store maximum %d", recall.limit, recall.query, maxContextSearchRecords)
	}
	if _, err := RecallContextTool().Execute(ctx, map[string]any{"id": "abc", "limit": float64(20000)}); err != nil {
		t.Fatalf("a body page of 20000 characters is a page, not a record count: %v", err)
	}
}
