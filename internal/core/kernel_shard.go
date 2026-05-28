package core

import (
	"fmt"
	"sync"
	"time"

	"codenerd/internal/logging"
	"codenerd/internal/types"

	"codeberg.org/TauCeti/mangle-go/ast"
	"codeberg.org/TauCeti/mangle-go/engine"
)

// =============================================================================
// KERNEL SHARD — Domain-Specific Sub-Kernel
// =============================================================================

// KernelShard wraps a RealKernel with domain metadata for the hierarchical
// kernel architecture. Each shard owns a specific domain of predicates
// (e.g., ROUTING, WORLD, TOOLS, POLICY) and evaluates independently.
type KernelShard struct {
	mu     sync.RWMutex
	domain string       // Domain name (e.g., "routing", "world", "tools", "policy")
	kernel *RealKernel  // The underlying kernel instance

	// Domain ownership
	ownedPredicates map[string]bool      // Predicates this shard is authoritative for
	exportedPreds   []string             // Predicate names exported to the cortex

	// Schema/policy files loaded by this shard
	schemaFiles []string
	policyFiles []string

	// router is the Track-D per-shard fact coordinator. When non-nil, the
	// shard's Assert/Query/Retract paths consult it before touching the
	// inner kernel and dispatch to the predicate's owning shard if it
	// isn't this one. nil means "single-store mode" — every operation
	// hits the local kernel exactly as it did before Track D. The router
	// is installed by CortexKernel.RegisterShard only when the
	// per-shard-facts feature flag is enabled.
	router *ShardFactRouter

	// Observability metrics
	evalCount        int64         // Total evaluations performed
	queryCount       int64         // Total queries served
	lastEvalDuration time.Duration // Duration of last evaluation
	dirtyCount       int64         // Times factsDirty was set
	exportHitCount   int64         // Times an external predicate callback was called
}

// KernelShardConfig contains configuration for creating a KernelShard.
type KernelShardConfig struct {
	Domain          string   // Domain name
	SchemaFiles     []string // Paths to schema .mg files to load
	PolicyFiles     []string // Paths to policy .mg files to load
	OwnedPredicates []string // Predicate names this shard owns
	ExportedPreds   []string // Predicate names to export to cortex
	ManglePath      string   // Path to mangle files directory
	WorkspaceRoot   string   // Workspace root for .nerd paths
}

// NewKernelShard creates a new domain-specific kernel shard.
func NewKernelShard(config KernelShardConfig) (*KernelShard, error) {
	if config.Domain == "" {
		return nil, fmt.Errorf("KernelShard domain name is required")
	}

	// Create the underlying kernel. We use NewRealKernelWithWorkspace if available,
	// otherwise the default constructor which loads all embedded schemas/policy.
	// For domain shards, the caller will replace schemas/policy with domain-specific content.
	var kernel *RealKernel
	var err error
	if config.WorkspaceRoot != "" {
		kernel, err = NewRealKernelWithWorkspace(config.WorkspaceRoot)
	} else if config.ManglePath != "" {
		kernel, err = NewRealKernelWithPath(config.ManglePath)
	} else {
		kernel, err = NewRealKernel()
	}
	if err != nil {
		return nil, fmt.Errorf("failed to create kernel for shard %s: %w", config.Domain, err)
	}

	// Build owned predicates map
	ownedPreds := make(map[string]bool, len(config.OwnedPredicates))
	for _, pred := range config.OwnedPredicates {
		ownedPreds[pred] = true
	}

	shard := &KernelShard{
		domain:          config.Domain,
		kernel:          kernel,
		ownedPredicates: ownedPreds,
		exportedPreds:   config.ExportedPreds,
		schemaFiles:     config.SchemaFiles,
		policyFiles:     config.PolicyFiles,
	}

	logging.Kernel("[shard:%s] created (owned=%d predicates, exported=%d predicates)",
		config.Domain, len(ownedPreds), len(config.ExportedPreds))

	return shard, nil
}

// Domain returns the shard's domain name.
func (s *KernelShard) Domain() string {
	return s.domain
}

// OwnsPredicates returns whether this shard is authoritative for the given predicate.
func (s *KernelShard) OwnsPredicate(pred string) bool {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.ownedPredicates[pred]
}

// OwnedPredicateList returns a stable copy of the predicates this shard owns.
// Order is unspecified. Returns nil if no predicates were registered.
func (s *KernelShard) OwnedPredicateList() []string {
	s.mu.RLock()
	defer s.mu.RUnlock()
	if len(s.ownedPredicates) == 0 {
		return nil
	}
	out := make([]string, 0, len(s.ownedPredicates))
	for p := range s.ownedPredicates {
		out = append(out, p)
	}
	return out
}

// Kernel returns the underlying RealKernel for direct access (internal use only).
func (s *KernelShard) Kernel() *RealKernel {
	return s.kernel
}

// router accessors are unexported; the router is installed by the CortexKernel
// at registration time and is otherwise opaque to callers.

// SetRouter installs the per-shard fact router. Passing nil restores the
// shard to single-store mode (every Assert/Query hits the local kernel).
// Callers should hold no other locks when invoking this method.
func (s *KernelShard) SetRouter(r *ShardFactRouter) {
	s.mu.Lock()
	s.router = r
	s.mu.Unlock()
}

// getRouter returns the currently-installed router, if any.
func (s *KernelShard) getRouter() *ShardFactRouter {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.router
}

// Assert delegates to the inner kernel, or routes via the coordinator when
// the shard does not own the predicate and a router has been installed.
func (s *KernelShard) Assert(fact types.Fact) error {
	if r := s.getRouter(); r != nil && !s.OwnsPredicate(barePredicate(fact.Predicate)) {
		return r.AssertVia(s, fact)
	}
	return s.assertLocal(fact)
}

// assertLocal asserts a fact into THIS shard's inner kernel unconditionally.
// It bypasses router dispatch and is used by the router itself to land facts
// in their owning shard. Callers outside this file should not invoke it.
func (s *KernelShard) assertLocal(fact types.Fact) error {
	s.mu.Lock()
	s.dirtyCount++
	s.mu.Unlock()
	return s.kernel.Assert(fact)
}

// AssertBatch delegates to the inner kernel, or routes facts to their owning
// shards via the coordinator when a router has been installed.
func (s *KernelShard) AssertBatch(facts []types.Fact) error {
	if r := s.getRouter(); r != nil {
		// Fast path: every fact is owned by this shard.
		allLocal := true
		for _, f := range facts {
			if !s.OwnsPredicate(barePredicate(f.Predicate)) {
				allLocal = false
				break
			}
		}
		if !allLocal {
			return r.AssertBatchVia(s, facts)
		}
	}
	return s.assertBatchLocal(facts)
}

// assertBatchLocal asserts a batch into THIS shard's inner kernel
// unconditionally. Router-internal use only.
func (s *KernelShard) assertBatchLocal(facts []types.Fact) error {
	s.mu.Lock()
	s.dirtyCount++
	s.mu.Unlock()
	return s.kernel.AssertBatch(facts)
}

// Retract delegates to the inner kernel, or routes to the owning shard when
// a router has been installed and this shard does not own the predicate.
func (s *KernelShard) Retract(predicate string) error {
	if r := s.getRouter(); r != nil && !s.OwnsPredicate(barePredicate(predicate)) {
		return r.RetractVia(s, predicate)
	}
	return s.retractLocal(predicate)
}

// retractLocal removes facts of a predicate from THIS shard's inner kernel.
// Router-internal use only.
func (s *KernelShard) retractLocal(predicate string) error {
	s.mu.Lock()
	s.dirtyCount++
	s.mu.Unlock()
	return s.kernel.Retract(predicate)
}

// RetractFact delegates to the inner kernel, or routes to the owning shard.
func (s *KernelShard) RetractFact(fact types.Fact) error {
	if r := s.getRouter(); r != nil && !s.OwnsPredicate(barePredicate(fact.Predicate)) {
		return r.RetractFactVia(s, fact)
	}
	return s.retractFactLocal(fact)
}

// retractFactLocal removes a specific fact from THIS shard's inner kernel.
// Router-internal use only.
func (s *KernelShard) retractFactLocal(fact types.Fact) error {
	s.mu.Lock()
	s.dirtyCount++
	s.mu.Unlock()
	return s.kernel.RetractFact(fact)
}

// Query delegates to the inner kernel and tracks metrics, or routes to the
// owning shard via the coordinator when this shard does not own the
// predicate and a router has been installed.
func (s *KernelShard) Query(predicate string) ([]types.Fact, error) {
	if r := s.getRouter(); r != nil && !s.OwnsPredicate(barePredicate(predicate)) {
		return r.QueryVia(s, predicate)
	}
	return s.queryLocal(predicate)
}

// queryLocal queries THIS shard's inner kernel unconditionally and tracks
// metrics. Router-internal use only.
func (s *KernelShard) queryLocal(predicate string) ([]types.Fact, error) {
	s.mu.Lock()
	s.queryCount++
	s.mu.Unlock()

	start := time.Now()
	results, err := s.kernel.Query(predicate)
	elapsed := time.Since(start)

	// Track eval duration if evaluation was triggered (lazy eval)
	if elapsed > 10*time.Millisecond {
		s.mu.Lock()
		s.lastEvalDuration = elapsed
		s.evalCount++
		s.mu.Unlock()
		logging.KernelDebug("[shard:%s] Query triggered eval: %v (predicate=%s, results=%d)",
			s.domain, elapsed, predicate, len(results))
	}

	return results, err
}

// LoadFacts delegates to the inner kernel.
func (s *KernelShard) LoadFacts(facts []types.Fact) error {
	return s.kernel.LoadFacts(facts)
}

// LoadSchemas loads the shard's domain-specific schema content into the kernel.
func (s *KernelShard) LoadSchemas(schemaContent string) {
	s.kernel.LoadSchemas(schemaContent)
}

// LoadPolicy loads the shard's domain-specific policy content into the kernel.
func (s *KernelShard) LoadPolicy(policyContent string) {
	s.kernel.LoadPolicy(policyContent)
}

// AppendPolicy adds additional policy rules to the kernel.
func (s *KernelShard) AppendPolicy(policy string) {
	s.kernel.AppendPolicy(policy)
}

// Evaluate forces a fixpoint evaluation on the inner kernel.
func (s *KernelShard) Evaluate() error {
	start := time.Now()
	err := s.kernel.Evaluate()
	elapsed := time.Since(start)

	s.mu.Lock()
	s.lastEvalDuration = elapsed
	s.evalCount++
	s.mu.Unlock()

	logging.KernelDebug("[shard:%s] Evaluate: %v (facts=%d)", s.domain, elapsed, s.FactCount())
	return err
}

// FactCount returns the number of EDB facts in this shard.
func (s *KernelShard) FactCount() int {
	return s.kernel.FactCount()
}

// IsDirty returns whether the shard has uncommitted mutations.
func (s *KernelShard) IsDirty() bool {
	return s.kernel.IsDirty()
}

// ExportCallbacks builds external predicate callbacks that allow the cortex
// kernel to query this shard's facts during its own fixpoint evaluation.
//
// Each exported predicate becomes an engine.ExternalPredicateCallback that
// queries this shard's store. The cortex registers these callbacks so when
// it evaluates cross-domain rules, it can pull facts from children on demand.
func (s *KernelShard) ExportCallbacks() map[ast.PredicateSym]engine.ExternalPredicateCallback {
	callbacks := make(map[ast.PredicateSym]engine.ExternalPredicateCallback)

	programInfo := s.kernel.GetProgramInfo()
	if programInfo == nil || programInfo.Decls == nil {
		return callbacks
	}

	for _, predName := range s.exportedPreds {
		// Find the matching PredicateSym in this shard's declarations
		for pred := range programInfo.Decls {
			if pred.Symbol == predName {
				// Capture for closure
				capturedPred := pred
				capturedShard := s

				callbacks[capturedPred] = &shardExternalCallback{
					shard:     capturedShard,
					predicate: capturedPred,
				}
				break
			}
		}
	}

	logging.KernelDebug("[shard:%s] ExportCallbacks: exported %d/%d predicates",
		s.domain, len(callbacks), len(s.exportedPreds))

	return callbacks
}

// Metrics returns observability metrics for this shard.
func (s *KernelShard) Metrics() ShardMetrics {
	s.mu.RLock()
	defer s.mu.RUnlock()

	return ShardMetrics{
		Domain:           s.domain,
		FactCount:        s.FactCount(),
		EvalCount:        s.evalCount,
		QueryCount:       s.queryCount,
		LastEvalDuration: s.lastEvalDuration,
		DirtyCount:       s.dirtyCount,
		ExportHitCount:   s.exportHitCount,
	}
}

// ShardMetrics contains observability metrics for a single shard.
type ShardMetrics struct {
	Domain           string
	FactCount        int
	EvalCount        int64
	QueryCount       int64
	LastEvalDuration time.Duration
	DirtyCount       int64
	ExportHitCount   int64
}

// =============================================================================
// SHARD EXTERNAL CALLBACK — Cross-Kernel Predicate Bridge
// =============================================================================

// shardExternalCallback implements engine.ExternalPredicateCallback.
// It bridges a child shard's predicate into the cortex kernel's evaluation
// using the Mangle engine's native external predicate API.
type shardExternalCallback struct {
	shard     *KernelShard
	predicate ast.PredicateSym
}

// ShouldPushdown returns false — we do full scans from the child shard.
func (cb *shardExternalCallback) ShouldPushdown() bool {
	return false
}

// ShouldQuery always returns true — child shard queries are in-process.
func (cb *shardExternalCallback) ShouldQuery(inputs []ast.Constant, filters []ast.BaseTerm, pushdown []ast.Term) bool {
	return true
}

// ExecuteQuery queries the child shard's store for facts matching the predicate.
// It reconstructs the query from inputs/filters, fetches from the child's store,
// and emits output tuples via the callback.
func (cb *shardExternalCallback) ExecuteQuery(inputs []ast.Constant, filters []ast.BaseTerm, pushdown []ast.Term, emit func([]ast.BaseTerm)) error {
	cb.shard.mu.Lock()
	cb.shard.exportHitCount++
	cb.shard.mu.Unlock()

	// Ensure the child shard is evaluated before querying
	if cb.shard.IsDirty() {
		if err := cb.shard.Evaluate(); err != nil {
			return fmt.Errorf("[shard:%s] lazy eval for export failed: %w", cb.shard.domain, err)
		}
	}

	// Get the child shard's store and query it
	store := cb.shard.kernel.GetStore()
	if store == nil {
		return nil
	}

	resultCount := 0
	store.GetFacts(ast.NewQuery(cb.predicate), func(a ast.Atom) error {
		// Emit all args as output terms
		emit(a.Args)
		resultCount++
		return nil
	})

	logging.KernelDebug("[shard:%s] ExportCallback: %s returned %d facts",
		cb.shard.domain, cb.predicate.Symbol, resultCount)

	return nil
}
