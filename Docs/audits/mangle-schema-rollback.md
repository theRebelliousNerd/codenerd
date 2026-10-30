---
doc-class: deep-dive
subsystem: mangle-schema-admission
implementation-status: partially-implemented
last-verified: 2026-10-30
verified-against: 486db57ce490302c65d7f484e7f6070ee0e70862
verification-method: static-source-inspection
---

# Generic Mangle schema rejection must preserve the accepted program

## Decision and evidence boundary

Make schema admission atomic in `internal/mangle.Engine`: a fragment rejected during stratification must leave the previously accepted program and all of its compiled metadata unchanged. The current loader removes the rejected source fragment but has already published its analyzed `ProgramInfo`. This is a concrete host-state inconsistency, not a demonstrated production misclassification or permission bypass.

**Source baseline:** public `theRebelliousNerd/codenerd` main at `486db57ce490302c65d7f484e7f6070ee0e70862`, inspected 2026-10-30. This assessment used read-only source inspection and independent source review. No inference, program execution, tests or benchmarks were run. No repository files were changed. All remedy and evaluation work below is proposed, not implemented by this audit.

The production continuation is limited but real: a malformed local `.nerd/mangle/learned_taxonomy.mg` containing rules can be rejected while the taxonomy engine is retained. Normal generated learned-exemplar writes are parsed and reconstructed as facts; they do not establish this rule-cycle trigger. Treat this as a rejection-integrity defect for the generic loader, not evidence that ordinary learning produces invalid rule cycles.

## The rejected-load branch

`LoadSchemaString` parses a candidate, appends it to `schemaFragments`, and invokes `rebuildProgramLocked`. On failure it removes the last fragment and returns an error. The intended rollback is visible and worth preserving. Inside rebuilding, however, `AnalyzeOneUnit` succeeds before `e.programInfo = programInfo` at line 371. `Stratify` runs at line 375 and can return an error at lines 376–377. The assignments to strata, predicate index, query context and rule-graph adoption are below that return. [Loader and rebuild](https://github.com/theRebelliousNerd/codenerd/blob/486db57ce490302c65d7f484e7f6070ee0e70862/internal/mangle/engine.go#L319-L403).

Let A be the last accepted program and B the analyzed combination of A plus the rejected fragment. Immediately after this failure:

| State | Retained value |
|---|---|
| `schemaFragments` | A, because the candidate was popped |
| `programInfo` | B, already assigned |
| `strata`, `predToStratum` | A |
| `predicateIndex`, `queryContext` | A |
| `rules` invalidation graph | A, because `adoptProgramLocked` was not reached |

The mutex prevents simultaneous observation during rebuilding; it does not repair this mixed state after unlocking. Parse failures return before appending, and analysis failures return before publishing `programInfo`; neither follows the problematic branch. A later successful rebuild can repair the metadata, but a caller is not required to perform one after a rejected load.

This matters to execution rather than only diagnostics. `evalWithGasLimit` passes the current `programInfo` and retained strata/map to the upstream evaluator. `rederiveLocked` iterates IDB symbols from current `programInfo` while computing invalidation through `e.rules`. Thus the two consumers no longer necessarily describe the same program. The exact downstream result depends on the rejected program and upstream behavior; this audit does not assert that every candidate yields a wrong row or panic. [Evaluation inputs](https://github.com/theRebelliousNerd/codenerd/blob/486db57ce490302c65d7f484e7f6070ee0e70862/internal/mangle/engine.go#L247-L268), [rederivation](https://github.com/theRebelliousNerd/codenerd/blob/486db57ce490302c65d7f484e7f6070ee0e70862/internal/mangle/engine_rederive.go#L153-L191).

## Production producer and consumer

`NewTaxonomyEngine` enables AutoEval, loads the static corpus, then calls `tryLoadLearned` and returns the taxonomy object. `SetWorkspace` also invokes the same loader while retaining the object. `tryLoadLearned` reads `.nerd/mangle/learned_taxonomy.mg` through `Engine.LoadSchema`; an error is logged and the function returns without replacing or disabling the engine. [Construction and workspace binding](https://github.com/theRebelliousNerd/codenerd/blob/486db57ce490302c65d7f484e7f6070ee0e70862/internal/perception/taxonomy.go#L97-L153), [local overlay loading](https://github.com/theRebelliousNerd/codenerd/blob/486db57ce490302c65d7f484e7f6070ee0e70862/internal/perception/taxonomy.go#L197-L222).

Later, `ClassifyInputWithMatches` reuses loaded static schemas and clears the fact store rather than resetting the program. It produces `verb_def`, synonyms/patterns, `context_token`, `user_input_string`, `candidate_intent` and `semantic_match` facts; `AddFacts` triggers automatic evaluation, and `GetFacts("potential_score")` supplies candidate scores. Clear does not restore the rejected program pointer to A. [Classification preparation](https://github.com/theRebelliousNerd/codenerd/blob/486db57ce490302c65d7f484e7f6070ee0e70862/internal/perception/taxonomy.go#L434-L503), [semantic inputs and materialized score read](https://github.com/theRebelliousNerd/codenerd/blob/486db57ce490302c65d7f484e7f6070ee0e70862/internal/perception/taxonomy.go#L505-L538).

The ordinary learning writer narrows the claim: it parses an exemplar and supplies `exemplar.source()` to `LoadSchemaString`, then persists the normalized fact. A local rule overlay, legacy/corrupt content or another generic-loader caller is needed for the cycle branch; normal accepted correction text is not a demonstrated attack channel. This is also distinct from `RealKernel` policy admission. [Normalized learning admission](https://github.com/theRebelliousNerd/codenerd/blob/486db57ce490302c65d7f484e7f6070ee0e70862/internal/perception/learning.go#L341-L379).

## Mangle semantics and primary sources

The host calls analysis and stratification separately. Its `mangle.Stratify` delegates to upstream `analysis.Stratify` and returns the rejection before layering. [Host adapter](https://github.com/theRebelliousNerd/codenerd/blob/486db57ce490302c65d7f484e7f6070ee0e70862/internal/mangle/strata_layers.go#L47-L61).

The inspected public upstream mirror, `google/mangle` at `7cd2543ac1817f8b92d42efe3c6ef82dededbeeb`, shows analysis constructing `ProgramInfo` separately from the stratifier. The latter rejects a negative dependency inside a strongly connected component; do-transform recursion is represented as a negative edge too. These are semantic admission failures after local rule analysis, not merely bad syntax. [Upstream analysis](https://github.com/google/mangle/blob/7cd2543ac1817f8b92d42efe3c6ef82dededbeeb/analysis/validation.go), [upstream stratification](https://github.com/google/mangle/blob/7cd2543ac1817f8b92d42efe3c6ef82dededbeeb/analysis/stratification.go#L43-L101).

**Provenance limit:** codeNERD pins `codeberg.org/TauCeti/mangle-go v0.7.0`; the accessible mirror declares that module path, but this audit did not independently compare the deployed Codeberg module bytes against the mirror. The host assignment ordering is directly established regardless; proposed runtime witnesses remain unexecuted. [Dependency pin](https://github.com/theRebelliousNerd/codenerd/blob/486db57ce490302c65d7f484e7f6070ee0e70862/go.mod#L5-L8), [mirror module identity](https://github.com/google/mangle/blob/7cd2543ac1817f8b92d42efe3c6ef82dededbeeb/go.mod#L1-L3).

## Proposed witness and acceptance criteria

Use a fresh generic engine with a valid, declared baseline program and at least one settled derived relation. Save the accepted program pointer, compiled metadata and materialized facts. Submit a second fragment whose variables are positively bound but whose rule negates its own head, using unique predicate names to avoid accidental declaration collisions. The repository already supplies this shape:

```mangle
Decl start(S) bound [/name].
start(/x).
paradox(X) :- start(X), ! paradox(X).
```

This is an illustrative unrun input, taken from the repository's adversarial fixture rather than an executed reproduction. [Self-negation fixture](https://github.com/theRebelliousNerd/codenerd/blob/486db57ce490302c65d7f484e7f6070ee0e70862/.codex/skills/stress-tester/assets/mangle-adversarial/safety/stratification_cycles.mg#L23-L27).

Verify the failure is specifically stratification, not parsing, duplicate declarations or variable safety. After rejection, require all accepted-program metadata to remain A. Then exercise `Evaluate`, `AddFacts`, `Clear` followed by rehydration, and both materialized and query reads; compare against an untouched control engine containing A and the same base facts. Add a later valid fragment to establish recovery. Include first-ever load rejection, repeated rejection and cancellation-independent error handling. A separate taxonomy fixture should load a malformed local overlay, observe its warning, then compare classification with an overlay-free control. Do not label these checks passed until run on the actual pinned dependency.

## Proposed repair and costs

Compile into local candidate state and publish only after all admission steps succeed. For the demonstrated branch, moving `programInfo` publication after successful `Stratify` is the minimal change. A stronger explicit accepted-program bundle can bind program, strata, lookup indexes, query context and invalidation graph together, provided it preserves base-IDB bookkeeping and existing locking. Do not repair rejection by silently clearing accepted facts or discarding a working taxonomy.

This does not require new Mangle predicates, prompt atoms, model calls, extra tokens or a second policy engine. The minimal repair reorders existing work rather than duplicating compilation. A staged bundle temporarily retains old and candidate metadata until publication, which is already substantially true during analysis; benchmark peak allocations and lock duration before claiming a performance improvement. Full engine/store cloning would add avoidable fact-volume-dependent memory and latency and is not necessary to establish this admission contract.

Evaluate successful and rejected loads separately across small and large schemas. Report analysis/stratification time, allocations, retained metadata after repeated failures, and post-rejection output parity. Compare against the current loader only where both produce correct accepted-program behavior; faster execution of an inconsistent program is not an efficiency benefit.

## Relationship to prior work

No exact duplicate was located in the current audit queue, February Mangle integration audit, learned-correction integrity audit, generic merge-invalidation audit, or searches for schema rollback and stratification. The M0/M2 journeys already distinguish analysis from stratification; this report concerns the host publication boundary after that distinction matters. Generic merge invalidation concerns an incomplete dependency graph for an accepted program, not publishing a rejected program beside an old graph. Learned-correction integrity concerns qualifier transport, persistence context and worker lifetime, not loader rollback. [Audit queue](https://github.com/theRebelliousNerd/codenerd/blob/486db57ce490302c65d7f484e7f6070ee0e70862/Docs/audits/ASTRA_QUEUE.md), [generic merge audit](https://github.com/theRebelliousNerd/codenerd/blob/486db57ce490302c65d7f484e7f6070ee0e70862/Docs/audits/mangle-generic-merge-invalidation.md), [learned-correction audit](https://github.com/theRebelliousNerd/codenerd/blob/486db57ce490302c65d7f484e7f6070ee0e70862/Docs/audits/learned-correction-integrity-lifecycle.md).

Search results were indexed at `073bffe69f82c964db043afea424488142f181ca`; the comparison to the pinned head covered 20 subsequent commits. Those commits added three unrelated audits and did not modify the generic engine or taxonomy source traced here. The negative duplicate search is bounded evidence, not a claim that no differently worded private or unindexed report exists.
