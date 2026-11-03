---
doc-class: deep-dive
subsystem: northstar-sqlpragmas
implementation-status: proposed-refinement
evidence-status: source-only
last-verified: 2026-11-02
verified-against: 1a9a5cf393e283ca0de4d1613326bc3a8c0ce3b5
verification-method: static-source-inspection
supersedes: []
---

# Preserve Northstar SQLite policy on every pooled connection

## Decision and evidence boundary

Make Northstar's database policy a property of **each physical connection before that connection is admitted to use**, including connections opened after startup. Keep schema-specific foreign-key enforcement separate from optional performance tuning. A successful constructor-level PRAGMA call is not a lifetime guarantee for a `*sql.DB` pool.

At the pinned revision, `northstar.NewStore` opens `northstar_knowledge.db` using the plain `sqlite3` path, calls the Hot profile once through the pool, then calls `EnableForeignKeys` once. It neither caps the pool nor installs a connection-birth hook. Ordinary concurrent readers can therefore cause later physical connections to open without Northstar's explicit FK opt-in or Hot-profile application. A subsequent transaction can use one of those connections. The Store's write mutex and SQL transactions preserve other important properties but do not transfer connection-local settings. [Constructor][store-open] [Shared helpers][pragmas] [Go connection model][go-db]

This is **known infrastructure debt with a concrete production reachability refinement**, not a newly discovered SQLite rule. The sqlpragmas TODO already states that Northstar's enforcement remains per-connection on an unpinned pool. It also defers connector adoption on the premise that existing pools are effectively single-connection. The production observer-to-document-read route below does not establish that premise for Northstar. [Existing acknowledgement and adoption premise][sql-todo] [Implemented-spec premise][sql-implemented]

The bounded conclusions are:

- **Established from source:** Northstar does not apply its explicit FK request or Hot profile at every physical connection's creation. Shared background handlers have a route to overlapping Store reads. The existing generic connector applies the profile but deliberately does not enable foreign keys.
- **Conditional on connection creation and the build:** an additional connection can have FK enforcement off. The pinned mattn driver leaves FK at the SQLite build's default unless the DSN opts in; ordinary defaults and builds with `sqlite_foreign_keys` differ. Do not claim every later connection always has FKs off.
- **Conditional on an invalid write using that connection:** a nonempty missing `RelatedCheck` can be stored by `RecordDriftEvent` without its declared reference being enforced. This review did not observe such a row or show that the normal Guardian producer supplies that invalid reference.
- **Not established:** production pool-growth frequency, orphaned production data, a bad alignment decision, a permission bypass, lost WAL mode after ordinary pool growth, worse synchronous durability, measured latency, memory consumption, or any performance improvement from a proposed repair.

All codeNERD implementation links pin public repository `theRebelliousNerd/codenerd` at **1a9a5cf393e283ca0de4d1613326bc3a8c0ce3b5**. Its `go.mod` pins `github.com/mattn/go-sqlite3 v1.14.52`; driver links below use that tag's commit **b0be46fa28d17ee0b65c79774ac0dad84b6db068**. Source and existing test definitions were read; **no repository code, tests, build, live model, benchmark, database mutation, or fault injection was run**. The witness and acceptance plan are unrun. Only this standalone audit and external source-reading scratch copies were written; no repository files were edited. [Dependency pin][go-mod]

## Architectural fit and existing ownership

Repository guidance requires deterministic executive control, production-path evidence, and explicit separation between intended capability and verified implementation. This proposal belongs in the Go driver/storage boundary: it does not add model discretion, a prompt, a new Mangle policy controller, or a wider permission. The leaf package's existing separation from `internal/store` should remain. [Repository guidance][guidance] [Architecture documentation rules][architecture-guidance] [Northstar guidance][northstar-guidance] [Leaf package contract][pragmas]

The target is not “all PRAGMAs must be globally strict.” The current package intentionally tolerates tuning failures and intentionally omits FKs from generic profiles because other schemas have compatibility concerns. Northstar separately opts into enforcement but currently warns and continues if the one-shot helper fails. Those are distinct policies. A strict Northstar connection-admission requirement would be a deliberate refinement of that warning-only behavior, not a description of today's constructor contract. [Northstar warning policy][store-open] [FK opt-in contract][fk-helper] [Connector failure policy][connector]

The pinned audit inventory, queue, relevant Northstar reports, and sqlpragmas corpus were screened with these boundaries:

| Existing work | What it already owns | This document's additional scope |
|---|---|---|
| sqlpragmas TODO and implemented specification | One-shot application hole, a generic per-connection connector, explicit acknowledgement of Northstar's unpinned FK setting | Trace a real Northstar concurrent-read route; distinguish per-connection FK admission from generic profile adoption; include replacement and honest degradation semantics. |
| sqlpragmas feature tests | Generic forced pool-growth cache-size control and optional FK helper behavior | Specify constructor-level Northstar witnesses and lifecycle coverage; do not describe these existing tests as newly proposed discoveries. |
| `northstar-lossless-strategic-editing` | Canonical vision preservation, base-bound updates, admitted editing and publication | No wizard conversion, stale edit, canonical JSON replacement, or model-generated requirement is needed here. Its as-built protections are not re-reported as absent. |
| `guardian-module-alignment-scope` | Typed alignment scope, coherent module evidence and coverage persistence | Current `alignment_coverage` is an additional declared FK consumer, not evidence that the earlier scope transport defect still exists. |
| `projectdoc-logical-root-identity` | Correct logical identity of the supported fallback project document | Here the selected Store path and vision may be correct; physical connection policy still lacks continuity. |

[Existing pool tests][pool-tests] [FK tests][fk-tests] [Strategic editing audit][related-edit] [Scope audit as built][related-scope] [Root identity audit][related-root] [Audit queue][queue]

This is a bounded nonduplication review, not a claim that every branch, issue, or historical report was searched. The July-era architecture prose is design context, not runtime proof; current helper and caller source override its stale claims that connector/configuration capabilities do not exist. No repo-wide tuning migration is proposed as an incidental fix for this one Store.

## The current constructor and lifetime mismatch

`Store` owns a private `*sql.DB`, a path, and a `sync.RWMutex`. Construction proceeds in this order:

1. Build the database path under the supplied `.nerd` directory and create the directory if necessary.
2. `sql.Open("sqlite3", dbPath)` using no explicit FK DSN option.
3. Apply `ProfileHot` through repeated `db.Exec` calls.
4. Execute `PRAGMA foreign_keys = ON` and read `PRAGMA foreign_keys` through the pooled handle; warn if either the call/read-back fails or the result is not on.
5. Create the schema through `db.Exec`, closing the DB if schema initialization fails.
6. Return the Store. There is no `SetMaxOpenConns`, per-connection connector, or pinned `sql.Conn` in this constructor. [Store opening and schema handoff][store-open]

Before publication, normal sequential construction will ordinarily reuse the initial connection. The finding does **not** require claiming that startup itself necessarily scatters its PRAGMAs across multiple connections or that its successful FK read-back was false. The defect is that its proof has the wrong lifetime and population: it observes startup use of the pool, not every connection the pool may later create.

The Go API distinguishes the pool from a physical connection. `DB` creates and releases connections automatically; a transaction binds to one selected connection. `Conn` explicitly reserves one connection. A pool-level `Exec` followed by a pool-level `QueryRow` does not, in general, reserve one connection across both calls. [Go DB and Conn contracts][go-db]

That distinction yields two separate limits:

- **Local verification limit:** `EnableForeignKeys(*sql.DB)` does not bind its set and read-back to one connection. Invoked on an already active multipool, the two operations may inspect different connections. In Northstar's unpublished startup sequence that race need not occur, but the helper's shape is not a general per-connection verifier.
- **Lifetime limit:** even a correctly configured and correctly verified initial connection does not configure later opens. Calling the helper again merely samples/configures another borrow, unless the implementation establishes connection identity.

SQLite requires FK configuration outside an active transaction; attempting to enable it inside the transaction is not a valid repair. Transaction affinity is useful once a properly admitted connection is selected, not a substitute for that admission. [SQLite FK enablement][sqlite-fk]

## A production path to concurrent Store reads

This route needs healthy chat boot, successful Northstar observer registration and initialization, a defined vision, and overlapping accepted events. It does not require a model to violate a policy or a caller to reach the private DB field.

| Handoff | Current source behavior | Why it matters |
|---|---|---|
| Shared chat boot | Creates `BackgroundObserverManager`, registers `northstar`, acquires the shared Guardian, initializes it, and installs a `BackgroundEventHandler` adapter | The handler is connected to the Store used by the session's Guardian. |
| Registry acquisition | `AcquireGuardian` constructs `NewStore` on first acquisition and reuses the Guardian by cleaned absolute `.nerd` path | One shared Guardian/DB handle is not one physical SQLite connection. |
| Deferred boot | `runBootDeferred` actually calls `observerMgr.Start()` | This is not merely an unused handler construction. |
| Manager lifecycle and dispatch | `Start` launches the event loop; accepted events are processed, and each Northstar dispatch starts a tracked goroutine | Processing the event queue serially does not serialize the work those goroutines perform. |
| Adapter and handler | Adapter calls `BackgroundEventHandler.HandleEvent`; it records an observation before considering automatic alignment triggers | The relevant read is upstream of any eventual alignment model call. |
| Relevance calculation | `recordEventObservation` calls `calculateRelevance`; with a vision this asks `DocumentRelevance` for the ingested-document signal | Concurrent events can enter this read path together. No vision takes the early default and does not prove this particular route. |
| Document retrieval | `DocumentRelevance` calls `ListIngestedDocs`; that method takes `RLock`, queries, scans rows, then releases them | Read locks coexist. If the first query retains a physical connection when the second asks for one, the pool may open another. |

[Boot wiring][chat-boot] [Registry][registry] [Deferred start][chat-start] [Manager start and event loop][manager-start] [Asynchronous dispatch][manager-dispatch] [Adapter][adapter] [Observation entry][observer] [Observation relevance call][observer-relevance] [Guardian relevance][relevance] [Document relevance entry][doc-relevance] [Document query][docs-read]

Go's default pool behavior permits a new connection when all existing ones are in use; a maximum-open limit is an explicit control, not something `RWMutex` provides. Short queries may complete before another begins, so the source route is a **possible schedule**, not evidence of actual pool growth on a particular session. A nonempty ingested corpus makes the overlap witness easier to control, but creating a second connection is governed by overlap, not by whether the corpus is semantically useful. [Go pool management][go-pool]

The Store's write lock does exclude its readers while a Store write runs. That is a useful positive control and changes the correct schedule: first two read holders can cause pool expansion; after both return, a later writer may borrow one of the now-idle new connections. A write need not run simultaneously with those reads to inherit their pool expansion. The mutex serializes application access; it does not label one idle connection as the only permitted writer or replay configuration onto it. [Read locking][docs-read] [Write transaction][drift-write]

Manual `/alignment` and campaign construction also acquire the shared Guardian. They are additional consumers of the same object, not proof that they always overlap or independently create a second connection. The campaign adapter has useful failure handling when Store creation/initialization fails; it must not be weakened to conceal a proposed stricter admission error. [Manual acquisition][manual] [Campaign acquisition][campaign]

## What settings actually differ

The correct comparison is **Northstar's requested policy versus a newly opened connection under the exact driver and build**, not “every PRAGMA resets to SQLite's generic default.” The Hot profile requests WAL, NORMAL synchronous mode, memory temp storage, 10,000 ms busy timeout, host-scaled mmap/cache budgets, and a 10,000-frame auto-checkpoint threshold. FK is separate. [Profile construction][profile] [Host scaling][host]

| Setting | Current Northstar request | Later ordinary open without the helper | Bounded consequence |
|---|---|---|---|
| `foreign_keys` | Explicit ON, one-shot read-back | Driver leaves it at the SQLite build's default when no FK DSN parameter is supplied | FK enforcement may differ; check build flags and read actual connection state. |
| `busy_timeout` | 10,000 ms | Pinned mattn driver initializes 5,000 ms | Policy mismatch is 10 seconds versus 5 seconds, **not 10 seconds versus no wait**. |
| `synchronous` | NORMAL | Pinned mattn driver explicitly initializes NORMAL | No claimed NORMAL-to-FULL or NORMAL-to-OFF regression from this constructor's pool growth. |
| `journal_mode` | WAL attempt on startup | A successfully established WAL mode persists in the database | Do not claim a later connection ordinarily reverts this file to DELETE or loses WAL. |
| `cache_size` | Negative host-scaled Hot value | No `_cache_size` DSN override; not replayed from the earlier connection | Consistent requested cache policy is not established. Actual cache use/performance is unmeasured. |
| `mmap_size`, `temp_store`, `wal_autocheckpoint` | Explicit profile statements | No Northstar profile replay at connection birth | Verify effective settings and platform/build limits rather than assuming identical state or exact allocation. |

[Driver defaults][driver-defaults] [Driver busy initialization][driver-busy] [Driver FK initialization][driver-fk] [Driver synchronous and cache initialization][driver-sync] [SQLite WAL persistence][sqlite-wal] [SQLite cache semantics][sqlite-pragma]

Two SQLite scope facts govern interpretation: FK enforcement is connection-specific and can have a compile-time default; cache-size changes last for the current session and are a suggested limit, not eager allocation. WAL is different: once successfully enabled for the file, it persists across reopen and is shared by its connections. [SQLite FK support][sqlite-fk] [SQLite PRAGMAs][sqlite-pragma] [SQLite WAL persistence][sqlite-wal]

The pinned driver starts `foreignKeys` at `-1`; only an explicit `_foreign_keys`/`_fk` option changes that variable and causes its FK statement to run. The driver's `sqlite_foreign_keys` build-tag file defines `SQLITE_DEFAULT_FOREIGN_KEYS=1`. Thus a default-on build can mask the FK symptom while leaving the application's missing explicit policy continuity unchanged. Root build guidance requests `sqlite_vec`; that is not the same tag as `sqlite_foreign_keys`, and this review did not inspect a deployed binary's compile options. [Driver DSN parsing][driver-fk-parse] [FK compile tag][driver-fk-tag] [Build guidance][guidance]

The repository connector's introductory “no WAL” characterization of later opens is too broad for this file-backed case. Its core birth-hook mechanism is useful; that comment is not a substitute for SQLite's persistence contract. Likewise, a copied assertion that the driver defaults to synchronous FULL would be wrong for v1.14.52. [Connector source][connector] [Pinned synchronous implementation][driver-sync]

The busy-timeout difference does not by itself prove failures or an exact five-second difference in user latency. A busy handler governs eligible lock waits, not every possible contention outcome. Workload, cancellation, lock topology, external writers and whether a lock becomes available all matter. The fair claim is heterogeneous configured policy, with contention consequences left to measurement. [SQLite busy-timeout API][sqlite-busy]

## The integrity consequence and its necessary conditions

The schema declares two relevant relationships:

- `drift_events.related_check` references `alignment_checks(id)`.
- `alignment_coverage.check_id` references `alignment_checks(id)`.

These declarations remain in the schema regardless of whether a particular connection enforces them. [Schema][schema]

`RecordDriftEvent` accepts a caller-provided `DriftEvent`, generates missing ID/time, serializes evidence, converts an empty `RelatedCheck` to SQL NULL, starts a transaction, inserts the row, increments `guardian_state.active_drift_count`, and commits. It does not separately look up a nonempty `RelatedCheck`. A failed insert returns before the counter update and the deferred rollback keeps the transaction from committing. [Drift storage][drift-write]

A missing-reference insertion is therefore a conditional source witness requiring **all** of the following:

1. A connection eligible for that transaction has FK enforcement off, whether because it was opened later without the opt-in or because enforcement was unavailable and the current constructor explicitly continued.
2. A caller supplies a nonempty `RelatedCheck` that does not exist in `alignment_checks`.
3. `db.Begin` selects that unenforced connection.
4. No independent constraint, serialization, SQL, commit or I/O failure prevents the transaction from committing.

Under that schedule the insert and count update may commit together despite the missing parent. On a properly enforced connection the same missing-parent input is rejected and the count remains unchanged. That is a lost storage invariant, **not** a claim that a transaction half-commits or that the counter becomes inconsistent with the number of stored active rows.

The ordinary Guardian path is a strong mitigating control: `persistAlignmentOutcome` first calls `RecordAlignmentCheck` and returns immediately if it fails; only failed/blocked results produce a drift with `RelatedCheck = check.ID`. `RecordAlignmentCheck` inserts any coverage with the parent check in the same transaction. These routes do not naturally manufacture the missing-parent input needed for the witness. FK-on remains worthwhile protection against future callers, mistakes and invalid state, but the absence of that guard must not be inflated into an observed orphan incident. [Normal parent-before-child route][normal-write] [Atomic check and coverage][check-write]

An empty `RelatedCheck` is intentionally stored as NULL and is valid even under FK enforcement. The existing drift tests commonly exercise that case. Rejecting every empty reference would change the API's supported semantics and is not the repair proposed here. [NULL handling][drift-write] [Existing drift tests][drift-tests]

Enabling FKs also does not retroactively repair historical data. Any migration decision needs a separate inventory of violations, preserve/report behavior, and explicit repair authority; it must not silently delete evidence or relabel unknown history as valid. `PRAGMA foreign_key_check` is the relevant diagnostic, not an assertion that startup ON proves old rows clean. [SQLite integrity diagnostic][sqlite-pragma]

## What the existing helpers and tests do prove

### Generic connector support exists but does not complete FK admission

`OpenWithPragmas` gets the registered driver, constructs a connector, and returns `sql.OpenDB`. `pragmaConnector.Connect` opens one physical connection and runs `applyToDriverConn` before returning it to the pool. This is the right lifecycle boundary for profile application and survives later pool growth/replacement. [Connector lifecycle][connector]

Three qualifications prevent a one-line “use the existing helper” recommendation:

1. `pragmasForHost` intentionally excludes `foreign_keys`. Adopting `OpenWithPragmas` alone fixes the **place where profile statements are attempted**, not Northstar's FK opt-in. Adding the existing one-shot FK helper afterward still leaves that integrity policy outside the birth hook.
2. `applyToDriverConn` logs/counts individual errors and continues. Attempted tuning is not verified tuning, and a required integrity assertion should not silently inherit optional performance settings' error policy.
3. The helper resolves the active host class when it applies the profile. If host-class configuration changes during a pool's lifetime, old and new connections can intentionally receive different budgets. A connection-policy contract should say whether it snapshots a policy at Store creation or supports a controlled revision; an ordinary consistency test must hold this input fixed. [Profile exclusion][profile] [Connector soft failure][connector] [Host-class resolution][host]

The comment suggesting `SetMaxOpenConns(1)` is adequate describes a useful growth restriction, not a complete reinitialization protocol. Even a maximum of one does not mean the same physical connection must live forever. A replacement still comes through the driver's open path. Conversely, `SetMaxIdleConns(1)` alone only constrains retained idle connections and is not a cap on concurrent opens. [Helper pool guidance][pragmas] [Go lifetime controls][go-pool]

### Existing tests are meaningful but narrower than this Store guarantee

- The FK helper test verifies a constraint in a helper fixture. Its `openFreshTempDB` fixture pins the pool to one open/idle connection. It does not establish a growing Northstar Store's guarantee.
- The generic growth tests hold four `sql.Conn` leases simultaneously, show that the connector gives each the requested cache size, and deliberately expect plain apply-once to leave later connections untuned. That is already good mechanism coverage, not a missing test to rediscover.
- The default-profile tests deliberately assert FK remains off for their ordinary fixture/build, reflecting compatibility policy. Do not globally switch FK on and merely rewrite those expectations.
- Northstar's inspected Store tests cover ordinary creation, round trips, empty/nonempty histories and counter updates. Parallel test functions generally create separate Stores; `t.Parallel` across independent temp databases is not evidence that a single Store grew its pool.
- The open-site inventory recognizes a pragma marker anywhere in an enclosing function. It can legitimately classify this constructor as “applies pragmas” while proving neither per-connection continuity nor FK enforcement. Its source-presence check is useful but is not a behavioral connection-policy oracle.

[FK fixture and assertions][fk-tests] [Pinned helper fixture][pragma-fixture] [Generic growth controls][pool-tests] [Northstar Store tests][store-tests] [Inventory marker rules][open-audit]

No test above was executed during this review. Their definitions establish intended scope, not a new pass receipt.

## Proposed connection policy contract

The names below are design requirements, not implemented APIs or new configuration flags.

**C1 — Schema integrity belongs to the Store.** Northstar explicitly declares whether it requires FK enforcement. Generic Hot/Query/Bulk/ReadOnly profiles continue to exclude schema-wide integrity decisions. Other SQLite schemas are not opted in as collateral work.

**C2 — Every physical connection is admitted independently.** Initial open, pool expansion, idle/lifetime replacement and recovery after a discarded connection all run the same Northstar policy before queries or transactions can borrow the connection. A successful startup sample is insufficient.

**C3 — Required state is verified on the connection that will be used.** Apply FK ON outside a transaction and read back on that same physical connection. A rejection, missing result, off value or unavailable FK support is not reported as enforced. The admission code must not hand that connection to ordinary FK-dependent work in an enforced mode.

**C4 — Optional tuning has explicit outcome semantics.** Preserve the existing best-effort profile where appropriate, but distinguish requested, attempted, accepted/effective and unavailable settings. Do not report a successful hook invocation as proof that every setting took effect. Verify selected effective values needed by the chosen readiness contract; tolerate documented mmap/platform limits without inventing exact equivalence.

**C5 — Availability policy is explicit.** Preferred target: inability to establish required Northstar FKs returns an actionable Store/connection error before use. This strengthens current warning-only behavior and needs an accepted product decision. If compatibility requires continued unenforced operation, it must be an explicitly named degraded state with truthful capability/diagnostic reporting, not an ordinary success mislabeled “FK enabled.” Do not silently broaden a campaign gate to turn an initialization error into a pass.

**C6 — Pool size and policy continuity are separate controls.** Choose a finite pool budget using measured workload/resource needs. A one-connection policy can be valid, but it still needs connection-birth initialization and replacement coverage. Audit reserved-connection/transaction nesting before imposing a cap, so a correctness fix does not create self-deadlocks.

**C7 — Existing data and policy versions remain distinguishable.** Record enough non-sensitive context to identify driver/version/build, resolved profile and host class, pool policy, and enforcement outcome. Decide whether host class is captured or revised. Legacy FK violations are diagnostic findings requiring an explicit migration decision, not silently “fixed” during a tuning refactor.

**C8 — Preserve data semantics and authority boundaries.** Keep valid parent/child inserts, NULL related checks, transactions, counter atomicity, scope coverage, canonical vision editing, shutdown and Guardian sharing intact. No new tool permission, alignment verdict, model capability, or Mangle safety policy follows from a storage configuration change.

These contracts refine the Store's lifecycle while preserving the package's deliberate separation between tuning and schema policy. They do not require a new general callback registry or a repository-wide database abstraction.

## Alternatives and their costs

| Alternative | What it solves | Residual and cost |
|---|---|---|
| **Northstar-scoped per-connection admission plus existing profile connector** | Reuses the established birth boundary; applies and verifies FK on each connection while retaining generic profile policy | Requires a narrow way to express Northstar's required setting and error policy. Must avoid double-opening or handing out a failed connection; test driver interfaces and cleanup. Preferred design direction. |
| **Driver DSN FK opt-in plus profile connector** | Pinned mattn's `_foreign_keys=on` is parsed/applied during each driver open, including replacements; connector handles other profile statements | Correct DSN construction/escaping and exact driver semantics matter. Driver executes the FK PRAGMA but does not itself read it back here, so unsupported/no-op builds still need same-connection verification for a strict guarantee. Do not expose arbitrary user-supplied PRAGMAs. |
| **One-connection pool plus connection-birth policy** | Bounds connection count and simplifies scheduling while preserving replacement initialization | Serializes reads, potentially raising queue latency. Need cancellation and nesting checks, and a deliberate decision whether lost parallelism is acceptable. Measure against the current supported workload. |
| **One-connection cap only** | Prevents ordinary concurrent pool expansion while that initial connection remains alive | Does not establish replacement initialization. Useful mitigation but not full C2/C3 acceptance. Never call it a lifetime fix. |
| **Explicitly leased connection per operation, configured and verified before beginning its transaction** | Establishes local connection identity and can enforce policy at use | Touches every operation, increases repeated work and leak/nesting risk, and lets an omitted route escape. A centralized birth boundary is easier to audit. |
| **Compile every binary with default FK ON** | Masks the missing explicit opt-in for those builds | Changes other schemas' compatibility policy, depends on build provenance, does not fix Hot-profile continuity, and is not a Northstar-owned solution. |
| **Run pooled PRAGMAs before each write or put FK ON inside the transaction** | Appears small | Pooled setup can target another connection; transaction-time FK changes are ineffective. Reject as a claimed guarantee. |

[Driver open options and hook][driver-fk-parse] [Driver verification gap][driver-fk] [Driver hook failure handling][driver-hook] [Existing connector][connector]

A Northstar-specific opt-in need not make all tuning failures fatal. Conversely, avoiding a global API migration is not a reason to make a required FK verification failure indistinguishable from a tolerated mmap limitation. The exact public helper shape should be chosen during implementation after checking current callers; this document proposes behavior, not a pre-approved patch.

## Performance and resource evaluation without invented gains

The primary benefit is deterministic policy continuity. Faster execution is a hypothesis, not a result. Apply these comparisons fairly:

- **Initialization cost:** per-connection PRAGMA application and read-back add work when a physical connection opens, not necessarily on every query. Record initial startup and replacement costs separately from warm steady-state operation. Count actual opens, retries and rejected connections.
- **Capacity cost:** Hot requests a workstation cache ceiling of 2 GiB and mmap window of 8 GiB per applicable connection; Laptop and Micro scale those budgets by four and sixteen. Applying a previously missed profile to every connection can increase aggregate potential resource use. These are ceilings/requests, not evidence of allocation or RSS. Do not multiply them into an asserted observed memory bill. [Profile sizes][profile] [Host classes][host]
- **Pool-cap tradeoff:** a cap can reduce aggregate connection/cache overhead and writer contention while increasing read queueing. Current Store writes already hold the write lock, so increasing a pool does not make those writes concurrently execute through the same Store. Compare relevant read-heavy and mixed workloads, not a synthetic workload that bypasses its locks.
- **Contention tradeoff:** making the 10-second policy uniform can avoid some earlier lock failures and can also make unsuccessful waits longer. Never report a longer configured timeout as inherently faster or safer. Keep effective timeouts equal when comparing two implementation mechanisms; evaluate timeout policy changes as a separate treatment.
- **Integrity overhead:** enforcing an FK entails checking a parent relationship. Keep FK enforcement equal across candidate performance runs; a configuration that “wins” by omitting the integrity guarantee is not an equivalent treatment.
- **WAL and checkpoint confounds:** keep journal mode, checkpoint policy, dataset, storage, process topology and warm/cold cache state explicit. Pool continuity should not get credit for WAL that was already persistent or NORMAL synchronous mode already established by the driver.

A later benchmark should record operation mix, dataset size, ingested-document count/bytes, offered load, actual pool size, admitted connection generations, host class, driver/build, FK state, busy errors, cancellations, throughput, latency distributions and resident memory. Report all attempted operations and errors; do not drop blocked or rejected cases from the denominator. No numeric improvement threshold is manufactured here.

## Unrun witness and acceptance plan

### First establish the constructor boundary deterministically

Use a disposable **file-backed** Store under a temporary `.nerd` directory with the pinned driver and declared build flags. Do not use bare `:memory:`: distinct SQLite connections can otherwise refer to distinct databases, confounding policy with schema visibility.

A package-scoped regression can reserve the constructor's initially configured connection with `db.Conn`, keep that lease open, and then obtain another connection. Holding both leases forces distinct physical connections instead of repeatedly sampling one idle connection. Read `foreign_keys`, `busy_timeout`, `cache_size` and the bounded control settings on each reserved connection. First confirm actual compile behavior; use an FK-default-off supported build for the negative witness and a default-on build as a masking control.

Expected source prediction for a healthy default-off baseline:

| Observation | Initial configured connection | Additional connection |
|---|---|---|
| Explicit FK enforcement | ON after successful startup | Build default, ordinarily OFF in this treatment |
| Busy timeout | 10,000 ms | 5,000 ms under pinned mattn defaults |
| Cache-size request | Resolved Hot budget | Not configured by Northstar's profile |
| Journal mode after successful WAL setup | WAL | WAL, a positive persistence control |
| Synchronous mode | NORMAL | NORMAL, a positive driver-default control |

This is a source prediction, not recorded output. Assert the exact FK value only after establishing the binary's default; do not turn a default-on build into a spurious failure of the research claim.

### Exercise Store semantics rather than only raw SQL

On a fresh disposable Store, reserve the initial connection without taking the Store mutex, then call `RecordDriftEvent` with a unique nonempty nonexistent parent. That public method must acquire another pool connection for its transaction. Compare error/result, persisted drift row and active count. With current ordinary default-off driver behavior the invalid pair is predicted to commit; with correct per-connection enforcement the call must fail and neither row nor count change. Use cleanup that releases the reserved lease even on failure.

This deliberately controlled package-level witness is **not** a claim that production reserves the private connection this way. It isolates the consequence once the production-permitted pool expansion occurs. Also cover a valid committed parent and an empty reference; both must remain accepted, and successful resolution must preserve the existing count behavior.

Do not use one arbitrary `db.QueryRow("PRAGMA foreign_keys")` afterward as proof about the connection that performed the write. Observe both lease identity and semantic effect. After release, ensure the caller can continue and close cleanly; a “fixed” missing-parent rejection obtained by hanging forever on a one-connection cap is not a pass.

### Complete the lifecycle and production matrix

| Case | Required evidence for a repair | Purpose |
|---|---|---|
| Initial Store open | Required state verified on the connection before normal use; healthy schema/vision round trip | Preserve normal startup. |
| Forced pool growth | Every held connection satisfies the selected required policy and expected supported tuning | Close population gap. |
| Connection replacement | Intentionally retire an idle connection through supported pool controls or an isolated driver seam; the new generation is initialized before use | Defeat cap-only fixes. Do not claim production config already sets an idle/lifetime timeout. |
| FK build unavailable or ON ineffective | No normal enforced-mode admission; precise error/degraded capability according to the accepted product policy | Distinguish a no-op PRAGMA from success. |
| Optional tuning rejected | Useful warning/metrics, no falsely “verified” state, and availability behavior matches the optional-setting contract | Preserve intentional best effort. |
| FK-default-on build | Same explicit Northstar policy; no dependence on off-by-default assumptions | Avoid accidental build dependence. |
| Parent-first Guardian outcome | Failed/blocked checks persist parent and then valid drift; successful checks do not create drift; coverage remains atomic | Preserve existing normal controls. |
| NULL reference | Unrelated drift without a parent remains valid | Avoid inventing a mandatory relationship. |
| Existing invalid historical row | Diagnostic reports it without destructive repair or invented validity | Separate readiness from migration. |
| Host-class stability and change | Same resolved input yields expected settings; deliberate changes follow documented snapshot/revision semantics | Avoid unexplained mixed budgets. |
| Real observer route | Shared chat boot, start, adapter and accepted events reach overlapping document reads against one Store; record physical opens and outcomes | Establish production reachability, not helper-only correctness. |
| No vision or observer disabled | Expected no-document-relevance/default path; no invented pool-growth claim | Negative control for the production witness. |
| Cancellation and shutdown | Held queries/leases release; observers stop; Store close completes; no goroutine/connection leak | Avoid trading configuration loss for lifecycle failure. |
| Normal workload comparison | Equal data, build, FK, timeout/profile, event load and admission rules; errors included | Bound performance claims. |

For production-route testing, first use deterministic synchronization or an isolated driver seam to hold the first document query while another accepted event reaches the same Store. The synchronization must not bypass the Store lock or replace the production constructor. Observe that there are two live physical connections, not merely two goroutines or two `*sql.DB` pointers. Then repeat without artificial delay for workload characterization; inability to observe natural overlap does not invalidate the forced lifecycle contract, and forced overlap does not estimate natural frequency.

A later implementation task should run focused sqlpragmas/Northstar regressions and relevant chat/observer integration checks, followed by the repository's required aggregate checks in its supported `sqlite_vec`/CGO environment. This docs-only review did none of them. Existing test names, proposed assertions and expected schedules are not acceptance receipts.

## Documentation and closure criteria

Before implementation, refine the Northstar and sqlpragmas specifications with C1–C8, the chosen availability policy, pool budget decision, and a checkable gap exit. Preserve the leaf's existing compatibility decision and generic best-effort tuning contract. Update the claim that adoption is safe to defer because this Store is effectively single-connection; either enforce that capacity contract with lifecycle-safe configuration or document and test the multipool route.

An implementation can close this gap only when it shows, for the actual `NewStore` route, that **every admitted physical connection has the required Northstar integrity state**, replacement cannot bypass it, supported valid/NULL writes retain their semantics, and enforcement failures have the accepted explicit outcome. Generic connector existence, an AST pragma marker, a single successful read-back, parallel tests over unrelated databases, or an uneventful sequential session is insufficient.

The useful result of this review is the narrower decision: preserve policy at the physical connection boundary without exaggerating the effect. There is a real continuity hole and a source-supported route to it; there is no observed orphan, demonstrated runtime incident, or measured performance result in this document.

## Source references

All repository source links below pin the reviewed codeNERD commit; driver links pin the commit behind v1.14.52. Official SQLite and Go documentation was consulted on November 2, 2026.

[store-open]: https://github.com/theRebelliousNerd/codenerd/blob/1a9a5cf393e283ca0de4d1613326bc3a8c0ce3b5/internal/northstar/store.go#L31-L85
[pragmas]: https://github.com/theRebelliousNerd/codenerd/blob/1a9a5cf393e283ca0de4d1613326bc3a8c0ce3b5/internal/sqlpragmas/pragmas.go#L1-L38
[fk-helper]: https://github.com/theRebelliousNerd/codenerd/blob/1a9a5cf393e283ca0de4d1613326bc3a8c0ce3b5/internal/sqlpragmas/pragmas.go#L133-L183
[profile]: https://github.com/theRebelliousNerd/codenerd/blob/1a9a5cf393e283ca0de4d1613326bc3a8c0ce3b5/internal/sqlpragmas/pragmas.go#L94-L236
[host]: https://github.com/theRebelliousNerd/codenerd/blob/1a9a5cf393e283ca0de4d1613326bc3a8c0ce3b5/internal/sqlpragmas/hostclass.go#L17-L131
[connector]: https://github.com/theRebelliousNerd/codenerd/blob/1a9a5cf393e283ca0de4d1613326bc3a8c0ce3b5/internal/sqlpragmas/connector.go#L13-L143
[sql-todo]: https://github.com/theRebelliousNerd/codenerd/blob/1a9a5cf393e283ca0de4d1613326bc3a8c0ce3b5/Docs/architecture/sqlpragmas/TODO.md#L109-L129
[sql-implemented]: https://github.com/theRebelliousNerd/codenerd/blob/1a9a5cf393e283ca0de4d1613326bc3a8c0ce3b5/Docs/architecture/sqlpragmas/IMPLEMENTED_SPEC.md#L417-L435
[go-mod]: https://github.com/theRebelliousNerd/codenerd/blob/1a9a5cf393e283ca0de4d1613326bc3a8c0ce3b5/go.mod#L1-L24
[guidance]: https://github.com/theRebelliousNerd/codenerd/blob/1a9a5cf393e283ca0de4d1613326bc3a8c0ce3b5/agents.md
[architecture-guidance]: https://github.com/theRebelliousNerd/codenerd/blob/1a9a5cf393e283ca0de4d1613326bc3a8c0ce3b5/Docs/architecture/AGENTS.md
[northstar-guidance]: https://github.com/theRebelliousNerd/codenerd/blob/1a9a5cf393e283ca0de4d1613326bc3a8c0ce3b5/internal/northstar/agents.md
[related-edit]: https://github.com/theRebelliousNerd/codenerd/blob/1a9a5cf393e283ca0de4d1613326bc3a8c0ce3b5/Docs/audits/northstar-lossless-strategic-editing.md#L156-L200
[related-scope]: https://github.com/theRebelliousNerd/codenerd/blob/1a9a5cf393e283ca0de4d1613326bc3a8c0ce3b5/Docs/audits/guardian-module-alignment-scope.md#L138-L181
[related-root]: https://github.com/theRebelliousNerd/codenerd/blob/1a9a5cf393e283ca0de4d1613326bc3a8c0ce3b5/Docs/audits/projectdoc-logical-root-identity.md
[queue]: https://github.com/theRebelliousNerd/codenerd/blob/1a9a5cf393e283ca0de4d1613326bc3a8c0ce3b5/Docs/audits/ASTRA_QUEUE.md
[chat-boot]: https://github.com/theRebelliousNerd/codenerd/blob/1a9a5cf393e283ca0de4d1613326bc3a8c0ce3b5/cmd/nerd/chat/session_shared_boot.go#L270-L282
[registry]: https://github.com/theRebelliousNerd/codenerd/blob/1a9a5cf393e283ca0de4d1613326bc3a8c0ce3b5/internal/northstar/registry.go#L46-L115
[chat-start]: https://github.com/theRebelliousNerd/codenerd/blob/1a9a5cf393e283ca0de4d1613326bc3a8c0ce3b5/cmd/nerd/chat/model_update.go#L892-L914
[manager-start]: https://github.com/theRebelliousNerd/codenerd/blob/1a9a5cf393e283ca0de4d1613326bc3a8c0ce3b5/internal/shards/observer_manager.go#L168-L193
[manager-dispatch]: https://github.com/theRebelliousNerd/codenerd/blob/1a9a5cf393e283ca0de4d1613326bc3a8c0ce3b5/internal/shards/observer_manager.go#L369-L459
[adapter]: https://github.com/theRebelliousNerd/codenerd/blob/1a9a5cf393e283ca0de4d1613326bc3a8c0ce3b5/cmd/nerd/chat/session_boot_helpers.go#L47-L72
[observer]: https://github.com/theRebelliousNerd/codenerd/blob/1a9a5cf393e283ca0de4d1613326bc3a8c0ce3b5/internal/northstar/observer.go#L303-L311
[relevance]: https://github.com/theRebelliousNerd/codenerd/blob/1a9a5cf393e283ca0de4d1613326bc3a8c0ce3b5/internal/northstar/guardian.go#L633-L675
[docs-read]: https://github.com/theRebelliousNerd/codenerd/blob/1a9a5cf393e283ca0de4d1613326bc3a8c0ce3b5/internal/northstar/docs.go#L87-L124
[manual]: https://github.com/theRebelliousNerd/codenerd/blob/1a9a5cf393e283ca0de4d1613326bc3a8c0ce3b5/cmd/nerd/chat/model_helpers.go#L391-L441
[campaign]: https://github.com/theRebelliousNerd/codenerd/blob/1a9a5cf393e283ca0de4d1613326bc3a8c0ce3b5/internal/northstar/campaign_observer.go#L44-L78
[schema]: https://github.com/theRebelliousNerd/codenerd/blob/1a9a5cf393e283ca0de4d1613326bc3a8c0ce3b5/internal/northstar/store.go#L126-L179
[drift-write]: https://github.com/theRebelliousNerd/codenerd/blob/1a9a5cf393e283ca0de4d1613326bc3a8c0ce3b5/internal/northstar/store.go#L616-L668
[check-write]: https://github.com/theRebelliousNerd/codenerd/blob/1a9a5cf393e283ca0de4d1613326bc3a8c0ce3b5/internal/northstar/store.go#L482-L560
[normal-write]: https://github.com/theRebelliousNerd/codenerd/blob/1a9a5cf393e283ca0de4d1613326bc3a8c0ce3b5/internal/northstar/guardian.go#L814-L843
[store-tests]: https://github.com/theRebelliousNerd/codenerd/blob/1a9a5cf393e283ca0de4d1613326bc3a8c0ce3b5/internal/northstar/store_test.go
[drift-tests]: https://github.com/theRebelliousNerd/codenerd/blob/1a9a5cf393e283ca0de4d1613326bc3a8c0ce3b5/internal/northstar/store_test.go#L417-L439
[fk-tests]: https://github.com/theRebelliousNerd/codenerd/blob/1a9a5cf393e283ca0de4d1613326bc3a8c0ce3b5/internal/sqlpragmas/features_test.go#L65-L109
[pragma-fixture]: https://github.com/theRebelliousNerd/codenerd/blob/1a9a5cf393e283ca0de4d1613326bc3a8c0ce3b5/internal/sqlpragmas/pragmas_test.go#L14-L29
[pool-tests]: https://github.com/theRebelliousNerd/codenerd/blob/1a9a5cf393e283ca0de4d1613326bc3a8c0ce3b5/internal/sqlpragmas/features_test.go#L185-L257
[open-audit]: https://github.com/theRebelliousNerd/codenerd/blob/1a9a5cf393e283ca0de4d1613326bc3a8c0ce3b5/internal/sqlpragmas/open_site_audit_test.go#L259-L281
[driver-defaults]: https://github.com/mattn/go-sqlite3/blob/b0be46fa28d17ee0b65c79774ac0dad84b6db068/sqlite3.go#L1229-L1244
[driver-busy]: https://github.com/mattn/go-sqlite3/blob/b0be46fa28d17ee0b65c79774ac0dad84b6db068/sqlite3.go#L1664-L1667
[driver-fk-parse]: https://github.com/mattn/go-sqlite3/blob/b0be46fa28d17ee0b65c79774ac0dad84b6db068/sqlite3.go#L1397-L1417
[driver-fk]: https://github.com/mattn/go-sqlite3/blob/b0be46fa28d17ee0b65c79774ac0dad84b6db068/sqlite3.go#L1850-L1855
[driver-sync]: https://github.com/mattn/go-sqlite3/blob/b0be46fa28d17ee0b65c79774ac0dad84b6db068/sqlite3.go#L1903-L1922
[driver-hook]: https://github.com/mattn/go-sqlite3/blob/b0be46fa28d17ee0b65c79774ac0dad84b6db068/sqlite3.go#L1924-L1936
[driver-fk-tag]: https://github.com/mattn/go-sqlite3/blob/b0be46fa28d17ee0b65c79774ac0dad84b6db068/sqlite3_opt_foreign_keys.go#L7-L15
[sqlite-fk]: https://www.sqlite.org/foreignkeys.html#fk_enable
[sqlite-pragma]: https://www.sqlite.org/pragma.html
[sqlite-wal]: https://www.sqlite.org/wal.html#persistence_of_wal_mode
[sqlite-busy]: https://www.sqlite.org/c3ref/busy_timeout.html
[go-db]: https://pkg.go.dev/database/sql@go1.27.3#DB
[go-pool]: https://go.dev/doc/database/manage-connections

[observer-relevance]: https://github.com/theRebelliousNerd/codenerd/blob/1a9a5cf393e283ca0de4d1613326bc3a8c0ce3b5/internal/northstar/observer.go#L438-L452
[doc-relevance]: https://github.com/theRebelliousNerd/codenerd/blob/1a9a5cf393e283ca0de4d1613326bc3a8c0ce3b5/internal/northstar/docs.go#L214-L228
