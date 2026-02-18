package core

// SetVirtualStore attaches a VirtualStore so virtual predicates can be resolved
// via the native external predicate API.
//
// #17: No longer wraps the store. External predicates are registered at eval
// time via BuildExternalPredicates() + engine.WithExternalPredicates().
func (k *RealKernel) SetVirtualStore(vs *VirtualStore) {
	k.mu.Lock()
	defer k.mu.Unlock()
	k.virtualStore = vs
	// Mark policy dirty so external predicates get re-registered on next eval
	k.policyDirty = true
}

// GetVirtualStore returns the currently attached VirtualStore (if any).
func (k *RealKernel) GetVirtualStore() *VirtualStore {
	k.mu.RLock()
	defer k.mu.RUnlock()
	return k.virtualStore
}
