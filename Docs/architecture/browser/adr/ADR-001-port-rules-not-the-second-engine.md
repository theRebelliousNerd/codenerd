---
doc-class: governance
subsystem: browser
implementation-status: accepted-not-implemented
last-verified: 2026-09-29
verified-against: 6597099c
supersedes: []
---

# ADR-001: Port Deductive Rules into the Live Kernel, Not the Secondary Engine

## Context

Upstream BrowserNERD 1.2.0 includes a standalone deductive reasoning engine implemented across several files (`indexed_store.go`, `engine.go`, `closure.go`, `catalog.go`, `stats.go`, `query_check.go`, and `schemas/embed.go`). That engine was constructed to serve an isolated Model Context Protocol (MCP) server process, managing its own indexing, stratification, and query evaluation via `codeberg.org/TauCeti/mangle-go`.

Embedding that secondary engine inside codeNERD would introduce significant architectural violations:
1. **Parallel Reasoning Reality**: codeNERD's foundational North Star dictates that the live kernel (`RealKernel` / `SystemKernel`) is the sole executive authority. Hosting a secondary, private Mangle engine inside `internal/browser` creates two competing sources of truth.
2. **Disconnected Evidence**: Facts derived in a private browser engine cannot participate in system-wide joins with task context, CodeDOM models, or constitutional permission rules.
3. **Redundant Code and Maintenance**: Maintaining two separate Mangle evaluation runtimes multiplies dependency overhead and maintenance risk.

## Decision

We will not port BrowserNERD's standalone engine files (`indexed_store.go`, `engine.go`, `closure.go`, `catalog.go`, `stats.go`, `query_check.go`, or `schemas/embed.go`).

Instead:
1. BrowserNERD's deductive rules (causal temporal diagnosis, asset 4xx discrimination, bucketed error attribution, and action candidate ranking) are ported directly into codeNERD's embedded schema and policy corpus (`internal/core/defaults/schemas_browser.mg` and `internal/core/defaults/policy/browser.mg`).
2. Go remains strictly an automation driver and sensor: it captures CDP events, sanitizes strings, structures event data, and asserts immutable facts into codeNERD's live kernel.
3. Tools such as `browser_reason` and `browser_mangle` query the live kernel directly using session-scoped atom queries.

## Consequences

### Positive
- Preserves codeNERD's unified reasoning model: all browser facts are queryable alongside system facts.
- Browser facts can directly inform constitutional safety policies and higher-level orchestration decisions.
- Eliminates thousands of lines of redundant Mangle runtime and indexing code.
- Ensures all evaluation benefits from codeNERD's centralized performance optimizations and telemetry.

### Negative
- Any specialized indexing or query caching formerly handled by `indexed_store.go` must be evaluated for performance in codeNERD's kernel; if heavy joins degrade latency, kernel-level optimizations must be introduced centrally rather than locally in the browser package.
- Complex conjunction queries in `browser_mangle` must remain constrained until the live kernel exposes safe multi-atom session-bound validation.

## Witness

**Witness:** Test `TestAssetHttpErrorDiscrimination` in `internal/core/defaults/policy/browser_test.go` and schema declaration `Decl net_http_error` in `internal/core/defaults/schemas_browser.mg`.
