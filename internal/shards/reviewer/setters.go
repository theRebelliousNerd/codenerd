// Package reviewer implements the Reviewer ShardAgent per §7.0 Sharding.
// This file contains dependency injection setter methods.
package reviewer

import (
	"codenerd/internal/articulation"
	"codenerd/internal/core"
	"codenerd/internal/types"
)

// =============================================================================
// DEPENDENCY INJECTION
// =============================================================================

// SetLLMClient sets the LLM client for semantic analysis.
func (r *ReviewerShard) SetLLMClient(client types.LLMClient) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.llmClient = client
}

// SetSessionContext sets the session context (for dream mode, etc.).
func (r *ReviewerShard) SetSessionContext(ctx *core.SessionContext) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.config.SessionContext = ctx
}

// SetParentKernel sets the Mangle kernel for logic-driven review.
func (r *ReviewerShard) SetParentKernel(k types.Kernel) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if rk, ok := k.(*core.RealKernel); ok {
		r.kernel = rk
	} else {
		panic("ReviewerShard requires *core.RealKernel")
	}
}

// SetVirtualStore sets the virtual store for action routing.
func (r *ReviewerShard) SetVirtualStore(vs *core.VirtualStore) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.virtualStore = vs
}

// SetLearningStore sets the learning store for persistent autopoiesis.
func (r *ReviewerShard) SetLearningStore(ls core.LearningStore) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.learningStore = ls
	// Load existing patterns from store
	r.loadLearnedPatterns()
}

// SetHolographicProvider sets the holographic context provider for package-aware analysis.
func (r *ReviewerShard) SetHolographicProvider(hp HolographicProvider) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.holographicProvider = hp
}

// SetPromptAssembler sets the JIT prompt assembler for dynamic prompt compilation.
func (r *ReviewerShard) SetPromptAssembler(pa *articulation.PromptAssembler) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.promptAssembler = pa
}
