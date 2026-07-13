# tactile — Alignment / Vision Review

> Last verified: **2026-07-13**  
> Against: codeNERD north star + living `internal/tactile/` code

## Scoring rubric

| Score | Meaning |
|------:|---------|
| 5 | Fully realized and wired |
| 4 | Implemented with minor gaps |
| 3 | Partial / platform-skewed |
| 2 | Designed, weakly used |
| 1 | Stub or aspirational |
| 0 | Absent / contradicts north star |

## Dimensions

| Dimension | Score | Evidence |
|-----------|------:|----------|
| **Inversion of control** (LLM creative, logic executive) | **5** | Package comment and design: *minimal logic*; constitutional checks belong in VirtualStore. Tactile executes what it is told. `internal/tactile/types.go` package docs. |
| **Motor / effector clarity** | **5** | Explicit neuroscience framing: Perception / Kernel / Articulation / **Tactile**. README + package docs. |
| **Structured feedback to kernel** | **4** | `AuditEvent.ToFacts`, `FileAuditEvent.ToFacts`, test/build analyzers → `Fact`. VirtualStore `injectTactileFact` wires to `kernel.Assert`. Some predicates may lack full policy consumers. |
| **Sandbox spectrum** | **4** | Direct, Docker ephemeral, Persistent Docker, Linux namespaces, Firejail, Windows Job Objects, Linux cgroups. Composite routes by `SandboxMode`. Firejail/namespace not registered by default Composite on all hosts. |
| **Resource limits** | **4** | `ResourceLimits` + platform enforcers (rlimits, cgroups, job objects, Docker flags). DirectExecutor on Windows claims limited rlimit support in capabilities. |
| **Cross-platform** | **4** | Build tags for windows / linux / darwin / unix. Real Windows job-object code; Linux namespace/cgroup/firejail; Darwin falls back to Docker or direct. |
| **JIT prompt atoms** | **n/a** | Not an LLM-facing package; correctly has no prompt surface. |
| **Constitutional default-deny** | **2–3** | Tactile does **not** enforce `permitted(...)`. Correct layering *if* all callers go through VirtualStore. Direct CLI/campaign use of `NewDirectExecutor()` can bypass sandbox and policy unless caller enforces. |
| **Wiring completeness** | **3** | Chat boot uses **DirectExecutor** + FileEditor adapter, not always full Composite+audit path. `initModernExecutor` exists on VirtualStore for audited composite — parallel path. |
| **Observability** | **4** | `CategoryTactile`, StartTimer, ExecutionMetrics, optional JSONL file audit. |
| **Test confidence** | **4** | Dense unit tests for audit facts, docker args, factory, files, platform windows; less live Docker/namespace integration on CI. |
| **Subpackages (python/swebench)** | **3** | Solid domain model; consumers outside package are sparse compared to core Executor/FileEditor path. |

**Overall alignment: ~4.0 / 5** — tactile is a mature motor layer that respects the north star (dumb effector, smart kernel). Main tension is **who constructs which executor** (direct vs composite vs modern audited) and ensuring every effectful path remains policy-gated.

## North-star quote fit

> Logic determines reality; the model merely describes it.

Tactile fits as **reality’s hands**: it does not invent next actions. It must not become a second policy engine. Current code mostly honors that; expansion should stay in limits/sandbox/audit quality, not intent classification.

## Risks to alignment

1. **Bypass culture** — callers constructing `DirectExecutor` for convenience without VirtualStore permission.
2. **Fact spam without consumers** — many execution facts may be asserted without corresponding rules that change `next_action`.
3. **Success semantics** — `Success=true` with non-zero exit is correct for infrastructure vs command outcome, but easy to mis-handle in policy if not using `execution_nonzero` / exit codes carefully.
