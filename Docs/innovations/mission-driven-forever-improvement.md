---
doc-class: deep-dive
subsystem: campaign
implementation-status: proposed
evidence-status: static-source-and-literature-review
last-verified: 2026-10-30
verified-against: 89ee924f59829d92186bb6fc88a39ebe0c6c2ac6
supersedes: []
---

# Mission driven forever improvement through the existing recurse loop

## Recommendation

Extend CodeNERD's existing recurse loop so that every admitted improvement has a traceable reason to exist: an unmet North Star requirement, an observed failure, a measured inefficiency, or an explicitly authorized experiment. Carry that reason through planning, implementation, independent verification, adoption and subsequent observation. Keep the loop running while authorized work remains productive, but make every individual claim and every deployment bounded and reversible.

The proposed advance is a connection between systems CodeNERD already has. Recurse supplies the long-running scheduler and ratchet. North Star supplies strategic direction and constraints. Campaigns supply scoped work and verification. Debug-self and explicit self-diagnosis supply operational evidence and repair entry points. Prompt evolution supplies a controlled learning lifecycle. The missing integration is a shared, revision-aware account of which obligation a change addresses, which evidence actually discharges it, and what should happen when that evidence becomes stale.

This is an integrative proposal, not a new controller, second endless command, implementation authorization or demonstrated self-improvement result. Repository claims are pinned to public main commit [`89ee924f59829d92186bb6fc88a39ebe0c6c2ac6`](https://github.com/theRebelliousNerd/codenerd/commit/89ee924f59829d92186bb6fc88a39ebe0c6c2ac6). No repository code, tests, runtime, models, training or benchmarks were executed for this study. Proposed predicates and record names below are design notation, not supported APIs.

## Start from the existing program

[Journey 10](https://github.com/theRebelliousNerd/codenerd/blob/89ee924f59829d92186bb6fc88a39ebe0c6c2ac6/Docs/journeys/10-forever-loop.md) already defines the operating mode: continuously improve the current workspace from dependency leaves toward entry points, using the existing `nerd campaign recurse` and `/recurse` surfaces. It explicitly rejects a parallel replacement command. [Root guidance](https://github.com/theRebelliousNerd/codenerd/blob/89ee924f59829d92186bb6fc88a39ebe0c6c2ac6/agents.md) supplies the architectural rule: the model creates and reasons; Mangle derives executive decisions; host tools measure and execute permitted effects. The North Star is continuing pressure toward desired behavior, not a declaration that the entire project is finished.

The following status distinctions matter before adding work.

| Area | Existing foundation | Proposed extension |
|---|---|---|
| Recurse | Workspace DAG, measured findings, policy selection and stalls, rotating improvement angles, ratchet, Git history and resumable journal | Admit mission-linked work and record requirement-specific evidence, without another scheduler |
| North Star | Vision validation and authority synchronization; alignment assessments; typed revision-bound risk mitigation and explicit waiver distinctions | Requirement evidence contracts and a production writer that knows which checks support which requirement |
| Campaigns | Attempt campaigns, observed execution, write guards, acceptance machinery and North Star risk observation | Preserve strategic obligation identity through decomposition, refinement, resume and final evidence publication |
| Debugging | Debug-self log projection and capped reports; separate explicit self-diagnosis and gated fix continuation | Bind reported problem, diagnosis, repair, rebuilt artifact and original-symptom recheck into one lifecycle |
| Learning | Quarantine, exact-version trials, evidence claims, bounded recording/drain, idempotent ledger and adaptation spend | Join actual served candidate lineage to controlled, independent incremental-utility evaluation |
| Verification | Typed evidence and existing gate/domain machinery | Reuse those contracts for mission claims; do not infer strategic acceptance from aggregate activity |

These entries mean inspected source paths or documented implementation reconciliations exist, not that this review measured their runtime success. The [North Star blueprint's As built and Adopted sections](https://github.com/theRebelliousNerd/codenerd/blob/89ee924f59829d92186bb6fc88a39ebe0c6c2ac6/Docs/architecture/northstar/proposed-vision-and-evidence-blueprint.md#as-built-2026-10-27) already close substantial earlier gaps. Its explicitly deferred evidence writer and typed requirement evidence are the starting point here. The [prompt-evolution reconciliation](https://github.com/theRebelliousNerd/codenerd/blob/89ee924f59829d92186bb6fc88a39ebe0c6c2ac6/Docs/audits/prompt-evolution-lifecycle-and-evidence-accounting.md#as-built-2026-10-27) likewise supersedes the historical deficiencies at the beginning of that document.

Existing [agent-native architecture research](https://github.com/theRebelliousNerd/codenerd/blob/89ee924f59829d92186bb6fc88a39ebe0c6c2ac6/Docs/audits/agent-native-architecture-grand-design.md) already proposes a maintained evidence graph and CodeDOM change packets. [Plan-bound expert decks](https://github.com/theRebelliousNerd/codenerd/blob/89ee924f59829d92186bb6fc88a39ebe0c6c2ac6/Docs/innovations/plan-bound-expert-decks.md) already proposes scoped, fresh advisory artifacts and selective consultation. This proposal specializes those ideas to the existing forever loop; it does not claim to originate them.

## The current strategic seam

The direct recurse improvement path reads a node's or workspace's `nerd.md` North Star purpose and requirements into text in [`recurse_cycle.go`](https://github.com/theRebelliousNerd/codenerd/blob/89ee924f59829d92186bb6fc88a39ebe0c6c2ac6/internal/campaign/recurse_cycle.go#L1162-L1190). [`recurse_attempt.go`](https://github.com/theRebelliousNerd/codenerd/blob/89ee924f59829d92186bb6fc88a39ebe0c6c2ac6/internal/campaign/recurse_attempt.go#L207-L232) supplies that text to an extend attempt. The current [policy](https://github.com/theRebelliousNerd/codenerd/blob/89ee924f59829d92186bb6fc88a39ebe0c6c2ac6/internal/core/defaults/policy/recurse.mg#L333-L398) rotates stabilize, harden, simplify and extend; extend's improvement metric is an increased test count.

This does not mean North Star is disconnected. Attempt campaigns also use the campaign observer and risk gate. Nor does it mean a test-count increase is the only acceptance condition: the ratchet separately applies verification, gate-regression, forbidden-write and metric constraints. The narrower opportunity is to add a direct typed join between a strategic requirement and its current behavioral evidence. A new test can be useful without establishing the particular capability the selected requirement describes.

The present verification scope also needs precise wording. The ordinary changed-attempt path remeasures the node's own gates and workspace-wide build/lint gates, plus the target workspace gate when fixing one. Workspace-wide tests and audits are run at pass boundaries rather than universally after every individual attempt. The [policy states this distinction](https://github.com/theRebelliousNerd/codenerd/blob/89ee924f59829d92186bb6fc88a39ebe0c6c2ac6/internal/core/defaults/policy/recurse.mg#L260-L264), and the [host selects the after-checks accordingly](https://github.com/theRebelliousNerd/codenerd/blob/89ee924f59829d92186bb6fc88a39ebe0c6c2ac6/internal/campaign/recurse_cycle.go#L959-L978). Do not describe each kept attempt as freshly passing every whole-workspace test and audit.

The CLI additionally supplies the optional error-free `MeasureGates` callback and full-refresh interval in its [configuration](https://github.com/theRebelliousNerd/codenerd/blob/89ee924f59829d92186bb6fc88a39ebe0c6c2ac6/cmd/nerd/cmd_campaign_recurse.go#L228-L240). The [chat configuration](https://github.com/theRebelliousNerd/codenerd/blob/89ee924f59829d92186bb6fc88a39ebe0c6c2ac6/cmd/nerd/chat/campaign_recurse.go#L150-L180) does not supply that optional callback. Both still use the shared ordinary loop. A mission-evidence adapter must expose the effective checks and surface configuration, test both routes, and either explicitly preserve or deliberately reconcile this difference; a CLI-only measurement must not become an assumed chat guarantee.

Treat strategic distance as a structured set of unmet or uncertain obligations, not one invented percentage. A relationship graph can establish that a requirement supports a capability; it cannot establish that software satisfies the requirement. Similarly, a green risk assessment says the proposal is aligned or admissible under that assessment, not that the resulting behavior works.

## A requirement evidence bridge

Introduce an obligation record with a stable requirement ID and version, its authoritative vision/spec revision, affected production paths, permitted effects, evidence contract and current disposition. Start with one narrow capability whose success can be checked through a normal production entry path. Do not transduce the entire vision into thousands of mandatory tasks in one pass.

The bridge needs five distinct records:

1. **Requirement definition.** The behavioral statement, scope, user authority, constraints and version. Semantic edits create a new version rather than silently reusing past acceptance.
2. **Verification contract.** Which observable behavior could support or contradict that statement; fixture/environment/tool identity; minimum scope; required negative controls; and whether human judgment remains necessary. The model may propose this contract, but reviewed policy admits it.
3. **Evidence observation.** What actually ran or was inspected, against which source and executable identity, with outcome, completeness, scope, producer identity and retained artifacts. A runner error remains an unavailable observation, not a negative test result or a pass.
4. **Applicability decision.** Whether the observation still applies after code, dependencies, configuration, data, tools, requirements or environment change. Initially use conservative whole-revision invalidation where dependency coverage is unknown; narrow it only with a sound read/dependency contract.
5. **Acceptance disposition.** Supported for the declared scope, contradicted, unknown, blocked, waived or superseded. Preserve the evidence that produced it. A waiver carries authority and expiry and does not become verification.

Keep the full North Star structure: Mission, Problem, Vision, Personas, Capabilities, Risks, Requirements and Constraints. Mission and Vision guide priorities; persona pain identifies intended benefit; capability relationships group behavior; risks determine assurance needs; requirements supply scoped obligations; constraints restrict every proposed route. Preserve `serves`, `supports` and `addresses` relationships and their endpoint versions. Neither flattening this graph to a single vision string nor adding a naked numeric completion score preserves its meaning.

A minimal illustrative sidecar could have this shape; it is deliberately separate from the current supported import schema:

```yaml
obligation:
  id: requirement_id
  requirement_version: content_digest
  vision_version: content_digest
  supports: [{capability_id: capability_id, version: content_digest}]
  addresses: [{risk_id: risk_id, version: content_digest}]
  contract_id: reviewed_behavioral_contract
  contract_version: content_digest
  authority_ref: owner_scope_record
  effects_scope_ref: admitted_paths_and_effects
evidence:
  id: immutable_observation_id
  attempt_id: durable_campaign_attempt
  check_id: named_check
  check_version: content_digest
  source_revision: full_commit_sha
  workspace_snapshot: content_digest
  executable_digest: content_digest
  environment_ref: versioned_environment_manifest
  scope_ref: measured_domain
  completeness: complete_or_partial_or_unavailable
  outcome: passed_or_failed_or_error_or_canceled
  artifacts: [bounded_immutable_reference]
decision:
  obligation_id: requirement_id
  requirement_version: content_digest
  evidence_ids: [immutable_observation_id]
  policy_version: content_digest
  applicability: current_or_stale_or_unknown
  disposition: supported_or_contradicted_or_unknown_or_blocked_or_waived
```

An evidence-to-obligation edge is keyed by both endpoint identities and versions plus the contract version, not a filename or display label. A campaign split or merge retains explicit predecessor/successor edges; reusing a task ordinal cannot transfer acceptance. Keep execution outcome separate from acceptance disposition and preserve supersession history. Schema validation, migration, declarations and every producer/consumer would need spec-first implementation; this example does not authorize new fields in live North Star JSON.

The host records observations and identities. Mangle derives obligation state from admitted evidence contracts, completeness, applicability and observed outcomes. It must not derive success from the absence of failure unless the required observation domain is explicit and complete. Semantic interpretation remains a fallible model or human judgment where appropriate; formalizing its result does not turn it into a theorem.

A possible data flow is:

```text
versioned requirement and admitted evidence contract
  -> current evidence deficit
  -> scoped work proposal
  -> existing recurse selection and attempt campaign
  -> observed mutation and production-path checks
  -> current applicability and acceptance disposition
  -> retained evidence, invalidation dependencies and next deficit
```

Make the initial writer an adapter over existing campaign/session verification results. It should attach a requirement receipt only when a named check is explicitly bound to the requirement contract. Do not automatically label every passing test as supporting every capability touched by its package. The [blueprint](https://github.com/theRebelliousNerd/codenerd/blob/89ee924f59829d92186bb6fc88a39ebe0c6c2ac6/Docs/architecture/northstar/proposed-vision-and-evidence-blueprint.md#as-built-2026-10-27) already states why automatic writeback is deferred: existing production paths do not know which checks verify vision-level risks.

### A worked example of a mission linked attempt

Consider a generic requirement that a canceled background operation must leave unrelated user edits intact. This is an illustrative acceptance contract, not a claim that the current CodeNERD implementation fails it. A source audit or observed conflict first opens a deficit with its evidence and uncertainty. The admitted reproducer creates independent operation and user-write identities, exercises the normal cancellation route, and checks both the operation's owned cleanup and the user's resulting bytes. Its negative controls include no concurrent user write, a changed expected postimage, and cancellation before any operation effect.

The existing campaign receives the bounded work item, affected interfaces and witness. The worker can propose a repair, but cannot remove the preservation assertion or redefine a conflict as success. The verification adapter records source, fixture, runtime and scope identity and publishes the observations. Mangle derives scoped acceptance only when the required observations are complete and current. A later edit to the cancellation or ownership boundary reopens the requirement's evidence obligation. A newly added unrelated test, a passing general build and a model saying the problem is fixed do not substitute for this witness.

If the repair is accepted but the production executable has not been rebuilt or restarted, the source claim can be supported while the active-runtime claim remains pending. That distinction is valuable operational output rather than a reason to hide incomplete deployment behind one green status.

## Choose useful work without turning the vision into an endless queue

Recurse should retain its dependency-aware traversal and existing repair priorities. Add mission-linked candidates to its existing admission and selection machinery. A candidate must identify an observed deficit, the smallest useful proposed change, a falsifiable acceptance contract, expected effects, dependencies and an uncertainty disposition. Repeated proposals over unchanged evidence should coalesce instead of purchasing another plan.

Separate urgent repair from exploratory improvement. A concrete user-visible failure deserves a repair route; a missing capability deserves a bounded extension; an uncertain bottleneck deserves measurement before optimization. A missing tool or unavailable evidence source is an explicit blocker, not a reason to cycle through more expensive models indefinitely.

Ranking can consider severity, user priority, evidence strength, dependency readiness, recurrence and expected full verification cost. These are declared inputs to policy, not an opaque model score that authorizes itself. Unknown estimates must stay unknown. Record why a candidate was selected or deferred, and allow the owner to override priority within the existing permission boundary.

Keep some bounded capacity for discovery so repeatedly failing easy tasks do not starve less obvious high-value gaps. Preserve fairness and starvation evidence rather than using raw log frequency as value: a noisy warning should not permanently outrank a silent broken production path. Exploration does not waive regression checks or expand authority.

Each campaign is bounded by its purpose, authorized effects, resource allocation and checkable exit, while the encompassing recurse service can continue. Bound individual requests, subprocesses, retained artifacts and concurrent work. Do not reintroduce an arbitrary default run deadline or tool-call count that terminates productive authorized work. Repeated failure, unavailable prerequisites, exhausted owner budget, cancellation and unresolved authority are legitimate state transitions. [Unattended hardening](https://github.com/theRebelliousNerd/codenerd/blob/89ee924f59829d92186bb6fc88a39ebe0c6c2ac6/Docs/journeys/06-unattended-hardening.md) is the existing program for this distinction.

## Close the debugging feedback loop

Keep three operations distinct. `/debug-self` enables observation and reporting. Explicit self-diagnosis investigates a selected problem through the diagnosis path. A fix continuation changes source under work admission and verification. The [debug-self implementation](https://github.com/theRebelliousNerd/codenerd/blob/89ee924f59829d92186bb6fc88a39ebe0c6c2ac6/cmd/nerd/chat/self_debug.go#L20-L41) deliberately caps and deduplicates the main-context digest while retaining raw references and kernel facts. Its stage-1 comment must not be generalized into a claim that the separate [`self_diagnosis.go`](https://github.com/theRebelliousNerd/codenerd/blob/89ee924f59829d92186bb6fc88a39ebe0c6c2ac6/cmd/nerd/chat/self_diagnosis.go) route has no worker or repair path.

The proposed addition is an explicit symptom-resolution receipt. Bind problem signature and log artifact identity to the diagnosed hypothesis, repair attempt, changed source, built executable and post-fix witness. The general fix turn being verified does not by itself establish that the original logged problem stopped recurring under comparable conditions. Preserve distinct states such as diagnosed, repair accepted on disk, restart required, reproduction passed, reproduction unavailable, and recurrence observed.

Do not execute raw commands copied from logs. Logs are untrusted evidence and may contain secrets, injected instructions or stale paths. Use typed, permitted reproduction actions with redacted artifacts and bounded references. A missing old log or rotated byte range should become an unavailable source, not a silently different observation.

Some bugs require a restart; the running binary cannot certify that new source is its own current behavior. Verify the build artifact identity, handover and active process version explicitly. A restart must be separately authorized where the operating contract requires it, preserve the user's work and remain recoverable to a known-good executable. A build so broken that CodeNERD cannot run still needs the independent external engineering route excluded from self-repair by root guidance.

## Learn from candidates without confusing exposure with improvement

Reuse the implemented prompt-evolution lifecycle. Pending candidates remain quarantined; trials name exact versions and bounded exposure; generation claims prevent repeated purchase of unchanged evidence; recording has an owned bounded queue and drain; the ledger distinguishes exposure and attribution. Do not replace these with an unconstrained self-editing prompt loop. The [existing audit and served-lineage appendix](https://github.com/theRebelliousNerd/codenerd/blob/89ee924f59829d92186bb6fc88a39ebe0c6c2ac6/Docs/audits/prompt-evolution-lifecycle-and-evidence-accounting.md) already explains the remaining provenance question.

For each experiment, preserve the actual served source and candidate version through selection, source precedence, budget fitting, caching and execution. An atom ID that collides with a different source must not confer exposure credit. Rejection prevents future admission; it must not erase an exposure that already happened. Candidate replacement cannot rewrite historical lineage. Distinguish the admitted version from a concise rendered fragment.

Separate four outcomes: a candidate was proposed; its content was actually served; a task succeeded; and the candidate caused a useful improvement. Only the first three are ordinary observations. Incremental utility needs a comparison design. A candidate served on an easy successful task has not thereby proved beneficial, and more trials on adaptively selected successful tasks do not establish generalization.

Use development workloads for discovery, a separately controlled validation set for promotion decisions, and retained holdout tasks not exposed to the candidate generator for independent assessment. Track all accesses and disclosures. Dwork and colleagues show why adaptive reuse can overfit a holdout and develop mechanisms for controlled reuse; merely naming a set “held out” does not implement their guarantees. For CodeNERD, begin with strict access separation, limited promotion queries and refreshed task families before considering a formal reusable-holdout mechanism. [Generalization in Adaptive Data Analysis and Holdout Reuse](https://arxiv.org/abs/1506.02629v2).

Sequential trials should use a prespecified analysis compatible with repeated looks. Howard and colleagues develop time-uniform confidence sequences under stated statistical assumptions. This motivates an appropriate sequential decision method, not blanket confidence guarantees for correlated repositories, changing agents or arbitrary adaptive test selection. Account for multiplicity across candidates and task families; a confidence sequence for one endpoint does not solve all adaptive selection bias. [Time-uniform confidence sequences](https://arxiv.org/abs/1810.08240v9).

Use staged resource allocation to reject clearly unpromising candidates cheaply while retaining expensive final validation for survivors. Hyperband studies adaptive resource allocation and early stopping for hyperparameter optimization. Its principle motivates a comparison arm here, but short coding tasks or small suites may rank candidates differently from long campaigns, so low-fidelity screening must be calibrated and may not replace final assurance. [Hyperband](https://jmlr.org/papers/v18/16-558.html).

The Darwin Gödel Machine demonstrates an empirical self-modification research approach with an archive of agent variants and benchmark evaluation under sandboxing and oversight. It supports investigating diverse candidate lineages rather than only the latest greedy mutation. It does not prove that any CodeNERD change is safe, that an endless process is monotonic, or that benchmark gains transfer to production. A bounded archive with explicit lineage and retirement is the appropriate experiment, not uncontrolled branching. [Darwin Gödel Machine](https://arxiv.org/abs/2505.22954v3).

Promotion should require declared benefit on the target workload, no unacceptable regressions on protected strata, complete cost accounting and the required human/policy authorization. Unknown evidence remains a reason to defer. Never let the candidate modify its own promotion threshold, evaluation harness, hidden workloads or safety envelope as part of the same ordinary improvement trial.

## Preserve owned effects and recoverable adoption

Existing recurse already journals work, uses a dedicated branch, measures before and after and commits or compensates. Current cancellation code calls `undoOwned`, so the historical broad-rollback finding must not be presented as if no repair landed. See the [current attempt path](https://github.com/theRebelliousNerd/codenerd/blob/89ee924f59829d92186bb6fc88a39ebe0c6c2ac6/internal/campaign/recurse_cycle.go#L908-L1124) and [prior ownership audit](https://github.com/theRebelliousNerd/codenerd/blob/89ee924f59829d92186bb6fc88a39ebe0c6c2ac6/Docs/audits/recurse-compensation-write-ownership.md).

The integrated loop should make its recovery contract explicit across process boundaries: immutable attempt ID, admitted scope, base revision, observed effect receipts, expected postimages, verification identity, adoption decision and durable commit identity. Record an in-flight intent before effects; reconcile outcomes rather than assuming a missing completion event means no change occurred. A crash after commit but before journal publication must recover the committed result once, not repeat it.

Do not reconstruct ownership solely from everything different from HEAD. Preserve unrelated user and external-editor changes. Recovery should stop with a conflict when an effect's ownership or expected version cannot be established. An in-memory receipt and a graceful shutdown drain are not crash durability. Likewise, a read-then-restore version check is not an atomic compare-and-swap against arbitrary writers. These are contracts to validate on the current implementation, not claims that a particular destructive crash has been reproduced.

Keep scope-aware gate work. Earlier [scoped measurement reconciliation](https://github.com/theRebelliousNerd/codenerd/blob/89ee924f59829d92186bb6fc88a39ebe0c6c2ac6/Docs/audits/gate-scoped-recurse-current-main-addendum.md), [comparison-domain admission](https://github.com/theRebelliousNerd/codenerd/blob/89ee924f59829d92186bb6fc88a39ebe0c6c2ac6/Docs/audits/gate-comparison-domain-admission.md) and [overlay evidence conservation](https://github.com/theRebelliousNerd/codenerd/blob/89ee924f59829d92186bb6fc88a39ebe0c6c2ac6/Docs/audits/scoped-gate-overlay-evidence-conservation.md) already own that design space. Reuse landed protections and report precisely which domains were newly checked versus retained. Periodic full checks provide additional detection, not retrospective proof that every earlier partial snapshot was globally current.

## Optional concurrency after serial evidence is trustworthy

Keep sequential recurse as the default and first experimental baseline. Optional concurrency should begin with independent read-only diagnosis or candidate preparation, not simultaneous mutation of the same checkout. More workers are useful only when dependencies, effects, model capacity and verification resources permit independent progress.

For concurrent candidates, use isolated workspaces and explicit base revisions, read/write/effect footprints and resource ownership. A clean merge is insufficient: evidence for each branch may no longer apply to the combined result. Serialize authoritative adoption, invalidate affected evidence and run the required integration checks against the assembled state. Cancellation of one consumer must not destroy a shared observation another live consumer owns.

Do not require one resident agent per directory or a new general multi-agent architecture. Reuse campaign dependency and ownership machinery and the existing expert-deck proposal. Run a paired study before allowing parallelism to increase provider spend or mutation scope. If coordination and revalidation consume more than they save, retain the serial mode.

## Innovation frontier as a falsifiable candidate portfolio

Beyond repair, maintain a bounded portfolio of distinct hypotheses: better context construction, a deterministic tool that removes repetitive reasoning, a stronger behavioral oracle, a new missing capability, or a cheaper route to the same independently accepted result. This is a proposed experiment layered on recurse, not a claim that an innovation engine is already implemented. Candidate diversity should represent different mechanisms, not many paraphrases of the same idea.

Use a discovery funnel: observed friction or strategic deficit; explicit hypothesis; cheapest discriminating observation; isolated prototype if authorized; matched baseline comparison; independent acceptance; reversible adoption; later recurrence and drift monitoring. Each stage needs a falsifier. For example, a proposed tool that claims to save turns is rejected if total verified-task cost rises after including its build, maintenance and verification cost, or if it omits necessary context. A candidate with no usable test is deferred as an unmeasured hypothesis rather than promoted by enthusiasm.

Retain a small number of informative rejected variants and their negative evidence so the system can revisit them when dependencies change. Expire redundant or unsupported branches. Owner authority determines which capability families, experiments, external services and resource envelopes are in scope; novelty does not create authority.

Compositional experiments are a separate hypothesis. Two independently useful changes can interact badly: an aggressive context filter can remove the evidence a new verifier needs, while a new planner can change which tasks enter an otherwise calibrated trial. Evaluate A, B and A+B against the same baseline on matched task strata, record interaction effects and verify the combined artifact. A successful individual candidate cannot donate its acceptance to an untested combination. Start with a small factorial comparison, not an exponentially growing combination search.

## Costs and feedback that influence the next decision

Measure the full cost of accepted improvement: discovery, source/index refresh, planning, consultation, inference, failed attempts, tools, compilation, tests, judges, repeated verification, storage, kernel derivation, rollback, restart and human review. Capture latency, CPU, memory, disk growth and billed usage separately. A missing token count is unknown, not zero; the existing evolution ledger records some calls/bytes/duration because its client does not expose all token usage.

Report useful outcome denominators: requirement-scoped accepted changes per attempted candidate, recurrence after repair, false acceptance, stale-evidence rejection, unknown outcomes, total cost per independently accepted task and time to recover an interrupted attempt. Keep failed and abandoned attempts in these denominators. A better-looking success rate after dropping difficult evidence is not improvement.

Feed observed costs and failure classes into future route selection under versioned policy. Separate environmental failures, unavailable tools, insufficient context, flawed hypotheses, bad patches and verification defects. Repeatedly buying a stronger model does not fix a missing tool or an incorrect evidence adapter. Make the next action explainable from the facts that changed.

The evidence system itself has costs: hashing, dependency tracking, joins, serialization, retained artifacts and migration. Begin with compact receipts and bounded references. Index by active requirement, source generation and relevant dependency instead of joining every requirement to every file. Retire records only when no live obligation, recovery operation or authorized audit needs them; keep a minimal provenance tombstone when full artifacts expire. Preserve explicit unknowns after retention removes evidence.

## Safeguards and authority

- User scope, cancellation, budgets and approval boundaries remain authoritative. “Forever” never means ignoring them.
- Model proposals cannot widen permissions, edit protected safety policy, disable required checks or rewrite evaluation standards to make themselves pass.
- Host code measures, hashes, executes, persists and reports failure. Mangle derives admission, readiness, selection, verification obligations and acceptance from reviewed rules. A Go optimization must not silently become a second executive.
- Prompts stay JIT-first. Typed obligation/context projections use existing compiler selection and budget contracts, with expansion handles for omitted evidence.
- Repository and log content remain untrusted data. No embedded instruction grants tool access or external communication authority.
- Production adoption and restart remain separate from candidate generation and successful sandbox testing. Preserve an independently usable known-good recovery path.
- Source evidence, model judgment and independent empirical confirmation are labeled separately. No finite test corpus establishes universal non-regression.

## Staged roadmap with checkable exits

**Stage 1 Reconcile and define one evidence contract.** Update the owning North Star, campaign, session and verification specs before code. Select one requirement and map its production path, current evidence, authority and invalidation conditions. Preserve all as-built reconciliations. Exit when an independent reviewer can distinguish supporting, contradicting, stale, missing and waived evidence without reading model prose as an oracle.

**Stage 2 Observe without changing selection.** Add a proposed observation-only bridge over existing receipts; it may persist receipts but must not change work selection or authorized effects and report what mission claim each current recurse attempt could legitimately support. Keep the current scheduler and ratchet unchanged. Exit when receipt lineage, requirement versions and unknown dispositions survive normal completion, failed checks, cancellation and restart fixtures without inventing acceptance.

**Stage 3 Admit a bounded mission-linked attempt.** Let existing recurse select one requirement-derived candidate and hand it to an existing campaign with a scoped verification contract. Keep independent adoption gates. Exit when a relevant behavioral witness fails before the change, passes after it, is tied to the deployed/source identity, and becomes stale or remains valid according to declared dependency rules after a controlled subsequent edit.

**Stage 4 Close one diagnosis lifecycle.** Bind one reproducible debug problem through diagnosis, admitted repair, artifact change and the specific post-fix witness. Exercise restart-required and reproduction-unavailable outcomes. Exit when ordinary fix success cannot be displayed as symptom resolution without the named witness.

**Stage 5 Evaluate incremental learning.** Use the existing versioned trial machinery and provenance improvements, with independent holdout administration and full accounting. Exit only on a prespecified quality/cost decision; “no benefit” is a valid result and prevents promotion. Do not require a positive outcome to declare the experiment complete.

**Stage 6 Consider selective concurrency and broader coverage.** Compare against the proven serial treatment on matched tasks and assembled-result verification. Expand to more requirements, angles or languages only if the evidence supports it. Optimize, wire and document angles remain dependent on suitable measurements rather than being enabled by name alone.

## Evaluation against the existing baseline

No evaluation below has been run. Preserve the pinned current recurse implementation as the baseline. Use a separate authorized execution plan for deterministic fixtures and any later live-model trials.

Compare treatments incrementally: current recurse; current recurse plus passive obligation receipts; mission-linked selection with unchanged verification; requirement-specific verification; symptom-bound repair follow-up; controlled learning; and optional isolated concurrency. Avoid a single comparison that changes the model, prompts, tools, budgets and scheduler together. Hold source, task family, allowed effects, provider/model identity and acceptance requirements fixed where possible, and report deviations.

Deterministic fixtures should cover requirement edits, ambiguous transduction, missing evidence, wrong scopes, unavailable checks, source changes during verification, identical IDs from different sources, rejected candidates with in-flight exposures, duplicate/conflicting receipts, user edits during compensation, crash-before/after commit, changed runtime binary and expired artifacts. Use the actual entry-path adapters and kernel policy, not only unit-level record constructors. Existing test definitions are useful controls, not proof these proposed cases pass.

Production-path tasks should include a capability that is implemented but unwired, a regression with a reproducible symptom, a passing test-count increase that does not satisfy the chosen requirement, a costly candidate with no incremental benefit, and a useful change whose evidence is invalidated by an interacting edit. Include foreign-workspace examples to avoid encoding CodeNERD's own package layout as general policy.

Primary outcomes are independently accepted behavior, false acceptance, recurrence, applicability correctness, preserved user work, recovery completeness, constraint adherence and full cost. Secondary outcomes include selection fairness, evidence acquisition latency, stale-observation waste and retained state growth. Stratify failures and quality by task family; aggregate means must not hide a protected-surface regression. Report uncertainty and incomplete observations, and retain the negative results.

The proposal succeeds if CodeNERD can explain and substantiate this chain for each adopted improvement: which authorized behavior was missing, why this work was selected, what actually changed, what current evidence supports the result, what remains unknown, and what new observation would reopen the obligation. The existing recurse loop supplies the continuity. The proposed bridge makes its continuing activity accountable to the system it is meant to become.
