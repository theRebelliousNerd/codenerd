# Portable recurse deliberation and a behavioral crucible

**Status: design proposal grounded in static source inspection.** Public CodeNERD revision `fc32417e3498357be16144f07ed472bcbe4ebbab`, inspected 2026-11-03. No tests, builds, models, gates, experiments or application paths were executed. “Implemented” below means the cited source contains the behavior, not that this review demonstrated it at runtime. This document uses public CodeNERD material and public primary references only.

## Decision in brief

Do not add a competing agent framework. Recurse already has an ideation partner and a requirements interrogator in its bounded mission path. Strengthen that path into a **portable, evidence-producing design funnel**: clarify the intended behavior; challenge a small set of alternatives; admit a versioned acceptance contract; perform only authorized experiments; retain a change through the existing campaign and ratchet; report exactly which requirement the evidence supports.

“Any codebase” must mean **a common protocol with explicit capability limits and safe degradation**, not a promise that every language, build system, dependency graph, test runner, operating system or production environment is already understood. A repository with unknown tooling can receive useful read-only design work. It must not acquire a verified-improvement claim merely because the generic loop can visit its root.

The genuinely incremental work is: an adapter capability manifest, a typed bridge from candidate success prose to reviewed behavioral evidence, language-independent oracle protection, and a bounded crucible with explicit exit states. The existing mission innovation already proposes requirement IDs, behavioral evidence, holdouts and adapter reuse; this design makes the portability and interrogation contracts concrete rather than presenting those ideas as new. [Existing direction][existing-innovation] [Existing evaluation direction][existing-eval]

## 1. What exists, and what does not yet follow

| Capability | Source-grounded status | Consequence |
|---|---|---|
| Recurse mission integration | **Implemented:** CLI builds a mission and invokes the shared recurse driver; production mission construction wires the normal shard manager, read tools and restricted consultations. | Extend this integration instead of introducing another scheduler. [CLI][cli] [Factory][factory] |
| Divergence and challenge | **Implemented:** proposer runs first, challenger assesses its candidates; later rounds receive the previous round. Candidate and consultation counts are bounded. | The proposed “Socratic” roles already exist. [Roles][roles] [Round][round] |
| Mission design boundary | **Implemented:** the selected attempt, mission and vision enter a design-only thread in `propose` mode; scope and check come from the existing attempt. | Deliberation is downstream of ordinary node/finding/angle selection, not presently an unrestricted strategic work generator. [Preparation][mission-prepare] |
| Structured objections and stopping | **Implemented:** critical/important items distinguish data, design and values; policy controls round, no-progress and tool bounds; values questions have a constrained outcome. | Keep these controls. Broaden evidence contracts, not conversational freedom. [Validation][validate] [Policy][deliberation-policy] [Values][values] |
| Executable candidate acceptance | **Partial:** `SuccessTest` is a required string; validation checks its presence, not whether it is an executable or adequate behavioral oracle. | A persuasive success sentence is a design artifact, not a test result. [Candidate][roles] [Validation][validate] |
| Mission evidence | **Implemented but intentionally narrow:** receipts and policy can support a gate deficit; they do not certify arbitrary mission or North Star satisfaction. | A separate, explicit requirement-to-check join is needed for stronger claims. [Receipt][receipt] [Mission policy][mission-policy] |
| Generality | **Partial:** workspace-derived graphs and gates support several ecosystems, while metrics and automatic oracle inputs have narrower coverage. | Publish a capability profile, including unknowns, per invocation. [Graph][graph] [Detection][detect-tools] [Oracle][oracle] |
| Portable crucible contract | **Proposed here:** typed requirements, reviewed checks, bounded discriminating observations and independent acceptance. | Its effectiveness and operational cost remain **unverified**. |

The mission architecture explicitly leaves reviewed behavioral contracts and stronger evidence validity for further work. Nothing here should erase its as-built controls or resurrect historical audit findings as current defects. [Mission scope][mission-spec] [Explicit future work][mission-future]

## 2. Define the portable boundary honestly

### 2.1 Current seams that matter

**Graph discovery is not complete semantic dependency analysis.** Current code uses Go package discovery, Python import scanning, relative JS/TS import resolution and Rust path dependencies. Cycles collapse; a workspace with no recognized nodes becomes a single `.` node. This is useful scheduling structure, not proof that dynamic imports, generated code, service calls, schema consumers or external build dependencies are covered. Cross-cutting visits do not repair an unknown dependency model by themselves. [Graph][graph]

**Project commands already provide an important escape hatch.** `nerd.md` commands override detected gates of the same kind; explicit extra gates remain distinguishable. Detection recognizes Go, Python, JS/TS and Rust markers, and unknown command programs are not automatically tied to one language. Preserve this project authority rather than hard-coding a universal toolchain. [Declaration precedence][detect] [Toolchain detection][detect-tools]

**A gate verdict and a requirement verdict are different.** Gates preserve argv, scope, declared successful exit codes and environment. Cancellation or failure to start produces an unverified result. Detected pytest exit 5 is separately marked as no tests, although it is not a gate failure. A successful command therefore needs interpretation under its declared coverage contract before it can support a behavioral requirement. [Gate contract][gate-type] [Runner][gate-run]

**The ratchet is deliberately not “everything is green.”** Improvement retention requires an observed test-gate verdict and a positive metric, together with other nonregression and scope conditions. The host marks testing from a completed test verdict, not only a passing one; policy permits some pre-existing failures when they do not worsen by its finding-count criterion. That is useful incremental repair semantics, but must not be described as universal behavioral correctness. The new requirement oracle is additional evidence, not a rewrite of this baseline contract. [Observed verdict][ratchet-host] [Ratchet policy][ratchet-policy]

**Metrics are proxies with specific adapters.** Test counting uses naming/regular-expression conventions for four language families; source-line counting recognizes a fixed extension set; coverage parsing recognizes Go and pytest-cov output. More tests, fewer lines or increased reported coverage can guide an angle without establishing the user's intended capability. An unavailable metric must stay unavailable, not become zero or an excuse to invent a positive improvement. [Metrics][metrics] [Coverage][coverage]

**Automatic oracle protection is uneven across languages.** `VerificationInputs` currently hashes admitted existing Go tests, `nerd.md`, Go module/work files and files under `testdata/`. It is not a general collector for Python tests, JS test files, Rust inline tests, their manifests/lockfiles, or arbitrary verification scripts. Mission preparation saves this map; pre-write admission and the post-attempt ratchet consult it alongside protected paths and selected scope. This is an automatic-coverage limit, **not** evidence that all foreign test files are writable: configured protection, project restrictions and ordinary host permissions still apply. [Collector][oracle] [Preparation][mission-prepare] [Pre-write][mission-write] [Post-attempt][ratchet-host]

Default mission protection also contains CodeNERD-specific implementation paths alongside general control files. A portable adapter must supplement protection with the target repository's actual verifier boundary; it must never replace or silently narrow host-owned guards. [Defaults][mission-config]

### 2.2 Proposed capability manifest

At admission, create a host-produced, versioned manifest:

- Workspace canonical identity, revision/snapshot, membership rules, adapter identity and schema version.
- Recognized languages and build roots; graph source and limitations; unresolved imports and generated/external boundaries.
- Declared gate IDs, exact command descriptors, working directory, environment identity, toolchain identity, expected exits, timeout, output parsing and coverage domain.
- Verification-input inventory: existing tests, fixtures, snapshots/goldens, runner configuration, manifests, lockfiles, scripts, generated expectations and dependencies. Mark inventory completeness as known, partial or unknown.
- Available metrics and their definitions; distinguish static counts, executed cases and behavioral assertions.
- Available isolation and rollback facilities; subprocess/network/credential permissions; artifact retention and output limits.

Capabilities should be separate fields, not one misleading “supported” Boolean. A repository may have reliable build gates but an unknown import graph; reliable graph order but no runnable test gate; or executable tests whose fixture dependency closure is incomplete.

Use three user-facing dispositions: **design-only**, **executable with stated limits**, and **requirement-evaluable under named contracts**. No adapter is approved by extension recognition alone. Unknown ecosystems can use owner-declared commands and scoped contracts without pretending to have language-specific analysis.

## 3. A bounded Socratic workflow

The following is a **proposed strengthening of the existing path**, not a claim that new fields or phases are already implemented.

### Stage A — Frame the selected opportunity

Input is the already selected recurse attempt plus owner mission, relevant requirement versions, capability manifest and prior receipts. Preserve the distinction between an observed defect, an improvement hypothesis and a preference.

Produce a compact problem frame:

1. Who needs what observable behavior, under which conditions?
2. What is the current evidence of a deficit, including contrary evidence?
3. What must remain unchanged, and what effects are explicitly forbidden?
4. What would falsify the proposed benefit?
5. Which unknowns can be resolved by existing read-only probes, and which need owner choice or new execution permission?

For an open-ended “make this better,” propose a small set of measurable interpretations; do not silently turn taste into a requirement. The design can return a useful unresolved frame without starting writes.

### Stage B — Generate alternatives, including not changing code

Reuse the proposer. Require genuinely different mechanisms rather than paraphrases: a narrow repair, improved evidence or tooling, a simpler existing route, and where appropriate deferral/no change. The current divergent-round minimum of two candidates can be met with a useful baseline alternative, not fabricated diversity. [Round][round]

Each candidate should carry: requirement/version references; causal hypothesis; source-backed rationale; smallest scope; expected benefits and harms; assumptions; compatibility constraints; proposed discriminating observations; estimated full cost; and a clear rejection condition. Unknown estimates remain marked unknown.

The proposer may suggest new tests or tools. It may not grant permissions, edit the authoritative requirement, decide its own acceptance thresholds, or replace the selected production check.

### Stage C — Interrogate without becoming an endless interview

Reuse `requirements_interrogator`. For each candidate, identify the strongest concrete counterexample and the smallest unresolved obligation. Require coverage of normal behavior, boundary cases, error handling, cancellation, restart, compatibility, data handling and deployment only where relevant—not a mandatory thousand-question checklist.

Keep current data/design/values categories, but attach **authority and effect requirements as separate typed blockers** rather than misclassifying permission as a solvable fact:

| Objection | Bounded response |
|---|---|
| Missing factual evidence | An allowed read probe with a named expected discriminant; otherwise mark unresolved. |
| Design contradiction | Revise or reject the candidate; connect the revision to the objection key. |
| User value trade-off | Ask one concrete choice with viable alternatives, then stop the mission pending an explicit decision. |
| Missing authority | Request the specific permission through the existing host channel; no model answer can satisfy it. |
| Missing adapter/oracle | Return a design-only result or an authorized adapter-development task; never substitute confidence. |

The existing fingerprint excludes reworded briefs and uses critical item identities and probe observations. Preserve that anti-churn behavior. A strengthened progress record should additionally identify which objection was resolved and by what new evidence; renaming an objection or producing longer prose is not progress. [Fingerprint and probes][progress]

Keep the mission's existing values stop. Do not import generic permissive-mode “assume the recommendation” behavior into unattended mission requirements. Source preparation uses `propose`, and mission policy stops on `ask_once`; a new run must still satisfy source, budget and permission admission. [Preparation][mission-prepare] [Mission stop][mission-policy]

### Stage D — Freeze a reviewed behavioral contract

Before a mutating experiment, promote selected success prose into an owner- or policy-admitted contract. Suggested schema:

| Record | Required meaning |
|---|---|
| Requirement | Stable ID/version, authoritative source, intended behavior and explicit exclusions. |
| Scope | Production entry path, supported inputs/environment, relevant dependencies and permitted effects. |
| Oracle | Check IDs and versions, fixtures, expected observations, forbidden outcomes, and interpretation of no tests/skips/timeouts. |
| Validity | Source/requirement/toolchain/configuration identities and conservative invalidation rules. |
| Comparison | Baseline result, candidate result, negative controls and nonregression obligations. |
| Authority | Owner admission or reviewed policy basis; no grant inferred from tool output or candidate prose. |
| Outcome | Supported, contradicted, inconclusive, stale, unavailable or waived, with a reason and retained evidence references. |

“Waived” records an authorized exception; it is not “supported.” A newly authored test is candidate evidence until independently reviewed/admitted. If repairing an existing test or acceptance script is the actual objective, give that oracle change a separate reviewed contract rather than letting an implementation attempt weaken its own judge.

NASA's requirements guidance motivates uniquely traceable, unambiguous and verifiable requirements; its verification matrix connects requirements to verification approaches. These are useful engineering principles, not a claim that this lightweight protocol implements NASA's full process. [Requirement guidance](https://www.nasa.gov/reference/appendix-c-how-to-write-a-good-requirement/) [Verification matrix](https://www.nasa.gov/reference/system-engineering-handbook-appendix/)

## 4. The crucible: falsification, not a vote

A **crucible** is the bounded stage that tries to break the candidate's argument and, when authorized, its implementation. It does not mean adding many agents until they agree.

### Design crucible — read-only

Use the existing deliberation evidence envelope to compare the candidate against the frozen constraints, dependency boundaries and one or more concrete counterexamples. Produce a claim/evidence table: what is observed, inferred, contradicted or unknown. Source citations must identify actual material; a citation-shaped string is not proof.

The present broker obtains tools from policy and executes through registry guards, truncating oversized evidence with an explicit marker. Preserve its no-shell/no-write role boundary. If a critical observation is truncated or not available, narrow the read or leave the claim unknown. Repository comments, documents and tool output remain evidence, not instructions that can widen authority. [Read-tool broker][read-tools]

### Experimental crucible — separately authorized

An executable experiment belongs to the normal host/campaign path, **not** to an expanded deliberation read tool. The host validates the admitted command, workspace, permissions, fixture dependencies, time/output limits and isolation before execution. Preserve argv/environment/exit semantics end to end; do not reduce a command to a display string. [Attempt descriptor][attempt] [Gate execution][gate-run]

Treat build and test commands as code execution. For example, Cargo documents that build scripts are compiled and run before package compilation. A command named “check” or “test” is not automatically a read-only or harmless probe. Its filesystem, network, subprocess and credential effects require the corresponding host authorization. [Cargo build scripts](https://doc.rust-lang.org/cargo/reference/build-scripts.html)

Prefer the cheapest observation that distinguishes alternatives:

1. Inspect an existing production path or counterexample.
2. Use an already admitted fixture/check.
3. If needed, propose a minimal reproducer and obtain the required admission.
4. Execute baseline and candidate under comparable conditions.
5. Apply the existing ratchet **and** the requirement contract; record both verdicts separately.

For a genuine new capability there may be no meaningful failing baseline test. Record “behavior absent” through an admitted baseline observation; do not manufacture a test failure or claim an unchanged baseline proves improvement.

For stochastic performance, specify workload, warmup, sample plan and decision criterion before results. For service/API effects, use explicitly authorized disposable resources or return unavailable. A local source patch is not proof that a deployed binary now exhibits the claimed behavior.

### Exit states and handoff

- **Ready design:** critical design obligations resolved; no effects authorized by this label.
- **Admitted experiment:** host permission, scope, oracle and budget are valid.
- **Supported change:** required current observations satisfy the admitted contract, and ordinary adoption safeguards pass.
- **Rejected:** counterexample or regression contradicts the candidate.
- **Inconclusive:** evidence is insufficient, flaky or incompatible.
- **Blocked:** missing authority, environment, adapter capability or owner preference.
- **Stopped:** budget, cancellation, no progress or freshness boundary reached.

Every state carries next evidence needed, not an automatic retry. Rejected and inconclusive attempts remain in cost and outcome denominators. Resume revalidates authority, source and contract identity; it never blindly replays an uncertain external effect.

## 5. Portable oracle ownership and acceptance

The most concrete generality improvement is an adapter interface for **verification inputs and evidence interpretation**, not just graph discovery.

An adapter proposes an inventory; host-owned logic canonicalizes paths, applies workspace membership, preserves immutable identity and protects the admitted inventory. Existing protection is the minimum. For Python include test discovery/configuration and fixtures; for JS/TS include scripts, runner configuration, snapshots and lockfiles; for Rust account for inline tests, integration tests, Cargo configuration and build scripts. These examples are an initial checklist, not a claim of complete dependency closure.

Where production and tests share a file, whole-file immutability may prohibit legitimate work. The safe first implementation can conservatively defer such attempts or require a separately reviewed oracle-change operation. A future structural projection may pin test-bearing regions, but only after the adapter demonstrates that surrounding macros, imports and helper code cannot silently change their meaning. Do not promise region-level soundness based on a textual diff.

Conservative whole-snapshot freshness is a defensible starting point. Narrow dependency-based reuse only when the adapter can explain why an unrelated change does not affect a result. Changes to the runner, environment, fixtures or expected outputs can invalidate evidence even if production source is unchanged.

Maintain two ledgers:

- **Work/adoption ledger:** selected scope, writes, ordinary gate comparison, compensation/adoption and commit identity.
- **Requirement-evidence ledger:** contract version, admitted oracle, source/runtime identity, actual observations and applicability.

The second references the first; it does not overwrite it. A kept incremental repair may leave a strategic requirement unknown. A supported local requirement may still be insufficient for deployment approval. Current mission receipts already caution against collapsing these meanings. [Receipt][receipt]

## 6. Mangle, host execution, LLMs and JEV

**Mangle remains the executive.** Reviewed rules determine admission, outstanding blockers, priority, required evidence, stopping and acceptance. Host code measures identities and observations, validates external data, enforces permissions, executes and persists. LLMs propose interpretations, alternatives and counterexamples. None of these roles should quietly become a second policy engine. Current recurse and mission policy already separate measured inputs from decisions. [Ratchet][ratchet-policy] [Mission rules][mission-policy]

A policy proof explains why rules derived an outcome from supplied facts; it does not establish that model-authored premises are true or that a test measures the right requirement. Keep observation provenance and trust class on facts. Avoid inferring “all evidence present” merely from the absence of a failure fact when the observation inventory is incomplete.

**JEV is optional evidence, not an oracle of correctness or permission.** Public architecture describes typed probabilistic judgments consumed by rules, including missing/unsettled dispositions. No source inspected here establishes a general JEV crucible acceptance path. A proposed use might flag ambiguous requirements or unsupported claims for review; it must have a defined consumer, a bounded state representation and calibration on the actual task families. Missing/unsettled judgments cannot become approval, and a high probability cannot substitute for the admitted behavioral witness. [JEV architecture][jev] [Evidence policy][jev-policy]

Do not multiply near-identical judges and treat agreement as independent evidence. Self-Refine demonstrates benefits from iterative feedback on its studied tasks; separate research finds intrinsic reasoning self-correction can fail without external feedback. These results motivate an empirical comparison and external witnesses, not a universal claim that deliberation always helps or never helps. [Self-Refine](https://arxiv.org/abs/2303.17651) [Self-correction study](https://arxiv.org/abs/2310.01798)

## 7. Budgets, costs and operational controls

Current defaults already bound mission cycles, elapsed time, estimated spend, no-progress cycles, serial concurrency and context bytes. Routing separately bounds rounds, evidence calls, candidates and context. Keep those controls; the existing architecture notes that estimated spend is not a hard provider invoice ceiling and that a paid phase can overshoot before the next admission. [Mission defaults][mission-config] [Routing defaults][routing-config] [Budget semantics][mission-spec]

The proposed contract should account for **all** stages: discovery, proposal, interrogation, specialist consultations, probes, command execution, verification, artifact retention and recovery. Two role invocations per round are only the starting cost; their evidence loops and consultations add work. A candidate cap is not a complete cap on tokens, wall time or subprocess work. [Round][round] [Role loop][agent-bounds]

Before an expensive experiment reserve estimated verification and cleanup capacity. Report unknown pricing/usage as unknown and preserve current fail-closed mission behavior. Do not spend the entire remaining budget writing a patch that cannot be checked or safely reconciled.

Track model calls/tokens and estimated spend; host CPU/wall time; tool/gate invocations; changed/hashed bytes; retained artifacts; owner questions; and full cost per independently supported change. Compare gains against orchestration and maintenance cost. Hashing more oracle inputs and retaining richer receipts have real I/O/storage costs; bounded references, retention policy and explicit expired-evidence states are preferable to unbounded transcript accumulation.

Initially keep serial adoption. Parallel ideas may share read-only evidence only with clear snapshot identity; concurrent mutable experiments need isolated workspaces, separate usage/effect ownership and verification of the assembled result. They are not prerequisites for this design.

## 8. Fair evaluation plan — all unrun

No outcome numbers are claimed. Evaluate in two layers.

### Deterministic contract and production-path fixtures

Exercise both CLI and chat entry paths, real adapters, normal host guards and policy consumers—not only constructed receipt objects.

| Fixture family | Expected contract |
|---|---|
| Clear requirement with existing reproducer | Short path; no unnecessary interview; current witness retained. |
| Ambiguous preference | One concrete values fork; no writes while unresolved. |
| Unsupported language with declared check | Useful generic flow with declared limits; no invented graph/coverage. |
| No test gate; no tests collected; skipped or truncated output | Distinct, truthful evidence dispositions; no behavioral pass by default. |
| Pre-existing failing tests | Preserve current ratchet semantics; do not misreport full green or requirement support. |
| Python/JS test or fixture edited to make a check pass | Protected/rejected under the new admitted oracle inventory. |
| Rust inline test or macro dependency changed | Conservative refusal or separate oracle review; no claimed structural guarantee. |
| Custom script, environment or lockfile changed | Evidence becomes stale or is explicitly re-admitted. |
| Wrong node/working directory or same argv with different environment | No receipt reuse across incompatible identities. |
| Permission absent; malicious instructions in repository evidence | No authority expansion; blocked/read-only result. |
| Candidate merely rewords objections | No-progress bound applies. |
| Source/vision changes during deliberation | Freshness recheck blocks stale execution. |
| Cancellation, provider uncertainty, crash around effects/commit | Existing ownership and recovery safeguards preserved; no blind retry. |
| JEV absent, unsettled, wrong or overconfident | No unsupported acceptance and no silent permission grant. |

A fixture passing is evidence for its boundary, not proof of universal portability.

### Matched task study

Use a prespecified, repository-disjoint sample across Go, Python, JS/TS, Rust, mixed-language and at least one unsupported ecosystem with owner-declared gates. Include custom tooling, unconventional layouts, generated artifacts and initially broken repositories. Separate development tasks from frozen evaluation tasks.

Compare incrementally: (A) ordinary recurse, (B) current mission deliberation, (C) mission plus typed contracts/capability reporting, (D) the complete bounded crucible. Keep model/provider identity, task scope, permitted effects and independently administered acceptance fixed where feasible. Use matched full-cost budgets; report both quality at equal budget and cost to reach the same accepted endpoint. If JEV or a different challenger model is studied, vary it separately rather than confounding every treatment.

Primary outcomes: independently supported requirement changes per attempt; false acceptance; regressions; owner-question burden; unavailable/inconclusive fraction; total cost and elapsed time; recovery correctness. Report failed and abandoned attempts, capability failures and budget exhaustion. Reviewers should not infer treatment from polished explanation text when judging behavior. Repeated tasks are not independent merely because the random seed differs; cluster uncertainty by repository/task family and disclose sample limitations.

Include adversarial negative controls: more counted tests with no relevant assertion, a faster but incorrect candidate, a green command checking the wrong target, an oracle weakened by the patch, and a convincing but unsupported mission narrative. A negative or cost-neutral result is a valid outcome and should prevent automatic rollout.

## 9. Adoption sequence and definition of done

1. **Reconcile owning specifications.** Extend existing mission, verification, workspace and deliberation contracts. Record current capability limits and terminology before adding code.
2. **Observation-only manifest.** Emit capability and oracle-inventory reports without changing scheduling or adoption. Review foreign repositories and unsupported cases.
3. **Contract bridge.** Bind one small requirement family to existing host receipts; keep unsupported cases design-only.
4. **Portable oracle protection.** Add adapters conservatively; preserve existing guard ownership and require explicit review for oracle changes.
5. **Bounded crucible.** Admit experiments through normal permission/execution paths, then run the fair comparison above under separate authorization.
6. **Expand only on evidence.** More languages, metrics, optional judgments or concurrency require their own capability and evaluation evidence.

Done means an owner can inspect one adopted attempt and answer: what behavior was intended, which alternatives were considered, what could have disproved the choice, what authority permitted effects, what actually ran, which immutable evidence supports the result, what remains unknown, and what would invalidate it. It does **not** mean the agents agreed, the mission sounded complete, or the loop ran indefinitely.

## Pinned public source references

[cli]: https://github.com/theRebelliousNerd/codenerd/blob/fc32417e3498357be16144f07ed472bcbe4ebbab/cmd/nerd/cmd_campaign_recurse.go#L151-L263
[factory]: https://github.com/theRebelliousNerd/codenerd/blob/fc32417e3498357be16144f07ed472bcbe4ebbab/cmd/nerd/cmd_campaign_mission.go#L40-L68
[mission-prepare]: https://github.com/theRebelliousNerd/codenerd/blob/fc32417e3498357be16144f07ed472bcbe4ebbab/internal/campaign/recurse_mission.go#L174-L316
[receipt]: https://github.com/theRebelliousNerd/codenerd/blob/fc32417e3498357be16144f07ed472bcbe4ebbab/internal/campaign/recurse_mission.go#L68-L97
[roles]: https://github.com/theRebelliousNerd/codenerd/blob/fc32417e3498357be16144f07ed472bcbe4ebbab/internal/deliberation/types.go#L14-L50
[validate]: https://github.com/theRebelliousNerd/codenerd/blob/fc32417e3498357be16144f07ed472bcbe4ebbab/internal/deliberation/types.go#L132-L169
[round]: https://github.com/theRebelliousNerd/codenerd/blob/fc32417e3498357be16144f07ed472bcbe4ebbab/internal/deliberation/round.go#L10-L72
[read-tools]: https://github.com/theRebelliousNerd/codenerd/blob/fc32417e3498357be16144f07ed472bcbe4ebbab/internal/deliberation/read_tools.go#L14-L81
[agent-bounds]: https://github.com/theRebelliousNerd/codenerd/blob/fc32417e3498357be16144f07ed472bcbe4ebbab/internal/deliberation/read_tools.go#L84-L180
[progress]: https://github.com/theRebelliousNerd/codenerd/blob/fc32417e3498357be16144f07ed472bcbe4ebbab/internal/deliberation/outcome.go#L118-L181
[values]: https://github.com/theRebelliousNerd/codenerd/blob/fc32417e3498357be16144f07ed472bcbe4ebbab/internal/deliberation/outcome.go#L16-L74
[deliberation-policy]: https://github.com/theRebelliousNerd/codenerd/blob/fc32417e3498357be16144f07ed472bcbe4ebbab/internal/core/defaults/policy/deliberation.mg#L18-L74
[mission-policy]: https://github.com/theRebelliousNerd/codenerd/blob/fc32417e3498357be16144f07ed472bcbe4ebbab/internal/core/defaults/policy/mission.mg#L8-L32
[mission-config]: https://github.com/theRebelliousNerd/codenerd/blob/fc32417e3498357be16144f07ed472bcbe4ebbab/internal/config/mission.go#L10-L94
[routing-config]: https://github.com/theRebelliousNerd/codenerd/blob/fc32417e3498357be16144f07ed472bcbe4ebbab/internal/config/routing.go#L13-L32
[mission-write]: https://github.com/theRebelliousNerd/codenerd/blob/fc32417e3498357be16144f07ed472bcbe4ebbab/internal/campaign/recurse_mission_scope.go#L13-L74
[ratchet-host]: https://github.com/theRebelliousNerd/codenerd/blob/fc32417e3498357be16144f07ed472bcbe4ebbab/internal/campaign/recurse_cycle.go#L1082-L1180
[ratchet-policy]: https://github.com/theRebelliousNerd/codenerd/blob/fc32417e3498357be16144f07ed472bcbe4ebbab/internal/core/defaults/policy/recurse.mg#L258-L329
[attempt]: https://github.com/theRebelliousNerd/codenerd/blob/fc32417e3498357be16144f07ed472bcbe4ebbab/internal/campaign/recurse_cycle.go#L43-L75
[graph]: https://github.com/theRebelliousNerd/codenerd/blob/fc32417e3498357be16144f07ed472bcbe4ebbab/internal/campaign/recurse_workspace.go#L24-L91
[detect]: https://github.com/theRebelliousNerd/codenerd/blob/fc32417e3498357be16144f07ed472bcbe4ebbab/internal/gates/detect.go#L15-L91
[detect-tools]: https://github.com/theRebelliousNerd/codenerd/blob/fc32417e3498357be16144f07ed472bcbe4ebbab/internal/gates/detect.go#L93-L213
[gate-type]: https://github.com/theRebelliousNerd/codenerd/blob/fc32417e3498357be16144f07ed472bcbe4ebbab/internal/gates/gates.go#L60-L113
[gate-run]: https://github.com/theRebelliousNerd/codenerd/blob/fc32417e3498357be16144f07ed472bcbe4ebbab/internal/gates/run.go#L15-L97
[metrics]: https://github.com/theRebelliousNerd/codenerd/blob/fc32417e3498357be16144f07ed472bcbe4ebbab/internal/gates/metrics.go#L17-L86
[coverage]: https://github.com/theRebelliousNerd/codenerd/blob/fc32417e3498357be16144f07ed472bcbe4ebbab/internal/gates/metrics.go#L171-L204
[oracle]: https://github.com/theRebelliousNerd/codenerd/blob/fc32417e3498357be16144f07ed472bcbe4ebbab/internal/evidence/change.go#L379-L423
[mission-spec]: https://github.com/theRebelliousNerd/codenerd/blob/fc32417e3498357be16144f07ed472bcbe4ebbab/Docs/architecture/campaign/MISSION-IMPROVEMENT.md#L86-L128
[mission-future]: https://github.com/theRebelliousNerd/codenerd/blob/fc32417e3498357be16144f07ed472bcbe4ebbab/Docs/architecture/campaign/MISSION-IMPROVEMENT.md#L280-L284
[existing-innovation]: https://github.com/theRebelliousNerd/codenerd/blob/fc32417e3498357be16144f07ed472bcbe4ebbab/Docs/innovations/mission-driven-forever-improvement.md#L52-L133
[existing-eval]: https://github.com/theRebelliousNerd/codenerd/blob/fc32417e3498357be16144f07ed472bcbe4ebbab/Docs/innovations/mission-driven-forever-improvement.md#L187-L241
[jev]: https://github.com/theRebelliousNerd/codenerd/blob/fc32417e3498357be16144f07ed472bcbe4ebbab/Docs/architecture/jev-turn-judgments.md#L14-L74
[jev-policy]: https://github.com/theRebelliousNerd/codenerd/blob/fc32417e3498357be16144f07ed472bcbe4ebbab/internal/core/defaults/policy/jev_evidence.mg#L1-L52

