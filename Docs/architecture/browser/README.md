# browser — Architecture Corpus (`internal/browser`)

> Last verified against codebase: 2026-07-13  
> Status: Living Reference Document — code-grounded full corpus  
> Language: Go (module `codenerd`)  
> Primary package: `internal/browser/`  
> Scale: **3** non-test Go files ≈ **1,900** lines; **6** test files; **0** package-local `.mg`  
> Companion Mangle: `internal/core/defaults/schemas_browser.mg`, `policy/browser.mg`, `policy/browser_honeypot.mg`

## Scope

This corpus documents the **Browser Physics** surface: Rod/Chrome session lifecycle, DOM/React reification into Mangle facts, CDP event streams, and honeypot detection that feeds the logic kernel.

It is **not**:

- The CodeDOM / surgical edit CLI (`cmd/nerd/dom_*.go`) — separate surface
- The research modular tools wrapper (`internal/tools/research/browser.go`) — consumer of this package
- VirtualStore action routing (`internal/core/virtual_store_*.go`) — downstream executor path

## Document map

| Doc | Role |
|-----|------|
| [IMPLEMENTED_SPEC.md](IMPLEMENTED_SPEC.md) | **Flagship** living architecture + inventory + deep dives |
| [00-ALIGNMENT-VISION-REVIEW.md](00-ALIGNMENT-VISION-REVIEW.md) | North-star alignment scores |
| [01-VISION.md](01-VISION.md) | Target product/architecture vision |
| [02-CURRENT-STATE.md](02-CURRENT-STATE.md) | Precise on-disk inventory |
| [03-GAP-ANALYSIS.md](03-GAP-ANALYSIS.md) | Spec vs reality, priorities |
| [04-ARCHITECTURAL-PRINCIPLES.md](04-ARCHITECTURAL-PRINCIPLES.md) | Binding package principles |
| [05-INTERNAL-ARCHITECTURE.md](05-INTERNAL-ARCHITECTURE.md) | Components, data flow, state machines |
| [06-PUBLIC-API-AND-TYPES.md](06-PUBLIC-API-AND-TYPES.md) | Exported types and constructors |
| [07-DEPENDENCY-MAP.md](07-DEPENDENCY-MAP.md) | Upstream/downstream with evidence |
| [08-WIRING-AND-INTEGRATION.md](08-WIRING-AND-INTEGRATION.md) | CLI, boot, shards, tools, Mangle |
| [09-SAFETY-AND-INVARIANTS.md](09-SAFETY-AND-INVARIANTS.md) | Safety, concurrency, constitutional hooks |
| [10-TESTING-ALIGNMENT.md](10-TESTING-ALIGNMENT.md) | Tests, gaps, commands |
| [11-OBSERVABILITY.md](11-OBSERVABILITY.md) | Logging categories, timers |
| [12-FAILURE-MODES.md](12-FAILURE-MODES.md) | Concrete failures + mitigations |
| [TODO.md](TODO.md) | Prioritized backlog |
| [OPEN-QUESTIONS.md](OPEN-QUESTIONS.md) | Real open design questions |
| [_progress.md](_progress.md) | Rebuild progress log |

### Legacy filenames (superseded)

Earlier stub names (`01-DOMAIN-MODEL.md`, `02-CURRENT-STATE-BROWSER.md`, …) are superseded by the table above. Prefer the canonical set.

## Verify

```powershell
# Unit + coverage tests (no Chrome required for most)
go test ./internal/browser/ -count=1

# Integration tests (need Chrome/CDP; build tag integration)
go test -tags=integration ./internal/browser/ -count=1 -timeout 120s

# Honeypot policy unit tests (companion Mangle)
go test ./internal/core/defaults/policy/ -run Honeypot -count=1
```

### Manual CLI smoke (optional)

```powershell
# Long-running Chrome + control URL under .nerd/browser/
.\nerd.exe browser launch
# separate shell:
.\nerd.exe browser session https://example.com
.\nerd.exe browser snapshot <session-id>
```

## Fact-flow placement

```
user intent → kernel next_action → VirtualStore / modular tools / tactile router
  → internal/browser.SessionManager (Chrome/Rod)
  → EngineSink.AddFacts (DOM / React / net / events)
  → Mangle predicates (schemas_browser.mg + browser*.mg policy)
  → safe_interactable / is_honeypot / spatial rules
  → articulation / further actions
```

North star: the LLM does not “decide” the DOM is safe; **Mangle derives** `is_honeypot` / `safe_interactable` from asserted CSS/geometry facts.
