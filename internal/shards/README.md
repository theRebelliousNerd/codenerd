# internal/shards/

**Status:** live. This package holds shard *registration*, *matching*, *consultation*, and
*observation* — plus the one shard implementation that never moved (`RequirementsInterrogatorShard`).
The shard *implementations* for domain personas are gone; those live in `internal/session/`.

**Last verified:** 2026-08-08 against the code in this tree. Every file:line below was read, not
remembered. The previous version of this file described a December 2024 architecture and was wrong
in ways that mattered — it claimed `registration.go` mapped `/coder` to intents via a
`MapLegacyCommand` function that does not exist, and pointed at
`internal/core/defaults/policy/intent_routing_rules.mg` and `internal/prompt/config_defaults.go` for routing rules that
live elsewhere.

---

## The two execution paths, and which one you want

There are two, they are not redundant, and picking the wrong one is the most common mistake here.

| | `ShardManager` (`internal/core/shards/`) | Clean loop (`internal/session/`) |
|---|---|---|
| Serves | **system shards** — long-lived background services with Go implementations | **everything else** — domain personas and user-defined agents |
| Members | `perception_firewall`, `world_model_ingestor`, `executive_policy`, `constitution_gate`, `legislator`, `mangle_repair`, `tactile_router`, `campaign_runner`, `session_planner`, `requirements_interrogator`, `image_generator` | `coder`, `tester`, `reviewer`, `researcher`, `nemesis`, and every `.nerd/agents/<name>/` agent |
| Behavior comes from | hand-written Go (`internal/shards/system/`) | JIT-compiled prompt atoms + config atoms |
| Entry | `ShardManager.Spawn` (`internal/core/shards/manager_spawn.go:133`) | `JITExecutor.Execute` (`internal/session/task_executor.go`) |

`Cortex.SpawnTask` is the front door and routes between them:
`internal/system/factory.go:358` (`SpawnTaskWithTarget`) sends image shards and any type whose
profile is `types.ShardTypeSystem` to `ShardManager`, and everything else to `TaskExecutor`.

**If `ShardManager` is asked for a type it has no factory for, it delegates to the clean loop**
(`internal/core/shards/manager_spawn.go:262-289`). That is the seam that makes the two paths one
system rather than two. It is wired at boot by `SetTaskDelegator`
(`internal/core/shards/manager.go:297`), called from `internal/system/factory.go:1481`,
`cmd/nerd/chat/session_boot.go`, and both campaign boot paths in `cmd/nerd/cmd_campaign.go`.

> **Never fabricate a shard.** `BaseShardAgent.Execute`
> (`internal/core/shards/agents.go:114`) returns an **error**. It is lifecycle plumbing — state,
> kernel handle, LLM handle, permissions — that concrete shards embed; it has no task semantics.
> It used to return `("BaseShardAgent execution", nil)`, and because `ShardManager` installed it
> for every type with no factory, that placeholder became the answer for all four domain personas
> and every user agent. Campaign consultations recorded it as specialist advice and parsed a
> confidence out of it, the retry verifier accepted it as a completed retry, and
> `nerd spawn <anything>` printed it and exited 0. Guarded by
> `internal/core/shards/hollow_spawn_test.go`.

---

## What is in this package

| File | What it does | Reached from |
|---|---|---|
| `registration.go` | Registers every system-shard factory and profile with `ShardManager`, injecting kernel / LLM / VirtualStore / learning store / prompt assembler. Also owns `DefaultShardPredicateManifests` (`registration.go:36`), the predicate-ownership table for the per-shard fact router. | `RegisterAllShardFactories` (`registration.go:333`); manifests consumed at `internal/system/factory.go:945` |
| `requirements_interrogator.go` | The one surviving ephemeral shard implementation. Kept because its ask-the-user Socratic loop has no JIT equivalent. | registered at `registration.go:204`, `cmd/nerd/chat/session_boot.go:778` |
| `consultation.go` | Cross-specialist consultation protocol: request, batch, cache, parse structured advice. Talks to any `ConsultationSpawner` (`consultation.go:61`). | `cmd/nerd/chat/session_boot.go:1027`, `session_shared_boot.go:348`, `cmd/nerd/cmd_campaign.go:485` and `:920` |
| `matching.go` | Maps file patterns / imports / content to specialist names (`CoreTechnologyPatterns`), and classifies specialists as executor / advisor / observer. | `cmd/nerd/chat/delegation_modes.go:263-272` |
| `observer_manager.go` | Observer registry for shard lifecycle events. | `cmd/nerd/chat/session_boot.go`, `session_shared_boot.go` |
| `system/` | The system-shard implementations themselves. | via the factories in `registration.go` |

`registration.go` does **not** contain a `MapLegacyCommand`. Legacy persona names are mapped to
intent verbs in two places instead: `normalizeTaskIntentVerb`
(`internal/session/task_executor.go:106-144`) for CLI/`Cortex.SpawnTask` input, and
`personaToIntent` (`cmd/nerd/chat/delegation_routing.go:297`) for chat delegation.

---

## User-defined agents (the `.nerd/agents/` pipeline)

This is what `nerd init` creates (GoExpert, MangleExpert, …) and what `nerd define-agent` and the
chat `/define-agent` wizard create. The full chain, in order:

```
.nerd/agents/<Name>/prompts.yaml          authored (or generated)
        |
        |  internal/prompt/sync/synchronizer.go SyncAll
        |  run at boot: internal/system/factory.go:1098
        v
.nerd/shards/<lower(name)>_knowledge.db   prompt_atoms table
        |
        |  prompt.RegisterAgentDBWithJIT  — internal/system/factory.go:1202
        |  keyed case-insensitively        — internal/prompt/compiler_db.go shardDBKey
        v
JIT compiler shardDBs[<lower(name)>]
        |
        |  selected only when CompilationContext.ShardID names it
        |  internal/prompt/compiler.go:902-906 (collectAtomsWithStats)
        v
compiled system prompt
```

The verb → agent binding is the piece that closes it. `Executor.buildCompilationContext`
(`internal/session/executor.go:619-650`) resolves the persona in this order:

1. `perception.GetShardTypeForVerb(verb)` — built-in taxonomy (`/fix` → `coder`). **Checked first**,
   so a core verb can never be mistaken for an agent.
2. `session.UserAgentFromIntentVerb(verb)` (`internal/session/task_executor.go:45`) — everything
   else. Accepts `/consult/<name>` (chat delegation) and `/<name>` (`nerd spawn <name>`), returns a
   lower-cased agent name.

Both `ShardID` **and** `ShardType` get set. `ShardID` selects the atom DB; `ShardType` supplies the
shard dimension that `jit_compiler.mg`'s `blocked_by_context` needs in order to *exclude* other
personas' atoms. With `ShardType` absent, every shard-gated atom in the corpus is admitted and the
agent is handed 25+ contradictory identities — the documented hollow-output failure.

Tools come from a config atom registered per agent at boot:
`registerUserAgentConfigAtoms` (`internal/system/factory.go:1336`, called at `:1403`) reads the
`tools` array from `.nerd/agents.json` and grants it **unioned with the read-only core set**, under
both `/<name>` and `/consult/<name>`. Declaring a tool there is how a specialist earns write access;
kernel `permitted(...)` still gates every mutation.

### Adding an agent

```bash
nerd define-agent --name RustExpert --topic "Tokio async runtime"
# writes .nerd/agents/RustExpert/prompts.yaml, syncs .nerd/agents.json, then researches
```

Then edit `prompts.yaml` (identity / methodology / domain atoms, each with `content`,
`content_concise`, `content_min` so the JIT budget can degrade rather than drop it), and:

```bash
nerd spawn RustExpert "port this module to tokio"
# chat: /spawn RustExpert port this module to tokio
```

The single writer for that layout is `system.WriteAgentDefinition`
(`internal/system/agent_definition.go`); the chat wizard and the CLI both call it, so their
templates cannot drift.

### Casing

`nerd init` creates lower-case directories; the chat wizard preserves what you typed;
`.nerd/agents.json` may hold either. Discovery keys on the directory name
(`internal/system/agent_registry.go:44`) while every verb arrives lower-cased. The shard-DB map
normalizes both ends (`internal/prompt/compiler_db.go` `shardDBKey`), so casing does not matter —
but do not reintroduce a case-sensitive lookup, because the failure is silent.

---

## Adding a system shard

Only for genuinely long-lived Go services. Everything task-shaped should be a persona or a
user agent instead.

1. Implement it in `internal/shards/system/`, embedding `BaseSystemShard`
   (`internal/shards/system/base.go:263`) and **defining `Execute`**.
2. Register a factory in `registration.go` (`registerSystemShards` / `registerLogicShards` /
   `registerPlanningShards`) with the dependencies it needs.
3. Define a profile with `Type: types.ShardTypeSystem` in `defineSystemShardProfiles`
   (`registration.go:395`).
4. If the chat TUI must also start it, mirror the registration in
   `cmd/nerd/chat/session_boot.go:671-812` — that file has its own factory registrations, and a
   shard registered in only one of the two places exists in only one of the two runtimes.

---

## Known dead code

Called out rather than silently left to look live:

- **`Spawner.SpawnSpecialist` / `loadSpecialistConfig`** (`internal/session/spawner.go:302` and
  `:535`) have no production caller — only `tests/e2e/`. They read
  `.nerd/agents/<name>/config.yaml`, a file no code writes. The live user-agent path is
  `Spawner.Spawn` via `JITExecutor`, documented above. Treat `config.yaml` as an unreleased
  hand-authoring hook, not as how agents work.
- **`EffectiveAgentRuntimeConfig.IdentityPrompt`** is produced by `ConfigFactory.Generate`
  (`internal/prompt/config_factory.go:110`) and validated, but the executor's system prompt always
  comes from `compileResult.Prompt` (`internal/session/executor.go`). The field carries tools and
  policies, not identity.

`TaskRequest.Persona` and `TaskRequest.ConfigRef` used to belong on this list — every caller set
them and nothing read them. They are gone; `IntentVerb` is the single routing key
(`internal/session/task_executor.go:25`).

---

## Tests that pin this

| Test | Guards |
|---|---|
| `internal/core/shards/hollow_spawn_test.go` | `BaseShardAgent.Execute` fails loudly; no-factory spawn errors or delegates; image shards are never delegated to the worker LLM |
| `internal/session/user_agent_wiring_test.go` | verb → agent-name extraction; built-in verbs keep their persona; custom verbs set both shard dimensions |
| `internal/system/user_agent_prompt_test.go` | prompts.yaml → knowledge DB → JIT → compiled prompt, with a real Mangle kernel; case-insensitive shard-DB lookup; declared tools reach the config atom |
| `internal/shards/registration_manifest_test.go` | predicate ownership is unambiguous |
