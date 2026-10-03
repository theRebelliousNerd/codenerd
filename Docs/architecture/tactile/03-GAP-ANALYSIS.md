# tactile — Gap Analysis

## Bounded argv evidence (2026-10-02)

**VERIFIED CURRENT:** `internal/tactile/python/environment.go#Environment.RunPytest` (`internal/tactile/python/environment.go:654`) now sends a constant Python launcher and separate literal selectors through the runtime, preserving the inherited PATH with the running virtualenv interpreter directory prepended. `artifact:.corpus-build/runs/all-features-20261002/round4-python.receipt.json` passes the complete Python and SWE-bench package gate (61 PASS events including subtests): actual host selector/failure status, sentinel absence, helper execution, PATH/environment inheritance and cancellation controls remain intact. Prior core governed-routing/verdict controls pass in `round3-python-wiring.receipt.json`; those use a strict runtime fake. GAP-TACTILE-PYTHON-ARGV is partial, not closed: a genuine Linux container and normal production action witness remain required. This current evidence supersedes the old shell-joining description in the historical accepted row below, not the broader July audit.

**PROPOSED UPLIFT — Python argv host-probe boundary:** preserve literal arguments, exact inherited venv PATH prefix/tail, genuine helper execution, failing-test status, cancellation and sentinel absence. Supported production Linux/container helper lookup remains unqualified and must be live-verified. A Windows host probe must explicitly resolve a helper through that inherited PATH and assert the selected venv executable before launching it: Windows `subprocess` executable resolution does not let the supplied environment override PATH. This platform distinction does not authorize a shell, an expected base-interpreter substitution, or removal of Linux/container coverage. [Python subprocess documentation](https://docs.python.org/3/library/subprocess.html), [PATH lookup documentation](https://docs.python.org/3/library/shutil.html#shutil.which).

> Last verified: **2026-08-09**

## Method

Compare vision ([01-VISION.md](01-VISION.md)) and north star to living code. Distinguish **real gaps** from **intentional non-goals**.

## Spec vs reality matrix

| Vision item | Reality | Gap? |
|-------------|---------|------|
| All shell via Executor | Yes when callers use it | **Call-site discipline** |
| Policy-before-motor | VirtualStore path yes; bare executor constructors free | **Medium** — process/convention gap |
| Fact completeness when logger wired | Strong event→fact mapping | **Low** — wiring not always on |
| Sandbox spectrum | Direct/Docker/NS/Firejail/Job/cgroup | **Partial** — default Composite only none+docker |
| Platform realism | Solid per-OS code | **Low–Medium** — GetPlatformExecutor asymmetry Windows vs Darwin |
| Structured results | ExecutionResult rich | **None** |
| File motor + facts | FileEditor complete | **Low** — FileOpPatch emits no facts |
| python/swebench layers | Implemented | **Medium** — external adoption thin |
| Output analyzers multi-lang | Go-centric | **Low** intentional for now |
| No local .mg | Correct | **n/a** — decls global |

## Prioritized gaps

### P0 — correctness / safety process

| ID | Gap | Why it matters | Suggested direction |
|----|-----|----------------|---------------------|
| G-P0-1 | Callers can construct `NewDirectExecutor()` and run shell **outside** VirtualStore permission | Bypasses constitutional default-deny | Document contract; prefer factory through VS; audit callers |
| G-P0-2 | Chat boot uses Direct, not always audited Composite (`initModernExecutor` parallel path) | Execution facts may not hit kernel on primary UX path | Unify boot on modern audited executor |

### P1 — capability honesty

| ID | Gap | Why | Direction |
|----|-----|-----|-----------|
| G-P1-1 | Composite never auto-registers namespace/firejail | Those backends require explicit registration; unavailable explicit modes now fail closed | Register when available if default discovery is desired |
| G-P1-3 | Windows `GetPlatformExecutor` ignores Docker availability for return type | Platform “best” less useful on Windows | Align with Darwin Composite behavior |
| G-P1-4 | RetryExecutor delay not real sleep | Retries may spin | Use `time.After` + ctx |

### P2 — integration depth

| ID | Gap | Why | Direction |
|----|-----|-----|-----------|
| G-P2-1 | Audit predicates are Decl-aligned, but policy consumption is sparse | Facts without rules remain telemetry only | Add consumers only where a concrete executive decision needs them |
| G-P2-2 | docker stats not used for ResourceUsage | Docker path has SupportsResourceUsage false | Optional stats parse |
| G-P2-3 | PersistentDocker idle timeout config unused for eviction | Config field vs behavior drift | Implement idle reaper or drop field |
| G-P2-4 | SWE-bench harness not first-class CLI command surface | Benchmark path harder to operate | Optional CLI later |

### Closed in the 2026-07-13 audit

- Repeated composite construction formerly performed a synchronous Docker probe
  for every instance. Availability is now cached for 30 seconds with focused
  negative-cache coverage.
- The composite factory formerly probed Docker through default configuration
  even when the caller supplied another binary/config. It now forwards the
  actual `ExecutorConfig`, with a focused configuration-propagation regression.

### Closed in the 2026-08-09 audit

- OutputAnalyzer formerly reused five tester/world predicates with incompatible
  arities and types. It now emits dedicated `execution_*` summaries declared in
  `schemas_shards.mg`, and a real-kernel test proves every emitted fact is
  accepted and queryable.
- Audit-file replacement/rotation no longer leaks or retains closed handles;
  sink errors are observable, secrets are redacted, and stored output is bounded.
- `AuditedExecutorWrapper` now supplies lifecycle events for executors that do
  not implement `SetAuditCallback`.

### Closed in the 2026-09-25 wave-3 pass

- G-P0-1 (bare direct executors outside permission): governed by
  `tactile.DirectBypassRegistry` and its static gate; bypasses audit into the
  kernel (`NewFactAuditedExecutor`).
- G-P0-2 (chat boot on Direct): VirtualStore always runs commands on its
  audited composite, which now inherits the caller's config; `Cortex.Executor`
  is `VirtualStore.AuditedExecutor`.
- G-P1-1 (namespace/firejail never registered): `registerPlatformIsolation`
  registers probed backends; the namespace executor uses a user namespace
  when unprivileged.
- G-P1-3 (Windows `GetPlatformExecutor`): `GetPlatformExecutor` and the
  `ExecutorFactory` that was its only caller were removed on all platforms.
- G-P1-4 (RetryExecutor busy-wait): `RetryExecutor` had no consumer and was
  removed (open question 5 answered by removal).
- G-P2-3 (idle timeout unused): `PersistentDockerExecutor.reapIfIdle` enforces
  `IdleTimeout` in the health pass.
- G-P2-4 (SWE-bench not a CLI surface): `nerd swebench evaluate` routes setup
  and evaluate through the kernel and prints the verdict the kernel derives.
- Python/SWE-bench handlers returned Success for work nothing did; they now
  drive `python.Environment`/`swebench.Harness` in persistent containers and
  fail honestly. `ExecInContainer`'s argv (three `--`) could never have worked
  and is fixed.
- Still open: G-P2-1 (consumer rules for execution facts -- add only where an
  executive decision needs one), G-P2-2 (docker stats), G-P3-1..3 (intentional
  or cosmetic).

### P3 — polish

| ID | Gap |
|----|-----|
| G-P3-1 | OutputAnalyzer Go-only |
| G-P3-2 | FileOpPatch no facts |
| G-P3-3 | Combined stdout/stderr not true interleave (concatenate) |
| G-P3-4 | Package README structure mentions `executor.go` legacy SafeExecutor — file not present in tree |

## Non-gaps (do not “fix”)

| Item | Why not a gap |
|------|----------------|
| No permission engine inside tactile | North star: executive is kernel/VS |
| No prompt atoms | Not LLM-facing |
| No Vectryx coupling | Correct isolation |
| Success=true on non-zero exit | Documented intentional semantics |
| Env allowlist vs full environment | Security feature |

## Gap summary scorecard

| Area | Status |
|------|--------|
| Core execute path | Strong |
| Sandbox default composition | Weak-moderate |
| Fact generation | Strong |
| Fact consumption | Moderate |
| Boot wiring | Moderate |
| Platform depth | Strong (code), uneven (selection) |
| Benchmark stack | Strong library / weak productization |

## 2026-10-02 accepted typed-Python execution obligation

| Gap ID | Capability | Current state | Target state | Severity | Phase | Blocking dependencies | Exit criteria |
|---|---|---|---|---|---|---|---|
| GAP-TACTILE-PYTHON-ARGV | Literal pytest argument execution | **PARTIAL:** `internal/core/virtual_store_python.go#VirtualStore.handlePythonRunPytest` (`internal/core/virtual_store_python.go:285`) forwards test_args to `internal/tactile/python/environment.go#Environment.RunPytest` (`internal/tactile/python/environment.go:641`). The driver joins arguments into command text and sends it through `#Environment.execInRepoVenv` at line 778, which invokes sh -c. This is source-confirmed shell interpretation; no exploit has been executed in this audit. | **PROPOSED UPLIFT:** invoke the virtualenv Python/pytest executable through ContainerRuntime with separate literal argv, retaining working directory, caller context, configured timeout, output, and true test exit status. | Critical | Accepted, not implemented | Root regression and governed production-route gates | Capturing-runtime tests prove selectors with metacharacters, spaces, quotes, and command-substitution text remain literal arguments or receive typed refusal; no sh -c invocation occurs. Failure status and cancellation are preserved. A root-run governed action cannot create a shell sentinel, while a valid selected test still executes. |

The initial packet owns internal/tactile/python/environment.go and planned:internal/tactile/python/pytest_argv_test.go. It must not widen permissions, modify the protected kernel policies, or convert other setup commands into a new model-facing shell surface. Production routing and test-result derivation remain separate root verification obligations; a recording-runtime test alone cannot close the gap.

**PROPOSED UPLIFT — compatibility requirement:** literal execution must preserve the inherited container PATH with the virtualenv bin directory prepended, including when selected tests spawn virtualenv-installed commands. Do not substitute the host PATH or a fixed container PATH. A constant typed Python launcher may establish that environment before invoking pytest, but caller selectors must remain separate literal arguments; no caller-derived shell command is allowed. Root acceptance includes a real subprocess lookup witness in addition to invocation-structure assertions.
