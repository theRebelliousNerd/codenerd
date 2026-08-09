# 09 — Safety and Invariants: browser

> Last verified against codebase: 2026-08-09

## 1. Constitutional surface

From `internal/core/defaults/policy/constitution.mg`:

| Action atom | Listed safe_action? |
|-------------|---------------------|
| `/browser_navigate` | Yes |
| `/browser_screenshot` | Yes |
| `/browser_read_dom` | Yes |
| `/browser_extract` / `/browser_click` / `/browser_type` / `/browser_close` | Yes |
| `/browser_observe` / `/browser_act` | Yes |
| `/browser_mangle` / `/browser_wait` / `/browser_reason` | Yes |

`safe_action` is not authority. The effective JIT allowlist first constrains availability, then the session executor asserts an exact `pending_action` and requires matching `permitted(Action, Target, Payload)`. Package methods themselves do **not** call `permitted(...)` — direct package callers are trusted integration code.

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

1. **CreateSession is shared by default** — preserves one browser profile; explicit isolation is available.
2. **Fork clones state into isolation** — cookies + storage copied deliberately, not shared live.
3. **SessionStore does not write cookies** — redacted metadata only, owner-only.
4. **Control URL is sensitive** — `control.txt` is owner-only.

## 4. Concurrency invariants

1. All map access through `SessionManager.mu` (RLock for reads, Lock for mutations).  
2. Event throttler is concurrent-safe.  
3. Concurrent List/GetSession covered by tests.  
4. Event stream must tolerate engine `AddFacts` errors (log, continue).  
5. `startMu` serializes lifecycle setup without holding the state write lock across Chrome connect.
6. Session streams use manager-owned contexts and must be canceled by tab/browser/shutdown close.
7. Element refs are opaque and session-scoped; navigation clears the registry generation.
8. An observation registers fingerprints only if its capture generation is still current; racing navigation fails the observation closed.

## 5. Resource invariants

1. `Shutdown` cancels streams, closes tracked pages/isolated contexts, then browsers.
2. Integration tests defer Shutdown.  
3. CLI launch waits for signal before shutdown.  
4. CLI session deliberately **does not** shutdown (documented operator flow) — risk of orphan Chrome if launch was process-local.

## 6. Fact integrity invariants

1. `react_state` hook indices coerced to int64 (avoid float64 /number mismatch).  
2. Session-manager CSS/attribute facts use their declared string shapes; atom-shaped duplicate assertions are rejected rather than creating mixed identity worlds.
3. Event and derived reasoning facts carry SessionID; DOM node IDs are session-qualified.
4. DOM max 200 nodes and page hook buffer max 200 events — incomplete but finite.
5. Nil sink keeps only the navigation lifecycle stream and errors ReifyReact — production Cortex boot must never use it.
6. Every fact batch is copy-on-write redacted before its configured sink.

## 7. Reasoning/query invariants

1. `browser_mangle` is read-only: no fact mutation or caller-supplied rule submission.
2. Queries must parse as one atom, select an allowed browser predicate, and include the requested SessionID.
3. Query scans, returned facts, condition count, polling, and timeout are capped.
4. `browser_wait` is fresh-only by default; callers use `browser_act.started_ms` as the action watermark.
5. `browser_reason` refreshes page state, scopes current-route evidence by default, and re-redacts public fact arguments.

## 8. Content safety (out of package)

No URL allowlist / SSRF guard inside SessionManager. Any reachable URL the process can access is navigable. Higher layers (policy, tool schema, operator) must constrain targets if required.

## 9. Sensitive evidence

Browser URLs, headers, input/DOM/React values, console text, metadata, and text tool results pass through the redactor. Type logs text **length**, not content. Model-directed artifact paths must pass the symlink-aware writable-root policy; persisted files use owner-only permissions. Screenshots still contain page pixels and must be treated as sensitive evidence by callers.
