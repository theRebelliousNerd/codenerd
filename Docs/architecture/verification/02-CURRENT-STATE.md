# verification — Current State

> Last verified: **2026-07-13**  
> Source of truth: `internal/verification/` on disk

## Package metrics

| Metric | Value |
|--------|------:|
| Non-test Go files | 1 |
| Test Go files | 3 |
| Mangle sources | 0 |
| Approx. production LOC | ~820 |
| Approx. test LOC | ~950 |
| Exported types | 6 (+ 1 error var) |
| Exported constructors / methods | 4 (`New…`, `SetTaskExecutor`, `SetSessionContext`, `VerifyWithRetry`) |
| Primary call sites | chat process + dual boot paths |

## File roles

| File | Role | Hotspot? |
|------|------|----------|
| `verifier.go` | Entire production surface | **Yes** — single god-file by design |
| `verifier_test.go` | Smoke tests: review classify, basic quality, parse fences, truncate | Medium |
| `verifier_normalize_test.go` | Persona → intent table; SetTaskExecutor nil | Low |
| `verifier_gaps_test.go` | Broad table tests for edges, heuristics, enrich, spawn errors | **Yes** — densest test file |

## Structural inventory

### `TaskVerifier` fields

| Field | Type | Purpose |
|-------|------|---------|
| `mu` | `sync.RWMutex` | Protects session + executor assignment |
| `client` | `perception.LLMClient` | Judge + selector LLM |
| `localDB` | `*store.LocalStore` | Persistence |
| `shardMgr` | `*coreshards.ShardManager` | List/spawn specialists |
| `taskExecutor` | `session.TaskExecutor` | Preferred execution |
| `autopoiesis` | `*autopoiesis.Orchestrator` | Tool generation corrective |
| `sessionID` | `string` | Persistence key |
| `turnCount` | `int` | Persistence key |

### Control-flow hotspots (line anchors approximate)

| Region | Lines (approx) | What happens |
|--------|----------------|--------------|
| Package + types | 1–92 | Violations, correctives, result types |
| Spawn + normalize | 94–170 | Executor/mgr dispatch, persona map |
| Constructor / session | 172–193 | DI |
| `VerifyWithRetry` | 195–264 | Main loop |
| `isReviewTask` / `verifyTask` | 266–374 | LLM judge + prompts |
| Correctives + specialists | 376–537 | Context enrichment |
| Store + basic check + parse | 539–663 | Persist + fallback |
| Shard selection | 665–819 | LLM + heuristic + parse |

## Behavioral capabilities present today

- Retry loop with default `maxRetries=3`  
- Dual verification criteria (review vs implementation)  
- JSON parse with markdown fence stripping  
- Heuristic quality fallback when parse fails  
- Heuristic and LLM shard re-selection  
- Task enrichment with failure feedback + anti-mock banner  
- Autopoiesis tool generation on corrective type `tool`  
- Specialist keyword matching (rod, golang, react, mangle, sql, api, testing)  
- Session-scoped persistence when LocalStore available  
- Intent verb coercion for TaskExecutor validators  

## Behavioral capabilities absent / stubbed

- No package-local README or agents.md  
- No `.mg` Decl surface  
- No consumption of `GetVerificationHistory` / `GetQualityViolationStats`  
- No direct Context7 or web research client (comment notes researcher removed)  
- No metrics counters (only logging on store errors)  
- No interface abstraction for `TaskVerifier` itself (concrete type in chat model)  
- No parallel attempt execution  
- No configurable confidence threshold beyond Success && empty violations  

## Runtime placement

| Environment | Wired? | Notes |
|-------------|--------|-------|
| Interactive chat boot | Yes | `session_boot.go` / `session_shared_boot.go` |
| Mutation delegation | Yes | `process.go` + `shouldVerifyDelegation` |
| Non-mutation delegation | Skipped | Direct spawn path |
| Campaign CLI | Not direct | Campaign sets TaskExecutor on VS/orchestrator; no `NewTaskVerifier` there found |
| Headless `nerd run` | Via same chat/cortex boot if shared | Depends on boot path constructing Verifier |

## Quality of existing thin corpus (pre-rebuild)

Prior `Docs/architecture/verification/*` files were **auto-inventory stubs** (~generic tables, swap-package-name risk). This rebuild replaces them with package-specific narrative. Obsolete filenames (e.g. `01-DOMAIN-MODEL.md`) may still exist as redirects if left on disk; prefer the document map in [README.md](README.md).
