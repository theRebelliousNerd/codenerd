# tactile — Architectural Principles

> Last verified: **2026-07-13**  
> Binding for work inside `internal/tactile/` and its callers.

## P1 — Motor, not mind

Tactile **executes**. It does not decide whether an action is allowed, ethical, or strategically correct. Permission is VirtualStore + Mangle `permitted(...)`.

**Violation:** adding binary allowlists or intent heuristics inside DirectExecutor as “policy”.

## P2 — Explicit Command contract

All shell work uses `tactile.Command` with binary/args — not opaque shell strings as the only representation (callers may wrap shells, but the type remains structured).

## P3 — Distinguish infrastructure from command outcome

`ExecutionResult.Success` means the **execution machinery** worked. Exit codes and `Killed` carry command fate. Facts must preserve this split (`execution_success` vs `execution_nonzero` vs `execution_failure`).

## P4 — Default-deny sandbox networking

Docker ephemeral runs default network mode to **`none`** unless explicitly allowed. Prefer fail-closed isolation.

## P5 — Environment allowlisting

Do not pass full `os.Environ()` by default. Start from `AllowedEnvironment` and add explicit `Command.Environment`.

## P6 — Bound every capture

Timeouts and max output bytes are mandatory defaults. Unbounded subprocess output is a DoS against the agent itself.

## P7 — Auditability is part of the interface

Every meaningful motor event should be expressible as `AuditEvent` / `FileAuditEvent` and convertible to `Fact`. Callbacks may be nil, but conversion must stay pure and complete.

## P8 — Avoid import cycles with core

Tactile defines its own `Fact` and does not import `internal/core`. Adapters live in core (`TactileFileEditorAdapter`).

## P9 — Platform via build tags, not runtime spaghetti

OS-specific isolation (job objects, cgroups, clone flags) lives in tagged files with a shared `GetPlatformExecutor` / helpers. Keep portable types in untagged files.

## P10 — Layers for long workflows

Ephemeral Docker ≠ persistent Docker ≠ python.Environment ≠ swebench.Harness. Do not collapse multi-step state into `docker run --rm` loops when state must persist.

## P11 — Composite honesty

If a sandbox mode is requested, either execute under that mode or **fail**. Silent downgrade to unsandboxed direct violates user/policy expectations.

Composite now enforces this: only absent/none isolation selects Direct, while an
explicit unavailable mode fails closed.

## P12 — Minimal external surface for subpackages

`python` and `swebench` may depend on tactile; tactile root must not depend on swebench. Keep benchmark schema out of core types.

## Decision checklist (before changing tactile)

1. Does this add policy/intent? → Wrong package.  
2. Does this change fact predicates? → Update core Decl + consumers.  
3. Does this change Success semantics? → Document + tests + campaign handlers.  
4. Does this widen env/network? → Security review.  
5. Is there a wiring caller, or only a new type? → Wire or mark dormant honestly.
