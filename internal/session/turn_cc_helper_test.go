package session

import (
	"context"

	"codenerd/internal/perception"
	"codenerd/internal/prompt"
)

// turnCC is the turn's compile context as the executor builds it
// (turnCompilationContext), for tests that read only the context.
func turnCC(e *Executor, ctx context.Context, intent perception.Intent) *prompt.CompilationContext {
	cc, _ := e.turnCompilationContext(ctx, intent)
	return cc
}
