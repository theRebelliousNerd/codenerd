package core

import (
	"codenerd/internal/logging"

	"github.com/google/mangle/ast"
	"github.com/google/mangle/factstore"
)

var virtualPredicateHandlers = map[string]struct{}{
	"query_learned":         {},
	"query_session":         {},
	"recall_similar":        {},
	"query_knowledge_graph": {},
	"query_activations":     {},
	"has_learned":           {},
	"query_traces":          {},
	"query_trace_stats":     {},
}

type virtualFactStore struct {
	base    factstore.FactStore
	virtual *VirtualStore
}

func newVirtualFactStore(base factstore.FactStore, virtual *VirtualStore) factstore.FactStore {
	if virtual == nil {
		return base
	}
	if existing, ok := base.(*virtualFactStore); ok {
		existing.virtual = virtual
		return existing
	}
	vfs := &virtualFactStore{base: base}
	vfs.virtual = virtual
	return vfs
}

func (vfs *virtualFactStore) Add(atom ast.Atom) bool {
	return vfs.base.Add(atom)
}

func (vfs *virtualFactStore) Merge(other factstore.ReadOnlyFactStore) {
	vfs.base.Merge(other)
}

func (vfs *virtualFactStore) Contains(atom ast.Atom) bool {
	return vfs.base.Contains(atom)
}

func (vfs *virtualFactStore) ListPredicates() []ast.PredicateSym {
	return vfs.base.ListPredicates()
}

func (vfs *virtualFactStore) EstimateFactCount() int {
	return vfs.base.EstimateFactCount()
}

func (vfs *virtualFactStore) GetFacts(query ast.Atom, fn func(ast.Atom) error) error {
	if vfs.virtual != nil {
		if _, ok := virtualPredicateHandlers[query.Predicate.Symbol]; ok {
			atoms, err := vfs.virtual.Get(query)
			if err != nil {
				// Log at Error level for visibility (was Warn).
				// Virtual predicate failures can cause silent logic bugs.
				logging.Get(logging.CategoryVirtualStore).Error(
					"Virtual predicate %s query failed (returning empty): %v",
					query.Predicate.Symbol, err)
				// Continue to base store - this is intentional fallback.
				// Consider returning error if virtual predicates are critical.
			} else {
				for _, atom := range atoms {
					if !factstore.Matches(query.Args, atom.Args) {
						continue
					}
					if err := fn(atom); err != nil {
						return err
					}
				}
			}
		}
	}
	return vfs.base.GetFacts(query, fn)
}
