# 08 — Wiring and Integration (`internal/logging`)

> Last verified: **2026-07-13**

## 1. Boot wiring

### 1.1 Process entry (`cmd/nerd/main.go`)

```
main()
  logging.Initialize(cwd)          // eager; ignore error
  config.GlobalConfig()            // features, etc.
  observability.LogStartupMetrics()
  rootCmd.Execute()
```

Comment in `main.go`: Initialize is idempotent so PersistentPreRun / interactive chat can call again safely.

### 1.2 Non-interactive Cobra commands

`PersistentPreRunE` (skipped for pure interactive root):

1. Build zap production logger (verbose → debug)  
2. `logging.Initialize(workspace or cwd)` — warn on failure, continue  
3. Optionally load `config.yaml` for timeout (uses `logging.BootDebug` for diagnostics)

`PersistentPostRun`:

1. zap `Sync`  
2. `logging.CloseAll()` — **category files only**

### 1.3 Interactive chat

Root `RunE` does **not** use PersistentPreRun logging branch (skip condition for interactive). Chat boot:

- `cmd/nerd/chat/session_boot.go` → `logging.Initialize(workspace)`  
- `cmd/nerd/chat/session_shared_boot.go` → same  

Long-lived TUI sessions therefore depend on chat boot for file logging; process-level early `main` Initialize may have already run with **cwd** workspace before `Chdir(--workspace)`.

**Implication:** If operator starts `nerd --workspace D:\other` and Once already fired on CWD, logs may target the first workspace. This is a real wiring edge case (see gaps).

## 2. Config wiring

| Loader | Path | Consumed by logging package? |
|--------|------|------------------------------|
| `logging.loadConfig` | `.nerd/config.json` | **Yes** |
| `config.Load` YAML | `.nerd/config.yaml` | **No** (not directly) |
| `config.GlobalConfig` JSON features | user config | **No** for category files |

Operators must ensure `debug_mode` appears in the JSON file this package reads.

Example from live trees: `.nerd/config.json` may include `"json_format": false` under `logging`.

## 3. Call-site wiring patterns

### Pattern A — convenience

```go
logging.Boot("starting scan")
logging.WorldError("parse failed: %v", err)
```

### Pattern B — Get + category

```go
logging.Get(logging.CategoryMangle /* or Kernel */).Info(...)
```

### Pattern C — Timer around hot work

```go
t := logging.StartTimer(logging.CategoryStore, "ValidateAgentDB")
defer t.Stop()
```

Evidence: `internal/init/validation.go`, `shared_kb.go`.

### Pattern D — Audit scoped to session/shard

```go
logging.AuditWithContext(sessionID, shardID, cat).ActionComplete(...)
```

### Pattern E — LLM I/O at provider boundary

Callers at perception/articulation/API should wrap LLM invocations with `LogLLMRequest` / `LogLLMResponse` / `LogLLMError` when tracing is desired.

## 4. Kernel / VirtualStore / shards

| Surface | Logging role |
|---------|--------------|
| Kernel | Callers log asserts/queries; audit helpers available |
| VirtualStore | Category `virtual_store`; action audit helpers |
| Shards | Categories per domain + `shards`; lifecycle audit helpers |
| Perception → articulation | Categories + optional LLM I/O |

No registration table inside logging — **pull** model (callers invoke).

## 5. Not wired

| Expected integration | Status |
|----------------------|--------|
| Auto-load audit.mg into kernel each turn | **Not wired** |
| CloseAudit on CLI PersistentPostRun | **Not wired** |
| CloseLLMIOLogger on chat exit | **Not wired** (OS FD close) |
| Sync config.yaml format → json_format | **Not wired** |
| Prompt-atom selection based on log volume | **Not a goal** |

## 6. Wiring audit checklist for new features

When adding a subsystem:

1. Prefer existing `Category` over inventing parallel log dirs  
2. If new domain: add constant + convenience + document in architecture corpus  
3. For multi-step work: Timer + start/end Info  
4. For safety-sensitive actions: `Audit().SafetyCheck` next to real `permitted` check (does not replace it)  
5. For LLM boundaries: optional `LogLLM*` behind `trace_llm_io`  
6. Grep that debug paths do not log secrets  

## 7. Fact-flow position (reminder)

```
user_intent → kernel → next_action → VirtualStore → articulation
                 │           │              │
                 └──── logging side channel only ───┘
```

Logging is never a step in the executive pipeline.
