# 05 — Internal architecture

## Component ownership

```text
internal/config
  aggregate/load       user_config.go (JSON), config.go (legacy YAML)
  effective resolution UserConfig.Get*, Config.Get*, defaults
  domain types         llm, limits, memory, JIT, shard, execution,
                       integrations, world, build, UX, logging
  process bridges      features.SetActive; globalLLMTimeouts
```

`UserConfig` is broad but not self-validating. Optional nested blocks mix
pointers, values, maps and slices. Effective getters sometimes copy a struct but
retain referenced map/slice storage (`GetBuildConfig`, `GetIntegrations`), while
UX getters may return the stored pointer directly. There is no general immutable
snapshot contract.

## JSON lifecycle

```text
LoadUserConfig(path)
  ReadFile
    absent -> &UserConfig{} and return before feature activation
    error  -> typed wrapper error
  decodeStrictJSON (unknown fields and trailing values rejected)
  features.SetActive(cfg.Features)
  log features summary
  return mutable pointer
```

**VERIFIED CURRENT.** Present JSON with no `features` block resets the feature
registry through `SetActive(nil)`; missing JSON preserves the prior registry.
`internal/features/config_roundtrip_test.go#TestLoadUserConfig_InstallsFeaturesIntoRegistry`
documents both branches.

Save marshals then calls `writePrivateFileAtomically`: create directory `0700`,
write/chmod/sync/close a same-directory temporary, rename, then chmod `0600`.
It does not semantically validate, lock/compare an expected snapshot, update
features, reconfigure logging/JIT/scheduler, or create a recovery backup. The
wizard loads and merges before Save; focused tests prove representative nested
field preservation, pre-rename failure preservation, Unix `0600`, and round-trip
reload. They do not prove concurrent-writer conflict handling or Windows ACLs.

## Legacy YAML lifecycle

```text
DefaultConfig -> ReadFile -> yaml.Unmarshal into defaults -> applyEnvOverrides
```

Missing YAML returns defaults plus env. `Load` does not call `Validate` or
`ValidateCoreLimits`. Env keys later in `applyEnvOverrides` overwrite earlier
provider selection, so the actual winner depends on fixed code order when
multiple variables exist.

## Effective resolution

| Resolver | Merge rule | State implication |
|---|---|---|
| `GetActiveProvider` | explicit provider, else priority key search | returns provider plus empty key on explicit mismatch |
| `HasExplicitLLMSelection` | recognizes provider/key/engine-specific blocks | shared boot will not rescue an unusable explicit choice with an ambient backend |
| `GetCoreLimits` | replace zero fields with JSON defaults | negative/out-of-range values survive |
| `GetEffectiveMaxConcurrentAPICalls` | min of positive engine cap and core | no full lower/upper validation |
| `GetEffectiveAPISchedulerPolicy` | engine defaults then optional pointer overrides | reads process-global slot timeout |
| `GetContextWindowConfig` | field-by-field zero defaults | no reserve-sum/range validation |
| `GetEffectiveJITConfig` | default/override then clamp budget/reserve | trace false can be explicit only when JIT block exists |
| `GetExecution` | defaults binaries/env/directory/timeout | shared Cortex projects all fields; campaigns omit timeout and containment; dormant legacy boot uses defaults |
| `GetIntegrations` | returns stored struct/map or empty map | URL/protocol/timeout validation deferred |

## State and concurrency

| State | Scope/lifetime | Synchronization |
|---|---|---|
| `*UserConfig` | caller/workspace by convention | none; mutable maps/pointers may alias getters |
| active features | process global | atomic ownership in `internal/features`; missing-file preservation |
| `globalLLMTimeouts` | process global | none around Set/Get |
| logging config | process global, first workspace wins | `sync.Once` initialization; no reload |
| scheduler | process global consumer projection | configured outside package; snapshot identity absent |

## Error and recovery model

The loader distinguishes missing/read/parse errors. Shared Cortex boot now
propagates a present-invalid file before perception and does not ambient-fallback
an explicit unusable LLM choice; secondary consumers still soften some errors.
No config error has a stable class, field path, schema version, source provenance,
or redaction metadata. Persistence has no backup/version rollback, so recovery
still depends on operator inspection and restart rather than a package transaction.
