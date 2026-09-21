# internal/browser

Browser automation for the agent loop: Chrome tabs driven over CDP, with
DOM/React state reified into Mangle facts for the live kernel.

> Verified 2026-09-20 against `main` (`231cfa7`, the same tree
> `Docs/architecture/diff` is pinned to).

The package comment states the scope in one line — browser automation with
DOM/React reification into Mangle facts, adapted from BrowserNERD
(`internal/browser/session_manager.go:1-3`).

Operating rules for the subsystem live beside the code, not here:
`internal/browser/agents.md:1-38` (profile sharing vs explicit isolation,
manager-owned lifetimes, sanitization before sinks, evidence handling,
session-scoped refs). This directory describes the code; that file governs
its use.

## Layout

59 Go files: 44 in the package root and three subpackages —
`internal/browser/security` (6 files: `path_policy`, `redactor`, platform
permission files), `internal/browser/specs` (6 files: `catalog`, `parser`,
`types`, plus tests), `internal/browser/testspec` (3 files: `parser`,
`types`, plus test). 32 of the 59 files are `*_test.go`. By filename stem,
the root clusters into `session_*`, `progressive_*`, `fact_*`,
`contract_audit*`, `docker_*`, `honeypot*`, `element_registry`,
`declarative_matcher`, `kernel_bridge`, `flight_recorder`, and `repo_trace`
— role descriptions below cover only what this pass re-derived from the
code (see scope note); the rest are inventory, not claims.

## Public API

All ranges are in `internal/browser/session_manager.go` (1017 lines) unless
noted.

| Symbol | Where | Notes |
|---|---|---|
| `Session` / `BrowserInstance` | 25-35 / 46-52 | public metadata: ids, URL/title/status, isolation flag, timestamps; browser control URL, default flag, tab count |
| `Config` / `DefaultConfig` | 94-130 / 205-230 | debugger URL, launch flags, headless, viewport, timeouts, session store, logging/ingestion switches, tab/browser caps, evidence settings, spec config, container correlation |
| `NewSessionManager` / `NewSessionManagerWithSink` | 373-379 / 382-384 | constructors; the `WithSink` variant injects the fact sink |
| `SessionManager.Start` / `Shutdown` | 488-495 / 522-524 | connect and teardown (`ensureStarted` at 497-505) |
| `CreateSession` / `Attach` / `Page` / `GetSession` | 573-575 / 578-580 / 583-592 / 607-615 | session lifecycle and page access |
| `List` / `ListSessions` / `DefaultSessionID` | 527-542 / 548-562 / 566-570 | enumeration |
| `ForkSession` | 801-868 | isolated fork (isolation is the default for forks per `agents.md:10-12`) |
| `Navigate` / `Click` / `Type` / `Screenshot` | 871-920 / 923-953 / 956-984 / 987-1016 | tab actions |
| `ReifyReact` / `Registry` | 639-798 / 619-630 | React/DOM reification and the element registry |
| `EngineSink` / `FactQuerier` / `engineAdapter` | 320-322 / 329-331 / 334-344 | kernel seam: `AddFacts` and `QueryFacts` |
| `LoadSpecs` / `SpecsConfig` / `SpecsEnabled` | 451-456 / 459-464 / 467 | spec-catalog wiring |
| `ResolveOutputPath` / `SanitizeForEvidence` / `WorkspaceRoot` | 470-472 / 475-477 / 480-485 | evidence-path handling |
| `CorrelateContainerErrors` | 428-448 | container-log correlation (BP-25) |
| `DiscoverContract` | `contract_audit.go:336-370` | contract discovery over a repo root |
| `AuditDiscoveryFacts` / `AssertAuditDiscovery` | `contract_audit_facts.go:131-148` / `153-166` | fact emission and assertion for discoveries |
| `ContractAuditInput` / `ContractAuditDiscovery` / `AuditFinding` | `contract_audit.go:191-198` / `201-207` / `31-36` | discovery I/O types |

## Seams and consumers

- CDP driver: `github.com/go-rod/rod` and `lib/proto`
  (`session_manager.go:20-21`); the only browser engine in the tree.
- Internal seams: `internal/browser/security` and
  `internal/browser/specs` are imported directly by the session manager
  (`session_manager.go:15-16`); the kernel is reached through
  `internal/logging` and `internal/mangle` (`session_manager.go:17-18`)
  via the `EngineSink`/`FactQuerier` interfaces, never a concrete kernel
  type.
- Tests carry their own sink: `TestEngineSink` with `AddFacts`/`GetFacts`
  (`internal/browser/browser_integration_test.go:20-39`), plus
  navigation, interaction, and no-frame-race integration tests
  (`browser_integration_test.go:41-233`).

## Scope note

Behavioral claims in `INTERNALS.md` are drawn from `session_manager.go`,
`contract_audit.go`, `contract_audit_facts.go`,
`browser_integration_test.go`, and `agents.md` only. Every other file in
the layout is listed as inventory in `WIRING-AND-NOT-BUILT.md`, with no
behavioral claim attached.

## Further reading

- `INTERNALS.md` — config and defaults, the session model, the manager's
  method groups, and the contract-audit pipeline.
- `WIRING-AND-NOT-BUILT.md` — what is wired and reachable, what degrades
  by design, what this pass did not re-derive, and which old claims are
  retired.
