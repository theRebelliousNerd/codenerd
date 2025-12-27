package prompt

import "sync"

// SimpleRegistry is an in-memory implementation of ConfigAtomProvider.
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

// GetAtom retrieves a ConfigAtom from the registry.
func (r *SimpleRegistry) GetAtom(intent string) (ConfigAtom, bool) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	atom, ok := r.atoms[intent]
	return atom, ok
}
