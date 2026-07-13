# sqlpragmas — Alignment / Vision Review

> Last verified: **2026-07-13**  
> Scored against codeNERD north star with **source evidence** from `internal/sqlpragmas/`.

## Scoring rubric

| Score | Meaning |
|------:|---------|
| 5 | Fully aligned; evidence in code |
| 4 | Strong; minor gaps |
| 3 | Partial / indirect |
| 2 | Weak or accidental |
| 1 | Misaligned or absent where needed |
| n/a | Dimension does not apply |

---

## Dimensions

### 1. LLM as creative center — **n/a (5 as “correctly absent”)**

sqlpragmas has zero LLM surface: no prompts, no atoms, no model calls. Infrastructure stays out of the creative path. **Evidence:** package imports only `database/sql`, `fmt`, `internal/logging` (`pragmas.go`).

### 2. Logic / Mangle as executive — **n/a (5 as “correctly absent”)**

No Mangle Decl, no `permitted`, no fact derivation. Pragma selection is a Go enum, not a policy rule. Appropriate: executive control over *actions* is kernel/policy; open-time SQLite tuning is deterministic infrastructure.

### 3. Constitutional safety (default deny) — **3 / 5 (indirect)**

Does not participate in action permissioning. Safety properties that *do* apply:

| Property | Evidence |
|----------|----------|
| Never fail open on pragma | Debug-log, continue (`ApplyDefaultPragmas`) |
| ReadOnly avoids write pragmas | `ProfileReadOnly` omits WAL/sync/checkpoint |
| FK not silently enabled | Comment + omission in `pragmasFor` |

Gaps: no audit fact that a DB opened with profile X; no policy gate on bulk mmap sizes.

### 4. JIT prompt atoms — **n/a**

Not LLM-facing. Correct non-participation.

### 5. Inversion of control / separation of roles — **5 / 5**

Package does one job: apply PRAGMAs. It does not open DBs, migrate schemas, or register stores. Callers own open; this leaf owns tuning. **Evidence:** function contract comments lines 53–59.

### 6. Wiring integrity / cycle hygiene — **5 / 5**

Primary *raison d’être*. **Evidence:** package comment lines 13–15; `internal/store/pragmas.go` re-export for non-cyclic call sites; `internal/mcp/store.go` imports sqlpragmas directly.

### 7. Determinism & reproducibility — **4 / 5**

Given profile P, pragma list is pure (`pragmasFor`). Observable DB state after apply is **platform-capped** (mmap), so exact mmap is not bit-identical cross-OS; cache/journal/sync/busy are tested exact on mattn. Integration tests lock the matrix.

### 8. Observability — **3 / 5**

Failures visible at Debug under `CategoryStore`. Success silent. No structured fields (profile name, DSN fingerprint). Adequate for spam-free opens; weak for “why is this DB slow” forensics.

### 9. Test alignment — **4 / 5**

Strong unit + integration coverage of all four profiles, nil safety, idempotency. Weakness: single driver (mattn); no pure-Go modernc test path in-package.

### 10. Scope discipline (leaf purity) — **5 / 5**

No config, no features flags, no store types. Hardcoded numbers are a *product* gap (see gap analysis) but preserve leaf purity.

### 11. Long-horizon / campaign durability — **4 / 5**

WAL + NORMAL + busy_timeout + generous cache/mmap support long sessions and bulk corpus builds that campaigns/assaults rely on. No campaign-specific profile (Hot/Bulk cover the needs).

### 12. Operator clarity — **3 / 5**

API is small and well-commented. Dual entry (`sqlpragmas` vs `store`) reduces discoverability for new contributors.

---

## Summary scorecard

| Dimension | Score |
|-----------|------:|
| LLM creative center | n/a ✓ |
| Mangle executive | n/a ✓ |
| Constitutional safety | 3 |
| JIT prompts | n/a ✓ |
| Separation of roles | 5 |
| Wiring / cycles | 5 |
| Determinism | 4 |
| Observability | 3 |
| Testing | 4 |
| Leaf purity | 5 |
| Durability support | 4 |
| Operator clarity | 3 |

**Weighted judgment:** Highly aligned as **infrastructure leaf**. Main improvement surface is operational (config, dual-driver CI, import ergonomics), not north-star philosophy.

## North-star fit statement

> codeNERD separates creativity (LLM) from executive control (Mangle). sqlpragmas sits under both: it makes the **durable substrate** that memory and tools depend on open with consistent performance and concurrency posture, without ever claiming policy authority.
