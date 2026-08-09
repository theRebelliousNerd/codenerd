# browser — Architecture Corpus (`internal/browser`)

> Last verified against codebase: 2026-08-09
> Status: Living Reference Document — code-grounded full corpus  
> Language: Go (module `codenerd`)  
> Primary package: `internal/browser/`  
> Scale: **19** checked-in non-test Go files ≈ **7,007** lines (**18** per platform); **17** package test files; **0** package-local `.mg`
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
| [BROWSERNERD-PARITY.md](BROWSERNERD-PARITY.md) | Pinned BrowserNERD feature-parity contract and delivery gates |
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
go test ./internal/browser/... -count=1

# Integration tests (need Chrome/CDP; build tag integration)
go test -tags=integration ./internal/browser/ -count=1 -timeout 120s

# Honeypot policy unit tests (companion Mangle)
go test ./internal/core/defaults/policy/ -run Honeypot -count=1

# Production modular-registry progressive route (Chrome required)
go test -tags=integration ./internal/tools/research/ -run TestBrowserProgressiveTools_Live -count=1

# Production live-kernel reasoning/wait route (Chrome required)
go test -tags=integration ./internal/tools/research/ -run TestBrowserReasoningToolsLiveCortexRoute -count=1 -v
```

Native lifecycle/security settings live in `.nerd/config.json` under
`browser` (not `integrations.servers.browser`), including
`multi_tab_default`, `max_tabs`, `max_browsers`, `idle_tab_timeout_ms`,
`extra_sensitive_keys`, `writable_roots`, `evidence_enabled`, `evidence_dir`,
`max_evidence_files`, `max_evidence_file_bytes`, and the nested bounded
`specs` catalog (`sources`, roots/indexes/globs, and delivery ceilings).

Agent-facing progressive behavior is native and JIT-selected:
`browser_observe` returns bounded slices and opaque generation-bound refs;
`browser_act` consumes those refs through a closed, stop-on-error operation list.
`browser_act.started_ms` is the freshness watermark for `browser_wait`, while
`browser_reason` and read-only `browser_mangle` consume session-scoped evidence
from the same Cortex kernel that receives browser facts. `browser_evidence`
reads or explicitly exports bounded redacted per-session JSONL provenance.
`browser_specs` lists/ranks workspace-confined Markdown and checks declared
single-atom present/absent invariants against that same live session kernel.
`browser_test` creates, inspects, generates, and runs bounded selector-free
semantic fixtures; sensitive values use execution-only `value_env` expansion.
`browser_observe` and `browser_act` can attach ranked excerpts with
`include_specs` and bounded `spec_terms`.

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
