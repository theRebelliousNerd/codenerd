---
doc-class: deep-dive
subsystem: mangle-extensional-deltas
implementation-status: partially-implemented
last-verified: 2026-10-30
verified-against: 89ee924f59829d92186bb6fc88a39ebe0c6c2ac6
verification-method: static-source-inspection
---

# Extensional deltas must preserve remaining admitted atom support

## Finding and evidence boundary

Preserve the kernel's bounded EDB delta optimization, but make removal operate on admitted atom support rather than assuming each host canonical key denotes a unique store atom. Two supported host representations can have different canonical keys and nevertheless become the same Mangle String constant. Removing one key then removes the shared materialized atom even when the other key remains in the EDB. A subsequent full evaluation restores it.

This is a source-supported **kernel API and full-versus-cone equivalence defect**. The affected evaluator is production code, but this review has **not established a stock production pair of producers that creates the overlapping aliases**. The witness below is a trusted host composition using supported APIs and a narrow `/string` declaration, not an observed user incident, model-accessible exploit, or demonstrated safety bypass.

Source freeze: public main `89ee924f59829d92186bb6fc88a39ebe0c6c2ac6`, inspected October 30, 2026, approximately 04:38–04:41 UTC. Root/core guidance and existing Mangle audits were inspected. The merged `research-main-20261030-0411` delta contains consultation-cache, Gemini output-cap and shard-registration audits, not this issue. An independent source challenge supported the narrow witness and its qualification. No tests, inference, runtime reproduction, benchmarks or repository modifications were performed. This file is a research draft, not an implementation.

## The representation boundary

For a declaration `Decl alias_probe(X) bound [/string].`, consider these host inputs:

| Host argument | Host canonical key | Admitted Mangle atom |
|---|---|---|
| `string("/tmp")` | `alias_probe(/tmp)` | `alias_probe("/tmp")` |
| `types.MangleString("/tmp")` | `alias_probe("/tmp")` | `alias_probe("/tmp")` |

`canonValue` special-cases plain strings through the name-shape heuristic. It does not special-case `MangleString`; that named string type falls through to quoted formatting. Admission is later: `addFactIfNewLockedErr` first checks the canonical key, then converts and admits the fact, and stores it under the original key. Thus both rows can be retained. String interning does not normalize `MangleString` into a plain string. [Canonicalization](https://github.com/theRebelliousNerd/codenerd/blob/89ee924f59829d92186bb6fc88a39ebe0c6c2ac6/internal/core/kernel_facts.go#L158-L258), [admission and insertion](https://github.com/theRebelliousNerd/codenerd/blob/89ee924f59829d92186bb6fc88a39ebe0c6c2ac6/internal/core/kernel_facts.go#L446-L490), [interning](https://github.com/theRebelliousNerd/codenerd/blob/89ee924f59829d92186bb6fc88a39ebe0c6c2ac6/internal/core/kernel_facts_intern.go#L30-L69).

`Fact.ToAtom` makes an explicit `MangleString` a String constant. For the plain string, the shape heuristic initially makes a Name, but the narrow `/string` declaration rejects that Name and accepts String, so the Decl adapter changes it to the same String constant. The narrow declaration is essential: do not substitute `/any`, which accepts the initial Name and therefore does not establish this alias. [Host conversion](https://github.com/theRebelliousNerd/codenerd/blob/89ee924f59829d92186bb6fc88a39ebe0c6c2ac6/internal/types/types.go#L225-L253), [Decl adapter](https://github.com/theRebelliousNerd/codenerd/blob/89ee924f59829d92186bb6fc88a39ebe0c6c2ac6/internal/core/kernel_fact_decl.go#L182-L239).

Mangle's fact store is a set, not a multiset of host supports. The inspected upstream store's Add rejects an equal existing atom, while Remove deletes that atom; it does not count the two distinct Go facts that produced it. This is the correct store contract. The missing accounting is at the host delta boundary. [Primary upstream interface](https://github.com/google/mangle/blob/7cd2543ac1817f8b92d42efe3c6ef82dededbeeb/factstore/factstore.go#L47-L84), [set insertion and removal](https://github.com/google/mangle/blob/7cd2543ac1817f8b92d42efe3c6ef82dededbeeb/factstore/factstore.go#L187-L225).

## Proposed isolated witness

Use a successfully initialized kernel with this trusted schema and policy, with no program initial facts, external predicates or provenance recorder:

```mangle
Decl alias_probe(X) bound [/string].
Decl alias_visible(X) bound [/string].
alias_visible(X) :- alias_probe(X).
```

1. Assert `Fact{Predicate: "alias_probe", Args: []any{string("/tmp")}}`.
2. Assert `Fact{Predicate: "alias_probe", Args: []any{types.MangleString("/tmp")}}`.
3. Evaluate successfully and query both predicates. The EDB holds two host rows; the materialized set holds one `alias_probe("/tmp")` and its one derived `alias_visible("/tmp")`.
4. Call `RetractExactFact` with only the original plain-string fact. Exact matching does not equate plain string with `MangleString`, so the explicit-string EDB row survives.
5. Query both predicates, allowing the normal lazy cone evaluation. Source predicts both answers become empty even though the surviving host row still supports them.
6. Force a full `Evaluate`, without another EDB mutation. Source predicts both answers return.

The public exact-retraction boundary compacts matching host rows and records the removed key in the EDB index. The next lazy evaluation selects a cone; `alias_probe` has no rule head, so preparation uses `applyExtensionalDeltaLocked`, which removes the recorded admitted atom unconditionally. Its addition loop considers only keys added since the previous evaluation. The surviving alias was already held, so it is not restored. `alias_visible` is rederived over the missing input. [Exact retraction](https://github.com/theRebelliousNerd/codenerd/blob/89ee924f59829d92186bb6fc88a39ebe0c6c2ac6/internal/core/kernel_facts.go#L922-L953), [host equality](https://github.com/theRebelliousNerd/codenerd/blob/89ee924f59829d92186bb6fc88a39ebe0c6c2ac6/internal/core/kernel_facts.go#L1037-L1142), [index removal](https://github.com/theRebelliousNerd/codenerd/blob/89ee924f59829d92186bb6fc88a39ebe0c6c2ac6/internal/core/kernel_edb_index.go#L91-L107), [extensional preparation](https://github.com/theRebelliousNerd/codenerd/blob/89ee924f59829d92186bb6fc88a39ebe0c6c2ac6/internal/core/kernel_cone_store.go#L29-L48), [delta removal and additions](https://github.com/theRebelliousNerd/codenerd/blob/89ee924f59829d92186bb6fc88a39ebe0c6c2ac6/internal/core/kernel_edb_delta.go#L113-L140).

A full evaluation instead repopulates a fresh store from all retained EDB facts, including the survivor. This difference needs no failing evaluation, raw-store escape, alias mutation, concurrency, numeric rounding, cache hit or lattice behavior. [Fresh full evaluation](https://github.com/theRebelliousNerd/codenerd/blob/89ee924f59829d92186bb6fc88a39ebe0c6c2ac6/internal/core/kernel_eval.go#L319-L367).

## Scope and existing protections

The defect requires distinct retained host keys for one admitted atom, evaluation before removal, a removal that leaves another support, and an eligible cone evaluation. Removing both aliases does not lose a surviving support. A fresh full evaluation repairs the discrepancy. A predicate that is itself rule-derived takes the remove-and-restore path rather than the extensional delta path. A plain string paired with an explicit string under `/any` is not this witness. Ordinary facts with one host representation remain the intended efficient case.

The existing typed-fingerprint audit concerns the opposite direction of mismatch: equal canonical keys for different admitted atoms and conditional prompt-cache reuse. Here distinct keys map to one atom, and materialization itself becomes wrong without any cache. Fixing fingerprint hashing alone cannot fix support removal. [Prior identity audit](https://github.com/theRebelliousNerd/codenerd/blob/89ee924f59829d92186bb6fc88a39ebe0c6c2ac6/Docs/audits/mangle-content-fingerprint-typed-identity.md). The performance audit already requires full/cone semantic equivalence; this is an acceptance refinement of its newly implemented extensional delta optimization, not a proposal to abandon cones. [Performance audit](https://github.com/theRebelliousNerd/codenerd/blob/89ee924f59829d92186bb6fc88a39ebe0c6c2ac6/Docs/audits/mangle-kernel-performance.md).

## Proposed contract and alternatives

**A store atom must remain present while any retained EDB row admits to it under the current program.** Host deletion semantics and Mangle set semantics must be connected explicitly.

- Maintain a typed admitted-atom support index or count, with equality checking in hash buckets. Remove a materialized extensional atom only when its last support disappears. Include predicate and arity, handle program-dependent reconversion, and copy/share ownership correctly across clones.
- As a conservative bounded repair, check surviving support within the affected predicate before removing a potentially aliased atom, or rebuild that predicate's extensional membership. This costs more than O(delta) for those predicates but need not force whole-program recompilation or whole-world rebuilding.
- Canonicalizing all EDB identity after admission is a larger alternative. It may change observable host deduplication and exact-retraction behavior, so it should not be slipped in as an invisible optimization. Boot facts admitted before declarations and policy changes need an explicit migration/reindex contract.

Do not simply disable removals, mark the operation successful while leaving stale atoms, or globally force full evaluation as the final design without measuring the cost. Preserve the existing full fallback and admission checks.

## Verification and complete cost accounting

Proposed deterministic tests, not executed: both alias insertion orders; either alias removed; both removed; remove/re-add before evaluation; exact batch and transaction paths; clean clones and parent isolation; simple and indexed stores; a `/any` nonalias control; a genuinely changed value; program-bound changes; and an unrelated retained relation. Assert host EDB survivors, materialized base membership and complete derived closure against a fresh full-evaluation reference. The final state must not depend on whether a full or cone path happened to run.

Measure small and large predicates with zero, sparse and dense aliases. Report mutation/admission cost, support-index bytes, clone cost, cone preparation, engine time, lock holding and total query latency. An admitted-atom index adds hashing/equality and lifecycle maintenance; a survivor scan adds affected-predicate work on removal. Charge both normal single-representation workloads and alias-heavy workloads, not merely the fastest lookup. Compare equal answers, not an incorrect faster baseline. No extra model calls, embeddings, training or live inference are needed for the deterministic repair. No speedup or downstream task-quality gain is established here.

**Checkable exit:** removing one host representation cannot remove an atom still supported by another retained representation; cone and full evaluation agree for the same admitted EDB; ordinary delta workloads retain their measured efficiency where safe.

## Upstream provenance limitation

CodeNERD pins `codeberg.org/TauCeti/mangle-go v0.7.0`. The primary upstream implementation cited above is the accessible `google/mangle` mirror's v0.7.0 commit `7cd2543ac1817f8b92d42efe3c6ef82dededbeeb`. Exact downloaded Codeberg module bytes were not independently compared in this read-only pass. The host key/admission/delta omissions are directly established in CodeNERD; runtime predictions remain source-derived, conditional on the deployed store matching the inspected set contract, and unexecuted.
