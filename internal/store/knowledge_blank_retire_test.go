package store

import (
	"context"
	"errors"
	"strings"
	"sync/atomic"
	"testing"
)

// countingBlankRefuser counts embed calls and refuses blank text, as the real
// embedders do.
func countingBlankRefuser(calls *atomic.Int64) *MockEmbeddingEngine {
	embed := func(_ context.Context, text string) ([]float32, error) {
		calls.Add(1)
		if strings.TrimSpace(text) == "" {
			return nil, errors.New("embedding: the text is empty or whitespace")
		}
		return []float32{0.1, 0.2, 0.3, 0.4}, nil
	}
	return &MockEmbeddingEngine{
		EmbedFunc:         embed,
		EmbedWithTaskFunc: func(ctx context.Context, text, _ string) ([]float32, error) { return embed(ctx, text) },
	}
}

// An ecosystem paragraph with zero bytes was sent to the embedder on every
// boot's backfill, refused, logged as a failure, and tried again on the next
// boot. A row with no text is kept but never embedded; rows with text still
// get their vectors.
func TestKnowledgeBackfill_ARowWithNoTextIsNeverEmbedded(t *testing.T) {
	s, err := NewLocalStore(":memory:")
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	for _, a := range [][2]string{{"ecosystem paragraph 888/1141 bytes=0", ""}, {"ecosystem paragraph 889/1141", "   \n"}, {"ecosystem paragraph 890/1141", "Run the tests before committing."}} {
		if err := s.StoreKnowledgeAtom(a[0], a[1], 0.95, "init/ecosystem"); err != nil {
			t.Fatal(err)
		}
	}
	var calls atomic.Int64
	s.SetEmbeddingEngine(countingBlankRefuser(&calls))

	added, err := s.BackfillKnowledgeAtomVectors(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if added != 1 || calls.Load() != 1 {
		t.Fatalf("backfill added %d vector(s) with %d embed call(s); want 1 and 1 (blank rows are not embedded)", added, calls.Load())
	}
	// A second boot has nothing left to do.
	if added, _ := s.BackfillKnowledgeAtomVectors(context.Background()); added != 0 || calls.Load() != 1 {
		t.Fatalf("a second backfill added %d and called the embedder %d time(s) in all", added, calls.Load())
	}
	// Storing a blank atom with an engine present does not embed it either.
	if err := s.StoreKnowledgeAtomWithEmbedding(context.Background(), "ecosystem paragraph 1/2", "", 0.95, "init/ecosystem"); err != nil {
		t.Fatal(err)
	}
	if calls.Load() != 1 {
		t.Fatalf("a blank atom was sent to the embedder when stored (%d calls)", calls.Load())
	}
}

// An import that owns a source retires the rows it no longer produces, and
// their vectors, and keeps the rest.
func TestRetireKnowledgeAtomsFromSource_KeepsOnlyWhatTheImportHas(t *testing.T) {
	s, err := NewLocalStore(":memory:")
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	s.SetEmbeddingEngine(&MockEmbeddingEngine{})
	ctx := context.Background()
	for _, a := range [][3]string{{"kept", "still here", "init/ecosystem"}, {"gone", "file deleted", "init/ecosystem"}, {"other", "another import", "init/document"}} {
		if err := s.StoreKnowledgeAtomWithEmbedding(ctx, a[0], a[1], 0.9, a[2]); err != nil {
			t.Fatal(err)
		}
	}
	n, err := s.RetireKnowledgeAtomsFromSource("init/ecosystem", map[string]bool{ComputeContentHash("kept", "still here"): true})
	if err != nil || n != 1 {
		t.Fatalf("retired %d (%v), want 1", n, err)
	}
	atoms, _ := s.GetAllKnowledgeAtoms()
	var concepts []string
	for _, a := range atoms {
		concepts = append(concepts, a.Concept)
	}
	if got := strings.Join(concepts, ","); got != "kept,other" && got != "other,kept" {
		t.Fatalf("atoms after retirement = %s, want kept and other", got)
	}
	var vectors int
	if err := s.db.QueryRow(`SELECT COUNT(*) FROM vectors WHERE json_extract(metadata, '$.concept') = 'gone'`).Scan(&vectors); err != nil {
		t.Fatal(err)
	}
	if vectors != 0 {
		t.Fatalf("the retired atom's vector survived (%d)", vectors)
	}
	if _, err := s.RetireKnowledgeAtomsFromSource("", nil); err == nil {
		t.Fatal("an empty source was accepted")
	}
}
