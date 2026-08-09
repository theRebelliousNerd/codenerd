# 03 — Gap Analysis: browser

> Last verified against codebase: 2026-08-09
> Vision: [01-VISION.md](01-VISION.md) · Reality: [02-CURRENT-STATE.md](02-CURRENT-STATE.md)

## 1. Spec vs reality matrix

| Vision item | Reality | Status |
|-------------|---------|--------|
| Rod session lifecycle | `SessionManager.Start/Create/Attach/Shutdown` | **Done** |
| DOM → Mangle facts | `captureDOMFacts` / `SnapshotDOM` | **Done** |
| React Fiber reification | `ReifyReact` | **Done** (best-effort; skips if no fiber key) |
| CDP event stream | session-scoped nav, console, request/response/failure, DOM update, toast, click/input/state poll | **Done** |
| Honeypot via Mangle | `HoneypotDetector` + `browser_honeypot.mg` | **Done** (suspicious URL only if Go asserts intermediate fact) |
| Spatial policy | `browser.mg` left_of/above, constrained to interactable | **Done** |
| CLI operator surface | launch / session / snapshot | **Partial** — no click/type/screenshot/list/fork cobra verbs |
| Single Cortex-owned manager | System factory constructs it and binds tactile + modular research tools; lazy fallback remains for narrow standalone use | **Done in system boot** |
| VS executes browser actions | `handleBrowse` returns hard failure requiring shard | **Partial / intentional stub** |
| Modular tools execute browser | Legacy six plus observe/act/mangle/wait/reason/evidence/specs/test resolve the Cortex-owned manager and live kernel | **Done through BPAR-4** |
| Tactile router has manager | System factory injects its manager; legacy chat path remains conditional | **Partial** |
| Constitution gates | Effective allowlist plus exact pending payload permission for every registered browser spelling | **Done** (policy + tests) |
| Fact sink = production kernel | System factory adapts browser facts into live `SystemKernel.AssertBatch`; CLI keeps an export-only engine | **Done in Cortex; CLI intentionally separate** |
| Session reattach | Load store as `detached`; CLI snapshot re-Attach by TargetID | **Partial** — TargetID may be stale after Chrome restart |
| Header ingestion | Config flag exists; default `EnableHeaderIngestion` false in DefaultConfig | **Optional off** |
| `honeypot_suspicious_url` generation | Policy comment: assert from Go; no URL pattern analyzer found in package | **Gap** |
| Clip / overflow honeypot reasons in Go | Reason table includes clip/overflow; policy rules incomplete vs Go list | **Partial mismatch** |
| Multi-browser lifecycle | One manager can bound, list, select, promote, and close multiple browser processes; progressive act exposes lifecycle operations | **Package/agent done; CLI expansion pending** |
| Progressive observation/action | Bounded modes, opaque generation refs, closed sequential operations, JIT atoms | **Done (BPAR-2)** |
| Live-kernel reasoning/waits | Read-only bounded queries, action watermarks, fresh fact/condition/stability waits, derived diagnosis, session isolation | **Done (BPAR-3)** |
| Bounded browser flight evidence | Redacted per-session JSONL, hard read/export/scan ceilings, rotation/pruning, confined current-user-only export, live route proof | **Done (BPAR-4 evidence slice)** |
| Workspace browser specs | Confined named Markdown corpora, bounded indexes/scans/parsing/ranking, native/compatible invariants, same-kernel live checks and observe/act context | **Done (BPAR-4 spec slice)** |
| Declarative browser tests | Strict portable YAML/JSON, semantic targets, execution-only `value_env`, recorder-backed generation, fresh-baseline assertions, same-kernel replay/diagnosis | **Done (BPAR-4 test slice)** |

## 2. Priority ranking

### P0 — Correctness / safety wiring

1. **Complete BPAR-5 audit/repository-trace/Docker-correlation and final evaluation** without creating a second fact store or authority root.
2. **Close execution path**: either implement VS handleBrowse via the shared manager, or keep the modular-tool + router guarantee explicit.
3. Add bounded fact retention/GC so long-lived sessions do not retain event history forever.

### P1 — Policy completeness

4. Implement Go-side `honeypot_suspicious_url` assertion or remove from reason table / policy.  
5. Align Go honeypot reason checks (`honeypot_clip_hidden`, `honeypot_overflow_hidden`, `honeypot_no_keyboard`) with actual `.mg` rules (clip/overflow not fully derived in policy).  
6. Ensure `safe_interactable` is consulted before agent click/type (currently not enforced inside SessionManager — policy is advisory unless caller checks).

### P2 — Operator / product surface

7. Cobra verbs: list, attach, click, type, screenshot, fork, honeypot-analyze.  
8. TUI slash commands / status page for browser sessions.  
9. On-demand Start from chat when intent routes to browser_*.

### P3 — Hardening

10. Consider session-store encryption beyond the shipped redaction/private-file baseline.
11. Bounded fact retention / epoch cleanup for long-lived event streams.

The complete pinned uplift contract is [BROWSERNERD-PARITY.md](BROWSERNERD-PARITY.md).

## 3. Non-gaps (do not “fix”)

| Observation | Why not a gap |
|-------------|----------------|
| No package-local `.mg` | Schemas/policy correctly live in core defaults and load via kernel_init |
| Default Headless=false | Operator-visible default; headless set in tests and can be configured |
| maxNodes = 200 | Intentional budget against fact explosion |
| Event throttle default 100ms | Intentional |
| Chat boot does not spawn Chrome | Explicit design comment in `session_boot.go` — avoids TUI cost |
| VirtualStore refuses direct browse | Intentional sandboxing comment — must not become silent no-op without alternative |

## 4. Risk if ignored

- Observe/act/reason/evidence/specs/test are bounded and live; audit/repository-trace/Docker views remain absent, so final delivery evidence is still incomplete.
- HoneypotDetector unused in tool path → clicks trap links.  
- CLI remains a separate operator process; progressive tools need explicit multi-browser selection to avoid accidental process growth.
- `browser_mangle` deliberately cannot submit rules or mutate facts; a future rule sandbox needs an explicit authority and resource contract.
