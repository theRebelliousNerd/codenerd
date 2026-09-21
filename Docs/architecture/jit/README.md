# internal/jit

The typed handoff between JIT config production and the session tool loop:
one struct plus its validation. `internal/jit/` holds no Go source of its
own — there are no `.go` files at that level — so the only package is
`internal/jit/config` (`package config`,
`internal/jit/config/types.go:1`): two files, 145 lines.

Verified 2026-09-21 against commit `3463477` (`main`). All line references
are to `internal/jit/config/types.go` or
`internal/jit/config/types_test.go` unless another path is given.

## What it is

`EffectiveAgentRuntimeConfig` (`types.go:14-20`) carries five fields from the
producer to the executor: `IdentityPrompt`, `IntentVerb`, `Persona`,
`AllowedTools`, and `Policies`. YAML and JSON tags are snake_case so
specialist files at `.nerd/agents/<name>/config.yaml` use the natural
convention (`types.go:12-13`). The struct owns no I/O, loads no policy, and
enforces no tool gate; it is data plus `Validate`.

## Public API

| Symbol | Where | Notes |
|---|---|---|
| `EffectiveAgentRuntimeConfig` | `types.go:14-20` | `IdentityPrompt`, `IntentVerb`, `Persona`, `AllowedTools`, `Policies` |
| `Validate` | `types.go:34-52` | non-blank identity; at least one canonical policy; no duplicates; tools and persona unchecked |
| `TestAgentConfigValidation` | `types_test.go:7-91` | 8-case table pinning every rule |

## Who produces it, who consumes it

- `ConfigFactory.Generate` (`internal/prompt/config_factory.go:96-160`)
  merges config atoms per intent (falling back to `/general`), stamps the
  compiled prompt as identity, and returns validation failures as errors.
- `ConfigFactory.GenerateFallback`
  (`internal/prompt/config_factory.go:203-232`) builds the degraded config
  when compilation fails; it never validates.
- The executor carries it on `ExecutorConfig`
  (`internal/session/executor.go:189-190`), accepts precompiled injection via
  `SetAgentConfig` (`internal/session/executor.go:617-620`), compiles per turn
  (`compileConfig`, `internal/session/executor.go:1397`), and runs the tool
  loop under it (`internal/session/executor.go:1089`).
- The change-benchmark harness injects one inline
  (`cmd/tools/change_benchmark/main.go:175`).

## Further reading

- `INTERNALS.md` — the struct fields, the `Validate` rules, and the test pins.
- `WIRING-AND-NOT-BUILT.md` — what is reachable, what is unchecked, and what
  the design assumes that the code does not do.
