package prompt

import (
	"context"
	"fmt"
	"strings"
	"time"

	"golang.org/x/sync/errgroup"
	"golang.org/x/sync/singleflight"

	"codenerd/internal/logging"
	"codenerd/internal/usage"
)

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

// compile is the cache-aware entry. Its phases:
//
//  1. Preflight: name the kernel state the prompt depends on. A reusable
//     kernel identity seen before with this exact context skips the clone,
//     the context assertion and the full-snapshot walk (compiler_preflight.go).
//  2. Source capture: read the generation of every non-kernel source
//     (compiler_generation.go). The cache key is context + kernel fingerprint
//     + configuration revision + source vector.
//  3. Lookup. A hit returns here, having cloned nothing when the preflight
//     was reused.
//  4. Miss: acquire a private selection scope (if the preflight was reused)
//     and run the compile once per key under singleflight; publish it only if
//     the source vector is unchanged afterwards and nothing degraded.
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
	cc.SessionContext = nil
	cc.UserIntent = nil

	preflightStart := time.Now()
	pf, err := c.runPreflight(ctx, cc, config, configRevision)
	if err != nil {
		return nil, err
	}
	releaseAtReturn := true
	defer func() {
		if releaseAtReturn {
			pf.release()
		}
	}()
	cc.kernelSnapshotFingerprint = pf.fingerprint
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	if strings.TrimSpace(cc.AvailableSpecialists) == "" {
		if err := InjectAvailableSpecialists(cc, ""); err != nil {
			logging.Get(logging.CategoryJIT).Debug("Failed to resolve specialists before cache lookup: %v", err)
		}
	}

	src := c.snapshotSources(cc)
	vector := src.captureSourceVector(ctx)
	cacheKey := promptCacheKey(cc.Hash(), configRevision, vector.token())
	preflightDuration := time.Since(preflightStart)
	c.counters.recordPreflight(preflightDuration)

	if vector.tracked() {
		if cached, ok := c.cache.Get(cacheKey); ok {
			if err := ctx.Err(); err != nil {
				return nil, err
			}
			hits := c.counters.cacheHits.Add(1)
			logging.Get(logging.CategoryJIT).Info("Prompt cache HIT for %s (hash=%s, hits=%d, preflight_reused=%v)",
				cc.String(), cacheKey[:8], hits, pf.reused)
			return privateResult(cached, true), nil
		}
	}

	// Miss: selection needs a private scope. A reused preflight has none yet;
	// acquiring it may land on a newer kernel generation, in which case the
	// snapshot is recomputed there and the key follows it.
	refreshed, err := c.ensureSelectionScope(ctx, cc, config, configRevision, pf)
	if err != nil {
		return nil, err
	}
	if refreshed {
		cc.kernelSnapshotFingerprint = pf.fingerprint
		cacheKey = promptCacheKey(cc.Hash(), configRevision, vector.token())
	}
	cc.Kernel = pf.kernel
	if err := ctx.Err(); err != nil {
		return nil, err
	}

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
		if vector.tracked() {
			if cached, ok := c.cache.Get(cacheKey); ok {
				return promptCompilationOutcome{result: cached, cacheHit: true}, nil
			}
		}
		return c.compileMiss(ctx, missInput{
			cc:             cc,
			config:         config,
			cacheKey:       cacheKey,
			kernel:         pf.kernel,
			src:            src,
			vector:         vector,
			dynamicAtoms:   pf.dynamic,
			preflight:      preflightDuration,
			preflightReuse: pf.reused,
		})
	})

	completion := make(chan singleflight.Result, 1)
	go func() {
		defer c.wg.Done()
		outcome := <-resultChannel
		pf.release()
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
		c.counters.cacheHits.Add(1)
	}
	if outcome.Shared {
		c.counters.joiners.Add(1)
		logging.Get(logging.CategoryJIT).Info("Prompt compilation joined via singleflight for hash=%s", cacheKey[:8])
	}
	return privateResult(compiled.result, compiled.cacheHit), nil
}

// missInput is everything one miss compile reads, fixed before it starts.
type missInput struct {
	cc             *CompilationContext
	config         CompilerConfig
	cacheKey       string
	kernel         KernelQuerier
	src            compileSources
	vector         sourceVector
	dynamicAtoms   []*PromptAtom
	preflight      time.Duration
	preflightReuse bool
}

// compileMiss runs one real compile: collect, select, resolve, fit, assemble,
// then publish under the generation-bound rules.
func (c *JITPromptCompiler) compileMiss(ctx context.Context, in missInput) (any, error) {
	cc, config := in.cc, in.config
	misses := c.counters.cacheMisses.Add(1)
	c.counters.compiles.Add(1)

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
		c.counters.compileNanos.Add(int64(time.Since(compileStart) + in.preflight))
	}()
	stats := &CompilationStats{
		ShardID:         cc.ShardID,
		OperationalMode: cc.OperationalMode,
		IntentVerb:      cc.IntentVerb,
		PreflightMs:     in.preflight.Milliseconds(),
		PreflightReused: in.preflightReuse,
	}

	logging.Get(logging.CategoryJIT).Info("Compiling prompt (cache MISS): %s (hash=%s, misses=%d)",
		cc.String(), in.cacheKey[:8], misses)

	// Step 1.5: Collect all candidate atoms from all sources concurrently
	collectStart := time.Now()
	g, gCtx := errgroup.WithContext(ctx)

	var candidates []*PromptAtom
	var breakdown sourceBreakdown
	var knowledgeAtoms, learningAtoms []*PromptAtom
	degraded := &degradation{}
	pending := &pendingCatalogs{}

	g.Go(func() error {
		var err error
		candidates, breakdown, err = c.collectAtomsWithStats(gCtx, cc, in.src, in.vector, pending, degraded)
		if err != nil {
			return fmt.Errorf("failed to collect atoms: %w", err)
		}
		return nil
	})
	g.Go(func() error {
		knowledgeAtoms = c.collectKnowledgeAtomsFrom(gCtx, cc, config, in.src, degraded)
		return nil // Non-fatal
	})
	g.Go(func() error {
		learningAtoms = c.collectLearningAtomsFrom(gCtx, cc, config, in.src, degraded)
		return nil // Non-fatal
	})
	if err := g.Wait(); err != nil {
		return nil, err
	}
	if err := ctx.Err(); err != nil {
		return nil, fmt.Errorf("prompt compilation canceled while collecting atoms: %w", err)
	}

	candidates = append(candidates, in.dynamicAtoms...)
	candidates = append(candidates, knowledgeAtoms...)
	candidates = append(candidates, learningAtoms...)

	stats.CollectAtomsMs = time.Since(collectStart).Milliseconds()
	stats.AtomsCandidates = len(candidates)
	stats.EmbeddedAtoms = breakdown.embedded
	stats.ProjectAtoms = breakdown.project
	stats.ShardAtoms = breakdown.shard
	stats.EvolvedAtoms = breakdown.evolved
	stats.StrategyAtoms = breakdown.strategy

	logging.Get(logging.CategoryJIT).Debug(
		"Collected %d candidate atoms (embedded=%d, project=%d, shard=%d, evolved=%d, strategy=%d, kernel=%d, knowledge=%d, learning=%d) in %dms",
		len(candidates), breakdown.embedded, breakdown.project, breakdown.shard,
		breakdown.evolved, breakdown.strategy, len(in.dynamicAtoms), len(knowledgeAtoms), len(learningAtoms),
		stats.CollectAtomsMs,
	)

	// Step 2: Select atoms based on context (Mangle rules + vector search)
	selectStart := time.Now()
	scored, vectorMs, err := c.selector.selectAtomsWithTimingKernel(ctx, candidates, cc, in.kernel)
	if err != nil {
		return nil, fmt.Errorf("failed to select atoms: %w", err)
	}
	if err := ctx.Err(); err != nil {
		return nil, fmt.Errorf("prompt compilation canceled during selection: %w", err)
	}
	// A selected head brings its sections; a lone section brings its head --
	// each only when the kernel admitted it in this compile.
	if len(scored) > 0 {
		admitted, err := expansionAdmission(in.kernel)
		if err != nil {
			return nil, fmt.Errorf("read section admission: %w", err)
		}
		scored = expandSections(scored, candidates, admitted)
	}
	stats.SelectAtomsMs = time.Since(selectStart).Milliseconds()
	stats.VectorQueryMs = vectorMs

	// Step 3: Resolve dependencies and handle conflicts
	resolveStart := time.Now()
	ordered, err := c.resolver.Resolve(scored)
	if err != nil {
		return nil, fmt.Errorf("failed to resolve dependencies: %w", err)
	}
	stats.ResolveDepsMs = time.Since(resolveStart).Milliseconds()

	// Steps 4-5: fit, render, assemble, enforce (compiler_render.go).
	budget := cc.AvailableTokens()
	if budget <= 0 {
		budget = config.DefaultTokenBudget
	}
	stats.TokenBudget = budget
	fitted, prompt, err := c.fitAndAssemble(ordered, cc, budget, stats)
	if err != nil {
		return nil, err
	}

	stats.Duration = time.Since(compileStart) + in.preflight

	result := c.buildResultWithStats(candidates, scored, fitted, prompt, budget, stats)
	if result.Manifest != nil {
		result.Manifest.ContextHash = in.cacheKey
	}

	// Publication. The key was minted from the source vector captured before
	// any source was read; re-capture now. Only an unchanged, fully tracked
	// vector and an undegraded compile may enter the cache -- otherwise this
	// result describes a generation the key does not name.
	after := in.src.captureSourceVector(ctx)
	stats.DegradedSources = degraded.list()
	switch {
	case !in.vector.tracked():
		stats.Uncacheable = "untracked source: " + strings.Join(in.vector.untracked, ",")
	case len(stats.DegradedSources) > 0:
		stats.Uncacheable = "degraded source: " + strings.Join(stats.DegradedSources, ",")
		c.counters.degraded.Add(1)
	case !after.equal(in.vector):
		stats.Uncacheable = "source generation changed during compile"
		c.counters.sourceChurn.Add(1)
	}
	c.lastResult.Store(result)
	if stats.Uncacheable == "" {
		c.cache.Put(in.cacheKey, result)
		for source, entry := range pending.entries {
			c.catalog.install(source, entry)
		}
		logging.Get(logging.CategoryJIT).Debug("Stored compiled prompt in cache (hash=%s)", in.cacheKey[:8])
	} else {
		c.counters.uncacheable.Add(1)
		logging.Get(logging.CategoryJIT).Info("Compiled prompt not cached (hash=%s): %s", in.cacheKey[:8], stats.Uncacheable)
	}

	c.logCompilationStats(stats, result)
	if config.DebugMode {
		c.logCompilationManifest(stats, result)
	}
	return promptCompilationOutcome{result: result}, nil
}
