# transparency — Architecture Corpus (`internal/transparency`)

> Last verified against codebase: **2026-07-13**  
> Status: Living Reference Document  
> Language: Go (module `codenerd`)  
> Primary package: `internal/transparency/`  
> Implementation: **8** non-test `.go` files · **9** test files · **0** `.mg`

## Scope

This corpus documents codeNERD’s **operator-facing transparency layer**: Glass Box telemetry, always-on tool events, shard-phase observation, constitutional safety explanations, error classification, and Mangle derivation explainers.

It is **not** the kernel (`internal/core`), **not** the chat TUI (`cmd/nerd/chat`), and **not** the logging subsystem (`internal/logging`). Those consume this package; this package does not host them.

### What this package is for

Make internal operations visible **on demand** (and, for tools, always):

- Which shard is running, in which phase, for how long  
- Why a constitutional gate blocked an action  
- Why a Mangle fact / `next_action` was derived  
- Categorized errors with remediation  
- Live Glass Box event stream into the TUI  

### Design slogan (from `doc.go`)

> Opt-in · Non-intrusive · Lazy · Explains **why**, not only **what**

## Document map

| Doc | Role |
|-----|------|
| [IMPLEMENTED_SPEC.md](IMPLEMENTED_SPEC.md) | **Flagship** living architecture + inventory |
| [00-ALIGNMENT-VISION-REVIEW.md](00-ALIGNMENT-VISION-REVIEW.md) | North-star alignment scores with evidence |
| [01-VISION.md](01-VISION.md) | Target product/architecture vision |
| [02-CURRENT-STATE.md](02-CURRENT-STATE.md) | Precise on-disk inventory and hotspots |
| [03-GAP-ANALYSIS.md](03-GAP-ANALYSIS.md) | Spec vs reality, priorities, non-gaps |
| [04-ARCHITECTURAL-PRINCIPLES.md](04-ARCHITECTURAL-PRINCIPLES.md) | Binding package principles |
| [05-INTERNAL-ARCHITECTURE.md](05-INTERNAL-ARCHITECTURE.md) | Components, data flow, state machines |
| [06-PUBLIC-API-AND-TYPES.md](06-PUBLIC-API-AND-TYPES.md) | Exported types/funcs that matter |
| [07-DEPENDENCY-MAP.md](07-DEPENDENCY-MAP.md) | Upstream/downstream with evidence |
| [08-WIRING-AND-INTEGRATION.md](08-WIRING-AND-INTEGRATION.md) | Boot, chat, VirtualStore, shards |
| [09-SAFETY-AND-INVARIANTS.md](09-SAFETY-AND-INVARIANTS.md) | Concurrency, default-deny relation, gates |
| [10-TESTING-ALIGNMENT.md](10-TESTING-ALIGNMENT.md) | Tests, gaps, commands |
| [11-OBSERVABILITY.md](11-OBSERVABILITY.md) | Events, status, debug surfaces |
| [12-FAILURE-MODES.md](12-FAILURE-MODES.md) | Concrete failure modes + mitigations |
| [TODO.md](TODO.md) | Prioritized backlog |
| [OPEN-QUESTIONS.md](OPEN-QUESTIONS.md) | Real open design questions |
| [_progress.md](_progress.md) | Rebuild journal |

## Verify

```powershell
go test ./internal/transparency/...
go test -race ./internal/transparency/...
```

Cross-surface (consumers, not this package):

```powershell
go test ./cmd/nerd/chat/ -run GlassBox
go test ./internal/core/shards/ -run Transparency
```

## Fact-flow placement

```
user input → perception → user_intent → kernel (next_action / permitted)
  → VirtualStore / shards / tools
       │
       ├─ GlassBoxEventBus  (opt-in debug stream → TUI)
       ├─ ToolEventBus      (always-on tool lines → TUI)
       └─ TransparencyManager (toggle: phases, safety, verbose errors)
  → articulation → chat / stdout
```

Transparency **observes and explains**; it does not derive `permitted(...)` or select `next_action`. The Mangle kernel remains the executive.

## Quick orientation

| Concern | Primary file(s) |
|---------|-----------------|
| Package purpose | `doc.go` |
| Master coordinator | `transparency.go` (`TransparencyManager`) |
| Glass Box bus | `event_bus.go` + `glass_box_events.go` |
| Always-on tools | `ToolEventBus` in `glass_box_events.go` |
| Shard phases | `shard_observer.go` |
| Safety blocks | `safety_reporter.go` |
| Errors + remediation | `error_classifier.go` |
| Proof / decision narrative | `explainer.go` (uses `internal/mangle` traces) |
| Config schema | `internal/config/ux.go` → `TransparencyConfig` |

## Related corpora

- CLI / TUI glass box UX: `Docs/architecture/cli/` (esp. telemetry docs)  
- Kernel / VirtualStore: `Docs/architecture/core/`  
- Shard manager: `Docs/architecture/shards/` (or core shards docs)  
- Config: `Docs/architecture/config/`  
