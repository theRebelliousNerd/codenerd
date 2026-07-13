# 11 — Observability: features

> Last verified against codebase: **2026-07-13**

## 1. Package-local telemetry

**None.** By design, `internal/features` does not import `internal/logging` or emit metrics. Observability is **pushed to the boundary**.

## 2. Boot Summary (primary signal)

| What | Where |
|------|-------|
| Call | `features.Summary()` after `SetActive` |
| Emitter | `internal/config/user_config.go` → `logging.Get(logging.CategoryBoot).Info("%s", features.Summary())` |
| When | Successful parse of user config JSON |

### Summary formats

| Active state | String |
|--------------|--------|
| nil | `features: defaults active` |
| non-nil | `features: diff_eval=%v flight_recorder=%v provenance=%v system_shards=%v per_shard_facts=%v dark_mode=%v skip_onboarding=%v taxonomy_fast=%v fast_scan_workers=%d fast_ast_max_bytes=%d` |

### Known readability issue

Bool fields are `*bool`. The Summary format uses `%v` on those pointers, which prints **pointer addresses**, not `true`/`false`. Integers format correctly.  
**Impact:** Boot log may be hard to triage without correlating config file.  
**Mitigation ideas (not implemented here):** dereference in Summary, or log resolved `Is*` values at the config boundary.

## 3. Indirect observability (consumer-side)

| Flag effect | Observable signal |
|-------------|-------------------|
| FlightRecorder on | `StartFlightRecorder` success/warn on stderr; panic dump path on crash |
| Provenance on | session_boot `logStep("Provenance recording enabled via features flag")` |
| SystemShards off | `logStep("System shards disabled via feature flag; skipping boot")` |
| DiffEval | Kernel eval path timing/logs under CategoryKernel (see core corpus) |
| PerShardFacts | Cortex may expose FactRouter for observability (`FactRouter()` method) |
| DarkMode | Visual theme only |
| Scan tunables | Scanner concurrency/file skips (world logs if any) |

## 4. Metrics

No feature-flag-specific counters in the features package. Cortex route hit/miss counters relate to PerShardFacts machinery in **core**, not features.

## 5. Debug hooks

| Hook | Use |
|------|-----|
| `features.Active()` | Inspect raw active config in debugger/tests |
| `t.Setenv` + accessors | Force paths in tests |
| `CODENERD_DIFF_EVAL=0` | Deterministic full-eval in ops |
| `NERD_FLIGHTREC=0` | Disable recorder without editing JSON |

## 6. Categories (when logging exists at boundary)

| Category | Typical line |
|----------|--------------|
| Boot | `features: …` Summary |
| Kernel | rebuild/eval messages influenced by DiffEval |
| (stderr) | Flight recorder warnings from main |

## 7. Operator playbook

1. Confirm config loaded: look for Boot `features:` line.  
2. If line says `defaults active`, no Features block was installed (or SetActive(nil)).  
3. For DiffEval issues, force env `CODENERD_DIFF_EVAL=0` and restart.  
4. For shard boot skip, check `CODENERD_SYSTEM_SHARDS` and config `system_shards`.  
5. Remember legacy `NERD_DISABLE_SYSTEM_SHARDS` is a **list**, not the master switch.  
