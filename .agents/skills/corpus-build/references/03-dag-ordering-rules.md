# DAG Ordering Rules (codeNERD)

Build plans use leveled dependency DAGs. Work units within a level may run in parallel
(up to 3 concurrent worktrees). Levels execute sequentially.

## Levels

### Level 0 — Types, Decl, interfaces

Foundation types, Mangle `Decl`s, interfaces, constants. No same-level import cycles.

### Level 1 — Implementations

Go logic implementing Level 0 contracts. Policy rules that only need Level 0 Decl.

### Level 2 — Tests

Unit / integration tests for Levels 0–1.

### Level 3 — Wiring surfaces

Shard registration intents, VirtualStore routes, CLI registration, prompt atom wiring,
MCP/tool registration. Prefer **intent files** for reserved hubs.

### Level 4 — Verification

`verify_surfaces.py`, serial full-package gates, doc status reconcile.

## Reserved-file pattern

When multiple WUs would edit the same hub, builders write:

```
.corpus-build/intents/<WU-NNN>_intents.json
```

```json
{
  "work_unit": "WU-003",
  "intents": [
    {
      "target_file": "internal/shards/registration.go",
      "action": "register_shard",
      "content": "// registration snippet"
    }
  ]
}
```

Level 4 wiring-auditor applies intents serially.

### Default reserved hubs

| File | Reason |
|------|--------|
| `internal/shards/registration.go` | Shared registration |
| `internal/core/virtual_store.go` | Route table |
| `internal/core/virtual_store_routing.go` | Route table split |
| `internal/core/defaults/schemas.mg` | Global Decl surface |
| `cmd/nerd/main.go` / command registration files | CLI hub |
| Shared prompt indexes if present | Atom catalog |

## Cycle detection

Run `python scripts/build_dag.py <plan.json> --json`. Cycles fail the plan.
