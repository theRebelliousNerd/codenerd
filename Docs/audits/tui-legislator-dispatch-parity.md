---
doc-class: research-proposal
subsystem: tui-system-task-dispatch
implementation-status: proposed-not-implemented
last-verified: 2026-10-29
verified-against: 9dcb01b9736bf4763aa3ac391d29f133d73c6611
verification-method: static-source-inspection-only
---

# TUI Legislator dispatch parity

## Summary and decision

The explicit TUI command `/legislate <constraint>` advertises safety-rule synthesis and ratification, but its inspected call path sends the request to the raw JIT `TaskExecutor`, not to the registered Legislator system implementation. Normalizing the name `legislator` into the intent verb `/legislate` does not select that Go implementation. This is a bounded backend-dispatch gap in an explicit command, not evidence of a permission bypass, an inevitable false-success message, or a demonstrated model failure.

The recommended first change is a narrow command adapter that invokes the registered Legislator service while retaining the TUI's cancellation, result publication, observer, and accounting behavior. A reusable typed dispatcher is a reasonable subsequent design, but must preserve the full request and execution context; the existing string-based Cortex wrappers are not a lossless replacement for every rich chat call. If the service contract cannot be supported, the honest alternative is to rename or disable the command as proposal-only rather than advertise ratification.

This is a source-only proposal. No implementation, tests, application runtime, inference experiments, or repository execution were performed. The evidence establishes which backend the command selects before model inference; it does not establish whether a particular natural-language directive would compile, be accepted, or be installed by the specialized service.

## Review snapshot and novelty boundary

The code snapshot is [`theRebelliousNerd/codenerd` main at `9dcb01b9736bf4763aa3ac391d29f133d73c6611`][main], rechecked on 2026-10-29 at 12:13 UTC. All code links below are pinned to that commit. Active code-branch heads [`a53cfc3`][branch1], [`80af186`][branch2], and [`aa7988d`][branch3] were unchanged; at 12:14 UTC, their `cmd/nerd/chat/delegation.go` and `commands_handlers_analysis.go` files were verified byte-identical to main. The inspected [`Research1018` head `f0791c0`][research] added unrelated audits: symbol focus, revision sentinels, omitted arguments, JIT relevance budgets, vector repair lifecycles, Mangle fingerprints, KNN recall limits, and projection-cache allocation. Its latest two-audit advance contained no production change. These are snapshot observations, not claims about later work or every downstream component on another branch.

The audit search covered 92 main-branch audits. The neighboring [predicate inventory semantic fidelity audit][predicateaudit] owns the fidelity of the predicate corpus available to the Legislator, not whether this TUI command invokes the Legislator service. The existing shard architecture documentation already distinguishes registered Go system implementations from JIT personas and identifies the Cortex front door. The session implementation specification records name-to-verb normalization, including `legislator` to `/legislate`; a normalization entry does not demonstrate execution-backend equivalence. This proposal links to, rather than duplicates, the predicate-corpus audit. [Existing architecture distinction][architecture], [documented name map][namemap].

The scope is deliberately narrow: the explicit `/legislate` command, its backend-selection contract, and a secondary image-routing parity control. It does not assert that every system-named persona must execute through a system shard, and does not include `/clarify`, whose generic route has separate explicit configuration support.

## Observable command contract

The command registry routes `/legislate` to its dedicated handler. Its help describes synthesis and ratification of a safety rule. The handler accepts a constraint, emits the progress text “Legislator engaged. Compiling and ratifying rule...”, and schedules `spawnShard("legislator", task)`. That is stronger than a request for generic advice about writing a rule. The supplied usage example is `/legislate Stop using fmt.Printf; use log.Info instead.` [Command registration][commands], [command help][help], [handler and example][handler].

The registered service is concrete, not merely a prompt persona. Registration creates `LegislatorShard`, attaches the parent kernel, virtual store, LLM client, and prompt assembler, and defines an on-demand `TypeSystem` profile. Its `Execute` path compiles the directive, performs a fast stratification check, asks `RuleCourt.RatifyRule`, validates learned rules against the kernel schema, and then calls `HotLoadLearnedRule`. Direct Mangle-looking input takes a validation-only compilation branch; natural-language generation follows a separate feedback-backed path. These checks and side effects are the material distinction from assigning the same name to a general JIT task. [Factory registration][registration], [system profile][profile], [service execution][legislator], [direct-rule compilation][compile].

This contract is not a promise that any arbitrary directive will succeed. Missing dependencies, readiness, corpus quality, generation, rule validation, court rejection, or hot-load failure can prevent application. In particular, several rejection paths return explanatory text with a nil Go error. Backend completion must therefore remain distinct from rule acceptance and installation. [Legislator rejection and application branches][legislator].

## Conditional witness and exact call path

A sufficient witness has the following preconditions:

- The normal shared TUI boot has succeeded.
- The Legislator profile and factory are registered and not disabled, and `taskExecutor` is available.
- The user submits a nonempty constraint to the explicit command, for example the command's own harmless logging-style example above.
- The returned Bubble Tea command is actually executed, including its relevant batched child command.

The example only supplies nonempty input to reveal dispatch. It is not asserted to be a schema-valid policy or a directive the current compiler will necessarily accept.

Under these conditions, the inspected source yields this path:

1. Shared TUI boot obtains both `cortex.ShardManager` and `cortex.TaskExecutor`. The factory constructs the latter as `session.NewJITExecutor`. Merely retaining a shard manager does not make it the execution backend. [Shared boot][boot], [JIT construction][jitfactory].
2. `handleCmdLegislate` trims the command prefix and invokes `spawnShard("legislator", task)`. [Handler][handler].
3. `spawnShard` obtains the session operation context and calls `m.spawnTask(ctx, shardType, task)`. That helper only checks for a task executor, adds model context, and calls `m.taskExecutor.Execute`. It contains no registered-system or image-provider dispatch. [TUI wrapper][wrapper], [raw task helper][helper].
4. `shardTypeToTaskRequest` constructs a request with `IntentVerb` and `Task`, leaving persona normalization to the executor. The executor consults `persona_verb`, whose policy maps `/legislator` to `/legislate`. That is a name conversion, not a `ShardManager.Spawn` call. [Request conversion][conversion], [normalization][normalization], [persona policy][persona].
5. In the inspected default policy, `/legislate` is not one of the isolated verbs. The ordinary inline route clones the session executor, builds a preset intent, and invokes `ProcessWithIntent`. The defect is already established by backend selection; it does not depend on predicting the subsequent model response. [Isolation policy][isolation], [JIT execution][jit].

The inspected default runtime `ConfigAtom` catalog has no dedicated `/legislate` entry, and the routing policy has no corresponding `verb_persona` entry that substitutes for the registered host service. This is not an absence of Legislator prompt content: the repository has identity and system prompt atoms for that role. Missing runtime config entries can use general policies, and an unknown verb has a general tool floor. That floor includes read/search, subagent expansion, and MCP tools; it must not be described as absolutely read-only. Extensions or independent tool capabilities might perform other work. The supported conclusion is narrower: this first-class command does not select the dedicated request-bound Legislator callback through the inspected dispatch path. A persona identity alone is not execution authority. [Runtime config catalog][configcatalog], [config fallback][promptfallback], [Legislator prompt content][identity], [verb routing and general floor][routing], [core tool floor][corefloor].

After execution, `spawnShard` converts the returned result into facts, attempts to publish them, records execution for learning, reports an error if one was returned, or publishes a generic “Shard Execution Complete” wrapper. `ResultToFacts` stores output/context and success/error facts; it does not parse the output as a rule or call `HotLoadLearnedRule`. This fact-publication bridge is therefore not itself ratification. None of those operations independently proves that a rule was installed. A generic model could return advice, prose, a refusal, an error, or something else; this review does not predict its output or claim that the wrapper inevitably says a policy was applied. Generic verdict-from-evidence concerns are already documented; the receipt extension below is specifically about proposed, rejected, and applied rules. [Result publication and observers][wrapper], [result-to-fact implementation][resultfacts], [existing verdict work][verdictaudit].

## Existing correct paths and counterevidence

The Cortex front door already distinguishes system execution from JIT execution. `SpawnTaskWithTarget` normalizes the shard name, sends image aliases to the dedicated shard-manager route, sends a registered `TypeSystem` profile to `ShardManager.Spawn`, and otherwise constructs a JIT request. The manager resolves an exact registered factory before its JIT-delegator fallback. This supplies a concrete backend-parity reference, not a proposal to resurrect deleted domain-shard implementations. [Cortex dispatch][cortex], [factory precedence][managerfactory].

The current synchronous manager path waits for `GetResult`; on cancellation it detaches the abandoned result. It does not merely return an asynchronous spawn ID as a completed result. The direct CLI also distinguishes system tasks and has a helper that waits for a terminal result or context cancellation. These facts rule out an outdated explanation that the remedy only needs to wait for a currently fire-and-forget system spawn. [Manager wait and cancellation][managerwait], [CLI system dispatch and wait][cli].

The observed Cortex wrapper deliberately returns an observation with output but no verdict for a system fallback. It does not synthesize a `Done` receipt. Any adapter should preserve this unknown-outcome meaning until it has service-specific evidence. Likewise, nil error from Legislator cannot be converted directly into “applied”, because stratification, court, and schema rejection can be successful backend completions. [Observed fallback][observed], [service rejection branches][legislator].

Other production code can independently hot-load learned rules. That possibility is not a demonstrated replacement for a request-bound `/legislate` handoff. The inspected system activation policy covers other automatic system work, not an explicit Legislator handoff for this command. Consequently, this finding is not the global claim that no rule can ever be installed after a generic task. [Automatic system activation policy][activation].

The existing command tests cover the no-argument usage case and include an argument-bearing `/legislate` command in a broader command-handler test. The latter calls `handleCommand`; it does not execute the returned `tea.Cmd` to verify the chosen backend, court execution, or persistence. This is a source observation about coverage, not a report of a test failure. No tests were run. [Usage test][usagetest], [handler test][handlertest].

## Secondary image parity control

The generic TUI `/spawn` handler uses the same helper. If given `image_generator`, it reaches the JIT executor's explicit image-alias refusal instead of the dedicated image route that Cortex and the shard manager provide. The refusal is an intentional provider-safety boundary: image generation must not silently use the worker LLM. It must remain intact. Since the `/spawn` help does not explicitly advertise images, this is secondary corroboration and a useful regression control, not the core user-visible contract behind this proposal. [Generic spawn handler][spawnhandler], [image refusal][normalization], [Cortex image route][cortex], [manager image guards][managerwait], [missing-factory image guard][managerfactory].

## Proposed backend contract

Backend selection and persona normalization should be separate responsibilities. Normalization answers which intent or persona was named; dispatch answers which registered implementation is authorized and available to execute that command. Selecting a service must not widen its existing permissions or interpret a model's rule text as permission to apply it.

For this command, require the following behavior:

- A supported and available explicit Legislator command invokes the registered Legislator service exactly once. An unavailable or disabled service produces an explicit unavailable result, not a silent generic substitute advertised as ratification.
- The request retains the user's original words and the relevant `TaskRequest.Request`, `Task`, `Target`, and `Constraint` semantics, together with priority, session context, model context where appropriate, authorization/restriction context, cancellation, budgets, and accounting. These are preservation requirements, not a claim that all are fields of `TaskRequest` itself. [Current request fields][request].
- The result separates backend completion from the policy effect. A typed receipt can distinguish `proposed`, `rejected`, `applied`, `failed`, and `unverified`, bind the final admitted or installed rule digest and source/request identity, retain the original candidate where useful, and carry the actual validation and application evidence. Feedback sanitization or kernel repair may change the rule text; do not assume the first proposal equals the installed rule. Rejection is not application; absent evidence is not success. Cancellation or an ambiguous side-effect boundary should remain unverified unless an authoritative receipt settles it.
- `applied` means the existing guarded application path has supplied evidence for that final rule. Generic `Done`, a nil Go error, an agent label, or plausible prose is insufficient. This does not promise propagation to every domain or kernel; cross-kernel learned-policy propagation was not investigated.
- Existing learned-rule validation, RuleCourt checks, protected permission heads, and host approval prohibitions remain enforced. Do not auto-apply emitted rule text outside that path or relax controls to make a demonstration pass.
- Preserve the TUI's operation lifetime, status and observer events, result/fact publication, execution records, and user-facing error handling. Publication failures must not automatically replay an already installed effect. A result receipt and an effect identity are needed to reconcile reporting without duplicate installation.

## Options and staged remedy

| Option | Scope and benefit | Main cost or limitation |
| --- | --- | --- |
| Narrow explicit command adapter | Route `/legislate` to its registered service, preserving the surrounding TUI lifecycle. Lowest dispatch blast radius. | Another adapter must be maintained; typed application receipts still require a service-aware boundary. |
| Shared typed service and JIT dispatcher | Reuse one backend decision across relevant entry points while keeping generic personas and user agents on JIT. | Broader API and compatibility work; each context, request, and observation field needs explicit preservation. |
| Honest proposal-only command | Rename, constrain, or disable the current command so its copy promises only what the generic path provides. | Does not deliver the existing synthesis-and-ratification feature; users need a clearly documented service path. |

Start with the narrow adapter unless a typed dispatcher is already being introduced for a justified broader migration. Keep backend selection local and deterministic. Generalize only with explicit per-command contracts; do not infer from this finding that every system-like name or every existing chat call should be switched wholesale.

In particular, replacing every rich TUI call with the current Cortex string wrapper is not sufficient. The inspected wrappers reconstruct only `IntentVerb`, `Task`, and optionally `Target`, whereas `TaskRequest` also carries `Request` and `Constraint`, and other chat execution paths have richer session and observation needs. A shared dispatcher should accept and preserve the existing structured request and relevant context, rather than flattening them and recreating parallel routing inputs. [Cortex request construction][cortex], [observed request construction][observed], [request semantics][request].

Add a typed service receipt before making an “applied” UI claim. During a staged rollout, an absent service receipt should remain unknown/unverified, not be inferred from the output text. Retain the existing rejection details for users and diagnostics, and preserve the source/request identity and final-rule evidence if a later publication step fails.

## Costs and failure containment

The backend decision itself can be a local profile/factory lookup; it does not inherently need another model call. The execution costs are different, not automatically lower. A direct rule avoids the natural-language synthesis branch, but that alone does not guarantee zero provider calls elsewhere, such as a configured kernel-repair interceptor during hot-load. Natural-language compilation and repair can make different model calls than the generic JIT loop. Its adapter already carries a cost guard, and its generation path requires JIT prompt support. RuleCourt sandboxing, validation, and hot-load work have real compute or I/O costs that belong to the command's advertised effect. Measure these components rather than claiming a token or latency improvement without observations. [Dispatch lookup][cortex], [service stages][legislator], [direct-rule branch][compile], [natural-language synthesis branch][nlsynthesis].

Reuse the existing priority, budget, shutdown, and cancellation mechanisms. Bound concurrency and distinguish queue time, service time, model calls, validation, application, and publication. The service profile currently allows a long timeout; routing parity is not permission to create unbounded work or invent new deadline semantics. The manager already propagates usage/session/broker context and applies the configured timeout before execution. Keep compatibility and adapter-maintenance costs explicit. [System profile][profile], [manager cancellation][managerwait], [manager execution context][managercontext], [JIT priority propagation][jit].

Important failure cases include unavailable/disabled registration, missing dependencies, invalid schema, failed compilation, court rejection, timeout/cancellation, hot-load failure, and post-effect result publication failure. The last case is especially important: reporting can fail after an effect is installed. Do not retry the whole command merely to repair reporting, and do not mark an uncertain effect applied without evidence. This proposal does not claim that all of those cases currently occur; they are requirements for a safe remedy.

## Future fair evaluation

The following evaluation is proposed, not executed. Pin the repository, configuration, fixture corpus, available factories, model choice if any, and permission policy before comparison.

1. **Deterministic routing witness.** Drive the actual argument-bearing command through its returned `tea.Cmd`, executing the selected task child of the batch without running spinner work indefinitely. With fake service and JIT endpoints, assert one selected Legislator service call and zero raw JIT calls. Verify unavailable/disabled-service behavior separately. Testing only `handleCommand` construction does not establish execution parity.
2. **Negative routing controls.** Confirm ordinary coder/persona and user-agent tasks still reach JIT. Confirm image aliases retain their dedicated-provider requirement and never fall through to the worker path when the image client or factory is missing. Do not remove the existing fail-closed guard to make this control pass.
3. **Application receipts.** Adapt established direct-rule fixtures and independently qualify them through the registered service to exercise both rejection and application without relying on natural-language inference. A fixture accepted by the kernel alone is not yet proof of acceptance by the full court and service path. Assert the distinction among backend completion, nil-error rejection, and a verified application receipt for the final admitted/installed rule; retain the original candidate when repair changes it. Existing learned-grant tests supply narrowing and host-witness refusal controls, but their definitions were only read, not run here. Do not invent a valid rule or weaken permissions for the test. [Existing fixture definitions][grantfixtures].
4. **Preservation and failure controls.** Check original request text, target, constraints, priority, session and authorization/restriction context, model context, observer/accounting signals, cancellation, and unknown observation semantics. Cover nil dependencies, unavailable JIT/prompt/corpus support where relevant, invalid schema, court rejection, timeout, and hot-load failure. Exercise publication failure after application and assert no effect replay.
5. **Optional separate model evaluation.** Only after deterministic backend parity is established, compare natural-language compilation outcomes on fixed inputs and configuration. Report rejection, application, failure, and unverified outcomes separately; record generation/repair calls and validation/application costs. Do not count generic `Done` as policy installation or conflate model variance with dispatch correctness.
6. **Fair alternatives.** Compare the narrow adapter, shared typed dispatcher, and honest proposal-only contract against the same input set. Assess contract fidelity, preserved context, failure behavior, maintenance scope, and measured cost. A proposal-only variant may be honest while intentionally not satisfying the original ratification feature; make that distinction explicit rather than scoring fluent prose as an applied rule.

Acceptance requires demonstrated backend selection and honest effect reporting, not guaranteed acceptance of every directive. Boot readiness, intent perception, predicate-corpus fidelity, and natural-language synthesis quality are related but distinct concerns; this dispatch proposal neither claims to regress them nor resolves them by itself.

## Evidence limits

The central result is a static, conditional call-path finding at an immutable source snapshot. The specialized handler, shared boot wiring, raw JIT dispatch, registered service, factory precedence, and service validation/application stages were inspected directly. Branch and audit inventory checks narrow the novelty claim to the inspected snapshot. No end-to-end success, latency, token count, actual policy mutation, or model-generated answer was observed. Generic extensions may have independent capabilities, and background learning may apply unrelated rules; neither is established here as the missing request-bound service dispatch.

[main]: https://github.com/theRebelliousNerd/codenerd/commit/9dcb01b9736bf4763aa3ac391d29f133d73c6611
[branch1]: https://github.com/theRebelliousNerd/codenerd/commit/a53cfc308e8836c72370adaa46deba3f7b00180d
[branch2]: https://github.com/theRebelliousNerd/codenerd/commit/80af1863d3a3aa46a722ac8b0c8a0985582d4995
[branch3]: https://github.com/theRebelliousNerd/codenerd/commit/aa7988dec4c62116656ea722ff73b687a8509908
[research]: https://github.com/theRebelliousNerd/codenerd/commit/f0791c0641aa51979f78f2a12a7faed0202498a1
[predicateaudit]: https://github.com/theRebelliousNerd/codenerd/blob/9dcb01b9736bf4763aa3ac391d29f133d73c6611/Docs/audits/predicate-inventory-semantic-fidelity.md
[namemap]: https://github.com/theRebelliousNerd/codenerd/blob/9dcb01b9736bf4763aa3ac391d29f133d73c6611/Docs/architecture/session/IMPLEMENTED_SPEC.md#L631-L633
[verdictaudit]: https://github.com/theRebelliousNerd/codenerd/blob/9dcb01b9736bf4763aa3ac391d29f133d73c6611/Docs/journeys/impl/S1-verdict-from-evidence.md
[architecture]: https://github.com/theRebelliousNerd/codenerd/blob/9dcb01b9736bf4763aa3ac391d29f133d73c6611/internal/shards/README.md#L16-L35
[commands]: https://github.com/theRebelliousNerd/codenerd/blob/9dcb01b9736bf4763aa3ac391d29f133d73c6611/cmd/nerd/chat/commands.go#L109-L110
[help]: https://github.com/theRebelliousNerd/codenerd/blob/9dcb01b9736bf4763aa3ac391d29f133d73c6611/cmd/nerd/chat/command_categories.go#L349-L354
[handler]: https://github.com/theRebelliousNerd/codenerd/blob/9dcb01b9736bf4763aa3ac391d29f133d73c6611/cmd/nerd/chat/commands_handlers_analysis.go#L15-L36
[registration]: https://github.com/theRebelliousNerd/codenerd/blob/9dcb01b9736bf4763aa3ac391d29f133d73c6611/internal/shards/registration.go#L795-L802
[profile]: https://github.com/theRebelliousNerd/codenerd/blob/9dcb01b9736bf4763aa3ac391d29f133d73c6611/internal/shards/registration.go#L1050-L1063
[legislator]: https://github.com/theRebelliousNerd/codenerd/blob/9dcb01b9736bf4763aa3ac391d29f133d73c6611/internal/shards/system/legislator.go#L219-L277
[compile]: https://github.com/theRebelliousNerd/codenerd/blob/9dcb01b9736bf4763aa3ac391d29f133d73c6611/internal/shards/system/legislator.go#L279-L307
[nlsynthesis]: https://github.com/theRebelliousNerd/codenerd/blob/9dcb01b9736bf4763aa3ac391d29f133d73c6611/internal/shards/system/legislator.go#L310-L359
[boot]: https://github.com/theRebelliousNerd/codenerd/blob/9dcb01b9736bf4763aa3ac391d29f133d73c6611/cmd/nerd/chat/session_shared_boot.go#L87-L106
[jitfactory]: https://github.com/theRebelliousNerd/codenerd/blob/9dcb01b9736bf4763aa3ac391d29f133d73c6611/internal/system/factory.go#L2692-L2705
[wrapper]: https://github.com/theRebelliousNerd/codenerd/blob/9dcb01b9736bf4763aa3ac391d29f133d73c6611/cmd/nerd/chat/delegation.go#L759-L824
[helper]: https://github.com/theRebelliousNerd/codenerd/blob/9dcb01b9736bf4763aa3ac391d29f133d73c6611/cmd/nerd/chat/delegation.go#L35-L47
[conversion]: https://github.com/theRebelliousNerd/codenerd/blob/9dcb01b9736bf4763aa3ac391d29f133d73c6611/cmd/nerd/chat/delegation_routing.go#L354-L361
[normalization]: https://github.com/theRebelliousNerd/codenerd/blob/9dcb01b9736bf4763aa3ac391d29f133d73c6611/internal/session/task_executor.go#L144-L198
[persona]: https://github.com/theRebelliousNerd/codenerd/blob/9dcb01b9736bf4763aa3ac391d29f133d73c6611/internal/core/defaults/policy/delegation.mg#L499-L512
[isolation]: https://github.com/theRebelliousNerd/codenerd/blob/9dcb01b9736bf4763aa3ac391d29f133d73c6611/internal/core/defaults/policy/delegation.mg#L516-L539
[jit]: https://github.com/theRebelliousNerd/codenerd/blob/9dcb01b9736bf4763aa3ac391d29f133d73c6611/internal/session/task_executor.go#L358-L448
[promptfallback]: https://github.com/theRebelliousNerd/codenerd/blob/9dcb01b9736bf4763aa3ac391d29f133d73c6611/internal/prompt/config_factory.go#L127-L151
[configcatalog]: https://github.com/theRebelliousNerd/codenerd/blob/9dcb01b9736bf4763aa3ac391d29f133d73c6611/internal/prompt/config_factory.go#L408-L552
[identity]: https://github.com/theRebelliousNerd/codenerd/blob/9dcb01b9736bf4763aa3ac391d29f133d73c6611/internal/prompt/atoms/identity/legislator.yaml#L1-L25
[routing]: https://github.com/theRebelliousNerd/codenerd/blob/9dcb01b9736bf4763aa3ac391d29f133d73c6611/internal/core/defaults/policy/intent_routing_rules.mg#L403-L497
[corefloor]: https://github.com/theRebelliousNerd/codenerd/blob/9dcb01b9736bf4763aa3ac391d29f133d73c6611/internal/core/defaults/policy/intent_routing_rules.mg#L265-L276
[cortex]: https://github.com/theRebelliousNerd/codenerd/blob/9dcb01b9736bf4763aa3ac391d29f133d73c6611/internal/system/factory.go#L520-L558
[managerfactory]: https://github.com/theRebelliousNerd/codenerd/blob/9dcb01b9736bf4763aa3ac391d29f133d73c6611/internal/core/shards/manager_spawn.go#L266-L311
[managerwait]: https://github.com/theRebelliousNerd/codenerd/blob/9dcb01b9736bf4763aa3ac391d29f133d73c6611/internal/core/shards/manager_spawn.go#L150-L203
[managercontext]: https://github.com/theRebelliousNerd/codenerd/blob/9dcb01b9736bf4763aa3ac391d29f133d73c6611/internal/core/shards/manager_spawn.go#L384-L449
[resultfacts]: https://github.com/theRebelliousNerd/codenerd/blob/9dcb01b9736bf4763aa3ac391d29f133d73c6611/internal/core/shards/manager_spawn.go#L691-L740
[cli]: https://github.com/theRebelliousNerd/codenerd/blob/9dcb01b9736bf4763aa3ac391d29f133d73c6611/cmd/nerd/cmd_spawn.go#L173-L259
[observed]: https://github.com/theRebelliousNerd/codenerd/blob/9dcb01b9736bf4763aa3ac391d29f133d73c6611/internal/system/factory.go#L575-L597
[activation]: https://github.com/theRebelliousNerd/codenerd/blob/9dcb01b9736bf4763aa3ac391d29f133d73c6611/internal/core/defaults/policy/system_shards.mg#L23-L40
[usagetest]: https://github.com/theRebelliousNerd/codenerd/blob/9dcb01b9736bf4763aa3ac391d29f133d73c6611/cmd/nerd/chat/commands_test.go#L384-L400
[handlertest]: https://github.com/theRebelliousNerd/codenerd/blob/9dcb01b9736bf4763aa3ac391d29f133d73c6611/cmd/nerd/chat/commands_test.go#L699-L733
[spawnhandler]: https://github.com/theRebelliousNerd/codenerd/blob/9dcb01b9736bf4763aa3ac391d29f133d73c6611/cmd/nerd/chat/commands_handlers_analysis.go#L573-L594
[request]: https://github.com/theRebelliousNerd/codenerd/blob/9dcb01b9736bf4763aa3ac391d29f133d73c6611/internal/session/task_executor.go#L20-L63
[grantfixtures]: https://github.com/theRebelliousNerd/codenerd/blob/9dcb01b9736bf4763aa3ac391d29f133d73c6611/internal/core/learned_grant_path_test.go#L118-L178
