# Vision: logic that can be trusted, explained, and replayed

> Target owner: `internal/mangle`
>
> Current truth belongs in [02-CURRENT-STATE.md](02-CURRENT-STATE.md); this file
> deliberately describes the desired contract.

## Product promise

Mangle should make codeNERD feel creatively powerful without making executive
behavior mysterious. A model may invent a useful approach, rule, or explanation.
It may not invent authority. The system accepts logic only after deterministic
syntax, schema, safety, resource, and policy gates, then records enough evidence
for an operator to understand what happened.

The desired user experience is:

1. the same request and world state produce the same logical decision;
2. invalid or unsafe generated logic is rejected with an actionable reason;
3. full and incremental evaluation enforce identical safety options;
4. a derived action can be explained without exposing secrets or replaying an
   external effect;
5. degraded paths are visible instead of silently changing semantics.

## North-star architecture

```text
language and code
      |
      v
perception / embeddings -------- fuzzy meaning belongs here
      |
      v
typed facts + provenance
      |
      +------------------------------+
      | model-authored rule proposal |
      | structured synth + feedback  |
      +---------------+--------------+
                      v
             parse -> analyze -> validate
                      |
                      v
       one evaluation contract (full or differential)
       gas + externals + provenance + cancellation
                      |
                      v
       derived next_action + core-owned permitted/3
                      |
                      v
              bounded evaluation receipt
```

The LLM remains the creative center: it frames problems, proposes rules, and
explains results. Mangle remains the executive substrate: it unifies exact facts,
computes stratified consequences, and makes absence meaningful under the closed
world assumption. Core policy remains the constitutional owner.

## Target contracts

### One language contract

**PROPOSED UPLIFT:** all model-facing and human-facing Mangle authoring should
teach and enforce the same small set of truths:

- every predicate has a `Decl` before runtime use;
- `/atom` identifiers and enum values are not interchangeable with strings;
- variables in negated atoms are bound by earlier positive atoms;
- recursion ranges over a finite domain and has no cycle through negation;
- aggregation uses the `|> do ... let ...` transform pipeline;
- fuzzy matching and broad string manipulation happen before facts reach Mangle.

The upstream parser and analyzer are authoritative. Regex prevalidators may make
errors friendlier, but they must never be the final semantic gate.

### One evaluation contract

**VERIFIED CURRENT:** full and differential modes now enforce the same positive
created-fact limit through `internal/mangle/differential.go#DifferentialEngine.evalOptions`.

**PROPOSED UPLIFT:** extend that parity contract to external-predicate callbacks,
provenance recorder, cancellation boundary, and result accounting. A mode may
decline an option and request a visible fallback; it may not silently drop it.

The first acceptance target, `mangle-diff-eval-option-parity-v1` in
[TODO.md](TODO.md), is verified for created-fact limits. True delta propagation is
deliberately later: optimization must not outrun semantic and safety parity.

### One generation contract

**PROPOSED UPLIFT:** model-authored logic defaults to `mangle_synth_v1`, a bounded
schema that represents clauses, atoms, comparisons, negation, and transform
statements. Free-form text remains a compatibility input that must pass the same
normalization, parse, schema, protected-head, arity, and stratification checks.

Every generation session has per-attempt and total timeouts, a retry budget, and
a terminal rejection result. A failure is a typed learning signal, not permission
to loosen validation.

### One explanation and replay contract

**PROPOSED UPLIFT:** an evaluation can emit a redacted, bounded receipt containing:

- program, schema, policy, and EDB fingerprints;
- full or differential mode and the effective evaluator options;
- input delta identity and fallback reason;
- created-fact count, strata, duration, and terminal error class;
- proof/provenance correlation IDs for selected derived facts.

Replay consumes that receipt in a no-effects sandbox and asserts parity. It does
not repeat tool calls, store prompt bodies, or replace constitutional policy.
`mangle-explainable-replay-v1` in [TODO.md](TODO.md) pins this horizon.

## Desired boundaries

| Concern | Target owner | Explicit non-owner |
|---|---|---|
| Natural-language similarity | perception, embeddings, retrieval | Mangle exact unification |
| Program parsing and analysis | `internal/mangle` chokepoints + mangle-go | shards calling raw parsers |
| Constitutional permission | core schemas and policy | learned rules, synth, feedback |
| Model rule proposal | system shards + feedback/synth | core policy silently accepting text |
| Effect execution | VirtualStore and registered tools | Mangle engine |
| Prompt text and selection budgets | prompt/articulation | `.mg` policy files |
| Evaluation receipt schema | `planned: internal/mangle` with observability consumer | unstructured log scraping |

## Success is falsifiable

The vision is not met until all of these can fail a regression test:

1. A learned head attempts to define `permitted`, an approval, or a pipeline
   result and is rejected.
2. An undeclared predicate, wrong arity, atom/string mismatch, unsafe negation,
   negative cycle, or malformed aggregation cannot reach learned storage.
3. The same finite program and EDB produce result parity in full and differential
   modes with a low created-fact limit; both stop at the limit.
4. Concurrent kernel, synth, sanitizer, and CLI parses pass under `go test -race`.
5. Structured generation loops terminate at configured attempt, session, and
   timeout boundaries.
6. A replay receipt reproduces the selected derivation but cannot execute a tool
   or reveal a redacted payload.
7. An operator can distinguish full, differential, fallback, rejected, and
   resource-exhausted outcomes without reading source code.

## Non-goals

- Mangle is not a semantic search engine, regular-expression engine, or string
  transformation language.
- This package does not decide product intent or write `permitted/3` policy.
- A receipt is not a second fact store, a prompt archive, or a license to retain
  secrets.
- Differential evaluation is not required for every workload; correct visible
  fallback is a valid outcome.
- Structured synth does not remove model creativity. It constrains the interface
  between creativity and executive logic.

## Dependency-ordered horizon

1. Preserve verified created-fact-limit parity while extending the explicit
   fallback contract for other evaluator options.
2. Route every production parser call through the process-wide lock and prove it
   under race.
3. Make the package-local intent corpus's non-runtime role explicit or remove the
   duplicate authority after consumers migrate.
4. Standardize structured synth across every model-authored-rule producer.
5. Unify proof, provenance, diagnostics, and bounded evaluation receipts.
6. Introduce explainable no-effects replay.
7. Pursue true delta propagation only with full/diff semantic, safety, and
   observability parity oracles.
