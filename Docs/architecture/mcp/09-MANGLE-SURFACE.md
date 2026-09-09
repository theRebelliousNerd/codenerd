# 09 — Mangle Surface: MCP

> Last verified against codebase: 2026-09-09  
> Extra deep-dive (policy-heavy package)

## 1. Split of responsibility

| Artifact | Location | Role |
|----------|----------|------|
| **Decls (EDB + IDB names)** | `internal/core/defaults/schemas_mcp.mg` | Boot-loaded via `kernel_init.go`; Sections 50.1–50.10 |
| **Selection + gating rules (IDB)** | `internal/core/defaults/policy/policy_mcp.mg` | Section 50 hybrid selection and control-plane gating; swept by the `defaults/policy/*.mg` embed |
| **Permanent EDB** | Go `FactEmitter` (`internal/mcp/facts.go`) | Server, tool, capability, affinity, usage, resource, prompt, facet, risk, handle |
| **Runtime temp facts** | Go `JITToolCompiler` | `mcp_tool_vector_score` assert/retract, by exact fact |
| **Verb permissions** | `internal/core/defaults/policy/constitution.mg` | `safe_action` for the five `mcp_*` verbs |
| **Verb routing** | `internal/core/defaults/policy/intent_routing_rules.mg` | `modular_tool_allowed` for the five verbs |
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

on discover.

**Fact emission is implemented.** `internal/mcp/facts.go` mirrors runtime state
into the kernel as EDB, subject-keyed so a re-analysis replaces a tool's facts
wholesale rather than layering new ones over stale ones. The kernel adapter
retracts by exact fact string — a wildcard retraction is a silent no-op there —
so every subject key remembers exactly which strings it asserted.

## 6. Control-plane predicates (Section 50.10)

Derived at discovery by `internal/mcp/facets.go`, emitted by `FactEmitter`, and
consumed by the gating rules in `policy_mcp.mg`.

| Predicate | Kind | Meaning |
|-----------|------|---------|
| `mcp_tool_facet(ToolID, Facet)` | EDB | `/read` `/search` `/analyze` `/write` `/execute` `/manage` |
| `mcp_tool_risk(ToolID, Risk)` | EDB | `/safe` `/mutating` `/destructive` `/arbitrary` |
| `mcp_tool_risk_source(ToolID, Source)` | EDB | `/annotation` `/capability` `/name` `/schema` `/default` |
| `mcp_result_handle(Handle, ToolID, Bytes)` | EDB | An outstanding, still-expandable shaped result |
| `mcp_tool_gated(ToolID)` | IDB | Requires `confirm_risk` before dispatch |
| `mcp_tool_browsable(ToolID)` | IDB | Safe enough for an unfiltered exploratory listing |
| `mcp_server_facet_available(ServerID, Facet)` | IDB | This server currently fills this facet |

Two design points worth keeping:

- **Risk source is carried, not discarded.** A server that declared
  `readOnlyHint` has told us the answer; a name-prefix guess is an inference. A
  rule that grants a write on the strength of a guess should be able to say so,
  and `mcp_tool_gated` uses exactly that distinction — `/mutating` with source
  `/default` is gated, because nothing said it was safe.
- **Go derives, Mangle decides.** Classification is mechanical and lives in Go;
  what a classification is *allowed* to do is policy. `KernelRiskGate` queries
  `mcp_tool_gated` and enforces the verdict, and a failed query is treated as
  gated rather than permitted.

## 7. Guardrails reminder

- Every predicate needs `Decl` before use.  
- Variables uppercase; atoms `/lowercase`.  
- Negation only after positive binding.  
- Aggregation uses `|> do … let …` (not used heavily in this section).  
- Do not push fuzzy NL matching into Mangle — embeddings first.

## 8. Verification

- `internal/mcp/policy_golden_test.go` loads the real `schemas_mcp.mg` and
  `policy/policy_mcp.mg` from disk and pins `mcp_tool_selected` against golden
  fixtures, so a Decl or rule edit that breaks stratification fails there.
- `internal/tools/catalog_policy_parity_test.go` fails if a registered tool has
  no `safe_action` or `requires_permission` fact — verified by deliberately
  removing `safe_action(/mcp_call)`, which produced
  `tools with no policy entry: [mcp_call]`.
- `internal/tools/catalog_golden_test.go` fails if a registered tool is absent
  from `modular_tool_allowed`.

All three hydrate `mcpctl.RegisterAll`, so the control-plane verbs are inside
the gates rather than beside them.
