# Tactile Docker execution: argv, effective configuration, and daemon ownership

## Scope and verdict

Source-only audit of `theRebelliousNerd/codenerd`, pinned to **`14a8da612092f270e95c39e35cf1a0111877486e`**, inspected 2026-11-02. This is a documentation deliverable, not an implementation or qualification receipt. No repository code was changed; no tests, builds, Docker commands, workloads, live model calls, or exploit reproductions were run. All proposed witnesses below remain **not run**.

**The immediately established blocker is malformed Docker launch argv.** The ephemeral `run` builder emits two option terminators before the image; persistent `create` emits three. Docker consumes the first terminator, then treats the next literal `--` as the image. This is separate from the already-fixed persistent `exec` builder. Production Python/SWE-bench setup actually reaches persistent creation, so source-level wiring does not establish a working container environment. [`internal/tactile/docker.go:443–454`](https://github.com/theRebelliousNerd/codenerd/blob/14a8da612092f270e95c39e35cf1a0111877486e/internal/tactile/docker.go#L443-L454), [`internal/tactile/persistent_docker.go:384–410`](https://github.com/theRebelliousNerd/codenerd/blob/14a8da612092f270e95c39e35cf1a0111877486e/internal/tactile/persistent_docker.go#L384-L410).

Two additional contracts matter independently: composite routing must use effective configuration, and ending a Docker client must not be reported as proof that daemon-owned work ended. Persistent output, pool admission, and idle reaping also need explicit resource/liveness guarantees. These are source-supported gaps, not measured incidents. For new environments using the current malformed creation path, later lifecycle risks are **latent behind that earlier failure**; do not present all findings as one observed failure chain.

### Guidance, prior work, and non-duplication

The repository's [root guidance](https://github.com/theRebelliousNerd/codenerd/blob/14a8da612092f270e95c39e35cf1a0111877486e/agents.md), [tactile guidance](https://github.com/theRebelliousNerd/codenerd/blob/14a8da612092f270e95c39e35cf1a0111877486e/internal/tactile/agents.md), [core guidance](https://github.com/theRebelliousNerd/codenerd/blob/14a8da612092f270e95c39e35cf1a0111877486e/internal/core/agents.md), and [architecture guidance](https://github.com/theRebelliousNerd/codenerd/blob/14a8da612092f270e95c39e35cf1a0111877486e/Docs/architecture/AGENTS.md) preserve policy ownership in Mangle/VirtualStore, default-deny effects, bounded execution, and the distinction between implemented code and production evidence. This audit proposes motor/configuration/lifecycle work, not a permission relaxation or a new model-facing shell surface.

Existing [tactile gap analysis](https://github.com/theRebelliousNerd/codenerd/blob/14a8da612092f270e95c39e35cf1a0111877486e/Docs/architecture/tactile/03-GAP-ANALYSIS.md) credits the persistent `ExecInContainer` terminator repair and idle eviction, and separately leaves genuine Linux-container Python argv qualification open. Those credits stand:

- `dockerExecArgs` now emits one separator before the container ID; its recording-CLI regression compares the complete argv. It does not exercise `run` or `create`. [Builder](https://github.com/theRebelliousNerd/codenerd/blob/14a8da612092f270e95c39e35cf1a0111877486e/internal/tactile/persistent_docker.go#L684-L700), [regression](https://github.com/theRebelliousNerd/codenerd/blob/14a8da612092f270e95c39e35cf1a0111877486e/internal/tactile/persistent_docker_unix_test.go#L14-L52).
- Python `RunPytest` uses a constant Python launcher and separate literal arguments. The problem here is container launch and ownership, not a claim that this fixed path still joins selectors into shell text. [Current pytest path](https://github.com/theRebelliousNerd/codenerd/blob/14a8da612092f270e95c39e35cf1a0111877486e/internal/tactile/python/environment.go#L654-L686).
- Idle eviction exists. The new issue is eviction without an active-operation lease, not the old “IdleTimeout unused” issue.
- The [session verification cleanup audit](https://github.com/theRebelliousNerd/codenerd/blob/14a8da612092f270e95c39e35cf1a0111877486e/Docs/audits/verification-process-cleanup-parity.md) concerns OS subprocess descendants. Docker daemon workloads have a different ownership boundary; killing the CLI's host process group is not a substitute for daemon-side cancellation.

Existing architecture [safety requirements](https://github.com/theRebelliousNerd/codenerd/blob/14a8da612092f270e95c39e35cf1a0111877486e/Docs/architecture/tactile/09-SAFETY-AND-INVARIANTS.md) and [testing guidance](https://github.com/theRebelliousNerd/codenerd/blob/14a8da612092f270e95c39e35cf1a0111877486e/Docs/architecture/tactile/10-TESTING-ALIGNMENT.md) are design inputs, not evidence that their broad guarantees already hold for every backend.

## 1. Production reachability and the first failing boundary

### Ordinary execution is not universally Docker-backed

`CompositeExecutor` registers Direct as its default and registers Docker only when its availability probe succeeds. `DefaultExecutorConfig` has no default sandbox. The normal `handleExecCmdModern` builds a command without `Sandbox`, then invokes the modern executor. Its result metadata includes the actual `SandboxUsed`. Therefore ordinary default shell execution is host-direct; this audit does **not** claim all user commands currently fail through Docker. Ephemeral Docker is an explicit/configured executor capability. [Factory](https://github.com/theRebelliousNerd/codenerd/blob/14a8da612092f270e95c39e35cf1a0111877486e/internal/tactile/factory.go#L35-L65), [defaults](https://github.com/theRebelliousNerd/codenerd/blob/14a8da612092f270e95c39e35cf1a0111877486e/internal/tactile/types.go#L426-L440), [ordinary caller](https://github.com/theRebelliousNerd/codenerd/blob/14a8da612092f270e95c39e35cf1a0111877486e/internal/core/virtual_store_actions.go#L245-L281).

### Python and SWE-bench have a real persistent-container caller

1. `VirtualStore.RouteActionResult` performs the kernel permission gate before dispatch. Python and SWE-bench setup have explicit dispatch cases. These findings assume an authorized action reaches them; they do not establish a constitutional bypass. [Gate](https://github.com/theRebelliousNerd/codenerd/blob/14a8da612092f270e95c39e35cf1a0111877486e/internal/core/virtual_store_routing.go#L108-L136), [dispatch](https://github.com/theRebelliousNerd/codenerd/blob/14a8da612092f270e95c39e35cf1a0111877486e/internal/core/virtual_store_routing.go#L580-L610).
2. On first use, `bench()` uses an injected `ContainerRuntime` if supplied; otherwise it creates `PersistentDockerExecutor(DefaultContainerPoolConfig())`, starts health/idle checking when available, and attaches the audit callback. A strict injected fake proves its own contract, not the production Docker transport. [Runtime ownership](https://github.com/theRebelliousNerd/codenerd/blob/14a8da612092f270e95c39e35cf1a0111877486e/internal/core/virtual_store_python.go#L44-L86).
3. Python setup checks availability, creates `python.Environment`, then calls `Initialize` before publishing the environment into the workbench and calling `Setup`. Initialization invokes `CreateContainer` and then `StartContainer`. [Handler](https://github.com/theRebelliousNerd/codenerd/blob/14a8da612092f270e95c39e35cf1a0111877486e/internal/core/virtual_store_python.go#L200-L220), [initialize](https://github.com/theRebelliousNerd/codenerd/blob/14a8da612092f270e95c39e35cf1a0111877486e/internal/tactile/python/environment.go#L216-L259).
4. SWE-bench setup similarly constructs a harness and calls `Initialize` before storing it and running setup. [SWE-bench handler](https://github.com/theRebelliousNerd/codenerd/blob/14a8da612092f270e95c39e35cf1a0111877486e/internal/core/virtual_store_python.go#L541-L561), [harness delegation](https://github.com/theRebelliousNerd/codenerd/blob/14a8da612092f270e95c39e35cf1a0111877486e/internal/tactile/swebench/harness.go#L65-L73).

With an ordinary Docker CLI following the inspected grammar, the malformed image operand prevents these newly created environments from reaching start, dependency installation, or pytest. Setup returns a failure rather than falsely declaring ready. A successful Docker version probe establishes neither image availability nor correct launch argv.

## 2. Findings and conditional consequences

### F1 — Extra terminators become image operands

The exact tails, omitting preceding options, are:

| Builder | Current tail | Required positional boundary |
|---|---|---|
| Ephemeral run | `-- -- IMAGE BINARY ARG...` | `-- IMAGE BINARY ARG...` |
| Persistent create | `-- -- -- IMAGE COMMAND...` | `-- IMAGE COMMAND...` |
| Persistent exec | `-- CONTAINER BINARY ARG...` | Already correct |

Docker's official [run reference](https://docs.docker.com/reference/cli/docker/container/run/) and [create reference](https://docs.docker.com/reference/cli/docker/container/create/) put the image first after options. More decisively, the inspected upstream implementation sets `copts.Image = args[0]`, with the remainder becoming the command. The vendored pflag parser stops at the first standalone `--` and appends all remaining tokens unchanged. Thus additional separators are not harmless repeated protection. [Docker CLI run](https://github.com/docker/cli/blob/8c8a77521af82b08bf3499cafb4c0e2e7b024936/cli/command/container/run.go#L33-L58), [create](https://github.com/docker/cli/blob/8c8a77521af82b08bf3499cafb4c0e2e7b024936/cli/command/container/create.go#L47-L72), [vendored pflag](https://github.com/docker/cli/blob/8c8a77521af82b08bf3499cafb4c0e2e7b024936/vendor/github.com/spf13/pflag/flag.go#L1131-L1160). Upstream is pinned independently to `8c8a77521af82b08bf3499cafb4c0e2e7b024936`; this is reference grammar, not a claim about an installed runtime version.

The consequence is an invalid image operand, not shell injection or a container escape. Exact diagnostic text and daemon behavior were not observed. Existing run-builder tests inspect options and suffixes, which can all be correct while the image position is wrong. [Representative test](https://github.com/theRebelliousNerd/codenerd/blob/14a8da612092f270e95c39e35cf1a0111877486e/internal/tactile/docker_platform_test.go#L12-L46).

**Acceptance:** compare complete argv for all three builders, retaining one defensive separator and literal command arguments; then qualify real `create → start → exec → remove` through the governed production route. Do not close this finding with the existing exec-only test.

### F2 — Backend selection precedes effective defaults

`CompositeExecutor.Validate` and `Execute` select using the unmerged command. `selectExecutor` treats nil/empty sandbox as none, and its specialized-limit choice inspects only explicit command limits. `ExecutorConfig.Merge` applies `DefaultSandbox` and nil `DefaultLimits` later, inside the selected executor. [Selection](https://github.com/theRebelliousNerd/codenerd/blob/14a8da612092f270e95c39e35cf1a0111877486e/internal/tactile/factory.go#L141-L203), [merge](https://github.com/theRebelliousNerd/codenerd/blob/14a8da612092f270e95c39e35cf1a0111877486e/internal/tactile/types.go#L443-L481).

A concrete conditional path is:

- The caller supplies `ExecutorConfig.DefaultSandbox.Mode = SandboxDocker`.
- The command has nil `Sandbox` and no explicit specialized resource request.
- Composite selects Direct; Direct validates the still-nil sandbox.
- Direct then merges the Docker default, but does not revalidate/reroute it.
- Direct executes `cmd.Binary` on the host and correctly reports `SandboxUsed=none`, even though its stored merged command now describes Docker.

This follows directly from [`internal/tactile/direct.go:107–176`](https://github.com/theRebelliousNerd/codenerd/blob/14a8da612092f270e95c39e35cf1a0111877486e/internal/tactile/direct.go#L107-L176). It is a configuration-policy mismatch, not proof that the shipped nil default promised isolation. Explicit per-command unavailable sandbox modes already fail closed and must keep doing so.

Similarly, nil command limits can acquire resource defaults too late to influence `limitsExecutor` selection. That proves selection ignores default-enforced-limit intent; actual enforcement must still be evaluated per platform/backend rather than assuming every direct platform ignores every limit. A nonnil limits struct only inherits timeout/output fields, so absence and zero-field semantics also need deliberate definition.

**Acceptance:** resolve and copy effective configuration before both validation and selection; ensure the selected backend validates exactly that effective request. Preserve intentional explicit `SandboxNone` overrides if permitted by the existing contract. Nil sandbox, empty-mode sandbox, and explicit none must be distinct, documented cases. Avoid mutating shared caller/default limit pointers during merge.

### F3 — Client cancellation is not a daemon termination receipt

Both Docker execution paths use `exec.CommandContext` on the local Docker binary and report `Killed=true` after the context ends. Neither path performs a corresponding daemon-side stop/remove/exec cancellation and confirmation in that execution flow. [Ephemeral execution/cancellation](https://github.com/theRebelliousNerd/codenerd/blob/14a8da612092f270e95c39e35cf1a0111877486e/internal/tactile/docker.go#L213-L313), [persistent execution/cancellation](https://github.com/theRebelliousNerd/codenerd/blob/14a8da612092f270e95c39e35cf1a0111877486e/internal/tactile/persistent_docker.go#L571-L681).

Go's [CommandContext contract](https://pkg.go.dev/os/exec#CommandContext) defaults to killing the command process. Here that process is the Docker client, not the daemon-owned workload. Docker [`--rm`](https://docs.docker.com/reference/cli/docker/container/run/#clean-up---rm) removes a container when it exits; it is not a receipt that client death caused that exit. Consequently the source supports a **missing termination guarantee**: a workload that survives client cancellation could continue consuming resources or changing its environment. It does not prove every canceled command survives, that a particular Docker version leaks, or that current malformed launches already ran a workload.

Persistent cancellation needs an explicit policy: terminate only the owned execution, or quarantine/terminate the entire environment when per-execution containment cannot be guaranteed. Removing a shared container unconditionally could kill unrelated active work. A timeout flag or audit event must not certify a stronger boundary than was observed.

### F4 — Partial initialization and shutdown do not preserve every cleanup obligation

If creation succeeds but start fails, `Environment.Initialize` retains its container pointer and returns without rollback. The Python handler publishes the environment only after initialization succeeds. Thus a failed start can leave a container tracked by the runtime but absent from the workbench's environment collection. This is a **post-launch-repair/alternative-runtime condition**, not a claim that malformed current create produced an orphan. [Initialization](https://github.com/theRebelliousNerd/codenerd/blob/14a8da612092f270e95c39e35cf1a0111877486e/internal/tactile/python/environment.go#L245-L259), [publication order](https://github.com/theRebelliousNerd/codenerd/blob/14a8da612092f270e95c39e35cf1a0111877486e/internal/core/virtual_store_python.go#L209-L220).

Cleanup does exist: explicit teardown calls force removal, and store close tears down known Python/SWE-bench environments. But close snapshots and clears those maps, shares a single 60-second context across sequential teardowns, logs failures, then stops the health loop. `PersistentDockerExecutor.Stop` stops background checking, not all containers. Idle reaping can eventually help while running; shutdown can remove that opportunity. [Teardown](https://github.com/theRebelliousNerd/codenerd/blob/14a8da612092f270e95c39e35cf1a0111877486e/internal/tactile/python/environment.go#L325-L347), [close](https://github.com/theRebelliousNerd/codenerd/blob/14a8da612092f270e95c39e35cf1a0111877486e/internal/core/virtual_store_python.go#L89-L117), [runtime Stop](https://github.com/theRebelliousNerd/codenerd/blob/14a8da612092f270e95c39e35cf1a0111877486e/internal/tactile/persistent_docker.go#L232-L249).

**Acceptance:** acquire cleanup ownership as soon as creation is acknowledged, before readiness publication. Failed cleanup must remain a visible owned obligation with bounded retry/reconciliation, not disappear because setup failed or close cleared a map. Cleanup should use its own bounded context rather than the already-canceled work context. Honor any deliberate preserve-for-debugging policy explicitly; do not silently infer it from unused config fields.

### F5 — Idle reaping can race an active execution

`performHealthChecks` calls `reapIfIdle` before probing health. The reaper reads `LastExecAt`, falling back to creation time, releases its lock, then force-removes an eligible container. `ExecInContainer` updates `LastExecAt` only after the command returns; it acquires no active-operation lease before launching. [Reaper](https://github.com/theRebelliousNerd/codenerd/blob/14a8da612092f270e95c39e35cf1a0111877486e/internal/tactile/persistent_docker.go#L263-L313), [exec/stat update](https://github.com/theRebelliousNerd/codenerd/blob/14a8da612092f270e95c39e35cf1a0111877486e/internal/tactile/persistent_docker.go#L552-L681).

Two schedules are possible: a long execution exceeds the idle threshold before its completion timestamp advances, or a short execution begins on an old container just before reaping. The second does not require a 30-minute command. Existing idle coverage labels a container “busy” through a recent timestamp; it does not run overlapping work. [Existing idle test](https://github.com/theRebelliousNerd/codenerd/blob/14a8da612092f270e95c39e35cf1a0111877486e/internal/tactile/persistent_docker_unix_test.go#L55-L91).

**Acceptance:** operation admission and transition to reaping must be mutually exclusive, using active leases/state generations or an equivalent protocol. Record idle age from completion when no admitted work remains. Cover concurrent admission after the idle snapshot, not only timestamp comparisons. Apply the ownership model to other operations that require a live container, not just exec.

### F6 — Resource contracts differ materially between paths

| Contract | Established implementation | Required clarification or correction |
|---|---|---|
| Captured output | Persistent exec uses unbounded `bytes.Buffer`; `ContainerExecOptions` has no output budget. Direct and ephemeral use separate limited writers, each with the full configured cap. | Bound persistent capture at ingestion. Decide/enforce a shared stdout+stderr budget because `MaxOutputBytes` is documented as combined. Report truncation/discarded bytes without converting truncation into test success. |
| Pool size | `MaxContainers=10` exists in default config, but `CreateContainer` has no capacity admission/reservation check before creation or insertion. | Enforce capacity atomically across concurrent creates; include in-flight reservations and release on failure. A configured number is not an enforced bound. |
| CPU quantity | `MaxCPUTimeMs` promises CPU-time consumption; ephemeral Docker translates it to a repeating CFS quota. | Do not equate cumulative CPU time with CPU rate. Enforce the promised dimension, expose a separately named CPU-rate setting, or reject unsupported intent. |
| Persistent CPU/memory | Creation emits `--memory` and `--cpus` from options/defaults. Python defaults request 4 GiB and two CPUs. | Credit these per-container constraints, but do not infer a pool/global bound, output-memory bound, or total CPU-time limit. |
| Network scope | Ephemeral builder defaults to none. Python default config enables network and initialization selects bridge. | Do not advertise every tactile/Python operation as network-isolated; make setup/test network policy explicit without widening permission. |

Sources: [resource field semantics](https://github.com/theRebelliousNerd/codenerd/blob/14a8da612092f270e95c39e35cf1a0111877486e/internal/tactile/types.go#L100-L128), [persistent options/defaults](https://github.com/theRebelliousNerd/codenerd/blob/14a8da612092f270e95c39e35cf1a0111877486e/internal/tactile/persistent_docker.go#L78-L130), [persistent creation](https://github.com/theRebelliousNerd/codenerd/blob/14a8da612092f270e95c39e35cf1a0111877486e/internal/tactile/persistent_docker.go#L319-L437), [persistent capture](https://github.com/theRebelliousNerd/codenerd/blob/14a8da612092f270e95c39e35cf1a0111877486e/internal/tactile/persistent_docker.go#L584-L642), [direct capture](https://github.com/theRebelliousNerd/codenerd/blob/14a8da612092f270e95c39e35cf1a0111877486e/internal/tactile/direct.go#L184-L195), [ephemeral capture](https://github.com/theRebelliousNerd/codenerd/blob/14a8da612092f270e95c39e35cf1a0111877486e/internal/tactile/docker.go#L226-L238), [CPU conversion](https://github.com/theRebelliousNerd/codenerd/blob/14a8da612092f270e95c39e35cf1a0111877486e/internal/tactile/docker.go#L418-L435), [Python defaults](https://github.com/theRebelliousNerd/codenerd/blob/14a8da612092f270e95c39e35cf1a0111877486e/internal/tactile/python/environment.go#L65-L80), [Python network selection](https://github.com/theRebelliousNerd/codenerd/blob/14a8da612092f270e95c39e35cf1a0111877486e/internal/tactile/python/environment.go#L226-L243).

For example, `MaxCPUTimeMs=5000` becomes quota 500000 microseconds per 100000-microsecond period: a nominal ceiling of five CPUs, not a lifetime allowance of five CPU seconds. This is a source/dimensional inference from Docker's [official CFS quota definitions](https://docs.docker.com/engine/containers/resource_constraints/#configure-the-default-cfs-scheduler), not a benchmark result. Output collected by the host client is also outside the container's own memory limit. Fixing launch grammar alone therefore does not close resource qualification.

## 3. Proposed execution contract

This is a proposed uplift, not a description of shipped guarantees.

1. **Normalize before authorization-dependent execution decisions.** Resolve effective sandbox/limits immutably before backend validation and selection. Retain the existing policy gate; if effective execution materially changes policy-relevant properties, its authorization must cover them. Unsupported required isolation/limits fail closed, never silently substitute host execution.
2. **One typed argv boundary per Docker subcommand.** Options precede one terminator; image/container/binary/arguments retain their exact roles. Validate operand shape where required, without introducing shell concatenation. Keep the already-correct exec and typed pytest behavior.
3. **Separate ownership from readiness.** Track resource acquisition before setup success. A container may be owned-but-not-ready, active, idle, canceling, cleanup-pending, or removed. These are proposed lifecycle states; do not claim existing enums already establish their transitions.
4. **Cancellation settles both transport and effect.** Distinguish cancellation requested, client reaped, daemon work terminated, and cleanup confirmed/unknown. A deadline cannot silently turn “unknown” into “killed.” Preserve nonzero workload exit separately from launch/transport failure and cancellation.
5. **Lease before use; reserve before create.** Admission, reaping, close, and capacity accounting must share one ownership protocol. No new operation may start after removal is committed; no idle collector may remove active work. Unrelated sessions/containers remain untouched.
6. **Bound each resource in its own unit.** Wall time, CPU rate, cumulative CPU time, daemon memory, host output, PID count, and pool concurrency are distinct. Capability declarations and receipts must state enforced dimensions, unsupported dimensions, and effective bounds.
7. **Close retains unresolved custody.** Failed cleanup is observable and reconcilable using exact owned identities. Do not scan/remove arbitrary user containers, delete debug-preserved state indiscriminately, or rely solely on a label shared by all codeNERD instances.

No new LLM request, planner heuristic, permission predicate, or broad Docker socket exposure is necessary to repair these boundaries. Any new kernel facts must have declared schemas and real-kernel validation; a callback fake is insufficient. Existing `Success=true` on nonzero process exit is intentional execution semantics, not a test-pass verdict, and must not be “fixed” by conflating these outcomes.

## 4. Fair acceptance and evaluation controls

All cells below are **proposed / not run**. Deterministic seams should locate defects cheaply; only daemon-backed checks can qualify daemon effects.

| Layer | Witness | Control and pass condition |
|---|---|---|
| Exact argv | Complete run/create/exec vectors with ordinary operands, leading-dash arguments, spaces, literal metacharacters, stdin, env, mount and resource flags | Existing correct exec vector remains unchanged; invalid operands refuse before execution; no shell interpretation. Compare whole vectors, not presence/suffix tests. |
| Effective config | Nil sandbox + Docker default; explicit Docker available/unavailable; intentional none; nil limits + enforced defaults | Default nil/no-isolation still selects Direct. Configured isolation selects its backend or refuses. Validate and Execute agree; source inputs are not mutated. |
| Governed setup | Normal authorized Python and SWE-bench setup with the production runtime | Real create/start/exec occur; denied action creates nothing; unavailable runtime returns failure; ready facts follow actual setup. Retain CodeDOM/world-model context for any product-level assessment. |
| Daemon cancellation | Start bounded work, obtain a readiness acknowledgment, cancel before its own completion/deadline | Observe effect-side cessation and exact owned identity cleanup/quarantine, not merely CLI exit or `Killed`. A sibling workload remains alive. Include quiet and output-producing work. |
| Partial lifecycle | Create succeeds, start fails; setup fails; cleanup fails; close deadline expires | Every acquired resource is removed or remains explicitly owned/reconcilable. A canceled work context cannot prevent bounded cleanup. No false ready/terminated receipt. |
| Idle race | Hold an exec open, advance controlled idle time; also admit exec after idle snapshot but before remove | Active operation survives, later genuine idle state reaps, and no admission occurs after removal commitment. No timing-only “sleep and hope” oracle. |
| Resource bounds | Concurrent creates at capacity, overflowing stdout and stderr, CPU-time versus rate requests | No over-admission, bounded host capture, accurate truncation; declared CPU dimension enforced or refused. Keep workload and host limits fixed across baseline/candidate. |
| Existing Python behavior | Literal selectors, failing test, venv-installed helper lookup, cancellation, sentinel absence | Genuine Linux container verifies argv and inherited PATH behavior. No host Python substitution, relaxed selector checks, or shell-based workaround. |

Record baseline and candidate commit, Docker CLI/daemon versions, OS/kernel/runtime configuration, image digest, cold/warm image/cache state, effective config, exact backend, timestamps, output counters, and final owned-resource inventory. Pinning an image digest avoids mutable-tag drift; preloading it separates launch correctness from network/pull availability. Keep mounts/network/permissions, work payload, deadlines, and observation horizon identical across variants.

For cancellation, readiness must prove daemon workload execution before cancellation; otherwise a fast launch failure can masquerade as successful cleanup. A finite observation interval must cover the fixture's intended delayed effect, and any inability to establish termination must be reported as inconclusive rather than a pass. Use disposable, scoped resources and externally bounded fixtures; never use destructive host probes or production datasets.

Do not score a model lower for the preexisting malformed transport, or score a repair higher by bypassing policy, using an injected success fake, moving work to the host, expanding permissions, hiding failing tests, weakening resource limits, or dropping setup from the denominator. If a later end-to-end agent comparison is authorized, keep task corpus, provider/model configuration, tool access, context budget, test expectations, and accounting constant. Track infrastructure failures separately from task resolution.

## 5. Costs, sequencing, and status

**Expected engineering cost, not an estimate measured here:** argv normalization and deterministic tests are narrow; immutable config normalization has cross-backend compatibility risk; cancellation/cleanup ownership is the larger lifecycle change because persistent state and concurrent operations must survive safely. Lease/capacity bookkeeping is small per operation but requires coordinated transitions. Daemon confirmation introduces bounded control-plane calls and cleanup latency. Bounded output reduces host memory exposure but requires honest truncation behavior. Image pulls, setup, snapshots, and preserved environments contribute disk/network/runtime costs that a per-container CPU/memory limit does not cap.

Sequence the work so that evidence is interpretable:

1. Repair and statically qualify argv; retain the existing exec/pytest protections.
2. Establish effective-default selection with positive and refusal controls.
3. Establish resource ownership, cancellation, cleanup, and active leases before broad live qualification.
4. Qualify the normal governed Python/SWE-bench path against a real daemon, then update each architecture claim only to the scope actually demonstrated.

**Current status:** source inspection supports the launch and configuration defects and the documented lifecycle/resource gaps. Implementation, unit validation, daemon qualification, production-path qualification, and performance/cost measurement are all **not performed in this audit**. No runtime pass, production incident, exploit, or whole-feature completion is claimed.

