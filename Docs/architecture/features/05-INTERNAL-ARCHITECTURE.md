# 05 — Internal Architecture: features

> Last verified against codebase: **2026-07-13**  
> Single-file package; architecture is the **registry pattern**, not many modules.

## 1. Components

```
┌──────────────────────────────────────────────────────────┐
│                    features.go                           │
│                                                          │
│  FeaturesConfig  ──JSON tags──►  .nerd/config.json       │
│                                                          │
│  DefaultFeaturesConfig()                                 │
│  FullyEnabledFeaturesConfig()                            │
│                                                          │
│  active: atomic.Pointer[FeaturesConfig]                  │
│       ▲                    │                             │
│       │ SetActive          │ Active / resolve*           │
│       │                    ▼                             │
│  resolveBool(env, field, def) ──► Is*Enabled()           │
│  parseUint / parseInt64     ──► FastScan* / FastAST*     │
│  Summary()                                               │
└──────────────────────────────────────────────────────────┘
```

There is **no** subsystem graph inside the package — only:

| Component | Responsibility |
|-----------|----------------|
| Config struct | On-disk / in-memory shape |
| Default factories | Two named postures |
| Atomic registry | Process-wide active config |
| Resolvers | Precedence + parsing |
| Public accessors | Stable API for consumers |

## 2. State machine (registry)

```
                 SetActive(nil)
         ┌──────────────────────────┐
         │                          │
         ▼                          │
   ┌───────────┐   SetActive(cfg)  │
   │  NIL      │ ─────────────────►│  NON-NIL SNAPSHOT
   │ (defaults │ ◄─────────────────│  (copied FeaturesConfig)
   │  + env)   │   SetActive(nil)  │
   └───────────┘                   └──┬──────────────────
                                      │
                                      │ SetActive(other)
                                      ▼
                                 replace snapshot
```

Reads never block writers beyond atomic pointer semantics. No version counter — last writer wins.

## 3. Boolean resolve flow

```
IsXxxEnabled()
    │
    ▼
os.Getenv(ENV)
    │
    ├─ "" ─────────────────────────────┐
    ├─ 1/true/TRUE/True → return true  │
    ├─ 0/false/FALSE/False → false     │
    └─ other → fall through            │
                                       ▼
                         active.Load() != nil && field != nil ?
                              │ yes → *field
                              │ no  → compile-time def
```

## 4. Numeric resolve flow

```
FastScanWorkers() / FastASTMaxBytes()
    │
    ├─ env set and parse OK (>0 digits) → return N
    ├─ env set and parse fail → fall through
    ├─ active non-nil → return field (may be 0)
    └─ else → 0  (call-site default)
```

Note: active zero **does not** fall through to a package default — the package’s “default” for numerics **is** zero.

## 5. Data ownership

| Data | Owner | Lifetime |
|------|-------|----------|
| On-disk JSON | `internal/config` + filesystem | Until next Save/Load |
| Active snapshot | `features` package | Process lifetime |
| Env vars | OS / test harness | Process / test scope |
| Compile defaults | function constants in features.go | Binary |

## 6. Concurrency model

- **Readers:** any goroutine, including hot evaluate paths — wait-free `atomic.Load`.  
- **Writers:** expected rare (boot, tests, maybe reload). `Store` of new pointer.  
- **Hazard:** if a future writer mutates through `Active()` without copy, races appear — contract forbids mutation.  
- **No mutex** — intentional.

## 7. Error model

- Bool resolve: never errors; invalid env ignored.  
- Numeric env parse fail: ignored; fall through.  
- Only internal parse helpers return `errBadInt` (`features: invalid integer override`).  
- Package does not surface errors to accessors.

## 8. Extension point (how to add a flag)

1. Add `*bool` or int field + JSON tag + env in comment.  
2. Add accessor calling `resolveBool` or numeric pattern.  
3. Set Default + FullyEnabled.  
4. Wire consumer.  
5. Add precedence test.  
6. Optionally seed via `DefaultUserConfig` / init (already FullyEnabled-based for bools).  
