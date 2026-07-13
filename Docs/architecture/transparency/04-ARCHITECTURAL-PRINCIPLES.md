# transparency — Architectural Principles

> Last verified: 2026-07-13  
> Binding for work in `internal/transparency/` and for consumers emitting into it.

## P1 — Observe, never govern

Transparency types **must not** assert Mangle facts, alter `next_action`, or grant `permitted`. They format and emit side-channel data only. Policy stays in `internal/core/defaults/policy/` and the kernel.

## P2 — Non-intrusive by construction

Emit paths **must not block** the executive loop. Prefer:

- Buffered channels  
- Non-blocking send with drop  
- Early return when disabled  

If you add a synchronous subscriber callback that does I/O, you violated this principle.

## P3 — Always-on tools, opt-in depth

`ToolEventBus` remains **ungated** by Glass Box / Transparency master switches. Deeper subsystem streams (Glass Box categories, shard phases, verbose errors) are config/toggle gated. Do not merge these into one “debug only” bit.

## P4 — Separate Track A and Track B ownership

- **Track A:** `TransparencyManager` + observer/reporter/classifier façades.  
- **Track B:** `GlassBoxEventBus` / `ToolEventBus` instances.  

Do not force all telemetry through the Manager unless you also redesign boot. Document which track a new feature uses.

## P5 — Explain “why”, not only “what”

New user-facing strings should answer:

- Why blocked / why derived / what to try next  

Prefer structured fields (`Rule`, `Category`, remediation lists) over opaque log dumps. When attaching to derivation trees, preserve EDB vs IDB distinction (`SourceEDB` / derived).

## P6 — Config is the schema of truth for toggles

Feature flags live on `config.TransparencyConfig`. Do not invent parallel env vars inside this package. If a flag appears in `GetStatus()`, it must have a real effect or be removed/labeled experimental in docs and status text.

## P7 — Sequence and turn affinity for multi-producer streams

Glass Box events carry sequence IDs and optional `TurnID`. Producers should set meaningful `Category`, `Summary`, `Source`, and `Duration` when known. Consumers may filter/clear by turn—do not overload Summary as a free-form log blob if Details exists.

## P8 — Bounded history

In-memory rings (`maxHistory` on observer/reporter, bus buffer, chat max events) stay **bounded**. Unbounded append = memory leak under long sessions/campaigns.

## P9 — Dependency direction: core → transparency, never reverse into UI

`internal/core` and shards may import `transparency`. `transparency` may import `config` and `mangle` for formatting. It must not import `cmd/nerd`, Bubble Tea, or TUI types. UI adapts events to messages; events stay UI-agnostic.

## P10 — Heuristics are labeled heuristics

String-matching classifiers (`ClassifyError`, `classifyViolation`, `ExplainSafetyAction`) are **best-effort UX**. Prefer structured error types at the source when accuracy matters. Do not claim formal verification of safety classification.

## P11 — Lazy expensive work

Building full derivation narratives, large status dumps, or recursive trees only when a command/UI path requests them. Hot emit paths stay allocation-light (struct values, short strings).

## P12 — Wiring audit before deletion

This repo has partial integrations. Before removing “unused” Manager APIs or Observer:

1. Grep chat, core, shards, tests.  
2. Prefer feeding dormant APIs over deleting if product still wants them.  
3. If demoting, update `GetStatus` and config so operators aren’t lied to.

## Principle → file anchors

| Principle | Primary anchors |
|-----------|-----------------|
| P1 | package-wide; no kernel assert APIs |
| P2 | `event_bus.go`, `glass_box_events.go` ToolEventBus |
| P3 | `ToolEvent` comment; Manager defaults |
| P4 | `transparency.go` vs bus constructors in chat |
| P5 | `explainer.go`, `safety_reporter.go`, `error_classifier.go` |
| P6 | `config/ux.go`, `GetStatus` |
| P7 | `GlassBoxEvent`, sequence in Emit |
| P8 | maxHistory fields |
| P9 | import graph |
| P10 | ClassifyError switch |
| P11 | Explainer call sites |
| P12 | `SetTransparencyManager` |
