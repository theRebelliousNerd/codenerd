# Multi step decomposition lowercases target identity before execution

## Decision

Preserve original source spans when case-insensitive multi-step matching extracts executable tasks. Current corpus matching lowercases the entire request and returns captures from that normalized copy. Explicit-sequence decomposition uses those captures as both structured targets and task text. A normal admitted request naming `Client.go` and `Server.go` can consequently dispatch requests naming `client.go` and `server.go` instead.

The supported finding is a **deterministic target and task identity change at a production handoff**, not an observed wrong-file write. On a case-sensitive filesystem the altered path may be missing or may name another existing file; identifiers and case-sensitive literal content can differ even on a case-insensitive filesystem. A model, retained original request, context, delivery judgment, or ordinary safety gate may recover or contain the problem. Those protections do not make the extracted task faithful by construction.

**Evidence status:** static source and test-text inspection only. No tests, application execution, inference, failure injection, benchmarks, or repository changes were performed. Main was revalidated at `388205138ba6d47c1cff0c5e97c896dfa642a391` on 2026-10-30. Audit inventories were checked through `research-main-20261030-1117`, pinned at `c3a596c8e9444a523b3368a25843cb3bad2de112`.

## Scope and prior repairs

The [repository guidance](https://github.com/theRebelliousNerd/codenerd/blob/388205138ba6d47c1cff0c5e97c896dfa642a391/agents.md) distinguishes model interpretation from the kernel's executive decisions and requires validation through normal production paths with CodeDOM/world context. The proposed change respects that split: Go can perform case-insensitive language matching while preserving the bytes of the target it hands to execution; Mangle continues to decide admission and routing.

Several adjacent improvements are real and should remain intact:

- [`parseVerbAndTarget`](https://github.com/theRebelliousNerd/codenerd/blob/388205138ba6d47c1cff0c5e97c896dfa642a391/cmd/nerd/chat/multistep_decomposer.go#L642-L727) now favors the leading verb and whole-word matches instead of finding a short synonym inside an unrelated word. Its target is sliced from the text it receives. This audit concerns the already-lowercased text supplied by its caller, not a request to undo that lexical repair.
- [`anchorVerbPairToIntent`](https://github.com/theRebelliousNerd/codenerd/blob/388205138ba6d47c1cff0c5e97c896dfa642a391/cmd/nerd/chat/delegation_multistep.go#L151-L198) restores the perceived target and first-step constraint for the specific `verb_pair_chain` strategy. It intentionally does not apply to explicit sequences with independently named targets. Extending it by assigning every step one parent target would corrupt such sequences rather than repair this finding.
- The [slash-payload audit's as-built repair](https://github.com/theRebelliousNerd/codenerd/blob/388205138ba6d47c1cff0c5e97c896dfa642a391/Docs/audits/slash-command-payload-integrity.md#L179-L190) preserves whitespace in direct human `/write` and `/append` payloads. The present path is natural-language multi-step decomposition, not those command handlers or native write-tool transport.
- The [multi-step fallback-condition audit](https://github.com/theRebelliousNerd/codenerd/blob/388205138ba6d47c1cff0c5e97c896dfa642a391/Docs/audits/multistep-fallback-condition.md) addresses conditional-edge semantics and outcome-dependent scheduling. The witness here is an ordinary sequential plan; its dependencies can be correct while its targets are not.
- The [11:17 multi-step delivery-failure continuation](https://github.com/theRebelliousNerd/codenerd/blob/c3a596c8e9444a523b3368a25843cb3bad2de112/Docs/audits/multistep-delivery-failure-evidence.md) addresses failed worker evidence omitted after execution. This finding precedes the first worker and does not require a failed return or enabled delivery judging.

## The exact production transformation

### Matching normalizes more than control words

[`MatchMultiStepPattern`](https://github.com/theRebelliousNerd/codenerd/blob/388205138ba6d47c1cff0c5e97c896dfa642a391/cmd/nerd/chat/multistep_corpus.go#L661-L707) creates `lower := strings.ToLower(input)`, runs every clause regex against `lower`, and returns submatches from that string. Lowercase text is suitable for some recognition operations, but the returned captures also contain filenames, symbols, and other task content.

The [`explicit_first_then` pattern](https://github.com/theRebelliousNerd/codenerd/blob/388205138ba6d47c1cff0c5e97c896dfa642a391/cmd/nerd/chat/multistep_corpus.go#L93-L111) already uses case-insensitive regexes and has priority 100. For the source-derived witness:

> First fix Client.go, then test Server.go

the selected explicit pattern's two nonempty captures are `fix client.go` and `test server.go`. The optional final capture is empty. This is a deduction from the string transformation and regex, not a run of the code.

[`decomposeExplicitSequence`](https://github.com/theRebelliousNerd/codenerd/blob/388205138ba6d47c1cff0c5e97c896dfa642a391/cmd/nerd/chat/multistep_decomposer.go#L63-L91) parses each nonempty capture, assigns its target to `TaskStep.Target`, and constructs the task with `formatShardTask`. [`decomposeTask`](https://github.com/theRebelliousNerd/codenerd/blob/388205138ba6d47c1cff0c5e97c896dfa642a391/cmd/nerd/chat/delegation_multistep.go#L187-L198) returns this multi-step result; the verb-pair anchor declines to change an `explicit_sequence`.

The resulting source-derived pair is precise:

| Step | Requested target | Dispatched structured target | Formatted task |
|---|---|---|---|
| 1 | `Client.go` | `client.go` | `fix file:client.go` |
| 2 | `Server.go` | `server.go` | `write_tests for server.go` |

The second task is **not** `run_tests`: [`formatShardTask`](https://github.com/theRebelliousNerd/codenerd/blob/388205138ba6d47c1cff0c5e97c896dfa642a391/cmd/nerd/chat/delegation.go#L551-L563) formats this `/test` target as a test-writing request. The audit does not depend on changing that existing mapping.

### The route can admit this plan

The example contains the real `fix … test` compound shape recognized by [`multiStepSignals`](https://github.com/theRebelliousNerd/codenerd/blob/388205138ba6d47c1cff0c5e97c896dfa642a391/cmd/nerd/chat/delegation_multistep.go#L124-L148). The [`is_multi_step` rule](https://github.com/theRebelliousNerd/codenerd/blob/388205138ba6d47c1cff0c5e97c896dfa642a391/internal/core/defaults/policy/delegation.mg#L538-L570) accepts that strong signal without requiring the weak keyword/verb-count combination.

This does not bypass normal routing. The [multi-step lane](https://github.com/theRebelliousNerd/codenerd/blob/388205138ba6d47c1cff0c5e97c896dfa642a391/internal/core/defaults/policy/routing_arbitration.mg#L224-L266) still requires a mutation intent, at least two routed steps, no direct-answer or earlier winning lane, and work admission. The witness assumes normal perception returns the relevant mutation interpretation and those conditions hold; it does not claim a particular live model always selects that interpretation. The plan's `/fix` and `/test` steps have ordinary routed handlers, and its second step depends on the first.

[`decideRoute`](https://github.com/theRebelliousNerd/codenerd/blob/388205138ba6d47c1cff0c5e97c896dfa642a391/cmd/nerd/chat/delegation_routing.go#L166-L190) obtains and publishes the decomposed plan. The [`RouteMultiStep` branch](https://github.com/theRebelliousNerd/codenerd/blob/388205138ba6d47c1cff0c5e97c896dfa642a391/cmd/nerd/chat/process.go#L519-L528) then calls `executeMultiStepTask` with those steps and the original input. This is ordinary chat routing, not only a direct helper test or dream-plan execution.

### Execution trusts the transformed target

[`executeMultiStepTask`](https://github.com/theRebelliousNerd/codenerd/blob/388205138ba6d47c1cff0c5e97c896dfa642a391/cmd/nerd/chat/process_dream.go#L705-L747) constructs the worker request from the step task, assigns `req.Target = step.Target`, and calls `spawnTaskWithContext`. The [observed-execution adapter](https://github.com/theRebelliousNerd/codenerd/blob/388205138ba6d47c1cff0c5e97c896dfa642a391/cmd/nerd/chat/delegation.go#L96-L105) passes that request to the task executor rather than rebuilding its target from `rawInput`.

On the ordinary inline path, [`presetIntentForTask`](https://github.com/theRebelliousNerd/codenerd/blob/388205138ba6d47c1cff0c5e97c896dfa642a391/internal/session/task_executor.go#L234-L252) keeps the request target; the [task adapter](https://github.com/theRebelliousNerd/codenerd/blob/388205138ba6d47c1cff0c5e97c896dfa642a391/internal/session/task_executor.go#L427-L449) sends that preset into `ProcessWithIntent`. The executor [uses the preset instead of re-perceiving](https://github.com/theRebelliousNerd/codenerd/blob/388205138ba6d47c1cff0c5e97c896dfa642a391/internal/session/executor.go#L1403-L1413). Its [compilation context](https://github.com/theRebelliousNerd/codenerd/blob/388205138ba6d47c1cff0c5e97c896dfa642a391/internal/session/executor.go#L1647-L1654) also carries that target. Thus this is not merely a lowercased display label: an execution and context-selection input has changed.

Preset reuse is itself a useful control and cost optimization. The repair should make its incoming request faithful, not add another model call to guess the original capitalization.

## Consequences and actual containment

The deterministic equivalence is useful: the mixed-case request and an otherwise identical all-lowercase request produce the same explicit-sequence captures and step targets. On a case-sensitive workspace containing both `Client.go` and `client.go`, those user requests can designate different source files. The decomposition cannot preserve that distinction once it has discarded case. A workspace containing only `Client.go` instead makes a missing or unresolved lowercase target a possible outcome.

The final outcome remains conditional. The worker may inspect the workspace, use a retained conversation or brief, infer the intended file, ask a question, or refuse to proceed. An unavailable target does not prove that another file will be created, and a dispatched task does not prove a tool executed. The source establishes altered requests, not an incident rate, a guaranteed mutation, or a permission escape.

Important controls remain:

- The original input is still passed to `executeMultiStepTask` and later [delivery setup and judgment](https://github.com/theRebelliousNerd/codenerd/blob/388205138ba6d47c1cff0c5e97c896dfa642a391/cmd/nerd/chat/process_dream.go#L774-L789). Delivery may detect unmet work and request remediation. It is not necessarily blind to the originally requested capitalization, and this audit does not assert total history loss.
- Request restrictions are put on the [parent operation context](https://github.com/theRebelliousNerd/codenerd/blob/388205138ba6d47c1cff0c5e97c896dfa642a391/cmd/nerd/chat/process.go#L280-L281). Case transformation is not evidence that these restrictions, constitutional authorization, or normal tool gates are bypassed.
- Work-mode admission, dependency handling, failure reporting, and the distinction between a returned step and a verified step still apply. A hard failure in step 1 can prevent dependent step 2 from running; the first request's target transformation already exists before that outcome.
- Quote stripping and whole-word matching reduce false multi-step signals. They do not preserve a quoted literal's case once an actual multi-step request is matched against the lowercased copy. Likewise, direct single-step delegation and the anchored verb-pair path are useful negative controls, not evidence that all decomposition strategies share the defect.

The substantive risk is work on the wrong subject or avoidable recovery work at an otherwise-authorized boundary. It can manifest as poorer initial context, unresolved targets, edits/tests for an unintended case-variant file, or a later corrective round. Those effects require normal-entry evaluation; they must not be presented as measured improvements merely because the source-level rewrite is clear.

## Proposed preservation contract

**Normalize recognition tokens, not source-bearing captures.** Keep the original input immutable. Match clause regexes case-insensitively on that input and return original-text captures or source-span references. The explicit patterns already declare case-insensitivity; verify the rest of the corpus rather than assuming every pattern has the same flag.

Extend the same contract to fallback splitting. [`splitOnSequentialConnectors` and `splitOnParallelConnectors`](https://github.com/theRebelliousNerd/codenerd/blob/388205138ba6d47c1cff0c5e97c896dfa642a391/cmd/nerd/chat/multistep_decomposer.go#L798-L875) also split a lowercased string and return those pieces. A matcher-only change would leave those producers lossy. Recognize delimiters without replacing the retained clause text. If source indices are used, derive them against the original string; do not assume offsets in transformed Unicode text are interchangeable with original byte offsets.

Keep canonical verb normalization, current pattern priority, whole-word boundaries, dependency inference, and whitespace grammar distinct from literal preservation. The proposed invariant is not that all surrounding command syntax survives byte-for-byte: it is that a captured target or literal is not case-folded as a side effect of recognizing the command. Paths should undergo only the explicit existing workspace/path contract, not speculative case repair.

For multiple targets, preserve each step's source identity independently. A blanket assignment from `intent.Target` cannot represent two separately named files and would undermine the present verb-pair repair's limited purpose. If a clause is ambiguous, retain an unresolved state or ask through the existing flow rather than guessing that the lowercase spelling is canonical.

Retain original source spans, strategy identity, and step index in focused diagnostics when useful. Avoid duplicating full prompts in durable logs solely to diagnose this issue. Do not solve it by running another parser model or adding an unrestricted shell/file-search escape hatch. A deterministic preservation change should not need a new model round or a new permission engine.

## Evaluation and total cost

The inspected [verb-pair regression test](https://github.com/theRebelliousNerd/codenerd/blob/388205138ba6d47c1cff0c5e97c896dfa642a391/cmd/nerd/chat/delegation_multistep_test.go#L9-L37) checks that the anchor preserves the perceived `Mean in stats.go` target and constraint. The [verb-parser tests](https://github.com/theRebelliousNerd/codenerd/blob/388205138ba6d47c1cff0c5e97c896dfa642a391/cmd/nerd/chat/multistep_parse_verb_test.go#L5-L38) protect lexical recognition. Neither establishes case preservation across corpus capture and real dispatch. The existing [multi-step routing journey](https://github.com/theRebelliousNerd/codenerd/blob/388205138ba6d47c1cff0c5e97c896dfa642a391/cmd/nerd/chat/task_routing_arbitration_e2e_test.go#L570-L589) checks surrounding routing properties, not exact case-bearing worker targets. No test was run for this review.

Recommended evaluation, not performed:

1. **Deterministic source contract:** feed the exact witness through corpus matching and decomposition; compare original target substrings with both `TaskStep.Target` and formatted task text. Include multiple distinct targets, upper/lower variants, mixed-case identifiers, quoted content, and Unicode. Verify unchanged verbs, priority, dependency edges, and step count.
2. **Fallback and negative controls:** exercise connector splitting, numbered/parallel strategies where applicable, direct single-step delegation, anchored verb-pair requests, all-lowercase requests, and quoted pseudo-verbs that should not become plans. Changing syntax case must not change literal case or unexpectedly change the winning strategy.
3. **Normal production handoff:** enter the request through normal chat/perception and policy admission, keep CodeDOM/world context enabled, and record the actual worker requests. A helper-only test or synthetic task request cannot establish this boundary. Use deterministic provider responses to isolate transport, then separately measure live interpretation and recovery without claiming the former proves semantic quality.
4. **Workspace subjects:** use a case-sensitive fixture with only the requested mixed-case files, then one containing distinct uppercase/lowercase variants with independently recognizable contents. Capture reads, writes, verification subjects, and final request delivery. Also test a case-insensitive environment; do not count successful path aliasing there as proof that symbols or literal contents are preserved.
5. **Controls and recovery:** verify work-mode holds, typed restrictions, permission denials, dependency failures, honest verification labels, and downstream delivery behavior. Record when the original input repairs a bad target so results distinguish prevention from eventual recovery.

Compare identical intended work and context before and after the change. Report exact target preservation first, then wrong-subject actions, unresolved targets, extra exploration or corrective rounds, verified completion, and user intervention. Do not classify a model's successful recovery as proof there was no transport defect; equally, do not classify an altered request as a completed wrong-file mutation.

The likely implementation is local parser/transport work, but its regression surface includes strategy selection, Unicode boundaries, connector handling, and caller expectations. Measure CPU, allocations, retained source-span bytes, and parsing latency instead of presuming that avoiding lowercasing is a net optimization. There is no inherent extra inference cost. Any added metadata, larger task text, changed context selection, model/tool retries, and human clarification must still be counted end to end. The useful objective is faithful execution with equal or better delivered quality, not fewer calls obtained by failing to find the intended target.
