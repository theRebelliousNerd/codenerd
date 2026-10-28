---
doc-class: deep-dive
subsystem: docscheck-orient
implementation-status: planned
last-verified: 2026-10-28
verified-against: 356f65e0e6bb639adaa90ab36212daa714114871
verification-method: static-source-inspection
supersedes: []
---

# Documentation evidence: references are not realization

## Decision

Keep the live R6 structural checker. Do not restore removed fact wrappers merely to eliminate a dead-code report. Before implementing the planned specification-alignment sensor, separate **a cited reference**, **a resolved declaration**, **a claim of implementation**, and **current acceptance evidence**. The kernel must never promote one into the next simply because a path, name, confidence score, or passing but unrelated test exists.

This is a refinement of the evidence contract inside `GAP-ORIENT-14..17`, not another proposal to add runtime documentation checking. The distinct issue is a contradiction between the existing plan and its documentation standard: the plan converts explicit code citations directly into implementation-realization links, while the standard requires planned documents to cite current attachment seams for capabilities that do not yet exist.

**Status:** source-inspected design proposal, not a reproduced runtime failure. The proposed spec-alignment path is not claimed to execute today. No implementation, tests, inference, training, or benchmarks were run. Main was checked twice during this investigation and remained at the source pin above.

## 1. The dead-code premise has already changed

[Commit b0f04d78, 2026-10-28 04:46 UTC](https://github.com/theRebelliousNerd/codenerd/commit/b0f04d78bf62f1d986271e646d0edf6555a5adb5) deliberately removed `Problem.Fact`, `Facts`, `PackageReport.Facts`, `PackageReport.Count`, the `doc_problem` declaration, and its host-witness registration. The commit describes the kernel surface as unconsumed. The current dead-code baseline contains no `docscheck` rows.

The surviving production path is straightforward:

| Stage | Current source | Meaning |
|---|---|---|
| Operator entry | `runDocsCheck` and `docsCheckInto` in [cmd_docs.go:64–96](https://github.com/theRebelliousNerd/codenerd/blob/356f65e0e6bb639adaa90ab36212daa714114871/cmd/nerd/cmd_docs.go#L64-L96) | Runs checks, emits text or JSON, and exits nonzero for findings. |
| Structural measurement | `Checker.Check` and `CheckPackage` in [docscheck.go:121–207](https://github.com/theRebelliousNerd/codenerd/blob/356f65e0e6bb639adaa90ab36212daa714114871/internal/docscheck/docscheck.go#L121-L207) | Examines the selected architecture-package directories. |
| Structured findings | `Problem` and `PackageReport` in [problem.go:51–78](https://github.com/theRebelliousNerd/codenerd/blob/356f65e0e6bb639adaa90ab36212daa714114871/internal/docscheck/problem.go#L51-L78) | Retains package, file, stable code, and message. |
| Counts and output | `Summarize` and `WriteJSON` in [problem.go:101–137](https://github.com/theRebelliousNerd/codenerd/blob/356f65e0e6bb639adaa90ab36212daa714114871/internal/docscheck/problem.go#L101-L137) | Computes counts directly and serializes findings plus summary. |
| Runtime specification alignment | [orient capability plan, §§2–5](https://github.com/theRebelliousNerd/codenerd/blob/356f65e0e6bb639adaa90ab36212daa714114871/Docs/architecture/orient/10-SPEC-ALIGNMENT.md#L46-L126) | Target state, not evidence of a live caller. |

The [original grader commit](https://github.com/theRebelliousNerd/codenerd/commit/dafdcb002c2ec5bb359a936250cd9fbc3bf344f9) explains the useful capability: replacing a gitignored, hardcoded Python grade with structured, addressable remediation findings. That capability survives. The count method was a convenience; the fact methods were a future adapter, not an alternate working runtime.

The [orientation gap matrix](https://github.com/theRebelliousNerd/codenerd/blob/356f65e0e6bb639adaa90ab36212daa714114871/Docs/architecture/orient/03-GAP-ANALYSIS.md) already owns runtime integration and foreign-template learning under GAP-ORIENT-17. The [linked capability specification's references to the removed `doc_problem` producer/schema](https://github.com/theRebelliousNerd/codenerd/blob/356f65e0e6bb639adaa90ab36212daa714114871/Docs/architecture/orient/10-SPEC-ALIGNMENT.md#L108-L125) must be reconciled when that work begins. Reintroducing a governed surface requires a producer, declarations, admission policy, consumers, invalidation, and acceptance evidence together—not merely copying old helpers back.

## 2. The specific semantic promotion gap

The current plan defines `unit_realizes(CodeUnit, SpecUnit)` as an implementation/satisfaction relationship. It then proposes that any explicit specification path or symbol citation, or source comment naming a requirement, produce that relationship directly. See [10-SPEC-ALIGNMENT.md:38–62](https://github.com/theRebelliousNerd/codenerd/blob/356f65e0e6bb639adaa90ab36212daa714114871/Docs/architecture/orient/10-SPEC-ALIGNMENT.md#L38-L62).

The [R6 standard:138–152](https://github.com/theRebelliousNerd/codenerd/blob/356f65e0e6bb639adaa90ab36212daa714114871/Docs/journeys/09-architecture-doc-standard.md#L138-L152) requires planned/target-state documents to cite the existing seam where a future capability would attach. Such a citation is correct documentation precisely when it does **not** establish that the capability is implemented.

Consequently:

1. A target-state document can cite an existing executor where a new obligation should be enforced.
2. Its explicit reference resolves to a real declaration.
3. The planned citation rule would emit a semantic realization link.
4. That link would be available to status derivation even though the cited code is only the attachment point.

This is a source-grounded counterexample to the **proposed link rule**, not a demonstrated false `/aligned` verdict. The plan also requires passing tests for alignment; those additional requirements might prevent some false alignment. They do not repair the meaning of the realization relation, prove that the tests exercise this requirement, or distinguish an attachment seam from its completed behavior.

The same distinction applies to a document describing a deprecated design, a known defect, an alternative rejected by an ADR, or a source comment saying a requirement is not yet implemented. Path/name recognition is deterministic; the relationship it recognizes is not necessarily implementation.

## 3. What today's checker actually establishes

`docscheck` deliberately preserves the old structural grade. It is not advertised here as a behavioral verifier.

- **File witness:** `witnessResolves` normalizes a permitted path, strips numeric position suffixes, and checks `os.Stat`. It does not validate cited line contents or behavior.
- **Symbol witness:** a word-boundary pattern can match a declaration, a use, a comment, or a string in a tracked Go file.
- **Test witness:** a line-oriented scan looks for a function spelling, including generic declarations. It does not require a runnable Go test signature or a `_test.go` file, and does not execute the function.
- **Predicate witness:** a name followed by an opening parenthesis in a tracked Mangle file establishes textual occurrence, not that a particular policy is loaded or derives a required result.
- **Cache:** witness results and tracked-file lists live for a checker instance. This is convenient for one CLI grade, not a revision-aware runtime cache.

These observations follow from [witnessResolves:424–480](https://github.com/theRebelliousNerd/codenerd/blob/356f65e0e6bb639adaa90ab36212daa714114871/internal/docscheck/docscheck.go#L424-L480), [testDeclOnLine:555–605](https://github.com/theRebelliousNerd/codenerd/blob/356f65e0e6bb639adaa90ab36212daa714114871/internal/docscheck/docscheck.go#L555-L605), and [searchTracked/repoFiles/loadTracked:643–717](https://github.com/theRebelliousNerd/codenerd/blob/356f65e0e6bb639adaa90ab36212daa714114871/internal/docscheck/docscheck.go#L643-L717). Existing test definitions explicitly accept ordinary and generic functions as test witnesses; their presence is evidence of the intended parity contract, not evidence this investigation executed them.

Go's actual test discovery distinguishes test filenames and signatures; its package loader also distinguishes ordinary packages, test variants, and external test packages. Those are stronger identities than a bare function name. [Go testing documentation](https://pkg.go.dev/testing), [go/packages configuration](https://pkg.go.dev/golang.org/x/tools/go/packages#Config).

There is also a separate, stronger tracked citation checker. `audit_doc_citations` checks existing regular-file paths, line ranges, Go declarations in the cited file/package, and Mangle declarations rather than arbitrary token occurrences; its `loadPkg` uses `go/parser`, and `declNames` excludes comments. See [its declared command contract](https://github.com/theRebelliousNerd/codenerd/blob/356f65e0e6bb639adaa90ab36212daa714114871/cmd/tools/audit_doc_citations/main.go#L1-L29), [`Audit` and citation judgments](https://github.com/theRebelliousNerd/codenerd/blob/356f65e0e6bb639adaa90ab36212daa714114871/cmd/tools/audit_doc_citations/audit.go), and [declaration extraction](https://github.com/theRebelliousNerd/codenerd/blob/356f65e0e6bb639adaa90ab36212daa714114871/cmd/tools/audit_doc_citations/symbols.go). Preserve and reuse that work. It does not establish that the cited declaration realizes the surrounding requirement or that a relevant acceptance witness ran. A line being inside a file is also not proof that it is the location of the cited behavior.

The R6 standard already separates deterministic structure checking from judgment about whether claims are true. This proposal does not erase that distinction, claim lexical `docscheck` is the only citation checker, or require either legacy tool to prove arbitrary prose. It prevents a future runtime from silently upgrading structural observations into stronger claims.

## 4. Proposed contract: preserve the evidence kind

The following are conceptual records, not implemented predicates or a migration patch.

| Evidence family | Required identity/provenance | What it establishes |
|---|---|---|
| Reference | Document revision, section/span, target spelling, context role | The document refers to a target. |
| Resolution | Reference ID, canonical entity/path, source revision, parser/version, scope, ambiguity/coverage | The target resolves in the observed domain. |
| Relation claim | Requirement ID, code entity, proposed role, claimant, evidence IDs | Someone claims implementation, attachment, example, dependency, contradiction, or historical relation. |
| Relation disposition | Claim ID, decision, policy version, accountable review evidence | The claim was accepted/rejected/left unresolved under a declared procedure. |
| Acceptance obligation | Requirement and behavior scope, predeclared witness contract | What evidence would count toward demonstrating this requirement. |
| Execution receipt | Contract, run, package-qualified target, source/build configuration, outcome, artifact handle | A specific check actually ran against a specific state. |
| Scan manifest | Run ID, input generation, policy digest, requested domain, observed domain, deficits, terminal state | Whether absence of findings may be interpreted within this scope. |

The model remains useful for proposing relationships, interpreting intent, and identifying missing obligations. A host validates cited bytes and identifiers, records provenance, and measures execution. Mangle derives which claims and receipts are admissible for a status or action. A reviewed semantic claim remains a reviewed claim; the system must not relabel it a formal proof.

**Who approves what:** for an initial conservative deployment, the task's user or an explicitly delegated repository approver accepts the requirement-to-witness contract and its permitted scope. An already approved, versioned contract may be reused only while its inputs and authority remain valid. Model proposals do not approve themselves. A relation disposition records the approver identity, authority source, contract version, and cited evidence; a policy may admit mechanically verifiable dispositions only under an explicitly approved rule. The host validates provenance and measures execution, but does not invent semantic approval. Mangle checks the authority and evidence prerequisites and derives admission. Changes that broaden the covered requirement or relax the witness bar require renewed approval under the same delegation policy. These are proposed authority rules, not claims about today's runtime.

### Admission rules

1. **Reference is not realization.** Explicit citations initially create reference edges. Planned-seam, historical, negative, example, and dependency roles remain useful for retrieval and impact analysis without counting as implementations.
2. **Resolution is scoped.** Ambiguous bare names yield candidates, not the first repository-wide hit. Require canonical file/package/symbol identity and actual source bytes; comments and string literals do not become declarations.
3. **No test-name shortcut.** Resolve the executable test identity and build configuration separately from its declaration. A current pass is attached to the requirement only through an accepted acceptance contract. A generic repository test pass cannot discharge every cited requirement.
4. **No confidence-only completion.** The plan's model-reported confidence threshold may prioritize review; it must not manufacture host evidence. Establish calibration on held-out relationships before allowing any score-based semantic admission.
5. **Positive coverage before negative inference.** No observed finding is not equivalent to a completed clean scan. A nonempty required-domain manifest, completed observation, and explicit coverage are prerequisites for clean/current claims. Missing or partial requirements inventory is unknown, not vacuous alignment.
6. **Revision invalidation crosses both sides.** A change to code, document, membership, test/build configuration, acceptance mapping, or policy invalidates dependent dispositions. A commit ID alone is insufficient for dirty worktrees.
7. **Host-owned evidence.** If runtime facts are introduced, host measurements and final dispositions must be protected from model writes. `ModelObservationPolicy` already articulates the reason: a model cannot witness its own completion. Preserve that boundary rather than recovering only the old `doc_problem` declaration. [mangle_updates.go:25–47](https://github.com/theRebelliousNerd/codenerd/blob/356f65e0e6bb639adaa90ab36212daa714114871/internal/core/mangle_updates.go#L25-L47).

A proposed `/aligned` result therefore means “all enumerated, applicable obligations have admissible current evidence under this policy and observation scope,” not “the software is universally correct.” Its explanation should expose remaining review assumptions, unsupported languages, unexecuted witnesses, and accepted exceptions.

**The scope is part of the result, not a footnote:** every exported status must carry the requirement set/domain, scan generation, code and document revisions, contract/policy versions, and coverage/deficits. A result for one package or supported language subset cannot be displayed or consumed as unqualified workspace `/aligned`. Unsupported or unobserved requirements inside the requested domain keep that domain unknown/incomplete. A whole-workspace claim requires a complete whole-workspace manifest; a valid subset result remains visibly subset-scoped even when every obligation in that subset passes.

## 5. Reuse existing infrastructure; do not build a second world model

The current `StructureIndex` already carries canonical refs, file spans, revisions, parse state, generation, and observation deficits. Its declaration IDs alone are not universal addresses: the source explicitly distinguishes directory-keyed refs because repeated package names occur. Reuse those identities and freshness signals, while retaining their coverage limitations. [structure_index.go:34–71,101–145](https://github.com/theRebelliousNerd/codenerd/blob/356f65e0e6bb639adaa90ab36212daa714114871/internal/world/structure_index.go#L34-L145).

Integration sequence:

1. Refine the relation semantics and acceptance cases in the existing orientation capability specification before coding.
2. Preserve the current CLI's structural grade. Add an explicitly identified richer evidence view rather than silently changing what an old zero-problem report certifies.
3. Consume the shared structural snapshot for resolvable declarations. Parse document sections and roles against its explicit generation. Unsupported or incomplete observations remain explicit.
4. Attach existing package-qualified execution evidence by stable handles. Do not run every test merely because a document names it; the executive schedules only the applicable missing obligations.
5. Publish one complete scan generation atomically, or publish explicit incomplete status. Do not accumulate unscoped findings that survive repairs or use a failed refresh as proof that previous findings disappeared.
6. Use shadow decisions before making documentation evidence a completion gate. Compare why obligations would be admitted/refused, then enable enforcement only after the defined false-promotion checks pass.

This proposal consumes, rather than duplicates, the contracts in [structure coverage and refactor preconditions](https://github.com/theRebelliousNerd/codenerd/blob/356f65e0e6bb639adaa90ab36212daa714114871/Docs/audits/structure-coverage-and-refactor-preconditions.md), [package-qualified test evidence](https://github.com/theRebelliousNerd/codenerd/blob/356f65e0e6bb639adaa90ab36212daa714114871/Docs/audits/test-evidence-capability.md), and [verification evidence/judge handoff](https://github.com/theRebelliousNerd/codenerd/blob/356f65e0e6bb639adaa90ab36212daa714114871/Docs/audits/verification-evidence-and-judge-contracts.md). Their ownership remains structural coverage, execution attribution, and advisory verification respectively. This document owns the transition from a documentation reference to a claim about requirement realization.

## 6. Research grounding and limitations

- **Alor, Khatoonabadi, and Shihab, 2025:** their documentation-to-code traceability study distinguishes identification, explanation, and multi-step reconstruction. It reports naming-based assumptions, phantom links, and architectural overgeneralization among false positives. This supports retaining discovered links as qualified evidence rather than making them completion authority. Its two-project benchmark is not a CodeNERD accuracy estimate. [Paper](https://arxiv.org/abs/2506.16440).
- **Dąbrowski et al., August 2026:** their cross-task study combines open-source-model experiments and an exploratory industrial traceability case study, finding task-dependent performance rather than a uniformly best model. This argues for evaluating role classification and evidence admission separately, not inheriting a confidence threshold from general model quality. [Paper](https://arxiv.org/abs/2608.21531).
- **Guo et al., 2024:** the requirements-traceability chapter separates link recovery from link maintenance and discusses dataset, metric, and human-evaluation choices. CodeNERD needs both initial resolution and change-driven invalidation; strong initial retrieval does not establish enduring validity. [Chapter](https://arxiv.org/abs/2405.10845).
- **W3C PROV-O:** qualified relationships distinguish entities, activities, attribution, derivation, and revisions. Use that separation as a design vocabulary for evidence provenance; adopting RDF/OWL is neither required nor proposed. [Recommendation](https://www.w3.org/TR/prov-o/).

These sources support the design distinctions. They do not prove the proposed contract improves CodeNERD or remove the need for independent acceptance witnesses.

## 7. Cost model and expected trade-offs

Let U be unique textual witnesses, B eligible source bytes, S indexed declarations, and E maintained evidence edges. Current `searchTracked` may reread eligible files for each unique witness; repeated identical witnesses are cached. The worst-case cold miss workload is therefore proportional to U × B, excluding filesystem and regex constants. This is source-derived complexity, not measured runtime.

A shared declaration snapshot can target one parse/index pass plus U lookups, with incremental refresh proportional to changed content and affected dependencies. Extra memory is proportional to declarations and evidence edges, plus persisted artifacts. Stronger package/build-aware resolution is more expensive than lexical matching, so use it where the requested evidence grade requires it; do not eagerly type-check the whole repository for every structural grade.

Model costs should be confined to ambiguous semantic relationships and obligation review. Retrieve bounded candidate sets from exact references, packages, and dependency neighborhoods. Store evidence once and expose handles/paged findings rather than repeating full documents or logs in prompts. Paging must retain unresolved work, not truncate it into a false clean result.

Measure total task cost: cold and warm scan latency, bytes read, reparses, index size, model input/output tokens, human review burden, redundant executions, and turns to an independently accepted repair. Stronger admission may initially increase abstentions and requests for evidence. That is a trade-off to report, not hide as reduced recall. No dollar savings, latency improvement, or defect rate is claimed here.

## 8. Proposed evaluation and exit criteria

All cases below are proposed evaluations; none were executed for this document.

### Deterministic adversarial fixtures

| Case | Required outcome |
|---|---|
| A planned capability cites its real future attachment seam | Reference/resolution succeed; implementation is not inferred. |
| A known-gap paragraph or rejected ADR cites an existing symbol | The negative/historical role cannot discharge the gap. |
| Target spelling occurs only in a comment, string, use site, or Mangle body occurrence | Text occurrence is retained if useful; stronger declaration/execution evidence is withheld. |
| Identically named declarations or tests exist in different packages | Resolution is qualified or ambiguous; evidence from one package cannot discharge another. |
| A declared witness is excluded by build tags, skipped, canceled, or never run | No passing execution receipt is invented. |
| An unrelated test passes while the cited requirement remains unimplemented | Its receipt does not satisfy the requirement's acceptance contract. |
| Code, spec, membership, mapping, or build configuration changes after a pass | Dependent current-evidence status is invalidated before the next decision. |
| Scan is empty because required scope was never enumerated, unreadable, or unsupported | Unknown/incomplete, not clean/aligned. |
| A model attempts to write resolved-host evidence or final disposition | Rejected through the actual model-envelope admission path. |
| An unapproved mapping, unauthorized disposition, or self-approved relaxation supplies otherwise valid receipts | Authority admission fails; valid approved-contract controls remain admissible. |
| Every supported package passes but required unsupported packages remain unobserved | Any valid subset result retains its domain/generation; no unqualified workspace alignment appears. |
| A repair removes a prior finding and a later scan fails | The system distinguishes repaired, stale, and unavailable evidence; no stale accumulation or false erasure. |

**Hard exit:** zero false promotion on this explicitly enumerated adversarial set, with positive controls showing genuine current evidence can be admitted. This is a finite acceptance bar, not a statistical zero-error claim.

### Semantic-link study

Create a blinded, independently adjudicated set of documentation sections and code units with distinct labels: implementation, attachment seam, dependency, example, historical, contradiction, unrelated, and ambiguous. Split by repository/subsystem or time so near-duplicate sections do not leak into evaluation.

Compare three treatments: the plan's direct citation-to-realization rule; exact resolution plus role claims; and the full contract with acceptance receipts. Report per-role precision/recall, false promotion into implementation/alignment, abstention, reviewer disagreement, and evidence freshness after edits. If confidence is used, report calibration and selective error at each abstention threshold, not just a single aggregate F1.

### Normal-entry-path acceptance

Follow the [orientation guidance](https://github.com/theRebelliousNerd/codenerd/blob/356f65e0e6bb639adaa90ab36212daa714114871/internal/orient/agents.md): actual init/boot and incremental-scan hooks must produce the evidence, not only a test helper constructing ideal facts. Exercise a spec-first coding task through the normal session boundary, including a code-only change, coordinated code/spec change, interrupted scan, stale receipt, and valid accepted change.

**Ship condition:** the intended status derives from those host-observed paths; failed or incomplete evidence cannot masquerade as alignment; existing R6 CLI grading remains explicitly understood; and measured total cost is acceptable against the project's declared quality/latency budget. If not, keep the richer sensor in shadow mode without weakening the evidence distinction.

## Branch-status addendum: CLI evidence view implemented; runtime alignment still planned

**Branch qualification, 2026-10-28 (source checked at 08:57–09:04 UTC).** The original analysis remains a statement about its pinned baseline, not the whole repository today. Main remains [`356f65e`](https://github.com/theRebelliousNerd/codenerd/commit/356f65e0e6bb639adaa90ab36212daa714114871). Implementation landed on `feat/conversation-brief` in [`e033fc4`](https://github.com/theRebelliousNerd/codenerd/commit/e033fc454e20482c5d008212991b09e5b997b9d7); this addendum inspects [`462153c`](https://github.com/theRebelliousNerd/codenerd/commit/462153c50a504ee5830ba3aa4667d73c6db9d9a2). The [four intervening commits](https://github.com/theRebelliousNerd/codenerd/compare/e033fc454e20482c5d008212991b09e5b997b9d7...462153c50a504ee5830ba3aa4667d73c6db9d9a2) do not change these implementation files. This is static source and test-definition inspection: no tests, builds, provider calls, or live browser runs were executed for this addendum. “Implemented” below means present on that feature pin, not merged to main or independently runtime-qualified.

**Implemented on the feature branch.** `nerd docs evidence` reaches a distinct evidence view without replacing the existing structural checker. Parsed declarations, citation roles, explicit ambiguity/unknown outcomes, and a scoped input-digest manifest separate reference resolution from realization. A clean result requires a complete scan with no failing required references; incomplete/empty scans cannot quietly certify cleanliness. The manifest explicitly lists relation claims, dispositions, acceptance obligations and execution receipts as not observed. [CLI caller](https://github.com/theRebelliousNerd/codenerd/blob/462153c50a504ee5830ba3aa4667d73c6db9d9a2/cmd/nerd/cmd_docs.go#L134-L169), [declaration extraction](https://github.com/theRebelliousNerd/codenerd/blob/462153c50a504ee5830ba3aa4667d73c6db9d9a2/internal/docdecl/docdecl.go), [roles](https://github.com/theRebelliousNerd/codenerd/blob/462153c50a504ee5830ba3aa4667d73c6db9d9a2/internal/docscheck/evidence_extract.go), [manifest and clean condition](https://github.com/theRebelliousNerd/codenerd/blob/462153c50a504ee5830ba3aa4667d73c6db9d9a2/internal/docscheck/evidence.go#L177-L234), [terminal-state/generation calculation](https://github.com/theRebelliousNerd/codenerd/blob/462153c50a504ee5830ba3aa4667d73c6db9d9a2/internal/docscheck/evidence.go#L285-L307).

**Partial.** This is a static CLI sensor, not the proposed live Mangle alignment authority. It does not create accepted implementation relations, human/host dispositions, per-obligation current test receipts or an `/aligned` completion gate. Whole-scan generation identity is not per-disposition invalidation. The feature's architecture-spec repair and [explicit remaining work](https://github.com/theRebelliousNerd/codenerd/blob/462153c50a504ee5830ba3aa4667d73c6db9d9a2/Docs/audits/documentation-evidence-contract.md#L48) should not be read as those runtime paths shipping.

**Unverified here.** Tests define planned-seam, negative/historical, ambiguity, test-identity, incomplete-domain and changed-input witnesses; model-produced documentation evidence is also tested at the observation filter. Those test sources were inspected, not executed. Any scan counts, timing or passing-test claims in the feature's as-built account remain attributed implementation-author results, not measurements repeated by this addendum. [Evidence witnesses](https://github.com/theRebelliousNerd/codenerd/blob/462153c50a504ee5830ba3aa4667d73c6db9d9a2/internal/docscheck/evidence_test.go), [authority witness](https://github.com/theRebelliousNerd/codenerd/blob/462153c50a504ee5830ba3aa4667d73c6db9d9a2/internal/core/doc_evidence_authority_test.go).

