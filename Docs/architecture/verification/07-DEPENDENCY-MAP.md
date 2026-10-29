# verification — Dependency Map

> Last verified: **2026-07-13**

## Upstream (this package imports)

```
internal/verification
  ├─ codenerd/internal/autopoiesis     # Orchestrator.GenerateTool
  ├─ codenerd/internal/core/shards     # ShardManager List/Spawn
  ├─ codenerd/internal/logging         # StoreError, SystemShardsWarn
  ├─ codenerd/internal/perception      # LLMClient.CompleteWithSystem
  ├─ codenerd/internal/session         # TaskExecutor, TaskRequest
  ├─ codenerd/internal/store           # LocalStore.StoreVerification
  ├─ context
  ├─ crypto/sha256
  ├─ encoding/hex
  ├─ encoding/json
  ├─ errors
  ├─ fmt
  ├─ strings
  └─ sync
```

| Import | Symbols used | Required? |
|--------|--------------|-----------|
| `perception` | `LLMClient` | Soft — nil soft-succeeds verify |
| `store` | `*LocalStore` | Soft — nil skips persist |
| `core/shards` | `*ShardManager` | Soft for list; hard if no TaskExecutor either |
| `session` | `TaskExecutor`, `TaskRequest` | Preferred hard path when set |
| `autopoiesis` | `*Orchestrator`, `ToolNeed`, `GenerateTool` | Soft — tool corrective only |
| `logging` | category helpers | Soft — diagnostics |

**No imports of:** `cmd/…`, `internal/core` kernel, `internal/prompt`, `internal/mangle`, `internal/articulation`.

## Downstream (who imports verification)

| Consumer | Files | Use |
|----------|-------|-----|
| `cmd/nerd/chat` | `session_boot.go`, `session_shared_boot.go` | Construct + SetTaskExecutor |
| `cmd/nerd/chat` | `model_types.go`, `model_update.go` | Hold `*TaskVerifier` on model/config |
| `cmd/nerd/chat` | `process.go` | `VerifyWithRetry` on mutation delegate |
| `cmd/nerd/chat` | `helpers.go` | Format result/escalation using types |

Evidence command:

```powershell
rg "codenerd/internal/verification" -g "*.go" --glob "!*_test.go"
```

## Sibling persistence (store side)

```
internal/store
  local_core.go            # CREATE TABLE task_verifications
  local_verification.go    # StoreVerification, GetVerificationHistory,
                           # GetQualityViolationStats
```

Verification package is a **writer**. Store is the **reader** for any future learning UI/CLI.

## Dependency graph (runtime)

```
                    perception.LLMClient
                           ▲
                           │
autopoiesis.Orchestrator ◄─┤
                           │
session.TaskExecutor  ◄────┤── TaskVerifier
                           │
core/shards.ShardManager ◄─┤
                           │
store.LocalStore  ◄────────┘
                           │
                    cmd/nerd/chat process
                           │
              kernel.LoadFacts (via ResultToFacts)
              articulation.ProcessLLMResponseAllowPlain (format only)
```

## Coupling notes

| Coupling | Risk | Mitigation in code |
|----------|------|--------------------|
| Persona map duplicated with chat | Drift | Comment + tests on normalize; no shared import |
| LLMClient interface | Mockability good | nil path tested |
| Concrete ShardManager / LocalStore | Harder unit isolation | nil guards |
| Autopoiesis concrete | Tool path only | optional |
| Chat string-matches error text | Brittle if message changes | Prefer errors.Is (gap) |

## Cycles

None observed. Package carefully avoids importing chat despite mirrored persona map.

## Transitive heavy weights

Pulling `verification` transitively pulls session + shards + store + autopoiesis + perception surfaces. Acceptable for chat boot; avoid importing verification from leaf libraries that should stay light.
