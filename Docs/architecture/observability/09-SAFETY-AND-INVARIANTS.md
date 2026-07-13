# 09 — Safety and Invariants: observability

> Last verified against codebase: 2026-07-13  
> Package: `internal/observability/`

## 1. Constitutional safety relationship

This package is **not** a policy actor:

- Does not evaluate `permitted(...)`.  
- Does not assert facts into the kernel.  
- Does not execute tools or VirtualStore actions.  
- Cannot grant, deny, or soft-bypass agent permissions.

Safety contribution is **forensic**: when the process misbehaves, operators get boot posture and (when dump works) a Go execution trace. That supports high-assurance operations without participating in the executive loop.

## 2. Hard invariants

| ID | Invariant | Enforcement |
|----|-----------|-------------|
| I1 | Leaf imports only (logging + stdlib) | Code review; import block |
| I2 | At most one active FlightRecorder wrapper | `flightMu` + nil checks; mirrors runtime |
| I3 | Second Start does not replace/stop first | Early return when `flight != nil` |
| I4 | Dump does not stop recorder | Implementation + `TestFlightRecorder_DoubleDumpKeepsRecorderRunning` |
| I5 | Dump buffers before disk write | `bytes.Buffer` then `WriteFile` |
| I6 | Empty `nerdDir` rejected | Explicit error |
| I7 | Metrics paths must exist in runtime | `TestStartupMetricPaths_AllSupported` |
| I8 | `LogStartupMetrics` must not panic | `TestLogStartupMetrics_NoPanic` |
| I9 | Feature gating stays outside package | No env/config read for enablement here |
| I10 | Observability failure does not block CLI | main warns and continues |

## 3. Concurrency

| Resource | Protection |
|----------|------------|
| `flight` pointer | `sync.Mutex` on read-modify paths |
| Dump I/O | Uses pointer copy; mutex not held across WriteTo/WriteFile |
| Metrics path | No shared mutable package state beyond constants |

**Test constraint:** only one FlightRecorder per process — tests **must** `StopFlightRecorder` in cleanup (`resetFlightRecorder`). Parallel tests that all Start will fight; current tests serialize via package-level cleanup patterns.

**Race testing:** `go test -race ./internal/observability/...` is recommended; mutex paths are simple but dump vs stop races are possible if stop is concurrent.

## 4. Filesystem safety

| Operation | Mode / notes |
|-----------|--------------|
| Create `.nerd/traces` | `0755` via `MkdirAll` |
| Write trace file | `0644` |
| Path construction | `filepath.Join` under provided root — **caller must pass trusted workspace** |

No path traversal sanitization beyond Join semantics. Callers must not pass untrusted remote paths as `nerdDir`.

## 5. Secrets and privacy

| Concern | Assessment |
|---------|------------|
| Metrics content | Scheduler/GC counters — no API keys |
| Trace content | May include **function names, stacks, goroutine activity**; can reveal code paths and timing — treat as sensitive workspace artifacts |
| Logs | Boot category only; no LLM payloads |

Do not ship `.nerd/traces/*.trace` to untrusted parties without review.

## 6. Panic path safety

| Rule | Behavior |
|------|----------|
| Re-raise original panic | Always after dump attempt |
| Nested panic in dump | Would replace original — dump is best-effort and avoids panic (errors returned) |
| Recover scope | Only panics unwinding through main’s defer |

Chat-level `recover` that does **not** re-panic will skip dump — intentional isolation vs complete crash telemetry. Document for operators; do not silently claim global coverage.

## 7. Resource bounds

| Resource | Bound |
|----------|-------|
| Ring memory | Production 64 MiB max bytes (configable only via Start args) |
| Ring time | Production 30 s min age |
| Dump buffer | Peak ≈ ring size while writing |
| Disk | Unbounded accumulation of dump files over many panics |

On memory-constrained hosts, disable via `NERD_FLIGHTREC=0`.

## 8. Mangle / Decl

**N/A.** No `.mg` sources; no Decl obligations.

## 9. Default-deny analogy

The package itself is not a permission system, but it observes a related discipline:

- **No surprise network egress** (none).  
- **No surprise kernel mutations** (none).  
- **Feature default on for flight recorder** is an ops choice (cheap forensic value), not a security deny/allow for agent actions.

Do not flip flight recorder default to off without an operator communication plan — first-panic value is the product argument for default-on.
