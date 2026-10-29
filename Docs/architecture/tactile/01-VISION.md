# tactile — Vision

> Last verified: **2026-07-13**

## Product position

Tactile is codeNERD’s **motor cortex**: the only package that should own *how* the agent touches the host and containers. Everything else reasons *whether* and *what*.

### Target experience

1. **Every shell effect** runs through an `Executor` with:
   - explicit `Command` (binary/args/env/cwd/limits/sandbox)
   - bounded timeout and output capture
   - structured `ExecutionResult`
   - `AuditEvent` trail convertible to Mangle facts

2. **Every file mutation** runs through `FileEditor` (or a VirtualStore adapter over it) with:
   - line-oriented edit primitives for CodeDOM / surgical edits
   - content hashes for change detection
   - `file_read` / `file_written` / `lines_*` / `modified` facts

3. **Sandbox is a first-class dimension**, not an afterthought:
   - `none` for trusted local work (still policy-gated upstream)
   - `docker` for hermetic / untrusted workloads
   - `namespace` / `firejail` on Linux when available
   - persistent containers for multi-step Python / SWE-bench workflows

4. **Platform realism**:
   - Windows: Job Objects, taskkill tree, Docker when present
   - Linux: cgroups, namespaces, firejail preference ladder
   - macOS: Docker preferred for isolation; direct otherwise

5. **Benchmark and project envs** are **layers on top of motor primitives**, not parallel execution systems:
   - `python.Environment` owns clone → venv → patch → test
   - `swebench` remains a thin evaluation wrapper

## Non-goals

| Non-goal | Why |
|----------|-----|
| Intent understanding / planning | Kernel + perception |
| Permission / constitution | VirtualStore + Mangle `permitted` |
| Prompt assembly | `internal/prompt`, articulation |
| Fuzzy matching of shell output into NL | Keep analyzers structured; full NL is articulation |
| Client-app-specific workflows | Stay general (SWE-bench is a *benchmark harness*, not app product logic baked into core kernel) |

## Target architecture (steady state)

```
                    permitted(action)?
                           │
                           ▼
              ┌────────────────────────┐
              │   VirtualStore routes  │
              └────────────┬───────────┘
                           │
           ┌───────────────┼────────────────┐
           ▼               ▼                ▼
     FileEditor      CompositeExecutor   PersistentDocker
           │               │                │
           │         mode select            │
           │    ┌──────┼──────┐             │
           │    ▼      ▼      ▼             ▼
           │  Direct Docker NS/FJ     python.Environment
           │    │      │      │             │
           └────┴──────┴──────┴─────────────┘
                           │
                     Audit / Facts
                           ▼
                        Kernel
```

## Success criteria

| Criterion | Indicator |
|-----------|-----------|
| Single effector contract | All shell runs use `tactile.Executor` |
| Policy-before-motor | No production path executes shell without prior permission check (caller responsibility) |
| Fact completeness | Start/complete/kill/error + file ops always produce injectible facts when logger wired |
| Sandbox honesty | `SandboxUsed` and `execution_sandbox` reflect actual mode |
| Recoverable results | Infrastructure failure vs non-zero exit vs kill are distinguishable |
| Portable defaults | `DefaultExecutorConfig` works on Windows and Unix without secret env leakage |

## Evolution direction

1. Unify boot path on **audited CompositeExecutor** (or platform best) rather than bare Direct.  
2. Close Decl/policy gaps for all emitted predicates.  
3. Harden RetryExecutor timing (real clock, not busy-loop).  
4. Expand OutputAnalyzer beyond Go-centric patterns when needed — still structured.  
5. Keep python/swebench as optional depth, not required for day-to-day coding agent loop.
