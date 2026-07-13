# tactile feature cards

> Sole authoritative `NERD_FEATURE` surface for the tactile corpus.

## P0: Preserve requested isolation and construction configuration

<!-- NERD_FEATURE
id: tactile-failclosed-backend-selection-v1
owner: tactile
status: verified
kind: truth-gap
depends_on: []
affects: [tactile, core, system]
-->

**Value.** A command that explicitly requests Docker, namespace, or Firejail can
never run directly on the host because the requested backend is missing, and
factory-created Docker executors retain the caller's limits.

**Evidence and observed gap.** `internal/tactile/factory.go#CompositeExecutor.selectExecutor`
previously returned the direct default for every unregistered mode, while
`internal/tactile/factory.go#ExecutorFactory.CreateDocker` constructed default
configuration. Both are now corrected; the earlier Docker probe amplification
and composite configuration drift were already repaired in
`internal/tactile/docker.go#DockerExecutor.detectDocker`.

**Desired behavior.** Only absent/`none` isolation uses direct execution. An
explicit unavailable mode returns a stable error. Every construction route
preserves the supplied `ExecutorConfig`, and availability probes remain bounded.

**Non-goals.** This does not make Docker mandatory, claim identical platform
controls, or move permission decisions into tactile.

**Affected contracts.** Composite selection, factory creation, VirtualStore
modern execution, effective time/output/environment bounds.

**Positive acceptance.** The four focused tests in
`internal/tactile/docker_detection_test.go` prove cached negative probes,
composite config parity, explicit fail-closed selection, and factory config
parity; package and race gates pass.

**Negative acceptance.** Omitted isolation still selects direct; unavailable
Docker remains an error; no test invokes the real Docker daemon.

**Rollback.** Revert selection/config/tests together only if every caller gains
an equivalent typed preflight that cannot silently downgrade isolation.

## P0: Govern every direct-executor bypass

<!-- NERD_FEATURE
id: tactile-direct-bypass-registry-v1
owner: tactile
status: proposed
kind: truth-gap
depends_on: [tactile-failclosed-backend-selection-v1]
affects: [tactile, core, system, cli, campaign]
-->

**Value.** Reviewers and operators can prove whether each host process launch
crossed the constitutional and audited route.

**Evidence and observed gap.** `internal/core/virtual_store.go#VirtualStore.initModernExecutor`
builds the audited composite route, but `internal/system/factory.go#initExecutionLayer`,
`cmd/nerd/chat/session_boot.go`, DOM commands, and campaign commands also
construct direct executors. The live tree has no single typed exception registry
that classifies their permission and audit boundaries.

**Desired behavior.** Route production effect paths through VirtualStore, or
register a narrow exception with owner, caller, permission proof, audit sink,
limits, and removal/review date. Make unregistered direct construction fail a
static or integration gate.

**Non-goals.** Do not ban package-local tests, hide platform adapters, or require
containers for harmless read-only probes.

**Affected contracts.** Boot wiring, CLI commands, campaigns, action IDs,
permission evidence, audit fact injection.

**Positive acceptance.** A generated/live call-site inventory accounts for each
production constructor; integration tests prove governed routes deny mismatched
action/target/payload and emit correlated results.

**Negative acceptance.** Adding a new production `NewDirectExecutor` without a
registered contract fails CI; an exception cannot grant permission itself.

**Rollback.** Keep adapters behind the registry while migrating consumers; do
not restore an undocumented bypass.

## P1: Emit one bounded execution receipt

<!-- NERD_FEATURE
id: tactile-effect-receipt-v1
owner: tactile
status: proposed
kind: leverage
depends_on: [tactile-direct-bypass-registry-v1]
affects: [tactile, core, observability, transparency, articulation]
-->

**Value.** One record answers what was authorized, which backend and effective
limits were used, what completed, what was truncated, and which facts reached the
kernel—without storing unbounded command output.

**Evidence and observed gap.** `Command`, `ExecutionResult`, `AuditEvent`, and
`FileAuditEvent` carry overlapping session/request/effect data, while
`internal/tactile/audit.go#AuditEvent.ToFacts` emits several lifecycle facts.
There is no versioned idempotent receipt joining the permission envelope to all
of those observations.

**Desired behavior.** Define a versioned receipt with action/request/session IDs,
permission digest, backend, effective limits, timing, exit/kill/truncation class,
output digests and bounded previews, fact-injection status, and idempotency key.

**Non-goals.** Do not persist secrets, full stdout/stderr, environment values, or
turn the receipt into authorization.

**Affected contracts.** VirtualStore, tactile audit callbacks, logging,
transparency, articulation, retention/redaction policy.

**Positive acceptance.** Success, non-zero exit, cancellation, timeout,
infrastructure failure, output truncation, and partial fact-injection fixtures
produce one correlated receipt; duplicates converge by idempotency key.

**Negative acceptance.** Denied actions have no execution receipt; previews stay
bounded and redacted; receipt-write failure is visible but never re-executes an
effect.

**Rollback.** Dual-write existing facts and the receipt until parity is proven;
disable receipt persistence without weakening execution bounds.

## P2: Derive backend admission from typed requirements

<!-- NERD_FEATURE
id: tactile-mangle-admission-plan-v1
owner: tactile
status: proposed
kind: north-star
depends_on: [tactile-effect-receipt-v1]
affects: [tactile, core, mangle, system]
-->

**Value.** The logic executive can prove that an available backend satisfies the
exact isolation, network, time, memory, process, and output contract before any
process starts.

**Evidence and observed gap.** `internal/tactile/types.go#ExecutorCapabilities`
reports coarse backend abilities and `ResourceLimits` describes requested
bounds, but backend selection is a Go map lookup rather than a derived admission
decision with provenance.

**Desired behavior.** Assert typed execution requirements and observed backend
capabilities, derive one exact short-lived admission plan in Mangle, and require
the executor to verify the plan digest and effective limits. Default deny when a
required control is unsupported.

**Non-goals.** Mangle does not launch processes; tactile does not infer fuzzy
requirements; missing host controls are not represented as success.

**Affected contracts.** Mangle declarations/policy, executor capabilities,
VirtualStore authorization, platform probes, execution receipts.

**Positive acceptance.** Cross-platform matrices prove only satisfying backends
are admitted; stale capability or plan facts, unsupported network isolation,
excess limits, and mismatched payloads are denied before execution.

**Negative acceptance.** An unavailable sandbox cannot fall back to host direct;
capability observations expire; admission cannot outlive its action ID.

**Rollback.** Retain the current fail-closed Go selection behind a feature gate;
rollback removes derived admission but never restores isolation downgrade.
