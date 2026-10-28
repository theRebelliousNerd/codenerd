package northstar

import (
	"context"
	"fmt"
	"strings"
	"sync"

	"codenerd/internal/broker"
	"codenerd/internal/core"
	"codenerd/internal/prompt"
)

// CompileTokenBudget is the prompt budget shared by the derive phase compile
// and init's own JIT compile. The 0.75 share and 120000 fallback are the
// existing PromptBudget call, not a new tunable; inputWindow is the serving
// client's input cap (broker.InputWindow; 0 when it reports none), which bounds
// the share when it is smaller than the enforced window. Reserved is a tenth of
// the budget so Validate's reserved < budget holds.
func CompileTokenBudget(inputWindow int) (budget, reserved int) {
	budget = broker.Default().PromptBudgetFor(0.75, 120000, inputWindow)
	reserved = budget / 10
	if reserved >= budget {
		reserved = budget / 2
	}
	return budget, reserved
}

// northstarJITKernelAdapter adapts *core.RealKernel to prompt.KernelQuerier.
// prompt.Fact and core.Fact are distinct named types even though core.Fact
// aliases types.Fact, so the adapter copies fields. A scoped clone keeps
// selector facts off the cached kernel.
type northstarJITKernelAdapter struct {
	kernel *core.RealKernel
}

type northstarJITCompilationScope struct {
	*northstarJITKernelAdapter
}

var (
	_ prompt.KernelFactSnapshotQuerier = (*northstarJITKernelAdapter)(nil)
	_ prompt.KernelFactSnapshotQuerier = (*northstarJITCompilationScope)(nil)
	_ prompt.KernelQuerier             = (*northstarJITKernelAdapter)(nil)
	_ prompt.KernelRetracter           = (*northstarJITKernelAdapter)(nil)
	_ prompt.KernelScopeProvider       = (*northstarJITKernelAdapter)(nil)
	_ prompt.KernelCompilationScope    = (*northstarJITCompilationScope)(nil)
)

func (adapter *northstarJITKernelAdapter) QueryAll() (map[string][]prompt.Fact, error) {
	if adapter == nil || adapter.kernel == nil {
		return nil, fmt.Errorf("northstar JIT kernel adapter has nil kernel")
	}
	facts, err := adapter.kernel.QueryAll()
	if err != nil {
		return nil, err
	}
	result := make(map[string][]prompt.Fact, len(facts))
	for predicate, rows := range facts {
		converted := make([]prompt.Fact, len(rows))
		for index, fact := range rows {
			arguments := make([]any, len(fact.Args))
			copy(arguments, fact.Args)
			converted[index] = prompt.Fact{Predicate: fact.Predicate, Args: arguments}
		}
		result[predicate] = converted
	}
	return result, nil
}

func (a *northstarJITKernelAdapter) Query(predicate string) ([]prompt.Fact, error) {
	if a == nil || a.kernel == nil {
		return nil, fmt.Errorf("northstar JIT kernel adapter has nil kernel")
	}
	facts, err := a.kernel.Query(predicate)
	if err != nil {
		return nil, err
	}
	out := make([]prompt.Fact, len(facts))
	for i, fact := range facts {
		out[i] = prompt.Fact{Predicate: fact.Predicate, Args: fact.Args}
	}
	return out, nil
}

func (a *northstarJITKernelAdapter) AssertBatch(facts []any) error {
	if a == nil || a.kernel == nil {
		return fmt.Errorf("northstar JIT kernel adapter has nil kernel")
	}
	if len(facts) == 0 {
		return nil
	}
	rows := make([]core.Fact, 0, len(facts))
	for _, fact := range facts {
		switch v := fact.(type) {
		case string:
			input := strings.TrimSpace(v)
			if input == "" || input == "." {
				continue
			}
			input = strings.TrimSuffix(input, ".")
			parsed, err := core.ParseFactString(input)
			if err != nil {
				return fmt.Errorf("northstar JIT adapter: failed to parse fact %q: %w", v, err)
			}
			rows = append(rows, parsed)
		case prompt.Fact:
			rows = append(rows, core.Fact{Predicate: v.Predicate, Args: v.Args})
		case core.Fact:
			rows = append(rows, v)
		default:
			return fmt.Errorf("northstar JIT adapter: unsupported fact type %T", fact)
		}
	}
	if len(rows) == 0 {
		return nil
	}
	return a.kernel.LoadFacts(rows)
}

func (a *northstarJITKernelAdapter) Retract(predicate string) error {
	if a == nil || a.kernel == nil {
		return fmt.Errorf("cannot retract %q from nil northstar JIT kernel", predicate)
	}
	return a.kernel.Retract(predicate)
}

func (s *northstarJITCompilationScope) Close() error {
	if s != nil {
		s.northstarJITKernelAdapter = nil
	}
	return nil
}

func (a *northstarJITKernelAdapter) NewCompilationScope() (prompt.KernelCompilationScope, error) {
	if a == nil || a.kernel == nil {
		return nil, fmt.Errorf("cannot create northstar JIT compilation scope from nil kernel")
	}
	return &northstarJITCompilationScope{
		northstarJITKernelAdapter: &northstarJITKernelAdapter{kernel: a.kernel.Clone()},
	}, nil
}

var (
	derivePromptOnce sync.Once
	deriveCompiler   *prompt.JITPromptCompiler
	derivePromptErr  error
)

// deriveIdentityAtom is the skeleton atom shared by the three derive phases.
const deriveIdentityAtom = "northstar/derive/identity"

// PhaseTarget names the model a derive prompt is compiled for. Provider and
// Model pin atoms the way a session turn does: jit_compiler.mg treats both as
// fail-closed dimensions, so an empty pair admits no provider- or model-pinned
// atom. InputWindow is the serving client's input cap in tokens
// (broker.InputWindow; 0 when it reports none) and bounds the prompt budget.
type PhaseTarget struct {
	Provider    string
	Model       string
	InputWindow int
}

// PhasePromptFor returns the PhasePrompt that compiles each derive phase for
// target from the embedded northstar/derive atoms. Init's own JIT compiler
// replaces that corpus with the init atoms, so derive phases cannot go through
// it. The kernel is cached; a boot failure stays cached for the process.
func PhasePromptFor(target PhaseTarget) PhasePrompt {
	return func(ctx context.Context, phase string) (string, error) {
		return compilePhasePrompt(ctx, phase, target)
	}
}

func compilePhasePrompt(ctx context.Context, phase string, target PhaseTarget) (string, error) {
	if err := ctx.Err(); err != nil {
		return "", err
	}
	phase = strings.TrimPrefix(strings.TrimSpace(phase), "/")
	if !oneOf(phase, "derive_classify", "derive_vision", "derive_requirements") {
		return "", fmt.Errorf("unknown northstar derivation phase %q", phase)
	}
	derivePromptOnce.Do(func() {
		deriveCompiler, derivePromptErr = newNorthstarAtomCompiler("northstar/derive/")
	})
	if derivePromptErr != nil {
		return "", derivePromptErr
	}
	budget, reserved := CompileTokenBudget(target.InputWindow)
	cc := prompt.NewCompilationContext()
	cc.WithProviderModel(target.Provider, target.Model)
	cc.NorthstarPhase = "/" + phase
	cc.ShardType = "/researcher"
	cc.OperationalMode = "/planning"
	cc.IntentVerb = "/research"
	cc.TokenBudget = budget
	cc.ReservedTokens = reserved
	res, err := deriveCompiler.Compile(ctx, cc)
	if err != nil {
		return "", err
	}
	if res == nil || strings.TrimSpace(res.Prompt) == "" {
		return "", fmt.Errorf("northstar phase %q compiled an empty prompt", phase)
	}
	expected := "northstar/derive/" + strings.TrimPrefix(phase, "derive_")
	found := false
	// The identity atom is the skeleton every derive phase shares, and the
	// phase atom's own pieces (expected + "/...") are its protocol; any other
	// atom would mix one phase's protocol into another.
	for _, atom := range res.IncludedAtoms {
		if atom.ID == expected {
			found = true
		} else if atom.ID != deriveIdentityAtom && !strings.HasPrefix(atom.ID, expected+"/") {
			return "", fmt.Errorf("northstar phase %q selected unrelated atom %q", phase, atom.ID)
		}
	}
	if found {
		return res.Prompt, nil
	}
	return "", fmt.Errorf("northstar phase %q compiled without required atom %q", phase, expected)
}
