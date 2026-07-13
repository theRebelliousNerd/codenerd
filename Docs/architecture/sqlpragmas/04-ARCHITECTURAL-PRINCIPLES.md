# sqlpragmas — Architectural Principles

> Last verified: **2026-07-13**  
> Binding principles specific to this package. New changes should not violate them without an explicit corpus update.

---

## P1 — Remain a leaf

**Statement:** `sqlpragmas` must not import `internal/store`, `internal/core`, `internal/mcp`, `internal/prompt`, or any mid-layer product package.

**Why:** Cycle-breaking is the package’s structural purpose (`pragmas.go` package comment).

**Test:** `go list -f '{{.Imports}}' codenerd/internal/sqlpragmas` stays tiny.

---

## P2 — Best-effort tuning, never fail open

**Statement:** `ApplyDefaultPragmas` never returns an error and never closes the DB. Per-PRAGMA failures are Debug-logged and skipped.

**Why:** Driver and platform variance (modernc vs mattn, RO mode, OS mmap caps).

**Forbidden:** Turning a single failed PRAGMA into a fatal open without multi-site migration.

---

## P3 — Profiles are the vocabulary, not free-form lists

**Statement:** Product code selects a `PragmaProfile`; it does not paste 7 PRAGMA strings at each open.

**Why:** Consistency, reviewability, single place to change defaults.

**Exception:** Schema-local extras (e.g. `foreign_keys=ON`) after apply, documented at the site.

---

## P4 — Order matters: journal_mode first

**Statement:** Writable profiles set `journal_mode=WAL` before WAL-dependent pragmas.

**Why:** Source comment on `pragmasFor`; checkpoint/sync semantics depend on journal mode.

---

## P5 — ReadOnly must not attempt write pragmas

**Statement:** `ProfileReadOnly` omits `journal_mode`, `synchronous`, and `wal_autocheckpoint`.

**Why:** Avoid SQLite rejections and Debug spam on `mode=ro` opens.

**Test:** `TestApplyDefaultPragmas_ProfileReadOnly_NoJournalChange` and integration row.

---

## P6 — Foreign keys are not a silent default

**Statement:** Do not add `PRAGMA foreign_keys = ON` to the shared presets without a coordinated data/schema program.

**Why:** Historical schemas declare FKs without enforcement; enabling is a behavior change on user data.

---

## P7 — Sizes are upper bounds, not requirements

**Statement:** Document and test mmap as “enabled (positive)” not “exactly N bytes” on all platforms.

**Why:** SQLite and OS caps (especially Windows CGO builds).

---

## P8 — Apply immediately after Open

**Statement:** Callers apply pragmas before schema init and first query so the first pooled connection is tuned.

**Why:** Function contract comments; reduces half-tuned races.

---

## P9 — Prefer direct leaf import when store is unavailable

**Statement:** Packages that cycle with `store` import `sqlpragmas` directly. Packages already deep in `store` may use re-exports.

**Why:** `internal/store/pragmas.go` and mcp pattern.

---

## P10 — Silent success, noisy only on anomaly

**Statement:** Successful apply logs nothing; failures log Debug under `CategoryStore`.

**Why:** Opens are frequent; Info spam would drown real store signals.

---

## P11 — Profile table changes are API changes

**Statement:** Changing cache/mmap/busy defaults is a cross-system performance and durability change.

**Why:** Fan-out across store, prompt, mcp, system, tools, chat boot.

**Process:** Update tests + this corpus + note in store/architecture docs.

---

## P12 — No policy cosplay

**Statement:** Do not encode action permission, shard policy, or “may this tool open a DB” in sqlpragmas.

**Why:** Constitutional executive authority lives in Mangle/kernel; this package tunes sockets to the file, not rights.
