package prompt

import (
	"context"
	"fmt"
	"strings"

	"codenerd/internal/config"
	"codenerd/internal/types"
)

// promptCompiler is the compile entry point RoleCompiler drives.
type promptCompiler interface {
	Compile(ctx context.Context, cc *CompilationContext) (*CompilationResult, error)
}

// RoleCompiler compiles the system prompt of a non-shard role (a critic, a
// compressor, a rule generator) from the atoms that declare that role's shard
// type. It is how every model call that is not a persona turn gets its
// persona, instructions and output contract: from atoms, never from Go text.
//
// A role's atoms supersede the Piggyback output protocol with conflicts_with
// (see atoms/eval/delegation_judge.yaml), so a role whose reply Go parses
// carries one output contract.
type RoleCompiler struct {
	compiler promptCompiler
	jit      config.JITConfig
}

var _ types.RolePromptCompiler = (*RoleCompiler)(nil)

// NewRoleCompiler binds compiler and the JIT budgets roles compile under.
func NewRoleCompiler(compiler promptCompiler, jit config.JITConfig) (*RoleCompiler, error) {
	if compiler == nil {
		return nil, fmt.Errorf("%w: no prompt compiler", types.ErrRolePromptUnavailable)
	}
	return &RoleCompiler{compiler: compiler, jit: jit}, nil
}

// CompileRolePrompt compiles req.Role's system prompt. A compile failure or an
// empty prompt is an error wrapping types.ErrRolePromptUnavailable.
func (r *RoleCompiler) CompileRolePrompt(ctx context.Context, req types.RolePromptRequest) (string, error) {
	if r == nil || r.compiler == nil {
		return "", fmt.Errorf("%w: no prompt compiler", types.ErrRolePromptUnavailable)
	}
	role := strings.Trim(strings.TrimSpace(req.Role), "/")
	if role == "" {
		return "", fmt.Errorf("%w: no role named", types.ErrRolePromptUnavailable)
	}
	cc := NewCompilationContext()
	cc.ShardType = "/" + role
	cc.ShardName = role
	cc.OperationalMode = "/active"
	if intent := strings.Trim(strings.TrimSpace(req.Intent), "/"); intent != "" {
		cc.IntentVerb = "/" + intent
	}
	cc.SemanticQuery = req.SemanticQuery
	if r.jit.TokenBudget > 0 {
		cc.TokenBudget = r.jit.TokenBudget
	}
	if r.jit.ReservedTokens > 0 {
		cc.ReservedTokens = r.jit.ReservedTokens
	}
	if r.jit.ReservedTokensFallbackRatio > 0 {
		cc.ReservedTokensFallbackRatio = r.jit.ReservedTokensFallbackRatio
	}
	if r.jit.SemanticTopK > 0 {
		cc.SemanticTopK = r.jit.SemanticTopK
	}
	compiled, err := r.compiler.Compile(ctx, cc)
	if err != nil {
		return "", fmt.Errorf("%w: compile the %s prompt: %w", types.ErrRolePromptUnavailable, role, err)
	}
	if compiled == nil || strings.TrimSpace(compiled.Prompt) == "" {
		return "", fmt.Errorf("%w: the %s prompt compiled empty", types.ErrRolePromptUnavailable, role)
	}
	return compiled.Prompt, nil
}
