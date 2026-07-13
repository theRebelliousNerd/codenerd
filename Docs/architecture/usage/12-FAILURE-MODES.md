# usage — Failure Modes

> Last verified: **2026-07-13**

## Catalog

### F1 — Corrupt `usage.json` on disk

| | |
|--|--|
| **Symptom** | Fresh session starts at zero totals despite history |
| **Cause** | Manual edit, crash mid-write, partial JSON |
| **Detection** | File present but totals reset; Load would error if called alone |
| **Mitigation today** | NewTracker ignores Load error; continues empty |
| **Hardening** | Atomic write (temp + rename); keep `.bak`; log Load failure via `logging` |

### F2 — Debounce dirty race loses last Tracks

| | |
|--|--|
| **Symptom** | Last few Track calls missing after quiet period |
| **Cause** | Track between Save snapshot and `dirty=false` leaves new data with dirty still true, no new AfterFunc; or process exit before 5s timer |
| **Detection** | Stats in-memory > file on disk |
| **Mitigation today** | Explicit `Save()`; tests force dirty to skip timer |
| **Hardening** | Re-check dirty after Save under same lock; flush on Cortex.Close / chat shutdown; store and reset timer |

### F3 — Dual in-memory trackers overwrite each other

| | |
|--|--|
| **Symptom** | Totals bounce or lose one path’s events |
| **Cause** | Chat `NewTracker` and Cortex `NewTracker` both Load/Save same file without merging |
| **Detection** | Two components each hold Tracker; concurrent Saves |
| **Mitigation today** | Hope single-owner per process path |
| **Hardening** | Single factory; chat receives Cortex tracker; file lock or merge-on-save |

### F4 — Silent non-metering (nil context tracker)

| | |
|--|--|
| **Symptom** | LLM used tokens; usage.json unchanged |
| **Cause** | Caller forgot `NewContext`; or tracker nil from boot failure |
| **Detection** | ZAI structured logs show tokens but file flat |
| **Mitigation today** | Nil-safe no-op (correct for ambient design) |
| **Hardening** | Startup self-check; optional debug mode that warns once per process if Track skipped N times |

### F5 — Non-ZAI engines never Track

| | |
|--|--|
| **Symptom** | Multi-engine workspace undercounts systematically |
| **Cause** | Only `client_zai.go` implements Track |
| **Detection** | Provider tables missing xAI/Anthropic/etc. |
| **Mitigation today** | None in usage package |
| **Hardening** | Shared helper `usage.TrackFromLLMUsage(ctx, …)` called from all clients |

### F6 — Attribution all `"unknown"`

| | |
|--|--|
| **Symptom** | ByShardType/BySession useless |
| **Cause** | LLM call uses root ctx without `WithShardContext`; or string key collision overwritten |
| **Detection** | Stats maps dominated by `"unknown"` |
| **Mitigation today** | Spawn path sets keys; Track degrades safely |
| **Hardening** | Thread ctx carefully in stream/process; typed keys for attribution (breaking change) |

### F7 — MkdirAll failure

| | |
|--|--|
| **Symptom** | NewTracker returns error; no tracker |
| **Cause** | `.nerd` is a file; permission denied; read-only FS |
| **Detection** | Boot stderr warning; Usage page “not available” |
| **Mitigation today** | CLI/chat continue without metering |
| **Hardening** | Same as today — soft fail at higher layer |

### F8 — Disk full / WriteFile error

| | |
|--|--|
| **Symptom** | In-memory Stats grow; disk stale |
| **Cause** | Save returns error; Track’s AfterFunc ignores Save error |
| **Detection** | File mtime old; process still running |
| **Mitigation today** | Error discarded by AfterFunc |
| **Hardening** | Log Save errors; keep dirty true on failure; retry |

### F9 — Unbounded `BySession` growth

| | |
|--|--|
| **Symptom** | Large usage.json; memory growth |
| **Cause** | Every distinct session_id is a permanent map key |
| **Detection** | File size growth over months of unique sessions |
| **Mitigation today** | None |
| **Hardening** | Cap map size; rollup old sessions; optional prune API |

### F10 — Negative or absurd token counts

| | |
|--|--|
| **Symptom** | Totals decrease or explode |
| **Cause** | Provider bug; Add allows negatives (tested) |
| **Detection** | Nonsensical Stats |
| **Mitigation today** | None clamping |
| **Hardening** | Reject negative inputs in Track; max cap per call |

### F11 — Type assertion panic on FromContext

| | |
|--|--|
| **Symptom** | Process crash |
| **Cause** | Someone stores non-`*Tracker` under `contextKey{}` — **impossible from outside package** because key is unexported |
| **Mitigation today** | Unexported key; FromContext assumes type |
| **Note** | Only same-package misuse could break this |

## Failure × principle matrix

| Failure | Violates | Severity |
|---------|----------|----------|
| F2 dirty race | Durability | Medium |
| F3 dual tracker | Workspace single-writer | Medium |
| F4/F5 silent miss | Operator trust | High (data quality) |
| F8 ignored Save err | Soft observability | Medium |
| F1 corrupt | Soft-fail (by design) | Low for uptime / High for history |

## Recommended operator recovery

1. Backup `usage.json` if investigating.  
2. If corrupt: delete or restore backup; next boot starts clean.  
3. If undercount: fix wiring, do not expect retroactive fill.  
4. Before kill -9 on long jobs: ensure a code path calls `Save()` (request feature if missing on shutdown).
