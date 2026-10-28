---
doc-class: deep-dive
subsystem: northstar-chat
implementation-status: planned
last-verified: 2026-10-28
verified-against: 556f9fe092d0745248414ebdf2f6fe0db4d93dd5
verification-method: static-source-inspection
supersedes: []
---

# Preserve strategic intent through Northstar editing

## Decision

Complete Northstar's **lossless strategic editing capability**. Treat the interactive wizard as an editable view of the canonical vision, not as a replacement record. An unchanged view must preserve the complete vision; an intentional edit must change only its declared fields and explicitly derived consequences. Model-generated requirements remain proposals until admitted against that same canonical graph.

Do not revive `parseGeneratedRequirements`. The production generation path already calls the shared JSON derivation library. Adding the missing relationship fields is an existing integration obligation, not a new research result. The additional proposal here is a **base-bound semantic edit contract** covering ordinary reload and save, concurrent changes, graph identity, and evidence-bearing fields that the UI does not display.

This is a documentation-only design, not an implementation or an observed runtime incident. No code, tests, builds, inference, training, or benchmarks were run. Sources were read at `35ede84560a179897ed9627493d872fe96db126f`; a subsequent comparison to the pinned main revision above confirmed the relevant Northstar, chat, guidance, and specification files were unchanged. The intervening commit concerned orientation treatment shadow mode.

## Existing capability and research boundaries

The repository requires JIT-first model behavior, Mangle-derived executive decisions, spec-first work, and inspection of real wiring before treating dead code as unfinished capability. These requirements govern this proposal. [Repository guidance](https://github.com/theRebelliousNerd/codenerd/blob/556f9fe092d0745248414ebdf2f6fe0db4d93dd5/agents.md#L1-L117)

Several useful pieces already exist:

- `generateRequirementsWithLLM` calls `northstar.DeriveRequirements` using the planner/main client and phase prompt. The library pages the full serialized wizard input, validates JSON requirements, preserves every returned object, and reserves existing IDs. Its parser is not the old line-oriented helper. [TUI caller](https://github.com/theRebelliousNerd/codenerd/blob/556f9fe092d0745248414ebdf2f6fe0db4d93dd5/cmd/nerd/chat/northstar_llm.go#L17-L60), [shared derivation](https://github.com/theRebelliousNerd/codenerd/blob/556f9fe092d0745248414ebdf2f6fe0db4d93dd5/internal/northstar/derive.go#L438-L504), [parser](https://github.com/theRebelliousNerd/codenerd/blob/556f9fe092d0745248414ebdf2f6fe0db4d93dd5/internal/northstar/derive_parse.go#L393-L475)
- The caller refuses linked generated requirements because its DTO cannot preserve `Supports` or `Addresses`. That explicit refusal is safer than silently removing links. The orientation spec already records the missing adapter fields and retirement of the old parser. Do not present either as a newly discovered architecture idea. [Existing specification](https://github.com/theRebelliousNerd/codenerd/blob/556f9fe092d0745248414ebdf2f6fe0db4d93dd5/Docs/architecture/orient/07-NORTH-STAR-DERIVATION.md#L145-L154), [scoped guidance](https://github.com/theRebelliousNerd/codenerd/blob/556f9fe092d0745248414ebdf2f6fe0db4d93dd5/cmd/nerd/chat/agents.md)
- Canonical `WizardDocument`, `Vision`, conversion, validation, SQLite storage, and fact projection support relationship-bearing records. Risks additionally carry mitigation status, revision-bound evidence, and waivers. The CLI canonical import is therefore a real alternative, not an imaginary future path. [Canonical conversion](https://github.com/theRebelliousNerd/codenerd/blob/556f9fe092d0745248414ebdf2f6fe0db4d93dd5/internal/northstar/bridge.go#L87-L282), [CLI import](https://github.com/theRebelliousNerd/codenerd/blob/556f9fe092d0745248414ebdf2f6fe0db4d93dd5/cmd/nerd/cmd_northstar.go#L634-L690)
- `InstallDerivedVision` already preserves an existing authoritative vision by writing a separate candidate and uses an insert-if-absent transaction for a fresh authority. This proposal does not replace that path. [Derived installation](https://github.com/theRebelliousNerd/codenerd/blob/556f9fe092d0745248414ebdf2f6fe0db4d93dd5/internal/northstar/derive.go#L600-L708)
- The Northstar blueprint already proposes typed requirement acceptance evidence, and its as-built section explicitly leaves that and the automatic mitigation-evidence writer deferred. Those are separate capabilities. This proposal preserves existing strategic data through edits; it does not claim to implement requirement verification or automatically generate risk evidence. [Blueprint and deferred work](https://github.com/theRebelliousNerd/codenerd/blob/556f9fe092d0745248414ebdf2f6fe0db4d93dd5/Docs/architecture/northstar/proposed-vision-and-evidence-blueprint.md#L774-L870)

The root `Docs/audits/*.md` corpus was checked for Northstar, wizard, requirement-generation, and view-update overlap. The current blueprint and orientation specification cover the narrower field and evidence gaps. No inspected audit specifies the no-op edit law and base-bound update protocol proposed here. This is a bounded novelty check, not a claim of novelty in the research field.

## Source grounded unfinished capability

### A normal edit path can narrow the authoritative graph

The production path is reachable without automatic requirement generation:

1. `/northstar` calls `loadExistingNorthstar` and puts an existing mission into the summary phase. [Command handler](https://github.com/theRebelliousNerd/codenerd/blob/556f9fe092d0745248414ebdf2f6fe0db4d93dd5/cmd/nerd/chat/commands_handlers.go#L888-L927)
2. The loader unmarshals `.nerd/northstar.json` into `NorthstarWizardState`. Its `Capability` has neither `ID` nor `Serves`; its `Risk` lacks `ID`, mitigation status, evidence, and waiver; its requirement lacks `Supports` and `Addresses`. These fields cannot survive this decoding into the narrow structs. [Wizard types](https://github.com/theRebelliousNerd/codenerd/blob/556f9fe092d0745248414ebdf2f6fe0db4d93dd5/cmd/nerd/chat/northstar_types.go#L41-L63), [loader](https://github.com/theRebelliousNerd/codenerd/blob/556f9fe092d0745248414ebdf2f6fe0db4d93dd5/cmd/nerd/chat/northstar_types.go#L192-L228)
3. Summary accepts `save`, which calls `saveNorthstar(false)`. The save path marshals the reduced wizard, converts it to the canonical domain model, and calls `Guardian.UpdateVision`. Missing capability/risk IDs are synthesized from array positions by `ToVision`. [Summary action](https://github.com/theRebelliousNerd/codenerd/blob/556f9fe092d0745248414ebdf2f6fe0db4d93dd5/cmd/nerd/chat/northstar_wizard.go#L821-L856), [persistence adapter](https://github.com/theRebelliousNerd/codenerd/blob/556f9fe092d0745248414ebdf2f6fe0db4d93dd5/cmd/nerd/chat/northstar_persistence.go#L147-L186), [ID conversion](https://github.com/theRebelliousNerd/codenerd/blob/556f9fe092d0745248414ebdf2f6fe0db4d93dd5/internal/northstar/bridge.go#L181-L217)

**Static consequence:** opening and saving a rich canonical vision without editing its displayed values can remove relationships and mitigation evidence/waivers and replace custom capability/risk identities with positional IDs. This does not require malformed model output. It is an information-loss boundary between two live representations. No execution was performed to measure its frequency or reproduce an incident.

Validation does not recover omitted intent: it validates the resulting record's structure, and an empty relationship list can be structurally valid. The store preserves an existing `CreatedAt` when the incoming value is zero; this report does **not** claim the database creation timestamp is reset. [Validation](https://github.com/theRebelliousNerd/codenerd/blob/556f9fe092d0745248414ebdf2f6fe0db4d93dd5/internal/northstar/validate.go#L64-L132), [store timestamp handling](https://github.com/theRebelliousNerd/codenerd/blob/556f9fe092d0745248414ebdf2f6fe0db4d93dd5/internal/northstar/store.go#L196-L281)

### Mutation ordering also belongs to the edit contract

`saveNorthstar` writes `.mg`, knowledge atoms, direct kernel assertions, and JSON before it calls the canonical persistence adapter. Warnings are accumulated; the campaign branch starts a campaign after that sequence. Thus the canonical validator is not a preflight for every effect of this UI action. `Guardian.UpdateVision` itself commits authority before exports and logs export failures. These facts motivate explicit admission and publication outcomes, not a claim that a single SQLite transaction already covers files and kernel state. [UI save ordering](https://github.com/theRebelliousNerd/codenerd/blob/556f9fe092d0745248414ebdf2f6fe0db4d93dd5/cmd/nerd/chat/northstar_persistence.go#L23-L134), [Guardian update](https://github.com/theRebelliousNerd/codenerd/blob/556f9fe092d0745248414ebdf2f6fe0db4d93dd5/internal/northstar/guardian.go#L287-L325)

The capability matters because these are operational inputs. Relationship facts drive orphan/coverage reasoning; mitigation status and evidence determine whether high-risk warnings remain active. Preserving records is not equivalent to proving their semantics, but losing records changes what the kernel can reason about. [Current policy consumers](https://github.com/theRebelliousNerd/codenerd/blob/556f9fe092d0745248414ebdf2f6fe0db4d93dd5/internal/core/defaults/policy/prompt_northstar.mg#L18-L109)

### Canonical snapshots also need real isolation

The proposed editor cannot assume `Guardian.GetVision` already returns a fully detached graph. It calls `cloneVision`, which copies the outer `Capabilities` and `Requirements` slices but shallow-copies their elements. The nested `Serves`, `Supports`, and `Addresses` slice backing arrays can therefore remain shared with the Guardian's in-memory vision. Mutating an existing link element through such a returned value can affect that in-memory value before any explicit save. This is a static aliasing deduction, not an executed mutation or race finding. The currently traced JSON-loading wizard does not call this getter; the issue matters when reusing the canonical API for the proposed editor. [Getter](https://github.com/theRebelliousNerd/codenerd/blob/556f9fe092d0745248414ebdf2f6fe0db4d93dd5/internal/northstar/guardian.go#L273-L278), [clone implementation](https://github.com/theRebelliousNerd/codenerd/blob/556f9fe092d0745248414ebdf2f6fe0db4d93dd5/internal/northstar/guardian.go#L831-L856)

Do not generalize this to every nested field: persona slices are detached, and `cloneRisk` separately copies mitigation-evidence records and the waiver object. Those existing protections should be retained. [Risk clone](https://github.com/theRebelliousNerd/codenerd/blob/556f9fe092d0745248414ebdf2f6fe0db4d93dd5/internal/northstar/mitigation.go#L147-L154)

## Proposed semantic edit contract

### Canonical base and a bounded view

Start editing from a canonical vision snapshot and a host-issued content generation. Keep complete canonical data outside the displayed view. Do not use a Git commit alone as this generation: vision data can change without a source commit. Do not use mtime as proof that an edit is based on current authority.

The generation guard must cover **every canonical writer**, including CLI imports, Guardian updates, accepted file reconciliation, and derived installation, not just the new UI. Each successful canonical change must advance the shared generation, or admission must atomically compare the current complete canonical content against the expected base inside the authority transaction. A UI-only counter would miss a concurrent CLI import. File edits that have not been admitted remain candidates, not a second authoritative generation.

The immutable base, mutable candidate, live Guardian state, and any concurrently open edit session must be independently deep-detached across all mutable nested containers and pointers. Copying only the outer slices is insufficient. Candidate mutation must neither mutate the base used to compute the diff nor alter live state. Cancelling or abandoning a candidate must have no semantic effect on the Guardian, SQLite authority, files, knowledge, or kernel. Apply the same ownership rule when an admitted candidate is installed: a retained caller reference must not mutate it after publication.

Reuse the canonical types where practical. If the UI remains a projection, its write-back must merge explicit edits into the retained canonical base; absent view fields mean **not edited**, never delete. A save with no semantic delta should not rewrite authority or its projections.

Use stable entity IDs for edit targets. Array position is presentation order, not identity. Existing supported IDs survive unchanged. Legacy records without IDs need a one-time visible migration before relationship edits, not repeated renumbering as arrays change. Unknown future schema fields must be retained safely or cause an unsupported-version refusal; silently ignoring them and replacing the document is not acceptable.

### Proposed edits and derived admission

Represent proposed operations as typed, ID-addressed changes: add requirement, update requirement field, add/remove a named relationship, update a capability, or delete a specified entity. Distinguish explicit deletion from omission. Include the base generation and the fields/entities each operation expects to read.

The host parses records, resolves IDs, computes the candidate and a semantic diff, and reports observations. Reviewed Mangle policy derives whether the proposal is admissible, requires conflict resolution, or exceeds the actor's authority. Go remains the I/O and transformation layer, not a second discretionary executive.

An LLM may propose requirements through the existing JIT derivation path, but cannot grant itself permission to change strategic authority, mitigation verification, waivers, or safety policy. The existing user-authorized edit flow remains the authority boundary. Generated links must resolve against the canonical snapshot, not reconstructed `cap_1`/`risk_1` aliases from a narrowed view.

Initially, reject a stale base before any mutation and provide a concise semantic comparison. Automatic rebase is optional later work: it must distinguish disjoint fields from graph conflicts such as deleting a capability referenced by a concurrent requirement. Last-writer-wins is not sufficient evidence of intent preservation.

### Preserve evidence without laundering it

Preserve unchanged evidence and waiver records exactly in a no-op edit. If an authorized edit changes the subject a record supports, keep the historical record but require explicit applicability reassessment under reviewed rules. Do not silently carry a verified label onto a different risk, and do not turn a waiver into a pass. Conversely, a harmless UI round-trip must not erase evidence just because the view does not display it.

The present risk evidence contract is bound to Git HEAD, not a full semantic dependency graph. Subject-sensitive invalidation would be a proposed extension, not a description of current enforcement. It should compose with the existing evidence blueprint rather than introduce a competing acceptance system.

### Commit once and disclose projection state

Validate the complete candidate and check its expected base in the same authority transaction that commits the edit. Do not publish ad-hoc kernel or file projections first. Canonical JSON, Mangle facts, and any semantic-search representation should be derived from the admitted version, not independently encoded by the UI.

Return separate outcomes for rejected, conflicted, committed, and projection-pending edits. A committed authority with a failed export is not an unchanged rejected edit. A pending projection is not a fully successful save, and dependent campaign start needs an explicit policy decision about whether the required generation is active. Reuse the existing effect-receipt and recovery work for broader publication mechanics rather than claiming local JSON Patch provides distributed atomicity. [Related effect-receipt proposal](https://github.com/theRebelliousNerd/codenerd/blob/556f9fe092d0745248414ebdf2f6fe0db4d93dd5/Docs/audits/tool-effect-receipts-and-validation-completeness.md)

## Research basis and limits

The view-update problem is established research. Foster and colleagues define a lens using a forward view and a write-back operation that also receives the original source. Their no-op and read-after-write laws motivate two useful obligations here: writing back an unchanged view preserves the source, and reading an admitted update returns the intended view. Applying those laws to Northstar is an engineering proposal, not proof that its current or future code satisfies them. [Foster et al, POPL 2005](https://www.cis.upenn.edu/~bcpierce/papers/newlenses-popl.pdf)

RFC 6902 provides explicit JSON operations and a `test` precondition. RFC 5789 discusses conditional updates and atomic application of an HTTP patch. They support guarded deltas as a design pattern. They do not define Northstar entity identity, grant authorization, prove semantic correctness, or make SQLite, exports, caches, and kernel facts one transaction. An ID-addressed domain protocol may be safer than raw array-index JSON Patch. [RFC 6902](https://www.rfc-editor.org/rfc/rfc6902.html), [RFC 5789](https://www.rfc-editor.org/rfc/rfc5789.html)

## Alternatives and delivery sequence

1. **Canonical DTO reuse only.** The smallest useful baseline: retain all currently supported fields and eliminate the narrowed replacement. This should be evaluated first. It does not by itself prevent stale whole-document overwrites or ensure failure-safe effect ordering.
2. **Canonical snapshot plus explicit delta.** Preferred next capability: deterministic preservation, reviewable intent, stale-base refusal, and one authority admission point. It costs more integration work but addresses the end-to-end boundary rather than another field patch.
3. **Regenerate the whole vision with an LLM.** Not preferred for ordinary edits. It adds inference cost and makes unrelated semantic drift harder to separate from requested changes. Whole-vision drafting already has a separate candidate path.
4. **Treat CLI import as the complete answer.** The existing canonical import is useful and should remain supported. It is not a substitute for a working interactive edit contract when the advertised UI loads and saves existing authority.

Sequence the work through the relevant Northstar, chat/CLI, and orientation specifications. First specify and later verify no-op and single-field round-trips. Then add base-bound admission. Add richer conflict/rebase or semantic evidence invalidation only with a demonstrated need and explicit ownership. Retire the old parser separately; doing so is cleanup, not evidence that the capability is finished.

## Evaluation plan without executing it

Use the normal `/northstar` load, review, edit, save, and campaign entry paths. Helper-only tests are supporting evidence, not the exit. Proposed fixtures and experiments:

| Case | Required observation |
|---|---|
| Rich vision opened and saved without edits | Same canonical semantic value, stable entity IDs, relationships, mitigation evidence, waivers, and effective projected facts, with policy version and exogenous context held fixed, especially `current_time` and `git_state(/head, Sha)` |
| Candidate link mutated and then cancelled | Base snapshot, a second snapshot, live Guardian value, SQLite, exports, knowledge, and kernel remain semantically unchanged; exercise existing-element writes and append/reorder paths |
| Caller mutates its candidate after admission | The installed authority and live Guardian remain unchanged through that retained reference |
| One description edited | Exactly the requested semantic delta plus explicitly derived invalidation; no unrelated loss |
| Reorder capabilities or risks | IDs and edges retain meaning; no positional retargeting |
| Generate linked requirements | Links survive review and canonical import; unknown targets are diagnosed before effects |
| Delete a referenced entity | Explicit dependent-link resolution or rejection, never silent pruning from authoritative intent |
| Concurrent canonical import while UI is open | Stale edit refused with no partial mutation; current authority remains intact |
| Invalid candidate | No authority, JSON, Mangle, knowledge, or kernel change from the attempted edit |
| Authority commit followed by export failure | Committed versus projection-pending state accurately reported; recovery does not reapply the semantic edit |
| Changed risk with retained historical evidence | Historical evidence is inspectable but not automatically treated as applicable to the changed subject |
| Legacy and future schema versions | Documented migration or visible refusal, never silent narrowing |
| Save and start campaign | Campaign consumes the admitted required generation or is visibly deferred |

Compare three systems at matched edit tasks: current UI, canonical DTO reuse, and the proposed base-bound edit contract. Include the existing CLI canonical-import path as a representation-preservation control, not as a substitute for UI success. Report complete task denominators, unintended field/edge changes, lost-update rate, correctly rejected conflicts, completion rate, recovery outcome, and required human repair. For model-generated requirements, separately measure useful requirement/link accuracy; passing structural preservation does not establish semantic quality.

Keep projection preservation distinct from legitimate changes in derived conclusions: a waiver may expire when `current_time` advances, and evidence may become stale when HEAD changes. A no-op edit must not itself change the projected vision facts; equality of downstream effective facts is expected only under the same policy and exogenous facts. Add separate changed-time and changed-HEAD fixtures that require the existing invalidation rules to react normally rather than freezing them to make the round-trip look stable.

No-op and unrelated-field preservation are zero-loss contract checks, not average scores. Any violated case blocks a correctness claim for that supported schema/path. Statistical user/model evaluations need frozen fixtures, a held-out set, explicit model/provider/configuration, and uncertainty intervals; no advantage is asserted before those runs.

## Costs and boundedness

For a vision with N records, E relationship/evidence entries, and B bytes of variable-sized payload, straightforward validation/diff and fully detached snapshots require O(N + E + B) work/storage, plus database and projection costs. Conflict review may temporarily require a third snapshot. Stable-ID lookup avoids repeatedly scanning arrays for each operation. These are algorithmic estimates, not measurements; deep-detachment cost must not be hidden by benchmarking shallow copies.

Manual edits and no-op round-trips need no additional model call. Optional semantic assistance adds calls and review overhead that must be counted. Measure total input/output tokens, retries, discarded generations, validation time, projection work, storage growth, latency, and human repair per accepted edit across all attempts. A delta may reduce model or review payload, but it cannot omit context needed to understand a changed requirement or risk; no token savings percentage is claimed.

Retain only the current edit's necessary base/candidate working state, with an explicit policy for durable history. Do not introduce arbitrary run-level turn limits. Cancellation, loss of authority, invalid state, and unresolved conflict are explicit dispositions.

## Proposed gap record and stopping condition

`NS-EDIT-01` is a local proposal identifier, not an assigned canonical repository gap.

**Capability:** an authorized user can review, modify, and save a rich existing Northstar through the normal UI without losing unedited strategic intent, accepting stale overwrites, or receiving a false fully-published save result.

**Exit evidence:** a later authorized production-path verification demonstrates isolated snapshots and cancel-without-effects; the no-op and single-field preservation laws; stable IDs across reordering; linked generation retained; stale and invalid edits rejected before effects; and accurate committed/projection-pending recovery behavior. Documentation and current-state claims are updated only to what that evidence establishes.

The present research stops at this source-grounded proposal. No repository change, live vision import, code execution, or verification result is implied.
