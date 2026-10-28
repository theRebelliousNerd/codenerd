# Learned correction integrity and worker lifecycle

codeNERD already learns intent examples, persists them, retrieves semantic neighbors, and renders them into the normal understanding prompt. The remaining work is not to revive unused constructors. It is to make a correction mean the same thing on the durable backend as in memory, carry the caller's deadline through the actual embedding and SQL operation, and finish accepted background work before its dependencies close.

**Source pin:** `theRebelliousNerd/codenerd` main at [`356f65e0e6bb639adaa90ab36212daa714114871`](https://github.com/theRebelliousNerd/codenerd/commit/356f65e0e6bb639adaa90ab36212daa714114871), commit time 2026-10-28 06:24:06 UTC. Inspection performed 2026-10-28. This is a source-based research and design document. No implementation, repository edits, test executions, production probes, or performance measurements were performed.

## Scope and relationship to existing work

The existing [perception capability recovery document](https://github.com/theRebelliousNerd/codenerd/blob/356f65e0e6bb639adaa90ab36212daa714114871/Docs/audits/perception-capability-recovery.md) already proposes workspace and generation ownership, turn-scoped semantic facts, and learning-to-decision receipts. Its October 28 update says the contextless wrappers were deleted or moved to test seams, while the multi-workspace ownership model and receipts remain deferred. This document does not claim those proposals as new.

The findings here occur with **one workspace and one classifier**. They concern the contract across the active adapters, asynchronous worker, and shutdown sequence:

1. A correction's constraint is stored but not returned by SQL-backed semantic recall.
2. The bounded context supplied for a learned embedding is replaced with `context.Background()` on the durable path.
3. Taxonomy consolidation is not drained by `Cortex.Close` before classifier and LocalStore closure; chat stops it afterward. Its queue also has no closed-admission state.

These are narrower than a complete semantic-owner redesign and can be specified without deciding concurrent workspace support. A lifetime owner is still necessary, but an explicit single-owner runtime contract is a useful first delivery, not a claim of multi-workspace isolation.

The separate [prompt evolution lifecycle document](https://github.com/theRebelliousNerd/codenerd/blob/356f65e0e6bb639adaa90ab36212daa714114871/Docs/audits/prompt-evolution-lifecycle-and-evidence-accounting.md) now records implementation of a bounded queue, drain, generation claims, attribution ledger, and trial admission for **PromptEvolver**. Those implementations do not automatically govern **TaxonomyEngine.ConsolidationWorker**. Similar names should not make one subsystem's completed work count as another's acceptance.

The root [repository contract](https://github.com/theRebelliousNerd/codenerd/blob/356f65e0e6bb639adaa90ab36212daa714114871/agents.md) requires spec-first changes, JIT-first model-facing behavior, normal production validation, and inspection of wiring before deleting unused functions. The [store guidance](https://github.com/theRebelliousNerd/codenerd/blob/356f65e0e6bb639adaa90ab36212daa714114871/internal/store/agents.md) requires recall after reopen and explicit context/ownership behavior; [perception guidance](https://github.com/theRebelliousNerd/codenerd/blob/356f65e0e6bb639adaa90ab36212daa714114871/internal/perception/agents.md) requires context-aware construction and keeps Mangle and session policy as executive authority.

## What the flagged constructors actually enable

`perception.NewLearnedCorpusStore` and `store.NewLearnedCorpusStore` were convenience entries to a real two-layer capability: a perception-facing corpus adapter and a writable SQLite-backed learned-pattern store. On current main the contextless forms are test seams; production uses their `WithContext` replacements. The capability is not dead.

The actual boot and use chain is:

| Stage | Live path | What it establishes |
|---|---|---|
| Workspace and taxonomy setup | `factory.go` sets the shared taxonomy workspace, client, and taxonomy store | The durable learned corpus has a workspace destination and the critic has a configured client |
| Perception boot | `initKernel → InitPerceptionLayerWithContext → InitSemanticClassifierWithContext` | The common Cortex boot attempts semantic initialization, with optional degradation when the parent is alive |
| Learned backend construction | `NewSemanticClassifierFromConfigWithContext → NewLearnedCorpusStoreWithContext → store.NewLearnedCorpusStoreWithContext` | With config, embedding engine, and a workspace, the adapter opens `.nerd/learned_patterns.db` |
| Runtime producer | `Executor.ProcessWithIntent → queueTaxonomyLearning → QueueForLearning → ConsolidationWorker` | Normal turns can enqueue a recent conversation window for critic extraction |
| Correction write | `PersistLearnedFact → SharedSemanticClassifier.AddLearnedPattern → LearnedCorpusStore.Add → backend.AddPattern` | Accepted-by-this-code-path facts can reach durable semantic storage |
| Runtime consumer | `UnderstandingTransducer.ParseIntentWithContext → SemanticClassifier.Classify → LearnedCorpusStore.Search → LLMTransducer.Understand/BuildPrompt` | Retrieved examples affect the normal understanding input |

Pinned evidence: [workspace setup](https://github.com/theRebelliousNerd/codenerd/blob/356f65e0e6bb639adaa90ab36212daa714114871/internal/system/factory.go#L966-L969), [client and taxonomy store setup](https://github.com/theRebelliousNerd/codenerd/blob/356f65e0e6bb639adaa90ab36212daa714114871/internal/system/factory.go#L1164-L1175), [common boot](https://github.com/theRebelliousNerd/codenerd/blob/356f65e0e6bb639adaa90ab36212daa714114871/internal/system/factory.go#L1391-L1416), [learned adapter constructor](https://github.com/theRebelliousNerd/codenerd/blob/356f65e0e6bb639adaa90ab36212daa714114871/internal/perception/semantic_classifier.go#L1123-L1170), [normal executor enqueue](https://github.com/theRebelliousNerd/codenerd/blob/356f65e0e6bb639adaa90ab36212daa714114871/internal/session/executor.go#L1349-L1366), [window selection](https://github.com/theRebelliousNerd/codenerd/blob/356f65e0e6bb639adaa90ab36212daa714114871/internal/session/executor_learning.go#L248-L315), and [normal understanding consumer](https://github.com/theRebelliousNerd/codenerd/blob/356f65e0e6bb639adaa90ab36212daa714114871/internal/perception/understanding_adapter.go#L388-L425).

This separates three systems that must not be conflated:

- **Learned intent examples:** `learned_patterns.db`, taxonomy `learned_exemplar`, semantic retrieval; this document's subject.
- **Per-shard LearningStore:** facts saved and recalled by shard type; not the same constructor or database.
- **Prompt evolution:** candidate prompt atoms, trials, attribution and generation ledger; independently wired by `factory_learning.go`.

## Finding one  Durable semantic recall drops the correction qualifier

### The exact transport loss

`CorpusEntry` and perception's `SemanticMatch` carry `Constraint`. `AddLearnedPattern` builds a `CorpusEntry` containing the supplied constraint. The SQL backend persists it into `learned_patterns.constraint_text`. Thus the problem is not that the write API has no place to put a qualifier. [Write adapter](https://github.com/theRebelliousNerd/codenerd/blob/356f65e0e6bb639adaa90ab36212daa714114871/internal/perception/semantic_classifier.go#L606-L616), [SQL write](https://github.com/theRebelliousNerd/codenerd/blob/356f65e0e6bb639adaa90ab36212daa714114871/internal/store/learned_store.go#L394-L423).

The return path loses it at multiple explicit seams:

- The SQL ANN query selects `lp.constraint_text` and scans it into a local `constraintText`, but never transfers it to the returned match.
- The store package's `SemanticMatch` type has no constraint field.
- The brute-force fallback query selects pattern, verb, target, and embedding, but not the constraint.
- The perception adapter constructs returned matches with `Constraint: ""`.

See [ANN query and scan](https://github.com/theRebelliousNerd/codenerd/blob/356f65e0e6bb639adaa90ab36212daa714114871/internal/store/learned_store.go#L543-L594), [store result type](https://github.com/theRebelliousNerd/codenerd/blob/356f65e0e6bb639adaa90ab36212daa714114871/internal/store/learned_store.go#L616-L625), [fallback query](https://github.com/theRebelliousNerd/codenerd/blob/356f65e0e6bb639adaa90ab36212daa714114871/internal/store/learned_store.go#L470-L509), and [adapter conversion](https://github.com/theRebelliousNerd/codenerd/blob/356f65e0e6bb639adaa90ab36212daa714114871/internal/perception/semantic_classifier.go#L1185-L1213).

The in-memory result conversion instead copies `c.entry.Constraint`. Therefore passing a test against the memory fallback does not establish backend parity. [Memory conversion](https://github.com/theRebelliousNerd/codenerd/blob/356f65e0e6bb639adaa90ab36212daa714114871/internal/perception/semantic_classifier.go#L1094-L1105).

### What this means and what it does not

For a source-derived example, learn a mapping with a nonempty qualifier such as “include the TUI integration.” If that row is later selected by SQL-backed semantic search, the semantic match returned to the understanding prompt has an empty constraint. The normal prompt renderer explicitly prints `Verb`, `Target`, and `Constraint` for eligible examples; it cannot reconstruct a field the adapter discarded. [Prompt rendering](https://github.com/theRebelliousNerd/codenerd/blob/356f65e0e6bb639adaa90ab36212daa714114871/internal/perception/transducer_llm.go#L298-L320).

This is not evidence that every request loses every constraint. Current user input remains present; taxonomy persistence also writes a fact containing the qualifier; another context source could supply it. Nor does a recalled example authorize any action. The supported finding is that **the durable learned-semantic-example path does not preserve its own qualifier**, including after reopen. The prior recovery document correctly noted that the prompt supports constraints; this trace identifies why SQL-backed learned examples fail to populate that support.

### Required capability

Define one learned-example transport contract across memory, SQLite ANN, SQLite brute force, and prompt consumption:

`Example = {id, revision, phrase, verb, target, constraint, confidence, provenance}`

The immediate compatibility fix need not introduce every proposed receipt field. At minimum the existing text fields must round-trip without silent loss, and “qualifier absent in the original” must not be confused with “qualifier unavailable or dropped.” Unknown legacy information should be represented honestly, not guessed.

Keep semantic similarity and confidence separate. A similarity score is not a probability that a learned constraint applies to this request. Current explicit user constraints remain authoritative over recalled examples, and Mangle still decides action permission.

If qualifiers are bounded for prompt safety, do not truncate away a negation and present the remainder as the original full constraint. Prefer an explicitly omitted/degraded example, a bounded typed representation, or a retrievable reference with a visible incompleteness marker. Do not fix this by adding unrelated free-form prompt instructions outside the existing JIT contract.

## Finding two  The durable write discards its operation context

### Where the deadline is lost

`PersistLearnedFact` obtains `learnedPatternContext()`, whose context has `embedding.request_timeout`, and passes it to `AddLearnedPattern`. On the memory fallback, `AddLearnedPattern` calls the embedding engine using that context. On the durable backend it deliberately avoids a duplicate embedding call and delegates embedding to the store. That optimization is valid.

But `LearnedCorpusStore.Add` has no context parameter and calls:

`backend.AddPattern(context.Background(), ...)`

The store then correctly uses the context it receives for both the embedding call and `ExecContext`; it simply receives the wrong one. The precise failure is at the perception adapter, not in the backend's use of its argument. [Deadline creation](https://github.com/theRebelliousNerd/codenerd/blob/356f65e0e6bb639adaa90ab36212daa714114871/internal/perception/learning.go#L17-L28), [caller](https://github.com/theRebelliousNerd/codenerd/blob/356f65e0e6bb639adaa90ab36212daa714114871/internal/perception/learning.go#L289-L300), [single-embedding branch](https://github.com/theRebelliousNerd/codenerd/blob/356f65e0e6bb639adaa90ab36212daa714114871/internal/perception/semantic_classifier.go#L583-L615), [context replacement](https://github.com/theRebelliousNerd/codenerd/blob/356f65e0e6bb639adaa90ab36212daa714114871/internal/perception/semantic_classifier.go#L1217-L1227), [backend embedding and SQL](https://github.com/theRebelliousNerd/codenerd/blob/356f65e0e6bb639adaa90ab36212daa714114871/internal/store/learned_store.go#L348-L423).

This is a hot-operation gap, distinct from the recently completed context-aware constructor. The backend constructor now propagates cancellation through connection admission, readiness, schema transaction and vector backfill. Its acceptance does not prove runtime addition or search cancellation.

### Exact timeout distinctions

Do not describe this as “there is no timeout anywhere” or as a universal infinite hang:

| Layer | Current source behavior | Consequence |
|---|---|---|
| Foreground turn | Enqueues a history window asynchronously | The usual turn does not synchronously wait for the critic and embedding |
| Critic call | `LearnFromInteraction` creates a per-call deadline when configured | The critic has its own model-call bound; this is not the discarded learned-embedding context |
| Learned-pattern operation | `learnedPatternContext` creates a deadline, but durable Add replaces it | Caller cancellation and the operation-wide deadline do not reach backend embedding or SQL |
| Ollama HTTP request | Constructor installs `EmbedRequestTimeout()` into `http.Client.Timeout` | A configured per-request timeout still exists |
| Ollama embedding operation | May perform preflight/model handling, up to three request attempts with backoff, and long-input splitting | A per-request timeout is not an end-to-end learned-operation deadline |
| Model pull | Uses its own pull client and timeout | Pull cost is not interchangeable with one embed request's bound |
| Worker Stop | Drains queued jobs and waits | Drain duration can include many bounded calls; there is no immediate cancellation of all pending work |
| Legacy perception close step | Uses the eight-second legacy step timeout while its goroutine may continue | Returning a timeout is not cancellation, joined cleanup, or a bound on the whole shutdown |

Pinned references: [critic deadline](https://github.com/theRebelliousNerd/codenerd/blob/356f65e0e6bb639adaa90ab36212daa714114871/internal/perception/learning.go#L202-L223), [Ollama client timeout and embedding entry](https://github.com/theRebelliousNerd/codenerd/blob/356f65e0e6bb639adaa90ab36212daa714114871/internal/embedding/ollama.go#L74-L105), [attempts and backoff](https://github.com/theRebelliousNerd/codenerd/blob/356f65e0e6bb639adaa90ab36212daa714114871/internal/embedding/ollama.go#L110-L168), [pull client](https://github.com/theRebelliousNerd/codenerd/blob/356f65e0e6bb639adaa90ab36212daa714114871/internal/embedding/ollama.go#L551-L561), [worker](https://github.com/theRebelliousNerd/codenerd/blob/356f65e0e6bb639adaa90ab36212daa714114871/internal/perception/consolidation.go#L31-L104), and [close helper](https://github.com/theRebelliousNerd/codenerd/blob/356f65e0e6bb639adaa90ab36212daa714114871/internal/system/cortex_close.go#L275-L295).

The comment near `learnedPatternContext` still describes an Ollama literal socket timeout. Current Ollama construction already reads the installed request bound. The implementation, not that stale comment, determines the distinction above. The eight-second constant applies to named legacy close steps, not every close operation or total shutdown; PromptEvolver separately allows its configured record-drain interval plus the legacy step allowance. See the [legacy constant](https://github.com/theRebelliousNerd/codenerd/blob/356f65e0e6bb639adaa90ab36212daa714114871/internal/system/cortex_close.go#L16-L20) and [evolver-specific close bound](https://github.com/theRebelliousNerd/codenerd/blob/356f65e0e6bb639adaa90ab36212daa714114871/internal/system/cortex_close.go#L164-L180).

### Required capability

Pass an explicit context from the selected learning operation into the perception store adapter, backend embedding, and SQL work. Preserve the existing exactly-one-embedding behavior on the durable path. Reject a pre-canceled operation before starting model work or mutation.

There are two legitimate parents and they must not be confused:

- A foreground explicit “learn this” operation can inherit the caller's request context.
- A background accepted job may intentionally outlive the turn that enqueued it. It should inherit a worker-owned lifetime and a per-operation deadline, not the already-completed foreground turn and not an unbounded background root.

Shutdown policy chooses whether to finish or cancel accepted jobs; the adapter should not make that choice accidentally by replacing the context. Preserve typed cancellation and distinguish “not committed,” “durably committed but semantic projection pending,” and “completed.” Do not report a successful semantic update because a different persistence stage succeeded.

Context propagation is necessary but does not by itself guarantee a strict wall-clock return deadline. The backend acquires an ordinary `s.mu.Lock()` after embedding, and that lock wait does not observe context cancellation. Specify context-aware admission to the write critical section, or an explicitly bounded and joined waiting contract; do not disguise an uncancellable wait in a detached goroutine. After acquiring admission, revalidate both `ctx.Err()` and the store's open state before SQL use, because embedding deliberately runs outside the lock. Driver cancellation and mandatory cleanup must also be characterized before claiming a hard end-to-end duration bound. A canceled request can require cleanup time; it must not acquire fresh mutation authority while waiting.

## Finding three  Drain does not own the dependencies it uses

### The current worker contract

`ConsolidationWorker` starts one goroutine, has a buffered queue of 100 histories, drops new work when full, and on `Stop` closes `quit`, drains the queue, and waits. `Stop` is idempotent through `sync.Once`. This is already better than an unbounded detached goroutine per turn; it should be preserved rather than replaced casually. [Worker implementation](https://github.com/theRebelliousNerd/codenerd/blob/356f65e0e6bb639adaa90ab36212daa714114871/internal/perception/consolidation.go#L10-L104).

However, `Enqueue` checks only channel capacity, not whether admission has closed. After `Stop` returns, another call can place a history into an empty queue with no worker left to read it. During shutdown there is also no admission barrier that fixes the set of jobs to be drained. The public enqueue method returns no admission status, so its caller cannot distinguish an accepted job from a dropped one except through logs.

The queue's capacity bounds the number of slices, not necessarily the byte size of all retained history. The critic later clamps transcript fields, but that occurs when processing a job. A useful byte-bound contract must account for the queued strings or define a bounded serialized input before admission.

### Production close ordering

The normal Cortex close path invokes `ClosePerceptionLayer`, which closes only the shared semantic classifier, and subsequently closes `LocalDB`. It does not invoke `ShutdownSharedTaxonomy` or `StopWorker`. The chat model invokes `m.cortex.Close()` first and `perception.ShutdownSharedTaxonomy()` afterward. A repository search at the source pin found production `ShutdownSharedTaxonomy` use in chat shutdown, not in Cortex shutdown; the other inspected caller is test cleanup. [Cortex close](https://github.com/theRebelliousNerd/codenerd/blob/356f65e0e6bb639adaa90ab36212daa714114871/internal/system/cortex_close.go#L200-L225), [perception close](https://github.com/theRebelliousNerd/codenerd/blob/356f65e0e6bb639adaa90ab36212daa714114871/internal/perception/transducer.go#L439-L450), [chat ordering](https://github.com/theRebelliousNerd/codenerd/blob/356f65e0e6bb639adaa90ab36212daa714114871/cmd/nerd/chat/model_lifecycle.go#L80-L98).

This does not prove every exit loses work. If no jobs are queued or in flight, the ordering may be harmless for that exit. If a short-lived process simply terminates, its remaining goroutines terminate too. Neither case establishes acknowledged-learning durability. With work in flight, the source lacks a join-before-close relationship for the stores that `PersistLearnedFact` uses.

A concrete **possible interleaving**, not a reproduced incident, is:

1. The worker enters backend `AddPattern`, snapshots its embedding engine, and performs embedding outside the store mutex.
2. Shutdown closes the learned store while embedding is in flight; store `Close` sets `s.db = nil`.
3. Embedding returns; `AddPattern` acquires the store mutex and calls `s.db.ExecContext` without a closed-store check.

The source supports a nil-database dereference risk under that interleaving. Its incidence has not been measured, and the proposed regression must synchronize the phases rather than rely on sleep timing. Other possible outcomes are that a late worker finds the shared classifier already cleared and skips semantic persistence, or attempts a LocalStore write after closure. [Embedding outside the lock](https://github.com/theRebelliousNerd/codenerd/blob/356f65e0e6bb639adaa90ab36212daa714114871/internal/store/learned_store.go#L364-L399), [store close](https://github.com/theRebelliousNerd/codenerd/blob/356f65e0e6bb639adaa90ab36212daa714114871/internal/store/learned_store.go#L598-L613), [shared classifier clearing](https://github.com/theRebelliousNerd/codenerd/blob/356f65e0e6bb639adaa90ab36212daa714114871/internal/perception/semantic_classifier.go#L1304-L1316), [multi-stage persistence](https://github.com/theRebelliousNerd/codenerd/blob/356f65e0e6bb639adaa90ab36212daa714114871/internal/perception/learning.go#L259-L329).

### Minimal single-owner lifetime contract

Before a multi-workspace redesign, define an explicit single-runtime owner for the taxonomy worker and the dependencies it uses. That owner must handle boot, boot rollback, normal headless exit, chat exit, and same-process reinitialization consistently.

Proposed state machine:

`new → running → admission_closed → draining_or_cancelling → joined → dependencies_closed`

Required invariants:

1. Admission and closure linearize: every submission gets one result, `accepted`, `rejected_full`, or `rejected_closed`.
2. Each accepted job is either processed to a terminal outcome, durably deferred for later processing, or explicitly accounted as abandoned under the configured shutdown policy.
3. The worker's critic, taxonomy engine, semantic backend, embedding engine, and LocalStore remain alive until the jobs that use them have joined.
4. Closing a store is not a substitute for canceling or joining its callers. Store methods also reject use after close defensively.
5. A shutdown timeout returns an incomplete-shutdown result with outstanding work accounted for. It does not claim the worker finished, and it must not allow unjoined work to continue using released dependencies.
6. A stopped package singleton is not silently reused on the next boot. Either reopen a new owned worker or reject reinitialization with a visible contract.
7. Dropped learning is visible as learning loss, not as a successful observation. A full queue may remain an acceptable policy, but it is an explicit choice with counters and admission receipts.

The precise placement can be chosen during spec work. Moving the current chat stop call earlier addresses one ordering edge but does not by itself establish headless ownership, admission closure, cancellation propagation, or restart behavior.

## Persistence outcome and admission boundaries

`PersistLearnedFact` currently hot-loads taxonomy first, attempts knowledge-graph and semantic writes best-effort, then appends the taxonomy file. SQL/semantic failures can be logged while the function returns success if the final file append succeeds. A final file failure can be returned after earlier stores have already changed. That is a deliberate multi-stage operation, not a single transaction. [Current order and error behavior](https://github.com/theRebelliousNerd/codenerd/blob/356f65e0e6bb639adaa90ab36212daa714114871/internal/perception/learning.go#L239-L329).

The earlier recovery document already identified partial-persistence divergence. The narrow extension needed for this work is a result that exposes the semantic projection's outcome and lets lifecycle accounting know what actually finished. For example:

- normalized descriptor accepted or rejected;
- durable authoritative record committed or not committed;
- taxonomy projection installed, pending, or failed;
- semantic projection installed, pending, or failed;
- retry/reconciliation identity and last error class.

Do not require a distributed transaction across all files and databases to restore one qualifier. An incremental approach can first make the existing result honest, then select an authoritative record and idempotent projections if crash recovery is required. A durable outbox increases write and storage cost and needs retention/privacy rules; an in-memory queue alone cannot claim crash durability.

Admission policy is a separate boundary. The chat confirmation path stages candidates and calls `PersistLearnedFact` after confirmation; the background consolidation path calls it directly on a nonempty critic result. The present document does not decide whether those policies should be identical or whether a configured automatic-learning mode is desirable. Before broadening the lifetime of automatic learning, document that decision explicitly in the package spec and ensure the appropriate kernel policy controls it. A critic's confidence, a successful SQL write, and a permitted future action are three different claims. [Chat staging and confirmation](https://github.com/theRebelliousNerd/codenerd/blob/356f65e0e6bb639adaa90ab36212daa714114871/cmd/nerd/chat/model_handlers.go#L297-L400), [worker direct persistence](https://github.com/theRebelliousNerd/codenerd/blob/356f65e0e6bb639adaa90ab36212daa714114871/internal/perception/consolidation.go#L68-L92), [candidate policy](https://github.com/theRebelliousNerd/codenerd/blob/356f65e0e6bb639adaa90ab36212daa714114871/internal/core/defaults/policy/learning.mg).

## Verification plan

These are proposed tests and experiments, **not executed results**. Existing tests establish useful narrower intentions: [reopen recall and fallback](https://github.com/theRebelliousNerd/codenerd/blob/356f65e0e6bb639adaa90ab36212daa714114871/internal/store/learned_uplift_test.go#L23-L82), [in-memory single embedding](https://github.com/theRebelliousNerd/codenerd/blob/356f65e0e6bb639adaa90ab36212daa714114871/internal/perception/semantic_classifier_uplift_test.go#L142-L167), and [deadline-helper shape](https://github.com/theRebelliousNerd/codenerd/blob/356f65e0e6bb639adaa90ab36212daa714114871/internal/perception/learning_uplift_test.go#L193-L205). They do not establish an end-to-end SQL-backed qualifier round trip or prove that the helper's context reaches the embedding operation.

| Scenario | Required observation | Why the normal path matters |
|---|---|---|
| Nonempty constraint written and retrieved immediately | Phrase, verb, target, and qualifier preserved | Direct SQL row inspection alone misses the adapter loss |
| Same record after close and reopen | Same semantic descriptor, not merely same row count | Durable behavior is the capability |
| ANN absent | Brute-force result preserves the same fields and completeness status | Fixing only ANN leaves the fallback defective |
| In-memory and SQL paths with identical embeddings | Same qualified example reaches `BuildPrompt` when selected | Memory-only tests currently miss durable-path differences |
| Current request contradicts a learned qualifier | Current explicit request wins; recalled source remains visible | Transport fidelity must not become unauthorized constraint inheritance |
| Pre-canceled durable Add | No embed, no SQL write, typed cancellation | Proves the context crosses the live adapter |
| Cancellation during a blocked embedding | The operation exits through the passed context and records no completed semantic projection | Use a synchronized context-aware fake, not elapsed-time guessing |
| Retryable HTTP error with small caller budget | Embedding retry work observes the caller deadline and starts no subsequent attempt after cancellation | Distinguishes operation and request clocks without claiming all lock/cleanup waits are bounded |
| Cancellation after embedding, during SQL contention | SQL observes the same context; outcome identifies commit state | An embedding-only test misses the persistence portion |
| Cancellation while waiting for the store write lock | Admission observes cancellation or satisfies its explicit bounded-wait contract; on acquisition, context and closed state are revalidated before any write | Passing a context to SQL does not cancel an ordinary mutex wait |
| Enqueue racing Stop | Every submission has one admission result; accepted set accounted | Proves admission closure, not just idempotent `close(quit)` |
| Enqueue after Stop | `rejected_closed`, no stranded job | Current channel capacity check cannot express this |
| Close while learned embedding is blocked | Cancellation/join precedes backend close; no late nil-database use | Exercises the source-derived interleaving |
| Normal chat shutdown with queued learning | Worker reaches terminal state before classifier and LocalStore close | Chat currently stops it after Cortex close |
| Normal headless Cortex shutdown | Same ordering and loss accounting without TUI cleanup | Avoids a chat-only fix |
| Boot fails after worker admission | Owned work and handles are joined/closed without publication | Constructor cancellation alone is not worker rollback |
| Same-process reopen | A new working queue exists or unsupported lifecycle is rejected explicitly | `sync.Once` stop is not restart |
| Full queue with large histories | Loss status and memory budget are explicit | Number of jobs is not a byte bound |
| One projection fails while another commits | Result distinguishes partial completion and retry identity | Prevents log-only failures from masquerading as learned-semantic success |

Use barriers to place embedding, SQL and shutdown at the intended phase. Keep backend configurations visible: with `sqlite_vec` and cgo as the production build requires, and an intentional missing-index fallback. Preserve normal CodeDOM and world-model context in the end-to-end runs; socket tests isolate a seam but are not full-system acceptance.

For behavioral quality, prepare a small correction corpus with held-out paraphrases, qualified mappings, near-neighbor distractors, explicit revocations, contradictory current instructions, and negative examples that should not inherit a prior constraint. Keep models, tools, prompt atoms, retrieval settings, and repair budget fixed across baseline and treatment. Do not use the generation examples as the only evaluation set. Report both field-preservation failures and downstream intent/constraint correctness; fixing serialization does not establish improved task accuracy by itself.

## Full cost and evaluation accounting

The existing durable Add avoids a duplicate embedding. Retain that property: “make context propagation correct” must not quietly restore two model calls per correction. Replaying an already committed correction should not repurchase its embedding unless its descriptor or embedding-model identity changed and the migration is intentional.

Account for the whole path:

`C_total = C_critic + C_embedding + C_persistence + C_retrieval + C_prompt + C_task_repair + C_shutdown_recovery + C_observability + C_review`

Use disjoint categories and either convert them to one declared monetary unit or report a resource vector. Do not add tokens, milliseconds and reviewer minutes directly.

At minimum collect:

- critic calls, input/output tokens where reported, empty results, accepted candidates and rejected candidates;
- embedding logical operations versus HTTP attempts, long-input chunks, model-pull work, failures and retries;
- SQL operations, bytes and transaction time, queue count and queued bytes, peak retained memory;
- semantic retrieval latency, selected examples, qualified-example inclusion, prompt tokens;
- task correction rate, extra tool/model turns, verified task success and safety regressions;
- shutdown latency, accepted-job completion, abandoned jobs, recovery attempts and unresolved projection backlog;
- review effort and any extra time users spend re-teaching lost constraints.

The values are unknown in this inspection. No percentage saving or reliability improvement is claimed.

For a fixed closed admission set of N serial jobs, drain work is the sum of each job's critic, embedding, persistence, and bookkeeping work. A per-request HTTP timeout does not bound this whole sum; retries, chunks, pulls and all N jobs matter. A bounded queue limits outstanding count, but the current 100-slot capacity alone does not yield a memory-byte guarantee or a maximum shutdown latency. Conversely, adding a total drain budget without a durable deferred state can reduce exit time by discarding useful accepted work. Report that tradeoff rather than calling either result universally better.

Compare four treatments:

1. Current production behavior.
2. Qualifier transport parity only.
3. Parity plus end-to-end operation context.
4. Parity, context, and joined worker lifecycle with explicit admission outcomes.

Run source-equivalent deterministic fixtures first, then normal boot/turn/shutdown workloads. Increase independently the number of queued histories, history bytes, embedding latency, SQL contention, and reopen count. Fair comparisons preserve accepted workload: a treatment that silently drops more jobs is not faster learning.

A successful outcome is fewer lost qualifiers and unaccounted accepted jobs at noninferior task correctness, with visible cost of durable storage and drain. Reduced foreground latency is not expected merely from moving work that was already asynchronous.

## Proposed delivery order and exit criteria

These are local research identifiers, not existing repository commitments.

| Proposed gap | Priority | Scope | Checkable exit |
|---|---|---|---|
| GAP-LCI-01 Qualified learned-example parity | P1 | Store result, perception adapter, ANN and fallback, prompt path | A nonempty qualifier survives immediate recall and reopen on each supported backend and reaches the selected normal exemplar |
| GAP-LCI-02 Operation context continuity | P1 | Runtime Add and its backend embedding/SQL operation | Caller cancellation/deadline survives every adapter; durable path embeds once; pre-canceled work has no side effects |
| GAP-LCI-03 Single-owner consolidation lifetime | P1 | Queue admission, worker cancellation/drain, Cortex/chat close and reopen | Accepted jobs accounted; headless and chat join before dependencies close; no admission after Stop or use-after-close interleaving |
| GAP-LCI-04 Honest projection outcomes | P1 for reporting, P2 for crash recovery | Multi-stage persistence result and optional durable retry record | Partial results are not labeled semantic success; a requested crash-durability contract has restart reconciliation |
| GAP-LCI-05 Behavioral and cost evaluation | P2 | Held-out corrections and normal lifecycle workloads | Quality, loss, latency, resource use and task outcomes reported together, including negative or no-benefit results |

Start by amending the perception/store capability specs with field fidelity and runtime context exits, and the system/session lifetime spec with queue ownership and shutdown order. Then implement the smallest coherent slice and update current-state/wiring rows only with actual verification evidence. Keep the broader multi-workspace owner and decision-receipt proposals linked rather than claiming them completed.

The defining capability is simple: when codeNERD says it has learned a qualified mapping, the durable semantic consumer receives that qualified mapping, and the runtime knows whether the work finished before its resources went away. Reconnecting old constructors cannot establish that contract.

