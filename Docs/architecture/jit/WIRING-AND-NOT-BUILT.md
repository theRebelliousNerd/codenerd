# jit: wiring and what is NOT built

Verified 2026-09-21 against commit `3463477` (`main`). Read from
`internal/jit/config/types.go`, `internal/prompt/config_factory.go`,
`internal/prompt/compiler.go`, `internal/session/executor.go`,
`internal/session/build_verify.go`, `internal/session/change_gates.go`, and
`cmd/tools/change_benchmark/main.go`.

## Wired and reachable

- Main turn: `Generate` merges one `ConfigAtom` per intent and falls back to
  `/general` read-only tools for unregistered verbs
  (`internal/prompt/config_factory.go:109-136`), stamps prompt and tools
  (`:141-146`), and returns validation failures as errors (`:153-157`). The
  executor compiles per turn (`internal/session/executor.go:1069` via
  `compileConfig`, `:1397`) and runs the tool loop under the result
  (`:1089`), converting names to definitions in `buildToolDefinitions`
  (`:1831-1832`).
- Precompiled injection wins: a config set through `SetAgentConfig`
  (`internal/session/executor.go:617-620`) is carried on `ExecutorConfig`
  (`:189-190`) and preferred by `compileConfig` (`:1352`, `:1399-1400`). The
  benchmark harness uses exactly this path with an inline config
  (`cmd/tools/change_benchmark/main.go:175`).
- The config threads through verification and gate helpers in
  `internal/session/build_verify.go` (`:215`, `:296`, `:552`, `:649`) and
  `internal/session/change_gates.go` (`:209`, `:329`, `:426`).
- `CompilationResult` can carry the config
  (`internal/prompt/compiler.go:289`); `ResolveAllowedTools`
  (`internal/prompt/config_factory.go:169-200`) exposes the same atom merge
  for agreement before selection.

## Exists but unchecked

- `GenerateFallback` never calls `Validate`
  (`internal/prompt/config_factory.go:203-232`): a truncated identity, an
  unknown intent with no `/general` atom, or empty policies produce a config
  the main path would reject as an error.
- `Persona` is always `""` on factory-built configs: neither construction
  site sets it (`internal/prompt/config_factory.go:141-146`, `:226-231`),
  and the type only calls it descriptive metadata (`types.go:31-33`).
- `Policies` are validated as strings, never loaded: `IsDefaultPolicyFile`
  proves membership in the embedded inventory
  (`internal/core/policy_inventory.go:137-141`); no per-agent policy set is
  loaded and no set identity or version is recorded.
- Validation is caller-invoked, not structural: the type cannot stop a
  producer from skipping it, and one does.

## Assumed by the design, not done by the code

- The fail-closed allowlist (`internal/jit/config/types.go:31-33`) holds only
  if execution consults `AllowedTools`. The struct states the contract; the
  executor keeps it.
- The struct carries no model, workspace, loop-limit, or safety fields
  (`internal/jit/config/types.go:14-20`). Loop budgets live on the session
  `ExecutorConfig` (`internal/prompt/config_factory.go:148-152`); anything
  reading per-agent model or workspace settings off this type is reading
  fields that do not exist.
- Nothing imports bare `internal/jit`: it is a directory, not a package. The
  import path is `codenerd/internal/jit/config`.
