# 01 — Vision: features

> Last verified against codebase: **2026-07-13**  
> Package: `internal/features`

## 1. Product role

Operators and developers need **predictable levers** for modernization paths without recompiling and without teaching every package about `.nerd/config.json`. Features is that lever set: a tiny, stable contract between **config load** and **runtime path selection**.

It is **not** a product surface users talk to in natural language. Ideal end-state UX is:

- Flags appear in `.nerd/config.json` with documented keys  
- CI can force with env  
- `nerd` boot log shows the active snapshot  
- Optional future: a status subcommand or `/features` slash that prints the same Summary  

## 2. Architectural vision

```
┌─────────────────────────────────────────────────────────┐
│  Human / CI / init wizard                               │
│    .nerd/config.json  |  env  |  FullyEnabled seed      │
└───────────────────────────┬─────────────────────────────┘
                            │
                            ▼
┌─────────────────────────────────────────────────────────┐
│  internal/config.LoadUserConfig                         │
│    SetActive + Boot Summary                             │
└───────────────────────────┬─────────────────────────────┘
                            │
                            ▼
┌─────────────────────────────────────────────────────────┐
│  internal/features  (LEAF)                              │
│    atomic active + resolveBool + numeric env            │
└───────┬───────────┬───────────┬───────────┬─────────────┘
        │           │           │           │
        ▼           ▼           ▼           ▼
     core/eval   cortex     world scan   CLI / UX
```

### Principles baked into the vision

1. **Leaf forever** — never import logging, config, core, or store.  
2. **Narrow flag set** — each flag is a marathon-era modernization concern, not a kitchen-sink remote-config product.  
3. **Two postures** — conservative compile defaults vs modern FullyEnabled seed.  
4. **Safe partials** — flags whose subsystems are incomplete (PerShardFacts) stay off in FullyEnabled until coordinators ship.  
5. **Testability** — every accessor re-reads env so `t.Setenv` works mid-test.

## 3. Target capabilities (near-term)

| Capability | Vision | Today |
|------------|--------|-------|
| Unified precedence | env > config > default for all flags | **Done** for bools and numerics |
| Boot visibility | always log Summary after load | **Done** in LoadUserConfig |
| Consumer parity | every accessor has a real call site | **Almost** (TaxonomyFast gap) |
| Operator inspect | CLI/TUI list of resolved flags | **Missing** |
| Comment/spec sync | one source of truth for defaults | **Partial** (stale core comments) |
| Migration of env prefixes | single prefix family | **Open** |

## 4. Non-goals

- Remote feature-flag services, percentage rollouts, user cohorts  
- Mangle-level feature predicates (`feature_enabled(...)`) unless a future design explicitly needs logic-side visibility  
- Replacing constitutional policy  
- Storing large configuration (models, limits, engines stay in `internal/config`)  
- Hot-reload from filesystem watches (explicit `SetActive` is enough)

## 5. Success metrics

- Zero import cycles involving features  
- New flag PRs always include field + accessor + default + seed decision + at least one consumer or explicit “registry-only temporary” note  
- Unit tests never accidentally run DiffEval-heavy paths unless opted in  
- Boot triage can answer “what flags were live?” from one log line  
