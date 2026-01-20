package perception

import (
	"testing"

	"codenerd/internal/store"
)

func TestTaxonomyStore_Integration(t *testing.T) {
	// 1. Init LocalStore
	localDB, err := store.NewLocalStore(":memory:")
	if err != nil {
		t.Fatalf("Failed to create local store: %v", err)
	}
	defer localDB.Close()

	// 2. Create TaxonomyStore
	ts := NewTaxonomyStore(localDB)

	// 3. Store definitions
	err = ts.StoreVerbDef("/test", "/category", "/shard", 10)
	if err != nil {
		t.Errorf("StoreVerbDef failed: %v", err)
	}

	err = ts.StoreVerbSynonym("/test", "testing")
	if err != nil {
		t.Errorf("StoreVerbSynonym failed: %v", err)
	}

	err = ts.StoreVerbPattern("/test", "^test.*")
	if err != nil {
		t.Errorf("StoreVerbPattern failed: %v", err)
	}

	// 4. Store learned exemplar (float conversion check)
	// 0.85 -> 85
	err = ts.StoreLearnedExemplar("pattern", "/verb", "/target", "/constraint", 0.85)
	if err != nil {
		t.Errorf("StoreLearnedExemplar failed: %v", err)
	}

	// 5. Load and Verify
	facts, err := ts.LoadAllTaxonomyFacts()
	if err != nil {
		t.Fatalf("LoadAllTaxonomyFacts failed: %v", err)
	}

	if len(facts) != 4 {
		t.Errorf("Expected 4 facts, got %d", len(facts))
	}

	// Check exemplar specifically for float conversion
	foundExemplar := false
	for _, f := range facts {
		if f.Predicate == "learned_exemplar" {
			foundExemplar = true
			if len(f.Args) < 5 {
				t.Errorf("Exemplar fact has wrong arg count: %d", len(f.Args))
				continue
			}
			// Confidence is the 5th arg (index 4)
			// It might be int, int64, or float64 depending on how Mangle unmarshals/stores logic
			// StoreFact uses []interface{}, but LoadAllFacts returns []store.StoredFact where Args is []interface{} loaded from JSON.
			// JSON numbers are float64 by default in Go.

			conf := f.Args[4]
			var confVal int64
			switch v := conf.(type) {
			case float64:
				confVal = int64(v)
			case int64:
				confVal = v
			case int:
				confVal = int64(v)
			default:
				t.Errorf("Unexpected type for confidence: %T", conf)
			}

			if confVal != 85 {
				t.Errorf("Expected confidence 85, got %d", confVal)
			}
		}
	}

	if !foundExemplar {
		t.Error("learned_exemplar fact not found")
	}
}
