package prompt

import "sync"

// SimpleRegistry is an in-memory ConfigAtomProvider test double. Production
// uses DefaultConfigAtomProvider (NewDefaultConfigAtomProvider) as the single
// intent -> tools/policies authority.
type SimpleRegistry struct {
	atoms map[string]ConfigAtom
	mu    sync.RWMutex
}

// NewSimpleRegistry creates a new SimpleRegistry.
func NewSimpleRegistry() *SimpleRegistry {
	return &SimpleRegistry{
		atoms: make(map[string]ConfigAtom),
	}
}

// Register adds a ConfigAtom to the registry.
func (r *SimpleRegistry) Register(intent string, atom ConfigAtom) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.atoms[intent] = atom
}

// GetAtom retrieves a ConfigAtom from the registry. The returned atom is a
// clone: callers routinely append to Tools, and sharing the registry's slice
// would corrupt every later lookup of the same intent.
func (r *SimpleRegistry) GetAtom(intent string) (ConfigAtom, bool) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	atom, ok := r.atoms[intent]
	if !ok {
		return atom, false
	}
	return atom.Clone(), true
}
