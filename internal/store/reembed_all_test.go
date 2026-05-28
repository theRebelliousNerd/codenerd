package store

import (
	"context"
	"os"
	"path/filepath"
	"testing"
)

func TestReembedAllDBsForce(t *testing.T) {
	tempDir := t.TempDir()

	// Create a dummy DB
	dbPath := filepath.Join(tempDir, "test.db")
	store, err := NewLocalStore(dbPath)
	if err != nil {
		t.Fatalf("Failed to create LocalStore: %v", err)
	}

	// Insert some dummy data so re-embed actually has work
	_, err = store.db.Exec(`INSERT INTO prompt_atoms (atom_id, content, token_count, content_hash, category, embedding) VALUES ('atom1', 'text', 1, 'hash', 'cat', '[0.1, 0.2, 0.3, 0.4]')`)
	if err != nil {
		t.Fatalf("Failed to insert prompt atom: %v", err)
	}

	// Create a shard learning DB
	shardsDir := filepath.Join(tempDir, "shards")
	if err := os.MkdirAll(shardsDir, 0755); err != nil {
		t.Fatalf("Failed to create shards dir: %v", err)
	}
	learningStore, err := NewLearningStore(shardsDir)
	if err != nil {
		t.Fatalf("Failed to create LearningStore: %v", err)
	}
	learningStore.Save("coder", "test_pred", []any{"arg1"}, "campaign1")
	learningStore.Close()

	// Now run the force re-embed
	ctx := context.Background()
	mockEngine := &MockEmbeddingEngine{
		EmbedFunc: func(ctx context.Context, text string) ([]float32, error) {
			return []float32{0.1, 0.2, 0.3, 0.4}, nil
		},
		DimensionsFunc: func() int { return 4 },
	}
	store.SetEmbeddingEngine(mockEngine)
	store.Close()

	var msgs []string
	progress := func(msg string) {
		msgs = append(msgs, msg)
	}

	result, err := ReembedAllDBsForce(ctx, []string{tempDir}, mockEngine, progress)
	if err != nil {
		t.Fatalf("ReembedAllDBsForce failed: %v", err)
	}

	if result.DBCount == 0 {
		t.Errorf("Expected >0 DBCount, got %d", result.DBCount)
	}
	if result.AtomsDone == 0 {
		t.Errorf("Expected >0 AtomsDone, got %d", result.AtomsDone)
	}
	if result.LearningsDone == 0 {
		t.Errorf("Expected >0 LearningsDone, got %d", result.LearningsDone)
	}
	if len(msgs) == 0 {
		t.Errorf("Expected progress messages, got none")
	}

	// Test with nil engine
	_, err = ReembedAllDBsForce(ctx, []string{tempDir}, nil, nil)
	if err == nil {
		t.Errorf("Expected error when engine is nil")
	}
}
