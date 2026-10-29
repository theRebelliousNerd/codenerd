# Northstar — Implemented Spec (Deep-Dive)

> Last verified against codebase: 2026-08-15  
> Status: Living Reference Document  
> Language: Go  
> Primary sources: `internal/northstar/`  
> Scale: **4** non-test Go files ≈ **2,196** lines; **6** test files ≈ **3,135** lines; **0** package-local `.mg`  
> Adjacent: `cmd/nerd/chat/` (wizard, boot, `/alignment`), `cmd/nerd/cmd_northstar.go`, `internal/campaign/`, `internal/prompt/atoms/northstar/`

## 1. Overview

`internal/northstar` implements the **permanent Northstar Guardian**: a core system agent that holds project vision, records observations, runs LLM-backed (or soft-fallback) alignment checks, tracks drift, and can project vision into the Mangle kernel as `northstar_*` facts.

Unlike user specialists under `.nerd/agents/`, Northstar is built into the runtime library. Package comment (`types.go`):

- Prompt atoms: `internal/prompt/atoms/northstar/`
- Knowledge DB: `.nerd/northstar_knowledge.db`

### Key characteristics

| Property | Value |
|----------|-------|
| Role | Strategic vision guardian (not OODA executor) |
| Storage | SQLite (`Store`) |
| Judgment | Optional `LLMClient`; structured SCORE/RESULT parse |
| Kernel | Optional `KernelClient` retract+assert |
| Observers | Campaign, Task, Background event handler |
| Logging | `logging.CategoryNorthstar` |
| Safety model | Soft by default; hard only at campaign observer boundaries |

### High-level flow

```
Define vision ──► Store.SaveVision / Guardian.UpdateVision
                      │
                      ├─► refreshKernelFacts (if kernel set)
                      │
Events / manual ──► CheckAlignment ──► AlignmentCheck
                      │                     │
                      │                     ├─ RecordAlignmentCheck
                      │                     └─ Drift if failed|blocked
                      ▼
              Campaign risk / TUI / Background assessment
```

Fact-flow relationship:

```
user_intent → kernel → next_action → VirtualStore → articulation
                    ↑
         northstar_* facts (optional projection)
```

---

## 2. Implementation status

| Component | Status | Notes |
|-----------|--------|-------|
| Domain types + `ToFacts` | **Implemented** | Partial vs full Decl surface |
| SQLite Store + schema | **Implemented** | `ingested_docs` used by `docs.go`; `GetMetrics`/`GetDriftHistory` added |
| Guardian init / config | **Implemented** | Warn on no vision |
| LLM alignment pipeline | **Implemented** | Inline prompts |
| Soft fallbacks (no vision/LLM) | **Implemented** | Availability bias |
| Drift recording | **Implemented** | Failed/blocked only |
| CampaignObserver | **Implemented** | Wired in campaign package |
| TaskObserver | **Implemented** | Sparse external callers |
| BackgroundEventHandler | **Implemented** | Chat boot + adapter |
| Chat `/alignment` | **Implemented** | Ephemeral guardian |
| Chat `/northstar` wizard | **Implemented** (outside pkg) | JSON/MG oriented |
| CLI `nerd northstar` | **Implemented** (outside pkg) | Does not use this package |
| Kernel wire on all boots | **Implemented** | Both chat boots call `SetParentKernel`; pinned by `TestChatBootPaths_ShouldWireGuardianKernelIdentically` |
| JSON ↔ SQLite sync | **Implemented** | `bridge.go` `SyncVisionAuthority`, run from `Guardian.Initialize` |
| Alignment history CLI | **Implemented** | `nerd northstar history\|drift\|state\|sync` |
| Embedding relevance | **Implemented** | `docs.go`: `ingested_docs` API + injectable `Embedder`, cosine similarity, keyword fallback |
| JIT alignment prompts | **Implemented** | `atoms/northstar/guardian_alignment.yaml` + `alignment_prompt.go`, parity-tested |

**Overall:** living production library. The integration gaps that mattered for operators (dual vision authority, boot kernel wiring, no history surface) are closed; see [13-OPERATOR-RUNBOOK.md](13-OPERATOR-RUNBOOK.md).

---

## 3. Source inventory

### 3.1 Layout

```
internal/northstar/
  types.go              # Vision model, enums, defaults, ToFacts
  store.go              # SQLite knowledge DB
  guardian.go           # Guardian + alignment + observe helpers
  observer.go           # Campaign / Task / Background handlers
  *_test.go             # Dense unit suite
```

### 3.2 Largest non-test files

| Path | ≈Lines | Purpose |
|------|-------:|---------|
| `store.go` | 732 | Persistence, schema, rollups |
| `guardian.go` | 677 | Alignment engine + kernel refresh |
| `observer.go` | 482 | Integrator-facing observers |
| `types.go` | 305 | Shared domain model |

### 3.3 Tests

| Path | ≈Lines |
|------|-------:|
| `guardian_test.go` | 1103 |
| `store_test.go` | 710 |
| `types_test.go` | 623 |
| `observer_test.go` | 514 |
| `types_facts_test.go` | 114 |
| `guardian_warn_test.go` | 71 |

---

## 4. Domain model (deep dive)

### 4.1 Vision

`Vision` captures product strategy:

- **Mission / Problem / VisionStmt** — narrative spine  
- **Personas** — pain points + needs  
- **Capabilities** — timeline (`now`/`next`/`later`) + priority  
- **Risks** — likelihood/impact + mitigation text  
- **Requirements** — type + priority  
- **Constraints** — free strings  

Timestamps `CreatedAt` / `UpdatedAt` managed by Store on save (preserves created_at on update).

### 4.2 Alignment lifecycle types

`AlignmentCheck` records who/what/when:

- **Trigger** — manual, phase_gate, periodic, high_impact, task_complete, session_start, campaign_start  
- **Result** — passed, warning, failed, blocked, skipped  
- **Score** — 0.0–1.0  
- **Suggestions** — LLM list  

Threshold defaults: warning ≥0.7 pass; ≥0.5 warning; ≥0.3 failed; else blocked (`classifyScore`).

### 4.3 Observations & drift

Observations tag session activity with relevance. Drift events link to checks, support resolve with resolution text, and maintain `active_drift_count`.

---

## 5. Store architecture

### 5.1 Location

`NewStore(nerdDir)` → `{nerdDir}/northstar_knowledge.db`  
Typical: `{workspace}/.nerd/northstar_knowledge.db`

### 5.2 Tables

| Table | Cardinality | Notes |
|-------|-------------|-------|
| `vision` | 0–1 | CHECK id=1 |
| `observations` | N | indexes session/type/timestamp |
| `alignment_checks` | N | duration_ms |
| `drift_events` | N | FK related_check; resolved flag |
| `guardian_state` | 1 | rollup singleton |
| `ingested_docs` | N | **no API** |

### 5.3 Rollup math

Alignment score update:

```text
overall_alignment = overall_alignment * 0.8 + score * 0.2
```

Task counter resets to 0 on every recorded check; increments via `IncrementTaskCount`.

### 5.4 Concurrency

Package-level `sync.RWMutex` around DB operations plus SQL transactions for multi-statement updates.

---

## 6. Guardian architecture

### 6.1 Dependencies

```text
Guardian
  store  *Store
  config GuardianConfig
  llm    LLMClient        // optional
  kernel KernelClient     // optional
  state  *GuardianState
  vision *Vision
  mu     sync.RWMutex
```

### 6.2 Initialize

1. `LoadVision` / `GetState`  
2. Clone into fields  
3. Info or **Warn** (no vision + path)  
4. `refreshKernelFacts`

### 6.3 CheckAlignment branches

| Branch | Result | Score |
|--------|--------|------:|
| vision == nil | skipped | 1.0 |
| llm == nil | passed | 0.8 |
| LLM error | warning | 0.7 |
| parse default | warning | 0.7 if unparsed |
| explicit RESULT | as parsed | SCORE if valid |
| score only | classifyScore | SCORE |

Always persists outcome (best-effort). Logs Info on success path.

### 6.4 Prompt contract (LLM)

System prompt embeds mission/problem/vision, personas, capabilities, requirements, risks, constraints.

Required response shape:

```text
SCORE: <0.0-1.0>
RESULT: <passed|warning|failed|blocked>
EXPLANATION: <one sentence>
SUGGESTIONS: <comma-separated|none>
```

Parser tolerates quotes/trailing commas; strips `"`; uses `strings.SplitSeq`.

### 6.5 Relevance

- Text: bag-of-words overlap with mission/problem/vision (words length > 3).  
- Path: high-impact patterns → 0.9 else 0.5.  
- Decisions default relevance 0.8.

### 6.6 Path matching

`matchesHighImpactPath`:

1. `path.Match` full path (slash-normalized)  
2. Prefix if pattern ends with `/`  
3. Wildcard base name match or prefix-before-`*`  
4. Exact equality  

---

## 7. Observers (deep dive)

### 7.1 CampaignObserver

Owned by campaign orchestrator via `SetNorthstarObserver`.

| Method | Hard fail? |
|--------|------------|
| `StartCampaign` | Yes if blocked |
| `OnPhaseStart` | Yes if blocked (non-first phase) |
| `OnPhaseComplete` | No (observe only) |
| `OnTaskComplete` | Returns check; caller ignores error today (`_, _ =`) |
| `EndCampaign` | Observe only |

Periodic: every `checkEveryNTasks` in phase. High-impact: uses guardian path config.

### 7.2 TaskObserver

Session-scoped; delegates periodic logic to `Guardian.OnTaskComplete`. Records errors as pattern observations.

### 7.3 BackgroundEventHandler

Intended implementation of shards handler without importing shards:

1. Build subject/context  
2. Always record observation  
3. Skip assessment if no vision  
4. Map event type → trigger  
5. CheckAlignment  
6. Map result → Level (`proceed`/`note`/`clarify`/`block`)

Adapter in chat maps to `shards.ObserverAssessment`.

---

## 8. Mangle surface

### 8.1 Emitted predicates

See `Vision.ToFacts` and `06-PUBLIC-API-AND-TYPES.md`. Sentinel `northstar_defined()` always appended when vision non-nil.

### 8.2 Relational facts

`northstar_serves`, `northstar_supports` and `northstar_addresses` are emitted
from `Capability.Serves`, `Requirement.Supports` and `Requirement.Addresses`.
`unserved_persona`, `orphan_capability`, `orphan_requirement`,
`risk_addressing_requirement`, `unaddressed_high_risk` and
`strategic_warning(/critical_unmitigated_risk, …)` in
`internal/core/defaults/policy/prompt_northstar.mg` depend on them and could
never fire before.

A link whose target does not exist in the same vision is **dropped**, not
emitted: a dangling `northstar_serves(cap, persona_ghost)` would make
`unserved_persona` silently wrong.

### 8.3 Mitigation encoding

The `northstar_mitigation` strategy slot is Decl'd `/name` and so cannot hold
free text. Two facts are emitted per mitigated risk:

- `northstar_mitigation(RiskID, /mit_<slug>_<hash8>)` — readable and injective
- `northstar_mitigation_text(RiskID, Text)` — the operator's own words
  (Decl added to `internal/core/defaults/schemas_misc.mg`)

Previously every mitigation was the same constant `/mitigation`, so two risks
with opposite strategies unified.

### 8.4 Injectable / JIT context (consumers)

Core/articulation can treat northstar facts and `northstar_phase` as compile context. Package only ensures facts *can* exist.

---

## 9. Integration map

| Integrator | Path | Uses |
|------------|------|------|
| Chat boot | `session_boot.go` | Store, Guardian, BackgroundEventHandler |
| Shared boot | `session_shared_boot.go` | + SetParentKernel |
| Adapter | `session_boot_helpers.go` | Handler bridge |
| Alignment UX | `model_helpers.go` | Ephemeral CheckAlignment |
| Campaign | `orchestrator_*`, `risk_scoring.go` | CampaignObserver lifecycle |
| Init | `internal/init` | DB path convention |
| CLI northstar | `cmd_northstar.go` | **JSON/MG only** (no import) |
| Wizard | `chat/northstar_*.go` | JSON/MG + atoms |

---

## 10. Vision authority (resolved)

| Artifact | Role | Writer | Reader |
|----------|------|--------|--------|
| kernel `northstar_*` facts | **executive** | `Guardian.refreshKernelFacts` | policy, JIT context, `nerd query` |
| `.nerd/northstar_knowledge.db` | **durable record** | `Store.SaveVision`, `Guardian.UpdateVision` | Guardian, `/alignment`, campaign observer, CLI |
| `.nerd/northstar.json` | import + export surface | wizard, `nerd northstar load`/`sync`, exports | `LoadVisionJSON` during reconciliation |
| `.nerd/northstar.mg` | export surface | exports (rendered from `Vision.ToFacts`) | humans |

`SyncVisionAuthority` (bridge.go) reconciles the JSON surface with the store and
runs inside `Guardian.Initialize`, so every boot path converges. Direction is
last-writer-wins on file mtime vs `updated_at`; equal content is a no-op so
boots never churn the files. Full rules and operator recipes:
[13-OPERATOR-RUNBOOK.md](13-OPERATOR-RUNBOOK.md).

---

## 11. Configuration surface

`GuardianConfig` JSON-serializable fields (see defaults in §5 of internal architecture).  
`AlignmentModel` is honoured when the injected client implements
`ModelSelectingLLMClient` (`CompleteWithSystemModel`); otherwise the guardian
logs once and runs on the client's default model rather than mutating a shared
client. Threshold ordering (`block <= failure <= warning`) is repaired by
`NormalizeGuardianConfig` in `NewGuardian` — an inverted set previously made
whole result bands unreachable and classified failing work as passed.

Campaign-side: `NorthstarGateToggle` controls whether observer is active for risk scoring (auto/enabled/disabled).

---

## 12. Testing summary

~114 unit tests; strong coverage of:

- Vision/state cloning  
- Alignment branches and parser edge cases (JSON-like noise, scientific notation, empty response)  
- Drift refresh  
- Path matching  
- Store transactions / timestamps  
- Observer lifecycle  

Boot wiring and vision authority are now covered:
`bridge_test.go`, `guardian_wiring_test.go` (threshold repair, registry
refcounting, model selection, prompt atoms, boot-path source parity),
`kernel_integration_test.go` (boot with vision → `northstar_defined()` true in a
real kernel, and stale-fact retraction), `docs_test.go` (ingested docs,
embedding relevance, metrics), and
`cmd/nerd/chat/northstar_adapter_test.go`.

Commands:

```powershell
go test ./internal/northstar/...
go test -race ./internal/northstar/...
```

---

## 13. Gaps pointer

Full matrix: [03-GAP-ANALYSIS.md](03-GAP-ANALYSIS.md).  
Failure catalog: [12-FAILURE-MODES.md](12-FAILURE-MODES.md).  
Wiring: [08-WIRING-AND-INTEGRATION.md](08-WIRING-AND-INTEGRATION.md).

Top three:

1. Unify JSON/MG and SQLite vision authority.  
2. Kernel wiring parity on all boot paths.  
3. Surface SQLite history/drift to operators.

---

## 14. Principles pointer

Binding principles: [04-ARCHITECTURAL-PRINCIPLES.md](04-ARCHITECTURAL-PRINCIPLES.md).

North-star alignment scores: [00-ALIGNMENT-VISION-REVIEW.md](00-ALIGNMENT-VISION-REVIEW.md).

---

## 15. Change policy for this corpus

When editing `internal/northstar/`:

1. Update this IMPLEMENTED_SPEC if public API or fact shape changes.  
2. Re-verify reverse deps if observers/constructors change.  
3. Re-run package tests before handoff.  
4. Bump last-verified date.
