---
doc-class: governance
subsystem: features
implementation-status: not-applicable
last-verified: 2026-09-21
verified-against: 34634770970153e78c1e250fdab7abd888dcce6f
supersedes: []
---

# features: principles and constraints

Any change to `internal/features` must respect these principles. Each states
the rule, the code it comes from, and the ruling a reviewer enforces.

## P1 — Leaf package: depend on nothing internal

`internal/features` exists as its own package so low-level subsystems can read
flags without an import cycle with `internal/config`
(`internal/features/features.go:1-13`, layering rule at
`internal/features/features.go:7-11`). The package imports only stdlib
(`internal/features/features.go:57-62`: `fmt`, `os`, `strings`,
`sync/atomic`).

**Ruling:** Add nothing here that pulls in another `internal/` package —
including logging (see P8). A new import of any `codenerd/internal/*` path is
a violation.

## P2 — Tables are the single source of truth

`boolFlags` (`internal/features/features.go:276-291`) ties each boolean's
name, canonical env, legacy env, accessor closure, and default together;
`intFlags` (`internal/features/features.go:296-303`) does the same for the two
integer overrides. `Summary`, `Resolved`, `Deprecations`, and the CLI schema
all read these tables so they cannot drift from the accessors. Adding a flag
means three things in lockstep (`internal/features/features.go:46-54`):
field on `FeaturesConfig`, public `IsXXX` helper with env/config/default
precedence, default in `DefaultFeaturesConfig`.

**Ruling:** Never hard-code a flag name, env var, or default outside the
tables. A new toggle without a table row plus accessor plus default is
incomplete.

## P3 — Four-layer precedence, canonical wins

`resolveBool` (`internal/features/features.go:419-434`) decides in order:
canonical env, legacy env, active config, default. The legacy name is read
only when the canonical one is absent or unparseable
(`internal/features/features.go:416-418`), so migrating a shell profile in
either order never double-applies. `Resolved`
(`internal/features/features.go:308-328`) reports the winning source per flag
(`env` / `legacy-env` / `config` / `default`,
`internal/features/features.go:243-257`).

**Ruling:** No accessor may reorder these layers or skip one. Env always beats
config; config always beats default; canonical always beats legacy.

## P4 — Stray exports never flip a bit

`envBool` (`internal/features/features.go:444-458`) trims whitespace and
accepts exactly `1`/`0` and case-insensitive `true`/`false`
(`internal/features/features.go:451-454`); anything else — `yes`, a typo —
returns nil ("no override", `internal/features/features.go:456-457`).
`envInt` applies the same discipline to numbers: only positive digit strings
count (`internal/features/features.go:581-595` via `parseInt64`,
`internal/features/features.go:597-609`).

**Ruling:** Misconfiguration is silent by design. Never "helpfully" coerce an
unrecognized value to true, and never warn on it at this layer (warnings live
in `Deprecations`, P8). `CODENERD_DARK_MODE=yes` is no override — it falls through to config then default.

## P5 — No snapshots: resolve per call under an atomic pointer

`active` (`internal/features/features.go:221`) holds one `*FeaturesConfig`.
`SetActive` (`internal/features/features.go:233-241`) copies the struct
(`internal/features/features.go:238-240`) and swaps the pointer; `nil` resets
to defaults. `Active` (`internal/features/features.go:408`) loads the
pointer, and every accessor dereferences it on every call
(`internal/features/features.go:419-434`, load at line 428) — there is no
snapshot to go stale, so a late install changes what subsequent reads return.

**Ruling:** Callers must not mutate the pointer `Active()` returns, and must
not cache a resolved value across a `SetActive`. Copy-on-install plus
load-per-call is the concurrency contract; do not add a mutex or a cached
copy.

## P6 — Absent is not false for booleans; zero is not a value for integers

Every boolean field is `*bool` so "user wrote `false`" is distinguishable from
"key absent, use the default" (`internal/features/features.go:64-68`). The
integer overrides are zero-valued instead: zero means "call site picks"
(`internal/features/schema.go:38-39`). `FastScanWorkers`
(`internal/features/features.go:557-565`) and `FastASTMaxBytes`
(`internal/features/features.go:568-576`) return env, then active config, then
`0`. The caller owns the default: workers `max(min(NumCPU,20),4)` and a 2 MiB
AST cutoff (`internal/world/scanner_config.go:29-38`; workers at
`internal/world/scanner_config.go:30-33`, cutoff at
`internal/world/scanner_config.go:35-38`).

**Ruling:** Never interpret `0` as a configured worker count or byte limit,
and never change a `*bool` field to a plain `bool` — that collapses the
absent/false distinction the precedence chain depends on.

## P7 — Conservative defaults: dangerous or expensive stays off

`DefaultFeaturesConfig` (`internal/features/features.go:156-168`) is
intentionally conservative — all false except `SystemShards:true` — because
the values govern unit tests and first boot before config is read
(`internal/features/features.go:140-155`). The per-flag reasons are load-bearing:

- Flight recorder drives the execution tracer and can abort the process with
  `throw("traceRegion: out of memory")` under load, so it ships off and opts
  in with a memory watchdog (`internal/features/features.go:460-470`;
  accessor at `internal/features/features.go:471-474`).
- Provenance allocates per-derivation event objects; enable only when
  `/explain` needs proof trees (`internal/features/features.go:476-482`).
- Per-shard facts installs a dispatch-only router with no cross-shard joins,
  so flipping it silently deletes cross-domain derivations
  (`internal/features/features.go:503-512`; audit at
  `internal/features/features.go:175-201`, accessor at
  `internal/features/features.go:509-512`).
- Fast taxonomy skips the scenario sweep entirely; defaulting on would make
  verification skip itself on ordinary runs
  (`internal/features/features.go:527-538`).
- Prompt evolution spends API budget grading past turns in the background
  while recording stays free and `/evolve` stays manual
  (`internal/features/features.go:540-552`).

To turn everything on, flip the file, not this function; tests use
`SetActive(&FullyEnabledFeaturesConfig{})`
(`internal/features/features.go:153-155`).

**Ruling:** A new flag that allocates, traces, spends tokens, or skips
verification defaults off. `FullyEnabledFeaturesConfig`
(`internal/features/features.go:202-215`) keeps `PerShardFacts:false` and
`PromptEvolution:false` for the audited reasons above — flipping either
requires resolving the stated blocker, not optimism.

## P8 — Report, don't log; show resolved, not raw

This package must not import logging, so it returns strings the caller
surfaces (`internal/features/features.go:33-36`; `SetActive` caller-log
contract at `internal/features/features.go:227-232`).
`Deprecations` (`internal/features/features.go:341-363`) returns strings
rather than logging (`internal/features/features.go:334-336`), and a legacy
variable set but shadowed by the canonical one is still reported — it is doing
nothing, which is exactly what the operator needs to hear
(`internal/features/features.go:338-340`). `Summary`
(`internal/features/features.go:374-391`) reports resolved values, not raw
config fields; the prior version printed `*bool` fields and hid env overrides
(`internal/features/features.go:365-373`).

**Ruling:** `SetActive` callers log `Summary()` and warn on each
`Deprecations()` entry. Never log from inside this package, and never render
raw `FeaturesConfig` fields as the effective state.

## P9 — Legacy names are migration debt with a removal criterion

Canonical is `CODENERD_` + upper-cased key; four flags predate the rule and
keep a `NERD_*` legacy spelling that is still read but always loses to the
canonical name and is reported deprecated
(`internal/features/features.go:22-36`). Removal criterion
(`internal/features/features.go:38-44`): legacy names go away once a release
has shipped with `Deprecations()` surfaced at boot and the operator docs
still spelling the old names are fixed — delete `legacyEnvVar` from
`boolFlags`/`intFlags` and the `TestEnvMigration_*` tests fail loudly rather
than behavior changing silently.

**Ruling:** Do not add a new legacy alias, and do not remove an old one until
both halves of the criterion hold. Renames ship canonical-first with the old
name demoted to legacy, never deleted in the same change.

## P10 — Schema is generated, not transcribed

`ConfigSchemaJSON` (`internal/features/schema.go:21-52`) builds the documented
config snippet from `boolFlags`/`intFlags` rather than hand-writing it, so a
doc snippet cannot drift from the accessors
(`internal/features/schema.go:12-16`). The result is deliberately not strict
JSON — it carries `//` comments (`internal/features/schema.go:18-20`).
Machines use `ConfigSchemaKeys` (`internal/features/schema.go:56-65`).

**Ruling:** `nerd features --schema` prints `ConfigSchemaJSON()` and docs
embed that same output. Never hand-edit a config example without regenerating
it from the tables.
