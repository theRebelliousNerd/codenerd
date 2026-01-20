package core

// GetBaseFacts returns the raw EDB facts loaded into the kernel.
// This is useful for debugging and proof tree generation.
func (k *RealKernel) GetBaseFacts() []Fact {
	k.mu.RLock()
	defer k.mu.RUnlock()
	// Return a copy to be safe
	facts := make([]Fact, len(k.facts))
	copy(facts, k.facts)
	return facts
}
