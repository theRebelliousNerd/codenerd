---
doc-class: deep-dive
subsystem: perception
implementation-status: proposed
last-verified: 2026-10-29
verified-against: 95654a64562f02c00040ac28ca9cda4733c96fd9
verification-method: static-source-inspection
supersedes: []
---

# Taxonomy initialization completeness after a partial write

## Decision and impact boundary

Treat this as a bounded durable initialization and upgrade refinement, not a demonstrated classification outage. `EnsureDefaults` can leave a verb's durable synonym/pattern group incomplete and then consider it complete on every retry because its definition row exists. The same skip prevents adding newly shipped synonyms or patterns for an already-stored verb. No failure frequency, runtime incident, or ordinary classification regression was established.

**The counterevidence is central:** normal construction first installs complete built-in defaults in memory, and normal boot hydration adds persisted rows rather than clearing those defaults. This masks missing durable companion rows during ordinary initialization. The inspected `matchVerbFromCorpus` entry is exposed by `DebugTaxonomy`, not established as the canonical ordinary NLU path; the inspected shipped verifier uses a no-store path. Therefore the source does not justify saying ordinary user turns lose synonyms, fail routing, or select the wrong persona. A later database-authoritative clear/hydration API demonstrates how missing durable rows can become visible, but its existence does not establish ordinary production reachability.

This is source-only research at [main 95654a6][pin]. No tests, inference, training, application execution, failure injection, or implementation was performed. Only public repository evidence is used.

## Exact persistence mechanism

[`EnsureDefaults`][ensure] builds its `have` map exclusively from `verb_def` facts. When a verb exists, it skips the entire entry. For a missing verb, it writes the definition first, then each synonym, then each pattern, returning immediately upon an error. [`TaxonomyStore`][adapter] maps those operations to separate `StoreFact` calls. [`StoreFact`][write] executes an individual SQL insertion/upsert; there is no group transaction around the calls in `EnsureDefaults`.

A sufficient, unexecuted source-derived sequence is:

1. The definition for a missing built-in verb commits.
2. A later synonym or pattern write fails, or the process stops before it completes.
3. The next invocation loads the committed definition and marks that verb present.
4. It skips the missing companions and can return success without restoring them.

This does not require simultaneous workspaces or shared-owner races. It also does not mean every storage error creates partial state: a failure before the definition commits leaves the verb eligible for the next attempt, and a fully completed group stays complete.

The independent upgrade case needs no failure: add a new built-in synonym or pattern to an already-stored verb. Its definition exists, so this routine does not add the new companion. Whether existing workspace customization should intentionally suppress such additions is an ownership decision the current per-verb Boolean does not express.

## Wiring, masking, and existing protection

The normal [system factory][factory] attaches the taxonomy store, calls `EnsureDefaults`, reports a warning on failure, and then calls `HydrateFromDB`. Thus the persistence initializer is production-wired; it is not dead code. But [`NewTaxonomyEngine`][constructor] has already populated the in-memory default definitions, synonyms and patterns. [`hydrateFromDBLocked`][hydrate] adds persisted facts and refreshes the shared corpus without first clearing those defaults.

[`ClassifyInputWithMatches`][clear] does clear the engine EDB and, when a store exists, hydrate that database instead of taking the no-store defaults branch. This is a conditional consumer of durable completeness, not evidence of an ordinary NLU regression. Before claiming user-visible impact, establish a real caller that reaches this store-backed clear/hydration sequence under the normal production configuration. Do not substitute a diagnostic entry or a no-store verifier for that evidence.

The current [diagnostic adapter][debug] calls `matchVerbFromCorpus`, and the [shipped verifier][verifier] invokes that adapter without attaching a persistence store. The architecture's updated [gap disposition][gap] explicitly identifies this as the remaining caller rather than the canonical NLU route. These controls narrow the impact claim.

The existing [missing-verbs regression][test] correctly verifies that an older database receives missing whole verbs, foreign `/custom` survives, and a repeated merge does not increase fact count. That fix must be credited. Its fixture stores only the `/review` definition, but its assertions inspect definition presence and total repeat-call count, not whether `/review` acquires its required companions. This supplies a useful future incomplete-group control without establishing that the current tests were run or failed.

## Proposed contract

Define the operation's promise explicitly: after successful reconciliation, every applicable built-in seed group has its required definition and companion tuples durably present, or an explicit customization/exclusion policy explains which companions were intentionally withheld. A parent row is not a completion marker.

The smallest compatible repair is additive tuple-level reconciliation of missing built-in synonyms/patterns plus transactional insertion of a new group. Keep user/learned additions. Do not update existing definition metadata or remove old rows under the guise of repairing missing companions: those require provenance and an explicit upgrade policy. If a corpus version or completion marker is introduced, publish it only after its required tuples commit; interrupted work must remain retryable.

Go measures and persists typed corpus state. Mangle remains the executive authority. This proposal does not add a Go routing decision, change learning admission, widen permissions, or invent a regex policy bank. Follow the repository's [spec-first guidance][guidance] before any implementation.

## Alternatives and tradeoffs

| Alternative | What it resolves | What remains |
|---|---|---|
| Transaction around each new verb group | Prevents new definition-only groups from interrupted insertion | Does not heal old partial groups or add companions to existing verbs |
| Tuple-level idempotent reconciliation | Heals existing partial groups and supports additive companion upgrades | Readers can observe a partial group during reconciliation unless publication is isolated |
| Reconciliation plus group transaction | Narrow recovery and upgrade contract with atomic new groups | Must define customization and existing-row precedence |
| Versioned built-in corpus overlay with separately owned user additions | Clearer migration, replacement and removal semantics | Larger schema/provenance migration, compatibility, and rollback cost |
| Explicitly retain the current definition-only promise | No implementation cost; documentation becomes honest | Missing companions remain unrepaired, so callers must not interpret success as complete durable initialization |

## Costs and proposed evaluation

No model call or embedding is intrinsically required. Count tuple reads, missing-tuple writes, transaction duration, lock contention, allocation, storage migration work, and boot latency. Avoid blindly rewriting every existing tuple: `StoreFact` refreshes timestamps, type and priority on conflict. Compare no-op boot cost separately from repair and upgrade cost. No speedup or user-quality improvement is claimed.

Future deterministic controls, not executed here:

1. Seed a definition alone, then a definition with only some synonyms, then one missing only patterns. Require exact required tuple closure, not merely verb presence or a row-count increase.
2. Interrupt/fail each write boundary, reopen, retry, and establish repair or atomic rollback. Exercise failures both before and after the definition commit.
3. Introduce a new built-in companion for an existing verb and verify the chosen upgrade/customization contract.
4. Preserve foreign verbs and user additions. Repeat successful reconciliation and verify no unnecessary writes or metadata churn.
5. Verify both normal additive boot hydration and an explicitly store-backed clear/hydration consumer. Keep the no-store path as a distinct control. If ordinary classification impact is claimed later, first establish its live caller and then measure it using deterministic doubles rather than treating diagnostic behavior as production proof.
6. Compare current behavior, transaction-only, tuple-reconciliation, and combined variants on identical fixtures. Report durable exactness, recovery completeness, SQL work and latency separately from classification behavior.

## Relationship to existing research

The latest [literal-fidelity proposal][literal] concerns serialization of learned examples, and the [learned-correction lifecycle audit][lifecycle] concerns learned-example transport, contexts and draining. This proposal is about built-in taxonomy group installation and its retry predicate, not learned-fact escaping, semantic-owner isolation, outbox recovery, or ordinary classification quality. The known earlier JIT carry/cache identity work was excluded rather than rediscovered. Targeted current audit/source searches found no exact treatment of this seed-group mechanism; that does not establish exhaustive review of every historical branch or outstanding PR.

[pin]: https://github.com/theRebelliousNerd/codenerd/commit/95654a64562f02c00040ac28ca9cda4733c96fd9
[ensure]: https://github.com/theRebelliousNerd/codenerd/blob/95654a64562f02c00040ac28ca9cda4733c96fd9/internal/perception/taxonomy.go#L247-L292
[adapter]: https://github.com/theRebelliousNerd/codenerd/blob/95654a64562f02c00040ac28ca9cda4733c96fd9/internal/perception/taxonomy_persistence.go#L21-L34
[write]: https://github.com/theRebelliousNerd/codenerd/blob/95654a64562f02c00040ac28ca9cda4733c96fd9/internal/store/local_cold.go#L63-L89
[factory]: https://github.com/theRebelliousNerd/codenerd/blob/95654a64562f02c00040ac28ca9cda4733c96fd9/internal/system/factory.go#L1182-L1193
[constructor]: https://github.com/theRebelliousNerd/codenerd/blob/95654a64562f02c00040ac28ca9cda4733c96fd9/internal/perception/taxonomy.go#L101-L144
[hydrate]: https://github.com/theRebelliousNerd/codenerd/blob/95654a64562f02c00040ac28ca9cda4733c96fd9/internal/perception/taxonomy.go#L224-L244
[clear]: https://github.com/theRebelliousNerd/codenerd/blob/95654a64562f02c00040ac28ca9cda4733c96fd9/internal/perception/taxonomy.go#L443-L472
[test]: https://github.com/theRebelliousNerd/codenerd/blob/95654a64562f02c00040ac28ca9cda4733c96fd9/internal/perception/taxonomy_uplift_test.go#L58-L129
[guidance]: https://github.com/theRebelliousNerd/codenerd/blob/95654a64562f02c00040ac28ca9cda4733c96fd9/agents.md
[literal]: https://github.com/theRebelliousNerd/codenerd/blob/566290cb86e4fb2dd45ce0e87e7626d88c2a5af0/Docs/audits/learned-example-literal-fidelity.md
[debug]: https://github.com/theRebelliousNerd/codenerd/blob/95654a64562f02c00040ac28ca9cda4733c96fd9/internal/perception/debug.go
[verifier]: https://github.com/theRebelliousNerd/codenerd/blob/95654a64562f02c00040ac28ca9cda4733c96fd9/cmd/tools/verify_taxonomy/main.go
[gap]: https://github.com/theRebelliousNerd/codenerd/blob/95654a64562f02c00040ac28ca9cda4733c96fd9/Docs/architecture/perception/03-GAP-ANALYSIS.md
[lifecycle]: https://github.com/theRebelliousNerd/codenerd/blob/95654a64562f02c00040ac28ca9cda4733c96fd9/Docs/audits/learned-correction-integrity-lifecycle.md

Primary reference: [SQLite atomic commit](https://www.sqlite.org/atomiccommit.html) describes transaction atomicity; applying a transaction here is a proposed design, not evidence this multi-call sequence is already atomic.
