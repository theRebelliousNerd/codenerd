---
doc-class: deep-dive
subsystem: knowledge-acquisition
implementation-status: proposed
last-verified: 2026-10-30
verified-against: 388205138ba6d47c1cff0c5e97c896dfa642a391
verification-method: static-source-inspection
---

# Preserve Go major version module pins through knowledge acquisition

**Decision:** distinguish a technology family used for expert routing from the exact module path and version used for knowledge acquisition. A dependency recognized as Echo must retain its actual `github.com/labstack/echo/v4` identity when documentation, repository usage and refresh decisions depend on that identity.

**Finding:** current source can recognize a valid major-version Go dependency and select its expert, but then construct only a generic research topic for that dependency. The exact module and version disappear before the acquisition sources run. Consequently Go documentation and repository-usage acquisition report the subject inapplicable, and an upgrade of that dependency cannot trigger version-change refresh for the generic topic. Generic web research, Context7 and other advice can still work.

**Evidence status:** read-only inspection of public CodeNERD main `388205138ba6d47c1cff0c5e97c896dfa642a391`, reconfirmed as main on 2026-10-30 at approximately 08:45 UTC. The witness is a conditional source prediction, not an observed production incident or executed regression. No application execution, tests, dependency download, model inference, provider request or benchmark was performed. This document proposes changes and evaluation; it implements neither.

## Scope and prior ownership

The knowledge acquisition contract already distinguishes a research topic from a pinned package and carries its ecosystem and version. The version is explicitly intended to let refresh policy detect older acquisitions. The defect is the integration between technology classification and that existing contract, not a proposal to make all expert knowledge version-specific. [Subject contract](https://github.com/theRebelliousNerd/codenerd/blob/388205138ba6d47c1cff0c5e97c896dfa642a391/internal/knowledge/acquire.go#L26-L58)

Root guidance assigns orchestration and context decisions to Mangle, requires stale evidence not to persist as current truth, and asks for validation through real production paths. The repair should supply faithful dependency observations to that executive policy, not add a parallel model-selected research path or force every expert row into the prompt. [Repository guidance](https://github.com/theRebelliousNerd/codenerd/blob/388205138ba6d47c1cff0c5e97c896dfa642a391/agents.md)

The nearby [knowledge acquisition subject-ownership audit](https://github.com/theRebelliousNerd/codenerd/blob/77069960b69aba14a67dbd682d6f65435e8ada50/Docs/audits/knowledge-acquisition-subject-ownership.md) owns a different boundary: distinct, valid subjects acquire colliding slug-based concept labels, and replacement deletes another subject's rows. This report loses the exact module identity **before acquisition**. It needs no slug collision, deletion, concurrent writer or pruning.

The existing [refresh and prune audit](https://github.com/theRebelliousNerd/codenerd/blob/388205138ba6d47c1cff0c5e97c896dfa642a391/Docs/audits/knowledge-refresh-prune.md) owns generation-fenced retirement; the [knowledge miss audit](https://github.com/theRebelliousNerd/codenerd/blob/388205138ba6d47c1cff0c5e97c896dfa642a391/Docs/audits/knowledge-miss-retirement.md) owns genuine gap admission and recovery. The expert-deck proposal concerns plan-bound advice, not Go module identity. None supplies the missing family-to-versioned-module mapping here.

Deduplication included the relevant main audits and innovations, searches for pinned-package and major-version knowledge handling, and the added-file inventories of public research branches 0715, 0747 and 0817. No exact owner was found in that bounded review. It is not an exhaustive historical novelty claim.

## What works before the boundary

A major-version suffix is part of a Go module's identity. Go's documented convention uses distinct module paths such as `example.com/mod/v2` for incompatible major versions, rather than treating the suffix as decorative text. A technology classifier may group those paths into a family, but documentation and dependency evidence must retain the selected path. [Go module reference](https://go.dev/ref/mod#major-version-suffixes)

CodeNERD's manifest parser does preserve that information. For each `go.mod` requirement, it records `req.Mod.Path`, `req.Mod.Version` and the manifest path. A valid Echo v4 requirement therefore arrives as `github.com/labstack/echo/v4`, not an already-stripped base name. [Manifest pin structure and parser](https://github.com/theRebelliousNerd/codenerd/blob/388205138ba6d47c1cff0c5e97c896dfa642a391/internal/init/manifest_pins.go#L14-L24), [Go parsing](https://github.com/theRebelliousNerd/codenerd/blob/388205138ba6d47c1cff0c5e97c896dfa642a391/internal/init/manifest_pins.go#L80-L90)

The technology vocabulary contains `github.com/labstack/echo → echo`, along with other unversioned family roots such as Fiber, JWT and Redis. The dependency scanner checks for the vocabulary path within the manifest text and records the family key. Thus an Echo v4 path can be recognized as Echo. The catalog has an Echo-triggered WebAPIExpert entry, although the clean witness below only requires an already-ready agent with an explicit `echo` domain; it does not assume every recommendation is accepted. [Vocabulary](https://github.com/theRebelliousNerd/codenerd/blob/388205138ba6d47c1cff0c5e97c896dfa642a391/internal/init/technology_vocabulary.go#L34-L69), [scanner](https://github.com/theRebelliousNerd/codenerd/blob/388205138ba6d47c1cff0c5e97c896dfa642a391/internal/init/scanner.go#L182-L209), [catalog](https://github.com/theRebelliousNerd/codenerd/blob/388205138ba6d47c1cff0c5e97c896dfa642a391/internal/orient/ecosystem_agents.mg#L226-L236)

Expert routing also has a broader family relation. `TechnologyImportRoots` emits each Go vocabulary path and a slash-delimited prefix. Boot publishes those facts and recorded agent domains. Policy joins a task file's actual import against that prefix and derives `expert_needed` for a matching ready agent. `github.com/labstack/echo/v4` starts with the Echo family's slash-delimited prefix. The finding therefore does **not** allege that all expert selection fails for versioned modules. [Import roots](https://github.com/theRebelliousNerd/codenerd/blob/388205138ba6d47c1cff0c5e97c896dfa642a391/internal/init/technology_vocabulary.go#L209-L237), [boot observations](https://github.com/theRebelliousNerd/codenerd/blob/388205138ba6d47c1cff0c5e97c896dfa642a391/internal/system/shard_expert_facts.go#L70-L82), [registry domains](https://github.com/theRebelliousNerd/codenerd/blob/388205138ba6d47c1cff0c5e97c896dfa642a391/internal/system/shard_expert_facts.go#L115-L139), [routing policy](https://github.com/theRebelliousNerd/codenerd/blob/388205138ba6d47c1cff0c5e97c896dfa642a391/internal/core/defaults/policy/shard_experts.mg#L42-L55), [expert admission](https://github.com/theRebelliousNerd/codenerd/blob/388205138ba6d47c1cff0c5e97c896dfa642a391/internal/core/defaults/policy/shard_experts.mg#L89-L96)

## The missing relation is not a reversed prefix bug

`KnowledgeSubjects` asks `pinnedPackages` for the packages belonging to each domain. That helper compares each vocabulary entry `pkg` with manifest pins using:

```go
pin.Module == pkg ||
    (eco == EcosystemGo && strings.HasPrefix(pkg, pin.Module+"/"))
```

These are two legitimate cases:

- The vocabulary names the required module exactly.
- The vocabulary names a package inside a required parent module; that package receives the parent's version.

The second test has the correct direction for its stated package-inside-module purpose. It must not simply be reversed or removed. What is missing is a **third relation**: a vocabulary family root can correspond to an actual manifest module with a Go semantic major-version suffix. [Existing matching contract](https://github.com/theRebelliousNerd/codenerd/blob/388205138ba6d47c1cff0c5e97c896dfa642a391/internal/init/knowledge_sources.go#L77-L105)

For `pkg = github.com/labstack/echo` and `pin.Module = github.com/labstack/echo/v4`, neither existing condition holds. No other vocabulary entry in the inspected Echo family supplies the exact versioned path.

With no pinned match, `KnowledgeSubjects` checks whether the domain is a concept, then falls back to a plain topic. Echo is not one of the concept domains, so the resulting subject is equivalent to:

```text
Agent       = the ready agent's key
Topic       = "echo"
Technology  = "echo"
Module      = ""
Ecosystem   = ""
Version     = ""
Key()       = "echo"
```

If an identical generic topic was already admitted, topic deduplication can avoid another copy, but it still does not create the missing pinned subject. The witness does not depend on duplicate topics. [Fallback](https://github.com/theRebelliousNerd/codenerd/blob/388205138ba6d47c1cff0c5e97c896dfa642a391/internal/init/knowledge_sources.go#L45-L74), [concept vocabulary](https://github.com/theRebelliousNerd/codenerd/blob/388205138ba6d47c1cff0c5e97c896dfa642a391/internal/init/concept_vocabulary.go), [subject key](https://github.com/theRebelliousNerd/codenerd/blob/388205138ba6d47c1cff0c5e97c896dfa642a391/internal/knowledge/acquire.go#L52-L58)

Merely adding a broad reverse-prefix match while continuing to emit `Module: pkg` would also be incomplete: it could recognize v4 but send the unversioned base to documentation and refresh. The actual `/v4` identity must survive construction.

## The real consumers lose two sources and one refresh trigger

### Acquisition and source provenance

Init builds subjects from manifest pins and the agent's domains/topics, then calls `knowledge.Fill` with its configured acquirer. `NewAcquirer` contains the real web chain, Context7, GoDoc and repository sources. This shared subject builder is not only a test helper. [Init call](https://github.com/theRebelliousNerd/codenerd/blob/388205138ba6d47c1cff0c5e97c896dfa642a391/internal/init/agents_knowledge.go#L130-L149), [configured acquirer](https://github.com/theRebelliousNerd/codenerd/blob/388205138ba6d47c1cff0c5e97c896dfa642a391/internal/init/knowledge_sources.go#L128-L149), [source set](https://github.com/theRebelliousNerd/codenerd/blob/388205138ba6d47c1cff0c5e97c896dfa642a391/internal/knowledge/wiring.go#L79-L102)

For the plain `echo` subject:

- GoDoc returns `ErrInapplicable` because no Go module is present, before invoking either `go list` or `go doc`.
- The repository source returns `ErrInapplicable` before querying importers because its module is empty.
- Web receives a question about the topic, without the module disambiguator it adds for pinned subjects.
- Context7 receives the topic rather than the exact module path.

The first two outcomes are mechanically established. The last two are weaker inputs, not proof that their eventual answers are wrong. [GoDoc](https://github.com/theRebelliousNerd/codenerd/blob/388205138ba6d47c1cff0c5e97c896dfa642a391/internal/knowledge/sources.go#L184-L206), [repository applicability](https://github.com/theRebelliousNerd/codenerd/blob/388205138ba6d47c1cff0c5e97c896dfa642a391/internal/knowledge/sources.go#L271-L283), [web question](https://github.com/theRebelliousNerd/codenerd/blob/388205138ba6d47c1cff0c5e97c896dfa642a391/internal/knowledge/sources.go#L80-L88), [Context7 input](https://github.com/theRebelliousNerd/codenerd/blob/388205138ba6d47c1cff0c5e97c896dfa642a391/internal/knowledge/sources.go#L124-L145)

### Version change evidence

Every outcome copies `subject.Version`, including successful web and Context7 outcomes. The acquisition ledger therefore records an empty version for the generic topic. Measurement only produces `knowledge_acquired_version` when the outcome is acquired and its recorded version is nonempty. [Outcome construction](https://github.com/theRebelliousNerd/codenerd/blob/388205138ba6d47c1cff0c5e97c896dfa642a391/internal/knowledge/acquire.go#L160-L188), [ledger write](https://github.com/theRebelliousNerd/codenerd/blob/388205138ba6d47c1cff0c5e97c896dfa642a391/internal/knowledge/acquire.go#L305-L318), [measurement](https://github.com/theRebelliousNerd/codenerd/blob/388205138ba6d47c1cff0c5e97c896dfa642a391/internal/knowledge/measure.go#L42-L55)

The background refresher reads current manifest pins, asserts `dependency_pin` under the exact module path, and reconstructs wanted subjects through the same helper. Policy derives `/version_changed` by joining the acquired subject/version to that dependency path and rejecting the old version. The generic `echo` subject has neither a recorded version nor the matching module key. [Refresh observation](https://github.com/theRebelliousNerd/codenerd/blob/388205138ba6d47c1cff0c5e97c896dfa642a391/internal/system/knowledge_refresh.go#L101-L129), [version rule](https://github.com/theRebelliousNerd/codenerd/blob/388205138ba6d47c1cff0c5e97c896dfa642a391/internal/core/defaults/policy/knowledge_refresh.mg#L64-L82)

The factory registers this refresh step, boot constructs it from the session's actual sources, and selected work calls `knowledge.Refresh`; explicit refresh uses the same run method. This is a missing input to live policy, not dormant policy code. [Factory registration](https://github.com/theRebelliousNerd/codenerd/blob/388205138ba6d47c1cff0c5e97c896dfa642a391/internal/system/factory.go#L2919-L2922), [boot](https://github.com/theRebelliousNerd/codenerd/blob/388205138ba6d47c1cff0c5e97c896dfa642a391/internal/system/knowledge_refresh_boot.go#L38-L58), [execution](https://github.com/theRebelliousNerd/codenerd/blob/388205138ba6d47c1cff0c5e97c896dfa642a391/internal/system/knowledge_refresh.go#L145-L169), [explicit entry](https://github.com/theRebelliousNerd/codenerd/blob/388205138ba6d47c1cff0c5e97c896dfa642a391/internal/system/knowledge_refresh.go#L200-L207)

### Actual context delivery

The JIT compiler searches the speaking expert's registered KB and pushed experts' KBs, converts returned rows to prompt atoms, and leaves them subject to normal selection and budget. Missing GoDoc/repository acquisitions cannot contribute their rows through that path. Other existing rows remain available, and the absence of one source is not proof of complete context failure. [Collection and conversion](https://github.com/theRebelliousNerd/codenerd/blob/388205138ba6d47c1cff0c5e97c896dfa642a391/internal/prompt/compiler_db.go#L324-L386), [pushed-expert retrieval](https://github.com/theRebelliousNerd/codenerd/blob/388205138ba6d47c1cff0c5e97c896dfa642a391/internal/prompt/compiler_experts.go#L132-L171)

## A bounded witness and meaningful controls

A clean fixture needs one readable root `go.mod` requiring a valid `github.com/labstack/echo/v4` release; a ready registered agent with domain `echo`; enabled acquisition; and no unversioned Echo requirement that independently matches the vocabulary. No malformed manifest, unavailable Go toolchain or failed source call is necessary to explain the missing subject.

First initialize that agent's knowledge through the real subject builder. Then change only the required v4 release within the same major path while successful generic research is still fresh. The current source predicts:

1. No subject whose `Module` is the actual `/v4` path.
2. GoDoc and repository outcomes classified inapplicable for generic `echo`.
3. No acquired-version fact for that generic subject.
4. No version-change refresh for it after the pin change.

For a no-refresh control, assume generic web/Context7 acquired successfully and remain fresh, with no genuine gap, failed-source retry or newly available source. The claim is that the dependency change itself supplies no refresh reason, not that every possible reason is disabled.

Existing safeguards must remain:

- Exact unversioned matches work; the current subject fixture covers Cobra.
- Standard-library packages intentionally have no manifest version.
- Package-inside-parent matching is a valid existing branch.
- Inapplicable is distinct from unavailable or failed, and outcomes are recorded rather than silently dropped.
- Refresh budgets, priorities and wanted-subject checks still govern any newly admitted work.
- Web or Context7 may already supply adequate advice; later age-based refresh can research the generic topic again.
- The current prune-generation fence remains valid and unrelated. This witness requires no pruning.
- Correctly constructed pinned subjects still depend on source success, useful content and JIT relevance; preserving the path does not guarantee every answer enters a prompt.

The inspected `TestKnowledgeSubjects_PinnedModulesAndTopics` covers an unversioned Cobra module, SQL, React, an unpinned topic and malformed manifest reporting. It does not exercise a vocabulary family with a `/vN` manifest path. Tests were read, not run. [Existing fixture](https://github.com/theRebelliousNerd/codenerd/blob/388205138ba6d47c1cff0c5e97c896dfa642a391/internal/init/knowledge_sources_test.go#L15-L67)

## Repair contract

Update the owning specification before implementation:

> Classification may group supported module paths into a technology family. Acquisition and invalidation preserve the actual admitted dependency path and version. Grouping never substitutes an unversioned family label for a versioned module identity.

Keep the current exact-match and package-inside-parent relationships. Add reviewed Go semantic-major matching as a separate relationship. Prefer the Go module ecosystem's parsing rules over ad hoc substring deletion. Treat unsupported or ambiguous paths explicitly; do not broaden a family root into permission to accept every similarly named repository or arbitrary descendant module.

When this new relation matches, the acquired subject must keep the actual path, such as `github.com/labstack/echo/v4`, with its observed version and ecosystem. The technology remains `echo`. If v1 and v4 are both required, keep distinct exact module subjects rather than selecting the first matching family member or folding them into one generic acquisition.

Preserving the actual module also lets the existing world-import helper restrict package/importer discovery to that path and its descendants. Passing an unversioned family root instead would broaden the query across major versions and could read the wrong module in GoDoc. [Existing path-scoped import helper](https://github.com/theRebelliousNerd/codenerd/blob/388205138ba6d47c1cff0c5e97c896dfa642a391/internal/knowledge/wiring.go#L110-L169)

Do not change GoDoc's inapplicable guard to execute a prose topic as a package, invent a version for generic research, or put source execution decisions into model prose. Mangle continues to decide research admission, freshness, priority and budgets. Host code classifies and preserves measured identity and executes permitted source operations.

This narrow repair does not certify all dependency resolution cases. Multiple manifests, workspace selection, replacements, local module changes and effective versions selected by Go are separate scope questions. Preserve their raw provenance and test explicit behavior rather than claiming a suffix fix makes all version evidence exact.

## Migration and compatibility

Existing `echo` acquisitions may be legitimate general technology knowledge. Do not relabel them as `echo/v4`, assign the current pin retroactively, or delete them merely because a new exact subject appears. Their saved provenance did not establish that module version.

Instead, newly recognized exact subjects enter the existing acquisition lifecycle under their own keys. On an unlearned subject, policy can schedule fresh work within the normal budget; historical generic rows can remain general guidance until an explicit retention rule disposes of them. Record enough migration state to distinguish new pinned coverage from retained generic coverage and to explain any extra work.

Avoid treating a newly recognized family as authority to reacquire every historical topic at once. Plan for bounded backfill and interrupted progress. Failed acquisition must remain failed or unavailable without inventing pinned knowledge; cancellation should not mark the new subject acquired.

Changing ownership keys, concepts or vectors can interact with the separately identified subject-ownership defect. An implementation must not retire legacy generic rows through an unscoped concept deletion merely to tidy migration. If the repairs are delivered together, evaluate the combination; neither audit grants the other an automatic correctness result.

Compatibility also includes curated agents and topics. Keep existing user-authored generic questions and domain assignments, and make path matching observable without rewriting agent definitions. Rollback should not reinterpret newly exact records as generic ones or erase their provenance.

## Evaluation and full costs

All evaluation below is proposed and unrun. Begin with deterministic integration, not live-model grading.

Use the pinned main implementation, including its existing pruning and miss-retirement protections, as baseline. Hold manifests, roster, source payloads, clocks, policy budgets and storage settings fixed. Compare the proposed identity-preserving mapping against that baseline, not against an artificially disabled knowledge system.

The primary fixture should call the real manifest parser and `KnowledgeSubjects`, construct the real `NewAcquirer`, and substitute deterministic external I/O only. A Go command stub should assert that GoDoc requests the exact `/v4` module and an imported package under it; the repository source should read actual fixture import facts. Do not bypass the defect by constructing a corrected `Subject` in the test. Check the recorded subject, version, outcome and row provenance, then exercise the actual Cortex refresh entry after a controlled pin upgrade.

Required controls:

- Exact unversioned module match and standard-library handling.
- Existing package-inside-parent behavior, with no reversed-prefix regression.
- A supported `/v2` or `/v4` path, and two simultaneous majors in one manifest.
- Similarly named unrelated modules, malformed major suffixes and ordinary nested modules that are not semantic-major variants.
- npm, plain topics and concept domains unchanged.
- Matching import paths and absent-import fallback, with no cross-major importer leakage.
- A fresh successful acquisition followed by a patch/minor pin update; unchanged pin as the no-work control.
- Source unavailable, failed or empty; budget displacement; cancellation; reopen and repeated refresh.
- Legacy generic acquisitions alongside new exact subjects; interrupted and repeated migration.
- The current generation-fenced prune control and subject-ownership collision control if shared acquisition code changes.

Primary acceptance is identity continuity: the correct module reaches the producer, its materialization and receipt retain provenance, an admitted version change causes the correct refresh, and irrelevant or unchanged pins do not. Positive context-delivery checks should use the ordinary expert-selection and JIT path with distinctive source content, relevant queries and sufficient budget. Negative cases should continue to omit irrelevant or unavailable guidance. A stored-row count alone is insufficient evidence of useful delivery.

Measure module classification work, database reads and writes, number of subjects, source calls, Go process time, Context7/web requests, embeddings, vectors, disk growth, refresh latency, cache invalidation, migration effort, retries and human review. More accurate coverage can legitimately increase cost because local documentation and version-triggered research previously never ran. Distinct major-version subjects can also increase storage and candidate competition. No token savings or efficiency gain is established.

A later authorized quality study should use paired tasks that depend on the pinned API, with fixed model/provider, tools, budgets and independent acceptance criteria. Include unchanged-dependency and unrelated-task controls. Report task correctness, stale-API mistakes, source availability, total provider/tool/test cost and latency, including failed and abandoned attempts. Preserving a path is a deterministic correctness improvement at this interface; downstream coding gains remain an empirical question.

## Acceptance boundary

The repair is complete when a recognized versioned Go dependency remains an exact dependency through subject construction, acquisition, persistence, refresh admission and eligible expert context delivery. Family-level routing may stay broad; version-sensitive evidence must not become generic merely because the vocabulary uses the family root.

