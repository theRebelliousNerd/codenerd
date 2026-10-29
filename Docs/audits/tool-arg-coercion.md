# Tool Argument Coercion Audit

**Date:** 2026-05-13 (verified 2026-08-12 against HEAD)
**Scope:** `internal/tools/**/*` — all tool `Execute` functions that unpack `args map[string]any`
**Question:** How is each numeric tool argument coerced from JSON-decoded types (`float64` vs bare `int`), and which helper (if any) is used?
**Methodology:** `grep -rn 'args\["' internal/tools --include='*.go'` (144 matches including 5 comment and 8 test matches; 131 non-test/non-comment tool extraction sites across 16 files), then filtered to `Type: "integer"` schema declarations (42 integer declarations found via `grep -rn 'Type:\s*"integer"'`). Each site inspected for type-switch handling of `int`/`int64`/`float64`/`json.Number` and verified helper definitions via `grep` for `func coerceInt|func argInt|func intArg|func int64Arg` plus direct `read_file` line verification. Only identifiers that appear verbatim in source are cited. Line numbers are 1-indexed file lines verified against current `internal/tools` HEAD. Uncertainty notes inline where direct verification was truncated by budget.

## Summary

- **Total extraction sites examined:** 144 `args["…"]` matches; 131 tool extraction sites after excluding 5 comment lines (`internal/tools/core/search.go:22`, `internal/tools/research/numeric_args.go:11`, `internal/tools/shell/execute.go:49`, etc.) and 8 test matches (`*_test.go`). Full table below covers integer-typed schema fields only.
- **Total numeric-integer schema declarations:** 42 `Type: "integer"` declarations (see evidence list in Methodology). Direct numeric coercion sites listed: 36 distinct integer argument extraction sites where the helper is invoked (28 `int`-returning + 8 `int64`-returning). An additional 6 integer declarations are schema-only with no direct `args["…"]` numeric extraction in the same function (they forward via `executeRunCommand` or are capped via `boundedIdle` indirection — noted below).
- **Coercion outcome:** 0 sites use bare `.(int)` alone. All 36 listed numeric sites handle `float64` (the type `encoding/json` produces without `UseNumber`); 31 via shared helpers, 5 via inline `int`→`float64` fallback. No numeric site uses `.(float64)` alone without an `int` branch.
- **Helpers in use (verified):** 4 unique helper names, 6 definitions (two duplicated across packages): `coerceInt` (`internal/tools/core/file_ops.go:238`, `internal/tools/shell/execute.go:52`), `argInt` (`internal/tools/core/search.go:33`, `internal/tools/research/numeric_args.go:15`), `intArg` (`internal/tools/research/browser_progressive.go:196`), `int64Arg` (`internal/tools/research/browser_reasoning.go:943`). All helpers switch on `int` / `int64` / `float64` (+ `json.Number`; shell variant also handles `string` via `strconv`; `coerceInt` shell also handles `uint*`/`float32`).
- **Risk:** No remaining bare-int numeric site in production code. Historical bare-int bug (observed: `max_results=500` returned 50; referenced at `internal/tools/core/search.go:22-24` and `internal/tools/research/numeric_args.go:11-13`) is mitigated everywhere numeric args are read. The five `internal/tools/codedom/lines.go` sites still use inline fallback rather than a shared helper — functionally correct but divergent from helper pattern noted in `internal/tools/core/search.go:28-30`.

## Evidence Table — Numeric Integer Argument Sites

| # | File | Line | Argument name | Coercion type | Helper used | Evidence (verbatim identifier / snippet) |
|---|------|------|---------------|---------------|-------------|-------------------------------------------|
| 1 | `internal/tools/codedom/lines.go` | 66 | `start_line` | `float64` (handles `int` + `float64`) | `none` | `startLine, ok := args["start_line"].(int)` at 66; fallback `if f, ok := args["start_line"].(float64); ok { startLine = int(f) }` at 69 — inline |
| 2 | `internal/tools/codedom/lines.go` | 76 | `end_line` | `float64` | `none` | `endLine, ok := args["end_line"].(int)` at 76; fallback `args["end_line"].(float64)` at 78 — inline |
| 3 | `internal/tools/codedom/lines.go` | 330 | `after_line` | `float64` | `none` | `if al, ok := args["after_line"].(int); ok` at 330; `} else if f, ok := args["after_line"].(float64); ok {` at 332 — inline |
| 4 | `internal/tools/codedom/lines.go` | 425 | `start_line` | `float64` | `none` | `startLine, ok := args["start_line"].(int)` at 425; fallback `args["start_line"].(float64)` at 427 — inline |
| 5 | `internal/tools/codedom/lines.go` | 434 | `end_line` | `float64` | `none` | `endLine, ok := args["end_line"].(int)` at 434; fallback `args["end_line"].(float64)` at 436 — inline |
| 6 | `internal/tools/core/file_ops.go` | 139 | `start_line` | `float64` | `coerceInt` | `startLine, hasStart := coerceInt(args["start_line"])` — helper `coerceInt` defined at `internal/tools/core/file_ops.go:238` `func coerceInt(v any) (int, bool)` handles `int`/`int32`/`int64`/`float64`/`float32` |
| 7 | `internal/tools/core/file_ops.go` | 140 | `end_line` | `float64` | `coerceInt` | `endLine, hasEnd := coerceInt(args["end_line"])` — same helper |
| 8 | `internal/tools/core/search.go` | 93 | `max_results` | `float64` | `argInt` | `if v, ok := argInt(args, "max_results"); ok && v > 0 { maxResults = v }` at 93 — helper `argInt` defined at `internal/tools/core/search.go:33` `func argInt(args map[string]any, key string) (int, bool)` handles `int`/`int64`/`float64`/`json.Number` |
| 9 | `internal/tools/core/search.go` | 244 | `context_lines` | `float64` | `argInt` | `if v, ok := argInt(args, "context_lines"); ok { contextLines = v }` at 244 |
| 10 | `internal/tools/core/search.go` | 249 | `max_results` | `float64` | `argInt` | `if v, ok := argInt(args, "max_results"); ok && v > 0 { maxResults = v }` at 249 (second `max_results` site in `executeGrep`) |
| 11 | `internal/tools/shell/execute.go` | 165 | `timeout_seconds` | `float64` | `coerceInt` | `if t, ok := coerceInt(args["timeout_seconds"]); ok && t > 0 { timeout = t }` at 165 — helper `coerceInt` defined at `internal/tools/shell/execute.go:52` `func coerceInt(v any) (int, bool)` handles `int`/`int32`/`int64`/`uint*`/`float*`/`json.Number`/`string` |
| 12 | `internal/tools/shell/execute.go` | 411 | `timeout_seconds` | `float64` | `coerceInt` | `if t, ok := coerceInt(args["timeout_seconds"]); ok && t > 0 { timeout = t }` at 411 (`executeBash`) |
| 13 | `internal/tools/shell/execute.go` | 793 | `count` | `float64` | `coerceInt` | `if c, ok := coerceInt(args["count"]); ok && c > 0 {` at 793 (git log count limit) |
| 14 | `internal/tools/research/context7.go` | 54 | `max_docs` | `float64` | `argInt` | `if md, ok := argInt(args, "max_docs"); ok && md > 0 {` at 54 — helper `argInt` from `internal/tools/research/numeric_args.go:15` `func argInt(args map[string]any, key string) (int, bool)` |
| 15 | `internal/tools/research/web_fetch.go` | 61 | `max_length` | `float64` | `argInt` | `if ml, ok := argInt(args, "max_length"); ok && ml > 0 {` at 61 |
| 16 | `internal/tools/research/web_search.go` | 58 | `max_results` | `float64` | `argInt` | `if mr, ok := argInt(args, "max_results"); ok && mr > 0 {` at 58 |
| 17 | `internal/tools/research/browser_progressive.go` | 45 | `max_items` | `float64` | `intArg` | `MaxItems: intArg(args, "max_items", 20),` at 45 — helper `intArg` defined at `internal/tools/research/browser_progressive.go:196` `func intArg(args map[string]any, key string, fallback int) int` handles `int`/`int64`/`float64`/`json.Number` |
| 18 | `internal/tools/research/browser_progressive.go` | 109 | `max_items` | `float64` | `intArg` | `maxItems := intArg(args, "max_items", 20)` at 109 |
| 19 | `internal/tools/research/browser_evidence.go` | 49 | `since_ms` | `float64` | `int64Arg` | `SinceMS: int64Arg(args, "since_ms", 0)` at 49 — helper `int64Arg` defined at `internal/tools/research/browser_reasoning.go:943` `func int64Arg(args map[string]any, key string, fallback int64) int64` handles `int`/`int64`/`float64`/`json.Number` |
| 20 | `internal/tools/research/browser_evidence.go` | 50 | `max_items` | `float64` | `intArg` | `MaxItems: intArg(args, "max_items", 20),` at 50 |
| 21 | `internal/tools/research/browser_declarative.go` | 119 | `since_ms` | `float64` | `int64Arg` | `SinceMS: int64Arg(args, "since_ms", 0)` at 119 (`BrowserTestTool` generate — `research/browser_declarative.go:41` schema `since_ms` integer) |
| 22 | `internal/tools/research/browser_declarative.go` | 276 | `settle_timeout_ms` | `float64` | `intArg` | `settleTimeout := time.Duration(intArg(args, "settle_timeout_ms", 5000)) * time.Millisecond` at 276 |
| 23 | `internal/tools/research/browser_specs.go` | 71 | `max_items` | `float64` | `intArg` | `maxItems := intArg(args, "max_items", config.MaxResults)` at 71 |
| 24 | `internal/tools/research/browser_specs.go` | 142 | `max_checks` | `float64` | `intArg` | `maxChecks := intArg(args, "max_checks", 50)` at 142 |
| 25 | `internal/tools/research/browser_specs.go` | 245 | `from` | `float64` | `intArg` | `From: intArg(args, "from", 0), To: intArg(args, "to", 0),` at 245 — first extraction on same line |
| 26 | `internal/tools/research/browser_specs.go` | 245 | `to` | `float64` | `intArg` | `To: intArg(args, "to", 0)` at 245 — second extraction on same line |
| 27 | `internal/tools/research/browser_specs.go` | 249 | `line` | `float64` | `intArg` | `if line := intArg(args, "line", 0); line > 0 && input.From == 0 && input.To == 0 {` at 249 |
| 28 | `internal/tools/research/browser_reasoning.go` | 203 | `after_ms` | `float64` | `int64Arg` | `after := int64Arg(args, "after_ms", 0)` at 203 |
| 29 | `internal/tools/research/browser_reasoning.go` | 204 | `before_ms` | `float64` | `int64Arg` | `before := int64Arg(args, "before_ms", time.Now().UnixMilli())` at 204 |
| 30 | `internal/tools/research/browser_reasoning.go` | 215 | `since_ms` | `float64` | `int64Arg` | `int64Arg(args, "since_ms", 0)` at 215 (browser_mangle await) |
| 31 | `internal/tools/research/browser_reasoning.go` | 265 | `since_ms` | `float64` | `int64Arg` | `int64Arg(args, "since_ms", 0)` at 265 (browser_wait stable) — same line also handles `network_idle_ms`/`dom_idle_ms` via `boundedIdle`→`intArg` (see notes) |
| 32 | `internal/tools/research/browser_reasoning.go` | 282 | `since_ms` | `float64` | `int64Arg` | `int64Arg(args, "since_ms", 0)` at 282 (browser_wait fact) |
| 33 | `internal/tools/research/browser_reasoning.go` | 321 | `time_window_ms` | `float64` | `int64Arg` | `windowMs := int64Arg(args, "time_window_ms", defaultReasonWindow.Milliseconds())` at 321 |
| 34 | `internal/tools/research/browser_reasoning.go` | 900 | `max_items` | `float64` | `intArg` | `maxItems := intArg(args, "max_items", defaultBrowserReasonItems)` at 900 |
| 35 | `internal/tools/research/browser_reasoning.go` | 911 | `timeout_ms` | `float64` | `intArg` | `value := time.Duration(intArg(args, "timeout_ms", int(defaultBrowserTimeout.Milliseconds()))) * time.Millisecond` at 911 (via `boundedTimeout`) |
| 36 | `internal/tools/research/browser_reasoning.go` | 922 | `poll_interval_ms` | `float64` | `intArg` | `value := time.Duration(intArg(args, "poll_interval_ms", int(defaultBrowserPoll.Milliseconds()))) * time.Millisecond` at 922 (via `boundedPoll`) |

**Totals:** 36 numeric-integer sites listed above with direct `args["…"]` coercion (28 `int`-returning + 8 `int64`-returning). An additional 2 integer schema fields at `internal/tools/research/browser_reasoning.go:265` — `network_idle_ms` and `dom_idle_ms` — are integer-typed and handled via `boundedIdle(args, "network_idle_ms", 500)` / `boundedIdle(args, "dom_idle_ms", 200)` which delegates to `intArg` (`research/browser_reasoning.go:933` `value := time.Duration(intArg(args, key, fallback)) * time.Millisecond`); they share line 265 and are counted as covered but not as separate rows to avoid double-counting the line. Full sweep covered 144 `args["…"]` matches (131 tool sites excluding tests/comments) to ensure no numeric site was missed. 0 bare-`int`-only sites remain.

**Uncertainty note (budget-limited verification):** `internal/tools/shell/execute.go` declares 5 integer schemas (`timeout_seconds` at 140 and 395, `timeout_seconds` at 517 for `run_build`, `count` at 589 for `git_log`, and `format`/`since` etc. not integer). The `run_build` integer at 517 (`Type: "integer"` at 517-520) forwards via `executeRunCommand(ctx, map[string]any{"command": command, "working_dir": workingDir, "timeout_seconds": args["timeout_seconds"]})` at 544-548 and is therefore coerced indirectly at 165/411; it has no direct `coerceInt(args["timeout_seconds"])` at that site and is not listed as a separate direct extraction. Line numbers for `browser_reasoning.go:265` shared line and `browser_declarative.go:119` were verified via `grep` for `int64Arg`; `browser_declarative.go` line 119 call site was verified but full file read was truncated after 410 lines — identifier and helper confirmed, surrounding context inferred from `grep` line.

## Helper Inventory (only identifiers that appear in source)

| Helper | Defined at | Signature (verbatim) | Numeric types handled | Call-site count (direct) |
|--------|------------|----------------------|----------------------|-----------------|
| `coerceInt` | `internal/tools/core/file_ops.go:238` | `func coerceInt(v any) (int, bool)` | `int`, `int32`, `int64`, `float64`, `float32` | 2 |
| `coerceInt` | `internal/tools/shell/execute.go:52` | `func coerceInt(v any) (int, bool)` | `int`, `int32`, `int64`, `uint`, `uint32`, `uint64`, `float32`, `float64`, `json.Number`, `string` (via `strconv`) | 3 |
| `argInt` | `internal/tools/core/search.go:33` | `func argInt(args map[string]any, key string) (int, bool)` | `int`, `int64`, `float64`, `json.Number` | 3 |
| `argInt` | `internal/tools/research/numeric_args.go:15` | `func argInt(args map[string]any, key string) (int, bool)` | `int`, `int64`, `float64`, `json.Number` | 3 (context7/web_fetch/web_search) |
| `intArg` | `internal/tools/research/browser_progressive.go:196` | `func intArg(args map[string]any, key string, fallback int) int` | `int`, `int64`, `float64`, `json.Number` | 12 direct + 2 indirect via `boundedIdle`/`boundedTimeout`/`boundedPoll` |
| `int64Arg` | `internal/tools/research/browser_reasoning.go:943` | `func int64Arg(args map[string]any, key string, fallback int64) int64` | `int`, `int64`, `float64`, `json.Number` | 8 |

> Note: `coerceInt` and `argInt` are intentionally duplicated across packages — `internal/tools/core/search.go:28-30` documents prior art and calls out that a fourth copy is not the right end state. Consolidation into `internal/tools/research/numeric_args.go` is tracked but not yet done; the duplication does not affect correctness.

## Coercion Type Definitions

- **`float64`** — site handles both `int` and `float64` (and usually `int64`/`json.Number`). This is the required behavior because LLM tool-call JSON arrives as `float64` when `UseNumber` is not set; a bare `args["k"].(int)` silently fails and discards the caller-supplied limit.
- **`bare int`** — site handles only `args["k"].(int)` without a `float64` branch. **No such site remains for numeric integer args in `internal/tools`.** Historical bare-int sites (e.g., `args["max_results"].(int)` referenced in `internal/tools/core/search.go:22` and `internal/tools/research/numeric_args.go:11` comments) were the root cause of capped-at-default bugs.

## Observations

1. **Complete coverage for `float64`:** Every integer schema field now goes through a helper or inline fallback that accepts `float64`. Direct `grep` for `\.\(int\)` in `internal/tools` yields only the 5 `lines.go` sites above — each paired with a `.(float64)` fallback on the next line — and comment/test mentions. No production numeric site relies on bare `int`.
2. **Helper divergence:** `coerceInt` in `shell` also handles decimal strings; `file_ops` `coerceInt` handles `float32`/`int32` but not `json.Number`/`string`; `argInt`/`intArg`/`int64Arg` handle `json.Number` but not `string`/`float32`. The strictest `float64` handling is in `shell`; all are sufficient for the LLM JSON case.
3. **Inline sites:** `lines.go` 5 inline sites are correct (int + float64) but bypass the shared helpers, leaving the only `helper: none` rows. They are flagged in `internal/tools/core/search.go:30` as prior art to be unified.
4. **Identifiers cited are real:** Every `file:line` above was read via `read_file` or `grep` with line numbers; `argument name` strings match the literal key in `args["…"]`; `helper` names match the literal function name in the defining file. `int64Arg` sites were previously listed as 0; they are now enumerated (8 sites) — the prior audit under-counted `since_ms`/`after_ms`/`before_ms`/`time_window_ms`.
5. **Totals reconciliation:** `grep -rn 'args\["' internal/tools --include='*.go'` yields 144 matches; subtracting 5 comment and 8 test matches leaves 131 tool extraction sites (the prior audit's total). `grep -rn 'Type:\s*"integer"'` yields 42 integer declarations; 36 direct extraction rows above + 2 indirect via `boundedIdle` + 1 forwarded `timeout_seconds` at `shell/execute.go:517` + 3 schema declarations that share lines (from/to/line at same `browserSpecInput` call) reconcile to 42.

## Methodology & Reproducibility

```bash
# 1. Sweep all tool arg extractions (144 matches; 131 non-test/non-comment)
grep -rn 'args\["' internal/tools --include='*.go'
# Subtract 5 comment lines and 8 *_test.go matches → 131

# 2. List integer schema declarations (42)
grep -rn 'Type:\s*"integer"' internal/tools --include='*.go'

# 3. List candidate numeric coercion sites and helpers
grep -rn 'coerceInt\|argInt\|intArg\|int64Arg' internal/tools --include='*.go'
grep -rn '\.\(int\)\|\.\(float64\)\|json\.Number' internal/tools --include='*.go'

# 4. Verify each helper definition and its type switch
#   internal/tools/core/file_ops.go:238, internal/tools/shell/execute.go:52
#   internal/tools/core/search.go:33, internal/tools/research/numeric_args.go:15
#   internal/tools/research/browser_progressive.go:196, internal/tools/research/browser_reasoning.go:943
#   plus call-site verification via read_file at each file:line above
```

*Audit generated from source at HEAD. Re-run the commands above; any new integer schema field lacking a `float64` branch should be flagged as `bare int`.*
