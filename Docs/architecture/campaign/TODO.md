# TODO — campaign architecture / engineering backlog

> Last verified: **2026-08-15**  
> Ordered by dependency, not calendar estimates.

## P0 — Safety & correctness

- [x] Audit every production `NewOrchestrator` call site sets non-nil `TaskExecutor`  
      *Audit result: all 5 sites write the field, but 3 write a value that can be
      nil at runtime (`m.taskExecutor` in chat ×2, `s.TaskExecutor()` in the
      campaign runner shard). `validateOrchestratorConfig` now REQUIRES a usable
      TaskExecutor — the old `TaskExecutor OR ShardManager` guard was a lie,
      since ShardManager is monitoring-only. Typed nils are rejected via
      reflection. Guard: `TestNewOrchestrator_WhenTaskExecutorNil_ShouldRejectConfig`,
      `TestSetTaskExecutor_WhenNil_ShouldKeepExistingExecutor`, plus the existing
      source-level sweep.*
- [x] Surface risk preflight blocks in CLI/chat UX (not only CategoryCampaign logs)  
      *`Run` returns `*RiskBlockedError` carrying the whole `RiskGateEvaluation`;
      `campaign.FormatRiskBlock` renders gate-by-gate. Wired into
      `campaignOutcome` (Cobra) and the chat `campaignErrorMsg` branch. Advisory
      findings are emitted as `risk_gate_advisory` events on the channel the UIs
      already read, and `LastRiskEvaluation()` exposes them after a successful start.*
- [x] Golden tests for `ToFacts` predicate/arity stability  
      *`testdata/tofacts_predicates.golden` pins predicate/arity/argument-kind.
      A second test cross-checks every emitted predicate against its `Decl` in
      `internal/core/defaults`. It immediately found three predicates asserted
      with no Decl at all (`task_soft_dependency`, `requires_resource`,
      `task_sub_campaign` — now declared) and a confidence emitted as float64
      into a `/number` slot (now scaled to an int, as `campaign_metadata` does).*
- [x] Confirm checkpoint-fail never completes phase (regression test)  
      *`TestRunPhase_WhenCheckpointFails_ShouldNotCompletePhase` plus the
      passing and exhausted cases. The bounded escape hatch after
      `maxPhaseCheckpointAttempts` is covered too, including the requirement
      that failed checkpoint records survive so "completed" never reads as
      "verified".*

## P1 — Wiring completeness

- [x] Default-wire IntelligenceGatherer when Cortex boot has world/git/MCP  
      *`defaultWireIntelligence` builds `IntelligenceGatherer` and
      `EdgeCaseDetector` from the two ingredients a Cortex boot always has (a
      `*core.RealKernel` and a workspace). Explicit config still wins. Missing
      them was invisible: the campaign ran, just unplanned-for and — because
      gate wiring keys off availability — with the edge risk gate off.*
- [x] Define hard vs soft advisory blocking contract; implement hard path  
      *Contract in `campaign_rules.mg` §13 (see 09-MANGLE-SURFACE). Go measures
      the preflight and asserts it; the kernel derives `campaign_risk_block`
      (hard) and `campaign_risk_warning` (soft); Go enforces only what the
      kernel derived. Protected surfaces, northstar alignment, a critical
      advisor REJECT, force-block, and gated-with-safety-signals are hard;
      everything else is advisory. A Go mirror covers the case where
      `campaign_rules.mg` is not loaded, and a test proves the two agree.*
- [x] Document or implement Cobra assault configuration parity with chat  
      *Implemented: `nerd campaign assault [scope]` with flags for every
      `AssaultConfig` field, `--dry-run`, and reuse of the exact `campaign start`
      boot via a consumed-once plan override.
      `TestCampaignAssaultFlags_CoverEveryAssaultConfigField` fails if a new
      config field ships without a flag.*
- [x] Nested `campaign_ref` e2e for propagate/absorb/transform policies  
      *`campaign_ref_e2e_test.go` drives the policies through `runPhase` with a
      real kernel (so eligibility comes from the Mangle derivation), and also
      pins the returned envelope, the unset-policy default, the missing-target
      error and the live-lifecycle case.*

## P2 — JIT & maintainability

- [x] Migrate high-traffic roles from `prompts.go` into `internal/prompt/atoms/`  
      *All seven roles are served by `internal/prompt/atoms/campaign/*`.
      `CampaignRoleAtomFamily` makes the mapping explicit in code and
      `TestCampaignRoles_HaveAtomCoverage` fails if a role loses its atoms.*
- [x] Keep `StaticPromptProvider` as thin fallback only  
      *Documented as last-resort and now audible: it logs a warning naming the
      atom family it stood in for, because a campaign planned from the frozen
      prompt is otherwise indistinguishable from one planned properly.*
- [x] Refresh `internal/campaign/README.md` modular map + date (code package doc)  
      *Rewritten 2026-08-15: full module map, required wiring, the hard/soft
      risk table, durability + journal commands, assault CLI, prompt path,
      observability.*

## P3 — Durability ops

- [x] Journal verify/replay operator command  
      *`journal_ops.go` (`VerifyCampaignJournal`, `ReplayCampaignJournal`,
      `ListCampaignJournals`) plus `nerd campaign journal verify|replay`.
      Verification is read-only by design — `recoverJournalSequence` already
      owns truncation, and a second repair policy would diverge from it. Verify
      exits non-zero on defects so it works as a CI gate.*
- [x] Chaos test: kill during snapshot rename  
      *`TestSaveCampaign_WhenKilledDuringSnapshotRename_ShouldLeavePreviousSnapshotIntact`.
      It found a real defect: `renameAtomicReplace` removed the committed
      snapshot before retrying, so a kill at that instant left the campaign with
      no snapshot at all. It now moves the old file aside and restores it if the
      replacement does not land.*
- [x] Assault summary export (aggregate results → single report file)  
      *`assault_report.go` + `nerd campaign report`: per-stage and per-target
      aggregates, worst offenders first, failure samples with log paths, triage
      summary, written as `summary.md` and `summary.json`. Partial runs are
      flagged, because "0 failures" from batches that never executed reads
      exactly like a clean sweep.*

## P4 — Observability

- [x] Closed enum or constants file for OrchestratorEvent.Type strings  
      *`orchestrator_events.go`, with `TestOrchestratorEventTypes_AreClosedSet`
      reading the emit sites via AST so a bare literal cannot ship.*
- [x] Optional metrics hooks (task duration histograms) without coupling to one backend  
      *`MetricsSink`: four methods, primitive arguments, nil = inert.
      `InMemoryMetrics` included for tests and summaries. Wired at task
      completion/failure, phase completion, each checkpoint, and risk preflight.*

## Docs (this corpus)

- [x] Full rebuild 2026-07-13 under `Docs/architecture/campaign/`
- [x] Re-verify line counts after large refactors *(counted 2026-08-15; see 02-CURRENT-STATE)*
- [x] Cross-link from CLI corpus assault section when CLI docs change

## Open follow-ups

- `internal/shards/system/campaign_runner.go` now fails construction when its
  executor was never set (correct fail-closed behaviour) but retries on a timer.
  A one-shot "campaign supervision disabled: no task executor" log would be
  kinder. Owned by `internal/shards`, not this package.
- `!pred(X, _)` does not exclude matching rows in this Mangle build; §13 works
  around it with a fully-bound helper. Other `.mg` files use the same pattern
  (e.g. `campaign_rules.mg` §7 `campaign_task_shard_override`) and may be
  silently deriving too much.

## Explicit non-goals

- Per-file assault tasks for huge repos  
- Replacing kernel `permitted` with campaign-local ACL  
- Client-app-specific campaign types in this package  
