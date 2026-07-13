# 09 — Safety and Invariants: browser

> Last verified against codebase: 2026-07-13

## 1. Constitutional surface

From `internal/core/defaults/policy/constitution.mg`:

| Action atom | Listed safe_action? |
|-------------|---------------------|
| `/browser_navigate` | Yes |
| `/browser_screenshot` | Yes |
| `/browser_read_dom` | Yes |
| `/browser_click` | **No** |
| `/browser_type` | **No** |
| `/browser_close` | **No** |

Default deny elsewhere still applies: if an action is not permitted, kernel must not schedule it. Package methods themselves do **not** call `permitted(...)` — safety is caller/kernel responsibility.

## 2. Honeypot as safety layer

Policy files derive traps from evidence:

| Intermediate | Signals |
|--------------|---------|
| `honeypot_css_hidden` | display none |
| `honeypot_css_invisible` | visibility hidden |
| `honeypot_opacity_hidden` | opacity 0 |
| `honeypot_offscreen` | x or y &lt; -1000 |
| `honeypot_zero_size` | w,h &lt; 2 |
| `honeypot_aria_hidden` | aria-hidden true |
| `honeypot_no_keyboard` | tabindex -1 |
| `honeypot_pointer_events_none` | pointerEvents none |
| `honeypot_suspicious_url` | **must be asserted from Go** (string match unavailable in Mangle) |

`browser.mg` also defines `honeypot_detected` via computed_style/geometry and `safe_interactable(ID) :- interactable(ID, _), !honeypot_detected(ID)`.

**Invariant (desired):** agents click only `safe_interactable` nodes.  
**Invariant (code today):** SessionManager.Click does not query that predicate.

## 3. Isolation invariants

1. **CreateSession uses Incognito** — reduces isolation per session context.  
2. **Fork clones state** — cookies + storage copied deliberately, not shared live.  
3. **SessionStore does not write cookies** — metadata only; reduced secret sprawl on disk.  
4. **Control URL in control.txt** — local workspace file; treat as sensitive (CDP is powerful).

## 4. Concurrency invariants

1. All map access through `SessionManager.mu` (RLock for reads, Lock for mutations).  
2. Event throttler is concurrent-safe.  
3. Concurrent List/GetSession covered by tests.  
4. Event stream must tolerate engine `AddFacts` errors (log, continue).  
5. `Start` holds write lock across connect — callers should not nest Start under other locks.

## 5. Resource invariants

1. `Shutdown` closes all tracked pages then browser.  
2. Integration tests defer Shutdown.  
3. CLI launch waits for signal before shutdown.  
4. CLI session deliberately **does not** shutdown (documented operator flow) — risk of orphan Chrome if launch was process-local.

## 6. Fact integrity invariants

1. `react_state` hook indices coerced to int64 (avoid float64 /number mismatch).  
2. Dual CSS encodings keep policy matches across string vs atom worlds.  
3. DOM max 200 nodes — incomplete but finite.  
4. Nil engine disables streams and errors ReifyReact — fail closed for reification, not for raw Navigate.

## 7. Content safety (out of package)

No URL allowlist / SSRF guard inside SessionManager. Any reachable URL the process can access is navigable. Higher layers (policy, tool schema, operator) must constrain targets if required.

## 8. Logging of sensitive data

Navigate logs full URLs; Type logs text **length** not content (good). Screenshots return raw bytes to caller — tools may base64 them. Avoid logging cookie dumps (Fork does not log cookie values).
