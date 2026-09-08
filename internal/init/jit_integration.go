// Package init - JIT Prompt Compiler Integration
// This file implements JIT prompt compilation for init phases.
package init

import (
	"context"
	"fmt"
	"strings"

	"codenerd/internal/core"
	"codenerd/internal/logging"
	"codenerd/internal/prompt"
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
	_ prompt.KernelQuerier          = (*initJITKernelAdapter)(nil)
	_ prompt.KernelRetracter        = (*initJITKernelAdapter)(nil)
	_ prompt.KernelScopeProvider    = (*initJITKernelAdapter)(nil)
	_ prompt.KernelCompilationScope = (*initJITCompilationScope)(nil)
)

// newInitJITKernelAdapter wraps a non-nil *core.RealKernel.
func newInitJITKernelAdapter(kernel *core.RealKernel) *initJITKernelAdapter {
	return &initJITKernelAdapter{kernel: kernel}
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

// isExpertInitPhase reports whether an init phase initializes experts
// (Type 3 / persistent specialist agents). These are the phases where the
// JIT kernel must be loaded: rule-based skeleton selection degrades to
// fallback prompts when the kernel is missing.
func isExpertInitPhase(phase string) bool {
	switch strings.TrimPrefix(strings.ToLower(strings.TrimSpace(phase)), "/") {
	case "agents", "kb_agent", "kb_complete":
		return true
	default:
		return false
	}
}

// verifyJITKernelLoaded checks that the init kernel booted and can serve
// queries. It is called before wiring the kernel into the JIT compiler and
// again on the expert phases so a regressed init path fails loudly instead
// of silently falling back to static prompts.
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

// assembleJITPrompt attempts to use the JIT compiler to generate a prompt for an init phase.
// Falls back to a simple prompt if JIT compilation fails.
func (i *Initializer) assembleJITPrompt(ctx context.Context, phase, task string, profile *ProjectProfile) (string, error) {
	// Check if we have a JIT compiler available
	if i.config.LLMClient == nil {
		return "", fmt.Errorf("no LLM client available for prompt assembly")
	}

	// Try to create JIT compiler
	jitCompiler, err := i.createJITCompiler()
	if err != nil {
		logging.Boot("Failed to create JIT compiler: %v, using fallback", err)
		return i.buildFallbackPrompt(phase, task), nil
	}

	// Build compilation context for this phase
	cc := BuildInitCompilationContext(phase, task, profile)

	// Compile the prompt
	result, err := jitCompiler.Compile(ctx, cc)
	if err != nil {
		logging.Boot("JIT prompt compilation failed for phase %s: %v, using fallback", phase, err)
		return i.buildFallbackPrompt(phase, task), nil
	}

	logging.Boot("JIT compiled prompt for init phase %s: %d bytes, %d atoms", phase, len(result.Prompt), result.AtomsIncluded)

	// Combine compiled prompt with task
	fullPrompt := fmt.Sprintf("%s\n\nTask: %s", result.Prompt, task)
	return fullPrompt, nil
}

// createJITCompiler creates a JIT prompt compiler for init phases.
// This is a lazy initialization - we only create it when needed.
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
	// Create init-specific atoms
	// These are hardcoded for now - in production, these would come from an embedded corpus
	atoms := []*prompt.PromptAtom{
		prompt.NewPromptAtom(
			"init_analysis_guidance",
			prompt.CategoryInit,
			"When analyzing codebases during initialization, focus on: language detection, framework identification, dependency mapping, and architectural patterns. Provide concise 2-3 sentence summaries.",
		),
		prompt.NewPromptAtom(
			"init_profile_guidance",
			prompt.CategoryInit,
			"When generating project profiles, extract: project name, language, framework, build system, entry points, test directories, and architectural patterns. Structure as JSON-compatible data.",
		),
		prompt.NewPromptAtom(
			"init_facts_guidance",
			prompt.CategoryInit,
			"When generating Mangle facts, use proper syntax: predicates are lowercase_with_underscores, name constants start with /, strings are quoted, statements end with periods. Example: project_language(/go).",
		),
		prompt.NewPromptAtom(
			"init_kb_agent_guidance",
			prompt.CategoryInit,
			"When creating agent knowledge bases, research topics deeply and extract: core concepts, best practices, code examples, anti-patterns, and common gotchas. Aim for 20-50 quality atoms per agent.",
		),
		prompt.NewPromptAtom(
			"init_agents_guidance",
			prompt.CategoryInit,
			"When recommending agents, analyze project dependencies and architecture. Suggest language experts, framework experts, and domain specialists. Prioritize agents that provide high-value knowledge.",
		),
		prompt.NewPromptAtom(
			"researcher_core_mission",
			prompt.CategoryIdentity,
			"You are the ResearcherShard, a deep research specialist. Your purpose: gather knowledge from documentation, analyze codebases, and build knowledge bases for specialist agents. Prioritize accuracy, relevance, and conciseness.",
		),
	}

	// Configure atom selectors
	atoms[0].Subcategory = "init"
	atoms[0].InitPhases = []string{"/analysis"}
	atoms[0].ShardTypes = []string{"/researcher"}
	atoms[0].Priority = 80

	atoms[1].Subcategory = "init"
	atoms[1].InitPhases = []string{"/profile"}
	atoms[1].ShardTypes = []string{"/researcher"}
	atoms[1].Priority = 80

	atoms[2].Subcategory = "init"
	atoms[2].InitPhases = []string{"/facts"}
	atoms[2].ShardTypes = []string{"/researcher"}
	atoms[2].Priority = 90
	atoms[2].IsMandatory = true

	atoms[3].Subcategory = "init"
	atoms[3].InitPhases = []string{"/kb_agent"}
	atoms[3].ShardTypes = []string{"/researcher"}
	atoms[3].Priority = 85

	atoms[4].Subcategory = "init"
	atoms[4].InitPhases = []string{"/agents"}
	atoms[4].ShardTypes = []string{"/researcher"}
	atoms[4].Priority = 75

	atoms[5].Subcategory = "mission"
	atoms[5].ShardTypes = []string{"/researcher"}
	atoms[5].Priority = 100
	atoms[5].IsMandatory = true

	// Create embedded corpus
	corpus := prompt.NewEmbeddedCorpus(atoms)

	// Restore the JIT kernel in the init path: wire the initializer's live
	// Mangle kernel through the adapter so rule-based skeleton selection
	// (selector.go loadSkeletonAtoms) has a kernel to query. Without this,
	// every init compile runs without Mangle rules and silently degrades.
	opts := []prompt.CompilerOption{
		prompt.WithEmbeddedCorpus(corpus),
	}
	opts = append(opts, prompt.WithKernel(newInitJITKernelAdapter(i.kernel)))

	// Create JIT compiler with embedded corpus (+ kernel when available)
	compiler, err := prompt.NewJITPromptCompiler(opts...)
	if err != nil {
		return nil, fmt.Errorf("failed to create JIT compiler: %w", err)
	}

	i.jitCompiler = compiler
	return compiler, nil
}

// buildFallbackPrompt creates a simple fallback prompt when JIT is unavailable.
func (i *Initializer) buildFallbackPrompt(phase, task string) string {
	basePrompt := `You are the ResearcherShard, a deep research specialist for codeNERD.

Your purpose: gather knowledge from documentation, analyze codebases, and build knowledge bases for specialist agents.

Core principles:
- Prioritize accuracy over speed
- Extract concrete, actionable knowledge
- Provide concise summaries (2-3 sentences)
- Focus on what's most useful for an AI coding agent
`

	// Phase-specific guidance
	phaseGuidance := map[string]string{
		"analysis": `
When analyzing codebases:
- Detect language, framework, and build system
- Identify architectural patterns
- Map key dependencies
- Locate entry points and test directories
- Provide a concise 2-3 sentence summary
`,
		"profile": `
When generating project profiles:
- Extract project metadata (name, language, framework)
- Identify build system and architecture
- List key dependencies with versions
- Structure as JSON-compatible data
`,
		"facts": `
When generating Mangle facts:
- Use proper syntax: lowercase_predicates, /name_constants, "quoted strings", ending periods
- Example: project_language(/go).
- Include project identity, language, framework, patterns
`,
		"agents": `
When recommending agents:
- Analyze project dependencies and tech stack
- Suggest language experts (GoExpert, PythonExpert, etc.)
- Suggest framework experts (WebAPIExpert, FrontendExpert, etc.)
- Suggest domain specialists (SecurityAuditor, TestArchitect, etc.)
- Prioritize high-value knowledge sources
`,
		"kb_agent": `
When creating agent knowledge bases:
- Research topics deeply (20-50 atoms per agent)
- Extract: core concepts, best practices, code examples, anti-patterns
- Focus on what makes the agent valuable
- Maintain high quality (score >= 0.5)
`,
		"kb_complete": `
When completing knowledge base creation:
- Ensure comprehensive coverage of agent's domain
- Verify atom quality (no duplicates, high relevance)
- Provide summary of knowledge coverage
`,
	}

	guidance := phaseGuidance[phase]
	if guidance == "" {
		guidance = "\n(No specific phase guidance available)\n"
	}

	return fmt.Sprintf("%s%s\nTask: %s", basePrompt, guidance, task)
}

// withJITPrompt wraps an LLM call with JIT prompt compilation.
// This is a helper for init phases that need to make LLM calls.
func (i *Initializer) withJITPrompt(ctx context.Context, phase, task string, profile *ProjectProfile, handler func(ctx context.Context, prompt string) (string, error)) (string, error) {
	if ctx == nil {
		ctx = context.Background()
	}
	// Assemble prompt using JIT (with fallback)
	prompt, err := i.assembleJITPrompt(ctx, phase, task, profile)
	if err != nil {
		logging.Boot("Failed to assemble JIT prompt: %v", err)
		// Use fallback
		prompt = i.buildFallbackPrompt(phase, task)
	}

	// Fast-path: parent already done. Skip the provider call entirely; a
	// skipped call is not a provider attempt so it is not recorded. The
	// caller falls back to its static template on the returned ctx error.
	if ctxErr := ctx.Err(); ctxErr != nil {
		return "", ctxErr
	}

	// All init LLM calls pass this chokepoint, so record one outcome per actual
	// provider attempt. Prompt compilation failures use the local fallback and
	// are not provider attempts.
	//
	// Providers must honor cancellation. Keep their lifetime and usage accounting
	// joined to this call rather than returning while a detached request runs.
	response, callErr := handler(ctx, prompt)
	i.recordLLMCall(callErr)
	return response, callErr
}

// BuildInitCompilationContext creates a CompilationContext for an init phase.
// This can be used directly with the JIT compiler for advanced use cases.
func BuildInitCompilationContext(phase, task string, profile *ProjectProfile) *prompt.CompilationContext {
	cc := prompt.NewCompilationContext()

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

	// Set token budget (init prompts can be larger)
	cc.TokenBudget = 120000
	cc.ReservedTokens = 10000

	// Use task as semantic query for vector search
	if task != "" {
		cc.SemanticQuery = task
	}

	return cc
}
