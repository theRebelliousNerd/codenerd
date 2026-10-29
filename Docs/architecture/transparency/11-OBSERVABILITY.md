# transparency — Observability

> Last verified: 2026-07-13  
> This package **is** an observability surface. It is complementary to `internal/logging`.

## 1. Two observability planes

| Plane | Package | Audience | Persistence |
|-------|---------|----------|-------------|
| Logging | `internal/logging` | Developers, log files | Files / sinks |
| Transparency | `internal/transparency` | Operators in TUI / status | Ephemeral rings + chat scrollback |

Do not replace structured logs with Glass Box events, or vice versa. Prefer:

- Logs for forensic timelines  
- Glass Box / tools for live session understanding  
- Explainer for logic “why”  

## 2. Glass Box categories as taxonomy

| Category | Typical producers | Example summary |
|----------|-------------------|-----------------|
| perception | perception pipeline (when wired) | intent parse result |
| kernel | kernel adapters (when wired) | next_action, denies |
| jit | prompt/JIT (when wired) | atom selection stats |
| shard | ShardManager | spawn / complete |
| control | chat control path | working / control packets |
| routing | VirtualStore | `exec_cmd path` + duration |

Display prefix: `[PERCEPTION]`, `[KERNEL]`, … via `DisplayPrefix()`.

## 3. Live activity pulse (consumer)

Chat `handleGlassBoxEvent` updates:

- `activityLine` / `activityAt` / category icon  
- short activity trail (dedup consecutive summaries)  
- optional permanent scrollback when glass box enabled  

Drain batching (`maxGlassBoxDrain = 64`) keeps Bubble Tea frames responsive under storms.

## 4. Tool telemetry (always-on)

| Field | Use |
|-------|-----|
| ToolName | Badge / title |
| Success | Success vs failure styling |
| Duration | Latency signal |
| Result | Truncated outcome (VS ~160 chars; router ~500) |

## 5. Manager status document

`GetStatus()` emits markdown:

- Enabled/Disabled  
- Feature flag table (including flags that may be status-only)  
- Active operations from ShardObserver  
- Recent safety blocks (up to 5)  

Surfaced by `/transparency` when enabled.

## 6. Bus Stats

`GlassBoxEventBus.Stats()`:

```
Enabled, SubscriberCount, BufferedEvents, TotalEmitted, CategoryCount, Verbose
```

**Missing today:** drop count, per-category histograms, last event time. Useful for “is Glass Box healthy?” debug.

## 7. Debug hooks

| Hook | Location |
|------|----------|
| `/glassbox status|verbose|category` | chat |
| `/transparency on|off` | chat |
| `/why` / logic pane | Explainer |
| logging when ToolEventBus attached | system base setter |
| logging TransparencyManager attached | ShardManager |

## 8. What to emit (producer guide)

**Do emit**

- Shard spawn/complete with duration  
- Tool/action outcomes  
- Kernel decisions that affect UX (next_action, deny)  
- JIT selection when JITExplain intended  

**Avoid flooding**

- Per-fact kernel pings without duration/milestone  
- Full file contents in Summary  
- Secrets in Details  

Prefer `EmitImmediate` for milestones; batched `Emit` for high-frequency orientation noise (unless verbose full-stream is on).

## 9. Correlation fields

| Field | Correlate |
|-------|-----------|
| TurnID | Chat turn |
| Source | Shard ID / action type |
| ID (sequence) | Cross-async order |
| Timestamp | Wall clock alignment with logs |

There is **no** distributed trace ID in this package. Campaign/session IDs live elsewhere; producers may embed them in Details if needed.

## 10. Metrics export

None built-in (no Prometheus/OTel). Future: optional Stats scraper or JSON sink without coupling to TUI.
