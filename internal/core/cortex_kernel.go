package core

import (
	"fmt"
	"os"
	"strings"
	"sync"
	"time"

	"codenerd/internal/logging"
	"codenerd/internal/types"

	"github.com/google/mangle/analysis"
)

// =============================================================================
// CORTEX KERNEL — Hierarchical Kernel Hub
// =============================================================================
//
// CortexKernel federates a collection of domain-specific KernelShards.
// It implements the types.Kernel interface so it can be used as a drop-in
// replacement for RealKernel in the rest of the system.
//
// Routing logic:
//   - Mutations (Assert/Retract) are routed to the shard that owns the predicate.
//   - Queries are routed to the authoritative shard for that predicate.
//   - If no shard owns a predicate, it goes to the "cortex" shard (catch-all).
//
// Cross-domain predicates are bridged via ExternalPredicateCallbacks — each
// child exports selected predicates that the cortex shard can query during
// its own fixpoint evaluation.

// CortexKernel is the top-level kernel that manages domain shards.
type CortexKernel struct {
	mu     sync.RWMutex
	shards map[string]*KernelShard // domain -> shard

	// Routing tables (predicate -> domain name)
	predicateOwner map[string]string // predicate -> domain that owns it

	// The "cortex" shard handles unowned predicates and cross-domain rules.
	cortexDomain string

	// Metrics
	routeMissCount int64 // Mutations/queries for unowned predicates
	routeHitCount  int64 // Successfully routed mutations/queries
}

// NewCortexKernel creates a new hierarchical kernel hub.
// The cortexDomain is the name of the catch-all shard for unowned predicates.
func NewCortexKernel(cortexDomain string) *CortexKernel {
	return &CortexKernel{
		shards:         make(map[string]*KernelShard),
		predicateOwner: make(map[string]string),
		cortexDomain:   cortexDomain,
	}
}

// RegisterShard adds a domain shard to the cortex.
// The shard's owned predicates are indexed for fast routing.
func (c *CortexKernel) RegisterShard(shard *KernelShard) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	domain := shard.Domain()
	if _, exists := c.shards[domain]; exists {
		return fmt.Errorf("domain %s already registered", domain)
	}

	c.shards[domain] = shard

	// Index owned predicates for routing
	for pred := range shard.ownedPredicates {
		if existingDomain, conflict := c.predicateOwner[pred]; conflict {
			logging.Get(logging.CategoryKernel).Warn(
				"[cortex] predicate '%s' claimed by both '%s' and '%s' — last wins",
				pred, existingDomain, domain)
		}
		c.predicateOwner[pred] = domain
	}

	logging.Kernel("[cortex] registered shard '%s' (owned=%d predicates)",
		domain, len(shard.ownedPredicates))
	return nil
}

// GetShard returns a shard by domain name.
func (c *CortexKernel) GetShard(domain string) (*KernelShard, bool) {
	c.mu.RLock()
	defer c.mu.RUnlock()
	shard, ok := c.shards[domain]
	return shard, ok
}

// routeToShard returns the shard that owns the given predicate.
// Falls back to the cortex shard if no domain claims ownership.
func (c *CortexKernel) routeToShard(predicate string) *KernelShard {
	// Extract bare predicate name from pattern like "user_intent(/current, X)"
	barePred := predicate
	if idx := strings.Index(predicate, "("); idx > 0 {
		barePred = strings.TrimSpace(predicate[:idx])
	}

	c.mu.RLock()
	defer c.mu.RUnlock()

	if domain, ok := c.predicateOwner[barePred]; ok {
		if shard, ok := c.shards[domain]; ok {
			c.mu.RUnlock()
			c.mu.Lock()
			c.routeHitCount++
			c.mu.Unlock()
			c.mu.RLock()
			return shard
		}
	}

	// Route miss — use cortex shard
	c.mu.RUnlock()
	c.mu.Lock()
	c.routeMissCount++
	c.mu.Unlock()
	c.mu.RLock()

	if shard, ok := c.shards[c.cortexDomain]; ok {
		return shard
	}

	// No cortex shard — return first available shard as last resort
	for _, shard := range c.shards {
		return shard
	}
	return nil
}

// =============================================================================
// types.Kernel INTERFACE IMPLEMENTATION
// =============================================================================

// Assert routes a fact to the owning shard based on predicate.
func (c *CortexKernel) Assert(fact types.Fact) error {
	shard := c.routeToShard(fact.Predicate)
	if shard == nil {
		return fmt.Errorf("[cortex] no shard available for predicate '%s'", fact.Predicate)
	}
	return shard.Assert(fact)
}

// AssertBatch routes facts to their respective shards, batching per-shard.
func (c *CortexKernel) AssertBatch(facts []types.Fact) error {
	// Group facts by destination shard
	batches := make(map[string][]types.Fact)
	for _, fact := range facts {
		shard := c.routeToShard(fact.Predicate)
		if shard == nil {
			return fmt.Errorf("[cortex] no shard available for predicate '%s'", fact.Predicate)
		}
		batches[shard.Domain()] = append(batches[shard.Domain()], fact)
	}

	// Assert each batch to its shard
	for domain, batch := range batches {
		shard, _ := c.GetShard(domain)
		if shard == nil {
			continue
		}
		if err := shard.AssertBatch(batch); err != nil {
			return fmt.Errorf("[cortex] shard '%s' AssertBatch failed: %w", domain, err)
		}
	}
	return nil
}

// Retract removes all facts of a predicate from the owning shard.
func (c *CortexKernel) Retract(predicate string) error {
	shard := c.routeToShard(predicate)
	if shard == nil {
		return fmt.Errorf("[cortex] no shard available for predicate '%s'", predicate)
	}
	return shard.Retract(predicate)
}

// RetractFact removes a specific fact from the owning shard.
func (c *CortexKernel) RetractFact(fact types.Fact) error {
	shard := c.routeToShard(fact.Predicate)
	if shard == nil {
		return fmt.Errorf("[cortex] no shard available for predicate '%s'", fact.Predicate)
	}
	return shard.RetractFact(fact)
}

// RetractExactFactsBatch removes exact facts, routing each to the correct shard.
func (c *CortexKernel) RetractExactFactsBatch(facts []types.Fact) error {
	// Group by shard
	batches := make(map[string][]types.Fact)
	for _, fact := range facts {
		shard := c.routeToShard(fact.Predicate)
		if shard == nil {
			continue
		}
		batches[shard.Domain()] = append(batches[shard.Domain()], fact)
	}

	for domain, batch := range batches {
		shard, _ := c.GetShard(domain)
		if shard == nil {
			continue
		}
		if err := shard.kernel.RetractExactFactsBatch(batch); err != nil {
			return fmt.Errorf("[cortex] shard '%s' RetractExactFactsBatch failed: %w", domain, err)
		}
	}
	return nil
}

// RemoveFactsByPredicateSet removes all facts with predicates in the set, routing to correct shards.
func (c *CortexKernel) RemoveFactsByPredicateSet(predicates map[string]struct{}) error {
	// Group predicates by owning shard
	shardPreds := make(map[string]map[string]struct{})
	for pred := range predicates {
		shard := c.routeToShard(pred)
		if shard == nil {
			continue
		}
		domain := shard.Domain()
		if shardPreds[domain] == nil {
			shardPreds[domain] = make(map[string]struct{})
		}
		shardPreds[domain][pred] = struct{}{}
	}

	for domain, preds := range shardPreds {
		shard, _ := c.GetShard(domain)
		if shard == nil {
			continue
		}
		if err := shard.kernel.RemoveFactsByPredicateSet(preds); err != nil {
			return fmt.Errorf("[cortex] shard '%s' RemoveFactsByPredicateSet failed: %w", domain, err)
		}
	}
	return nil
}

// Query routes a query to the owning shard.
func (c *CortexKernel) Query(predicate string) ([]types.Fact, error) {
	shard := c.routeToShard(predicate)
	if shard == nil {
		return nil, fmt.Errorf("[cortex] no shard available for predicate '%s'", predicate)
	}
	return shard.Query(predicate)
}

// QueryAll returns all derived facts from ALL shards, merged.
func (c *CortexKernel) QueryAll() (map[string][]types.Fact, error) {
	c.mu.RLock()
	defer c.mu.RUnlock()

	merged := make(map[string][]types.Fact)
	for domain, shard := range c.shards {
		results, err := shard.kernel.QueryAll()
		if err != nil {
			return nil, fmt.Errorf("[cortex] shard '%s' QueryAll failed: %w", domain, err)
		}
		for pred, facts := range results {
			merged[pred] = append(merged[pred], facts...)
		}
	}
	return merged, nil
}

// LoadFacts distributes facts to their respective shards.
func (c *CortexKernel) LoadFacts(facts []types.Fact) error {
	// Group by destination shard
	batches := make(map[string][]types.Fact)
	for _, fact := range facts {
		shard := c.routeToShard(fact.Predicate)
		if shard == nil {
			return fmt.Errorf("[cortex] no shard available for predicate '%s'", fact.Predicate)
		}
		batches[shard.Domain()] = append(batches[shard.Domain()], fact)
	}

	for domain, batch := range batches {
		shard, _ := c.GetShard(domain)
		if shard == nil {
			continue
		}
		if err := shard.LoadFacts(batch); err != nil {
			return fmt.Errorf("[cortex] shard '%s' LoadFacts failed: %w", domain, err)
		}
	}
	return nil
}

// UpdateSystemFacts updates system facts across all shards.
func (c *CortexKernel) UpdateSystemFacts() error {
	c.mu.RLock()
	defer c.mu.RUnlock()

	for domain, shard := range c.shards {
		if err := shard.kernel.UpdateSystemFacts(); err != nil {
			return fmt.Errorf("[cortex] shard '%s' UpdateSystemFacts failed: %w", domain, err)
		}
	}
	return nil
}

// GetProgramInfo returns the cortex shard's ProgramInfo.
// For domain-specific program info, use GetShard directly.
func (c *CortexKernel) GetProgramInfo() *analysis.ProgramInfo {
	c.mu.RLock()
	defer c.mu.RUnlock()

	if shard, ok := c.shards[c.cortexDomain]; ok {
		return shard.kernel.GetProgramInfo()
	}
	return nil
}

// Reset clears all facts in all shards while keeping schemas/policies.
func (c *CortexKernel) Reset() {
	c.mu.RLock()
	defer c.mu.RUnlock()

	for _, shard := range c.shards {
		shard.kernel.Reset()
	}
	logging.Kernel("[cortex] all shards reset")
}

// AppendPolicy adds policy rules to the cortex shard.
func (c *CortexKernel) AppendPolicy(policy string) {
	c.mu.RLock()
	defer c.mu.RUnlock()

	if shard, ok := c.shards[c.cortexDomain]; ok {
		shard.AppendPolicy(policy)
	}
}

// =============================================================================
// CORTEX TRANSACTION — Batched Multi-Shard Mutations
// =============================================================================

// CortexTransaction batches mutations across shards and commits atomically.
type CortexTransaction struct {
	cortex  *CortexKernel
	asserts []types.Fact
	retracts []retractOp
}

type retractOp struct {
	predicate string
	fact      *types.Fact // nil means retract all of predicate
}

// Transaction creates a new batched transaction across all shards.
func (c *CortexKernel) Transaction() *CortexTransaction {
	return &CortexTransaction{
		cortex:  c,
		asserts: make([]types.Fact, 0, 32),
		retracts: make([]retractOp, 0, 16),
	}
}

// Assert queues a fact assertion.
func (t *CortexTransaction) Assert(fact types.Fact) {
	t.asserts = append(t.asserts, fact)
}

// Retract queues a predicate retraction (all facts of that predicate).
func (t *CortexTransaction) Retract(predicate string) {
	t.retracts = append(t.retracts, retractOp{predicate: predicate})
}

// RetractFact queues an exact fact retraction.
func (t *CortexTransaction) RetractFact(fact types.Fact) {
	t.retracts = append(t.retracts, retractOp{predicate: fact.Predicate, fact: &fact})
}

// Commit executes all queued operations, routing each to the correct shard.
// Retracts execute first, then asserts. Each shard's kernel gets at most
// one Transaction+Commit, minimizing fixpoint evaluations.
func (t *CortexTransaction) Commit() error {
	timer := logging.StartTimer(logging.CategoryKernel, "CortexTransaction.Commit")

	// Group operations by shard domain
	type shardOps struct {
		retracts []retractOp
		asserts  []types.Fact
	}
	perShard := make(map[string]*shardOps)

	getOps := func(pred string) *shardOps {
		shard := t.cortex.routeToShard(pred)
		if shard == nil {
			return nil
		}
		domain := shard.Domain()
		if perShard[domain] == nil {
			perShard[domain] = &shardOps{}
		}
		return perShard[domain]
	}

	for _, r := range t.retracts {
		ops := getOps(r.predicate)
		if ops != nil {
			ops.retracts = append(ops.retracts, r)
		}
	}
	for _, a := range t.asserts {
		ops := getOps(a.Predicate)
		if ops != nil {
			ops.asserts = append(ops.asserts, a)
		}
	}

	// Execute per-shard transactions
	for domain, ops := range perShard {
		shard, ok := t.cortex.GetShard(domain)
		if !ok {
			continue
		}

		tx := shard.kernel.Transaction()
		for _, r := range ops.retracts {
			if r.fact != nil {
				tx.RetractFact(*r.fact)
			} else {
				tx.Retract(r.predicate)
			}
		}
		for _, a := range ops.asserts {
			tx.Assert(a)
		}
		if err := tx.Commit(); err != nil {
			timer.Stop()
			return fmt.Errorf("[cortex] shard '%s' transaction commit failed: %w", domain, err)
		}
	}

	elapsed := timer.Stop()
	logging.Kernel("[cortex] transaction committed: %d retracts, %d asserts across %d shards (%dms)",
		len(t.retracts), len(t.asserts), len(perShard), elapsed.Milliseconds())
	return nil
}

// =============================================================================
// OBSERVABILITY
// =============================================================================

// AllMetrics returns observability metrics for all shards.
func (c *CortexKernel) AllMetrics() []ShardMetrics {
	c.mu.RLock()
	defer c.mu.RUnlock()

	metrics := make([]ShardMetrics, 0, len(c.shards))
	for _, shard := range c.shards {
		metrics = append(metrics, shard.Metrics())
	}
	return metrics
}

// LogMetrics logs a summary of all shard metrics.
func (c *CortexKernel) LogMetrics() {
	metrics := c.AllMetrics()
	totalFacts := 0
	totalEvals := int64(0)
	totalQueries := int64(0)

	for _, m := range metrics {
		totalFacts += m.FactCount
		totalEvals += m.EvalCount
		totalQueries += m.QueryCount
		logging.Kernel("[cortex] shard=%s facts=%d evals=%d queries=%d lastEval=%v dirty=%d exports=%d",
			m.Domain, m.FactCount, m.EvalCount, m.QueryCount,
			m.LastEvalDuration, m.DirtyCount, m.ExportHitCount)
	}

	c.mu.RLock()
	routeHits := c.routeHitCount
	routeMisses := c.routeMissCount
	c.mu.RUnlock()

	logging.Kernel("[cortex] TOTAL: shards=%d facts=%d evals=%d queries=%d routeHits=%d routeMisses=%d",
		len(metrics), totalFacts, totalEvals, totalQueries, routeHits, routeMisses)
}

// TotalFactCount returns the total number of EDB facts across all shards.
func (c *CortexKernel) TotalFactCount() int {
	c.mu.RLock()
	defer c.mu.RUnlock()

	total := 0
	for _, shard := range c.shards {
		total += shard.FactCount()
	}
	return total
}

// EvaluateAll forces fixpoint evaluation on all dirty shards.
// Returns the total time spent evaluating.
func (c *CortexKernel) EvaluateAll() (time.Duration, error) {
	c.mu.RLock()
	defer c.mu.RUnlock()

	var totalTime time.Duration
	for _, shard := range c.shards {
		start := time.Now()
		if err := shard.kernel.Evaluate(); err != nil {
			return totalTime, fmt.Errorf("shard %s evaluate failed: %w", shard.Domain(), err)
		}
		totalTime += time.Since(start)
	}
	return totalTime, nil
}

// =============================================================================
// SYSTEM KERNEL INTERFACE SUPPORT
// =============================================================================

// Evaluate fulfills the SystemKernel interface by evaluating all shards.
func (c *CortexKernel) Evaluate() error {
	_, err := c.EvaluateAll()
	return err
}

// LoadFactsFromFile parses facts from a Mangle file and asserts them to the correct shards.
func (c *CortexKernel) LoadFactsFromFile(path string) error {
	bytes, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	
	parsedFacts, err := ParseFactsFromString(string(bytes))
	if err != nil {
		return err
	}
	
	// Convert core.Fact to types.Fact since Cortex uses types.Fact
	var typeFacts []types.Fact
	for _, f := range parsedFacts {
		typeFacts = append(typeFacts, types.Fact{
			Predicate: f.Predicate,
			Args:      f.Args,
		})
	}
	
	return c.LoadFacts(typeFacts)
}

// ConsumeBootPrompts collects all PROMPT directives from all underlying shards.
func (c *CortexKernel) ConsumeBootPrompts() []HybridPrompt {
	c.mu.RLock()
	defer c.mu.RUnlock()

	var allPrompts []HybridPrompt
	for _, shard := range c.shards {
		prompts := shard.kernel.ConsumeBootPrompts()
		allPrompts = append(allPrompts, prompts...)
	}
	return allPrompts
}
