package articulation

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"

	"codenerd/internal/logging"
	"codenerd/internal/prompt"
	"codenerd/internal/types"
)

// =============================================================================
// PROMPT ASSEMBLER - Dynamic System Prompt Generation from Kernel State
// =============================================================================
// The PromptAssembler queries the Mangle kernel for context atoms and shard
// templates, then composes fully dynamic system prompts for shard execution.
// This enables kernel-driven prompt injection and context selection.

// KernelQuerier defines the interface for querying the Mangle kernel.
// This abstracts the kernel dependency for testability.
type KernelQuerier interface {
	Query(predicate string) ([]types.Fact, error)
}

// PromptContext holds all the context needed to assemble a system prompt.
type PromptContext struct {
	ShardID    string                  // Unique identifier for this shard instance
	ShardType  string                  // Type: coder, tester, reviewer, researcher
	SessionCtx *types.SessionContext   // Session context from the Blackboard
	UserIntent *types.StructuredIntent // Parsed user intent from perception
	CampaignID string                  // Active campaign ID (if any)
	// SemanticQuery overrides the default semantic search query for JIT selection.
	SemanticQuery string
	// SemanticTopK overrides the default semantic search top-K for JIT selection.
	SemanticTopK int
}

// defaultUseJIT is set from the USE_JIT_PROMPTS environment variable.
// JIT is enabled by default; set USE_JIT_PROMPTS=false to disable.
var defaultUseJIT = os.Getenv("USE_JIT_PROMPTS") != "false"

// PromptAssembler queries the kernel and assembles dynamic system prompts.
// It supports an optional JIT compiler for context-aware prompt generation.
type PromptAssembler struct {
	kernel KernelQuerier

	// JIT compiler integration (Phase 5)
	jitCompiler *prompt.JITPromptCompiler // Optional JIT compiler
	useJIT      bool                      // Feature flag for JIT usage
	mu          sync.RWMutex              // Protects JIT fields

	// JIT budget overrides (optional; defaults set by prompt.NewCompilationContext)
	tokenBudget                 int
	reservedTokens              int
	semanticTopK                int
	reservedTokensFallbackRatio int
}

// NewPromptAssembler creates a PromptAssembler with the given kernel querier.
// Returns an error if kernel is nil.
// JIT compilation is enabled by default; set USE_JIT_PROMPTS=false to disable.
func NewPromptAssembler(kernel KernelQuerier) (*PromptAssembler, error) {
	if kernel == nil {
		return nil, fmt.Errorf("kernel querier is required")
	}
	return &PromptAssembler{
		kernel: kernel,
		useJIT: defaultUseJIT,
	}, nil
}

// NewPromptAssemblerWithJIT creates a PromptAssembler with JIT compilation enabled.
// The JIT compiler is used for context-aware prompt generation when available.
// Returns an error if kernel is nil.
func NewPromptAssemblerWithJIT(kernel KernelQuerier, jitCompiler *prompt.JITPromptCompiler) (*PromptAssembler, error) {
	if kernel == nil {
		return nil, fmt.Errorf("kernel querier is required")
	}

	pa := &PromptAssembler{
		kernel:      kernel,
		jitCompiler: jitCompiler,
		useJIT:      jitCompiler != nil, // Enable JIT if compiler is provided
	}

	if jitCompiler != nil {
		logging.Articulation("PromptAssembler initialized with JIT compiler")
	}

	return pa, nil
}

// toCompilationContext converts a PromptContext to a prompt.CompilationContext.
// This bridges the existing PromptContext structure with the JIT compiler's context.
func (pa *PromptAssembler) toCompilationContext(pc *PromptContext) *prompt.CompilationContext {
	cc := prompt.NewCompilationContext()

	stableShardID := func(instanceID, fallback string) string {
		instanceID = strings.TrimSpace(instanceID)
		if instanceID == "" {
			return fallback
		}
		lastDash := strings.LastIndex(instanceID, "-")
		if lastDash <= 0 || lastDash >= len(instanceID)-1 {
			return fallback
		}
		suffix := instanceID[lastDash+1:]
		for _, r := range suffix {
			if r < '0' || r > '9' {
				return fallback
			}
		}
		return instanceID[:lastDash]
	}

	// Set shard context
	cc.ShardType = "/" + pc.ShardType
	// ShardID must be the stable agent name to match registered shard DBs and atom tags.
	// pc.ShardID may be an ephemeral instance ID (e.g., coder-123), so keep it separately.
	cc.ShardID = stableShardID(pc.ShardID, pc.ShardType)
	cc.ShardInstanceID = pc.ShardID

	// Apply configured JIT budgets (if any)
	tokenBudget, reservedTokens, semanticTopK, reservedTokensFallbackRatio := pa.getBudgetConfig()
	if tokenBudget > 0 {
		cc.TokenBudget = tokenBudget
	}
	if reservedTokens > 0 {
		cc.ReservedTokens = reservedTokens
	}
	if semanticTopK > 0 {
		cc.SemanticTopK = semanticTopK
	}
	if reservedTokensFallbackRatio > 0 {
		cc.ReservedTokensFallbackRatio = reservedTokensFallbackRatio
	}
	switch pc.ShardType {
	case "legislator", "mangle_repair":
		// Keep Mangle system prompts focused to avoid massive context dumps.
		if cc.TokenBudget > 60000 {
			cc.TokenBudget = 60000
		}
		if cc.ReservedTokens > 4000 {
			cc.ReservedTokens = 4000
		}
		if cc.SemanticTopK > 10 {
			cc.SemanticTopK = 10
		}
	}

	normalizeTag := func(v string) string {
		v = strings.TrimSpace(v)
		if v == "" {
			return ""
		}
		if strings.HasPrefix(v, "/") {
			return v
		}
		return "/" + v
	}

	// Extract from SessionContext if available
	if pc.SessionCtx != nil {
		// Determine operational mode
		if pc.SessionCtx.DreamMode {
			cc.OperationalMode = "/dream"
		} else if pc.SessionCtx.TestState == "/failing" {
			cc.OperationalMode = "/tdd_repair"
		} else {
			cc.OperationalMode = "/active"
		}

		// Map test state
		cc.FailingTestCount = len(pc.SessionCtx.FailingTests)

		// Map diagnostics count
		cc.DiagnosticCount = len(pc.SessionCtx.CurrentDiagnostics)

		// Map campaign context
		if pc.SessionCtx.CampaignActive {
			cc.CampaignPhase = pc.SessionCtx.CampaignPhase
			cc.CampaignName = pc.SessionCtx.CampaignGoal
		}

		// Determine if this is a large refactor from impacted files
		if len(pc.SessionCtx.ImpactedFiles) > 10 {
			cc.IsLargeRefactor = true
		}

		// Check for high churn from git context
		if pc.SessionCtx.GitUnstagedCount > 20 {
			cc.IsHighChurn = true
		}

		// Derive world model flags from session safety state.
		if len(pc.SessionCtx.SafetyWarnings) > 0 || len(pc.SessionCtx.BlockedActions) > 0 {
			cc.HasSecurityIssues = true
		}

		// Map optional contextual selectors from ExtraContext if present.
		if pc.SessionCtx.ExtraContext != nil {
			if v := pc.SessionCtx.ExtraContext["build_layer"]; v != "" {
				cc.BuildLayer = normalizeTag(v)
			}
			if v := pc.SessionCtx.ExtraContext["init_phase"]; v != "" {
				cc.InitPhase = normalizeTag(v)
			}
			if v := pc.SessionCtx.ExtraContext["northstar_phase"]; v != "" {
				cc.NorthstarPhase = normalizeTag(v)
			}
			if v := pc.SessionCtx.ExtraContext["ouroboros_stage"]; v != "" {
				cc.OuroborosStage = normalizeTag(v)
			}
			// Frameworks can be provided as comma-separated list or repeated keys.
			if v := pc.SessionCtx.ExtraContext["frameworks"]; v != "" {
				rawFws := strings.FieldsFunc(v, func(r rune) bool { return r == ',' || r == ';' || r == ' ' })
				for _, fw := range rawFws {
					if tag := normalizeTag(fw); tag != "" {
						cc.Frameworks = append(cc.Frameworks, tag)
					}
				}
			} else if v := pc.SessionCtx.ExtraContext["framework"]; v != "" {
				cc.Frameworks = []string{normalizeTag(v)}
			}
			if v := pc.SessionCtx.ExtraContext["language"]; v != "" {
				cc.Language = normalizeTag(v)
			}
			if v := pc.SessionCtx.ExtraContext["reflection_hits"]; v != "" {
				cc.HasReflectionHits = true
			}
		}

		if len(pc.SessionCtx.ReflectionHits) > 0 {
			cc.HasReflectionHits = true
		}
	}

	// Extract from UserIntent if available
	if pc.UserIntent != nil {
		cc.IntentVerb = pc.UserIntent.Verb
		cc.IntentTarget = pc.UserIntent.Target

		// Use target as semantic query for vector search
		if pc.UserIntent.Target != "" {
			cc.SemanticQuery = pc.UserIntent.Target
		}

		// Infer language from intent target if not already set.
		if cc.Language == "" && pc.UserIntent.Target != "" {
			cc.Language = inferLanguageFromTarget(pc.UserIntent.Target)
		}
	}

	if pc.SemanticQuery != "" {
		cc.SemanticQuery = pc.SemanticQuery
	}
	if pc.SemanticTopK > 0 {
		cc.SemanticTopK = pc.SemanticTopK
	}

	// Force Mangle language for system autopoiesis and rule synthesis prompts.
	if cc.Language == "" && shouldForceMangleLanguage(pc.ShardType) {
		cc.Language = "/mangle"
	}

	// Set campaign ID if present
	if pc.CampaignID != "" {
		cc.CampaignID = pc.CampaignID
	}

	// Store references to external context (for advanced JIT features)
	cc.SessionContext = pc.SessionCtx
	cc.UserIntent = pc.UserIntent

	return cc
}

// inferLanguageFromTarget tries to map a file extension to a JIT language tag.
// Returns empty string if no confident mapping is found.
func inferLanguageFromTarget(target string) string {
	ext := strings.ToLower(strings.TrimPrefix(filepath.Ext(target), "."))
	switch ext {
	case "go":
		return "/go"
	case "py":
		return "/python"
	case "ts", "tsx":
		return "/typescript"
	case "js", "jsx":
		return "/javascript"
	case "rs":
		return "/rust"
	case "java":
		return "/java"
	case "gl", "mg", "mangle":
		return "/mangle"
	default:
		return ""
	}
}

// shouldForceMangleLanguage determines if a shard prompt should default to /mangle.
func shouldForceMangleLanguage(shardType string) bool {
	normalized := strings.ToLower(strings.TrimSpace(strings.TrimPrefix(shardType, "/")))
	if normalized == "" {
		return false
	}
	if strings.Contains(normalized, "autopoiesis") {
		return true
	}
	switch normalized {
	case "legislator", "mangle_repair":
		return true
	}
	return strings.Contains(normalized, "mangle")
}

// AssembleSystemPrompt constructs a complete system prompt for a shard.
// It queries the kernel for context atoms and templates, then combines:
// 1. Base Piggyback Protocol instructions
// 2. Shard-specific template (from kernel or fallback)
// 3. Kernel-derived context atoms
// 4. Session context
func (pa *PromptAssembler) AssembleSystemPrompt(ctx context.Context, input any) (string, error) {
	timer := logging.StartTimer(logging.CategoryArticulation, "AssembleSystemPrompt")
	defer timer.Stop()

	if input == nil {
		return "", fmt.Errorf("prompt context is required")
	}

	var pc *PromptContext
	switch v := input.(type) {
	case *PromptContext:
		pc = v
	case map[string]any:
		var err error
		pc, err = pa.mapToPromptContext(v)
		if err != nil {
			return "", fmt.Errorf("failed to map context: %w", err)
		}
	default:
		return "", fmt.Errorf("unsupported prompt context type: %T", input)
	}

	if pc == nil {
		return "", fmt.Errorf("prompt context is nil after mapping")
	}

	logging.Articulation("Assembling system prompt for shard=%s (type=%s)", pc.ShardID, pc.ShardType)

	// Try JIT compilation if enabled (safely access compiler under read lock)
	pa.mu.RLock()
	compiler := pa.jitCompiler
	useJIT := pa.useJIT
	pa.mu.RUnlock()

	if useJIT && compiler != nil {
		cc := pa.toCompilationContext(pc)
		result, err := compiler.Compile(ctx, cc)
		if err == nil {
			// Ensure Piggyback Protocol is present when required
			if shouldAppendPiggybackProtocol(cc.ShardType, result.Prompt) &&
				(!strings.Contains(result.Prompt, "control_packet") || !strings.Contains(result.Prompt, "surface_response")) {
				logging.Articulation("JIT prompt missing Piggyback Protocol - appending mandatory suffix")
				result.Prompt += "\n\n" + PiggybackProtocolSuffix
			}

			logging.Articulation("JIT compiled prompt: %d bytes, %d atoms, %.1f%% budget",
				len(result.Prompt), result.AtomsIncluded, result.BudgetUsed*100)
			return result.Prompt, nil
		}
		// Telemetry: record JIT fallback into the kernel if possible.
		reason := err.Error()
		if len(reason) > 400 {
			reason = reason[:400]
		}
		_ = compiler.AssertFacts([]string{
			fmt.Sprintf("jit_fallback(%s, %q).", cc.ShardType, reason),
		})
		logging.Get(logging.CategoryArticulation).Warn("JIT compilation failed, falling back to legacy assembler: %v", err)
		// Fall through to legacy assembly
	}

	// Legacy prompt assembly (fallback)
	var sb strings.Builder

	usedLegacyTemplate := false

	// 1. Prefer a kernel-provided base template if available.
	baseline, tmplErr := pa.queryShardTemplate(pc.ShardType)
	if tmplErr == nil && baseline != "" {
		usedLegacyTemplate = true // ensure Piggyback suffix if kernel template omitted it
	} else {
		// Otherwise assemble baseline prompt from embedded mandatory atoms.
		// This keeps Piggyback, safety, shard identity, and methodology in YAML atoms.
		cc := pa.toCompilationContext(pc)
		var err error
		baseline, err = prompt.AssembleEmbeddedBaselinePrompt(cc)
		if err != nil || baseline == "" {
			// Emergency fallback to hard-coded legacy template.
			if err != nil {
				logging.Get(logging.CategoryArticulation).Warn("Baseline assembly failed: %v, falling back to legacy templates", err)
			}
			baseline = pa.getFallbackTemplate(pc.ShardType)
			usedLegacyTemplate = true
		}
	}
	sb.WriteString(baseline)
	sb.WriteString("\n\n")

	// 2. Query and inject context atoms from kernel
	contextAtoms, err := pa.queryContextAtoms(pc.ShardID)
	if err != nil {
		logging.Get(logging.CategoryArticulation).Warn("Failed to query context atoms: %v", err)
		// Continue without context atoms - not fatal
	}
	if len(contextAtoms) > 0 {
		sb.WriteString("// =============================================================================\n")
		sb.WriteString("// KERNEL-INJECTED CONTEXT (Derived from Logic)\n")
		sb.WriteString("// =============================================================================\n\n")
		for _, atom := range contextAtoms {
			sb.WriteString(fmt.Sprintf("- %s\n", atom))
		}
		sb.WriteString("\n")
	}

	// 3. Build and inject session context
	sessionCtx := pa.buildSessionContext(pc)
	if sessionCtx != "" {
		sb.WriteString("// =============================================================================\n")
		sb.WriteString("// SESSION CONTEXT (Blackboard State)\n")
		sb.WriteString("// =============================================================================\n")
		sb.WriteString(sessionCtx)
		sb.WriteString("\n")
	}

	// 4. Inject user intent if available
	intentCtx := pa.buildIntentContext(pc)
	if intentCtx != "" {
		sb.WriteString("// =============================================================================\n")
		sb.WriteString("// USER INTENT (Parsed by Perception)\n")
		sb.WriteString("// =============================================================================\n")
		sb.WriteString(intentCtx)
		sb.WriteString("\n")
	}

	// 5. If we had to fall back to hard-coded legacy templates, ensure Piggyback suffix.
	if usedLegacyTemplate && !prompt.IsStructuredOutputOnly(pc.ShardType) {
		sb.WriteString(PiggybackProtocolSuffix)
	}

	result := sb.String()
	logging.ArticulationDebug("Assembled system prompt: %d bytes", len(result))
	return result, nil
}

func shouldAppendPiggybackProtocol(shardType string, promptText string) bool {
	if prompt.IsStructuredOutputOnly(shardType) {
		return false
	}

	normalized := strings.ToLower(strings.TrimSpace(strings.TrimPrefix(shardType, "/")))
	switch normalized {
	case "perception", "perception_firewall":
		return false
	}

	return true
}

// queryShardTemplate queries the kernel for the base template for a shard type.
// Predicate: shard_prompt_base(ShardType, Template)
func (pa *PromptAssembler) queryShardTemplate(shardType string) (string, error) {
	logging.ArticulationDebug("Querying shard_prompt_base for type=%s", shardType)

	facts, err := pa.kernel.Query("shard_prompt_base")
	if err != nil {
		return "", fmt.Errorf("failed to query shard_prompt_base: %w", err)
	}

	// Look for matching shard type
	// Expected format: shard_prompt_base(/shardType, "template string")
	var matches []string
	shardAtom := "/" + shardType
	for _, fact := range facts {
		if len(fact.Args) < 2 {
			continue
		}

		factType := types.ExtractString(fact.Args[0])
		if factType == shardAtom || factType == shardType {
			if template := types.ExtractString(fact.Args[1]); template != "" {
				matches = append(matches, template)
			}
		}
	}

	if len(matches) > 0 {
		sort.Strings(matches)
		logging.ArticulationDebug("Found %d matching kernel templates for %s, selected alphabetically", len(matches), shardType)
		return matches[0], nil
	}

	logging.ArticulationDebug("No kernel template found for %s, using fallback", shardType)
	return "", fmt.Errorf("no template found for shard type: %s", shardType)
}

// queryContextAtoms queries the kernel for context atoms to inject into this shard.
// Predicate: injectable_context(ShardID, Atom)
func (pa *PromptAssembler) getInjectableContextFacts(shardID string) ([]types.Fact, error) {
	queries := []string{
		fmt.Sprintf("injectable_context(%q, _)", shardID),
	}
	if shardID != "*" {
		queries = append(queries, "injectable_context(\"*\", _)")
	}
	if shardID != "/_all" {
		queries = append(queries, "injectable_context(\"/_all\", _)")
	}

	var allFacts []types.Fact
	var lastErr error
	for _, q := range queries {
		facts, err := pa.kernel.Query(q)
		if err != nil {
			lastErr = err
			continue
		}
		allFacts = append(allFacts, facts...)
	}

	if allFacts == nil && lastErr != nil {
		return nil, lastErr
	}
	return allFacts, nil
}

// queryContextAtoms queries the kernel for context atoms to inject into this shard.
// Predicate: injectable_context(ShardID, Atom)
func (pa *PromptAssembler) queryContextAtoms(shardID string) ([]string, error) {
	logging.ArticulationDebug("Querying injectable_context for shard=%s", shardID)

	facts, err := pa.getInjectableContextFacts(shardID)
	if err != nil {
		return nil, fmt.Errorf("failed to query injectable_context: %w", err)
	}

	var atoms []string
	seen := make(map[string]struct{})
	for _, fact := range facts {
		if len(fact.Args) < 2 {
			continue
		}

		atom := types.ExtractString(fact.Args[1])
		if atom != "" {
			if _, exists := seen[atom]; !exists {
				seen[atom] = struct{}{}
				atoms = append(atoms, atom)
			}
		}
	}

	// Deterministically sort atoms alphabetically to prevent random prompt ordering
	sort.Strings(atoms)

	logging.ArticulationDebug("Found %d unique injectable context atoms for shard=%s", len(atoms), shardID)
	return atoms, nil
}

// buildSessionContext formats the session context for prompt injection.
func (pa *PromptAssembler) buildSessionContext(pc *PromptContext) string {
	if pc.SessionCtx == nil {
		return ""
	}

	var sb strings.Builder
	ctx := pc.SessionCtx

	// Dream mode indicator
	if ctx.DreamMode {
		sb.WriteString("\nMODE: DREAM (Simulation Only - DO NOT EXECUTE)\n")
		sb.WriteString("You are in simulation mode. Describe what you WOULD do, but do not actually perform any actions.\n\n")
	}

	// Current diagnostics (highest priority)
	if len(ctx.CurrentDiagnostics) > 0 {
		sb.WriteString("\nCURRENT BUILD/LINT ERRORS (must address):\n")
		maxCount := 20
		for i, diag := range ctx.CurrentDiagnostics {
			if i >= maxCount {
				sb.WriteString(fmt.Sprintf("  - ... and %d more build/lint errors\n", len(ctx.CurrentDiagnostics)-maxCount))
				break
			}
			sb.WriteString(fmt.Sprintf("  %s\n", diag))
		}
	}

	// Test state
	if ctx.TestState == "/failing" || len(ctx.FailingTests) > 0 {
		sb.WriteString("\nTEST STATE: FAILING\n")
		if ctx.TDDRetryCount > 0 {
			sb.WriteString(fmt.Sprintf("  TDD Retry: %d (fix root cause, not symptoms)\n", ctx.TDDRetryCount))
		}
		maxCount := 20
		for i, test := range ctx.FailingTests {
			if i >= maxCount {
				sb.WriteString(fmt.Sprintf("  - ... and %d more failing tests\n", len(ctx.FailingTests)-maxCount))
				break
			}
			sb.WriteString(fmt.Sprintf("  - %s\n", test))
		}
	}

	// Recent findings from other shards
	if len(ctx.RecentFindings) > 0 {
		sb.WriteString("\nRECENT FINDINGS:\n")
		maxCount := 20
		for i, finding := range ctx.RecentFindings {
			if i >= maxCount {
				sb.WriteString(fmt.Sprintf("  - ... and %d more findings\n", len(ctx.RecentFindings)-maxCount))
				break
			}
			sb.WriteString(fmt.Sprintf("  - %s\n", finding))
		}
	}

	// Reflection hits (System 2 memory)
	if len(ctx.ReflectionHits) > 0 {
		sb.WriteString("\nREFLECTION HITS:\n")
		maxCount := 20
		for i, hit := range ctx.ReflectionHits {
			if i >= maxCount {
				sb.WriteString(fmt.Sprintf("  - ... and %d more reflection hits\n", len(ctx.ReflectionHits)-maxCount))
				break
			}
			sb.WriteString(fmt.Sprintf("  - %s\n", hit))
		}
	}

	// Impacted files
	if len(ctx.ImpactedFiles) > 0 {
		sb.WriteString("\nIMPACTED FILES:\n")
		maxCount := 20
		for i, file := range ctx.ImpactedFiles {
			if i >= maxCount {
				sb.WriteString(fmt.Sprintf("  - ... and %d more impacted files\n", len(ctx.ImpactedFiles)-maxCount))
				break
			}
			sb.WriteString(fmt.Sprintf("  - %s\n", file))
		}
	}

	// Git context (Chesterton's Fence)
	if ctx.GitBranch != "" || len(ctx.GitRecentCommits) > 0 {
		sb.WriteString("\nGIT CONTEXT:\n")
		if ctx.GitBranch != "" {
			sb.WriteString(fmt.Sprintf("  Branch: %s\n", ctx.GitBranch))
		}
		if ctx.GitUnstagedCount > 0 {
			sb.WriteString(fmt.Sprintf("  Unstaged changes: %d\n", ctx.GitUnstagedCount))
		}
		if len(ctx.GitModifiedFiles) > 0 {
			sb.WriteString(fmt.Sprintf("  Modified files: %d\n", len(ctx.GitModifiedFiles)))
		}
		if len(ctx.GitRecentCommits) > 0 {
			sb.WriteString("  Recent commits (context for why code exists):\n")
			maxCount := 20
			for i, commit := range ctx.GitRecentCommits {
				if i >= maxCount {
					sb.WriteString(fmt.Sprintf("    - ... and %d more commits\n", len(ctx.GitRecentCommits)-maxCount))
					break
				}
				sb.WriteString(fmt.Sprintf("    - %s\n", commit))
			}
		}
	}

	// Campaign context
	if ctx.CampaignActive {
		sb.WriteString("\nCAMPAIGN CONTEXT:\n")
		if ctx.CampaignPhase != "" {
			sb.WriteString(fmt.Sprintf("  Phase: %s\n", ctx.CampaignPhase))
		}
		if ctx.CampaignGoal != "" {
			sb.WriteString(fmt.Sprintf("  Goal: %s\n", ctx.CampaignGoal))
		}
		if len(ctx.TaskDependencies) > 0 {
			sb.WriteString("  Blocked by: ")
			sb.WriteString(strings.Join(ctx.TaskDependencies, ", "))
			sb.WriteString("\n")
		}
	}

	// Prior shard outputs (cross-shard context)
	if len(ctx.PriorShardOutputs) > 0 {
		sb.WriteString("\nPRIOR SHARD RESULTS:\n")
		maxCount := 20
		for i, output := range ctx.PriorShardOutputs {
			if i >= maxCount {
				sb.WriteString(fmt.Sprintf("  - ... and %d more prior shard results\n", len(ctx.PriorShardOutputs)-maxCount))
				break
			}
			status := "SUCCESS"
			if !output.Success {
				status = "FAILED"
			}
			sb.WriteString(fmt.Sprintf("  [%s] %s: %s - %s\n",
				output.ShardType, status, output.Task, output.Summary))
		}
	}

	// Recent actions
	if len(ctx.RecentActions) > 0 {
		sb.WriteString("\nRECENT SESSION ACTIONS:\n")
		maxCount := 20
		for i, action := range ctx.RecentActions {
			if i >= maxCount {
				sb.WriteString(fmt.Sprintf("  - ... and %d more recent actions\n", len(ctx.RecentActions)-maxCount))
				break
			}
			sb.WriteString(fmt.Sprintf("  - %s\n", action))
		}
	}

	// Domain knowledge (Type B specialists)
	if len(ctx.KnowledgeAtoms) > 0 || len(ctx.SpecialistHints) > 0 {
		sb.WriteString("\nDOMAIN KNOWLEDGE:\n")
		maxCount := 20
		for i, atom := range ctx.KnowledgeAtoms {
			if i >= maxCount {
				sb.WriteString(fmt.Sprintf("  - ... and %d more knowledge atoms\n", len(ctx.KnowledgeAtoms)-maxCount))
				break
			}
			sb.WriteString(fmt.Sprintf("  - %s\n", atom))
		}
		for i, hint := range ctx.SpecialistHints {
			if i >= maxCount {
				sb.WriteString(fmt.Sprintf("  - ... and %d more hints\n", len(ctx.SpecialistHints)-maxCount))
				break
			}
			sb.WriteString(fmt.Sprintf("  - HINT: %s\n", hint))
		}
	}

	// Available tools (Ouroboros-generated)
	if len(ctx.AvailableTools) > 0 {
		sb.WriteString("\nAVAILABLE TOOLS:\n")
		maxCount := 20
		for i, tool := range ctx.AvailableTools {
			if i >= maxCount {
				sb.WriteString(fmt.Sprintf("  - ... and %d more tools\n", len(ctx.AvailableTools)-maxCount))
				break
			}
			sb.WriteString(fmt.Sprintf("  - %s: %s\n", tool.Name, tool.Description))
			if tool.BinaryPath != "" {
				sb.WriteString(fmt.Sprintf("    Binary: %s\n", tool.BinaryPath))
			}
		}
	}

	// Safety constraints
	if len(ctx.BlockedActions) > 0 || len(ctx.SafetyWarnings) > 0 {
		sb.WriteString("\nSAFETY CONSTRAINTS:\n")
		maxCount := 20
		for i, blocked := range ctx.BlockedActions {
			if i >= maxCount {
				sb.WriteString(fmt.Sprintf("  BLOCKED: ... and %d more blocked actions\n", len(ctx.BlockedActions)-maxCount))
				break
			}
			sb.WriteString(fmt.Sprintf("  BLOCKED: %s\n", blocked))
		}
		for i, warning := range ctx.SafetyWarnings {
			if i >= maxCount {
				sb.WriteString(fmt.Sprintf("  WARNING: ... and %d more safety warnings\n", len(ctx.SafetyWarnings)-maxCount))
				break
			}
			sb.WriteString(fmt.Sprintf("  WARNING: %s\n", warning))
		}
	}

	// Compressed history (if small enough)
	if ctx.CompressedHistory != "" && len(ctx.CompressedHistory) < 1500 {
		sb.WriteString("\nSESSION HISTORY (compressed):\n")
		sb.WriteString(ctx.CompressedHistory)
		sb.WriteString("\n")
	}

	return sb.String()
}

// buildIntentContext formats the user intent for prompt injection.
func (pa *PromptAssembler) buildIntentContext(pc *PromptContext) string {
	if pc.UserIntent == nil {
		return ""
	}

	var sb strings.Builder
	intent := pc.UserIntent

	sb.WriteString(fmt.Sprintf("Intent ID: %s\n", intent.ID))
	sb.WriteString(fmt.Sprintf("Category: %s\n", intent.Category))
	sb.WriteString(fmt.Sprintf("Verb: %s\n", intent.Verb))
	sb.WriteString(fmt.Sprintf("Target: %s\n", intent.Target))
	if intent.Constraint != "" {
		sb.WriteString(fmt.Sprintf("Constraint: %s\n", intent.Constraint))
	}

	return sb.String()
}

// getFallbackTemplate returns a hardcoded fallback template for a shard type.
func (pa *PromptAssembler) getFallbackTemplate(shardType string) string {
	switch shardType {
	case "coder":
		return coderFallbackTemplate
	case "tester":
		return testerFallbackTemplate
	case "reviewer":
		return reviewerFallbackTemplate
	case "researcher":
		return researcherFallbackTemplate
	default:
		return genericFallbackTemplate
	}
}

// =============================================================================
// PIGGYBACK PROTOCOL SUFFIX (Mandatory for user-facing shards)
// =============================================================================

// PiggybackProtocolSuffix is the standard suffix appended to all shard prompts.
// It enforces the dual-channel output format required by the articulation layer.
const PiggybackProtocolSuffix = `
// =============================================================================
// OUTPUT PROTOCOL: PIGGYBACK ENVELOPE (MANDATORY)
// =============================================================================

You MUST output a JSON object with this exact structure. No exceptions.

{
  "control_packet": {
    "intent_classification": {
      "category": "/mutation|/query|/instruction",
      "verb": "/action_verb",
      "target": "target_path_or_concept",
      "constraint": "any constraints",
      "confidence": 0.0-1.0
    },
    "mangle_updates": [
      "predicate(arg1, arg2).",
      "another_fact(x, y)."
    ],
    "memory_operations": [
      {"op": "promote_to_long_term|forget|store_vector|note", "key": "...", "value": "..."}
    ],
    "self_correction": {
      "triggered": false,
      "hypothesis": "..."
    },
    "knowledge_requests": [
      {
        "specialist": "agent_name|researcher|_any_specialist",
        "query": "specific question for the specialist",
        "purpose": "why this knowledge is needed",
        "priority": "required|optional"
      }
    ],
    "reasoning_trace": "<REASONING_TRACE: your actual step-by-step reasoning>",
    "context_feedback": {
      "overall_usefulness": 0.0,
      "helpful_facts": [],
      "noise_facts": [],
      "missing_context": "<MISSING_CONTEXT: what would have helped, or empty string>"
    },
    "tool_requests": [
      {
        "id": "req_1",
        "tool_name": "read_file",
        "tool_args": {"path": "path/to/file.go"},
        "purpose": "Read source to answer accurately",
        "required": true
      }
    ]
  },
  "surface_response": "<SURFACE_RESPONSE: your reply to the user, plain language, never this placeholder>"
}

Angle-bracket text in this example is a placeholder describing what to write; never emit it verbatim. "helpful_facts" and "noise_facts" list the injected predicate names that helped or hurt; leave them empty when none did.

## CRITICAL: THOUGHT-FIRST ORDERING

The control_packet MUST be fully formed BEFORE you write the surface_response.
Think first, speak second. The control packet is your commitment to what you're about to say.

## MANGLE UPDATES FORMAT

Mangle facts use this syntax:
- Predicates are lowercase with underscores: task_status, file_modified
- Name constants start with /: /complete, /pending, /coder
- Strings are quoted: "path/to/file.go"
- Every statement ends with a period: .

Examples:
- task_status(/current_task, /complete).
- file_modified("internal/foo.go", /write).
- shard_executed(/coder, "fix bug", /success).

## KNOWLEDGE REQUESTS (Optional)

Use knowledge_requests when you need information you don't have:
- specialist: Name of an agent to consult, "researcher" for web search, or "_any_specialist"
- query: The specific question to answer
- purpose: Why this knowledge is needed (helps with context handoff)
- priority: "required" (blocking) or "optional" (best-effort)

NEVER say "I don't have that information" - request knowledge instead!
The system will gather knowledge and re-invoke you with the results.

## REQUIRED FIELDS

When Piggyback Protocol is active, the output MUST include ALL fields shown in the schema.
If a field is not needed, emit an empty value instead:
- arrays: []
- strings: ""
- objects: include all keys with empty values
`

// =============================================================================
// FALLBACK TEMPLATES (Used when kernel has no template)
// =============================================================================

const coderFallbackTemplate = `// =============================================================================
// CODER SHARD - Code Generation and Modification
// =============================================================================

You are the Coder Shard of codeNERD. Your purpose is to generate, modify, and refactor code.

## Core Responsibilities
1. Generate new code following project patterns
2. Modify existing code to fix bugs or add features
3. Refactor code for clarity and performance
4. Follow the language idioms and project conventions

## Absolute Rules
1. NEVER ignore errors - always handle them explicitly
2. NEVER break existing functionality - preserve semantic integrity
3. NEVER add features not requested - do exactly what is asked
4. ALWAYS emit complete files, not diffs

## Output Requirements
- For file modifications: provide COMPLETE new file content
- For new files: provide the full file content
- Include rationale explaining your changes
`

const testerFallbackTemplate = `// =============================================================================
// TESTER SHARD - Test Generation and Execution
// =============================================================================

You are the Tester Shard of codeNERD. Your purpose is to generate tests and analyze test results.

## Core Responsibilities
1. Generate comprehensive tests for code
2. Analyze test failures and suggest fixes
3. Ensure adequate test coverage
4. Follow table-driven test patterns where appropriate

## Absolute Rules
1. NEVER generate tests that always pass (test meaningful behavior)
2. NEVER skip edge cases - test boundaries and error conditions
3. ALWAYS include both positive and negative test cases
4. ALWAYS follow the project's testing conventions
`

const reviewerFallbackTemplate = `// =============================================================================
// REVIEWER SHARD - Code Review and Analysis
// =============================================================================

You are the Reviewer Shard of codeNERD. Your purpose is to review code for quality, security, and correctness.

## Core Responsibilities
1. Identify bugs, security issues, and code smells
2. Suggest improvements and optimizations
3. Verify code follows project conventions
4. Check for common vulnerabilities

## Absolute Rules
1. NEVER hallucinate issues that don't exist
2. NEVER ignore actual problems to be "nice"
3. ALWAYS provide actionable feedback with specific line references
4. ALWAYS distinguish between critical issues and style preferences
`

const researcherFallbackTemplate = `// =============================================================================
// RESEARCHER SHARD - Knowledge Gathering and Documentation
// =============================================================================

You are the Researcher Shard of codeNERD. Your purpose is to gather knowledge and provide domain expertise.

## Core Responsibilities
1. Research APIs, libraries, and frameworks
2. Extract knowledge from documentation
3. Provide domain-specific guidance
4. Build specialist knowledge for future reference

## Absolute Rules
1. NEVER invent information - cite sources when possible
2. NEVER provide outdated information - verify currency
3. ALWAYS synthesize knowledge into actionable insights
4. ALWAYS structure findings for easy consumption
`

const genericFallbackTemplate = `// =============================================================================
// GENERIC SHARD
// =============================================================================

You are a specialist shard of codeNERD. Execute your task precisely and efficiently.

## Core Principles
1. Do exactly what is asked
2. Handle errors explicitly
3. Provide clear reasoning for your actions
4. Follow the output protocol exactly
`

// =============================================================================
// UTILITY FUNCTIONS
// =============================================================================

// AssembleQuickPrompt is a convenience function for assembling a prompt with minimal context.
// It creates a PromptAssembler, assembles the prompt, and returns the result.
func AssembleQuickPrompt(ctx context.Context, kernel KernelQuerier, shardID, shardType string) (string, error) {
	pa, err := NewPromptAssembler(kernel)
	if err != nil {
		return "", err
	}

	pc := &PromptContext{
		ShardID:   shardID,
		ShardType: shardType,
	}

	return pa.AssembleSystemPrompt(ctx, pc)
}

// WithSessionContext returns a new PromptContext with session context added.
func (pc *PromptContext) WithSessionContext(ctx *types.SessionContext) *PromptContext {
	pc.SessionCtx = ctx
	return pc
}

// WithIntent returns a new PromptContext with user intent added.
func (pc *PromptContext) WithIntent(intent *types.StructuredIntent) *PromptContext {
	pc.UserIntent = intent
	return pc
}

// WithCampaign returns a new PromptContext with campaign ID added.
func (pc *PromptContext) WithCampaign(campaignID string) *PromptContext {
	pc.CampaignID = campaignID
	return pc
}

// WithSemanticQuery overrides the semantic search query and top-K for JIT selection.
func (pc *PromptContext) WithSemanticQuery(query string, topK int) *PromptContext {
	pc.SemanticQuery = query
	if topK > 0 {
		pc.SemanticTopK = topK
	}
	return pc
}

// =============================================================================
// JIT COMPILER INTEGRATION
// =============================================================================
// These methods enable JIT prompt compilation as an optional enhancement.

// JITReady returns true if JIT compilation is available and enabled.
// JIT compilation requires both a JIT compiler instance and the feature flag to be enabled.
func (pa *PromptAssembler) JITReady() bool {
	pa.mu.RLock()
	defer pa.mu.RUnlock()
	return pa.useJIT && pa.jitCompiler != nil
}

// EnableJIT enables or disables JIT compilation.
// When disabled, the legacy prompt assembly is used.
func (pa *PromptAssembler) EnableJIT(enable bool) {
	pa.mu.Lock()
	defer pa.mu.Unlock()
	pa.useJIT = enable
	if enable {
		logging.Articulation("JIT compilation enabled")
	} else {
		logging.Articulation("JIT compilation disabled")
	}
}

// SetJITCompiler sets the JIT compiler instance.
// If compiler is non-nil and useJIT is true, JIT compilation will be used.
func (pa *PromptAssembler) SetJITCompiler(compiler *prompt.JITPromptCompiler) {
	pa.mu.Lock()
	defer pa.mu.Unlock()
	pa.jitCompiler = compiler
	if compiler != nil {
		logging.Articulation("JIT compiler attached to PromptAssembler")
	} else {
		logging.Articulation("JIT compiler detached from PromptAssembler")
	}
}

// GetJITCompiler returns the current JIT compiler, or nil if not set.
func (pa *PromptAssembler) GetJITCompiler() *prompt.JITPromptCompiler {
	pa.mu.RLock()
	defer pa.mu.RUnlock()
	return pa.jitCompiler
}

// IsJITEnabled returns true if JIT is enabled (regardless of compiler availability).
func (pa *PromptAssembler) IsJITEnabled() bool {
	pa.mu.RLock()
	defer pa.mu.RUnlock()
	return pa.useJIT
}

// SetJITBudgets overrides token budgets for compiled prompts.
func (pa *PromptAssembler) SetJITBudgets(tokenBudget, reservedTokens, semanticTopK, reservedTokensFallbackRatio int) {
	pa.mu.Lock()
	defer pa.mu.Unlock()
	if tokenBudget > 0 {
		pa.tokenBudget = tokenBudget
	}
	if reservedTokens > 0 {
		pa.reservedTokens = reservedTokens
	}
	if semanticTopK > 0 {
		pa.semanticTopK = semanticTopK
	}
	if reservedTokensFallbackRatio > 0 {
		pa.reservedTokensFallbackRatio = reservedTokensFallbackRatio
	}
	if pa.tokenBudget > 0 && pa.reservedTokens >= pa.tokenBudget {
		// Clamp to a safe fallback so Validate() doesn't fail.
		fallbackRatio := pa.reservedTokensFallbackRatio
		if fallbackRatio <= 0 {
			fallbackRatio = 10
		}
		pa.reservedTokens = pa.tokenBudget / fallbackRatio
		if pa.reservedTokens >= pa.tokenBudget {
			pa.reservedTokens = 0
		}
	}
}

func (pa *PromptAssembler) getBudgetConfig() (int, int, int, int) {
	pa.mu.RLock()
	defer pa.mu.RUnlock()
	return pa.tokenBudget, pa.reservedTokens, pa.semanticTopK, pa.reservedTokensFallbackRatio
}
