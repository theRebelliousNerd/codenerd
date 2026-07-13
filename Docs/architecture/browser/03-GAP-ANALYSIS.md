# 03 — Gap Analysis: browser

> Last verified against codebase: 2026-07-13  
> Vision: [01-VISION.md](01-VISION.md) · Reality: [02-CURRENT-STATE.md](02-CURRENT-STATE.md)

## 1. Spec vs reality matrix

| Vision item | Reality | Status |
|-------------|---------|--------|
| Rod session lifecycle | `SessionManager.Start/Create/Attach/Shutdown` | **Done** |
| DOM → Mangle facts | `captureDOMFacts` / `SnapshotDOM` | **Done** |
| React Fiber reification | `ReifyReact` | **Done** (best-effort; skips if no fiber key) |
| CDP event stream | nav, console, net, DOM update, click/input/state poll | **Done** |
| Honeypot via Mangle | `HoneypotDetector` + `browser_honeypot.mg` | **Done** (suspicious URL only if Go asserts intermediate fact) |
| Spatial policy | `browser.mg` left_of/above, constrained to interactable | **Done** |
| CLI operator surface | launch / session / snapshot | **Partial** — no click/type/screenshot/list/fork cobra verbs |
| Single Cortex-owned manager | Boot leaves nil; research uses separate singleton with nil engine | **Gap** |
| VS executes browser actions | `handleBrowse` returns hard failure requiring shard | **Partial / intentional stub** |
| Modular tools execute browser | `internal/tools/research/browser.go` works but **no EngineSink** | **Partial** |
| Tactile router has manager | Field + setter + routes; boot only wires if non-nil | **Wiring gap** (always nil at boot) |
| Constitution gates | navigate/screenshot/read_dom safe; click/type not listed | **Done** (policy) |
| Fact sink = production kernel | CLI builds throwaway `mangle.NewEngine`; research uses nil | **Gap** for agent-visible world model |
| Session reattach | Load store as `detached`; CLI snapshot re-Attach by TargetID | **Partial** — TargetID may be stale after Chrome restart |
| Header ingestion | Config flag exists; default `EnableHeaderIngestion` false in DefaultConfig | **Optional off** |
| `honeypot_suspicious_url` generation | Policy comment: assert from Go; no URL pattern analyzer found in package | **Gap** |
| Clip / overflow honeypot reasons in Go | Reason table includes clip/overflow; policy rules incomplete vs Go list | **Partial mismatch** |
| Multi-engine concurrent Chrome | One browser per manager; multiple managers possible | **Risk** |

## 2. Priority ranking

### P0 — Correctness / safety wiring

1. **Own a Cortex-scoped SessionManager** with engine sink = live kernel (or kernel-facing adapter), construct on first browser action, inject into tactile router + modular tools.  
2. **Close execution path**: either implement VS handleBrowse via shared manager, or guarantee modular tool + router always share that manager.  
3. Document that **research tools currently reify nothing** (nil engine) — agents cannot see DOM facts from tool path alone.

### P1 — Policy completeness

4. Implement Go-side `honeypot_suspicious_url` assertion or remove from reason table / policy.  
5. Align Go honeypot reason checks (`honeypot_clip_hidden`, `honeypot_overflow_hidden`, `honeypot_no_keyboard`) with actual `.mg` rules (clip/overflow not fully derived in policy).  
6. Ensure `safe_interactable` is consulted before agent click/type (currently not enforced inside SessionManager — policy is advisory unless caller checks).

### P2 — Operator / product surface

7. Cobra verbs: list, attach, click, type, screenshot, fork, honeypot-analyze.  
8. TUI slash commands / status page for browser sessions.  
9. On-demand Start from chat when intent routes to browser_*.

### P3 — Hardening

10. Session store encryption / redaction of URLs with secrets.  
11. Bounded fact retention / GC for long-lived event streams.  
12. Explicit cancel of event-stream goroutines on session close (today Shutdown closes pages; stream exit relies on ctx / page death).

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

- Agents call `browser_navigate` via modular tools, get text status only, **hallucinate page content**.  
- HoneypotDetector unused in tool path → clicks trap links.  
- Multiple Chrome processes if CLI + research + future chat each Start independently.  
- Policy tests pass while production path never loads page facts into the same engine that evaluates rules.
