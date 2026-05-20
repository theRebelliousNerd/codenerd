package store

import (
	"testing"
)

func TestTraceStore_Reflection_Extra(t *testing.T) {
	ls, err := NewLocalStore(":memory:")
	if err != nil {
		t.Fatalf("Failed to create local store: %v", err)
	}
	defer ls.Close()
	s := ls.GetTraceStore()

	// 1. Setup mock trace
	trace := &ReasoningTrace{
		ID:           "trace1",
		SessionID:    "session1",
		ShardType:    "coder",
		TaskContext:  "fix bug",
		UserPrompt:   "hello",
		ErrorMessage: "failed to build",
		Success:      false,
	}
	err = s.StoreReasoningTrace(trace)
	if err != nil {
		t.Fatalf("StoreReasoningTrace failed: %v", err)
	}

	// 2. CountTraceEmbeddingBacklog
	count, err := s.CountTraceEmbeddingBacklog("model1", 128, "task1")
	if err != nil {
		t.Errorf("CountTraceEmbeddingBacklog failed: %v", err)
	}
	if count != 1 {
		t.Errorf("Expected 1 candidate backlog, got %d", count)
	}

	// 3. ListTraceEmbeddingCandidates
	cands, err := s.ListTraceEmbeddingCandidates(10, false, "model1", 128, "task1")
	if err != nil {
		t.Errorf("ListTraceEmbeddingCandidates failed: %v", err)
	}
	if len(cands) != 1 {
		t.Fatalf("Expected 1 candidate, got %d", len(cands))
	}
	if cands[0].ID != "trace1" {
		t.Errorf("Unexpected candidate ID: %s", cands[0].ID)
	}

	// List with skipSuccess (our trace is failure, so it should still be included)
	candsSkip, err := s.ListTraceEmbeddingCandidates(10, true, "model1", 128, "task1")
	if err != nil || len(candsSkip) != 1 {
		t.Errorf("ListTraceEmbeddingCandidates skipSuccess failed: %v", len(candsSkip))
	}

	// 4. ListAllTraceEmbeddingCandidates
	allCands, err := s.ListAllTraceEmbeddingCandidates(10, 0)
	if err != nil {
		t.Errorf("ListAllTraceEmbeddingCandidates failed: %v", err)
	}
	if len(allCands) != 1 {
		t.Errorf("Expected 1 total candidate, got %d", len(allCands))
	}

	// 5. Build trace descriptor
	desc := buildTraceDescriptor(cands[0])
	if desc == "" {
		t.Errorf("Expected non-empty descriptor")
	}

	// 6. ApplyTraceEmbeddingUpdates
	updates := []TraceEmbeddingUpdate{
		{
			ID:                "trace1",
			SummaryDescriptor: "desc",
			DescriptorVersion: 1, // traceDescriptorVersion = 1 typically
			DescriptorHash:    "hash",
			Embedding:         []byte{1, 2, 3},
			EmbeddingModelID:  "model1",
			EmbeddingDim:      128,
			EmbeddingTask:     "task1",
		},
	}
	err = s.ApplyTraceEmbeddingUpdates(updates)
	if err != nil {
		t.Errorf("ApplyTraceEmbeddingUpdates failed: %v", err)
	}

	// Count should now be 0 for this model/dim/task, wait if DescriptorVersion matches
	// Wait, we need to check traceDescriptorVersion which is 1. If it matches, count is 0.
	count, _ = s.CountTraceEmbeddingBacklog("model1", 128, "task1")
	if count != 0 {
		t.Errorf("Expected 0 candidate backlog after update, got %d", count)
	}

	// 7. truncateText
	trunc := truncateText("hello", 10)
	if trunc != "hello" {
		t.Errorf("Expected hello, got %s", trunc)
	}
	trunc2 := truncateText("hello world", 5)
	if trunc2 != "hello" {
		t.Errorf("Expected hello, got %s", trunc2)
	}
	trunc3 := truncateText("hello", 0)
	if trunc3 != "hello" {
		t.Errorf("Expected hello, got %s", trunc3)
	}
}
