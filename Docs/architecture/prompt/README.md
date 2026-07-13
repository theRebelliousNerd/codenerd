# Prompt JIT: the instructions a model gets for this turn

> Corpus: `prompt` | Live owner: `internal/prompt` | Verified: 2026-07-13

## In one minute

codeNERD does not give every model call one giant, permanent system prompt. The
prompt package compiles a turn-specific prompt from small YAML **atoms**: identity,
safety, method, language, framework, current-world context, and examples. The
Mangle kernel selects the deterministic skeleton; optional semantic retrieval
helps rank contextual flesh; Go resolves dependencies, fits the token budget,
and assembles the final text. The visible result is a model that receives the
instructions relevant to the user's current request without making prompt prose
the authority for tool execution.

`VERIFIED CURRENT` — the package contains 25 non-test Go files, 34 package test
files, one `sync` source/test pair, 333 embedded YAML files, and 888 atoms loaded
at runtime. The bounded receipts are `internal/prompt/embedded.go#LoadEmbeddedCorpus`,
`internal/prompt/embedded_test.go#TestLoadEmbeddedCorpus`, and `go test -count=1
-timeout=120s ./internal/prompt/...` (PASS on 2026-07-13).

## Its place in codeNERD

The LLM remains the creative center: it interprets the assembled instructions and
proposes text or tool calls. The Mangle kernel remains the executive: JIT selection
queries logic, while later session execution still checks the actual tool against
constitutional policy. `AllowedTools` is capability context, not proof that an
action is permitted. The package boundary is therefore:

```text
perception intent + world/session facts
  -> CompilationContext
  -> JITPromptCompiler.Compile
       embedded/SQLite/evolved/knowledge candidates
       -> Mangle skeleton + contextual flesh
       -> dependency order -> budget -> assembly
  -> prompt text + manifest + optional runtime config
  -> LLM
  -> session tool gate -> permitted(Action, Target, Payload)
  -> tool effect -> articulation
```

`VERIFIED CURRENT` — `internal/system/factory.go#initIntelligenceLayer` constructs
the atom loader, synchronizer, embedded corpus, kernel adapter, vector searcher,
project DB, compiler, and articulation bridge. `internal/system/factory.go#initFinalExecutors`
injects that compiler plus `internal/prompt/config_factory.go#NewDefaultConfigFactory`
into the clean session executor.

## A representative journey

Suppose a user asks, “Create a retry helper and run its tests.”

1. Perception produces an intent. `internal/session/executor.go#Executor.ProcessWithIntent`
   asserts `user_intent`, then `internal/session/executor.go#Executor.buildCompilationContext`
   turns the verb, target, mode, diagnostics, tests, and session state into a
   `CompilationContext`.
2. `internal/prompt/compiler.go#JITPromptCompiler.Compile` validates the context,
   checks its LRU cache, asserts compile facts, and collects embedded, project,
   shard, evolved, kernel-injected, knowledge, and learning atoms.
3. `internal/prompt/selector.go#AtomSelector.SelectAtomsWithTiming` loads identity,
   protocol, safety, and methodology as the Mangle-selected skeleton. It selects
   optional language, framework, context, and exemplar flesh with logic plus an
   optional bounded vector search.
4. `internal/prompt/resolver.go#DependencyResolver.Resolve` orders dependencies;
   `internal/prompt/budget.go#TokenBudgetManager.Fit` chooses standard, concise,
   or minimum representations; `internal/prompt/assembler.go#FinalAssembler.Assemble`
   renders templates such as runtime tool names.
5. `internal/prompt/config_factory.go#ConfigFactory.Generate` can attach a runtime
   config. The session resolves those tool names to live definitions and calls the
   model. Actual tool execution remains behind `internal/session/executor_tools.go#Executor.checkSafety`.
6. The user sees only the articulated response. Operators can inspect included
   atoms through `internal/prompt/compiler.go#JITPromptCompiler.GetLastResult` and
   `cmd/nerd/ui/jit_page.go#JITPageModel.UpdateContent`.

Failure is visible but intentionally tiered. Missing skeleton logic fails prompt
compilation; vector/flesh failures degrade to context matching or skeleton-only;
the clean executor currently replaces a failed compile with a minimal hardcoded
baseline. `VERIFIED CURRENT` — a no-tool retry changes the versioned cache identity,
selects `system/tool_nudge/no_tool_call_retry`, and renders the exact current tool
surface; `internal/system/prompt_kernel_scope_test.go#TestKernelAdapter_RetryContextBypassesPreRetryCache`
proves the production adapter path.

## What exists today

| Applicability lane | Evidence-backed answer |
|---|---|
| Mangle | `VERIFIED CURRENT` — Go emits `compile_context/2`, `current_context/2`, `prompt_atom/5`, selector, dependency, conflict, and vector-hit facts through `internal/prompt/context.go#CompilationContext.GenerateFacts` and `internal/prompt/selector.go#AtomSelector.buildContextFacts`. Declarations live in `internal/core/defaults/schemas_prompts.mg#prompt_atom/5`; selection lives in `internal/core/defaults/policy/jit_selection.mg#selected_atom/1`. |
| Permission and safety | `VERIFIED CURRENT` — safety atoms are skeleton content and config requests policy enforcement, but this package does not derive `permitted/3`. The authoritative default-deny gate is downstream in `internal/session/executor_tools.go#Executor.checkSafety`; prompt text and `AllowedTools` cannot authorize an effect. |
| Fact flow | `VERIFIED CURRENT` — `user_intent` becomes a compilation context in `internal/session/executor.go#Executor.buildCompilationContext`; the compiled prompt reaches the model in `internal/session/executor.go#Executor.generateResponse`; Piggyback control data returns through `internal/session/executor.go#Executor.processPiggybackControlPacket`. |
| JIT and agents | `VERIFIED CURRENT` — 23 atom categories are declared by `internal/prompt/atoms.go#AllCategories`; selectors, budgets, template expansion, structured-output filtering, per-agent YAML synchronization, and shard DB registration are live. Runtime load is proven by `internal/prompt/embedded_test.go#TestEmbeddedCorpusHasSystemAtoms` and `internal/prompt/sync/synchronizer_test.go#TestAgentSynchronizerSyncAll`. |
| Wiring | `VERIFIED CURRENT` — boot occurs in `internal/system/factory.go#initIntelligenceLayer`; agent DB lifecycle is connected in `internal/system/factory.go#initShardManagement`; the session consumer is constructed in `internal/system/factory.go#initFinalExecutors`; system shards also receive an articulation prompt assembler through `internal/shards/registration.go#shardFactoryRegistrar.createAssembler`. |
| State and concurrency | `VERIFIED CURRENT` — compiler caches, DB maps, config, last result, and close/wait lifecycle have locks or atomics; same-key compiles use `singleflight`. `internal/prompt/compiler.go#acquireCompilationKernel` obtains a private scope and `internal/system/factory_adapters.go#KernelAdapter.NewCompilationScope` clones the production `RealKernel`, so selector facts never enter the live kernel. Mixed-context isolation, error, cancellation, and retry are proven by `internal/system/prompt_kernel_scope_test.go`. `PARTIAL` — third-party `KernelQuerier` implementations that provide neither `KernelScopeProvider` nor `KernelRetracter` retain a best-effort compatibility path without isolation guarantees. |
| Recovery | `VERIFIED CURRENT` — vector and optional knowledge sources time out or degrade; invalid context and skeleton loss return errors; agent DB registration clears cache; `internal/prompt/compiler.go#JITPromptCompiler.Close` waits for compiles and closes owned DBs. `PARTIAL` — executor fallback preserves a response path but is generic and loses the selected safety/methodology skeleton. |
| Observability | `VERIFIED CURRENT` — `CompilationStats`, `PromptManifest`, selected/dropped atom entries, cache hit/miss counters, structured logs, and the TUI inspector exist in `internal/prompt/compiler.go#CompilationStats`, `internal/prompt/manifest.go#PromptManifest`, and `cmd/nerd/ui/jit_page.go#JITPageModel`. `PARTIAL` — there is no durable, turn-correlated decision receipt joining prompt selection to later permission and outcome. |
| Testing | `VERIFIED CURRENT` — 235 listed tests cover compilation, selection, budget, assembly, strict loading, ordered corpus parity, fail-closed sync, config, malformed inputs, and concurrency. The checked-in validator passes 333 files/888 atoms with `-fail-on-warn`; both the full prompt/sync race suite and focused production scope/cache race gate pass. `PARTIAL` — no package fuzz target or external-adapter scope-conformance suite exists. |

The detailed source inventory is in [Current State](02-CURRENT-STATE.md). Current
truth, aspiration, and authoritative feature cards are deliberately separated.

## North star

Every LLM turn should carry an inspectable **prompt decision receipt**: which
versioned atoms were eligible, which exact context facts selected or rejected
them, how the budget transformed them, which capabilities were described, and
how that decision related to the later permission and outcome. Compilation
should be deterministic under identical typed inputs, isolated between concurrent
turns, and reproducible offline without replaying side effects.

Non-goals:

- Prompt prose never replaces constitutional `permitted(Action, Target, Payload)`.
- Mangle never becomes a fuzzy text store; semantic retrieval proposes candidates,
  and logic reasons over typed facts.
- The prompt package does not own perception, tool execution, model transport, or
  articulation.
- “More atoms” is not inherently better; relevance, provenance, and boundedness
  outrank corpus size.

See [Vision](01-VISION.md) for goals, measurable targets, and phased uplift.

## Improvement frontier

The first three truth repairs are now verified: field-complete cache identity,
production kernel snapshot isolation, and one strict atom parser with ordered
888-atom parity. The remaining frontier is:

1. `PROPOSED UPLIFT` — require every external `KernelQuerier` adapter to provide
   a compilation scope or fail closed, retiring the unscoped compatibility path.
2. `PROPOSED UPLIFT` — generate capability atoms from the live tool registry and
   treat the old `SimpleRegistry` catalog as migration input or remove it.
3. `PROPOSED UPLIFT` — persist a redacted, turn-correlated prompt decision receipt
   and expose “why selected?”, “why dropped?”, and “what changed from last turn?”
   in the inspector.
4. `PROPOSED UPLIFT` — add shadow compilation: compare a candidate selector/budget
   policy with production while only production affects the model.
5. `DEFERRED` — a counterfactual replay lab may test alternate atom sets against
   recorded, redacted contexts and outcome rubrics. It must never replay tools or
   auto-promote safety atoms.

The machine-readable backlog and rollback contracts are in [TODO](TODO.md).

## Choose a reading route

| Time | Route |
|---|---|
| 90 seconds | This README, then [Current State](02-CURRENT-STATE.md#executive-summary) and [Gap Analysis](03-GAP-ANALYSIS.md#priority-order). |
| 10 minutes | [Vision](01-VISION.md), [Internal Architecture](05-INTERNAL-ARCHITECTURE.md), [Wiring](08-WIRING-AND-INTEGRATION.md), and [Safety](09-SAFETY-AND-INVARIANTS.md). |
| Deep implementation | [Implemented Spec](IMPLEMENTED_SPEC.md), [Prompt JIT Deep Dive](13-PROMPT-JIT-DEEP-DIVE.md), [Testing](10-TESTING-ALIGNMENT.md), and [Failure Modes](12-FAILURE-MODES.md). |
| Build or review an uplift | [Gap Analysis](03-GAP-ANALYSIS.md), [TODO](TODO.md), [Open Questions](OPEN-QUESTIONS.md), then [_progress](_progress.md). |
