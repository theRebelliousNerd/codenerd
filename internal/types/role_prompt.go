package types

import (
	"context"
	"errors"
)

// ErrRolePromptUnavailable is returned when a role's system prompt cannot be
// compiled: no compiler was wired, or the compile failed or came back empty.
// Every model-facing role prompt is compiled from prompt atoms; there is no
// embedded text to fall back to, so the caller reports this error rather than
// sending a different prompt.
var ErrRolePromptUnavailable = errors.New("role system prompt unavailable")

// RolePromptRequest names the role whose system prompt is compiled.
type RolePromptRequest struct {
	// Role is the shard type the role's atoms answer to, without a leading
	// slash ("critic", "context_compressor"). Atoms for the role carry
	// shard_types: ["/<role>"], a regime dimension, so only those atoms and
	// the shared substrate reach the compile.
	Role string
	// Intent is the optional intent verb ("judge") that keeps worker intent
	// atoms out of a non-worker compile.
	Intent string
	// SemanticQuery steers retrieval of flesh atoms for the task at hand.
	SemanticQuery string
}

// RolePromptCompiler compiles a role's system prompt from prompt atoms. The
// JIT prompt compiler implements it (prompt.RoleCompiler); packages that
// cannot import internal/prompt, because it imports them, depend on this
// interface instead.
type RolePromptCompiler interface {
	CompileRolePrompt(ctx context.Context, req RolePromptRequest) (string, error)
}
