# Expert selection needs invocation owned temporary facts

## Finding and scope

Protect temporary evidence for the whole expert-selection invocation, and read its complete decision from a coherent snapshot. Current source permits two successful selections for the same canonical path to interfere: one invocation can retract the shared extension tuple while the other still needs it, producing an empty expert list without an error.

This is a conditional, source-predicted concurrency failure, not a reproduced incident, measured quality regression, or scanner/world persistent-fact erasure finding. No code, tests, model calls, or benchmarks were run for this addendum. The witness requires extension-only relevant evidence; matching topology or import evidence can prevent the predicted omission.

The source baseline is main `bc610aeb9e8223823c9e84bc132de993d11199c2`, verified at 21:22 UTC on 29 October 2026. Prior-guidance comparison included research heads `adb16e4f7e69318e3ed0a06854e04db4769783a8`, `76d6bff4c8cb17a9da185750045bfa285afc79f9`, and `75240b6649e9e7fce0c117ac5b03c6f4f0a95840`. The reviewed inventories contained no exact owner for this selector-specific lifetime contract.

## How one selection can remove another selection’s support

[`Select`][select] creates an atomic unique task ID, asserts its facts, reads decisions, then retracts the asserted batch. Most inputs include the task ID, but `taskFacts` emits `file_extension(canonicalPath, ext)` without it. Unique task IDs therefore do not establish ownership of every asserted tuple.

RealKernel [deduplicates equal tuples][dedup]. Its [batch assertion][assert] and [exact batch retraction][retract] each lock independently; retraction removes matching canonical tuples without an owner count. A later [query reevaluates dirty state][query]. [Cortex forwards these operations to shards][cortex], and registration puts [extension][extensionroute] plus [roster and task inputs][routing] in the world shard. This witness does not depend on a missing cross-shard join.

Consider two independent calls, A and B, with the same canonical target `policy.gl`, a nonblind `/review` or `/fix` verb, and a healthy ready expert/domain roster for `/mangle`. The [language mapping][language] recognizes `.gl` as `/mangle`. Require no independently matching `file_topology` language evidence and no import-domain or override evidence supporting that expert. The [expert rules][rules] provide these alternative derivation paths; they are important controls, not negligible exceptions.

1. A asserts its task facts and F = `file_extension("policy.gl", ".gl")`.
2. B asserts distinct task facts and the identical F. Set deduplication leaves one F.
3. B completes all decision reads, then retracts its asserted batch, including F.
4. A performs its first `expert_needed` read. With F absent and no alternative support, no expert is derived. The reader returns an empty decision without error; `Select` then cleans up A’s facts and returns that decision.

A’s task identity remains present until its cleanup; B need not erase it. Both calls can succeed. Per-method mutexes prevent unsafe simultaneous mutation, but do not make this compound assert/read/retract sequence atomic.

A related conditional interleaving places B’s cleanup after A’s first successful need query. A can retain the initial need while later file, confidence, execution/advice, or strategic-routing reads observe a different state. [The reader issues those queries separately][read]. The desired contract consequently covers the complete decision, not just nonempty `expert_needed`.

## Shared execution makes overlap possible

[`CloneForTask`][clone] isolates executor history while reusing `e.kernel`. [Task execution][task] either spawns or uses that clone and carries `req.Target` into processing. The [spawner][spawner] and [subagent constructor][spawn] also pass the same kernel into an executor; [subagent execution][run] runs asynchronously with its preset target. Capacity and scheduling controls remain, including the [default maximum of ten active subagents][capacity].

A concrete target-bearing route is background work. [Process routing][process] copies `intent.Target` into the background job; [the model update][update] clears loading and starts the command while chat remains available. The [background registry][jobs] keys jobs by job ID; [dispatch][background] copies the target into `TaskRequest` and calls `spawnTaskWithContext`. The [read-only routing policy][arbitration] does not itself establish a same-path mutex.

Thus two separately admitted target-bearing background jobs can overlap on a shared kernel. This establishes a possible route, not a claim that every review is exposed: admission, timing, target equality, and absence of independent evidence remain conditions.

**Normal workspace scanning is a substantial rescue condition.** [Query delegation attempts workspace loading][scanstart] when a scanner exists. Both [full scanning][scanfull] and [incremental scanning][scanincremental] emit topology even without an AST parser. Unsupported AST parsing alone therefore does not establish missing evidence; a successfully scanned `.mg` file can retain `/mangle` support after F disappears.

There is also a concrete healthy-scan realization: a readable, successfully scanned `policy.gl`. Mangle maps `.gl` to `/mangle`, but the scanner’s [language-detection map][detectlanguage] omits `.gl` and returns `unknown`. Its topology tuple therefore does not independently restore the `/mangle` derivation. With no matching import or override, the conditional witness still applies. This mapping difference establishes the witness, not a separate scanner-remediation proposal. A not-yet-existing intended target, absent successful scan membership, or no prior topology plus a [scan/application error][scanload] are other possible absence conditions; none changes permissions.

Generic [`executeParallelMode` specialist fanout][fanout] is not sufficient evidence for this witness. Its [`spawnTask`][delegation] to [`shardTypeToTaskRequest`][taskrequest] path supplies verb/task without a target; [the preset intent][preset] does not parse one from prose. Initial selection there can see an empty target.

The older [`languageOfFile` helper][helper] already asserts a temporary `file_extension(path, ext)`, queries it, and defers `RetractFact` by first argument. It can participate when its raw path matches the selector’s canonical path. The shared temporary-fact pattern predates the expert consumer; it is not wholly new. Nor does this finding require the scanner to emit extension facts: current topology evidence is a rescue path.

## What can reach the prompt

[`buildCompilationContext` runs before JIT compilation][context] and [computes language, compilation needs, and needed experts][fields]. [`neededExperts`][expertcontext] calls the selector on the executor kernel, returns names, and logs actual errors; a successful empty decision does not take that error path. [Step compilation][step] calls it again.

The [expert-source resolver][compiler] considers `CompilationContext.Experts` and skips the speaker itself, duplicates, and missing databases. An empty expert list therefore supplies no source through this particular push path. For selected experts, prompt atoms and bounded knowledge retrieval still face normal relevance, context, conflict, dependency, and budget constraints.

The [generation snapshot][generation] includes expert sources, but [private preflight][preflight] occurs after field construction. It cannot reconstruct an expert name already omitted from that field. Direct speaker databases, project sources, and embedded sources may independently provide useful domain context. This is neither a claim that all knowledge disappears nor a guarantee that restoring a source improves task quality or inserts its content into the final prompt. Authorization and acceptance checks remain unchanged.

## Ownership relative to prior guidance

The [generation-aware JIT audit][jitaudit] owns private JIT scope and preflight; [the performance audit][perfaudit] owns clone, cone, and retraction costs; [consultation guidance][consultation] owns consultation coalescing. None is the same as owning this helper’s temporary evidence across decision reads.

The earlier [campaign-context audit][campaign] concerns retained presentation facts and scheduler ownership, while [roster identity guidance][roster] concerns the wrong workspace roster. Neither should absorb this conditional concurrent extension loss. The [session contract][session] already describes a shared kernel with task-identity cleanup; this addendum makes the unscoped measurement exception explicit.

## Proposed contract and implementation choices

Each invocation should own its temporary measurements. Its cleanup must not remove another invocation’s support or preexisting independent state. Needs, files, confidence, and routing should describe one coherent selection snapshot. Assertion, read, cancellation, and cleanup failures need bounded cleanup of owned state, without guessing replacement experts.

Compare these designs before choosing one:

- **Task-scoped measurements.** Add invocation identity to the measurement relation and its joins, while reusing the Mangle language mapping rather than duplicating extension policy in Go. Account for schema, routing, indexes, and cleanup. This retains parallel selection.
- **Private selection snapshots.** Capture the correct world/roster dependency closure and define source-generation consistency, capability lifetime, close behavior, and error cleanup. Existing JIT isolation is a useful precedent, not a drop-in answer: [its Cortex adapter selects the primary RealKernel][adapter], which alone does not establish that selector world inputs are included.
- **Serialization or explicit leases.** Full-call coordination must cover every temporary producer, including `languageOfFile`, and preserve preexisting facts. An experts-only mutex is insufficient. Query-before-assert is non-atomic; an “inserted” flag does not protect borrowers after the inserter exits. Two mutation transactions do not span the intervening reads, and the [transaction interface][transaction] provides mutation operations rather than a query snapshot.

Whichever design is chosen, define coherence if world or roster state changes during the call; ownership alone does not freeze those inputs. Do not silently change global set semantics into reference counting. If leases are chosen, define ownership, prior-state handling, rollback, and bounded owner retention explicitly.

Current shared writes dirty the kernel and can cause reevaluation; broad reads also collect concurrent tasks’ results before filtering by task ID. Treat that as a cost to compare, not another established correctness finding. Task scoping adds keys and join work. Snapshots add copy/index/setup and retained-memory costs, even with shared overlays. Serialization adds queueing, head-of-line blocking, and cancellation interactions, especially in p95/p99 latency. Leases add active-owner bookkeeping and rollback complexity.

No extra LLM call is inherently required. Correctly admitted knowledge can increase prompt tokens or displace other content. Reduced retries, better answers, and net savings remain hypotheses.

## Evidence needed before implementation acceptance

Use deterministic barriers, not sleeps. Pause A after assertion, let B finish, then resume A’s first read; compare with an isolated baseline. Repeat with cleanup after A’s first need query to test full-decision coherence. Exercise RealKernel and correctly registered CortexKernel.

Cover same and different paths, serial calls, private kernels, canonical/raw-path overlap with `languageOfFile`, empty targets, excluded verbs, absent experts, and topology/import rescue, including healthy scanned `.gl` versus scanned `.mg` as paired controls. Inspect different-workspace ownership on a shared kernel explicitly; if unsupported, reject that configuration rather than conflating it with this witness. Include cancellation and assertion/read/retraction failure cleanup, preserving unrelated state.

An integration witness should capture actual `CompilationContext.Experts` and an authorized fake JIT request containing a unique expert-knowledge marker under a sufficient admission budget. A stored database row alone is not proof of prompt delivery. Include missing-database, own-speaker, direct-source, and budget controls. Drive two explicitly target-bearing background jobs with fake providers rather than assuming a natural-language classifier chooses the required route.

[Existing selector test definitions][tests] cover serial success and cleanup plus negative verb/target and complexity cases; they were read, not run. This does not claim that all concurrency tests are absent.

Measure selection latency, dirty-generation traffic, evaluation count, cone/full fallback, cleanup costs, CPU, allocation, retained memory, and p50/p95/p99 under controlled concurrency. Cleanup does not imply a synchronous full fixpoint on every call; lazy evaluation and the next queries affect its cost. Compare request tokens and cache behavior as well as correctness. Any later quality claim needs held-out matched tasks, model capabilities, permissions, and unchanged acceptance gates. No performance or quality result is established here.

## Pinned source references

[select]: https://github.com/theRebelliousNerd/codenerd/blob/bc610aeb9e8223823c9e84bc132de993d11199c2/internal/experts/select.go#L79-L127
[read]: https://github.com/theRebelliousNerd/codenerd/blob/bc610aeb9e8223823c9e84bc132de993d11199c2/internal/experts/select.go#L143-L198
[dedup]: https://github.com/theRebelliousNerd/codenerd/blob/bc610aeb9e8223823c9e84bc132de993d11199c2/internal/core/kernel_facts.go#L446-L463
[assert]: https://github.com/theRebelliousNerd/codenerd/blob/bc610aeb9e8223823c9e84bc132de993d11199c2/internal/core/kernel_facts.go#L648-L706
[retract]: https://github.com/theRebelliousNerd/codenerd/blob/bc610aeb9e8223823c9e84bc132de993d11199c2/internal/core/kernel_facts.go#L956-L994
[query]: https://github.com/theRebelliousNerd/codenerd/blob/bc610aeb9e8223823c9e84bc132de993d11199c2/internal/core/kernel_query.go#L40-L57
[routing]: https://github.com/theRebelliousNerd/codenerd/blob/bc610aeb9e8223823c9e84bc132de993d11199c2/internal/shards/registration.go#L236-L243
[language]: https://github.com/theRebelliousNerd/codenerd/blob/bc610aeb9e8223823c9e84bc132de993d11199c2/internal/core/defaults/policy/coder_language.mg#L75-L79
[rules]: https://github.com/theRebelliousNerd/codenerd/blob/bc610aeb9e8223823c9e84bc132de993d11199c2/internal/core/defaults/policy/shard_experts.mg#L21-L87
[clone]: https://github.com/theRebelliousNerd/codenerd/blob/bc610aeb9e8223823c9e84bc132de993d11199c2/internal/session/executor.go#L624-L628
[task]: https://github.com/theRebelliousNerd/codenerd/blob/bc610aeb9e8223823c9e84bc132de993d11199c2/internal/session/task_executor.go#L358-L441
[spawn]: https://github.com/theRebelliousNerd/codenerd/blob/bc610aeb9e8223823c9e84bc132de993d11199c2/internal/session/subagent.go#L164-L177
[process]: https://github.com/theRebelliousNerd/codenerd/blob/bc610aeb9e8223823c9e84bc132de993d11199c2/cmd/nerd/chat/process.go#L509-L526
[update]: https://github.com/theRebelliousNerd/codenerd/blob/bc610aeb9e8223823c9e84bc132de993d11199c2/cmd/nerd/chat/model_update.go#L168-L173
[background]: https://github.com/theRebelliousNerd/codenerd/blob/bc610aeb9e8223823c9e84bc132de993d11199c2/cmd/nerd/chat/background.go#L175-L235
[arbitration]: https://github.com/theRebelliousNerd/codenerd/blob/bc610aeb9e8223823c9e84bc132de993d11199c2/internal/core/defaults/policy/routing_arbitration.mg#L316-L326
[helper]: https://github.com/theRebelliousNerd/codenerd/blob/bc610aeb9e8223823c9e84bc132de993d11199c2/internal/session/work_steps.go#L685-L714
[context]: https://github.com/theRebelliousNerd/codenerd/blob/bc610aeb9e8223823c9e84bc132de993d11199c2/internal/session/executor.go#L1361-L1392
[expertcontext]: https://github.com/theRebelliousNerd/codenerd/blob/bc610aeb9e8223823c9e84bc132de993d11199c2/internal/session/expert_context.go#L17-L34
[step]: https://github.com/theRebelliousNerd/codenerd/blob/bc610aeb9e8223823c9e84bc132de993d11199c2/internal/session/work_steps.go#L811-L815
[compiler]: https://github.com/theRebelliousNerd/codenerd/blob/bc610aeb9e8223823c9e84bc132de993d11199c2/internal/prompt/compiler_experts.go#L49-L135
[generation]: https://github.com/theRebelliousNerd/codenerd/blob/bc610aeb9e8223823c9e84bc132de993d11199c2/internal/prompt/compiler_generation.go#L117
[preflight]: https://github.com/theRebelliousNerd/codenerd/blob/bc610aeb9e8223823c9e84bc132de993d11199c2/internal/prompt/compiler_pipeline.go#L76-L90
[jitaudit]: https://github.com/theRebelliousNerd/codenerd/blob/bc610aeb9e8223823c9e84bc132de993d11199c2/Docs/audits/jit-generation-aware-optimization.md#L199-L218
[perfaudit]: https://github.com/theRebelliousNerd/codenerd/blob/bc610aeb9e8223823c9e84bc132de993d11199c2/Docs/audits/mangle-kernel-performance.md
[consultation]: https://github.com/theRebelliousNerd/codenerd/blob/bc610aeb9e8223823c9e84bc132de993d11199c2/Docs/audits/consultation-coordination-and-safe-reuse.md#L116-L139
[campaign]: https://github.com/theRebelliousNerd/codenerd/blob/75240b6649e9e7fce0c117ac5b03c6f4f0a95840/Docs/audits/campaign-context-seed-scheduler-ownership.md
[roster]: https://github.com/theRebelliousNerd/codenerd/blob/75240b6649e9e7fce0c117ac5b03c6f4f0a95840/Docs/audits/specialist-roster-workspace-identity.md
[session]: https://github.com/theRebelliousNerd/codenerd/blob/bc610aeb9e8223823c9e84bc132de993d11199c2/internal/session/README.md#L42-L43
[adapter]: https://github.com/theRebelliousNerd/codenerd/blob/bc610aeb9e8223823c9e84bc132de993d11199c2/internal/system/factory_adapters.go#L91-L135
[transaction]: https://github.com/theRebelliousNerd/codenerd/blob/bc610aeb9e8223823c9e84bc132de993d11199c2/internal/types/transaction.go#L10-L18
[tests]: https://github.com/theRebelliousNerd/codenerd/blob/bc610aeb9e8223823c9e84bc132de993d11199c2/internal/experts/select_test.go#L31-L99
[cortex]: https://github.com/theRebelliousNerd/codenerd/blob/bc610aeb9e8223823c9e84bc132de993d11199c2/internal/core/cortex_kernel.go#L487-L678
[spawner]: https://github.com/theRebelliousNerd/codenerd/blob/bc610aeb9e8223823c9e84bc132de993d11199c2/internal/session/spawner.go#L462-L470
[run]: https://github.com/theRebelliousNerd/codenerd/blob/bc610aeb9e8223823c9e84bc132de993d11199c2/internal/session/subagent.go#L304-L356
[capacity]: https://github.com/theRebelliousNerd/codenerd/blob/bc610aeb9e8223823c9e84bc132de993d11199c2/internal/session/spawner.go#L134-L139
[scanstart]: https://github.com/theRebelliousNerd/codenerd/blob/bc610aeb9e8223823c9e84bc132de993d11199c2/cmd/nerd/chat/process.go#L498-L505
[scanload]: https://github.com/theRebelliousNerd/codenerd/blob/bc610aeb9e8223823c9e84bc132de993d11199c2/cmd/nerd/chat/process_dream_delegation.go#L90-L120
[scanfull]: https://github.com/theRebelliousNerd/codenerd/blob/bc610aeb9e8223823c9e84bc132de993d11199c2/internal/world/fs.go#L358-L388
[scanincremental]: https://github.com/theRebelliousNerd/codenerd/blob/bc610aeb9e8223823c9e84bc132de993d11199c2/internal/world/incremental_scan.go#L415-L433
[detectlanguage]: https://github.com/theRebelliousNerd/codenerd/blob/bc610aeb9e8223823c9e84bc132de993d11199c2/internal/world/fs.go#L580-L661
[fanout]: https://github.com/theRebelliousNerd/codenerd/blob/bc610aeb9e8223823c9e84bc132de993d11199c2/cmd/nerd/chat/delegation_modes.go#L17-L46
[delegation]: https://github.com/theRebelliousNerd/codenerd/blob/bc610aeb9e8223823c9e84bc132de993d11199c2/cmd/nerd/chat/delegation.go#L35-L47
[taskrequest]: https://github.com/theRebelliousNerd/codenerd/blob/bc610aeb9e8223823c9e84bc132de993d11199c2/cmd/nerd/chat/delegation_routing.go#L354-L360
[preset]: https://github.com/theRebelliousNerd/codenerd/blob/bc610aeb9e8223823c9e84bc132de993d11199c2/internal/session/task_executor.go#L234-L252
[extensionroute]: https://github.com/theRebelliousNerd/codenerd/blob/bc610aeb9e8223823c9e84bc132de993d11199c2/internal/shards/registration.go#L106
[jobs]: https://github.com/theRebelliousNerd/codenerd/blob/bc610aeb9e8223823c9e84bc132de993d11199c2/cmd/nerd/chat/background.go#L69-L78
[fields]: https://github.com/theRebelliousNerd/codenerd/blob/bc610aeb9e8223823c9e84bc132de993d11199c2/internal/session/executor.go#L1549-L1553
