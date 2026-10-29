# 00 — Alignment & Vision Review: observability (`internal/observability`)

> Last verified against codebase: 2026-07-13  
> Status: Living Reference Document — code-grounded  
> Source: `internal/observability/` (2 non-test Go files ≈ 326 lines; 3 test files ≈ 369 lines)

## 1. North-star statement

codeNERD separates **creative** LLM work from **executive** Mangle control. Host diagnostics must support that split without becoming a second executive or a silent bypass:

1. Capture enough **runtime truth** at boot and on crash to debug the host, without inventing agent policy.
2. Remain a **leaf**: never import core, session, shards, or tools (no cycles; no side-channel control).
3. Prefer **cheap always-on** flight recording (default on) so the first production panic pays for the feature.
4. Emit through **shared logging categories**, not ad-hoc stdout sprawl.

`internal/observability` is the process-level answer to (1)–(4) for metrics + execution traces. Product glass-box and kernel explainability are other packages.

## 2. Alignment dimensions

| Dimension | Score (0–5) | Evidence |
|-----------|-------------|----------|
| Creative/executive split | **5** | No LLM calls, no Mangle facts, no VirtualStore routes — pure host diagnostics (`runtime_metrics.go`, `flight_recorder.go`) |
| Fact-flow non-interference | **5** | Not on `user_intent → next_action` path; only process boot/panic in `cmd/nerd/main.go` |
| Leaf purity / import discipline | **5** | Imports only stdlib + `internal/logging`; comment documents leaf intent |
| Wiring completeness | **4** | Metrics + start + panic dump wired in `main`; feature flag live; **on-demand dump claimed in features comments but unwired** |
| Test grounding | **4** | Metric path support test, format helpers, start/stop/dump, panic lifecycle, double-dump; no main-integration test |
| Operator discoverability | **2** | Artifacts under `.nerd/traces/`; no Cobra/`/diag` dump command; operators must know panic path + `go tool trace` |
| Safety / side-effect hygiene | **4** | Mutex singleton; buffer-before-write dump; start failure is warn-not-fatal; empty nerdDir rejected |
| Forward-compat with Go runtime | **5** | `TestStartupMetricPaths_AllSupported` pins metric names to `runtime/metrics.All()`; Go 1.25+ FlightRecorder API used |
| Scope honesty | **5** | Package header limits claims to startup metrics + flight recorder; does not pretend to be full APM |
| North-star tooling cost | **4** | Default-on 64 MiB ring is intentional; no runtime sampling of agent turns (may need higher layers later) |

**Overall alignment: 4.3 / 5** — excellent leaf diagnostics with real wiring; residual gap is operator-facing on-demand dump and panic-scope limits, not missing core code.

## 3. What “good” looks like (package-specific)

| Good | Bad |
|------|-----|
| Boot snapshot in CategoryBoot with structured fields | Silent metrics that require a debugger attach |
| Ring buffer dump on panic with path printed to stderr | Process death with no trace artifact |
| Feature flag + env override (`NERD_FLIGHTREC`) | Hard-coded always-on with no escape hatch for constrained hosts |
| Metric paths tested against live runtime | Hard-coded paths that go KindBad after Go upgrade |
| Buffer-then-write dump | Streaming dump that corrupts recorder state on I/O failure |
| Clear separation from prompt “flight recorder” | Overloaded terms without docs |

## 4. Misalignment risks

1. **False confidence on panics recovered inside chat** — main’s defer never runs; operators assume every panic dumps.  
2. **Features comment vs reality** — `/diag flightrec` not implemented; docs must not claim it.  
3. **Disk pressure** — 64 MiB ring + trace files under `.nerd/traces/`; no retention policy in this package.  
4. **Workspace vs CWD** — dump uses workspace captured at `main` start (`os.Getwd`); `--workspace` chdir for interactive chat happens later inside `rootCmd.RunE`, so panic dump directory may not match the active workspace flag in all edge cases.

## 5. Related corpora

- `Docs/architecture/logging/` — CategoryBoot sink  
- `Docs/architecture/features/` — flight recorder flag  
- `Docs/architecture/cli/` — sole production importer; CLI telemetry overview  
- `Docs/architecture/prompt/` — distinct PromptManifest “flight recorder”
