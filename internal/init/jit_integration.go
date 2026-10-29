// Package init - JIT Prompt Compiler Integration
// This file implements JIT prompt compilation for init phases.
package init

import (
	"context"
	"fmt"
	"slices"
	"strings"
	"time"

	"codenerd/internal/config"
	"codenerd/internal/core"
	"codenerd/internal/logging"
	"codenerd/internal/northstar"
	"codenerd/internal/perception"
	"codenerd/internal/prompt"
	"codenerd/internal/types"
)

// initJITKernelAdapter adapts *core.RealKernel to prompt.KernelQuerier.
//
// The init path historically constructed its JIT compiler with only an
// embedded corpus because *core.RealKernel does not nominally satisfy
// prompt.KernelQuerier: core.Fact (aliased from types.Fact) and prompt.Fact
// are structurally identical but distinct named types, and RealKernel's
// AssertBatch takes []Fact while the JIT interface takes []any (fact
// strings). This adapter bridges that gap without an import cycle
// (internal/system already has an equivalent adapter, but system imports
// init, so init cannot reuse it).
type initJITKernelAdapter struct {
	kernel *core.RealKernel
}

var (
	_ prompt.KernelContentFingerprinter = (*initJITKernelAdapter)(nil)
	_ prompt.KernelContentFingerprinter = (*initJITCompilationScope)(nil)
	_ prompt.KernelQuerier              = (*initJITKernelAdapter)(nil)
	_ prompt.KernelRetracter            = (*initJITKernelAdapter)(nil)
	_ prompt.KernelScopeProvider        = (*initJITKernelAdapter)(nil)
	_ prompt.KernelCompilationScope     = (*initJITCompilationScope)(nil)
)

// newInitJITKernelAdapter wraps a non-nil *core.RealKernel.
func newInitJITKernelAdapter(kernel *core.RealKernel) *initJITKernelAdapter {
	return &initJITKernelAdapter{kernel: kernel}
}

// ContentFingerprint names the kernel's complete content for the prompt
// compiler's cache identity (core.RealKernel.ContentFingerprint).
func (adapter *initJITKernelAdapter) ContentFingerprint() (string, error) {
	if adapter == nil || adapter.kernel == nil {
		return "", fmt.Errorf("init JIT kernel adapter has nil kernel")
	}
	return adapter.kernel.ContentFingerprint()
}

// Query converts []core.Fact to []prompt.Fact.
func (a *initJITKernelAdapter) Query(predicate string) ([]prompt.Fact, error) {
	if a == nil || a.kernel == nil {
		return nil, fmt.Errorf("init JIT kernel adapter has nil kernel")
	}
	facts, err := a.kernel.Query(predicate)
	if err != nil {
		return nil, err
	}
	result := make([]prompt.Fact, len(facts))
	for i, f := range facts {
		result[i] = prompt.Fact{
			Predicate: f.Predicate,
			Args:      f.Args,
		}
	}
	return result, nil
}

// AssertBatch accepts JIT fact strings (e.g. `compile_context(...)`) plus
// prompt.Fact / core.Fact values, normalizes them to core.Fact, and loads
// them into the kernel.
func (a *initJITKernelAdapter) AssertBatch(facts []any) error {
	if a == nil || a.kernel == nil {
		return fmt.Errorf("init JIT kernel adapter has nil kernel")
	}
	if len(facts) == 0 {
		return nil
	}
	coreFacts := make([]core.Fact, 0, len(facts))
	for _, f := range facts {
		switch v := f.(type) {
		case string:
			input := strings.TrimSpace(v)
			if input == "" || input == "." {
				continue
			}
			// core.ParseFactString appends its own trailing ".", so strip
			// any caller-supplied dot first to avoid a doubled "..".
			input = strings.TrimSuffix(input, ".")
			parsed, err := core.ParseFactString(input)
			if err != nil {
				return fmt.Errorf("init JIT adapter: failed to parse fact %q: %w", v, err)
			}
			coreFacts = append(coreFacts, parsed)
		case prompt.Fact:
			coreFacts = append(coreFacts, core.Fact{
				Predicate: v.Predicate,
				Args:      v.Args,
			})
		case core.Fact:
			coreFacts = append(coreFacts, v)
		default:
			return fmt.Errorf("init JIT adapter: unsupported fact type %T", f)
		}
	}
	if len(coreFacts) == 0 {
		return nil
	}
	return a.kernel.LoadFacts(coreFacts)
}

// Retract removes all facts for a predicate. The JIT compiler uses this on
// compatibility adapters to clean up ephemeral compile_context facts when a
// scoped clone is unavailable.
func (a *initJITKernelAdapter) Retract(predicate string) error {
	if a == nil || a.kernel == nil {
		return fmt.Errorf("cannot retract %q from nil init JIT kernel", predicate)
	}
	return a.kernel.Retract(predicate)
}

// initJITCompilationScope is an isolated kernel view for one prompt
// compilation. It owns a Clone of the init kernel so selector assertions
// never mutate the live executive kernel.
type initJITCompilationScope struct {
	*initJITKernelAdapter
}

// Close discards the scoped clone. All compile_context/selector facts die
// with the final adapter reference.
func (s *initJITCompilationScope) Close() error {
	if s != nil {
		s.initJITKernelAdapter = nil
	}
	return nil
}

// NewCompilationScope snapshots the init kernel for one JIT prompt
// compilation, isolating concurrent compiles from each other and from the
// live kernel.
func (a *initJITKernelAdapter) NewCompilationScope() (prompt.KernelCompilationScope, error) {
	if a == nil || a.kernel == nil {
		return nil, fmt.Errorf("cannot create init JIT compilation scope from nil kernel")
	}
	return &initJITCompilationScope{
		initJITKernelAdapter: newInitJITKernelAdapter(a.kernel.Clone()),
	}, nil
}

// verifyJITKernelLoaded checks that the init kernel booted and can serve
// queries. It is called before wiring the kernel into the JIT compiler, so a
// regressed init path fails loudly: rule-based skeleton selection needs a
// kernel, and there is no static prompt to compile without one.
func (i *Initializer) verifyJITKernelLoaded(phase string) error {
	if i == nil || i.kernel == nil {
		return fmt.Errorf("init kernel is nil (phase %q)", phase)
	}
	if !i.kernel.IsInitialized() {
		return fmt.Errorf("init kernel is not initialized (phase %q)", phase)
	}
	// Probe query: any Declared predicate works. compile_context is asserted
	// by every JIT compile, so it exercises the full query path (lazy eval,
	// program info, stratification) without requiring seed facts.
	if _, err := i.kernel.Query("compile_context"); err != nil {
		return fmt.Errorf("init kernel probe query failed (phase %q): %w", phase, err)
	}
	return nil
}

// initAtomPrefix is the corpus slice init compiles from
// (atoms/init/researcher_guidance.yaml).
const initAtomPrefix = "init/researcher/"

// initMissionAtom is the persona every init compile must carry.
const initMissionAtom = "init/researcher/mission"

// compileInitSystemPrompt compiles the system prompt for one init phase, for
// the client route will send it to: the compile carries that client's provider
// and model, so pinned atoms match the model that reads them, and its input cap
// bounds the budget. The task steers retrieval only; it is the user message,
// never part of the system prompt. A compile that cannot run, or that comes
// back without the init persona, is an error wrapping
// types.ErrRolePromptUnavailable: there is no static prompt to send instead.
func (i *Initializer) compileInitSystemPrompt(ctx context.Context, route llmRoute, phase, task string, profile *ProjectProfile) (string, error) {
	jitCompiler, err := i.createJITCompiler()
	if err != nil {
		return "", fmt.Errorf("%w: init phase %s: create JIT compiler: %w", types.ErrRolePromptUnavailable, phase, err)
	}
	cc := initCompilationContext(route, phase, task, profile)
	result, err := jitCompiler.Compile(ctx, cc)
	if err != nil {
		return "", fmt.Errorf("%w: init phase %s: compile: %w", types.ErrRolePromptUnavailable, phase, err)
	}
	if result == nil || strings.TrimSpace(result.Prompt) == "" {
		return "", fmt.Errorf("%w: init phase %s compiled an empty prompt", types.ErrRolePromptUnavailable, phase)
	}
	if !slices.ContainsFunc(result.IncludedAtoms, func(atom *prompt.PromptAtom) bool { return atom.ID == initMissionAtom }) {
		return "", fmt.Errorf("%w: init phase %s compiled without %s (is the phase in its init_phases?)", types.ErrRolePromptUnavailable, phase, initMissionAtom)
	}
	logging.Boot("JIT compiled prompt for init phase %s (%s client %s): %d bytes, %d atoms",
		phase, route.slot, formatLLMIdentity(route.provider, route.model), len(result.Prompt), result.AtomsIncluded)
	return result.Prompt, nil
}

// createJITCompiler creates the init JIT prompt compiler over the
// init/researcher/* atoms of the embedded corpus, on the initializer's kernel.
// It is created on first use and cached.
func (i *Initializer) createJITCompiler() (*prompt.JITPromptCompiler, error) {
	i.jitMu.Lock()
	defer i.jitMu.Unlock()
	if i.jitClosed {
		return nil, fmt.Errorf("initializer is closed")
	}
	if i.jitCompiler != nil {
		return i.jitCompiler, nil
	}
	if err := i.verifyJITKernelLoaded("createJITCompiler"); err != nil {
		return nil, err
	}
	embedded, err := prompt.LoadEmbeddedCorpus()
	if err != nil {
		return nil, fmt.Errorf("load embedded prompt corpus: %w", err)
	}
	var atoms []*prompt.PromptAtom
	for _, atom := range embedded.All() {
		if strings.HasPrefix(atom.ID, initAtomPrefix) {
			atoms = append(atoms, atom)
		}
	}
	if len(atoms) == 0 {
		return nil, fmt.Errorf("the embedded corpus has no %s* atoms", initAtomPrefix)
	}

	// The initializer's live Mangle kernel, through the adapter, is what
	// rule-based skeleton selection (selector.go runSelection /
	// querySkeletonAtoms) queries.
	compiler, err := prompt.NewJITPromptCompiler(
		prompt.WithEmbeddedCorpus(prompt.NewEmbeddedCorpus(atoms)),
		prompt.WithKernel(newInitJITKernelAdapter(i.kernel)),
		prompt.WithWorkspace(i.config.Workspace),
	)
	if err != nil {
		return nil, fmt.Errorf("failed to create JIT compiler: %w", err)
	}

	i.jitCompiler = compiler
	return compiler, nil
}

// initCall is one init model request.
type initCall struct {
	// slot picks the client the request is sent to.
	slot llmSlot
	// phase selects the init atoms the JIT compile includes, and labels the
	// attempt in the metrics.
	phase string
	// task is the request's own instruction.
	task    string
	profile *ProjectProfile
	// system, when set, is a system prompt the caller compiled from its own
	// corpus for the same client (northstar derivation does); init does not
	// compile another.
	system string
	// query marks a request that is a search query, not a completion: no
	// system prompt is compiled for it.
	query bool
}

// withJITPrompt sends one init model request. It is the chokepoint every init
// call crosses: it resolves the client the call's slot routes to, compiles the
// system prompt for that client, bounds the request by InitConfig.Timeout
// (zero: llm_timeouts.per_call_timeout, read when the call is made), and
// records one outcome per provider attempt against the serving client. The
// handler is given the routed client and the system prompt; it sends
// call.task as the user message. A prompt that cannot be compiled is an error
// and no provider attempt.
func (i *Initializer) withJITPrompt(ctx context.Context, call initCall, handler func(ctx context.Context, client perception.LLMClient, systemPrompt string) (string, error)) (string, error) {
	if ctx == nil {
		ctx = context.Background()
	}
	route := i.route(call.slot)
	if route.client == nil {
		return "", fmt.Errorf("init phase %s: no LLM client is configured", call.phase)
	}
	systemPrompt := call.system
	if strings.TrimSpace(systemPrompt) == "" && !call.query {
		compiled, err := i.compileInitSystemPrompt(ctx, route, call.phase, call.task, call.profile)
		if err != nil {
			return "", err
		}
		systemPrompt = compiled
	}

	// Fast-path: parent already done. Skip the provider call entirely; a
	// skipped call is not a provider attempt so it is not recorded.
	if ctxErr := ctx.Err(); ctxErr != nil {
		return "", ctxErr
	}
	callCtx, cancel := initRequestContext(ctx, i.config.Timeout)
	defer cancel()

	// Providers must honor cancellation. Keep their lifetime and usage
	// accounting joined to this call rather than returning while a detached
	// request runs.
	response, callErr := handler(callCtx, route.client, systemPrompt)
	i.recordLLMCall(route, call.phase, callErr)
	return response, callErr
}

// initRequestContext bounds one model request. timeout is InitConfig.Timeout;
// zero or less reads llm_timeouts.per_call_timeout now, so the configured value
// that is current when the request is made applies, and a non-positive
// configured value adds no clock of its own (the caller's context still ends
// the request).
func initRequestContext(parent context.Context, timeout time.Duration) (context.Context, context.CancelFunc) {
	if timeout <= 0 {
		timeout = config.GetLLMTimeouts().PerCallTimeout
	}
	if timeout <= 0 {
		return context.WithCancel(parent)
	}
	return context.WithTimeout(parent, timeout)
}

// initCompilationContext is the CompilationContext for one init phase,
// compiled for the client route sends the prompt to.
func initCompilationContext(route llmRoute, phase, task string, profile *ProjectProfile) *prompt.CompilationContext {
	cc := prompt.NewCompilationContext()

	// Name the model that will read this prompt. Provider and model are
	// fail-closed regime dimensions in jit_compiler.mg: left empty they block
	// every pinned atom, including a planner-pinned overlay.
	cc.WithProviderModel(route.provider, route.model)

	// Set shard context (init uses researcher)
	cc.ShardType = "/researcher"
	cc.ShardID = "init_researcher"

	// Set init phase
	cc.InitPhase = "/" + phase

	// Set operational mode (init is always active)
	cc.OperationalMode = "/active"

	// Set language context if available
	if profile != nil {
		switch profile.Language {
		case "go", "Go", "golang":
			cc.Language = "/go"
		case "python", "Python":
			cc.Language = "/python"
		case "typescript", "TypeScript":
			cc.Language = "/typescript"
		case "javascript", "JavaScript":
			cc.Language = "/javascript"
		case "rust", "Rust":
			cc.Language = "/rust"
		}

		// Set framework if available
		if profile.Framework != "" {
			// Convert framework to selector format
			cc.Frameworks = []string{"/" + strings.ToLower(profile.Framework)}
		}
	}

	// A share of the enforced window, bounded by the serving client's own
	// input cap (a planner or worker slot's max_input_tokens). Reserved is a
	// tenth of the budget: a fixed 10000 exceeded the whole budget once a
	// capped slot or a small window floored it at config.MinPromptTokens, and
	// Validate then failed every init compile.
	cc.TokenBudget, cc.ReservedTokens = northstar.CompileTokenBudget(route.window)

	// Use task as semantic query for vector search
	if task != "" {
		cc.SemanticQuery = task
	}

	return cc
}
