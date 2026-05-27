package perception

import (
	"testing"
)

func TestTaxonomyEngine_SetClient(t *testing.T) {
	e, err := NewTaxonomyEngine()
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	e.SetClient(nil)
	if e.client != nil {
		t.Errorf("expected client to be nil")
	}
}

func TestTaxonomyEngine_StopWorker(t *testing.T) {
	e, err := NewTaxonomyEngine()
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	// Calling StopWorker on a stopped or unstarted engine shouldn't panic.
	// Second + third invocations exercise the sync.Once idempotency guard
	// added for Bug #17 — without it close(quit) would panic on the 2nd call.
	e.StopWorker()
	e.StopWorker()
	e.StopWorker()
}

func TestTaxonomyEngine_EnsureDefaults(t *testing.T) {
	e, err := NewTaxonomyEngine()
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	err = e.EnsureDefaults()
	if err == nil {
		t.Errorf("expected error when EnsureDefaults is called without a store")
	}
}

func TestTaxonomyEngine_SetWorkspaceAndQueue(t *testing.T) {
	e, _ := NewTaxonomyEngine()
	e.SetWorkspace("/tmp")
	if !e.HasWorkspace() {
		t.Errorf("expected true")
	}
	e.QueueForLearning(nil)
}

func TestLearnedCorpusStore_Search(t *testing.T) {
	s, _ := NewLearnedCorpusStore(nil, 4, nil)
	// Add dummy entry
	s.entries = append(s.entries, CorpusEntry{TextContent: "test", Verb: "/test"})
	s.embeddings["test"] = []float32{1.0, 0.0, 0.0, 0.0}
	
	res, err := s.Search([]float32{1.0, 0.0, 0.0, 0.0}, 5)
	if err != nil {
		t.Errorf("expected no error, got %v", err)
	}
	if len(res) != 1 {
		t.Errorf("expected 1 result, got %d", len(res))
	}
}

