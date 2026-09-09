package prompt

import "codenerd/internal/logging"

// StrategyProvider supplies learned problem-solving strategies as prompt atoms
// for one compilation.
//
// It exists because the System Prompt Learning strategy database was, for its
// whole life, write-only. Strategies are seeded per problem type at boot and
// then refined from real failures by the evolution cycle — and nothing ever
// put one in front of a model. A knowledge base the agent cannot read is not
// knowledge; this is the seam that lets it read.
//
// The provider is asked once per compilation and must be cheap: it runs inside
// the prompt compiler's atom-collection phase, on the turn's critical path. It
// must return atoms already carrying their selectors, because they are fitted
// to the token budget by exactly the same scoring the other four sources go
// through — a strategy earns its place or gets dropped like anything else.
//
// Implemented by prompt_evolution.StrategyAtomProvider.
type StrategyProvider interface {
	StrategyAtoms(cc *CompilationContext) []*PromptAtom
}

// RegisterStrategyProvider installs the learned-strategy source. Passing nil
// removes it, which restores the pre-registration behaviour exactly: four
// sources instead of five.
func (c *JITPromptCompiler) RegisterStrategyProvider(p StrategyProvider) {
	c.dbMu.Lock()
	defer c.dbMu.Unlock()

	c.strategyProvider = p
	if p == nil {
		logging.JIT("Cleared strategy provider")
		return
	}
	logging.JIT("Registered learned-strategy provider")
}

// collectStrategyAtoms gathers the strategies that apply to this context.
//
// A provider that panics is contained here rather than taking the turn with
// it: the strategy source is the newest and least load-bearing of the five,
// and a prompt without its strategies is still a working prompt.
func (c *JITPromptCompiler) collectStrategyAtoms(cc *CompilationContext) (atoms []*PromptAtom) {
	c.dbMu.RLock()
	provider := c.strategyProvider
	c.dbMu.RUnlock()

	if provider == nil {
		return nil
	}

	defer func() {
		if r := recover(); r != nil {
			logging.Get(logging.CategoryJIT).Warn("Strategy provider panicked: %v; compiling without strategies", r)
			atoms = nil
		}
	}()
	return provider.StrategyAtoms(cc)
}
