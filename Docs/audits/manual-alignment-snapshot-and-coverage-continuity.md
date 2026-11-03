# Complete manual alignment's snapshot binding and coverage display

## Conclusion

On a fresh normal TUI session, a scoped manual `/alignment` request can omit available module purpose and requirements because chat binds the shared Guardian's **fact sink**, but not its **fact reader**. The command passes a correct typed bare-path scope; the missing dependency prevents that scope from resolving. The Guardian correctly records an unknown module-coverage outcome and can still produce an assessed project-level verdict. The TUI then discards the coverage object and renders only that verdict, score, explanation and suggestions.

This is a **TUI boot-and-adapter parity continuation** of the existing module-alignment-scope repair, not a claim that Guardian core hides incomplete evidence or that project-only fallback is wrong. The narrow repair is to supply the session's existing snapshot-capable kernel to the Guardian and preserve independent coverage information through the manual result adapter. Do not force incomplete module coverage to replace a legitimate project verdict with failure.

**Source pin:** main `388205138ba6d47c1cff0c5e97c896dfa642a391`, rechecked 2026-10-30 at approximately 14:14 UTC. Static source review only: no tests, builds, model calls, runtime reproduction or repository modifications. The control-flow conclusion was independently challenged; no production incident, bad model judgment, security bypass or measured performance impact is established.

## Current status and prior coverage

Main remained unchanged during this pass. The latest inspected research branch, `research-main-20261030-1346`, tip `0b9eab6cc9f406a17bec847a84c3259e4e0cbf41`, contains four documentation-only additions concerning campaign refinement dependency kind, generated-tool retirement admission, review severity admission and semantic expansion candidate windows. Those are separate topics, not an implementation repair for this boundary. [Research tip](https://github.com/theRebelliousNerd/codenerd/commit/0b9eab6cc9f406a17bec847a84c3259e4e0cbf41).

The queue marks `guardian-module-alignment-scope` fixed at `135405dfc`. That work is materially present: structural scope and roles are separate from prose, module facts are read through a snapshot interface, unavailable evidence remains explicit, project fallback is preserved, and coverage is recorded with each check. Its as-built section specifically credits manual bare-path support and the corrected independence of unknown scope from passed or blocked project verdicts. This note must accompany that repair, not replace or relabel it as ineffective. [Queue](https://github.com/theRebelliousNerd/codenerd/blob/388205138ba6d47c1cff0c5e97c896dfa642a391/Docs/audits/ASTRA_QUEUE.md#L116-L126), [prior audit and as-built](https://github.com/theRebelliousNerd/codenerd/blob/388205138ba6d47c1cff0c5e97c896dfa642a391/Docs/audits/guardian-module-alignment-scope.md#L139-L206).

The prior audit's manual control assumed a shared Guardian with a querier. Current manual-scope tests also install the querier in their fixture before invoking the Guardian directly. The residual is the normal **fresh TUI assembly and its result transport**, not another proposal to extract paths from prose. [Scoped test fixture](https://github.com/theRebelliousNerd/codenerd/blob/388205138ba6d47c1cff0c5e97c896dfa642a391/internal/northstar/alignment_scope_test.go#L44-L73), [manual/background scope tests](https://github.com/theRebelliousNerd/codenerd/blob/388205138ba6d47c1cff0c5e97c896dfa642a391/internal/northstar/alignment_scope_test.go#L240-L267).

The older architecture wiring page already discussed parent-kernel parity and manual UX wiring, but its early sections describe a removed legacy boot file and per-command Guardians. Its later checklist credits the campaign constructor's querier binding. Neither establishes that current shared chat boot binds the reader, or that the manual result message retains coverage. Prefer the current source below over those historical descriptions. [Older wiring discussion and later checklist](https://github.com/theRebelliousNerd/codenerd/blob/388205138ba6d47c1cff0c5e97c896dfa642a391/Docs/architecture/northstar/08-WIRING-AND-INTEGRATION.md#L24-L68), [checklist](https://github.com/theRebelliousNerd/codenerd/blob/388205138ba6d47c1cff0c5e97c896dfa642a391/Docs/architecture/northstar/08-WIRING-AND-INTEGRATION.md#L105-L124).

The five inspected active code/QA tips—`51cfd42649552866d4aa54c53b68acf5740eb09b`, `a53cfc308e8836c72370adaa46deba3f7b00180d`, `80af1863d3a3aa46a722ac8b0c8a0985582d4995`, `aa7988dec4c62116656ea722ff73b687a8509908` and `767a3291d6287b238417f2466592a9002810d05a`—still had no `SetQuerier` in the inspected shared-boot or manual-helper files. Those older files differ from main and predate current scope transport; this is not a claim of whole-branch identity or exhaustive branch proof. The bounded duplicate check covered current guidance, queue, relevant Northstar audits/specifications and recent research inventories through 1346.

## Live consumer path

### 1. The normal chat boot supplies the wrong half of the kernel dependency

`Model.Init` invokes `performSystemBoot`, which delegates to `performSystemBootShared`. Shared boot obtains a session kernel from the Cortex. `sessionKernels` deliberately returns the Cortex-facing session kernel rather than substituting the catch-all RealKernel for normal session reads. [TUI entry](https://github.com/theRebelliousNerd/codenerd/blob/388205138ba6d47c1cff0c5e97c896dfa642a391/cmd/nerd/chat/model_lifecycle.go#L193-L205), [boot wrapper](https://github.com/theRebelliousNerd/codenerd/blob/388205138ba6d47c1cff0c5e97c896dfa642a391/cmd/nerd/chat/session_shared_boot.go#L30-L34), [kernel acquisition](https://github.com/theRebelliousNerd/codenerd/blob/388205138ba6d47c1cff0c5e97c896dfa642a391/cmd/nerd/chat/session_shared_boot.go#L89-L105), [session kernel ownership](https://github.com/theRebelliousNerd/codenerd/blob/388205138ba6d47c1cff0c5e97c896dfa642a391/cmd/nerd/chat/session_shared_boot.go#L336-L372).

Shared boot acquires the workspace Guardian, sets the LLM client, calls `SetParentKernel(kernel)`, initializes the Guardian, and attaches its background handler. It does not call `SetQuerier`. [Actual assembly](https://github.com/theRebelliousNerd/codenerd/blob/388205138ba6d47c1cff0c5e97c896dfa642a391/cmd/nerd/chat/session_shared_boot.go#L270-L284).

These are not synonymous setters. `SetParentKernel` assigns `g.kernel` for fact injection; `SetQuerier` assigns `g.querier` for module resolution. `NewGuardian` initializes neither, and `Initialize` loads/reconciles vision and state and projects facts without deriving a querier from the parent kernel. [Constructor](https://github.com/theRebelliousNerd/codenerd/blob/388205138ba6d47c1cff0c5e97c896dfa642a391/internal/northstar/guardian.go#L88-L95), [separate dependencies and initialization](https://github.com/theRebelliousNerd/codenerd/blob/388205138ba6d47c1cff0c5e97c896dfa642a391/internal/northstar/guardian.go#L149-L220).

This is a missing binding rather than an unavailable production API: `CortexKernel.QueryAll` exists and merges shard results, propagating a shard query error. The proposed repair should use the established session kernel, not create a separate kernel or maintain a second module-fact cache. This review does not establish cross-shard transactional isolation beyond that API's implemented behavior. [Snapshot implementation](https://github.com/theRebelliousNerd/codenerd/blob/388205138ba6d47c1cff0c5e97c896dfa642a391/internal/core/cortex_kernel.go#L732-L748).

### 2. The manual command preserves scope but does not repair the missing reader

The slash handler calls `runAlignmentCheck(subject)`. That helper acquires the same workspace Guardian, sets an available client, initializes it, builds recent session context, and calls `CheckAlignment` with `ManualAlignmentScope(m.workspace, subject)` and a 60-second context. It never supplies `m.kernel` as a querier. Its shared acquisition and deferred release correctly avoid opening a new independent Guardian for every command. [Command entry](https://github.com/theRebelliousNerd/codenerd/blob/388205138ba6d47c1cff0c5e97c896dfa642a391/cmd/nerd/chat/commands_handlers_analysis.go#L519-L535), [manual execution](https://github.com/theRebelliousNerd/codenerd/blob/388205138ba6d47c1cff0c5e97c896dfa642a391/cmd/nerd/chat/model_helpers.go#L391-L457).

`ManualAlignmentScope` intentionally recognizes an entire bare path, including `pkg/a/file.go`, as a `manual_target`. Ordinary prose is intentionally project-only; incidental path mentions are not extracted. That behavior is a control to preserve, not a second defect. [Manual scope contract](https://github.com/theRebelliousNerd/codenerd/blob/388205138ba6d47c1cff0c5e97c896dfa642a391/internal/northstar/alignment_scope.go#L66-L86).

The process-wide registry returns the existing Guardian for the same canonicalized `.nerd` directory. It is therefore relevant that `BuildCampaignObserver` **does** set both parent kernel and querier when supported. A prior same-process, same-workspace campaign-observer construction can supply the missing reader and make later manual checks resolve. The witness below explicitly excludes that warm-up; this finding does not say every manual request omits module context. [Shared registry](https://github.com/theRebelliousNerd/codenerd/blob/388205138ba6d47c1cff0c5e97c896dfa642a391/internal/northstar/registry.go#L54-L91), [campaign's correct binding](https://github.com/theRebelliousNerd/codenerd/blob/388205138ba6d47c1cff0c5e97c896dfa642a391/internal/northstar/campaign_observer.go#L44-L78).

### 3. Guardian core handles the absence honestly

For a scoped request, `resolveAlignmentScope` reads `g.querier`, requires the `QueryAll` snapshot capability, and returns unknown/incomplete coverage with `module fact snapshot unavailable` if it is absent. It supplies no module section. A project-only request returns resolved/complete without needing that reader. [Exact resolution branch](https://github.com/theRebelliousNerd/codenerd/blob/388205138ba6d47c1cff0c5e97c896dfa642a391/internal/northstar/alignment_modules.go#L13-L52).

`CheckAlignment` retains that coverage and explicitly permits a project-level assessment without module refinements. Its final request includes project vision, the scope section and issue reasons, subject, and additional context. No-client, unavailable-prompt and completion failures remain unassessed unknown outcomes. [Fallback and completion](https://github.com/theRebelliousNerd/codenerd/blob/388205138ba6d47c1cff0c5e97c896dfa642a391/internal/northstar/guardian.go#L396-L438), [request composition](https://github.com/theRebelliousNerd/codenerd/blob/388205138ba6d47c1cff0c5e97c896dfa642a391/internal/northstar/guardian.go#L498-L519).

Core logs retain coverage completeness and full issue reasons. The store writes coverage and the check in the same transaction, and background assessment metadata carries the independent scope outcome and reasons. These are substantial existing mitigations. The missing information is **not lost everywhere**; a successful store write or the logs can expose it. [Structured logs](https://github.com/theRebelliousNerd/codenerd/blob/388205138ba6d47c1cff0c5e97c896dfa642a391/internal/northstar/guardian.go#L468-L495), [transactional history](https://github.com/theRebelliousNerd/codenerd/blob/388205138ba6d47c1cff0c5e97c896dfa642a391/internal/northstar/store.go#L483-L560), [background metadata](https://github.com/theRebelliousNerd/codenerd/blob/388205138ba6d47c1cff0c5e97c896dfa642a391/internal/northstar/observer.go#L343-L367).

### 4. The TUI result adapter drops that independent outcome

The successful manual return copies only subject, result, score, explanation and suggestions. `alignmentCheckMsg` has no coverage or check-ID field. The message handler formats that reduced object and pushes it into chat. [Copy boundary](https://github.com/theRebelliousNerd/codenerd/blob/388205138ba6d47c1cff0c5e97c896dfa642a391/cmd/nerd/chat/model_helpers.go#L450-L456), [message shape](https://github.com/theRebelliousNerd/codenerd/blob/388205138ba6d47c1cff0c5e97c896dfa642a391/cmd/nerd/chat/model_types.go#L970-L978), [message consumer](https://github.com/theRebelliousNerd/codenerd/blob/388205138ba6d47c1cff0c5e97c896dfa642a391/cmd/nerd/chat/model_update.go#L227-L235).

The formatter prints a passed checkmark and score for an assessed pass. It correctly prints “not assessed” for skipped or unknown verdicts, but has no separate coverage section. An explanation may voluntarily mention missing module evidence; the host cannot guarantee that disclosure because it has discarded the typed information. [Formatter](https://github.com/theRebelliousNerd/codenerd/blob/388205138ba6d47c1cff0c5e97c896dfa642a391/cmd/nerd/chat/model_helpers.go#L499-L565).

## Narrow source-derived witness

Assume a normal fresh TUI process with:

- a loaded project vision and a healthy session kernel containing valid declarations, effective purpose and requirements for module `pkg/a`;
- no prior same-workspace campaign-observer construction that wired the registry Guardian;
- an available alignment client and usable JIT prompt;
- a valid workspace-relative bare path `pkg/a/file.go`;
- recent conversational context that does not independently duplicate the relevant module declaration.

The operator runs `/alignment pkg/a/file.go`.

1. Chat boot's Guardian has a parent kernel but no querier.
2. The manual command correctly produces a `paths/manual_target` scope, then uses that same Guardian without binding the reader.
3. Resolution returns unknown module coverage with `module fact snapshot unavailable`, not the available module purpose or requirements.
4. The alignment request still contains the project vision, typed scope, explicit scope issue, subject and recent session context. The omission is deterministic under these prerequisites; the model's eventual verdict is not.
5. If the model returns an admitted project pass—for example a valid score of 0.95 and `RESULT: passed` under default thresholds, with an explanation that does not repeat the coverage issue—the result can display PASSED and a score without a deterministic module-coverage warning.
6. The core check still has unknown coverage and its reason; successful persistence and logging retain them. Nothing in this witness claims an action was authorized, a campaign gate was bypassed, or an unsafe change was accepted.

That conditional pass is compatible with the intended fallback. Existing core regression definitions explicitly exercise both a 0.95 pass and a blocked verdict with an absent querier, preserving each project verdict while recording unknown coverage. The proposed UI repair should communicate that distinction rather than overturn it. [Incomplete-scope contract tests](https://github.com/theRebelliousNerd/codenerd/blob/388205138ba6d47c1cff0c5e97c896dfa642a391/internal/northstar/alignment_scope_test.go#L143-L226), [strict verdict parser](https://github.com/theRebelliousNerd/codenerd/blob/388205138ba6d47c1cff0c5e97c896dfa642a391/internal/northstar/alignment_verdict.go#L35-L107).

The same boot-owned Guardian is attached to the background handler, but this note does not require or assert a live production file-modification event. The directly exposed manual command is sufficient. Likewise, a project-only prose request is not missing requested module coverage.

## Repair contract

### Bind the existing authority at the assembly boundary

Introduce or reuse one narrowly scoped Guardian dependency-binding operation for production consumers. It should distinguish the fact sink from the reader, bind the live session's supported snapshot source before a scoped check, handle nil and typed-nil dependencies, and retain the existing error/unknown behavior for unsupported kernels.

A minimal chat repair can attach the querier during shared boot; manual execution should have an explicit contract for whether it requires that boot-owned binding or defensively binds the current session kernel. Tests must enforce that contract instead of silently relying on a campaign to run first. Do not instantiate a campaign merely to repair the dependency.

Preserve the workspace registry and its acquire/release ownership. Do not reopen per-command SQLite stores or fork Guardian state. If a shared Guardian can be acquired by multiple live session owners, define which session kernel may replace the reader and what happens on that owner's shutdown; a convenience setter should not silently bless a closed or foreign workspace source. This is a design obligation for the proposed binding, not a separately established concurrency incident.

Do not substitute three independent predicate queries for the current snapshot contract, infer requirements from chat prose, copy module facts into a stale parallel cache, or treat a parent-kernel write interface as proof of read capability. Current scope validation, path roles, longest-prefix/nested module selection and rejection of partial malformed evidence remain in force.

### Carry coverage to the operator without changing the verdict

Extend the manual result transport to retain an owned coverage value, with check identity if useful for correlating history and logs. Render two distinct answers:

- **Project alignment:** existing verdict, assessed score, explanation and suggestions.
- **Module coverage:** intentional project-only, resolved scoped coverage, or unavailable/incomplete scoped coverage with a concise reason and a route to full details.

For the witness, an honest display is a project PASSED result followed by “Module coverage unavailable: module fact snapshot unavailable,” not a fabricated module pass and not a forced project failure. For ordinary prose, say that the check was intentionally project-only, not that the system degraded.

Use typed coverage, not reparsing model explanation or querying mutable current facts after completion. The displayed coverage must describe the request that actually produced this check. Preserve unknown/unassessed score handling, blocked verdicts, suggestions, source identities, scope roles and the existing durable record. Treat absent coverage on historical results as “not recorded,” never reconstructed completeness.

No database migration is required merely to transport existing coverage into the current message; the existing table already supports it. Do not rewrite old checks or retroactively assert that their module context was available. A source/display rollout should update the stale wiring description and add a dated continuation to the earlier audit rather than erase its original finding.

## Costs and tradeoffs

- **Real work becomes reachable.** Correct binding means eligible manual path checks will now perform the snapshot and module-context construction they previously skipped. `QueryAll` reads all shard facts, and current module rendering sends full selected requirements. Count evaluation/materialization, allocation, module validation, prompt bytes/tokens, JIT selection, provider latency and rejection. The prior audit already disclosed this broad snapshot and unbounded-request cost; it is not a new performance discovery. [Existing cost limitation](https://github.com/theRebelliousNerd/codenerd/blob/388205138ba6d47c1cff0c5e97c896dfa642a391/Docs/audits/guardian-module-alignment-scope.md#L201-L206).
- **No free model-quality claim.** Supplying local purpose may improve relevance, change decisions or add noise. A complete scope label is not proof of compliance with requirements.
- **Timeouts are not blanket budgets.** Preserve the existing completion context and shutdown ownership. The synchronous snapshot API shown here accepts no context; do not claim the current 60-second completion context strictly bounds every preceding read. A selective/cancelable snapshot design would require its own contract and evaluation.
- **Transport and display cost.** Carrying coverage adds slice/string ownership and rendering work; large module/path/reason lists can make chat noisy. Keep a bounded summary with explicit omitted-count/detail access rather than silently truncating the evidentiary record. No memory or latency measurements were performed.
- **Shared ownership complexity.** A common binder reduces divergent callers but must not overwrite a valid reader with nil, mix workspace identities, or extend a kernel's lifetime without its owner. Avoid holding Guardian locks while making model calls or expensive snapshot reads.
- **Compatibility.** Project-only/manual, no-vision, unavailable-client and provider-failure behavior must remain available. Do not turn this advisory manual UX fix into a new permission gate or automatic retry/model-call loop.

## Fair evaluation plan — not run

1. **Production-boundary regression:** fresh registry, normal chat assembly's Guardian binding, valid module facts and a scoped slash command. Capture the actual final model request and the resulting chat message. Require the module purpose/requirements and `manual_target` role to reach the request; require resolved coverage in both history and UI.
2. **Before/after discriminator:** do not pre-wire the Guardian inside the fixture. The unmodified fresh path should expose unknown snapshot availability despite usable session facts; the repaired path should resolve. The existing direct Guardian fixture already calls `SetQuerier`, so it cannot alone catch this assembly error.
3. **Warm-state control:** construct the campaign observer first in the same workspace and show why the old manual path can already work. Repeat without warm-up to prevent an order-dependent false pass.
4. **Intentional project-only control:** ordinary prose, including an incidental path mention, remains project-only and requires no module snapshot. Do not widen scope from arbitrary prose.
5. **Unavailable evidence:** missing/typed-nil reader, Query-only reader, snapshot error, absent module facts, malformed or conflicting requirements, outside-workspace paths and unresolved globs. Retain project passed/blocked verdicts as appropriate, but display typed unknown coverage and reasons.
6. **Assessment availability:** no vision remains skipped; no client, prompt compilation failure, completion failure and invalid verdict remain unassessed unknown. Score display must not confuse an unassessed check with a measured zero.
7. **Identity and freshness:** change a module requirement between checks, switch workspace, and close an owner. Verify no stale/foreign reader is adopted and that request, returned coverage, durable record and UI refer to the same check. Preserve current source-digest semantics.
8. **Large inputs and costs:** vary shard fact count, selected modules and requirement volume; report complete end-to-end latency, allocations, query work, prompt tokens, provider rejection and UI size. Compare against both the fresh incomplete baseline and a correctly wired campaign-warmed baseline; the latter separates binding correctness from snapshot work already paid elsewhere.
9. **Quality, if later authorized:** use fixed model/settings and independent expected judgments over globally aligned/local-conflict, globally conflicting/local-refinement, and benign cases. Report false positives, false negatives, unknowns and coverage-disclosure correctness. More blocking is not itself better alignment.

Existing campaign querier-wiring tests and core unknown-coverage tests are useful controls, not missing work to recreate wholesale. What is required is coverage of the **actual chat assembly → manual check → message → display** path. [Campaign binding tests](https://github.com/theRebelliousNerd/codenerd/blob/388205138ba6d47c1cff0c5e97c896dfa642a391/internal/northstar/guardian_querier_wiring_test.go#L36-L111), [core prompt difference](https://github.com/theRebelliousNerd/codenerd/blob/388205138ba6d47c1cff0c5e97c896dfa642a391/internal/northstar/guardian_querier_wiring_test.go#L115-L166).

**Completion criterion:** a fresh scoped manual request uses the available session-owned module snapshot without campaign warm-up, or explicitly reports why it could not; the operator sees that coverage outcome alongside, and independently of, the preserved project verdict. No implementation or executable verification is claimed in this note.

