---
doc-class: research-proposal
subsystem: embedding-long-input
implementation-status: proposed-refinement
evidence-status: static-source-and-algebraic-analysis
last-verified: 2026-11-04
verified-against: 0023cedb706b87419436a3a3fce58ee0e932150a
supersedes: []
---

# Embedding pooling needs an explicit recursive weight contract

## Decision

Specify whether long-input pooling is a global length-weighted centroid or an intentionally hierarchical transform before changing its arithmetic. The current Ollama path normalizes every intermediate pair, discards that pair's resultant magnitude, and then gives its unit direction the weight of the entire parent substring. Consequently, leaf influence depends on the recursive split tree as well as leaf lengths. With three fixed, equal-length, unit-vector leaves, this changes the final direction and can reverse a cosine top-one result. [Recursive producer][recursive] [Binary pooling][pool] [Ranking consumer][ranking]

**Recommended contract:** for an already chosen set of successful chunks, preserve each chunk's declared weight through an associative accumulation, then normalize once at the pooled result. Changing how those same chunks are grouped for execution must not change their mathematical weights. This does not require native whole-text and chunked embeddings to be equal, nor does it assert that one pooled vector is the best retrieval representation.

The important qualification is that the current comments describe a normalized mean of the two halves. They accurately describe the binary helper and do **not** explicitly promise a flat centroid over all terminal chunks. The finding is therefore a concrete, consequential **composition-contract gap**, not proof that `poolVectors` violates its own two-input formula. If hierarchical pooling is deliberate, retain it only with an explicit representation contract, topology identity and evaluation. Do not describe it as preserving only global chunk-length weights.

This is a focused extension to the existing embedding representation proposal, which already calls for representation versioning and warns that pooling is lossy. The new contribution is the precise nested-normalization witness, its reachable store/recall consequence, and a bounded acceptance contract. It does not reopen the already repaired batch truncation or generic model-stamp defects. [Existing representation audit][prior]

## Evidence and execution boundary

Public `main` resolved to **0023cedb706b87419436a3a3fce58ee0e932150a** on November 4, 2026. All repository links below use that commit. Root guidance, scoped embedding guidance, embedding architecture, current source, authored tests and the neighboring audits were inspected. Current campaign/recurse guidance and the existing recurse audit inventory were screened first; no separate novel recurse finding was established in that bounded screen. This report selects the independent long-input numerical boundary rather than repackaging those issues. [Commit][pin] [Root guidance][root] [Embedding guidance][guidance] [Architecture][architecture]

No repository program, build, test, gate, benchmark, model inference, training or provider request was run. The fixture, call sequence, vectors and rankings below are deductions from the pinned code and elementary algebra, not captured runtime output. Source files and research material were read or copied into scratch storage; no implementation, repository file, credentials or settings changed.

| Status | What is established |
|---|---|
| Implemented, source verified | Ollama single-input recursion, context-length refusal classification, rune-based split weights, normalization at every merge, ordinary engine attachment and cosine recall. |
| Implemented, limited protection | Leaf vectors are validated and checked against configured dimension; incomplete piece execution returns an error rather than a partial pooled vector. These checks do not specify the composite weighting rule. |
| Partially specified | The code explains pairwise normalized means; the prior audit calls for versioned pooling and quality evaluation. Neither inspected source specifies whether invariance to regrouping fixed terminal chunks is required. |
| Proposed here | A fixed-leaf weight-conservation contract, associative accumulator if that contract is chosen, explicit hierarchical alternative, and corresponding migration/acceptance work. |
| Unverified | Runtime reproduction, effect frequency in a real corpus, provider-vector distribution, task quality, latency, cost and any superiority of flat pooling. |

The architecture corpus contains older model/default/line-number descriptions. It was read for intent, not treated as proof over newer implementation. Root guidance explicitly requires reading code where generated architecture claims are stale. [Root guidance][root] [Architecture][architecture]

## Actual producer-to-consumer path

### Recursive pooling is reachable in ordinary embedding

`OllamaEngine.Embed` checks that text is embeddable and invokes `embedLong` with the engine's shared learned ceiling and `embedOnce`. A response containing the context-length failure text becomes `InputTooLongError`; the recursive helper reacts to that typed error by splitting. The provider leaf path rejects invalid vectors and dimension mismatches before returning a successful leaf. [Single-input entry][entry] [Refusal and validation][provider]

`embedLong` either returns one successful provider vector or recursively embeds both halves and passes those returned vectors to `poolVectors`. It supplies the rune length of each immediate substring, not an accumulator describing the successful terminal chunks below it. `poolVectors` computes a weighted two-vector mean and immediately scales every nonzero result to unit length. Its return type carries only the normalized vector; the pre-normalization magnitude cannot reach the next merge. [Recursive producer][recursive] [Binary pooling][pool]

Natural boundaries are chosen in the middle half of the substring, preferring paragraph breaks, line breaks, punctuation and spaces before a midpoint fallback. Boundary-based splits trim surrounding whitespace. The resulting tree is thus an operational result of available text boundaries, provider refusals and the learned ceiling, not a separately declared semantic weighting tree. This description does not prove that any particular provider produces harmful chunks. [Split selection][split] [Ceiling][ceiling]

### The vector can be stored and ranked without another pooling check

Normal boot constructs the configured embedding engine and attaches it to the local store when available. `StoreVectorWithEmbedding` invokes ordinary `Embed` for an engine without the task-aware interface, then serializes the result with its engine-space stamp. Semantic recall embeds the query and selects the indexed or brute-force cosine path. The brute-force consumer calculates cosine similarity, sorts descending and applies the requested limit. [Factory wiring][factory] [Store producer][store] [Recall entry][recall] [Ranking consumer][ranking]

The current engine-name eligibility gate is useful: it prevents known different model identities from being ranked together. It does not inspect a pooling tree or prove that a normalized aggregate conserves terminal-chunk weights. The counterexample uses the same model identity and dimensions throughout, so it needs no alias change, stale model, migration failure, malformed vector or unsupported concurrent writer. [Space identity][identity]

Native Ollama batching is already present. A successful native batch does not use this recursive pooling path. A refused multi-text chunk falls back to per-text `Embed`, and a one-text chunk also takes that route. The finding therefore applies when splitting actually occurs, not to every embedding or every batch. GenAI's inspected path is separate; this report does not generalize the Ollama recursion to it. [Batch routing][batch] [GenAI implementation][genai]

## Concrete three-leaf witness

This is a proposed provider-double fixture, not an executed model experiment. It deliberately uses unit vectors so no result depends on unequal native vector norms.

Configure a two-dimensional Ollama test engine with a fixed installed model identity. Let the text be:

```text
X repeated 128 times, immediately followed by Y repeated 128 times,
then one newline, then Z repeated 128 times.
```

There are no spaces or other boundaries inside the three repeated-character blocks. The complete text has 385 runes. The double returns a context-length error for inputs longer than 128 runes and returns these successful vectors:

| Exact successful text | Rune weight | Vector |
|---|---:|---|
| `X` repeated 128 times | 128 | A = (1, 0) |
| `Y` repeated 128 times | 128 | B = (0, 1) |
| `Z` repeated 128 times | 128 | C = (1, 0) |

The first call uses a fresh ceiling. Source-derived execution is:

1. The 385-rune request is refused. The splitter searches its middle region, with `lo=96` and `hi=289`, and finds the newline at index 256. Cutting after it and trimming yields a 256-rune left substring and 128-rune right substring.
2. The 256-rune left request is refused. It has no natural separator and splits at its midpoint into the 128-rune X and Y blocks.
3. Both left leaves succeed. The remembered refusal length is 256, so each 128-rune leaf remains eligible for a direct request.
4. The 128-rune right Z block also succeeds.
5. The helper normalizes A and B before combining their returned unit direction with C. [Recursive producer][recursive] [Split selection][split]

The embedding-request lengths are therefore 385, 256, 128, 128, 128 under this schedule. That excludes any model-readiness request and is not a general provider-call estimate.

Ignoring small float32 rounding, the left aggregate is:

```text
L = normalize(A + B) = (1, 1) / sqrt(2)
```

The current root gives L weight 256 and C weight 128:

```text
H = normalize(2L + C)
  = normalize((sqrt(2) + 1, sqrt(2)))
```

A single length-weighted accumulation over the same successful leaves is:

```text
F = normalize(A + B + C)
  = (2, 1) / sqrt(5)
```

These vectors have different directions. The distinction is not an omitted final normalization: both are already unit vectors. At the second merge, normalization has amplified the two-leaf resultant before the subtree receives its full rune weight.

At each merge, the helper normalizes the weighted sum of its two child outputs. That local sum need not equal the weighted sum of the terminal leaves below it, and its magnitude is unavailable to the next merge. Relative to sum pooling, an intermediate group's agreement or cancellation changes the scale that is discarded. Near-cancellation and heterogeneous subtrees therefore require explicit controls; identical leaf directions hide the difference. This is algebra about the selected vectors, not a claim about how often an embedding model emits any such arrangement.

### A ranking consequence, without a quality claim

Use query Q = (0, 1), and another document with unit vector D = `(sqrt(481)/25, 12/25)`. The competing document's cosine score is exactly 0.48.

- Flat accumulation gives `cos(Q,F) = 1/sqrt(5)`, about 0.4472.
- Current recursive normalization gives `cos(Q,H) = sqrt(2)/sqrt(5+2sqrt(2))`, about 0.5054.
- Thus the pooled document ranks below D under F and above D under H.

The inspected brute-force consumer can expose that difference at `limit=1` because it sorts these scores and then truncates. The example establishes that the arithmetic is observable at a real consumer, rather than only a different internal norm. It does **not** establish which document is semantically more relevant, that a live model produces these vectors for the chosen strings, or that a production answer has been wrong. An indexed-path witness remains an explicit unrun acceptance case. [Ranking consumer][ranking]

## What existing tests establish and miss

The multi-paragraph long-input fixture returns the same vector for every accepted piece and checks output dimension, unit norm, boundary behavior and learned-ceiling reuse. Identical directions make hierarchical and flat aggregation coincide, so that test cannot distinguish the two contracts. [Multi-level fixture][tests-multi]

The Unicode fixture does check unequal rune weights, but its accepted inputs are one left leaf and one right leaf. It is a good check of the two-input weighting rule and byte/rune distinction. It does not require the weight law to compose through an intermediate normalized result. Its opposite-vector case also intentionally accepts an all-zero aggregate. Separate tests ensure a leaf error does not return partial output; preserve that guarantee. [Two-leaf fixtures][tests-two] [Piece-error fixture][tests-error]

These are authored tests read at the source pin. Nothing in this report treats them as passing in this session or claims that the entire test corpus lacks every possible overlapping assertion.

## Proposed contract and implementation boundary

### Preferred: preserve terminal weights, normalize at the pooled boundary

For a fixed ordered list of successful terminal chunks and weights, define the pooled result from the sum of `weight × vector`, with one final nonzero normalization. Equivalent parenthesizations of those same weighted terms must agree within a declared floating-point tolerance. Preserve input order for acquisition and receipts; the contract concerns aggregation grouping, not a claim that reordered natural-language text has the same meaning.

A bounded implementation can return an internal aggregate containing:

- A dimension-checked unnormalized weighted sum, accumulated in a declared precision.
- The total weight of the successful terminal chunks.
- Enough status to reject incomplete acquisition rather than return a partial vector.

Merge those aggregates by addition. Normalize only when producing the public pooled vector. Preserve the unsplit successful-provider path unless its normalization contract is deliberately changed. The leaf weights must be accumulated from the successful chunks, rather than reconstructed from ancestor substring lengths: deeper trimming can remove separators, so those two quantities are not always equal. [Split selection][split]

Decide explicitly whether the weighted terms use raw provider vectors or individually normalized leaf vectors. The witness uses unit leaves and is independent of that decision. Silently changing both leaf normalization and recursive grouping would make the evaluation unable to isolate the correction.

Define zero-result behavior rather than assuming every aggregate can be normalized. Current tests admit exact cancellation as a zero vector. Preserve that behavior or change it through an explicit contract with consumer handling. A numerical-quality decision must not silently become a new whole-task retry or permission policy. Scoped guidance keeps embedding a semantic leaf; relevance and executive decisions still belong to their existing consumers and Mangle. [Two-leaf fixtures][tests-two] [Embedding guidance][guidance]

### Alternative: intentionally retain hierarchical pooling

Hierarchical pooling can be a deliberate representation, but then the split tree and local normalization are part of its semantics. Specify why an operational context-limit tree should determine semantic weighting, which variations are compatible, and which measurements would justify that choice. Version the transform and test the exact three-leaf output as intended behavior. A passing dimension or unit-norm check is insufficient evidence for either alternative.

This alternative is not ruled out by the source analysis. The decision requested is concrete: **must fixed successful leaf weights survive regrouping, or is topology-dependent reweighting intentional?** Do not fix the binary helper by assumption while leaving the representation contract ambiguous.

### Persistent-vector transition is part of the work

A change to the pooled direction is a representation change even when provider, model name and dimension remain identical. Current generic vector eligibility compares the engine-name stamp; it will not distinguish old recursive pooling from a new flat transform. Extending identity or choosing a bounded migration is already an open item in the representation audit. Reuse that work instead of inventing a second migration system. [Space identity][identity] [Existing representation audit][prior]

The inspected generic row stores the vector and model stamp without a pooling-mode receipt. Do not assume a maintainer can selectively identify every historically pooled row from its text length: model context behavior and prior split history may differ. Establish a conservative rebuild or explicit compatibility policy before rollout, while preserving keyword availability and existing foreign/unattributed handling. This is proposed migration work, not a claim that current storage is corrupted. [Store producer][store] [Ceiling][ceiling]

## Primary-source context and limits

- OpenAI's long-input embedding recipe is a concrete precedent for collecting chunk vectors and their lengths, computing their weighted average, and normalizing once afterwards. It uses token lengths, whereas this code uses runes. It is an archived recipe, not a claim about current provider limits or a mandate that codeNERD adopt its models or tokenizer. [OpenAI long-input recipe](https://developers.openai.com/cookbook/examples/embedding_long_inputs)
- Zaheer and colleagues' *Deep Sets* studies permutation-invariant set functions and the separation of aggregation from a subsequent transformation. It motivates making aggregation invariants explicit. It does not show that document order is irrelevant or prove that flat pooling is optimal for code search. [Deep Sets, primary paper](https://arxiv.org/html/1703.06114v3)
- Reimers and Gurevych's *Sentence-BERT* evaluates embedding-based similarity using cosine comparison. It supports treating the embedding construction as part of the measured retrieval system, not assuming any unit vector is interchangeable. Its results do not validate this Ollama split strategy or the proposed correction. [Sentence-BERT, primary paper](https://aclanthology.org/D19-1410/)

The counterexample itself follows from the pinned formula, not from transferring another model's empirical results. No citation is offered as proof of task-quality improvement.

## Costs and failure modes

For K successful chunks of dimension d, both pairwise pooling and a sum accumulator perform O(Kd) vector arithmetic. A streaming aggregate need not retain all K leaf vectors; its vector state can be O(d), alongside text/split traversal state. The current recursive implementation also allocates intermediate vectors, so peak-memory differences should be measured rather than assumed. Removing intermediate normalizations can reduce arithmetic work, but no latency claim is made: provider calls and text processing may dominate. [Recursive producer][recursive] [Binary pooling][pool]

For a fixed refusal/split schedule, changing only aggregation requires no additional embedding requests. A conservative corpus reembedding does: it costs corpus reads, provider inference, durable writes and temporary migration state. A richer representation receipt adds storage and compatibility rules. Those costs are real even if the local algebra change is small.

Important failure modes for the proposed design include incorrect leaf-weight accounting after trimming; accidentally normalizing leaves under a new convention; floating-point cancellation or overflow in a larger accumulator; returning an incomplete sum after a leaf error; mixing old/new pooled rows under one identity; and claiming invariant grouping while changing the chunks themselves. The grouping invariant is conditional on fixed successful chunks and weights. It does not eliminate representation changes caused by genuinely different chunk boundaries, provider versions or model outputs.

Do not use additional model deliberation to compensate for this deterministic boundary. A requirements interrogator can help choose the representation contract, but it cannot reconstruct the lost intermediate magnitude from the normalized vector passed to a parent. Keep the decision in the specification and the chosen arithmetic in host code.

## Unrun acceptance and falsification plan

1. **Exact nested witness.** Through `OllamaEngine.Embed` and a scripted local provider, assert the five request lengths and three successful chunks above. Under the preferred contract, require F within a justified float32 tolerance; under a deliberate hierarchical contract, require H and its representation label. Do not test only a hand-built final vector.
2. **Regrouping law.** With fixed leaf vectors and weights, compare balanced, left-nested and right-nested aggregation. Test heterogeneous directions, unequal weights and different depths. Preserve the source order and chunk identity while changing only merge grouping.
3. **Cancellation and zero sums.** Include an intermediate exact cancellation followed by another nonzero chunk, near cancellation, and a final zero sum. Assert finite output and the chosen zero-result disposition. Merely asserting unit norm excludes intentional zero behavior and is not sufficient.
4. **Unicode and trimming.** Retain the existing multibyte/rune cases, then add boundaries trimmed at more than one level. Account from successful terminal chunks and make any excluded separator policy explicit.
5. **Leaf norm convention.** Include non-unit provider vectors to distinguish raw-vector weighting from normalized-leaf weighting. Require the selected convention, not whichever passes the unit-leaf witness.
6. **Failure and cancellation controls.** Preserve no-partial-result behavior for either child failure, context cancellation and dimension mismatch. Avoid adding provider retries for a deterministic aggregation error.
7. **Real store/recall composition.** Store the nested document and fixed competitor using the normal embedding entry, then query Q with `limit=1`. Qualify both brute-force and sqlite-vec paths after their normal readiness/repair conditions. Record exact source/model/transform identities and the resulting ranking. The expected mathematical rank depends on the chosen contract, not an assumed semantic ground truth.
8. **Unaffected paths.** Verify unsplit single embeds, successful native batches and configured GenAI behavior do not silently acquire a new normalization or representation rule. Check native-batch fallback separately from native success.
9. **Transition.** Create pre-change pooled rows, activate the new representation contract, and verify the declared migration/eligibility policy. Do not accept equal model names and dimensions as a substitute for a completed transition.
10. **Task and cost evaluation.** Only after authorized execution, compare fixed corpora and queries under hierarchical, flat and, where practical, chunk-level retrieval. Measure retrieval relevance, downstream task outcomes, provider calls, downstream model turns and tokens, wall time, allocations and migration work. Include representative CodeDOM/world-model-enabled tasks where applicable and report their configuration. Use normal production paths and include tail-dependent queries. A more stable centroid is not automatically a better retrieval system.

An explicit contract requiring H resolves the intended-behavior question but does not falsify the demonstrated grouping dependence. Cases with no nested merge, or with subtree directions and magnitudes that make H and F coincide, are controls rather than evidence of general grouping invariance. A runtime reproduction should falsify the proposed call trace if the pinned source follows a different route under the stated fixture. Closing this report requires an explicit pooling decision and evidence that its implementation, persistent identity and ordinary consumer behavior agree.

## Relationship to existing audits

The current audit-tree screen and bounded GitHub code searches found no `poolVectors` discussion in `Docs/audits`. Searches for pooling and `long_input.go` led to the existing representation audit and unrelated embedding/provider references. Search results resolved to the same source pin. This is a bounded novelty check, not a claim about private discussions or every historical branch.

The principal overlap is intentional: `embedding-representation-and-migration.md` already says chunking/pooling should be versioned and evaluated, and distinguishes full-input coverage from semantic equivalence. This report does not claim those requirements were absent. It supplies a previously unanalyzed composition law, a precise three-leaf fixture, an actual cosine consumer, and an accumulator design with leaf-weight and transition requirements. [Existing representation audit][prior]

`embedding-stats-observation-boundary.md` concerns diagnostics triggering runtime/index maintenance. That ownership issue is separate and is neither reproduced nor reclassified here. Existing recurse graph, gate, recovery, metric and adoption findings are likewise not reused as a new recursion report. [Stats audit][stats]

## Pinned repository references

[pin]: https://github.com/theRebelliousNerd/codenerd/commit/0023cedb706b87419436a3a3fce58ee0e932150a
[root]: https://github.com/theRebelliousNerd/codenerd/blob/0023cedb706b87419436a3a3fce58ee0e932150a/agents.md
[guidance]: https://github.com/theRebelliousNerd/codenerd/blob/0023cedb706b87419436a3a3fce58ee0e932150a/internal/embedding/agents.md
[architecture]: https://github.com/theRebelliousNerd/codenerd/blob/0023cedb706b87419436a3a3fce58ee0e932150a/Docs/architecture/embedding/INTERNALS.md
[entry]: https://github.com/theRebelliousNerd/codenerd/blob/0023cedb706b87419436a3a3fce58ee0e932150a/internal/embedding/ollama.go#L95-L106
[provider]: https://github.com/theRebelliousNerd/codenerd/blob/0023cedb706b87419436a3a3fce58ee0e932150a/internal/embedding/ollama.go#L205-L267
[ceiling]: https://github.com/theRebelliousNerd/codenerd/blob/0023cedb706b87419436a3a3fce58ee0e932150a/internal/embedding/long_input.go#L24-L50
[recursive]: https://github.com/theRebelliousNerd/codenerd/blob/0023cedb706b87419436a3a3fce58ee0e932150a/internal/embedding/long_input.go#L52-L83
[split]: https://github.com/theRebelliousNerd/codenerd/blob/0023cedb706b87419436a3a3fce58ee0e932150a/internal/embedding/long_input.go#L85-L117
[pool]: https://github.com/theRebelliousNerd/codenerd/blob/0023cedb706b87419436a3a3fce58ee0e932150a/internal/embedding/long_input.go#L119-L141
[batch]: https://github.com/theRebelliousNerd/codenerd/blob/0023cedb706b87419436a3a3fce58ee0e932150a/internal/embedding/ollama.go#L278-L408
[genai]: https://github.com/theRebelliousNerd/codenerd/blob/0023cedb706b87419436a3a3fce58ee0e932150a/internal/embedding/genai.go#L153-L245
[factory]: https://github.com/theRebelliousNerd/codenerd/blob/0023cedb706b87419436a3a3fce58ee0e932150a/internal/system/factory.go#L1731-L1774
[store]: https://github.com/theRebelliousNerd/codenerd/blob/0023cedb706b87419436a3a3fce58ee0e932150a/internal/store/vector_store.go#L132-L218
[recall]: https://github.com/theRebelliousNerd/codenerd/blob/0023cedb706b87419436a3a3fce58ee0e932150a/internal/store/vector_store.go#L462-L515
[ranking]: https://github.com/theRebelliousNerd/codenerd/blob/0023cedb706b87419436a3a3fce58ee0e932150a/internal/store/vector_store_bruteforce.go#L16-L81
[identity]: https://github.com/theRebelliousNerd/codenerd/blob/0023cedb706b87419436a3a3fce58ee0e932150a/internal/store/vector_identity.go#L15-L90
[tests-multi]: https://github.com/theRebelliousNerd/codenerd/blob/0023cedb706b87419436a3a3fce58ee0e932150a/internal/embedding/long_input_test.go#L18-L84
[tests-two]: https://github.com/theRebelliousNerd/codenerd/blob/0023cedb706b87419436a3a3fce58ee0e932150a/internal/embedding/long_input_test.go#L108-L203
[tests-error]: https://github.com/theRebelliousNerd/codenerd/blob/0023cedb706b87419436a3a3fce58ee0e932150a/internal/embedding/long_input_test.go#L207-L245
[prior]: https://github.com/theRebelliousNerd/codenerd/blob/0023cedb706b87419436a3a3fce58ee0e932150a/Docs/audits/embedding-representation-and-migration.md
[stats]: https://github.com/theRebelliousNerd/codenerd/blob/0023cedb706b87419436a3a3fce58ee0e932150a/Docs/audits/embedding-stats-observation-boundary.md
