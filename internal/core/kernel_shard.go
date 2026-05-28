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

// Kernel returns the underlying RealKernel for direct access (internal use only).
func (s *KernelShard) Kernel() *RealKernel {
	return s.kernel
}

// Assert delegates to the inner kernel.
func (s *KernelShard) Assert(fact types.Fact) error {
	s.mu.Lock()
	s.dirtyCount++
	s.mu.Unlock()
	return s.kernel.Assert(fact)
}

// AssertBatch delegates to the inner kernel.
func (s *KernelShard) AssertBatch(facts []types.Fact) error {
	s.mu.Lock()
	s.dirtyCount++
	s.mu.Unlock()
	return s.kernel.AssertBatch(facts)
}

// Retract delegates to the inner kernel.
func (s *KernelShard) Retract(predicate string) error {
	s.mu.Lock()
	s.dirtyCount++
	s.mu.Unlock()
	return s.kernel.Retract(predicate)
}

// RetractFact delegates to the inner kernel.
func (s *KernelShard) RetractFact(fact types.Fact) error {
	s.mu.Lock()
	s.dirtyCount++
	s.mu.Unlock()
	return s.kernel.RetractFact(fact)
}

// Query delegates to the inner kernel and tracks metrics.
func (s *KernelShard) Query(predicate string) ([]types.Fact, error) {
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
