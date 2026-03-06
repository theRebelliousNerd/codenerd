package perception

import (
	"codenerd/internal/mangle"
	"codenerd/internal/store"
	"encoding/json"
	"fmt"
	"math"
)

// TaxonomyStore handles persistence of taxonomy facts to the local SQLite database.
type TaxonomyStore struct {
	store *store.LocalStore
}

// NewTaxonomyStore creates a new taxonomy persistence wrapper.
func NewTaxonomyStore(s *store.LocalStore) *TaxonomyStore {
	return &TaxonomyStore{store: s}
}

// StoreVerbDef persists a verb definition.
func (ts *TaxonomyStore) StoreVerbDef(verb, category, shard string, priority int) error {
	return ts.store.StoreFact("verb_def", []interface{}{verb, category, shard, priority}, "taxonomy", 100)
}

// StoreVerbSynonym persists a verb synonym.
func (ts *TaxonomyStore) StoreVerbSynonym(verb, synonym string) error {
	return ts.store.StoreFact("verb_synonym", []interface{}{verb, synonym}, "taxonomy", 100)
}

// StoreVerbPattern persists a verb regex pattern.
func (ts *TaxonomyStore) StoreVerbPattern(verb, pattern string) error {
	return ts.store.StoreFact("verb_pattern", []interface{}{verb, pattern}, "taxonomy", 100)
}

// StoreLearnedExemplar persists a learned usage pattern to the knowledge graph.
// Note: confidence is 0.0-1.0 float, but stored as 0-100 integer for Mangle compatibility.
func (ts *TaxonomyStore) StoreLearnedExemplar(pattern, verb, target, constraint string, confidence float64) error {
	// Convert confidence from float (0.0-1.0) to int (0-100) for Mangle schema compatibility
	confInt := int64(confidence * 100)
	// We use "taxonomy" as the source so it gets picked up by LoadAllTaxonomyFacts.
	return ts.store.StoreFact("learned_exemplar", []interface{}{pattern, verb, target, constraint, confInt}, "taxonomy", 100)
}

// LoadAllTaxonomyFacts loads all taxonomy-related facts from the database.
func (ts *TaxonomyStore) LoadAllTaxonomyFacts() ([]mangle.Fact, error) {
	storedFacts, err := ts.store.LoadAllFacts("taxonomy")
	if err != nil {
		return nil, err
	}

	var facts []mangle.Fact
	for _, sf := range storedFacts {
		args := normalizeTaxonomyFactArgs(sf.Predicate, sf.Args)
		facts = append(facts, mangle.Fact{
			Predicate: sf.Predicate,
			Args:      args,
		})
	}
	return facts, nil
}

// HydrateEngine loads all persisted taxonomy facts into the given Mangle engine.
func (ts *TaxonomyStore) HydrateEngine(engine *mangle.Engine) error {
	facts, err := ts.LoadAllTaxonomyFacts()
	if err != nil {
		return fmt.Errorf("failed to load taxonomy facts: %w", err)
	}

	if len(facts) == 0 {
		return nil
	}

	// Batch insert for efficiency
	return engine.AddFacts(facts)
}

func normalizeTaxonomyFactArgs(predicate string, args []interface{}) []interface{} {
	if len(args) == 0 {
		return args
	}

	out := make([]interface{}, len(args))
	copy(out, args)

	switch predicate {
	case "verb_def":
		if len(out) > 3 {
			out[3] = normalizeWholeNumber(out[3])
		}
	case "learned_exemplar":
		if len(out) > 4 {
			out[4] = normalizeWholeNumber(out[4])
		}
	}

	return out
}

func normalizeWholeNumber(v interface{}) interface{} {
	switch n := v.(type) {
	case int:
		return int64(n)
	case int32:
		return int64(n)
	case int64:
		return n
	case float64:
		if math.Trunc(n) == n {
			return int64(n)
		}
		return n
	case json.Number:
		if i, err := n.Int64(); err == nil {
			return i
		}
		if f, err := n.Float64(); err == nil {
			if math.Trunc(f) == f {
				return int64(f)
			}
			return f
		}
	}
	return v
}
