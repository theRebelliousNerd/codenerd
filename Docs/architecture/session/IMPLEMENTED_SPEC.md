# session — Implemented Spec (Deep-Dive)

> Last verified against codebase: 2026-07-13  
> Status: Living Reference Document  
> Language: Go  
> Module: `codenerd`  
> Primary sources: `internal/session/`  
> Scale: **6** non-test Go ≈ **3,149** lines; **14** test files; **0** package-local `.mg`  
> Flagship component: **`Executor`** (`executor.go` + `executor_tools.go`)

---

## 1. Overview

`internal/session` is codeNERD’s **clean execution loop**: a unified, JIT-driven agent runtime that replaced tens of thousands of lines of hardcoded domain-shard Go with a thin executive shell around:

| Concern | Owner outside session | Session role |
|---------|----------------------|--------------|
| Natural-language → intent | `perception.Transducer` | Call + assert `user_intent` |
| Prompt specialization | `prompt` JIT compiler | Build `CompilationContext`, compile, fall back |
| Tool/policy identity | `jit/config` + ConfigFactory | Generate / inject `EffectiveAgentRuntimeConfig` |
| Constitutional permission | Mangle policy via `types.Kernel` | Assert `pending_action`, query `permitted` |
| Tool implementation | `tools.Global()`, Ouroboros `core.ToolRegistry` | Route, timeout, allow-list |
| Side-effect executive layers | `core.VirtualStore` as `InteractiveExecutiveGate` | Preflight + post-validate on interactive path |
| Surface/control envelope | `articulation` | Piggyback parse, mangle_updates filter |

Package slogan (from `executor.go` package comment):

> No shards. No spawn. No factories. Clean.

In practice, **spawn and factories still exist** as higher-level surfaces (`Spawner`, `ConfigFactory` interface), but domain behavior no longer lives as giant Go shard classes. Specialization is JIT config + prompt atoms.

### 1.1 Key characteristics

| Property | Value |
|----------|-------|
| Primary entry (interactive turn) | `Executor.Process` / `ProcessWithIntent` |
| Primary entry (system tasks) | `TaskExecutor` → `JITExecutor` |
| Parallel workers | `Spawner` → `SubAgent` (own history, shared kernel/store) |
| Safety default | `EnableSafetyGate: true`, fail-closed if kernel nil |
| Tool iteration caps | `MaxToolCalls=50`, `MaxToolIterations=8`, `ToolTimeout=5m` |
| Token budget default | `DefaultTokenBudget = 65536` |
| History cap (executor) | 50 turns |
| Compression (subagent) | threshold 10 → LLM summary + recent half |
| Logging category | `logging.CategorySession` (`"session"`) |

### 1.2 High-level control flow

```
                    ┌─────────────────────────────────────────┐
                    │  Process / ProcessWithIntent            │
                    └─────────────────┬───────────────────────┘
                                      │
           ┌──────────────────────────┼──────────────────────────┐
           │ OBSERVE                  │ ORIENT                   │ JIT
           │ Transducer or preset     │ CompilationContext       │ prompt+config
           │ assert user_intent       │ kernel world facts       │
           └──────────────────────────┼──────────────────────────┘
                                      │
                           ┌──────────▼──────────┐
                           │   runToolLoop       │
                           │ LLM ↔ tools (≤8)    │
                           └──────────┬──────────┘
                                      │
              ┌───────────────────────┼───────────────────────┐
              │ checkSafety           │ modular / Ouroboros   │ executive gate
              │ pending_action        │ tools.Global()        │ preflight+validate
              │ query permitted       │ ToolRegistry          │
              └───────────────────────┼───────────────────────┘
                                      │
                           ┌──────────▼──────────┐
                           │ Piggyback control   │
                           │ history + persist   │
                           │ taxonomy learning   │
                           └─────────────────────┘
```

Fact-flow alignment with the product north star:

```
user_intent → kernel (policy/orient facts) → next creative step (LLM)
  → tool calls gated by permitted(...) → VirtualStore/tools → articulation
```

Session is **not** a second kernel. It is the **runtime that asks the kernel** and **executes only what policy allows**.

---

## 2. Implementation status

| Component | Status | Notes |
|-----------|--------|-------|
| `Executor` clean loop | **Implemented** | `executor.go` Process/ProcessWithIntent |
| Native multi-turn tool loop | **Implemented** | `ToolResultsProvider` path in `runToolLoop` |
| Piggyback tool path | **Partial** | Single-round tools; no multi-iter Piggyback feedback |
| Constitutional `checkSafety` | **Implemented** | Fail-closed; safe_action payload fallback |
| `InteractiveExecutiveGate` | **Implemented** | Optional type-assert on VirtualStore |
| No-tool-call retry (Mangle + atom) | **Implemented** | `intent_requires_tool_call` + JIT nudge |
| Ouroboros dual registry | **Implemented** | Catalog + execute; generation handoff is assert-only |
| Session turn persistence | **Partial** | Async `SessionPersister`; atomsJSON empty |
| Piggyback memory ops | **Partial** | Asserted/logged; Cold Storage not fully wired |
| `Spawner` lifecycle | **Implemented** | Pending-spawn reservation, Cleanup, metrics |
| Specialist load from `.nerd/agents/` | **Implemented** | Path-traversal + 1MB guards |
| `SubAgent` isolation | **Implemented** | Own history; shared kernel carefully scoped |
| Memory compression | **Implemented** | `SemanticCompressor` + threshold trim fallback |
| `TaskExecutor` / `JITExecutor` | **Implemented** | Inline clone vs subagent; async wait/stop |
| Intent verb normalization | **Implemented** | CLI shard-type aliases → `/verb` |
| Preset intent for delegated tasks | **Implemented** | Skips perception; task-scoped intent IDs |
| `CloneForTask` isolation | **Implemented** | Prevents history contamination |
| Package-local Mangle | **N/A** | Queries global policy corpus only |
| Full Piggyback multi-turn | **Gap** | Documented as future work in code comments |

**Overall:** production living package — **not** pre-implementation.

---

## 3. Source inventory

### 3.1 Layout

```
internal/session/
  executor.go              # Executor, Process*, JIT helpers, Piggyback, history, persist
  executor_tools.go        # runToolLoop, safety, tool route, no-tool retry
  spawner.go               # Spawner, specialist load, intent→name maps
  subagent.go              # SubAgent lifecycle, metrics, compression
  task_executor.go         # TaskExecutor, JITExecutor, verb normalize
  semantic_compressor.go   # LLM history summarizer
  README.md                # Package-level overview (slightly stale slogans)
  *_test.go                # 14 test files + mocks_test.go
```

### 3.2 Non-test sources (approximate line counts)

| Path | Lines | Role |
|------|------:|------|
| `internal/session/executor.go` | ~1057 | Core loop, interfaces, Piggyback, persistence |
| `internal/session/executor_tools.go` | ~587 | Tool loop + constitutional gate |
| `internal/session/spawner.go` | ~517 | Subagent factory + registry |
| `internal/session/subagent.go` | ~471 | Isolated runner + memory |
| `internal/session/task_executor.go` | ~419 | Unified task API for Cortex/campaigns |
| `internal/session/semantic_compressor.go` | ~104 | Compressor implementation |
| **Total non-test** | **~3149** | |

### 3.3 Test inventory

| Path | Focus |
|------|--------|
| `executor_test.go` | checkSafety unit paths, extractTarget |
| `executor_process_test.go` | Process end-to-end with mocks; fail-closed; caps |
| `executor_boundary_test.go` | Payload size, empty name, nil args |
| `executor_mangle_test.go` | parseMangleArg(s) |
| `check_safety_real_kernel_test.go` | Real kernel glob/write |
| `check_safety_write_large_test.go` | Large write_file payload path |
| `spawner_test.go` | Spawn success, max limit, lifecycle |
| `spawner_improvements_test.go` | Concurrency, config fallback |
| `spawner_gaps_test.go` | TOCTOU, StopAll races, GetByName |
| `spawner_config_test.go` | Specialist YAML load |
| `subagent_test.go` | Run, compress, cancel, double-stop |
| `task_executor_test.go` | Inline/subagent/async, isolation, aliases |
| `semantic_compressor_test.go` | Empty, extremes, timeout, unprintables |
| `mocks_test.go` | Shared mocks for kernel/LLM/VS/JIT |

Cross-package e2e under `tests/e2e/` (session_* , orchestrator_*, piggyback_*, campaign_session_*, etc.) exercise real wiring.

---

## 4. Executor deep dive (flagship)

### 4.1 Type and dependencies

```go
// internal/session/executor.go
type Executor struct {
    mu sync.RWMutex

    kernel       types.Kernel
    virtualStore types.VirtualStore
    llmClient    types.LLMClient

    jitCompiler   JITCompiler
    configFactory ConfigFactory
    transducer    perception.Transducer

    ouroborosRegistry *core.ToolRegistry

    conversationHistory []perception.ConversationTurn
    sessionContext      *types.SessionContext

    sessionPersister SessionPersister
    sessionID        string

    config ExecutorConfig
    EffectiveAgentRuntimeConfig *config.EffectiveAgentRuntimeConfig
}
```

Local interfaces (to avoid tight import cycles / over-coupling):

| Interface | Methods | Purpose |
|-----------|---------|---------|
| `JITCompiler` | `Compile(ctx, *prompt.CompilationContext)` | Prompt atom selection |
| `ConfigFactory` | `Generate(ctx, *CompilationResult, intents...)` | Allowed tools / policies |
| `SessionPersister` | `StoreSessionTurn`, `StoreCompressedState` | Cross-session continuity |
| `InteractiveExecutiveGate` | `PreflightDestructiveToolCall`, `ValidateInteractiveToolResult` | Dreamer + validators on interactive path |

`InteractiveExecutiveGate` is implemented by `*core.VirtualStore` (`virtual_store_interactive_gate.go`). When the store does not implement it, gates are **skipped gracefully**.

### 4.2 Configuration

`DefaultExecutorConfig()`:

| Field | Default | Rationale |
|-------|---------|-----------|
| `MaxToolCalls` | 50 | Hard budget per Process |
| `MaxToolIterations` | 8 | LLM↔tools rounds |
| `ToolTimeout` | 5 minutes | Per tool call |
| `EnableSafetyGate` | true | Constitutional default |
| `TokenBudget` | 65536 (`DefaultTokenBudget`) | Avoid silent atom drop at 8192 |

Historical bug (documented in comments): hardcoded 8192 token budget caused spawned agents to lose mandatory prompt atoms on modern long-context models.

### 4.3 Construction and mutators

| API | Behavior |
|-----|----------|
| `NewExecutor(...)` | Wires deps; empty history; default config |
| `SetSessionContext` | Dream mode / blackboard (legacy stateful) |
| `CloneForTask` | Shares deps; **isolates** history, session, persister, agent config |
| `SetConfig` / `SetAgentConfig` | Caps / inject precompiled config (SubAgent path) |
| `SetHistory` / `GetHistory` / `ClearHistory` | Conversation isolation |
| `SetOuroborosRegistry` | Dual-registry Piggyback++ |
| `SetSessionPersister` / `SetSessionID` | Turn storage |

**Why CloneForTask exists:** inline task execution used to run on the shared session executor, which (a) raced on `SetSessionContext` and (b) contaminated interactive history with delegated task turns.

### 4.4 Process / ProcessWithIntent

`Process(ctx, input)` → `ProcessWithIntent(ctx, input, nil)`.

#### Phase A — Guards

- Empty input → error  
- nil context → `context.Background()`  
- Already-cancelled context → error  

#### Phase B — OBSERVE

| Mode | Behavior |
|------|----------|
| `preset == nil` | `transducer.ParseIntentWithContext(ctx, input, history)` |
| `preset != nil` | Skip perception LLM; use routing-layer intent |

**Why preset matters:** delegated tasks arrive with verb already decided. Re-classifying the machine-generated task string burned an LLM call **and** could pick the wrong persona.

#### Phase C — Intent facts on shared kernel

```
user_intent(IntentID, Category, Verb, Target, Constraint)
```

| Run kind | Intent ID | Cleanup |
|----------|-----------|---------|
| Interactive | `/current_intent` | Left for policy/session |
| Task (`preset != nil`) | `/task_intent_<counter>` | `defer RetractFact` |

SubAgents share the session kernel; concurrent writes to `/current_intent` would clobber the interactive turn mid-flight. Task-scoped IDs are the isolation strategy.

#### Phase D — ORIENT (`buildCompilationContext`)

Builds `prompt.CompilationContext`:

- `IntentVerb`, `IntentTarget`  
- `OperationalMode`: `/active` or `/dream` (from request-scoped `types.GetSessionContext` **or** legacy field)  
- `TokenBudget` from config  
- Optional world: `FailingTestCount` from `test_state(/failing)`, `DiagnosticCount` from `diagnostic`  

#### Phase E — JIT prompt

| Condition | Result |
|-----------|--------|
| `jitCompiler == nil` | Baseline: “You are an AI assistant…” |
| Compile error | Same baseline (warn log) |
| Success | Full atom-selected system prompt |

#### Phase F — JIT config (`compileConfig`)

| Condition | Result |
|-----------|--------|
| Injected `EffectiveAgentRuntimeConfig` | Use as-is (SubAgent) |
| `configFactory == nil` | Empty config |
| Factory error | Empty config + warn (LLM may still chat) |
| Success | Allowed tools list drives tool defs |

Empty allowed-tools is **not** fail-closed for chat: the model can still produce text without tools.

#### Phase G — Tool loop (`runToolLoop` in `executor_tools.go`)

See §5.

#### Phase H — Articulate / post-process

1. `processPiggybackControlPacket` extracts surface text + control side-effects  
2. History append user + assistant (incl. thought summary/signature if present)  
3. Cap history at 50 turns  
4. `perception.SharedTaxonomy.QueueForLearning` if available  
5. `persistTurn` async if persister set  
6. If tools errored **and** final response empty → set `result.Error`  

Returns `*ExecutionResult` with Response, Intent, ToolCallsExecuted, Duration, Error.

### 4.5 ExecutionResult

```go
type ExecutionResult struct {
    Response          string
    Intent            perception.Intent
    ToolCallsExecuted int
    Duration          time.Duration
    Error             error
}
```

---

## 5. Tool loop architecture

### 5.1 `runToolLoop` decision tree

```
generateResponse(...)
  │
  ├─ no tool_calls?
  │    ├─ intentRequiresToolCall(verb)?  // Mangle: intent_requires_tool_call/1
  │    │    └─ retryWithNoToolNudge once (world_state no_tool_call_retry atom)
  │    └─ else return text
  │
  ├─ PiggybackToolProvider.ShouldUsePiggybackTools()?
  │    └─ executeToolBatchPiggyback (single round; no results feedback)
  │
  └─ native path
       for iter < MaxToolIterations:
         execute each ToolCall (budget MaxToolCalls)
         if !ToolResultsProvider → return after first batch (warn)
         CompleteWithToolResults(...)
         if no more tool_calls → return final
       warn max iterations
```

### 5.2 Why no-tool retry exists

Observed failure mode (comment cites `.nerd/logs/2026-05-28`): model returns planning-only text for a `/create` intent; orchestrator marks step complete with **zero side effects**.

Mitigation is **neuro-symbolic**, not a hardcoded Go string:

1. Policy derives `intent_requires_tool_call/1` from `action_mapping/2` + `side_effecting_action/1`  
2. Go queries kernel; if true and no tools, clone compilation context  
3. Set `PreviousAttemptNoToolCall` + `AvailableTools`  
4. JIT injects `system/tool_nudge/no_tool_call_retry` atom with `{{available_tools}}`  
5. Single reissue  

If kernel unavailable, `intentRequiresToolCall` returns **false** (do not block final answers on missing policy).

### 5.3 `generateResponse` dual mode

| Mode | When | Mechanism |
|------|------|-----------|
| Piggyback++ | `PiggybackToolProvider` + `ShouldUsePiggybackTools()` | Inject tool catalog into system prompt; `CompleteWithSystem`; parse control packet tool_requests |
| Native tools | Default with AllowedTools | `CompleteWithTools` + ToolDefinition schemas from modular registry |
| Text only | No tools | `CompleteWithSystem` |

Piggyback exists so tool use coexists with Gemini grounding tools that cannot mix with native function calling.

### 5.4 Tool catalog (Piggyback)

`buildToolCatalogForPiggyback` merges:

1. Modular tools from `tools.Global()` filtered by `cfg.AllowedTools`  
2. Ouroboros tools from `ouroborosRegistry.ListTools()`  

Plus encouragement to emit `missing_tool_for(...)` mangle_updates for generation. Uses `catalogBuilderPool` (`sync.Pool` of `strings.Builder`) for hot long sessions.

### 5.5 `executeToolCall` pipeline

Strict order:

1. **Allow-list**: modular name in config **or** Ouroboros tool exists  
2. **Constitutional gate** (`checkSafety`) if enabled  
3. **Preflight** `InteractiveExecutiveGate.PreflightDestructiveToolCall`  
4. **Timeout** context (`ToolTimeout`)  
5. **Execute** modular registry first, else Ouroboros JSON-args binary  
6. **Post-validate** `ValidateInteractiveToolResult` (modular success path)  

Modular path returns `result.Result` string; Ouroboros returns raw string. Tool results fed back to native loop are truncated at **16 KiB** (`truncateToolResult`).

### 5.6 Allow-list semantics

```go
// isToolAllowed: empty AllowedTools means unrestricted (true)
```

This is intentional for bootstrap / empty config fallback, but means a failed ConfigFactory can widen tools. Safety still depends on `checkSafety` + executive gate when enabled.

---

## 6. Constitutional safety (`checkSafety`)

### 6.1 Algorithm

Located in `executor_tools.go`. Invoked only when `EnableSafetyGate` is true.

| Step | Behavior |
|------|----------|
| Empty tool name | **Deny** (would become `/` atom) |
| Kernel nil + gate on | **Deny fail-closed** (“god mode” prevention) |
| Kernel nil + gate off | Allow |
| Normalize action | Prefix `/` if missing → `MangleAtom` |
| Args nil | `{}` JSON (not `null`) for permitted match |
| Extract target | keys: path, filename, filepath, file, url, target, query, pattern, glob, dir, directory |
| Payload | JSON marshal; **>100KB → deny** (no silent truncate) |
| Assert | `pending_action(ActionID, ActionType, Target, Payload, Timestamp)` |
| Query | all `permitted` facts; match action+target+payload exactly |
| Cleanup | `defer RetractFact(pending)` |
| Fallback | If exact payload miss but `safe_action` holds for verb → **allow with warn** |
| Else | Deny |

### 6.2 Why safe_action fallback

Large `write_file` contents can fail exact payload match when Mangle string-normalizes escapes/truncation, even though the action is constitutionally safe. Fallback remains **fail-closed for unknown tools**.

### 6.3 Mangle predicates touched by session (not Decl’d here)

Session **asserts/queries** global policy surface:

| Predicate | Direction | Purpose |
|-----------|-----------|---------|
| `user_intent` | assert | Orient routing |
| `pending_action` | assert+retract | Safety gate input |
| `permitted` | query | Constitutional allow |
| `safe_action` | query | Payload-match fallback |
| `intent_requires_tool_call` | query | No-tool retry |
| `test_state` / `diagnostic` | query | Compilation context |
| `self_correction` | assert | Piggyback autopoiesis signal |
| `memory_operation` | assert | Future cold storage |
| Filtered Piggyback updates | assert | `missing_tool_for`, `observation`, task/diagnostic/review/modified… |

Allowed Piggyback mangle predicates are hard-coded in `processMangleUpdatesFromEnvelope` via `core.MangleUpdatePolicy` (max 100). Blocked updates get constitutional override on the envelope.

---

## 7. Spawner and SubAgent

### 7.1 Spawner

`Spawner` holds shared deps and a map of active `*SubAgent`.

**Spawn capacity protocol:**

1. Lock; if `countActive()+pendingSpawns >= max` → error  
2. Increment `pendingSpawns` (reservation)  
3. Unlock; generate JIT config (may be slow)  
4. Create `SubAgent`  
5. Lock; decrement pending; re-check active; register  

Default: `MaxActiveSubagents=10`, token budget `DefaultTokenBudget`.

**Entry points:**

| Method | Use |
|--------|-----|
| `Spawn(ctx, SpawnRequest)` | Full control; does **not** auto-start Run |
| `SpawnForIntent(intent, task)` | Maps verb → name/type then Spawn |
| `SpawnSpecialist(name, task)` | Load `.nerd/agents/{name}/config.yaml`; auto `go agent.Run` |

Specialist load guards:

- Reject names with `..` or path separators  
- Max YAML size 1MB  
- Prefer `virtualStore.ReadRaw`; fallback `os.ReadFile`  
- Missing file → ConfigFactory with empty prompt shell  

**Intent → name map** (`determineAgentName`):

| Verbs | Name |
|-------|------|
| `/fix`, `/implement`, `/refactor`, `/create` | coder |
| `/test`, `/cover`, `/verify` | tester |
| `/review`, `/audit`, `/check` | reviewer |
| `/research`, `/learn`, `/document` | researcher |
| else | executor |

Category `/system` → `SubAgentTypeSystem`; else ephemeral.

Lifecycle ops: `Get`, `GetByName` (first non-terminal), `Stop`, `StopAll`, `Cleanup`, `ListActive`, `GetMetrics`.

### 7.2 SubAgent state machine

```
IDLE ──Run()──► RUNNING ──success──► COMPLETED
                    │
                    └──error/cancel──► FAILED
```

States are `int32` atomic (`SubAgentState`).

`Run` is async:

- Optional `CtxKeyModelCapability` hint from agent name (coder/reviewer high reasoning; tester high speed; researcher balanced)  
- Timeout from config (default 30m in `DefaultSubAgentConfig`; spawner may override via `appconfig.GetLLMTimeouts().ShardExecutionTimeout`)  
- `execute` → `ProcessWithIntent` with preset intent when `IntentVerb` set  
- Sync history back; `CompressMemory(ctx, 10)`  

`Wait` / `WaitWithContext` poll every 100ms. `Stop` cancels context.

### 7.3 Memory compression

When history length > threshold:

1. Keep last `max(threshold/2, 1)` turns  
2. Compress older via `Compressor` (default `SemanticCompressor`)  
3. Reconstruct: `[MEMORY SUMMARY] …` + recent  
4. On compress failure: hard trim to threshold  
5. Cap summary at 4096 chars  

Compressor releases lock during LLM call to avoid blocking Stop/metrics.

---

## 8. TaskExecutor / JITExecutor

### 8.1 Interface

```go
type TaskExecutor interface {
    Execute(ctx, TaskRequest) (string, error)
    ExecuteWithContext(ctx, TaskRequest, *SessionContext, SpawnPriority) (string, error)
    ExecuteAsync(ctx, TaskRequest) (taskID string, err error)
    GetResult(taskID) (result string, done bool, err error)
    WaitForResult(ctx, taskID) (string, error)
}
```

Migration narrative in source: consumers move from `ShardManager.Spawn` → `TaskExecutor`; `JITExecutor` is the current implementation.

`SpawnConsultation` on `JITExecutor` implements `shards.ConsultationSpawner` via `/consult/<name>` intents.

### 8.2 Intent normalization

`normalizeTaskIntentVerb` accepts:

- Already-canonical `/fix`  
- Bare shard types: coder→`/fix`, tester→`/test`, reviewer→`/review`, researcher→`/research`, generalist→`/implement`, nemesis→`/review`, image generators→`/create`, tool_generator→`/generate_tool`  
- Other bare identifiers → `/lowercase`  
- Invalid whitespace → error  

### 8.3 Execute routing

```
ExecuteWithContext
  normalize verb
  inject CtxKeyPriority if missing
  if DreamMode → always subagent
  if needsSubagent(verb) → subagent   // /research,/implement,/refactor,/campaign
  else CloneForTask + ProcessWithIntent(preset)
```

Async path:

1. `Spawn` (without auto-run)  
2. Register `TaskResult{Completed:false}` **before** `go agent.Run` (TOCTOU fix)  
3. Return task ID  

`WaitForResult` on cancel: **Stop** subagent to prevent zombie LLM spend.

### 8.4 Agent name map (JITExecutor)

Extends spawner map with nemesis (`/attack`), legislator (`/legislate`), planner (`/plan`), consult strip.

---

## 9. Semantic compressor

`SemanticCompressor` formats turns as XML-ish `<turn role="…">` blocks (skip empty, strip unprintables), truncates ~64k chars, asks LLM to summarize decisions/facts/task state. System role: “context compressor”.

Used by SubAgent default; injectable via `SetCompressor`.

---

## 10. Integration map (who calls session)

| Consumer | How |
|----------|-----|
| `internal/system.factory` `initFinalExecutors` | Builds Executor, Spawner, JITExecutor; wires VS task delegator; optional DB persister |
| `Cortex` fields | `TaskExecutor`, `SessionExecutor`, `SessionSpawner` |
| `Cortex.SpawnTask` / related | Routes through TaskExecutor |
| `cmd/nerd/chat` | Boot helpers, delegation_routing → TaskRequest, model_types hold session refs |
| `cmd/nerd/cmd_campaign.go` | Local Executor/Spawner/JITExecutor for campaign orchestrator |
| `internal/campaign` | Orchestrator types/handlers depend on TaskExecutor |
| `internal/verification` | Imports session (verifier wiring) |
| `tests/e2e/*` | Heavy integration suite |

Adapters in `internal/system` (`sessionKernelAdapter`, `sessionVirtualStoreAdapter`, `sessionLLMAdapter`, `taskDelegatorAdapter`) bridge Cortex concrete types to session interfaces without import cycles.

Worker LLM note: task path may use `shardLLMClient` (local Ollama worker) instead of main TUI client when configured (`initFinalExecutors` comments).

---

## 11. Concurrency model

| Component | Locking |
|-----------|---------|
| Executor | `sync.RWMutex` for history, config, registries, session fields |
| Spawner | `sync.RWMutex` + pending spawn counter |
| SubAgent | `sync.RWMutex` + atomic state |
| JITExecutor results | `sync.RWMutex` |
| Task intent IDs | `atomic.AddUint64` |
| Catalog builders | `sync.Pool` |
| Persist turn | fire-and-forget goroutine |

Shared kernel between concurrent SubAgents is **intentional** with task-scoped intent IDs; tool safety still asserts/retracts `pending_action` per call (serialization depends on kernel implementation).

---

## 12. Gaps pointer

See [03-GAP-ANALYSIS.md](03-GAP-ANALYSIS.md) and [TODO.md](TODO.md). Highest-impact residual gaps:

1. Piggyback multi-turn tool feedback incomplete  
2. Memory operations / atomsJSON persistence partial  
3. Empty AllowedTools = unrestricted (with safety gate still on)  
4. Package README still claims “No spawn” while Spawner is first-class  
5. `SpawnSpecialist` vs `Spawn` auto-start inconsistency  
6. Wait-by-polling rather than completion channels  

---

## 13. Related corpora

| Corpus | Relationship |
|--------|--------------|
| `Docs/architecture/core/` | Kernel, VirtualStore, InteractiveExecutiveGate, ToolRegistry |
| `Docs/architecture/prompt/` | JIT compiler, atoms, config factory |
| `Docs/architecture/perception/` | Transducer, Intent, taxonomy learning |
| `Docs/architecture/articulation/` | Piggyback envelope processing |
| `Docs/architecture/tools/` | Modular tool registry |
| `Docs/architecture/campaign/` | Orchestrator uses TaskExecutor |
| `Docs/architecture/cli/` | Chat boot + delegation consumers |
| `Docs/architecture/shards/` | ConsultationManager, residual shard manager |
| `internal/session/README.md` | Package-local overview (verify against this corpus) |

---

## 14. Verify commands

```powershell
go test ./internal/session/...
go test ./tests/e2e/ -run "Session|Executor|Piggyback|Orchestrator" -count=1
```

---

## 15. Change discipline

When editing session:

1. Prefer extending ProcessWithIntent / TaskExecutor seams over new parallel loops  
2. Keep safety fail-closed; do not “fix open for tests” without explicit gate disable  
3. New LLM-facing behavior → prompt atoms (JIT), not hardcoded system strings in Go  
4. Audit wiring (Cortex, VirtualStore SetTaskExecutor, campaign) before declaring unused  
5. Run unit tests; for safety changes prefer real-kernel tests  

---

## 16. Quick reference: Process phase checklist

| # | Phase | Code | Fail behavior |
|---|-------|------|---------------|
| 1 | Guard input/ctx | `ProcessWithIntent` | Error return |
| 2 | Observe / preset | `observe` or preset | Error if transducer fails |
| 3 | Assert intent | kernel.Assert | Warn continue |
| 4 | Compilation context | `buildCompilationContext` | Always builds |
| 5 | JIT prompt | `jitCompiler.Compile` | Baseline prompt |
| 6 | JIT config | `compileConfig` | Empty config |
| 7 | LLM + tools | `runToolLoop` | Fatal LLM error; tool errs collected |
| 8 | No-tool nudge | optional | Best-effort single retry |
| 9 | Safety per tool | `checkSafety` | Block tool |
| 10 | Executive gate | type assert | Skip if absent |
| 11 | Piggyback post | `processPiggybackControlPacket` | Raw text fallback |
| 12 | History / learn / persist | append / Queue / go Store | Best-effort |

This is the authoritative living spec for `internal/session` as of **2026-07-13**.
