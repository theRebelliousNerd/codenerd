# internal/core

The Mangle kernel and its executive: the deductive fact store, the policy
fixpoint, and the action router the rest of the agent loop talks to. Two
halves — `RealKernel` owns the EDB and evaluates policy; `VirtualStore`
routes actions through boot guard, Dreamer, constitution, allowlists, and
validators.

Verified 2026-09-21 against commit `3463477` (`main`).

## What lives here

| Area | Key files | Role in one line |
|---|---|---|
| Fact store (EDB) | `internal/core/kernel_facts.go` | `Assert` (`kernel_facts.go:515-562`), `Retract` (`kernel_facts.go:570-625`), canonical keys (`canonFact`, `kernel_facts.go:155-167`), dedup on insert (`addFactIfNewLockedErr`, `kernel_facts.go:442-486`) |
| Rebuild and eval | `internal/core/kernel_eval.go` | `rebuildProgram` (`kernel_eval.go:61-156`), `evaluate` (`kernel_eval.go:171-306`), lifecycle helpers `Clear`/`Reset`/`Clone` (`kernel_eval.go:392-493`) |
| Query surface | `internal/core/kernel_query.go` | `Query` (`kernel_query.go:24-142`), `QueryWithBindings` (`kernel_query.go:145-246`), `QueryCallback` (`kernel_query.go:249-362`); bulk reads `QueryAll` (`kernel_facts.go:791-851`) and `GetDerivedFacts` (`kernel_facts.go:1083-1134`) |
| Learned rules | `internal/core/kernel_validation.go`, `internal/core/kernel_modules.go` | load-time validation `validateLearnedRulesContent` (`kernel_validation.go:197-361`), loop-risk check `checkInfiniteLoopRisk` (`kernel_validation.go:365-478`), repair `healLearnedRules` (`kernel_validation.go:557-688`) |
| Provenance | `internal/core/kernel_provenance.go` | opt-in derivation recording `RecordDerivation` (`kernel_provenance.go:42-60`), trees via `Explain` (`kernel_provenance.go:72-112`), toggled by `EnableProvenance` (`kernel_provenance.go:29-36`) |
| System facts | `internal/core/kernel_sysfacts.go` | `UpdateSystemFacts` (`kernel_sysfacts.go:24-107`) feeds git state via `parseGitStatus` (`kernel_sysfacts.go:193-223`) |
| Execution context | `internal/core/kernel_context.go`, `internal/core/kernel_exec_env.go`, `internal/core/kernel_registers.go`, `internal/core/kernel_policies.go`, `internal/core/kernel_eval_mode.go` | scoping, environment, register and policy plumbing around eval |
| Boot and constitution | `internal/core/kernel_boot.go`, `internal/core/kernel_constitutional.go`, `internal/core/kernel_knowledge_types.go`, `internal/core/fact.go` | startup checks, constitutional derivation, core fact types |
| Dreamer | `internal/core/dreamer*.go` | speculative simulation before routing; cache, contract, and intent subsystem alongside it |
| Cortex and sharding | `internal/core/cortex_kernel.go`, `internal/core/kernel_shard.go`, `internal/core/shard_fact_router.go` | routing efficacy counters and per-domain fact routing |
| Schedulers, audit, tools | `internal/core/api_scheduler.go`, `internal/core/audit*.go`, `internal/core/tool_invocation.go`, `internal/core/builtin_registry.go`, `internal/core/bridge.go`, `internal/core/validators/*` | LLM queue scheduling, action audit trail, tool invocation, builtins, validators |
| Executive | `internal/core/virtual_store*.go` | `RouteAction` pipeline: boot guard first (`virtual_store_routing.go:41-43`), Dreamer, constitution, allowlists, validators; guard switch `DisableBootGuard`/`IsBootGuardActive` (`virtual_store.go:395-408`) |

## Entry points for callers

- Assert and retract ground facts: `Assert` / `Retract` (`internal/core/kernel_facts.go:515-625`).
- Ask what holds: `Query` / `QueryWithBindings` / `QueryCallback` (`internal/core/kernel_query.go:24-362`).
- Route an action: `VirtualStore.RouteAction`, denied by default until policy derives `permitted`.
- Release the boot guard after startup: `DisableBootGuard` (`internal/core/virtual_store.go:395-408`); the chat host does this at `cmd/nerd/chat/process.go:151-155`.
- Inspect a derivation: `Explain` (`internal/core/kernel_provenance.go:72-112`), only after `EnableProvenance`.

## Further reading

- `INTERNALS.md` — fact lifecycle, rebuild/eval order, learned-rule validation, query semantics.
- `WIRING-AND-NOT-BUILT.md` — what is reachable, what exists but nothing calls, and what the design assumes that the code does not do.
- `11-OBSERVABILITY.md` — what observability the package exposes and where it goes.
