# 01 — Vision: observability

> Last verified against codebase: 2026-07-13  
> Status: Target architecture vision (product + engineering)  
> Package: `internal/observability/`

## 1. Mission

Make every `nerd` process **self-explaining at the host layer**:

- At boot, leave a durable, machine-parseable record of scheduler/GC/memory/toolchain posture.  
- While running, retain a **cheap rolling execution trace** that can be frozen on failure.  
- Never become an agent executive, policy engine, or LLM prompt surface.

Success is measured in **minutes to post-mortem**, not dashboards.

## 2. Product experience (operator)

### 2.1 Invisible when healthy

With defaults:

- Boot log shows one metrics line + structured event.  
- Flight recorder runs with ~64 MiB / 30 s window without operator action.  
- No interactive UX change.

### 2.2 Obvious when broken

On fatal panic that reaches main:

- stderr prints `Flight trace dumped to <abs path>`.  
- File lands under `.nerd/traces/flight_*.trace`.  
- Operator runs `go tool trace` (or future internal viewer) and sees goroutine/sched history leading into the fault.

### 2.3 Deliberate control

Operators can:

| Control | Mechanism (target + current) |
|---------|------------------------------|
| Disable ring | `NERD_FLIGHTREC=0` or `features.flight_recorder: false` — **current** |
| Force enable | `NERD_FLIGHTREC=1` — **current** |
| Dump on demand | `/diag flightrec` or `nerd diag flightrec` — **vision; not wired** |
| Tune ring size / age | config keys or flags — **vision; prod hardcodes 64MiB/30s** |
| Mid-session metrics sample | `nerd status --runtime` or internal API — **vision** |

## 3. Architectural vision

```
┌─────────────────────────────────────────────────────────┐
│  cmd/nerd (host)                                        │
│    boot → observability.LogStartupMetrics               │
│    boot → StartFlightRecorder (feature-gated)           │
│    panic / diag → DumpFlightRecord                      │
└───────────────────────────┬─────────────────────────────┘
                            │ emits only
                            ▼
┌─────────────────────────────────────────────────────────┐
│  internal/logging (CategoryBoot)                        │
└─────────────────────────────────────────────────────────┘

┌─────────────────────────────────────────────────────────┐
│  Cortex OODA path (core / session / shards / …)         │
│    uses logging, glass box, transparency, prompt manifest│
│    does NOT import internal/observability               │
└─────────────────────────────────────────────────────────┘
```

Principles locked for the vision:

1. **Leaf forever** — no reverse deps into observability from core.  
2. **Host-owned wiring** — only binary entry (and maybe a thin CLI diag command) call it.  
3. **Complement, don’t replace** — logging = streams; observability = boot sample + crash ring; glass box = agent-visible timeline.  
4. **Go-native first** — prefer `runtime/metrics` and `runtime/trace` over bespoke formats until product needs diverge.

## 4. Non-goals

| Non-goal | Why |
|----------|-----|
| OpenTelemetry full stack | Heavy; wrong layer for a leaf diagnostics package |
| Mangle predicates for GC stats | Kernel should not depend on host metric plumbing |
| Replacing `internal/logging` | Different responsibility |
| Prompt atom selection traces | Belongs to `internal/prompt` |
| Continuous sampling every OODA turn | Cost/complexity; use logging spans / glass box |
| Remote crash upload | Privacy + workspace local-first model |

## 5. Success criteria

| Criterion | Signal |
|-----------|--------|
| Boot visibility | Structured `runtime_metrics_startup` present in boot logs after every start |
| Crash visibility | ≥1 valid Go trace file after main-goroutine panic with flag on |
| Upgrade safety | Metric path test red on Go metric retirement |
| Flag control | `NERD_FLIGHTREC=0` yields no start attempt / no ring |
| Leaf constraint | `go list` deps stay logging-only for internal packages |
| Operator dump | On-demand dump path exists and is documented (future) |

## 6. Evolution stages

| Stage | State |
|-------|--------|
| **S0** Boot metrics only | Implemented |
| **S1** Flight recorder + panic dump | Implemented |
| **S2** On-demand dump + workspace-aware nerdDir | Partial / missing |
| **S3** Config-tunable ring parameters | Missing |
| **S4** Optional mid-run metrics sample helper | Missing |
| **S5** Retention / rotate for `.nerd/traces/` | Missing |

Vision for S2–S5 does **not** require growing this package into a framework — thin APIs + CLI wiring only.
