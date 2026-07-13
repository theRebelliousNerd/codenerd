# sqlpragmas — Wiring and Integration

> Last verified: **2026-07-13**  
> How the package is registered/called across boot, CLI, stores, and tools.

## Registration model

**There is no registration.** No `init()`, no kernel hook, no shard plugin, no VirtualStore route.

Wiring is **call-site discipline**: after `sql.Open`, invoke `ApplyDefaultPragmas`.

---

## Integration patterns

### Pattern A — Store package internals

Files under `internal/store/` call the re-exported `ApplyDefaultPragmas` without a package prefix (same package) or via the façade.

| File | Profile |
|------|---------|
| `local_core.go` | Hot |
| `learned_store.go` | Hot |
| `learning.go` | Hot |
| `tool_store.go` | Hot |
| `embedded_store.go` | ReadOnly |
| `migrations.go` | BulkBuild, Query |

These paths feed LocalStore / knowledge / learning used by Cortex boot.

### Pattern B — Mid-layer direct leaf (anti-cycle)

| Package | File | Profile | Why direct |
|---------|------|---------|------------|
| mcp | `store.go` | Hot | Avoid store cycle / coupling |
| prompt | `loader.go`, `compiler_db.go`, … | Hot / Bulk | Prompt stack independence |
| core | `predicate_corpus.go` | ReadOnly | Kernel-adjacent corpus |
| system | `factory.go`, `agent_registry.go` | Hot / Bulk / Query | Boot factory |
| init | `profile.go`, `validation.go` | Bulk / Query | Workspace init |
| northstar, context, perception, autopoiesis | various | Hot | Domain stores |

### Pattern C — CLI / tools

| Binary path | File | Profile |
|-------------|------|---------|
| Interactive boot | `cmd/nerd/chat/session_boot.go` | Hot |
| Chat ingest | `cmd/nerd/chat/ingest.go` | BulkBuild |
| corpus_builder | `cmd/tools/corpus_builder/main.go` | BulkBuild |
| prompt_builder | via `store` | BulkBuild |
| predicate_corpus_builder | via `store` | BulkBuild |
| query-kb | via `store` | Query |

---

## Boot path (chat)

```
nerd (interactive)
  → chat session boot
  → open project / system DBs
  → sqlpragmas.ApplyDefaultPragmas(..., ProfileHot)   // e.g. session_boot.go
  → factory may open more DBs with Hot / BulkBuild
  → stores (local_core, learned, …) also Apply Hot
  → OODA loop proceeds on already-tuned handles
```

sqlpragmas is **not** invoked per user turn; only at open.

## VirtualStore / kernel

No direct wiring. VirtualStore effects that touch SQLite go through store implementations that already applied pragmas at construction.

## Shards

Shard learning / persistence that uses `store` learning APIs inherits Hot profile from those openers. No shard-specific profile.

## Mangle / policy

None.

## Prompt JIT

`internal/prompt/compiler_db.go` and loaders apply Hot (or Bulk for embedding load) so atom DBs match agent-store posture.

## Config / features

No feature flag gates pragma application. Always-on when the open site calls it.

---

## Wiring checklist for new SQLite open sites

1. Choose profile (Hot / BulkBuild / Query / ReadOnly).  
2. If package can import `store` without cycles → `store.ApplyDefaultPragmas` is fine.  
3. If package is mid-layer or cycles with store → `sqlpragmas.ApplyDefaultPragmas`.  
4. Call immediately after successful `sql.Open`.  
5. Do not rely on sqlpragmas for FKs or migrations.  
6. Prefer tempfile + single-conn when writing tests that assert PRAGMA values.

## Anti-patterns

| Anti-pattern | Consequence |
|--------------|-------------|
| Open without Apply | Default SQLite journal/cache; worse concurrency |
| Copy-paste PRAGMA lists | Drift from leaf |
| Import store only for pragmas from mcp-like package | Cycle risk |
| Use Hot on pure RO `mode=ro` | Debug spam / failed journal PRAGMAs |
| Use ReadOnly then write | App error; not pragma package’s job to prevent |

## Verification of wiring

```powershell
rg "sql\.Open\(" -g "*.go" --glob "!*_test.go" -A 5
# Manually confirm ApplyDefaultPragmas within a few lines (or justified exception)
```
