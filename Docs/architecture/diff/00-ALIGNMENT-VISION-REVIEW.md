# 00 — Alignment & Vision Review: `internal/diff`

> Last verified against codebase: 2026-07-13  
> Status: Living Reference Document — code-grounded  
> Source: `internal/diff/` (1 non-test Go file ≈ 379 lines; 2 test files ≈ 949 lines)

## 1. North-star statement

codeNERD separates **creative** LLM work from **executive** Mangle control. Diff computation
is neither: it is a **deterministic transducer of text pairs into structured change facts**
for human review and UI rendering.

Alignment for this package is therefore:

1. Stay **pure and deterministic** (same inputs → same hunks, modulo cache/path labels).  
2. Never become a policy bypass (do not apply filesystem writes; do not assert permissions).  
3. Keep the human approval surface honest (binary short-circuit, stable hunk model).  
4. Bound cost so a pathological file cannot stall the TUI or agent loop.

## 2. Alignment dimensions

| Dimension | Score (0–5) | Evidence |
|-----------|-------------|----------|
| Creative/executive split | **5** | No LLM, no kernel; pure function of strings (`diff.go` `ComputeDiff`) |
| Fact-flow fidelity | **4** | Sits after act/propose, before human y/n — correct edge placement; not wired into Mangle atoms (by design) |
| Determinism | **5** | Library + fixed timeout + fixed context; no RNG |
| Safety at edge | **4** | Binary NUL gate, context clamp, 5s DiffTimeout; unbounded cache is residual risk |
| Test grounding | **4** | ~949 test lines covering add/delete/binary/cache/concurrency/benchmarks; residual TEST_GAP TODOs |
| Observability | **1** | No logging/metrics on timeout, cache hit, or binary short-circuit |
| JIT / atom discipline | **N/A** | Not LLM-facing prompt surface |
| Constitutional safety | **N/A / 5** | Not an effect surface; cannot violate `permitted` by itself |
| Integration honesty | **5** | Single real consumer path (`cmd/nerd/ui/diffview.go`); no fake “kernel-related” claims |

**Overall alignment: 4.2 / 5** for its role as a pure utility. Lowest score is observability;
highest is correct architectural placement outside the executive kernel.

## 3. What “good” looks like (diff-specific)

| Good | Bad |
|------|-----|
| Structured `FileDiff` for UI | Dumping raw unified-diff text as the only model |
| Binary short-circuit | Myers on multi-MB binary blobs |
| Timeout on pathological input | Unbounded hang on minified one-liners |
| Shared engine with explicit cache clear | Unbounded process memory from unique pair flood |
| Deep clone or immutable cached hunks | Callers mutate cached slices (shallow trap) |
| Human reviews apply path | Auto-write files from this package |

## 4. Role in inversion of control

```
LLM (creative)     — proposes new content / edit text
Mangle (executive) — permits action, sequences OODA
diff (descriptive) — materializes old→new as reviewable structure
Human / policy     — approve; VirtualStore/tools apply
```

The package **describes** change; it never **authorizes** or **executes** it. That is
correct north-star positioning.

## 5. Related corpora

- `Docs/architecture/cli/` — DiffApprovalView, CreateDiffFromStrings  
- `Docs/architecture/core/` — effectful apply path after approval  
- This corpus — implementation truth for the adapter itself  
