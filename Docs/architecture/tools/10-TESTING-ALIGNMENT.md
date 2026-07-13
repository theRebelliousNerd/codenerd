# tools — Testing Alignment

> Last verified: **2026-07-13**

## Package-local commands

```powershell
go test ./internal/tools/...
go test ./internal/tools/core/ -count=1
go test ./internal/tools/shell/ -count=1
go test ./internal/tools/codedom/ -count=1
go test ./internal/tools/research/ -count=1
```

## Coverage map

### Root registry

| File | Focus |
|------|-------|
| `registry_test.go` | New, register, dup, validate, category, execute, filter, global |
| `registry_extra_test.go` | Has/GetMultiple/Names; global singleton |
| `registry_boundary_test.go` | nil tool, nil ctx, empty intent, type mismatch, priority extremes, nil args |

### core

| File | Focus |
|------|-------|
| `register_test.go` | RegisterAll names present |
| `workspace_guard_test.go` | resolveWorkspacePath containment |
| `file_ops_test.go` | read/write/edit/delete/list |
| `search_test.go` | glob/grep/search_code definitions + execute |

### shell

| File | Focus |
|------|-------|
| `register_test.go` | RegisterAll |
| `execute_test.go` | mocked exec, defs, git, detect, coerce via helper process |
| `execute_extra_test.go` | coerceInt, autodetection |
| `shell_integration_test.go` | suite for real-ish command/bash/git/build/test |

### codedom

| File | Focus |
|------|-------|
| `elements_test.go` / `lines_test.go` | tools + ranges |
| `register_test.go` | RegisterAll |
| `impact_test.go` / `run_impacted_tests_extra_test.go` | parsers, runGoTests helpers |
| `benchmark_test.go` | perf smoke |

### research

| File | Focus |
|------|-------|
| `research_test.go`, `research_coverage_test.go` | broader coverage |
| `context7_tool_test.go`, `fetch_tool_test.go` | tool defs/behavior |

## Related tests outside package

| Location | What it proves |
|----------|----------------|
| `internal/session/*tool*`, `*safety*` | allowlist, payload size, nil cfg |
| `internal/core/virtual_store_tools` paths / coverage | hydrate, modular execute |
| `tests/e2e/tool_safety_fallback_config_test.go` | empty AllowedTools open; forbidden blocked |
| `tests/e2e/SessionExecutor_VirtualStore_Kernel_integration_test.go` | end-to-end allowlist |
| `tests/e2e/session_clean_loop_integration_test.go` | tool loop + timeout |
| `tests/e2e/piggyback_executor_full_boundary_test.go` | piggyback path |

## Gaps in testing

| Gap | Suggestion |
|-----|------------|
| No adversarial path traversal suite for grep/glob | Add tests that expect denial once guard lands |
| working_dir escape for shell | Property test under workspace root |
| Dual-registry hydrate race | Parallel hydrate smoke |
| Browser tools mostly unmocked | Requires Rod; mark integration build tag |
| Network web_search flaky | Prefer HTML fixture parse tests (partially present) |
| Mangle modular_tool_allowed catalog vs RegisterAll | Golden list test comparing names |

## Definition of done for tool PRs

1. Unit test for schema required-arg failure.  
2. Happy-path execute with temp dir / mocked exec.  
3. RegisterAll includes name.  
4. If path-touching: workspace escape test.  
5. Update intent_routing if allow semantics change.  
6. `go test ./internal/tools/...` green.
