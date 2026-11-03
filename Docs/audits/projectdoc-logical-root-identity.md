---
doc-class: deep-dive
subsystem: projectdoc-system-session-campaign-northstar
implementation-status: proposed-refinement
evidence-status: source-only
last-verified: 2026-11-02
verified-against: 69ae63408d576d594d5b4b14652a740407d80482
verification-method: static-source-inspection
supersedes: []
---

# Preserve the logical root of a fallback project document

## Decision and evidence boundary

Treat the supported **.nerd/nerd.md fallback as the workspace's logical root document**, while retaining its actual filesystem path for provenance. A document's storage directory is not necessarily the scope it governs.

At the pinned revision, discovery already supports this layout. LoadAll first calls Load, which selects root nerd.md or, when that is absent, .nerd/nerd.md. It preserves the selected file's workspace-relative physical path. Three downstream consumers then reconstruct semantic scope from that path:

1. The shared system factory recognizes only a bare nerd.md or a document whose directory is "." as the root prompt document. A valid fallback is loaded and projected into kernel facts, but the factory leaves its projectDoc pointer nil.
2. Northstar fact projection uses the document's physical parent directory as ModulePath, so the fallback's purpose and requirements are keyed by ".nerd", unlike the "." key produced by root nerd.md.
3. Recurse's worker North Star selector also recognizes only directory "." as root. For an ordinary subsystem without its own North Star, a declaration stored in the supported fallback does not supply the root fallback text to an extend attempt.

These are source-supported identity and context-delivery defects, not demonstrated model mistakes or production incidents. **They do not establish a write-protection bypass or disappearance of declared gates.** The factory asserts all successfully loaded document facts before choosing the prompt pointer. Session, VirtualStore, and registry write protections use kernel facts; gates.Detect, CriticalPaths, DocPaths, and recurse's own forbidden-path document use Load directly and therefore retain the fallback. [Discovery and loading][load] [Factory assertion and root selection][factory] [Northstar projection][facts-ns] [Recurse selector][recurse-ns]

All implementation links in this audit pin public repository theRebelliousNerd/codenerd at **69ae63408d576d594d5b4b14652a740407d80482**. Only source and existing test definitions were inspected. No repository program, build, test, model inference, training, benchmark, or failure injection was run. The proposed witness and acceptance plan below remain **unrun**. Only this standalone audit draft was written; no implementation or repository files were changed.

The checked-in codeNERD workspace itself has root nerd.md. The minimal witness concerns another supported workspace layout, or relocation of a valid root document to its explicitly supported fallback. It is not a claim that the checked-in repository currently boots without its project instructions. [Checked-in root document][repo-doc]

## Architectural intent and bounded nonduplication

Repository guidance requires creative work to remain with the model while Mangle supplies executive authority, context to remain bounded and revision-aware, and production wiring to be traced rather than inferred from a helper's existence. The scoped system and session guidance also separates prompt content from capability and exact permission. A missing instruction section must not be repaired by widening tools, skipping denials, or turning prose into authority. [Repository guidance][guidance] [System wiring guidance][system-guidance] [Session contracts][session-guidance]

The architecture documentation rules distinguish intended capability, current implementation, and verification. They explicitly reject claiming that existing code proves production reachability. This audit follows that distinction: fallback discovery is implemented; root identity is not conserved through all consumers; the repair and end-to-end evidence are proposed. [Architecture rules][architecture-guidance]

The projectdoc vision explicitly names root nerd.md with .nerd/nerd.md as its fallback. Its older current-state, gap, and wiring documents were read as context, not accepted as current runtime evidence. In particular, the old G6 describes missing Spawner forwarding. That forwarding now exists: the factory supplies the pointer to the Spawner, which supplies nonnil pointers to newly created executors. The present defect is **upstream misclassification of the supported root**, not another missing SetProjectDoc call. Similarly, old descriptions of fail-open project gates or dormant-only protection must not override current fail-closed source. [Projectdoc target][vision] [Older gap description][old-gaps] [Current factory wiring][factory-wire] [Current spawn forwarding][spawn]

The pinned audit inventory, audit queue, and the following closely related reports were screened:

| Existing work | Existing subject | Why this audit is not the same finding |
|---|---|---|
| specialist-roster-workspace-identity | A roster lookup using process CWD rather than the session workspace | Here the correct workspace is already supplied; one supported document inside it is assigned the wrong logical role. The queue records the roster issue as fixed. |
| guardian-module-alignment-scope | Explicit affected paths rather than prose subjects in Guardian alignment | Current Guardian has typed scope and coherent snapshots. This audit concerns incorrectly scoped producer facts before that machinery runs, and does not revive the old subject-string defect. The queue records that issue as fixed. |
| northstar-lossless-strategic-editing | Lossless wizard/canonical-vision updates and base-bound edits | No vision edit, save, DTO conversion, or model-generated requirement is needed for this witness. |
| recurse-gate-contract-continuity | Preserving command environment and accepted exits through gate-to-attempt-to-acceptance handoffs | Here direct gate discovery still reads the fallback correctly. The existing report has an as-built repair section; this audit does not re-report its older baseline as current. |
| gate-comparison-domain-admission and test-gate-memoization-identity | Comparable measurement domains and safe reuse of test evidence | No gate removal, narrower test selection, cached result, or measurement comparison is required to lose this prompt or root North Star. |

[Audit queue: roster][queue-roster] [Audit queue: Guardian][queue-guardian] [Roster audit][related-roster] [Guardian audit][related-guardian] [Strategic editing audit][related-edit] [Gate handoff as built][related-gate] [Gate domain audit][related-domain] [Memoization audit][related-memo]

This is a bounded nonduplication review of the pinned inventory and these related sources, not a claim to have searched every historical branch or every private issue. No inspected report establishes the complete fallback-root identity contract proposed here.

## What is already true

### Discovery supports a root in a non-root storage directory

Find tries workspace/nerd.md first, then workspace/.nerd/nerd.md. Load reads and strictly parses the selected file, then records its actual relative path in Document.Path. LoadAll calls Load before its module walk and appends that returned document first. Thus, with only the valid fallback present:

- Load returns a document whose Path is ".nerd/nerd.md".
- LoadAll appends that document as its root document.
- Module discovery does not need to discover or admit the .nerd directory for this root to be loaded.
- The root file wins if both supported locations exist.

These conclusions follow directly from the selected code, under the witness's ordinary readable-file and successful-workspace-membership assumptions. The existing Find precedence test also declares the fallback intentional, although that test was not run here. [Discovery and root append][load] [Find test][find-test]

Membership excludes .git and .nerd from the module population. This is useful and should remain: recognizing one designated fallback file as the root does not authorize treating arbitrary files beneath .nerd as source modules. LoadAll's explicit root load is the appropriate separate entry. [Module walk][loadall-walk] [Reserved-directory exclusion][membership]

### Machine facts and the advisory rendering are separate deliveries

Facts emits project_doc(Path, Schema), project name/language, canonical command facts, command environment, forbidden-path rules, required steps, conventions, and optional Northstar purpose/requirements. Markdown body is deliberately not projected into facts. The factory collects facts from **every** loaded document and calls kernel.LoadFacts before its root-selection loop. [Fact projection][facts] [Factory order][factory]

PromptSection renders a separate project-instructions section: project name, canonical commands and environment, protected paths, required steps, conventions, declared gates and body. Calling it on nil returns an empty string. The executor's withProjectInstructions leaves the compiled prompt unchanged when the section is empty; otherwise it appends the section. This is an explicit append after JIT compilation, not an atom-selection failure and not a claim that this source is scored by JIT. [Rendering][render] [Append boundary][append] [Production session assembly][session-assembly]

## The boundary that loses root identity

### A. Shared boot recognizes a storage layout instead of the selected role

The factory's root-selection predicate accepts a document when Path equals projectdoc.FileName, or filepath.Dir(Path), slash-normalized, equals ".". For ".nerd/nerd.md", neither condition holds. A valid root fallback therefore passes discovery, parsing, and fact loading but is not assigned to bctx.projectDoc. The boot log takes its "[no workspace root document]" branch despite LoadAll having loaded the designated workspace root fallback. This is not the malformed-document warning path. [Factory root selection and log][factory]

The comments explain the legitimate motivation: a module-only workspace must not promote the first module's prose into the global system prompt. That invariant must be preserved. Simply selecting docs[0] again is not a complete repair because LoadAll can return only module documents when no root exists. The factory's existing regression test explicitly protects that case. [Module-only regression test][factory-tests]

This distinguishes two states that the current pointer cannot distinguish after selection:

- **No designated root exists:** module facts may still be loaded, and global root prose must remain absent.
- **A designated fallback root exists:** its facts and root prose should be available just as for root nerd.md, with its actual source path retained.

Both currently leave projectDoc nil under the affected factory predicate.

### B. Parent and spawned requests faithfully propagate the wrong absence

The factory calls SetProjectDoc with bctx.projectDoc on both its session executor and its Spawner. Spawner creates a new SubAgent with the shared kernel and related runtime dependencies, then forwards its projectDoc only if nonnil. Those links are present; the fallback pointer has already been lost. [Factory consumer wiring][factory-wire] [Spawner forwarding][spawn]

The executor subsequently calls withProjectInstructions while assembling the system prompt, and passes that assembled prompt into runToolLoop. Because nil PromptSection is empty, this route contains no rendered fallback section. The missing content includes advisory body and the convenient prose restatement of frontmatter; the underlying successfully asserted facts do not disappear with it. [Nil renderer][render] [Append boundary][append] [Actual loop handoff][session-assembly]

**Source conclusion:** absent any other injection or independent read of the same content, this production assembly path does not append the fallback document to parent or spawned session prompts.

**Not established:** that every possible request path omits it, that no independent retrieved source supplies equivalent information, that the model necessarily chooses a wrong command, or that a task necessarily fails. A model can discover a rule through other context or a denial. Such compensation does not make the designated project-document delivery contract correct.

### C. Northstar projection changes the declared module key

Facts computes the Northstar ModulePath as filepath.Dir(Document.Path). It supplies "." only when the resulting string is empty; the ordinary root name already yields ".". Consequently identical Northstar frontmatter is projected differently after moving the selected document:

| Physical source | Present source fact | Northstar purpose/requirement scope |
|---|---|---|
| nerd.md | project_doc("nerd.md", schema) | "." |
| .nerd/nerd.md | project_doc(".nerd/nerd.md", schema) | ".nerd" |
| pkg/a/nerd.md discovered as a module | project_doc("pkg/a/nerd.md", schema) | "pkg/a" |

The first column should affect provenance. It should not turn the second supported root layout into a module-specific declaration. The source and existing root-scope test establish the exact "." versus ".nerd" difference; the test constructs Path "nerd.md" and does not exercise the fallback. [Northstar projection][facts-ns] [Root-scope test][northstar-test]

Current module policy derives effective purpose using the module key as supplied. It does not know that ".nerd" came from a root fallback. A purpose declaration produces effective_module_purpose for that key; a requirements-only module can inherit the independently stored project mission. **This audit does not equate root nerd.md's Northstar block with the separate canonical Northstar vision or claim that moving the file deletes that vision.** [Module policy][module-policy]

### D. Recurse can read the fallback for protection but not for worker purpose

RunRecurseCycles directly calls projectdoc.Load(root) and stores the result in r.doc. Later, its separate northStar(node) helper calls LoadAll and marks a document as root only if the document's physical parent is "."; a document in a path exactly named by node.Paths may be an own declaration. It returns the first nonempty Northstar text from own then root. [Recurse initialization][recurse-load] [Recurse selector][recurse-ns]

For an ordinary node such as pkg/a, with no matching module document and only .nerd/nerd.md declaring a purpose, neither own nor root is selected. The result is empty. The attempt constructor assigns this result to RecurseAttempt.NorthStar. recurseImproveTask renders that field only when nonempty and the angle is "extend". [Attempt assignment][attempt] [Extend consumer][attempt-consumer]

The direct consequence is loss of the designated root fallback text from **this extend-task context channel**. It is not evidence that all recurse angles require that text, that an extend attempt actually ran, or that the attempt was wrongly kept. A matching node's own nonempty North Star can mask the omission, while a root nerd.md is the working control.

### E. Guardian is a scope distortion, not a universal root-inheritance claim

The current Guardian resolves typed scope from a coherent fact snapshot. Its module view intentionally excludes the canonical "." root key, preserving ModuleForPath's project-root exclusion. It does **not** exclude ".nerd". Therefore fallback-produced Northstar facts can be treated as an ordinary ".nerd" module rather than as the logical root document. [Guardian module view][guardian-view] [Legacy-compatible module lookup][module-helper]

The selector uses segment-aware path relationships. For ordinary unrelated files, a ".nerd" module does not match. For an affected path beneath .nerd, or a broad directory scope "." that collects nested modules, it can be selected; actual assessment further depends on snapshot validity, effective purpose, configured vision/client, and the rest of Guardian admission. These are source-supported conditional consequences of the wrong producer key, not a measured alignment verdict. [Guardian typed-scope selection][guardian-select]

Do not claim that the normal "." root Northstar already governs every Guardian target and the fallback suddenly removes that coverage: the normal "." module key is deliberately excluded. Do not merge this defect with the already-addressed prose-subject scope issue. The invariant needed here is **equivalence between the two supported root locations under the current Guardian contract**, not an unauthorized change to that contract.

## Controls that remain in force

The defect needs a successful parse and successful kernel fact assertion. The factory has separate warning paths for invalid LoadAll output or rejected facts; those are distinct from the valid-fallback witness and must not be conflated with it. [Factory errors][factory]

| Control or consumer | Source route at the pin | Why the root-prompt omission does not remove it |
|---|---|---|
| Declared commands and gates | gates.Detect calls Load(root), then fromNerdMD converts commands and gates, preserving environment and accepted exit codes | It receives the fallback document directly and does not use bctx.projectDoc. Recurse calls this detector. |
| Session forbidden paths | projectForbidsWrite extracts recognized targets and queries ForbiddenByKernel; runWriteGuards is called before tool execution | It reads successfully asserted project_forbidden_path facts, not the advisory pointer. Missing authority, missing targets, and query errors refuse recognized writes. |
| VirtualStore write actions | executeAction calls projectForbidsWrite before capturing/editing state | The gate uses the kernel and refuses missing targets or authority for recognized write actions. |
| Modular and global registry writes | HydrateModularTools installs/binds the guard; Registry.Execute invokes it before tool.Execute | The bound guard queries the kernel for every extracted write target and has fail-closed error handling. |
| Critical-path declarations | CriticalPaths calls Load directly; Dreamer construction calls CriticalPaths | The fallback still supplies its declared list when valid and readable. |
| Documentation-path declarations | DocPaths calls Load directly | The same valid fallback remains visible without the prompt pointer. |
| Recurse forbidden changes | r.doc comes from direct Load; forbidden checks it and independently excludes .git/.nerd | The worker North Star selection defect does not remove this independent document or reserved-directory rule. |

[Gate discovery][gate-detect] [Recurse gate call][recurse-gates] [Session project gate][session-gate] [Guard dispatch][guard-dispatch] [Session dispatch order][dispatch] [VirtualStore gate][vs-gate] [VirtualStore dispatch][vs-dispatch] [Registry binding][registry-bind] [Registry dispatch][registry-dispatch] [Registry project guard][registry-guard] [Critical and docs loading][critical-docs] [Dreamer consumer][dreamer] [Recurse prohibition][recurse-forbid]

The session test TestProjectForbidsWrite_EnforcedWithoutThePromptCopy deliberately asserts facts without SetProjectDoc and expects protected writes to be denied while the prompt remains unchanged. That is exactly the separation this audit preserves. It is existing test intent, **not a pass receipt from this review**. [Protection-without-prose test][no-prompt-test]

These controls constrain the finding. They are not a proof that all policy in the repository is correct, and this audit does not newly certify every possible effect path. Most importantly, a nil projectDoc must never be described as equivalent to no loaded project policy.

## Minimal source witness, not an executed reproduction

Use two otherwise identical fresh temporary workspaces with a valid supported language marker and ordinary readable source. Keep startup and kernel setup healthy. Do not use actual credentials or sensitive files.

- **Control R:** one selected root document at nerd.md.
- **Treatment F:** the exact same document bytes only at .nerd/nerd.md, with no root nerd.md.
- Both have an ordinary node pkg/a with no module-level nerd.md.
- The document contains a distinctive small body sentinel, a canonical test command, one forbid rule protecting a disposable sentinel file, a critical-path entry, a docs-path entry, one declared gate, and a Northstar purpose plus one requirement.
- Use safe deterministic stub checks for later execution, not a declaration that weakens production policy. Keep all prompt content well below existing render bounds so truncation cannot explain omission.

Hold workspace meaning, modules, task, available tools, kernel corpus, source revision, and model adapter fixed. The expected observations from current source are:

| Observation | Control R | Treatment F |
|---|---|---|
| Selected physical source | nerd.md | .nerd/nerd.md |
| LoadAll recognizes a designated root before module discovery | Yes | Yes |
| Kernel source fact and forbid fact after successful boot | Present | Present |
| Factory prompt root pointer | Selected | Nil |
| Actual parent/spawned request's dedicated project section | Contains sentinel, subject to normal request execution | Not appended through this channel |
| Northstar fact scope | "." | ".nerd" |
| Recurse root North Star for pkg/a, then extend context | Root text available | Empty through this selector |
| gates.Detect's declared gate contract | Available | Available |
| CriticalPaths and DocPaths | Declared entries | Same declared entries |
| Properly authorized attempt to write protected sentinel | Project gate must still deny | Project gate must still deny |

The table is a **source prediction**, not logged output. The request rows require the session to reach prompt assembly and the child to actually execute; construction alone is insufficient. A probe that merely calls Load or PromptSection directly with the returned fallback document would miss the factory defect because both helpers individually work.

A second treatment should include a real pkg/a/nerd.md with its own distinctive prose and North Star. Its body must not become global root instructions; its own North Star may legitimately override the root for that node. This protects against a repair that reintroduces the old module-promotion bug or changes intended module precedence.

## Proposed identity and publication contract

The following requirements are proposed, not existing API guarantees.

### ROOT-ID-1: separate physical provenance from governing scope

The loader must return, or expose through a single authoritative resolver, both:

- the actual selected source path, including ".nerd/nerd.md" when that is what was read;
- whether the document is the designated workspace root or an admitted module, and its logical scope ("." for either supported root location, the relative module directory for ordinary module documents).

The source path remains available to project_doc, prompt headers, parse diagnostics, and user navigation. Do not fix the problem by overwriting Path with "nerd.md": that would hide which file was read, mislabel diagnostics, and make two distinct sources indistinguishable to consumers needing provenance.

A resolved root-plus-modules envelope is one suitable design. A centralized semantic-root/scope helper is a smaller option if it covers every construction path and has an explicit contract for manually parsed or synthesized Document values. Either way, consumers must not independently invent filepath.Dir tests or depend on the first element of an untyped slice.

### ROOT-ID-2: preserve existing discovery and non-promotion behavior

Root nerd.md retains precedence over the fallback. Absence of both yields no root. An invalid selected root must not silently fall through to a different valid document. Ordinary module-only workspaces keep their facts without globally promoted prose. Arbitrary .nerd descendants remain excluded from module discovery. The change does not broaden filesystem membership, path permission, or instruction-file discovery.

These invariants must be tested together. Fixing fallback recognition by admitting the entire .nerd directory, by choosing docs[0] unconditionally, or by accepting any basename nerd.md as root fails this contract.

### ROOT-ID-3: carry the same admitted identity into all semantic consumers

After successful fact admission, the shared factory must attach the selected logical root to parent and Spawner. Northstar projection must use logical scope rather than storage directory. Recurse must select root and node documents using the same resolved role information, preserving own-before-root preference and its current nonempty-content behavior.

The existing raw source path is sufficient for provenance, but insufficient as the only semantic identifier. This repair should not create a second prompt or policy loader that can disagree with LoadAll about which document won.

For Guardian, the correction is to receive "."-scoped root facts from either supported root location and preserve its existing project-root exclusion. Changing Guardian to apply those root facts to every affected path is a separate design decision, not part of closing this defect.

### ROOT-ID-4: admission and diagnostics must describe the actual result

Successful fallback discovery and fact admission must not emit "no workspace root document" merely because the file lives beneath .nerd. Diagnostics should distinguish root absent, selected source invalid, fact admission failed, and root admitted with a concrete source path.

Do not accidentally attach advisory prose after a failed fact assertion while describing its rendered rules as enforced. Preserve the current sequencing or explicitly design and label a degraded state. A rendering-only success is not a machine-policy installation receipt.

### ROOT-ID-5: relocation equivalence, except for provenance

Moving identical valid document bytes between the two supported root locations, when only one is present, must preserve:

- selected root role and logical scope;
- admitted semantic frontmatter facts, except the source-path record;
- dedicated root instruction text, except its source header;
- child inheritance;
- root fallback selection in recurse;
- current Guardian scope classification;
- gate, critical-path, documentation-path, and forbidden-write behavior.

This is a semantic equivalence law, not byte equality of all prompts, logs, identities, or caches: source-path headers and diagnostics should truthfully change. If any cached or persisted artifacts depend on the old ".nerd" module key, their refresh/invalidation behavior needs a focused review. This audit did not establish that such persistence exists, so it does not prescribe a speculative migration.

### ROOT-ID-6: preserve authority, bounded context, and ownership

No repair may treat advisory prose as permission, bypass exact action authorization, widen the capability catalog, weaken write guards, drop root precedence, or remove project render bounds. New model-facing behavior must respect the repository's JIT-first contract; the narrow repair can preserve the current explicit project-content append without inventing a new prompt subsystem.

Avoid cross-workspace globals or implicit current-directory searches. The loader already receives the workspace. The identity correction should conserve that owner rather than introducing a new path-discovery mechanism.

## Costs and tradeoffs

This is primarily a correctness repair. No speedup or token saving was measured.

**Relevant primary research, with a narrow role:** Liu et al.'s *Lost in the Middle* evaluated multi-document question answering and key-value retrieval and found that relevant-information position could materially change performance in the tested long-context models. That motivates checking the actual outbound request and separating content inclusion from effective use. It does **not** establish measured relevance, an attention failure, a quality improvement, or superior prompt placement for codeNERD or its current providers. This finding is complete omission through a designated delivery channel, not a measured positional-attention defect; preserve the current insertion point for the minimal repair and evaluate placement separately if it is changed. [Original research, version 3](https://arxiv.org/abs/2307.03172v3).

- **Prompt cost can increase.** Current affected parent/child requests omit the section. Correct delivery adds its rendered content back. The body bound is 32 × 1024 and section bound 48 × 1024 in the existing code, with a 50-entry limit for restated lists. Despite "Chars" names, ClampText uses Go string lengths/slices with UTF-8 edge repair, and its marker can extend beyond the nominal retained-content limit. These are not exact tokenizer budgets or a promise that the final request has a hard 48 KiB ceiling. Preserve the controls and measure actual outbound input tokens. [Render limits][render-limits] [Clamping semantics][clamp]
- **Potential avoided work is conditional.** Foreknowledge of protected paths, canonical commands, and project guidance may avoid extra discovery calls or predictable rejected writes. Losing those instructions may cost a turn; restoring them does not guarantee any specific reduction. The repository's own renderer explains this rationale. [Restatement rationale][render-rationale]
- **No new model call is intrinsically needed.** Selecting a role already known during LoadAll and forwarding it is deterministic. This does not rule out costs from any broader redesign.
- **Repeated I/O should not be added accidentally.** The factory already calls LoadAll, and recurse's northStar helper calls it for each lookup. Fixing identity does not require an additional recursive scan per consumer. Reusing one admitted collection can reduce redundant work, but introducing caching is a separate freshness/ownership decision and is not required for the minimal repair. [Root collection][loadall-walk] [Recurse repeated loading][recurse-ns]
- **Scope metadata has a compatibility cost.** Adding an explicit role/scope field or collection wrapper requires updating construction sites and test fixtures; a helper using only recognized physical names is less invasive but risks perpetuating implicit assumptions. Prefer a small centralized contract whose absence is observable over many duplicated special cases.
- **Fact-key correction can change downstream observations.** ".nerd" becomes "." for the fallback's Northstar records; that is intentional parity with the supported root layout. Preserve source identity separately and verify consumers rather than treating changed query output as incidental.
- **Safety checks should not become a benchmark casualty.** Compare complete guarded execution paths. A faster path that omits facts, guards, scope witnesses, or root text has not delivered the same capability.

Measure boot/load wall time and allocations, filesystem walk/read counts, parent and spawned outbound prompt bytes/tokens, number of additional discovery/denied-write rounds, total task latency, and admitted semantic equivalence. Keep correctness outcomes separate from cost outcomes. Report failed, skipped, and inconclusive tasks rather than comparing only successful survivors.

## Fair end-to-end verification plan — all unrun

### Stage 1: focused deterministic identity cases

Inspect and extend tests for selected root role, physical path, and logical scope:

1. Root only; fallback only; both present with distinct sentinels; neither present; module-only.
2. Fallback plus multiple real modules, including deterministic ordering.
3. Invalid selected root with valid alternate present: retain explicit invalid-selection behavior, not silent substitution.
4. Invalid fallback and invalid module: preserve the documented error/admission contract without inventing a partial-success receipt.
5. A non-root hidden directory document and reserved .nerd descendants: retain existing membership rules.
6. Empty and relative workspace input, platform separator behavior, and canonical absolute workspace ownership where supported.
7. Synthesized/Parse-only documents: whatever semantic-scope API is chosen must define their behavior, not accidentally grant root status from an empty Path.
8. Root precedence and module own-before-root Northstar behavior.

Assertions must include project_doc provenance, Northstar "." scope, unchanged forbid/command facts, and absence of spurious ".nerd" module records for a designated root fallback. A test that only sees a nonnil Load result does not establish downstream correctness.

### Stage 2: normal boot to actual parent and child requests

Build temporary fixture workspaces and use the production shared factory/boot path, a real configured kernel corpus, normal JIT compilation, and a scripted recording LLM adapter. Preserve CodeDOM/world-model context wiring where the entry path normally enables it. Do not bypass prompt compilation, directly SetProjectDoc in the test, or mutate the factory's private field to force the desired outcome.

Run the same bounded task under Control R and Treatment F. Capture the **actual outbound parent request**, then exercise actual Spawner construction and child execution and capture its request. Assert the dedicated root section contains the sentinel exactly once, records the correct physical source, and has equivalent semantic content. Also assert the real module's distinct prose is not promoted globally.

If the production entry cannot be instantiated in the test environment, report it blocked or unrun with the exact prerequisite. A helper-only pass must remain helper-only.

### Stage 3: independently establish the retained controls

Using disposable protected targets and the normal authorization envelope, attempt one recognized protected write and one permitted unprotected control through each relevant supported route: session, VirtualStore, and bound modular/global registry. Supply legitimate authority for the control so an unrelated constitutional rejection cannot masquerade as proof of project protection. Assert the protected target's bytes remain unchanged and the refusal identifies project protection.

Separately inject an unavailable kernel or query error at the supported test seam and assert recognized writes fail closed. Exercise direct gates.Detect, CriticalPaths, and DocPaths under both layouts and compare returned contracts. Check precedence with distinct declarations in both physical files.

Do not claim that no file changed because a model happened not to request a write. Use deterministic proposals and record which guard actually denied them. Do not loosen a guard merely to make a test reach a later stage.

### Stage 4: recurse and Guardian consumers

Drive a real recurse attempt-construction path in an isolated disposable workspace with a node lacking its own document. Ensure an extend angle is actually selected and capture the task handed to execution. Assert the root purpose and requirement enter that task for both layouts. Then add a matching module North Star and require identical own-before-root behavior. Confirm declared gates and forbidden-change rules continue to apply.

For Guardian, provide the same coherent facts and typed scopes to the existing alignment path. Compare an ordinary source-file scope, a ".nerd" path scope, and a broad "." directory scope under both root layouts. Require parity with the current "." root exclusion and no phantom fallback ".nerd" module. Use deterministic adapter responses only to observe request contents and coverage state; do not report them as evidence of real model alignment quality.

The recurse test must not stop after calling northStar directly, nor infer a keepable result from presence of text. Preserve outer gate remeasurement, forbidden-write handling, and current acceptance requirements. A prompt-delivery pass does not certify autonomous improvement quality.

### Stage 5: ordinary workload and honest cost comparison

Only after deterministic parity is established, run paired authorized tasks through ordinary production entry paths, keeping model/provider configuration, tool availability, task inputs, source content, and document size controlled. Root versus fallback is a layout comparison; it should not also alter the model, tool budget, membership policy, or verification gates.

Measure outbound tokens and total work, not just compile-time estimates. Use small typical documents plus boundary-sized bodies and lists; verify truncation markers remain visible and kernel protection covers rules omitted from the prose rendering. Report model variability, task failures, and extra turns, including any case where adding correct instructions increases tokens but improves completion fidelity.

No live-model run is necessary to establish the deterministic identity defect. Live runs are needed only for claims about quality, turns, latency, or cost. No such claim is made here.

## Existing evidence and exit criteria

The inspected tests cover useful pieces but not the complete witness:

- Find tests fallback discovery and root precedence.
- LoadAll tests root/module ordering, module membership, reserved-directory exclusion, errors, absence, and POSIX paths.
- Factory tests prevent module-only promotion and choose a normal root over modules.
- Northstar fact tests establish "." for a manually constructed root Path.
- Session tests establish that the advisory pointer and machine protection are independent.

None of these inspected definitions connects a fallback-only layout through the shared factory, actual parent/spawned request, semantic Northstar facts, and recurse extend context. That is a bounded statement about inspected tests, not an assertion that no other coverage exists. [Find coverage][find-test] [LoadAll coverage][loadall-tests] [Factory coverage][factory-tests] [Northstar coverage][northstar-test] [Session separation coverage][no-prompt-test]

Close ROOT-ID only when the supported relocation law holds through those normal consumers and the retained-protection controls also pass. Record the implementation revision and exact evidence scope. Keep documentation status at proposed/source-only until implementation exists, and keep runtime verification pending until actually run. A local helper fix, a copied prompt string, a green parser test, or the absence of a crash is not end-to-end closure.

## Source links

All links below use the same inspected commit, including related documents read for overlap. Their historical claims remain historical; current runtime claims in this audit rely on the implementation sources linked above.

[load]: https://github.com/theRebelliousNerd/codenerd/blob/69ae63408d576d594d5b4b14652a740407d80482/internal/projectdoc/nerdmd.go#L253-L304
[loadall-walk]: https://github.com/theRebelliousNerd/codenerd/blob/69ae63408d576d594d5b4b14652a740407d80482/internal/projectdoc/nerdmd.go#L306-L388
[factory]: https://github.com/theRebelliousNerd/codenerd/blob/69ae63408d576d594d5b4b14652a740407d80482/internal/system/factory.go#L1551-L1608
[facts-ns]: https://github.com/theRebelliousNerd/codenerd/blob/69ae63408d576d594d5b4b14652a740407d80482/internal/projectdoc/facts.go#L118-L136
[recurse-ns]: https://github.com/theRebelliousNerd/codenerd/blob/69ae63408d576d594d5b4b14652a740407d80482/internal/campaign/recurse_cycle.go#L1321-L1351
[repo-doc]: https://github.com/theRebelliousNerd/codenerd/blob/69ae63408d576d594d5b4b14652a740407d80482/nerd.md#L1-L40
[guidance]: https://github.com/theRebelliousNerd/codenerd/blob/69ae63408d576d594d5b4b14652a740407d80482/agents.md#L1-L117
[system-guidance]: https://github.com/theRebelliousNerd/codenerd/blob/69ae63408d576d594d5b4b14652a740407d80482/internal/system/agents.md#L10-L30
[session-guidance]: https://github.com/theRebelliousNerd/codenerd/blob/69ae63408d576d594d5b4b14652a740407d80482/internal/session/agents.md#L1-L21
[architecture-guidance]: https://github.com/theRebelliousNerd/codenerd/blob/69ae63408d576d594d5b4b14652a740407d80482/Docs/architecture/AGENTS.md#L48-L109
[vision]: https://github.com/theRebelliousNerd/codenerd/blob/69ae63408d576d594d5b4b14652a740407d80482/Docs/architecture/projectdoc/01-VISION.md#L8-L25
[old-gaps]: https://github.com/theRebelliousNerd/codenerd/blob/69ae63408d576d594d5b4b14652a740407d80482/Docs/architecture/projectdoc/03-GAP-ANALYSIS.md#L58-L71
[factory-wire]: https://github.com/theRebelliousNerd/codenerd/blob/69ae63408d576d594d5b4b14652a740407d80482/internal/system/factory.go#L2660-L2699
[spawn]: https://github.com/theRebelliousNerd/codenerd/blob/69ae63408d576d594d5b4b14652a740407d80482/internal/session/spawner.go#L539-L559
[queue-roster]: https://github.com/theRebelliousNerd/codenerd/blob/69ae63408d576d594d5b4b14652a740407d80482/Docs/audits/ASTRA_QUEUE.md#L24-L42
[queue-guardian]: https://github.com/theRebelliousNerd/codenerd/blob/69ae63408d576d594d5b4b14652a740407d80482/Docs/audits/ASTRA_QUEUE.md#L117-L129
[related-roster]: https://github.com/theRebelliousNerd/codenerd/blob/69ae63408d576d594d5b4b14652a740407d80482/Docs/audits/specialist-roster-workspace-identity.md#L9-L45
[related-guardian]: https://github.com/theRebelliousNerd/codenerd/blob/69ae63408d576d594d5b4b14652a740407d80482/Docs/audits/guardian-module-alignment-scope.md#L14-L64
[related-edit]: https://github.com/theRebelliousNerd/codenerd/blob/69ae63408d576d594d5b4b14652a740407d80482/Docs/audits/northstar-lossless-strategic-editing.md#L12-L34
[related-gate]: https://github.com/theRebelliousNerd/codenerd/blob/69ae63408d576d594d5b4b14652a740407d80482/Docs/audits/recurse-gate-contract-continuity.md#L183-L209
[related-domain]: https://github.com/theRebelliousNerd/codenerd/blob/69ae63408d576d594d5b4b14652a740407d80482/Docs/audits/gate-comparison-domain-admission.md#L1-L45
[related-memo]: https://github.com/theRebelliousNerd/codenerd/blob/69ae63408d576d594d5b4b14652a740407d80482/Docs/audits/test-gate-memoization-identity.md#L1-L20
[find-test]: https://github.com/theRebelliousNerd/codenerd/blob/69ae63408d576d594d5b4b14652a740407d80482/internal/projectdoc/nerdmd_test.go#L268-L291
[membership]: https://github.com/theRebelliousNerd/codenerd/blob/69ae63408d576d594d5b4b14652a740407d80482/internal/workspace/membership.go#L606-L612
[facts]: https://github.com/theRebelliousNerd/codenerd/blob/69ae63408d576d594d5b4b14652a740407d80482/internal/projectdoc/facts.go#L52-L136
[render]: https://github.com/theRebelliousNerd/codenerd/blob/69ae63408d576d594d5b4b14652a740407d80482/internal/projectdoc/facts.go#L180-L312
[append]: https://github.com/theRebelliousNerd/codenerd/blob/69ae63408d576d594d5b4b14652a740407d80482/internal/session/executor_tools.go#L1355-L1385
[session-assembly]: https://github.com/theRebelliousNerd/codenerd/blob/69ae63408d576d594d5b4b14652a740407d80482/internal/session/executor.go#L1584-L1607
[factory-tests]: https://github.com/theRebelliousNerd/codenerd/blob/69ae63408d576d594d5b4b14652a740407d80482/internal/system/factory_projectdoc_test.go#L12-L188
[northstar-test]: https://github.com/theRebelliousNerd/codenerd/blob/69ae63408d576d594d5b4b14652a740407d80482/internal/projectdoc/northstar_facts_test.go#L74-L105
[module-policy]: https://github.com/theRebelliousNerd/codenerd/blob/69ae63408d576d594d5b4b14652a740407d80482/internal/core/defaults/policy/prompt_northstar.mg#L218-L249
[recurse-load]: https://github.com/theRebelliousNerd/codenerd/blob/69ae63408d576d594d5b4b14652a740407d80482/internal/campaign/recurse_cycle.go#L303-L329
[attempt]: https://github.com/theRebelliousNerd/codenerd/blob/69ae63408d576d594d5b4b14652a740407d80482/internal/campaign/recurse_cycle.go#L987-L1004
[attempt-consumer]: https://github.com/theRebelliousNerd/codenerd/blob/69ae63408d576d594d5b4b14652a740407d80482/internal/campaign/recurse_attempt.go#L188-L211
[guardian-view]: https://github.com/theRebelliousNerd/codenerd/blob/69ae63408d576d594d5b4b14652a740407d80482/internal/northstar/alignment_modules.go#L122-L181
[module-helper]: https://github.com/theRebelliousNerd/codenerd/blob/69ae63408d576d594d5b4b14652a740407d80482/internal/northstar/module.go#L131-L200
[guardian-select]: https://github.com/theRebelliousNerd/codenerd/blob/69ae63408d576d594d5b4b14652a740407d80482/internal/northstar/alignment_modules.go#L20-L115
[gate-detect]: https://github.com/theRebelliousNerd/codenerd/blob/69ae63408d576d594d5b4b14652a740407d80482/internal/gates/detect.go#L15-L90
[recurse-gates]: https://github.com/theRebelliousNerd/codenerd/blob/69ae63408d576d594d5b4b14652a740407d80482/internal/campaign/recurse_cycle.go#L492-L505
[session-gate]: https://github.com/theRebelliousNerd/codenerd/blob/69ae63408d576d594d5b4b14652a740407d80482/internal/session/executor_tools.go#L1648-L1699
[guard-dispatch]: https://github.com/theRebelliousNerd/codenerd/blob/69ae63408d576d594d5b4b14652a740407d80482/internal/session/write_guards.go#L30-L52
[dispatch]: https://github.com/theRebelliousNerd/codenerd/blob/69ae63408d576d594d5b4b14652a740407d80482/internal/session/executor_tools.go#L2632-L2652
[vs-gate]: https://github.com/theRebelliousNerd/codenerd/blob/69ae63408d576d594d5b4b14652a740407d80482/internal/core/virtual_store_projectdoc.go#L14-L71
[vs-dispatch]: https://github.com/theRebelliousNerd/codenerd/blob/69ae63408d576d594d5b4b14652a740407d80482/internal/core/virtual_store_routing.go#L392-L407
[registry-bind]: https://github.com/theRebelliousNerd/codenerd/blob/69ae63408d576d594d5b4b14652a740407d80482/internal/core/virtual_store_tools.go#L41-L68
[registry-dispatch]: https://github.com/theRebelliousNerd/codenerd/blob/69ae63408d576d594d5b4b14652a740407d80482/internal/tools/registry.go#L376-L423
[registry-guard]: https://github.com/theRebelliousNerd/codenerd/blob/69ae63408d576d594d5b4b14652a740407d80482/internal/core/virtual_store_write_guard.go#L55-L98
[critical-docs]: https://github.com/theRebelliousNerd/codenerd/blob/69ae63408d576d594d5b4b14652a740407d80482/internal/projectdoc/critical.go#L8-L29
[dreamer]: https://github.com/theRebelliousNerd/codenerd/blob/69ae63408d576d594d5b4b14652a740407d80482/internal/core/virtual_store.go#L550-L566
[recurse-forbid]: https://github.com/theRebelliousNerd/codenerd/blob/69ae63408d576d594d5b4b14652a740407d80482/internal/campaign/recurse_cycle.go#L1417-L1432
[no-prompt-test]: https://github.com/theRebelliousNerd/codenerd/blob/69ae63408d576d594d5b4b14652a740407d80482/internal/session/executor_projectdoc_test.go#L167-L215
[render-limits]: https://github.com/theRebelliousNerd/codenerd/blob/69ae63408d576d594d5b4b14652a740407d80482/internal/projectdoc/facts.go#L307-L356
[clamp]: https://github.com/theRebelliousNerd/codenerd/blob/69ae63408d576d594d5b4b14652a740407d80482/internal/types/elide.go#L78-L103
[render-rationale]: https://github.com/theRebelliousNerd/codenerd/blob/69ae63408d576d594d5b4b14652a740407d80482/internal/projectdoc/facts.go#L180-L185
[loadall-tests]: https://github.com/theRebelliousNerd/codenerd/blob/69ae63408d576d594d5b4b14652a740407d80482/internal/projectdoc/loadall_test.go#L27-L193

