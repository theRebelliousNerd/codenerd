---
doc-class: shipped
subsystem: features
implementation-status: shipped
last-verified: 2026-09-26
verified-against: 34634770970153e78c1e250fdab7abd888dcce6f
supersedes: []
---

# features: internals

Verified 2026-09-21 against commit `34634770970153e78c1e250fdab7abd888dcce6f` (`main`). Read from `internal/features/features.go`,
`internal/config/user_config.go:525-593`, `cmd/nerd/cmd_features.go`, and
`internal/world/scanner_config.go:29-38`.

One question: how does a flag get its value?

## The registry is a pointer, not a copy

`SetActive` swaps one `*FeaturesConfig` under an atomic
(`internal/features/features.go:233-241`); `Active()` loads it
(`internal/features/features.go:408`). Every accessor dereferences the
pointer on every call (`resolveBool`, `internal/features/features.go:419-434`,
line 428), so there is no snapshot to go stale: installing config late changes
what subsequent reads return. `SetActive(nil)` resets the registry and every
accessor falls back to its compile-time default. The reason the registry
exists at all is import direction — leaf packages (`internal/core`,
`internal/observability`, `internal/world`) cannot import `internal/config`,
so config installs once and leaves read forever
(`internal/config/user_config.go:557-561`).

## Booleans: four layers, strict parsing

`resolveBool(envVar, legacyEnvVar, fromActive, def)` decides in order:
canonical env, legacy env, active config, default. The legacy name is read
only when the canonical one is absent or unparseable, so renaming a shell
export in either order never double-applies
(`internal/features/features.go:410-434`). `envBool`
(`internal/features/features.go:444-458`) trims whitespace and accepts exactly
`1`/`0` and case-insensitive `true`/`false`; empty, `yes`, and typos return
nil ("no override"). A stray export can never flip a bit — it can only fail
to be an override, which is the silent-misconfiguration trade the package
chooses deliberately.

## Integers: same shape, zero means "you decide"

`FastScanWorkers` and `FastASTMaxBytes` read the `intFlags` table's env pair
first, then the active config, then return 0
(`internal/features/features.go:557-576`). Zero is not a value; it tells the
call site to pick its default — `max(min(NumCPU,20),4)` workers and a 2 MiB
AST cutoff (`internal/world/scanner_config.go:29-38`). `envInt` applies the
`envBool` discipline to numbers: only positive digit strings count
(`internal/features/features.go:581-609`).

## Inspectors: one registry, three views

- `Flag` (`internal/features/features.go:261-268`) is the resolved record:
  name, value, source (`env` / `legacy-env` / `config` / `default`), default,
  env names.
- `Resolved()` (`internal/features/features.go:308-328`) returns one `Flag`
  per boolean — the same slice both `nerd features` and the chat report render.
- `Deprecations()` (`internal/features/features.go:341-363`) names legacy
  `NERD_*` variables that are set but shadowed, i.e. the operator debugging
  the wrong knob. Both surfaces print these last for that reason
  (`cmd/nerd/cmd_features.go:75-83`).
- `Summary()` (`internal/features/features.go:374-391`) is the one-line boot
  log emitted after install (`internal/config/user_config.go:565`).
- `ConfigSchemaJSON()` feeds `nerd features --schema`
  (`cmd/nerd/cmd_features.go:31-32,37,90-91`): the documented config snippet
  is generated from the registry, not transcribed, so it cannot drift.

## Constructors (test and audit seeds)

`DefaultFeaturesConfig` (`internal/features/features.go:156-168`) builds the
all-defaults struct; `FullyEnabledFeaturesConfig`
(`internal/features/features.go:202-215`) builds the audit seed. Neither was
observed called from production code this pass — see WIRING-AND-NOT-BUILT.md.
