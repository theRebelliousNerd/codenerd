package prompt

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"codenerd/internal/logging"
	"codenerd/internal/store"
)

// Fact represents a logic fact (predicate + args).
type Fact struct {
	Predicate string
	Args      []interface{}
}

// KernelQuerier defines the interface for querying the Mangle kernel.
// This abstracts the kernel to avoid circular imports.
type KernelQuerier interface {
	// Query retrieves facts matching a predicate.
	Query(predicate string) ([]Fact, error)

	// AssertBatch adds multiple facts efficiently.
	AssertBatch(facts []interface{}) error
}

// VectorSearcher defines the interface for semantic search.
type VectorSearcher interface {
	// Search performs semantic search and returns atom IDs with scores.
	Search(ctx context.Context, query string, limit int) ([]SearchResult, error)
}

// SearchResult represents a semantic search result.
type SearchResult struct {
	AtomID string
	Score  float64
}

// CompilationStats provides comprehensive metrics for a single compilation.
// This is the "god-tier" observability structure for JIT prompt compilation.
type CompilationStats struct {
	// --- Timing Metrics ---
	// Total compilation duration from start to finish
	Duration time.Duration

	// Phase-level timing breakdown (all in milliseconds for log parsing)
	CollectAtomsMs int64 // Time to collect atoms from all sources
	SelectAtomsMs  int64 // Time for Mangle selection + vector search
	ResolveDepsMs  int64 // Time for dependency resolution
	FitBudgetMs    int64 // Time for budget fitting with polymorphism
	AssembleMs     int64 // Time to assemble final prompt text
	VectorQueryMs  int64 // Time spent specifically on vector search (subset of SelectAtomsMs)

	// --- Atom Counts ---
	// AtomsSelected is the number of atoms in the final compiled prompt
	AtomsSelected int

	// SkeletonAtoms is the count of mandatory/skeleton atoms (always included)
	SkeletonAtoms int

	// FleshAtoms is the count of probabilistic/flesh atoms (context-dependent)
	FleshAtoms int

	// AtomsCandidates is total atoms considered before filtering
	AtomsCandidates int

	// AtomsDropped is atoms rejected by selector or budget
	AtomsDropped int

	// --- Token Metrics ---
	// TokensUsed is the total tokens in the compiled prompt
	TokensUsed int

	// TokenBudget is the target budget for this compilation
	TokenBudget int

	// BudgetUtilization is TokensUsed/TokenBudget as percentage (0.0-1.0)
	BudgetUtilization float64

	// SkeletonTokens is tokens used by mandatory atoms
	SkeletonTokens int

	// FleshTokens is tokens used by optional atoms
	FleshTokens int

	// --- Source Breakdown ---
	// EmbeddedAtoms is count of atoms from embedded corpus
	EmbeddedAtoms int

	// ProjectAtoms is count of atoms from project database
	ProjectAtoms int

	// ShardAtoms is count of atoms from shard-specific database
	ShardAtoms int

	// --- Render Mode Distribution ---
	// StandardModeCount is atoms rendered in full/standard mode
	StandardModeCount int

	// ConciseModeCount is atoms rendered in concise mode
	ConciseModeCount int

	// MinModeCount is atoms rendered in minimal mode
	MinModeCount int

	// --- Cache & Fallback ---
	// FallbackUsed indicates whether legacy fallback compilation was used
	FallbackUsed bool

	// CacheHit indicates whether a cached skeleton was used
	CacheHit bool

	// CacheKey is the cache key used (for debugging)
	CacheKey string

	// --- Context Info (for correlation) ---
	// ShardID is the shard this compilation was for
	ShardID string

	// OperationalMode is the mode during compilation
	OperationalMode string

	// IntentVerb is the user intent that triggered compilation
	IntentVerb string
}

// String returns a human-readable summary of the compilation stats.
func (s *CompilationStats) String() string {
	return fmt.Sprintf(
		"JIT[%s] %dms | atoms=%d (skel=%d flesh=%d) | tokens=%d/%d (%.1f%%) | vector=%dms | cache=%v",
		s.ShardID,
		s.Duration.Milliseconds(),
		s.AtomsSelected,
		s.SkeletonAtoms,
		s.FleshAtoms,
		s.TokensUsed,
		s.TokenBudget,
		s.BudgetUtilization*100,
		s.VectorQueryMs,
		s.CacheHit,
	)
}

// ToLogFields returns the stats as a map for structured logging.
func (s *CompilationStats) ToLogFields() map[string]interface{} {
	return map[string]interface{}{
		"duration_ms":      s.Duration.Milliseconds(),
		"collect_ms":       s.CollectAtomsMs,
		"select_ms":        s.SelectAtomsMs,
		"resolve_ms":       s.ResolveDepsMs,
		"fit_ms":           s.FitBudgetMs,
		"assemble_ms":      s.AssembleMs,
		"vector_ms":        s.VectorQueryMs,
		"atoms_selected":   s.AtomsSelected,
		"skeleton_atoms":   s.SkeletonAtoms,
		"flesh_atoms":      s.FleshAtoms,
		"atoms_candidates": s.AtomsCandidates,
		"atoms_dropped":    s.AtomsDropped,
		"tokens_used":      s.TokensUsed,
		"token_budget":     s.TokenBudget,
		"budget_util":      s.BudgetUtilization,
		"skeleton_tokens":  s.SkeletonTokens,
		"flesh_tokens":     s.FleshTokens,
		"embedded_atoms":   s.EmbeddedAtoms,
		"project_atoms":    s.ProjectAtoms,
		"shard_atoms":      s.ShardAtoms,
		"standard_mode":    s.StandardModeCount,
		"concise_mode":     s.ConciseModeCount,
		"min_mode":         s.MinModeCount,
		"fallback_used":    s.FallbackUsed,
		"cache_hit":        s.CacheHit,
		"shard_id":         s.ShardID,
		"operational_mode": s.OperationalMode,
		"intent_verb":      s.IntentVerb,
	}
}

// CompilationResult holds the output of a prompt compilation.
type CompilationResult struct {
	// The compiled prompt text
	Prompt string

	// Atoms included in the prompt (for debugging/audit)
	IncludedAtoms []*PromptAtom

	// Token usage statistics
	TotalTokens    int
	BudgetUsed     float64 // Percentage of budget used
	MandatoryCount int
	OptionalCount  int

	// Categories breakdown
	CategoryTokens map[AtomCategory]int

	// Compilation metadata
	CompilationTimeMs int64
	AtomsCandidates   int // Total atoms considered
	AtomsSelected     int // Atoms that matched context
	AtomsIncluded     int // Atoms that fit in budget

	// Compilation manifest (Flight Recorder)
	Manifest *PromptManifest

	// Comprehensive compilation statistics
	Stats *CompilationStats
}

// JITPromptCompiler compiles optimal system prompts from atomic fragments.
// It combines rule-based selection (Mangle) with semantic search (vectors)
// to select the most relevant prompt atoms for a given context.
type JITPromptCompiler struct {
	// Embedded corpus (baked-in atoms)
	embeddedCorpus *EmbeddedCorpus

	// Project-level corpus database (.nerd/prompts/corpus.db)
	projectDB *sql.DB

	// Shard-specific atom databases (keyed by shard ID)
	shardDBs map[string]*sql.DB

	// Kernel for rule-based selection
	kernel KernelQuerier

	// Vector store for semantic search
	vectorSearcher VectorSearcher

	// Sub-components
	selector  *AtomSelector
	resolver  *DependencyResolver
	budgetMgr *TokenBudgetManager
	assembler *FinalAssembler

	// Observability
	lastResult *CompilationResult

	// Concurrency control
	mu sync.RWMutex

	// Configuration
	config CompilerConfig

	// LocalDB for semantic knowledge atom queries (Semantic Knowledge Bridge)
	localDB *store.LocalStore
}

// CompilerConfig holds configuration for the JIT compiler.
type CompilerConfig struct {
	// DefaultTokenBudget is the default token budget if not specified in context
	DefaultTokenBudget int

	// EnableVectorSearch enables semantic search for atom selection
	EnableVectorSearch bool

	// VectorSearchWeight is the weight of vector scores vs logic scores (0.0-1.0)
	VectorSearchWeight float64

	// MaxAtomsPerCategory caps atoms selected per category
	MaxAtomsPerCategory int

	// EnableCaching enables caching of compiled prompts
	EnableCaching bool

	// CacheTTLSeconds is the cache TTL in seconds
	CacheTTLSeconds int
}

// DefaultCompilerConfig returns a sensible default configuration.
func DefaultCompilerConfig() CompilerConfig {
	return CompilerConfig{
		DefaultTokenBudget:  100000,
		EnableVectorSearch:  true,
		VectorSearchWeight:  0.3, // 70% logic, 30% vector
		MaxAtomsPerCategory: 10,
		EnableCaching:       true,
		CacheTTLSeconds:     300, // 5 minutes
	}
}

// NewJITPromptCompiler creates a new JIT prompt compiler.
func NewJITPromptCompiler(opts ...CompilerOption) (*JITPromptCompiler, error) {
	config := DefaultCompilerConfig()

	compiler := &JITPromptCompiler{
		shardDBs:  make(map[string]*sql.DB),
		config:    config,
		selector:  NewAtomSelector(),
		resolver:  NewDependencyResolver(),
		budgetMgr: NewTokenBudgetManager(),
		assembler: NewFinalAssembler(),
	}

	// Apply options
	for _, opt := range opts {
		if err := opt(compiler); err != nil {
			return nil, fmt.Errorf("failed to apply compiler option: %w", err)
		}
	}

	logging.Get(logging.CategoryContext).Info("JITPromptCompiler initialized")
	return compiler, nil
}

// CompilerOption is a functional option for configuring the compiler.
type CompilerOption func(*JITPromptCompiler) error

// WithEmbeddedCorpus sets the embedded atom corpus.
func WithEmbeddedCorpus(corpus *EmbeddedCorpus) CompilerOption {
	return func(c *JITPromptCompiler) error {
		c.embeddedCorpus = corpus
		return nil
	}
}

// WithProjectDB sets the project-level atom database.
func WithProjectDB(db *sql.DB) CompilerOption {
	return func(c *JITPromptCompiler) error {
		c.projectDB = db
		return nil
	}
}

// WithKernel sets the Mangle kernel for rule-based selection.
func WithKernel(kernel KernelQuerier) CompilerOption {
	return func(c *JITPromptCompiler) error {
		c.kernel = kernel
		c.selector.SetKernel(kernel)
		return nil
	}
}

// WithVectorSearcher sets the vector searcher for semantic selection.
func WithVectorSearcher(vs VectorSearcher) CompilerOption {
	return func(c *JITPromptCompiler) error {
		c.vectorSearcher = vs
		c.selector.SetVectorSearcher(vs)
		return nil
	}
}

// WithConfig sets the compiler configuration.
func WithConfig(config CompilerConfig) CompilerOption {
	return func(c *JITPromptCompiler) error {
		c.config = config
		return nil
	}
}

// Compile generates a system prompt for the given context.
// This is the main entry point for prompt compilation.
func (c *JITPromptCompiler) Compile(ctx context.Context, cc *CompilationContext) (*CompilationResult, error) {
	// Legacy timer for backward compatibility
	timer := logging.StartTimer(logging.CategoryJIT, "JITPromptCompiler.Compile")
	defer timer.Stop()

	// Validate context first (before accessing any fields)
	if cc == nil {
		return nil, fmt.Errorf("compilation context is required")
	}

	if err := cc.Validate(); err != nil {
		return nil, fmt.Errorf("invalid compilation context: %w", err)
	}

	// Start comprehensive timing after validation
	compileStart := time.Now()
	stats := &CompilationStats{
		ShardID:         cc.ShardID,
		OperationalMode: cc.OperationalMode,
		IntentVerb:      cc.IntentVerb,
	}

	logging.Get(logging.CategoryJIT).Info("Compiling prompt: %s", cc.String())

	// Step 1: Collect all candidate atoms from all sources
	collectStart := time.Now()
	candidates, sourceBreakdown, err := c.collectAtomsWithStats(ctx, cc)
	if err != nil {
		return nil, fmt.Errorf("failed to collect atoms: %w", err)
	}
	stats.CollectAtomsMs = time.Since(collectStart).Milliseconds()
	stats.AtomsCandidates = len(candidates)
	stats.EmbeddedAtoms = sourceBreakdown.embedded
	stats.ProjectAtoms = sourceBreakdown.project
	stats.ShardAtoms = sourceBreakdown.shard

	logging.Get(logging.CategoryJIT).Debug(
		"Collected %d candidate atoms (embedded=%d, project=%d, shard=%d) in %dms",
		len(candidates), sourceBreakdown.embedded, sourceBreakdown.project, sourceBreakdown.shard,
		stats.CollectAtomsMs,
	)

	// Step 1.5: Assert context facts to kernel for Mangle-based selection
	// This enables policy.mg Section 45/46 rules to boost atoms matching current context
	if c.kernel != nil {
		contextFacts := cc.ToContextFacts()
		if len(contextFacts) > 0 {
			if err := c.kernel.AssertBatch(toInterfaceSlice(contextFacts)); err != nil {
				logging.Get(logging.CategoryJIT).Warn("Failed to assert context facts: %v", err)
				// Non-fatal - continue without context-based boosting
			} else {
				logging.Get(logging.CategoryJIT).Debug("Asserted %d context facts to kernel", len(contextFacts))
			}
		}
	}

	// Step 1.6: Collect dynamic kernel-injected atoms (injectable_context, specialist_knowledge)
	// These are ephemeral atoms derived from runtime logic and should be treated as mandatory flesh.
	dynamicAtoms, dynErr := c.collectKernelInjectedAtoms(cc)
	if dynErr != nil {
		logging.Get(logging.CategoryJIT).Warn("Failed to collect kernel-injected atoms: %v", dynErr)
	} else if len(dynamicAtoms) > 0 {
		candidates = append(candidates, dynamicAtoms...)
		stats.AtomsCandidates = len(candidates)
		logging.Get(logging.CategoryJIT).Debug("Appended %d kernel-injected atoms to candidates", len(dynamicAtoms))
	}

	// Step 1.7: Collect semantic knowledge atoms (Semantic Knowledge Bridge)
	// These are knowledge atoms from documentation ingestion that match the current context.
	knowledgeAtoms := c.collectKnowledgeAtoms(ctx, cc)
	if len(knowledgeAtoms) > 0 {
		candidates = append(candidates, knowledgeAtoms...)
		stats.AtomsCandidates = len(candidates)
		logging.Get(logging.CategoryJIT).Debug("Appended %d semantic knowledge atoms to candidates", len(knowledgeAtoms))
	}

	// Step 2: Select atoms based on context (Mangle rules + vector search)
	selectStart := time.Now()
	scored, vectorMs, err := c.selector.SelectAtomsWithTiming(ctx, candidates, cc)
	if err != nil {
		return nil, fmt.Errorf("failed to select atoms: %w", err)
	}
	stats.SelectAtomsMs = time.Since(selectStart).Milliseconds()
	stats.VectorQueryMs = vectorMs

	logging.Get(logging.CategoryJIT).Debug(
		"Selected %d atoms after scoring in %dms (vector=%dms)",
		len(scored), stats.SelectAtomsMs, stats.VectorQueryMs,
	)

	// Step 3: Resolve dependencies and handle conflicts
	resolveStart := time.Now()
	ordered, err := c.resolver.Resolve(scored)
	if err != nil {
		return nil, fmt.Errorf("failed to resolve dependencies: %w", err)
	}
	stats.ResolveDepsMs = time.Since(resolveStart).Milliseconds()

	logging.Get(logging.CategoryJIT).Debug(
		"Resolved to %d ordered atoms in %dms",
		len(ordered), stats.ResolveDepsMs,
	)

	// Step 4: Fit within token budget
	fitStart := time.Now()
	budget := cc.AvailableTokens()
	if budget <= 0 {
		budget = c.config.DefaultTokenBudget
	}
	stats.TokenBudget = budget

	fitted, err := c.budgetMgr.Fit(ordered, budget)
	if err != nil {
		return nil, fmt.Errorf("failed to fit budget: %w", err)
	}
	stats.FitBudgetMs = time.Since(fitStart).Milliseconds()

	logging.Get(logging.CategoryJIT).Debug(
		"Fitted %d atoms within budget of %d tokens in %dms",
		len(fitted), budget, stats.FitBudgetMs,
	)

	// Step 5: Assemble final prompt
	assembleStart := time.Now()
	prompt, err := c.assembler.Assemble(fitted, cc)
	if err != nil {
		return nil, fmt.Errorf("failed to assemble prompt: %w", err)
	}
	stats.AssembleMs = time.Since(assembleStart).Milliseconds()

	// Finalize timing
	stats.Duration = time.Since(compileStart)

	// Build result with comprehensive stats
	result := c.buildResultWithStats(candidates, scored, fitted, prompt, budget, stats)

	// Update observability state
	c.mu.Lock()
	c.lastResult = result
	c.mu.Unlock()

	// Log comprehensive stats using JIT category
	c.logCompilationStats(stats, result)

	return result, nil
}

// collectKernelInjectedAtoms queries runtime context predicates and turns them into ephemeral PromptAtoms.
// This lets JIT prompts include spreading-activation context and specialist knowledge without legacy injection.
func (c *JITPromptCompiler) collectKernelInjectedAtoms(cc *CompilationContext) ([]*PromptAtom, error) {
	if c.kernel == nil || cc == nil {
		return nil, nil
	}

	matchesShard := func(raw string) bool {
		raw = strings.TrimSpace(raw)
		if raw == "" {
			return false
		}
		if raw == "*" || raw == "/_all" {
			return true
		}
		rawTrim := strings.TrimPrefix(raw, "/")
		if cc.ShardInstanceID != "" && rawTrim == cc.ShardInstanceID {
			return true
		}
		return rawTrim == cc.ShardID
	}

	var dynamic []*PromptAtom

	// Injectable context atoms
	ctxFacts, err := c.kernel.Query("injectable_context")
	if err != nil {
		return nil, err
	}
	var ctxAtoms []string
	for _, fact := range ctxFacts {
		if len(fact.Args) < 2 {
			continue
		}
		factShardID := extractStringArg(fact.Args[0])
		if !matchesShard(factShardID) {
			continue
		}
		if atom, ok := fact.Args[1].(string); ok && strings.TrimSpace(atom) != "" {
			ctxAtoms = append(ctxAtoms, atom)
		} else if atomStr := extractStringArg(fact.Args[1]); strings.TrimSpace(atomStr) != "" {
			ctxAtoms = append(ctxAtoms, atomStr)
		}
	}
	if len(ctxAtoms) > 0 {
		var sb strings.Builder
		sb.WriteString("// KERNEL-INJECTED CONTEXT (from spreading activation)\n")
		for _, atom := range ctxAtoms {
			sb.WriteString("- ")
			sb.WriteString(atom)
			sb.WriteString("\n")
		}
		content := sb.String()
		id := fmt.Sprintf("kernel/context/%s", HashContent(content)[:8])
		pa := NewPromptAtom(id, CategoryContext, content)
		pa.IsMandatory = true
		pa.Priority = 95
		pa.ShardTypes = []string{cc.ShardID}
		dynamic = append(dynamic, pa)
	}

	// Specialist knowledge blocks
	knowledgeFacts, err := c.kernel.Query("specialist_knowledge")
	if err == nil {
		type block struct {
			topic   string
			content string
		}
		var blocks []block
		for _, fact := range knowledgeFacts {
			if len(fact.Args) < 3 {
				continue
			}
			factShardID := extractStringArg(fact.Args[0])
			if !matchesShard(factShardID) {
				continue
			}
			topic := extractStringArg(fact.Args[1])
			body := extractStringArg(fact.Args[2])
			if strings.TrimSpace(topic) != "" && strings.TrimSpace(body) != "" {
				blocks = append(blocks, block{topic: topic, content: body})
			}
		}
		if len(blocks) > 0 {
			var sb strings.Builder
			sb.WriteString("// SPECIALIST KNOWLEDGE (Type B/U expertise)\n")
			for _, b := range blocks {
				sb.WriteString("## ")
				sb.WriteString(b.topic)
				sb.WriteString("\n")
				sb.WriteString(b.content)
				sb.WriteString("\n\n")
			}
			content := strings.TrimRight(sb.String(), "\n")
			id := fmt.Sprintf("kernel/knowledge/%s", HashContent(content)[:8])
			pa := NewPromptAtom(id, CategoryKnowledge, content)
			pa.IsMandatory = true
			pa.Priority = 90
			pa.ShardTypes = []string{cc.ShardID}
			dynamic = append(dynamic, pa)
		}
	}

	return dynamic, nil
}

// sourceBreakdown tracks atom counts by source.
type sourceBreakdown struct {
	embedded int
	project  int
	shard    int
}

// collectAtomsWithStats gathers atoms and tracks source breakdown.
func (c *JITPromptCompiler) collectAtomsWithStats(ctx context.Context, cc *CompilationContext) ([]*PromptAtom, sourceBreakdown, error) {
	var allAtoms []*PromptAtom
	var breakdown sourceBreakdown

	c.mu.RLock()
	defer c.mu.RUnlock()

	// 1. Embedded corpus (always first)
	if c.embeddedCorpus != nil {
		embedded := c.embeddedCorpus.All()
		breakdown.embedded = len(embedded)
		allAtoms = append(allAtoms, embedded...)
	}

	// 2. Project database
	if c.projectDB != nil {
		projectAtoms, err := c.loadAtomsFromDB(ctx, c.projectDB)
		if err != nil {
			logging.Get(logging.CategoryJIT).Warn("Failed to load project atoms: %v", err)
		} else {
			breakdown.project = len(projectAtoms)
			allAtoms = append(allAtoms, projectAtoms...)
		}
	}

	// 3. Shard-specific database (if shard context is set)
	if cc.ShardID != "" {
		if shardDB, ok := c.shardDBs[cc.ShardID]; ok {
			shardAtoms, err := c.loadAtomsFromDB(ctx, shardDB)
			if err != nil {
				logging.Get(logging.CategoryJIT).Warn("Failed to load shard atoms: %v", err)
			} else {
				breakdown.shard = len(shardAtoms)
				allAtoms = append(allAtoms, shardAtoms...)
			}
		}
	}

	return allAtoms, breakdown, nil
}

// buildResultWithStats constructs the compilation result with comprehensive stats.
func (c *JITPromptCompiler) buildResultWithStats(
	candidates []*PromptAtom,
	scored []*ScoredAtom,
	fitted []*OrderedAtom,
	prompt string,
	budget int,
	stats *CompilationStats,
) *CompilationResult {
	result := &CompilationResult{
		Prompt:          prompt,
		IncludedAtoms:   make([]*PromptAtom, 0, len(fitted)),
		CategoryTokens:  make(map[AtomCategory]int),
		AtomsCandidates: len(candidates),
		AtomsSelected:   len(scored),
		AtomsIncluded:   len(fitted),
	}

	// Count atoms and tokens by type
	var skeletonTokens, fleshTokens int
	var standardCount, conciseCount, minCount int

	for _, oa := range fitted {
		result.IncludedAtoms = append(result.IncludedAtoms, oa.Atom)
		result.TotalTokens += oa.Atom.TokenCount
		result.CategoryTokens[oa.Atom.Category] += oa.Atom.TokenCount

		if oa.Atom.IsMandatory {
			result.MandatoryCount++
			skeletonTokens += oa.Atom.TokenCount
		} else {
			result.OptionalCount++
			fleshTokens += oa.Atom.TokenCount
		}

		// Track render mode distribution
		switch oa.RenderMode {
		case "concise":
			conciseCount++
		case "min":
			minCount++
		default:
			standardCount++
		}
	}

	if budget > 0 {
		result.BudgetUsed = float64(result.TotalTokens) / float64(budget)
	}

	result.CompilationTimeMs = stats.Duration.Milliseconds()

	// Update stats with computed values
	stats.AtomsSelected = len(fitted)
	stats.SkeletonAtoms = result.MandatoryCount
	stats.FleshAtoms = result.OptionalCount
	stats.AtomsDropped = len(candidates) - len(fitted)
	stats.TokensUsed = result.TotalTokens
	stats.BudgetUtilization = result.BudgetUsed
	stats.SkeletonTokens = skeletonTokens
	stats.FleshTokens = fleshTokens
	stats.StandardModeCount = standardCount
	stats.ConciseModeCount = conciseCount
	stats.MinModeCount = minCount

	// Build Manifest
	manifest := c.buildManifest(candidates, scored, fitted, budget)
	result.Manifest = manifest
	result.Stats = stats

	return result
}

// buildManifest constructs the prompt manifest for flight recording.
func (c *JITPromptCompiler) buildManifest(
	candidates []*PromptAtom,
	scored []*ScoredAtom,
	fitted []*OrderedAtom,
	budget int,
) *PromptManifest {
	manifest := &PromptManifest{
		Timestamp:   time.Now(),
		BudgetLimit: budget,
		Selected:    make([]AtomManifestEntry, 0, len(fitted)),
		Dropped:     make([]DroppedAtomEntry, 0),
	}

	// Calculate total tokens
	for _, oa := range fitted {
		manifest.TokenUsage += oa.Atom.TokenCount
	}

	// Lookup map for scores
	scoreMap := make(map[string]*ScoredAtom)
	for _, sa := range scored {
		scoreMap[sa.Atom.ID] = sa
	}

	// Lookup map for fitted
	fittedMap := make(map[string]bool)
	for _, oa := range fitted {
		fittedMap[oa.Atom.ID] = true

		// Find scores
		var logic, vector float64
		source := "flesh"
		if sa, ok := scoreMap[oa.Atom.ID]; ok {
			logic = sa.LogicScore
			vector = sa.VectorScore
			source = sa.Source
		}

		if oa.Atom.IsMandatory {
			source = "skeleton"
		}

		manifest.Selected = append(manifest.Selected, AtomManifestEntry{
			ID:          oa.Atom.ID,
			Category:    string(oa.Atom.Category),
			Source:      source,
			Priority:    oa.Atom.Priority,
			LogicScore:  logic,
			VectorScore: vector,
			RenderMode:  oa.RenderMode,
		})
	}

	// Identify dropped atoms
	// 1. Candidates rejected by Selector (Logic/Vector)
	for _, cand := range candidates {
		if _, ok := scoreMap[cand.ID]; !ok {
			manifest.Dropped = append(manifest.Dropped, DroppedAtomEntry{
				ID:     cand.ID,
				Reason: "Selector (Logic/Vector)",
			})
		}
	}

	// 2. Scored atoms rejected by Budget/Resolver
	for _, sa := range scored {
		if !fittedMap[sa.Atom.ID] {
			manifest.Dropped = append(manifest.Dropped, DroppedAtomEntry{
				ID:     sa.Atom.ID,
				Reason: "Budget/Dependency",
			})
		}
	}

	return manifest
}

// logCompilationStats logs comprehensive stats at the end of compilation.
func (c *JITPromptCompiler) logCompilationStats(stats *CompilationStats, result *CompilationResult) {
	logger := logging.Get(logging.CategoryJIT)

	// Summary line (INFO level)
	logger.Info("%s", stats.String())

	// Detailed breakdown (DEBUG level)
	logger.Debug(
		"Phase timing: collect=%dms select=%dms resolve=%dms fit=%dms assemble=%dms",
		stats.CollectAtomsMs, stats.SelectAtomsMs, stats.ResolveDepsMs,
		stats.FitBudgetMs, stats.AssembleMs,
	)

	logger.Debug(
		"Source breakdown: embedded=%d project=%d shard=%d",
		stats.EmbeddedAtoms, stats.ProjectAtoms, stats.ShardAtoms,
	)

	logger.Debug(
		"Render modes: standard=%d concise=%d min=%d",
		stats.StandardModeCount, stats.ConciseModeCount, stats.MinModeCount,
	)

	logger.Debug(
		"Token distribution: skeleton=%d flesh=%d total=%d/%d (%.1f%%)",
		stats.SkeletonTokens, stats.FleshTokens,
		stats.TokensUsed, stats.TokenBudget, stats.BudgetUtilization*100,
	)

	// Category breakdown (DEBUG level)
	if len(result.CategoryTokens) > 0 {
		for cat, tokens := range result.CategoryTokens {
			logger.Debug("Category %s: %d tokens", cat, tokens)
		}
	}

	// Structured log for machine parsing (if JSON mode enabled)
	logger.StructuredLog("info", "compilation_complete", stats.ToLogFields())
}

// GetLastResult returns the most recent compilation result.
// This is used by the TUI Prompt Inspector for observability.
func (c *JITPromptCompiler) GetLastResult() *CompilationResult {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.lastResult

}

// loadAtomsFromDB loads atoms from a SQLite database.
func (c *JITPromptCompiler) loadAtomsFromDB(ctx context.Context, db *sql.DB) ([]*PromptAtom, error) {
	if db == nil {
		return nil, nil
	}

	timer := logging.StartTimer(logging.CategoryContext, "JITPromptCompiler.loadAtomsFromDB")
	defer timer.Stop()

	// 1. Load Base Atoms
	query := `
		SELECT atom_id, version, content, token_count, content_hash,
		       description, content_concise, content_min,
		       category, subcategory, priority, is_mandatory, is_exclusive, created_at
		FROM prompt_atoms
	`

	rows, err := db.QueryContext(ctx, query)
	if err != nil {
		return nil, fmt.Errorf("failed to query atoms: %w", err)
	}
	defer rows.Close()

	var atoms []*PromptAtom
	atomMap := make(map[string]*PromptAtom)

	for rows.Next() {
		atom, err := scanAtom(rows)
		if err != nil {
			logging.Get(logging.CategoryContext).Warn("Failed to scan atom: %v", err)
			continue
		}
		atoms = append(atoms, atom)
		atomMap[atom.ID] = atom
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("error iterating atoms: %w", err)
	}

	// 2. Load Context Tags
	tagRows, err := db.QueryContext(ctx, "SELECT atom_id, dimension, tag FROM atom_context_tags")
	if err != nil {
		// Log warning but don't fail, maybe table is empty or migration pending
		logging.Get(logging.CategoryContext).Warn("Failed to query atom tags: %v", err)
		return atoms, nil
	}
	defer tagRows.Close()

	var atomID, dim, tag string
	for tagRows.Next() {
		if err := tagRows.Scan(&atomID, &dim, &tag); err != nil {
			continue
		}

		if atom, exists := atomMap[atomID]; exists {
			c.appendTag(atom, dim, tag)
		}
	}

	return atoms, nil
}

// scanAtom scans a database row into a PromptAtom.
func scanAtom(rows *sql.Rows) (*PromptAtom, error) {
	var atom PromptAtom
	var category string
	var desc, conc, min, sub, excl sql.NullString
	// Note: depends_on and conflicts_with are also relations now (via tags or separate table)
	// We load them via tags if we stored them there.

	err := rows.Scan(
		&atom.ID, &atom.Version, &atom.Content, &atom.TokenCount, &atom.ContentHash,
		&desc, &conc, &min,
		&category, &sub, &atom.Priority, &atom.IsMandatory, &excl, &atom.CreatedAt,
	)
	if err != nil {
		return nil, err
	}

	atom.Category = AtomCategory(category)
	atom.Subcategory = sub.String
	atom.Description = desc.String
	atom.ContentConcise = conc.String
	atom.ContentMin = min.String
	atom.IsExclusive = excl.String

	return &atom, nil
}

// appendTag helper to hydrate atom slices based on dimension
func (c *JITPromptCompiler) appendTag(atom *PromptAtom, dim, tag string) {
	switch dim {
	case "mode":
		atom.OperationalModes = append(atom.OperationalModes, tag)
	case "phase":
		atom.CampaignPhases = append(atom.CampaignPhases, tag)
	case "layer":
		atom.BuildLayers = append(atom.BuildLayers, tag)
	case "init_phase":
		atom.InitPhases = append(atom.InitPhases, tag)
	case "northstar_phase":
		atom.NorthstarPhases = append(atom.NorthstarPhases, tag)
	case "ouroboros_stage":
		atom.OuroborosStages = append(atom.OuroborosStages, tag)
	case "intent":
		atom.IntentVerbs = append(atom.IntentVerbs, tag)
	case "shard":
		atom.ShardTypes = append(atom.ShardTypes, tag)
	case "lang":
		atom.Languages = append(atom.Languages, tag)
	case "framework":
		atom.Frameworks = append(atom.Frameworks, tag)
	case "state":
		atom.WorldStates = append(atom.WorldStates, tag)
	case "depends_on":
		atom.DependsOn = append(atom.DependsOn, tag)
	case "conflicts_with":
		atom.ConflictsWith = append(atom.ConflictsWith, tag)
	}
}

// RegisterDB registers a named database with the JIT compiler.
// Known names:
//   - "corpus": Sets the project-level corpus database (embedded atoms synced to SQLite)
//   - "project": Alias for "corpus"
//
// The method opens the database file and registers it. The caller is responsible
// for ensuring the file exists. Call Close() to release all DB connections.
func (c *JITPromptCompiler) RegisterDB(name, dbPath string) error {
	db, err := sql.Open("sqlite3", dbPath)
	if err != nil {
		return fmt.Errorf("failed to open database %s: %w", dbPath, err)
	}

	// Verify connection is valid
	if pingErr := db.Ping(); pingErr != nil {
		db.Close()
		return fmt.Errorf("failed to ping database %s: %w", dbPath, pingErr)
	}

	c.mu.Lock()
	defer c.mu.Unlock()

	switch name {
	case "corpus", "project":
		// Close existing project DB if any
		if c.projectDB != nil {
			c.projectDB.Close()
		}
		c.projectDB = db
		logging.Get(logging.CategoryContext).Info("Registered corpus database: %s", dbPath)
	default:
		// Treat unknown names as shard IDs for flexibility
		c.shardDBs[name] = db
		logging.Get(logging.CategoryContext).Info("Registered database %s: %s", name, dbPath)
	}

	return nil
}

// RegisterShardDB registers a shard-specific atom database.
// The DB should be the agent's unified knowledge database (.nerd/shards/{name}_knowledge.db)
// which contains both knowledge_atoms and prompt_atoms tables.
func (c *JITPromptCompiler) RegisterShardDB(shardID string, db *sql.DB) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.shardDBs[shardID] = db
}

// UnregisterShardDB removes a shard database registration.
func (c *JITPromptCompiler) UnregisterShardDB(shardID string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	delete(c.shardDBs, shardID)
}

// LoadAtoms loads atoms from a database into memory.
// This is useful for pre-loading atoms for faster compilation.
func (c *JITPromptCompiler) LoadAtoms(ctx context.Context, db *sql.DB) ([]*PromptAtom, error) {
	return c.loadAtomsFromDB(ctx, db)
}

// GetConfig returns the current compiler configuration.
func (c *JITPromptCompiler) GetConfig() CompilerConfig {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.config
}

// SetConfig updates the compiler configuration.
func (c *JITPromptCompiler) SetConfig(config CompilerConfig) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.config = config
}

// SetLocalDB sets the LocalStore for semantic knowledge atom queries.
// This enables the Semantic Knowledge Bridge, allowing JIT to query
// knowledge atoms with embeddings for context-aware prompt assembly.
func (c *JITPromptCompiler) SetLocalDB(db *store.LocalStore) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.localDB = db
}

// collectKnowledgeAtoms queries the LocalStore for semantically relevant knowledge atoms
// and converts them to ephemeral PromptAtoms for JIT compilation.
// This is the core of the Semantic Knowledge Bridge - connecting stored documentation
// knowledge to runtime prompt assembly.
func (c *JITPromptCompiler) collectKnowledgeAtoms(ctx context.Context, cc *CompilationContext) []*PromptAtom {
	c.mu.RLock()
	db := c.localDB
	c.mu.RUnlock()

	if db == nil || cc == nil {
		return nil
	}

	// Build semantic query from compilation context
	// Combine intent, shard type, and language for best semantic match
	var queryParts []string
	if cc.IntentVerb != "" {
		queryParts = append(queryParts, cc.IntentVerb)
	}
	if cc.IntentTarget != "" {
		queryParts = append(queryParts, cc.IntentTarget)
	}
	if cc.ShardID != "" {
		queryParts = append(queryParts, cc.ShardID)
	}
	if cc.Language != "" {
		queryParts = append(queryParts, cc.Language)
	}
	if len(cc.Frameworks) > 0 {
		queryParts = append(queryParts, cc.Frameworks...)
	}

	if len(queryParts) == 0 {
		return nil
	}

	query := strings.Join(queryParts, " ")

	// Search for semantically relevant knowledge atoms
	atoms, err := db.SearchKnowledgeAtomsSemantic(ctx, query, 5)
	if err != nil {
		logging.Get(logging.CategoryJIT).Debug("Knowledge atom search failed: %v", err)
		return nil
	}

	if len(atoms) == 0 {
		return nil
	}

	// Convert to ephemeral PromptAtoms
	var result []*PromptAtom
	for _, atom := range atoms {
		// Format content with concept context
		content := atom.Content
		if atom.Concept != "" {
			// Extract meaningful category from concept (e.g., "doc/path/architecture/patterns" -> "architecture/patterns")
			parts := strings.Split(atom.Concept, "/")
			if len(parts) >= 3 {
				category := strings.Join(parts[2:], "/")
				content = fmt.Sprintf("[%s] %s", category, atom.Content)
			}
		}

		// Create prompt atom with appropriate priority
		// Priority 85 = below specialist_knowledge (90) but above regular context
		atomID := fmt.Sprintf("knowledge/%s", HashContent(content)[:8])
		pa := NewPromptAtom(atomID, CategoryKnowledge, content)
		pa.Priority = 85
		pa.IsMandatory = false // Knowledge is contextual, not mandatory
		if cc.ShardID != "" {
			pa.ShardTypes = []string{cc.ShardID}
		}

		result = append(result, pa)
	}

	logging.Get(logging.CategoryJIT).Debug(
		"Collected %d knowledge atoms for query: %s",
		len(result), truncateQuery(query, 50))

	return result
}

// truncateQuery truncates a query string for logging.
func truncateQuery(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen] + "..."
}

// Stats returns compilation statistics.
type CompilerStats struct {
	EmbeddedAtomCount int
	ProjectAtomCount  int
	ShardDBCount      int
	TotalCompilations int64
	AverageTimeMs     float64
}

// GetStats returns current compiler statistics.
func (c *JITPromptCompiler) GetStats() CompilerStats {
	c.mu.RLock()
	defer c.mu.RUnlock()

	stats := CompilerStats{
		ShardDBCount: len(c.shardDBs),
	}

	if c.embeddedCorpus != nil {
		stats.EmbeddedAtomCount = c.embeddedCorpus.Count()
	}

	return stats
}

// AssertFacts asserts raw Mangle fact strings into the kernel if available.
// This is used for telemetry/observability (e.g., jit_fallback/2).
// Facts may omit trailing periods; they will be normalized by the kernel adapter.
func (c *JITPromptCompiler) AssertFacts(facts []string) error {
	if c == nil || c.kernel == nil || len(facts) == 0 {
		return nil
	}
	return c.kernel.AssertBatch(toInterfaceSlice(facts))
}

// Close releases all resources held by the compiler.
func (c *JITPromptCompiler) Close() error {
	c.mu.Lock()
	defer c.mu.Unlock()

	// Close shard databases
	for id, db := range c.shardDBs {
		if err := db.Close(); err != nil {
			logging.Get(logging.CategoryContext).Warn("Failed to close shard DB %s: %v", id, err)
		}
	}
	c.shardDBs = make(map[string]*sql.DB)

	// Close project database
	if c.projectDB != nil {
		if err := c.projectDB.Close(); err != nil {
			return fmt.Errorf("failed to close project DB: %w", err)
		}
		c.projectDB = nil
	}

	return nil
}

// InjectAvailableSpecialists populates the context with discovered specialists.
// This enables the LLM to know what domain experts are available for consultation.
// Reads from .nerd/agents.json and formats as a markdown list for template injection.
func InjectAvailableSpecialists(ctx *CompilationContext, workspace string) error {
	if ctx == nil || workspace == "" {
		return nil
	}

	registryPath := filepath.Join(workspace, ".nerd", "agents.json")
	data, err := os.ReadFile(registryPath)
	if err != nil {
		// Graceful degradation - no specialists available
		ctx.AvailableSpecialists = "- **researcher**: Deep web research and documentation gathering\n- **reviewer**: Code review and analysis"
		return nil
	}

	// Parse the agent registry
	var registry struct {
		Agents []struct {
			Name        string `json:"name"`
			Type        string `json:"type"`
			Status      string `json:"status"`
			Description string `json:"description"`
			Topics      string `json:"topics"`
		} `json:"agents"`
	}
	if err := json.Unmarshal(data, &registry); err != nil {
		logging.Get(logging.CategoryJIT).Warn("Failed to parse agents.json: %v", err)
		ctx.AvailableSpecialists = "- **researcher**: Deep web research and documentation gathering\n- **reviewer**: Code review and analysis"
		return nil
	}

	// Build specialist descriptions
	var specialists []string
	for _, agent := range registry.Agents {
		if agent.Status != "ready" {
			continue
		}
		desc := agent.Description
		if desc == "" && agent.Topics != "" {
			desc = fmt.Sprintf("%s specialist (%s)", agent.Type, agent.Topics)
		} else if desc == "" {
			desc = fmt.Sprintf("%s domain specialist", agent.Type)
		}
		specialists = append(specialists, fmt.Sprintf("- **%s**: %s", agent.Name, desc))
	}

	// Add core shards as implicit specialists
	coreShards := []string{
		"- **researcher**: Deep web research and documentation gathering (Context7, GitHub, web search)",
		"- **reviewer**: Code review, hypothesis verification, and security analysis",
		"- **codebase**: Search within project files for patterns and implementations",
	}
	specialists = append(specialists, coreShards...)

	if len(specialists) == 0 {
		ctx.AvailableSpecialists = "No specialists available. Use **researcher** for general knowledge gathering."
	} else {
		ctx.AvailableSpecialists = strings.Join(specialists, "\n")
	}

	logging.Get(logging.CategoryJIT).Debug("Injected %d available specialists into context", len(specialists))
	return nil
}

// toInterfaceSlice converts a string slice to an interface slice.
// Used to pass context facts to the kernel's AssertBatch method.
func toInterfaceSlice(strs []string) []interface{} {
	result := make([]interface{}, len(strs))
	for i, s := range strs {
		result[i] = s
	}
	return result
}
