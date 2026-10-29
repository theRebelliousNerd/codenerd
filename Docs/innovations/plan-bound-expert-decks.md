# Plan-bound expert decks: reasoning delivered where the work needs it

## Decision

Make the plan the routing structure for expert reasoning. An expert should be able to contribute advice to a particular edit, a transition between edits, a campaign stage, or a verification obligation. Mangle should decide when that advice applies, whether existing knowledge is enough, whether fresh reasoning is needed, and whether the resulting evidence permits progress. The coding model should receive sufficient relevant context, including interacting constraints and access to the rationale behind them.

The deck metaphor is useful: several specialists contribute cards, and the next step receives its relevant hand. It must not mean random admission of critical advice. Mix specialties, but deal by applicability, dependency, freshness and policy. A security invariant must not disappear because it drew a bad hand.

The aim is to reduce avoidable model work without buying that reduction by omitting necessary reasoning or verification. Sometimes the right expert is a knowledge base the coder can read. Sometimes it is a separate model call, possibly to a model with stronger demonstrated performance on that class of problem. Sometimes the missing input is an observation or a tool result, and another model call would merely speculate more expensively.

The central distinction is **steering atoms versus mandates**. Steering helps the model make a good decision; mandates state authorized obligations the harness must account for. Both can travel with the plan, but their authority, lifecycle and acceptance conditions differ. This distinction must not be reduced to a prompt-priority number.

CodeNERD already has substantial context management. This design does not replace it or assume that the model is constantly at its context limit. Its new contribution is the lifecycle of plan-bound steering and mandates, connected to the existing JIT compiler, relevance, dependencies, budgeting and working ledger. Relevance and whole-task quality are the goals; a shorter prompt is only useful when it preserves the reasoning needed to complete the work.

**Status:** source-grounded design proposal, not implementation or a measured result. Current-source statements are pinned to CodeNERD main [`4d984f9c9b719b013a6400f0c88deb48d2e5eb82`](https://github.com/theRebelliousNerd/codenerd/commit/4d984f9c9b719b013a6400f0c88deb48d2e5eb82), revalidated after main advanced from `99c5526e` on October 29, 2026. The relevant source was rechecked, including the changed step compiler; no claim is made to have reviewed every intervening change. No implementation, tests, model calls, inference, training or benchmarks were performed for this document. No novelty, superiority, quality gain or token saving is claimed.

## 1. Start from what already exists

This is an extension of the existing architecture, not a parallel agent framework. Repository [guidance](https://github.com/theRebelliousNerd/codenerd/blob/4d984f9c9b719b013a6400f0c88deb48d2e5eb82/agents.md) assigns executive decisions to Mangle and makes JIT the standard for new model-facing behavior. The [prompt contract](https://github.com/theRebelliousNerd/codenerd/blob/4d984f9c9b719b013a6400f0c88deb48d2e5eb82/internal/prompt/agents.md) requires source-generation-aware cache identity, bounded retrieval, capability gating and no parallel hardcoded prompt path. The deck must preserve those requirements.

### Implemented building blocks

- A `PromptAtom` already carries an ID, version, content hash, token estimate, context selectors, dependencies, conflicts, mandatory status, tool requirements, and concise/minimal content. It has selectors for campaign phase, build layer, intent, language, framework and world state. [Atom contract](https://github.com/theRebelliousNerd/codenerd/blob/4d984f9c9b719b013a6400f0c88deb48d2e5eb82/internal/prompt/atoms.go#L136-L274).
- Mangle already selects and excludes atoms, propagates dependency constraints, and admits bounded runtime lookup results through `retrieved_context`. This witness does not override context, conflicts, dependencies or budget. [JIT policy](https://github.com/theRebelliousNerd/codenerd/blob/4d984f9c9b719b013a6400f0c88deb48d2e5eb82/internal/core/defaults/jit_compiler.mg).
- Experts already have persistent domain associations. Selection joins task files, language/import observations and registered domains. Language evidence is assigned 40 and import evidence 90; these are policy constants, not calibrated probabilities of competence. The shared session route derives consultation roles separately from knowledge relevance. [Expert policy](https://github.com/theRebelliousNerd/codenerd/blob/4d984f9c9b719b013a6400f0c88deb48d2e5eb82/internal/core/defaults/policy/shard_experts.mg), [selection result](https://github.com/theRebelliousNerd/codenerd/blob/4d984f9c9b719b013a6400f0c88deb48d2e5eb82/internal/experts/select.go#L44-L99).
- The compiler can lend selected experts' nonidentity prompt material and relevant knowledge rows to the ordinary coder. It need not run a separate expert consultation merely to expose stored knowledge. [Lending implementation](https://github.com/theRebelliousNerd/codenerd/blob/4d984f9c9b719b013a6400f0c88deb48d2e5eb82/internal/prompt/compiler_experts.go).
- Actual consultations produce task-specific prose. The shared executor runs selected consultations, waits for them, and appends answers or explicit failure/empty dispositions to the system prompt. A lead is asked to lay out work without editing; the task's executor keeps its own tools and gates. [Consultation path](https://github.com/theRebelliousNerd/codenerd/blob/4d984f9c9b719b013a6400f0c88deb48d2e5eb82/internal/session/expert_spawn.go#L44-L145).
- Campaign tasks already have IDs, dependencies, write sets, context links, artifacts and attempts; campaigns have revision numbers. Session work steps are thinner, primarily file/change records with execution outcomes. [Campaign structure](https://github.com/theRebelliousNerd/codenerd/blob/4d984f9c9b719b013a6400f0c88deb48d2e5eb82/internal/campaign/types.go), [session step structure](https://github.com/theRebelliousNerd/codenerd/blob/4d984f9c9b719b013a6400f0c88deb48d2e5eb82/internal/session/work_steps.go#L34-L65).
- Per-persona model/provider profiles have an existing executor hook. A caller-specified model/provider is retained, so configuring a profile is not itself proof that a particular call used it. [Profile context](https://github.com/theRebelliousNerd/codenerd/blob/4d984f9c9b719b013a6400f0c88deb48d2e5eb82/internal/session/shard_profile_context.go).

### Missing composition layer

An expert answer is presently appended prose rather than a task-scoped object selected at each consumer. Planned steps successfully compile fresh prompts and can omit the earlier consultation answer, while later verification still receives the original enriched prompt. The [planned-step consultation audit](https://github.com/theRebelliousNerd/codenerd/blob/2279ccc34cbb678eb98db9944800e8f2c83832d7/Docs/audits/planned-step-expert-consultation-continuity.md) owns that concrete defect and its controls.

This proposal adds a lifecycle connecting reasoning production, plan binding, prompt delivery and evidence. It should reuse, not duplicate, the [consultation coordination/reuse contract](https://github.com/theRebelliousNerd/codenerd/blob/4d984f9c9b719b013a6400f0c88deb48d2e5eb82/Docs/audits/consultation-coordination-and-safe-reuse.md), [generation-aware JIT design](https://github.com/theRebelliousNerd/codenerd/blob/4d984f9c9b719b013a6400f0c88deb48d2e5eb82/Docs/audits/jit-generation-aware-optimization.md), [dependency-closed budgeting](https://github.com/theRebelliousNerd/codenerd/blob/4d984f9c9b719b013a6400f0c88deb48d2e5eb82/Docs/audits/jit-dependency-closed-budgeting.md), and [planned-work conservation](https://github.com/theRebelliousNerd/codenerd/blob/4d984f9c9b719b013a6400f0c88deb48d2e5eb82/Docs/audits/planned-work-conservation.md). Main's complete 7,191-entry inventory and relevant neighboring documents were inspected for overlap; this is not a claim of exhaustive novelty research.

## 2. Two contracts: steering atoms and mandates

### Steering atoms

Steering atoms carry contextual advice: an idiom, pitfall, design alternative, useful example, hypothesis or recommendation. They can be selected and omitted by the existing relevance and budget machinery, subject to dependencies and the need to retain crucial caveats. A model may disagree with steering when it has evidence or a better approach. Merely not following a steering atom is not a failed task.

Steering still needs provenance, applicability and freshness. Optional does not mean untrusted text can widen permissions, overwrite requirements, or silently become a fact about the code. An expert's recommendation remains a recommendation until supported by observation or admitted as an obligation from an appropriate authority.

### Mandates

Mandates are explicitly authorized, scoped obligations with an acceptance contract. They may originate in user requirements, existing project specifications, applicable safety policy, or another authority the user has allowed the harness to enforce. Experts can propose mandates or identify that an existing mandate applies; they cannot grant their own proposal authority merely by saying “must.”

A mandate carries the authority reference, scope, applicability condition, required property, evidence contract, owner and disposition. Its enforcement state belongs in the obligation ledger/kernel, independently of whether its full prose is currently visible in a prompt. A JIT mandatory atom and a mandate are not synonyms: the first is a rendering inclusion rule, the second is an execution/acceptance obligation. A relevant mandate can produce a concise mandatory instruction while its witness and lifecycle remain separate structured data.

Satisfaction needs evidence appropriate to the claim. A named test requires the actual applicable test execution; a file-inclusion claim needs the effective build scope; a registration requirement needs a verified production path. Some requirements require human judgment or do not yet have a sound automated witness. In those cases the mandate remains unverified, awaits an authorized review, or is explicitly revised/waived by the appropriate authority. It must not become satisfied because the coding model or expert asserted success. If evidence is unavailable, report that state and follow the authorized disposition; do not loop forever, fake a proof, or quietly demote the mandate to optional steering.

Contradictory mandates require a decision by the appropriate authority or an evidence-backed scope resolution. They cannot be silently pruned by context budgeting. Interacting mandates need to be considered together even when that requires more context or different plan decomposition. The optimization target is a sufficient context for a sound decision, not the smallest possible hand.

### Shared card envelope

A card should retain one coherent instruction, warning, option or claim and the assumptions needed to interpret it. Splitting by sentence or a fixed token count can separate a recommendation from the exception that makes it safe. A card can have concise and expanded projections, but its applicability and caveats must survive compression.

The following JSON is illustrative design notation, not a current schema or executable configuration:

```json
{
  "card_id": "campaign-42/advice/migration-consumer-7",
  "version": 1,
  "producer": {"expert": "database-specialist", "consultation_id": "consult-9"},
  "parent": {"campaign_id": "campaign-42", "plan_revision": 3},
  "binding": {
    "producer_task": "change-schema",
    "consumer_task": "wire-service",
    "stage": "before-edit"
  },
  "guidance": "Preserve the old decoder until every reader accepts the new nullable field.",
  "assumptions": ["Old and new application versions can overlap during rollout."],
  "evidence_refs": ["source-observation-18", "deployment-requirement-4"],
  "read_dependencies": ["decoder-contract-revision", "rollout-contract-revision"],
  "kind": "steering",
  "disposition": "advisory_pending_policy_admission",
  "verification_proposal": "Exercise mixed-version reads before enabling the new write path.",
  "expanded_body_ref": "consult-9/answer",
  "status": "proposed"
}
```

The card is not allowed to assert its own authority. A host validates size, identity, allowed fields, referenced evidence and scope. Mangle decides admission and whether a verification proposal corresponds to an authorized requirement. Expert confidence alone cannot turn a suggestion into a mandatory gate. Existing user requirements, safety policy and verified contracts retain their authority; advice may explain how to satisfy them without being their source.

A useful open vocabulary within steering includes observations, warnings, implementation suggestions, alternative designs and unresolved questions. Mandates have a separate authority and evidence contract. Do not fix the content subtypes into an exhaustive taxonomy before actual workloads establish what is useful.

## 3. Bind advice to nodes, edges and conditions

### Node-bound cards

Some advice belongs to a stable task identity: a particular decoder edit, migration or test. Step number is presentation order, not identity. In a session plan, introduce stable node identity and a plan generation rather than binding cards solely to the current slice index. Campaigns can reuse their existing task/phase IDs.

### Edge-bound cards

The strongest extension is advice attached to the relationship between tasks. A schema expert can identify a contract that must hold between the migration producer and its service consumer. An integration expert can identify registration or configuration work that must follow implementation. That advice may not be retrieved by either file's local similarity score.

If a card identifies a required consumer that is absent from the plan, the system should propose a missing work item or flag an unresolved handoff. It should not merely insert a warning into a prompt and hope the model remembers to add a step. Admission of any new work remains governed by the user's goal and campaign effects contract.

### Conditional cards

Other advice becomes relevant when observations change: a step touches authentication, introduces a shared-state mutation, changes a public interface, or crosses a package boundary. Conditions should be represented by bounded measured/derived facts where possible. Mangle is not a natural-language theorem prover: arbitrary prose applicability needs transduction and retains uncertainty.

One card may bind to several nodes or edges. Rebinding must be explicit on split, merge, replacement and cancellation. A canceled node must not strand a mandatory obligation, and a new node must not inherit advice merely because it reused an old ordinal or filename.

## 4. Different consumers receive different hands

The planner needs sequencing constraints, unresolved choices and missing work. The editor needs local invariants, relevant examples and implementation pitfalls. The integration stage needs cross-component contracts. The verifier needs claims and witnesses. A repair pass needs the violated contract and its evidence.

Do not prepend the entire initial consultation to every step. That would fix delivery by sacrificing the existing per-step context focus. Instead, render bounded projections through JIT, retaining an expansion handle and the disposition of omitted material. The current `RetrievedContext` path is a plausible adapter, but task binding, freshness and artifact generations must become explicit inputs before reuse is sound.

Required material must fit with its dependency closure. If admitted mandatory guidance exceeds the available context, choose an explicit remedy such as splitting the work, requesting a permitted larger context, or returning an unresolved admission result. Do not quietly drop an invariant or truncate away its exception. Optional explanatory material can use concise projections, with the full body recoverable under the same access boundary.

Within one working loop, preserve the existing ledger/prefix behavior rather than replacing every prior prompt round whenever a card changes. Publish a bounded new applicability or invalidation event at a request boundary. Across step compiles, stable card ordering and immutable bodies can help reuse, but changing world state must still invalidate stale content. Do not introduce an independent context manager or assume aggressive compression is needed. When the current compiler can carry sufficient relevant material, keep it; measure interference and cost rather than treating length itself as a defect.

## 5. Mangle chooses when reasoning is worth buying

### Separate relevance from need for a model call

An expert can be relevant while no separate consultation is needed. The current language/import rules already express part of this distinction. Extend that separation: derive knowledge needs, unresolved questions and admissible routes independently. A strong import match should not automatically imply that stored knowledge is insufficient, and a weak lexical match should not exclude a critical cross-component constraint.

Use deterministic policy over observed inputs to choose among routes:

1. **Existing admitted card:** reuse a still-applicable answer for the same question and evidence state.
2. **Direct expert knowledge:** retrieve bounded relevant rows/atoms and give them to the coding model through JIT, with no separate consultation call.
3. **Targeted observation:** obtain missing source, structural, configuration or verification evidence through existing permitted tools.
4. **Ordinary expert consultation:** ask a narrow unresolved question with the necessary evidence bundle.
5. **Stronger-model consultation:** use a permitted model profile when measured task-family evidence or a policy-defined risk/ambiguity condition justifies the incremental cost.
6. **Explicit unresolved decision:** when evidence or authorization is missing, preserve the gap instead of looping through increasingly expensive guesses.

These are design alternatives, not an obligatory ascending ladder. A high-risk design question may go directly to the stronger permitted consultant. A missing file observation should not trigger a stronger model merely because a weaker model failed to guess its contents. A cheap local knowledge lookup may be best even when a strong model is available.

“No consultation call” does not mean the entire request makes zero model calls: the coder still runs, and existing optional relevance judgment can itself call a model. Count those separately. Avoid adding a new LLM router to every step merely to decide whether another LLM should be called. Precompute stable metadata and use existing task facts first; reserve uncertain semantic adjudication for cases that warrant its cost.

### Routing inputs and policy

Candidate signals include required domain coverage, source freshness, known unresolved contradictions, task effects, number of impacted interfaces, previous failure class, repeated unresolved attempts, and calibrated historical outcomes for the task family. Missing data must remain a distinct state. Current 40/90 scores must not be interpreted as probabilities of success or substituted for measured calibration.

The host reports facts and measured costs; Mangle derives admissible routes and the selected action under the current policy/configuration. Routing should expose a reason, such as valid-card reuse, knowledge-sufficient policy case, unresolved cross-domain contradiction, or failed bounded observation. A model's own request for a more expensive model is input to adjudicate, not authority to spend or widen access.

The objective is expected total verified-task cost under an assurance requirement, not minimum calls. Include the chance and cost of a wrong cheap route, unnecessary escalation, downstream repairs, and latency. If a statistical router is introduced later, version and validate it offline; let its prediction become evidence consumed by policy, not a replacement for capability or permission gates. [RouteLLM](https://arxiv.org/abs/2406.18665v4) is primary research on learned strong/weak model routing with preference data. Its reported tradeoffs motivate local controlled evaluation, not a transferable claim about CodeNERD or coding-task correctness.

### Stronger experts without stronger authority

A more capable or expensive model does not receive broader tools, filesystem access or permission by virtue of its tier. Keep consultation read-only where the existing contract is read-only. Use existing provider/profile machinery, verify the actual request identity, and record intended versus served model when the transport exposes it. Do not rely on a configuration label as proof of execution. Profile precedence, provider capabilities, cancellation and usage accounting must remain correct.

## 6. A concrete dealing algorithm

The following is a proposed algorithm, not a ready-to-run Mangle program:

1. Capture one immutable plan generation, applicable source/evidence generations, tool envelope and deck generation for this consumer.
2. Derive required domains and obligations from the task, node, dependency edges and current observations.
3. Retrieve candidate cards and expert knowledge with bounded indexes. Include dependency and conflict closure; a top-K prefilter must not make required material invisible.
4. Validate card provenance, applicability and source assumptions. Classify valid, stale, unsupported and explicitly rejected candidates separately.
5. Derive unresolved coverage and contradiction facts. A missing semantic judgment is not evidence that no contradiction exists.
6. Select an admissible route for each unresolved question: reuse, knowledge, observation, consultation, escalation or unresolved outcome. Coalesce equivalent requests under task, evidence, model and authorization identity.
7. On completed consultations, validate returned cards and publish a new deck generation. A failed or partial response stays visible without discarding other valid results.
8. Derive the current hand, respecting dependencies, mandatory obligations, tool requirements and budget. Render through JIT and record the actual delivered card versions after final budget fitting.
9. Execute the task with its original capability and verification contracts. Collect results and measure whether admitted obligations were satisfied, remained unresolved or became inapplicable with evidence.
10. Invalidate affected cards after source changes, update plan bindings after replanning, and retain enough provenance for the next decision. Archive superseded artifacts with bounded retention.

Derived relation names should be designed with the owning schema and stratification constraints. Do not copy this list as undeclared predicates or use a negative “no problem found” rule without an explicit complete candidate/obligation domain. Keep provisional facts scoped to the request and clean up only facts the request owns.

## 7. Disagreement is work, not a score tie

Current JIT `atom_conflicts` has directional supersession semantics in the policy. That is not a general representation of incompatible expert claims. Do not feed two disagreeing recommendations into that relation and let importance, vector score or lexical ID quietly choose reality. [Current conflict and supersession rules](https://github.com/theRebelliousNerd/codenerd/blob/4d984f9c9b719b013a6400f0c88deb48d2e5eb82/internal/core/defaults/jit_compiler.mg).

Instead, preserve a conflict record with the claims, shared scope, differing assumptions and required decision. Some apparent conflicts disappear when one card applies to rollout and another to steady state. A true contradiction can trigger targeted observation, a bounded arbitration request or a user decision when requirements themselves conflict. Majority vote is not a correctness oracle, particularly when experts share model, source or prompt lineage.

Advice is not automatically an obligation. Policy may admit a requirement backed by the user's specification, an existing invariant or an authorized safety rule. Optional design preferences remain options. This prevents experts from generating an unbounded tower of “must” statements that stops all work or silently expands scope.

## 8. Freshness, parallelism and reuse

Cards should declare the observations they depended on. A later edit can invalidate only the affected advice when dependency tracking is complete. Unknown read sets need conservative invalidation. Whole-workspace revision identity is a safe starting point but can force needless reconsultation; overly narrow identity can reuse advice that no longer applies.

Campaign tasks can run in parallel against different snapshots. Each consumer needs an immutable deck view, and publication of an expert result must not mutate a prompt already in flight. Reusing an agent name is not request equivalence. Reuse keys include question, task intent, evidence revisions, relevant configuration, permitted disclosure scope and model/producer identity where behavior depends on it.

Prefetching future advice can hide latency, but it is speculation with a cost. Start only for sufficiently stable near-ready plan nodes, bound concurrent work, and record cancellation or discard when the plan changes. A canceled consumer must not destroy shared work another live consumer owns. A consultation must not recursively spawn an unbounded expert tree; preserve existing recursion prevention and add budget ownership for any explicitly permitted nested work.

W3C [PROV-DM](https://www.w3.org/TR/prov-dm/) distinguishes generation, usage, derivation and invalidation. That distinction supports separate receipts for “expert answered,” “card admitted,” “card rendered,” and “obligation verified.” None implies the next. [Build Systems à la Carte](https://www.microsoft.com/en-us/research/publication/build-systems-la-carte/) provides a useful framework for separating scheduling and reuse. Applying those ideas to advice validity is a proposal, not a theorem establishing this system's correctness.

## 9. One contract across sessions and campaigns

The session route should carry typed consultation artifacts beside the prompt and bind them to stable work-step IDs. Preserve no-plan and failed-compile fallback behavior, while adding explicit delivery in successful step compiles. Step-specific expert selection must retain consultation and critique roles, not only the list of agent names.

The campaign route should attach cards to existing task/phase/dependency identities and preserve them across save/resume, task splitting and rolling refinement. Reuse `ContextFrom` and artifact references where their contracts fit, but do not assume that a generic result string provides validated advice scope or freshness. Task effects and write sets must remain unchanged by an advisory artifact.

Older chat delegation modes and the shared session path should converge on this carrier and route decision rather than consult twice or apply different escalation thresholds silently. Migration should make one component own each consultation request and one component own delivery. Keep existing behavior available as an explicit comparison/control during rollout, not as an invisible parallel source of repeated calls.

## 10. Full costs and failure modes

For S steps and A cards, naive applicability scanning is O(SA), plus graph closure and rendering. Indexing by task, domain, symbol and read dependency can reduce candidate work, but indexes have build, memory and invalidation costs. Mangle join cardinality can grow badly if every card is joined to every symbol; restrict the candidate domain with sound measured relations before expensive joins. Do not sacrifice required-card completeness for a faster top-K.

Account for all of: expert initialization, KB retrieval, embeddings, relevance judgments, card extraction/validation, routing, expert and stronger-model calls, provider queueing, canceled speculation, JIT/fixpoint work, final prompt tokens, prefix-cache behavior, tool execution, build/test work, retries, storage, migrations and human correction. A short local decision can save an expensive consultation, but a wrong route can cost many repair turns. Card atomization itself may require another model pass; prefer structured generation in an already-needed consultation where it is reliable, while retaining the raw answer and explicit parsing failure.

Primary failure modes include stale cards, wrong plan binding, omitted caveats, unsupported claim-to-obligation promotion, duplicate consultations, unbounded mandatory growth, unsupported tools, conflict suppression, contaminated cross-workspace reuse, and treating rendered advice as verified behavior. A powerful expert can still be wrong. An empty retrieval result can mean unavailable source rather than no useful knowledge. A budget rejection must not look like successful delivery.

[Lost in the Middle](https://arxiv.org/abs/2307.03172v3) studies sensitivity to information position on the evaluated models and tasks. It motivates testing placement and targeted context; it does not establish that this deck eliminates long-context problems in current coding models. Keep ordering and compression effects as measured hypotheses.

## 11. Evaluation: prove the route before claiming the savings

No evaluation below has been run. Compare current prose append, whole-answer copying into every step, static per-step attachments, plan-bound cards, and plan-bound cards with adaptive consultation. Add direct-KB-only and always-consult controls. A “stronger model everywhere” baseline can illuminate the quality/cost frontier if separately authorized and fully charged.

At the socket level, use deterministic consultation responses and actual parsing, plan binding, JIT, budget fitting and request construction. Test unique guidance markers, but score their appropriate presence and absence, not universal inclusion. Include reordered/split/merged/canceled nodes, dependency-edge cards, stale assumptions, failed/empty consultations, unavailable KBs, true/false conflicts, missing tools, constrained budgets and save/resume. Capture final provider requests, not only candidate lists.

At the complete-system level, use matched multi-file coding tasks and campaigns with known acceptance conditions. Include tasks solved by stored knowledge, tasks needing fresh synthesis, tasks needing observation rather than speculation, and tasks whose cross-domain integration constraints are easy to miss. Hold source, model versions, effective tools, allowed spend and verification requirements fixed. Report verified task success, constraint adherence, false completion, unnecessary edits, human review, latency, total tokens and monetary/compute cost including failed runs.

For routing, report both false-cheap decisions and unnecessary escalation. Stratify by domain, effect, ambiguity and dependency coupling; do not optimize only an aggregate average that hides security or integration failures. Compare a deterministic baseline with any later learned router on held-out task families. An offline oracle route is a diagnostic upper bound, not a deployable competitor. Calibration and thresholds must be revalidated when models, prompts or tool contracts change.

Scale steps and experts independently, varying how many unique questions and evidence revisions exist. Measure consultation coalescing, queue time, stale-prefetch waste, peak retained artifacts, policy fact counts and invalidation work. Compare equal assurance, not just equal token budget: skipping necessary expert work must not count as efficiency. Conversely, more expert calls or more delivered cards are not success metrics by themselves.

## 12. Implementation sequence and acceptance

First preserve typed consultation results and actual-delivery receipts across successful step recompilation. Then add stable plan bindings and direct-KB reuse under existing JIT constraints. Next add explicit contradiction/freshness dispositions and role reconciliation across chat, session and campaign. Only after these contracts are observable should adaptive stronger-model escalation and speculative prefetch be evaluated.

Each owning architecture specification should be updated before implementation: experts for route/result contracts, prompt for card admission and cache identity, session for per-step delivery, campaign for durable bindings and revision behavior. This document does not authorize code changes or runtime evaluation.

Acceptance requires that the system can explain why a card was delivered or omitted, why a consultation was purchased or avoided, which evidence makes reuse valid, and how an admitted obligation was actually discharged. The destination is not a giant prompt assembled more cleverly. It is a plan that carries the right reasoning to the exact decision that needs it, preserves uncertainty when reasoning is insufficient, and spends model calls where they can change the outcome.
