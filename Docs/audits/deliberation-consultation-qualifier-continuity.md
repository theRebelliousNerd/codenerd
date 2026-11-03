---
doc-class: deep-dive
subsystem: deliberation-consultation
implementation-status: proposed-refinement
evidence-status: static-source-inspection
last-verified: 2026-11-03
verified-against: 3330b2cc242f7878326649867aee2a93d7348ce5
supersedes: []
---

# Deliberation consultation advice loses its caveats and references

## Finding and evidence boundary

Preserve the qualifications attached to specialist advice when delivering it to deliberation. Both production adapters call the shared consultation manager, receive its structured response, and return only `answer.Advice`. A specialist following the requested response format puts limitations in `CAVEATS` and sources in `REFERENCES`; the manager parses and retains those fields correctly, but the adapters discard them. The challenger and saved round consequently receive the recommendation without those qualifications. [Mission adapter][mission-adapter] [Chat adapter][chat-adapter] [Response contract][contract] [Parser][parser] [Round handoff][round]

The deterministic finding is a lossy handoff: two valid specialist replies with the same advice and materially different caveats become identical consultation observations. A mistaken design or unnecessary work is a possible consequence, not an observed result. This review does not demonstrate an unsafe mutation, a failed gate being accepted, an incorrect mission receipt, or a violation of the independent permission controls.

This report is pinned to public main [3330b2cc242f7878326649867aee2a93d7348ce5][main], resolved on November 3, 2026. Before freezing the draft, main advanced to `c2c9172f5c0faf48b8763de01cfc632e8a7f318a`; the two production adapters, consultation parser and response types, and deliberation round, service and types were re-fetched there and had identical blob identities. Only public source, existing audits, specifications and authored tests were inspected. No repository program, test, build, model call, gate, benchmark or failure injection ran. No implementation is included. The examples and evaluation plan below are unexecuted source-derived witnesses.

## Existing contract and production route

Root guidance requires relevant evidence to survive bounded working-context transitions and puts executive decisions in Mangle. The session specification distinguishes readiness of a proposed reading from verification of its implementation. The shard specification describes consultation advice as evidence for the challenger, with inherited read-only restrictions. Preserve those boundaries: an expert's caveat can inform a challenge without becoming an independent permission or executable instruction. [Guidance][guidance] [Session contract][session-contract] [Shard contract][shard-contract]

The proposer can return a bounded `consultations` list alongside its candidates. Each entry names a specialist and a concrete question. `collectRound` invokes the service's consultation callback, creates a `ProbeResult`, and passes those observations to the requirements interrogator in `frame.Evidence`. This occurs before the challenger produces its assessment of the current candidates. [Proposer contract][proposer] [Round handoff][round]

There are two affected production constructions:

1. `newRecurseMission` creates a consultation manager using `session.TaskConsultationSpawner`, merges the mission read-only restriction into the context, and sends the specialist, question and original request to that manager. On success its callback returns `answer.Advice`.
2. The chat model's `deliberator` uses the existing consultation manager, likewise inherits and merges the read-only restriction, sends the specialist, question and request, and returns `answer.Advice`.

The defect is after a successful consultation. It does not depend on a missing manager, failed specialist, stale completed-answer cache or skipped permission check. The manager deliberately coalesces only concurrent identical work and does not retain a completed-answer cache; that existing protection should remain. [Mission adapter][mission-adapter] [Chat adapter][chat-adapter] [Manager][manager]

## Where the qualifications disappear

Neither adapter supplies a custom `ResponseContract`, so `buildPrompt` selects the shared default. That format explicitly requests separate `ADVICE`, `CONFIDENCE`, `REFERENCES` and `CAVEATS` sections. Separating an important limitation from the main recommendation therefore follows the requested protocol; it is not malformed specialist output. [Default contract][contract] [Contract selection][prompt-selection]

`parseArtifact` flushes the advice section when it reaches `CONFIDENCE`, `REFERENCES` or `CAVEATS`. `applySection` writes references and caveats to their own fields. `Artifact.receipt` copies those slices into the consumer's `Response`, together with raw text and parse status. At this point the information has survived parsing and delivery. The authored consultation tests even check defensive copying of references and caveats. [Parsing][parser] [Receipt][receipt] [Authored manager fixture][manager-test]

The subsequent narrowing is exact:

```go
return answer.Advice, nil
```

Both callbacks have a string-returning interface. `collectRound` receives that string and stores:

```go
ProbeResult{Key: consultation.Specialist, Tool: "consult", Output: advice}
```

There is no separate caveat, reference, raw-response or parse-status field in that record. `frame.Evidence` is assigned from these projected observations before the challenger is called. After the round is collected and judged, `Service.Run` appends the round and saves the thread. Later rounds replay the preceding round, which contains the same narrowed result. Saving and replaying it cannot restore content that never entered it. [Adapter returns][mission-adapter] [Chat return][chat-adapter] [Observation type][types] [Current challenger][round] [Persistence call][service]

This is a statement about the challenger payload and deliberation round record, not a claim that all provider traces or other diagnostics lose the original reply. The consultation response itself retained `Raw`. A separately enabled trace might preserve it, but these adapters do not give the challenger a reference to that response or include the omitted fields in the round. This review does not depend on proving universal loss from every log.

## Small source-derived witness

Use a normal first round with valid candidates, one successful consultation, a stable workspace, short strings, and ample configured context. No truncation, race, repeated specialist, cache reuse, invalid output or second consultation is needed.

For the same specialist and question, consider these two possible well-formed replies:

```text
ADVICE:
Use the proposed cached lookup.
CONFIDENCE: 80
REFERENCES:
cache.go:42
CAVEATS:
This recommendation requires tenant identity in the cache key. Without it,
results can cross tenant boundaries. The proposed key does not include it.
```

```text
ADVICE:
Use the proposed cached lookup.
CONFIDENCE: 80
REFERENCES:
cache.go:42
CAVEATS:
The proposed key includes tenant identity and the reviewed callers preserve it.
```

These are illustrative inputs for a proposed fixture, not recorded specialist responses. Their different caveats are the deliberate discriminant. The manager produces distinct caveat fields, but either adapter returns exactly `Use the proposed cached lookup.`. With the same specialist, the round's consultation observation is identical in both cases:

```json
{"key":"cache-expert","tool":"consult","output":"Use the proposed cached lookup."}
```

The challenger therefore cannot distinguish the blocking qualification from the favorable qualification using that observation. It may rediscover the issue through its own reads or other supplied evidence. The missing qualifier also does not force the challenger to approve: a cautious or independently informed challenger can still identify the defect. The source-supported result is that the specialist's already-acquired distinction is absent at the decision boundary.

A second control changes only a reference that identifies the basis for the advice, while leaving advice, confidence and caveats identical. The response retains the different source, but the projected observation again does not change. This establishes the reference loss independently of the caveat example. A source reference is still a specialist claim requiring appropriate grounding; preserving it does not certify its accuracy.

### Counterexamples and limits

- A specialist that repeats the limitation inside `ADVICE` preserves it through this adapter. The finding concerns material content placed only in its designated section.
- If parsing finds no nonempty advice, `parseArtifact` falls back to the entire reply. Some unstructured or empty-advice replies therefore retain caveats incidentally. The primary witness uses nonempty, correctly structured advice and does not take that fallback.
- A failed consultation becomes an observation with an error. The issue is a successful response whose retained fields are narrowed, not failure being silently reported as success.
- The original candidate assumptions, request and previous round can independently contain the same limitation. The witness places the discriminating information only in the new specialist caveat.
- Keeping a caveat would not itself make the design correct. Challenger assessment, Mangle readiness, mission freshness, worker permissions, acceptance and the recurse ratchet retain their existing authority. [Fallback][parser] [Errors][round] [Readiness policy][policy] [Mission execution contract][mission-contract]

## Impact and nonduplication

The immediate impact is semantic weakening of advice. A conditional recommendation can look unconditional, and the source needed to check it can disappear. The durable round then records only the weakened version. Possible downstream costs include repeating specialist work, extra reads, missed critical objections or selecting a design that later verification rejects. Their frequency and severity are unmeasured.

This bounded duplicate screen read the relevant existing reports and searched current repository material for `deliberation` with `Caveats`, for `answer.Advice`, and for consultation caveats. It found no exact predecessor describing these two adapter returns. This is not an exhaustive claim about all private discussion or historical branches.

- **Deliberation read invocation identity** concerns missing arguments on role-internal tool reads. This report needs no role tool call and preserves the full consultation question at dispatch. It identifies a different loss after the shared manager has already delivered a structured answer. The existing report explicitly keeps consultations outside its primary witnesses. [Read identity audit][read-audit]
- **Consultation coordination and safe reuse** introduced explicit artifacts, separate references/caveats and defensively copied responses. Those controls work at the inspected boundary. The defect is a newer consumer narrowing that response, not a reason to reopen completed-cache, coalescing or partial-batch claims. [Coordination audit][coordination]
- **Consultation cache snapshot** examines a separate session cache and evidence freshness. No completed-cache reuse is required here. [Cache audit][cache-audit]
- **Consultation knowledge semantic admission** concerns durable project knowledge becoming eligible for vector retrieval. This report concerns the immediate deliberation challenger, before any such retrieval. [Knowledge audit][knowledge-audit]
- **Portable recurse deliberation and crucible** already proposes an evidence-producing design funnel. This report gives one concrete current transport defect within that existing funnel rather than another orchestration framework. [Portable proposal][portable]

## Proposed bounded correction

Update the owning deliberation and adapter contracts before implementation. Carry a host-produced consultation result with at least the recommendation, its supplied caveats and references, parse disposition, and sufficient request/result identity to keep them together. Treat confidence as an explicitly model-reported claim if it is carried; it must not become a calibrated assurance score or new admission threshold.

Use the existing service, consultation manager, bounded role input and Mangle decisions. A typed consultation result is preferable to making every consumer rediscover the section protocol. A safe rendering of the complete response is another option, provided fields remain distinguishable and omission is explicit. The correction must cover both chat and mission adapters; fixing only one leaves the other production entry path unchanged.

Do not simply concatenate unlimited raw responses into the next prompt. Qualifications consume context too. Preserve a bounded relation between the recommendation and its limitations, or disclose that the complete recommendation is unavailable and provide a genuinely resolvable retained reference. A digest without access to the omitted content does not help the challenger. A budget decision must not leave positive advice present while silently cutting the caveat that determines where it applies. Keep secret-bearing or otherwise inadmissible content out of model and durable records under the existing data boundary; disclose any material redaction.

The minimal repair does not require restoring a growing transcript, introducing another cache, running more specialists, changing the host's readiness policy, broadening read permissions, or treating advisory text as executable authority. Exact question association and role-read argument receipts can compose with this change, but neither is needed to demonstrate the qualifier loss.

## Proposed verification and full cost accounting

All checks below remain unrun. Start with deterministic parsing and request capture, then exercise both actual adapter constructions and the normal deliberation service. Do not count a struct-only fixture as proof that the production challenger receives the fields.

1. Feed the two short structured replies above through the shared parser and manager. Confirm the source responses differ only in their caveats and that both parse successfully.
2. Capture the actual first challenger frame through each production adapter. Require the distinct caveats and their relation to the advice to remain visible. The old adapter should produce the collision; the repair should remove it.
3. Vary only a reference and require that distinction to reach the frame as a claimed source, without manufacturing host verification.
4. Persist and reload a completed round. Require the same qualified recommendation and explicit availability status, including later convergent replay. Do not rely solely on diagnostic logging.
5. Cover no caveats, multiple caveats, unstructured fallback, empty advice, malformed or absent sections, failed/canceled consultation and multiple results. Preserve existing error identities and read-only restrictions.
6. Exercise Unicode, JSON escaping, long sources and a near-bound frame. Check the actual serialized request, not just raw field lengths. Refusal, recoverable omission or closure must be explicit; silently discarding the limitation is not an acceptable fit strategy.
7. Keep independent controls for ordinary role reads, critical-data probes, values forks, mission source drift, permission refusal and test/ratchet acceptance. Carrying qualified advice must not promote it into observed test evidence.
8. With authorized live evaluation later, compare current and corrected routes on held-out design questions whose decisive condition is deliberately placed in a caveat. Include negative controls where no caveat changes the design and cases where the challenger can discover the issue independently.

Retaining fields adds serialized bytes, input tokens, copying, durable storage and possibly earlier context closure. Raw-response retention can duplicate content already represented as fields. A referenced representation adds storage, lookup latency, retention and unavailable-reference behavior. These are actual costs to measure, not free metadata.

Report qualifier delivery, correct condition handling, unsupported readiness, unresolved outcomes, repeated consultations and reads, valid completed designs, all model calls, input/output tokens, request bytes, persisted bytes, host allocations, latency and total end-to-end cost. Include failures and deferred designs. A reduction in repeated work is a hypothesis, and preserving qualifiers may increase token use while improving decision quality. No reliability, latency or token-efficiency gain is established by this static report.

The exit criterion is that the challenger and the saved round receive a specialist recommendation together with its material supplied qualifications, or explicitly know that the qualified recommendation is unavailable. Retaining only the positive `ADVICE` section does not meet that criterion.

[main]: https://github.com/theRebelliousNerd/codenerd/commit/3330b2cc242f7878326649867aee2a93d7348ce5
[guidance]: https://github.com/theRebelliousNerd/codenerd/blob/3330b2cc242f7878326649867aee2a93d7348ce5/agents.md#L19-L27
[session-contract]: https://github.com/theRebelliousNerd/codenerd/blob/3330b2cc242f7878326649867aee2a93d7348ce5/Docs/architecture/session/IMPLEMENTED_SPEC.md#L816-L873
[shard-contract]: https://github.com/theRebelliousNerd/codenerd/blob/3330b2cc242f7878326649867aee2a93d7348ce5/Docs/architecture/shards/IMPLEMENTED_SPEC.md#L486-L511
[proposer]: https://github.com/theRebelliousNerd/codenerd/blob/3330b2cc242f7878326649867aee2a93d7348ce5/internal/prompt/atoms/system/deliberation_proposer.yaml#L43-L52
[mission-adapter]: https://github.com/theRebelliousNerd/codenerd/blob/3330b2cc242f7878326649867aee2a93d7348ce5/cmd/nerd/cmd_campaign_mission.go#L40-L68
[chat-adapter]: https://github.com/theRebelliousNerd/codenerd/blob/3330b2cc242f7878326649867aee2a93d7348ce5/cmd/nerd/chat/deliberation.go#L18-L50
[contract]: https://github.com/theRebelliousNerd/codenerd/blob/3330b2cc242f7878326649867aee2a93d7348ce5/internal/consultation/prompt.go#L21-L33
[prompt-selection]: https://github.com/theRebelliousNerd/codenerd/blob/3330b2cc242f7878326649867aee2a93d7348ce5/internal/consultation/prompt.go#L52-L66
[parser]: https://github.com/theRebelliousNerd/codenerd/blob/3330b2cc242f7878326649867aee2a93d7348ce5/internal/consultation/prompt.go#L69-L156
[receipt]: https://github.com/theRebelliousNerd/codenerd/blob/3330b2cc242f7878326649867aee2a93d7348ce5/internal/consultation/types.go#L72-L123
[manager]: https://github.com/theRebelliousNerd/codenerd/blob/3330b2cc242f7878326649867aee2a93d7348ce5/internal/consultation/manager.go#L36-L111
[manager-test]: https://github.com/theRebelliousNerd/codenerd/blob/3330b2cc242f7878326649867aee2a93d7348ce5/internal/consultation/manager_test.go#L102-L148
[types]: https://github.com/theRebelliousNerd/codenerd/blob/3330b2cc242f7878326649867aee2a93d7348ce5/internal/deliberation/types.go#L39-L69
[round]: https://github.com/theRebelliousNerd/codenerd/blob/3330b2cc242f7878326649867aee2a93d7348ce5/internal/deliberation/round.go#L10-L72
[service]: https://github.com/theRebelliousNerd/codenerd/blob/3330b2cc242f7878326649867aee2a93d7348ce5/internal/deliberation/service.go#L130-L165
[policy]: https://github.com/theRebelliousNerd/codenerd/blob/3330b2cc242f7878326649867aee2a93d7348ce5/internal/core/defaults/policy/deliberation.mg#L18-L58
[mission-contract]: https://github.com/theRebelliousNerd/codenerd/blob/3330b2cc242f7878326649867aee2a93d7348ce5/Docs/architecture/campaign/MISSION-IMPROVEMENT.md
[read-audit]: https://github.com/theRebelliousNerd/codenerd/blob/3330b2cc242f7878326649867aee2a93d7348ce5/Docs/audits/deliberation-read-invocation-identity.md
[coordination]: https://github.com/theRebelliousNerd/codenerd/blob/3330b2cc242f7878326649867aee2a93d7348ce5/Docs/audits/consultation-coordination-and-safe-reuse.md
[cache-audit]: https://github.com/theRebelliousNerd/codenerd/blob/3330b2cc242f7878326649867aee2a93d7348ce5/Docs/audits/consultation-cache-snapshot.md
[knowledge-audit]: https://github.com/theRebelliousNerd/codenerd/blob/3330b2cc242f7878326649867aee2a93d7348ce5/Docs/audits/consultation-knowledge-semantic-admission.md
[portable]: https://github.com/theRebelliousNerd/codenerd/blob/3330b2cc242f7878326649867aee2a93d7348ce5/Docs/audits/portable-recurse-deliberation-crucible.md
