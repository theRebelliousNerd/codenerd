package perception

import (
	"codenerd/internal/mangle"
	"codenerd/internal/store"
	"fmt"
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
func (ts *TaxonomyStore) StoreLearnedExemplar(pattern, verb, target, constraint string, confidence float64) error {
	// We use "taxonomy" as the source so it gets picked up by LoadAllTaxonomyFacts.
	return ts.store.StoreFact("learned_exemplar", []interface{}{pattern, verb, target, constraint, confidence}, "taxonomy", 100)
}

// LoadAllTaxonomyFacts loads all taxonomy-related facts from the database.
func (ts *TaxonomyStore) LoadAllTaxonomyFacts() ([]mangle.Fact, error) {
	storedFacts, err := ts.store.LoadAllFacts("taxonomy")
	if err != nil {
		return nil, err
	}

	var facts []mangle.Fact
	for _, sf := range storedFacts {
		facts = append(facts, mangle.Fact{
			Predicate: sf.Predicate,
			Args:      sf.Args,
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
