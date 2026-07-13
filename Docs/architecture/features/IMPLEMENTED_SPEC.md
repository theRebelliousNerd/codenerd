# features — Implemented Spec (Deep-Dive)

> Last verified against codebase: **2026-07-13**  
> Status: Living Reference Document  
> Language: Go  
> Primary source: `internal/features/features.go`  
> Scale: **1** non-test file (**351** lines); **3** test files; **0** `.mg`

## 1. Overview

`internal/features` is codeNERD’s **process-wide modernization flag registry**. It is deliberately a **leaf package**: stdlib only (`fmt`, `os`, `sync/atomic`). No other `codenerd/internal/*` import is allowed.

### Problem it solves

Several low-level subsystems need to know whether expensive or experimental paths are live:

| Consumer need | Example |
|---------------|---------|
| Eval path selection | DifferentialEngine vs full re-eval (`internal/core/kernel_eval.go`) |
| Boot-time optional subsystems | Flight recorder (`cmd/nerd/main.go`), system shards (`session_boot.go`) |
| Memory-cost features | Provenance / DerivationRecorder |
| UX overrides | Dark mode, skip onboarding |
| Scanner tunables | Worker count, AST size cutoff (`internal/world`) |

Those packages **cannot** import `internal/config` without cycles (`config → store → … → core`). Features is the **transduction membrane** for toggles: config writes once via `SetActive`; everyone else reads via wait-free accessors.

### Key characteristics

| Property | Value |
|----------|-------|
| Package role | Leaf toggle registry |
| On-disk shape | `.nerd/config.json` → `features` JSON object |
| Active store | `atomic.Pointer[FeaturesConfig]` |
| Precedence | **env > active config > compile-time default** |
| Bool env accept | `1`/`true`/`TRUE`/`True`, `0`/`false`/`FALSE`/`False` only |
| Invalid env | Fall through (no silent flip) |
| Defaults posture | Conservative for eval/memory; cheap paths ON |
| Seed posture | `FullyEnabledFeaturesConfig()` for init/wizard (except PerShardFacts) |
| Mangle | None — flags are Go-side only |
| Constitutional safety | Does **not** implement `permitted(...)`; gates optional paths only |

### High-level data flow

```
.nerd/config.json
        │
        ▼
config.LoadUserConfig(path)
        │  features.SetActive(cfg.Features)   // may be nil
        │  logging.Boot ← features.Summary()
        ▼
active atomic.Pointer[FeaturesConfig]
        │
        ├─ env CODENERD_* / NERD_*  (if non-empty + recognized)
        │         wins
        ▼
IsDiffEvalEnabled / IsFlightRecorderEnabled / …
        │
        ▼
core · world · ux · cmd/nerd · chat boot
```

Fact-flow placement (honest): features sits **beside** the OODA spine, not on it. It does not assert `user_intent` or derive `next_action`. It **conditions** how kernel eval, shards, scanners, and CLI boot behave once Cortex is up.

```
user_intent → kernel → next_action → VirtualStore → articulation
                 ▲
                 │  (eval path / provenance / fact router gated by features)
```

---

## 2. Implementation status

| Component | Status | Notes |
|-----------|--------|-------|
| `FeaturesConfig` JSON shape | **Implemented** | Pointer bools + two int fields |
| `DefaultFeaturesConfig` | **Implemented** | Conservative compile-time defaults |
| `FullyEnabledFeaturesConfig` | **Implemented** | Init/wizard seed; PerShardFacts stays false |
| `SetActive` / `Active` | **Implemented** | Snapshot copy; nil resets |
| `Summary` | **Implemented** | Boot log string; package itself log-free |
| `resolveBool` precedence | **Implemented** | env → active → default |
| Per-flag `Is*` accessors | **Implemented** | 8 boolean flags |
| Numeric accessors | **Implemented** | `FastScanWorkers`, `FastASTMaxBytes` |
| Integer parse helpers | **Implemented** | Digit-only, reject ≤0 |
| Config install on load | **Implemented** | `internal/config/user_config.go` |
| DiffEval → kernel | **Wired** | `kernel_eval.diffEvalEnabled()` |
| FlightRecorder → main | **Wired** | `cmd/nerd/main.go` |
| Provenance → chat boot | **Wired** | `session_boot.go` → `kernel.EnableProvenance()` |
| SystemShards master switch | **Wired** | `session_boot.go` |
| PerShardFacts → cortex | **Wired** | `cortex_kernel.NewCortexKernel` |
| DarkMode → TUI | **Wired** | `cmd/nerd/ui/styles.go` |
| SkipOnboarding → UX | **Wired** | `internal/ux/migration.go` |
| Scan tunables → world | **Wired** | `world/scanner_config.go` |
| TaxonomyFast accessor | **Partial** | Registry exists; `cmd/tools/verify_taxonomy` reads **env only**, not `IsTaxonomyFastEnabled()` |
| PerShardFacts production default | **Off by design** | FullyEnabled keeps false; coordinator opt-in |
| CLI `/features` or status dump | **Missing** | No first-class user-facing flag inspector |
| Dynamic reload without re-load | **Partial** | `SetActive` works; no file watcher |

**Overall:** production-ready leaf registry for a small, intentional flag set — **not** a general feature-flag SaaS. Implementation completeness ≈ **90%** of its stated charter; residual gaps are consumer wiring and doc/comment drift.

---

## 3. Source inventory

### 3.1 Layout

```
internal/features/
  features.go                 # entire implementation (351 lines)
  features_test.go            # precedence, copy semantics, numerics, legacy env
  features_defaults_test.go   # DefaultFeaturesConfig + parseInt64 + error string
  config_roundtrip_test.go    # package features_test: config.LoadUserConfig boundary
```

### 3.2 File roles

| Path | Lines ≈ | Role |
|------|--------:|------|
| `internal/features/features.go` | 351 | Types, defaults, atomic registry, all accessors |
| `internal/features/features_test.go` | ~154 | Precedence contracts, SetActive snapshot, numerics |
| `internal/features/features_defaults_test.go` | ~43 | Default on/off matrix, parseInt64, featuresErr |
| `internal/features/config_roundtrip_test.go` | ~193 | External package test against `config.LoadUserConfig` |

### 3.3 Design constraints (from package comment)

1. **Own package**, not `internal/config` subdir — cycle avoidance.  
2. **Depends on nothing inside codeNERD.**  
3. Adding a flag means **three lockstep changes**: field on `FeaturesConfig`, public accessor with env+active+default, default in `DefaultFeaturesConfig` (+ seed key via init/config).  
4. Caller of `SetActive` owns Boot logging (`Summary()`), not this package.

---

## 4. Deep dive — `FeaturesConfig` and defaults

### 4.1 Struct shape

Every boolean is a **pointer** so JSON unmarshalling can distinguish:

- key absent → nil → accessor uses compile-time default  
- key present `false` → `*false` → user explicitly disabled  
- key present `true` → `*true`

| Field | JSON key | Env var | Compile default | FullyEnabled | Intent |
|-------|----------|---------|-----------------|--------------|--------|
| `DiffEval` | `diff_eval` | `CODENERD_DIFF_EVAL` | **false** | true | DifferentialEngine fast path |
| `FlightRecorder` | `flight_recorder` | `NERD_FLIGHTREC` | **true** | true | runtime/trace ring buffer |
| `Provenance` | `provenance` | `CODENERD_PROVENANCE` | **false** | true | Mangle DerivationRecorder |
| `SystemShards` | `system_shards` | `CODENERD_SYSTEM_SHARDS` | **true** | true | Master switch for Type-1 shards |
| `PerShardFacts` | `per_shard_facts` | `CODENERD_PER_SHARD_FACTS` | **false** | **false** | ShardFactRouter partition |
| `DarkMode` | `dark_mode` | `CODENERD_DARK_MODE` | **false** | true | Force dark TUI palette |
| `SkipOnboarding` | `skip_onboarding` | `NERD_SKIP_ONBOARDING` | **false** | true | Bypass first-run wizard |
| `TaxonomyFast` | `taxonomy_fast` | `CODENERD_TAXONOMY_FAST` | **true** | true | Fast verify_taxonomy path |
| `FastScanWorkers` | `fast_scan_workers` | `NERD_FAST_SCAN_WORKERS` | 0 | 0 | Override scan concurrency |
| `FastASTMaxBytes` | `fast_ast_max_bytes` | `NERD_FAST_AST_MAX_BYTES` | 0 | 0 | Skip large-file AST parse |

**Env naming inconsistency (real):** some flags use `CODENERD_*`, others legacy `NERD_*`. Documented per field; do not “normalize” without a migration plan.

### 4.2 Two default profiles

**`DefaultFeaturesConfig()`** — unit tests, ad-hoc kernel construction, pre-config boot:

- Cheap/safe ON: FlightRecorder, SystemShards, TaxonomyFast  
- Expensive/experimental OFF: DiffEval, Provenance, PerShardFacts, DarkMode, SkipOnboarding  

Rationale (from source): DiffEval’s first build is heavyweight (schema load + stratify); Provenance allocates per-derivation; tests should see **canonical full-eval** unless they opt in.

**`FullyEnabledFeaturesConfig()`** — what `DefaultUserConfig` / init seeds into `.nerd/config.json`:

- All booleans true **except** `PerShardFacts` remains false  
- Comment: enabling partition without coordinator readiness can soft-brick kernel paths  

**Important accuracy note:** `IsPerShardFactsEnabled()` does **not** hard-code `return false`. It uses normal `resolveBool`. The “always off” behavior of FullyEnabled is because the **seed struct sets the pointer to false**. Env `CODENERD_PER_SHARD_FACTS=1` or an explicit active true **does** enable the flag (covered by `TestPerShardFactsPrecedence`). Some older comments/tests phrase this as “short-circuit” or “hard-coded false” — that is **stale language** relative to the accessor body.

### 4.3 Stale comment elsewhere

`internal/core/kernel_eval.go` still claims `IsDiffEvalEnabled()` defaults TRUE and cites historical SPEC DEVIATION. **Current** features code defaults DiffEval **false**. Prefer `features.go` + `features_defaults_test.go` as truth.

---

## 5. Deep dive — resolution machinery

### 5.1 Active registry

```go
var active atomic.Pointer[FeaturesConfig]

func SetActive(cfg *FeaturesConfig) {
  if cfg == nil { active.Store(nil); return }
  c := *cfg          // copy — callers cannot mutate through original pointer
  active.Store(&c)
}

func Active() *FeaturesConfig { return active.Load() } // do not mutate
```

- **Wait-free reads** on hot paths (e.g. every `evaluate()` for DiffEval).  
- **Snapshot on write** — `TestSetActiveCopySemantics` locks this.  
- **Nil active** means “defaults only” (plus env).

### 5.2 `resolveBool`

1. If env non-empty and ∈ {1,true,TRUE,True} → true  
2. If env non-empty and ∈ {0,false,FALSE,False} → false  
3. Else if active has non-nil field → that value  
4. Else compile-time `def` argument  

Garbage env (`yes`, `maybe`, `2`) falls through — intentional.

### 5.3 Numeric accessors

- Env first; parse with digit-only `parseUint` / `parseInt64` (reject empty, 0, negatives, whitespace, decimals).  
- Else active field.  
- Else **0** meaning “call site uses its own default” (`world.DefaultScannerConfig`: CPU-clamped workers, 2MiB AST cutoff).

---

## 6. Deep dive — consumer integration map

| Flag accessor | Call site | Behavior when ON / OFF |
|---------------|-----------|-------------------------|
| `IsDiffEvalEnabled` | `internal/core/kernel_eval.go` `diffEvalEnabled()` | Routes evaluate through DifferentialEngine when true |
| `IsFlightRecorderEnabled` | `cmd/nerd/main.go` after `config.GlobalConfig()` | `observability.StartFlightRecorder(64MiB, 30s)` + panic dump |
| `IsProvenanceEnabled` | `cmd/nerd/chat/session_boot.go` | `kernel.EnableProvenance()` before first Evaluate |
| `IsSystemShardsEnabled` | `session_boot.go` | Skip entire system-shard boot block when false |
| `IsPerShardFactsEnabled` | `internal/core/cortex_kernel.go` `NewCortexKernel` | Construct `ShardFactRouter` when true |
| `IsDarkModeEnabled` | `cmd/nerd/ui/styles.go` | Force `DarkTheme()` |
| `IsOnboardingSkipped` | `internal/ux/migration.go` `ShouldShowOnboarding` | Return false immediately when true |
| `FastScanWorkers` / `FastASTMaxBytes` | `internal/world/scanner_config.go` | Override DefaultScannerConfig when >0 |
| `IsTaxonomyFastEnabled` | **(registry only)** | `cmd/tools/verify_taxonomy` uses raw `os.Getenv == "1"` — config/active path **not** consumed |

### System shards dual control

- **Master:** `IsSystemShardsEnabled()` / `CODENERD_SYSTEM_SHARDS` / `features.system_shards`  
- **Per-shard disable list (legacy):** `NERD_DISABLE_SYSTEM_SHARDS` comma list + CLI `--disable-system-shard` — parsed in session_boot, **not** in features  

`TestSystemShardsLegacyEnvIgnored` proves legacy env does not flip the master switch.

### Config install contract

`config.LoadUserConfig`:

- On successful parse → `features.SetActive(cfg.Features)` then Boot log `features.Summary()`  
- On **missing file** → returns empty config **without** calling SetActive (preserves prior registry) — locked by `TestLoadUserConfig` / nonexistent_file case  
- `DefaultUserConfig()` embeds `FullyEnabledFeaturesConfig()` for seed content  

---

## 7. Public API summary

| Symbol | Kind | Notes |
|--------|------|-------|
| `FeaturesConfig` | type | JSON-facing config block |
| `DefaultFeaturesConfig` | func | Conservative defaults |
| `FullyEnabledFeaturesConfig` | func | Wizard/seed defaults |
| `SetActive` | func | Install or clear registry |
| `Active` | func | Read pointer (immutable contract) |
| `Summary` | func | One-line boot description |
| `IsDiffEvalEnabled` … `IsTaxonomyFastEnabled` | funcs | Boolean gates |
| `FastScanWorkers` | func | int; 0 = use local default |
| `FastASTMaxBytes` | func | int64; 0 = use local default |

Unexported: `resolveBool`, `parseUint`, `parseInt64`, `featuresErr`, `errBadInt`.

---

## 8. Testing surface

| Test file | Covers |
|-----------|--------|
| `features_test.go` | env>active>default; invalid env; env 0 forces off; PerShardFacts; SystemShards legacy isolation; SetActive copy; numerics |
| `features_defaults_test.go` | Default on/off matrix; parseInt64 reject set; error string |
| `config_roundtrip_test.go` | LoadUserConfig installs registry; absent features → defaults; missing file preserves; FullyEnabled round-trip; numeric env win |

Downstream: `internal/core/kernel_features_test.go` asserts kernel respects DiffEval gate.

---

## 9. Gaps pointer

See [03-GAP-ANALYSIS.md](03-GAP-ANALYSIS.md) and [TODO.md](TODO.md). Headline gaps:

1. TaxonomyFast not using registry accessor in tool binary.  
2. Stale comments (kernel_eval default, PerShardFacts “short-circuit”, SystemShards field comment naming legacy env as master).  
3. No operator-facing “list live flags” command (only Boot Summary log).  
4. Env prefix inconsistency (`NERD_` vs `CODENERD_`).  
5. No formal schema validation of `features` block beyond JSON unmarshal.

---

## 10. North-star fit (one paragraph)

Features is **executive infrastructure**, not creative surface: it never prompts an LLM and never invents policy. It lets the **deterministic** side of the system choose evaluation and boot strategies. Constitutional safety remains in Mangle `permitted(...)`; features only enable optional machinery that must still obey policy when active. Prefer **wiring audits** before declaring a flag unused — TaxonomyFast is the canonical “accessor exists, consumer half-wired” case.
