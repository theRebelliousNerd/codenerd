# jit — Implemented Spec (Deep-Dive)

> Last verified against codebase: **2026-07-13**  
> Status: Living Reference Document  
> Language: Go  
> Module: `codenerd`  
> Primary sources: `internal/jit/config/`  
> Scale: **1** non-test Go file ≈ **70** lines; **1** test file ≈ **91** lines; **0** Mangle sources
> Ecosystem partners: `internal/prompt` (ConfigFactory, JIT compiler), `internal/session` (universal executor)

## 1. Overview

`internal/jit` is a **schema-only package** that defines the effective runtime configuration for JIT-driven agents in codeNERD. After the December 2024–era removal of hardcoded domain shards (CoderShard, TesterShard, …), specialization is produced at runtime:

1. **Intent** (perception / Mangle routing) selects persona behavior.  
2. **Prompt JIT** assembles identity and behavioral text from atoms.  
3. **ConfigFactory** assembles tools plus canonical policy paths resolved from
   stable core policy-set IDs.
4. Both land in **`EffectiveAgentRuntimeConfig`**.  
5. **Session Executor** runs one universal loop for every persona.

This package owns step 4’s **type system**. It does not compile prompts, execute tools, query Mangle, or boot Cortex.

### Key characteristics

| Property | Value |
|----------|-------|
| Import path | `codenerd/internal/jit/config` |
| Go package name | `config` (disambiguate from `internal/config`) |
| Public types | 4 structs + 1 method |
| Side effects | None |
| YAML specialist path | `.nerd/agents/<name>/config.yaml` |
| Primary consumers | `internal/prompt`, `internal/session` |
| North-star role | Typed handshake: creative identity ↔ executive bounds |

### High-level control flow

```
user input → perception Intent
                │
                ▼
     ┌──────────────────────┐
     │ JIT Prompt Compiler  │── Prompt text
     └──────────────────────┘
                │
                ▼
     ┌──────────────────────┐
     │ ConfigFactory        │── tools, policies, defaults
     └──────────┬───────────┘
                ▼
     EffectiveAgentRuntimeConfig   ← internal/jit/config
                │
                ▼
     Session Executor tool loop
                │
                ▼
     VirtualStore / tools (+ permitted / Dreamer gates)
```

Fact-flow remains:

```
user_intent → kernel next_action / permitted
  → JIT config + prompt → tools → articulation
```

---

## 2. Implementation status

| Component | Status | Notes |
|-----------|--------|-------|
| `EffectiveAgentRuntimeConfig` schema | **Implemented** | Stable, flat YAML tags |
| Nested ToolLoop / Safety / Workspace | **Implemented** (schema) | Partial runtime consumption |
| `Validate()` identity + canonical policies | **Implemented** | Rejects empty, noncanonical, missing, traversal-shaped, whitespace, and duplicate references |
| ConfigFactory population | **Implemented** | Lives in `internal/prompt` |
| Session tool allowlist | **Implemented** | `AllowedTools` enforced |
| Specialist YAML load | **Implemented and verified** | Spawner validates after bounded/path-contained unmarshal; hostile regressions cover blank identity, missing policies, traversal, and size |
| ToolLoop → executor | **Partial / unwired** | ExecutorConfig owns limits |
| Policies → kernel load | **Partial** | Canonical global boot-corpus members carried; no selective per-agent load or set version |
| RequirePolicyEnforcement flag | **Schema only** | Always set true by factory |
| Model / Workspace fields | **Schema only** | Optional YAML |
| Local Mangle | **N/A** | |
| Pre-implementation zero | **False** | Living production schema |

**Overall:** living, load-bearing **contract package** — small surface, large fan-out.

---

## 3. Source inventory

### 3.1 Package layout

```
internal/jit/
  config/
    types.go        # all types + Validate
    types_test.go   # Validate table tests
```

### 3.2 Files

| Path | Lines | Purpose |
|------|------:|---------|
| `internal/jit/config/types.go` | 70 | Schema + core-inventory validation |
| `internal/jit/config/types_test.go` | 91 | Unit tests |

### 3.3 Exported symbols (complete)

| Symbol | Kind | Location |
|--------|------|----------|
| `EffectiveAgentRuntimeConfig` | type | `types.go:14` |
| `ToolLoopConfig` | type | `types.go:26` |
| `SafetyConfig` | type | `types.go:32` |
| `WorkspaceConfig` | type | `types.go:36` |
| `Validate` | method | `types.go:52` |

---

## 4. Deep dive: `EffectiveAgentRuntimeConfig`

### 4.1 Purpose comment (source truth)

The struct “defines the configuration for a JIT-driven dynamic agent” and “maps the output of the JIT compiler to the Universal Executor.” YAML tags use snake_case so specialist files under `.nerd/agents/<name>/config.yaml` match natural YAML conventions.

### 4.2 Field semantics

#### Identity & routing

| Field | Semantics |
|-------|-----------|
| `IdentityPrompt` | System-facing identity / persona text. Required by Validate. Factory sets from `CompilationResult.Prompt`. |
| `IntentVerb` | Primary intent key (e.g. `/fix`). Metadata and factory primary intent. |
| `Persona` | Optional persona label for YAML specialists. |
| `Model` | Optional model override string; not the main multi-engine selector (`internal/config` / shared LLM client dominate). |

#### Capability envelope

| Field | Semantics |
|-------|-----------|
| `AllowedTools` | Names offered to the LLM and checked before modular or Ouroboros execution. Empty is `Validate`-legal but deny-all at the session capability gate. |
| `Policies` | Canonical paths in core's embedded default inventory. Validate requires ≥1 unique member. Default factories resolve stable persona set IDs, always including `policy/constitution.mg` and `policy/validation.mg`. |

#### Nested controls

| Field | Intended semantics | Actual consumer (2026-07-13) |
|-------|--------------------|------------------------------|
| `ToolLoop.MaxIterations` | Cap LLM↔tool turns | `ExecutorConfig.MaxToolIterations` (default 8) |
| `ToolLoop.MaxTotalCalls` | Cap total tool calls | `ExecutorConfig.MaxToolCalls` |
| `ToolLoop.FailOnToolError` | Abort on tool error | Heuristic on empty response + tool errs |
| `Safety.RequirePolicyEnforcement` | Must enforce policies | Flag unread in session |
| `Workspace.RootPath` | Agent FS root | CLI workspace / CWD |

### 4.3 Validation algorithm

```
Validate(c):
  if trim(c.IdentityPrompt) == "":
    return error("identity_prompt is required")
  if len(c.Policies) == 0:
    return error("at least one policy file is required")
  for policy in c.Policies:
    if not core.IsDefaultPolicyFile(policy):
      return error("not a canonical embedded policy reference")
    if policy already seen:
      return error("duplicate policy reference")
  return nil
```

Rationale from comments: without identity the runtime has no persona to ground the
LLM; policies anchor the executive layer and must be drawn from the same embedded
inventory as kernel boot. In practice specialist YAML enforces `Validate`;
factory/generated fallback paths do not yet uniformly enforce the same
full-config contract.

---

## 5. Deep dive: producers

### 5.1 ConfigFactory (`internal/prompt/config_factory.go`)

Not in this package, but the **canonical constructor** of the type:

1. Merge `ConfigAtom`s for intents (tools + policies, higher priority wins).  
2. `/consult/<persona>` falls back to `/general` atom.  
3. Emit `EffectiveAgentRuntimeConfig` with ToolLoop `{5,50,false}` and Safety `{true}`.  
4. `GenerateFallback` for compile failures.

Default atoms cover coder, tester, reviewer, researcher, nemesis,
tool_generator, and general intents. `mustDefaultPolicySet` resolves their stable
IDs through `core.DefaultAgentPolicySetFiles`; both built-in provider surfaces
share these exact sets.

### 5.2 JIT compiler attach

`CompilationResult.EffectiveAgentRuntimeConfig` may be filled when compiler has a factory (`compiler.go` step 6).

### 5.3 Specialist YAML

`session.Spawner.loadSpecialistConfig`:

- Path safety, 1 MiB cap, `yaml.Unmarshal`.  
- Calls `Validate` and returns a path-qualified error before injection.
- Missing file → factory with `"/"+name`.

---

## 6. Deep dive: consumers

### 6.1 Executor compile + tool loop

```
Execute:
  compile prompt
  cfg = compileConfig()  // injected or factory
  on err: cfg = empty
  runToolLoop(systemPrompt, input, cfg, ...)
```

`compileConfig`:

- If `e.EffectiveAgentRuntimeConfig != nil` → return it (SubAgent path).  
- Else factory.Generate with intent verb (default `/general`).  
- Nil factory → empty config.

`runToolLoop`:

- Builds tool definitions from `cfg.AllowedTools`.  
- Enforces allowlist on each call.  
- Loop bounds from **executor** config, not `cfg.ToolLoop`.  
- Optional no-tool retry when Mangle derives `intent_requires_tool_call`.

### 6.2 SubAgent / Spawner

Spawner generates or loads config into `SubAgentConfig`; SubAgent calls `executor.SetAgentConfig` before processing so compileConfig short-circuits.

---

## 7. Integration map

| Surface | Relationship |
|---------|--------------|
| Kernel | Policies are validated members of the globally loaded executive corpus; permission still derives through `permitted(...)` |
| VirtualStore | Tools execute after allowlist; interactive gates optional |
| Prompt JIT | Produces identity text + optional attach |
| Session | Universal executor |
| CLI boot | Wires `NewDefaultConfigFactory` |
| Campaign | Uses factory for multi-step personas |
| Tools registry | Resolves allowlist names |
| Config (user) | Orthogonal engine/timeout settings |

---

## 8. Testing inventory

| Layer | Location | Focus |
|-------|----------|-------|
| Unit | `types_test.go` | Identity, canonical policy, and duplicate validation |
| Core inventory | `core/policy_inventory_test.go` | Stable-set-to-embed and boot-module parity |
| Factory | `prompt/config_factory_test.go`, `config_policy_registry_test.go` et al. | Populate, Validate, canonical resolution, provider parity |
| Session | spawner/executor tests | YAML + allowlist |
| e2e | `tests/e2e/*` importing jit/config | Boundaries |

Commands: see README / `10-TESTING-ALIGNMENT.md`.

---

## 9. Safety model

1. **Schema gate:** identity + policies.  
2. **Tool gate:** allowlist membership.  
3. **Path gate:** specialist name + size.  
4. **Executive gate:** Mangle `permitted`, Dreamer preflight (core).  
5. **Degrade mode risk:** empty configs skip schema validation but are deny-all at
   the tool gate; the remaining risk is implicit availability/diagnostics, not an
   unrestricted registry capability.

See `09-SAFETY-AND-INVARIANTS.md` and `12-FAILURE-MODES.md`.

---

## 10. Observability

No logs in package. Consumers use `CategoryJIT` and `CategorySession`. Empty fallbacks and config generation failures are warn-level. See `11-OBSERVABILITY.md`.

---

## 11. Gaps pointer

Authoritative gap matrix: `03-GAP-ANALYSIS.md`.

Highest value fixes are **outside** this package’s LOC budget:

1. Validate generated/fallback configs or return an explicit typed degraded state.
2. Wire ToolLoop or stop setting it.  
3. Honor RequirePolicyEnforcement or remove.  
4. Carry stable policy-set identity/version into the turn and pin whether the
   canonical members describe the global corpus or drive selective per-agent loading.

---

## 12. Naming migration note

Older skill docs and tests may say **AgentConfig**. Canonical type name is **`EffectiveAgentRuntimeConfig`**. Nested historical shapes (`Tools.AllowedTools`, `Policies.Files`) were flattened; spawner tests assert flat YAML.

---

## 13. Non-goals of this corpus revision

- Implementing wiring fixes in Go (docs only).  
- Duplicating full prompt atom library docs (see `Docs/architecture/prompt/`).  
- Duplicating full executor docs (see `Docs/architecture/session/`).  
- Docs/Spec product 18-file templates.

---

## 14. Example specialist YAML (illustrative)

```yaml
identity_prompt: |
  You are a careful code reviewer. Prefer evidence over opinion.
intent_verb: "/review"
persona: "reviewer"
allowed_tools:
  - read_file
  - search_code
  - list_files
  - glob
  - grep
  - git_diff
  - git_log
  - run_command
  - get_elements
  - get_element
policies:
  - policy/constitution.mg
  - policy/validation.mg
  - reviewer.mg
tool_loop:
  max_iterations: 5
  max_total_calls: 50
  fail_on_tool_error: false
safety:
  require_policy_enforcement: true
workspace:
  root_path: ""
```

Until ToolLoop/Safety and selective policy semantics are wired, identity and
tools drive session behavior; policies are validated evidence of global-corpus
membership, not a per-agent kernel projection.

---

## 15. Architecture slogans (package-true)

- **Logic determines reality; the model merely describes it** — policies + permit still executive; this schema is the capability envelope for the model.  
- **JIT-first** — new personas are atoms + policy files, not new Go agent types.  
- **Wiring before deletion** — unused-looking fields may be incomplete integrations.

---

## 16. Cross-references

| Doc | Role |
|-----|------|
| [README.md](README.md) | Map + verify |
| [00-ALIGNMENT-VISION-REVIEW.md](00-ALIGNMENT-VISION-REVIEW.md) | Scores |
| [01-VISION.md](01-VISION.md) | Target vision |
| [02-CURRENT-STATE.md](02-CURRENT-STATE.md) | Inventory |
| [03-GAP-ANALYSIS.md](03-GAP-ANALYSIS.md) | Gaps |
| [04-ARCHITECTURAL-PRINCIPLES.md](04-ARCHITECTURAL-PRINCIPLES.md) | Principles |
| [05-INTERNAL-ARCHITECTURE.md](05-INTERNAL-ARCHITECTURE.md) | Flows |
| [06-PUBLIC-API-AND-TYPES.md](06-PUBLIC-API-AND-TYPES.md) | API |
| [07-DEPENDENCY-MAP.md](07-DEPENDENCY-MAP.md) | Deps |
| [08-WIRING-AND-INTEGRATION.md](08-WIRING-AND-INTEGRATION.md) | Boot/wiring |
| [09-SAFETY-AND-INVARIANTS.md](09-SAFETY-AND-INVARIANTS.md) | Safety |
| [10-TESTING-ALIGNMENT.md](10-TESTING-ALIGNMENT.md) | Tests |
| [11-OBSERVABILITY.md](11-OBSERVABILITY.md) | Logs |
| [12-FAILURE-MODES.md](12-FAILURE-MODES.md) | Failures |
| [TODO.md](TODO.md) | Backlog |
| [OPEN-QUESTIONS.md](OPEN-QUESTIONS.md) | Questions |

---

## 17. Verify (operator)

```powershell
go test ./internal/jit/...
go test ./internal/prompt/ -run "ConfigFactory|DefaultConfigFactory"
go test ./internal/session/ -run "Config|Spawner"
```

---

## 18. Change control

When editing the schema:

1. Update tags carefully (YAML specialists break on renames).  
2. Update `types_test.go` and factory Validate tests.  
3. Grep consumers for field use before removing “dead” fields.  
4. Refresh this corpus (date + gap matrix).  
5. Prefer conventional commits for code; docs-only rebuilds note date in `_progress.md`.
