# 07 — Dependency Map (perception)

> Last verified: **2026-07-13**

## Upstream (perception imports)

| Package | Why |
|---------|-----|
| `codenerd/internal/types` | `LLMClient` alias, session context, message/tool types |
| `codenerd/internal/config` | User config, engine, Gemini/Ollama/worker/CLI blocks |
| `codenerd/internal/core` | Kernel interface, default schemas, facts |
| `codenerd/internal/mangle` | Taxonomy dedicated engine |
| `codenerd/internal/embedding` | Query embeddings for semantic classify |
| `codenerd/internal/logging` | CategoryPerception timers/logs |
| `codenerd/internal/articulation` | Piggyback type aliases + schema constant |
| `codenerd/internal/store` | Learned corpus backend |
| `codenerd/internal/sqlpragmas` | SQLite cache pragmas for intent embeddings |
| `github.com/mattn/go-sqlite3` | Embedding cache DB |
| `golang.org/x/sync/errgroup` | Parallel corpus search |
| stdlib `net/http`, `os/exec`, etc. | Providers, CLI engines |

**Subpackage:** root perception imports `perception/xaioauth` for OAuth engine construction.  
`xaioauth` is intentionally **independent** of `XAIClient` API-key path.

## Downstream (who imports perception)

Evidence from greps (non-exhaustive):

| Consumer | Usage |
|----------|--------|
| `cmd/nerd/chat/*` | Transducer, clients, campaign assault verbs |
| `cmd/nerd/cmd_auth.go` | Auth + xaioauth |
| `cmd/nerd/cmd_direct_actions.go` | Direct action perception |
| `cmd/nerd/cmd_campaign.go` | Campaign LLM |
| `cmd/nerd/cmd_init_scan.go` | Init/scan perception hooks |
| `internal/session` (via tests & boot) | Executor transducer |
| `tests/e2e/*` | Many mock transducers + real UnderstandingTransducer contracts |
| `cmd/tools/verify_taxonomy` | Taxonomy verification |

## Layering constraints

```
cmd/nerd ──► perception ──► config / core / embedding / articulation / store
                │
                └──► mangle (taxonomy only; not full Cortex program load)

session / shards ──► perception.LLMClient / TracingLLMClient
```

Avoid:

- perception importing `cmd/*`  
- perception importing domain showcase apps  
- circular: articulation already depended for schema; perception must not pull full prompt compiler graph into hot client files if avoidable

## Data dependencies (on-disk)

| Path | Role |
|------|------|
| `.nerd/config.json` | Provider/engine/keys/models |
| `.nerd/intent_embeddings.db` | Cached intent embeddings |
| `.nerd/mangle/learned_taxonomy.mg` | Learned taxonomy rules |
| `~/.nerd/xai_oauth.json` | OAuth credentials |
| `~/.grok/auth.json` | Optional import source for xaioauth |

## Mangle surface (external to package files)

Perception **consumes** embedded content via `core.GetDefaultContent`:

- `schemas_intent.mg`, learning schemas, `policy/taxonomy_*.mg`  
- Routing predicates expected at runtime: `mode_from_*`, `shard_affinity_*`, `context_affinity_*`, etc.  
- Asserted: `semantic_match`, `user_intent`, `current_understanding`, `derived_*`

Full Decl ownership lives in `internal/core/defaults/` policy corpus — not this package.

## Coupling heat map

| Coupling | Intensity | Notes |
|----------|-----------|-------|
| config ↔ factory | High | Single path for engines |
| embedding ↔ classifier | High | Optional degrade |
| core kernel ↔ inject/routing | High | Facts + QueryRouting adapter |
| articulation ↔ types/schema | Medium | Aliases + Piggyback schema |
| chat process ↔ transducer | High | Every turn |
