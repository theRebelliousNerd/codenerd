# 02 — Current State: features

> Precise inventory as of **2026-07-13**. Paths relative to repo root.

## 1. Package facts

| Property | Value |
|----------|-------|
| Module path | `codenerd/internal/features` |
| Non-test Go files | **1** (`features.go`) |
| Non-test lines | **351** |
| Test files | **3** |
| Mangle sources | **0** |
| Internal imports | **none** |
| Stdlib imports | `fmt`, `os`, `sync/atomic` |
| External test imports | `testify`, `internal/config` (roundtrip only) |

## 2. File inventory

| Path | ≈Lines | Role |
|------|-------:|------|
| `internal/features/features.go` | 351 | Full implementation |
| `internal/features/features_test.go` | 154 | Precedence and accessor contracts |
| `internal/features/features_defaults_test.go` | 43 | Defaults + parseInt64 + error text |
| `internal/features/config_roundtrip_test.go` | 193 | Cross-package LoadUserConfig integration |

**Total package ≈ 741 lines** including tests.

## 3. Exported surface (complete)

### Types

| Name | Location |
|------|----------|
| `FeaturesConfig` | `features.go` ~L44–90 |

### Functions

| Name | Location | Role |
|------|----------|------|
| `DefaultFeaturesConfig` | ~L107 | Conservative defaults |
| `FullyEnabledFeaturesConfig` | ~L131 | Init/wizard seed |
| `SetActive` | ~L162 | Install/clear atomic registry |
| `Summary` | ~L175 | Boot description string |
| `Active` | ~L193 | Read active pointer |
| `IsDiffEvalEnabled` | ~L229 | Diff eval gate |
| `IsFlightRecorderEnabled` | ~L237 | Flight recorder gate |
| `IsProvenanceEnabled` | ~L245 | Provenance gate |
| `IsSystemShardsEnabled` | ~L256 | System shards master |
| `IsPerShardFactsEnabled` | ~L266 | Shard fact router gate |
| `IsDarkModeEnabled` | ~L273 | Dark palette force |
| `IsOnboardingSkipped` | ~L279 | Skip wizard |
| `IsTaxonomyFastEnabled` | ~L285 | Taxonomy tool fast path |
| `FastScanWorkers` | ~L292 | Scan concurrency override |
| `FastASTMaxBytes` | ~L305 | AST size cutoff override |

### Unexported

`resolveBool`, `parseUint`, `parseInt64`, `featuresErr`, `newErr`, `errBadInt`, package var `active`.

## 4. Reverse dependency inventory (importers)

| Importer | Usage |
|----------|--------|
| `internal/config/user_config.go` | Embeds `FeaturesConfig`; `SetActive` + `Summary` on load; FullyEnabled in DefaultUserConfig |
| `internal/core/kernel_eval.go` | `IsDiffEvalEnabled` |
| `internal/core/cortex_kernel.go` | `IsPerShardFactsEnabled` for `ShardFactRouter` |
| `internal/core/kernel_features_test.go` | DiffEval integration tests |
| `internal/world/scanner_config.go` | FastScanWorkers / FastASTMaxBytes |
| `internal/ux/migration.go` | `IsOnboardingSkipped` |
| `cmd/nerd/main.go` | `IsFlightRecorderEnabled` |
| `cmd/nerd/chat/session_boot.go` | Provenance + SystemShards |
| `cmd/nerd/ui/styles.go` | `IsDarkModeEnabled` |

Comments-only references: `internal/shards/registration.go` (Track D note).

**Not an importer of features:** `cmd/tools/verify_taxonomy` (reads env directly).

## 5. Default matrices (from tests)

### DefaultFeaturesConfig booleans

| Flag | Default |
|------|---------|
| FlightRecorder | true |
| SystemShards | true |
| TaxonomyFast | true |
| DiffEval | false |
| Provenance | false |
| PerShardFacts | false |
| DarkMode | false |
| SkipOnboarding | false |

### Numeric zero-meaning

| Accessor | Zero means |
|----------|------------|
| `FastScanWorkers()` | world uses `max(min(NumCPU,20),4)` |
| `FastASTMaxBytes()` | world uses 2 MiB |

## 6. Hotspots

| Hotspot | Why |
|---------|-----|
| `resolveBool` | Single precedence authority for all bools |
| `SetActive` copy | Prevents external mutation races |
| `LoadUserConfig` install | Only production writer of active (besides tests/wizard paths) |
| DiffEval default OFF | Test wall-time and eval semantics |
| PerShardFacts OFF in FullyEnabled | Incomplete coordinator safety |

## 7. What this package does **not** contain

- File I/O for config (config package owns that)  
- Logging implementation  
- Mangle Decl/rules  
- Feature discovery UI  
- Validation schema beyond pointer JSON tags  
