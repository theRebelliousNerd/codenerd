package prompt

import (
	"container/list"
	"context"
	"database/sql"
	"fmt"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"golang.org/x/sync/singleflight"

	userconfig "codenerd/internal/config"
	"codenerd/internal/logging"
	"codenerd/internal/store"
	"codenerd/internal/usage"

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

// KernelFactSnapshotQuerier exposes the complete evaluated facts of an owned
// compilation scope, including inputs to selection outside the injection rows.
type KernelFactSnapshotQuerier interface {
	QueryAll() (map[string][]Fact, error)
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
	"atom_requires_tool",
	"available_tool",
	"compile_shard",
	"retrieved_context",
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

	// Bug #5 fix: Prompt cache to prevent recompilation spam
	cache      map[string]*list.Element
	cacheList  *list.List
	cacheLimit int
	cacheMu    sync.Mutex
	cacheHits  int64
	cacheMiss  int64
	// compileNanos accumulates the wall time of compiles that actually ran.
	// Its denominator is totalCompilations, deliberately: averaging in cache
	// hits, which take microseconds, produces a number that describes the
	// cache rather than the compiler and gets smaller the better the cache
	// works.
	compileNanos int64

	// Observability
	lastResult atomic.Pointer[CompilationResult]

	// totalCompilations is the lifetime compile count backing GetStats. Only
	// real compiles (singleflight winners) count; cache hits and shared
	// results are served work, not compiled work. Unlike cacheMiss it is not
	// reset when the prompt cache is cleared, so it stays the denominator of
	// compileNanos, which is not reset either.
	totalCompilations atomic.Int64

	// Concurrency control
	configMu       sync.RWMutex
	dbMu           sync.RWMutex
	shardMu        sync.RWMutex
	compileGroup   singleflight.Group
	wg             sync.WaitGroup
	lifecycleMu    sync.Mutex
	closed         bool
	stopContext    context.Context
	stop           context.CancelFunc
	closeOnce      sync.Once
	closeErr       error
	configRevision uint64 // guarded by configMu; changes whenever SetConfig replaces compile behavior

	// Configuration
	config CompilerConfig

	// LocalDB for semantic knowledge atom queries (Semantic Knowledge Bridge)
	localDB *store.LocalStore

	// LearningStore for recalling learned intents and feedback (Semantic Knowledge Bridge)
	learningStore *store.LearningStore

	// Evolved atoms manager for System Prompt Learning (SPL)
	evolvedAtomMgr *EvolvedAtomManager

	// strategyProvider is the learned-strategy source (see strategy_atoms.go).
	// Nil until registered, which is the state for every compiler built
	// without a prompt evolver behind it.
	strategyProvider StrategyProvider
}

// CompilerConfig holds configuration for the JIT compiler.
type CompilerConfig struct {
	// DefaultTokenBudget is the default token budget if not specified in context
	DefaultTokenBudget int

	// VectorSearchWeight is the weight of vector scores vs logic scores (0.0-1.0)
	VectorSearchWeight float64

	// VectorSearchTimeout is the timeout duration for vector searches.
	//
	// Read in four places and set by no production caller, which is what the
	// dark-field gate reports -- but it is a knob with no SOURCE, not a broken
	// wire, and the difference is the whole triage. Both consumers coerce a
	// zero (`if timeout <= 0 { timeout = 10 * time.Second }`, here in
	// collectLearningAtoms and again in AtomSelector.SetVectorSearchTimeout)
	// and the selector's constructor defaults to the same ten seconds, so the
	// inert field costs nothing and semantic selection runs on the default.
	//
	// Checked because the opposite would have been bad: a timeout field read
	// and never written, with no guard, gives context.WithTimeout(ctx, 0) --
	// a deadline already past, which would disable vector search on every
	// compile while reporting a timeout nobody configured. That is the shape
	// worth ruling out, and it is ruled out.
	//
	// Making it live means adding a user-facing config key and plumbing it,
	// which is main's 938cd87 decision in reverse and belongs with whoever
	// wants the knob.
	VectorSearchTimeout time.Duration

	// DebugMode enables verbose JIT manifest logging
	DebugMode bool

	// KnowledgeSearchTimeout is the max time to wait for knowledge atom embedding and search
	KnowledgeSearchTimeout time.Duration

	// KernelContextRows caps injectable_context rows merged into the single
	// kernel-context atom. A non-positive value means unset and resolves to
	// the configured jit.kernel_context_rows at use.
	KernelContextRows int

	// KernelContextRowChars caps one injectable_context row. A non-positive
	// value resolves to jit.kernel_context_row_chars.
	KernelContextRowChars int

	// KernelInjectedAtomChars is the ceiling on either merged kernel-injected
	// atom. A non-positive value resolves to jit.kernel_injected_atom_chars.
	KernelInjectedAtomChars int

	// SpecialistKnowledgeBlocks caps specialist_knowledge topics merged into
	// one atom. A non-positive value resolves to jit.specialist_knowledge_blocks.
	SpecialistKnowledgeBlocks int

	// SpecialistTopicChars caps a specialist_knowledge topic heading. A
	// non-positive value resolves to jit.specialist_topic_chars.
	SpecialistTopicChars int

	// SpecialistBlockChars caps one specialist_knowledge body. A non-positive
	// value resolves to jit.specialist_block_chars.
	SpecialistBlockChars int

	// FOUR FIELDS WERE REMOVED HERE on 2026-09-11, all set by
	// DefaultCompilerConfig and read by nothing in the repository -- not
	// production, not tests, except tests asserting the literal they had just
	// been handed. Three of them described behaviour that does not exist,
	// which is worse than an absent knob: a reader reasoning about this
	// compiler believed them.
	//
	//   EnableVectorSearch   the selector has no such switch. It runs the
	//                        search when a vectorSearcher is installed and
	//                        skips it when one is not, and the factory decides
	//                        that by whether an embedding engine exists.
	//   MaxAtomsPerCategory  no per-category cap exists anywhere. Selection is
	//                        bounded by the token budget.
	//   CacheTTLSeconds      the prompt LRU has NO TTL. Entries live until size
	//                        eviction. The "// 5 minutes" beside it was
	//                        especially expensive, because this branch spent
	//                        real effort on Anthropic's five-minute prompt-cache
	//                        TTL and a second inert five-minute TTL in the same
	//                        subsystem is a trap laid for exactly that reader.
	//   EnableCaching        the LRU is unconditional; nothing checks a flag.
	//
	// Deleted rather than wired, which is main's 938cd87 judgement on
	// shard_profiles.max_output_tokens applied again: each of the three would
	// have meant BUILDING the behaviour it claims, and that is a feature
	// decision. VectorSearchWeight is the one of the five that was a genuine
	// missing wire and it is wired above.
}

// DefaultCompilerConfig returns a sensible default configuration.
// DefaultTokenBudget is a fallback: production boot (internal/system) overrides
// it from the configured JIT token budget through WithConfig. The injection
// bounds below are sourced from DefaultJITConfig, their single definition;
// a user-set jit.* value takes effect once boot maps it onto this struct
// (internal/system maps DefaultTokenBudget and DebugMode today).
func DefaultCompilerConfig() CompilerConfig {
	jitDefaults := userconfig.DefaultJITConfig()
	return CompilerConfig{
		DefaultTokenBudget: 200000, // 200k tokens default - callers should override from config
		// Mirrors AtomSelector's own default. The selector keeps one too, for
		// direct users that never see a CompilerConfig; this is the value the
		// compiler pushes into it.
		VectorSearchWeight:        0.3, // 70% logic, 30% vector
		DebugMode:                 false,
		KnowledgeSearchTimeout:    10 * time.Second,
		KernelContextRows:         jitDefaults.KernelContextRows,
		KernelContextRowChars:     jitDefaults.KernelContextRowChars,
		KernelInjectedAtomChars:   jitDefaults.KernelInjectedAtomChars,
		SpecialistKnowledgeBlocks: jitDefaults.SpecialistKnowledgeBlocks,
		SpecialistTopicChars:      jitDefaults.SpecialistTopicChars,
		SpecialistBlockChars:      jitDefaults.SpecialistBlockChars,
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
	compiler.stopContext, compiler.stop = context.WithCancel(context.Background())

	// Default the canonical built-in atoms. The embedded corpus is a
	// compile-time constant and production treats it as mandatory (the system
	// factory fails startup without it), so a bare compiler must not silently
	// compile empty prompts. An explicit WithEmbeddedCorpus option — including
	// nil for isolation — still overrides this default.
	if corpus, err := getEmbeddedCorpusCached(); err != nil {
		compiler.stop()
		return nil, fmt.Errorf("failed to load embedded prompt corpus: %w", err)
	} else {
		compiler.embeddedCorpus = corpus
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
	// The sibling knob, and it was wired nowhere. SetVectorWeight had no
	// production caller and CompilerConfig.VectorSearchWeight had no
	// reader: two halves of one missing wire, which is why the selector
	// and the config each carried their own 0.3 with the same
	// "70% logic, 30% vector" comment attached.
	//
	// The zero is guarded because the two setters do NOT agree about what
	// one means. SetVectorSearchTimeout reads zero as "unset" and
	// substitutes ten seconds; SetVectorWeight clamps to [0,1] and takes a
	// zero literally, as pure logic. So wiring this unguarded would make a
	// partially-filled CompilerConfig silently turn vector scoring off --
	// the exact silent-failure shape this is being fixed to remove. Pure
	// logic is expressed by installing no vector searcher at all
	// (selector.go skips the search when vectorSearcher is nil).
	if compiler.config.VectorSearchWeight > 0 {
		compiler.selector.SetVectorWeight(compiler.config.VectorSearchWeight)
	}

	logging.Get(logging.CategoryContext).Info("JITPromptCompiler initialized")
	return compiler, nil
}

// This is the main entry point for prompt compilation.
// Compile compiles a system prompt for cc and records the atom selection for
// co-use analysis.
//
// The recording wraps the whole compilation rather than sitting at each return
// site because two of the three success paths -- a prompt cache hit and a
// singleflight share -- return a result that was selected earlier. Those turns
// used those atoms just as much as the turn that compiled them, and a recorder
// that only saw cache misses would report a sample biased toward whichever
// contexts happen to be novel.
func (c *JITPromptCompiler) Compile(ctx context.Context, cc *CompilationContext) (*CompilationResult, error) {
	result, err := c.compile(ctx, cc)
	if err == nil && result != nil {
		CoUse().ObserveAtoms(usage.TurnIDFromContext(ctx), result.IncludedAtoms)
	}
	return result, err
}

func (c *JITPromptCompiler) compile(ctx context.Context, cc *CompilationContext) (*CompilationResult, error) {
	c.lifecycleMu.Lock()
	if c.closed {
		c.lifecycleMu.Unlock()
		return nil, fmt.Errorf("prompt compiler is closed")
	}
	c.wg.Add(1)
	c.lifecycleMu.Unlock()
	defer c.wg.Done()
	ctx, cancel := context.WithCancel(ctx)
	stopCancellation := context.AfterFunc(c.stopContext, cancel)
	defer stopCancellation()
	defer cancel()
	if err := ctx.Err(); err != nil {
		return nil, err
	}

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
	config, configRevision, selectorSettings := c.configSnapshot()
	cc.selectorVectorConfig = &selectorSettings
	if !cc.HasVectorWeight && config.VectorSearchWeight > 0 {
		cc.VectorWeight = config.VectorSearchWeight
		cc.HasVectorWeight = true
	}
	selectionKernel, releaseKernel, err := acquireCompilationKernel(c.kernel)
	if err != nil {
		return nil, fmt.Errorf("create isolated prompt compilation scope: %w", err)
	}
	releaseAtReturn := true
	defer func() {
		if releaseAtReturn {
			releaseKernel()
		}
	}()
	cc.Kernel = selectionKernel
	cc.SessionContext = nil
	cc.UserIntent = nil
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	snapshotStart := time.Now()
	if selectionKernel != nil {
		if err := selectionKernel.AssertBatch(cc.ToContextFacts()); err != nil {
			return nil, fmt.Errorf("assert private prompt context: %w", err)
		}
	}
	snapshotCollectStart := time.Now()
	dynamicAtoms, fingerprint, err := c.collectDynamicKernelSnapshotWithConfig(selectionKernel, cc, config)
	snapshotCollectDuration := time.Since(snapshotCollectStart)
	snapshotDuration := time.Since(snapshotStart)
	if err != nil {
		return nil, fmt.Errorf("capture dynamic prompt snapshot: %w", err)
	}
	cc.kernelSnapshotFingerprint = fingerprint
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	if strings.TrimSpace(cc.AvailableSpecialists) == "" {
		if err := InjectAvailableSpecialists(cc, ""); err != nil {
			logging.Get(logging.CategoryJIT).Debug("Failed to resolve specialists before cache lookup: %v", err)
		}
	}

	// Bug #5 fix: Check cache before compilation
	cacheKey := promptCacheKey(cc.Hash(), configRevision)
	c.cacheMu.Lock()
	if elem, ok := c.cache[cacheKey]; ok {
		c.cacheList.MoveToFront(elem)
		cached := elem.Value.(*promptCacheEntry).result
		c.cacheMu.Unlock()
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		atomic.AddInt64(&c.cacheHits, 1)
		logging.Get(logging.CategoryJIT).Info("Prompt cache HIT for %s (hash=%s, hits=%d)",
			cc.String(), cacheKey[:8], atomic.LoadInt64(&c.cacheHits))
		return privateResult(cached, true), nil
	}
	c.cacheMu.Unlock()
	// Singleflight to prevent Thundering Herd
	c.wg.Add(1)
	releaseAtReturn = false
	resultChannel := c.compileGroup.DoChan(cacheKey, func() (any, error) {
		ctx, cancelWork := context.WithCancel(context.WithoutCancel(ctx))
		stopWorkCancellation := context.AfterFunc(c.stopContext, cancelWork)
		defer stopWorkCancellation()
		defer cancelWork()
		if err := ctx.Err(); err != nil {
			return nil, fmt.Errorf("prompt compilation canceled before cache recheck: %w", err)
		}
		c.cacheMu.Lock()
		if elem, ok := c.cache[cacheKey]; ok {
			c.cacheList.MoveToFront(elem)
			cached := elem.Value.(*promptCacheEntry).result
			c.cacheMu.Unlock()
			return promptCompilationOutcome{result: cached, cacheHit: true}, nil
		}
		c.cacheMu.Unlock()
		atomic.AddInt64(&c.cacheMiss, 1)
		c.totalCompilations.Add(1)

		if err := ctx.Err(); err != nil {
			return nil, fmt.Errorf("prompt compilation canceled before selection: %w", err)
		}

		// Start comprehensive timing after validation
		compileStart := time.Now()
		// The same clock feeds the per-compilation Duration below and the
		// process-wide average `nerd jit` reports. Accumulating on EVERY exit
		// path, including the error ones, is deliberate: a compile that failed
		// after eight seconds still spent eight seconds, and an average that
		// counted only successes would get better the more often compilation
		// broke.
		defer func() {
			atomic.AddInt64(&c.compileNanos, int64(time.Since(compileStart)+snapshotDuration))
		}()
		stats := &CompilationStats{
			ShardID:         cc.ShardID,
			OperationalMode: cc.OperationalMode,
			IntentVerb:      cc.IntentVerb,
		}

		logging.Get(logging.CategoryJIT).Info("Compiling prompt (cache MISS): %s (hash=%s, misses=%d)",
			cc.String(), cacheKey[:8], atomic.LoadInt64(&c.cacheMiss))

		// Step 1.5: Collect all candidate atoms from all sources concurrently
		collectStart := time.Now()

		g, gCtx := errgroup.WithContext(ctx)

		var candidates []*PromptAtom
		var sourceBreakdown sourceBreakdown

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

		// 1.5c: Collect semantic knowledge atoms (Semantic Knowledge Bridge)
		g.Go(func() error {
			knowledgeAtoms = c.collectKnowledgeAtomsWithConfig(gCtx, cc, config)
			return nil // Non-fatal
		})

		// 1.5d: Collect learning atoms from autopoiesis memory (Semantic Knowledge Bridge)
		g.Go(func() error {
			learningAtoms = c.collectLearningAtomsWithConfig(gCtx, cc, config)
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

		stats.CollectAtomsMs = (time.Since(collectStart) + snapshotCollectDuration).Milliseconds()
		stats.AtomsCandidates = len(candidates)
		stats.EmbeddedAtoms = sourceBreakdown.embedded
		stats.ProjectAtoms = sourceBreakdown.project
		stats.ShardAtoms = sourceBreakdown.shard
		stats.EvolvedAtoms = sourceBreakdown.evolved
		stats.StrategyAtoms = sourceBreakdown.strategy

		logging.Get(logging.CategoryJIT).Debug(
			"Collected %d candidate atoms (embedded=%d, project=%d, shard=%d, evolved=%d, strategy=%d) in %dms",
			len(candidates), sourceBreakdown.embedded, sourceBreakdown.project, sourceBreakdown.shard,
			sourceBreakdown.evolved, sourceBreakdown.strategy, stats.CollectAtomsMs,
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
			budget = config.DefaultTokenBudget
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

		// Step 5b: Enforce the budget on what was actually assembled.
		//
		// Fit charges per-atom render-mode token counts. The assembler then
		// expands {{...}} placeholders and joins with separators, so the
		// emitted prompt can exceed the budget Fit reported as satisfied —
		// {{available_specialists}} alone interpolates the whole agent
		// registry over a 25-character placeholder. Until this step existed
		// the overshoot was detected in logCompilationStats and then shipped
		// anyway: the contract said "bounded", the wire said otherwise.
		fitted, prompt, err = c.enforceAssembledBudget(fitted, cc, prompt, budget)
		if err != nil {
			return nil, err
		}
		stats.AssembleMs = time.Since(assembleStart).Milliseconds()

		// Finalize timing
		stats.Duration = time.Since(compileStart) + snapshotDuration

		// Build result with comprehensive stats
		result := c.buildResultWithStats(candidates, scored, fitted, prompt, budget, stats)
		if result.Manifest != nil {
			result.Manifest.ContextHash = cacheKey
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
		if config.DebugMode {
			c.logCompilationManifest(stats, result)
		}

		return promptCompilationOutcome{result: result}, nil
	})

	completion := make(chan singleflight.Result, 1)
	go func() {
		defer c.wg.Done()
		outcome := <-resultChannel
		releaseKernel()
		completion <- outcome
	}()
	var outcome singleflight.Result
	select {
	case <-ctx.Done():
		return nil, ctx.Err()
	case outcome = <-completion:
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	if outcome.Err != nil {
		return nil, outcome.Err
	}
	compiled := outcome.Val.(promptCompilationOutcome)
	if compiled.cacheHit {
		atomic.AddInt64(&c.cacheHits, 1)
	}
	if outcome.Shared {
		logging.Get(logging.CategoryJIT).Info("Prompt compilation joined via singleflight for hash=%s", cacheKey[:8])
	}
	return privateResult(compiled.result, compiled.cacheHit), nil
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
	c.closeOnce.Do(func() {
		c.lifecycleMu.Lock()
		c.closed = true
		c.stop()
		c.lifecycleMu.Unlock()
		c.closeErr = c.closeResources()
	})
	return c.closeErr
}

func (c *JITPromptCompiler) closeResources() error {
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

// enforceAssembledBudget makes the assembled prompt honour the token budget.
//
// One mechanism: shed whole optional atoms, lowest priority first (a missing
// exemplar is legible; half an exemplar is a lie). If the mandatory skeleton
// alone does not fit, the compile fails with an error naming the budget and
// the skeleton. It used to cut the prompt head+tail with a marker instead;
// that cut the identity and safety atoms — the ones that constrain the model
// — and shipped the result as a prompt. A turn that does not run is a
// configuration error the operator can see and fix; a turn that runs on a
// constitution with its ending missing is not.
func (c *JITPromptCompiler) enforceAssembledBudget(
	fitted []*OrderedAtom,
	cc *CompilationContext,
	prompt string,
	budget int,
) ([]*OrderedAtom, string, error) {
	if budget <= 0 || prompt == "" || cc == nil {
		return fitted, prompt, nil
	}
	if EstimateTokens(prompt) <= budget {
		return fitted, prompt, nil
	}

	budgetMgr := c.budgetMgr
	if budgetMgr == nil {
		budgetMgr = NewTokenBudgetManager()
	}

	kept, shedPrompt, used := budgetMgr.ShedToFit(fitted, prompt, budget, func(atoms []*OrderedAtom) (string, error) {
		return c.assembler.Assemble(atoms, cc)
	})
	if used <= budget {
		return kept, shedPrompt, nil
	}

	logging.Get(logging.CategoryJIT).Error(
		"JIT[%s] mandatory skeleton exceeds budget after shedding every optional atom: %d tokens vs budget %d — refusing to compile a cut prompt; raise context_window.max_tokens or jit.token_budget, or split the mandatory atoms",
		cc.ShardID, used, budget,
	)
	return kept, shedPrompt, fmt.Errorf(
		"prompt budget of %d tokens cannot hold the mandatory skeleton (%d tokens after shedding every optional atom) for shard %q: raise context_window.max_tokens or jit.token_budget, or split the mandatory atoms",
		budget, used, cc.ShardID)
}
