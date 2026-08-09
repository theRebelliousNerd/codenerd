# 12 — Failure Modes: browser

> Last verified against codebase: 2026-08-09

## FM-01 — Chrome not installed / launcher fails

| | |
|--|--|
| **Symptom** | `Start` error: launch/connect failure; lifecycle tests Skip |
| **Cause** | No Chrome/Chromium; sandbox block; bad `Launch[0]` path |
| **Mitigation** | Install Chrome; set `Config.Launch`; use remote `DebuggerURL`; tests Skip |

## FM-02 — Stale control.txt / dead WebSocket

| | |
|--|--|
| **Symptom** | CLI session/snapshot cannot connect; Start fails with connect error |
| **Cause** | Launch process died; control file left behind |
| **Mitigation** | Relaunch; delete `.nerd/browser/control.txt`; Start health check closes stale browser and reconnects when manager still in-process |

## FM-03 — Detached session, dead TargetID

| | |
|--|--|
| **Symptom** | Snapshot reattach fails after Chrome restart |
| **Cause** | SessionStore has old TargetID; Chrome targets ephemeral |
| **Mitigation** | Create new session; treat store as soft cache; document TargetID non-durable across process |

## FM-04 — Nil EngineSink / engine

| | |
|--|--|
| **Symptom** | No DOM/console/network facts; ReifyReact errors “mangle engine not configured” |
| **Cause** | Focused/standalone caller passed nil sink; system binding was bypassed |
| **Mitigation** | System boot installs `browserKernelSink` and binds research tools; navigation-only tracking still maintains metadata/ref invalidation, but a sink is required for world-model updates |

## FM-05 — Fact type rejection in Mangle

| | |
|--|--|
| **Symptom** | AddFacts errors in stream logs; incomplete state |
| **Cause** | Arg type mismatch vs Decl (e.g. float hook index before coercion) |
| **Mitigation** | Keep int64 coercion and exact Decl shapes; do not emit atom-shaped duplicates for string predicates; schema-test new fields and exercise them against `RealKernel` |

## FM-06 — Element not found (Click/Type)

| | |
|--|--|
| **Symptom** | `element not found` error |
| **Cause** | Selector mismatch; page not loaded; SPA not hydrated; honeypot hidden but selector matches wrong node |
| **Mitigation** | SnapshotDOM first; wait/retry at caller; prefer stable selectors |

## FM-07 — Navigation timeout

| | |
|--|--|
| **Symptom** | Navigate error after NavigationTimeoutMs |
| **Cause** | Slow site, network, hung main frame |
| **Mitigation** | Raise timeout; check network facts if stream live; headful debug |

## FM-08 — Fact storm / memory growth

| | |
|--|--|
| **Symptom** | High CPU/memory; huge engines |
| **Cause** | SPA DOM thrash + low throttle + header ingestion |
| **Mitigation** | EventThrottleMs; minimal logging level; disable DOM stream; maxNodes already 200 |

## FM-09 — Honeypot false negative

| | |
|--|--|
| **Symptom** | Agent clicks trap; IsHoneypot false |
| **Cause** | No page facts emitted; engine without rules; suspicious URL not asserted; style via class not computed yet |
| **Mitigation** | Always emitPageFacts/SnapshotDOM before analyze; load policy; implement URL classifier |

## FM-10 — Honeypot false positive

| | |
|--|--|
| **Symptom** | Safe links empty; UI “invisible” but real |
| **Cause** | Aggressive zero-size/offscreen thresholds; transient layout |
| **Mitigation** | high_confidence_honeypot for multi-signal; re-snapshot after layout |

## FM-11 — Orphan Chrome processes

| | |
|--|--|
| **Symptom** | Zombie chromes after CLI session without launch lifecycle |
| **Cause** | `browser session` leaves browser up; process killed without Shutdown |
| **Mitigation** | Prefer `launch` long-running parent; use `CloseSession`/`CloseBrowser`; retain signal shutdown |

## FM-12 — Standalone CLI and Cortex manager diverge

| | |
|--|--|
| **Symptom** | Tool session IDs unknown to CLI store; facts not in kernel |
| **Cause** | CLI is a separate process/export engine; its session IDs are not Cortex identity |
| **Mitigation** | System routes share one Cortex-owned manager; keep CLI separation explicit in UX |

## FM-13 — React reification empty

| | |
|--|--|
| **Symptom** | ReifyReact returns empty or error; CLI logs “skipped” |
| **Cause** | Non-React page; production build without fiber debug; wrong root |
| **Mitigation** | Optional path; rely on DOM facts; do not treat as hard failure |

## FM-14 — Concurrent Start/Shutdown races

| | |
|--|--|
| **Symptom** | Rare panics or connect flaps |
| **Cause** | Lifecycle calls overlap while Chrome is connecting or closing |
| **Mitigation** | `startMu` serializes Start/LaunchAdditional/Shutdown; state lock protects maps |

## FM-15 — VS browse always fails

| | |
|--|--|
| **Symptom** | Action browse → error string requiring TactileRouterShard |
| **Cause** | Intentional handleBrowse stub |
| **Mitigation** | Use modular tools or wire shard manager; do not treat as package bug alone |

## FM-16 — Stale progressive element ref

| | |
|--|--|
| **Symptom** | `browser_act` refuses a previously returned `e<generation>_<n>` ref |
| **Cause** | Navigation advanced the session registry generation, or the page changed enough that bounded fingerprint re-identification failed |
| **Mitigation** | Run `browser_observe` again and act on a current ref; never fall back to a model-invented selector |

## FM-17 — Observation crosses navigation

| | |
|--|--|
| **Symptom** | Observe returns a stale-snapshot/navigation-changed error instead of elements |
| **Cause** | URL or registry generation changed while the page snapshot was being captured/materialized |
| **Mitigation** | Retry observe after navigation settles; generation-checked registration prevents stale DOM from repopulating refs |

## FM-18 — Wait succeeds from old evidence

| | |
|--|--|
| **Symptom** | A condition appears satisfied before the new action produces an event |
| **Cause** | Caller disabled fresh-only or omitted the action watermark while event facts accumulate |
| **Mitigation** | Pass `browser_act.started_ms` as `since_ms`; keep fresh-only enabled; use current-session predicates |

## FM-19 — Cross-session diagnosis leakage

| | |
|--|--|
| **Symptom** | One tab's console/network error appears in another tab's diagnosis |
| **Cause** | New event/rule omitted SessionID or a caller attempted a non-scoped query |
| **Mitigation** | Keep event and derived schemas session-scoped; `browser_mangle` enforces SessionID; retain live foreign-session exclusion coverage |

## FM-20 — Browser query rejected

| | |
|--|--|
| **Symptom** | `browser_mangle` rejects a rule, mutation, general predicate, or oversized request |
| **Cause** | Deliberate read-only allowlist and hard ceilings at the model/kernel boundary |
| **Mitigation** | Query one allowed browser predicate for one session; use shipped derived predicates; design a separately authorized rule sandbox if product requirements demand mutation |

## FM-21 — Evidence disabled, truncated, or rejected

| | |
|--|--|
| **Symptom** | `browser_evidence` reports disabled, a truncated bounded result, or refuses an export path |
| **Cause** | Workspace-less/disabled recorder, row/read/scan/file ceilings, an existing export target, or writable-root traversal/symlink confinement |
| **Mitigation** | Configure the native browser evidence block; narrow by session/type/time; choose a new export path below configured writable roots; never bypass the privacy policy |
