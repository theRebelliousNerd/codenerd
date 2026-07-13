# sqlpragmas — Gap Analysis

> Last verified: **2026-07-13**  
> Spec/vision vs reality for `internal/sqlpragmas/`.

## Method

Compare:

1. Package comments / intended contract in `pragmas.go`
2. Vision in [01-VISION.md](01-VISION.md)
3. Observed call sites and tests

Against what production still lacks.

---

## Spec vs reality matrix

| Intent | Reality | Gap? |
|--------|---------|------|
| Leaf package, no store cycle | Imports only sql/fmt/logging; mcp imports leaf | **Non-gap** |
| Four workload profiles | Implemented + tested | **Non-gap** |
| Best-effort apply, never fail open | Implemented | **Non-gap** |
| ReadOnly skips write pragmas | Implemented + tested | **Non-gap** |
| FK not forced on | Omitted with comment | **Non-gap** (policy choice) |
| Workstation-class defaults | Hardcoded 2–16 GiB class sizes | **Partial** — no laptop profile |
| Both drivers coexist | Design assumes yes; tests only mattn | **Gap** (test evidence) |
| Apply right after Open | Most call sites do; not enforced | **Soft gap** |
| Idempotent re-apply | Tested for Hot | **Non-gap** for Hot; other profiles untested for idempotency |
| Single discoverable API | Dual surface sqlpragmas + store | **Ergonomics gap** |
| Config override | Absent | **Gap** (product wish) |
| Multi-conn pool safety | Documented in tests; no Conn hook helper | **Gap** |
| Observability of profile choice | Silent success | **Gap** (ops) |
| 100% Open-site coverage | High but not formally audited every PR | **Process gap** |

---

## Prioritized gaps

### P1 — High blast radius if wrong (process, not missing code)

| Gap | Why it matters | Suggested direction |
|-----|----------------|---------------------|
| Default size change without fan-out review | Every agent DB open affected | Treat profile table as API; changelog + store/mcp smoke |
| Open sites that skip ApplyDefaultPragmas | Inconsistent performance / locking | Periodic `rg sql.Open` audit |

### P2 — Real product gaps

| Gap | Evidence | Suggested direction |
|-----|----------|---------------------|
| No modernc.org/sqlite test | Tests import mattn only | Build-tag integration test or CI matrix job |
| Multi-connection pool | PRAGMA is per-connection; `SetMaxOpenConns>1` without re-apply | Document caller responsibility; optional `sql.DB` setup hook later |
| Split import paths | `store.Apply…` vs `sqlpragmas.Apply…` | Architecture note + prefer leaf in new mid-layer code |

### P3 — Nice-to-have

| Gap | Notes |
|-----|-------|
| Config / env size overrides | Hosts with &lt;16 GB RAM may not need 4 GB cache |
| `EnableForeignKeys` helper | Reduces copy-paste when a schema is ready |
| Structured Debug (profile enum name) | Easier log grepping |
| Idempotency tests for Bulk/Query/ReadOnly | Symmetric to Hot |
| Exact mmap assert on Linux CI | Windows soft-assert remains |

---

## Non-gaps (do not “fix” incorrectly)

| Item | Why it is not a gap |
|------|---------------------|
| No Mangle surface | Correct for leaf infra |
| No error return from Apply | Intentional for driver coexistence |
| FK off by default | Explicit data-compat decision |
| No LLM / prompt atoms | Correct non-participation |
| Small LOC | Completeness is fan-out, not size |
| mmap not exact on Windows | Documented SQLite/OS cap; tests correct |

---

## Gap vs north star

| North-star concern | Gap level |
|--------------------|-----------|
| Creative/executive split | None — package correctly outside both |
| Safety default-deny for *actions* | N/A |
| Wiring before delete | **Do not delete** — reverse deps everywhere |
| JIT-first LLM behavior | N/A |

---

## Recommended near-term backlog (docs → optional code)

1. Keep this corpus current when profiles change.  
2. Add modernc test build if pure-Go opens increase.  
3. One-time audit: `sql.Open` without `ApplyDefaultPragmas` in `internal/` and `cmd/`.  
4. Defer config until a real host-class failure is reported.

See [TODO.md](TODO.md).
