# core — Testing Alignment

> Last verified: **2026-07-13**

## 1. Test inventory (shape)

| Area | Pattern | Focus |
|------|---------|-------|
| Kernel facts/query/eval | `kernel_*_test.go` | Assert, retract, lazy eval, clone |
| Features / diff eval | `kernel_features_test.go` | Flag routing |
| Provenance | `kernel_provenance_test.go` | Explain path |
| Transactions | `kernel_transactions` / `cortex_kernel_transaction_test.go` | Commit semantics |
| VirtualStore | `virtual_store*_test.go` | Route, handlers, integration |
| Dreamer | `dreamer*_test.go`, `dream_*_test.go` | Fail-closed, cache, plans |
| Validators | `validator_*_test.go`, `action_validator*_test.go` | File/exec/syntax |
| Shadow / TDD | `shadow_mode*_test.go`, `tdd_loop_test.go` | Simulation loops |
| Scheduler | `api_scheduler*_test.go` | Slots, leaks |
| Cortex / router | `cortex_kernel*_test.go`, `shard_fact_router_test.go` | Ownership |
| Policy goldens | `defaults/policy/*_test.go` + `testdata/*.edb` | Logic outcomes |
| Hybrid / corpus | `hybrid_loader_test.go`, `predicate_corpus*_test.go` | Loaders |
| Coverage boosts | `*_gaps_test.go`, `coverage_boost_test.go` | Edge branches |

## 2. Commands

```powershell
# Full core tree (includes shards subpackage tests)
go test ./internal/core/...

# Faster focus
go test ./internal/core/ -count=1 -run 'Kernel|VirtualStore|Dreamer'
go test ./internal/core/shards/ -count=1
go test ./internal/core/defaults/policy/ -count=1

# Race (when changing concurrency)
go test -race ./internal/core/ -run 'Scheduler|Spawn|Transaction|Cortex'

# Cross-boundary
go test ./tests/e2e/ -count=1 -run 'Kernel|VirtualStore|Dreamer|Shadow|Session'
```

CGO flags from root `AGENTS.md` when building binary; pure `go test` for most core packages does not need sqlite-vec unless tests open corpora with CGO deps.

## 3. Policy golden methodology

Under `defaults/policy/testdata/`:

| Pair | Domain |
|------|--------|
| `safety.edb` | Constitution-ish |
| `git_safety.edb` / `.golden` | Git policy |
| `honeypot.edb` / `.golden` | Browser honeypot |
| `campaign*.edb` / `.golden` | Campaign rules |
| `codedom_safety.*` | CodeDOM safety |
| `jit_logic.*` | JIT selection logic |
| `tdd_logic.*` / `tdd_loop.*` | TDD |

Tests load EDB + program slice and compare derived atoms to golden files. Prefer extending goldens when changing safety semantics.

## 4. Alignment to principles

| Principle | Test evidence |
|-----------|---------------|
| Default deny | safety policy tests, VS permitted tests |
| Fail-closed Dreamer | dreamer tests + e2e kernelclone |
| Boot / load | kernel_init / corpus tests |
| Clone isolation | dreamer_kernelclone e2e |
| Concurrent scheduler | slot leak tests |
| Validators | file/exec gap suites |

## 5. Known testing gaps

| Gap | Risk | Suggested coverage |
|-----|------|--------------------|
| Full ActionType matrix vs handler | Silent no-op verbs | Table test: every ActionType has case |
| HotLoad + permission cache | Stale allow | Integration after HotLoadRule |
| Diff-eval retract paths | Wrong IDB | Explicit retract→query tests with flag on/off |
| Multi-domain ownership conflicts | Last-wins | Cortex register conflict assertions |
| Long-horizon fact prune | EDB bloat | maybePruneActionLogs unit stress |
| Real MCP network | Flaky | Keep fakes; e2e optional tagged |

## 6. How to test a policy change

1. Identify Decl in `schemas_*.mg`.  
2. Add/adjust rules in `policy/*.mg`.  
3. Update or add `testdata/*.edb` + golden.  
4. `go test ./internal/core/defaults/policy/`.  
5. Boot kernel smoke: small Go test `NewRealKernel` or run `nerd` query.  
6. If analyze fails, inspect `debug_program_ERROR.mg`.

## 7. How to test a VS handler change

1. Unit test handler with mock tactile executor.  
2. RouteAction test for boot guard / constitution / permitted interactions.  
3. If destructive: dreamer block + allow cases.  
4. Inject fact assertions on `execution_result`.

## 8. CI expectations

`go test ./...` is the repo bar (root AGENTS.md). Core is on the critical path; regressions here fail large swaths of e2e.

## 9. Non-goals for unit tests

- Full LLM network  
- Full browser automation  
- Entire campaign assault runs (use campaign package / chat campaign tests)
