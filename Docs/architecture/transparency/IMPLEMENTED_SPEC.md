# transparency — Implemented Spec (Deep-Dive)

> Last verified against codebase: **2026-07-13**  
> Status: Living Reference Document  
> Language: Go  
> Primary sources: `internal/transparency/`  
> Scale: **8** non-test Go files ≈ **1,863** lines; **9** test files; **0** Mangle sources  
> Config home: `internal/config/ux.go` (`TransparencyConfig`)

## 1. Overview

`internal/transparency` is codeNERD’s **shared visibility kit**: types and small services that turn internal execution into human-readable status, safety explanations, error remediation, and live TUI telemetry.

It answers operator questions without becoming a second control plane:

| Question | Mechanism |
|----------|-----------|
| What is the agent doing right now? | Glass Box events + Tool events + optional shard phase tracking |
| Why did the kernel choose this action? | `Explainer` over `mangle.DerivationTrace` |
| Why was something blocked? | `SafetyReporter` / `ExplainSafetyAction` |
| What went wrong and how do I fix it? | `ClassifyError` / `ClassifiedError.Format` |
| Is deep visibility on? | `TransparencyManager.GetStatus` + `/transparency` |

### Key characteristics

| Property | Value |
|----------|-------|
| Master transparency toggle | Default **off** (`Enabled: false`) |
| Tool telemetry | Default **always on** (no master gate) |
| Glass Box bus | Created & **Enable()**’d at chat boot for producers |
| Executive authority | **None** — observe/format only |
| Concurrency stance | Non-blocking emit; drop if subscriber full |
| Mangle coupling | `Explainer` only (`internal/mangle` traces) |
| Package-local policy | None (no `.mg`) |

### Dual-track architecture

```
Track A — TransparencyManager (config-gated features)
  ShardObserver · SafetyReporter · FormatError/ClassifyError · GetStatus

Track B — Glass Box + Tools (instance-injected buses)
  GlassBoxEventBus · ToolEventBus
  (not owned by TransparencyManager)
```

Callers (chat boot, VirtualStore, ShardManager, system shards) wire Track B independently. Track A is toggled by config and `/transparency`.

### Fact-flow placement

```
user input
  → perception ──(optional)──► GlassBox CategoryPerception
  → user_intent / kernel evaluate
       · next_action / permitted remain kernel facts
       · Explainer formats derivation AFTER query/trace
  → VirtualStore action ──► ToolEvent + GlassBox CategoryRouting
  → ShardManager spawn ──► GlassBox CategoryShard
  → system router tools ──► ToolEvent
  → articulation / chat scrollback
```

Constitutional safety remains: **no `permitted` ⇒ deny**. Transparency only explains the outcome.

---

## 2. Implementation status

| Component | Status | Notes |
|-----------|--------|-------|
| Package docs / principles | **Implemented** | `doc.go` |
| TransparencyManager | **Implemented** | Enable/Disable/Toggle/Status/shard+safety façades |
| ShardObserver + phases | **Implemented** | Under-fed from live shard spawn vs Glass Box |
| SafetyReporter | **Implemented** | History + format; auto-feed partial |
| ExplainSafetyAction | **Implemented** | Hypothetical risk analysis (string heuristics) |
| Error classifier | **Implemented** | Pattern match on error text |
| Explainer | **Implemented** | Trace/fact/decision/narrative/QuickExplain |
| OperationSummary format | **Implemented** | Formatter only; producers optional |
| GlassBoxEvent types | **Implemented** | 6 categories |
| GlassBoxEventBus | **Implemented** | Batch 50ms/20, verbose immediate, filter, stats |
| ToolEventBus | **Implemented** | Always-on buffered channel (50) |
| Config schema | **Implemented** | `TransparencyConfig` in config package |
| Chat boot wiring | **Implemented** | session_boot / session_shared_boot |
| VirtualStore emit | **Implemented** | `emitToolAndRoutingEvents` |
| ShardManager Glass Box emit | **Implemented** | `emitShardEvent` |
| System shard ToolEvent | **Implemented** | router + base setters |
| `/transparency` slash | **Implemented** | on/off/toggle + status |
| `/glassbox` slash | **Implemented** | chat package (consumer) |
| `/why` + Explainer | **Implemented** | chat + model_update path |
| Manager→ShardObserver live feed | **Partial** | APIs exist; ShardManager stores Manager as `any` without phase calls |
| JITExplain / StreamReasoning flags | **Partial / surface** | Shown in status; package does not enforce |
| Drop metrics | **Missing** | Silent drop on full channel |
| Machine-readable export | **Missing** | Markdown/strings for humans |

**Overall:** living production package — **not** pre-implementation.

---

## 3. Source inventory

### 3.1 Layout

```
internal/transparency/
  doc.go                        # package overview + principles
  transparency.go               # TransparencyManager
  shard_observer.go             # phases + PhaseObserver
  safety_reporter.go            # violations + ExplainSafetyAction
  error_classifier.go           # ClassifiedError
  explainer.go                  # derivation narratives
  glass_box_events.go           # events, categories, ToolEventBus
  event_bus.go                  # GlassBoxEventBus
  *_test.go                     # 9 test files
```

### 3.2 Role of each source file

#### `doc.go`

Declares the product contract:

1. Opt-in  
2. Non-intrusive  
3. Lazy (expensive work only when requested)  
4. Informative (“why” not just “what”)  

Lists intended surfaces: shard phases, safety explanations, JIT explain, proof trees, operation summaries, error categorization. **Not all listed items are fully enforced inside this package** (see gaps).

#### `transparency.go` — TransparencyManager

Coordinates feature toggles and subcomponents:

- Fields: `config *config.TransparencyConfig`, `shardObserver`, `safetyReporter`, `enabled`, `mu`.  
- `NewTransparencyManager(nil)` invents a minimal default config.  
- Enable/Disable cascade to observer + safety reporter.  
- Façades: `StartShard`, `UpdateShardPhase`, `EndShard`, `ReportSafetyViolation` check `IsEnabled()` and relevant flags.  
- `GetStatus()` markdown table of feature flags + active executions + recent violations.  
- `FormatError` always classifies; full remediation format only when enabled + `VerboseErrors`.

Does **not** construct or hold Glass Box / Tool buses.

#### `shard_observer.go`

State machine for shard execution visibility:

| Phase | Meaning |
|-------|---------|
| Idle | not active |
| Initializing | start |
| Loading | context load |
| Analyzing | analysis |
| Generating | LLM/codegen |
| Executing | tools/actions |
| Complete / Failed | terminal |

`StartExecution` tracks even when disabled; **notifications and phase history** only when `enabled`. Active list excludes Idle/Complete/Failed. Max history 100. `PhaseObserver` callback interface for external UI.

#### `safety_reporter.go`

- Classifies violations: destructive, protected path, secret exposure, resource limit, policy, unauthorized, unknown.  
- Heuristics on action/target/rule strings.  
- Ring buffer max 50.  
- `FormatViolation` markdown.  
- `ExplainSafetyAction` for proactive `/safety`-style analysis (not kernel query).

#### `error_classifier.go`

Categories: Safety, Config, API, Kernel, Shard, Filesystem, Network, Timeout, Unknown.  
`ClassifyError` lowercases `err.Error()` and matches substrings.  
`GetRecoveryGuide` returns category-level slash-command oriented steps.

#### `explainer.go`

Depends on `mangle.DerivationTrace` / `DerivationNode` / `SourceEDB`:

- Tree walk with `maxDepth` (default 5).  
- Rule name → English map (strategy_selector, permission_gate, …).  
- `ExplainDecision` prefers root `next_action`.  
- `QuickExplain` one-liners for common predicates.  
- `OperationSummary` + `FormatOperationSummary` for post-hoc ops.

#### `glass_box_events.go`

Categories:

| Constant | Value | Intent |
|----------|-------|--------|
| `CategoryPerception` | perception | intent/entities |
| `CategoryKernel` | kernel | facts/derivations |
| `CategoryJIT` | jit | atom selection |
| `CategoryShard` | shard | lifecycle |
| `CategoryControl` | control | control packets |
| `CategoryRouting` | routing | tools/routing |

`GlassBoxEvent`: ID, Timestamp, Category, Summary, Details, TurnID, Duration, Source.  
`ToolEvent` / `ToolEventBus`: always-on, buffer 50, drop if full.

#### `event_bus.go` — GlassBoxEventBus

| Setting | Default |
|---------|---------|
| batchWindow | 50ms |
| batchLimit | 20 |
| subscribe buffer | 512 |
| sequence | atomic uint64 |

Behavior:

- Disabled → Emit no-op.  
- Verbose → `EmitImmediate` (live stream).  
- Category filter: empty map = allow all.  
- Flush sorts by sequence before dispatch.  
- `ClearTurn(turnID)` drops buffered events for a turn.  
- `Close` disables + closes all subscriber channels.  
- `Stats()` for ops/debug.

---

## 4. Deep dives — main flows

### 4.1 Glass Box event lifecycle

```
Producer (VS / ShardManager / future)
    │
    ▼
bus.Emit(event) ── if !enabled → return
    │
    ├─ verbose? ──► EmitImmediate ──► for each sub: non-blocking send
    │
    └─ filter categories ──► assign ID+Timestamp ──► buffer
                              ├─ len >= batchLimit → flushLocked
                              └─ else AfterFunc(batchWindow) → flush
flushLocked:
  sort by ID → send each event to each sub (drop if full) → clear buffer
```

**Consumer (chat):** `initGlassBox` → `Subscribe()` → `listenGlassBoxEvents` tea.Cmd → `drainGlassBoxEvents` (up to 64) → `handleGlassBoxEvent` updates activity pulse + optional scrollback lines.

### 4.2 Always-on tool events

```
VirtualStore.emitToolAndRoutingEvents
  │
  ├─ ToolEventBus.Emit(ToolEvent{ToolName, Result, Success, Duration})
  └─ GlassBoxEventBus.EmitImmediate(CategoryRouting, ...)

system/router tool completion
  └─ ToolEventBus.Emit(...)
```

Chat `initToolEventBus` subscribes and surfaces tool lines independent of Glass Box toggle. **This is intentional product behavior**, not a bug.

### 4.3 TransparencyManager toggle path

```
/transparency [on|off|∅]
  → handleTransparencyCommand
  → NewTransparencyManager if nil
  → Enable | Disable | Toggle
  → if enabled: append GetStatus() markdown
```

Enable path:

1. `enabled = true`  
2. If `ShardPhases`: observer.Enable()  
3. safetyReporter.Enable()  

Disable disables both subcomponents regardless of flags.

### 4.4 Safety violation report path

```
ReportSafetyViolation(action, target, rule)
  → require Enabled && SafetyExplanations
  → SafetyReporter.ReportViolation
       classifyViolation heuristics
       append history (cap 50)
  → return *SafetyViolation (or nil)
```

Callers must invoke this; constitutional denies do not automatically reach the reporter unless wired upstream.

### 4.5 Explain /why path

```
/why <fact>  (chat)
  → kernel/heuristic trace producing mangle.DerivationTrace
  → model_update traceUpdateMsg with ShowInChat
  → transparency.NewExplainer().ExplainTrace(trace)
  → assistant markdown message
```

Separately, `/explain` uses provenance recording (chat) for higher fidelity—**outside** this package but complementary.

### 4.6 Error formatting path

```
err → ClassifyError → ClassifiedError
  TransparencyManager.FormatError:
    if enabled && VerboseErrors → classified.Format()  # prefix + details + remediation
    else → Prefix + Summary + Details only (no "Suggested fixes" section unless Format)
```

Note: disabled path still classifies and prints prefix/summary—it is not raw `err.Error()` alone.

---

## 5. Integration map

### 5.1 Who constructs what

| Site | Constructs |
|------|------------|
| `cmd/nerd/chat/session_boot.go` | `NewTransparencyManager`, `NewGlassBoxEventBus`+Enable, `NewToolEventBus` |
| `cmd/nerd/chat/session_shared_boot.go` | Same pattern (shared boot) |
| `cmd/nerd/chat/session.go` | Manager (session path) |
| `commands_handlers_misc.go` | Manager lazy if nil on `/transparency` |
| `model_update.go` | `NewExplainer` per `/why` display |

### 5.2 Who receives buses

| Receiver | Method |
|----------|--------|
| `ShardManager` | `SetGlassBoxBus`, `SetTransparencyManager` |
| `VirtualStore` | `SetGlassBoxBus`, `SetToolEventBus` |
| `BaseSystemShard` | `SetGlassBox`, `SetToolEventBus` |
| Chat `Model` | fields + CortexConfig passthrough |

### 5.3 Import direction

```
cmd/nerd/chat ──imports──► transparency
internal/core ──imports──► transparency
internal/core/shards ──imports──► transparency
internal/shards/system ──imports──► transparency

transparency ──imports──► config, mangle (explainer only)
transparency ──does not import──► cmd, core, chat
```

Cycle avoidance is intentional: chat injects buses into core/shards via setters.

---

## 6. Concurrency & performance contracts

| Contract | Implementation |
|----------|----------------|
| Emit never blocks forever | `select` + `default` drop |
| Ordering within batch | sort by sequence ID at flush |
| Live mode | verbose skips batch timer |
| Shared maps | RWMutex on bus subscribers/categories; bufferMu for buffer |
| Atomic flags | `enabled`, `sequence` on bus; Manager uses mutex for enabled |
| History bounds | phase 100, violations 50, chat ring 500 (consumer) |

**Performance:** batching reduces Bubble Tea message thrash; verbose trades latency for fidelity.

---

## 7. Relationship to constitutional safety

Transparency **amplifies** safety UX:

- Surfaces blocks with remediation  
- Points users at `/shadow`, `/whatif`, `/query permitted`  
- Classifies permission language as `[SAFETY]`  

It **must not**:

- Grant override  
- Assert `permitted`  
- Soften default deny  

Any “override” wording in remediation is instructional for the operator, not an API that flips policy.

---

## 8. Gaps pointer

See [03-GAP-ANALYSIS.md](03-GAP-ANALYSIS.md) for the full matrix. Headline gaps:

1. `ShardObserver` vs live Glass Box shard events (dual systems, uneven feed).  
2. Config flags shown in status without package-level enforcement (`JITExplain`, `StreamReasoning`, `OperationSummaries`).  
3. Safety auto-reporting from deny paths incomplete.  
4. No drop counters / observability of lost events.  
5. `TransparencyManager` typed as `any` on ShardManager.

Non-gaps:

- Drop-on-full is intentional non-intrusion.  
- Explainer living outside Manager is fine (stateless formatter).  
- ToolEvent always-on is product design.

---

## 9. Verification

```powershell
go test ./internal/transparency/...
go test -race ./internal/transparency/...
```

Spot-check consumers when changing event shapes:

```powershell
go test ./cmd/nerd/chat/ -count=1 -run "GlassBox|Transparency|ToolEvent"
go test ./internal/core/ -count=1 -run "ToolEvent|GlassBox"
```

---

## 10. Document index

| Doc | Contents |
|-----|----------|
| [README.md](README.md) | Map + verify |
| [00-ALIGNMENT-VISION-REVIEW.md](00-ALIGNMENT-VISION-REVIEW.md) | Scores |
| [01-VISION.md](01-VISION.md) | Target vision |
| [02-CURRENT-STATE.md](02-CURRENT-STATE.md) | Inventory |
| [03-GAP-ANALYSIS.md](03-GAP-ANALYSIS.md) | Gaps |
| [04-ARCHITECTURAL-PRINCIPLES.md](04-ARCHITECTURAL-PRINCIPLES.md) | Principles |
| [05-INTERNAL-ARCHITECTURE.md](05-INTERNAL-ARCHITECTURE.md) | Components |
| [06-PUBLIC-API-AND-TYPES.md](06-PUBLIC-API-AND-TYPES.md) | API |
| [07-DEPENDENCY-MAP.md](07-DEPENDENCY-MAP.md) | Deps |
| [08-WIRING-AND-INTEGRATION.md](08-WIRING-AND-INTEGRATION.md) | Wiring |
| [09-SAFETY-AND-INVARIANTS.md](09-SAFETY-AND-INVARIANTS.md) | Invariants |
| [10-TESTING-ALIGNMENT.md](10-TESTING-ALIGNMENT.md) | Tests |
| [11-OBSERVABILITY.md](11-OBSERVABILITY.md) | Ops |
| [12-FAILURE-MODES.md](12-FAILURE-MODES.md) | Failures |
| [TODO.md](TODO.md) | Backlog |
| [OPEN-QUESTIONS.md](OPEN-QUESTIONS.md) | Questions |

---

## 11. Changelog note for this rebuild

Corpus rewritten **2026-07-13** from full source read of all 8 package files plus reverse-dependency grep of chat boot, VirtualStore, ShardManager, and system router. Replaces thin auto-inventory stubs with behavioral narrative.
