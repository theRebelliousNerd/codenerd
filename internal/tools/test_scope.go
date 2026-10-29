package tools

import "context"

// TestScope reports the workspace-relative Go packages ("./internal/session")
// the caller has written so far. It is read when a test tool is called with no
// packages, so it is a function, not a list: the answer is the write set at the
// moment of the call.
type TestScope func() []string

type testScopeKey struct{}

// WithTestScope returns a context in which a test tool called with no packages
// tests the scope's packages and not the whole module.
//
// The default used to be "./...". On a repository whose suite outlasts the
// tool's own time limit that call cannot succeed: it holds the turn for the
// full limit and returns a timeout. A documentation task paid that on
// 2026-09-21 (ledger P10). The whole module is still one argument away:
// packages ["./..."].
func WithTestScope(ctx context.Context, scope TestScope) context.Context {
	return context.WithValue(ctx, testScopeKey{}, scope)
}

// TestScopeFrom returns the scope ctx carries. ok is false outside a session
// (a direct tool call, a test), where no write set exists to scope by.
func TestScopeFrom(ctx context.Context) (scope TestScope, ok bool) {
	scope, ok = ctx.Value(testScopeKey{}).(TestScope)
	return scope, ok && scope != nil
}
