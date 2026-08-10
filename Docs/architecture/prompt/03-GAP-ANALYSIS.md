# Prompt JIT gap analysis

> Current evidence: [Current State](02-CURRENT-STATE.md). Target contract:
> [Vision](01-VISION.md). Authoritative implementation cards: [TODO](TODO.md).

## Method

Claims were traced from the live `internal/prompt` tree through production
construction and consumers, then checked with strict atom validation, ordered
runtime parity, package tests, and focused production-adapter race tests. A source
path is not proof until its named symbol and behavioral discriminator agree.

## Spec versus reality

| Capability | Reality | Verdict |
|---|---|---|
| Runtime atom compilation | Full collect -> select -> resolve -> fit -> assemble path in `internal/prompt/compiler.go#JITPromptCompiler.Compile` | `VERIFIED CURRENT` |
| Deterministic skeleton | Requires a kernel and Mangle-selected identity/protocol/safety/methodology in `internal/prompt/selector.go#AtomSelector.loadSkeletonAtoms` | `VERIFIED CURRENT` |
| Degradable flesh | Vector/flesh errors fall back or drop while skeleton survives | `VERIFIED CURRENT` |
| Exact cache identity | `CompilationContext.Hash` uses `compilation-context-v2`, length-prefixed fields, and canonical set encoding; retry/tool changes produce different keys | `VERIFIED CURRENT` (closed G1) |
| Request-scoped logic facts | Production `KernelAdapter.NewCompilationScope` clones the live `RealKernel`; deferred close discards selector facts on every exit | `VERIFIED CURRENT` (closed G2); third-party adapter residual |
| One atom schema | Validator, filesystem loader, embedded loader, and synchronizer use `ParsePromptAtomYAML`; 333 files produce the same ordered 888 IDs | `VERIFIED CURRENT` (closed G3) |
| Tool/config capability generation | Production provider is live; alternate `SimpleRegistry` catalog is test-only and drifted | `PARTIAL` (G5) |
| Decision receipt | In-memory manifest and logs exist, but no durable turn/permission/outcome correlation | `PARTIAL` (G4) |
| Safe prompt evolution | Evolved atom directories and manager exist; shadow/replay promotion evidence is absent | `PARTIAL` (G8) |
| Verification breadth | 235 listed tests; full prompt/sync and focused production scope/cache race gates pass; no fuzz or external-adapter conformance suite | `PARTIAL` (G7) |

## Closed truth gaps

### Closed G1: no-tool retry cache collision

`VERIFIED CURRENT` — `internal/prompt/context.go#CompilationContext.Hash` now hashes
every prompt-affecting scalar and set-like field under schema
`compilation-context-v2`. Values are length-prefixed; frameworks and available
tools are sorted and deduplicated without mutating caller state.
`internal/system/prompt_kernel_scope_test.go#TestKernelAdapter_RetryContextBypassesPreRetryCache`
compiles an initial prompt, changes retry state and tools, and proves the retry
atom plus exact new tools appear without stale `read_file` capability text.

### Closed G2: production prompt facts shared across compiles

`VERIFIED CURRENT` — `internal/prompt/compiler.go#acquireCompilationKernel` asks a
`KernelScopeProvider` for a compilation-owned evaluator. Production
`internal/system/factory_adapters.go#KernelAdapter.NewCompilationScope` clones the
primary `RealKernel`; skeleton and flesh assert/query only that clone and deferred
scope close discards it on success, budget error, cancellation, and panic.

Evidence:

- `internal/system/prompt_kernel_scope_test.go#TestKernelAdapter_CompilationScopesIsolateConcurrentPrompts`
- `internal/system/prompt_kernel_scope_test.go#TestKernelAdapter_CompilationScopeDoesNotLeakOnBudgetError`
- `internal/system/prompt_kernel_scope_test.go#TestKernelAdapter_CompilationScopeDoesNotLeakOnCancellation`
- `internal/prompt/compiler_scope_test.go#TestJITPromptCompiler_CompilationScopeClosesAfterSelectorPanic`
- focused `go test -race` over the four production scope/cache tests: PASS

`PARTIAL` residual — a third-party `KernelQuerier` that implements neither
`KernelScopeProvider` nor `KernelRetracter` still compiles through the compatibility
path without guaranteed isolation. Predicate-wide retraction is also unsafe for
concurrent third-party compiles, so adapter conformance or fail-closed behavior is
still a product decision.

### Closed G3: atom authoring/runtime contract drift

`VERIFIED CURRENT` — `internal/prompt/atom_schema.go#AtomDefinition` and
`ParsePromptAtomYAML` are the single versioned strict contract. Unknown fields,
unsupported schema versions, invalid sequence members, duplicate IDs, invalid
dependencies, and unknown world states fail closed. Legacy `agent_types`, metadata,
nested selectors, and semantic-string versions are observable migrations bounded
to 2027-01-01; built-ins may not use them.

The two checked-in legacy files are canonical now. `KnownWorldStates` derives from
the typed live dimensions and includes `reflection_hits` and
`no_tool_call_retry`. `cmd/tools/validate_prompt_atoms#TestCheckedInCorpusOrderedParity`
pins validator/filesystem/embedded order, count 888, and ordered-ID digest.

Receipt: `go run ./cmd/tools/validate_prompt_atoms -root internal/prompt/atoms
-fail-on-warn` -> 333 files, 888 atoms, zero issues.

## Open gaps

### Gap G4: manifest is not an end-to-end decision receipt

`internal/prompt/manifest.go#PromptManifest` records context hash, token usage,
selected atoms, and dropped atoms. It does not identify the session/turn, exact
source version, permission decision, model response, tool effect, redaction policy,
or retention. `GetLastResult` is one process-local pointer.

Blocked goal: G4. Severity: important operability.

### Gap G5: alternate ConfigAtom catalog is dormant and drifted

Production uses `internal/prompt/config_factory.go#NewDefaultConfigAtomProvider`.
`internal/prompt/config_defaults.go#RegisterDefaultConfigAtoms` is referenced only
by tests and uses a different intent/tool vocabulary. Policy-set names now share a
canonical registry, but the tool capability catalogs are not generated from the
live registry.

Blocked goal: G3 contract consolidation. Severity: important maintenance.

### Gap G6: cache TTL and cached-result ownership are underspecified

`CompilerConfig.CacheTTLSeconds` defaults to 300, but entries carry no timestamp
and lookup enforces no TTL. Size eviction and explicit clear are live. Cache hits
also return a shared `*CompilationResult`; callers are expected to treat it as
immutable, but the type does not enforce that contract.

Blocked goal: G1 hardening. Severity: important freshness and ownership honesty.

### Gap G7: verification residuals remain

There is no `Fuzz*` target. Full prompt/sync and focused production adapter tests
pass under `-race`; however, external adapters still lack a conformance suite
proving scope-or-fail-closed behavior.

Blocked goals: operational hardening. Severity: important verification.

### Gap G8: evolved atoms lack shadow/replay evidence

`EvolvedAtomManager` can load promoted and pending files, but there is no
turn-correlated shadow comparison, outcome rubric, promotion receipt, or safe
counterfactual replay. Automated optimization without those gates could confound
model, context, prompt, and tool-policy changes.

Blocked goal: G5. Severity: strategic; moonshot remains deferred.
### Gap G9: 2026-08-09 canonical atom precedence vs stale corpus.db built-ins — PARTIALLY FIXED (runtime), OPEN (boot DB reconciliation)

Current reality (truth-corrected 2026-08-09, commit 1ad8238e): runtime canonical first-source precedence is fixed — embedded atoms are now collected first and corpus.db copies are deduplicated during prompt collection, so stale database copies no longer shadow embedded atoms at runtime. Validator, filesystem, and embedded parity checked count/order/digest (888 IDs). Remaining OPEN: boot database reconciliation and stale built-in removal — corpus.db still retains 878 stale built-in rows diverging from the 888-ID canonical corpus and the synchronizer does not yet reconcile/replace stale corpus.db built-in rows to match embedded content on boot; stale built-in removal is still required.

Required contract (still OPEN — DB side): boot reconciliation of stale built-ins while preserving project-only atoms. On every boot, the synchronizer must treat the embedded corpus as the authority for any built-in atom ID, reconcile/replace stale corpus.db built-in rows to match the embedded content (and remove stale built-in duplicates), and never drop project-only (non-built-in) atoms.

Negative acceptance exam (OPEN, must be added): a test that seeds a temporary corpus.db with stale built-in copies (878 stale records diverging from the current embedded 888-ID corpus), boots the synchronizer, and asserts (a) the stale built-ins are reconciled to the canonical embedded content (or removed) and (b) a project-only atom present only in corpus.db survives the reconciliation.

### Gap G10: 2026-08-09 prompt/world/session task-integrity coupling — OPEN

Current reality (truth-corrected): the pre-delegation world scan was fresh and exposed dirty state; the cleanup was allowed because ownership baseline and shell scope enforcement were absent — there was no immutable pre-task ownership baseline snapshotted at task start and no shell-effect attribution/scope-check, so the task could revert pre-existing dirty tracked work and delete an untracked directory under the task path. The world became stale only after unreported shell mutations because no incremental refresh ran, while prompt/sync runtime precedence itself was already fixed in 1ad8238e.

Required contracts (all OPEN): (1) capture an immutable pre-task ownership
baseline; (2) detect, attribute, and scope-check shell effects before reporting
success; (3) never revert or delete pre-existing dirty or untracked paths; and
(4) incrementally retract and reassert world facts after every accepted
mutation.

Negative acceptance exams (OPEN): (a) git checkout of dirty tracked work in a
temporary repo — task must not revert the pre-existing dirty tracked file to
HEAD; (b) recursive deletion of an untracked directory (e.g., rm -rf of an
untracked folder) in a temporary repo — task must not delete the pre-existing
untracked directory. Both must reproduce the violation without losing either
artifact and must pass only when scope-checked shell detection and baseline
preservation are implemented.



## North-star alignment map

| Gap | State | Required evidence |
|---|---|---|
| G1 retry/cache identity | Closed | Keep v2 field and retry regressions green; version future field changes |
| G2 production isolation | Closed with external-adapter residual | Require scope-capable adapters or fail closed; add adapter conformance tests |
| G3 atom contract drift | Closed | Keep strict CLI and ordered 888-ID parity green; remove migrations on schedule |
| G4 decision receipt | Open | Versioned redacted schema; one receipt per model call; permission/outcome correlation test |
| G5 dormant config catalog | Open | One authority or generated tool parity; live registry conformance test |
| G6 TTL/ownership | Open | Tested TTL semantics or removed setting; immutable/copy-on-read result contract |
| G7 verification residuals | Open | Fix test mock race; add fuzz and external-adapter conformance gates |
| G8 no shadow/replay | Open | Shadow-only diff receipts, manual promotion, rollback drill, no side-effect replay |
| G9 canonical atom precedence vs stale corpus.db (2026-08-09) | PARTIALLY FIXED (runtime 1ad8238e: embedded first, DB deduplicated), OPEN for boot DB reconciliation/removal | Boot DB reconciliation: stale corpus.db built-ins reconciled/removed to 888-ID embedded canonical; project-only atoms preserved; runtime precedence already verified — add DB seeding exam |
| G10 prompt/world/session task-integrity coupling (2026-08-09) | OPEN (pre-delegation scan was fresh; staleness only after unreported shell mutations; cleanup allowed by missing baseline/scope-check) | Immutable pre-task ownership baseline; shell effects from run_command/bash detected/attributed/scope-checked and fail closed before success; pre-existing dirty/untracked never reverted/deleted; accepted mutations trigger incremental world retraction/reassertion so world not stale; negative exams for git checkout of dirty tracked work and rm -rf of untracked directory in temp repo without loss |

## Priority order

### Critical — 2026-08-09 task-integrity incident (do first)

1. G9 canonical precedence: boot DB reconciliation/removal of stale corpus.db built-ins while preserving project-only atoms — runtime precedence already fixed in 1ad8238e; add negative exam (878 stale vs 888 canonical + project-only survival).
2. G10 task-integrity coupling: immutable pre-task baseline, run_command/bash shell-effect detection/attribution/scope-check fail-closed before success, no revert/delete of pre-existing dirty/untracked (pre-delegation scan was fresh), incremental world retraction/reassertion so world stale only after unreported mutations; add both negative exams (dirty checkout + untracked rm -rf in temp repo).

### Important now (after task-integrity)

3. G7 pin external-adapter isolation policy and add conformance/fuzz gates.
4. G5 generate or remove the dormant capability catalog.
5. G6 make TTL and cached-result ownership honest.
6. G4 persist a redacted end-to-end receipt.

### Strategic after receipts

7. G8 shadow selection and only then the deferred replay lab.

## Recommendations

| Horizon | Recommendation | Rollback boundary |
|---|---|---|
| Immediate — incident | Reconcile stale corpus.db built-ins to embedded canonical on boot (project-only preserved); runtime first-source precedence already fixed in 1ad8238e (embedded collected first, DB deduplicated) — verify DB reconciliation/removal still OPEN; add seeding exam (878 stale vs 888 canonical) | Restore prior synchronizer; keep stale-DB fixture for regression |
| Immediate — incident | Capture immutable pre-task ownership baseline; detect/attribute/scope-check every run_command/bash mutation and fail closed before success; enforce no revert of pre-existing dirty tracked work and no delete of pre-existing untracked paths — pre-delegation scan was fresh and exposed dirty state, cleanup was allowed because baseline/scope-check were absent; wire accepted mutations to incremental world retraction/reassertion so world becomes stale only after unreported shell mutations without refresh; add both temp-repo negative exams | Gate shell detection behind flag; revert baseline snapshot if unstable; keep world reassertion off until scoped |
| Immediate | Add scope-provider conformance tests and pin fail-closed behavior for unscoped adapters | Retain compatibility wrapper during migration |
| Immediate | Require `KernelScopeProvider` in production-facing constructors or explicitly reject unscoped adapters | Compatibility flag or adapter wrapper during migration |
| Short term | Generate or delete alternate ConfigAtom tool catalog; test live registry parity | Revert generated manifest to last known-good version |
| Short term | Implement or remove cache TTL; make cached results immutable/copy-on-read | Clear process cache and restore prior ownership contract |
| Short term | Add durable redacted receipt and inspector diff | Disable persistence while preserving in-memory manifest |
| Strategic | Shadow compiler and bounded replay lab | Shadow off switch; never replay tools; atom promotion remains manual |
