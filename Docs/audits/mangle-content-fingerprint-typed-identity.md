# Content fingerprints must preserve admitted Mangle value types

## Decision, pin and scope

Keep the new constant-time kernel fingerprint, but do not use the existing, type-erasing deduplication key as the complete semantic content identity. In the current implementation, an integer and an integral floating-point value can have the **same fingerprint input while remaining different admitted Mangle constants**.

Source freeze: CodeNERD main **`9dcb01b9736bf4763aa3ac391d29f133d73c6611`**, rechecked 2026-10-29 at 10:13 UTC. The optimization landed in `df76c7fd82deea7a54ee45d6dc8b49225889c043`; this is a **current-main** finding, not a branch-only concern.

**What is established:** a source-supported equality-contract defect for supported polymorphic/custom schemas. The exact canonical keys collide *before hashing*; this is not a speculative FNV collision, a birthday-bound argument, or an attack on SHA-256.

**What remains conditional:** stale JIT output additionally requires a stable nonvolatile program, unchanged separately hashed injections/context/non-kernel sources, a retained prompt-cache entry, and a selection/rendering dependency that distinguishes the typed values. No shipped mixed-type producer-to-selection incident was established. The concrete JIT composition below is a proposed trusted/custom-policy fixture, not a claim that ordinary stock prompts are currently wrong.

This review included current guidance, the previous JIT/performance audits and research through `research-main-20261029-0946`, plus independent source challenge. No implementation, tests, execution, inference, training, profiling, local Git or runtime reproduction was performed.

## 1. The exact semantic alias

The content fingerprint hashes the compiled program key, the two accumulated EDB digest words and the fact count. Each contribution is based on the predicate and its existing canonical deduplication key. Only programs with volatile external-dependent heads additionally include instance/generation identity. [Fingerprint implementation](https://github.com/theRebelliousNerd/codenerd/blob/9dcb01b9736bf4763aa3ac391d29f133d73c6611/internal/core/kernel_content_fingerprint.go#L19-L86), [index contribution maintenance](https://github.com/theRebelliousNerd/codenerd/blob/9dcb01b9736bf4763aa3ac391d29f133d73c6611/internal/core/kernel_edb_index.go#L64-L106).

For a fact `fp_value(V)`:

| Host value | Existing canonical key | Admitted value under `bound [/any]` |
|---|---|---|
| `int64(1)` | `fp_value(1)` | `ast.Number(1)` |
| `float64(1)` | `fp_value(1)` | `ast.Float64(1)` |

The canonicalizer formats integers in decimal and floats with `FormatFloat(..., 'g', -1, 64)`, so an integral float loses its type marker in that key. Conversion, however, creates distinct Number and Float64 constants. [Canonical key and values](https://github.com/theRebelliousNerd/codenerd/blob/9dcb01b9736bf4763aa3ac391d29f133d73c6611/internal/core/kernel_facts.go#L158-L209), [float key formatting](https://github.com/theRebelliousNerd/codenerd/blob/9dcb01b9736bf4763aa3ac391d29f133d73c6611/internal/core/kernel_facts.go#L255-L258), [typed conversion](https://github.com/theRebelliousNerd/codenerd/blob/9dcb01b9736bf4763aa3ac391d29f133d73c6611/internal/types/types.go#L255-L262).

Admission does **not** always normalize integral floats into integers. It skips adaptation when the declaration already accepts the converted constant. A permissive `/any` bound accepts both. Integral-float narrowing is the repair for a slot whose bounds reject the float and accept `/number`, not a global conversion. [Admission/adaptation](https://github.com/theRebelliousNerd/codenerd/blob/9dcb01b9736bf4763aa3ac391d29f133d73c6611/internal/core/kernel_fact_decl.go#L158-L211).

The inspected v0.7.0 mirror's constant equality checks type before value; Number and Float64 are different even when they represent the same mathematical quantity. CodeNERD's own readback returns `int64` for Number and `float64` for Float64. [Upstream constructors](https://github.com/google/mangle/blob/7cd2543ac1817f8b92d42efe3c6ef82dededbeeb/ast/ast.go#L313-L320), [type-sensitive equality](https://github.com/google/mangle/blob/7cd2543ac1817f8b92d42efe3c6ef82dededbeeb/ast/ast.go#L746-L773), [host readback](https://github.com/theRebelliousNerd/codenerd/blob/9dcb01b9736bf4763aa3ac391d29f133d73c6611/internal/core/kernel_query.go#L425-L452).

Thus equal program keys, canonical EDB keys and row counts do not necessarily imply equal typed materialized content or pattern answers.

## 2. Minimal proposed witness — not run

Use one trusted program with no external premises:

```mangle
Decl fp_value(V) bound [/any].
Decl fp_integer_present() bound [].
fp_integer_present() :- fp_value(1).
```

First, use two independently created kernels with this identical program:

- Kernel A receives only `Fact{Predicate: "fp_value", Args: []any{int64(1)}}`.
- Kernel B receives only `Fact{Predicate: "fp_value", Args: []any{float64(1)}}`.

After successful evaluation, both have the same canonical EDB identity and count, so the new content fingerprints match. Their `fp_value` readbacks have different Go numeric types, and the integer-literal premise should derive `fp_integer_present()` only in A under the inspected type-sensitive engine semantics.

A same-kernel transition is also important for cache testing:

1. Assert integer 1 and evaluate.
2. Record its content fingerprint and warm the relevant consumer.
3. Retract that exact integer fact through the supported API.
4. Assert floating-point 1 and evaluate again.
5. Compare the final fingerprint and typed/derived answers.

**Retraction is essential.** Simply asserting the float while the integer remains is rejected as a canonical duplicate before conversion, so that is not the witness. The integer is removed first and the float is then genuinely admitted. Normal assertion/retraction dirtying is used; this does not depend on old missing-generation paths, raw store access or argument mutation. [Dedup before conversion, then index insertion](https://github.com/theRebelliousNerd/codenerd/blob/9dcb01b9736bf4763aa3ac391d29f133d73c6611/internal/core/kernel_facts.go#L455-L489), [assertion dirtying](https://github.com/theRebelliousNerd/codenerd/blob/9dcb01b9736bf4763aa3ac391d29f133d73c6611/internal/core/kernel_facts.go#L539-L555), [exact retraction](https://github.com/theRebelliousNerd/codenerd/blob/9dcb01b9736bf4763aa3ac391d29f133d73c6611/internal/core/kernel_facts.go#L922-L951).

This is a strict refinement of an old deduplication equivalence assumption. The new risk is using that equivalence to certify complete content across separately admitted states, not discovering that canonical deduplication already collapses numeric spellings.

## 3. Why the old snapshot hashing distinguished this pair

Before the optimization, the JIT fingerprint walked `QueryAll` and encoded each returned argument as a pair containing its Go type name and value. The Number/Float64 readback distinction therefore made this exact pair different even when ordinary JSON number spelling was equal. The new path replaces those records with `ContentFingerprint()`. [Previous implementation at f84](https://github.com/theRebelliousNerd/codenerd/blob/f84ce0df57b21de3725b8fa02d13e88f74192714/internal/prompt/compiler_kernel.go#L176-L218), [current implementation](https://github.com/theRebelliousNerd/codenerd/blob/9dcb01b9736bf4763aa3ac391d29f133d73c6611/internal/prompt/compiler_kernel.go#L176-L204).

This is not a claim that the old public `Fact` representation preserved every AST distinction—for example, names and strings have separate concerns. It establishes a regression for the specific numeric pair, without requiring a complete defense of the previous representation.

## 4. Conditional JIT integration consequence

Advancing the kernel generation correctly invalidates the **preflight memo**, but does not by itself make the **compiled-prompt key** different. On a preflight miss, the compiler acquires a new scope and computes a new fingerprint. It then builds the result-cache key from the context containing that fingerprint, configuration revision and source vector; a matching result can return before selection. [Preflight generation/memo behavior](https://github.com/theRebelliousNerd/codenerd/blob/9dcb01b9736bf4763aa3ac391d29f133d73c6611/internal/prompt/compiler_preflight.go#L37-L59), [snapshot publication](https://github.com/theRebelliousNerd/codenerd/blob/9dcb01b9736bf4763aa3ac391d29f133d73c6611/internal/prompt/compiler_preflight.go#L91-L105), [result lookup](https://github.com/theRebelliousNerd/codenerd/blob/9dcb01b9736bf4763aa3ac391d29f133d73c6611/internal/prompt/compiler_pipeline.go#L89-L139).

A proposed end-to-end fixture can register a stable, otherwise eligible mandatory atom `probe/atom`, and use a trusted host-owned rule:

```mangle
blocked_by_context("probe/atom") :- fp_value(1).
```

The existing selection rules exclude blocked mandatory atoms, and the Go selector consumes `selected_result`. An uncached integer-state compile should omit the atom, while an uncached float-state compile should admit it, with the remaining eligibility/budget conditions held equal. If the integer-state result is warmed, the numeric transition can nevertheless recreate its result-cache key and bypass the new selection. [Mandatory gating](https://github.com/theRebelliousNerd/codenerd/blob/9dcb01b9736bf4763aa3ac391d29f133d73c6611/internal/core/defaults/jit_compiler.mg#L213-L217), [selection projection](https://github.com/theRebelliousNerd/codenerd/blob/9dcb01b9736bf4763aa3ac391d29f133d73c6611/internal/core/defaults/jit_compiler.mg#L387-L395), [selector consumer](https://github.com/theRebelliousNerd/codenerd/blob/9dcb01b9736bf4763aa3ac391d29f133d73c6611/internal/prompt/selector.go#L1034-L1077).

Required controls:

- No external-dependent heads: volatile programs include instance/generation in their fingerprint and do not follow this reusable-content case.
- Same caller context, config revision, tracked source vector and retained prompt entry.
- No changed `injectable_context` or `specialist_knowledge` output: injected atom IDs/content are separately hashed and can rescue this case.
- A real uncached selection difference must be demonstrated; an unequal internal fact that no prompt consumer reads is an identity-contract failure, not proof of changed prompt text.

## 5. Production applicability and mitigations

The new fingerprint is on the production JIT path through the system adapter, not an unused helper. [Adapter forwarding](https://github.com/theRebelliousNerd/codenerd/blob/9dcb01b9736bf4763aa3ac391d29f133d73c6611/internal/system/factory_adapter_snapshot.go#L9-L30). However, no stock scalar producer was established that switches this exact numeric type under a permissive declaration and then changes JIT selection.

There are real permissive declarations, but their presence alone is not enough. The inspected `intelligence_world_fact` slot is `/any`; its stock producer supplies the entire `wf.Args` container, which `ToAtom` JSON-encodes as a string. That is **not** the scalar Number-versus-Float64 witness. [Declaration](https://github.com/theRebelliousNerd/codenerd/blob/9dcb01b9736bf4763aa3ac391d29f133d73c6611/internal/core/defaults/schemas_intelligence.mg#L9-L11), [producer](https://github.com/theRebelliousNerd/codenerd/blob/9dcb01b9736bf4763aa3ac391d29f133d73c6611/internal/campaign/decomposer.go#L351-L357), [container encoding](https://github.com/theRebelliousNerd/codenerd/blob/9dcb01b9736bf4763aa3ac391d29f133d73c6611/internal/types/types.go#L275-L287).

Narrow `/number` bounds are a positive control: integral floats are adapted into Number, so equal fingerprints for those normalized values are appropriate. Fractional/out-of-range values can fail admission instead. Likewise, changed injections, different source generations, cold/evicted caches and volatile-program identity can prevent stale prompt reuse.

The new fingerprint tests exercise repeated string-valued assertions/retractions, clones, program changes and volatility. Those are useful controls, but their string-only random vocabulary does not cover accepted values sharing a canonical key but differing in admitted type. The tests were read, not run. [Current tests](https://github.com/theRebelliousNerd/codenerd/blob/9dcb01b9736bf4763aa3ac391d29f133d73c6611/internal/core/kernel_content_fingerprint_test.go#L42-L113).

## 6. Contract and alternatives

**Contract:** equality of reusable content identities must imply equality of the typed facts and derived answers that the consumer can observe, modulo an explicit collision policy. A cache identity must not erase a distinction the admission/evaluator preserves.

Prefer maintaining an incremental digest of an unambiguous **admitted typed representation**, rather than of a convenient pre-admission deduplication spelling. Include predicate/arity, constant kind and lossless value encoding; preserve structured-value boundaries. Reuse the conversion already performed at admission where practical.

Program changes and boot-time facts require care: admission is declaration-directed and can re-encode values. A digest of admitted atoms must be refreshed when the program changes their interpretation, and must correspond to the successfully evaluated state. Removal must subtract the same contribution that insertion added. Do not silently change deduplication/retraction semantics merely to repair fingerprint identity.

Alternatives:

- A lossless typed host-input digest can conservatively distinguish extra states, but must cover all supported conversion inputs. False cache misses are safer than false hits; raw type tags alone are not a complete serialization design.
- A verified generation-scoped fallback can retain O(1) lookup while sacrificing cross-generation/content-restoration reuse. It is safe only where mutation-generation coverage is established; it is not a reason to ignore existing epoch-contract work.
- The previous full-snapshot method can serve as a bounded comparison/fallback for this pair, but wholesale restoration would surrender the optimization's intended scaling benefit. Use a typed reference representation for broader parity.

The prior JIT audit already requires identity/selection/injections to describe one private snapshot. This is a new representation mismatch inside the recently substituted identity, not another missing-source-generation or external-cache-epoch finding. [Existing identity contract](https://github.com/theRebelliousNerd/codenerd/blob/9dcb01b9736bf4763aa3ac391d29f133d73c6611/Docs/audits/jit-generation-aware-optimization.md#L50-L68).

## 7. Full costs and fair evaluation — proposed, not run

Maintaining an admitted typed digest adds per-admission serialization/hash work proportional to the changed value, and possibly retained digest storage per fact. A program/declaration change may require a full EDB digest refresh alongside already-required reconversion. Measure that separately from O(1) fingerprint reads. A generation fallback increases correct cache misses, cloning and selection work; a full-snapshot fallback increases scans, allocation and sorting. No model, embedding or training call is needed for the identity repair. No measured speedup, token saving or production failure rate is claimed.

Future evaluation should include:

1. The two-kernel numeric pair and the same-kernel retract/replace sequence, checking fingerprint identity, typed readbacks, literal queries and the derived marker.
2. `/number` normalization, genuinely changed numeric values, float32/float64, name/string and structured values, no-op duplicate admission, removal, program-bound changes, boot loading, clean/dirty clones and both store kinds.
3. The conditional JIT fixture with warm result cache, preflight miss, stable injections/source vector, and a fresh uncached compiler as the selection oracle. Also change injections and enable volatile dependency handling as rescue controls.
4. Existing source-vector, failed-evaluation and scope-isolation guards. Do not weaken them to manufacture cache hits.
5. Mutation throughput, digest rebuild cost, fingerprint latency, cache hit/miss distribution, clone/cone/selection time, allocations and retained memory across world sizes and mutation rates. Compare equal typed answers and selected/rendered atom sets, not just shorter cache-key time.

**Exit condition:** the accepted numeric pair no longer shares a reusable content identity unless admission deliberately normalizes it to the same constant; a warm cache cannot skip a genuinely changed selection; unchanged-content fast paths retain measured benefits.

### Upstream provenance

CodeNERD pins `codeberg.org/TauCeti/mangle-go v0.7.0`. Engine type/equality inspection used the accessible corresponding GitHub mirror commit `7cd2543ac1817f8b92d42efe3c6ef82dededbeeb`; exact downloaded Codeberg module bytes were not independently compared. Host key erasure, typed conversion/readback and JIT key wiring are directly established by CodeNERD source. Runtime derivation predictions remain qualified by that provenance boundary and the absence of an executed reproduction.

