---
doc-class: deep-dive
subsystem: expert-mandate-verification
implementation-status: proposed-refinement
evidence-status: source-only
last-verified: 2026-10-30
verified-against: 8d373fb6ca05ef03fec59b923c5f1925066d623a
verification-method: static-source-and-test-definition-inspection
supersedes: []
---

# Expert mandates need package-qualified test identity

## Decision and evidence boundary

Preserve the implemented distinction between expert advice and explicitly authorized mandates, but bind a test mandate to the test identity its scope actually requires. The current path admits a bare Go test name and discharges it with a passing test of that name in any package represented by the turn's current test result. Package identity is already present in the execution evidence; the mandate consumer discards it.

The concrete conditional witness is a mandate about package A's `TestDecoderKeepsOldColumn`, that test remaining skipped, and an unrelated package B's identically named test passing in the current run. The current rule can discharge the name-only mandate from B's pass. This is a mismatch with the proposed **scoped** mandate contract, not an unconditional bypass under every possible reading of the current bare-name syntax: if the intended contract is explicitly “some package's test with this name must pass,” the rule implements that weaker existential contract. It cannot currently express or enforce the package-specific obligation described here.

The source baseline is public main [`8d373fb6ca05ef03fec59b923c5f1925066d623a`][baseline], rechecked at approximately 07:13 UTC on October 30, 2026. The relevant mandate, test-evidence, and verification rules were re-fetched after main advanced from `d490b08dc891d00087525f4d521ac7caa99a5c83`; they were unchanged. This review inspected source and existing test definitions only. No repository program, tests, model inference, training, benchmark, or executable reproduction was run. No implementation or measured quality improvement is claimed.

## Primary tool reference

[Go’s testing documentation](https://pkg.go.dev/testing#T.SkipNow) distinguishes a skipped test from a passing test, and [test2json](https://pkg.go.dev/cmd/test2json) describes the structured event stream. The witness below relies on retaining both outcome and package identity; it is not an executed test result.

## Contract and relationship to prior work

The [repository guidance][guidance] assigns executive decisions to Mangle and requires current, grounded evidence. The [session guidance][session-guidance] preserves explicit capability boundaries, revision-bound completion, and distinct artifact/check/behavior stages. The [plan-bound expert-decks proposal][decks] distinguishes steering from authorized obligations and requires mandates to retain scope, applicability, authority, and an appropriate evidence contract. That proposal supplies the intended scoped contract; it is not evidence that every proposed field is already implemented.

The current implementation has important parts of that design: typed consultation outcomes, file-sensitive adviser delivery, explicit registry authority, host-protected mandate facts, and actual test-result witnesses. This report concerns their composition at test identity, not a proposal to replace them with another verifier or agent framework.

The earlier [test-evidence-capability audit][test-audit] established package-qualified identity for baseline failure attribution and records that fix as built. This is **a new consumer of that established principle**, not a new discovery that package qualification matters. Baseline attribution now distinguishes a failure in A from a namesake failure in B; the newly added mandate-discharge rule again compares only the name. The [workspace receipt audit][receipt-audit] concerns working-directory identity for process reuse, not this Go test-case join.

The current audit inventory, [implementation queue][queue], expert-card scope/repair/critique audits, and the additions from research branches `research-main-20261030-0544`, `research-main-20261030-0614`, and `research-main-20261030-0642` were inspected for overlap. Those branch additions are merged at the new baseline; none of their inspected documents owns this mandate-specific witness. This is bounded overlap checking, not exhaustive novelty research. Earlier expert-card scope and final-repair delivery gaps are implemented in current source and are not reopened here.

## Producer-to-consumer trace

### 1. The consultation knows files but requests a name-only requirement

The consultation task names the files in the expert's domain and asks for lines shaped `REQUIRE <TestName>: <why>`. It says the line binds only when the user granted mandate authority. Answered consultations are parsed for mandate lines and recorded as cards. [Consultation prompt][consultation-prompt], [answer parsing][consultation]

An `expertCard` retains its `Files`, but `cardMandate` stores only `Test` and `Why`. `parseMandates` splits the name from the explanation and validates a `Test`-prefixed identifier. A package-qualified spelling containing a slash or dot is rejected; it is not a supported way to preserve scope through the current syntax. [Card and parser contract][parser]

### 2. Admission projects away even the originating card identity

`recordCards` publishes the card's file rows separately, but emits each mandate as `turn_card_mandate(Turn, Card, Idx, Test)`. The mandate tuple has no package. The rule admits `turn_mandate_admitted(Turn, Test)` from an answered card and an agent's `/mandate` authority; neither the card's file scope nor its explanation participates in that projection. [Fact publication][record], [admission][mandates]

The registry's explicit `authority` field is measured into `shard_agent_authority`. This authority check must be retained; an expert does not grant itself enforcement power by writing `REQUIRE`. [Registry field][authority-field], [host measurement][authority-measurement]

### 3. Execution evidence retains the package

`testfacts.Result.Facts` emits one `test_case(Package, Test, Status, ElapsedMs)` for each test with a verdict, using distinct `/pass`, `/fail`, and `/skip` states. The session adapter prepends the turn and preserves those arguments. [Test facts][test-facts], [turn adapter][turn-facts]

`syncTestGateFacts` replaces previous rows for this turn. When `FinalTestCheck` exists, it uses that result; otherwise it uses `TestCheck`. It then publishes the parsed run through `assertTurnTestFacts`. This is current turn evidence, not arbitrary historical global `test_state`. [Gate synchronization][sync]

The declared relation is `turn_test_case(Turn, Pkg, Test, Status, ElapsedMs)`. Nearby baseline attribution explicitly documents why the package/name pair matters. [Execution schema and attribution][test-schema]

### 4. Mandate discharge ignores the retained package

The final join is:

```mangle
turn_mandate_discharged(Turn, Test) :-
    turn_mandate_admitted(Turn, Test),
    turn_test_case(Turn, _, Test, /pass, _).
```

An admitted mandate is unmet only when this discharge is absent. Thus any current same-name pass in the represented packages is sufficient, including when another package's same-name row is `/skip`. Two cards that intended different package-specific tests with the same name also collapse into one admitted `(Turn, Test)` pair. [Discharge and unmet rules][mandates]

This is not a missing cross-shard join. Registration places authority and mandate inputs with the test/verdict consumer and supplies the answered-card input as a bounded consumer copy. [Shard wiring][shard-wiring]

## Conditional witness, including final `/fix` verification

Use an otherwise normally admitted Go `/fix` task in a single module with packages `example/a` and `example/b`:

1. A registered expert is actually selected and consulted for A, answers successfully, and has an explicit `/mandate` grant. Its answer makes clear that **A's** decoder test must pass, using the supported `REQUIRE TestDecoderKeepsOldColumn: ...` line. The file scope and rationale establish the intended package-specific obligation; the parser does not preserve that qualification in the admitted requirement.
2. A already contains `TestDecoderKeepsOldColumn`, but it is skipped under this run's normal conditions. It remains unchanged: the witness does not delete a test or introduce a new skip. Other tests in A pass and can exercise the changed lines.
3. B has an unrelated test with the same name that passes. No failing-baseline exemption is required; the relevant suite can have a successful exit with A's skip and B's pass.
4. The applicable narrow gates, importer checks, and other required checks permit final verification to run. `closeChangeEvidence` derives the final scope for `/fix`, calls `finalFixTests`, and stores its result separately in `FinalTestCheck`. [Closure handoff][closure]
5. `finalFixTests` first revalidates narrow obligations, then runs `verifyTests(..., []string{"./..."}, "-count=1")` when those checks are eligible. It snapshots inputs before and after and retains nonpassing outcomes rather than calling them passes. Grant a completed, current run here: its structured result includes A's `/skip` and B's `/pass`. [Final suite][final-suite]
6. Gate synchronization publishes that final result. The mandate rule joins B's pass and derives `turn_mandate_discharged(Turn, "TestDecoderKeepsOldColumn")`, despite no pass for the intended A test.

The same identity problem exists in a narrow run when both packages are in the written-package scope. The final full-suite path matters because the `/fix` witness need not rely on only a narrow subset being executed; a broader, correctly current run does not repair a scope-erasing join. [Written-package selection][packages], [own test gate][own-gate]

An alternative witness leaves A's intended test absent from the outset, with no deletion. The skipped-existing-test case is the primary control because it preserves an explicit A test identity in the actual result and avoids conflating missing-test creation, removal protection, or baseline attribution with the join defect.

**Established by source:** the mandate representation and discharge relation cannot distinguish these package-specific obligations. **Not established:** an executed failure, a production frequency, or inevitable false whole-turn completion. Other review, behavior acceptance, coverage, build, or self-reported-incompleteness checks may independently block the task. The ordinary write-verification arm reads `!turn_has_unmet_mandate`; losing that one blocker is not proof every other condition holds. [Verification policy][verification]

## Protections and negative controls to retain

- **Authority:** no grant or no answered card means no admitted mandate. Failed/empty consultation outcomes do not manufacture one.
- **Host ownership:** authority, cards, mandates, and test rows are protected against model-authored fact updates. The witness uses legitimate producer evidence, not forged assertions. [Protected facts][protected]
- **Currentness:** gate replacement and `FinalTestCheck` preference prevent this witness from being justified with another turn's or an obsolete repair's pass. Final snapshots and full-suite outcome handling remain valuable safeguards.
- **Actual status:** A's skip alone does not discharge the mandate. The additional same-name pass in B is essential. Removing B's pass, changing its name, or supplying only a prior-turn B pass are negative controls.
- **Correct execution:** a current pass for the intended A test is a positive control. Unique names in the applicable scope do not exhibit this package collision.
- **Other gates:** a red build, nonconclusive final suite, unmet coverage, or failing behavior acceptance can still withhold completion. Do not weaken them to demonstrate the identity contract.
- **Semantic boundary:** if policy deliberately declares the requirement global/existential, record that explicitly. Do not label the current rule incorrect for that weaker contract while simultaneously presenting it to users as a scoped test obligation.

The existing [mandate policy test][policy-test] exercises authority, unmet state, and one same-turn pass in one package. The [session mandate test][session-test] exercises parsing and whether an authorized requirement is named to the turn. These are useful tests; the inspected definitions do not establish package-collision behavior. They were read, not run, and this is not a claim to have searched every test in the repository.

## Proposed contract and implementation choices

Keep the existing expert-card and test-evidence machinery. Extend the admitted obligation so it retains the identity required by its scope: producing card/requirement ID, authority/provenance, package, exact test identity, applicability, and the current execution evidence that can discharge it. Mangle should derive admission and discharge from host-validated inputs; an extra model judge is not needed merely to compare identities.

There are two compatible design directions:

1. **Qualified requirement syntax.** Let the expert propose a package/test pair, validate it against supported workspace/package metadata, and retain that pair through admission, rendering, and evidence matching. Preserve an explicit unresolved state when the test is proposed but does not yet exist or its package cannot be resolved. Expert-proposed strings are not authoritative filesystem or module identities.
2. **Compatibility resolution for bare names.** Preserve current syntax when the host can establish the intended package from an explicit scoped requirement. Ambiguous names must not mean “choose any passing match.” Expose the ambiguity or require a scoped supplement. If the host resolves a name to several required tests, make the quantifier explicit rather than silently choosing existential or universal semantics.

Do not simply concatenate a card filename with a module name, or assume the test belongs to the first file's directory. Nested modules, external test packages, build variants, and a legitimate mandate about a downstream consumer complicate that mapping. The expert's consultation scope is evidence for resolution, not an infallible definition of the obligation's package. Nor should a global uniqueness convention replace identity: repositories legitimately reuse test names across packages.

Preserve distinct obligations when two authorized cards use the same test name for different scopes. Conversely, avoid paying for duplicate execution when two mandates truly require the same current package-qualified witness; reuse should follow identity and applicability rather than a card count. Keep the existing current-result replacement and revision closure. This refinement does not authorize broader tool permissions, extra writes outside the task, or automatic test execution merely because an expert named a test.

## Full costs and fair evaluation

No evaluation below was run. Qualification adds parser/schema migration, package resolution, metadata cache identity/invalidation, more keys and join/index work, rendering, and explicit unresolved-case handling. It can also correctly require additional test execution or clarification that the weaker name-only rule avoided. That extra cost is not a regression in assurance. New fact cardinality is proportional to distinct qualified obligations rather than distinct bare names; many same-named tests can therefore increase active state. Retain bounded current state and normal turn cleanup rather than accumulating every historical receipt in the kernel.

No extra LLM call is inherently necessary for a host-resolvable package/test identity. If semantic ambiguity requires another consultation, count its queueing, input/output tokens, latency, and failed attempts. Include package discovery, filesystem/module metadata reads, JIT/prompt effects, test process CPU and wall time, repair/reverification, retained memory, and human resolution. Any reduction in retries or escaped defects is a hypothesis until measured; a shorter prompt or fewer test runs is not success by itself.

### Deterministic contract fixtures

Exercise actual consultation parsing, card publication, registered authority, current parsed test-result projection, and mandate rules in both RealKernel and correctly registered Cortex. Capture the derived discharge and unmet states, not just the raw requirement string.

| Fixture | Scoped-contract expectation |
|---|---|
| A.TestX skipped; B.TestX passes in the current own/final run | A mandate remains unmet. |
| A.TestX absent from the outset; B.TestX passes | A mandate remains unresolved/unmet; no invented A pass. |
| A.TestX passes; B.TestX skipped or absent | A mandate is discharged. |
| Authorized A and B mandates share the name; only B passes | Preserve A's separate debt. |
| Same name, wrong turn or superseded run | No discharge from historical evidence. |
| Both intended package-qualified tests pass | Both obligations discharge without unnecessary duplicate execution. |
| Unauthorized card, failed or empty consultation | No mandatory obligation is admitted. |
| Bare name with ambiguous package resolution | Explicit unresolved disposition, not an arbitrary match. |
| Unique resolvable bare name | Backward-compatible successful behavior. |
| Full-suite timeout, cancellation, or changing snapshot | Existing nonconclusive handling remains; do not manufacture completion. |
| Independent red/coverage/acceptance gate | It still blocks irrespective of mandate outcome. |

Include sequential repair replacement, concurrent turns, nested modules, unavailable package metadata, and tests whose package or variant cannot be inferred from the card's initial files. Exercise the real `/fix` closure handoff to `FinalTestCheck`; otherwise a fixture may only prove an intermediate relation and miss which run actually owns final evidence.

### Compare identity independently from additional verification

First replay identical immutable parsed observations and requirement inputs through current and qualified contracts, with manually specified scope ground truth. Hold the executed test set constant so extra runs cannot take credit for the identity correction. Report false discharge, false unresolved outcomes, ambiguous resolutions, fact counts, query latency, allocation, and cleanup cost.

Only after contract correctness is established should a whole-system comparison use normal production entry paths, matched repositories/tasks, identical source/model/tool permissions and acceptance gates, and independent behavior checks. Count verified task success, escaped mandate violations, unnecessary work, retries, reviewer corrections, total tokens and monetary/compute costs, and p50/p95/p99 latency including failed and canceled attempts. Stratify same-name multi-package cases separately from unique-name controls. More mandates, delivered prose, or test invocations are not outcome metrics in themselves.

The checkable exit is narrow: **a passing test in B cannot discharge an authorized requirement for A's test merely because their unqualified names match, while a genuinely matching current witness remains reusable.**

## Pinned references

[baseline]: https://github.com/theRebelliousNerd/codenerd/commit/8d373fb6ca05ef03fec59b923c5f1925066d623a
[guidance]: https://github.com/theRebelliousNerd/codenerd/blob/8d373fb6ca05ef03fec59b923c5f1925066d623a/agents.md
[session-guidance]: https://github.com/theRebelliousNerd/codenerd/blob/8d373fb6ca05ef03fec59b923c5f1925066d623a/internal/session/agents.md
[decks]: https://github.com/theRebelliousNerd/codenerd/blob/8d373fb6ca05ef03fec59b923c5f1925066d623a/Docs/innovations/plan-bound-expert-decks.md
[test-audit]: https://github.com/theRebelliousNerd/codenerd/blob/8d373fb6ca05ef03fec59b923c5f1925066d623a/Docs/audits/test-evidence-capability.md
[receipt-audit]: https://github.com/theRebelliousNerd/codenerd/blob/8d373fb6ca05ef03fec59b923c5f1925066d623a/Docs/audits/workspace-test-receipt-invocation-identity.md
[queue]: https://github.com/theRebelliousNerd/codenerd/blob/8d373fb6ca05ef03fec59b923c5f1925066d623a/Docs/audits/ASTRA_QUEUE.md
[consultation]: https://github.com/theRebelliousNerd/codenerd/blob/8d373fb6ca05ef03fec59b923c5f1925066d623a/internal/session/expert_spawn.go#L122-L158
[consultation-prompt]: https://github.com/theRebelliousNerd/codenerd/blob/8d373fb6ca05ef03fec59b923c5f1925066d623a/internal/session/expert_spawn.go#L212-L235
[parser]: https://github.com/theRebelliousNerd/codenerd/blob/8d373fb6ca05ef03fec59b923c5f1925066d623a/internal/session/expert_cards.go#L39-L98
[record]: https://github.com/theRebelliousNerd/codenerd/blob/8d373fb6ca05ef03fec59b923c5f1925066d623a/internal/session/expert_cards.go#L169-L191
[mandates]: https://github.com/theRebelliousNerd/codenerd/blob/8d373fb6ca05ef03fec59b923c5f1925066d623a/internal/core/defaults/policy/shard_experts.mg#L359-L378
[authority-field]: https://github.com/theRebelliousNerd/codenerd/blob/8d373fb6ca05ef03fec59b923c5f1925066d623a/internal/system/agent_registry.go#L90-L95
[authority-measurement]: https://github.com/theRebelliousNerd/codenerd/blob/8d373fb6ca05ef03fec59b923c5f1925066d623a/internal/system/shard_expert_facts.go#L116-L123
[test-facts]: https://github.com/theRebelliousNerd/codenerd/blob/8d373fb6ca05ef03fec59b923c5f1925066d623a/internal/testfacts/facts.go#L35-L68
[turn-facts]: https://github.com/theRebelliousNerd/codenerd/blob/8d373fb6ca05ef03fec59b923c5f1925066d623a/internal/session/turn_test_facts.go#L35-L84
[sync]: https://github.com/theRebelliousNerd/codenerd/blob/8d373fb6ca05ef03fec59b923c5f1925066d623a/internal/session/gate_facts.go#L48-L92
[test-schema]: https://github.com/theRebelliousNerd/codenerd/blob/8d373fb6ca05ef03fec59b923c5f1925066d623a/internal/core/defaults/policy/coder_safety.mg#L343-L357
[shard-wiring]: https://github.com/theRebelliousNerd/codenerd/blob/8d373fb6ca05ef03fec59b923c5f1925066d623a/internal/shards/registration.go#L294-L315
[closure]: https://github.com/theRebelliousNerd/codenerd/blob/8d373fb6ca05ef03fec59b923c5f1925066d623a/internal/session/change_evidence.go#L74-L112
[final-suite]: https://github.com/theRebelliousNerd/codenerd/blob/8d373fb6ca05ef03fec59b923c5f1925066d623a/internal/session/gate_memo.go#L202-L250
[packages]: https://github.com/theRebelliousNerd/codenerd/blob/8d373fb6ca05ef03fec59b923c5f1925066d623a/internal/session/test_verify.go#L139-L186
[own-gate]: https://github.com/theRebelliousNerd/codenerd/blob/8d373fb6ca05ef03fec59b923c5f1925066d623a/internal/session/build_verify.go#L581-L599
[verification]: https://github.com/theRebelliousNerd/codenerd/blob/8d373fb6ca05ef03fec59b923c5f1925066d623a/internal/core/defaults/policy/coder_safety.mg#L728-L740
[protected]: https://github.com/theRebelliousNerd/codenerd/blob/8d373fb6ca05ef03fec59b923c5f1925066d623a/internal/core/mangle_updates.go#L320-L330
[policy-test]: https://github.com/theRebelliousNerd/codenerd/blob/8d373fb6ca05ef03fec59b923c5f1925066d623a/internal/core/expert_mandate_policy_test.go
[session-test]: https://github.com/theRebelliousNerd/codenerd/blob/8d373fb6ca05ef03fec59b923c5f1925066d623a/internal/session/expert_mandate_test.go
