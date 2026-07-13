# sqlpragmas — Failure Modes

> Last verified: **2026-07-13**  
> Concrete failures, symptoms, and mitigations.

---

## FM1 — Driver rejects a PRAGMA

| | |
|--|--|
| **Cause** | modernc.org/sqlite (or older SQLite) rejects a statement mattn accepts |
| **Symptom** | Debug log `pragma "…" failed: …`; other pragmas still applied |
| **User impact** | Often none; worst case missing WAL/cache tuning |
| **Mitigation** | Best-effort design; fix list only if functional regression; dual-driver tests |
| **Detection** | Enable store Debug; compare `PRAGMA` read-backs |

---

## FM2 — Read-only connection + writable profile

| | |
|--|--|
| **Cause** | Caller opens `?mode=ro` but applies Hot/Query/Bulk |
| **Symptom** | Multiple Debug failures for journal_mode / synchronous / checkpoint |
| **User impact** | Log noise; RO semantics still hold |
| **Mitigation** | Use `ProfileReadOnly`; tests guard ReadOnly list |
| **Example correct path** | `internal/core/predicate_corpus.go` |

---

## FM3 — mmap request silently capped

| | |
|--|--|
| **Cause** | OS/SQLite caps mmap below 8/16 GiB (common on Windows) |
| **Symptom** | `PRAGMA mmap_size` returns smaller positive value |
| **User impact** | Less memory-mapping than documented; still better than 0 |
| **Mitigation** | Tests assert `> 0` only; do not treat exact GiB as invariant |

---

## FM4 — Multi-connection pool without re-apply

| | |
|--|--|
| **Cause** | `SetMaxOpenConns(N>1)` after apply; new connections inherit incomplete pragma state |
| **Symptom** | Intermittent slow queries or lock behavior depending on which conn is used |
| **User impact** | Hard-to-reproduce performance bugs |
| **Mitigation** | Keep pools small; re-apply; driver connection hook (future) |
| **Tests** | Package tests force MaxOpen=1 to avoid false confidence |

---

## FM5 — Open site forgets ApplyDefaultPragmas

| | |
|--|--|
| **Cause** | New code path opens SQLite without helper |
| **Symptom** | Default journal (often delete), small cache, busy_timeout 0 |
| **User impact** | `SQLITE_BUSY`, write stalls under concurrent agent+tool |
| **Mitigation** | Wiring audit (`rg sql.Open`); code review checklist |
| **Not mitigated by** | This package itself (cannot intercept Open) |

---

## FM6 — Host with low RAM + Hot/Bulk sizes

| | |
|--|--|
| **Cause** | 2–4 GiB cache requests on small laptop |
| **Symptom** | Memory pressure; OS paging; SQLite may not allocate full cache |
| **User impact** | System-wide slowdown rare but possible |
| **Mitigation** | SQLite treats sizes as upper bounds; future config profile for modest hosts |

---

## FM7 — Enabling foreign_keys in shared defaults (hypothetical change)

| | |
|--|--|
| **Cause** | Well-intentioned PR adds `PRAGMA foreign_keys=ON` to presets |
| **Symptom** | Inserts/updates fail on existing DBs with orphan rows |
| **User impact** | Broken prompt/strategy/northstar writes for some users |
| **Mitigation** | Principle P6; coordinated migration; opt-in helper |

---

## FM8 — Idempotent re-open thrash (non-failure)

| | |
|--|--|
| **Cause** | Autopoiesis / prompt paths re-open and re-apply |
| **Symptom** | Extra Execs; no drift (Hot tested) |
| **User impact** | Negligible cost |
| **Mitigation** | Idempotency test; leave as-is |

---

## FM9 — Unknown profile int cast to Hot

| | |
|--|--|
| **Cause** | `PragmaProfile(42)` from bad cast |
| **Symptom** | Receives Hot preset, not empty |
| **User impact** | Usually fine; may be heavier than intended |
| **Mitigation** | Prefer named consts; optional future log on unknown |

---

## FM10 — CGO tests fail to compile

| | |
|--|--|
| **Cause** | Environment lacks C toolchain / sqlite for mattn |
| **Symptom** | `go test ./internal/sqlpragmas` build error |
| **User impact** | Dev cannot verify locally |
| **Mitigation** | Install CGO toolchain; use project sqlite headers pattern from root `AGENTS.md` |

---

## Failure mode summary matrix

| ID | Severity | Likelihood | Package mitigates? |
|----|----------|------------|--------------------|
| FM1 | Low–Med | Med (modernc) | Yes (soft fail) |
| FM2 | Low | Low if disciplined | Yes (profile) |
| FM3 | Low | High on Win | Yes (design) |
| FM4 | Med | Med | Partial (docs) |
| FM5 | Med–High | Med | No (process) |
| FM6 | Low | Low on target hosts | Partial |
| FM7 | High if done | Controllable | Policy |
| FM8 | None | High | Yes |
| FM9 | Low | Low | Soft default |
| FM10 | Dev-only | Env-dep | N/A |
