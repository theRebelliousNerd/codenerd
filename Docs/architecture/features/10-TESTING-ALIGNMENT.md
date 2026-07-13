# 10 — Testing Alignment: features

> Last verified against codebase: **2026-07-13**

## 1. Commands

```powershell
# Package unit + external boundary tests
go test ./internal/features/...

# Kernel integration with DiffEval gate
go test ./internal/core/ -run Features

# Optional verbose
go test ./internal/features/... -v -count=1
```

No CGO required for `internal/features` itself.

## 2. Test inventory

| File | Package | Focus |
|------|---------|-------|
| `features_test.go` | `features` | Precedence, PerShardFacts, SystemShards legacy, SetActive copy, numerics |
| `features_defaults_test.go` | `features` | Default on/off matrix, parseInt64, err string |
| `config_roundtrip_test.go` | `features_test` | LoadUserConfig install, absent features, missing file preserve, FullyEnabled, numeric env |

### Downstream tests (not in package, but contract-critical)

| File | Focus |
|------|-------|
| `internal/core/kernel_features_test.go` | Kernel routes through DiffEval gate |

## 3. Contracts locked by tests

| Contract | Test |
|----------|------|
| env=1 wins over active=false | `TestResolveBoolPrecedence/env_wins_over_active` |
| active=false wins over default true paths | `.../active_wins_over_default` |
| DiffEval default false when clean | `.../default_kicks_in...` |
| Invalid env (`yes`) falls through | `.../invalid_env_value...` |
| env=0 forces off over active true | `.../env_0_forces_off...` |
| PerShardFacts default off; active true; env 0 | `TestPerShardFactsPrecedence` |
| Legacy `NERD_DISABLE_SYSTEM_SHARDS` ≠ master | `TestSystemShardsLegacyEnvIgnored` |
| SetActive snapshots | `TestSetActiveCopySemantics` |
| Numeric env > config > 0 | `TestNumericAccessors` / `TestNumericOverrides` |
| DefaultFeaturesConfig matrix | `TestDefaultFeaturesConfig` |
| parseInt64 reject set | `TestParseInt64` |
| LoadUserConfig installs flags | `TestLoadUserConfig_InstallsFeaturesIntoRegistry` |
| Missing file preserves registry | `.../nonexistent_file_preserves_active_registry` |
| Env overrides after load | `TestEnvOverridesActiveConfig` |
| FullyEnabled round-trip (PerShardFacts false) | `TestFullyEnabledConfigRoundTrip` |

## 4. Coverage posture

| Area | Assessment |
|------|------------|
| resolveBool truth table | **Strong** |
| Default factories | **Strong** for bools; numerics zero-only |
| SetActive nil/copy | **Strong** |
| Config boundary | **Strong** |
| Concurrent SetActive/Is* | **Absent** (rely on atomic correctness) |
| Summary() string format | **Absent** |
| Every env name listed once | Implicit via accessor tests |
| DarkMode / Onboarding / Flight / Provenance accessors unit tests | **Mostly via round-trip FullyEnabled**, not individual precedence suites |
| TaxonomyFast tool integration | **Not tested against features accessor** |

## 5. Gaps (testing)

| Gap | Severity | Note |
|-----|----------|------|
| No race detector-focused concurrent test | Low | Atomic is simple |
| Summary prints `*bool` with `%v` (addresses) untested | Medium | Operability |
| Comment says PerShardFacts short-circuit; test name/comment may mislead | Low | Behavior tested correctly for FullyEnabled false |
| No table-driven test for all eight env var names | Low | Could prevent rename drift |

## 6. Recommended test additions (backlog only — docs do not implement)

1. Table test: for each accessor, env true/false/garbage/empty × active nil/true/false.  
2. Assert Summary contains readable true/false (if Summary fixed).  
3. External test that `IsTaxonomyFastEnabled` matches tool behavior after SetActive — once tool is wired.  
4. `-race` short concurrent SetActive/IsDiffEvalEnabled stress.

## 7. Alignment with package principles

Tests correctly prefer **public accessors** over raw field inspection for behavioral contracts (especially config_roundtrip). This matches principle P12 in [04-ARCHITECTURAL-PRINCIPLES.md](04-ARCHITECTURAL-PRINCIPLES.md).
