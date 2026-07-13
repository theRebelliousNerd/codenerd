# 06 — Public API and Types: features

> Last verified against codebase: **2026-07-13**  
> Source: `internal/features/features.go`

## 1. `FeaturesConfig`

On-disk / in-memory toggle block embedded by `config.UserConfig` as `Features *features.FeaturesConfig` with JSON key `features`.

| Field | Go type | JSON | Env | Notes |
|-------|---------|------|-----|-------|
| DiffEval | `*bool` | `diff_eval` | `CODENERD_DIFF_EVAL` | DifferentialEngine |
| FlightRecorder | `*bool` | `flight_recorder` | `NERD_FLIGHTREC` | Trace ring buffer |
| Provenance | `*bool` | `provenance` | `CODENERD_PROVENANCE` | DerivationRecorder |
| SystemShards | `*bool` | `system_shards` | `CODENERD_SYSTEM_SHARDS` | Master Type-1 switch |
| PerShardFacts | `*bool` | `per_shard_facts` | `CODENERD_PER_SHARD_FACTS` | ShardFactRouter |
| DarkMode | `*bool` | `dark_mode` | `CODENERD_DARK_MODE` | Force dark theme |
| SkipOnboarding | `*bool` | `skip_onboarding` | `NERD_SKIP_ONBOARDING` | Skip UX wizard |
| TaxonomyFast | `*bool` | `taxonomy_fast` | `CODENERD_TAXONOMY_FAST` | Tool fast path |
| FastScanWorkers | `int` | `fast_scan_workers` | `NERD_FAST_SCAN_WORKERS` | 0 = unset |
| FastASTMaxBytes | `int64` | `fast_ast_max_bytes` | `NERD_FAST_AST_MAX_BYTES` | 0 = unset |

All fields use `omitempty` on JSON tags.

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

- No active: `"features: defaults active"`  
- Active: sprintf of all fields (pointer values printed as Go pointer format for bools — note: formats `*bool` as addresses/values depending on fmt; string includes workers and max bytes)

Consumers: Boot log in `LoadUserConfig`.

## 4. Boolean accessors

Each returns `bool` via `resolveBool(env, fieldGetter, default)`.

| Function | Env | Default arg |
|----------|-----|-------------|
| `IsDiffEvalEnabled` | `CODENERD_DIFF_EVAL` | false |
| `IsFlightRecorderEnabled` | `NERD_FLIGHTREC` | true |
| `IsProvenanceEnabled` | `CODENERD_PROVENANCE` | false |
| `IsSystemShardsEnabled` | `CODENERD_SYSTEM_SHARDS` | true |
| `IsPerShardFactsEnabled` | `CODENERD_PER_SHARD_FACTS` | false |
| `IsDarkModeEnabled` | `CODENERD_DARK_MODE` | false |
| `IsOnboardingSkipped` | `NERD_SKIP_ONBOARDING` | false |
| `IsTaxonomyFastEnabled` | `CODENERD_TAXONOMY_FAST` | true |

Accepted env truthy/falsey sets are fixed in `resolveBool` (not locale-aware beyond listed casings).

## 5. Numeric accessors

| Function | Return | Env | Active field | Zero meaning |
|----------|--------|-----|--------------|--------------|
| `FastScanWorkers` | `int` | `NERD_FAST_SCAN_WORKERS` | `FastScanWorkers` | use local default |
| `FastASTMaxBytes` | `int64` | `NERD_FAST_AST_MAX_BYTES` | `FastASTMaxBytes` | use local default |

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
| `resolveBool` | Shared bool resolver |
| `parseUint` / `parseInt64` | Strict positive digit parsers |
| `featuresErr` | Error type for bad int |
| `errBadInt` | Singleton parse error |
| `active` | Atomic registry |

## 8. Stability notes

- JSON keys are a **compat surface** for user configs.  
- Env var names are a **compat surface** for CI.  
- Renaming either requires migration docs and dual-read if users exist.  
- Accessor names are the stable Go API for internal packages.  
