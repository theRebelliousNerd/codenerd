# core — Mangle Surface Deep-Dive

> Last verified: **2026-07-13**  
> Sources: `internal/core/defaults/**/*.mg` loaded by `kernel_init.go` `loadMangleFiles`  
> This is the **constitution + executive logic** embedded into every nerd binary.

## 1. Load pipeline

```
//go:embed defaults/*.mg defaults/schema/*.mg defaults/policy/*.mg
                │
                ▼
loadMangleFiles()
  1. schemas.mg (index)
  2. schemas_*.mg + chaos.mg modules (ordered list)
  3. ALL defaults/policy/*.mg (directory iteration)
  4. Optional root modules (taxonomy, jit_compiler, reviewer, …)
  5. embedded intent facts
  6. defaults/learned.mg
  7. User: .nerd/mangle/{extensions,policy_overrides,learned}.mg (hybrid)
                │
                ▼
rebuildProgram: schemas + policy + learned → parse → analyze → stratify
```

**Critical:** Policy directory failure is fatal (no monolithic fallback).

## 2. Schemas modular map

Index file: `defaults/schemas.mg` (documentation + quick reference).

| Module file | Domain | Notable Decl families |
|-------------|--------|------------------------|
| `schemas_intent.mg` | Intent | `user_intent`, focus |
| `schemas_world.mg` | World model | `file_topology`, `symbol_graph`, diagnostics |
| `schemas_execution.mg` | Actions | `next_action`, TDD/action envelopes |
| `schemas_browser.mg` | Browser | DOM / spatial |
| `schemas_project.mg` | Project/session | profile, preferences, checkpoints |
| `schemas_dreamer.mg` | Speculative | dream_state, projected_*, panic-related |
| `schemas_memory.mg` | Memory tiers | activation, knowledge links |
| `schemas_knowledge.mg` | Knowledge/LSP | atoms, semantic match |
| `schemas_learning.mg` | Learning | exemplars, overrides |
| `schemas_state.mg` | Ouroboros state | tool gen state machine |
| `chaos.mg` | Adversarial | nemesis / panic testing |
| `schemas_safety.mg` | Constitution | `permitted`, dangerous_*, appeals |
| `schemas_analysis.mg` | Strategy/impact | activation scores, impact |
| `schemas_misc.mg` | Northstar/bench | goals, benchmarks |
| `schemas_codedom.mg` | Code DOM | elements, signatures |
| `schemas_codedom_polyglot.mg` | Languages | polyglot facts |
| `schemas_testing.mg` | Verification | traces, pytest |
| `schemas_campaign.mg` | Campaigns | campaign/task/phase |
| `schemas_intelligence.mg` | Campaign intel | context intelligence |
| `schemas_tools.mg` | Tools/ouroboros | registration, routing |
| `schemas_mcp.mg` | MCP | MCP decls |
| `schemas_prompts.mg` | JIT prompts | atoms, selection scores |
| `schemas_reviewer.mg` | Review | findings, dataflow |
| `schemas_shards.mg` | Delegation | shard_profile, results |
| `schemas_coder.mg` | Coder domain | coder-specific decls |
| `schemas_context.mg` | Context compile | compilation pipeline |

### Safety Decl highlights (`schemas_safety.mg`)

```mangle
Decl permitted(ActionType, Target, Payload) bound [/name, /string, /string].
Decl forbidden(ActionType) bound [/name].
Decl dangerous_action(ActionType) bound [/name].
Decl dangerous_content(ActionType, Payload) bound [/name, /string].
Decl admin_override(User) bound [/string].
Decl signed_approval(ActionType) bound [/name].
# + appeal / temporary_override family
```

### Execution Decl highlights

`next_action`, `pending_action`, `action_executed`, tool invocation envelopes — see `schemas_execution.mg` / `schemas_shards.mg` for permission-check pipeline decls referenced by `system_core.mg`.

## 3. Policy modular map

All `defaults/policy/*.mg` are loaded. Grouped for humans:

### 3.1 Safety & gates

| File | Role |
|------|------|
| `constitution.mg` | `permitted` / `safe_action` / dangerous paths |
| `git_safety.mg` | Commit/history protection |
| `codedom_safety.mg` | Element edit safety |
| `dreamer.mg` | `panic_state` / `dream_block` |
| `shadow_mode.mg` | Simulation rules |
| `commit_gate.mg` | Commit gating |
| `browser_honeypot.mg` | Honeypot detection |
| `validation.mg` / `verification.mg` | Validation loops |

### 3.2 System OODA / routing

| File | Role |
|------|------|
| `system_core.mg` | Intent pending, permission pipeline helpers |
| `system_ooda.mg` | OODA stage rules |
| `system_routing.mg` | Routing outcomes |
| `system_session.mg` | Session lifecycle |
| `system_shards.mg` | System shard coordination |
| `system_world.mg` | World update rules |
| `system_config.mg` | Config-driven flags |
| `system_autopoiesis.mg` | Self-mod rules |
| `routing_arbitration.mg` | Competing routes |
| `tool_routing.mg` | Tool selection |
| `delegation.mg` | Delegate intents |
| `perception_routing.mg` | Perception→intent bridges |

### 3.3 Campaign

`campaign_core.mg`, `campaign_phases.mg`, `campaign_tasks.mg`, `campaign_planning.mg`, `campaign_context.mg`, `campaign_autopoiesis.mg`.

### 3.4 Coder domain

`coder_*.mg` — build, campaign, classification, context, diagnostics, impact, language, learning, observability, patterns, quality, safety, tdd, workflow.

### 3.5 JIT / prompts

`jit_logic.mg`, `jit_selection.mg`, `jit_config.mg`, `prompt_context.mg`, `prompt_northstar.mg`, `context_compilation.mg`.

### 3.6 TDD / impact / intelligence

`tdd_logic.mg`, `tdd_loop.mg`, `test_impact.mg`, `impact.mg`, `intelligence.mg`, `strategy.mg`, `prioritization.mg`, `activation.mg`.

### 3.7 Other

`autopoiesis.mg`, `learning.mg`, `knowledge.mg`, `browser.mg`, `bridge.mg`, `capabilities.mg`, `clarification.mg`, `data_flow.mg`, `shards.mg`, `taxonomy_*.mg`, `trace_logic.mg`, `codedom_*.mg`.

### 3.8 Root modules folded into policy

`doc_taxonomy.mg`, `topology_planner.mg`, `build_topology.mg`, `campaign_rules.mg`, `selection_policy.mg`, `taxonomy.mg`, `inference.mg`, `jit_compiler.mg`, `reviewer.mg`, `tester.mg`, `go_safety.mg`, `benchmarks.mg`.

## 4. Constitution deep-dive

### Safe actions (examples)

File ops (`/read_file`, `/write_file`, `/glob`, …), analysis, review, tests, knowledge, browser read-ish ops, campaign verbs, TDD repair verbs, ouroboros pipeline verbs, `/ask_user`, lifecycle `/heartbeat`, etc.

### Default permit rule (conceptual)

```mangle
permitted(Action, Target, Payload) :-
    safe_action(Action),
    pending_action(_, Action, Target, Payload, _),
    !dangerous_content(Action, Payload),
    !dangerous_content(Action, Target).
```

### Deny documentation rules

`permission_denied(Action, "Dangerous Action")` when dangerous without admin/signed approval.

### Bridge

`permitted` also derives from executor-facing `permitted_action` + `permission_check_result(..., /permit, ...)`.

## 5. Dreamer policy deep-dive

`defaults/policy/dreamer.mg`:

```mangle
critical_file("go.mod").
critical_file("go.sum").

panic_state(Action, "critical_file_missing") :-
    projected_fact(Action, /file_missing, File),
    critical_file(File).

panic_state(Action, "dangerous_exec") :-
    projected_fact(Action, /exec_danger, _).

panic_state(Action, "deletes_tested_symbol") :-
    projected_fact(Action, /file_missing, _),
    projected_fact(Action, /impacts_test, _).

panic_state(Action, "critical_path_missing") :-
    projected_fact(Action, /critical_path_hit, _).

dream_block(Action, Reason) :-
    panic_state(Action, Reason).
```

Go Dreamer must emit matching `projected_fact` atoms (`dreamer.go` `projectEffects`).

## 6. System core pipeline (executive)

`system_core.mg` sketches:

```
user_intent → pending_intent → intent_ready_for_executive
pending_action → pending_permission_check → permission_check_result
  → action_permitted | action_blocked
  → action_ready_for_routing → routing_result
```

Helpers exist specifically for **safe negation** (bound predicates instead of `!foo(_,_)`).

## 7. Intent schema tree

`defaults/schema/` modular intent corpus:

- `intent.mg`, `intent_index.mg`, domain files (`intent_code_mutations.mg`, `intent_testing.mg`, …)  
- `intent_routing.mg` — persona/routing related  
- `prompts.mg` — prompt-related schema pieces  

Hybrid loader can inject intents as boot data for classification.

## 8. Predicate corpus DB

`predicate_corpus.go` + `predicate_corpus.db` bake Decl metadata (args, error patterns, examples) for validation / tooling — complementary to raw `.mg` text.

## 9. Authoring rules for this surface

1. **Decl first** in the correct `schemas_*.mg` module; ensure module is in `schemaFiles` list if new file.  
2. **Rules** in `policy/*.mg` — never invent predicates without Decl.  
3. **Variables** Uppercase; **name constants** `/atom`.  
4. **Negation safety** — bind first; use helper predicates (`permission_checked`, `has_active_shard` style).  
5. **Do not** put fuzzy NLP regex farms in Mangle.  
6. **Goldens** under `policy/testdata` for safety-critical rules.  
7. **User overrides** go in `.nerd/mangle`, not by editing embed casually without rebuild.  
8. On failure, inspect `debug_program_ERROR.mg`.

## 10. Interaction with Go executive

| Mangle concept | Go consumer |
|----------------|-------------|
| `permitted/3` | `VirtualStore.CheckKernelPermitted` |
| `safe_action/1` | Permission cache rebuild |
| `next_action` | Session/orchestration queries |
| `panic_state` | Dreamer evaluateProjection |
| `projected_action` / `projected_fact` | Dreamer projectEffects |
| `critical_path_prefix` | Dreamer assertCriticalPathFacts |
| `execution_result` | VS inject after handlers |
| `security_violation` | VS on deny |
| prompt/JIT selection preds | `internal/prompt` compiler |

## 11. Size & maintenance pressure

The concatenated program is large. Operational consequences:

- Boot parse/analyze cost  
- Higher chance of Decl conflicts when adding modules  
- Need modular ownership discipline  

Prefer small surgical rule files over new monoliths.

## 12. Testing the surface

```powershell
go test ./internal/core/defaults/policy/...
go test ./internal/core/ -run 'NewRealKernel|LoadPolicy|HotLoad'
```

Golden pairs are the contract for safety-critical derivations.

## 13. What this surface is not

- Not the prompt atom library (`internal/prompt/atoms`)  
- Not domain shard free-text system prompts  
- Not the sqlite knowledge graph schema  

It is the **logical constitution and executive derivation layer** of codeNERD.
