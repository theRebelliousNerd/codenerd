# Research browser ownership and evidence retirement

**Source review and capability proposal for codeNERD**  
**Reviewed:** 2026-10-28  
**Pinned main:** `356f65e0e6bb639adaa90ab36212daa714114871`  
**Status (original review):** Proposal only. No repository edits, implementation, builds, tests, or live Chrome runs were performed in the review. **Status now: built 2026-10-28; see "As-built status" below.**

## As-built status (2026-10-28)

Worktree `codenerd-brief`, branch `feat/conversation-brief`. Every finding below was re-checked against the code at that
commit (pre-fix lines are `git show HEAD:<path>`); each built item has a test aimed at the defect. Only the ownership witness was mutation-checked (see Tests). Chrome
(`/usr/bin/google-chrome`) was available, so the live tests ran rather than skipped.

### Findings: verdicts

| Finding | Verdict | Evidence (pre-fix) |
|---|---|---|
| Default headless Chrome is outside Cortex cleanup | **Real, fixed** | `researchHeadless = mgr` (`browser_read.go:149`); `Cortex.Close` only did `ClearBrowserManager` + manager shutdown (`cortex_close.go:153`). |
| Sink consults a mutable global kernel per batch; read pins a different kernel | **Real, fixed** | `researchKernelSink.AddFacts` called `getBrowserKernel()` per batch (`browser_read.go:68-71`); the read captured another kernel at `:281`. |
| Headless manager has no fact retractor | **Real, fixed** | `NewSessionManagerWithSink(researchHeadlessConfig(), researchKernelSink{})` with no `SetFactRetractor` (`browser_read.go:145`); only the interactive manager got one (`factory.go:1988`). |
| DOM watcher asserts directly, ignores errors, is not joined | **Real, fixed** | `_ = kernel.Assert(...)` (`browser_read.go:352`); `go wait()` with no join. |
| `browser_epoch` / `browser_stream_saturated` outside the retirement set | **Real, fixed** | published through `addFacts` (`fact_epoch.go:137`, `:202`), never added to `budget.tracked` (`:213`). |
| Headless config does not inherit launch / reaper / redaction keys / limits; error names an unconsumed setting | **Real, fixed** | `researchHeadlessConfig()` took no operator input; the factory did not copy `browser.reaper` into even the interactive manager (`factory.go:1956`). |
| `CloseSession` error discarded | **Real, fixed** | `_ = mgr.CloseSession(...)` (`browser_read.go:199`). |
| Retraction key is the pre-sanitization copy | **Real, fixed, wider than the doc said** | `budget.tracked = append(budget.tracked, facts...)` kept raw facts while `addFacts` stored sanitized ones (`fact_epoch.go:213`); the progressive-observe snapshot (`progressive_observe.go`, `replaceProgressiveControlSnapshot`) had the same mismatch. A fact whose string the redactor rewrote (a URL with `?token=`) could never be retracted. |
| `researchPageBrowser` / `shutdownResearchBrowser` are test seams | **Correct; unchanged** | `test_seams_test.go`. They now read the runtime instead of a global; `TestMain` closes the shared test runtime. |
| Per-epoch budget is not a runtime ceiling | **Correct; left** | See "Deliberately left". |

### What was built

**Ownership (research package).** `BrowserRuntime` (`internal/tools/research/browser_runtime.go:46`) is the single owner of
the bound interactive manager, the kernel, and the lazily started headless manager. `SetBrowserRuntime(mgr, kernel,
opts...)` (`:74`) builds and binds it and returns it to the caller. The old `researchHeadless` global, `browserMgr` /
`browserKernel` / `browserKernelOwner` trio and `researchKernelSink` are gone; the process binding is one
`*BrowserRuntime`, so manager and kernel can no longer be read from different binds.
- `admit` (`:169`) leases the runtime per read: it refuses when the runtime is closed or has no kernel, counts the read,
  and derives the read's context so `Close` cancels it. `headlessManager` (`:204`) launches Chrome once with a sink and
  retractor fixed to the runtime's kernel (`runtimeKernelSink`, `:282`) and shuts down a launch that raced `Close`.
- `Close` (`:238`) closes admission, unbinds (compare-and-clear, so a replacement is never touched), cancels and joins
  admitted reads, then shuts down the headless Chrome. A context that expires first still shuts Chrome down and reports
  the stragglers. It is idempotent.
- `Cortex` owns the runtime: `initAutopoiesisAndBrowser` creates it (`factory.go:1996`), `Cortex.Close` closes it before the
  interactive manager (`cortex_close.go:154`), and boot rollback already runs `Cortex.Close`, so it uses the same path.

**Effective configuration.** The factory passes `browser.launch`, `browser.reaper`, `browser.extra_sensitive_keys`,
`browser.max_tabs`, `browser.max_browsers` (`BrowserRuntimeOptions`). `researchLaunchArgv` (`:305`) uses the operator's binary
and flags, dropping and logging `--user-data-dir` and `--remote-debugging-*` (research never runs in a personal
profile or attaches to a debugger); with no `browser.launch` it discovers Chrome as before. Headless, isolated tabs,
header ingestion off, evidence recording off, no session store and no debugger URL are unchanged. The missing-Chrome message
("set browser.launch") is now true. The factory also now copies `browser.reaper` into the interactive manager's config,
which it previously ignored (the `GAP-BROWSER-06` "config adapters" dependency).

**Read lifetime (`browser_read.go`).** `readBrowserPage` (`:119`) runs entirely on its lease. The DOM sensor
(`watchResearchDOM`, `:326`) publishes `dom_updated` through the new `SessionManager.PublishSessionFacts`, so its facts are
sanitized, budgeted, and tracked; `stop()` cancels and joins the listener before the tab closes. A sensor that could not
install or publish, or a stream that dropped facts (saturated budget), makes the read fail as observation-incomplete
(`incomplete`, `:305`) instead of certifying a quiet page. Tab close runs after the join; its error becomes
`browserPage.CleanupErr` beside the intact content (`web_fetch` and `web_search` append a "cleanup pending" notice; Context7
logs it because its body is archived source text) or is joined to the read's own error. No second navigation is triggered.

**Evidence retirement (`internal/browser`).** Tracking now keeps the sanitized facts the sink stored (stream path and
progressive snapshots). `browser_epoch` and `browser_stream_saturated` go through `publishMarker` and retire with their
epoch. A saturated epoch increments `SessionFactStats.SaturatedEpochs` before its notice is retracted (the bounded
receipt); `SessionFactStats.Saturated` reports the live epoch. `forgetSessionBudget` returns the retraction error;
`CloseSession`, `CloseBrowser`, `shutdown` and the stale-session path report it, and a failed batch stays queued as exact
facts (`PendingRetirements`) and is retried at the next retirement and at shutdown. Nothing is widened to a predicate or
session sweep.

### Tests

`internal/browser/fact_retirement_test.go`: `TestRetirement_WhenFactIsRedacted_ShouldRetractStoredSpelling`,
`TestRetirement_WhenProgressiveSnapshotRedacted_ShouldRetractStoredSpelling`,
`TestRetirement_WhenSessionCloses_ShouldRetireLifecycleMarkers`,
`TestRollSessionEpoch_WhenEpochSaturated_ShouldKeepIncompletenessReceipt`,
`TestCloseSession_WhenRetractionFails_ShouldReportAndRetryExactFacts`,
`TestPublishSessionFacts_ShouldBeBudgetedTrackedAndRejectClosedSession`; `internal/browser/kernel_bridge_test.go`
(`TestRollSessionEpoch_WhenRetractorWired_ShouldCollectRetiredFacts`) now also pins that the second roll retires the
epoch-2 watermark and keeps the new one.

`internal/tools/research/browser_runtime_test.go`: `TestBrowserRuntime_Close_RefusesNewReadsAndNeverRelaunches`,
`TestBrowserRuntime_Close_CancelsThenWaitsForAdmittedReads`, `TestBrowserRuntime_Close_BoundedByContextReportsStragglers`,
`TestBrowserRuntime_Close_LeavesReplacementUsable`, `TestResearchLease_PinsManagerAndKernelAcrossRebind`,
`TestRuntimeKernelSink_WritesToItsOwnKernelAfterRebind`, `TestResearchLaunchArgv_ConfiguredLaunchWinsMinusOwnedProfileFlags`,
`TestResearchLaunchArgv_UnsetLaunchDiscoversBinary`, `TestResearchHeadlessConfig_InheritsOperatorSettingsAndKeepsNarrowing`,
`TestResearchDOMWatch_IncompleteObservationIsNotStable`, and live Chrome:
`TestReadBrowserPage_RetiresEveryTransientFactWithTheTab`, `TestReadBrowserPage_CleanupFailureKeepsContentAndReportsPending`,
`TestBrowserRuntime_Close_EndsInFlightReadAndChrome` (a blocked read ends; Chrome stops accepting connections; no facts remain),
`TestReadBrowserPage_StaysOnItsKernelWhenRuntimeRebinds`. The existing `browser_read_test.go` suite was kept and moved onto
one shared test runtime.

`internal/system/factory_research_browser_test.go`: `TestCortexClose_EndsResearchHeadlessChrome` (production boot, a real
`web_fetch`, then `Cortex.Close`; the witness is the absence of any process whose command line carries the private
`TMPDIR` profile root) and `TestBootRollback_EndsResearchHeadlessChrome` (same, through a failing late boot step). With the
`researchBrowser` close step disabled both fail with the Chrome process tree still alive, so the witness is not blind.

### Deliberately left

- **Multi-runtime admission.** The process still has one binding; a second `SetBrowserRuntime` replaces it. Reads already
  admitted stay pinned to the old runtime and drain on its `Close`; new reads use the replacement. Simultaneous runtimes
  sharing one process binding are not advertised, per the doc's own instruction not to claim isolation before evaluating it.
  Making the executor inject the caller's runtime needs a change to the tool-execution signature outside this package.
- **Interactive browser tools** (`browser_navigate`, reasoning, specs, ...) still call `getBrowserManager()` and
  `getBrowserKernel()` separately. Both now come from the same binding, but a rebind between the two calls can still pair
  them wrongly. They are not research page reads and have no lease; reasoning tools that need a pair should take one
  snapshot. Not done here to keep the change to the read path.
- **Cumulative retention ceiling.** Exact retirement bounds a closed read's footprint. An open interactive session still
  accumulates up to the per-epoch event budget per epoch; a whole-runtime ceiling is a policy decision, not a defect fix.
- **`browser.navigation_timeout_ms`** is not inherited: research reads are bounded by `research.web_fetch_timeout` and friends
  through the request context.
- **Retirement of a read's receipt.** No new persistent per-read receipt store was added. The terminal state is the return
  value (`browserPage`, `CleanupErr`, the typed incompleteness error); the durable content cache already owns recoverable
  content, and the saturation receipt is `SaturatedEpochs`.
- **Quantitative measurements** (long-session fact growth plateau, PID tree accounting across many reads) were not run; the
  tests assert zero residual facts after a closed read and process exit after `Close`.

---

## Original review (2026-10-28, pre-implementation)

## Conclusion

codeNERD already has browser-backed research. The unfinished capability is **a research read whose browser process, active operations, kernel evidence, and cleanup result have one runtime owner from admission through retirement**.

The default research path creates a separate headless `SessionManager` in a process global. The normal Cortex close path clears and shuts down the boot-bound manager, not that headless manager. The headless event sink looks up whichever global kernel is currently bound for every batch. It has no fact retractor, while a separate DOM watcher writes directly into a captured kernel and bypasses the manager's epoch accounting. A read defers tab close but ignores its error. These are connected ownership gaps, not a reason to delete research, restore test helpers to production, replace the browser with HTTP, or introduce another reasoning engine.

The useful next step is a spec-first owner-and-retirement contract, implemented through the existing browser manager and live kernel. Keep lazy headless startup and isolated tabs. Make their ownership, effective configuration, event scope, and terminal cleanup explicit. Preserve a compact research receipt while retiring transient browser observations.

## Scope and prior work

This review follows the root instruction to inspect wiring before deleting unused code and the system instruction that Cortex owns browser resources. Both are current source guidance: [root guidance](https://github.com/theRebelliousNerd/codenerd/blob/356f65e0e6bb639adaa90ab36212daa714114871/agents.md), [system guidance](https://github.com/theRebelliousNerd/codenerd/blob/356f65e0e6bb639adaa90ab36212daa714114871/internal/system/agents.md), and [browser guidance](https://github.com/theRebelliousNerd/codenerd/blob/356f65e0e6bb639adaa90ab36212daa714114871/internal/browser/agents.md).

The flagged `researchPageBrowser` and `shutdownResearchBrowser` functions live in `test_seams_test.go`, which explicitly labels them conveniences called only by tests. The first inspects an existing manager; the second removes and shuts down the headless singleton. Their classification as test seams is correct. Their placement does not demonstrate production ownership. [Test seam source](https://github.com/theRebelliousNerd/codenerd/blob/356f65e0e6bb639adaa90ab36212daa714114871/internal/tools/research/test_seams_test.go#L10-L32).

The existing [orientation disclosure audit](https://github.com/theRebelliousNerd/codenerd/blob/356f65e0e6bb639adaa90ab36212daa714114871/Docs/audits/orientation-treatment-disclosure-contract.md) already classifies those helpers and explicitly leaves browser lifecycle correctness unproved. It does not supply this production lifecycle/evidence-retirement contract. The [research-read spec](https://github.com/theRebelliousNerd/codenerd/blob/356f65e0e6bb639adaa90ab36212daa714114871/Docs/architecture/tools/13-RESEARCH-READS-THROUGH-BROWSER.md) already records that custom launch configuration does not reach headless research. The [browser process-lifecycle spec](https://github.com/theRebelliousNerd/codenerd/blob/356f65e0e6bb639adaa90ab36212daa714114871/Docs/architecture/browser/16-PROCESS-LIFECYCLE.md) already records missing reaper configuration adapters. Those omissions are dependencies here, not newly discovered features.

The older tools [open question Q6](https://github.com/theRebelliousNerd/codenerd/blob/356f65e0e6bb639adaa90ab36212daa714114871/Docs/architecture/tools/OPEN-QUESTIONS.md) mentions process-global browser ownership, but it is dated July and describes the earlier package-Once design. It is historical context, not evidence of today's execution. Current source establishes the narrower, still-live headless ownership gap below.

This proposal does not reopen orientation treatment, generic task completion, general context retrieval, browser feature parity, or broad research quality. It concerns the production resources and transient evidence created by browser-backed page reads.

## What already runs

The existing path has valuable behavior that a repair must preserve.

1. Factory boot builds a browser manager from much of the operator configuration, binds its event sink to the boot kernel, and installs live query and exact-fact retraction adapters. It then calls `research.SetBrowserRuntime`. [Factory wiring](https://github.com/theRebelliousNerd/codenerd/blob/356f65e0e6bb639adaa90ab36212daa714114871/internal/system/factory.go#L1932-L1993).
2. Research refuses when the bound manager or kernel is absent. `research.browser_headless=false` uses the bound manager; the default headless path uses the separate lazy manager. [Research runtime selection](https://github.com/theRebelliousNerd/codenerd/blob/356f65e0e6bb639adaa90ab36212daa714114871/internal/tools/research/browser_read.go#L139-L171).
3. A read opens an isolated about-blank tab, navigates, waits for load and DOM/network quiet, extracts rendered HTML/text, and defers tab close. `web_fetch`, `web_search`, and `context7_fetch` call this path. Missing runtime, launch, navigation, status, and stability failures are not silently replaced by a direct HTTP page read. [Read implementation](https://github.com/theRebelliousNerd/codenerd/blob/356f65e0e6bb639adaa90ab36212daa714114871/internal/tools/research/browser_read.go#L180-L326), [web fetch](https://github.com/theRebelliousNerd/codenerd/blob/356f65e0e6bb639adaa90ab36212daa714114871/internal/tools/research/web_fetch.go), [web search](https://github.com/theRebelliousNerd/codenerd/blob/356f65e0e6bb639adaa90ab36212daa714114871/internal/tools/research/web_search.go), [Context7](https://github.com/theRebelliousNerd/codenerd/blob/356f65e0e6bb639adaa90ab36212daa714114871/internal/tools/research/context7.go).
4. `SessionManager` already owns connection lifetime independently of a caller's short request context; has process/profile cleanup; closes and drains its session stream before retirement; and supports exact fact retirement when an owner installs a retractor. This is infrastructure to wire, not a missing browser engine. [Connection and launch lifecycle](https://github.com/theRebelliousNerd/codenerd/blob/356f65e0e6bb639adaa90ab36212daa714114871/internal/browser/session_lifecycle.go#L155-L250), [close and shutdown](https://github.com/theRebelliousNerd/codenerd/blob/356f65e0e6bb639adaa90ab36212daa714114871/internal/browser/session_lifecycle.go#L543-L696), [fact retirement](https://github.com/theRebelliousNerd/codenerd/blob/356f65e0e6bb639adaa90ab36212daa714114871/internal/browser/fact_epoch.go).

This is source-confirmed wiring, not a claim that the live behavior passed evaluation in this review.

## Source findings

### The default research process is outside Cortex cleanup ownership

**Source fact.** `researchHeadlessManager` stores a successful launch in package-global `researchHeadless` and returns it on future calls. `SetBrowserRuntime` and `ClearBrowserManager` modify only `browserMgr`, `browserKernel`, and `browserKernelOwner`. `Cortex.Close` calls `ClearBrowserManager(c.BrowserManager)` and shuts down that manager. The headless singleton is not transferred into Cortex ownership or closed there. The only assignment clearing `researchHeadless` found in the reviewed source/search is the test helper. [Headless construction](https://github.com/theRebelliousNerd/codenerd/blob/356f65e0e6bb639adaa90ab36212daa714114871/internal/tools/research/browser_read.go#L85-L150), [binding and clearing](https://github.com/theRebelliousNerd/codenerd/blob/356f65e0e6bb639adaa90ab36212daa714114871/internal/tools/research/browser.go#L16-L70), [Cortex close](https://github.com/theRebelliousNerd/codenerd/blob/356f65e0e6bb639adaa90ab36212daa714114871/internal/system/cortex_close.go#L151-L162).

**Inference.** Once a default headless research read has launched Chrome, successful closure of the owning Cortex does not establish termination of that research process or cleanup of its owned profile. This is material in a long-lived process, repeated boot/close flows, and ordinary one-shot exit. No OS process survival or profile leak was measured here. Browser reaping at a later startup is a recovery mechanism, not evidence that the original close completed.

### A research read does not retain one immutable runtime binding

**Source fact.** `researchKernelSink.AddFacts` calls `getBrowserKernel()` for every event batch and returns nil when no kernel is bound. The comment expressly says the sink outlives an individual runtime binding. `readBrowserPage` obtains its manager at entry but obtains its kernel later, after navigation/status handling. Its separate DOM watcher captures that later kernel, while the manager's sink continues resolving the current global kernel. [Sink and binding](https://github.com/theRebelliousNerd/codenerd/blob/356f65e0e6bb639adaa90ab36212daa714114871/internal/tools/research/browser_read.go#L65-L82), [read kernel capture and watcher](https://github.com/theRebelliousNerd/codenerd/blob/356f65e0e6bb639adaa90ab36212daa714114871/internal/tools/research/browser_read.go#L280-L296).

**Inference.** A runtime replacement during an in-flight read can redirect subsequent manager facts to the replacement kernel while the watcher and stability query retain the earlier kernel. Clearing the binding can instead drop manager facts as apparent success. Mutexes protect individual global accesses; they do not preserve a read's owner across those accesses. Existing compare-and-clear behavior correctly prevents an older manager from clearing a newer binding, but does not pin ongoing research operations to either owner. This is a possible source-derived interleaving, not an observed cross-workspace incident.

### Closing a research tab does not retire all of its transient facts

**Source fact.** The headless manager is created with `researchKernelSink{}` and is not given `SetFactRetractor`. Its constructor infers a query adapter only if the sink implements `FactQuerier`; it does not infer a retractor. In `fact_epoch.go`, stream facts are retained for exact retirement only when a retractor exists. Without that adapter, tab close drops accounting rather than retracting the observations already asserted to the kernel. The bound interactive manager receives the adapter in factory boot, but the separate research manager does not. [Headless construction](https://github.com/theRebelliousNerd/codenerd/blob/356f65e0e6bb639adaa90ab36212daa714114871/internal/tools/research/browser_read.go#L139-L150), [manager construction](https://github.com/theRebelliousNerd/codenerd/blob/356f65e0e6bb639adaa90ab36212daa714114871/internal/browser/session_manager.go#L399-L435), [tracking and retirement](https://github.com/theRebelliousNerd/codenerd/blob/356f65e0e6bb639adaa90ab36212daa714114871/internal/browser/fact_epoch.go#L21-L49), [close and stream accounting](https://github.com/theRebelliousNerd/codenerd/blob/356f65e0e6bb639adaa90ab36212daa714114871/internal/browser/fact_epoch.go#L149-L217).

**Additional source fact.** `watchResearchDOM` writes `dom_updated` using `kernel.Assert` directly, ignores its error, starts a separate event goroutine, and returns cancellation without a completion join. Those facts are outside `addStreamFacts`, its per-epoch accounting, and its tracked retirement set. Simply adding a retractor to the headless manager therefore does not solve the complete read-lifetime problem. [DOM watcher](https://github.com/theRebelliousNerd/codenerd/blob/356f65e0e6bb639adaa90ab36212daa714114871/internal/tools/research/browser_read.go#L328-L390).

**Additional source fact.** The stream-event budget is per session **and per epoch**, not a lifetime ceiling for that session: `RollSessionEpoch` increments the epoch and resets asserted/dropped/saturation accounting when navigation rolls the epoch. `browser_epoch` watermarks and `browser_stream_saturated` notices are published through `addFacts` directly, outside the `budget.tracked` retirement set. The saturation notice deliberately bypasses the exhausted event budget so evidence of incompleteness survives. Its live purpose is valid, but its eventual retention/retirement still needs an owner; installing a retractor alone does not collect these direct-publication facts. [Epoch reset and watermark](https://github.com/theRebelliousNerd/codenerd/blob/356f65e0e6bb639adaa90ab36212daa714114871/internal/browser/fact_epoch.go#L103-L144), [saturation notice and tracked stream path](https://github.com/theRebelliousNerd/codenerd/blob/356f65e0e6bb639adaa90ab36212daa714114871/internal/browser/fact_epoch.go#L175-L217).

**Inference.** A long series of individually bounded, successfully closed reads can leave cumulative transient browser observations in the live kernel. A per-session, per-epoch event budget that resets on navigation does not impose a whole-runtime retention ceiling; new reads also create new sessions. Retention magnitude, eventual kernel impact, and user-visible failure rates remain unmeasured. Lost sensor/assertion evidence can also make an apparent quiet period less trustworthy; evaluation must distinguish a quiet document from a broken observer rather than assume they are equivalent.

### The headless manager has different effective configuration

**Source fact.** `researchHeadlessConfig` starts from browser defaults, forces headless/DOM ingestion, disables header ingestion and evidence recording, clears workspace/session-store fields, and discovers Chrome independently. It does not receive the bound manager's custom launch, reaper, tab/browser limits, or extra sensitive-key configuration. Some differences are deliberate and appropriate; others are pre-existing documented wiring gaps. [Headless config](https://github.com/theRebelliousNerd/codenerd/blob/356f65e0e6bb639adaa90ab36212daa714114871/internal/tools/research/browser_read.go#L94-L137), [factory config mapping](https://github.com/theRebelliousNerd/codenerd/blob/356f65e0e6bb639adaa90ab36212daa714114871/internal/system/factory.go#L1954-L1980).

**Inference.** The configured browser can be usable while default research fails to find Chrome; configured resource and privacy treatment can differ between interactive browsing and research. A missing-Chrome error recommends `browser.launch`, although the headless path does not consume that setting. This does not prove secret leakage or resource exhaustion. It proves that the operator's effective research contract needs an explicit mapping rather than an implicit copy of unrelated defaults.

### A successful content read can hide cleanup failure

**Source fact.** `readBrowserPage` discards the error from deferred `CloseSession`. `CloseSession` removes the session record before attempting to close resources, then returns the close error. The production reader does not surface that error. [Read defer](https://github.com/theRebelliousNerd/codenerd/blob/356f65e0e6bb639adaa90ab36212daa714114871/internal/tools/research/browser_read.go#L191-L200), [session close](https://github.com/theRebelliousNerd/codenerd/blob/356f65e0e6bb639adaa90ab36212daa714114871/internal/browser/session_lifecycle.go#L543-L584).

**Inference.** Manager-list cardinality returning to baseline is weaker evidence than actual target/context closure. A read can return useful content while cleanup failed, but there is no structured distinction at this boundary. The future contract should retain that distinction rather than either discard valid content or pretend cleanup succeeded.

## Proposed capability contract

These are proposed requirements, not descriptions of current behavior.

### One owner and one binding per admitted read

Introduce an explicit research-browser runtime resource owned by the Cortex boot/rollback/close transaction. It may hold an interactive manager and a separate lazily started headless manager. It must not create a second Mangle engine.

Each read acquires a lease on one immutable tuple: runtime generation, manager, kernel, effective configuration, and cancellation lifetime. Its manager event sink, DOM sensor, stability query, and retirement writer all use that tuple. A global pointer may remain a compatibility discovery mechanism, but not a mutable dependency consulted mid-read. Either reject unsupported simultaneous runtime admission explicitly or inject the caller's runtime through the production execution path; do not advertise multi-runtime isolation before evaluating it.

Shutdown first closes admission. It then cancels or drains admitted reads according to a specified policy, joins their sensors/streams, retires their transient evidence, shuts down owned managers, and reports any remaining cleanup obligation. An old owner must neither detach nor terminate a replacement owner's resources. A new request after admission closes must not relaunch the old manager. Boot failure must use the same ownership path as ordinary close.

### Complete transient evidence retirement with retained research value

Use the existing live-kernel exact-retraction infrastructure, with all read-lifetime event producers included. A direct watcher must either publish through the manager's owned tracked path or register an equivalent exact retirement set with the same owner. Include `browser_epoch` and `browser_stream_saturated` in the lifecycle inventory even though they bypass stream accounting. Preserve a saturation notice while it is needed to judge that epoch's completeness; at retirement, explicitly retain the necessary terminal incompleteness evidence in the bounded receipt and retire obsolete live markers according to the spec. Join producers before final retirement so a late callback cannot republish retired facts.

Audit the exact representation retained for removal: the browser's `addFacts` sanitizes values before handing them to the sink. The removal set must identify the actual asserted, sanitized facts, not assume a pre-sanitization copy compares equal. [Sanitizing publication boundary](https://github.com/theRebelliousNerd/codenerd/blob/356f65e0e6bb639adaa90ab36212daa714114871/internal/browser/fact_redaction.go#L8-L15). This is a design/verification obligation, not a separate measured incident.

Retire continuous sensor observations, not the useful research result. Retain a bounded receipt identifying the read, runtime generation, requested/resolved source where available, terminal fetch/stability state, sensor completeness, content/evidence handle, and cleanup state. The durable evidence store and existing content cache remain responsible for recoverable research content. Do not turn every browser event into permanent knowledge merely to make the result auditable.

If a sensor cannot install or publish, the result is observation-incomplete, not proven stable. If retirement fails, report pending retirement and preserve the owned identity needed for retry; never broaden removal to another read's facts. The kernel should derive any model-facing reuse/completion consequence from typed evidence. Go owns launching, transport, cancellation, and cleanup mechanics.

### Effective configuration and truthful terminal results

Define which operator settings research inherits and which it intentionally narrows. At minimum review Chrome binary/flags, cleanup policy, resource limits, and custom redaction keys. Preserve headless default, isolated tabs, disabled header ingestion, and disabled bulk evidence recording unless a separate authorized requirement changes them. Do not accidentally inherit a personal profile or debugger attachment simply because the interactive browser uses one.

Expose effective configuration provenance without leaking credentials. Missing-Chrome errors must recommend a setting the research path actually consumes. Tab/read cleanup needs an independently represented terminal state: content obtained, cleanup complete; content obtained, cleanup pending/failed; canceled; or read failed. A cleanup error should not force an unnecessary repeat navigation that spends time and creates more resources.

No model-facing process-kill or browser-reaper tool is proposed. Lifecycle cleanup remains infrastructure, consistent with the browser ADR.

## Evaluation plan

The following are proposed acceptance experiments. None ran in this review. They should use ordinary production factory/tool paths as the primary evidence, with focused package checks as supporting diagnostics.

| Scenario | Evidence required | Failure the experiment must catch |
|---|---|---|
| Default headless read followed by Cortex close | Owned Chrome PID/tree exit, allocated temporary profile removal, no active read/sensor, explicit cleanup result | TestMain cleanup hiding a production ownership omission |
| Read cancellation during launch, navigation, stability, and extraction | Caller cancellation honored; all admitted resources accounted; terminal cleanup joined or truthfully pending | Cancellation returning before ownership has been settled |
| Runtime A read paused while B binds | A facts/waits stay with A or A is explicitly canceled; B receives none of A's transient facts | Dynamic sink reassignment and split manager/watcher authority |
| Close A after B binds | B remains usable; A cannot launch again; A resources terminate | Blanket singleton shutdown or stale-owner cleanup damaging the replacement |
| Many sequential dynamic-page reads and navigation epochs | Actual CDP targets/contexts return to baseline; transient fact growth plateaus; residual `browser_epoch`, `browser_stream_saturated`, and direct DOM facts follow explicit retention policy; retained receipts grow only by policy | Manager-list baseline hiding close failure or accumulating closed-session and retired-epoch facts |
| DOM watcher and assertion failure | Sensor incompleteness is surfaced; quiet/stable is not falsely certified | Missing events interpreted as absence of activity |
| Exact retirement after redaction | Published sensitive-key facts are removed using their sanitized representation | Retraction keys no longer matching stored facts |
| Configured nonstandard Chrome and reaper policy | Both research and interactive paths follow declared effective settings; invalid config fails before launch | Misleading launch remediation or silently ignored cleanup limits |
| Forced close/retraction failure | Useful content remains addressable; failure is reported and retry is idempotent | Re-fetching content to repair cleanup or reporting full success |
| Explicit profile and debugger attachment controls | No deletion/termination of unowned resources | Ownership repair broadening cleanup authority |
| Normal success and unstable-page controls | Existing rendered-content, deadline, isolation, and no-HTTP-fallback semantics remain intact | A lifecycle repair weakening research correctness |

Current tests are useful but insufficient for this contract. `browser_read_test.go` binds a sentinel manager and a lightweight reasoning kernel, creates local page fixtures, checks content and manager tab counts, and shuts down the global research browser from `TestMain`. Binding tests cover compare-and-clear semantics without concurrent in-flight research. Preserve these checks, but do not report them as a production Cortex lifecycle proof. [Research test setup](https://github.com/theRebelliousNerd/codenerd/blob/356f65e0e6bb639adaa90ab36212daa714114871/internal/tools/research/browser_read_test.go#L18-L60), [tab count assertion](https://github.com/theRebelliousNerd/codenerd/blob/356f65e0e6bb639adaa90ab36212daa714114871/internal/tools/research/browser_read_test.go#L89-L115), [binding tests](https://github.com/theRebelliousNerd/codenerd/blob/356f65e0e6bb639adaa90ab36212daa714114871/internal/tools/research/browser_binding_test.go#L10-L49).

## Costs and expected value

**No measured speed, token, memory, or dollar savings are claimed.** This is primarily a correctness and capability restoration. It can reduce avoidable retry and long-session degradation, but that benefit needs measurement.

- **Startup and process cost:** Preserve lazy launch and warm reuse within one owner. Per-read Chrome launch is a simple isolation option but would add launch latency and process churn; it is not the recommended default. Multiple genuinely concurrent runtimes may need separate managers and therefore additional Chrome memory, unless a shared pool proves generation-scoped event isolation and reference-counted ownership.
- **Evidence memory:** Exact retirement temporarily retains removable facts. The existing epoch tracker documents that retention cost. Persistent growth should be traded for bounded active-read memory, with explicit limits and incomplete-observation handling. A timer-based event throttle is not a total retention contract.
- **Kernel work:** Publication, exact retirement, and compact terminal receipts add deterministic operations. Measure assertion/retraction cost, stability-query latency, live fact count, and post-read residual facts as workload length increases. Keep raw event churn out of model context.
- **Latency:** Correctly joining cleanup may make completion slower than returning while cleanup is abandoned. Report fetch latency and cleanup latency separately. Do not call a lower return latency an improvement if processes or callbacks remain outstanding.
- **Model tokens:** Successful lifecycle operation should not add a large prompt schema or tool catalog. A compact result/receipt and targeted JIT guidance on actionable incomplete states are sufficient. Measure retry turns and tool-result tokens alongside semantic success; a smaller result that conceals an incomplete read is not an efficiency win.
- **Implementation scope:** The likely seams are research runtime binding/reader, browser fact publication/retirement, factory configuration/ownership, Cortex close, and affected specs. No quantitative engineering estimate is justified without deciding whether multi-runtime operation is supported or explicitly excluded in the first phase.

## Suggested spec work and closure criteria

Extend the existing tools research-read spec and browser process/evidence lifecycle specs rather than create an unrelated subsystem. Update system ownership documentation for lazy resources acquired after boot. Reuse existing launch/reaper adapter gap rows where applicable; add narrowly scoped research-owner and transient-evidence rows only where no existing row covers the behavior.

The capability is complete when a production research invocation can establish all of the following: one pinned runtime authority, complete or explicitly incomplete observation, useful content retained by handle, every active producer joined or truthfully pending, transient evidence retired without touching another owner, and all owned Chrome resources closed on the specified runtime lifecycle edge. A dead-code-budget change, a test-only cleanup hook, or a tab-list baseline alone does not meet that exit.

The most useful implementation order is ownership/admission first, complete event retirement second, effective configuration and result semantics alongside those changes, then production entry-path and long-session evidence. Each step should begin with the package spec changes and end with cited current-state updates. This document authorizes none of those implementation steps; it supplies the source-grounded scope for review.
