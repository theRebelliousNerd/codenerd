package prompt

// NewCompilationContextWithBudget is a test convenience: a default context
// with a specific token budget. Production callers build the context once
// and set TokenBudget from the configured JIT budget directly.
func NewCompilationContextWithBudget(tokenBudget int) *CompilationContext {
	cc := NewCompilationContext()
	if tokenBudget > 0 {
		cc.TokenBudget = tokenBudget
	}
	return cc
}
