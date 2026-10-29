package northstar

import (
	"context"
	"fmt"
	"strings"
	"sync"

	"codenerd/internal/core"
	"codenerd/internal/prompt"
	"codenerd/internal/types"
)

// =============================================================================
// ALIGNMENT PROMPT
// =============================================================================
//
// The guardian's system prompt is compiled by the JIT compiler from the atoms
// in internal/prompt/atoms/northstar/guardian_alignment.yaml: the role frame,
// the module-refinement rule (only when a module northstar governs the
// subject), the evaluation task, the SCORE/RESULT reply contract and the
// closing instruction. There is no Go copy to fall back to: a compile that
// fails or comes back without its atoms is an error, and the check is unknown.
//
// The project vision, the governing module's northstar, the subject and its
// context are data. They travel in the user message, never in the compiled
// prompt.

// Alignment check phases (northstar_phases in guardian_alignment.yaml). A
// module check is a plain check plus the module-refinement rule.
const (
	alignmentPhase       = "/alignment_check"
	moduleAlignmentPhase = "/module_alignment_check"
)

// guardianAtomPrefix is the corpus slice the guardian compiles from; no other
// atom (codeNERD identity, tool use, Piggyback) belongs in its prompt.
const guardianAtomPrefix = "northstar/guardian/"

// Atoms every alignment compile must carry; atomGuardianModuleRefinement is
// required too when the check governs a module.
const (
	atomGuardianRole             = "northstar/guardian/role"
	atomGuardianModuleRefinement = "northstar/guardian/module_refinement"
	atomGuardianTask             = "northstar/guardian/task"
	atomGuardianOutputContract   = "northstar/guardian/output_contract"
	atomGuardianUserInstruction  = "northstar/guardian/user_instruction"
)

var (
	alignmentCompilerOnce sync.Once
	alignmentCompiler     *prompt.JITPromptCompiler
	alignmentCompilerErr  error
)

// newNorthstarAtomCompiler builds a JIT compiler over the embedded atoms whose
// IDs start with prefix, on a private kernel.
func newNorthstarAtomCompiler(prefix string) (*prompt.JITPromptCompiler, error) {
	corpus, err := prompt.LoadEmbeddedCorpus()
	if err != nil {
		return nil, err
	}
	var atoms []*prompt.PromptAtom
	for _, atom := range corpus.All() {
		if strings.HasPrefix(atom.ID, prefix) {
			atoms = append(atoms, atom)
		}
	}
	if len(atoms) == 0 {
		return nil, fmt.Errorf("the embedded corpus has no %s* atoms", prefix)
	}
	kernel, err := core.NewRealKernel()
	if err != nil {
		return nil, err
	}
	return prompt.NewJITPromptCompiler(
		prompt.WithKernel(&northstarJITKernelAdapter{kernel: kernel}),
		prompt.WithEmbeddedCorpus(prompt.NewEmbeddedCorpus(atoms)),
	)
}

// compileAlignmentSystemPrompt compiles the guardian's system prompt. A
// failure wraps types.ErrRolePromptUnavailable.
func compileAlignmentSystemPrompt(ctx context.Context, refinesModule bool) (string, error) {
	alignmentCompilerOnce.Do(func() {
		alignmentCompiler, alignmentCompilerErr = newNorthstarAtomCompiler(guardianAtomPrefix)
	})
	if alignmentCompilerErr != nil {
		return "", fmt.Errorf("%w: build the alignment prompt compiler: %w", types.ErrRolePromptUnavailable, alignmentCompilerErr)
	}
	required := []string{atomGuardianRole, atomGuardianTask, atomGuardianOutputContract, atomGuardianUserInstruction}
	cc := prompt.NewCompilationContext()
	cc.NorthstarPhase = alignmentPhase
	if refinesModule {
		cc.NorthstarPhase = moduleAlignmentPhase
		required = append(required, atomGuardianModuleRefinement)
	}
	cc.ShardType = "/northstar_guardian"
	cc.OperationalMode = "/active"
	cc.TokenBudget, cc.ReservedTokens = CompileTokenBudget(0)
	res, err := alignmentCompiler.Compile(ctx, cc)
	if err != nil {
		return "", fmt.Errorf("%w: compile the alignment prompt: %w", types.ErrRolePromptUnavailable, err)
	}
	if res == nil || strings.TrimSpace(res.Prompt) == "" {
		return "", fmt.Errorf("%w: the alignment prompt compiled empty", types.ErrRolePromptUnavailable)
	}
	included := make(map[string]bool, len(res.IncludedAtoms))
	for _, atom := range res.IncludedAtoms {
		included[atom.ID] = true
	}
	for _, id := range required {
		if !included[id] {
			return "", fmt.Errorf("%w: the alignment prompt compiled without %s", types.ErrRolePromptUnavailable, id)
		}
	}
	if !refinesModule && included[atomGuardianModuleRefinement] {
		return "", fmt.Errorf("%w: a project-only alignment prompt compiled with %s", types.ErrRolePromptUnavailable, atomGuardianModuleRefinement)
	}
	return res.Prompt, nil
}
