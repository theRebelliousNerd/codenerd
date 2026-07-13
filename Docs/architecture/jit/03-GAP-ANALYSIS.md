# 03 — Gap Analysis: JIT config

> Last verified against codebase: **2026-07-13**  
> Package: `internal/jit/config` vs vision of full JIT-driven agent constitution

## 1. Spec vs reality matrix

| Capability | Vision | Reality | Gap severity |
|------------|--------|---------|--------------|
| Shared runtime config type | One type for all personas | `EffectiveAgentRuntimeConfig` | **None** |
| Identity required | Non-empty identity | `Validate` enforces; hot paths may skip Validate | **Medium** |
| Policies required | ≥1 policy file | `Validate` enforces; empty fallbacks skip | **Medium** |
| Tool allowlist | Bound LLM tool surface | Enforced when non-empty; empty cfg allows zero defs / permissive edge | **Low–Medium** |
| ToolLoop limits from config | Per-agent loop knobs | Factory sets defaults; executor uses `ExecutorConfig` | **High** (dead knobs) |
| FailOnToolError | Fail turn on tool error | Not wired to `cfg.ToolLoop` | **Medium** |
| RequirePolicyEnforcement | Fail closed without policy | Flag set `true` by factory; no session branch | **High** |
| Model override per agent | Per-agent model | Field exists; session uses shared LLM client | **Medium** |
| Workspace root per agent | Scoped FS | Field exists; not primary workspace binding | **Low** |
| Persona field | Explicit persona atom key | Optional; intent verb carries routing | **Low** |
| Validate on YAML load | Specialist configs validated | `loadSpecialistConfig` unmarshals without `Validate()` | **High** |
| Validate after factory | Generated configs always valid | Factory defaults pass tests; Generate does not call Validate | **Low** (tests cover) |
| Policy files applied to kernel | Load `Policies` into Mangle | Names carried; kernel policy corpus largely global defaults | **High** (system-level) |
| Empty-config degrade mode | Explicit “degraded” type/flag | Opaque zero value | **Medium** |
| Docs / skill name drift | Single type name | Skill still mentions `AgentConfig` in snippets | **Low** |

## 2. Non-gaps (do not “fix”)

| Observation | Why it is intentional |
|-------------|----------------------|
| Package is tiny (59 LOC) | Schema-only design; logic belongs in prompt/session |
| No Mangle under `internal/jit` | Policies are *references*, not rule sources |
| No logging in package | Pure types; consumers own observability |
| No constructors | Zero-value + YAML + factory construction are enough |
| AllowedTools not required by Validate | Zero tools is valid for pure-reasoning turns |

## 3. Priority backlog (schema-adjacent)

### P0 — Correctness / safety

1. **Call `Validate()` after specialist YAML unmarshal** (`session/spawner.go` `loadSpecialistConfig`) or reject invalid agents.  
2. **Decide empty-config semantics**: either deny tools when cfg is zero, or mark degrade mode and log loudly.  
3. **Wire or delete `Safety.RequirePolicyEnforcement`** — a bool that is always true but never read is a false assurance.

### P1 — Schema honesty

4. **Wire `ToolLoop` into `runToolLoop`** *or* stop populating it in ConfigFactory and document executor-only limits.  
5. **Document or implement `Policies` application** (kernel assert/load) so Validate’s requirement is not cargo-cult.  
6. **Optional: `Model` routing** when multi-engine agents need per-persona models.

### P2 — Hygiene

7. Update skill references to `EffectiveAgentRuntimeConfig` only.  
8. Consider `Validate` called at end of `ConfigFactory.Generate` for defense in depth.  
9. Add package-level doc comment / root `doc.go` clarifying “schema only; see prompt+session”.

## 4. Dependency on other packages’ gaps

| External gap | Impact on jit schema |
|--------------|----------------------|
| Domain shards deleted; intent routing must stay complete | Wrong `IntentVerb` → wrong ConfigAtom → wrong tools |
| Tool registry name drift | Allowlist names that don’t match registry → silent tool drop in `buildToolDefinitions` |
| Global policy corpus incomplete | `Policies: ["coder.mg"]` may not map to loadable files |

## 5. Summary

The package **meets its narrow contract** (typed config + structural validation). The **system gap** is partial **field actualization** in session and missing Validate on YAML loads. Closing gaps is mostly **consumer wiring**, not growing `internal/jit`.
