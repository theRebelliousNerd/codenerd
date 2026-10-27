---
doc-class: north-star
subsystem: northstar
implementation-status: target-state
last-verified: 2026-10-27
verified-against: fb1d74d0f13b7d0d647c968ae3c1d830eae9b555
supersedes: []
---

# CodeNERD Northstar: trustworthy software delivery, not convincing activity

> **Proposed replacement vision, not a claim of shipped capability.** This document contains a reviewable JSON import candidate and its corresponding Mangle fact projection. It does not edit the live Northstar, execute CodeNERD, run tests, or change policy.
>
> **The short version:** Make ambitious software changes trustworthy: understand the whole system, wire the change into reality, prove what can be proved, expose what cannot, and spend less total work getting there.

The `north-star` document class and `target-state` implementation status follow the repository's [architecture documentation vocabulary](https://github.com/theRebelliousNerd/codenerd/blob/fb1d74d0f13b7d0d647c968ae3c1d830eae9b555/Docs/journeys/09-architecture-doc-standard.md#L24-L40). This document proposes a final-state contract; it does not supersede or rewrite the existing package specification.

## 1. What changes from the wizard draft

The original ambition is right. The weak point is its representation: a long vision string, one broad persona, one capability, and no risk, requirement, constraint, or relationship graph. A richer Northstar should make the ambition inspectable and actionable without pretending that English sentences are executable verification rules.

This proposal preserves the essential bet:

- **Models create; the kernel governs.** Creativity and synthesis belong to models. Executive decisions should follow from typed facts and rules, not the model's feeling that it is finished.
- **Integration is part of correctness.** A function that works in isolation but is never registered, configured, called, persisted, or shut down correctly is not a delivered feature.
- **The north star supplies continuous direction.** It is not a checklist that declares the whole project finished. Individual tasks can finish against scoped obligations while the project keeps reducing the distance to the final behavior.
- **Quality comes first.** Token efficiency is a constraint on delivering accepted behavior, not permission to remove checks, hide retries, or turn uncertainty into confidence.
- **Autonomy earns trust.** Long-running work must retain authority, recoverability, fresh evidence, bounded resources, and meaningful progress. Cancellation and approval boundaries are not failures of autonomy.

The content below is deliberately more operational than a slogan but less detailed than an implementation backlog. Stable IDs preserve traceability as descriptions evolve.

## 2. Strong language, with engineering meaning

| Original aspiration | Meaning retained here | Misreading rejected |
|---|---|---|
| Runs unattended for weeks | Productive, authorized work survives interruptions and sessions with evidence, budgets, checkpoints, and takeover intact. | Ignore cancellation, approvals, cost limits, stalls, or external safety constraints. |
| Every claim is a citation | Material acceptance claims identify applicable evidence, scope, revision, and uncertainty. | A citation or a derivation guarantees the underlying premise is true. |
| Closes 100% of SWE-bench | Keep full closure of a specified benchmark protocol as an explicit moonshot. | Claim a result, universal correctness, or a timeless score without a version, split, budget, and denominator. |
| Never greps around | Compile targeted, revision-aware context instead of repeatedly searching without a model of the codebase. | Ban necessary source inspection, freshness checks, or lawful fallback when an index is incomplete. |
| The harness forces | Required obligations cannot silently become optional; acceptance must be distinguished from attempted work. | A vision sentence automatically wires every enforcement path or authorizes deployment. |
| Improves itself | Propose, isolate, evaluate, review, and reversibly adopt improvements. | Rewrite safety gates, erase regressions, or promise mathematical non-regression on all possible tasks. |

The fact store should preserve **attempts as attempts**, observations as observations, hypotheses as hypotheses, and accepted claims with their supporting evidence. Erasing attempted work would damage debugging and cost accounting; promoting it to success would damage trust.

## 3. Compatibility and authority: read before copying

This is the **on-disk import shape**, not the lowercase canonical Go model and not a wizard-session snapshot:

- Top-level keys are `Mission`, `Problem`, `Vision`, `Personas`, `Capabilities`, `Risks`, `Requirements`, and `Constraints`.
- Nested keys remain lowercase. Explicit capability, risk, and requirement IDs avoid positional renumbering.
- `serves` names an existing persona; `supports` and `addresses` name existing capability and risk IDs. All references below resolve within this candidate.
- Roadmap values use `now`, `6mo`, `1yr`, `3yr`, and `moonshot`. These are planning horizons, not delivery promises or status fields.
- The candidate omits timestamps rather than fabricating history. It omits wizard UI state, `ResearchDocs`, `ExtractedFacts`, and requirement `source`: those are not retained as canonical vision content by `ToVision`.
- Human-readable persona names are part of their generated fact identity. Renaming a persona changes its ID; changing a constraint's position changes its generated `constraint_N` ID.

The current bridge declares SQLite the durable record and JSON the import/export surface. `SyncVisionAuthority` reconciles competing content using file mtime versus store update time; it is not a transactional merge of concurrent edits. The Mangle file is a projection, not the place to maintain this candidate. Equal JSON/store content does not guarantee an existing manually edited Mangle file is regenerated. Preserve the current authoritative vision, review a semantic diff, and use the supported update path when adoption is authorized.

**Source anchors at the inspected revision:**

Clickable pinned sources: [JSON authority and bridge](https://github.com/theRebelliousNerd/codenerd/blob/fb1d74d0f13b7d0d647c968ae3c1d830eae9b555/internal/northstar/bridge.go#L82-L144), [vision-to-facts mapping](https://github.com/theRebelliousNerd/codenerd/blob/fb1d74d0f13b7d0d647c968ae3c1d830eae9b555/internal/northstar/types.go#L44-L188), [alignment guardian](https://github.com/theRebelliousNerd/codenerd/blob/fb1d74d0f13b7d0d647c968ae3c1d830eae9b555/internal/northstar/guardian.go#L331-L388), and [relationship reasoning policy](https://github.com/theRebelliousNerd/codenerd/blob/fb1d74d0f13b7d0d647c968ae3c1d830eae9b555/internal/core/defaults/policy/prompt_northstar.mg#L29-L84).
- `internal/northstar/bridge.go:88`, `WizardDocument`; `:148`, `ToVision`; `:282`, `LoadVisionJSON`; `:354`, `RenderVisionMangle`; `:436`, `SyncVisionAuthority`.
- `internal/northstar/types.go:44`, `PersonaFactID`; `:56`, `ToFacts`; `:160`, `MitigationStrategyAtom`; `:220`, `parsePriority`; `:235`, `parseRiskImpact`.
- Repository direction: [root guidance](https://github.com/theRebelliousNerd/codenerd/blob/fb1d74d0f13b7d0d647c968ae3c1d830eae9b555/agents.md), especially “The Vision” and “Mangle Guardrails.” The older package vision is intent, not proof of current wiring.

The exact line anchors should be reviewed against the linked revision before adoption; source provenance is not a substitute for a runtime import check. No import or evaluation was executed for this document.

## 4. Proposed JSON import candidate

The mission states the product's purpose. The vision describes the finished behavior. Capabilities say what it can do; requirements say what must hold; risks explain why the requirements matter; constraints express boundaries that progress must not cross. Relationships give the existing coverage rules something useful to reason about.

The JSON is the authoring candidate. The fact block later is its reviewable projection, not a second independent source of truth.

```json
{
  "Mission": "Make ambitious software changes trustworthy: combine model creativity with logic-governed execution to turn developer intent into correctly integrated, evidence-backed behavior, while preserving the project's purpose, human authority, and engineering understanding.",
  "Problem": "Coding agents can produce convincing local changes without understanding the whole execution path, preserving requirements, or proving the requested behavior. Developers inherit almost-right code, unwired features, stale context, fragile tests, hidden costs, and confident completion claims that require expensive review and repair.",
  "Vision": "CodeNERD is the coding harness engineers trust with consequential, long-horizon work. It understands a repository as a living system of purposes, contracts, dependencies, state, and evidence; derives executive decisions through its Mangle kernel; and gives models the focused context needed to solve the next real obligation. A feature is complete only to the extent that its intended behavior is integrated into authorized production paths and supported by current evidence. Work can continue across sessions while progress, resources, and permissions permit, without losing the plot or confusing activity with achievement. The system improves its tools and methods through reviewable, reversible changes and retained regression evidence. Quality comes first; token efficiency means less total work per accepted outcome, never cheaper-looking failure.",
  "Personas": [
    {
      "name": "Backend engineers",
      "pain_points": [
        "Almost-right code passes local checks but fails real behavior.",
        "New features exist but are not reachable through production wiring.",
        "Stale context and flaky tests cause repeated debugging and review."
      ],
      "needs": [
        "Trace a task from intent through callers, contracts, state, and effects.",
        "Receive minimal reviewable changes with fresh verification and explicit uncertainty."
      ]
    },
    {
      "name": "Maintainers and reviewers",
      "pain_points": [
        "Agent changes hide regressions, unsupported claims, and architectural drift.",
        "Large patches and opaque reasoning transfer verification work to humans."
      ],
      "needs": [
        "Understand why a change is needed and which evidence supports each acceptance claim.",
        "Preserve project constraints, ownership, and recoverable decision history."
      ]
    },
    {
      "name": "Platform and security engineers",
      "pain_points": [
        "Unbounded autonomy, unsafe tool access, and partial failures threaten production.",
        "Deployment configuration, migrations, observability, and rollback are omitted."
      ],
      "needs": [
        "Enforce least privilege and explicit approval boundaries.",
        "Know what executed, what changed, and whether recovery remains possible."
      ]
    },
    {
      "name": "Engineering leads",
      "pain_points": [
        "Token savings conceal retries, tool costs, failed tasks, and human rework.",
        "Benchmark scores do not establish dependable production delivery."
      ],
      "needs": [
        "Compare total cost at matched quality and authorized operating conditions.",
        "See capability gaps, delivery risk, and evidence-based progress toward the vision."
      ]
    }
  ],
  "Capabilities": [
    {
      "id": "cap_intent",
      "description": "Translate a software request into an explicit behavior contract, preserve unresolved intent as uncertainty, and trace the contract to project purpose and affected subsystems.",
      "timeline": "now",
      "priority": "critical",
      "serves": [
        "Backend engineers",
        "Maintainers and reviewers"
      ]
    },
    {
      "id": "cap_context",
      "description": "Maintain revision-aware codebase understanding and compile focused, recoverable working context from CodeDOM, facts, domain knowledge, retrieval, and history.",
      "timeline": "now",
      "priority": "critical",
      "serves": [
        "Backend engineers",
        "Maintainers and reviewers"
      ]
    },
    {
      "id": "cap_wiring",
      "description": "Deliver changes through real execution paths, including registration, dependency injection, interfaces, configuration, persistence, errors, and lifecycle behavior where applicable.",
      "timeline": "now",
      "priority": "critical",
      "serves": [
        "Backend engineers",
        "Platform and security engineers"
      ]
    },
    {
      "id": "cap_evidence",
      "description": "Derive completion from scoped behavioral evidence and unresolved obligations, not model confidence, compilation alone, or an alignment score.",
      "timeline": "now",
      "priority": "critical",
      "serves": [
        "Backend engineers",
        "Maintainers and reviewers"
      ]
    },
    {
      "id": "cap_control",
      "description": "Mediate actions through typed tools and policy; distinguish requested, permitted, executed, validated, and accepted outcomes while preserving cancellation and operator authority.",
      "timeline": "now",
      "priority": "critical",
      "serves": [
        "Platform and security engineers",
        "Maintainers and reviewers"
      ]
    },
    {
      "id": "cap_efficiency",
      "description": "Optimize total tokens, compute, latency, and human rework per accepted outcome while preserving accuracy and reporting all retries, retrieval, tool, and verification costs.",
      "timeline": "now",
      "priority": "high",
      "serves": [
        "Engineering leads",
        "Backend engineers"
      ]
    },
    {
      "id": "cap_coordination",
      "description": "Coordinate specialist work through explicit obligations, dependency and ownership contracts, bounded fan-out, reusable evidence, and conflict-aware integration.",
      "timeline": "6mo",
      "priority": "high",
      "serves": [
        "Backend engineers",
        "Engineering leads"
      ]
    },
    {
      "id": "cap_operations",
      "description": "Treat deployment readiness, migration compatibility, observability, rollback, and operational failure handling as explicit task obligations when the requested scope requires them.",
      "timeline": "6mo",
      "priority": "high",
      "serves": [
        "Platform and security engineers",
        "Maintainers and reviewers"
      ]
    },
    {
      "id": "cap_continuity",
      "description": "Resume long-horizon campaigns without losing intent, authority, evidence freshness, or ownership; detect stalls and repeated failures and recover within explicit resource budgets.",
      "timeline": "1yr",
      "priority": "high",
      "serves": [
        "Engineering leads",
        "Platform and security engineers"
      ]
    },
    {
      "id": "cap_improvement",
      "description": "Improve CodeNERD's tools and policies through authorized, isolated, reviewable changes with independent gates, held-out evaluations, and reversible rollout; never self-widen safety or permission boundaries.",
      "timeline": "1yr",
      "priority": "high",
      "serves": [
        "Maintainers and reviewers",
        "Engineering leads"
      ]
    },
    {
      "id": "cap_transfer",
      "description": "Transfer verified engineering knowledge across projects without transferring private data, stale assumptions, unauthorized permissions, or unearned confidence.",
      "timeline": "3yr",
      "priority": "medium",
      "serves": [
        "Backend engineers",
        "Platform and security engineers"
      ]
    },
    {
      "id": "cap_frontier",
      "description": "Pursue complete resolution of a named SWE-bench protocol and dependable weeks-long authorized campaigns as separate research targets, demonstrating generalization beyond public benchmarks and reporting failures honestly.",
      "timeline": "moonshot",
      "priority": "medium",
      "serves": [
        "Engineering leads",
        "Maintainers and reviewers"
      ]
    }
  ],
  "Risks": [
    {
      "id": "risk_false_done",
      "description": "Plausible explanations or narrow checks are promoted into unsupported completion claims.",
      "likelihood": "high",
      "impact": "high",
      "mitigation": "Require revision-bound evidence for each acceptance obligation and preserve unknown outcomes."
    },
    {
      "id": "risk_unwired",
      "description": "A feature is implemented locally but omitted from production entry paths or lifecycle integration.",
      "likelihood": "high",
      "impact": "high",
      "mitigation": "Trace entry-to-effect paths and require integration witnesses for affected boundaries."
    },
    {
      "id": "risk_stale",
      "description": "Stale, partial, or poisoned context steers decisions as if it were current trusted evidence.",
      "likelihood": "high",
      "impact": "high",
      "mitigation": "Track revision, provenance, coverage, and trust; invalidate dependent evidence after changes."
    },
    {
      "id": "risk_authority",
      "description": "Autonomous actions exceed scope, leak data, or weaken the controls that constrain the agent.",
      "likelihood": "medium",
      "impact": "high",
      "mitigation": "Enforce host-owned permissions and explicit approval independently of model output."
    },
    {
      "id": "risk_liveness",
      "description": "Repeated failures, uncontrolled fan-out, or retained state exhaust resources without meaningful progress.",
      "likelihood": "high",
      "impact": "high",
      "mitigation": "Budget complete work, detect repeated failure signatures, checkpoint, and surface repair or stop obligations."
    },
    {
      "id": "risk_metric",
      "description": "Benchmark gaming or incomplete accounting makes cheaper or more accurate behavior appear without real improvement.",
      "likelihood": "high",
      "impact": "high",
      "mitigation": "Use frozen held-out protocols, matched budgets, full denominators, and complete cost ledgers."
    },
    {
      "id": "risk_regression",
      "description": "Self-improvement or cross-project reuse degrades existing behavior or imports unsafe assumptions.",
      "likelihood": "medium",
      "impact": "high",
      "mitigation": "Isolate changes, retain regression gates, evaluate held-out tasks, and support rollback."
    },
    {
      "id": "risk_opacity",
      "description": "Automation erodes developer understanding and makes review, diagnosis, and takeover harder.",
      "likelihood": "medium",
      "impact": "medium",
      "mitigation": "Expose concise rationale, evidence, residual uncertainty, and recoverable decision history."
    }
  ],
  "Requirements": [
    {
      "id": "req_contract",
      "type": "functional",
      "description": "Before consequential changes, record observable acceptance criteria, scope, assumptions, affected boundaries, and unresolved questions; distinguish inferred intent from user requirements.",
      "priority": "must_have",
      "supports": [
        "cap_intent",
        "cap_wiring"
      ],
      "addresses": [
        "risk_false_done",
        "risk_unwired"
      ]
    },
    {
      "id": "req_executive",
      "type": "functional",
      "description": "Executive decisions about next obligations, context selection, permission, delegation, and acceptance must be explainable as kernel derivations over typed evidence; Go supplies observations, effects, and bounded adapters.",
      "priority": "must_have",
      "supports": [
        "cap_control",
        "cap_coordination",
        "cap_context"
      ],
      "addresses": [
        "risk_authority",
        "risk_opacity"
      ]
    },
    {
      "id": "req_context",
      "type": "functional",
      "description": "Every context item that supports a decision must retain origin, revision or freshness scope, trust, and retrieval coverage; source edits invalidate dependent conclusions and evicted context remains recoverable.",
      "priority": "must_have",
      "supports": [
        "cap_context",
        "cap_transfer"
      ],
      "addresses": [
        "risk_stale"
      ]
    },
    {
      "id": "req_integration",
      "type": "functional",
      "description": "For each affected feature, produce an applicability-qualified entry-to-effect wiring review and evidence for relevant registration, configuration, contracts, state, errors, lifecycle, and user-visible behavior.",
      "priority": "must_have",
      "supports": [
        "cap_wiring",
        "cap_operations"
      ],
      "addresses": [
        "risk_unwired"
      ]
    },
    {
      "id": "req_acceptance",
      "type": "functional",
      "description": "No required obligation becomes accepted without current applicable evidence; skipped, cancelled, unavailable, malformed, or inconclusive checks remain explicit non-passing states. An authorized scoped waiver is recorded as waived, never relabeled passed.",
      "priority": "must_have",
      "supports": [
        "cap_evidence",
        "cap_control"
      ],
      "addresses": [
        "risk_false_done"
      ]
    },
    {
      "id": "req_tests",
      "type": "functional",
      "description": "Select risk-appropriate behavioral, regression, integration, and negative checks from the contract; inspect whether checks exercise intended behavior rather than merely asserting implementation details.",
      "priority": "must_have",
      "supports": [
        "cap_evidence",
        "cap_wiring",
        "cap_improvement"
      ],
      "addresses": [
        "risk_false_done",
        "risk_regression"
      ]
    },
    {
      "id": "req_effects",
      "type": "functional",
      "description": "Record action permission, execution, effects, validation, and acceptance separately; failed validation must not erase a mutation, and retries must respect idempotency or explicit recovery.",
      "priority": "must_have",
      "supports": [
        "cap_control",
        "cap_operations",
        "cap_continuity"
      ],
      "addresses": [
        "risk_authority",
        "risk_liveness"
      ]
    },
    {
      "id": "req_cost",
      "type": "non_functional",
      "description": "Account for every model call, cache read/write class, retry, discarded result, retrieval/index maintenance, tool invocation, verification run, coordination overhead, wall time, and human repair in system-level comparisons.",
      "priority": "must_have",
      "supports": [
        "cap_efficiency",
        "cap_frontier"
      ],
      "addresses": [
        "risk_metric",
        "risk_liveness"
      ]
    },
    {
      "id": "req_progress",
      "type": "non_functional",
      "description": "Continue only within authorized scope and budgets; derive progress and stall evidence from changes in obligations and results, retain resumable checkpoints, and stop promptly for cancellation or authority loss.",
      "priority": "must_have",
      "supports": [
        "cap_continuity",
        "cap_coordination"
      ],
      "addresses": [
        "risk_liveness",
        "risk_authority"
      ]
    },
    {
      "id": "req_parallel",
      "type": "functional",
      "description": "Assign explicit ownership and interfaces to parallel work; validate shared evidence before reuse, surface disagreements, and integrate against a current revision without overwriting concurrent work.",
      "priority": "should_have",
      "supports": [
        "cap_coordination",
        "cap_wiring"
      ],
      "addresses": [
        "risk_stale",
        "risk_unwired"
      ]
    },
    {
      "id": "req_operations",
      "type": "functional",
      "description": "When deployment is in scope, make migrations, compatibility, configuration, secrets boundaries, telemetry, rollout, and rollback individually accepted or explicitly inapplicable; never equate merge with deployment.",
      "priority": "must_have",
      "supports": [
        "cap_operations",
        "cap_wiring"
      ],
      "addresses": [
        "risk_unwired",
        "risk_authority"
      ]
    },
    {
      "id": "req_improvement",
      "type": "constraint",
      "description": "Self-improvement may propose changes but may not modify its own safety-gate or permission logic; authorized changes require independent evaluation, retained regression coverage, and rollback.",
      "priority": "must_have",
      "supports": [
        "cap_improvement",
        "cap_transfer"
      ],
      "addresses": [
        "risk_regression",
        "risk_authority"
      ]
    },
    {
      "id": "req_report",
      "type": "functional",
      "description": "Report delivered behavior, evidence scope, unverified claims, residual risks, actual costs, and required human action in a concise reviewable handoff; preserve details for inspection.",
      "priority": "must_have",
      "supports": [
        "cap_evidence",
        "cap_efficiency",
        "cap_intent"
      ],
      "addresses": [
        "risk_opacity",
        "risk_false_done",
        "risk_metric"
      ]
    },
    {
      "id": "req_evaluate",
      "type": "non_functional",
      "description": "Evaluate complete systems on frozen task sets and production-like paths with disclosed model, tools, budget, environment, failure denominator, human intervention, and benchmark version; no superiority claim without matching evidence.",
      "priority": "must_have",
      "supports": [
        "cap_frontier",
        "cap_efficiency",
        "cap_improvement"
      ],
      "addresses": [
        "risk_metric",
        "risk_regression"
      ]
    }
  ],
  "Constraints": [
    "Human intent, authorization, cancellation, and data boundaries outrank autonomous progress; vision text grants no permissions.",
    "The model is the creative center and the Mangle kernel is the executive; deterministic derivation is only as trustworthy as its inputs, rules, and coverage.",
    "Quality and capability come before token minimization; never lower acceptance standards, hide work, or skip required verification to improve an efficiency metric.",
    "Use typed policy-mediated operations, not unrestricted model shell access or ad-hoc tool bypasses.",
    "Use JIT prompt atoms and revision-aware world-model context; targeted source inspection is a legitimate freshness fallback, not a failure of the vision.",
    "Treat observations, attempts, hypotheses, derivations, and acceptance claims as distinct; preserve provenance and do not convert missing evidence into success.",
    "Northstar alignment assesses purpose and constraints; it does not replace action permission, behavioral verification, or human approval.",
    "Preserve concurrent work and make consequential changes reviewable and recoverable; deployment and destructive actions require their own authority.",
    "Keep per-request context, retained state, retries, and parallel work bounded while allowing authorized productive tasks to continue.",
    "Public benchmark closure and weeks-long autonomy are targets, not present results; no benchmark score implies universal correctness."
  ]
}
```

## 5. Corresponding Mangle fact set

This is a **documentation projection** of the JSON above using the inspected `ToFacts` mapping, including its relationships and mitigation strategy atoms. It is not evidence that the facts have been loaded or the proposals implemented. Do not copy these facts over the generated live file.

Strings remain quoted; timeline, type, likelihood, and mitigation strategy values are Mangle name constants; priorities and impacts are integers. Every clause ends with a period. The host's existing schema supplies declarations. The current mapping expresses critical/must-have as 100, high/should-have as 80, medium as 50, and low/nice-to-have as 20; risk impact uses 100/50/20. These are ordinal policy inputs, **not probabilities or measured utilities**.

Mitigation atoms use the current text-derived slug plus eight hexadecimal SHA-256 characters, with readable mitigation text emitted separately. These compact identities are practical identifiers, not mathematical collision-free proofs. Editing a mitigation changes its strategy atom.

The candidate JSON and Mangle strings are ASCII-only to avoid conflating their escape languages. JSON uses escapes such as `\u0041`; the inspected Mangle grammar uses a braced Unicode form such as `\u{0041}`. A JSON string serializer is therefore not a general Mangle serializer. This particular candidate avoids that mismatch; it does not establish serializer equivalence for arbitrary text.

CodeNERD pins `codeberg.org/TauCeti/mangle-go v0.7.0` in [the inspected `go.mod`, line 9](https://github.com/theRebelliousNerd/codenerd/blob/fb1d74d0f13b7d0d647c968ae3c1d830eae9b555/go.mod#L9). The [exact corresponding upstream grammar](https://github.com/google/mangle/blob/7cd2543ac1817f8b92d42efe3c6ef82dededbeeb/parse/gen/Mangle.g4) is the version-pinned syntax reference used here, rather than a generic Datalog tutorial or an older pseudo-version document. The grammar covers syntax; the host's schemas and `ToFacts` mapping determine these predicates' types and meanings. A source-linked grammar is not evidence that this candidate has passed the installed parser, that a deployed binary matches the dependency file, or that a policy consumer enforces each requirement. Those are distinct future verification obligations; none was executed for this document.

```mangle
northstar_mission("global", "Make ambitious software changes trustworthy: combine model creativity with logic-governed execution to turn developer intent into correctly integrated, evidence-backed behavior, while preserving the project's purpose, human authority, and engineering understanding.").
northstar_problem("global", "Coding agents can produce convincing local changes without understanding the whole execution path, preserving requirements, or proving the requested behavior. Developers inherit almost-right code, unwired features, stale context, fragile tests, hidden costs, and confident completion claims that require expensive review and repair.").
northstar_vision("global", "CodeNERD is the coding harness engineers trust with consequential, long-horizon work. It understands a repository as a living system of purposes, contracts, dependencies, state, and evidence; derives executive decisions through its Mangle kernel; and gives models the focused context needed to solve the next real obligation. A feature is complete only to the extent that its intended behavior is integrated into authorized production paths and supported by current evidence. Work can continue across sessions while progress, resources, and permissions permit, without losing the plot or confusing activity with achievement. The system improves its tools and methods through reviewable, reversible changes and retained regression evidence. Quality comes first; token efficiency means less total work per accepted outcome, never cheaper-looking failure.").
northstar_persona("persona_Backend engineers", "Backend engineers").
northstar_pain_point("persona_Backend engineers", "Almost-right code passes local checks but fails real behavior.").
northstar_pain_point("persona_Backend engineers", "New features exist but are not reachable through production wiring.").
northstar_pain_point("persona_Backend engineers", "Stale context and flaky tests cause repeated debugging and review.").
northstar_need("persona_Backend engineers", "Trace a task from intent through callers, contracts, state, and effects.").
northstar_need("persona_Backend engineers", "Receive minimal reviewable changes with fresh verification and explicit uncertainty.").
northstar_persona("persona_Maintainers and reviewers", "Maintainers and reviewers").
northstar_pain_point("persona_Maintainers and reviewers", "Agent changes hide regressions, unsupported claims, and architectural drift.").
northstar_pain_point("persona_Maintainers and reviewers", "Large patches and opaque reasoning transfer verification work to humans.").
northstar_need("persona_Maintainers and reviewers", "Understand why a change is needed and which evidence supports each acceptance claim.").
northstar_need("persona_Maintainers and reviewers", "Preserve project constraints, ownership, and recoverable decision history.").
northstar_persona("persona_Platform and security engineers", "Platform and security engineers").
northstar_pain_point("persona_Platform and security engineers", "Unbounded autonomy, unsafe tool access, and partial failures threaten production.").
northstar_pain_point("persona_Platform and security engineers", "Deployment configuration, migrations, observability, and rollback are omitted.").
northstar_need("persona_Platform and security engineers", "Enforce least privilege and explicit approval boundaries.").
northstar_need("persona_Platform and security engineers", "Know what executed, what changed, and whether recovery remains possible.").
northstar_persona("persona_Engineering leads", "Engineering leads").
northstar_pain_point("persona_Engineering leads", "Token savings conceal retries, tool costs, failed tasks, and human rework.").
northstar_pain_point("persona_Engineering leads", "Benchmark scores do not establish dependable production delivery.").
northstar_need("persona_Engineering leads", "Compare total cost at matched quality and authorized operating conditions.").
northstar_need("persona_Engineering leads", "See capability gaps, delivery risk, and evidence-based progress toward the vision.").
northstar_capability("cap_intent", "Translate a software request into an explicit behavior contract, preserve unresolved intent as uncertainty, and trace the contract to project purpose and affected subsystems.", /now, 100).
northstar_capability("cap_context", "Maintain revision-aware codebase understanding and compile focused, recoverable working context from CodeDOM, facts, domain knowledge, retrieval, and history.", /now, 100).
northstar_capability("cap_wiring", "Deliver changes through real execution paths, including registration, dependency injection, interfaces, configuration, persistence, errors, and lifecycle behavior where applicable.", /now, 100).
northstar_capability("cap_evidence", "Derive completion from scoped behavioral evidence and unresolved obligations, not model confidence, compilation alone, or an alignment score.", /now, 100).
northstar_capability("cap_control", "Mediate actions through typed tools and policy; distinguish requested, permitted, executed, validated, and accepted outcomes while preserving cancellation and operator authority.", /now, 100).
northstar_capability("cap_efficiency", "Optimize total tokens, compute, latency, and human rework per accepted outcome while preserving accuracy and reporting all retries, retrieval, tool, and verification costs.", /now, 80).
northstar_capability("cap_coordination", "Coordinate specialist work through explicit obligations, dependency and ownership contracts, bounded fan-out, reusable evidence, and conflict-aware integration.", /6mo, 80).
northstar_capability("cap_operations", "Treat deployment readiness, migration compatibility, observability, rollback, and operational failure handling as explicit task obligations when the requested scope requires them.", /6mo, 80).
northstar_capability("cap_continuity", "Resume long-horizon campaigns without losing intent, authority, evidence freshness, or ownership; detect stalls and repeated failures and recover within explicit resource budgets.", /1yr, 80).
northstar_capability("cap_improvement", "Improve CodeNERD's tools and policies through authorized, isolated, reviewable changes with independent gates, held-out evaluations, and reversible rollout; never self-widen safety or permission boundaries.", /1yr, 80).
northstar_capability("cap_transfer", "Transfer verified engineering knowledge across projects without transferring private data, stale assumptions, unauthorized permissions, or unearned confidence.", /3yr, 50).
northstar_capability("cap_frontier", "Pursue complete resolution of a named SWE-bench protocol and dependable weeks-long authorized campaigns as separate research targets, demonstrating generalization beyond public benchmarks and reporting failures honestly.", /moonshot, 50).
northstar_serves("cap_intent", "persona_Backend engineers").
northstar_serves("cap_intent", "persona_Maintainers and reviewers").
northstar_serves("cap_context", "persona_Backend engineers").
northstar_serves("cap_context", "persona_Maintainers and reviewers").
northstar_serves("cap_wiring", "persona_Backend engineers").
northstar_serves("cap_wiring", "persona_Platform and security engineers").
northstar_serves("cap_evidence", "persona_Backend engineers").
northstar_serves("cap_evidence", "persona_Maintainers and reviewers").
northstar_serves("cap_control", "persona_Platform and security engineers").
northstar_serves("cap_control", "persona_Maintainers and reviewers").
northstar_serves("cap_efficiency", "persona_Engineering leads").
northstar_serves("cap_efficiency", "persona_Backend engineers").
northstar_serves("cap_coordination", "persona_Backend engineers").
northstar_serves("cap_coordination", "persona_Engineering leads").
northstar_serves("cap_operations", "persona_Platform and security engineers").
northstar_serves("cap_operations", "persona_Maintainers and reviewers").
northstar_serves("cap_continuity", "persona_Engineering leads").
northstar_serves("cap_continuity", "persona_Platform and security engineers").
northstar_serves("cap_improvement", "persona_Maintainers and reviewers").
northstar_serves("cap_improvement", "persona_Engineering leads").
northstar_serves("cap_transfer", "persona_Backend engineers").
northstar_serves("cap_transfer", "persona_Platform and security engineers").
northstar_serves("cap_frontier", "persona_Engineering leads").
northstar_serves("cap_frontier", "persona_Maintainers and reviewers").
northstar_risk("risk_false_done", "Plausible explanations or narrow checks are promoted into unsupported completion claims.", /high, 100).
northstar_mitigation("risk_false_done", /mit_require_revision_bound_evidence_for_each_b427919f).
northstar_mitigation_text("risk_false_done", "Require revision-bound evidence for each acceptance obligation and preserve unknown outcomes.").
northstar_risk("risk_unwired", "A feature is implemented locally but omitted from production entry paths or lifecycle integration.", /high, 100).
northstar_mitigation("risk_unwired", /mit_trace_entry_to_effect_paths_and_require_247ca580).
northstar_mitigation_text("risk_unwired", "Trace entry-to-effect paths and require integration witnesses for affected boundaries.").
northstar_risk("risk_stale", "Stale, partial, or poisoned context steers decisions as if it were current trusted evidence.", /high, 100).
northstar_mitigation("risk_stale", /mit_track_revision_provenance_coverage_and_t_f7864902).
northstar_mitigation_text("risk_stale", "Track revision, provenance, coverage, and trust; invalidate dependent evidence after changes.").
northstar_risk("risk_authority", "Autonomous actions exceed scope, leak data, or weaken the controls that constrain the agent.", /medium, 100).
northstar_mitigation("risk_authority", /mit_enforce_host_owned_permissions_and_expli_67198a5a).
northstar_mitigation_text("risk_authority", "Enforce host-owned permissions and explicit approval independently of model output.").
northstar_risk("risk_liveness", "Repeated failures, uncontrolled fan-out, or retained state exhaust resources without meaningful progress.", /high, 100).
northstar_mitigation("risk_liveness", /mit_budget_complete_work_detect_repeated_fai_466e90ff).
northstar_mitigation_text("risk_liveness", "Budget complete work, detect repeated failure signatures, checkpoint, and surface repair or stop obligations.").
northstar_risk("risk_metric", "Benchmark gaming or incomplete accounting makes cheaper or more accurate behavior appear without real improvement.", /high, 100).
northstar_mitigation("risk_metric", /mit_use_frozen_held_out_protocols_matched_bu_8edd8e89).
northstar_mitigation_text("risk_metric", "Use frozen held-out protocols, matched budgets, full denominators, and complete cost ledgers.").
northstar_risk("risk_regression", "Self-improvement or cross-project reuse degrades existing behavior or imports unsafe assumptions.", /medium, 100).
northstar_mitigation("risk_regression", /mit_isolate_changes_retain_regression_gates_9cdde574).
northstar_mitigation_text("risk_regression", "Isolate changes, retain regression gates, evaluate held-out tasks, and support rollback.").
northstar_risk("risk_opacity", "Automation erodes developer understanding and makes review, diagnosis, and takeover harder.", /medium, 50).
northstar_mitigation("risk_opacity", /mit_expose_concise_rationale_evidence_residu_f98531eb).
northstar_mitigation_text("risk_opacity", "Expose concise rationale, evidence, residual uncertainty, and recoverable decision history.").
northstar_requirement("req_contract", /functional, "Before consequential changes, record observable acceptance criteria, scope, assumptions, affected boundaries, and unresolved questions; distinguish inferred intent from user requirements.", 100).
northstar_requirement("req_executive", /functional, "Executive decisions about next obligations, context selection, permission, delegation, and acceptance must be explainable as kernel derivations over typed evidence; Go supplies observations, effects, and bounded adapters.", 100).
northstar_requirement("req_context", /functional, "Every context item that supports a decision must retain origin, revision or freshness scope, trust, and retrieval coverage; source edits invalidate dependent conclusions and evicted context remains recoverable.", 100).
northstar_requirement("req_integration", /functional, "For each affected feature, produce an applicability-qualified entry-to-effect wiring review and evidence for relevant registration, configuration, contracts, state, errors, lifecycle, and user-visible behavior.", 100).
northstar_requirement("req_acceptance", /functional, "No required obligation becomes accepted without current applicable evidence; skipped, cancelled, unavailable, malformed, or inconclusive checks remain explicit non-passing states. An authorized scoped waiver is recorded as waived, never relabeled passed.", 100).
northstar_requirement("req_tests", /functional, "Select risk-appropriate behavioral, regression, integration, and negative checks from the contract; inspect whether checks exercise intended behavior rather than merely asserting implementation details.", 100).
northstar_requirement("req_effects", /functional, "Record action permission, execution, effects, validation, and acceptance separately; failed validation must not erase a mutation, and retries must respect idempotency or explicit recovery.", 100).
northstar_requirement("req_cost", /non_functional, "Account for every model call, cache read/write class, retry, discarded result, retrieval/index maintenance, tool invocation, verification run, coordination overhead, wall time, and human repair in system-level comparisons.", 100).
northstar_requirement("req_progress", /non_functional, "Continue only within authorized scope and budgets; derive progress and stall evidence from changes in obligations and results, retain resumable checkpoints, and stop promptly for cancellation or authority loss.", 100).
northstar_requirement("req_parallel", /functional, "Assign explicit ownership and interfaces to parallel work; validate shared evidence before reuse, surface disagreements, and integrate against a current revision without overwriting concurrent work.", 80).
northstar_requirement("req_operations", /functional, "When deployment is in scope, make migrations, compatibility, configuration, secrets boundaries, telemetry, rollout, and rollback individually accepted or explicitly inapplicable; never equate merge with deployment.", 100).
northstar_requirement("req_improvement", /constraint, "Self-improvement may propose changes but may not modify its own safety-gate or permission logic; authorized changes require independent evaluation, retained regression coverage, and rollback.", 100).
northstar_requirement("req_report", /functional, "Report delivered behavior, evidence scope, unverified claims, residual risks, actual costs, and required human action in a concise reviewable handoff; preserve details for inspection.", 100).
northstar_requirement("req_evaluate", /non_functional, "Evaluate complete systems on frozen task sets and production-like paths with disclosed model, tools, budget, environment, failure denominator, human intervention, and benchmark version; no superiority claim without matching evidence.", 100).
northstar_supports("req_contract", "cap_intent").
northstar_supports("req_contract", "cap_wiring").
northstar_addresses("req_contract", "risk_false_done").
northstar_addresses("req_contract", "risk_unwired").
northstar_supports("req_executive", "cap_control").
northstar_supports("req_executive", "cap_coordination").
northstar_supports("req_executive", "cap_context").
northstar_addresses("req_executive", "risk_authority").
northstar_addresses("req_executive", "risk_opacity").
northstar_supports("req_context", "cap_context").
northstar_supports("req_context", "cap_transfer").
northstar_addresses("req_context", "risk_stale").
northstar_supports("req_integration", "cap_wiring").
northstar_supports("req_integration", "cap_operations").
northstar_addresses("req_integration", "risk_unwired").
northstar_supports("req_acceptance", "cap_evidence").
northstar_supports("req_acceptance", "cap_control").
northstar_addresses("req_acceptance", "risk_false_done").
northstar_supports("req_tests", "cap_evidence").
northstar_supports("req_tests", "cap_wiring").
northstar_supports("req_tests", "cap_improvement").
northstar_addresses("req_tests", "risk_false_done").
northstar_addresses("req_tests", "risk_regression").
northstar_supports("req_effects", "cap_control").
northstar_supports("req_effects", "cap_operations").
northstar_supports("req_effects", "cap_continuity").
northstar_addresses("req_effects", "risk_authority").
northstar_addresses("req_effects", "risk_liveness").
northstar_supports("req_cost", "cap_efficiency").
northstar_supports("req_cost", "cap_frontier").
northstar_addresses("req_cost", "risk_metric").
northstar_addresses("req_cost", "risk_liveness").
northstar_supports("req_progress", "cap_continuity").
northstar_supports("req_progress", "cap_coordination").
northstar_addresses("req_progress", "risk_liveness").
northstar_addresses("req_progress", "risk_authority").
northstar_supports("req_parallel", "cap_coordination").
northstar_supports("req_parallel", "cap_wiring").
northstar_addresses("req_parallel", "risk_stale").
northstar_addresses("req_parallel", "risk_unwired").
northstar_supports("req_operations", "cap_operations").
northstar_supports("req_operations", "cap_wiring").
northstar_addresses("req_operations", "risk_unwired").
northstar_addresses("req_operations", "risk_authority").
northstar_supports("req_improvement", "cap_improvement").
northstar_supports("req_improvement", "cap_transfer").
northstar_addresses("req_improvement", "risk_regression").
northstar_addresses("req_improvement", "risk_authority").
northstar_supports("req_report", "cap_evidence").
northstar_supports("req_report", "cap_efficiency").
northstar_supports("req_report", "cap_intent").
northstar_addresses("req_report", "risk_opacity").
northstar_addresses("req_report", "risk_false_done").
northstar_addresses("req_report", "risk_metric").
northstar_supports("req_evaluate", "cap_frontier").
northstar_supports("req_evaluate", "cap_efficiency").
northstar_supports("req_evaluate", "cap_improvement").
northstar_addresses("req_evaluate", "risk_metric").
northstar_addresses("req_evaluate", "risk_regression").
northstar_constraint("constraint_0", "Human intent, authorization, cancellation, and data boundaries outrank autonomous progress; vision text grants no permissions.").
northstar_constraint("constraint_1", "The model is the creative center and the Mangle kernel is the executive; deterministic derivation is only as trustworthy as its inputs, rules, and coverage.").
northstar_constraint("constraint_2", "Quality and capability come before token minimization; never lower acceptance standards, hide work, or skip required verification to improve an efficiency metric.").
northstar_constraint("constraint_3", "Use typed policy-mediated operations, not unrestricted model shell access or ad-hoc tool bypasses.").
northstar_constraint("constraint_4", "Use JIT prompt atoms and revision-aware world-model context; targeted source inspection is a legitimate freshness fallback, not a failure of the vision.").
northstar_constraint("constraint_5", "Treat observations, attempts, hypotheses, derivations, and acceptance claims as distinct; preserve provenance and do not convert missing evidence into success.").
northstar_constraint("constraint_6", "Northstar alignment assesses purpose and constraints; it does not replace action permission, behavioral verification, or human approval.").
northstar_constraint("constraint_7", "Preserve concurrent work and make consequential changes reviewable and recoverable; deployment and destructive actions require their own authority.").
northstar_constraint("constraint_8", "Keep per-request context, retained state, retries, and parallel work bounded while allowing authorized productive tasks to continue.").
northstar_constraint("constraint_9", "Public benchmark closure and weeks-long autonomy are targets, not present results; no benchmark score implies universal correctness.").
northstar_defined().
```

## 6. Why these are better facts, not just more prose

### A useful graph instead of an inspirational blob

The supported graph is:

`persona <- serves - capability <- supports - requirement - addresses -> risk`

Every proposed capability serves a real persona; every requirement supports at least one capability; every listed risk has both mitigation prose and an addressing requirement. This gives the existing relationship-based rules a connected planning structure.

But **graph coverage is not semantic coverage**. A requirement linked to a risk does not prove it mitigates the risk. A capability linked to a persona does not prove it satisfies every need. A mitigation string's existence does not prove it works. Human review and behavioral evidence still matter.

### Explicit distinctions the wizard draft lacked

1. **Purpose is not permission.** “Improve the product” never grants deployment, destructive action, data export, or permission to modify the controls.
2. **Alignment is not correctness.** An aligned change can be wrong; a technically correct change can be off-mission.
3. **Execution is not validation.** A tool can mutate something and then fail validation. The mutation must remain visible.
4. **Validation is not task acceptance.** Passing a check proves only what that check actually covered.
5. **Local correctness is not integration.** Boundary contracts and reachable behavior are part of the feature.
6. **Activity is not progress.** More turns, edits, facts, or tests do not necessarily reduce unresolved obligations.
7. **Lower tokens are not better efficiency by themselves.** Savings that shift cost to retries, tools, compute, review, or production incidents are not wins.
8. **Target-state facts are not implementation-status facts.** The schema has no shipped-status field on capabilities; `/now` means priority horizon, not “already works.”

### Keep the horizon honest

The four now-critical delivery capabilities—intent, context, wiring, and evidence—form one end-to-end chain. Control constrains that chain; efficiency measures it. Coordination and operations expand its useful reach. Continuity and improvement extend its duration and learning. Transfer and frontier goals stretch generalization without weakening the present contract.

“Day one” here means **a must-work target for a trustworthy product**, not a claim about today's binary. A partially wired implementation should remain explicitly partial in the package's current-state and gap documents.

## 7. Acceptance roadmap: turn each horizon into evidence

These are proposed evaluation obligations, not completed tests or measured outcomes.

| Capability | Evidence needed before claiming readiness | Adversarial case |
|---|---|---|
| Intent | Trace request, explicit requirements, assumptions, affected modules, and acceptance criteria; show which uncertainty was resolved by whom. | Request and existing documentation disagree; user intent is underspecified. |
| Context | Demonstrate retrieval coverage and freshness across source edits, missing indexes, renames, and evicted context. | A previously empty lookup becomes relevant after a new file appears. |
| Wiring | Trace an authorized production entry point through registration/configuration/interface/state/effect and observe the requested behavior. | Unit test passes but feature flag, DI registration, migration, or error path is missing. |
| Evidence | Bind checks to revision, environment, scope, and completion obligation; distinguish pass, fail, incomplete, unavailable, and waived. | Check times out or model says “passed” while receipts show failure. |
| Control | Show that denied/cancelled actions do not proceed and that partial effects are preserved for recovery. | Mutation happens before validator rejects the result. |
| Efficiency | Compare complete delivered-task distributions at matched acceptance quality and disclosed budgets. | Lower model tokens conceal expensive search, repeated tests, or human repair. |
| Coordination | Demonstrate ownership, dependency contracts, explicit disagreement, revision-safe integration, and bounded fan-out. | Two workers modify the same contract or reuse stale shared findings. |
| Operations | Exercise applicable configuration, migration, telemetry, rollout, and rollback obligations with authorized environments. | Merge succeeds while old clients or rollback fail. |
| Continuity | Interrupt and resume with bounded state, retained obligations, renewed authority, and fresh evidence. | Restart after external code changes or authority revocation. |
| Improvement | Preserve independent gates and held-out tasks; show reviewable changes and a functioning rollback route. | Candidate improves a public benchmark while weakening an existing gate. |
| Transfer | Demonstrate compatible reuse without leaking private content or carrying stale permission and environment assumptions. | Similar API names have different semantics in another repository. |
| Frontier | Publish a named benchmark protocol, complete task denominator, intervention policy, costs, and separate long-duration campaign evidence. | Public benchmark overfitting or a weeks-long run that is active but unproductive. |

Use both **complete-system comparisons** and **subsystem replacement experiments**. For system comparisons, include the full harness, models, retrieval, tools, verification, caches, and human intervention. For subsystem comparisons, hold its surrounding contract and workload fixed, replace only the component under examination, and report interface/adaptation costs. Do not compare an isolated cheap component with a fully provisioned competing system.

A fair primary score is a quality-cost frontier, not a magic scalar:

- Accepted task rate, false-completion rate, regression rate, and required human intervention.
- Tokens by provider billing class, compute/tool time, wall time and tail latency, and human review/repair time.
- Cost per accepted task using **all assigned tasks and their incurred costs**, including failed and abandoned tasks.
- Calibration of “unknown,” coverage of required checks, and evidence freshness.
- Long-horizon recovery, bounded-state behavior, and operator takeover.

For a task set $T$, an accounting convention is:

$$
C_{all} = C_{models} + C_{retrieval/index} + C_{tools} + C_{verification} + C_{coordination} + C_{storage} + C_{human}
$$

$$
C_{accepted} = \frac{\sum_{t\in T} C_{all}(t)}{\sum_{t\in T} \mathbf{1}[\text{independently accepted}(t)]}
$$

Do not report a finite per-accepted-task estimate if the denominator is zero. Avoid double-counting components already included in provider or tool charges; publish the ledger convention. Report human time separately as well as any monetized assumption. The same dollar total can hide very different latency and reliability profiles.

## 8. What this document cannot wire by itself

This candidate uses current supported fields. It cannot, by itself, create a typed acceptance evidence model, fresh verification receipts, permission gates, or independent evaluation. Natural-language requirements are useful planning data but not executable enforcement.

Static inspection of the pinned source highlights why that boundary matters:

- `Guardian.CheckAlignment` in `internal/northstar/guardian.go:346-378` treats absent vision as skipped, no LLM as passed with an assumption, and completion errors as warning. Those outcomes must not be promoted into “behavior verified.”
- `Guardian.parseAlignmentResponse` in `internal/northstar/guardian.go:550-603` honors an explicit result independently of the score. A malformed/empty response can retain the default score 0.7 and then be classified as passed at the [default warning threshold](https://github.com/theRebelliousNerd/codenerd/blob/fb1d74d0f13b7d0d647c968ae3c1d830eae9b555/internal/northstar/types.go#L430-L444); therefore the proposed unknown/non-passing contract is a target, not a current guarantee. A polished vision does not repair this path.
- The coverage rules in `internal/core/defaults/policy/prompt_northstar.mg:29-84` reason over relationship existence. They do not inspect test results or establish semantic need satisfaction.
- `Guardian.buildAlignmentSystemPrompt` in `internal/northstar/guardian.go:435-528` includes vision content; richer vision also increases alignment prompt cost. Graph usefulness must be balanced against repeated prompt injection, not assumed free.

A future typed bridge should associate each obligation with a target revision, relevant entry paths/modules, evidence kind and scope, execution outcome, expiry/invalidation conditions, and acceptance disposition. This is a **proposed extension**, not additional JSON fields hidden inside the current candidate. Keep it in a separate schema proposal until every writer, reader, persistence path, and consumer is explicitly wired.

The repository guidance also excludes using CodeNERD to repair a build so broken that CodeNERD cannot run. Recovery in that case needs an independent external engineering route; self-improvement must not assume its own executable is always available.

The architectural seam is: host tooling records typed observations and effects; the kernel derives obligation and acceptance state under reviewed rules; Northstar checks whether the selected work remains consistent with project purpose. None of those roles substitutes for the others.

### Candidate follow-on gaps

These IDs are local proposal identifiers, not claims that a canonical gap has been registered or closed.

| Local proposal ID | Buildable next question | Exit evidence |
|---|---|---|
| NS-VISION-01 | Can imports reject malformed or dangling references instead of silently losing intent? | A later authorized import suite demonstrates diagnostics and no partial authority change. |
| NS-VISION-02 | Can unavailable/malformed alignment assessments remain explicit unknown states? | Each unavailable/error/malformed case is distinguishable from affirmative alignment and behavioral acceptance. |
| NS-VISION-03 | Can required obligations reference fresh typed evidence instead of only prose? | A changed revision invalidates dependent acceptance; incomplete checks cannot derive accepted obligations. |
| NS-VISION-04 | Can campaign and production paths consume gate results consistently? | Authorized path-level witnesses show configured blocking results are propagated, with documented advisory behavior. |
| NS-VISION-05 | Can context selection preserve useful strategic guidance without repeating the whole vision every turn? | Equivalent task quality and coverage with complete prompt, retrieval, and cache costs reported. |

## 9. Adoption review, without changing the live system here

**Important warning about this richer graph:** every risk below has mitigation prose, but every mitigation is still proposed. The current `has_mitigation` rule checks fact existence, not implementation or verification. Consequently, importing this candidate may suppress the existing high-risk *unmitigated* warnings even when the proposed controls do not exist. Relationship coverage can similarly remove orphan warnings without establishing readiness. Do not interpret fewer warnings as a capability gain or safety clearance.

A future schema/policy extension should distinguish proposed, implemented, and independently verified mitigation state and retain evidence/expiry links. This is not a supported field silently added to the JSON. Until such an extension is wired, track that status explicitly in reviewed gap records and do not use the absence of current coverage warnings as an acceptance gate. A human-authorized waiver also remains **waived**, not passed; its authority, scope, rationale, and expiry need separate records.

1. Review the candidate's mission, persona names, stable IDs, priorities, and actual intended scope. Change the text rather than keeping goals no one means to fund.
2. Confirm the live revision's schema and serializer still match this document. Review any concurrent vision edits before choosing a new authoritative candidate.
3. Preserve/export the current authoritative vision and compare semantic content. Do not use fabricated timestamps or file-touch tricks to force a reconciliation winner.
4. When authorized, update through the supported JSON/wizard flow and let the host generate facts. Do not hand-edit the generated Mangle file as a parallel authority.
5. In a separately authorized verification pass, inspect the stored vision, projected relationships, warnings, and prompt/gate consumers. Check the accepted import and projection, not merely that a file exists.
6. Keep implementation status and evidence in current-state/gap records. Do not rewrite target capabilities as “shipped” because the import succeeded.

**The intended result is not a longer inspirational prompt. It is a connected, reviewable product contract: who the system serves, what it must accomplish, what can go wrong, what must hold, and which boundaries remain inviolable.**
