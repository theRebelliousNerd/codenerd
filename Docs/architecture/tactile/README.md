# tactile — the motor boundary

> `VERIFIED CURRENT` against the live tree on 2026-07-13. The package is
> `internal/tactile`; constitutional permission remains outside this boundary.

## In one minute

Tactile is where a codeNERD decision becomes a process, a container command, or
a file edit. Its users are the VirtualStore and a small set of CLI/campaign
adapters; its visible outcome is a bounded `ExecutionResult` or `FileResult`
plus audit facts that the kernel can reason about. The common contract is
`internal/tactile/executor_interface.go#Executor`, with typed inputs and results
in `internal/tactile/types.go#Command` and
`internal/tactile/types.go#ExecutionResult`.

Tactile does not decide whether an effect is allowed. It validates what its
backend can execute, applies time/output/isolation limits, performs the effect,
and reports what happened. Calling it without the upstream permission route is
therefore a trust-boundary decision, not a harmless convenience.

## Its place in codeNERD

```text
LLM proposes work
  -> perception asserts intent
  -> Mangle derives an exact permitted action
  -> VirtualStore checks action + target + payload
  -> tactile selects an available backend and performs the effect
  -> audit/result facts return to the kernel
  -> articulation explains the outcome
```

`VERIFIED CURRENT`: `internal/core/virtual_store_routing.go#VirtualStore.RouteAction`
is the principal effect route, while
`internal/core/virtual_store.go#VirtualStore.CheckKernelPermitted` is the exact
authorization check. `internal/core/virtual_store.go#VirtualStore.initModernExecutor`
constructs an audited composite tactile executor and
`internal/core/virtual_store.go#VirtualStore.injectTactileFact` returns its facts
to the kernel. This preserves the north-star split: the model is creative,
Mangle and VirtualStore are executive, and tactile is motor control.

The package also owns direct, Docker, persistent-Docker, platform-limit,
file-edit, Python-environment, and SWE-bench adapters. It does not own prompt
selection, policy, action routing, or user-facing prose.

## A representative journey

Consider a constitution-cleared request to run `go test ./internal/tactile`.

1. The router hands the exact action envelope to
   `internal/core/virtual_store_routing.go#VirtualStore.RouteAction`; a mismatch
   in action, target, or payload is rejected before tactile.
2. The VirtualStore translates the effect into
   `internal/tactile/types.go#Command` and invokes its modern executor.
3. `internal/tactile/factory.go#CompositeExecutor.selectExecutor` selects the
   requested backend. An explicit but unavailable sandbox now fails closed;
   only an omitted/`none` sandbox may use the direct default. This is pinned by
   `internal/tactile/docker_detection_test.go#TestCompositeExecutorExplicitMissingSandboxFailsClosed`.
4. For host execution, `internal/tactile/direct.go#DirectExecutor.Execute`
   merges configured limits, creates a deadline context, captures bounded
   stdout/stderr, classifies cancellation/non-zero exit/infrastructure failure,
   and emits lifecycle audit events.
5. `internal/tactile/audit.go#AuditEvent.ToFacts` converts the observed result
   into facts; the VirtualStore injects them into the kernel for subsequent
   policy, planning, and articulation.

Failure is explicit at each boundary. Missing permission never reaches tactile;
missing requested isolation returns an error; a deadline yields a killed result;
an executable that returns non-zero is distinguished from infrastructure
failure; excess output is discarded while its byte count is retained.

## What exists today

- `VERIFIED CURRENT`: direct execution enforces deadline and output ceilings in
  `internal/tactile/direct.go#DirectExecutor.Execute`; timeout, cancellation,
  non-zero exit, and truncation are covered in
  `internal/tactile/tactile_test.go#TestDirectExecutor_Timeout` and
  `internal/tactile/tactile_test.go#TestDirectExecutor_OutputTruncation`.
- `VERIFIED CURRENT`: Docker availability is a bounded, expiring shared probe in
  `internal/tactile/docker.go#DockerExecutor.detectDocker`; configuration is
  preserved through both composite and factory construction, proven by the
  focused tests in `internal/tactile/docker_detection_test.go`.
- `VERIFIED CURRENT`: file edits resolve a working directory, hash before/after
  content, and emit structured facts through
  `internal/tactile/files.go#FileEditor`; path behavior is exercised by
  `internal/tactile/files_test.go#TestFileEditor_PathSecurity`.
- `PARTIAL`: the audited VirtualStore path is strong, but several boot, DOM, and
  campaign call sites still construct `NewDirectExecutor` explicitly. Their
  upstream permission assumptions are not expressed as one typed exception
  registry.
- `PARTIAL`: capability reporting exists through
  `internal/tactile/types.go#ExecutorCapabilities`, but the kernel does not yet
  choose a backend from a typed, Mangle-governed resource/isolation requirement.
- `N-A — JIT and agents`: tactile has no LLM prompt or specialist behavior by
  design; JIT selects creative context upstream, and only the resulting
  constitution-cleared action enters this package.

The complete realized inventory and contracts live in
[IMPLEMENTED_SPEC.md](IMPLEMENTED_SPEC.md) and
[02-CURRENT-STATE.md](02-CURRENT-STATE.md). Safety and failure semantics are in
[09-SAFETY-AND-INVARIANTS.md](09-SAFETY-AND-INVARIANTS.md) and
[12-FAILURE-MODES.md](12-FAILURE-MODES.md).

## North star

Every physical effect should cross one explainable chain: proposed intent,
exact permission, backend admission, bounded execution, observed result, and
turn-correlated articulation. Requested isolation must never silently weaken,
resource limits must be enforceable rather than decorative, and a restart or
retry must not duplicate an already completed effect.

Non-goals are equally important: tactile will not become a second policy engine,
interpret fuzzy model text, choose product strategy, or pretend all platforms
provide identical containment. When a host cannot satisfy a required control,
the correct result is an observable denial or degradation—not an invisible host
fallback.

## Improvement frontier

The safe immediate repair is to enumerate every direct-executor construction and
route it through the audited VirtualStore path or a typed, tested exception. The
bounded leverage step is a versioned execution receipt carrying action/request
identity, permission decision, backend, effective limits, result, and audit-fact
status without persisting unbounded output.

The longer-horizon option is Mangle-governed backend admission: assert required
isolation/resource facts and observed backend capabilities, derive an exact
admission plan, and require tactile to verify that plan before execution. Tactile
still performs no policy reasoning; it enforces the selected contract and
returns evidence. Authoritative cards and acceptance boundaries are in
[TODO.md](TODO.md).

## Choose a reading route

- **90 seconds:** this README, then
  [03-GAP-ANALYSIS.md](03-GAP-ANALYSIS.md).
- **10 minutes:** add [05-INTERNAL-ARCHITECTURE.md](05-INTERNAL-ARCHITECTURE.md),
  [08-WIRING-AND-INTEGRATION.md](08-WIRING-AND-INTEGRATION.md), and
  [09-SAFETY-AND-INVARIANTS.md](09-SAFETY-AND-INVARIANTS.md).
- **Deep implementation:** read [IMPLEMENTED_SPEC.md](IMPLEMENTED_SPEC.md),
  [06-PUBLIC-API-AND-TYPES.md](06-PUBLIC-API-AND-TYPES.md),
  [10-TESTING-ALIGNMENT.md](10-TESTING-ALIGNMENT.md), and the source beginning at
  `internal/tactile/factory.go#NewCompositeExecutorWithConfig`.

Governance and signed evidence are recorded in [_progress.md](_progress.md);
unresolved design choices remain in [OPEN-QUESTIONS.md](OPEN-QUESTIONS.md).
