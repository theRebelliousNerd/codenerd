# Exact model pinning must preserve serving tags

CodeNERD maintainers should treat serving-model tag preservation as a conformance requirement for exact prompt pins. The current normalization makes `qwen3:8b` and `qwen3:32b` equivalent before evidence grouping and Mangle admission. This can pool failures across distinct serving models and make an atom pinned to one eligible for the other. The remedy is a versioned identity contract, not another Mangle safety rule or a new learning architecture.

## Scope and ownership

This assessment is pinned to main commit `9dcb01b9736bf4763aa3ac391d29f133d73c6611`, rechecked unchanged at 19:20 UTC on October 29, 2026. The eleven primary files were identical at Mangle commit `a53cfc308e8836c72370adaa46deba3f7b00180d`, session commit `80af1863d3a3aa46a722ac8b0c8a0985582d4995`, and OAuth commit `aa7988dec4c62116656ea722ff73b687a8509908`. Routing claims apply to pinned main; OAuth executor routing differs. These are source-derived predictions, not executed tests or observed performance results. No model was downloaded or called.

A bounded screen of 110 top-level audit documents on main and research commit `ff603b35653abf15148d22e617cb1824d869e534` found no treatment of tagged serving-model pin equivalence. This addendum complements [prompt evolution lifecycle and evidence accounting](https://github.com/theRebelliousNerd/codenerd/blob/9dcb01b9736bf4763aa3ac391d29f133d73c6611/Docs/audits/prompt-evolution-lifecycle-and-evidence-accounting.md), which owns lifecycle and served-candidate receipts. [Generation-aware optimization](https://github.com/theRebelliousNerd/codenerd/blob/9dcb01b9736bf4763aa3ac391d29f133d73c6611/Docs/audits/jit-generation-aware-optimization.md) owns cache scope. No collision of complete cache keys is claimed here; a cold compilation is sufficient.

Serving-model pin identity is also distinct from vector representation and the [prior JIT learning semantic identity audit](https://github.com/theRebelliousNerd/codenerd/blob/ff603b35653abf15148d22e617cb1824d869e534/Docs/audits/jit-learning-semantic-identity-admission.md), which concerns vector identity rather than serving-model tags.

## The current equivalence is deliberate but too broad for exact scope

The current [`PinScopeModel` contract](https://github.com/theRebelliousNerd/codenerd/blob/9dcb01b9736bf4763aa3ac391d29f133d73c6611/internal/autopoiesis/prompt_evolution/types.go#L233-L265) makes exact model scope the default; family and provider scopes require explicit opt-in. However, [`NormalizeModelToken`](https://github.com/theRebelliousNerd/codenerd/blob/9dcb01b9736bf4763aa3ac391d29f133d73c6611/internal/prompt/pinning.go#L97-L124) lowercases, keeps the last path segment, strips recognized vendor prefixes, and removes any colon suffix. The comment describes Bedrock-style suffixes, but the operation is not provider-specific. Both Ollama identifiers consequently become `qwen3`.

This is not merely an untested regression: an [existing test](https://github.com/theRebelliousNerd/codenerd/blob/9dcb01b9736bf4763aa3ac391d29f133d73c6611/internal/prompt/pinning_test.go#L31-L48) explicitly expects `gemma4:12b` to normalize to `gemma4`. A fix must revise that established equivalence, its tests, and persisted representations rather than silently changing a helper.

Ollama's [official Qwen3 tag listing](https://ollama.com/library/qwen3/tags), inspected at 19:14 UTC, identifies `qwen3:8b` with manifest prefix `500a1f067a9f` and `qwen3:32b` with `030ee887880f`. These are distinct public fixtures. Different tag strings do not always imply different weights: aliases can coincide. Nevertheless, exact identity should preserve their distinction absent a justified alias rule. Tag identity is also not immutable weight identity; a mutable provider alias remains a limitation even after this repair.

## Evidence grouping can cross the promised boundary

For a typed serving pin containing provider `ollama` and model `qwen3:8b`, [`pinSelectors`](https://github.com/theRebelliousNerd/codenerd/blob/9dcb01b9736bf4763aa3ac391d29f133d73c6611/internal/autopoiesis/prompt_evolution/atom_generator.go#L298-L315) produces provider selector `ollama` and model selector `qwen3` under exact scope. [`applyPin`](https://github.com/theRebelliousNerd/codenerd/blob/9dcb01b9736bf4763aa3ac391d29f133d73c6611/internal/autopoiesis/prompt_evolution/pin_policy.go#L36-L62) requires provider, model, and role, then overwrites and normalizes selectors. The producer therefore creates the lossy pin legitimately; malformed input is unnecessary.

The [feedback collector](https://github.com/theRebelliousNerd/codenerd/blob/9dcb01b9736bf4763aa3ac391d29f133d73c6611/internal/autopoiesis/prompt_evolution/feedback_collector.go#L532-L594) reads unconsumed failures, computes `GroupKeyFor` before applying the minimum count, and normalizes the recorded model under exact scope. Consider otherwise qualifying failures with the same problem, role, and provider: two from `qwen3:8b` and one from `qwen3:32b`. At threshold three, the current equivalence predicts one qualifying group of three. Exact partitioning produces groups of two and one, neither qualifying.

This does not guarantee generation. All three records must survive agent-mistake and verdict/version filtering; the [evolution cycle](https://github.com/theRebelliousNerd/codenerd/blob/9dcb01b9736bf4763aa3ac391d29f133d73c6611/internal/autopoiesis/prompt_evolution/evolver_cycle.go#L174-L223) rechecks the threshold and retains shared-cause, claim, and budget gates. Its [`servingPinFor`](https://github.com/theRebelliousNerd/codenerd/blob/9dcb01b9736bf4763aa3ac391d29f133d73c6611/internal/autopoiesis/prompt_evolution/evolver_cycle.go#L343-L356) chooses the first record with raw provenance rather than requiring raw-model equality throughout the group.

## Admission faithfully enforces the wrong identity

The [Ollama client](https://github.com/theRebelliousNerd/codenerd/blob/9dcb01b9736bf4763aa3ac391d29f133d73c6611/internal/perception/client_ollama.go#L22-L60) passes the configured model unchanged into its OpenAI-compatible transport. Its [`ModelIdentity`](https://github.com/theRebelliousNerd/codenerd/blob/9dcb01b9736bf4763aa3ac391d29f133d73c6611/internal/perception/client_ollama.go#L134-L139) returns provider `ollama` and that model. The default configuration supplies no model, so the witness requires an explicitly configured tag rather than an assumed default. The [session executor](https://github.com/theRebelliousNerd/codenerd/blob/9dcb01b9736bf4763aa3ac391d29f133d73c6611/internal/session/executor.go#L1492-L1510) takes identity from the actual `llmForVerb` client into the compilation context.

Now consider an existing, legitimately generated, reviewed, and promoted atom from the 8b model. Compile cold for the same role on 32b, with ordinary context, capability, conflict, and budget checks otherwise satisfied. [`ModelPinTokens`](https://github.com/theRebelliousNerd/codenerd/blob/9dcb01b9736bf4763aa3ac391d29f133d73c6611/internal/prompt/context.go#L425-L440) reduces the serving identity to the same token. [Atom selector normalization and Go matching](https://github.com/theRebelliousNerd/codenerd/blob/9dcb01b9736bf4763aa3ac391d29f133d73c6611/internal/prompt/atoms.go#L312-L331) use the same normalization, with [`MatchesContext`](https://github.com/theRebelliousNerd/codenerd/blob/9dcb01b9736bf4763aa3ac391d29f133d73c6611/internal/prompt/atoms.go#L499-L525) accepting matching exact or family tokens.

The compiler emits [model and provider selector facts](https://github.com/theRebelliousNerd/codenerd/blob/9dcb01b9736bf4763aa3ac391d29f133d73c6611/internal/prompt/selector_facts.go#L146-L160) and [context facts](https://github.com/theRebelliousNerd/codenerd/blob/9dcb01b9736bf4763aa3ac391d29f133d73c6611/internal/prompt/context.go#L858-L870). Mangle's [`satisfied_constraint` rule](https://github.com/theRebelliousNerd/codenerd/blob/9dcb01b9736bf4763aa3ac391d29f133d73c6611/internal/core/defaults/jit_compiler.mg#L68-L85) joins equal tags; its [model constraints fail closed](https://github.com/theRebelliousNerd/codenerd/blob/9dcb01b9736bf4763aa3ac391d29f133d73c6611/internal/core/defaults/jit_compiler.mg#L139-L161). Those rules correctly enforce the information they receive. The lost tag, not a missing logic guard, permits cross-model eligibility.

Eligibility is not guaranteed inclusion. The [evolved atom manager](https://github.com/theRebelliousNerd/codenerd/blob/9dcb01b9736bf4763aa3ac391d29f133d73c6611/internal/prompt/evolved_atoms.go#L278-L305) context-matches promoted candidates; quarantined candidates require exact-version trial admission, and trial copies are not mandatory. Selection fit remains necessary. Fixtures should use the typed generator or accepted canonical selectors: the [schema parser](https://github.com/theRebelliousNerd/codenerd/blob/9dcb01b9736bf4763aa3ac391d29f133d73c6611/internal/prompt/atom_schema.go#L631-L644) rejects raw colon-bearing model values, so raw-tag YAML is not a valid witness.

If the candidate survives compilation and selection fit, the [session consumer](https://github.com/theRebelliousNerd/codenerd/blob/9dcb01b9736bf4763aa3ac391d29f133d73c6611/internal/session/executor.go#L1430-L1448) assigns the compiled prompt to `systemPrompt` and passes it to `runToolLoop`; the identity collision alone does not guarantee emission of the candidate body.

## Limits on impact

[Factory initialization](https://github.com/theRebelliousNerd/codenerd/blob/9dcb01b9736bf4763aa3ac391d29f133d73c6611/internal/system/factory_learning.go#L91-L171) installs the evolved manager and trial sink. [Automatic evolution defaults off](https://github.com/theRebelliousNerd/codenerd/blob/9dcb01b9736bf4763aa3ac391d29f133d73c6611/internal/system/factory_learning.go#L373-L386); manager registration is not conditional on that flag. This is not a claim of default automatic exploitation or measured performance harm.

Unpinned atoms remain unpinned. Missing-model and different-provider contexts still fail the applicable pin checks; distinct non-colon model identifiers need not collide. Approved trials, version hashes, generation gates, capabilities, and permissions remain real controls. The finding concerns exact identity and evidence attribution, not authority expansion.

## Recommended identity contract

Represent exact scope as provider plus exact served identifier, encoded into a versioned, collision-preserving, Mangle-safe token. Keep the opaque identifier available as provenance. Do not globally remove colon suffixes. If transport decoration must be normalized, use a provider-aware adapter with explicit, documented alias evidence and dedicated acceptance cases. Do not infer aliases through an LLM or substitute a different model.

Keep family and provider policies separate. Mangle should remain the admission authority; Go should supply faithful identity facts. Review existing Bedrock suffix, date, and routing equivalences intentionally. Exact tags alone cannot prove immutable weights, configuration, or behavioral interchangeability.

## Migration must preserve evidence ownership

Version canonicalization across stored selectors, relevant context and cache versioning, and candidate digests. Rebuild pins and group derivations from available raw provenance. [Startup migration](https://github.com/theRebelliousNerd/codenerd/blob/9dcb01b9736bf4763aa3ac391d29f133d73c6611/internal/autopoiesis/prompt_evolution/pin_migration.go#L34-L75) already repins from raw generated-atom provider and model, but calls the same lossy policy today; it is a scaffold, not an existing rescue. [Recorded feedback](https://github.com/theRebelliousNerd/codenerd/blob/9dcb01b9736bf4763aa3ac391d29f133d73c6611/internal/system/factory_learning.go#L308-L315) also preserves provider and model.

A candidate produced from mixed-model evidence cannot safely be repaired merely by repinning it to its one recorded generation model. Quarantine or review ambiguous mixed provenance. Preserve consumed-evidence claims and idempotence; do not automatically regenerate or repay for all historical evidence. Unknown legacy identity must not become a guessed narrow pin.

Changed candidate hashes should invalidate prior trial envelopes and require explicit reapproval, not silently inherit authorization. Document migration for authored canonical YAML. Do not retain the old broad token as an exact-scope compatibility alternative, which would restore the bypass. Plan backups, atomic failure handling, and resumable migration before enabling the new version.

## Proposed verification without model calls

The following checks are proposed, not run. Use controlled fake Ollama `ModelIdentity` clients, actual pin and grouping helpers, the canonical parser and persistence path, real-kernel admission alongside Go matching, and mocked factory/session requests.

| Case | Required result |
| --- | --- |
| Exact 8b versus 32b | Distinct tokens and groups; an 8b pin matches 8b and refuses 32b |
| Missing identity or different provider | Applicable pinned atom refused; unpinned behavior unchanged |
| Two 8b failures and one 32b failure at threshold three | Neither exact group qualifies; role and problem boundaries also preserved |
| Proven aliases and existing routing forms | Only intentionally accepted equivalences survive, including reviewed Bedrock cases |
| Canonical authored selectors and persisted candidates | Parser, reload, promoted matching, and authorized trials agree |
| Unknown or mixed legacy provenance | No guessed authorization; claims remain idempotent across retries and interrupted migration |

Measure wrong-group and wrong-admission counts first, then useful-atom recall under the approved scope. Record cache hits and misses, CPU, allocations, fact bytes, migration I/O, and generation and review budgets. Compare total turn quality and failed work on matched models, tasks, tools, and budgets before claiming benefit; identity correctness alone does not guarantee better outputs. No inference or training is required to establish the identity contract discrepancy.

## Costs and acceptance decision

Encoding costs scale with identifier length and can increase encoded fact, manifest, and cache bytes. Migration adds reads, writes, backups, failure handling, and human review. Splitting groups yields fewer observations per group and can delay eligibility. Narrower identity can create more prompt variants and reduce reuse where otherwise equal complete keys are separated. Separate groups that eventually qualify may require additional generation calls. Stricter exact scope may also omit useful transfer, which should be evaluated against explicitly opted-in family scope rather than recovered accidentally. Accept the contract change for faithful exact admission and attribution, not for an unmeasured latency, quality, or spending improvement.
