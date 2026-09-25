# jit: wiring and what is NOT built

Re-verified 2026-09-25 (lane A build-out); first written 2026-09-21 against
`3463477`. Read from `internal/jit/config/types.go`,
`internal/prompt/config_factory.go`, `internal/prompt/compiler.go`,
`internal/session/executor.go`, `internal/session/executor_tools.go`,
`internal/session/spawner.go`, `internal/core/policy_inventory.go`, and
`cmd/tools/change_benchmark/main.go`.

## Wired and reachable

- Main turn: `Generate` (`internal/prompt/config_factory.go:96`) merges one
  `ConfigAtom` per intent, falls back to `/general` read-only tools for
  unregistered verbs, stamps prompt and tools, and returns validation failures
  as errors (`:153`). The executor compiles per turn (`compileConfig`,
  `internal/session/executor.go:1401`) and runs the tool loop under the
  result. A config that fails to compile becomes an empty config
  (`executor.go:1073-1076`), which grants no tool.
- **The allowlist is enforced at execution, fail-closed.** Every tool call is
  checked against `AllowedTools` before it runs
  (`internal/session/executor_tools.go:2253`), and a nil or empty allowlist
  allows nothing (`isToolAllowed`, `:2505`). Pinned by
  `TestExecutorToolCapabilityEnvelopeFailsClosed` and
  `TestExecutorExecuteToolCallRequiresEffectiveCapability`
  (`internal/session/executor_capability_test.go:26`, `:61`).
- Precompiled injection wins: a config set through `SetAgentConfig`
  (`executor.go:605`) is preferred by `compileConfig`. Its callers are the
  subagent (`internal/session/subagent.go:297`), which injects the config its
  spawner generated through `Generate` or, on failure, an empty one
  (`spawner.go:367-369`), and the benchmark harness with an inline config
  (`cmd/tools/change_benchmark/main.go:179`).
- The config threads through verification and gate helpers in
  `internal/session/build_verify.go` and `internal/session/change_gates.go`.
- `CompilationResult` carries the config when the compiler has a factory
  (`internal/prompt/compiler.go:871`); `ResolveAllowedTools`
  (`config_factory.go:169`) exposes the same atom merge for agreement before
  selection.

## Exists but unchecked (each declined in this pass, with the reason)

- `GenerateFallback` never calls `Validate` (`config_factory.go:203`). It has
  no production caller: only `internal/prompt/config_factory_test.go` calls
  it. The production fallback for a failed compile is the empty config above,
  which fails closed at `isToolAllowed`. Declined: deleting or reshaping a
  test-only constructor is a maintainer call.
- `Persona` is always `""` on factory-built configs, and nothing reads it
  (`types.go` calls it descriptive metadata). Declined: filling a field no
  consumer reads would be decoration.
- `Policies` are validated as strings, never loaded per agent: `Validate`
  checks each entry against the embedded inventory (`IsDefaultPolicyFile`,
  `internal/core/policy_inventory.go:129`). Every file a policy set can name
  (`DefaultAgentPolicySetFiles`, `:146`) is part of the default policy the
  kernel already loads whole at boot, so the per-agent list is a subset of what
  is live. Declined: loading a per-agent policy set would mean a kernel per
  agent, an architectural decision.
- Validation is caller-invoked, not structural. No production producer skips
  it with a non-empty config: `Generate` validates, the spawner and executor
  fall back to the empty config, and the benchmark's inline config is valid.
  Declined: a constructor-only type would touch every producer for no
  behaviour change.

## Assumed by the design, not done by the code

- The struct carries no model, workspace, loop-limit, or safety fields
  (`internal/jit/config/types.go:14-20`). Loop budgets live on the session
  `ExecutorConfig`; anything reading per-agent model or workspace settings off
  this type is reading fields that do not exist.
- Nothing imports bare `internal/jit`: it is a directory, not a package. The
  import path is `codenerd/internal/jit/config`.
- The 2026-09-21 note "the fail-closed allowlist holds only if execution
  consults `AllowedTools`" is settled: execution does (above).
