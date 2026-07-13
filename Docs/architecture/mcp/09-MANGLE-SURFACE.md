# 09 — Mangle Surface: MCP

> Last verified against codebase: 2026-07-13  
> Extra deep-dive (policy-heavy package)

## 1. Split of responsibility

| Artifact | Location | Role |
|----------|----------|------|
| **Decls (EDB + IDB names)** | `internal/core/defaults/schemas_mcp.mg` | Boot-loaded via `kernel_init.go` list entry `schemas_mcp.mg` |
| **Selection rules (IDB)** | `internal/mcp/policy_mcp.mg` | Section 50 hybrid selection — **package-local; not confirmed in policy load list** |
| **Runtime temp facts** | Go `JITToolCompiler` | `mcp_tool_vector_score` assert/retract |
| **Related non-MCP tool policy** | `internal/core/defaults/policy/tool_routing.mg` | Section 40 generic tool relevance |

## 2. Decl inventory (schemas_mcp.mg)

### Servers

| Predicate | Meaning |
|-----------|---------|
| `mcp_server_registered(ServerID, Endpoint, Protocol, RegisteredAt)` | Server known |
| `mcp_server_status(ServerID, Status)` | `/connected` etc. |
| `mcp_server_capabilities(ServerID, Capability)` | `/tools`, `/resources`, … |
| `mcp_server_name(ServerID, Name)` | Display name |

### Tools (EDB-ish)

| Predicate | Meaning |
|-----------|---------|
| `mcp_tool_registered(ToolID, ServerID, RegisteredAt)` | Discovered tool |
| `mcp_tool_name` / `description` / `condensed` | Text metadata |
| `mcp_tool_capability(ToolID, Capability)` | `/read`, `/write`, … |
| `mcp_tool_category(ToolID, Category)` | `/filesystem`, … |
| `mcp_tool_domain(ToolID, Domain)` | `/go`, `/general`, … |
| `mcp_tool_shard_affinity(ToolID, ShardType, Score)` | 0–100 |
| `mcp_tool_analyzed(ToolID)` | Analysis done |
| `mcp_tool_usage` / `last_used` / `success_rate` | Stats Decls |

### Derived / selection

| Predicate | Meaning |
|-----------|---------|
| `mcp_tool_available(ToolID)` | Available (connected/disconnected rules in policy) |
| `mcp_tool_vector_score(ToolID, Score)` | 0–100 from Go |
| `mcp_tool_base_relevance(ShardType, ToolID, Score)` | Affinity ≥ 30 |
| `mcp_tool_intent_boost` / `domain_boost` | Intent/language boosts |
| `mcp_tool_relevance(ShardType, ToolID, Score)` | Combined |
| `mcp_tool_selected(ShardType, ToolID, RenderMode)` | Final mode `/full`… |
| `mcp_tool_skeleton(ToolID)` | Always-selected classes |

Note: `intent_requires_capability/3` is declared in `schemas_tools.mg` and reused.

## 3. Policy rules summary (`policy_mcp.mg`)

### Availability

- Available if registered and server `/connected` **or** `/disconnected` (offline cached still “available” for selection of known tools).

### Base relevance

- From `mcp_tool_shard_affinity` with `Score >= 30` and available.

### Boosts

- Intent boost +30 when capability matches `user_intent` verb via `intent_requires_capability`.  
- Domain boost +20 when `file_topology` language matches tool domain; +10 for `/general`.

### Combined score

- With vector: `(Base+Intent+Domain)*0.7 + Vector*0.3` via integer `fn:mult/div`.  
- Without vector: logic sum only.

### Render assignment

| Score | Mode |
|------:|------|
| ≥ 70 | `/full` |
| 40–69 | `/condensed` |
| 20–39 | `/minimal` |

### Skeleton

- Category `/filesystem` + capability `/read`  
- Category `/search` + capability `/search`  
- Skeleton ⇒ always `/full` if available

### Static EDB in policy

Large `intent_requires_capability` table for verbs: read/view/show, write/create, search/find, run/execute/test/build, format/convert, delete/remove, etc.

## 4. Go query contract

```text
Query: mcp_tool_selected("<shardType>", ToolID, RenderMode)
```

Compiler accepts render modes with or without leading `/` (`full` / `/full`, case-insensitive). Default unknown → condensed.

## 5. What is **not** asserted by Go today

Grep of `.go` sources shows **no** bulk assert of:

- `mcp_tool_registered`  
- `mcp_tool_capability`  
- `mcp_tool_shard_affinity`  
- `mcp_server_status`  

on discover. Only **vector scores** are asserted during compile. Therefore policy body for base relevance **cannot fire** on real discovered tools until a fact emitter is added.

## 6. CLI mangle-check drift

`cmd/nerd/cmd_mangle_check.go` references `internal/mcp/schemas_mcp.mg`. Actual Decl file: `internal/core/defaults/schemas_mcp.mg`. Package-local policy remains `internal/mcp/policy_mcp.mg`.

## 7. Guardrails reminder

- Every predicate needs `Decl` before use.  
- Variables uppercase; atoms `/lowercase`.  
- Negation only after positive binding.  
- Aggregation uses `|> do … let …` (not used heavily in this section).  
- Do not push fuzzy NL matching into Mangle — embeddings first.

## 8. Recommended completion path

1. Move or include `policy_mcp.mg` in defaults policy load order (after schemas).  
2. On SaveTool / SaveServer, assert/retract corresponding EDB.  
3. Optionally assert usage predicates from `RecordToolUsage`.  
4. Golden tests under `internal/core/defaults/policy/testdata` for section 50.
