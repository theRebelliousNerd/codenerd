# 08 — Wiring and Integration: features

> Last verified against codebase: **2026-07-13**

## 1. Boot sequence (binary)

```
cmd/nerd/main.go main()
    │
    ├─ config.GlobalConfig()     // loads .nerd/config.json path
    │       │
    │       └─ LoadUserConfig
    │              features.SetActive(cfg.Features)   // may be nil
    │              logging.Boot ← features.Summary()
    │
    ├─ observability.LogStartupMetrics()
    │
    ├─ if features.IsFlightRecorderEnabled():
    │       observability.StartFlightRecorder(...)
    │       defer panic → DumpFlightRecord
    │
    └─ rootCmd.Execute()
            │
            └─ interactive chat → session_boot
                   if IsProvenanceEnabled → kernel.EnableProvenance
                   if !IsSystemShardsEnabled → skip shard boot
                   else start system shards + NERD_DISABLE_SYSTEM_SHARDS list
```

**Ordering requirement:** config load **before** any feature-gated boot check. main.go comments document this explicitly for FlightRecorder and system shards.

## 2. Config install site

**File:** `internal/config/user_config.go`

| Hook | Behavior |
|------|----------|
| `UserConfig.Features` | `*features.FeaturesConfig` JSON field |
| After successful JSON parse | `features.SetActive(cfg.Features)` |
| Missing file | Return empty config; **do not** SetActive |
| `DefaultUserConfig` | Features = FullyEnabledFeaturesConfig() |

## 3. Kernel / cortex wiring

### DiffEval

- **File:** `internal/core/kernel_eval.go`  
- **Bridge:** `diffEvalEnabled() bool { return features.IsDiffEvalEnabled() }`  
- **When:** re-read each evaluate so env toggles apply between passes  
- **Tests:** `internal/core/kernel_features_test.go`

### PerShardFacts

- **File:** `internal/core/cortex_kernel.go`  
- **When:** `NewCortexKernel` constructs `NewShardFactRouter()` iff `IsPerShardFactsEnabled()`  
- **Tests:** router tests install router via `SetFactRouter` when flag is off (default)

### Provenance

- **Not** checked inside `RealKernel` for the feature flag continuously.  
- **Boot:** `session_boot.go` calls `kernel.EnableProvenance()` if flag on.  
- **Interactive:** `/explain`-related paths may enable via kernel methods (`commands_handlers_misc.go` uses kernel’s own `IsProvenanceEnabled`, not features).

## 4. CLI / TUI wiring

| Flag | File | Integration |
|------|------|-------------|
| FlightRecorder | `cmd/nerd/main.go` | Start + panic dump |
| SystemShards | `cmd/nerd/chat/session_boot.go` | Master skip + legacy list |
| Provenance | `session_boot.go` | Enable before Evaluate |
| DarkMode | `cmd/nerd/ui/styles.go` | Theme selection |

## 5. World / UX wiring

| Flag | File | Integration |
|------|------|-------------|
| FastScanWorkers / FastASTMaxBytes | `internal/world/scanner_config.go` `DefaultScannerConfig` | Override if > 0 |
| SkipOnboarding | `internal/ux/migration.go` `ShouldShowOnboarding` | Early false |

## 6. Init / seed wiring

FullyEnabled lands in user config via `DefaultUserConfig`. Init/wizard paths that write that struct therefore seed modern flags (except PerShardFacts). Exact init command paths live under CLI/config corpora; features only supplies the struct factory.

## 7. Wiring gaps journal

| Item | Status | Notes |
|------|--------|-------|
| TaxonomyFast registry | **Half-wired** | Accessor + defaults exist; tool uses raw env equality to `"1"` only |
| Observability package import of features | **Indirect** | main gates StartFlightRecorder; observability does not need features import |
| Dynamic reload of config.json | **Not wired** | No fsnotify → SetActive; restart or explicit reload required |
| Mangle-visible feature facts | **Not wired** | No `feature_flag(...)` atoms asserted into kernel |

## 8. Fact-flow honesty

Features is **not** registered as a VirtualStore action or shard. Integration is **side-channel configuration** for subsystems that *are* on the fact-flow path. When auditing “why did eval take the diff path?”, look at Boot Summary and env, not at Mangle proofs.
