# Gap analysis: from working substrate to uniform executive contract

> Compared on 2026-07-13: [01-VISION.md](01-VISION.md) against the live state in
> [02-CURRENT-STATE.md](02-CURRENT-STATE.md). A gap is not an implementation claim.

## Executive summary

The package is substantial production infrastructure, not a skeleton. Engine,
feedback, synthesis, schema validation, proof, LSP, and differential code all
exist and have package tests. The former differential created-fact-limit gap is
closed. The production parser-bypass gap is also closed. The highest-priority
remaining gap is narrower: unified read APIs do not see the store populated by
the fast path.

Created-fact gas parity is now fixed and regression-proven. Optimization work
should still stop behind the remaining parser, exported read-contract, and
observable fallback gates. True delta propagation is valuable only after full and
differential modes enforce comparable safety and evidence.

## Closed in the P0 follow-up

| Former gap | Resolution evidence | Verdict |
|---|---|---|
| Differential created-fact limit was not forwarded, and zero-config kernel defaults differed | `DifferentialEngine.evalOptions` supplies `WithCreatedFactLimit` to unified atom, legacy atom, and legacy fact evaluation; `effectiveDerivedFactLimitLocked` supplies the same 500,000 kernel default to full and diff configuration. Package and kernel parity regressions cover both boundaries | **VERIFIED CURRENT** |

## Evidence-backed matrix

| Gap | Current evidence | Target | Severity | Disposition |
|---|---|---|---|---|
| Raw parser calls bypass the shared lock | Sanitizer, synth, both system fact adapters, and test helpers now call `internal/mangle` wrappers; a whole-module AST scan rejects raw parser selectors outside `parse_lock.go` | All parsing enters `ParseUnit`/`ParseAtom`; concurrent mixed callers pass under race | **CLOSED 2026-08-09** | `TestCodeUsesSerializedMangleParser` and `TestProductionParserCallersShareSerializedEntryPoint`; core concurrency race slice also passes |
| Unified fast path has an unguarded read contract | `ApplyAtomDelta` populates `unifiedStore`; `Query` and `Snapshot` read/copy only `strataStores` | Either keep both stores coherent or reject unsupported read APIs with a typed error | **P0 / High correctness for exported API** | Pin mode contract, reproduce, add Query/Snapshot tests; kernel's current use is limited to `CopyAllFactsTo` |
| Evaluator options are not one typed contract | Full and diff modes now share gas; external callbacks and provenance remain full-path-only | One option surface with explicit support/fallback result | **P1 / Medium** | Preserve the verified gas regression; retain full fallback until each additional option is proven |
| Package-local intent rules shadow runtime intent rules | Package tests load `internal/mangle/intent_routing.mg`; boot loads `internal/core/defaults/schema/intent_routing.mg`; files differ | One authority or an explicit generated fixture with parity checks | **P1 / Medium** | Decide owner, replace stale comments, and test the live embedded module list |
| Structured synth is not the universal model rule protocol | Legislator requires synth; core/executive/constitution feedback loops use default off | Structured output by default for every model-authored rule, with explicit compatibility fallback | **P1 / Medium** | Inventory callers, pin per-caller schema constraints, migrate with negative tests |
| Proof tree and provenance are parallel stories | `internal/mangle/proof_tree.go#ProofTreeTracer` coexists with `internal/core/kernel_provenance.go` | One correlation model and bounded evaluation receipt | **P2 / Medium** | First standardize result/fallback telemetry; then attach proof/provenance IDs |
| No durable bounded evaluation receipt | Logs, engine stats, and traces are separate and path-specific | Redacted program/fact fingerprints, mode/options, result counts, fallback/error, proof IDs | **P2 / Medium** | Build `mangle-explainable-replay-v1` after option parity |
| Schema validator still relies partly on regex extraction | `internal/mangle/schema_validator.go#SchemaValidator.extractDeclsFromText` counts commas in textual Decls | Prefer analyzed `ProgramInfo` declarations and retain regex only for preflight diagnostics | **P2 / Medium** | Introduce ProgramInfo-fed constructor/update path; test complex bounds/descriptors |
| Snapshot is a deep copy despite COW wording | `internal/mangle/differential.go#DifferentialEngine.Snapshot` enumerates and copies facts | Honest API name/docs or bounded structural sharing with isolation proof | **P3 / Low until measured** | Measure memory and latency before redesign |
| True delta propagation is not implemented | Unified mode reuses a store but runs full-program seminaive evaluation per changed batch | Delta-aware propagation with full/diff semantic oracle | **P3 / Medium-high leverage** | Only after P0/P1 parity; benchmark representative OODA deltas |
| SIMD intersection has no production caller | `internal/mangle/simd_intersect_test.go` tests it; review found no non-test call | Either a measured caller or explicit library-only/deprecation decision | **P3 / Low** | Do not wire for aesthetics; require a profile and benchmark |

## Probable product findings

The audit recorded two bounded findings without changing product code:

- `.corpus-build/findings/mangle-differential-gas-bypass.md` — created-fact
  limit bypass, now fixed with a three-route finite regression.
- `.corpus-build/findings/mangle-unified-fast-path-read-contract.md` — exported
  Query/Snapshot paths do not read the store populated by unified ApplyAtomDelta.

These are source-grounded findings. The first is marked fixed with bounded
before/after evidence. The second has a finite exported-API reproduction and
remains open.

## Non-gaps and rejected shortcuts

| Item | Classification | Why |
|---|---|---|
| `permitted/3` is absent from this package | **REJECTED as a gap** | Core policy owns constitutional permission; this package must protect, not duplicate, that authority |
| Mangle does not perform fuzzy user-intent matching | **REJECTED as a gap** | Perception/embeddings should map language into structured facts before exact deduction |
| Differential evaluation defaults off | **ASSUMPTION to retain until parity closes** | Conservative default avoids making the incomplete option contract universal |
| Core full evaluation calls mangle-go directly | **OPEN QUESTION, not automatically a gap** | A wrapper-only rule may add coupling without value; the needed invariant is one observable contract, not necessarily one Go type |
| Feedback retries are bounded | **VERIFIED CURRENT safety feature** | Termination is intended; exhausted repair should reject or escalate rather than retry forever |
| Two-bucket legacy stratification | **REJECTED as an immediate gap** | Source comments record a measured regression from finer per-stratum setup; real leverage is option parity and delta propagation |
| Deep-copy snapshots | **ASSUMPTION acceptable at current scale** | Redesign requires measured memory pressure and a proven isolation contract |

## Dependency order

```text
verified diff gas regression ──> remaining option contracts ──> broader eligibility
                 |                              |
                 +------------------------------+──> result/fallback receipt

parse chokepoint ──> mixed race gate [closed] ──> structured synth rollout

intent authority decision ──> runtime corpus parity tests

receipts + proof correlation ──> no-effects replay

all parity gates ──> true delta propagation
```

## Closeout gates

A future corpus update may close a row only with all applicable evidence:

1. a focused regression that fails on the prior behavior;
2. the minimal causal implementation change;
3. full/diff or caller/callee wiring proof, not just package compilation;
4. negative tests for silent option loss, unbounded work, protected-head spoofing,
   raw parser concurrency, and unsupported mode use;
5. targeted package tests plus the relevant core or CLI integration tests;
6. `go test -race` when parser/store concurrency changes;
7. updated current-state, safety, testing, failure, and TODO evidence;
8. a rollback boundary, usually disabling differential mode or the new receipt
   producer without changing policy semantics.
