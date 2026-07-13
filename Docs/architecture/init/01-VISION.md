# init — Vision

> Last verified: 2026-07-13

## Product intent

`nerd init` is the **cold-start ceremony** for a workspace: one command that turns an ordinary repository into a codeNERD-aware project with:

1. A **`.nerd/` control plane** (config, sessions, mangle overlays, tools, agents, campaigns).
2. A **project identity** (`profile.json` + `profile.mg`) the kernel and prompts can bind against.
3. **Specialist memory** (shared concepts + Type-3 agent KBs + core coder/reviewer/tester KBs).
4. **Prompt JIT substrate** (corpus.db, agent `prompts.yaml`, reload into knowledge DBs).
5. **Session continuity hooks** (`session.json`, `sessions/*.json`) used by chat.

Vision is not “run another LLM chat wizard.” Vision is **deterministic scaffolding + optional LLM enrichment**.

## Target user journeys

### A. First contact

```
cd my-project
nerd init
# → .nerd/ exists, profile language detected, KBs seeded
nerd chat
# → Cortex boots with world facts + agents ready
```

### B. Refresh without full re-init

```
nerd scan
# → reload topology facts + profile.mg into kernel / world snapshot
```

### C. Force upgrade

```
nerd init --force
# → migrate schemas, append atoms (upgrade mode), refresh profile
```

### D. Operator hygiene

```
nerd init --cleanup-backups
# → remove migration backups after validation
```

## Architectural vision

| Concern | Target behavior |
|---------|-----------------|
| Detection | Multi-language, monorepo-aware (2-level glob), lockfile transitive deps |
| Agents | Language/framework/dep specialists + optional user-defined Type U |
| Knowledge | Base atoms always; research via modular tools when keys present |
| Logic | Profile facts + optional doc_ingestion facts; user mangle templates |
| Progress | 22-phase ETA-aware progress for TUI/CLI consumers |
| Failure | Prefer warnings over hard-fail when enrichment fails |
| JIT | Init-phase prompt atoms; defer deep research to session Executor |

## Non-goals

- Replacing full Cortex boot / OODA loop.
- Enforcing `permitted(...)` on every file write during init.
- Being the long-term home for domain shard implementations (researcher/tool_generator removed by design).
- Inventing sibling-platform-style product surfaces (foreign-product-surface, etc.).

## Success criteria

1. Uninitialized workspace becomes `IsInitialized` true (`profile.json` present).
2. Profile language/deps accurate enough to recommend correct agents for major stacks (Go, Python, TS, Rust, Kotlin).
3. Agent KBs pass `ValidateAllAgentDBs` tables + minimum atom heuristics.
4. Chat can load session state and report initialized status without re-running init.
5. Force re-init upgrades rather than blindly destroying learned preferences (CLI messaging; preferences path preserves when not wiped by operator).
