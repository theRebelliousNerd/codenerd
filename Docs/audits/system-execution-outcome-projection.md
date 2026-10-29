---
doc-class: deep-dive
subsystem: chat
implementation-status: proposed
last-verified: 2026-10-29
verified-against: 9dcb01b9736bf4763aa3ac391d29f133d73c6611
verification-method: static-source-inspection
---

# System execution failure must survive the TUI shard-result adapter

## Finding and scope

A completed system execution can retain an accurately measured failure in its original `execution_result` while the TUI adapter constructs a successful shard result from that same row. The parser preserves `false`; the adapter subsequently discards it and passes a nil error to `ResultToFacts`, whose nil-error branch emits `shard_success`. If the generated facts load successfully, the kernel also receives that contradictory success assertion.

This is a conditional outcome-projection defect, not an observed incident or runtime reproduction. It does not establish successful final prose, verified `/done`, durable persistence, permission bypass, or training effects. The proposed repair conserves the measured outcome after residual result selection; it does not claim that the current baseline proves full turn ownership or replace that work.

## Where the outcome changes

The source chain is narrow and explicit:

1. [Execution parsing][parser] reads `Args[3]` through `parseBool` into `Success`, preserving false, including `/false`. [Action mapping][mapping] recognizes delegated actions such as `delegate_reviewer`.
2. [System-result collection][collection] selects fresh execution rows against a baseline, conditionally waits up to 1,200 ms, and builds a response.
3. [Response construction][adapter] sorts newest first, maps a delegated action, formats `exec.Output`, and calls `buildShardResultPayload(shardType, task, exec.Output, nil)`. It does not forward `exec.Success`. It ignores the `LoadFacts` error and returns an assistant message with the payload.
4. [Payload construction][payload] mints a synthetic shard ID and invokes `ResultToFacts`. [That converter][facts] emits `shard_error` for a nonnil error, but `shard_success` for nil, alongside bounded output and recent context.

Thus an adapter-supplied nil error is treated as execution success, regardless of the measured status or what the formatter says. The false source row need not be malformed, misparsed, or assigned to the wrong turn. Even perfect membership cannot repair the loss of outcome.

Existing [parser fixtures][parsertests] assert `/true` and `/false` handling, and [turn-result fixtures][membershiptests] cover identity and empty-wait behavior. These inspected tests credit the repaired mechanisms; they do not exercise this failed mapped-delegation payload. They were not run.

[Model update][model] consumes `assistantMsg.ShardResult` and pushes the message to the surface. [Result storage][storage] retains fact strings in `ExtraData["facts"]`, sets `lastShardResult`, and maintains a ten-result in-memory window. These are handoff and retention mechanisms, not guarantees of durable persistence or subsequent model uptake. Nor does the synthetic shard ID establish equality with any `compile_shard` identity.

## A conditional production source route

A useful proposed fixture fixes the typed intent and routing facts rather than promising that natural-language wording will classify identically: read-only `/query`, action `/review`, concrete target `billing.go`, `scope_is_file`, and a reviewer candidate below the configured confidence threshold. Exclude clarification, early-return, multi-step, hold, and other diverting branches; in particular, assume no `clarification_question`.

[Evidence routing][evidence] prevents a direct answer for this case; [`/review` is a workhorse action][workhorse]. [Delegation policy][delegation] can still derive `delegate_task` independently of the [confidence condition on `should_delegate`][confidence]. Without `delegate_lane_ready`, the [lowest-precedence evidence lane][gather] produces `gather_evidence`. The [low-confidence clarification rule][clarifiers] is mutation-only, while the concrete target avoids the missing-target clarification rule.

[Turn processing][baseline] clears and rebases execution results, then [performs evidence gathering and reaches the residual system-delegation call][process] when `!answerDirectly`. This is not the ordinary read-only `RouteDelegate` background branch, which returns earlier.

The [system executive intent handler][intent] queries pending `delegate_task` and maps reviewer tasks without a `route_decision` gate in the inspected source. [Executive guards][executive] still apply barriers and boot checks before emitting `pending_action`.

The witness additionally requires an **already activated, running system router**, an available kernel, shard manager and event bus, enabled system shards, and an allowed action. [Constitution handling][constitution] checks permission and emits `permitted_action` only when the tactile router is already running; the permission verdict survives independently.

The nearby comment claiming that the router is never spawned in production is not sufficient counterevidence. [Factory initialization][factory] installs an on-demand watcher. [System policy][systempolicy] derives routing readiness from a permitted verdict, and an [activation rule][activation] activates the router; the [watcher][watcher] uses startup, event and sweep processing, with a one-second debounce and 30-second fallback. [On-demand shard management][ondemand] can spawn enabled profiles. The [router lifecycle][routerlife] explicitly supports on-demand execution continuing until context cancellation or `StopCh`, including an initial drain. Prior normal activity can therefore support the already-running prerequisite, but this inspection does not prove that a cold router receives the first action or that every first turn executes it.

The [router][router] checks routing and rate limits before invoking VirtualStore synchronously. [VirtualStore routing][vsrouting] retains boot, Dreamer, constitution and permitted-action validation. All actual guards must allow the action; this finding does not bypass them.

For an observed delegation worker or provider error, [the producer][producer] calls `ExecuteObserved` and returns `ActionResult{Success:false, Error:err.Error(), FactsToAdd:delegation_failed}` with a **nil Go error** and omitted, therefore empty, output. [Completion recording][completion] writes the real false status and bounded output to `execution_result`. A nil transport error is consequently compatible with a measured execution failure.

That completed row must arrive after the rebased baseline and before the selected observation, including any conditional 1.2-second wait. Choose one relevant fresh row without a newer competing execution. If the row is absent, late, blocked, or produced with a dormant router, this particular projection is not established. Late-row cross-turn ownership and slow-fallback duplicate execution remain separate obligations.

## Counterevidence limits the claim

The [direct fallback][fallback] passes `spawnErr` into payload construction and returns the error; it is not this defect. The [ordinary background delegation branch][background] also returns before this residual route. `run_tests` and unmapped actions in [response construction][adapter] do not use the shard payload and must remain controls.

Formatting is not itself a success declaration. [Delegation formatting][formatting] uses an interpreter for reviewer/tester results; coder headings say “Result” or “Output.” Its [interpretation-error fallback][interpretation] explicitly reports interpretation failure and includes raw output. No particular final wording follows from the contradictory fact.

Failure evidence is not erased. The original false row and `delegation_failed` remain; [audit, tool-event and operation-receipt paths][receipts] preserve failed status, including error information where available. [System summaries][summaries] preserve status, although collection/display depends on debug or `showSystemActions` settings. The error may already be visible. The issue is conflicting derived outcome, not universally hidden failure.

## Relationship to existing audits and current lanes

[The TUI evidence ownership audit][ownership] owns baseline, route and read-evidence membership. Its as-built discussion credits membership repairs and retains late-row and fallback obligations. This addendum begins after membership and asks whether the selected row's measured outcome survives translation.

[Delegated failure settlement][settlement] owns normal executor settlement and partial state; [verdict handoff][handoff] owns later verification status and reconstitution. Their repairs deserve credit, but neither establishes this residual execution-row conversion. A bounded audit search found the older ownership audit mentioning `buildResponseFromExecutions`, and no joint `execution_result` plus `shard_success` hit. This is not an exhaustive absence claim.

Inspection beginning at 17:37 UTC on October 29 compared the pinned main snapshot with code lanes `a53cfc308e8836c72370adaa46deba3f7b00180d`, `80af1863d3a3aa46a722ac8b0c8a0985582d4995`, `aa7988dec4c62116656ea722ff73b687a8509908`, and QA `51cfd42649552866d4aa54c53b68acf5740eb09b`. Main resolved to the same pin again at 17:48 UTC. The inspected adapter, parser, converter, producer, router and executive-intent blobs matched. Factory differences did not change the inspected on-demand watcher function. This does not claim every file matched.

Research tip `89329ac439acb7a8576f0a6189c1122ec9e6bd04` added streaming-grounding source ownership and project-knowledge lexical-term budget work, not this outcome adapter. No tests or runtime executions were performed for this audit.

## Proposed outcome contract

Introduce a typed adapter carrying observed success, failure or unknown; original action ID and turn identity; bounded output; and available error or partial-result observations. Execution transport outcome must remain distinct from later verification acceptance.

A known false status must never become success because payload construction supplies nil as its error argument. A missing or malformed status must remain unknown, not acquire a success default. Where the execution row lacks an error column, expose explicit failure with “diagnostic unavailable”; do not fabricate the original error.

Prefer a correlated typed diagnostic receipt when available. The current `delegation_failed` fact lacks an action ID, so joining it arbitrarily by shard type is unsafe. Preserve the original row and attach its action ID as correlated provenance, rather than blindly replacing shard IDs with action IDs. Migrate identity without silently breaking consumers that depend on synthetic IDs.

The same typed state should drive generated facts, UI status and retained result metadata. Preserve current permission checks, fail-closed behavior and output bounds. Do not automatically redispatch, add an LLM adjudicator, or retry without policy authorization.

A minimal repair can map observed false to a bounded explicit adapter error now, followed by a versioned receipt migration. However, the current `ResultToFacts` error branch omits `shard_output`; preserving partial output and original diagnostics requires deliberate typed-receipt handling, not merely switching the argument. Suppressing every system success would avoid some false positives but discard legitimate measured successes; reject that blanket workaround. Any handling of historical success records must require original receipts rather than reinterpret unsupported history.

## Cost and compatibility

No additional model or embedding calls are required. Typed projection is bounded O(rows) work alongside the existing scan and sort. A correlated diagnostic lookup can be indexed and bounded. Added status fields, facts, logs and storage must retain byte and count limits.

The real costs are schema versioning, backward compatibility, malformed or historical statuses, synthetic-ID dependencies, and preserving partial output on failure. Formatting changes might avoid interpreter calls, but no latency or spend saving has been measured. Ownership repair remains separate and should not be hidden inside this migration.

## Proposed evaluations not run

Compare the baseline, minimal typed mapping and full receipt design using the same controlled source fixtures and the production TUI entry path. Cover:

- The actual producer's false status with empty output; true status; false with nonempty partial output; and missing or malformed status.
- Old-baseline, late and competing fresh rows; cancellation; and failed `LoadFacts`.
- Absent, cold and warm routers; disabled system shards; missing event bus; and permission denial.
- High-confidence background delegation and direct fallback with nonnil error, plus `run_tests` and unmapped-action controls.
- Both `showSystemActions` settings, including independently visible failure evidence.

Measure original-to-derived status equality, forbidden success assertions, explicit error visibility, original-action correlation, dispatch count and permission invariance. Record CPU, allocations, fact count, bounded bytes, latency and provider spend. Acceptance requires preserving measured failures without extra execution or weakened guards. Runtime acceptance remains unproved.

[parser]: https://github.com/theRebelliousNerd/codenerd/blob/9dcb01b9736bf4763aa3ac391d29f133d73c6611/cmd/nerd/chat/process_dream_parsing.go#L10-L32
[parsertests]: https://github.com/theRebelliousNerd/codenerd/blob/9dcb01b9736bf4763aa3ac391d29f133d73c6611/cmd/nerd/chat/fact_readback_test.go#L9-L21
[membershiptests]: https://github.com/theRebelliousNerd/codenerd/blob/9dcb01b9736bf4763aa3ac391d29f133d73c6611/cmd/nerd/chat/turn_results_test.go#L37-L101
[mapping]: https://github.com/theRebelliousNerd/codenerd/blob/9dcb01b9736bf4763aa3ac391d29f133d73c6611/cmd/nerd/chat/process_dream_parsing.go#L79-L95
[collection]: https://github.com/theRebelliousNerd/codenerd/blob/9dcb01b9736bf4763aa3ac391d29f133d73c6611/cmd/nerd/chat/process_dream_delegation.go#L152-L175
[adapter]: https://github.com/theRebelliousNerd/codenerd/blob/9dcb01b9736bf4763aa3ac391d29f133d73c6611/cmd/nerd/chat/process_dream_delegation.go#L192-L239
[payload]: https://github.com/theRebelliousNerd/codenerd/blob/9dcb01b9736bf4763aa3ac391d29f133d73c6611/cmd/nerd/chat/process_dream_delegation.go#L282-L295
[facts]: https://github.com/theRebelliousNerd/codenerd/blob/9dcb01b9736bf4763aa3ac391d29f133d73c6611/internal/core/shards/manager_spawn.go#L691-L740
[model]: https://github.com/theRebelliousNerd/codenerd/blob/9dcb01b9736bf4763aa3ac391d29f133d73c6611/cmd/nerd/chat/model_update.go#L138-L162
[storage]: https://github.com/theRebelliousNerd/codenerd/blob/9dcb01b9736bf4763aa3ac391d29f133d73c6611/cmd/nerd/chat/model_lifecycle.go#L266-L295
[routerlife]: https://github.com/theRebelliousNerd/codenerd/blob/9dcb01b9736bf4763aa3ac391d29f133d73c6611/internal/shards/system/router.go#L194-L254
[evidence]: https://github.com/theRebelliousNerd/codenerd/blob/9dcb01b9736bf4763aa3ac391d29f133d73c6611/internal/core/defaults/policy/routing_arbitration.mg#L104-L131
[workhorse]: https://github.com/theRebelliousNerd/codenerd/blob/9dcb01b9736bf4763aa3ac391d29f133d73c6611/internal/core/defaults/policy/routing_arbitration.mg#L69-L75
[delegation]: https://github.com/theRebelliousNerd/codenerd/blob/9dcb01b9736bf4763aa3ac391d29f133d73c6611/internal/core/defaults/policy/delegation.mg#L95-L98
[confidence]: https://github.com/theRebelliousNerd/codenerd/blob/9dcb01b9736bf4763aa3ac391d29f133d73c6611/internal/core/defaults/policy/delegation.mg#L339-L344
[gather]: https://github.com/theRebelliousNerd/codenerd/blob/9dcb01b9736bf4763aa3ac391d29f133d73c6611/internal/core/defaults/policy/routing_arbitration.mg#L272-L310
[clarifiers]: https://github.com/theRebelliousNerd/codenerd/blob/9dcb01b9736bf4763aa3ac391d29f133d73c6611/internal/core/defaults/policy/routing_arbitration.mg#L374-L416
[baseline]: https://github.com/theRebelliousNerd/codenerd/blob/9dcb01b9736bf4763aa3ac391d29f133d73c6611/cmd/nerd/chat/process.go#L274-L283
[process]: https://github.com/theRebelliousNerd/codenerd/blob/9dcb01b9736bf4763aa3ac391d29f133d73c6611/cmd/nerd/chat/process.go#L881-L907
[intent]: https://github.com/theRebelliousNerd/codenerd/blob/9dcb01b9736bf4763aa3ac391d29f133d73c6611/internal/shards/system/executive_intent.go#L457-L514
[executive]: https://github.com/theRebelliousNerd/codenerd/blob/9dcb01b9736bf4763aa3ac391d29f133d73c6611/internal/shards/system/executive.go#L517-L588
[constitution]: https://github.com/theRebelliousNerd/codenerd/blob/9dcb01b9736bf4763aa3ac391d29f133d73c6611/internal/shards/system/constitution.go#L454-L479
[factory]: https://github.com/theRebelliousNerd/codenerd/blob/9dcb01b9736bf4763aa3ac391d29f133d73c6611/internal/system/factory.go#L2199-L2227
[systempolicy]: https://github.com/theRebelliousNerd/codenerd/blob/9dcb01b9736bf4763aa3ac391d29f133d73c6611/internal/core/defaults/policy/system_core.mg#L73-L83
[activation]: https://github.com/theRebelliousNerd/codenerd/blob/9dcb01b9736bf4763aa3ac391d29f133d73c6611/internal/core/defaults/policy/system_core.mg#L161-L164
[watcher]: https://github.com/theRebelliousNerd/codenerd/blob/9dcb01b9736bf4763aa3ac391d29f133d73c6611/internal/core/ondemand_watcher.go#L12-L69
[ondemand]: https://github.com/theRebelliousNerd/codenerd/blob/9dcb01b9736bf4763aa3ac391d29f133d73c6611/internal/core/shards/manager_ondemand.go#L44-L88
[router]: https://github.com/theRebelliousNerd/codenerd/blob/9dcb01b9736bf4763aa3ac391d29f133d73c6611/internal/shards/system/router.go#L293-L417
[vsrouting]: https://github.com/theRebelliousNerd/codenerd/blob/9dcb01b9736bf4763aa3ac391d29f133d73c6611/internal/core/virtual_store_routing.go#L35-L179
[producer]: https://github.com/theRebelliousNerd/codenerd/blob/9dcb01b9736bf4763aa3ac391d29f133d73c6611/internal/core/virtual_store_actions.go#L904-L937
[completion]: https://github.com/theRebelliousNerd/codenerd/blob/9dcb01b9736bf4763aa3ac391d29f133d73c6611/internal/core/virtual_store_routing.go#L187-L205
[fallback]: https://github.com/theRebelliousNerd/codenerd/blob/9dcb01b9736bf4763aa3ac391d29f133d73c6611/cmd/nerd/chat/process_dream_delegation.go#L245-L279
[background]: https://github.com/theRebelliousNerd/codenerd/blob/9dcb01b9736bf4763aa3ac391d29f133d73c6611/cmd/nerd/chat/process.go#L497-L526
[formatting]: https://github.com/theRebelliousNerd/codenerd/blob/9dcb01b9736bf4763aa3ac391d29f133d73c6611/cmd/nerd/chat/process_dream_delegation.go#L297-L310
[interpretation]: https://github.com/theRebelliousNerd/codenerd/blob/9dcb01b9736bf4763aa3ac391d29f133d73c6611/cmd/nerd/chat/process_dream_delegation.go#L453-L464
[receipts]: https://github.com/theRebelliousNerd/codenerd/blob/9dcb01b9736bf4763aa3ac391d29f133d73c6611/internal/core/virtual_store_routing.go#L208-L307
[summaries]: https://github.com/theRebelliousNerd/codenerd/blob/9dcb01b9736bf4763aa3ac391d29f133d73c6611/cmd/nerd/chat/process_dream.go#L550-L690
[ownership]: https://github.com/theRebelliousNerd/codenerd/blob/9dcb01b9736bf4763aa3ac391d29f133d73c6611/Docs/audits/tui-evidence-turn-ownership.md#L250-L263
[settlement]: https://github.com/theRebelliousNerd/codenerd/blob/9dcb01b9736bf4763aa3ac391d29f133d73c6611/Docs/audits/delegated-turn-failure-settlement.md
[handoff]: https://github.com/theRebelliousNerd/codenerd/blob/9dcb01b9736bf4763aa3ac391d29f133d73c6611/Docs/audits/delegated-verdict-handoff.md

