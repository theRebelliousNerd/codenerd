# Exact model pins: numeric serving-tag residual after the alphanumeric fix

## Status and scope

At CodeNERD main `4cb0b3650de09085351014d522beb440c7e97ae3`, inspected on 2026-10-30, `NormalizeModelToken` no longer strips every colon suffix: `qwen3:8b` and `qwen3:32b` retain distinct sanitized tags. **Credit this repair: the original headline witness is fixed.** The remaining exception treats any nonempty all-digit suffix as a Bedrock API revision, although the helper receives no provider argument and does not establish a Bedrock-shaped identifier before applying that exception. [Current normalization](https://github.com/theRebelliousNerd/codenerd/blob/4cb0b3650de09085351014d522beb440c7e97ae3/internal/prompt/pinning.go#L86-L126).

This is a narrow revalidation of the existing [exact-model pin identity audit](https://github.com/theRebelliousNerd/codenerd/blob/4cb0b3650de09085351014d522beb440c7e97ae3/Docs/audits/exact-model-pin-tag-identity.md), not a new broad pinning proposal. The earlier document owns the full production trace, migration and evidence-ownership contract. Its original pre-fix conclusion must not be read as the status of every serving tag today.

The findings below are source-derived conditional predictions, not reproduced incidents. Inspection was read-only; no implementation, tests, inference, training, model downloads or runtime experiments were performed. No measured latency or quality improvement is claimed.

## Numeric spelling is not alias evidence

Official Ollama source at `8861adaa77839c961d3c574232a8865d65a426b6` parses the suffix into `Name.Tag` without deleting numeric values. Validity includes the tag, and character validation accepts digits, including the first character. Equality compares tags; path construction includes the tag. Consequently `local-model:1` and `local-model:2` are valid distinct names. They may intentionally alias the same artifact, but their spelling does not establish equivalence; they can designate different local artifacts. No such models were created or queried in this review. [Parser](https://github.com/ollama/ollama/blob/8861adaa77839c961d3c574232a8865d65a426b6/types/model/name.go#L146-L158), [validity](https://github.com/ollama/ollama/blob/8861adaa77839c961d3c574232a8865d65a426b6/types/model/name.go#L268-L281), [character validation](https://github.com/ollama/ollama/blob/8861adaa77839c961d3c574232a8865d65a426b6/types/model/name.go#L344-L375), [path and equality](https://github.com/ollama/ollama/blob/8861adaa77839c961d3c574232a8865d65a426b6/types/model/name.go#L292-L314).

This establishes a valid identifier fixture, not immutable weight identity. Mutable serving aliases, model configuration and artifact provenance remain separate issues.

## Producer-to-consumer residual

Under the current normalizer, both names become `local_model`. The relevant downstream consumers remain live:

| Boundary | Current effect |
|---|---|
| Exact atom pin producer | `pinSelectors` normalizes `ServingPin.Model` to the collapsed exact token. [Source](https://github.com/theRebelliousNerd/codenerd/blob/4cb0b3650de09085351014d522beb440c7e97ae3/internal/autopoiesis/prompt_evolution/atom_generator.go#L298-L315) |
| Exact failure-group producer | `GroupKeyFor` independently normalizes the recorded model with the same helper, while retaining provider, role and problem boundaries. [Source](https://github.com/theRebelliousNerd/codenerd/blob/4cb0b3650de09085351014d522beb440c7e97ae3/internal/autopoiesis/prompt_evolution/feedback_collector.go#L575-L594) |
| Serving identity | Ollama's client returns its configured raw model string; it does not itself remove the tag at this accessor. [Source](https://github.com/theRebelliousNerd/codenerd/blob/4cb0b3650de09085351014d522beb440c7e97ae3/internal/perception/client_ollama.go#L128-L132) |
| Context matching | `ModelPinTokens` derives exact/family tokens; `MatchesContext` accepts a matching model token after provider matching. Both numeric names satisfy the same collapsed exact selector. [Tokens](https://github.com/theRebelliousNerd/codenerd/blob/4cb0b3650de09085351014d522beb440c7e97ae3/internal/prompt/pinning.go#L230-L243), [matching](https://github.com/theRebelliousNerd/codenerd/blob/4cb0b3650de09085351014d522beb440c7e97ae3/internal/prompt/atoms.go#L505-L530) |

The previous audit's conditional grouping witness therefore survives with numeric tags: two otherwise qualifying failures from `local-model:1` plus one from `local-model:2`, under the same provider, role and problem and a minimum count of three, predict one qualifying exact-model group rather than separate groups of two and one. This does not guarantee generation: unconsumed-evidence, verdict, minimum-count, shared-cause and budget controls still matter.

A second conditional witness begins with an already legitimately admitted exact pin originating from tag `:1`, then cold-compiles the same role under tag `:2`. The model-identity check cannot distinguish them. This is wrong eligibility, not guaranteed rendered inclusion or a complete-cache collision claim. All other relevant context, tool, dependency, conflict, trial, relevance and token-budget controls remain. Cross-provider matching still has its independent provider guard; unknown-model contexts do not become valid merely because numeric suffix handling is wrong.

## Complete the existing identity contract

Retain the repaired alphanumeric-tag behavior. Add numeric-tag cases (`:0`, `:1`, `:2`) and make transport-revision aliasing provider-aware and explicitly justified instead of inferring it merely from digits. Keep the documented bare-name/`:latest` equivalence only where the backend contract supports it. Exact and intentionally broader family scope must remain different policies.

Go should emit faithful identity facts; Mangle should continue deciding admission from those facts. Adding more rules after the tag has disappeared cannot reconstruct the lost distinction. Nor should an LLM guess whether two names are equivalent.

The original audit's raw-provenance migration, mixed-group review, versioned identity and trial-reapproval obligations remain. This pass has not established that those migrations were implemented or that historical mixed groups were repaired. A stored broad token cannot safely reveal which numeric tag originally produced it. Prefer repinning from retained raw provenance; ambiguous mixed evidence needs explicit review rather than a guessed tag. Do not widen compatibility matching to accept both old broad and new narrow exact tokens, because that restores the collision.

## Full costs and tradeoffs

Preserving tag identity adds small per-identifier processing/storage costs but can have wider system consequences. More distinct exact groups hold fewer observations each, may reach generation thresholds later, and may require separate generation and review work if several groups eventually qualify. Narrower matching may suppress guidance that happened to transfer usefully; evaluate that transfer under deliberate family scope, not accidental exact-scope equivalence.

Provider-aware normalization introduces API migration and compatibility work across every pin producer, feedback grouper, persisted selector reader and compilation-context producer. Versioned persistence adds migration reads/writes, backups, failure handling, resumability and potentially human review of ambiguous provenance. Updating pins can change candidate digests and invalidate trial envelopes, creating legitimate reapproval work. Preserve evidence-consumption idempotence rather than automatically regenerating all historical candidates.

Distinct identities can also increase downstream context/selection variants and reduce reuse where other key components are otherwise equal. Exact cache behavior must be inspected and measured rather than assuming every preserved tag necessarily creates an extra miss. No provider throughput, token saving, monetary benefit or net end-to-end improvement follows from this correction alone. Correct attribution is the immediate objective.

## Fair evaluation and acceptance

The following checks are proposed, not run. Use controlled model identities, real grouping and pin helpers, canonical persistence and the real-kernel cold-compilation path before any separately authorized provider evaluation. No model call is necessary to establish the identity invariant.

| Case | Acceptance condition |
|---|---|
| `local-model:1` versus `local-model:2` under exact scope | Distinct groups and selectors; a tag-1 pin does not pass tag-2 exact matching |
| Numeric `:0` and multi-digit tags | Preserved unless a documented provider-specific alias rule actually applies |
| Same-tag positive | Existing legitimate eligibility retained |
| `qwen3:8b` versus `qwen3:32b` | Existing alphanumeric fix remains effective |
| Justified Bedrock revision aliases | Only intentionally reviewed equivalences retained |
| Bare name/`:latest` | Equivalence governed by the relevant backend contract |
| Different provider or unknown model | Existing fail-closed pin behavior retained |
| Two-plus-one qualifying records at threshold three | No exact-model threshold crossing by pooling different numeric tags |
| Persisted raw provenance, ambiguous legacy provenance and interrupted migration | Correct repinning where justified; no guessed authority, lost consumption state or duplicate generation |
| Explicit family scope | Broader transfer remains an intentional, separately tested choice |

Compare the current implementation against provider-aware normalization with the same corpus, source generations, policy, token budget and controlled eligibility inputs. Measure wrong-admission and wrong-group counts, same-model useful-atom recall, CPU/allocations, fact and storage bytes, migration I/O, cache behavior and review burden. Preserve the current safeguards in both arms.

If subsequent authorized model trials are desired, hold model artifacts, tasks, tools, budgets and verification criteria constant; evaluate verified completion and downstream repair, not just fewer prompts or more atom inclusion. Include failed attempts and maintenance cost. Adoption is justified first by preserving the promised exact scope; any broader performance claim requires separate evidence.
