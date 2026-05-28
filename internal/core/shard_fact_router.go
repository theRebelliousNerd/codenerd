package core

import (
	"fmt"
	"strings"
	"sync"
	"sync/atomic"

	"codenerd/internal/logging"
	"codenerd/internal/types"
)

// =============================================================================
// SHARD FACT ROUTER — Per-Shard Fact Store Coordinator (Track D)
// =============================================================================
//
// ShardFactRouter routes fact mutations and queries from one shard to the
// authoritative owner of a predicate. It is constructed and owned by the
// CortexKernel and handed to each registered KernelShard ONLY when the
// per-shard-facts feature flag is enabled.
//
// Lifecycle:
//   - CortexKernel.RegisterShard builds the router as shards are registered.
//   - Each shard gets a pointer to the same router so cross-shard joins go
//     through a single coordinator.
//   - When the feature flag is OFF the router is never installed; each
//     shard's Assert/Query falls through to its inner *RealKernel exactly
//     as it did before Track D. This keeps the OFF path byte-identical.
//
// Concurrency:
//   - The owner map is built up under the router's own mutex during
//     RegisterShard. It is then read under RLock on every Route call.
//   - Atomic counters track hits/misses without contending with the mutex.
//
// Boundary:
//   - The router does NOT load schemas or evaluate facts. It is purely a
//     dispatch table: predicate name → owning shard. The shards themselves
//     remain the units of evaluation; the router only chooses which one.

// ShardFactRouter maps predicate names to the shard that owns them.
// It is safe for concurrent use after construction.
type ShardFactRouter struct {
	mu        sync.RWMutex
	owners    map[string]*KernelShard // bare predicate name -> owning shard
	hitCount  int64                   // routed to a specific owner
	missCount int64                   // no owner found
	loopCount int64                   // routed back to the calling shard (no-op)
}

// NewShardFactRouter returns a router with no owners registered.
func NewShardFactRouter() *ShardFactRouter {
	return &ShardFactRouter{
		owners: make(map[string]*KernelShard),
	}
}

// RegisterOwner records that the given shard is the authoritative owner of
// the supplied predicates. Subsequent registrations for the same predicate
// log a warning and the LAST writer wins, matching CortexKernel.RegisterShard
// behavior.
func (r *ShardFactRouter) RegisterOwner(shard *KernelShard, predicates []string) error {
	if shard == nil {
		return fmt.Errorf("ShardFactRouter: nil shard")
	}
	r.mu.Lock()
	defer r.mu.Unlock()

	for _, pred := range predicates {
		if pred == "" {
			continue
		}
		if existing, conflict := r.owners[pred]; conflict && existing != shard {
			logging.Get(logging.CategoryKernel).Warn(
				"[shard-router] predicate '%s' claimed by both '%s' and '%s' — last wins",
				pred, existing.Domain(), shard.Domain())
		}
		r.owners[pred] = shard
	}
	return nil
}

// barePredicate extracts the predicate name from a string that may be a bare
// name ("user_intent") or a Mangle query pattern ("user_intent(/current, X)").
// Mirrors CortexKernel.routeToShard's normalization so router decisions stay
// consistent with cortex-level routing.
func barePredicate(predicate string) string {
	if idx := strings.Index(predicate, "("); idx > 0 {
		return strings.TrimSpace(predicate[:idx])
	}
	return predicate
}

// OwnerOf returns the shard registered as the authoritative owner of the
// given predicate, or nil if no shard claims it.
func (r *ShardFactRouter) OwnerOf(predicate string) *KernelShard {
	bare := barePredicate(predicate)
	r.mu.RLock()
	defer r.mu.RUnlock()
	return r.owners[bare]
}

// Route returns the owning shard for the predicate and increments the hit /
// miss counters. The caller (a KernelShard about to assert/query something it
// does not own itself) is passed so we can detect and count "loop" routes —
// the predicate's owner IS the caller — which a well-behaved caller should
// have short-circuited via OwnsPredicate.
func (r *ShardFactRouter) Route(caller *KernelShard, predicate string) *KernelShard {
	owner := r.OwnerOf(predicate)
	if owner == nil {
		atomic.AddInt64(&r.missCount, 1)
		return nil
	}
	if owner == caller {
		atomic.AddInt64(&r.loopCount, 1)
		return owner
	}
	atomic.AddInt64(&r.hitCount, 1)
	return owner
}

// AssertVia routes an Assert through the router. If a different shard owns
// the predicate, that shard's Assert is invoked. If no owner is registered,
// the local kernel is used as a fallback (preserving the pre-router behavior
// for unowned predicates so we never silently drop facts).
//
// `caller` is the shard whose Assert was called; it is passed so we can
// avoid re-entry when the caller IS the owner.
func (r *ShardFactRouter) AssertVia(caller *KernelShard, fact types.Fact) error {
	owner := r.Route(caller, fact.Predicate)
	if owner == nil || owner == caller {
		// No remote owner or owner is the caller itself — local store.
		return caller.assertLocal(fact)
	}
	logging.KernelDebug("[shard-router] Assert routed %s '%s' -> '%s'",
		fact.Predicate, caller.Domain(), owner.Domain())
	return owner.assertLocal(fact)
}

// QueryVia routes a Query through the router. If a different shard owns the
// predicate, that shard's Query is invoked. If no owner is registered, the
// local kernel is used as a fallback.
func (r *ShardFactRouter) QueryVia(caller *KernelShard, predicate string) ([]types.Fact, error) {
	owner := r.Route(caller, predicate)
	if owner == nil || owner == caller {
		return caller.queryLocal(predicate)
	}
	logging.KernelDebug("[shard-router] Query routed %s '%s' -> '%s'",
		predicate, caller.Domain(), owner.Domain())
	return owner.queryLocal(predicate)
}

// AssertBatchVia groups facts by owning shard and asserts each batch in one
// call to the owner. Facts whose predicate has no registered owner are
// asserted to the caller's local store as a fallback.
func (r *ShardFactRouter) AssertBatchVia(caller *KernelShard, facts []types.Fact) error {
	if len(facts) == 0 {
		return nil
	}
	batches := make(map[*KernelShard][]types.Fact, 2)
	for _, f := range facts {
		owner := r.Route(caller, f.Predicate)
		if owner == nil {
			batches[caller] = append(batches[caller], f)
			continue
		}
		batches[owner] = append(batches[owner], f)
	}
	for shard, batch := range batches {
		if err := shard.assertBatchLocal(batch); err != nil {
			return fmt.Errorf("[shard-router] AssertBatch into '%s' failed: %w",
				shard.Domain(), err)
		}
	}
	return nil
}

// RetractVia routes a Retract(predicate) call to the predicate's owner.
func (r *ShardFactRouter) RetractVia(caller *KernelShard, predicate string) error {
	owner := r.Route(caller, predicate)
	if owner == nil || owner == caller {
		return caller.retractLocal(predicate)
	}
	return owner.retractLocal(predicate)
}

// RetractFactVia routes a RetractFact(fact) call to the predicate's owner.
func (r *ShardFactRouter) RetractFactVia(caller *KernelShard, fact types.Fact) error {
	owner := r.Route(caller, fact.Predicate)
	if owner == nil || owner == caller {
		return caller.retractFactLocal(fact)
	}
	return owner.retractFactLocal(fact)
}

// Metrics returns observability counters for the router.
func (r *ShardFactRouter) Metrics() ShardRouterMetrics {
	return ShardRouterMetrics{
		HitCount:  atomic.LoadInt64(&r.hitCount),
		MissCount: atomic.LoadInt64(&r.missCount),
		LoopCount: atomic.LoadInt64(&r.loopCount),
	}
}

// ShardRouterMetrics aggregates routing observability counters.
type ShardRouterMetrics struct {
	HitCount  int64 // routed to a non-caller owner
	MissCount int64 // no owner registered
	LoopCount int64 // owner was the caller (no remote call)
}
