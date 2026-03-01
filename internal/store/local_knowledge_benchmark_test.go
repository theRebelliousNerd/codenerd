package store

import (
	"fmt"
	"os"
	"testing"
)

func BenchmarkStoreKnowledgeAtom(b *testing.B) {
	// Create a temporary database file
	tmpfile, err := os.CreateTemp("", "knowledge_benchmark_*.db")
	if err != nil {
		b.Fatal(err)
	}
	dbPath := tmpfile.Name()
	tmpfile.Close()
	defer os.Remove(dbPath)

	store, err := NewLocalStore(dbPath)
	if err != nil {
		b.Fatal(err)
	}
	defer store.Close()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		concept := fmt.Sprintf("concept_%d", i)
		content := fmt.Sprintf("This is the content for concept %d", i)
		err := store.StoreKnowledgeAtom(concept, content, 1.0)
		if err != nil {
			b.Fatalf("StoreKnowledgeAtom failed: %v", err)
		}
	}
}
