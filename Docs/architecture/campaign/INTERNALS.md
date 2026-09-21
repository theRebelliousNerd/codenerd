# campaign internals — how planning, gating, and execution work

**Question answered here:** by what mechanism does a goal become verified phases?
(What the package *is* → [README.md](README.md). What is wired vs not built →
[WIRING-AND-NOT-BUILT.md](WIRING-AND-NOT-BUILT.md).)

**Verified 2026-09-20 against the working tree at `refs/heads/main`.**

## 1. Decompose: goal → plan (LLM proposes, kernel disposes)

`Decomposer.Decompose` (`internal/campaign/decomposer.go:242`) takes a
`DecomposeRequest` (`:211` — goal, source paths, campaign type, hints, budgets)
and runs: intelligence gathering (Step 0, optional injected gatherer) →
source-document ingest → requirement extraction → LLM plan proposal → campaign
build → advisory-board review (Step 4b, optional). Every LLM call prefers
grounded completion with fallback to direct completion
(`internal/campaign/decomposer.go:187`). Empty goals, nil kernels, and
blank source paths are rejected up front (`:243`).

A plan the model leaves empty does not fail: it is synthesised as a generic
three-task scaffold and marked `Campaign.PlanDegraded`
(`internal/campaign/types.go:186`) so resumed runs and the CLI can tell a real
plan from a placeholder.

## 2. Kernel transduction: structs → facts

The kernel never sees Go structs. `Campaign.ToFacts`, `Phase.ToFacts`,
`Task.ToFacts` (`internal/campaign/types.go:496`, `:555`, `:618`) project the
plan into Mangle atoms, and plan updates reload them through `syncCampaignFacts`
(`internal/campaign/campaign_fact_sync.go:10`). Task identity in the fact layer pins
`Task.WriteSet` (`internal/campaign/types.go:308`) — the canonical file paths a
task may mutate — with the `Task.DeterministicWriteSet` helper
(`internal/campaign/types.go:730`).

## 3. Risk preflight: Go measures, kernel decides, Go enforces

`Orchestrator.runRiskPreflight` (`internal/campaign/risk_scoring.go:233`)
measures; the rules in `campaign_rules.mg` §13 grade; Go enforces the verdict.
The contract is recorded where it is enforced
(`internal/campaign/risk_gate_contract.go:14`):

- Protected surfaces — `internal/core`, `internal/mangle`, `internal/campaign`,
  `internal/perception`, `internal/articulation`
  (`internal/campaign/risk_scoring.go:19`) — turn blocked gates into hard stops.
- Gate modes are `/auto` / `/force_allow` / `/force_block`
  (`internal/campaign/risk_scoring.go:28`); per-gate toggles are
  `/auto` / `/enabled` / `/disabled` (`:36`); gates are
  `/northstar` / `/edge` / `/advisory` (`:46`) with outcomes
  `/passed` / `/blocked` / `/skipped` (`:54`).
- Hard blocks return a `*RiskBlockedError` (`:88`) wrapping the sentinel
  `ErrRiskGateBlocked` (`internal/campaign/risk_gate_contract.go:73`,
  surfaced through `Error`/`Unwrap` at `:93`–`:100`);
  `FormatRiskBlock` renders the full gate report for CLI and chat (`:120`).
- If the kernel rules are not loaded, a Go mirror grades the same contract
  rather than treating "no rules" as "no blocks"
  (`internal/campaign/risk_gate_contract.go:303`).

## 4. Checkpoints: verification that fails closed

`CheckpointRunner.Run` (`internal/campaign/checkpoint.go:72`) dispatches on the
phase's `VerificationMethod` (`/tests_pass`, `/builds`, `/manual_review`,
`/shard_validation`, `/nemesis_gauntlet`, `/none` —
`internal/campaign/types.go:137`):

- `runTestsCheckpoint` (`internal/campaign/checkpoint.go:109`) detects the
  project test command, forces JSON output for Go, and treats any non-zero exit
  as failure — including build breakage with zero tests counted.
- `runManualReviewCheckpoint` (`:246`) **escalates**: there is no human in
  non-interactive mode, so it delegates to shard validation and prefixes the
  details to keep the escalation visible. It never reports "reviewed".
- `runShardValidationCheckpoint` (`:260`) and
  `runNemesisGauntletCheckpoint` (`:348`) fail closed with no task executor,
  retract stale `checkpoint_verdict` facts before spawning (so a task shard
  cannot pre-approve its own phase), and accept only a well-formed
  `checkpoint_verdict/4` for the phase — prose is inert.
- Two deliberate passes-by-design: `/none` returns pass (`:90`), and an
  *unknown* method logs and skips (`:93`) — see WIRING-AND-NOT-BUILT.md.

## 5. Replan and rolling-wave refinement

`Replanner.Replan` (`internal/campaign/replan.go:169`) gathers failed/blocked
tasks and triggers, asks the LLM for fixes on a cloned campaign (cycle-checked),
bumps `RevisionNumber`, and reloads kernel facts. `ReplanForNewRequirement`
(`:237`) adds tasks with a per-phase duplicate guard. `RefineNextPhase`
(`:417`) progressively elaborates the upcoming phase. Bounds that stop infinite
loops: `Phase.CheckpointFailures` (`internal/campaign/types.go:260`) caps the
fail→replan→re-check loop, and `Task.ReplannedAtCap` (`:335`) limits
attempt-cap replanning to once per task per campaign.

## 6. Context and durability

`ContextPager` (`internal/campaign/context_pager.go:18`,
`NewContextPager` at `:37`) pages phase context against the token budget with
prefetch and compression. Persistence is snapshot
(`internal/campaign/orchestrator_lifecycle.go:132` — temp file,
checksum-verify, rename with restore-on-failure) plus the append-only journal
(`internal/campaign/orchestrator_journal.go`). Operators verify and replay
through `journal_ops.go` (`VerifyCampaignJournal` at `:208`,
`ReplayCampaignJournal` at `:293`). On completion `Run` sweeps workspace-root
files the campaign created but never declared
(`sweepUndeclaredRootWrites`,
`internal/campaign/orchestrator_execution.go:198`), bounded by the
`Campaign.RootBaseline` snapshot (`internal/campaign/types.go:197`).
