# usage — TODO

> Last verified: **2026-08-16**  
> Docs-only backlog derived from code. No commitment that items are scheduled.

## P0 — Metering completeness

- [x] Call `usage.FromContext` + `Track` from **every** production `perception.LLMClient` that receives usage metadata (not only ZAI). CLI engines excepted: their decoders receive no token counts.
- [x] Confirm streaming completion paths attach the same context and Track **once** with final billed tokens.
- [x] Standardize provider string ids (match config engine names). `perception.usageProviderID` + test against `config.SetAPIKeyForProvider`.

## P1 — Persistence correctness

- [x] Atomic save: write temp file then rename onto `usage.json`.
- [x] Fix dirty re-arm: under one critical section, Save then if mutations occurred while saving, keep dirty and re-arm timer.
- [x] Flush on Cortex close / chat shutdown (`Save` if dirty).
- [x] Use or remove `autoSaveTimer` field; prefer cancelable timer.

## P2 — Product surface

- [x] Either implement bounded `Events` ring **or** document reserved + stop implying raw event log.
- [x] Cost estimation: static price table keyed by model → fill `TokenCounts.Cost`.
- [x] UI: render `BySession`; optional cost column (`cmd/nerd/ui/usage_page.go` TODOs align).
- [x] Log Load/Save failures through `internal/logging`.

## P3 — Attribution & ops

- [x] Aggregate by shard **name** (or composite name+type) if operators need specialist-level spend.
- [x] Optional CLI: `nerd usage` / dump JSON to stdout for scripts.
- [x] Cap or prune `BySession` for long-lived workspaces.
- [x] Reject negative token inputs in `Track`.

## P4 — Architecture hygiene

- [x] Unify chat session tracker with Cortex tracker (single owner per process). `usage.Shared` refcounted registry.
- [x] Consider typed context keys for shard metadata. Done non-breakingly: typed keys on write, legacy string keys still honored on read.
- [x] Integration test: boot → NewContext → mock client Track → Save → reload.

## Still open

- [ ] Meter the CLI engines (`claude-cli`, `codex-cli`) — blocked until their
  response decoders surface token counts; nothing to record today.
  > Blocked upstream rather than deferred by preference: the CLI engine clients
  > do not surface token counts in their responses, so there is no number to
  > meter. The box cannot close until those decoders report usage.
- [x] Cross-process coordination of `usage.json` (two `nerd` processes on one
  workspace still last-writer-wins).
  > Closed. Save is now a cross-process read-merge-write. A platform-split
  > advisory lock (`internal/usage/filelock_windows.go` using `LockFileEx`,
  > `filelock_other.go` using `flock`) serialises writers across processes, since
  > `sync.Mutex` only covers goroutines inside one. Under that lock the tracker
  > re-reads `usage.json` and merges rather than overwriting: it keeps a
  > deep-copied baseline of the aggregates as of its last sync, so its own
  > contribution is `(current - baseline)` and the write is `(onDisk + contribution)`.
  > This works because every aggregate is an additive counter. Events are
  > deliberately not merged; they are a non-exhaustive ring with aggregates as the
  > durable record. A lock that cannot be acquired, or an unparseable `usage.json`,
  > degrades to the previous behaviour and still writes, because losing the merge
  > is survivable and losing the save is not. Covered by `crossprocess_test.go`,
  > including a double-count test that catches a shallow baseline copy.

## Explicit non-todos (unless product asks)

- Mangle predicates for hard token budgets  
- Cloud billing reconciliation  
- Vectryx / multi-workspace fleet dashboards  

## Doc maintenance

- [x] When Track producers expand, update [08-WIRING-AND-INTEGRATION.md](08-WIRING-AND-INTEGRATION.md) producer table.  
- [x] Keep IMPLEMENTED_SPEC status table in sync after code changes.
