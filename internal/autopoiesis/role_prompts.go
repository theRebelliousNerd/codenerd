package autopoiesis

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"codenerd/internal/types"
)

// The autopoiesis model calls that are not tool code generation compile their
// system prompt from role atoms (internal/prompt/atoms/autopoiesis/, each
// shard_types: ["/<role>"]) through the session's types.RolePromptCompiler.
// The user message carries only the data the call is about.
const (
	roleComplexityAnalyzer  = "complexity_analyzer"
	rolePersistenceAnalyzer = "persistence_analyzer"
	roleAgentPromptWriter   = "agent_prompt_writer"
	roleToolNeedRefiner     = "tool_need_refiner"
	roleToolProfiler        = "tool_profiler"
	roleToolQualityAssessor = "tool_quality_assessor"
	roleToolTestWriter      = "tool_test_writer"
	rolePanicMaker          = "panic_maker"
)

// compileRolePrompt compiles role's system prompt. Intent /judge keeps the
// worker intent atoms out: these roles answer in a shape Go parses, and their
// atoms supersede the Piggyback protocol. No compiler, or a failed compile, is
// an error wrapping types.ErrRolePromptUnavailable; there is no other prompt.
func compileRolePrompt(ctx context.Context, roles types.RolePromptCompiler, role string) (string, error) {
	if roles == nil {
		return "", fmt.Errorf("%w: no role prompt compiler for the %s", types.ErrRolePromptUnavailable, role)
	}
	system, err := roles.CompileRolePrompt(ctx, types.RolePromptRequest{Role: role, Intent: "judge"})
	if err != nil {
		if errors.Is(err, types.ErrRolePromptUnavailable) {
			return "", err
		}
		return "", fmt.Errorf("%w: %s: %w", types.ErrRolePromptUnavailable, role, err)
	}
	if strings.TrimSpace(system) == "" {
		return "", fmt.Errorf("%w: the %s prompt compiled empty", types.ErrRolePromptUnavailable, role)
	}
	return system, nil
}

// assembleOuroborosPrompt compiles a tool generation or refinement system
// prompt through the JIT assembler for the Ouroboros stage pc names. An
// assembler that is missing or not JIT-ready, or a failed assembly, is an
// error wrapping types.ErrRolePromptUnavailable.
func assembleOuroborosPrompt(ctx context.Context, assembler PromptAssembler, pc map[string]any) (string, error) {
	stage, _ := pc["ouroboros_stage"].(string)
	if assembler == nil || !assembler.JITReady() {
		return "", fmt.Errorf("%w: no JIT prompt assembler for the tool generator's %s stage", types.ErrRolePromptUnavailable, stage)
	}
	system, err := assembler.AssembleSystemPrompt(ctx, pc)
	if err != nil {
		return "", fmt.Errorf("%w: assemble the tool generator's %s prompt: %w", types.ErrRolePromptUnavailable, stage, err)
	}
	if strings.TrimSpace(system) == "" {
		return "", fmt.Errorf("%w: the tool generator's %s prompt assembled empty", types.ErrRolePromptUnavailable, stage)
	}
	return system, nil
}
