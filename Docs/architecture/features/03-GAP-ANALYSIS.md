# 03 — Gap Analysis: features

> Last verified against codebase: **2026-07-13**  
> Source of truth for “what code does”: `internal/features/features.go` + importers.

## 1. Spec vs reality matrix

| Expectation (package charter / comments) | Reality | Gap? |
|------------------------------------------|---------|------|
| Leaf package, no internal imports | True (`fmt`/`os`/`sync/atomic` only) | **Non-gap** |
| env > active > default | Implemented in `resolveBool` + numeric accessors | **Non-gap** |
| Pointer bools distinguish absent vs false | Struct design + LoadUserConfig path | **Non-gap** |
| Conservative compile defaults | Defaults + `features_defaults_test` | **Non-gap** |
| FullyEnabled for init seed | `DefaultUserConfig` uses FullyEnabled | **Non-gap** |
| PerShardFacts stays off until Track D ready | FullyEnabled sets false; env/active can still enable | **Partial** — product intent “off until ready” is seed-level, not hard lock in accessor |
| Every accessor has a consumer | Most yes; TaxonomyFast no tool usage of accessor | **Gap** |
| Field comment env for SystemShards | Comment cites inverted `NERD_DISABLE_SYSTEM_SHARDS`; code uses `CODENERD_SYSTEM_SHARDS` | **Doc gap** |
| kernel_eval comment on DiffEval default TRUE | Actual default false | **Doc gap (external)** |
| config_roundtrip “short-circuit” PerShardFacts | FullyEnabled false; accessor is normal resolveBool | **Doc gap in test comment** |
| Boot logging of flags | LoadUserConfig logs Summary | **Non-gap** |
| Missing config file does not clobber active | Verified by test | **Non-gap** |
| Operator-visible flag dump | Only Boot log | **Gap** |
| Unified env prefix | Mix of `CODENERD_*` and `NERD_*` | **Gap (consistency)** |
| Taxonomy fast path via features | Tool checks env `== "1"` only | **Wiring gap** |
| Flight recorder gated | main.go wired | **Non-gap** |
| DiffEval gated at evaluate | kernel_eval wired | **Non-gap** |

## 2. Prioritized gaps

### P0 — correctness / silent wrong behavior

None known in the registry itself. Precedence and snapshot semantics are tested.

### P1 — wiring / half-integration

| ID | Gap | Impact | Suggested fix (docs-only note) |
|----|-----|--------|--------------------------------|
| G1 | `IsTaxonomyFastEnabled` unused by `cmd/tools/verify_taxonomy` | Config/active true cannot enable tool fast path; env only if exactly `"1"` (stricter than resolveBool) | Tool should call `features.IsTaxonomyFastEnabled()` after optional SetActive, or document intentional env-only |
| G2 | Stale claims that PerShardFacts is hard-locked false | Misleads operators who set env expecting no effect | Align comments with resolveBool behavior |

### P2 — operability

| ID | Gap | Impact |
|----|-----|--------|
| G3 | No CLI/slash “show resolved features” | Triage depends on Boot log presence |
| G4 | Env prefix inconsistency | Cognitive load; harder docs |

### P3 — hygiene

| ID | Gap | Impact |
|----|-----|--------|
| G5 | SystemShards struct comment wrong env name | Contributors wire wrong env |
| G6 | kernel_eval SPEC DEVIATION comment outdated | Future agents “fix” wrong default |
| G7 | No JSON schema / validation of unknown feature keys | Typos in config silently ignored |

## 3. Non-gaps (do not “fix”)

| Item | Why it is intentional |
|------|----------------------|
| Package has no Mangle | Flags are boot/eval Go concerns |
| No logging inside features | Preserves leaf purity |
| DiffEval default OFF at compile time | Protects test wall time and canonical eval |
| PerShardFacts false in FullyEnabled | Incomplete coordinator risk |
| Invalid env falls through | Avoids stray exports flipping bits |
| Numeric zero = call-site default | Avoids forcing scanner policy into features |
| Separate master vs per-shard disable | Different control planes |

## 4. Completeness heuristic

| Area | Completeness |
|------|--------------|
| Registry implementation | **~95%** |
| Consumer wiring | **~85%** (TaxonomyFast) |
| Docs/comments consistency | **~70%** |
| Operator UX | **~40%** |
| **Package charter overall** | **~90%** |
