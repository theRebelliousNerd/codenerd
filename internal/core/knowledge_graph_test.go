package core

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"codenerd/internal/store"
)

// TestKnowledgeGraphHydration verifies that knowledge graph entries are
// properly loaded from LocalStore and hydrated into the Mangle kernel.
func TestKnowledgeGraphHydration(t *testing.T) {
	// Create temporary database
	tmpDir := t.TempDir()
	dbPath := filepath.Join(tmpDir, "test_knowledge.db")

	// Initialize LocalStore
	db, err := store.NewLocalStore(dbPath)
	if err != nil {
		t.Fatalf("Failed to create LocalStore: %v", err)
	}
	defer db.Close()

	// Add some knowledge graph entries
	testLinks := []struct {
		entityA  string
		relation string
		entityB  string
		weight   float64
	}{
		{"Go", "used_in", "codeNERD", 1.0},
		{"Mangle", "is_a", "Datalog_engine", 0.9},
		{"codeNERD", "implements", "neuro_symbolic_architecture", 0.95},
		{"Datalog", "enables", "logical_reasoning", 0.85},
	}

	for _, link := range testLinks {
		err := db.StoreLink(link.entityA, link.relation, link.entityB, link.weight, nil)
		if err != nil {
			t.Fatalf("Failed to store link %s->%s->%s: %v", link.entityA, link.relation, link.entityB, err)
		}
	}

	// Create kernel and VirtualStore
	kernel := NewRealKernel()
	vs := NewVirtualStore(nil)
	vs.SetLocalDB(db)
	vs.SetKernel(kernel)

	// Hydrate knowledge graph
	ctx := context.Background()
	count, err := vs.HydrateKnowledgeGraph(ctx)
	if err != nil {
		t.Fatalf("Failed to hydrate knowledge graph: %v", err)
	}

	// Verify count matches what we inserted
	if count != len(testLinks) {
		t.Errorf("Expected %d facts hydrated, got %d", len(testLinks), count)
	}

	// Initialize kernel to evaluate facts
	if err := kernel.Evaluate(); err != nil {
		t.Fatalf("Failed to evaluate kernel: %v", err)
	}

	// Query the kernel for knowledge_link facts
	facts, err := kernel.Query("knowledge_link")
	if err != nil {
		t.Fatalf("Failed to query knowledge_link: %v", err)
	}

	// Verify we got the expected facts
	if len(facts) != len(testLinks) {
		t.Errorf("Expected %d knowledge_link facts, got %d", len(testLinks), len(facts))
	}

	// Verify fact structure
	for _, fact := range facts {
		if fact.Predicate != "knowledge_link" {
			t.Errorf("Expected predicate 'knowledge_link', got '%s'", fact.Predicate)
		}
		if len(fact.Args) != 3 {
			t.Errorf("Expected 3 arguments for knowledge_link, got %d", len(fact.Args))
		}
	}

	// Test integration with HydrateLearnings (should include knowledge graph)
	count2, err := vs.HydrateLearnings(ctx)
	if err != nil {
		t.Fatalf("Failed to hydrate learnings: %v", err)
	}

	// Count should include at least the knowledge graph links
	if count2 < len(testLinks) {
		t.Errorf("HydrateLearnings should include knowledge graph (%d links), got %d total facts", len(testLinks), count2)
	}
}

// TestKnowledgeGraphEmptyDatabase verifies graceful handling of empty database.
func TestKnowledgeGraphEmptyDatabase(t *testing.T) {
	// Create temporary database
	tmpDir := t.TempDir()
	dbPath := filepath.Join(tmpDir, "test_empty.db")

	// Initialize LocalStore
	db, err := store.NewLocalStore(dbPath)
	if err != nil {
		t.Fatalf("Failed to create LocalStore: %v", err)
	}
	defer db.Close()

	// Create kernel and VirtualStore
	kernel := NewRealKernel()
	vs := NewVirtualStore(nil)
	vs.SetLocalDB(db)
	vs.SetKernel(kernel)

	// Hydrate empty knowledge graph
	ctx := context.Background()
	count, err := vs.HydrateKnowledgeGraph(ctx)
	if err != nil {
		t.Fatalf("Failed to hydrate empty knowledge graph: %v", err)
	}

	// Should succeed with 0 facts
	if count != 0 {
		t.Errorf("Expected 0 facts from empty database, got %d", count)
	}
}

// TestKnowledgeGraphNoDatabase verifies graceful handling when no database is configured.
func TestKnowledgeGraphNoDatabase(t *testing.T) {
	kernel := NewRealKernel()
	vs := NewVirtualStore(nil)
	vs.SetKernel(kernel)
	// Don't set LocalDB

	ctx := context.Background()
	count, err := vs.HydrateKnowledgeGraph(ctx)

	// Should succeed with 0 facts when no DB
	if err != nil {
		t.Errorf("Expected no error when DB is nil, got: %v", err)
	}
	if count != 0 {
		t.Errorf("Expected 0 facts when DB is nil, got %d", count)
	}
}

func init() {
	// Ensure test cleanup
	_ = os.RemoveAll("test_knowledge.db")
	_ = os.RemoveAll("test_empty.db")
}
