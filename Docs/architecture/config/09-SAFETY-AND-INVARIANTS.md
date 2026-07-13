# 09 — Safety and Invariants: config

> Last verified: 2026-07-13  
> Package: `internal/config` — safety **data** and **process** invariants (not Mangle Decl).

## 1. Role in constitutional safety

codeNERD’s executive safety is `permitted(...)` in the Mangle kernel. Config contributes:

| Contribution | Mechanism |
|--------------|-----------|
| Execution surface | `ExecutionConfig.AllowedBinaries`, `AllowedEnvVars` |
| Resource exhaustion | `CoreLimits` (memory, facts, shards, API calls, session duration) |
| Rate-limit politeness | `APISchedulerPolicy` + subscription defaults |
| CLI tool isolation | Codex sandbox read-only, shell tool disabled; Claude MaxTurns=1 |
| Secret handling | API keys in config file / env; not logged by config package (callers must not dump keys) |
| Provider honesty | Config-is-boss avoids accidental use of wrong vendor |

Config **does not** implement `permitted`. Empty allowlists filled by GetExecution still define a baseline binary set.

## 2. Invariants

### I1 — Explicit provider never falls back

If `Provider` is set, `GetActiveProvider` returns that provider and **only** its key (or empty key).

### I2 — Ollama is keyless with sentinel

`provider=ollama` returns apiKey `"ollama"` so callers that check non-empty key still construct local clients.

### I3 — Engine membership

`SetEngine` accepts only `api|claude-cli|codex-cli|xai-oauth`.

### I4 — Engine concurrency ≤ core concurrency

`GetEffectiveMaxConcurrentAPICalls` takes the minimum of core and engine caps when engine cap > 0.

### I5 — Core limit floors (YAML ValidateCoreLimits)

- MaxTotalMemoryMB ≥ 512  
- MaxConcurrentShards ≥ 1  
- MaxFactsInKernel ≥ 1000  
- MaxDerivedFactsLimit ≥ 1000  

UserConfig path does not automatically call this on load.

### I6 — Workspace root preference

go.mod always wins over nested `.nerd` to prevent state pollution.

### I7 — Features install is load-time

`features.SetActive` runs on LoadUserConfig; nil Features resets registry to compile-time defaults.

### I8 — Boolean false is representable

JIT/Reflection custom unmarshal tracks explicit false so defaults do not re-enable features users disabled.

### I9 — Image path isolation

Image generation models and shard types are detected and routed away from generic worker=ollama.

### I10 — Timeout chain consistency

Presets keep HTTP ≥ per-call alignment; slot wait ≥ per-call intent so schedulers do not cancel early while HTTP still runs.

## 3. Concurrency / reentrancy

- No package mutex on UserConfig; treat as single-writer (wizard/auth) + multi-reader.
- `globalLLMTimeouts` write only at boot.
- Do not call `SetLLMTimeouts` from concurrent hot paths.

## 4. Secrets

| Secret | Location |
|--------|----------|
| Provider API keys | UserConfig fields / YAML LLM.APIKey / env on YAML path |
| Context7 | `CONTEXT7_API_KEY` env preferred over file |
| xAI OAuth tokens | Outside package (`~/.nerd/xai_oauth.json`, `~/.grok/auth.json`) via XAIOAuth paths |

**Invariant:** config Save marshals keys into JSON if present — file mode 0644. Operators should treat `.nerd/config.json` as sensitive.

## 5. Mangle Decl

None in this package. Derived fact limits are **numeric budgets** consumed by the kernel, not rules.

## 6. Failure to load is not silent success for corrupt files

Missing file → empty OK. Malformed JSON/YAML → **error**. Soft-ignore at call sites is a wiring smell (see failure modes).
