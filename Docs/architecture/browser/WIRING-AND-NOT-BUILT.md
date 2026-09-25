# browser wiring and what is NOT built

> Verified 2026-09-20 against `main` (`231cfa7`).

## Wired and reachable

- The CDP driver is `go-rod/rod` (`session_manager.go:20-21`). Nothing
  else in the tree drives a browser; there is no second engine, matching
  the `agents.md:7-9` rule.
- The session manager imports `internal/browser/security` and
  `internal/browser/specs` directly (`session_manager.go:15-16`), and
  reaches the kernel only through `internal/logging` and
  `internal/mangle` via the `EngineSink`/`FactQuerier` interfaces
  (`session_manager.go:17-18, 320-344`). The sink is injectable:
  `NewSessionManagerWithSink` (`session_manager.go:382-384`).
- The test-only sink `TestEngineSink`
  (`browser_integration_test.go:20-39`) is the sanctioned package-local
  sink pattern (`agents.md:18-19`); production facts go to the live
  kernel.

## Degrades by design (not missing)

- Empty `CorrelationContainers` disables container correlation; empty
  `DockerPath` disables it without failing (`session_manager.go:121-129`).
- Zero event-throttle milliseconds yields a nil throttler whose `Allow`
  returns true (`session_manager.go:66-90`): throttling off, not an error.
- Evidence and epoch bounds are read through getters with defaults
  (`GetMaxEvidenceFiles` 240-248, `GetMaxEvidenceFileBytes` 251-259,
  `GetMaxEpochEventFacts` 197-202).

## Not re-derived in this pass

Behavioral claims above stop at `session_manager.go`,
`contract_audit.go`, `contract_audit_facts.go`,
`browser_integration_test.go`, and `agents.md`. The following exist in
the tree and are listed here as inventory only — no behavioral claim is
made about them: `session_lifecycle.go`, `session_manager_dom.go`,
`progressive_action.go`, `progressive_observe.go`, `element_registry.go`,
`declarative_matcher.go`, `kernel_bridge.go`, `fact_epoch.go`,
`fact_redaction.go`, `flight_recorder.go`, `contract_audit_report.go`,
`repo_trace.go`, `docker_correlation.go`, `docker_fetcher.go`,
`honeypot.go`, `honeypot_gate.go`, all of `security/`, `specs/`, and
`testspec/` beyond their import wiring, and every `*_test.go` not named
in `INTERNALS.md`.

## Old claims, retired or corrected

- `Docs/architecture/INDEX.md` described this package as "7 go / 10
  tests". The tree holds 59 Go files, 32 of them tests. The count was
  wrong, not stale.
- The same row linked `[SPEC](browser/IMPLEMENTED_SPEC.md)` and
  `corpus.toml` named that file its `implemented_spec`; the file is
  deleted. `corpus.toml` now points at `README.md`.
- Parity language ("BPAR-1 partial", `BROWSERNERD-PARITY.md`) had no
  citation in the code read for this pass and is not repeated here.
- The previous corpus's own unfinished list
  (`Docs/architecture/UNFINISHED-FEATURES.md`, "Browser (18)") named TUI
  status and slash command, the VS browse delegate, a CI integration
  job, header-ingestion default policy, and the BPAR-5 live parity gate
  as not done. Those items are carried here as unverified-in-code, not
  as findings: `honeypot_gate.go` and `fact_epoch.go` exist as files,
  but this pass did not read them and makes no claim about whether the
  honeypot-gate or fact-GC behavior is implemented.

## Wave-3 verification (2026-09-25)

Each carried-over "not done" item was checked against the code; each
unreachable function in the dead-code baseline was classified.

| Item | Class | Evidence |
|------|-------|----------|
| TUI `/browser` command | ALREADY DONE | `cmd/nerd/chat/commands.go` dispatches `/browser` to `Model.handleCmdBrowser` (`cmd/nerd/chat/commands_handlers_misc.go:375`); listed in `command_categories.go`. |
| VirtualStore browse delegate | ALREADY DONE (typed) / honest refusal (generic) | Typed `browser_navigate/extract/screenshot/click/type/close` actions route through the modular browser tools (`internal/core/virtual_store_actions.go`, `handleResearch` arg mapping); the generic `/browse` action refuses with `browser_routing(Op, /requires_shard)` rather than claiming success (`handleBrowse`). |
| Header-ingestion default policy | ALREADY DONE | `HeaderIngestionOff` is the operator default, `HeaderIngestionRedacted` the research default (`session_manager.go`, header-ingestion constants); `session_manager_dom.go` gates capture on `ShouldIngestHeaders()`. |
| Honeypot interaction gate | ALREADY DONE | `SessionManager.guardElement` (`honeypot_gate.go:78`) runs before click/type (`session_manager.go`) and progressive actions (`progressive_action.go`); boot binds the live kernel querier (`internal/system/factory.go`, `SetFactQuerier`). |
| Fact epoch GC | ALREADY DONE | Navigation calls `RollSessionEpoch` (`session_manager_dom.go`); boot binds `NewKernelFactRetractor`. |
| Browser CI integration job | DECLINE (environment) | CI has no Chrome; the live suites are gated tests. Not buildable without a browser runner. |
| BPAR-5 live parity gate | DECLINE (unowned claim) | No code or spec in the tree defines BPAR-5; nothing to build against. |
| Evidence privacy on append (`security.IsPrivatePath`) | REAL gap, WIRED | `FlightRecorder.Record` now re-verifies owner-only policy before appending to an existing trace, re-protects a loosened file and refuses one it cannot re-protect (`flight_recorder.go`, `ensurePrivateEvidence`). Proven by `TestFlightRecorderReprotectsLoosenedEvidenceBeforeAppend` (fails with the check removed). |
| Contract-audit report/resume (`BuildAuditReport`, `ResumeAuditEvidence` and helpers) | REAL gap, cross-lane | Built and tested in `contract_audit_report.go`, but the only consumer surface, `internal/tools/research/browser_audit.go`, exposes `operation: discover` only. Wiring `report`/`resume` operations belongs to the tools lane; left in the dead-code baseline until then. |

## Stubs

None. The 19 forwarding and template files previously in this directory
were removed; nothing here points at a document that no longer exists.
