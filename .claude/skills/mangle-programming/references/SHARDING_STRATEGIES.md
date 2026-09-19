# Sharding Strategies for Mangle Kernels

**Audience**: codeNERD architects designing how predicates, facts, and rules get partitioned across kernel instances, shards, or processes.
**Status**: Section 1 documents what is **wired in codeNERD today**. Sections 2–9 are **experimental designs** — promising but not implemented. Treat them as a design playground, not a contract.
**Target Mangle**: `codeberg.org/TauCeti/mangle-go v0.5.1+`. Several experimental patterns lean on fork-only features (temporal annotations, `WithDerivationRecorder`, `WithExternalPredicates`, `WithCreatedFactLimit`, simplecolumn/zstd) — see [960-FORK_FEATURES_v0.5.1](960-FORK_FEATURES_v0.5.1.md).

---

## 0. Why shard Mangle at all?

A single Mangle kernel is a beautifully simple thing: one fact store, one rule corpus, one stratification graph, one evaluation. You should resist sharding for as long as possible. Reasons you might still need it:

1. **Working-set pressure.** Some predicates have millions of facts (code graph, embeddings index); others have a handful (user intent, current OODA state). Holding them in one store hurts cache locality and bloats every iteration of seminaive evaluation.
2. **Rule isolation.** A reviewer shard does not need to know how the dreamer's bedtime stories are derived. Mixing rule corpora means every stratification recomputation touches everyone.
3. **Independent eval cadence.** Perception fires per user turn; long-horizon reflection fires on a clock. Different cadence = different evaluation lifecycle = natural shard boundary.
4. **Safety and provenance scope.** A constitutional policy shard wants to be the **only** writer of `permitted/4`; a coder shard must not be able to fabricate it. Predicate ownership is the formal expression of that boundary.
5. **Concurrent / federated authorship.** Multiple agents writing into one store want lock-free regions. Per-shard stores give you that for free.

If none of those apply, do not shard. The simplest correct kernel is the best kernel.

---

## 1. What codeNERD already does — wired today

### 1.1 Predicate-ownership routing (Track D)

**File**: [internal/core/shard_fact_router.go](../../../../internal/core/shard_fact_router.go)
**Activation**: per-shard-facts feature flag.

Each `KernelShard` declares a list of bare predicate names it owns. `ShardFactRouter` builds the predicate-name → owning-shard map at registration time. On `Assert(pred, args)` or `Query(pred, args)` from any shard, the router resolves the owner and forwards the operation.

```go
type ShardFactRouter struct {
    owners    map[string]*KernelShard
    hitCount  int64
    missCount int64
    loopCount int64
}
```

Properties:

- **Last-writer-wins on collisions.** Two shards claiming the same predicate logs a warning and the later registration takes it.
- **Self-routing is a no-op.** A shard querying its own predicate stays in-shard (`loopCount++`).
- **Misses fall through to a default store**, so the system is forgiving of partial wiring during bring-up.
- **Counters are atomic; map is RLock'd.** Read-heavy operation is uncontended.

This is the canonical pattern for codeNERD: **partition by predicate name, identity-route across shards, no replication**.

### 1.2 Shard typology

Defined in [internal/types/shard.go](../../../../internal/types/shard.go):

| Type | Lifecycle | Backing store | Use |
|------|-----------|---------------|-----|
| `ephemeral` | spawn → execute → die | RAM | One-shot ConfigAtom-driven LLM call |
| `persistent` | long-running, user-defined | SQLite | Specialists with domain knowledge |
| `system` | always on | RAM + tools | OODA, perception, articulation, virtual store |
| `user` | alias for `persistent` | SQLite | User-configured agents under `.nerd/agents/` |

Sharding strategy is **implicit in the lifecycle**: ephemeral shards do not own predicates (their work is funneled through system shards). Persistent shards may own a small predicate cluster representing their specialist knowledge.

### 1.3 Policy-driven shard classification

**File**: [internal/core/defaults/policy/shards.mg](../../../../internal/core/defaults/policy/shards.mg).

The Mangle layer encodes which model class each shard type wants, what counts as a struggling shard (failure-count aggregation), and what the executive should learn from observed traces. This is sharding **as a deductive subject** — the kernel reasons about shards using the same machinery it uses for everything else.

```mangle
Decl shard_failure_count(ShardType, N).
shard_struggling(ShardType) :-
    shard_failure_count(ShardType, N),
    N >= 3.
```

That means new sharding strategies do not need to be hardcoded in Go — they can be expressed as rules over the existing `shard_*` predicates.

### 1.4 What is **not** in the codebase

- No replication, no consensus, no leader election.
- No cross-process kernels. Every shard is in-process.
- No rebalancing — predicate ownership is set at registration and frozen.
- No per-shard gas / fact limits. The kernel has global limits; shards do not get individual budgets.
- No per-shard provenance recorder. Provenance, when wired (via the new `provenance` package), is kernel-wide.

These gaps are the territory the experimental designs below explore.

---

## 2. Partitioning schemes — choosing the shard key

Sharding starts with one question: **what value decides which shard a fact lives in?**

### 2.1 Predicate-name partitioning (what we have)

Shard key = the predicate symbol. Crisp boundaries, trivial routing, but assumes you can cleanly assign whole predicates. Breaks down when a single predicate is too hot (`source_line/2` would be one giant shard) or when the same predicate is read by many shards (it becomes a global bottleneck).

### 2.2 Hash partitioning (proposal)

Shard key = `hash(predicate, key_arg) mod N`. Distributes a hot predicate across `N` shards uniformly. Joins across shards become expensive (you have to fan out, then merge), but writes scale.

Implementation sketch: a `HashShardRouter` wraps the predicate router and, for designated "fat" predicates, hashes the first argument to pick the bucket. The router still presents a single logical predicate to rules; the engine sees the same Mangle program, but `Assert` / `Query` is now O(buckets).

```go
type FatPredicateConfig struct {
    Predicate string
    Arity     int
    KeyArg    int   // which argument to hash
    Buckets   int
}
```

Tradeoff: equality joins (`source_line(F, L)`, `source_line(F, M)`) inside a single rule no longer co-locate naturally. You need to either bias hashing on the join key, or accept fanout.

### 2.3 Range partitioning (proposal)

Shard key = lexicographic / numeric range over a chosen argument. Good for predicates with locality (`source_line/2` keyed by file path: all `internal/core/**` lines live in shard A, all `internal/prompt/**` lines in shard B). Bad when the range distribution is skewed (10× more facts in `internal/core/` than elsewhere).

Use when you need to colocate facts that join often. Pair with periodic rebalancing.

### 2.4 List / explicit partitioning

Shard key = a static mapping from a tag value to a shard. The `shard_type` predicate in `policy/shards.mg` is effectively this. Good for small, stable taxonomies.

### 2.5 Composite (multi-level) partitioning

Shard key = `(predicate_class, hash(key))`. First-level by class (perception vs reflection vs planning), second-level by hash within class. Equivalent to **nested sharding**, covered in §4.

---

## 3. Replicated vs partitioned facts

Mangle's algorithm is monotone within a stratum — if every shard sees the same input, it computes the same output. That means **some predicates want to be replicated, not partitioned**.

| Replicate when | Partition when |
|----------------|----------------|
| Predicate is small and read by many shards (constitution, type bounds, capability matrix). | Predicate is large and write-heavy (code graph, embedding index, observed events). |
| Reads dominate writes by orders of magnitude. | Writes are evenly distributed across the key space. |
| Read latency matters more than write throughput. | Memory pressure of holding the whole thing in one shard dominates. |

Implementation sketch: a `ReplicatedFactSet` is a fact store every shard reads from, with a single writer (typically a system shard). It can use `factstore.SimpleColumnStore` for compact in-memory layout, refreshed on snapshot rotations.

**Anti-pattern**: replicating a hot, write-heavy predicate. You will spend all your time propagating writes and invalidating snapshots. If a predicate is both hot and write-heavy, partition it; if it can't be partitioned, redesign the rule that depends on it.

---

## 4. Nested sharding (hierarchical partitioning)

The user asked specifically about this. The intuition: in a deep agent like codeNERD, predicate scope is naturally hierarchical — there are facts that belong to the **session**, facts that belong to a **shard within that session**, and facts that belong to a **substream of work within that shard**.

### 4.1 Three-level model

```
                ┌─────────────────────────┐
                │     KernelGlobal        │  constitution, type bounds, user_intent
                └──────────┬──────────────┘
                           │
            ┌──────────────┼──────────────┐
            │              │              │
       ┌────▼────┐    ┌────▼────┐    ┌────▼────┐
       │ Coder   │    │ Tester  │    │ Reviewer│   shard-local: persona facts,
       │ shard   │    │ shard   │    │ shard   │   working set, scratch facts
       └────┬────┘    └────┬────┘    └────┬────┘
            │              │              │
       ┌────▼────────┐ ┌───▼────────┐ ┌───▼────────┐
       │ build_X     │ │ test_run_Y │ │ review_Z   │  task-local: per-step facts,
       │ subagent    │ │ subagent   │ │ subagent   │  GC'd after the task completes
       └─────────────┘ └────────────┘ └────────────┘
```

- **Global** facts are replicated and read-only from below.
- **Shard-local** facts are partitioned by shard identity. Other shards reach them only via the router.
- **Task-local** facts live in a transient store created at task spawn and discarded at completion.

### 4.2 Evaluation across levels

Inheritance rule: a rule defined at level *N* sees its own facts plus everything above. A rule at task level can read shard-local facts; the inverse is not true.

This is implementable today by giving each level its own `factstore.FactStore` and composing them with a "stacked" store that consults parents on read-through:

```go
type StackedStore struct {
    parents []factstore.ReadOnlyFactStore // global → shard → task
    local   factstore.FactStore           // writable; task-local
}

func (s *StackedStore) Contains(a ast.Atom) bool {
    if s.local.Contains(a) { return true }
    for _, p := range s.parents { if p.Contains(a) { return true } }
    return false
}
```

Writes only ever go to `local`. Reads walk the stack from cheapest (local) to broadest (global).

### 4.3 Why this is interesting

- **Garbage collection is free**: drop the task-local store and the working set is gone, without touching the kernel-wide state.
- **Provenance is naturally scoped**: attach a `MemoryRecorder` to the task-local stack; you get a proof tree of just that task's derivations.
- **Speculation is cheap**: clone the task store, try a rule, throw the clone away on failure. `unionfind.UnionFind.Clone()` (fork feature) is the same idea at the substitution level.

### 4.4 Risk

If global rules reference shard-local predicates by name, you've defeated the hierarchy. Enforce **upward-only** references: a rule at level *N* may not mention any predicate first declared at a deeper level. Add this as a Mangle-level check using the existing analysis pass.

---

## 5. Stratum sharding — one shard per stratification layer

Mangle already computes a stratification graph at analysis time. The number of strata is bounded by the longest negation/aggregation chain. **What if each stratum became its own shard?**

- Stratum 0: pure EDB facts.
- Stratum 1: rules whose bodies use only stratum-0 predicates.
- Stratum *k+1*: rules whose bodies may use predicates up to stratum *k*, plus negation/aggregation over them.

Each stratum is evaluated to fixed point before the next starts (this is already what `engine.EvalStratifiedProgramWithStats` does internally). Making them shards means:

- Each stratum's working set lives in its own store.
- Lower strata are read-only to higher ones — they finish before the next stratum begins.
- You can pin each stratum to its own goroutine; lower-stratum fact emission can prefetch into the upper-stratum's input cache.

**When this pays off**: rule corpora with many independent rules in the same stratum (high horizontal parallelism). The kernel is doing a join-heavy evaluation; sharing memory traffic across rules in the same stratum hurts.

**When this doesn't pay off**: small programs where most strata have one rule.

Experimental hook: `engine.WithDeterministicOrder` plus careful instrumentation of per-stratum derivation counts (already exposed in `EvalProgramWithStats`) tells you whether splitting by stratum will help before you actually do it.

---

## 6. Temporal sharding (using the fork)

Now that the fork ships temporal facts (`@[start, end]` annotations, `interval-tree-indexed TemporalFactStore`), a new shard key is available: **time**.

### 6.1 Hot / warm / cold time buckets

- **Hot**: facts whose `@[start, end]` overlaps `[now - δ, now + δ]`. Lives in a high-performance in-memory `IntervalTreeStore`.
- **Warm**: facts whose interval is within the last *N* sessions. Lives in a `SimpleColumnStore`.
- **Cold**: facts older than that, compressed as zstd-backed simplecolumn bundles and re-hydrated on demand via `engine.WithExternalPredicates`.

Eviction policy is declarative: a Mangle rule decides what is hot and what is cold. The router consults that policy at startup.

### 6.2 Use cases

- Perception windows: session events, OODA traces, recent tool calls. They are all temporal; old ones do not need to be in-memory.
- Dream-state recompaction: dream cycles run cold-store consolidation rules, emit summary facts into the warm store, and clear the cold bucket of subsumed entries.
- Watchdog timers: facts with `@[T, T+timeout]` automatically lose relevance once `now > T+timeout` advances past them, so they fall off the hot store without bookkeeping.

### 6.3 The Mangle-side ergonomics

```mangle
# Parses and analyses on the pinned engine. codeNERD's kernel has no temporal store,
# so evaluation aborts ("temporal literal encountered but no temporal store
# configured"); until one is wired, tier eviction by age is done in Go.
Decl trace_event(SessionID, Kind) temporal bound [/name, /name].
Decl hot_trace(SessionID, Kind) bound [/name, /name].

# Hot trace events: anything in the last 5 minutes.
hot_trace(SID, K) :- <-[0m, 5m] trace_event(SID, K).
```

The router subscribes to `hot_trace` and `cold_candidate` and migrates facts between stores.

---

## 7. Provenance-aware sharding

The fork's `DerivationRecorder` opens up a sharding axis that did not exist before: **partition by who derived the fact**.

### 7.1 Single-writer provenance shards

Pair each shard with its own `MemoryRecorder`. When a fact crosses a shard boundary via the router, the receiving shard records a synthetic event marking the source. Now every derived fact carries a chain back to the shard that originated each premise.

This makes attribution explicit: a constitutional violation can be traced to the specific shard whose rule contributed the offending premise, not just to "the kernel."

### 7.2 Why this is a sharding strategy, not just observability

If you have per-shard provenance, you can use it to make routing decisions: "facts derived primarily from coder-shard premises should live in the coder shard, even if the predicate name is `world_model_fact`." That's a learned partitioning — let the recorder run for a while, count which shard contributed most to each fact, and reassign ownership.

This is the seed of self-optimizing sharding. None of it is wired today.

---

## 8. Other experimental patterns

### 8.1 Chinese-wall sharding (information barriers)

Some shards must not see certain predicates. A reviewer must see code but not the user's raw prompt; a dreamer must see the OODA trace but not the live tool registry. Encode the barrier as a routing rule: `barred(shard, predicate)` is asserted in policy; the router refuses to forward facts in either direction across the barrier. Mangle's existing safety analysis can verify that no rule reads across a barrier.

### 8.2 Shadow shards (fault-tolerance prototype)

Mirror a shard's writes to a passive shadow. On primary failure, swap routing entries to the shadow. Hot-path overhead is one extra `Assert` per write. Worth the cost only for shards whose state is hard to recompute (research-shard knowledge atoms, long-running session memory).

### 8.3 Bisimulation-driven sharding

Two predicates are **behaviorally equivalent** if every rule treats them the same way. If you detect such pairs (via the analysis pass that already exists), they can be merged into one shard without loss; conversely, if a single predicate has two disjoint behavioral roles in different rule clusters, it can be split. This is sharding as a refactoring, driven by what the rules actually do.

### 8.4 Migrating ("vibrating") shards

Track access patterns: which shard reads predicate *P* most often? Periodically reassign *P*'s ownership to that shard. Implementation: maintain a sliding window of read counts per `(reader_shard, predicate)`. When the dominant reader exceeds a threshold and is not the current owner, propose a migration. Use a quiescent window (e.g. between OODA iterations) to swap ownership atomically.

Risk: oscillation. Two shards reading *P* alternately will cause it to bounce. Damp with hysteresis (require dominance for *N* consecutive windows).

### 8.5 Federated sharding (across processes)

Move the router from in-process to RPC. Each shard runs in its own process; the router becomes a routing fabric speaking gRPC. The codeberg.org `mangle-service` repo (sister project) is the closest reference. Worthwhile when shards have very different resource profiles (the embedding-search shard wants a GPU; the constitution shard wants a single core and lots of cache). Not worthwhile in-tree without a concrete workload that demands it.

### 8.6 Gas-bounded shards

Pair each shard with `engine.WithCreatedFactLimit(N_shard)` rather than one global limit. A runaway rule in the reviewer cannot exhaust the coder's budget. The router reports gas exhaustion as a structured fact; the legislator can promote the shard to a less restrictive class or shut it down.

### 8.7 Dream-compaction shards

The dreamer is already a shard; treat its job as **compaction**. Read the warm and cold stores, derive consolidated summary facts, replace the underlying base facts with the summaries. This is sharding-as-GC: a shard whose job is to shrink other shards.

### 8.8 Stratified-promotion shards

Run a candidate rule corpus inside a sandbox shard. If it derives nothing unsafe within a budget, promote it to the live policy. The promotion is itself a Mangle derivation, so it leaves an audit trail. The legislator and prompt-evolution loops fit this pattern.

---

## 9. Choosing a strategy — practical checklist

Walk these in order. Stop at the first one that answers your problem.

1. **One kernel, no sharding.** Default. Measure first.
2. **Predicate-ownership routing** (what we have). Add when two shards' rule corpora are genuinely independent.
3. **Replication for small, hot, read-heavy predicates.** Add when one shard becomes a contention bottleneck for reads.
4. **Nested (task-local) sharding.** Add when you need disposable working sets per task.
5. **Hash partitioning of a single fat predicate.** Add when one predicate dominates the working set.
6. **Stratum sharding.** Add when stratification metrics show heavy intra-stratum parallelism.
7. **Temporal sharding.** Add when temporal annotations land in policy and the warm/cold ratio is skewed.
8. **Provenance-driven adaptive sharding.** Long-horizon. Requires the `MemoryRecorder` to have been on for many sessions.
9. **Federated, multi-process sharding.** Only when in-process resource asymmetry forces it.

Anti-pattern: skipping straight to (5)–(8) without (2)–(4). The instrumentation isn't there to know whether the more elaborate strategies are paying off, and you will spend the next quarter debugging routing instead of evaluation.

---

## See also

- [960-FORK_FEATURES_v0.5.1](960-FORK_FEATURES_v0.5.1.md) — temporal, provenance, gas limits.
- [700-OPTIMIZATION](700-OPTIMIZATION.md) — single-kernel perf engineering; do all of this before sharding.
- [800-THEORY](800-THEORY.md) — stratification semantics, the formal basis for §5.
- [950-ADVANCED_ARCHITECTURE](950-ADVANCED_ARCHITECTURE.md) — bisimulation and taint analysis, prerequisites for §8.3.
- [internal/core/shard_fact_router.go](../../../../internal/core/shard_fact_router.go) — the canonical implementation.
- [internal/core/defaults/policy/shards.mg](../../../../internal/core/defaults/policy/shards.mg) — current policy-level shard reasoning.
- [internal/shards/README.md](../../../../internal/shards/README.md) — legacy shard architecture and JIT-driven replacement.
