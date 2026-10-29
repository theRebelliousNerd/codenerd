# sqlpragmas — Safety and Invariants

> Last verified: **2026-07-13**

## Scope of “safety” here

This package does **not** implement constitutional `permitted(...)` checks. Safety means:

- Durable open does not abort on optional tuning failures  
- Read-only handles are not force-mutated by write PRAGMAs  
- Historical data is not retroactively constrained by FK enforcement  
- Concurrent agents can share DB files under WAL without needless lock storms  

---

## Invariants

### I1 — Apply never panics on nil

`ApplyDefaultPragmas(nil, profile)` returns. **Test:** `TestApplyDefaultPragmas_NilDB_NoPanic`.

### I2 — Apply never returns error / never closes

Callers may always continue to schema setup after invoke. Documented in function comment.

### I3 — Per-PRAGMA isolation

One failed PRAGMA does not stop subsequent PRAGMAs in the list.

### I4 — ReadOnly does not enable WAL

`ProfileReadOnly` must not set `journal_mode=WAL`. **Tests:** unit + integration.

### I5 — Writable profiles enable WAL (when driver allows)

Hot / BulkBuild / Query include `journal_mode=WAL`. Failure is Debug-only (I2 still holds).

### I6 — journal_mode precedes dependent pragmas

In writable lists, `journal_mode` is first. Preserves intended WAL semantics for checkpoint settings.

### I7 — Foreign keys remain off by default

Shared presets must not enable `foreign_keys` until product-wide decision. **Evidence:** comment in `pragmasFor`.

### I8 — Leaf import set is closed

No upward product imports. Protects global compile graph.

### I9 — Default profile is Hot

Unknown `PragmaProfile` values fall through to Hot list (safe oversize preference vs empty apply).

### I10 — busy_timeout is set on all profiles including ReadOnly

All four profiles include `busy_timeout = 10000` to reduce immediate `SQLITE_BUSY` under concurrent access.

---

## Concurrency

| Concern | Status |
|---------|--------|
| Package global mutable state | None |
| Logger fetch | `logging.Get` — package’s concurrency model |
| Same DB concurrent Apply | Relies on `database/sql` pool |
| Cross-process writers | WAL + busy_timeout are the mitigations |

No Mangle stratification issues (no Mangle).

---

## Data safety tradeoffs

| Setting | Tradeoff |
|---------|----------|
| `synchronous=NORMAL` + WAL | Faster commits; not the strongest durability mode (`FULL`) |
| Large `mmap_size` | Faster reads; OS may cap; memory pressure on tiny hosts |
| Large `cache_size` | Faster; RAM use; SQLite treats as max |
| FK off | Allows historically inconsistent rows to persist |

These are **product choices** for a coding-agent workstation, not general DB server defaults.

---

## Security notes

- PRAGMA strings are **constants** (or sprintf of fixed integers) — no user string concatenation into PRAGMA SQL.  
- No path handling in this package (callers own DSN).  
- Debug logs include PRAGMA text and error; avoid putting secrets into custom PRAGMAs if ever added.

---

## Constitutional / policy boundary

| Responsibility | Owner |
|----------------|-------|
| May agent execute tool X? | Mangle `permitted` / policy |
| May code open path Y? | Caller / OS permissions |
| How is SQLite tuned? | **sqlpragmas** |

Do not move policy into this leaf.
