# 10 — Testing Alignment: Context

> Last verified against codebase: 2026-07-13  
> Package: `internal/context`  
> Status: Living Reference Document

## 1. Commands

```powershell
go test ./internal/context/...
go test -race ./internal/context/...
go test -count=1 ./internal/context/...

# Integration harness (sibling package)
go test ./internal/testing/context_harness/...
```

## 2. Coverage map

| Area | Tests | Quality |
|------|-------|---------|
| Activation construct / score / filter / budget | `activation_test.go` | Strong |
| Recency, relevance, campaign, session | `activation_test.go` | Strong |
| Corpus priority SSOT | `activation_test.go` | Strong |
| Issue weight clamp | `activation_test.go` | Strong |
| Concurrent scoring | `activation_race_test.go` | Critical |
| Context setters | `activation_setters_test.go` | Adequate |
| ProcessTurn / compress / BuildContext / persist | `compressor_test.go` | Strong core paths |
| Constructors / metrics / trim / session ID | `compressor_accessors_test.go` | Strong |
| TokenBudget allocate/release/hard | `budget_helpers_test.go` | Strong |
| Token counter extras | `token_counter_extra_test.go` | Adequate |
| Serializer + corpus order | `serializer_test.go` | Strong |
| Feedback store persist + locks | `feedback_store_*.go` | Strong for unit |
| End-to-end chat compression switch | *not in package* | Relies on chat / harness |

## 3. Notable test behaviors

- **Issue score clamp tests** — adversarial high/negative weights.  
- **Race test** — concurrent ScoreFacts.  
- **Compression trigger** — budget threshold path in compressor_test.  
- **Feedback min samples** — usefulness zero until threshold.

## 4. Gaps

| Gap | Risk | Suggestion |
|-----|------|------------|
| Kernel `should_include_context` integration | Medium | Fixture kernel with compilation rules loaded |
| Async ProcessTurn + BuildContext race | Medium | Deterministic sync path test |
| Observation mask predicate consumption | Medium | Assert Go respects derived mask |
| Real tokenizer accuracy | Low | Document heuristic; optional golden |
| Full chat IsCompressionActive branch | Medium | Chat package tests or harness |
| LoadState version migration | Low | Future schema tests |

## 5. External harness

`internal/testing/context_harness/README.md` documents compressor + activation as first-class engines for long-horizon validation. Prefer that package for multi-component scenarios.

CLI: `cmd/nerd/cmd_test_context.go` / `nerd test-context`.

## 6. CI expectation

Package unit tests should pass without external LLM. Feedback tests use temp SQLite paths. CGO may be needed for sqlite drivers depending on build tags — match repo sqlite-vec conventions when linking full binary.
