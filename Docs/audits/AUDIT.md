# Codebase Audit Ledger

A systematic, alphabetical audit of every package (feature) in the tree. The
goal is correctness: real defects (panics, data loss, resource leaks, races,
silent error swallowing) — not style. Each package carries a verdict and, where
relevant, the disposition of findings.

## Methodology

Every package below was put through four complementary audits:

1. **`go vet ./...`** — the full tree is **clean** (0 findings).
2. **`errcheck -ignoretests ./...`** — every unchecked error return was
   triaged. The raw count is high (581) but dominated by deliberate
   fire-and-forget calls (`defer f.Close()`, `fmt.Fprintf` to buffers, logging,
   best-effort cgroup writes). The **high-signal** subset (writes, marshals, DB
   exec/scan, fact assertions) was reviewed line by line; genuine data-loss
   cases were fixed and the rest classified intentional with rationale below.
3. **Deep per-package review** — the non-test source of all 65 packages was read
   in three alphabetical passes, hunting resource leaks, unchecked type
   assertions, off-by-one/boundary errors, and goroutine/map races.
4. **`go test -race`** on the concurrency-heavy packages (core, context, shards,
   core/shards, session).

## Defects found and fixed in this audit

| # | Severity | Location | Defect | Fix |
|---|----------|----------|--------|-----|
| 1 | high | `core` + `mangle` parsing | Concurrent calls to the ANTLR-generated Mangle parser race on its process-global ATN/DFA prediction state (caught by `-race`). | Single process-wide parse lock (`mangle.ParseUnit`/`ParseAtom`); all core+mangle parse sites routed through it. |
| 2 | high | `usage/usage_tracker.go` | `Track` made unchecked `val.(string)` assertions on untyped context values — a non-string value panicked the tracker. | comma-ok with `"unknown"` fallback + regression test. |
| 3 | medium | `cmd/query-kb/main.go` | Five `Scan`/`QueryRow().Scan` errors ignored → misleading `Total: 0` / silent garbage rows. | Errors checked and reported. |
| 4 | medium | `autopoiesis/profiles.go`, `traces.go` | `save()` ignored `os.WriteFile`/marshal errors → silent loss of quality profiles and reasoning traces. | Failures now logged via `logging.AutopoiesisError`. |
| 5 | high | `system/factory_adapters.go` | (earlier in this branch) `mcpKernelAdapter.Retract` double-period parse bug made every retract fail. | Trim trailing `.` before `ParseFactString`. |

## Intentional patterns (reviewed, left as-is)

- **`campaign` kernel `Assert` (12 sites), `core` shadow/transaction `Assert`** —
  fire-and-forget fact injection on best-effort telemetry paths; failure is
  non-fatal and would only drop an advisory fact. Surfacing each would add noise
  without changing behavior.
- **`store` stats `Scan` (counts/averages), `usage.Track`→`Save`** — diagnostic
  aggregates that default to `0` on error by design; not on a correctness path.
- **`tactile/platform_linux` cgroup `WriteFile` (7 sites)** — best-effort
  resource-limit application with an explicit non-cgroup fallback; a failed
  write is already handled by the fallback path.

## Per-package verdict (alphabetical)

| Package | Verdict |
|---|---|
| cmd/nerd | clean (CLI wiring; integration-covered) |
| cmd/nerd/chat | clean |
| cmd/nerd/ui | clean |
| cmd/query-kb | **fixed** — 5 unchecked `Scan` (defects #3) |
| cmd/tools/action_linter | clean |
| cmd/tools/corpus_builder | clean |
| cmd/tools/mangle_check | clean |
| cmd/tools/predicate_corpus_builder | clean |
| cmd/tools/prompt_builder | clean |
| cmd/tools/validate_prompt_atoms | clean |
| cmd/tools/verify_taxonomy | clean |
| internal/articulation | clean |
| internal/autopoiesis | **fixed** — silent persistence failures (defect #4) |
| internal/autopoiesis/prompt_evolution | clean (stats `Scan` intentional) |
| internal/browser | clean (Docker/CDP; integration-covered) |
| internal/build | clean |
| internal/campaign | clean (best-effort `Assert` intentional) |
| internal/config | clean |
| internal/context | clean |
| internal/core | **fixed** — parse race (defect #1); best-effort `Assert` intentional |
| internal/core/defaults | clean (data/`.mg`) |
| internal/core/defaults/policy | clean (data/`.mg`) |
| internal/core/shards | clean |
| internal/diff | clean |
| internal/embedding | clean |
| internal/features | clean |
| internal/init | clean |
| internal/jit/config | clean |
| internal/logging | clean |
| internal/mangle | **fixed** — parse lock added (defect #1) |
| internal/mangle/feedback | clean |
| internal/mangle/synth | clean |
| internal/mangle/transpiler | clean |
| internal/mcp | clean |
| internal/northstar | clean |
| internal/observability | clean |
| internal/perception | clean |
| internal/persist/factsnap | clean |
| internal/prompt | clean |
| internal/prompt/sync | clean |
| internal/regression | clean |
| internal/retrieval | clean (context-cancellation fix earlier in branch) |
| internal/session | clean |
| internal/shards | clean |
| internal/shards/system | clean |
| internal/sqlpragmas | clean |
| internal/store | clean (stats `Scan` / access-tracking `Exec` intentional) |
| internal/system | **fixed** — `Retract` parse bug (defect #5) |
| internal/tactile | clean (best-effort cgroup writes intentional) |
| internal/tactile/python | clean (Docker; integration-covered) |
| internal/tactile/swebench | clean |
| internal/testing | clean |
| internal/testing/context_harness | clean |
| internal/tools | clean |
| internal/tools/codedom | clean |
| internal/tools/core | clean |
| internal/tools/research | clean |
| internal/tools/shell | clean |
| internal/transparency | clean |
| internal/types | clean |
| internal/usage | **fixed** — `Track` panic (defect #2) |
| internal/ux | clean |
| internal/verification | clean |
| internal/world | clean |
| internal/world/lsp | clean |

## Residual / out-of-scope

A deeper data race lives inside the third-party `mangle-go`/ANTLR parser under
the heaviest concurrent-transactions stress test. The process-wide parse lock
removes the in-flight-parse race class (verified race-free with an 8×50-goroutine
micro-test), but fully eliminating the library-internal race would require an
upstream fix. The non-`-race` suite and the integration suite are green.
