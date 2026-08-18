package prompt

import (
	"container/list"
	"context"
	"database/sql"
	"fmt"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"golang.org/x/sync/singleflight"

	"codenerd/internal/jit/config"
	"codenerd/internal/logging"
	"codenerd/internal/store"
	"codenerd/internal/transparency"

	"golang.org/x/sync/errgroup"
)

// Fact represents a logic fact (predicate + args).
type Fact struct {
	Predicate string
	Args      []any
}

// KernelQuerier defines the interface for querying the Mangle kernel.
// This abstracts the kernel to avoid circular imports.
type KernelQuerier interface {
	// Query retrieves facts matching a predicate.
	Query(predicate string) ([]Fact, error)

	// AssertBatch adds multiple facts efficiently.
	AssertBatch(facts []any) error
}

// KernelRetracter is an optional extension of KernelQuerier for predicate retraction.
// Implementations that support it (e.g., RealKernel) enable cleanup of ephemeral facts.
type KernelRetracter interface {
	Retract(predicate string) error
}

// KernelCompilationScope is an isolated kernel view used for one prompt
// compilation. Closing the scope must discard every fact asserted through it.
type KernelCompilationScope interface {
	KernelQuerier
	Close() error
}

// KernelScopeProvider creates per-compilation kernel views. Production kernel
// adapters implement this using a snapshot/clone so parallel prompt compiles
// never share selector facts.
type KernelScopeProvider interface {
	NewCompilationScope() (KernelCompilationScope, error)
}

var promptEphemeralPredicates = []string{
	"compile_context",
	"current_context",
	"atom",
	"prompt_atom",
	"atom_category",
	"atom_priority",
	"atom_tag",
	"is_mandatory",
	"vector_hit",
	"atom_requires",
	"atom_conflicts",
}

// VectorSearcher defines the interface for semantic search.
type VectorSearcher interface {
	// Search performs semantic search and returns atom IDs with scores.
	Search(ctx context.Context, query string, limit int) ([]SearchResult, error)

	// EmbedQuery generates an embedding vector for the given query.
	EmbedQuery(ctx context.Context, query string) ([]float32, error)
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

	// EvolvedAtoms is count of atoms from prompt evolution (SPL system)
	EvolvedAtoms int

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
	var util float64
	if s.TokenBudget > 0 {
		util = s.BudgetUtilization * 100
	}
	return fmt.Sprintf(
		"JIT[%s] %dms | atoms=%d (skel=%d flesh=%d) | tokens=%d/%d (%.1f%%) | vector=%dms | cache=%v",
		s.ShardID,
		s.Duration.Milliseconds(),
		s.AtomsSelected,
		s.SkeletonAtoms,
		s.FleshAtoms,
		s.TokensUsed,
		s.TokenBudget,
		util,
		s.VectorQueryMs,
		s.CacheHit,
	)
}

// ToLogFields returns the stats as a map for structured logging.
func (s *CompilationStats) ToLogFields() map[string]any {
	return map[string]any{
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
	TotalTokens int
	BudgetUsed  float64 // Percentage of budget used
	// MandatoryCount / OptionalCount hold the skeleton / flesh split by
	// CATEGORY (isSkeletonCategory), which is what the stats derived from them
	// are labelled as. The names predate the skeleton/flesh vocabulary; they do
	// NOT count the IsMandatory flag, and classifying on that flag is what made
	// the JIT summary report flesh=0 while flesh atoms were being selected.
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

	// JIT-generated Agent Config
	EffectiveAgentRuntimeConfig *config.EffectiveAgentRuntimeConfig

	// Comprehensive compilation statistics
	Stats *CompilationStats
}

// JITPromptCompiler compiles optimal system prompts from atomic fragments.
// It combines rule-based selection (Mangle) with semantic search (vectors)
// to select the most relevant prompt atoms for a given context.

// promptCacheEntry holds a cached compilation result and its key.
type promptCacheEntry struct {
	key    string
	result *CompilationResult
}

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

	// Bug #5 fix: Prompt cache to prevent recompilation spam
	cache      map[string]*list.Element
	cacheList  *list.List
	cacheLimit int
	cacheMu    sync.Mutex
	cacheHits  int64
	cacheMiss  int64

	// Observability
	lastResult atomic.Pointer[CompilationResult]

	// Concurrency control
	configMu     sync.RWMutex
	dbMu         sync.RWMutex
	shardMu      sync.RWMutex
	compileGroup singleflight.Group
	wg           sync.WaitGroup

	// Configuration
	config CompilerConfig

	// ConfigFactory for generating AgentConfigs
	configFactory *ConfigFactory

	// LocalDB for semantic knowledge atom queries (Semantic Knowledge Bridge)
	localDB *store.LocalStore

	// LearningStore for recalling learned intents and feedback (Semantic Knowledge Bridge)
	learningStore *store.LearningStore

	// Evolved atoms manager for System Prompt Learning (SPL)
	evolvedAtomMgr *EvolvedAtomManager
}

// CompilerConfig holds configuration for the JIT compiler.
type CompilerConfig struct {
	// DefaultTokenBudget is the default token budget if not specified in context
	DefaultTokenBudget int

	// EnableVectorSearch enables semantic search for atom selection
	EnableVectorSearch bool

	// VectorSearchWeight is the weight of vector scores vs logic scores (0.0-1.0)
	VectorSearchWeight float64

	// VectorSearchTimeout is the timeout duration for vector searches
	VectorSearchTimeout time.Duration

	// MaxAtomsPerCategory caps atoms selected per category
	MaxAtomsPerCategory int

	// EnableCaching enables caching of compiled prompts
	EnableCaching bool

	// CacheTTLSeconds is the cache TTL in seconds
	CacheTTLSeconds int

	// DebugMode enables verbose JIT manifest logging
	DebugMode bool

	// KnowledgeSearchTimeout is the max time to wait for knowledge atom embedding and search
	KnowledgeSearchTimeout time.Duration
}

// DefaultCompilerConfig returns a sensible default configuration.
// Note: DefaultTokenBudget should be overridden via WithDefaultTokenBudget() from config.ContextWindow.MaxTokens.
func DefaultCompilerConfig() CompilerConfig {
	return CompilerConfig{
		DefaultTokenBudget:     200000, // 200k tokens default - callers should override from config
		EnableVectorSearch:     true,
		VectorSearchWeight:     0.3, // 70% logic, 30% vector
		MaxAtomsPerCategory:    10,
		EnableCaching:          true,
		CacheTTLSeconds:        300, // 5 minutes
		DebugMode:              false,
		KnowledgeSearchTimeout: 10 * time.Second,
	}
}

// NewJITPromptCompiler creates a new JIT prompt compiler.
func NewJITPromptCompiler(opts ...CompilerOption) (*JITPromptCompiler, error) {
	config := DefaultCompilerConfig()

	compiler := &JITPromptCompiler{
		shardDBs:   make(map[string]*sql.DB),
		config:     config,
		selector:   NewAtomSelector(),
		resolver:   NewDependencyResolver(),
		budgetMgr:  NewTokenBudgetManager(),
		assembler:  NewFinalAssembler(),
		cache:      make(map[string]*list.Element),
		cacheList:  list.New(),
		cacheLimit: 1000, // Hard size limit for LRU cache
	}

	// Apply options
	for _, opt := range opts {
		if opt == nil {
			continue
		}
		if err := opt(compiler); err != nil {
			return nil, fmt.Errorf("failed to apply compiler option: %w", err)
		}
	}

	// Ensure selector has the timeout from config
	compiler.selector.SetVectorSearchTimeout(compiler.config.VectorSearchTimeout)

	logging.Get(logging.CategoryContext).Info("JITPromptCompiler initialized")
	return compiler, nil
}

// This is the main entry point for prompt compilation.
func (c *JITPromptCompiler) Compile(ctx context.Context, cc *CompilationContext) (*CompilationResult, error) {

	c.wg.Add(1)
	defer c.wg.Done()

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

	// Compile against a private snapshot. The assembler may enrich this copy
	// (for example, by resolving available specialists), but caller-owned state
	// and the cache identity remain stable and race-free.
	cc = cc.Clone()
	if strings.TrimSpace(cc.AvailableSpecialists) == "" {
		if err := InjectAvailableSpecialists(cc, ""); err != nil {
			logging.Get(logging.CategoryJIT).Debug("Failed to resolve specialists before cache lookup: %v", err)
		}
	}

	// Bug #5 fix: Check cache before compilation
	cacheKey := cc.Hash()
	c.cacheMu.Lock()
	if elem, ok := c.cache[cacheKey]; ok {
		c.cacheList.MoveToFront(elem)
		cached := elem.Value.(*promptCacheEntry).result
		c.cacheMu.Unlock()
		atomic.AddInt64(&c.cacheHits, 1)
		logging.Get(logging.CategoryJIT).Info("Prompt cache HIT for %s (hash=%s, hits=%d)",
			cc.String(), cacheKey[:8], atomic.LoadInt64(&c.cacheHits))
		return cached, nil
	}
	c.cacheMu.Unlock()
	// Singleflight to prevent Thundering Herd
	v, err, shared := c.compileGroup.Do(cacheKey, func() (any, error) {
		atomic.AddInt64(&c.cacheMiss, 1)

		selectionKernel, releaseKernel, err := acquireCompilationKernel(c.kernel)
		if err != nil {
			return nil, fmt.Errorf("create isolated prompt compilation scope: %w", err)
		}
		defer releaseKernel()

		if err := ctx.Err(); err != nil {
			return nil, fmt.Errorf("prompt compilation canceled before selection: %w", err)
		}

		// Start comprehensive timing after validation
		compileStart := time.Now()
		stats := &CompilationStats{
			ShardID:         cc.ShardID,
			OperationalMode: cc.OperationalMode,
			IntentVerb:      cc.IntentVerb,
		}

		logging.Get(logging.CategoryJIT).Info("Compiling prompt (cache MISS): %s (hash=%s, misses=%d)",
			cc.String(), cacheKey[:8], atomic.LoadInt64(&c.cacheMiss))

		// Step 1: Assert context facts to kernel for Mangle-based selection
		// We do this first so that context is available for kernel injection and Mangle-based selection.
		// This enables policy.mg Section 45/46 rules to boost atoms matching current context.
		if selectionKernel != nil {
			contextFacts := cc.ToContextFacts()
			if len(contextFacts) > 0 {
				if err := selectionKernel.AssertBatch(contextFacts); err != nil {
					logging.Get(logging.CategoryJIT).Warn("Failed to assert context facts: %v", err)
					// Non-fatal - continue without context-based boosting
				} else {
					logging.Get(logging.CategoryJIT).Debug("Asserted %d context facts to kernel", len(contextFacts))
				}
			}
		}

		// Step 1.5: Collect all candidate atoms from all sources concurrently
		collectStart := time.Now()

		g, gCtx := errgroup.WithContext(ctx)

		var candidates []*PromptAtom
		var sourceBreakdown sourceBreakdown

		var dynamicAtoms []*PromptAtom
		var knowledgeAtoms []*PromptAtom
		var learningAtoms []*PromptAtom

		// 1.5a: Collect static/selected atoms
		g.Go(func() error {
			var err error
			candidates, sourceBreakdown, err = c.collectAtomsWithStats(gCtx, cc)
			if err != nil {
				return fmt.Errorf("failed to collect atoms: %w", err)
			}
			return nil
		})

		// 1.5b: Collect dynamic kernel-injected atoms (injectable_context, specialist_knowledge)
		g.Go(func() error {
			var dynErr error
			dynamicAtoms, dynErr = c.collectKernelInjectedAtoms(cc)
			if dynErr != nil {
				logging.Get(logging.CategoryJIT).Warn("Failed to collect kernel-injected atoms: %v", dynErr)
			}
			return nil // Non-fatal
		})

		// 1.5c: Collect semantic knowledge atoms (Semantic Knowledge Bridge)
		g.Go(func() error {
			knowledgeAtoms = c.collectKnowledgeAtoms(gCtx, cc)
			return nil // Non-fatal
		})

		// 1.5d: Collect learning atoms from autopoiesis memory (Semantic Knowledge Bridge)
		g.Go(func() error {
			learningAtoms = c.collectLearningAtoms(gCtx, cc)
			return nil // Non-fatal
		})

		if err := g.Wait(); err != nil {
			return nil, err
		}
		if err := ctx.Err(); err != nil {
			return nil, fmt.Errorf("prompt compilation canceled while collecting atoms: %w", err)
		}

		// Merge concurrent results
		if len(dynamicAtoms) > 0 {
			candidates = append(candidates, dynamicAtoms...)
			logging.Get(logging.CategoryJIT).Debug("Appended %d kernel-injected atoms to candidates", len(dynamicAtoms))
		}

		if len(knowledgeAtoms) > 0 {
			candidates = append(candidates, knowledgeAtoms...)
			logging.Get(logging.CategoryJIT).Debug("Appended %d semantic knowledge atoms to candidates", len(knowledgeAtoms))
		}

		if len(learningAtoms) > 0 {
			candidates = append(candidates, learningAtoms...)
			logging.Get(logging.CategoryJIT).Debug("Appended %d learning atoms to candidates", len(learningAtoms))
		}

		stats.CollectAtomsMs = time.Since(collectStart).Milliseconds()
		stats.AtomsCandidates = len(candidates)
		stats.EmbeddedAtoms = sourceBreakdown.embedded
		stats.ProjectAtoms = sourceBreakdown.project
		stats.ShardAtoms = sourceBreakdown.shard
		stats.EvolvedAtoms = sourceBreakdown.evolved

		logging.Get(logging.CategoryJIT).Debug(
			"Collected %d candidate atoms (embedded=%d, project=%d, shard=%d, evolved=%d) in %dms",
			len(candidates), sourceBreakdown.embedded, sourceBreakdown.project, sourceBreakdown.shard,
			sourceBreakdown.evolved, stats.CollectAtomsMs,
		)

		// Step 2: Select atoms based on context (Mangle rules + vector search)
		selectStart := time.Now()
		scored, vectorMs, err := c.selector.selectAtomsWithTimingKernel(ctx, candidates, cc, selectionKernel)
		if err != nil {
			return nil, fmt.Errorf("failed to select atoms: %w", err)
		}
		if err := ctx.Err(); err != nil {
			return nil, fmt.Errorf("prompt compilation canceled during selection: %w", err)
		}
		stats.SelectAtomsMs = time.Since(selectStart).Milliseconds()
		stats.VectorQueryMs = vectorMs

		// NOTE: compile_context retraction is now handled by defer (see Step 1.5 above).
		// This ensures cleanup even if SelectAtomsWithTiming fails.

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

		// Defensive: a zero-value or partially-constructed compiler may have no
		// budget manager. Fall back to a default rather than nil-panicking.
		budgetMgr := c.budgetMgr
		if budgetMgr == nil {
			budgetMgr = NewTokenBudgetManager()
		}
		fitted, err := budgetMgr.Fit(ordered, budget)
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
		if result.Manifest != nil {
			result.Manifest.ContextHash = cacheKey
		}

		// Step 6: Generate Agent Config if factory is present
		if c.configFactory != nil {
			intents := []string{}
			if cc.IntentVerb != "" {
				intents = append(intents, cc.IntentVerb)
			}
			if cc.ShardType != "" {
				intents = append(intents, cc.ShardType)
			}

			if len(intents) > 0 {
				agentCfg, err := c.configFactory.Generate(ctx, result, intents...)
				if err != nil {
					logging.Get(logging.CategoryJIT).Warn("Failed to generate agent config: %v", err)
				} else {
					result.EffectiveAgentRuntimeConfig = agentCfg
					logging.Get(logging.CategoryJIT).Debug("Generated JIT Agent Config for intents: %v", intents)
				}
			}
		}

		// Update observability state
		// Performance: Uses an atomic pointer instead of coarse-grained locking to avoid blocking other compilations during stats updates.
		c.lastResult.Store(result)

		// Bug #5 fix: Store result in cache for future reuse
		c.cacheMu.Lock()
		if elem, ok := c.cache[cacheKey]; ok {
			c.cacheList.MoveToFront(elem)
			elem.Value.(*promptCacheEntry).result = result
		} else {
			entry := &promptCacheEntry{key: cacheKey, result: result}
			elem := c.cacheList.PushFront(entry)
			c.cache[cacheKey] = elem

			for c.cacheList.Len() > c.cacheLimit {
				oldElem := c.cacheList.Back()
				if oldElem != nil {
					c.cacheList.Remove(oldElem)
					oldEntry := oldElem.Value.(*promptCacheEntry)
					delete(c.cache, oldEntry.key)
				} else {
					break
				}
			}
		}
		c.cacheMu.Unlock()
		logging.Get(logging.CategoryJIT).Debug("Stored compiled prompt in cache (hash=%s)", cacheKey[:8])

		// Log comprehensive stats using JIT category
		c.logCompilationStats(stats, result)
		if c.config.DebugMode {
			c.logCompilationManifest(stats, result)
		}

		return result, nil
	})

	if err != nil {
		return nil, err
	}

	res := v.(*CompilationResult)
	if shared {
		logging.Get(logging.CategoryJIT).Info("Prompt compilation joined via singleflight for hash=%s", cacheKey[:8])
	}
	return res, nil
}

func acquireCompilationKernel(base KernelQuerier) (KernelQuerier, func(), error) {
	if base == nil {
		return nil, func() {}, nil
	}

	if provider, ok := base.(KernelScopeProvider); ok {
		scope, err := provider.NewCompilationScope()
		if err != nil {
			return nil, nil, err
		}
		if scope == nil {
			return nil, nil, fmt.Errorf("kernel scope provider returned nil scope")
		}
		return scope, func() {
			if err := scope.Close(); err != nil {
				logging.Get(logging.CategoryJIT).Warn("Failed to close prompt compilation scope: %v", err)
			}
		}, nil
	}

	// Compatibility path for lightweight adapters. Production uses an isolated
	// scope above; legacy kernels still get complete best-effort cleanup on
	// success, error, cancellation, and panic via the caller's defer.
	retracter, ok := base.(KernelRetracter)
	if !ok {
		return base, func() {}, nil
	}
	return base, func() {
		for _, predicate := range promptEphemeralPredicates {
			if err := retracter.Retract(predicate); err != nil {
				logging.Get(logging.CategoryJIT).Warn(
					"Failed to retract ephemeral prompt predicate %s: %v", predicate, err,
				)
			}
		}
	}, nil
}

// collectKernelInjectedAtoms queries runtime context predicates and turns them into ephemeral PromptAtoms.
// This lets JIT prompts include spreading-activation context and specialist knowledge without legacy injection.

// extractStringArgFast uses type switches for common primitives to avoid reflection
// and allocation overhead in hot loops, falling back to extractStringArg for complex types.
func extractStringArgFast(arg any) (string, error) {
	switch v := arg.(type) {
	case string:
		return v, nil
	case int:
		return strconv.Itoa(v), nil
	case int64:
		return strconv.FormatInt(v, 10), nil
	case float64:
		return strconv.FormatFloat(v, 'g', -1, 64), nil
	case bool:
		return strconv.FormatBool(v), nil
	default:
		return extractStringArg(arg)
	}
}

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

	dynamic := make([]*PromptAtom, 0, 2)

	// Injectable context atoms
	ctxFacts, err := c.kernel.Query("injectable_context")
	if err != nil {
		return nil, err
	}
	ctxAtoms := make([]string, 0, len(ctxFacts))
	for _, fact := range ctxFacts {
		if len(fact.Args) < 2 {
			continue
		}
		factShardID, err := extractStringArgFast(fact.Args[0])
		if err != nil || !matchesShard(factShardID) {
			continue
		}
		if atom, ok := fact.Args[1].(string); ok && strings.TrimSpace(atom) != "" {
			ctxAtoms = append(ctxAtoms, atom)
		} else if atomStr, err := extractStringArgFast(fact.Args[1]); err == nil && strings.TrimSpace(atomStr) != "" {
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
		id := "kernel/context/" + HashContent(content)[:8]
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
		blocks := make([]block, 0, len(knowledgeFacts))
		for _, fact := range knowledgeFacts {
			if len(fact.Args) < 3 {
				continue
			}
			factShardID, err := extractStringArgFast(fact.Args[0])
			if err != nil || !matchesShard(factShardID) {
				continue
			}
			topic, err1 := extractStringArgFast(fact.Args[1])
			body, err2 := extractStringArgFast(fact.Args[2])
			if err1 == nil && err2 == nil && strings.TrimSpace(topic) != "" && strings.TrimSpace(body) != "" {
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
			id := "kernel/knowledge/" + HashContent(content)[:8]
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
	evolved  int
}

// collectAtomsWithStats gathers atoms and tracks source breakdown.
func (c *JITPromptCompiler) collectAtomsWithStats(ctx context.Context, cc *CompilationContext) ([]*PromptAtom, sourceBreakdown, error) {
	var breakdown sourceBreakdown

	// Bolt: acquire locks only long enough to copy shared pointers. Database
	// queries must not hold compiler locks.
	c.dbMu.RLock()
	embeddedCorpus := c.embeddedCorpus
	projectDB := c.projectDB
	c.dbMu.RUnlock()

	var shardDB *sql.DB
	if cc.ShardID != "" {
		c.shardMu.RLock()
		shardDB = c.shardDBs[shardDBKey(cc.ShardID)]
		c.shardMu.RUnlock()
	}

	// Earlier sources are authoritative. In particular, the embedded corpus is
	// the canonical source for built-in atoms; the project database may contain
	// an older synchronized copy when embedding refresh is unavailable. Letting
	// both copies through can place contradictory instructions in one prompt.
	embeddedCount := 0
	if embeddedCorpus != nil {
		embeddedCount = embeddedCorpus.Count()
	}
	allAtoms := make([]*PromptAtom, 0, embeddedCount+100)
	seen := make(map[string]string, embeddedCount+100)
	appendSource := func(source string, atoms []*PromptAtom) (added, duplicates int) {
		for _, atom := range atoms {
			if atom == nil || atom.ID == "" {
				// Preserve existing invalid-atom handling in the selector/resolver.
				allAtoms = append(allAtoms, atom)
				added++
				continue
			}
			if _, exists := seen[atom.ID]; exists {
				duplicates++
				continue
			}
			seen[atom.ID] = source
			allAtoms = append(allAtoms, atom)
			added++
		}
		if duplicates > 0 {
			logging.Get(logging.CategoryJIT).Debug(
				"Skipped %d duplicate %s atom(s); earlier sources take precedence",
				duplicates, source,
			)
		}
		return added, duplicates
	}

	// 1. Embedded corpus: canonical built-in atoms always win duplicate IDs.
	if embeddedCorpus != nil {
		breakdown.embedded, _ = appendSource("embedded", embeddedCorpus.All())
	}

	// 2. Project database: adds project-only atoms, never stale built-in copies.
	if projectDB != nil {
		projectAtoms, err := c.loadAtomsFromDB(ctx, projectDB)
		if err != nil {
			logging.Get(logging.CategoryJIT).Warn("Failed to load project atoms: %v", err)
		} else {
			breakdown.project, _ = appendSource("project", projectAtoms)
		}
	}

	// 3. Shard-specific database: adds scoped atoms with unique IDs.
	if shardDB != nil {
		shardAtoms, err := c.loadAtomsFromDB(ctx, shardDB)
		if err != nil {
			logging.Get(logging.CategoryJIT).Warn("Failed to load shard atoms: %v", err)
		} else {
			breakdown.shard, _ = appendSource("shard", shardAtoms)
		}
	}

	// 4. Evolved atoms from SPL (System Prompt Learning).
	evolvedAtoms := c.collectEvolvedAtoms(cc)
	breakdown.evolved, _ = appendSource("evolved", evolvedAtoms)

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
		// Count the render-mode variant actually emitted (see contentForMode),
		// not the standard TokenCount — otherwise degraded (concise/min) atoms
		// over-report their token usage in the result stats.
		atomTokens := tokenCountForMode(oa.Atom, oa.RenderMode)
		result.TotalTokens += atomTokens
		result.CategoryTokens[oa.Atom.Category] += atomTokens

		// Skeleton vs flesh is a CATEGORY distinction (identity, protocol,
		// safety, methodology are skeleton — see isSkeletonCategory), not a
		// mandatory-flag distinction.
		//
		// These counters classified on oa.Atom.IsMandatory and were reported as
		// "skel=" / "flesh=" in the JIT summary line and as
		// skeleton_atoms/flesh_atoms in the compilation_complete event. Because
		// flesh-category atoms can also be mandatory, the summary printed
		// "atoms=23 (skel=23 flesh=0)" on a compilation whose own selector had
		// just logged "Merged 13 skeleton + 10 flesh = 23 total atoms".
		//
		// flesh=0 reads as "the probabilistic half of the architecture selected
		// nothing", which is a wrong and expensive conclusion to hand a reader
		// — it was drawn from this very line while diagnosing F-JIT-4. The two
		// logs now measure the same thing and must agree.
		if isSkeletonCategory(oa.Atom.Category) {
			result.MandatoryCount++
			skeletonTokens += atomTokens
		} else {
			result.OptionalCount++
			fleshTokens += atomTokens
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
	} else if budget <= 0 && result.TotalTokens > 0 {
		result.BudgetUsed = 1.0 // Over budget when budget is <= 0 and we have tokens
	} else {
		result.BudgetUsed = 0.0
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

	// Calculate total tokens using the render-mode variant actually emitted, so
	// the manifest matches contentForMode's output rather than over-reporting
	// the standard size of degraded (concise/min) atoms.
	for _, oa := range fitted {
		manifest.TokenUsage += tokenCountForMode(oa.Atom, oa.RenderMode)
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

// emitJITGlassBox publishes atom-selection telemetry to the Glass Box stream
// when the operator has JIT explain on.
//
// The compiler is constructed at boot before the Glass Box bus exists and
// takes no bus setter, so this goes through the process-wide transparency
// facade rather than an injected handle. It is a no-op (one atomic load) when
// JIT explain is off, which is the default.
func emitJITGlassBox(stats *CompilationStats, result *CompilationResult) {
	if stats == nil || !transparency.JITExplainEnabled() {
		return
	}

	source := stats.ShardID
	if source == "" {
		source = "jit"
	}
	summary := fmt.Sprintf("%d atoms · %d/%d tokens", stats.AtomsSelected, stats.TokensUsed, stats.TokenBudget)
	if stats.CacheHit {
		summary += " (cache hit)"
	}

	var details strings.Builder
	fmt.Fprintf(&details, "skeleton=%d flesh=%d candidates=%d dropped=%d\n",
		stats.SkeletonAtoms, stats.FleshAtoms, stats.AtomsCandidates, stats.AtomsDropped)
	fmt.Fprintf(&details, "sources: embedded=%d project=%d shard=%d evolved=%d\n",
		stats.EmbeddedAtoms, stats.ProjectAtoms, stats.ShardAtoms, stats.EvolvedAtoms)
	if stats.IntentVerb != "" {
		fmt.Fprintf(&details, "intent=%s\n", stats.IntentVerb)
	}
	if result != nil && result.Manifest != nil {
		ids := make([]string, 0, len(result.Manifest.Selected))
		for _, entry := range result.Manifest.Selected {
			ids = append(ids, entry.ID)
		}
		if len(ids) > 12 {
			ids = append(ids[:12], "…")
		}
		fmt.Fprintf(&details, "selected: %s", strings.Join(ids, ", "))
	}

	transparency.EmitJIT(summary, details.String(), source, stats.Duration)
}

// logCompilationStats logs comprehensive stats at the end of compilation.
func (c *JITPromptCompiler) logCompilationStats(stats *CompilationStats, result *CompilationResult) {
	logger := logging.Get(logging.CategoryJIT)

	emitJITGlassBox(stats, result)

	// Budget breach detection: budgetMgr.Fit accounts only per-atom render-mode
	// tokens. The final assembled prompt picks up extra tokens from headers,
	// category markers, and separators that Fit doesn't see. When that drift
	// pushes TokensUsed above TokenBudget, the prior code shipped the
	// over-budget prompt silently (logged at INFO) and the JIT cache would
	// re-serve it for every cache HIT on the same context, contaminating the
	// LLM with truncation-prone payloads. Surface it loudly so callers (and
	// log audits) can see when the budget contract is violated.
	if stats.TokenBudget > 0 && stats.TokensUsed > stats.TokenBudget {
		overshoot := stats.TokensUsed - stats.TokenBudget
		logger.Warn(
			"JIT[%s] budget breach: assembled %d tokens vs budget %d (+%d, %.1f%%) — assembler boilerplate exceeded budgetMgr.Fit accounting; consider reserving headroom in fit step",
			stats.ShardID, stats.TokensUsed, stats.TokenBudget, overshoot, stats.BudgetUtilization*100,
		)
	}

	// Summary line (INFO level)
	logger.Info("%s", stats.String())

	// Detailed breakdown (DEBUG level)
	logger.Debug(
		"Phase timing: collect=%dms select=%dms resolve=%dms fit=%dms assemble=%dms",
		stats.CollectAtomsMs, stats.SelectAtomsMs, stats.ResolveDepsMs,
		stats.FitBudgetMs, stats.AssembleMs,
	)

	logger.Debug(
		"Source breakdown: embedded=%d project=%d shard=%d evolved=%d",
		stats.EmbeddedAtoms, stats.ProjectAtoms, stats.ShardAtoms, stats.EvolvedAtoms,
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

// logCompilationManifest logs the prompt manifest (selected/dropped atom IDs) for debugging.
func (c *JITPromptCompiler) logCompilationManifest(stats *CompilationStats, result *CompilationResult) {
	if result == nil || result.Manifest == nil {
		return
	}

	manifest := result.Manifest
	selectedIDs := make([]string, 0, len(manifest.Selected))
	for _, entry := range manifest.Selected {
		selectedIDs = append(selectedIDs, entry.ID)
	}

	droppedIDs := make([]string, 0, len(manifest.Dropped))
	for _, entry := range manifest.Dropped {
		droppedIDs = append(droppedIDs, entry.ID)
	}

	fields := map[string]any{
		"context_hash":   manifest.ContextHash,
		"shard_id":       stats.ShardID,
		"intent_verb":    stats.IntentVerb,
		"selected_ids":   selectedIDs,
		"selected":       manifest.Selected,
		"selected_count": len(manifest.Selected),
		"dropped_ids":    droppedIDs,
		"dropped":        manifest.Dropped,
		"dropped_count":  len(manifest.Dropped),
	}

	// Use info level so manifests persist even when log level is set to info.
	logging.Get(logging.CategoryJIT).StructuredLog("info", "prompt_manifest", fields)
}

// GetLastResult returns the most recent compilation result.
// This is used by the TUI Prompt Inspector for observability.
func (c *JITPromptCompiler) GetLastResult() *CompilationResult {
	return c.lastResult.Load()
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
	c.shardMu.RLock()
	shardCount := len(c.shardDBs)
	c.shardMu.RUnlock()

	stats := CompilerStats{
		ShardDBCount: shardCount,
	}

	c.dbMu.RLock()
	if c.embeddedCorpus != nil {
		stats.EmbeddedAtomCount = c.embeddedCorpus.Count()
	}
	c.dbMu.RUnlock()

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
	c.wg.Wait()

	func() {
		c.shardMu.Lock()
		defer c.shardMu.Unlock()
		// Close shard databases
		for id, db := range c.shardDBs {
			if err := db.Close(); err != nil {
				logging.Get(logging.CategoryContext).Warn("Failed to close shard DB %s: %v", id, err)
			}
		}
		c.shardDBs = make(map[string]*sql.DB)
	}()

	var finalErr error
	func() {
		c.dbMu.Lock()
		defer c.dbMu.Unlock()
		// Close project database
		if c.projectDB != nil {
			if err := c.projectDB.Close(); err != nil {
				finalErr = fmt.Errorf("failed to close project DB: %w", err)
				return
			}
			c.projectDB = nil
		}
	}()

	return finalErr
}
