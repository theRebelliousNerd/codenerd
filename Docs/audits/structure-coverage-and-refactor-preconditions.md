---
doc-class: deep-dive
subsystem: world-codedom
implementation-status: partial
last-verified: 2026-10-28
verified-against: 24eb5faaa65a4f55cdc6061e809b09fc25d8376a
verification-method: static-source-inspection
supersedes: []
---

# CodeDOM: make structural absence and refactoring preconditions trustworthy

**Decision:** preserve the structural tools and their incremental parsing, but make their answers carry an explicit observation domain, coverage state, and generation. An empty answer must not become permission to delete a declaration when the index could not inspect its users. A syntactically valid rewrite must not consume offsets measured from different bytes.

**Status:** source-grounded findings and proposed design. No implementation, tests, inference, benchmarks, or failure injection were executed for this document. Conditional failure sequences below are proposed witnesses, not reproduced incidents. Source claims refer to the pinned revision above, not an assertion about later main.

## 1. The capability this serves

CodeNERD should answer where behavior is implemented, what invokes it, and which changes preserve or intentionally redirect its integration paths. Structural context is valuable because it can replace repeated search-and-read turns with a compact, addressable answer. That benefit depends on knowing what the answer establishes. “No caller observed in the indexed domain” and “this feature is unused” are not interchangeable.

The [repository North Star, `agents.md`, “How This Shapes Runtime Changes”](https://github.com/theRebelliousNerd/codenerd/blob/24eb5faaa65a4f55cdc6061e809b09fc25d8376a/agents.md#how-this-shapes-runtime-changes) requires revision-aware world/CodeDOM context and forbids stale evidence surviving a source change as current truth. Its Repo Contract also requires looking for wiring gaps before deleting unused-looking code. The proposal below makes these obligations concrete without replacing the model's creative role or moving executive choices out of Mangle.

This is not another keyword-cache audit. The [retrieval proposal](retrieval-freshness-coverage-and-context-value.md), §3, concerns invalidation of cached search result sets. This document concerns a different chain: filesystem observation → structural index → use-site discovery → destructive edit preconditions. The [tool-effect proposal](tool-effect-receipts-and-validation-completeness.md), §§3–5, concerns accounting after effects and post-validation; here the missing evidence must be resolved **before** a structural edit is allowed to rely on it.

## 2. What already exists and must be preserved

| Surface | Inspected evidence | Current conclusion |
|---|---|---|
| Production registration | `initFinalExecutors`, [`internal/system/factory.go:2432,2563–2566`](https://github.com/theRebelliousNerd/codenerd/blob/24eb5faaa65a4f55cdc6061e809b09fc25d8376a/internal/system/factory.go#L2563-L2566); `wireStructureProvider`, [`internal/system/structure_provider.go:11–20`](https://github.com/theRebelliousNerd/codenerd/blob/24eb5faaa65a4f55cdc6061e809b09fc25d8376a/internal/system/structure_provider.go#L11-L20) | Structural queries are wired into ordinary boot with a workspace. This is not a missing-provider implementation proposal. |
| Shared parsed index | `SharedStructureIndex`, [`internal/world/structure_shared.go:20–35`](https://github.com/theRebelliousNerd/codenerd/blob/24eb5faaa65a4f55cdc6061e809b09fc25d8376a/internal/world/structure_shared.go#L20-L35) | Existing workspace-keyed reuse avoids a new index for each query. Do not replace it with per-call full construction. |
| Content freshness | `StructureIndex.refreshLocked`, [`internal/world/structure_index.go:247–273`](https://github.com/theRebelliousNerd/codenerd/blob/24eb5faaa65a4f55cdc6061e809b09fc25d8376a/internal/world/structure_index.go#L247-L273); `contentStamp.trusts` and `resolveContent`, [`internal/world/content_stamp.go:112–147`](https://github.com/theRebelliousNerd/codenerd/blob/24eb5faaa65a4f55cdc6061e809b09fc25d8376a/internal/world/content_stamp.go#L112-L147) | Uses size, mtime, content-generation information, a trust-age rule, and content hashing when needed. It is **not** merely a size/mtime cache. |
| Same-size rewrite coverage in test source | `TestStructureIndexFollowsSameSizeRewriteWithRestoredMtime` and `TestStructureIndexRefreshesModulePathAfterSameSizeRewriteWithRestoredMtime`, [`internal/world/structure_index_test.go:260,389`](https://github.com/theRebelliousNerd/codenerd/blob/24eb5faaa65a4f55cdc6061e809b09fc25d8376a/internal/world/structure_index_test.go#L260-L454) | Existing regression definitions address this case. Their presence is not a claim this research ran them. The older world gap row describing only size/mtime is stale for this revision. |
| Broken-file visibility | `refreshLocked`, [`internal/world/structure_index.go:310–332`](https://github.com/theRebelliousNerd/codenerd/blob/24eb5faaa65a4f55cdc6061e809b09fc25d8376a/internal/world/structure_index.go#L310-L332); `FileStatus`, [`internal/world/structure_queries.go:576–587`](https://github.com/theRebelliousNerd/codenerd/blob/24eb5faaa65a4f55cdc6061e809b09fc25d8376a/internal/world/structure_queries.go#L576-L587) | Tracks parse errors and last-good declarations for a file that was read but became syntactically broken. Preserve this repair affordance. |
| Match strength and semantic caveats | `StructCallerMatch`, [`internal/world/structure_index.go:98–105`](https://github.com/theRebelliousNerd/codenerd/blob/24eb5faaa65a4f55cdc6061e809b09fc25d8376a/internal/world/structure_index.go#L98-L105); `executeCallersOf`, [`internal/tools/codedom/structure.go:359–387`](https://github.com/theRebelliousNerd/codenerd/blob/24eb5faaa65a4f55cdc6061e809b09fc25d8376a/internal/tools/codedom/structure.go#L359-L387) | Distinguishes exact/package-or-scope matches from by-name candidates. The empty-callers response already warns about function values and reflection. |
| Bounded output without silent row truncation | `renderPage`, [`internal/tools/codedom/structure.go:187–211`](https://github.com/theRebelliousNerd/codenerd/blob/24eb5faaa65a4f55cdc6061e809b09fc25d8376a/internal/tools/codedom/structure.go#L187-L211) | Produces explicit offset continuation and row counts. Do not remove pagination to obtain stronger consistency. |
| Edit protection | `checkRevision`, [`internal/tools/codedom/element_edit.go:174–181`](https://github.com/theRebelliousNerd/codenerd/blob/24eb5faaa65a4f55cdc6061e809b09fc25d8376a/internal/tools/codedom/element_edit.go#L174-L181); `commitFiles`, [`internal/tools/codedom/commit.go:47–106`](https://github.com/theRebelliousNerd/codenerd/blob/24eb5faaa65a4f55cdc6061e809b09fc25d8376a/internal/tools/codedom/commit.go#L47-L106) | Optional element revision checking and optimistic comparison of planned-write bytes already exist. The gap is not “no concurrency protection”; it is the wider structural read set and the earlier offset-producing snapshot. |

The existing source-freshness improvement solves a real problem. It does not, by itself, prove complete enumeration, complete use discovery, or snapshot consistency across multiple reads.

## 3. Finding S1: an incomplete refresh can look like successful disappearance

`StructureIndex.refreshLocked` explicitly ignores `WalkDir` errors, ignores `DirEntry.Info` errors, and later deletes prior entries absent from `seen` ([`internal/world/structure_index.go:188–219,279–288`](https://github.com/theRebelliousNerd/codenerd/blob/24eb5faaa65a4f55cdc6061e809b09fc25d8376a/internal/world/structure_index.go#L188-L288)). Consequently, an unreadable subtree can remove previously indexed declarations and call sites during an otherwise nil-error refresh. Membership exclusions and genuine deletion are legitimate reasons for removal; an IO failure is not equivalent evidence.

The earlier membership refresh can already fail closed, and a failed content-hash read is explicitly returned as an error ([`internal/world/structure_index.go:260–263`](https://github.com/theRebelliousNerd/codenerd/blob/24eb5faaa65a4f55cdc6061e809b09fc25d8376a/internal/world/structure_index.go#L260-L263)). This finding therefore concerns the walk/stat skips and the separate later parse read, not all read failures.

A second observation window follows the hash/stamp pass: `parseStructFile` independently reads bytes and returns nil on read failure; `refreshLocked` deletes that entry when the parse result is nil ([`parseStructFile`, `internal/world/structure_index.go:435–439`](https://github.com/theRebelliousNerd/codenerd/blob/24eb5faaa65a4f55cdc6061e809b09fc25d8376a/internal/world/structure_index.go#L435-L439); [`refreshLocked`, lines 291–308](https://github.com/theRebelliousNerd/codenerd/blob/24eb5faaa65a4f55cdc6061e809b09fc25d8376a/internal/world/structure_index.go#L291-L308)). This differs from syntax failure, for which last-good declarations are retained. There is no typed unreadable-file count in `StructureStats`; its fields are files, symbols, calls, reparsed, broken, and time (`StructureStats`, [`internal/world/structure_index.go:144–161`](https://github.com/theRebelliousNerd/codenerd/blob/24eb5faaa65a4f55cdc6061e809b09fc25d8376a/internal/world/structure_index.go#L144-L161)).

**Proposed witness:** warm an index containing a declaration in one readable directory and its only caller in another. After membership refresh succeeds, inject an enumeration failure during the subsequent structural crawl for the caller directory, leaving the declaration readable. Ask for callers and unreferenced symbols through the registered tool path. The required result is incomplete/unknown evidence, not an authoritative “no call site anywhere.” This witness has not been executed.

The production risk follows the tool contract: `executeCallersOf` renders empty rows with workspace-wide absence wording, while `UnreferencedSymbolsTool` describes its results as real orphans by name ([`internal/tools/codedom/structure.go:359–387,445–462`](https://github.com/theRebelliousNerd/codenerd/blob/24eb5faaa65a4f55cdc6061e809b09fc25d8376a/internal/tools/codedom/structure.go#L359-L462)). Those claims need a completed observation domain, not only an empty list.

**Priority:** correctness first. Stage a new generation and distinguish deliberately excluded, confirmed absent, unreadable, unsupported, broken, and canceled. Never relabel last-good state as current, but do not destroy it because a new observation failed.

## 4. Finding S2: use discovery can silently omit candidates before deletion

`StructureIndex.Uses` refreshes and selects candidate Go files under the index lock, then releases the lock and rereads those files. Read failures and Go parse failures are skipped with `continue`, and the method returns the accumulated uses with nil error ([`internal/world/structure_queries.go:323–390,440–446`](https://github.com/theRebelliousNerd/codenerd/blob/24eb5faaa65a4f55cdc6061e809b09fc25d8376a/internal/world/structure_queries.go#L323-L446)). The `StructureStats` returned describes the earlier index refresh, not the completeness of these later reads.

For Go and Mangle, `usesOutside` obtains the provider's uses and discards its stats string; `executeDeleteElement` proceeds to deletion when that use list is empty and no error occurred ([`internal/tools/codedom/element_edit.go:542–596`](https://github.com/theRebelliousNerd/codenerd/blob/24eb5faaa65a4f55cdc6061e809b09fc25d8376a/internal/tools/codedom/element_edit.go#L542-L596)). Script languages have a separate `scriptUsesOutside` path; this finding is not a claim that the Go-specific read loop implements script deletion.

**Conditional consequence:** if the only remaining Go user is omitted by these failure branches, this particular use-existence guard can allow deletion. Other permission, syntax, compilation, test, or task-completion checks may still reject the operation or its outcome. This document does not assert a universal safety-gate bypass or a completed incorrect production task.

There is a semantic limit even after IO failures are fixed. `Uses` is not a whole-program type-checked reference proof. Its method search is restricted to selected same-package/importer candidates and name/receiver evidence; `bareUses` intentionally includes possible local shadowing ([`internal/world/structure_queries.go:345–370,404–437,449–453`](https://github.com/theRebelliousNerd/codenerd/blob/24eb5faaa65a4f55cdc6061e809b09fc25d8376a/internal/world/structure_queries.go#L345-L453)). An “exact” label expresses this implementation's matching mode, not complete semantic reachability. Dynamic loading, registration by strings, reflection, external consumers, and unsupported languages require separately stated obligations.

**Proposed witness:** after candidate selection, inject a read or parse failure in the only user file. An authorized delete request must return a structured unresolved-reference obligation without changing the declaration. A genuinely absent-user control should still be eligible for the normal policy and edit checks. Add interface/reflection/external-public-API controls so replacing silent IO failure with a complete syntax scan does not accidentally promise whole-program safety.

## 5. Finding S3: optimistic write checks do not validate the original use-site read set

`executeRepoint` first calls `provider.Uses`, then resolves the destination and constructs a plan. `planRepoint` reloads each touched file. `rewriteUses` applies the earlier offsets to that newly loaded source, checking bounds and overlap but not carrying or comparing the use-discovery file revision ([`internal/tools/codedom/repoint.go:65–103,152–218,230–278`](https://github.com/theRebelliousNerd/codenerd/blob/24eb5faaa65a4f55cdc6061e809b09fc25d8376a/internal/tools/codedom/repoint.go#L65-L278)). `StructureUse` carries file, positions, qualifier, ref, and match, but no source revision (`StructureUse`, [`internal/tools/codedom/structure.go:85–97`](https://github.com/theRebelliousNerd/codenerd/blob/24eb5faaa65a4f55cdc6061e809b09fc25d8376a/internal/tools/codedom/structure.go#L85-L97)).

`commitFiles` compares current bytes against the later `plannedWrite.orig` snapshot, which is valuable protection against changes **after planning**. It cannot establish that offsets computed before planning refer to those bytes ([`internal/tools/codedom/commit.go:32–39,47–106`](https://github.com/theRebelliousNerd/codenerd/blob/24eb5faaa65a4f55cdc6061e809b09fc25d8376a/internal/tools/codedom/commit.go#L32-L106)). Nor does checking only planned writes establish that an unmodified file has not acquired a new use since discovery.

**Proposed witness:** produce use offsets, then change the identifier at an affected offset to a different same-length valid identifier before `planRepoint` reloads the file. Bounds can remain valid; a syntactically valid changed result alone is not proof that the intended old reference was rewritten. The corrected operation must detect the revision/token-identity mismatch before writing. A second fixture inserts a new use in a previously nonmatching eligible file between discovery and commit; the reference-set dependency must invalidate or be explicitly outside a narrower approved operation contract.

This is a read-set versus write-set distinction. A repair should extend the existing optimistic mechanism, not discard it or promise filesystem-wide transactions that CodeNERD does not possess.

## 6. Finding S4: offset pagination has no query-generation identity

Each structural query refreshes the index (`FindSymbol`, `Callers`, `Callees`, `Unreferenced` in [`internal/world/structure_index.go:686,839,926,989`](https://github.com/theRebelliousNerd/codenerd/blob/24eb5faaa65a4f55cdc6061e809b09fc25d8376a/internal/world/structure_index.go#L686-L1019)). The tool page interface accepts a numeric offset and rerenders the new result set (`pageArgs`, `renderPage`, [`internal/tools/codedom/structure.go:177–211`](https://github.com/theRebelliousNerd/codenerd/blob/24eb5faaa65a4f55cdc6061e809b09fc25d8376a/internal/tools/codedom/structure.go#L177-L211)).

Rows inserted or deleted ahead of the next offset can cause a client to duplicate or miss results when combining pages from different source states. The current “complete” footer means the end of that invocation's result list, not proof that previously collected pages form one consistent answer. This matters when a model enumerates all callers before a wiring change.

**Proposal:** continuation must name the query and source generation. Either page an immutable retained result, or reject a stale continuation and supply a bounded restart/delta instruction. The former costs retained memory; the latter costs recomputation and tokens during edit churn. Current read authorization must be checked before replaying retained rows.

## 7. Proposed contract: observation envelopes, not stronger prose

All names below are **proposed design notation**, not existing APIs or predicates.

```text
StructureObservation:
  workspace_identity
  membership_policy_revision
  generation_id
  query_digest + operation_kind
  source_manifest_digest
  module_and_build_configuration_digest
  analysis_kind + analysis_version
  enumeration: complete | incomplete | canceled
  analysis: complete_in_domain | partial | unsupported
  observation_domain: languages, roots, build variants, exclusions
  rows[] with source_revision and match_kind
  omissions[] with reason and authorized repair handle
  continuation: query_id + generation_id + cursor
  dependencies: file revisions + membership/configuration/range dependencies
```

The domain is essential: a complete scan of a declared syntactic domain is not complete program semantics. Intentional secret-path exclusion remains an exclusion, never an invitation to read the secret. A consumer whose required domain intersects an exclusion cannot conclude absence there. Error details and paths must themselves respect authorization.

### 7.1 Generation construction

1. Capture canonical workspace identity, membership policy, relevant module/build inputs, and a start observation epoch. Reuse unchanged immutable parsed file records rather than cloning every AST.
2. Enumerate the required domain. A walk/stat/read failure is an explicit observation deficit; do not convert it to confirmed deletion. Build candidates off to the side.
3. For changed files, capture one immutable byte buffer and derive both its digest and parsed structure from that buffer. This removes the hash-versus-later-parse split. Record before/after metadata checks where filesystem APIs require them; unstable reads are retryable/unknown, not a valid generation.
4. Distinguish syntax failure from IO failure. Preserve current broken-source diagnostics plus last-good repair context, but mark analysis incomplete for claims needing an accurate reference set.
5. Construct references, indices, and dependency summaries against one candidate generation. Validate membership/module inputs and cancellation, then atomically publish the generation pointer. On failure keep the previous generation accessible only as explicitly historical evidence.
6. Queries retain a generation handle for their full operation. A secondary source read must verify the recorded digest or contribute to a new observation, never mix old enclosing spans and new source bytes silently.

A filesystem crawl is not automatically a coherent filesystem snapshot. Exact cross-file snapshot guarantees require an immutable source snapshot or cooperative mutation/lease protocol. Without that, the envelope must describe an observation interval and validation evidence; operations requiring stronger consistency should refuse or acquire the supported ownership mechanism. Repeated rescanning until “it seems stable” is not a proof under arbitrary external writers.

### 7.2 Negative-evidence rule

For query q, domain D, and generation g, permit a **domain-qualified absence conclusion** only if:

```text
AbsentInDomain(q,D,g) =
  EnumerationComplete(D,g)
  AND AnalysisSufficientFor(q,D,g)
  AND NoUnresolvedObservationDeficits(q,D,g)
  AND ResultRows(q,D,g) is empty
```

The executive decides which analysis is sufficient for the requested operation. A local outline can tolerate another unrelated directory being unavailable, provided the requested domain is genuinely isolated. A claim of no uses across the workspace cannot. Public API deletion may require external-consumer review even when `AbsentInDomain` is true.

Go emits observed generation, coverage, and dependency facts. Mangle derives whether the intended operation may rely on those observations, whether a targeted refresh/type-aware analysis is required, and whether a deletion/repoint obligation remains unresolved. No model-authored “coverage complete” statement substitutes for host observations. New prompt behavior belongs in JIT atoms explaining the current obligation and available typed actions, not in ad-hoc hardcoded shard prose.

### 7.3 Refactor plan as a revision-bound dependency contract

Build the plan against immutable inputs. Each planned use carries file digest, normalized-source digest/version, offset span, expected token or resolved-symbol identity, and match strength. Before applying it, verify both source revision and the expected span; a coincidental equal spelling at a changed revision is not enough.

The precondition set must cover:

- **Written resources:** every file whose bytes change.
- **Read resources:** declarations, imports, configuration, and other inspected inputs whose facts justified the rewrite.
- **Set/range dependencies:** the scope in which no additional users were found. A new eligible file or a previously nonmatching file gaining a use is a relevant change, even though it was absent from the original hit list.

Use a conservative workspace/domain epoch initially. Dependency-specific invalidation can come later if measurement justifies its complexity. For edits controlled by CodeNERD, validate these dependencies under a shared mutation coordinator/lease before using the existing commit path. For external writers not governed by that protocol, state the residual race and use an immutable snapshot or refuse the stronger safety claim. Do not present a final hash check followed by an unlocked write as atomic compare-and-swap.

Unknown coverage should produce a repair obligation, not an endless model loop. Retry transient reads under a bounded IO budget; if no new evidence can be obtained, return the specific unresolved scope and permitted next action. This respects progress-based task continuation without manufacturing completion.

## 8. Primary foundations and what they do not prove

- [Kung and Robinson, *On Optimistic Methods for Concurrency Control* (1981), §§3–5](https://www.eecs.harvard.edu/~htk/publication/1981-tods-kung-robinson.pdf): validation concerns conflicts with a transaction's read set as well as its writes. The application here is validating the evidence that justified a refactor, not merely its destination bytes. Their database conditions do not automatically hold for external filesystem writers.
- [Mokhov, Mitchell and Peyton Jones, *Build Systems à la Carte* (2018), §4.2.2](https://simon.peytonjones.org/assets/pdfs/build-systems-original.pdf): verifying traces record dependency hashes to determine whether cached work remains valid. The design analogy is to parsed files and query dependencies. Source hashing alone does not capture membership/range dependencies or make a partial analysis semantically complete.
- [Go authors, gopls code transformation documentation, “Rename”](https://github.com/golang/tools/blob/master/gopls/doc/features/transformation.md#rename): type information supports shadowing/interface checks, while reflection and dynamic behavior still limit safety. A type-aware backend is a useful comparison and optional escalation, not a universal refactoring oracle or a reason to replace lightweight syntax tools for every query.

These are established ideas adapted to CodeNERD's current seams, not a novelty claim. No comparative accuracy, token reduction, latency gain, or whole-harness superiority is established here.

## 9. End-to-end costs and fair alternatives

The present refresh holds the index mutex through enumeration and worker completion; parsing is concurrency-limited by a semaphore, but a goroutine is launched for each stale candidate (`refreshLocked`, [`internal/world/structure_index.go:165–168,188–303`](https://github.com/theRebelliousNerd/codenerd/blob/24eb5faaa65a4f55cdc6061e809b09fc25d8376a/internal/world/structure_index.go#L165-L303)). Thus zero reparses is not zero refresh cost, and parser concurrency is not the same as bounded pending-worker count. `FindText` additionally rereads selected files after refresh (`FindText`, [`internal/world/structure_queries.go:135–184`](https://github.com/theRebelliousNerd/codenerd/blob/24eb5faaa65a4f55cdc6061e809b09fc25d8376a/internal/world/structure_queries.go#L135-L184)).

Let N be visited paths, B_delta changed-source bytes, B_verify bytes requiring validation, S live index size, R result rows, and P pages. A naive sequence of P refreshed pages pays approximately P enumeration passes plus query work; immutable result paging instead pays one query plus retained result/handle storage, subject to expiry and authorization checks. This is an accounting model, not a measured speedup.

Track separate units:

```text
model: uncached input, cached input, output, reasoning, retries
host: enumeration/stat calls, hashed bytes, parsed bytes, CPU, lock wait
storage: live index, staged generation delta, retained result pages, historical handles
validation: dependency checks, type-aware analysis, build/test obligations
outcome: wall time, unresolved tasks, false absence, incorrect edits, human repair
```

Copy-on-write generations require roughly S plus changed-record and retained-generation overhead, not necessarily two complete indices. Long-lived readers can retain large old generations; cap authorized retention by resource budget and return explicit expired handles. A byte-buffer cache reduces rereads but costs memory and requires secret-data controls. A worker pool bounds pending work better than a goroutine per changed file but must preserve cancellation joining. Watchers can reduce unnecessary enumeration only with an overflow/lost-event recovery contract; do not trade known full checks for unobservable staleness.

Compare four variants at identical task quality and authority:

1. **A: current implementation**, including all existing gates, revisions, and error behavior.
2. **B: explicit coverage and propagated failures**, retaining the current index organization. Lowest-complexity correctness fix.
3. **C: immutable generation/query handles plus read-set validation**, preserving B and existing optimistic writes.
4. **D: C with policy-selected type-aware escalation**, only for operations whose required evidence exceeds the syntax model.

B may eliminate most false absence at lower cost. C is justified by observed race/paging workloads, not fashion. D should not charge every navigation query for whole-program analysis. Evaluate parsing/coverage in isolation and complete coding tasks separately; fewer query tokens at the expense of more failed tasks, repeated scans, or human repairs is not efficiency.

## 10. Proposed validation program

No tests below were executed. Use deterministic injected seams rather than timing-dependent permission changes alone. OS permission behavior differs, particularly under privileged test users.

| Fixture | Required observation |
|---|---|
| Warm index; caller subtree enumeration fails | Prior generation not silently rewritten as complete; workspace absence conclusion unavailable; targeted valid positive evidence remains explicitly scoped. |
| Hash pass succeeds; changed file read fails during parsing | IO deficit recorded; last-good state historical, not deleted as confirmed absence. |
| Candidate use file becomes unreadable or unparsable after selection | Delete/repoint receives unresolved coverage, performs no write based on empty uses. |
| Rewrite at same byte offsets before planning, valid syntax preserved | Revision/span identity conflict before any mutation. |
| New use added in previously nonmatching file | Range/domain dependency invalidates plan; hit-only dependency tracking fails this control. |
| Row inserted/deleted before second page | Stable generation continuation or explicit stale-cursor response; no silent merged-page gap. |
| Same-size source/go.mod rewrite with restored mtime | Preserve existing content-freshness contract and regression definitions. |
| Real deletion and intentional ignore-policy change | Current eligible domain updates correctly; not permanently frozen by conservative retention. |
| Broken-source repair | Current errors and labelled last-good declarations remain available; absence safety is not relaxed. |
| Reflection, interfaces, function values, external public API | Explicit domain/semantic limitations; syntax absence does not certify safe removal. |
| Cancellation during work and before publication | Admitted workers joined or safely bounded; no generation published as complete after canceled verification. |
| Source mutation outside cooperative coordinator | Detect where possible; otherwise label unsupported consistency guarantee, never claim atomicity. |

For scale, vary eligible files from hundreds to tens of thousands, cold/warm index, changed fraction, directory depth, language mix, result-set size, failed-read frequency, and concurrent readers/writers. Measure the complete cost ledger, peak retained generations, p50/p95/p99 latency, and lock waiting. Do not import historical source-comment memory/latency numbers as measurements from this study.

For task evaluation, include locating production entry points, proving a purportedly unwired feature is actually registered, deleting genuinely obsolete local code, safe multi-file repointing, and intentionally unsupported dynamic cases. Keep task snapshots, model/version, task budget, approvals, and held-out behavioral checks identical across variants. Randomize paired assignments, report uncertainty intervals and failures, and count a correct refusal with an actionable coverage deficit separately from both success and silent wrong completion. Report cost per verified task together with verified-task rate so abstention cannot game efficiency.

## 11. Buildable research queue

These local research IDs do not assert adoption or replace the owning world/tools specifications.

| ID | Capability / present deficiency | Proposed exit and dependency |
|---|---|---|
| STRUCT-EVIDENCE-01 | Complete enumeration distinguished from skipped IO; S1 | Failure-injection fixtures preserve old historical generation and prohibit false current absence; positive deletion/membership controls pass. Depends on observation-state schema. |
| STRUCT-EVIDENCE-02 | Use-discovery completeness consumed by edit guard; S2 | Actual registered delete/repoint path refuses incomplete relevant use discovery before writes; narrow complete-domain controls remain usable. Depends on typed provider result and executive obligation wiring. |
| STRUCT-EVIDENCE-03 | Refactor offsets and full dependency set tied to source; S3 | Pre-plan byte substitution and newly introduced-user fixtures invalidate the plan without overwriting concurrent edits. Depends on generation identity and declared coordination limits. |
| STRUCT-EVIDENCE-04 | Stable paginated structural evidence; S4 | Inter-page changes cannot silently lose or duplicate rows in a claimed single answer; expiry/auth changes are explicit. Depends on result ownership and retention budget. |
| STRUCT-EVIDENCE-05 | Demonstrated complete-system value | Paired task study reports accuracy, actionable abstention, failures, full tokens/compute, and human repair; retain the simpler design when additional mechanisms do not earn their cost. |

Before implementation, reconcile the world and tools specifications with these findings, preserve already-correct freshness and permission controls, and choose the narrowest mechanism that establishes the required evidence. The first improvement is straightforward: stop turning an inability to observe into a claim that nothing is there.

## As built (2026-10-27)

Each finding was first reproduced by a failing test through the registered tools (unreadable caller directory let `delete_element` delete a used declaration and `callers_of` answer absence; repoint rewrote `lib.Bu1ld` measured as `lib.Build`; a user added after discovery survived a delete; paging had no source identity), then fixed.

**Implemented**

- **Envelope.** `codedom.Observation` (`internal/tools/codedom/observation.go`): `Generation`, `Index`, `Deficits`, `Complete()`. Every `StructureProvider` method returns it instead of a stats string; `world.observation` (`structure_provider.go`) builds it from `StructureStats.Generation`/`Deficits` (`internal/world/structure_refresh.go`).
- **S1.** `StructureIndex.refreshLocked` (moved to `structure_refresh.go`): an unlistable directory, stat failure, hash-read failure or parse-read failure is a `StructDeficit`; covered entries are kept as last observed (`deficitSet.covers`), never deleted. Genuine `ErrNotExist`, membership and secret exclusions still remove. The parse reads once through `structReadFile` and verifies the bytes against the stamped hash; a mismatch indexes the bytes read with no stamp so the next refresh re-observes. `StructureStats.scoped` keeps only deficits intersecting a path-scoped query (`FindSymbol`, `Outline`, `FindText`). `pageRequest.render` never prints absence prose or "complete" for an incomplete observation and names the deficits.
- **S2.** `StructureIndex.Uses` (`structure_queries.go`): unreadable or unparsable candidates are deficits; a candidate whose digest moved since the refresh is observed again (`usesAttempts` = 3), then reported. `usesOutside` returns the observation; `executeDeleteElement` and `executeRepoint` refuse an incomplete one via `unresolvedUses` before any write.
- **S3.** `StructUse`/`StructureUse` carry `Digest` (hex SHA-256 of raw bytes) and `Text`; `verifyUseSource` (`use_evidence.go`) refuses a plan whose file digest or span text differs. `commitFiles(ctx, planned, reads...)` re-establishes `readDependency`s under `commitMu` before the optimistic byte check: `usesUnchanged` (delete, repoint, delete-with-replace), `declarationStands` (repoint destination), `scriptSitesUnchanged` (script rename).
- **S4.** `offset` is replaced by `cursor` (`g<gen>.q<query digest>.o<row>`) on all eight paginated structural tools. A cursor for another query, a malformed cursor, or one from another generation is refused (`stale cursor`) with a restart instruction.
- JIT atom `capability/structure_queries/caller-marks-and-search-gate` explains the incomplete footer and cursor; seed reconciled. World gap rows `GAP-WORLD-STRUCTURE-REVISION` (stale) and `GAP-WORLD-STRUCTURE-EVIDENCE` updated.

**Ruled differently**

- Stale continuation is rejected, not served from retained results: no retention memory and no authorization replay; pages are rare at 150 rows.
- The read set is the re-observed use set itself, not a workspace epoch: an unrelated edit elsewhere does not invalidate a repoint. The re-observation re-enumerates the candidate domain, so a newly importing user is caught.
- A hash-read failure became a per-file deficit instead of failing every query; absence claims stay closed because the observation is incomplete.
- `usesAttempts` and `deficitsShown` are fixed protocol bounds like `structurePageSize`, not config tunables.

**Deferred**

- Mangle-derived obligations (§7.2: Go facts, Mangle decides sufficiency): no policy rule consumes coverage today; the host refuses at the only destructive consumers. Emitting facts without a consumer would be unwired.
- Secret-path exclusions are not yet reported as domain exclusions to `Uses`; a use inside a secret file stays invisible, as before.
- Writers outside codedom are not serialized by `commitMu`; the dependency check narrows, but does not close, that window. No atomicity is claimed.
- Type-aware escalation (variant D), the paired task study (STRUCT-EVIDENCE-05) and scale/latency measurement need live models or benchmarks.

## Status note (2026-10-28)

Re-verified on main: S1-S4 tests pass in `internal/world` and `internal/tools/codedom`. Fixed now: a Go/Mangle file excluded by `execution.secret_paths` is a counted (path-free) deficit of `Uses`, so delete/repoint refuse instead of treating an invisible user as absent (`structure_queries.go`, `structure_refresh.go`).
Ruled out: retained result paging and a workspace epoch (earlier rulings stand).
Left: Mangle-derived sufficiency obligations (no consumer beyond the host refusal), type-aware escalation and the paired study (need live models); writers outside codedom are not serialized by `commitMu`.
