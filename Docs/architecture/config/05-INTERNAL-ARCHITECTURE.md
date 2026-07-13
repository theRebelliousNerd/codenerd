# 05 — Internal Architecture: config

> Last verified: 2026-07-13  
> Source: `internal/config/`

## 1. Component map

```
┌──────────────────────────────────────────────────────────────────┐
│                         internal/config                          │
├────────────────────┬─────────────────────┬───────────────────────┤
│  Load path         │  Domain structs     │  Resolution layer     │
│  FindWorkspaceRoot │  llm.go engines     │  GetActiveProvider    │
│  LoadUserConfig    │  limits.go          │  GetEngine / SetEngine│
│  GlobalConfig      │  memory.go          │  GetCoreLimits        │
│  Load (YAML)       │  shard/execution    │  GetEffective*        │
│  Save / SaveUser   │  jit/reflection     │  Get* defaults        │
│  applyEnvOverrides │  world/build/ux     │  Validate*            │
└────────────────────┴─────────────────────┴───────────────────────┘
           │                    │                     │
           ▼                    ▼                     ▼
    features.SetActive     mcp.MCPServerConfig   time.Duration helpers
    logging.CategoryBoot   (via integrations)
```

## 2. Data flow: load

### 2.1 Live path (JSON)

```
cwd
 → FindWorkspaceRoot()
 → path = root/.nerd/config.json
 → os.ReadFile
    ├─ not exist → empty UserConfig{}, nil error
    ├─ read error → error
    └─ json.Unmarshal
         → features.SetActive(cfg.Features)
         → log features.Summary()
         → return *UserConfig
```

Callers then call `Get*` methods which **copy** nested structs and fill zeros — they do not mutate the stored pointers except when Save writes back.

### 2.2 Legacy path (YAML)

```
DefaultConfig()  // full value tree
 → ReadFile
    ├─ not exist → applyEnvOverrides → return
    └─ yaml.Unmarshal into defaults → applyEnvOverrides → return
```

## 3. Data flow: engine effective policy

```
UserConfig.APIScheduler (optional pointers)
UserConfig.CoreLimits
UserConfig.Engine + ClaudeCLI/CodexCLI/XAIOAuth
GetLLMTimeouts().SlotAcquisitionTimeout
        │
        ▼
GetEffectiveAPISchedulerPolicy()
        │
        ▼
EffectiveAPISchedulerPolicy {
  MaxConcurrentAPICalls,
  MinCallSpacing,
  AdaptiveConcurrency,
  AdaptiveFloor,
  AdaptiveRecoverAfter,
  SlotAcquireTimeout,
}
```

## 4. Key types and ownership

| Type | Owning file | Mutability |
|------|-------------|------------|
| `UserConfig` | user_config.go | Loaded then mutated by wizards/auth Save |
| `Config` | config.go | Loaded independently; not auto-synced with JSON |
| `LLMTimeouts` | llm_timeouts.go | Process global via Set/Get |
| `ShardProfile` | shard.go | Map on UserConfig or Config |
| `CoreLimits` | limits.go | Nested on both aggregates |
| `ContextWindowConfig` | memory.go | Nested; percent allocation model |
| `JITConfig` | jit.go | Bool set tracking on unmarshal |
| `features.FeaturesConfig` | external package | Installed on load |

## 5. State machines (operator-visible)

### 5.1 Onboarding

```
SetupComplete=false ──wizard──► SetupComplete=true
ExperienceLevel: beginner → intermediate → advanced → expert
TourStep increments until TourComplete
```

Stored in `OnboardingState`; not enforced by kernel.

### 5.2 Engine switch

```
api ◄──► claude-cli
  ▲         │
  │         ▼
  └── xai-oauth ◄──► codex-cli
```

`SetEngine` validates; invalid strings error. Persistence requires `Save`.

## 6. Merge / default strategy

| Layer | Behavior |
|-------|----------|
| File absent | Empty or DefaultConfig |
| Field omitted | Get* default or zero value type default |
| Field zero | Get* often replaces with default |
| Explicit false | Honored when `enabledSet` / pointer present |
| Env (YAML) | Overwrites after file load |
| Env (Context7) | Overrides JSON key at Get time |

There is **no** deep merge library — partial JSON unmarshals into zeroed struct, then Get* fills.

## 7. Concurrency

- Package is largely **stateless** after load except:
  - `globalLLMTimeouts` (no mutex — set at startup)
  - `features` package active pointer (owned by features)
- Callers typically share a `*UserConfig` pointer on chat model; Save is infrequent and not concurrent-safe beyond process norms.

## 8. Error model

| Operation | Missing file | Malformed | Invalid engine/provider |
|-----------|--------------|-----------|-------------------------|
| LoadUserConfig | empty OK | error | not checked at load |
| Load (YAML) | defaults OK | error | Validate() separate |
| SetEngine | n/a | n/a | error |
| Validate (Config) | n/a | n/a | error if bad provider / missing key (except ollama) |
