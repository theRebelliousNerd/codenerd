# 06 — Public API and Types: features

> Last verified against codebase: **2026-08-15**  
> Source: `internal/features/features.go`

## 1. `FeaturesConfig`

On-disk / in-memory toggle block embedded by `config.UserConfig` as `Features *features.FeaturesConfig` with JSON key `features`.

| Field | Go type | JSON | Env | Notes |
|-------|---------|------|-----|-------|
| Field | Go type | JSON | Canonical env | Legacy env (dual-read) | Notes |
|-------|---------|------|---------------|------------------------|-------|
| DiffEval | `*bool` | `diff_eval` | `CODENERD_DIFF_EVAL` | — | DifferentialEngine |
| FlightRecorder | `*bool` | `flight_recorder` | `CODENERD_FLIGHT_RECORDER` | `NERD_FLIGHTREC` | Trace ring buffer |
| Provenance | `*bool` | `provenance` | `CODENERD_PROVENANCE` | — | DerivationRecorder |
| SystemShards | `*bool` | `system_shards` | `CODENERD_SYSTEM_SHARDS` | — | Master Type-1 switch |
| PerShardFacts | `*bool` | `per_shard_facts` | `CODENERD_PER_SHARD_FACTS` | — | ShardFactRouter — see §9 |
| DarkMode | `*bool` | `dark_mode` | `CODENERD_DARK_MODE` | — | Force dark theme |
| SkipOnboarding | `*bool` | `skip_onboarding` | `CODENERD_SKIP_ONBOARDING` | `NERD_SKIP_ONBOARDING` | Skip UX wizard |
| TaxonomyFast | `*bool` | `taxonomy_fast` | `CODENERD_TAXONOMY_FAST` | — | Tool fast path |
| FastScanWorkers | `int` | `fast_scan_workers` | `CODENERD_FAST_SCAN_WORKERS` | `NERD_FAST_SCAN_WORKERS` | 0 = unset |
| FastASTMaxBytes | `int64` | `fast_ast_max_bytes` | `CODENERD_FAST_AST_MAX_BYTES` | `NERD_FAST_AST_MAX_BYTES` | 0 = unset |

All fields use `omitempty` on JSON tags.

### 1.1 JSON schema snippet (`features` block)

This is the user-facing reference for `.nerd/config.json`. It is **generated**,
not transcribed: `nerd features --schema` prints exactly `ConfigSchemaJSON()`,
which is built from the same tables the accessors read, so a new flag that is
undocumented fails `TestConfigSchemaJSON_ShouldListEveryRecognisedKey` rather
than shipping.

```jsonc
{
  // .nerd/config.json — every key below is optional.
  // Omitting a key uses the default shown; precedence is
  // canonical env > legacy env > this block > default.
  "features": {
    "diff_eval":           false,  // env: CODENERD_DIFF_EVAL
    "flight_recorder":     false,  // env: CODENERD_FLIGHT_RECORDER (legacy: NERD_FLIGHTREC)
    "provenance":          false,  // env: CODENERD_PROVENANCE
    "system_shards":       true,   // env: CODENERD_SYSTEM_SHARDS
    "per_shard_facts":     false,  // env: CODENERD_PER_SHARD_FACTS
    "dark_mode":           false,  // env: CODENERD_DARK_MODE
    "skip_onboarding":     false,  // env: CODENERD_SKIP_ONBOARDING (legacy: NERD_SKIP_ONBOARDING)
    "taxonomy_fast":       false,  // env: CODENERD_TAXONOMY_FAST
    "fast_scan_workers":   0,      // env: CODENERD_FAST_SCAN_WORKERS (legacy: NERD_FAST_SCAN_WORKERS); 0 = call site default
    "fast_ast_max_bytes":  0       // env: CODENERD_FAST_AST_MAX_BYTES (legacy: NERD_FAST_AST_MAX_BYTES); 0 = call site default
  }
}
```

Precedence for every key: **canonical env → legacy env → this block → default.**
An absent key is not the same as `false`; every boolean is a `*bool` so
"the user wrote false" and "the key is missing" stay distinguishable.

## 2. Factory functions

### `DefaultFeaturesConfig() FeaturesConfig`

Returns conservative defaults (see [02-CURRENT-STATE.md](02-CURRENT-STATE.md) matrix). Used when no active config and no env.

### `FullyEnabledFeaturesConfig() FeaturesConfig`

Returns modern-path seed for init/wizard. **PerShardFacts remains false.** Used by `config.DefaultUserConfig()`.

## 3. Registry functions

### `SetActive(cfg *FeaturesConfig)`

- `nil` → clear registry (defaults).  
- non-nil → **copy** and store.  
Idempotent for equal values; last write wins.

### `Active() *FeaturesConfig`

Wait-free load. May be nil. **Do not mutate** returned pointer.

### `Summary() string`

One line of **resolved** values with their source, e.g.
`features: diff_eval=false flight_recorder=true(env) … fast_scan_workers=0`.
Flags resolved from a non-default source carry the source in parentheses, so a
scan of the line shows what was deliberately changed. It no longer prints raw
`*bool` fields (a key absent from config.json used to log as "unset" even while
an env var was forcing it on) and no longer collapses to "defaults active".

Consumers: Boot log in `LoadUserConfig`, `nerd features`, `/features`.

### `Resolved() []Flag`

Every boolean toggle as the accessors actually resolve it, with `Source` one of
`env`, `legacy-env`, `config`, `default`, plus `EnvVar`, `LegacyEnvVar`,
`Default`. This is the operator-facing view; `Summary` is derived from it.

### `Deprecations() []string`

Legacy `NERD_*` variables currently set, each paired with its canonical
replacement. A legacy variable that is set but **shadowed** by the canonical one
is still reported as ignored — that is the case most likely to send someone
debugging the wrong knob. Returns strings rather than logging because
`features` is a leaf package and must not import `internal/logging`.

### `ConfigSchemaJSON() string` / `ConfigSchemaKeys() []string`

The generated user-facing schema for the `features` block (§1.1) and its key
list. `nerd features --schema` prints the former verbatim.

## 4. Boolean accessors

Each returns `bool` via `resolveBool(canonicalEnv, legacyEnv, fieldGetter, default)`.

| Function | Canonical env | Legacy env | Default arg |
|----------|---------------|------------|-------------|
| `IsDiffEvalEnabled` | `CODENERD_DIFF_EVAL` | — | false |
| `IsFlightRecorderEnabled` | `CODENERD_FLIGHT_RECORDER` | `NERD_FLIGHTREC` | false |
| `IsProvenanceEnabled` | `CODENERD_PROVENANCE` | — | false |
| `IsSystemShardsEnabled` | `CODENERD_SYSTEM_SHARDS` | — | true |
| `IsPerShardFactsEnabled` | `CODENERD_PER_SHARD_FACTS` | — | false |
| `IsDarkModeEnabled` | `CODENERD_DARK_MODE` | — | false |
| `IsOnboardingSkipped` | `CODENERD_SKIP_ONBOARDING` | `NERD_SKIP_ONBOARDING` | false |
| `IsTaxonomyFastEnabled` | `CODENERD_TAXONOMY_FAST` | — | false |

Accepted env truthy/falsey sets are fixed in `resolveBool` (not locale-aware beyond listed casings).

## 5. Numeric accessors

| Function | Return | Env | Active field | Zero meaning |
|----------|--------|-----|--------------|--------------|
| `FastScanWorkers` | `int` | `CODENERD_FAST_SCAN_WORKERS` (legacy `NERD_FAST_SCAN_WORKERS`) | `FastScanWorkers` | use local default |
| `FastASTMaxBytes` | `int64` | `CODENERD_FAST_AST_MAX_BYTES` (legacy `NERD_FAST_AST_MAX_BYTES`) | `FastASTMaxBytes` | use local default |

Parse rules: digits only; must be > 0; no sign, no spaces, no decimals.

## 6. Example usage patterns

### Production consumer (hot path)

```go
if features.IsDiffEvalEnabled() {
    // differential path
}
```

### Test setup

```go
t1 := true
features.SetActive(&features.FeaturesConfig{DiffEval: &t1})
t.Cleanup(func() { features.SetActive(nil) })
t.Setenv("CODENERD_DIFF_EVAL", "") // clear env pollution
```

### Config boundary (already implemented)

```go
features.SetActive(cfg.Features)
logging.Get(logging.CategoryBoot).Info("%s", features.Summary())
```

## 7. Non-exported API (do not re-export casually)

| Symbol | Role |
|--------|------|
| `resolveBool` | Shared bool resolver (canonical env → legacy env → active → default) |
| `envBool` / `envInt` | Env readers; unrecognised values are *not* overrides |
| `boolFlags` / `intFlags` | Single source of truth for name, env vars, accessor, default |
| `parseInt64` | Strict positive digit parser |
| `featuresErr` | Error type for bad int |
| `errBadInt` | Singleton parse error |
| `active` | Atomic registry |

## 8. Stability notes

- JSON keys are a **compat surface** for user configs.  
- Env var names are a **compat surface** for CI.  
- Renaming either requires migration docs and dual-read if users exist.  
- Accessor names are the stable Go API for internal packages.

## 9. `NERD_*` → `CODENERD_*` migration status

Four flags predate the `CODENERD_` convention. They are **dual-read**: the
legacy name still works, the canonical name wins when both are set, and
`Deprecations()` reports the legacy one either way.

| Legacy | Canonical | Flag |
|--------|-----------|------|
| `NERD_FLIGHTREC` | `CODENERD_FLIGHT_RECORDER` | `flight_recorder` |
| `NERD_SKIP_ONBOARDING` | `CODENERD_SKIP_ONBOARDING` | `skip_onboarding` |
| `NERD_FAST_SCAN_WORKERS` | `CODENERD_FAST_SCAN_WORKERS` | `fast_scan_workers` |
| `NERD_FAST_AST_MAX_BYTES` | `CODENERD_FAST_AST_MAX_BYTES` | `fast_ast_max_bytes` |

Removal criterion (also recorded in the package doc, so it does not become
permanent): drop the legacy names once a release has shipped with
`Deprecations()` surfaced at boot **and** the operator-facing docs under
`Docs/architecture/observability/` and `Docs/architecture/ux/` — which still
instruct operators to set `NERD_FLIGHTREC` / `NERD_SKIP_ONBOARDING` — have been
updated. Deleting `legacyEnvVar` from the tables then fails
`TestEnvMigration_LegacyVarsShouldBeTheKnownFour` loudly instead of changing
behavior silently.

`NERD_DISABLE_SYSTEM_SHARDS` is **out of scope**: it is parsed at its call site
as a comma-separated list of shard names and is not part of this registry.

## 10. `PerShardFacts` — why it is still opt-in

Audited 2026-08-15. The blocker the old comment named is gone; a different one
remains, so the flag does not flip.

- **Resolved:** the shard predicate manifest *is* auto-wired.
  `internal/system/factory.go`'s `defaultKernelShardConfigs` builds every
  `core.KernelShardConfig` from `shards.DefaultShardPredicateManifests`, and
  `CortexKernel.RegisterShard` installs the router and registers each shard's
  owned predicates with it.
- **Still open:** `ShardFactRouter` is a dispatch table, not a join coordinator.
  `AssertVia` / `QueryVia` / `RetractVia` route **one** predicate to its owning
  shard, but rule evaluation happens inside a shard's own `*RealKernel` over
  that shard's local facts. A rule body spanning predicates owned by two shards
  therefore derives nothing — a routing-shard rule joining `user_intent`
  (owner: `routing`) with `project_profile` (owner: `world`) sees an empty
  `project_profile`. Nothing implements distributed evaluation; the only
  occurrences of "cross-shard" in the tree are comments.

Turning the flag on would silently delete every cross-domain derivation rather
than failing loudly, which is the worst available failure mode for an executive
kernel. `FullyEnabledFeaturesConfig().PerShardFacts` stays `false`, pinned by
`TestPerShardFacts_ShouldRemainOptInEvenWhenFullyEnabled`. An explicit opt-in is
still honoured — the accessor is ordinary `resolveBool`, not a short-circuit —
and that is pinned too.  
