package core

// SetVirtualStore attaches a VirtualStore so virtual predicates can be resolved.
func (k *RealKernel) SetVirtualStore(vs *VirtualStore) {
	k.mu.Lock()
	defer k.mu.Unlock()
	k.virtualStore = vs
	k.wrapStoreLocked()
}

// GetVirtualStore returns the currently attached VirtualStore (if any).
func (k *RealKernel) GetVirtualStore() *VirtualStore {
	k.mu.RLock()
	defer k.mu.RUnlock()
	return k.virtualStore
}

func (k *RealKernel) wrapStoreLocked() {
	if k.virtualStore == nil {
		if wrapped, ok := k.store.(*virtualFactStore); ok {
			k.store = wrapped.base
		}
		return
	}
	k.store = newVirtualFactStore(k.store, k.virtualStore)
}
