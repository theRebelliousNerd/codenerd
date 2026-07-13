# config — Architecture Corpus (`internal/config`)

> Last verified against codebase: 2026-07-13  
> Status: Living Reference Document  
> Language: Go (module `codenerd`)  
> Primary package: `internal/config/`  
> Scale: **17** non-test Go files ≈ **3.1k** lines; **5** test files ≈ **1.6k** lines; **0** `.mg`

## Scope

This corpus documents the **configuration substrate** for codeNERD:

1. **`UserConfig`** — the live single source of truth loaded from `.nerd/config.json`
2. **`Config`** — legacy YAML aggregate (`Load`/`Save`) still used on some boot paths
3. **Engines** — `api` | `claude-cli` | `codex-cli` | `xai-oauth` (+ provider keys, worker/image/Ollama blocks)
4. **Limits & schedulers** — `CoreLimits`, `APISchedulerPolicy`, global `LLMTimeouts`
5. **Load paths** — workspace root discovery, env overrides, feature-flag install into `internal/features`

It is **not** the CLI surface (`Docs/architecture/cli/`), **not** the kernel (`Docs/architecture/core/`), and **not** product Spec templates (`Docs/Spec/`).

## Document map

| Doc | Role |
|-----|------|
| [IMPLEMENTED_SPEC.md](IMPLEMENTED_SPEC.md) | Authoritative living architecture + inventory + deep dives |
| [00-ALIGNMENT-VISION-REVIEW.md](00-ALIGNMENT-VISION-REVIEW.md) | North-star alignment scores with evidence |
| [01-VISION.md](01-VISION.md) | Target architecture for configuration |
| [02-CURRENT-STATE.md](02-CURRENT-STATE.md) | Precise on-disk inventory and hotspots |
| [03-GAP-ANALYSIS.md](03-GAP-ANALYSIS.md) | Spec vs reality, priorities, non-gaps |
| [04-ARCHITECTURAL-PRINCIPLES.md](04-ARCHITECTURAL-PRINCIPLES.md) | Binding principles for this package |
| [05-INTERNAL-ARCHITECTURE.md](05-INTERNAL-ARCHITECTURE.md) | Components, data flow, load/merge model |
| [06-PUBLIC-API-AND-TYPES.md](06-PUBLIC-API-AND-TYPES.md) | Exported types and helpers with file refs |
| [07-DEPENDENCY-MAP.md](07-DEPENDENCY-MAP.md) | Upstream/downstream packages |
| [08-WIRING-AND-INTEGRATION.md](08-WIRING-AND-INTEGRATION.md) | Boot, CLI, chat, system factory wiring |
| [09-SAFETY-AND-INVARIANTS.md](09-SAFETY-AND-INVARIANTS.md) | Config-is-boss, allowlists, concurrency ceilings |
| [10-TESTING-ALIGNMENT.md](10-TESTING-ALIGNMENT.md) | Tests, gaps, commands |
| [11-OBSERVABILITY.md](11-OBSERVABILITY.md) | Boot logs, debug categories, LLM I/O traces |
| [12-FAILURE-MODES.md](12-FAILURE-MODES.md) | Concrete failures + mitigations |
| [TODO.md](TODO.md) / [OPEN-QUESTIONS.md](OPEN-QUESTIONS.md) / [_progress.md](_progress.md) | Governance |

### Legacy filenames (redirects)

Earlier thin stubs used different names. Prefer the map above. Redirect stubs may remain for:

- `01-DOMAIN-MODEL.md` → [06-PUBLIC-API-AND-TYPES.md](06-PUBLIC-API-AND-TYPES.md)
- `02-CURRENT-STATE-CONFIG.md` → [02-CURRENT-STATE.md](02-CURRENT-STATE.md)
- `03-GAP-ANALYSIS-CONFIG.md` → [03-GAP-ANALYSIS.md](03-GAP-ANALYSIS.md)
- `04-INVARIANTS-AND-GATES.md` → [09-SAFETY-AND-INVARIANTS.md](09-SAFETY-AND-INVARIANTS.md)
- `05-CROSS-SYSTEM-WIRING.md` → [08-WIRING-AND-INTEGRATION.md](08-WIRING-AND-INTEGRATION.md)
- `06-TESTING-STRATEGY.md` → [10-TESTING-ALIGNMENT.md](10-TESTING-ALIGNMENT.md)
- `08-FAILURE-MODES.md` → [12-FAILURE-MODES.md](12-FAILURE-MODES.md)

## Role in fact-flow

```
user / env / .nerd/config.json
        │
        ▼
  internal/config  (UserConfig + Get* resolvers)
        │
        ├─► features.SetActive  (process-wide flags)
        ├─► perception clients  (provider, engine, keys)
        ├─► core limits / API scheduler
        ├─► JIT token budgets
        ├─► embedding / world / execution allowlists
        └─► logging categories / transparency / onboarding

user_intent → kernel → next_action → VirtualStore
   ▲ config does not decide actions; it supplies budgets, backends, and gates
```

## Verify

```powershell
go test ./internal/config/...
# reverse consumers (sample)
rg "codenerd/internal/config" -g "*.go" --stats
```

## Quality bar

Modeled on `Docs/architecture/cli/`: real file inventories, control-flow diagrams, wiring journals, honest dual-config history, and package-specific invariants — **not** auto-generated inventory stubs.
