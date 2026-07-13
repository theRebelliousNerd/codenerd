# verification — Architecture Corpus

> Last verified against codebase: **2026-07-13**  
> Status: Living Reference Document — **code-grounded full corpus**  
> Source: `internal/verification/`  
> Implementation: **1** non-test `.go` (~820 lines), **3** test files (~950 lines), **0** `.mg`

## Scope

`internal/verification` is the **quality-enforcing retry loop** that wraps delegated shard execution for mutation work. After a shard produces a result, `TaskVerifier` asks an LLM (or a heuristic fallback) whether the work is real code vs mocks/placeholders/hallucinated APIs, then optionally re-selects a shard, applies corrective context, and retries until success or `ErrMaxRetriesExceeded`.

It is **not**:

- Kernel constitutional safety (`permitted(...)`) — that lives in Mangle policy / core
- Prompt JIT compilation — verifier prompts are currently inline strings
- Campaign / assault scoring — different package surface
- Unit-test runners — it *detects* fake tests in *generated* output; it does not run `go test`

## Position in fact-flow

```
user input → perception → user_intent (Category=/mutation)
  → chat process decides RouteDelegate
  → shouldVerifyDelegation(intent)  [chat layer]
  → TaskVerifier.VerifyWithRetry
       spawnTask (TaskExecutor | ShardManager)
       verifyTask (LLM JSON or basicQualityCheck)
       selectBestShard / applyCorrectiveAction on failure
  → storeVerification → LocalStore.task_verifications
  → ResultToFacts → kernel.LoadFacts
  → formatVerifiedResponse | formatVerificationEscalation → TUI
```

## Document map

| Doc | Role |
|-----|------|
| [IMPLEMENTED_SPEC.md](IMPLEMENTED_SPEC.md) | **Flagship** living spec: flows, inventory, integration |
| [00-ALIGNMENT-VISION-REVIEW.md](00-ALIGNMENT-VISION-REVIEW.md) | North-star alignment scores + evidence |
| [01-VISION.md](01-VISION.md) | Target product/architecture vision |
| [02-CURRENT-STATE.md](02-CURRENT-STATE.md) | Precise on-disk inventory and hotspots |
| [03-GAP-ANALYSIS.md](03-GAP-ANALYSIS.md) | Spec vs reality matrix |
| [04-ARCHITECTURAL-PRINCIPLES.md](04-ARCHITECTURAL-PRINCIPLES.md) | Binding package principles |
| [05-INTERNAL-ARCHITECTURE.md](05-INTERNAL-ARCHITECTURE.md) | Components, state machine, data flow |
| [06-PUBLIC-API-AND-TYPES.md](06-PUBLIC-API-AND-TYPES.md) | Exported types and methods |
| [07-DEPENDENCY-MAP.md](07-DEPENDENCY-MAP.md) | Upstream/downstream with evidence |
| [08-WIRING-AND-INTEGRATION.md](08-WIRING-AND-INTEGRATION.md) | Boot, chat process, store, autopoiesis |
| [09-SAFETY-AND-INVARIANTS.md](09-SAFETY-AND-INVARIANTS.md) | Safety, concurrency, fail-open rules |
| [10-TESTING-ALIGNMENT.md](10-TESTING-ALIGNMENT.md) | Existing tests, gaps, commands |
| [11-OBSERVABILITY.md](11-OBSERVABILITY.md) | Logging and persistence surfaces |
| [12-FAILURE-MODES.md](12-FAILURE-MODES.md) | Concrete failures + mitigations |
| [TODO.md](TODO.md) | Prioritized backlog |
| [OPEN-QUESTIONS.md](OPEN-QUESTIONS.md) | Real open design questions |
| [_progress.md](_progress.md) | Rebuild progress log |

## Verify

```powershell
go test ./internal/verification/...
go test -race ./internal/verification/...
```

Chat-layer gating (caller, not this package):

```powershell
go test ./cmd/nerd/chat/ -run "ShouldVerify|Verification|shouldVerify"
```

## Key entry points

| Symbol | Path |
|--------|------|
| `NewTaskVerifier` | `internal/verification/verifier.go` |
| `(*TaskVerifier).VerifyWithRetry` | `internal/verification/verifier.go` |
| `(*TaskVerifier).SetTaskExecutor` | `internal/verification/verifier.go` |
| `(*TaskVerifier).SetSessionContext` | `internal/verification/verifier.go` |
| Call site | `cmd/nerd/chat/process.go` (mutation delegation) |
| Construction | `cmd/nerd/chat/session_boot.go`, `session_shared_boot.go` |
| Persistence | `internal/store/local_verification.go` (`task_verifications`) |
