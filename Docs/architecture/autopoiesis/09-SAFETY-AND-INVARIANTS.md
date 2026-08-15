# 09 — Safety and Invariants: Autopoiesis

> Last verified against codebase: **2026-08-15**  
> Package: `internal/autopoiesis`

Autopoiesis is a **high-privilege** subsystem: it writes Go source, compiles binaries, and executes them. Safety is layered. It does **not** replace session-level `permitted(...)` for ordinary agent actions; it gates **self-generated tools**.

## 1. Layered model

```
1. Need throttles (confidence, session cap, cooldown)
2. LLM proposal (untrusted)
3. Optional Mangle sanitizer (embedded logic)
4. SafetyChecker: AST facts + go_safety.mg + allowed pkgs
5. Thunderdome: adversarial inputs / resource bounds
6. Simulation: state transition legality (schemas_state.mg)
7. Compile size/timeout limits
8. Runtime: absolute path, scrubbed env, JSON contract, execute timeout
9. Parent kernel publication (discoverability, not full sandbox)
```

## 2. SafetyChecker invariants

| Invariant | Mechanism |
|-----------|-----------|
| Policy present | Embedded `go_safety.mg` loaded at init; **fail closed** if missing or empty — every `Check` returns a blocking `ViolationPolicy` (`loadGoSafetyPolicy`, `SafetyChecker.policyErr`) |
| Structural visibility | `ExtractASTFacts` emits imports/calls for policy |
| Panic propagation | `propagatePanicCalls` synthetic facts |
| Config-gated packages | `buildAllowedPackages` from AllowNetworking/FS/Exec |
| Structured outcome | `SafetyReport` with severity-ranked violations |
| Feedback loop | `FormatViolationsForFeedback` for regenerate |

**Violation types:** forbidden import, dangerous call, unsafe pointer, reflection, CGO, exec, panic, goroutine
leak, parse error, policy, PanicMaker kill.

`classifyImportViolation` maps a disallowed import onto the specific category (`unsafe` → unsafe pointer,
`reflect` → reflection, `C`/`runtime/cgo` → CGO, `os/exec`/`syscall` → exec) so the text fed back to the LLM
for regeneration names the hazard. `ViolationDangerousCall` is **not currently reachable** — the policy has
no call-level rule (OPEN-QUESTIONS Q11). `checker_failclosed_test.go` holds a golden sample for every
reachable type and an explicit reason for the two that are not.

## 3. Ouroboros governance invariants

| Invariant | Mechanism |
|-----------|-----------|
| Named need required | empty name fails at detection |
| Bounded retries | `RetryConfig.MaxRetries`, `MaxPanicRetries` |
| Bounded iterations | `ExecuteConfig.MaxIters`, Mangle `shouldHalt` |
| Panic isolation | deferred recover → StagePanic / stats |
| Size limit | `MaxToolSize` (default 100KB) |
| Timeouts | compile/execute 300s default |
| Networking default off | `AllowNetworking: false` in orchestrator-built config |
| Exec default off | `AllowExec: false`; opt in per workspace via `Config.AllowToolExec` |
| Single creation path | `ExecuteOuroborosLoop` is the only production route; enforced by `tool_creation_routing_test.go` |

## 4. Thunderdome invariants

| Invariant | Mechanism |
|-----------|-----------|
| Per-attack timeout | `ThunderdomeConfig.Timeout` (5s default) |
| Memory budget | `MaxMemoryMB` (100 default) |
| Isolated work dir | temp `thunderdome` path |
| Kill feedback | battle result formatted as safety-like violation for regen |

PanicMaker categories (config): resource, concurrency, nil/boundary/format via generated vectors.

## 5. Runtime execution invariants

| Invariant | Mechanism |
|-----------|-----------|
| Absolute binary path only | `RuntimeTool.Execute` rejects relative |
| Binary exists | `os.Stat` before exec |
| Structured I/O | JSON marshal/unmarshal; tool error field |
| Env scrubbing | `toolExecutionEnv()` |
| Context cancel | `exec.CommandContext` |
| Execute counter | atomic increment on success path |

## 5b. Execution policy

`OuroborosConfig.ExecutionMode` selects the backend and is the single switch:

| Mode | Isolation | When |
|------|-----------|------|
| `ExecuteCompiled` (default) | separate process, scrubbed env, hard context kill | product default |
| `ExecuteInterpreted` | in-process Yaegi; a timeout abandons the goroutine rather than killing it | no Go toolchain, diagnostics |

`AllowCompilationFallback` lets interpreted mode fall back to the registered binary. The interpreter's
import allowlist is derived from the same `SafetyChecker` (`NewYaegiExecutorForPolicy`) and additionally
strips ambient-authority packages, because it has no process boundary.

## 6. Yaegi path invariants (when used)

| Invariant | Mechanism |
|-----------|-----------|
| Import whitelist | derived from `SafetyChecker.allowedPkgs` — one policy, not two |
| Blocked regardless of config | os, os/exec, net, net/http, net/url, syscall, unsafe, plugin, reflect |
| Entry contract | `RunTool(input string) (string, error)` **or** the pipeline's own `func(context.Context, string) (string, error)` |

## 7. Learning / fact gas invariants

| Invariant | Mechanism |
|-----------|-----------|
| Learning fact bound | `MaxLearningFacts` (default 1000) |
| Retract before reassert | `SyncLearningsToKernel` patterns |
| NaN resistance | e2e + unit tests around scores |
| Empty tool name no-op | `RecordExecution` guards |

## 8. Kernel nil-safety

| Call | Without kernel |
|------|----------------|
| `assertToKernel` | no-op success |
| `ProcessKernelDelegations` | returns 0, nil |
| `SetKernel` then sync | registers restored tools |

Essential failure: Ouroboros **panics** if local Mangle engine cannot start — treat as boot-critical for tool generation.

## 9. Constitutional relationship

- Autopoiesis does not implement `permitted(...)`.  
- Tools it produces become effect channels once registered; **session policy still decides** whether invoking a tool is allowed.  
- Default deny for *agent* actions remains core policy; autopoiesis default deny is for *dangerous generated code*.

## 10. Operator checklist

1. Confirm `go_safety.mg` embeds successfully — a load failure now denies all tool generation, loudly.  
2. Keep `EnableThunderdome` true outside explicit experiments.  
3. Review `.nerd/tools` before sharing workspaces.  
4. Cap `MaxToolsPerSession` in untrusted environments.  
5. Grant `Config.AllowToolExec` only in workspaces you trust to run whatever a model writes.
6. Watch `Orchestrator.ExportMetrics().RejectRate`: a sustained climb means the model has lost the safety
   policy, not that the workspace got harder.
