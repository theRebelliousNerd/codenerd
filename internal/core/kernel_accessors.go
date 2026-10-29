package core

import "codeberg.org/TauCeti/mangle-go/analysis"

// GetBaseFacts returns the raw EDB facts loaded into the kernel.
// This is useful for debugging and proof tree generation.
func (k *RealKernel) GetBaseFacts() []Fact {
	if k == nil {
		return nil
	}
	k.mu.RLock()
	defer k.mu.RUnlock()
	// Return a copy to be safe
	facts := make([]Fact, len(k.facts))
	copy(facts, k.facts)
	return facts
}

// GetProgramInfo returns the analyzed program info.
func (k *RealKernel) GetProgramInfo() *analysis.ProgramInfo {
	if k == nil {
		return nil
	}
	k.mu.RLock()
	defer k.mu.RUnlock()
	return k.programInfo
}
