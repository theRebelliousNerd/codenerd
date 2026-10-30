---
doc-class: deep-dive
subsystem: northstar-campaign
implementation-status: partial
evidence-status: source-only
last-verified: 2026-10-30
verified-against: 6cb5f88e00eaf939d10dee46c16c68baf4b53d57
implementation-reviewed-against: fef98230c20a5751e4c32a2fd2757832a9d39cdc
supersedes: []
---

# Preserve module scope through Guardian alignment checks

## Conclusion and scope

Carry the affected workspace paths separately from the prose subject of a North Star alignment check. The current Guardian resolves module purpose and requirements by treating its entire subject string as a filesystem path. Production campaign callers instead supply a goal, phase name or task description. A campaign can therefore receive a project-level alignment assessment while the module-specific purpose and requirements for its known target paths never enter that assessment.

This is a concrete source-level handoff gap. It is not a demonstrated bad model verdict, permission bypass, unsafe commit or runtime incident. The proposed repair preserves the existing optional alignment integration, risk-policy decisions, independent verification and recurse ratchet. It does not turn alignment into behavioral acceptance or introduce a second controller.

All implementation citations are pinned to public main [`6cb5f88e00eaf939d10dee46c16c68baf4b53d57`](https://github.com/theRebelliousNerd/codenerd/commit/6cb5f88e00eaf939d10dee46c16c68baf4b53d57), rechecked on October 30, 2026. This review read source and existing test definitions only. It ran no repository code, tests, models, training or benchmarks and made no implementation changes. Proposed type names and dispositions below are design notation, not current APIs.

## The current boundary

The module machinery exists and has useful protections. `BuildCampaignObserver` acquires the shared Guardian and wires a `FactQuerier` when the kernel supports it. `ModuleForPath` selects the longest matching declared module using path-segment boundaries, so `internal/sessionx` does not match `internal/session`. `EffectiveModulePurpose` and `ModuleRequirementsFor` read kernel facts rather than a second independent module registry. [Observer construction](https://github.com/theRebelliousNerd/codenerd/blob/6cb5f88e00eaf939d10dee46c16c68baf4b53d57/internal/northstar/campaign_observer.go#L44-L78), [module lookup](https://github.com/theRebelliousNerd/codenerd/blob/6cb5f88e00eaf939d10dee46c16c68baf4b53d57/internal/northstar/module.go#L26-L111), [path selection](https://github.com/theRebelliousNerd/codenerd/blob/6cb5f88e00eaf939d10dee46c16c68baf4b53d57/internal/northstar/module.go#L131-L200).

The mismatch occurs at the input to that selector. `buildAlignmentPrompts` calls `moduleNorthstarSection(subject)`. That function calls `ModuleForPath(q, subject)`, then obtains the selected module's purpose and requirements. The additional context is appended to the user message afterward; it is not an alternate source of typed paths. Neither the selector nor its caller extracts paths from prose. [Prompt assembly](https://github.com/theRebelliousNerd/codenerd/blob/6cb5f88e00eaf939d10dee46c16c68baf4b53d57/internal/northstar/guardian.go#L476-L498), [module projection](https://github.com/theRebelliousNerd/codenerd/blob/6cb5f88e00eaf939d10dee46c16c68baf4b53d57/internal/northstar/guardian.go#L559-L596).

This also controls the JIT instruction shape. A nonempty module section selects `/module_alignment_check` and requires the module-refinement atom. An empty section selects `/alignment_check`, and the compiler explicitly rejects a project-only result containing that module atom. The omission is therefore upstream of otherwise deliberate prompt-selection checks. [JIT compilation contract](https://github.com/theRebelliousNerd/codenerd/blob/6cb5f88e00eaf939d10dee46c16c68baf4b53d57/internal/northstar/alignment_prompt.go#L84-L120).

## A production campaign witness

Consider a workspace with a project vision and a declared module `pkg/a` whose purpose and requirements are present in the kernel. The campaign has a planned write under `pkg/a`, its configured North Star risk gate is actually selected, and the Guardian has both a functioning querier and an available alignment client. Its goal is ordinary prose, such as “Repair cancellation behavior,” rather than a bare path. No model error is needed to establish the context omission.

1. Risk preflight collects `targetPaths` from the campaign and uses them to detect protected roots. The collector includes deterministic write sets, declared write sets and artifact paths. The scope is already available at this boundary. [Preflight](https://github.com/theRebelliousNerd/codenerd/blob/6cb5f88e00eaf939d10dee46c16c68baf4b53d57/internal/campaign/risk_scoring.go#L247-L260), [path collection](https://github.com/theRebelliousNerd/codenerd/blob/6cb5f88e00eaf939d10dee46c16c68baf4b53d57/internal/campaign/risk_scoring.go#L1160-L1181).
2. The North Star gate calls `StartCampaign(ctx, campaign.ID, campaign.Goal)` without those target paths. The observer passes the goal as the Guardian's subject and repeats it in additional context. [Risk-gate call](https://github.com/theRebelliousNerd/codenerd/blob/6cb5f88e00eaf939d10dee46c16c68baf4b53d57/internal/campaign/risk_scoring.go#L704-L718), [observer adapter](https://github.com/theRebelliousNerd/codenerd/blob/6cb5f88e00eaf939d10dee46c16c68baf4b53d57/internal/northstar/observer.go#L53-L75).
3. `ModuleForPath` normalizes the whole subject, then accepts only equality with a module path or a prefix of `module + "/"`. “Repair cancellation behavior” matches neither `pkg/a` nor `pkg/a/…`. The module section is empty despite the known target and valid module facts. [Exact matching rule](https://github.com/theRebelliousNerd/codenerd/blob/6cb5f88e00eaf939d10dee46c16c68baf4b53d57/internal/northstar/module.go#L131-L200).
4. The alignment request still contains the project vision, subject and additional context. If its model returns an otherwise admitted passed or warning result, the campaign North Star component is recorded as passed. That last outcome is conditional on the model response; the source alone establishes omission, not what the model would decide. [Request contents](https://github.com/theRebelliousNerd/codenerd/blob/6cb5f88e00eaf939d10dee46c16c68baf4b53d57/internal/northstar/guardian.go#L481-L498), [result grading](https://github.com/theRebelliousNerd/codenerd/blob/6cb5f88e00eaf939d10dee46c16c68baf4b53d57/internal/campaign/risk_scoring.go#L718-L742).

Recurse supplies a particularly direct instance. Its attempt constructor builds a goal beginning with `Fix … in …` or a capitalized improvement angle, and separately retains `a.Node.Paths` as the task's write set. Both CLI and chat construct their attempts through this same function. For a conventional module such as `pkg/a`, these generated goal strings are not module paths. [Generated goal and write set](https://github.com/theRebelliousNerd/codenerd/blob/6cb5f88e00eaf939d10dee46c16c68baf4b53d57/internal/campaign/recurse_attempt.go#L58-L80), [task scope](https://github.com/theRebelliousNerd/codenerd/blob/6cb5f88e00eaf939d10dee46c16c68baf4b53d57/internal/campaign/recurse_attempt.go#L121-L130), [CLI attempt](https://github.com/theRebelliousNerd/codenerd/blob/6cb5f88e00eaf939d10dee46c16c68baf4b53d57/cmd/nerd/cmd_campaign_recurse.go#L199-L220), [chat attempt](https://github.com/theRebelliousNerd/codenerd/blob/6cb5f88e00eaf939d10dee46c16c68baf4b53d57/cmd/nerd/chat/campaign_recurse.go#L157-L207).

The finding applies only when alignment actually runs. It does not claim every campaign selects this gate, or that missing module context necessarily changes the final decision. An independently sufficient global constraint, advisory review, permission refusal or failed verification can still reject the work.

This scope is not confined to hypothetical foreign-workspace metadata. The repository's own `internal/session/nerd.md` declares module purpose and the requirements `SESSION-VERIFY-HONEST`, `SESSION-NO-HOLLOW-SUCCESS` and `SESSION-GATE-NAMES-STABLE`. The document-to-facts producer emits module purpose and requirement records keyed by the document directory. These source definitions supply a concrete fixture, while the witness still explicitly requires their facts to be loaded into the assessed kernel. [Session declaration](https://github.com/theRebelliousNerd/codenerd/blob/6cb5f88e00eaf939d10dee46c16c68baf4b53d57/internal/session/nerd.md#L1-L26), [fact projection](https://github.com/theRebelliousNerd/codenerd/blob/6cb5f88e00eaf939d10dee46c16c68baf4b53d57/internal/projectdoc/facts.go#L118-L136).

## Other callers preserve the same mismatch

Campaign phase transitions pass the phase name as the observer's `phaseGoal`; the observer passes that value as the Guardian subject. The first phase is intentionally skipped because campaign start already checked the goal. Later phase checks hard-fail only on an explicit blocked result; unassessed checks are reported as such. Those enforcement choices are separate from the missing module scope. [Phase call and handling](https://github.com/theRebelliousNerd/codenerd/blob/6cb5f88e00eaf939d10dee46c16c68baf4b53d57/internal/campaign/orchestrator_phases.go#L425-L440), [phase observer](https://github.com/theRebelliousNerd/codenerd/blob/6cb5f88e00eaf939d10dee46c16c68baf4b53d57/internal/northstar/observer.go#L78-L110).

Task completion already supplies file paths separately, but the observer uses the task description, with an appended task ID, as the check subject. In the high-impact branch the paths appear only in additional-context prose. The periodic branch does not put those paths into its check context at all. The orchestrator discards the returned check and error, so this route establishes an advisory-assessment input gap, not a bypass of a task-completion gate. [Task caller](https://github.com/theRebelliousNerd/codenerd/blob/6cb5f88e00eaf939d10dee46c16c68baf4b53d57/internal/campaign/orchestrator_tasks.go#L686-L695), [task observer](https://github.com/theRebelliousNerd/codenerd/blob/6cb5f88e00eaf939d10dee46c16c68baf4b53d57/internal/northstar/observer.go#L134-L176).

The background adapter likewise turns an event into display prose before checking it. Its supported `file_modified` case formats “File modified: …”, then uses that sentence as the subject. This is an additional adapter-level example; it is not used here as proof that a particular production file-modification event is emitted. [Background adapter](https://github.com/theRebelliousNerd/codenerd/blob/6cb5f88e00eaf939d10dee46c16c68baf4b53d57/internal/northstar/observer.go#L281-L339).

## Existing protections and counterexamples

The Guardian is not wholly disconnected from module North Stars. A direct check whose subject is exactly `pkg/a/file.go` can select the module. The manual `/alignment` command forwards the user's subject, so a bare path can exercise that working route when its shared Guardian has a querier. Current helper tests explicitly use `internal/session/executor.go` and check that both module requirements and project mission enter the prompt. Their fixtures demonstrate the successful path-shaped input, not production prose-shaped caller coverage. [Manual command](https://github.com/theRebelliousNerd/codenerd/blob/6cb5f88e00eaf939d10dee46c16c68baf4b53d57/cmd/nerd/chat/commands_handlers_analysis.go#L560-L576), [manual check](https://github.com/theRebelliousNerd/codenerd/blob/6cb5f88e00eaf939d10dee46c16c68baf4b53d57/cmd/nerd/chat/model_helpers.go#L392-L441), [existing helper fixture](https://github.com/theRebelliousNerd/codenerd/blob/6cb5f88e00eaf939d10dee46c16c68baf4b53d57/internal/northstar/module_test.go#L108-L145), [querier fixture](https://github.com/theRebelliousNerd/codenerd/blob/6cb5f88e00eaf939d10dee46c16c68baf4b53d57/internal/northstar/guardian_querier_wiring_test.go#L114-L161).

Recurse also has an independent worker-context route. `northStar(node)` loads a matching node or root `nerd.md` purpose and requirements, and `recurseImproveTask` includes that text for the extend angle. This can inform the worker even when the independent Guardian request misses the module section. It does not supply that section to the Guardian or cover every fix and improvement angle. [Node North Star loading](https://github.com/theRebelliousNerd/codenerd/blob/6cb5f88e00eaf939d10dee46c16c68baf4b53d57/internal/campaign/recurse_cycle.go#L1162-L1192), [extend task context](https://github.com/theRebelliousNerd/codenerd/blob/6cb5f88e00eaf939d10dee46c16c68baf4b53d57/internal/campaign/recurse_attempt.go#L181-L203).

Preserve the protections already present:

- Project vision is rendered independently of module lookup. A missing module section does not remove global mission, requirements, risks or constraints from the alignment request. [Vision rendering](https://github.com/theRebelliousNerd/codenerd/blob/6cb5f88e00eaf939d10dee46c16c68baf4b53d57/internal/northstar/guardian.go#L501-L556).
- No vision produces skipped; absent client, prompt failure and completion failure produce unknown rather than an assumed passing assessment. The new scope contract should reuse that honest distinction. [Assessment failure handling](https://github.com/theRebelliousNerd/codenerd/blob/6cb5f88e00eaf939d10dee46c16c68baf4b53d57/internal/northstar/guardian.go#L383-L429).
- Protected-surface preflight refuses a missing configured observer, and the existing risk component maps blocked and inconclusive outcomes separately. This report identifies an input loss after observer wiring, not a missing observer. [Protected-surface admission](https://github.com/theRebelliousNerd/codenerd/blob/6cb5f88e00eaf939d10dee46c16c68baf4b53d57/internal/campaign/risk_scoring.go#L262-L295), [alignment outcome contract](https://github.com/theRebelliousNerd/codenerd/blob/6cb5f88e00eaf939d10dee46c16c68baf4b53d57/internal/campaign/risk_scoring.go#L693-L742).
- Action permission, write ownership, acceptance checks and the recurse ratchet remain separate authorities. A module-aligned proposal still needs authorization and current behavioral evidence. No assertion here establishes a whole-system completion bypass.

## A separate completeness problem at the same interface

`moduleNorthstarSection` also returns an empty section when module lookup or effective-purpose lookup fails. A requirement-query error leaves the purpose section present but omits the requirements. The helper APIs can report errors, yet this string-only projection erases the distinction between no governing module and unavailable module evidence. [Projection error handling](https://github.com/theRebelliousNerd/codenerd/blob/6cb5f88e00eaf939d10dee46c16c68baf4b53d57/internal/northstar/guardian.go#L564-L595).

The prose/path mismatch does not depend on a query error. The error behavior is a related repair obligation: adding typed paths alone would not ensure that a scoped assessment declares whether all required module context was obtained. There is no claim here that a kernel query failed in an observed run.

## Proposed scope and assessment contract

Specify the change in the owning North Star and campaign documentation before implementation. Keep semantic description and structural identity separate throughout the call chain.

**Request identity.** An alignment request should carry the subject prose, trigger, workspace identity and an explicit scope disposition: project-only, known affected paths or unresolved scope. Each supplied path should record whether it is planned write scope, an observed effect or contextual source material. A path known to the planner is evidence about intended scope, not proof that an effect occurred. Avoid widening permissions or work admission when constructing this request.

**Canonical paths.** Normalize paths through the workspace's existing canonical path contract, retaining relative identities expected by module facts. Admit equivalent separators and in-workspace absolute paths deliberately. Reject or represent out-of-workspace and ambiguous identities explicitly; do not silently trim them into another module. Do not regex-scan free text and call its incidental path mentions authoritative scope.

**Module selection.** Resolve every relevant scoped path, preserve the existing longest-directory-prefix behavior, and deduplicate by canonical module identity. A campaign touching two modules needs both relevant module contexts, not the one whose name happens to appear first in a goal. Preserve requirement identity as module plus requirement ID, and keep project mission and constraints alongside refinements. A global request may intentionally remain project-only; an unresolved scoped request must not be silently relabeled global.

**Completeness.** Return structured resolution information rather than only a rendered string: selected modules, unmatched paths, unavailable queries and omitted requirements. Distinguish a successful lookup finding no module from a failed lookup. If required module context cannot be retrieved or fitted, report a partial or unknown scoped assessment under reviewed policy. Preserve any legitimate project-only assessment as separately scoped information rather than laundering it into complete module coverage.

The accepted scope policy keeps scope resolution separate from the alignment
verdict. When affected paths or usable module evidence are unavailable, run the
existing project-level check and record unknown scope with the reason. Module
purpose and requirements enter only when affected paths resolve to declared
modules. A missing scope must not replace the project verdict with a clarifying
question or stall a campaign; project alignment also does not establish complete
module coverage.

**Policy and evidence.** Host code should canonicalize identities, retrieve a coherent fact view, serialize context and report errors. Mangle should derive the required scope, completeness obligations and existing risk consequences from admitted facts where those are executive decisions. Module-context presence is not proof that the model followed it, and alignment remains a fallible strategic judgment. Do not make a Go convenience fallback the new admission authority.

**Caller coverage.** Carry target scope through campaign start, phase transition and task observation. For recurse, retain the constructor's node paths rather than recovering them from its generated goal. For task completion, use observed effect identities where available and label artifact-only paths accurately. Maintain explicit behavior for manual path checks, manual prose checks and background events without trustworthy scope. Do not silently harden advisory task checks or change optional gate-selection behavior as part of this input repair.

**Publication.** Record which project/module identities and context generation supported an assessment, with selected and unresolved counts. Keep planned and observed scope distinguishable. Any caching or reuse must include scope and relevant source identity, not only subject prose. Scope expansion during a campaign should trigger the corresponding existing assessment obligation instead of donating an earlier narrow result to new work. These are proposed requirements; this audit does not establish that current assessments are cached or that a particular stale result was reused.

## Related primary research

[Liu and colleagues’ long-context study](https://arxiv.org/abs/2307.03172) found that the position of relevant information affected performance in their evaluated retrieval tasks. It motivates testing the actual rendered request and varying context position; it does not establish how a particular CodeNERD model will behave or make context presence proof of compliance.

## Evaluation through real adapters

No evaluation below was run. Begin with deterministic request-capture fixtures through production adapters before purchasing live-model judgments. Hold the project vision and module facts fixed and inspect the actual final system and user messages, not only a helper's return value.

| Case | Required observation |
|---|---|
| Prose goal with a known `pkg/a` write set | Campaign-start request contains `pkg/a` purpose and requirements, plus project vision |
| Recurse-generated fix and all improvement angles | Scope survives generated goal formatting on both CLI and chat paths |
| Pure-path manual check | Existing working module resolution remains intact |
| Prose manual check with no structural scope | Explicit project-only or unresolved result under the declared contract |
| Two modules and duplicate local requirement IDs | Both contexts retain separate identities without accidental conflation |
| Nested module and similarly prefixed sibling | Most-specific module wins per path; `pkg/a2` is not `pkg/a` |
| Module with requirements but inherited purpose | Kernel-derived inherited project purpose is retained with local requirements |
| Query failure at each resolution stage | Failure remains visible and cannot masquerade as no module or complete coverage |
| Context too large for the request budget | No silent loss of mandatory module requirements; partial/unknown is explicit |
| Phase and task adapters | Names/descriptions remain prose; actual scope travels separately; existing advisory semantics remain |
| No vision, absent querier/client, no declared module | Each existing availability state has an explicit, non-fabricated interpretation |
| Scope change after initial assessment | Prior coverage is not attributed to the newly added module |

For the inherited-purpose control, current policy already derives project mission for a known module without its own purpose. Preserve that behavior rather than inventing a competing fallback. [Existing inheritance policy](https://github.com/theRebelliousNerd/codenerd/blob/6cb5f88e00eaf939d10dee46c16c68baf4b53d57/internal/core/defaults/policy/prompt_northstar.mg#L217-L248).

After deterministic transport is established, a separately authorized matched study can evaluate whether relevant module context changes judgment quality. Include work aligned with both global and local purpose, work globally plausible but contrary to a local requirement, and work whose local purpose conflicts with a global constraint. Use independent expected classifications and unchanged model/settings. Report omitted-context rate, incorrect scope attribution, false positive/negative judgments, unknown outcomes and disagreement; do not declare success only when more checks block work.

Count the full cost: canonicalization, kernel queries, module deduplication, prompt bytes/tokens, JIT compilation, model latency, retries, storage and extra assessments caused by scope expansion. A naive per-path implementation can repeat the same three predicate scans for many files. Prefer one coherent relevant module view per check and deduplicate paths before rendering; evaluate an indexed or cached view only with source-identity invalidation and measured need. Charge any additional scope metadata and recovery logic to the treatment. No quality, token or performance improvement is established by this source review.

The completion criterion is narrow: for every actually executed scoped alignment check, the final request and recorded scope account for the governing module context or explicitly report why it is incomplete, while preserving project constraints and existing permission, risk and verification semantics.

## Relationship to existing research

The [mission-driven forever-improvement proposal](https://github.com/theRebelliousNerd/codenerd/blob/21808fc00ee04762982a056e4e53a98a131d0bae/Docs/innovations/mission-driven-forever-improvement.md) proposes a revision-aware bridge between strategic requirements and behavioral evidence. It already recognizes campaign alignment and recurse's separate extend-context route. This note concerns a smaller existing interface: known affected paths are not supplied to the module-context selector used by the alignment request. Repairing it needs neither the full mission evidence graph nor a new scheduler.

The [North Star blueprint](https://github.com/theRebelliousNerd/codenerd/blob/6cb5f88e00eaf939d10dee46c16c68baf4b53d57/Docs/architecture/northstar/proposed-vision-and-evidence-blueprint.md#as-built-2026-10-27) already owns validation, honest unknown outcomes and typed mitigation evidence. [Lossless strategic editing](https://github.com/theRebelliousNerd/codenerd/blob/6cb5f88e00eaf939d10dee46c16c68baf4b53d57/Docs/audits/northstar-lossless-strategic-editing.md) owns preserving canonical vision data and guarded edits. Neither is reopened here. The [forcing-and-completion journey](https://github.com/theRebelliousNerd/codenerd/blob/6cb5f88e00eaf939d10dee46c16c68baf4b53d57/Docs/journeys/02-forcing-and-completion.md) already distinguishes alignment from behavioral obligation enforcement; this note does not claim that distinction as new.

The bounded duplicate check covered those documents, the North Star architecture gap and failure-mode pages, relevant recurse/gate/ownership audits, the main and research branch path inventories, and repository searches for the exact module-selector symbols and related predicates. Search indexing returned an older source pin, so implementation conclusions were verified through exact-file reads at `32b8417bb9b0ec70a83b06226c67c906d0b30be0`, followed by blob-identity comparison when main advanced to the cited revision. The concrete session declaration was read directly at the cited revision. No inspected document specified this prose-subject versus structural-scope handoff defect. This is a bounded corpus check, not a claim that all historical branches or every external research source were exhaustively searched. The nine central Guardian/campaign source files inspected were blob-identical on research revision `21808fc00ee04762982a056e4e53a98a131d0bae` and pinned main.


## As built

Rechecked against worktree base `fef98230c20a5751e4c32a2fd2757832a9d39cdc`.
The research narrative above describes its earlier pin. Every transport and
completeness finding remained **real**; none was already fixed or ruled wrong.
Commits: none, following the author-only instruction.

| Finding | Ruling and authored repair |
|---|---|
| Guardian selects modules from subject prose; campaign start drops known paths | Real. `CheckAlignment` and `StartCampaign` now require separate workspace scope and path roles. |
| Recurse loses node scope on fix and improvement checks | Real. `campaignAlignmentScope` reads the shared constructor's write set for both command paths; the constructor now owns a copy of node paths. |
| Phase names replace scope; first-phase skip assumes coverage | Real. `startNextPhase` carries phase task scope. Reuse requires assessed coverage of those paths/roles and matching current vision/module digests; expansion or changed context invokes an enabled phase check. |
| Periodic/high-impact task checks discard paths; artifact paths are not observed modifications | Real. `spawnTask` retains executor-observed identities separately from its prose return. `completeTask` carries planned writes, artifacts and effects through both advisory branches; only observed effects produce file-change observations. |
| Background file targets become display prose | Real at the supported adapter. File targets now travel as observed effects; events without trustworthy scope still receive project alignment, with unknown scope and its reason in assessment metadata and history. No production emission incident is inferred. |
| Query errors disappear or requirements are omitted while purpose remains | Real. The string-only projection is deleted. A kernel snapshot supplies the module view; unavailable/malformed/conflicting context records unknown scope and issues while the project check proceeds without partial module refinements. A successful lookup with no governing module records unmatched paths and unknown scope. |

Existing protections were confirmed in `module.go`, `alignment_prompt.go`,
`alignment_verdict.go`, `policy/prompt_northstar.mg`, `risk_scoring.go`,
`projectdoc/facts.go` and `internal/session/nerd.md`: segment boundaries,
project constraints, kernel-derived inherited purpose, honest unavailable
outcomes and independent risk/permission/acceptance authorities. Manual bare
paths remain scoped; manual prose is explicitly project-only. Recurse's separate
extend-context route remains. Optional gate selection, task advisory semantics
and the recurse ratchet are unchanged. Neither the audit nor this repair
establishes bad model verdicts, production incidents or improved judgment quality.

Files: `internal/northstar/{alignment_scope,alignment_modules,guardian,observer,types,store}.go`;
`internal/campaign/{northstar_scope,risk_scoring,orchestrator_phases,orchestrator_tasks,orchestrator_task_handlers,types,recurse_attempt}.go`;
and `cmd/nerd/chat/model_helpers.go`. Coverage persists scope/roles, selected
modules, unmatched paths, the resolution outcome, issues and vision/module digests
atomically with the check in `alignment_coverage`; earlier rows retain absent
coverage. Logs, gate data and background metadata expose unknown scope and its
reason independently of the alignment verdict. No atoms or Mangle changed.
The as-built specifications are updated under `Docs/architecture/northstar/`
and `Docs/architecture/campaign/`.

Regression definitions: `internal/northstar/alignment_scope_test.go` and
`internal/campaign/northstar_scope_test.go`, plus updated API fixtures. They
capture final requests through risk, observer, phase, task, manual and background
adapters; cover multiple modules and local requirement IDs, real-kernel purpose
inheritance, nested/sibling boundaries, canonical paths, incomplete context,
first-phase scope/context changes, full large requirements, provider rejection,
and SQLite upgrade/history. Recurse cases cover fix, stabilize, harden, simplify
and extend through the constructor shared by CLI and chat.

The initial transport regressions have source-only fail-before evidence: the
prior gate had no scope argument, the prompt selected modules solely from prose,
and the projection suppressed lookup errors. The root session subsequently
reported four failing existing contracts: the background adapter returned
clarify instead of block/proceed, prompt composition rejected missing scope,
and the mitigation prompt fixture dereferenced a missing store. Those failures
establish executable fail-before evidence for the project-fallback repair;
pass-after verification remains pending.

The added incomplete-scope, background-scope and unavailable-evidence risk-gate
regressions were corrected because they wrongly made unknown scope override the
project verdict. Existing block/proceed and prompt contract assertions remain
intact. Coverage now tests passed and blocked verdicts with unknown scope and
persisted reasons; first-phase reuse still requires complete coverage.
Formatting and read-only diff checks pass. Status remains **partial** pending the
root session's owning-package and caller suites. The required private-terms
scanner was not found inside this repository and its scan also remains pending.

Full requirements are sent without an internal byte cap; completion failure,
including provider rejection of oversized input, remains unknown. A bounded
request-budget policy and selective coherent snapshot API remain future design
work. `QueryAll` currently materializes all kernel facts; first-phase reuse
validation can add a snapshot read. Neither performance nor model-window fit
has been measured, and no model-quality improvement is claimed.
