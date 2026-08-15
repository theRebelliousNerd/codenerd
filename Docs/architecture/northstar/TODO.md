# TODO — Northstar architecture / product backlog

> Last verified against codebase: 2026-08-15  
> Docs only — items are proposed work, not commitments

## P0 — Trust & correctness

- [x] **Single vision authority**: define and implement bridge between `.nerd/northstar.json` and `Store.SaveVision` (or reverse: export JSON from Store for CLI).
- [x] **Kernel wire parity**: call `SetParentKernel` in `session_boot.go` the same way as `session_shared_boot.go`.
- [x] **Document operator path**: after wizard, how facts get into Guardian DB (runbook in CLI corpus if needed).

## P1 — Product completeness

- [x] Persist wizard completion via `Guardian.UpdateVision` so `/alignment` and campaigns see the same vision.
- [x] CLI: `nerd northstar history|drift|state` over SQLite.
- [x] Emit unused relational facts (`northstar_serves`, `supports`, `addresses`).
- [x] Encode mitigation free text (or hash) instead of constant `/mitigation`.

## P2 — Quality & north-star discipline

- [x] Atomize `buildAlignmentSystemPrompt` / user prompt under `internal/prompt/atoms/northstar/`.
- [x] Use `GuardianConfig.AlignmentModel`.
- [x] Implement `ingested_docs` + embedding relevance path.
- [x] Integration test: boot with vision → `northstar_defined` query true.
- [x] Chat adapter unit tests for `northstarHandlerAdapter`.

## P3 — Nice-to-have

- [x] Metrics: checks total, blocked rate, mean score.
- [x] Structured log fields (subject, trigger, score).
- [x] Validate threshold ordering on `NewGuardian`.
- [x] Singleton Guardian per session (avoid dual DB handles for `/alignment`).

## Implementation notes (2026-08-15)

All items above are implemented in code and covered by tests in
`internal/northstar/` and `cmd/nerd/chat/northstar_adapter_test.go`.

- **Single vision authority** — `internal/northstar/bridge.go`. The SQLite store
  is the durable record; `.nerd/northstar.json` / `.mg` are import/export
  surfaces. `SyncVisionAuthority` reconciles bidirectionally and runs inside
  `Guardian.Initialize`, so every boot path converges. See
  [13-OPERATOR-RUNBOOK.md](13-OPERATOR-RUNBOOK.md).
- **Relational facts** — `Capability.Serves`, `Requirement.Supports` and
  `Requirement.Addresses` now emit `northstar_serves` / `northstar_supports` /
  `northstar_addresses`. Links whose target is absent from the vision are
  dropped rather than emitted dangling.
- **Mitigation encoding** — `northstar_mitigation(RiskID, /mit_<slug>_<hash>)`
  plus a new `northstar_mitigation_text(RiskID, Text)` Decl in
  `internal/core/defaults/schemas_misc.mg` carrying the free text.
- **Singleton Guardian** — `internal/northstar/registry.go`
  (`AcquireGuardian`/`ReleaseGuardian`, refcounted per `.nerd` dir).
- **Prompt atoms** — `internal/prompt/atoms/northstar/guardian_alignment.yaml`,
  resolved by `internal/northstar/alignment_prompt.go`. northstar deliberately
  does not import `internal/prompt` (leaf package); a parity test fails if the
  two copies diverge by a byte.

## Done (already in code — do not re-implement)

- [x] SQLite store with vision/obs/checks/drift/state  
- [x] Guardian alignment pipeline with soft fallbacks  
- [x] CampaignObserver hard-block on `blocked`  
- [x] BackgroundEventHandler + boot registration  
- [x] Dense unit test suite  
