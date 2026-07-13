# sqlpragmas — Internal Architecture

> Last verified: **2026-07-13**

## Components

There is essentially **one component**: a pure preset table + apply loop.

```
┌─────────────────────────────────────────────┐
│              sqlpragmas (leaf)              │
│                                             │
│  PragmaProfile ──► pragmasFor ──► []string  │
│                         │                   │
│                         ▼                   │
│              ApplyDefaultPragmas            │
│                 │            │              │
│                 ▼            ▼              │
│            db.Exec     logging.Debug        │
└─────────────────────────────────────────────┘
           ▲                    ▲
           │ re-export          │ direct
    internal/store         mcp, prompt, core, …
```

## Data flow

1. **Input:** `*sql.DB` (already opened), `PragmaProfile`.
2. **Transform:** profile → ordered list of SQL PRAGMA strings.
3. **Effect:** each string executed via `db.Exec`.
4. **Side channel:** Debug log on failure.
5. **Output:** none (void); DB connection state mutated per SQLite rules.

No persistent package state. No globals except what `logging.Get` may hold.

## Key types

| Type | Kind | Role |
|------|------|------|
| `PragmaProfile` | `int` enum | Workload discriminator |
| `[]string` (unexported lists) | PRAGMA statements | Preset body |

No interfaces. No structs. No options pattern.

## State machine (apply)

```
START
  │
  ├─ db == nil ──────────────────────► END
  │
  ▼
GET logger (CategoryStore)
  │
  ▼
LIST = pragmasFor(profile)
  │
  ▼
┌─ for p in LIST ─┐
│  Exec(p)        │
│   ├─ ok         │
│   └─ err → Debug│
└────────┬────────┘
         ▼
        END
```

## Profile selection logic

```
switch profile:
  BulkBuild → large write preset
  Query     → medium read-mostly preset
  ReadOnly  → no journal writes preset
  default   → Hot (includes ProfileHot and unknown ints)
```

**Design note:** using `default` for Hot means accidental `PragmaProfile(99)` still gets a sensible workstation preset rather than an empty list. That is intentional robustness, not a bug — but it means typos compile if cast from int.

## Interaction with connection pools

```
sql.Open  → pool exists, 0 live conns often
Apply*    → Exec forces a connection; PRAGMAs bind to THAT conn
later     → if MaxOpenConns > 1, new conns may not inherit all PRAGMAs
```

SQLite PRAGMAs are connection-scoped (with some journal_mode nuances that affect the DB file). codeNERD tests pin one connection. Production open sites typically use small pools; callers that raise pool size should re-apply or set driver-specific connection init hooks.

## Re-export layer architecture

```
caller_in_store_pkg:
  ApplyDefaultPragmas(db, ProfileHot)   // same package name

caller_via_store_import:
  store.ApplyDefaultPragmas(db, store.ProfileQuery)

caller_cannot_import_store:
  sqlpragmas.ApplyDefaultPragmas(db, sqlpragmas.ProfileHot)
```

`var ApplyDefaultPragmas = sqlpragmas.ApplyDefaultPragmas` means the store symbol is a **function variable**, not a wrapper — behavior is identical, including nil DB.

## Threading model

- No package-level mutex.
- Concurrent `ApplyDefaultPragmas` on the **same** `*sql.DB` is as safe as concurrent `Exec` (pool serializes per connection).
- Concurrent apply on **different** DBs is independent.

## Extension points (actual)

| Extension | How |
|-----------|-----|
| New profile | Add const + switch case + tests |
| New PRAGMA for all writable | Add to Hot/Bulk/Query lists carefully |
| Site-specific PRAGMA | Caller `db.Exec` after apply |

There is no plugin registry — by design.
