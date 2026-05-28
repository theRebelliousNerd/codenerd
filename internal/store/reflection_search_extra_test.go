package store

import (
	"os"
	"testing"
	"time"
)

func TestLocalStore_ReflectionSearch_Extra(t *testing.T) {
	ls, err := NewLocalStore(":memory:")
	if err != nil {
		t.Fatalf("Failed to create local store: %v", err)
	}
	defer ls.Close()

	// 1. Setup trace for Lexical search
	s := ls.GetTraceStore()
	trace := &ReasoningTrace{
		ID:        "trace1",
		SessionID: "session1",
		ShardType: "coder",
		Success:   true,
		CreatedAt: time.Now(),
	}
	err = s.StoreReasoningTrace(trace)
	if err != nil {
		t.Fatalf("StoreReasoningTrace failed: %v", err)
	}

	// Update descriptor
	updates := []TraceEmbeddingUpdate{
		{
			ID:                "trace1",
			SummaryDescriptor: "this is an amazing test descriptor for coding",
			DescriptorVersion: 1,
		},
	}
	s.ApplyTraceEmbeddingUpdates(updates)

	// RecallTracesLexical
	hits, err := ls.RecallTracesLexical("amazing descriptor", 5)
	if err != nil {
		t.Errorf("RecallTracesLexical failed: %v", err)
	}
	if len(hits) == 0 {
		t.Errorf("Expected hits, got 0")
	} else if hits[0].TraceID != "trace1" {
		t.Errorf("Expected trace1, got %s", hits[0].TraceID)
	}

	// RecallTracesByEmbedding (should fail with missing vec table)
	_, err = ls.RecallTracesByEmbedding([]float32{0.1, 0.2}, 5)
	if err == nil {
		t.Errorf("Expected error from RecallTracesByEmbedding with missing vec table")
	}

	if ls.vectorExt {
		err = ls.ensureTraceVecTable(4)
		if err != nil {
			t.Fatalf("ensureTraceVecTable failed: %v", err)
		}
		// Insert into reasoning_traces_vec
		queryBlob := encodeFloat32Slice([]float32{0.1, 0.2, 0.3, 0.4})
		_, err = ls.db.Exec("INSERT INTO reasoning_traces_vec (trace_id, embedding) VALUES (?, ?)", "trace1", queryBlob)
		if err != nil {
			t.Fatalf("Failed to insert into reasoning_traces_vec: %v", err)
		}

		hits, err := ls.RecallTracesByEmbedding([]float32{0.1, 0.2, 0.3, 0.4}, 5)
		if err != nil {
			t.Errorf("RecallTracesByEmbedding failed: %v", err)
		}
		if len(hits) == 0 {
			t.Errorf("Expected hits, got 0")
		}
	}

	// 2. Setup LearningStore for Lexical search
	tempDir, _ := os.MkdirTemp("", "learning_search_test")
	defer os.RemoveAll(tempDir)

	learnStore, _ := NewLearningStore(tempDir)
	defer learnStore.Close()

	// Add learning
	err = learnStore.Save("coder", "test_predicate", []any{"how to parse json properly with golang"}, "campaign1")
	if err != nil {
		t.Fatalf("Save failed: %v", err)
	}

	// Retrieve learning ID by listing candidates
	cands, err := learnStore.ListLearningEmbeddingCandidates("coder", 10, "", 0, "")
	if err != nil {
		t.Fatalf("ListLearningEmbeddingCandidates failed: %v", err)
	}
	if len(cands) == 0 {
		t.Fatalf("Expected 1 candidate, got 0")
	}

	// Update descriptor
	lUpdates := []LearningEmbeddingUpdate{
		{
			ID:             cands[0].ID,
			SemanticHandle: "how to parse json properly with golang",
			HandleVersion:  1,
		},
	}
	err = learnStore.ApplyLearningEmbeddingUpdates("coder", lUpdates)
	if err != nil {
		t.Fatalf("ApplyLearningEmbeddingUpdates failed: %v", err)
	}

	// RecallLearningsLexical
	lHits, err := learnStore.RecallLearningsLexical("parse json", 5)
	if err != nil {
		t.Errorf("RecallLearningsLexical failed: %v", err)
	}
	if len(lHits) == 0 {
		t.Errorf("Expected learning hits, got 0")
	}

	// RecallLearningsByEmbedding (returns nil or error with missing vec table)
	lVecHits, err := learnStore.RecallLearningsByEmbedding([]float32{0.1, 0.2}, 5)
	if err == nil && len(lVecHits) > 0 {
		t.Errorf("Expected error or no hits from RecallLearningsByEmbedding with missing vec table")
	}

	if ls.vectorExt {
		db, err := learnStore.getDB("coder")
		if err != nil {
			t.Fatalf("getDB failed: %v", err)
		}
		err = ensureLearningVecTable(db, 4)
		if err != nil {
			t.Fatalf("ensureLearningVecTable failed: %v", err)
		}

		// Insert into learnings_vec
		queryBlob := encodeFloat32Slice([]float32{0.1, 0.2, 0.3, 0.4})
		_, err = db.Exec("INSERT INTO learnings_vec (learning_id, embedding) VALUES (?, ?)", cands[0].ID, queryBlob)
		if err != nil {
			t.Fatalf("Failed to insert into learnings_vec: %v", err)
		}

		lHits2, err := learnStore.RecallLearningsByEmbedding([]float32{0.1, 0.2, 0.3, 0.4}, 5)
		if err != nil {
			t.Errorf("RecallLearningsByEmbedding failed: %v", err)
		}
		if len(lHits2) == 0 {
			t.Errorf("Expected learning hits by embedding, got 0")
		}
	}

	// Test extractKeywords directly via LexicalScore
	score := lexicalScore("apple banana cherry", []string{"banana", "date"})
	if score <= 0 {
		t.Errorf("Expected positive lexical score")
	}
}
