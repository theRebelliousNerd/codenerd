# usage — Architectural Principles

> Last verified: **2026-07-13**  
> Principles are **binding for this package**. Changes that violate them need an explicit design decision in OPEN-QUESTIONS / TODOs.

## P1 — Ambient metering, never control flow

`Track` and `FromContext` must not decide whether an LLM call runs. Absence of a tracker is success with no stats, not an error.

**Evidence:** ZAI uses `if tracker := usage.FromContext(ctx); tracker != nil { tracker.Track(...) }`.

## P2 — Soft-fail on disk

Corrupt or missing `usage.json` must not prevent Cortex boot. `NewTracker` may return a live empty tracker after a failed Load.

**Evidence:** `NewTracker` ignores Load error after create; comprehensive test `TestTracker_NewTracker_WhenCorruptFileExists_ShouldStillCreateTracker`.

## P3 — Workspace root is the unit of accounting

Tracker path is always `<workspace>/.nerd/usage.json`. Callers must pass the project workspace root (same discipline as config/session), not a random cwd.

**Evidence:** `NewTracker(workspacePath)`; boot uses `bctx.workspace` / `config.FindWorkspaceRoot`.

## P4 — Context carries the tracker; constructors do not globalize it

No package-level singleton tracker. Lifetime is owned by Cortex / chat Model; request paths inject via `NewContext`.

**Evidence:** typed `contextKey`; Cortex field `UsageTracker`; chat `usageTracker` field.

## P5 — Attribution degrades, never panics

Context values for shard/session keys are untyped. Type assertion failures → `"unknown"` bucket, not panic.

**Evidence:** comma-ok form in `Track`; `usage_tracker_context_test.go`.

## P6 — Stats returns isolated snapshots

Callers (UI) may mutate returned maps freely without corrupting the tracker.

**Evidence:** `copyTokenCountsMap` + `TestTracker_Stats_ShouldReturnCopy`.

## P7 — Aggregate-first persistence

Default durable form is aggregates, not full event streams (until an explicit ring buffer is designed). Keeps `usage.json` bounded.

**Evidence:** Track updates maps only; `Events` optional in schema.

## P8 — Sensor ≠ executive

Do not add Mangle Decl, `permitted`, or VirtualStore routes inside `internal/usage`. Budget enforcement, if required, is a higher layer that **reads** Stats or emits facts elsewhere.

## P9 — Operation tags are closed vocabulary by convention

Documented operations in types/comments: `chat`, `embedding`, `tool_gen`. Callers should use stable lowercase strings; free-form sprawl breaks operator tables.

## P10 — Wiring audit before deletion

`UsageEvent`, `Cost`, `autoSaveTimer`, and unused shard-name comments are integration seams or unfinished surface. Deleting them without reverse-dep + product check is forbidden under repo wiring rules.

## P11 — Mutex owns mutable state

All reads/writes of `data` and dirty flag transitions that touch shared maps go through `mu` (Save/Load/Track/Stats). Async AfterFunc must re-enter via `Save` / locked dirty clear.

## P12 — One version field, evolve carefully

`UsageData.Version` is currently `"1.0"`. Schema changes (new maps, event ring) should bump version and keep Load map-nil rehydration.
