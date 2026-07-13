# 12 — Failure Modes: features

> Last verified against codebase: **2026-07-13**

## FM1 — Stale active registry after config change

| | |
|--|--|
| **Symptom** | User edits `.nerd/config.json` features block; running process keeps old flags |
| **Cause** | No file watcher; SetActive only on LoadUserConfig |
| **Mitigation** | Restart process or re-invoke config load path that calls SetActive |
| **Detection** | Boot Summary only at load |

## FM2 — Missing config file preserves previous flags

| | |
|--|--|
| **Symptom** | Tests or tools call LoadUserConfig on missing path; expect defaults but still see prior flags |
| **Cause** | Documented: missing file does not SetActive |
| **Mitigation** | Call `features.SetActive(nil)` in test Cleanup; production usually loads once at boot |
| **Tests** | `nonexistent_file_preserves_active_registry` |

## FM3 — Invalid env silently ignored

| | |
|--|--|
| **Symptom** | User exports `CODENERD_DIFF_EVAL=yes` expecting on; flag stays off/active |
| **Cause** | resolveBool only accepts 1/true/0/false casings |
| **Mitigation** | Use documented values; treat as intentional |
| **Tests** | invalid env fall-through cases |

## FM4 — DiffEval unexpected ON/OFF

| | |
|--|--|
| **Symptom** | Slow tests or divergent eval behavior |
| **Cause** | Active FullyEnabled seed, env, or mistaken belief about default |
| **Mitigation** | Know: compile default is **false**; production seed may be true; env wins |
| **Note** | kernel_eval comments may still claim default true — distrust them |

## FM5 — PerShardFacts enabled too early

| | |
|--|--|
| **Symptom** | Soft-brick / wrong cross-shard joins if coordinator incomplete |
| **Cause** | Explicit env or active true while subsystem immature |
| **Mitigation** | FullyEnabled keeps false; only enable with Track D readiness |
| **Detection** | Cortex FactRouter non-nil |

## FM6 — System shards all off vs one off

| | |
|--|--|
| **Symptom** | Autopoiesis/observer never start, or only one disabled |
| **Cause** | Confusing `CODENERD_SYSTEM_SHARDS=0` (master) with `NERD_DISABLE_SYSTEM_SHARDS=name` (list) |
| **Mitigation** | Read session_boot dual control; use features master for all-off |
| **Tests** | `TestSystemShardsLegacyEnvIgnored` |

## FM7 — TaxonomyFast config ignored by tool

| | |
|--|--|
| **Symptom** | `features.taxonomy_fast: true` in config has no effect on verify_taxonomy |
| **Cause** | Tool only checks env `== "1"`; never calls IsTaxonomyFastEnabled / SetActive path may not run in tool main |
| **Mitigation** | Set `CODENERD_TAXONOMY_FAST=1` env for tool today; wire accessor later |

## FM8 — Numeric env parse fail falls through

| | |
|--|--|
| **Symptom** | `NERD_FAST_SCAN_WORKERS=-1` or `8.0` ignored |
| **Cause** | Digit-only positive parser |
| **Mitigation** | Use positive integers without sign |

## FM9 — Boot Summary hard to read

| | |
|--|--|
| **Symptom** | Log shows pointer-looking values for bool flags |
| **Cause** | `%v` on `*bool` without dereference |
| **Mitigation** | Read config file or call Is* in a debug session; fix Summary in a future code change |

## FM10 — Cross-test pollution

| | |
|--|--|
| **Symptom** | Flaky tests depending on package order |
| **Cause** | Global atomic active not cleaned |
| **Mitigation** | Always `t.Cleanup(func(){ SetActive(nil) })` and clear env with `t.Setenv` |
| **Pattern** | Established in features tests and config_roundtrip |

## FM11 — Flight recorder start failure

| | |
|--|--|
| **Symptom** | Warning on stderr; no ring buffer |
| **Cause** | `StartFlightRecorder` error when flag on |
| **Mitigation** | main continues; flag cannot force success |
| **Location** | `cmd/nerd/main.go` |

## FM12 — Import cycle if leaf broken

| | |
|--|--|
| **Symptom** | Build fails with import cycle involving features |
| **Cause** | features imports config/core/logging |
| **Mitigation** | Reject PRs that add internal imports; use external test package for config tests |
