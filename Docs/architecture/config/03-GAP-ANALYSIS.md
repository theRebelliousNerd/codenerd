# 03 — Gap Analysis: config

> Last verified: 2026-07-13  
> Compare: vision ([01-VISION.md](01-VISION.md)) vs code (`internal/config/`)

## 1. Spec vs reality matrix

| Desired property | Reality | Gap severity |
|------------------|---------|--------------|
| Single runtime aggregate | UserConfig + Config both live | **High** (drift risk) |
| Config-is-boss providers | Implemented on UserConfig | None |
| go.mod-first workspace | Implemented | None |
| Multi-engine | Implemented | None |
| Subscription-aware scheduler | Implemented | None |
| Env overrides on live path | Mostly YAML Config; Context7 special-cased | **Medium** |
| Full JSON schema validation | Partial (engine set; Validate on YAML Config for key/provider) | **Medium** |
| Hot reload | Not implemented | **Low** (by design today) |
| Symmetric defaults | Dual defaults for shards/timeouts/context | **High** |
| UIConfig on UserConfig | UIConfig type separate; Theme on UserConfig | **Low** |
| Documented load order | Code comments strong; operator docs thin outside this corpus | **Low** |
| Leaf-safe feature flags | SetActive on load | None |
| Embedding call-site discipline | Helper + comments; not compiler-enforced | **Low** |

## 2. Prioritized backlog (gaps only)

### P0 — Correctness / safety of operations

1. **Document and/or unify dual defaults**  
   Especially `MaxConcurrentShards` (12 vs 4) and execution timeout (30s vs 10m). Callers that still use YAML defaults can under/over-constrain shards.

2. **Clarify env override ownership**  
   Operators setting `XAI_API_KEY` may believe JSON UserConfig path picks it up; only YAML `applyEnvOverrides` (and specific helpers) do.

### P1 — Maintainability

3. **Deprecation path for `Config` YAML**  
   Either adapter that projects UserConfig → Config for old callers, or migrate `main.go` Load fully to JSON.

4. **Optional strict ValidateUserConfig**  
   Engine, provider, concurrent floors, percent sum for context reserves, known embedding providers.

### P2 — Product polish

5. Fold `UIConfig` (split pane) into UserConfig if TUI needs persistence.  
6. Hot-reload API for long sessions (explicit, not automatic).  
7. Config version field for migrations (`internal/ux/migration.go` already touches config).

## 3. Non-gaps (do not “fix”)

| Observation | Why it is OK |
|-------------|--------------|
| Empty config on missing file | Intentional first-run; Get* supply defaults |
| Pointer fields for optional blocks | Distinguishes omit vs zero |
| Custom UnmarshalJSON for bool defaults | Required for enabled=false honesty |
| Global LLMTimeouts singleton | Process-wide consistency; tests cover Get/Set |
| No Mangle in package | Config is not executive logic |
| CLI engines disable tools/shell by default | Aligns with tactile ownership |

## 4. Evidence pointers

- Dual defaults: `config.go` `defaultCoreLimits` vs `user_config.go` `GetCoreLimits`
- Env: `config.go` `applyEnvOverrides` vs `GetContext7APIKey` env priority
- Engines: `SetEngine` valid set in `user_config.go`
- Tests already lock many desired properties: `config_comprehensive_test.go`, `env_override_test.go`
