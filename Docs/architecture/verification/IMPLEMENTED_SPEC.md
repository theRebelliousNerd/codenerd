# verification — Implemented Spec

> Last verified against codebase: **2026-07-13**  
> Status: Living Reference Document — **code-grounded full corpus**  
> Mode: 1:1 with `internal/verification/`  
> **Implementation: 1 non-test `.go` (~820 LOC), 3 test files (~950 LOC), 0 `.mg`**

---

## 1. Purpose

codeNERD treats the LLM as a **creative center** that will cut corners under pressure: mock handlers, `TODO`, `panic("not implemented")`, invented APIs, empty tests. The **executive** answer is not another free-form chat turn; it is a **deterministic control loop** that:

1. Executes a delegated task through the normal shard/TaskExecutor path  
2. Judges the *result text* for quality violations  
3. On failure: re-selects a better shard, gathers corrective context, re-enriches the task, retries  
4. On exhaustion: returns `ErrMaxRetriesExceeded` so the chat layer escalates to the human  

Package comment (source of truth):

> *Package verification implements the quality-enforcing verification loop. This ensures tasks are completed PROPERLY — no shortcuts, no mock code, no corner-cutting. After shard execution, results are verified and automatically retried with corrective action until success or max retries.*

This package is the **post-delegation quality gate** for mutations. It does **not** replace Mangle `permitted(...)` (action legality). It answers a different question: *did the creative path produce real work?*

---

## 2. Implementation status

> Living code status — **not** pre-implementation zeroing.

| Component | Status | Completion | Evidence |
|-----------|--------|------------|----------|
| `TaskVerifier` core loop | Implemented | **95%** | `VerifyWithRetry` |
| LLM verification JSON path | Implemented | **90%** | `verifyTask` + `parseVerificationResponse` |
| Review-vs-impl prompt split | Implemented | **90%** | `isReviewTask` |
| Heuristic quality fallback | Implemented | **85%** | `basicQualityCheck` (subset of violation types) |
| Intent verb normalization | Implemented | **95%** | `normalizeIntentVerb` + tests |
| TaskExecutor preferred path | Implemented | **90%** | `spawnTask` |
| ShardManager fallback | Implemented | **85%** | `spawnTask` when executor nil |
| LLM shard re-selection | Implemented | **80%** | `selectBestShard` + `parseShardSelection` |
| Heuristic shard re-selection | Implemented | **85%** | `heuristicShardSelection` |
| Specialist matching | Implemented | **70%** | `findMatchingSpecialist` (keyword map; needs shardMgr) |
| Corrective: decompose | Implemented | **90%** | pure string hint |
| Corrective: tool via autopoiesis | Implemented | **70%** | `GenerateTool` if orch non-nil |
| Corrective: research/docs | **Partial** | **40%** | specialist re-spawn only; web/Context7 path commented out |
| Persistence | Implemented | **85%** | `storeVerification` → `LocalStore.StoreVerification` |
| Learning *from* history | **Missing** | **10%** | store writes; package does not read history back into decisions |
| Prompt-as-atoms (JIT) | **Missing** | **0%** | large inline system prompts |
| Mangle surface | N/A | **n/a** | no local `.mg`; no Decl predicates |
| Chat gate (`/mutation` only) | Implemented (caller) | **95%** | `shouldVerifyDelegation` in `cmd/nerd/chat` |
| Unit/integration tests | Implemented | **85%** | strong pure-logic coverage; limited live LLM/executor ITs |

**Overall (heuristic): ~82%** as a living, boot-wired production package with known partials on research/docs corrective paths and closed-loop learning.

---

## 3. Source inventory

### 3.1 Package tree

```
internal/verification/
  verifier.go                 # entire implementation (~820 lines)
  verifier_test.go            # core unit tests (~103 lines)
  verifier_normalize_test.go  # intent normalization + SetTaskExecutor (~42 lines)
  verifier_gaps_test.go       # expanded coverage (~801 lines)
```

| Path | ~Lines | Role |
|------|-------:|------|
| `internal/verification/verifier.go` | 820 | All production logic |
| `internal/verification/verifier_gaps_test.go` | 801 | Gap-fill tests (parse, heuristics, enrich, retry edges) |
| `internal/verification/verifier_test.go` | 103 | Baseline unit tests |
| `internal/verification/verifier_normalize_test.go` | 42 | `normalizeIntentVerb` table |

### 3.2 Related (not in package)

| Path | Role |
|------|------|
| `cmd/nerd/chat/process.go` | Call site: mutation + `VerifyWithRetry` |
| `cmd/nerd/chat/delegation_routing.go` | `shouldVerifyDelegation` |
| `cmd/nerd/chat/helpers.go` | `formatVerifiedResponse`, `formatVerificationEscalation` |
| `cmd/nerd/chat/session_boot.go` | Construct + wire verifier |
| `cmd/nerd/chat/session_shared_boot.go` | Shared-boot construct + wire |
| `cmd/nerd/chat/model_types.go` | `model.verifier`, `ChatConfig.Verifier` |
| `internal/store/local_verification.go` | Persistence API |
| `internal/store/local_core.go` | `task_verifications` DDL |
| `internal/session/task_executor.go` | Preferred spawn backend |

---

## 4. Public surface (authoritative)

### Types

| Type | Kind | Location | Notes |
|------|------|----------|-------|
| `QualityViolation` | string enum | `verifier.go:29` | 8 violation kinds |
| `CorrectiveType` | string enum | `verifier.go:43` | research / docs / tool / decompose |
| `CorrectiveAction` | struct | `verifier.go:53` | Type, Query, Reason, ShardHint |
| `ShardSelectionResult` | struct | `verifier.go:61` | Retry shard choice |
| `VerificationResult` | struct | `verifier.go:70` | JSON-compatible LLM output |
| `TaskVerifier` | struct | `verifier.go:81` | Stateful orchestrator |
| `ErrMaxRetriesExceeded` | error var | `verifier.go:26` | Escalation signal |

### Exported methods

| Symbol | Location | Role |
|--------|----------|------|
| `NewTaskVerifier` | `:173` | DI constructor (LLM, LocalStore, ShardManager, Autopoiesis) |
| `(*TaskVerifier).SetTaskExecutor` | `:95` | Prefer unified executor over ShardManager.Spawn |
| `(*TaskVerifier).SetSessionContext` | `:188` | sessionID + turn for persistence |
| `(*TaskVerifier).VerifyWithRetry` | `:199` | **Main entry** — execute / verify / correct / retry |

Unexported but load-bearing: `spawnTask`, `normalizeIntentVerb`, `verifyTask`, `isReviewTask`, `applyCorrectiveAction`, `findMatchingSpecialist`, `enrichTaskWithContext`, `storeVerification`, `basicQualityCheck`, `parseVerificationResponse`, `selectBestShard`, `heuristicShardSelection`, `parseShardSelection`, truncators.

---

## 5. Deep dive — VerifyWithRetry control loop

```
                    ┌─────────────────────────────┐
                    │ VerifyWithRetry(task, shard,│
                    │                 maxRetries) │
                    └─────────────┬───────────────┘
                                  │ maxRetries <= 0 → 3
                                  ▼
                         ┌────────────────┐
              attempt ──►│  spawnTask     │  normalizeIntentVerb
                         │  (executor |   │  TaskRequest / Spawn
                         │   shardMgr)    │
                         └───────┬────────┘
                                 │ exec error → return immediately
                                 ▼
                         ┌────────────────┐
                         │  verifyTask    │  nil client → soft success 0.5
                         │  LLM or parse  │  LLM err → soft success 0.3
                         │  fail → basic  │  isReviewTask → review prompt
                         └───────┬────────┘
                                 │
              success && no violations ──► store(success) ──► return result
                                 │
                                 ▼
                         store(failed)
                                 │
                    last attempt? ──yes──► return result + ErrMaxRetriesExceeded
                                 │ no
                                 ▼
                    selectBestShard (LLM or heuristic)
                    applyCorrectiveAction (specialist / tool / decompose)
                    enrichTaskWithContext → currentTask
                                 │
                                 └── loop attempt++
```

### 5.1 Attempt body (source-faithful)

For each attempt `0..maxRetries-1`:

1. **`spawnTask(ctx, currentShardType, currentTask)`**  
   - `normalizeIntentVerb` first (bare names → `/fix`, `/consult/<name>`, etc.)  
   - Prefer `taskExecutor.Execute` with `session.TaskRequest{IntentVerb, Task}`  
   - Else `shardMgr.Spawn`  
   - Else hard error: `"no executor available..."`  
   - **Spawn failures short-circuit the loop** (not counted as quality failure).

2. **`verifyTask(ctx, currentTask, result)`**  
   - If `client == nil`: return Success=true, Confidence=0.5, reason “No LLM client…”  
   - Branch system prompt on `isReviewTask(task)` (review quality vs implementation quality)  
   - Truncate result to 8000 chars (`truncateForVerification`)  
   - `client.CompleteWithSystem` → `parseVerificationResponse`  
   - Parse failure → **`basicQualityCheck(result)`** (string heuristics)

3. **If verify call itself errors** (LLM transport): outer loop substitutes Success=true, Confidence=0.3, reason `"Verification skipped: …"` — **fail-open**.

4. **Success criterion**: `verification.Success && len(QualityViolations)==0`.  
   Note: an LLM could set Success=true while still listing violations; the conjunction catches that.

5. **On failure** (and not last attempt):  
   - `storeVerification(..., success=false)`  
   - `selectBestShard` may change `currentShardType`  
   - `applyCorrectiveAction` may return additional context  
   - `enrichTaskWithContext` appends failure reason, violations, evidence, corrective context, and a hard-coded anti-mock reminder  

6. **On last attempt failure**: still returns `lastResult` + `lastVerification` **and** `ErrMaxRetriesExceeded`.

### 5.2 Review vs implementation verification

`isReviewTask` scans lowercase task text for keywords: `review`, `analyze`, `security_scan`, `complexity`, `audit`, `inspect`, `examine`, `assess`, `evaluate` (prefix or `keyword + space`).

| Mode | What success means |
|------|--------------------|
| Review | Review *output* is useful/coherent; reporting incomplete code is **success** |
| Implementation | No mock/placeholder/incomplete/hallucinated-API/etc. patterns; any violation forces fail |

Chat-layer note: `shouldVerifyDelegation` only runs verification for `intent.Category == "/mutation"`. Pure `/query` reviews usually **never enter** this package, so `isReviewTask` mainly matters when a mutation-shaped task description still contains review language, or if a future caller invokes `VerifyWithRetry` directly.

### 5.3 Quality violations

| Constant | JSON string | Detected by LLM prompt | Detected by `basicQualityCheck` |
|----------|-------------|------------------------|----------------------------------|
| `MockCode` | `mock_code` | Yes | Yes (`mock` / `Mock`) |
| `PlaceholderCode` | `placeholder` | Yes | Yes (TODO/FIXME/placeholder/stub) |
| `HallucinatedAPI` | `hallucinated_api` | Yes | **No** |
| `IncompleteImpl` | `incomplete` | Yes | Yes (`not implemented`) |
| `HardcodedValues` | `hardcoded` | Yes | **No** |
| `EmptyFunction` | `empty_function` | Yes | **No** |
| `MissingErrors` | `missing_errors` | Yes | **No** (heuristic *shard* selection only) |
| `FakeTests` | `fake_tests` | Yes | **No** (heuristic *shard* selection only) |

### 5.4 Corrective actions

| Type | Intended effect | Actual code path |
|------|-----------------|------------------|
| `CorrectiveResearch` | Research real APIs/docs | Specialist spawn if match; **no** dedicated web researcher (comment: “researcher removed — JIT clean loop handles research”) |
| `CorrectiveDocs` | Context7-style docs | Specialist with `"docs: "+query`; **no** direct Context7 call in this package |
| `CorrectiveTool` | Generate missing tool | `autopoiesis.GenerateTool` if orchestrator set |
| `CorrectiveDecompose` | Smaller tasks | Static markdown hint only |

**Always first:** if `action.ShardHint` matches a specialist via `findMatchingSpecialist`, spawn that specialist and use truncated output as context.

### 5.5 Intent normalization

`normalizeIntentVerb` prevents retry deaths when the selection LLM returns bare names:

| Input | Output |
|-------|--------|
| empty / whitespace | `/general` |
| already `/...` | unchanged |
| `coder` | `/fix` |
| `tester` | `/test` |
| `reviewer` | `/review` |
| `researcher` | `/research` |
| `nemesis` | `/attack` |
| `librarian` | `/learn` |
| `planner` | `/plan` |
| `legislator` | `/legislate` |
| `constitution` | `/audit` |
| anything else | `/consult/<name>` |

Documented as mirroring `cmd/nerd/chat` `personaToIntent` without importing chat (import-cycle avoidance).

### 5.6 Persistence shape

`storeVerification` no-ops if `localDB == nil`. Otherwise JSON-marshals violations / evidence / corrective action, SHA-256 hashes the **task text** (field named `result_hash` / `taskHashHex` in call — **task content hash, not result hash**), and calls:

```
LocalStore.StoreVerification(sessionID, turnCount, task, shardType, attempt,
  success, confidence, reason, violationsJSON, correctiveJSON, evidenceJSON, taskHashHex)
```

Schema lives in `internal/store/local_core.go` table `task_verifications` with indexes on session, success, shard_type. Readers: `GetVerificationHistory`, `GetQualityViolationStats` on store — **not consumed inside verification package**.

---

## 6. Integration map

### 6.1 Construction (boot)

Both interactive boot paths:

```go
taskVerifier := verification.NewTaskVerifier(llmClient, localDB, shardMgr, autopoiesisOrch)
taskVerifier.SetTaskExecutor(taskExecutor)
// → ChatConfig.Verifier → model.verifier
```

Files: `cmd/nerd/chat/session_boot.go`, `cmd/nerd/chat/session_shared_boot.go`.

### 6.2 Runtime call (process)

```go
if m.verifier != nil && shouldVerifyDelegation(intent) { // Category == "/mutation"
    m.verifier.SetSessionContext(m.sessionID, m.turnCount)
    result, verification, verifyErr := m.verifier.VerifyWithRetry(ctx, task, shardType, 3)
    // ResultToFacts + kernel.LoadFacts
    // verifyErr == ErrMaxRetriesExceeded (string-compared) → formatVerificationEscalation
    // else success → formatVerifiedResponse
}
// else: direct spawnTaskWithContext (no quality loop)
```

### 6.3 Dependency roles

| Dependency | Role in verifier |
|------------|------------------|
| `perception.LLMClient` | `CompleteWithSystem` for verify + shard select |
| `*store.LocalStore` | Persist attempts |
| `*coreshards.ShardManager` | List specialists; fallback Spawn |
| `session.TaskExecutor` | Preferred task execution |
| `*autopoiesis.Orchestrator` | `GenerateTool` on CorrectiveTool |
| `logging` | StoreError / SystemShardsWarn on marshal/store failure |

### 6.4 Not wired

- No Mangle predicates asserted by this package  
- No VirtualStore route for “verify”  
- No prompt atoms under `internal/prompt/atoms/` for verifier system prompts  
- No campaign orchestrator direct import (campaign has its own TaskExecutor wiring elsewhere)

---

## 7. Concurrency model

`TaskVerifier` holds `sync.RWMutex` protecting `sessionID`, `turnCount`, and `taskExecutor` assignment. `VerifyWithRetry` itself is **sequential per call** (no internal fan-out). Concurrent `VerifyWithRetry` on one instance would race on unexported fields mutated without locks (e.g. if future code shared one verifier across goroutines for parallel mutations). Current chat process path is single-turn sequential for a given model.

---

## 8. Gaps pointer

See [03-GAP-ANALYSIS.md](03-GAP-ANALYSIS.md) for the full matrix. Highest-signal gaps:

1. Fail-open verification (LLM down ⇒ treat as success)  
2. Research/docs corrective paths do not call research tooling  
3. Stored history is write-only from this package’s perspective  
4. Verifier prompts are not JIT atoms  
5. Chat compares `verifyErr.Error()` string instead of `errors.Is(..., ErrMaxRetriesExceeded)`  
6. `basicQualityCheck` covers only a subset of declared violation types  
7. `result_hash` column stores **task** hash  

---

## 9. Non-goals of this corpus revision

- Changing Go code or tests  
- Inventing Mangle policy for quality violations  
- Spec templates under `Docs/Spec/`  
- Claiming campaign/assault use this package without evidence  

---

## 10. Verify commands

```powershell
go test ./internal/verification/...
go test -count=1 ./internal/verification/...
go test -race ./internal/verification/...
```
