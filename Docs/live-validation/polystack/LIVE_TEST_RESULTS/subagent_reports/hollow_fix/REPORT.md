# Hollow / False Success Fix Report

**Repo:** `C:\CodeProjects\codeNERD`  
**Date:** 2026-07-13  
**Scope:** one-shot CLI coding paths — `nerd create`, `nerd run`, `nerd tool generate`, spawn/fix completion criteria

---

## Executive summary

Live matrix evidence showed three related failure modes where codeNERD printed success (exit 0) without durable side effects:

| Command | Symptom | Exit |
|---------|---------|------|
| `nerd create …` | "📋 Result:" with prose claiming files written; workspace unchanged | 0 |
| `nerd run …` | `Result: Next action: /delegate_coder` — no spawn | 0 |
| `nerd tool generate …` | "Generated self-tool …" but `tool list` empty | 0 |

All three are fixed by making success contingent on real execution/side effects and returning non-zero when required work did not land.

---

## Root causes

### 1. `nerd create` / write-oriented spawn (`runDirectAction` + TaskExecutor)

**Files:** `cmd/nerd/cmd_direct_actions.go`, `internal/session/task_executor.go`, `internal/session/executor.go`

1. **Wrong intent verb.** `runDirectAction("coder", "/create")` called `SpawnTask(ctx, "coder", task)`. `normalizeTaskIntentVerb("coder")` always maps to `/fix`, so create/refactor one-shots never used the `/create` intent.
2. **Prose-only completion treated as success.** `runToolLoop` already had a no-tool retry nudge, but after a second prose-only turn it still returned success. `ProcessWithIntent` returned `(result, nil)` even with zero tool calls. `runDirectAction` printed Result and exited 0.
3. **No write-tool accounting.** Even when tools ran (e.g. only `read_file`), write-oriented work could complete without `write_file`/`edit_file`.

Live evidence: `manual_create_go.out` claims `backend/main.go` was created with no tool metrics.

### 2. `nerd run` stops at `/delegate_coder` without executing

**Files:** `cmd/nerd/cmd_instruction.go`, `internal/core/defaults/policy/delegation.mg`

1. Policy derives `next_action(/delegate_coder)` via `action_mapping(/create|/fix|…, /delegate_coder)`.
2. `delegate_task(/coder, …)` rules only covered `/implement` and `/refactor` — **not** `/create`, `/fix`, `/write`, etc.
3. `runInstruction` only executed when `delegate_task` facts existed. On `next_action` alone it printed `"Next action: /delegate_coder"` and returned **nil** (exit 0), while still asserting `task_status(/manual_instruction, /complete)`.

Live evidence: `04_run_makefile.out`:

```json
"observation(/result, \"Next action: /delegate_coder\")"
"task_status(/manual_instruction, /complete)"
```

### 3. `nerd tool generate` hollow success

**Files:** `cmd/nerd/cmd_advanced.go`

1. Generate used `SpawnTask(ctx, "tool_generator", description)` → JIT `/generate_tool` path → model prose ("Generated self-tool …") without Ouroboros compile/register.
2. List only queried kernel `tool_registered` facts (session-local, often empty after boot) instead of the durable Ouroboros registry (`Orchestrator.ListTools()` / disk restore).

Live evidence: generate exit 0 with prose; immediate `tool list` → "No tools generated yet."

---

## Fixes implemented

### A. Hollow-success gate (session executor)

**`internal/session/executor.go` / `executor_tools.go`**

- Track `SuccessfulWriteTools` for successful `write_file` / `edit_file` / peers.
- `checkHollowSuccess`:
  - If intent requires tools (`intent_requires_tool_call` **or** write-oriented verb) and `ToolCallsExecuted == 0` → error.
  - If write-oriented (`/create`, `/fix`, `/refactor`, …) and `SuccessfulWriteTools == 0` → error.
  - Dream mode exempt (speculative).
- Hollow errors are **hard** return values from `ProcessWithIntent` (CLI exit non-zero). Soft tool failures remain on `result.Error` for chat compatibility.
- `JITExecutor.ExecuteWithContext` surfaces `result.Error` to SpawnTask callers.

### B. Direct actions use intent verb

**`cmd/nerd/cmd_direct_actions.go`**

- `SpawnTask(ctx, verb, target)` with `/create` / `/fix` / … instead of persona `"coder"`.
- Print partial result on failure; empty write-oriented result is also blocked.

### C. `nerd run` executes handoffs

**`cmd/nerd/cmd_instruction.go`**

- Execute `delegate_task` (including tool_generator → Ouroboros).
- If no `delegate_task`, map `next_action(/delegate_*)` → shard and **SpawnTask**.
- Non-delegate actions: `VirtualStore.RouteAction` after `DisableBootGuard`.
- Fail if nothing executed; status `/failed` instead of false `/complete`.

**`internal/core/defaults/policy/delegation.mg`**

- Added `delegate_task(/coder, …)` for `/create`, `/fix`, `/write`, `/delete`, `/debug`, `/commit`, `/git`.

### D. Tool generate via Ouroboros + durable list

**`cmd/nerd/cmd_advanced.go`**

- Generate: `DetectToolNeed` → `ExecuteOuroborosLoop`; fail on `!Success`, empty name, or tool not visible to list.
- `buildCLIToolNeed` when detection returns nil (explicit CLI is authoritative).
- List: prefer `Orchestrator.ListTools()`, merge kernel facts.

---

## Tests

| Package | Coverage |
|---------|----------|
| `internal/session` | `hollow_success_test.go` — write-oriented gate, dream exempt, ProcessWithIntent hollow create fails |
| `internal/session` | Updated JIT executor tests for hollow `/fix` / prose-only coder |
| `cmd/nerd` | `hollow_success_test.go` — `nextActionToShardType`, tool name slug, CLI need builder |

```
go test ./internal/session/ -count=1   # PASS
go test ./cmd/nerd/ -count=1 -run "Hollow|NextAction|WriteOriented|ToolName|BuildCLI|DirectActions"  # PASS
go build ./cmd/nerd/                  # PASS
```

---

## Files touched

| Path | Change |
|------|--------|
| `internal/session/executor.go` | SuccessfulWriteTools; hollow hard error |
| `internal/session/executor_tools.go` | write tool tracking; `checkHollowSuccess` |
| `internal/session/task_executor.go` | surface `result.Error` |
| `internal/session/hollow_success_test.go` | **new** |
| `internal/session/task_executor_test.go` | align with hollow rules |
| `cmd/nerd/cmd_direct_actions.go` | spawn by verb; hollow post-check |
| `cmd/nerd/cmd_instruction.go` | execute next_action handoffs; non-zero on failure |
| `cmd/nerd/cmd_advanced.go` | Ouroboros generate + durable list |
| `cmd/nerd/hollow_success_test.go` | **new** |
| `internal/core/defaults/policy/delegation.mg` | coder delegate_task for create/fix/… |

---

## Residual risk / follow-ups

1. **Live LLM still required** for create/run/generate to pass end-to-end; gates only prevent false green.
2. **`/test` hollow:** write-oriented list does not include `/test`; kernel `intent_requires_tool_call(/test)` must be present for prose-only test failures (policy already maps `/test` → `/run_tests` side-effecting).
3. **Piggyback single-turn:** still one-shot tool batch; hollow gate applies after, but multi-turn recovery depends on provider `ToolResultsProvider`.
4. **Tool generate cost:** full Ouroboros loop is intentional for durability; timeouts remain user `--timeout`.
5. **Chat path** already used Ouroboros for `/tool generate`; CLI is now aligned. Interactive create may still soft-surface hollow via `result.Error` in some UI paths — worth a chat-layer consistency pass if TUI also claims "Complete" without writes.

---

## Verification checklist (manual / live matrix)

1. `nerd create "minimal main.go"` without model tools → **non-zero**, message contains `hollow success blocked`.
2. `nerd run "add scripts/dev.ps1 …"` → either spawns coder and completes work, or **non-zero** (never exit 0 with only `Next action: /delegate_coder`).
3. `nerd tool generate "…"` success → `nerd tool list` shows the tool; empty/fail path → non-zero.
4. `nerd review …` prose-only still allowed (not write-oriented).
