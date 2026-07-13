# TODO — Northstar architecture / product backlog

> Last verified against codebase: 2026-07-13  
> Docs only — items are proposed work, not commitments

## P0 — Trust & correctness

- [ ] **Single vision authority**: define and implement bridge between `.nerd/northstar.json` and `Store.SaveVision` (or reverse: export JSON from Store for CLI).
- [ ] **Kernel wire parity**: call `SetParentKernel` in `session_boot.go` the same way as `session_shared_boot.go`.
- [ ] **Document operator path**: after wizard, how facts get into Guardian DB (runbook in CLI corpus if needed).

## P1 — Product completeness

- [ ] Persist wizard completion via `Guardian.UpdateVision` so `/alignment` and campaigns see the same vision.
- [ ] CLI: `nerd northstar history|drift|state` over SQLite.
- [ ] Emit or drop unused relational facts (`northstar_serves`, `supports`, `addresses`).
- [ ] Encode mitigation free text (or hash) instead of constant `/mitigation`.

## P2 — Quality & north-star discipline

- [ ] Atomize `buildAlignmentSystemPrompt` / user prompt under `internal/prompt/atoms/northstar/`.
- [ ] Use or remove `GuardianConfig.AlignmentModel`.
- [ ] Implement or remove `ingested_docs` + embedding relevance path.
- [ ] Integration test: boot with vision → `northstar_defined` query true.
- [ ] Chat adapter unit tests for `northstarHandlerAdapter`.

## P3 — Nice-to-have

- [ ] Metrics: checks total, blocked rate, mean score.
- [ ] Structured log fields (subject, trigger, score).
- [ ] Validate threshold ordering on `NewGuardian`.
- [ ] Singleton Guardian per session (avoid dual DB handles for `/alignment`).

## Done (already in code — do not re-implement)

- [x] SQLite store with vision/obs/checks/drift/state  
- [x] Guardian alignment pipeline with soft fallbacks  
- [x] CampaignObserver hard-block on `blocked`  
- [x] BackgroundEventHandler + boot registration  
- [x] Dense unit test suite  
