# campaign — Implemented Spec (Deep-Dive)

> Last verified against codebase: **2026-07-13**  
> Status: Living Reference Document  
> Language: Go  
> Primary sources: `internal/campaign/`  
> Policy companions: `internal/core/defaults/campaign_rules.mg`, policy Section 19 (base campaign SM), `build_topology.mg`  
> Scale (approx.): **~45** non-test Go sources, **~29** `*_test.go` files; largest units 500–1200 LOC each  

## 1. Overview

`internal/campaign` implements **long-horizon, multi-phase goal execution** for codeNERD. It is the system’s answer to work that cannot fit in a single OODA turn: greenfield builds, large features, migrations, audits, remediations, and **adversarial assault** stress campaigns.

Architectural posture matches the repo north star:

| Role | Owner in campaigns |
|------|--------------------|
| Creative / synthesis | LLM via `Decomposer`, `Replanner`, optional Gemini grounding/thinking helpers, shard specialists |
| Executive / scheduling | Mangle kernel facts + derived predicates (`current_phase`, `eligible_task`, `next_campaign_task`, `campaign_blocked`, `replan_trigger`, …) |
| Effectful work | `session.TaskExecutor` (JIT path), `tactile.Executor`, VirtualStore scope refresh |
| Durable memory of plan | JSON snapshot + append-only journal under `.nerd/campaigns/` |

Campaigns **do not** replace constitutional safety. Mutating tools and shards still need kernel permission at the action boundary. The orchestrator is a **phase scheduler and durability shell** around that loop.

### 1.1 High-level control flow

```
                    ┌─────────────────────────────────────┐
  Goal + docs  ───► │ Decomposer.Decompose                │
  (or Assault)      │  intel → ingest → reqs → LLM plan   │
                    │  → ToFacts → Mangle validate/refine │
                    └──────────────────┬──────────────────┘
                                       │ Campaign*
                                       ▼
                    ┌─────────────────────────────────────┐
                    │ Orchestrator.SetCampaign / Load      │
                    │  LoadFacts + journal recover + save  │
                    └──────────────────┬──────────────────┘
                                       │ Run(ctx)
                                       ▼
              ┌────────────────────────────────────────────────┐
              │ Main loop (orchestrator_execution.go)          │
              │  risk preflight → resetInProgress              │
              │  heartbeat + autosave goroutine                │
              │  while !done:                                  │
              │    pauseCh wait | getCurrentPhase              │
              │    ActivatePhase (ContextPager)                │
              │    runPhase (bounded parallel tasks)           │
              │    checkpoint → compress → rolling-wave        │
              └────────────────────────────────────────────────┘
```

### 1.2 Operator entry points

| Surface | Path | Behavior |
|---------|------|----------|
| Cobra | `cmd/nerd/cmd_campaign.go` | `campaign start/status/pause/resume/list` |
| JIT prompts | `cmd/nerd/campaign_jit_provider.go` | `CampaignJITProvider` → `articulation.PromptAssembler` |
| Chat / TUI | chat campaign handlers + `cmd/nerd/ui/campaign_page.go` | Progress UI; `/campaign assault …` |
| E2E | `tests/e2e/campaign_session_integration_test.go` | Session integration |
| Package README | `internal/campaign/README.md` | Operator-oriented overview (dated; prefer this corpus) |

---

## 2. Implementation status

| Component | Status | Notes |
|-----------|--------|-------|
| Domain model (`Campaign`/`Phase`/`Task`) | **Implemented** | `types.go`; `ToFacts()` for kernel load |
| Orchestrator modular core | **Implemented** | Split across `orchestrator_*.go` |
| Decomposer (LLM + validate) | **Implemented** | `decomposer.go` + documents/requirements/planning |
| Context pager | **Implemented** | Activate / prefetch / compress / budget reserves |
| Checkpoint runner | **Implemented** | tests/build/manual/shard/nemesis methods |
| Replanner + rolling-wave | **Implemented** | `replan.go`; phase complete triggers refine |
| Write-set lock manager | **Implemented** | Parallel mutating tasks |
| Durable journal + atomic snapshot | **Implemented** | event-before-ack pattern |
| Risk scoring / gate auto-wiring | **Implemented** | `risk_scoring.go`; preflight on `Run` |
| Intelligence gatherer | **Implemented** | Multi-system pre-planning report |
| Shard advisory board | **Implemented** | Consult + synthesize votes |
| Edge case detector | **Implemented** | create/extend/modularize file decisions |
| Tool pregenerator | **Implemented** | Ouroboros-oriented tool gaps |
| Adversarial assault | **Implemented** | Deterministic discover → batch → triage → remediation |
| PromptProvider (static + JIT wire) | **Implemented** | `campaign_prompts.go` + CLI adapter |
| Specialist knowledge injection | **Implemented** | `specialist_knowledge.go` |
| Northstar observer hooks | **Implemented** | Phase start/end; risk-gated |
| TaskExecutor migration path | **Implemented** | Prefer TE over direct shard spawn |
| Sub-campaign `/campaign_ref` | **Implemented** | Lifecycle + failure policies in types |
| Full CLI↔chat parity | **Partial** | Assault stronger in chat docs; Cobra tree is start/status/pause/resume/list |
| Intelligence always wired at boot | **Partial** | Optional DI; nil = skip steps |
| Advisory blocking hard-stop | **Partial** | Logs concerns; synthesis may not always abort plan |
| Package README currency | **Stale** | Still says Dec 2024 in places |

**Overall:** production-grade long-horizon engine — **not** pre-implementation.

---

## 3. Source inventory

### 3.1 Package layout

```
internal/campaign/
  orchestrator.go                 # marker + module map comment only
  orchestrator_types.go           # Orchestrator, Config, events
  orchestrator_init.go            # NewOrchestrator, setters, defaults
  orchestrator_lifecycle.go       # Load/Set/save, resetInProgress
  orchestrator_execution.go       # Run, heartbeat
  orchestrator_control.go         # Pause/Resume/Stop, progress
  orchestrator_phases.go          # phase queries + transitions
  orchestrator_tasks.go           # runPhase, rolling-wave, runSingleTask
  orchestrator_task_handlers.go   # TaskType routing + handlers
  orchestrator_task_results.go    # result cache / context inject
  orchestrator_task_transaction.go
  orchestrator_failure.go         # retries, logic escalation, repro tasks
  orchestrator_journal.go         # journal + atomic snapshot
  orchestrator_utils.go           # checkpoints, events, concurrency
  types.go                        # domain model + ToFacts
  decomposer.go                   # Decompose pipeline orchestration
  decomposer_documents.go         # ingest + classify + knowledge DB
  decomposer_requirements.go      # RAG-ish requirements extraction
  decomposer_planning.go          # LLM plan propose/build/validate/refine
  context_pager.go
  checkpoint.go
  replan.go
  assault_campaign.go / assault_types.go / assault_tasks.go / assault_prompts.go
  risk_scoring.go
  intelligence_gatherer.go / intelligence_gathering_methods.go / intelligence_formatting.go
  edge_case_detector.go
  tool_pregenerator.go
  shard_advisory_board.go
  write_set_lock_manager.go
  document_ingestor.go
  campaign_fact_sync.go
  campaign_prompts.go / prompts.go
  specialist_knowledge.go
  micro_checkpoint.go
  normalization.go / utils.go / errors.go
  task_mutation_types.go
  *_test.go, mocks_test.go, main_test.go
  README.md
```

### 3.2 Hotspot files (approx. line counts from prior inventory / structure)

| Path | ~Lines | Role |
|------|------:|------|
| `assault_tasks.go` | ~1150 | Discover/batch/triage execution + target discovery |
| `prompts.go` | ~1070 | Large static prompt corpus |
| `decomposer.go` | ~1050 | Decompose orchestration + intel seed |
| `edge_case_detector.go` | ~1050 | File action analysis |
| `risk_scoring.go` | ~1050 | Deterministic risk + gate evaluation |
| `replan.go` | ~1000 | Adaptive replan / refine next phase |
| `types.go` | ~900 | Domain + fact emission |
| `orchestrator_task_handlers.go` | ~720 | Per-type execution |
| `shard_advisory_board.go` | ~670 | Advisor consultation |
| `tool_pregenerator.go` | ~650 | Tool gap fill |
| `intelligence_gatherer.go` | ~650 | 12-system intelligence surface |
| `orchestrator_tasks.go` | ~540 | Phase scheduling loop |
| `context_pager.go` | ~500 | Token budget paging |
| `checkpoint.go` | ~480 | Verification runners |
| `orchestrator_failure.go` | ~430 | Failure/retry/escalation |
| `orchestrator_init.go` | ~380 | Construction + validation |

---

## 4. Domain model

### 4.1 Campaign types

Defined in `types.go`:

| `CampaignType` | Atom | Intent |
|----------------|------|--------|
| Greenfield | `/greenfield` | Build from scratch / specs |
| Feature | `/feature` | Major feature |
| Audit | `/audit` | Stability/security audit |
| Migration | `/migration` | Tech migration |
| Remediation | `/remediation` | Cross-codebase fixes |
| Adversarial assault | `/adversarial_assault` | Batched stress + triage |
| Custom | `/custom` | User-defined |

### 4.2 Status machines

**Campaign:** `/planning` → `/decomposing` → `/validating` → `/active` ⇄ `/paused` → `/completed` | `/failed`

**Phase:** `/pending` | `/in_progress` | `/completed` | `/failed` | `/skipped`

**Task:** `/pending` | `/in_progress` | `/completed` | `/failed` | `/skipped` | `/blocked`

On `LoadCampaign` / restart, `resetInProgress()` demotes in-flight phase/task statuses back to pending and rewrites kernel `campaign_task` facts (`orchestrator_lifecycle.go`).

### 4.3 Task types

Planning/execution types: `/file_create`, `/file_modify`, `/test_write`, `/test_run`, `/research`, `/shard_spawn`, `/tool_create`, `/verify`, `/document`, `/refactor`, `/integrate`, `/campaign_ref`.

Assault types: `/assault_discover`, `/assault_batch`, `/assault_triage`.

### 4.4 Task structural fields that matter

| Field | Use |
|-------|-----|
| `DependsOn` / `SoftDeps` | Hard/soft DAG for eligibility |
| `WriteSet` | Canonical paths for write-set lock manager |
| `Shard` / `ShardInput` / `ContextFrom` | Explicit specialist routing + result injection |
| `SubCampaignID` + `CampaignRef*` | Nested campaign contract |
| `Artifacts` | Outputs for paging + assault batch paths |
| `Attempts` / `NextRetryAt` | Durable retry/backoff |
| `Priority` / `Order` | Scheduling hints asserted as facts |

### 4.5 Fact emission (`ToFacts`)

`Campaign.ToFacts` / `Phase.ToFacts` / `Task.ToFacts` assert at least:

- `campaign/5`, `campaign_metadata/4`, `campaign_goal/2`, `campaign_progress/5`
- `source_document/...`, context profile facts
- `campaign_phase/6`, `phase_category/2`, `phase_objective/4`, `phase_dependency/3`, `phase_estimate/3`
- `campaign_task/5`, `task_priority/2`, `task_order/2`, `task_dependency/2`, soft deps, resources
- `task_artifact/...`, `task_attempt/...`, `context_compression/...` when present

This is the **transduction** of plan structure into the executive kernel.

---

## 5. Decomposer (goal → plan)

**Entry:** `Decomposer.Decompose(ctx, DecomposeRequest) (*DecomposeResult, error)`  
**Files:** `decomposer.go`, `decomposer_documents.go`, `decomposer_requirements.go`, `decomposer_planning.go`

### 5.1 Pipeline steps (actual code order)

| Step | Action | Failure mode |
|------|--------|--------------|
| 0 | Optional `IntelligenceGatherer.Gather` + `seedIntelligenceFacts` | Non-fatal warn |
| 1 | Ingest source documents (classify layers, metadata) | **Fatal** |
| 1b | Ingest into campaign knowledge DB (`knowledge.db`) | Non-fatal warn |
| 2 | Extract requirements (smart/RAG path) | **Fatal** |
| 3 | LLM propose plan (`llmProposePlan`, schema/retry) | **Fatal** |
| 4 | `buildCampaign` from `RawPlan` | — |
| 4b | Optional advisory board consult + synthesize | Non-fatal warn |
| 5 | `kernel.LoadFacts(campaign.ToFacts())` | **Fatal** |
| 6 | `validatePlan` (Mangle issues: cycles, unreachable, …) | Issues recorded; refine path |
| 7+ | Edge-case analysis / tool pregeneration when wired | Soft integration |
| | Link requirements → tasks | Coverage tracking |

Defaults: empty goal → `ErrEmptyGoal`; empty SourcePaths entries rejected; context budget default **200000** if 0.

### 5.2 Prompt path

- `PromptProvider` interface (`campaign_prompts.go`) with roles as `CampaignRole`
- Default `StaticPromptProvider` from large `prompts.go` corpus
- Production CLI wires JIT via `CampaignJITProvider` → articulator assembly

Gemini-specific: `GroundingHelper` / `ThinkingHelper` from `internal/tools/research` when client supports them (`completeWithGrounding`).

### 5.3 Assault bypasses LLM decomposition

`NewAdversarialAssaultCampaign(workspace, AssaultConfig)` builds a **fixed 4-phase** plan (Discovery → Assault Execution → Triage → Remediation) with deterministic task seeds. Execution phase tasks are filled at discover time. See §8.

---

## 6. Orchestrator

### 6.1 Construction

`NewOrchestrator(OrchestratorConfig) (*Orchestrator, error)` (`orchestrator_init.go`)

**Required:**

- `Kernel`, `LLMClient`, `Executor` (tactile), `VirtualStore`
- `TaskExecutor` **or** `ShardManager` (monitoring; TE preferred for execution)
- non-empty `Workspace`

**Defaults (unless `DisableTimeouts`):**

| Knob | Default |
|------|---------|
| CampaignTimeout | 4h |
| TaskTimeout | 30m |
| MaxRetries | 3 |
| ReplanThreshold | 3 |
| MaxParallelTasks | 3 (`defaultParallelTasks`) |
| HeartbeatEvery | 15s |
| AutosaveEvery | 1m |
| TaskResultCacheLimit | 100 |
| RetryBackoffBase / Max | 5s / 5m |
| WriteSetLockTimeout / Retry / Poll | 15s / 500ms / 10ms |
| RiskGateThreshold | ≥70 |
| EnableRiskAutoWiring / GlobalRiskGate | true in zero-config path |

Subcomponents always created: `ContextPager`, `CheckpointRunner`, `Replanner`, `Decomposer`, `writeSetLockManager`, static `PromptProvider`. Intelligence components optional via config or setters.

### 6.2 Run loop (`orchestrator_execution.go`)

1. Reject nil campaign / already running  
2. `runRiskPreflight` — can block start  
3. `resetInProgress`  
4. Mark running, status `/active`, optional campaign timeout context  
5. Start `runHeartbeatLoop` (progress emit + `campaign_heartbeat` facts + autosave)  
6. Loop:
   - cancel → status `/paused`, save, return  
   - if paused → block on `pauseCh` (closed channel = running)  
   - `getCurrentPhase` via `kernel.Query("current_phase")`  
   - if no phase: complete? blocked? else `startNextPhase`  
   - on phase transition: `ContextPager.ResetPhaseContext` + `ActivatePhase` + `PrefetchNextTasks(..., 3)`  
   - `runPhase`

### 6.3 Phase execution (`orchestrator_tasks.go`)

`runPhase` maintains:

- `active` map of in-flight task IDs  
- result channel  
- adaptive concurrency via `determineConcurrencyLimit`  
- pause / cancel select  

**Schedule path:**

1. Drain completed results  
2. If phase complete and idle → `runPhaseCheckpoint`  
   - fail → assert `replan_trigger` + `Replanner.Replan`  
   - pass → `CompressPhase`, `completePhase`, `triggerRollingWave`  
3. Else acquire write-set leases for eligible pending tasks (`getEligibleTasks` filters `NextRetryAt`)  
4. `go runSingleTask` with task timeout  
5. Idle with no eligible tasks + `campaign_blocked` → fail campaign  

**Rolling-wave:** VirtualStore `next_action` refresh_scope (best-effort) + `Replanner.RefineNextPhase` + retract/reload `campaign_phase`/`campaign_task` facts.

### 6.4 Task execution routing (`orchestrator_task_handlers.go`)

```
executeTask
  if task.Shard != "" → executeWithExplicitShard (context + specialist knowledge)
  else switch task.Type:
    assault_* | research | file_* | test_* | verify | shard_spawn |
    refactor | integrate | document | tool_create | campaign_ref | default generic
```

Unified spawn: `spawnTask` → `session.TaskExecutor.Execute(TaskRequest{IntentVerb, Task})`.

File tasks: coder shard first; **stat verification** that file exists; fallback direct LLM with path-traversal guards.

### 6.5 Failure handling (`orchestrator_failure.go`)

- Classify error taxonomy  
- Record `TaskAttempt`, assert `task_error`  
- Retry with exponential-ish backoff until `MaxRetries` → `/failed`  
- Logic failures on mutating tasks may insert `[diagnostic-repro]` tasks when escalation window trips  
- Auto-replan path when threshold/config allows  

### 6.6 Control surface

| Method | File | Effect |
|--------|------|--------|
| `Pause` / `Resume` | `orchestrator_control.go` | recreate/close `pauseCh` |
| `Stop` | same | cancelFunc |
| `GetProgress` | same | `Progress` DTO for UI |
| `LoadCampaign` / `SetCampaign` | lifecycle | disk JSON + kernel facts |
| `saveCampaign` | lifecycle | journal events + atomic write |

Persistence path: `.nerd/campaigns/<id>.json` and `.nerd/campaigns/<id>.journal.jsonl`.

---

## 7. Context pager

**File:** `context_pager.go`

Token budget split (default total 200k if unset):

| Reserve | % | Purpose |
|---------|--:|---------|
| Core | 5 | Identity / rules |
| Phase | 30 | Current phase |
| History | 15 | Compressed summaries |
| Working | 40 | Active task |
| Prefetch | 10 | Upcoming hints |

**ActivatePhase:** load context profile (or default schemas/tools/patterns) → boost focus patterns + scoped docs → assert `phase_context_atom` for artifacts → suppress irrelevant schemas (browser/memory when not needed) → estimate usage.

**CompressPhase:** query phase atoms → list accomplishments → LLM summary ≤100 words → store on phase for durability.

**PrefetchNextTasks / PruneIrrelevant / GetUsage / SetBudget** support long runs without unbounded context growth.

Kernel interactions use `activation`, `phase_context_atom`, pattern boosts/suppressions.

---

## 8. Adversarial assault

### 8.1 Intent

Durable, **batched** repo stress without exploding a large tree into per-file LLM tasks. Artifacts:

```
.nerd/campaigns/<slug>/
  knowledge.db
  assault/
    targets.json
    batches/batch_XXXX.json
    results/
    logs/
    triage/
```

### 8.2 Config (`assault_types.go`)

| Field | Default (Normalize) |
|-------|---------------------|
| Scope | `/subsystem` (`/repo`, `/module`, `/subsystem`, `/package`) |
| BatchSize | 10 |
| Cycles | 1 (cap 10) |
| DefaultTimeoutSeconds | 900 |
| Stages | go test + nemesis review |
| LogMaxBytes | 2MB |
| MaxRemediationTasks | 25 |
| ContextBudget | 200000 |
| Include/Exclude | optional path filters |

Stages: `/go_test`, `/go_test_race`, `/go_vet`, `/nemesis_review`, `/command` with `{{target}}`.

### 8.3 Phase graph (`assault_campaign.go`)

| Order | Name | Seed tasks |
|------:|------|------------|
| 0 | Discovery | one `/assault_discover` |
| 1 | Assault Execution | empty until discovery |
| 2 | Triage | one `/assault_triage` |
| 3 | Remediation | filled by triage; tests_pass checkpoint |

Hard phase dependencies chain 0→1→2→3.

### 8.4 Handlers (`assault_tasks.go`)

- **Discover:** mkdir tree, discover targets (Go packages/dirs by scope), write targets/batches, `appendTasksToPhase` with `/assault_batch` artifacts; idempotent if batches already exist  
- **Batch:** read batch artifact, run stages per target, persist results/logs  
- **Triage:** summarize failures, optional LLM remediation plan, append remediation tasks to phase 3  

Operator docs (root Agents.md): chat `/campaign assault …` for long-horizon validation.

---

## 9. Checkpoints & replan

### 9.1 CheckpointRunner

`checkpoint.go` — methods:

| Method | Behavior |
|--------|----------|
| `/tests_pass` | detect go/npm test cmd; JSON parse; exit code |
| `/builds` | detect build command |
| `/manual_review` | review path (shard/LLM) |
| `/shard_validation` | specialist validation spawn |
| `/nemesis_gauntlet` | adversarial review spawn |
| `/none` | always pass |

Also `RunAll` (phase objectives) and `RunQuick`.

### 9.2 Replanner

`replan.go`:

- `Replan(ctx, campaign, failedTaskID)` — gather failed/blocked + `replan_trigger` facts; LLM plan delta; apply add/remove/modify  
- `RefineNextPhase` — rolling-wave after success  
- Prompt budgets: max tasks/context chars for stable prompts  
- Optional grounding like decomposer  

Reasons: `/task_failed`, `/new_requirement`, `/user_feedback`, `/dependency_change`, `/blocked`.

---

## 10. Risk gating

`risk_scoring.go` (+ orchestrator wiring):

- Deterministic score from `RiskInputSnapshot` (phases/tasks, complexity, churn, safety warnings, tool gaps, …)  
- Threshold default **70**  
- Modes: `/auto`, `/force_allow`, `/force_block`  
- Per-gate toggles: advisory, edge, northstar (`/auto|/enabled|/disabled`)  
- Protected roots include `internal/core`, `internal/mangle`, `internal/campaign`, perception, articulation  
- `runRiskPreflight` at campaign start; may return block error  
- Resolved gate state recomputed when observers/boards attached  

Gates are **deterministic policy**, not LLM vibes — aligns with logic-as-executive.

---

## 11. Intelligence & pre-plan enrichment

| Component | Role |
|-----------|------|
| `IntelligenceGatherer` | Aggregate world/git/MCP/safety/history signals into `IntelligenceReport` |
| `ShardAdvisoryBoard` | Multi-advisor consult + `SynthesizeVotes` |
| `EdgeCaseDetector` | File create/extend/modularize decisions (`FileAction`) |
| `ToolPregenerator` | Detect tool gaps; generate before execute |
| `SpecialistKnowledgeProvider` | Inject specialist DB atoms into task context |
| `northstar.CampaignObserver` | Vision alignment on phase transitions |

These are **optional DI**. Zero-config orchestrator still runs; enrichment appears when callers wire them (CLI boot, tests, chat).

---

## 12. Durability: journal & write-set locks

### 12.1 Journal (`orchestrator_journal.go`)

- Append-only JSONL: seq, timestamp, event_type, campaign_id, payload, snapshot_checksum, event checksum (SHA-256)  
- `saveCampaign`: `snapshot_write_requested` → atomic temp write + verify checksum + rename → `snapshot_write_committed`  
- `recoverJournalSequence` on load to avoid seq reuse  
- fsync file (+ dir when supported)

### 12.2 Write-set locks (`write_set_lock_manager.go`)

- Normalize paths absolute, slash, case-fold on Windows  
- Sorted acquisition to reduce deadlock  
- Lease with idempotent `release`  
- Timeout → `ErrWriteSetLockTimeout` (scheduler continues without permanent fail)  
- Outside-workspace paths rejected (with documented test quirk for task id `t1`)

---

## 13. Integration map (who imports campaign)

| Consumer | Path | Use |
|----------|------|-----|
| CLI | `cmd/nerd/cmd_campaign.go` | start/status/pause/resume/list |
| JIT adapter | `cmd/nerd/campaign_jit_provider.go` | PromptProvider |
| UI | `cmd/nerd/ui/campaign_page.go` | Progress presentation |
| E2E | `tests/e2e/campaign_session_integration_test.go` | integration |
| Skills/docs | codenerd-builder campaign-orchestrator refs | human guidance |

Upstream packages used by campaign: `core`, `core/shards`, `perception`, `session`, `tactile`, `logging`, `types`, `northstar`, `tools/research`, `embedding` (document ingestor), `world` (edge detector scanner).

---

## 14. Concurrency model

| Mechanism | Purpose |
|-----------|---------|
| `o.mu` RWMutex | campaign pointer, pause state, config-ish fields |
| `resultsMu` | task result cache |
| `pauseCh` | pause without busy-wait |
| `cancelFunc` | Stop + parent cancel |
| task goroutines | bounded by `maxParallelTasks` / adaptive limit |
| heartbeat goroutine | progress + autosave independent of task progress |
| `journalSeq` atomic | monotonic event IDs |
| write-set manager mutex | file ownership |

---

## 15. Errors (`errors.go`)

Exported sentinels: `ErrDecompositionFailed`, `ErrTaskTimeout`, `ErrCampaignTimeout`, `ErrCheckpointFailed`, `ErrReplanExhausted`, `ErrNilDependency`, `ErrInvalidConfig`, `ErrNilCampaign`, `ErrNilKernel`, `ErrEmptyRequirement`, `ErrEmptyGoal`.  
Plus `ErrWriteSetLockTimeout` in lock manager.

---

## 16. Gaps pointer

See [03-GAP-ANALYSIS.md](03-GAP-ANALYSIS.md) and [TODO.md](TODO.md). High-signal partials:

1. Optional intelligence/advisory may log but not hard-block weak plans  
2. CLI assault flags vs chat assault entry asymmetry  
3. Direct LLM file fallbacks can bypass preferred shard path (by design for resilience — audit for safety)  
4. Mangle policy lives **outside** this package — wiring depends on kernel program load  
5. Package README version stamp lag  

---

## 17. Related deep-dives in this corpus

| Topic | Doc |
|-------|-----|
| Vision | [01-VISION.md](01-VISION.md) |
| File inventory | [02-CURRENT-STATE.md](02-CURRENT-STATE.md) |
| Architecture diagrams | [05-INTERNAL-ARCHITECTURE.md](05-INTERNAL-ARCHITECTURE.md) |
| API catalog | [06-PUBLIC-API-AND-TYPES.md](06-PUBLIC-API-AND-TYPES.md) |
| Mangle predicates | [09-MANGLE-SURFACE.md](09-MANGLE-SURFACE.md) |
| Safety | [09-SAFETY-AND-INVARIANTS.md](09-SAFETY-AND-INVARIANTS.md) |
| Wiring | [08-WIRING-AND-INTEGRATION.md](08-WIRING-AND-INTEGRATION.md) |
| Failures | [12-FAILURE-MODES.md](12-FAILURE-MODES.md) |

---

## 18. Verify commands

```powershell
$env:CGO_CFLAGS = "-IC:/CodeProjects/codeNERD/sqlite_headers"
go test ./internal/campaign/...
go test ./tests/e2e/ -run Campaign -count=1
```

---

*This document is the authoritative living architecture for `internal/campaign` as of 2026-07-13. Prefer code over narrative when they diverge; open a gap note rather than inventing APIs.*
