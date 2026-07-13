# 07 — Dependency Map: features

> Last verified against codebase: **2026-07-13**

## 1. Upstream (what features depends on)

```
internal/features
    └── stdlib: fmt, os, sync/atomic
```

**Zero** module-internal dependencies. This is intentional and must remain true.

External test package (`features_test`) additionally imports:

- `codenerd/internal/config` — only for LoadUserConfig round-trip  
- `github.com/stretchr/testify/require`

Those imports are **outside** `package features` to avoid cycles.

## 2. Downstream (who depends on features)

### Direct Go imports of `codenerd/internal/features`

| Package / file | Direction of control |
|----------------|----------------------|
| `internal/config/user_config.go` | **Writer** — SetActive, Summary, type embed, FullyEnabled seed |
| `internal/core/kernel_eval.go` | **Reader** — DiffEval |
| `internal/core/cortex_kernel.go` | **Reader** — PerShardFacts |
| `internal/core/kernel_features_test.go` | **Reader/Writer** — test SetActive |
| `internal/world/scanner_config.go` | **Reader** — scan tunables |
| `internal/ux/migration.go` | **Reader** — SkipOnboarding |
| `cmd/nerd/main.go` | **Reader** — FlightRecorder (after GlobalConfig load) |
| `cmd/nerd/chat/session_boot.go` | **Reader** — Provenance, SystemShards |
| `cmd/nerd/ui/styles.go` | **Reader** — DarkMode |

### Soft / comment references

| Location | Note |
|----------|------|
| `internal/shards/registration.go` | Mentions `IsPerShardFactsEnabled` in comments for Track D |
| `internal/core/shard_fact_router_test.go` | Comments about flag-off cortex (uses SetFactRouter for tests) |

### Env-only consumer (does **not** import features)

| Location | Behavior |
|----------|----------|
| `cmd/tools/verify_taxonomy/main.go` | `os.Getenv("CODENERD_TAXONOMY_FAST") == "1"` |

## 3. Layering diagram

```
                    ┌──────────────┐
                    │  cmd/nerd    │
                    │  chat / ui   │
                    └──────┬───────┘
                           │ reads Is*
              ┌────────────┼────────────┐
              │            │            │
              ▼            ▼            ▼
        ┌──────────┐ ┌──────────┐ ┌──────────┐
        │  core    │ │  world   │ │   ux     │
        └────┬─────┘ └────┬─────┘ └────┬─────┘
             │            │            │
             └────────────┼────────────┘
                          │ reads
                          ▼
                   ┌─────────────┐
                   │  features   │ ◄── writes ── internal/config
                   │   (LEAF)    │
                   └─────────────┘
```

## 4. Why not live under config?

If `IsDiffEvalEnabled` lived in `internal/config`, then `internal/core` would import config. Config already reaches toward store and broader app config — historical cycle risk is exactly why features was extracted (package comment L1–13).

## 5. Cyclic risk checklist

| Proposed change | Risk |
|-----------------|------|
| features → logging | Breaks leaf; config already logs for you |
| features → core | Immediate cycle with kernel_eval |
| config → features | **Allowed** (current) |
| core → features | **Allowed** (current) |
| features → config | **Forbidden** — use external `_test` package for boundary tests |

## 6. Related architecture corpora

| Corpus | Relationship |
|--------|----------------|
| `Docs/architecture/config/` | Sole production installer |
| `Docs/architecture/core/` | Primary semantic consumers |
| `Docs/architecture/cli/` | Boot-time consumers |
| `Docs/architecture/world/` | Numeric tunables |
| `Docs/architecture/observability/` | Flight recorder implementation (gated, not importer of features for logic — main imports both) |
