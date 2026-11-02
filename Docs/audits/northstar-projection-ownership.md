# Northstar projection ownership: a saved vision can leave the boot vision active

**Status:** source-supported defect candidate and proposed contract; static inspection only. No tests, application runs, reproductions, benchmarks, model calls or implementation changes were performed.

**Source freeze:** theRebelliousNerd/codenerd at **e7189c64b8822ffaa6c7d44a79543fd23a9a0b78**, inspected 2026-11-02. All implementation links below use that exact revision.

## Decision in brief

Treat generated Northstar rows as replaceable, owned data, not immutable program clauses. A healthy existing workspace can boot with vision A in .nerd/northstar.mg, then successfully commit vision B through the wizard. The Guardian removes host facts and asserts B, but A remains embedded in each booted kernel's program. Both full evaluation and the optimized cone path retain that program source; the cone code expressly re-adds its InitialFacts. No write failure, concurrency or model misconduct is necessary. [Boot loader][load] [Replacement][refresh] [Host removal][retract] [Cone replay][cone]

The concrete consequence is not merely a stale file: an old constraint continues to derive wildcard injectable_context. The JIT reader accepts that wildcard and renders distinct old and new strings into its kernel-context atom, subject to existing bounds and later prompt admission. The current update receipt can nevertheless say committed, a status documented as meaning that every projection was installed. [Policy consumer][policy] [JIT consumer][jit] [Receipt contract][receipt]

This is a strategic-context integrity defect, not an established tool-permission bypass. The Guardian also has a direct cached-Vision path; that path can see B while the kernel still reasons with A. Fix ownership and publication truth without weakening permission checks or changing vision-edit authority.

## Why this is a distinct audit

The lossless-strategic-editing audit already covers lossy wizard DTOs, preservation of stable links and hidden fields, stale-base rejection, commit-before-projection ordering, and reported projection failures. Its as-built section credits those improvements and leaves cross-process export atomicity open. Those changes should remain. The present witness assumes that validation, base comparison, database save, file exports, host retraction and host assertion all succeed. It concerns an older, independent program support that those operations never target; the projection can therefore be wrong without adding an error to the receipt. [Prior Northstar audit][prior-edit]

The Mangle performance audit explicitly requires program facts in touched predicates to survive maintenance. That is appropriate for genuine program facts; disabling their replay globally would violate that contract. The earlier corpus journey records Northstar's schema-text loading but does not connect it to subsequent Guardian replacement. [Program-fact control][prior-perf] [Corpus journey][journey]

Targeted searches of Docs/audits, Docs/journeys, Docs/architecture/northstar and .quality_assurance for Northstar with initial/program facts, stale/retract, replay and projection found no exact Northstar ownership report. This is a bounded duplicate screen, not a claim that every sentence in the repository was read. A nearby implementation already recognizes the same general hazard: orientation_projection.go separates declarations from rows specifically because program clauses cannot be removed by fact retraction. That existing control strengthens the diagnosis and is a starting point, not a new claim about orientation. [Existing ownership split][orientation]

## Production chain, including the optimized kernel

| Boundary | Current behavior | Ownership implication |
|---|---|---|
| TUI bootstrap | Shared boot calls BootCortexWithConfig, obtains the federated kernel through sessionKernels, and only later attaches and initializes the Guardian. | The kernel has already loaded workspace artifacts before Guardian reconciliation. |
| Domain construction | NewDomainCortex builds the production domain shards. Each configured workspace shard calls NewRealKernelWithWorkspace, whose constructor loads files and evaluates. | This is not limited to a single-store test kernel. |
| Northstar export | RenderVisionMangle serializes Vision.ToFacts as ordinary ground clauses; WriteVisionMangle atomically writes the file. | The generated document is data in purpose, but syntactically valid program facts. |
| Boot loading | loadMangleFiles appends the Northstar file's res.Logic to schemasBuilder. Only special hybrid directives populate res.Facts. | Normal generated rows do not enter the retractable bootFacts path. |
| Analysis and evaluation | writeProgramLocked includes schemas; rebuildProgram installs the analyzed program. Full evaluation uses that cached program; cone evaluation copies InitialFacts and InitialFactTimes into its subprogram. | Rebuilding the materialized store does not retire the boot vision. |
| Guardian replacement | refreshKernelFacts removes its predicate family then asserts the current Vision.ToFacts. Cortex delegates predicate removal to the selected underlying kernels. | Both bulk and fallback removal edit host EDB, not the program's clauses. |
| Publication receipt | UpdateVisionFromBase returns committed if publishVision reports no errors. | Nil mutation errors are treated as projection success without establishing exclusive current-generation support. |

Sources: [shared boot][boot], [session kernel identity][session-kernel], [Guardian boot][guardian-boot], [production factory][factory], [shard construction][shard], [constructor][constructor], [export][export], [loader][load], [program construction][program], [full evaluation][full], [cone evaluation][cone], [Guardian replacement][refresh], [Cortex removal][cortex-retract], [receipt][receipt].

### Two ownership domains, not a failed delete

RemoveFactsByPredicateSet filters k.facts and only schedules a rebuild if a host row was removed. Retract's fallback follows the same model and returns nil when no host row matches. rebuild invalidates cached atoms and marks dirty predicates; it does not reread Northstar's file or remove clauses from schemas. [Bulk removal][retract] [Fallback removal][fallback] [Lazy rebuild][rebuild]

Accordingly, a fresh full fixpoint is not a remedy. It repopulates host facts in a new store and evaluates the retained program. Nor is turning off the cone optimization a remedy: the full and cone paths agree about the continued existence of A. The optimized code is especially explicit that it runs even for an empty rule cone in order to restore program facts removed during preparation. [Full path][full] [Cone path][cone]

Publishing B also rewrites .nerd/northstar.mg, but the running kernel retains the text it loaded. The chat watcher is not an implicit repair: it watches .nerd/mangle and optional internal/mangle, and chat describes it as validation through the primary kernel's repair interceptor, not a hot replacement of this sibling Northstar file. [Publication][publish] [Watch roots][watcher] [Watcher purpose][watcher-purpose]

## Static witnesses and expected observations

These are source-derived evaluation designs, not executed demonstrations.

### A. Ordinary save after an existing-workspace boot

1. A valid prior vision A is stored and exported. Give A a unique constraint such as “Use the legacy compatibility format.”
2. Boot the normal TUI with that .nerd/northstar.mg already present. A's generated ground clauses are analyzed as program facts.
3. Open the wizard and make an admitted edit B that replaces this constraint with a distinct current instruction. Keep the rest of the document valid; no linked-entity deletion is needed.
4. Save through commitNorthstarVision. It passes the running kernel to the Guardian and calls UpdateVisionFromBase. That commits the canonical vision, writes the exports, and refreshes host facts.
5. Query the Northstar constraints and injectable_context after lazy evaluation. The expected defect is the old program-supported constraint surviving beside the new host-supported constraint. If all mutation and export calls return nil, the receipt remains committed.

The wizard explicitly promises canonical replacement and wires the actual kernel into that call. The rendering path lists a normal saved result; only a projection_pending receipt defers a requested campaign. This does not prove that a downstream campaign violates a rule, but it does show that the receipt is not a reliable prerequisite for exclusive current vision context. [Wizard commit][wizard] [Receipt][receipt] [UI result and campaign deferral][save]

Set deduplication can conceal the problem while A and B are identical. The witness requires a changed or removed row. Removing an old row entirely is equally instructive: absence from B does not retract its program support. Repeated B-to-C saves need not accumulate every intermediate host vision; the persistent residue is the boot-loaded A plus the latest host projection, absent other producers.

### B. The documented JSON-edit-and-reboot path

The generated file tells operators to edit JSON or use the wizard and reboot. In an existing workspace, let JSON B be valid and newer than stored A. Shared boot first loads .mg A. Guardian.Initialize subsequently runs SyncVisionAuthority; the newer JSON branch saves B and writes .mg B, then refreshes host facts. The first running kernel can therefore still contain A even though disk reconciliation completed. [Export guidance][export] [Boot order][boot] [Guardian initialization][initialize] [Newer JSON reconciliation][sync]

A later clean restart from successfully rewritten B can remove the A residue because a new kernel analyzes B rather than A. That is recovery by constructing a new program, not evidence that the original projection update worked. The no-change save shortcut is also not a repair: when the candidate equals stored content, UpdateVisionFromBase returns unchanged before publishing anything. [Receipt implementation][receipt]

## Consumer impact and important limits

The narrowest decisive consumer is a constraint. Vision.ToFacts assigns constraint rows, prompt_northstar.mg injects them whenever northstar_defined exists, and the compiler queries injectable_context, admits “*”, canonicalizes strings and creates one mandatory kernel-context atom. Different constraint strings survive deduplication. Existing row/text/budget limits can truncate or reject context; they do not select the latest Northstar generation. Use a short, small witness well below limits to isolate ownership. [Fact encoder][facts] [Policy][policy] [Reader][jit]

Mission changes also affect planning/coding injection when their active-family guards hold, and project-purpose fallback for modules without an own purpose. These are additional consumers, not prerequisites for the unconditional constraint witness. [Mission and module policy][policy]

Qualifiers:

- A first boot with only JSON and no pre-existing .mg does not embed A. The existing stale-retraction integration test uses precisely this shape: its fixture writes JSON, creates the kernel, then initializes the Guardian. It verifies host replacement but not replacement after a generated file was loaded. [Fixture][test-fixture] [Existing test][test]
- If the generated file is absent or was not successfully loaded, the specific retained-program support is absent. Other errors are separate.
- An unchanged save is not evidence of inconsistency; identical tuples are logically deduplicated. It is a useful negative control.
- The direct Guardian CheckAlignment path snapshots g.vision, so this finding does not establish that every alignment check uses stale content. It can instead create disagreement between direct alignment and kernel-derived prompts. [Direct path][direct]
- Northstar alignment does not itself grant tool permissions. No protected action, gate bypass, exploit frequency or observed user harm is claimed.
- The conclusion does not require races, nor does it establish cross-shard snapshot atomicity. Those are separate publication concerns.
- Authored program facts must remain valid supports. A fix cannot simply teach generic Retract to erase arbitrary source clauses.

## Proposed contract and smallest credible implementation direction

Let N be the Guardian-owned projection predicate set, V the committed vision and P(V) its canonical ToFacts result. For a successful attached-kernel publication, the generated Northstar contribution must equal P(V), modulo declared fact normalization. No tuple may remain supported solely by a retired generated vision. Downstream closure should equal a fresh evaluation over current P(V), the same reviewed program and the same non-Northstar inputs. This extends the existing principle that Vision.ToFacts owns the projection shape; it does not introduce a parallel encoder. [Existing principle][principle]

This is an ownership contract, not a demand that all possible user-authored clauses be silently deleted. Establish which source owns generated Northstar predicates. If custom program rules or facts intentionally contribute to the same names, define and surface that composition or reject the conflict; do not advertise replacement while secretly retaining another authority.

Proposed work, not implemented:

1. **Separate generated rows from program text at boot.** Parse the known generated fact surface into host data, preserving schema declarations in the reviewed schema corpus. Orientation already demonstrates this boundary. Do not blindly reuse its line-oriented parser without checking strings, escaping and all Northstar value types. [Existing parser][orientation-parser]
2. **Preserve production routing.** Moving rows out of every shard's program changes replication. Inventory every Northstar consumer and explicitly route the current projection to the shards whose rules join it, or use the existing appropriate sharing mechanism. A single-store pass alone would be insufficient.
3. **Publish one identifiable generation.** Bind current generated rows and a projection receipt to the canonical vision digest. Install the complete projection through a suitable owned batch/publication boundary. A failed or partial publish must not report exclusive-current projection success.
4. **Retire legacy support.** New-process boot under the new loader can avoid embedding old rows. Existing kernels that already compiled them need an explicit safe program reconstruction/restart transition; merely asserting B again cannot repair A.
5. **Verify the promised result.** Prefer a generation/ownership invariant over a generic “some facts exist” check. If read-back is used, compare normalized sets and the publication identity, not only counts or nil errors.
6. **Keep boundaries intact.** Preserve validation, compare-and-swap editing, detached snapshots, permission enforcement, generated-file atomic writes, ordinary immutable program facts, and evaluation limits. Fixing this defect does not justify a full engine replacement or model-based conflict resolver.

## Cost model and failure modes of the fix

For F generated rows, S actual consumer shards and affected rule closure C, a data-owned projection needs parsing/validation O(F), storage/replication roughly O(SF), plus the engine's existing maintenance work over C. These are planning terms, not measurements; joins and recursion can dominate. A full program reconstruction on every save additionally pays analysis/stratification across domain kernels and can contend with readers. Prefer correct owned data publication, but measure it rather than promising speed.

The present residue can increase fact/context volume and give contradictory strategic instructions. Token or model-quality impact is unmeasured. A corrected projection might reduce prompt bytes, but semantic correctness is the acceptance criterion, not fewer tokens.

Key fix risks are incomplete consumer replication, destructive removal of genuinely authored facts, parser drift from ToFacts, a digest published before rows, mixed generations visible during a multi-shard update, stale compile snapshots, and “unchanged” receipts that mask an earlier incomplete projection. Keep the durable-authority and projection outcomes distinct; do not roll back a committed edit merely because a projection fails.

## Fair evaluation plan — all unrun

Compare the pinned implementation, a narrowly changed ownership loader/publication design, and an independently built fresh-current-vision oracle. Keep policy, workspace, non-Northstar facts, validation rules and compiler limits identical.

| Case | Required observation |
|---|---|
| Existing .mg A, admitted constraint replacement B | Old unique constraint absent; B present; complete affected closure matches the fresh B oracle. |
| Remove constraint/capability/link using an admitted update | No support from the old generated document survives; unrelated facts stay unchanged. |
| JSON-only first boot | Existing supported behavior preserved, not mislabeled as a previously failing case. |
| A saved without semantic change | No duplicate logical rows and no unnecessary generation or file churn. |
| Newer JSON B with .mg A at boot | First session reconciles to B, not only the second restart. |
| A→B→C and B restart | Distinguish permanent boot residue from intermediate host accumulation; all reads match the intended generation. |
| Production domain Cortex and RealKernel | Same relevant facts and consumers, including module fallback and planner/coder context; no missing split joins after routing changes. |
| Full versus eligible cone evaluation | Same complete closure; genuine program facts in touched predicates remain intact. |
| Invalid/stale-base edit | Existing no-effect rejection and conflict semantics preserved. |
| Export, assertion or evaluation failure | Honest pending/error state; no successful-current claim for a partial projection. |
| No-op save after a pending projection | Defined repair behavior rather than silently declaring publication healthy. |
| Custom source contribution to owned names | Explicit ownership policy; no accidental deletion or invisible composition. |

Inspect persisted vision, exported file, host EDB, retained program facts, evaluated rows and compiled context separately. Record return statuses and errors. Measure cold boot, edit publication, first evaluated read, peak memory, replicated rows and prompt bytes across small and large visions; report p50/p95 and failure cases, not only successful averages. If later evaluating agent behavior, hold models, tools, budgets, approvals and task acceptance fixed and report contradictory-context exposure and verified outcomes separately from cost.

## Research grounding

Hu, Motik and Horrocks study maintenance of Datalog materialisations when inputs change, including the trade-offs of counting and delete/rederive families. The relevant lesson is to preserve the right supports and compare complete materialisations; their performance results do not justify changing this repository's engine or predict speedups here. [Optimised Maintenance of Datalog Materialisations, AAAI 2018](https://arxiv.org/abs/1711.03987v2)

Foster and colleagues' view-update work motivates explicit agreement between an update and the view subsequently read. Applied here, “committed and projected” should mean that reading the generated Northstar projection reflects the admitted canonical vision. This is a proposed engineering law, not a claim that the current system is a lens or that the paper proves this implementation correct. [Combinators for Bi-Directional Tree Transformations, POPL 2005](https://www.cis.upenn.edu/~bcpierce/papers/newlenses-popl.pdf)

**Checkable exit:** a normal existing-workspace save or JSON reconciliation retires all support owned by the previous generated vision; production consumers read the current projection; genuine program facts and other authorities are preserved; and the receipt does not certify a projection that remains mixed.

[boot]: https://github.com/theRebelliousNerd/codenerd/blob/e7189c64b8822ffaa6c7d44a79543fd23a9a0b78/cmd/nerd/chat/session_shared_boot.go#L89-L105
[session-kernel]: https://github.com/theRebelliousNerd/codenerd/blob/e7189c64b8822ffaa6c7d44a79543fd23a9a0b78/cmd/nerd/chat/session_shared_boot.go#L364-L372
[guardian-boot]: https://github.com/theRebelliousNerd/codenerd/blob/e7189c64b8822ffaa6c7d44a79543fd23a9a0b78/cmd/nerd/chat/session_shared_boot.go#L270-L284
[factory]: https://github.com/theRebelliousNerd/codenerd/blob/e7189c64b8822ffaa6c7d44a79543fd23a9a0b78/internal/system/factory.go#L1355-L1448
[shard]: https://github.com/theRebelliousNerd/codenerd/blob/e7189c64b8822ffaa6c7d44a79543fd23a9a0b78/internal/core/kernel_shard.go#L61-L96
[load]: https://github.com/theRebelliousNerd/codenerd/blob/e7189c64b8822ffaa6c7d44a79543fd23a9a0b78/internal/core/kernel_init.go#L404-L445
[constructor]: https://github.com/theRebelliousNerd/codenerd/blob/e7189c64b8822ffaa6c7d44a79543fd23a9a0b78/internal/core/kernel_init.go#L110-L143
[program]: https://github.com/theRebelliousNerd/codenerd/blob/e7189c64b8822ffaa6c7d44a79543fd23a9a0b78/internal/core/kernel_eval.go#L88-L156
[full]: https://github.com/theRebelliousNerd/codenerd/blob/e7189c64b8822ffaa6c7d44a79543fd23a9a0b78/internal/core/kernel_eval.go#L287-L409
[cone]: https://github.com/theRebelliousNerd/codenerd/blob/e7189c64b8822ffaa6c7d44a79543fd23a9a0b78/internal/core/kernel_eval_cone.go#L333-L371
[retract]: https://github.com/theRebelliousNerd/codenerd/blob/e7189c64b8822ffaa6c7d44a79543fd23a9a0b78/internal/core/kernel_facts.go#L1041-L1069
[fallback]: https://github.com/theRebelliousNerd/codenerd/blob/e7189c64b8822ffaa6c7d44a79543fd23a9a0b78/internal/core/kernel_facts.go#L880-L909
[rebuild]: https://github.com/theRebelliousNerd/codenerd/blob/e7189c64b8822ffaa6c7d44a79543fd23a9a0b78/internal/core/kernel_eval.go#L449-L463
[cortex-retract]: https://github.com/theRebelliousNerd/codenerd/blob/e7189c64b8822ffaa6c7d44a79543fd23a9a0b78/internal/core/cortex_kernel.go#L594-L618
[refresh]: https://github.com/theRebelliousNerd/codenerd/blob/e7189c64b8822ffaa6c7d44a79543fd23a9a0b78/internal/northstar/guardian.go#L223-L280
[initialize]: https://github.com/theRebelliousNerd/codenerd/blob/e7189c64b8822ffaa6c7d44a79543fd23a9a0b78/internal/northstar/guardian.go#L177-L220
[export]: https://github.com/theRebelliousNerd/codenerd/blob/e7189c64b8822ffaa6c7d44a79543fd23a9a0b78/internal/northstar/bridge.go#L351-L376
[sync]: https://github.com/theRebelliousNerd/codenerd/blob/e7189c64b8822ffaa6c7d44a79543fd23a9a0b78/internal/northstar/bridge.go#L515-L558
[publish]: https://github.com/theRebelliousNerd/codenerd/blob/e7189c64b8822ffaa6c7d44a79543fd23a9a0b78/internal/northstar/guardian.go#L328-L360
[receipt]: https://github.com/theRebelliousNerd/codenerd/blob/e7189c64b8822ffaa6c7d44a79543fd23a9a0b78/internal/northstar/edit.go#L406-L494
[wizard]: https://github.com/theRebelliousNerd/codenerd/blob/e7189c64b8822ffaa6c7d44a79543fd23a9a0b78/cmd/nerd/chat/northstar_edit.go#L196-L225
[save]: https://github.com/theRebelliousNerd/codenerd/blob/e7189c64b8822ffaa6c7d44a79543fd23a9a0b78/cmd/nerd/chat/northstar_persistence.go#L28-L98
[facts]: https://github.com/theRebelliousNerd/codenerd/blob/e7189c64b8822ffaa6c7d44a79543fd23a9a0b78/internal/northstar/types.go#L56-L148
[policy]: https://github.com/theRebelliousNerd/codenerd/blob/e7189c64b8822ffaa6c7d44a79543fd23a9a0b78/internal/core/defaults/policy/prompt_northstar.mg#L168-L248
[jit]: https://github.com/theRebelliousNerd/codenerd/blob/e7189c64b8822ffaa6c7d44a79543fd23a9a0b78/internal/prompt/compiler_kernel.go#L207-L259
[watcher]: https://github.com/theRebelliousNerd/codenerd/blob/e7189c64b8822ffaa6c7d44a79543fd23a9a0b78/internal/core/mangle_watcher.go#L47-L103
[watcher-purpose]: https://github.com/theRebelliousNerd/codenerd/blob/e7189c64b8822ffaa6c7d44a79543fd23a9a0b78/cmd/nerd/chat/session_shared_boot.go#L144-L148
[direct]: https://github.com/theRebelliousNerd/codenerd/blob/e7189c64b8822ffaa6c7d44a79543fd23a9a0b78/internal/northstar/guardian.go#L367-L393
[test]: https://github.com/theRebelliousNerd/codenerd/blob/e7189c64b8822ffaa6c7d44a79543fd23a9a0b78/internal/northstar/kernel_integration_test.go#L157-L199
[test-fixture]: https://github.com/theRebelliousNerd/codenerd/blob/e7189c64b8822ffaa6c7d44a79543fd23a9a0b78/internal/northstar/kernel_integration_test.go#L22-L52
[orientation]: https://github.com/theRebelliousNerd/codenerd/blob/e7189c64b8822ffaa6c7d44a79543fd23a9a0b78/internal/core/orientation_projection.go#L56-L61
[orientation-parser]: https://github.com/theRebelliousNerd/codenerd/blob/e7189c64b8822ffaa6c7d44a79543fd23a9a0b78/internal/core/orientation_projection.go#L112-L141
[prior-edit]: https://github.com/theRebelliousNerd/codenerd/blob/e7189c64b8822ffaa6c7d44a79543fd23a9a0b78/Docs/audits/northstar-lossless-strategic-editing.md#L156-L196
[prior-perf]: https://github.com/theRebelliousNerd/codenerd/blob/e7189c64b8822ffaa6c7d44a79543fd23a9a0b78/Docs/audits/mangle-kernel-performance.md#L140-L144
[journey]: https://github.com/theRebelliousNerd/codenerd/blob/e7189c64b8822ffaa6c7d44a79543fd23a9a0b78/Docs/journeys/M1-mangle-corpus-usage.md#L30-L43
[principle]: https://github.com/theRebelliousNerd/codenerd/blob/e7189c64b8822ffaa6c7d44a79543fd23a9a0b78/Docs/architecture/northstar/04-ARCHITECTURAL-PRINCIPLES.md#L26-L28

