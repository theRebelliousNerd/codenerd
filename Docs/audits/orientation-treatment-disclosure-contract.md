---
doc-class: deep-dive
subsystem: orient-workspace-world
implementation-status: proposal-with-source-grounded-gap
last-verified: 2026-10-28
verified-against: c8d324dfe60e57d7b4d54414cfdfa5cdee1258bb
verification-method: static-source-inspection
supersedes: []
---

# Finish orientation as a semantic disclosure controller—not another file index

## Decision in one paragraph

The dead-code audit's orientation and research candidates are not missing production entry points at the inspected revision: all seven now live in explicit test-only seams. Do not revive them. The substantive unfinished orientation capability is already specified as **GAP-ORIENT-09 / GAP-WS-05**: distinguish membership from semantic treatment so a repository can retain discoverable names without eagerly reading, parsing, embedding or presenting every body. Complete that capability with a **generation-bound disclosure contract**: the kernel chooses treatment, every consumer declares what evidence it actually acquired, and a task can request a narrowly scoped promotion without silently widening repository access or pretending an incompletely indexed subtree is absent. This is a refinement of CodeNERD's existing design, not a claim of a new field or measured benefit.

This document is source inspection and proposed design only. No tests, builds, inference, training, performance experiments or live repository operations were executed. Scope is the orientation-to-workspace-to-consumer boundary; general capability-cell architecture and CodeDOM editing remain in the [agent-native architecture grand design](https://github.com/theRebelliousNerd/codenerd/blob/codenerd-research-docs/Docs/audits/agent-native-architecture-grand-design.md) and existing CodeDOM documentation. The grand-design link intentionally targets the dedicated research branch, not main.

## 1. What happened to the audit candidates?

At [main c8d324df](https://github.com/theRebelliousNerd/codenerd/commit/c8d324dfe60e57d7b4d54414cfdfa5cdee1258bb):

| Audit candidate | Current evidence | Interpretation |
|---|---|---|
| `orient.Run` | [`internal/orient/test_seams_test.go`](https://github.com/theRebelliousNerd/codenerd/blob/c8d324dfe60e57d7b4d54414cfdfa5cdee1258bb/internal/orient/test_seams_test.go) constructs an engine, asserts supplied facts, evaluates and reads selected outcomes. | Test convenience over the real engine, not a missing command. |
| `orient.Inspect` | Same file builds a one-pass report from measurements; its comment contrasts this with init retaining the engine. | Test-only one-pass view, not evidence orientation is absent. |
| `Report.JSON` | Same file indents report serialization. | Test serializer; the production capability is not defined by this convenience function. |
| `researchPageBrowser`, `shutdownResearchBrowser` | [`internal/tools/research/test_seams_test.go`](https://github.com/theRebelliousNerd/codenerd/blob/c8d324dfe60e57d7b4d54414cfdfa5cdee1258bb/internal/tools/research/test_seams_test.go) explicitly labels these conveniences called only by tests. | Browser inspection/cleanup seams. This placement alone does not prove the production browser's entire lifecycle correct. |
| `GetDocURLsForTech`, `GetDocURLsForTechs` | Same test-only file performs static URL lookup and deduplication. | Test convenience, not a proposed production research policy. |

The existing [orientation TODO](https://github.com/theRebelliousNerd/codenerd/blob/c8d324dfe60e57d7b4d54414cfdfa5cdee1258bb/Docs/architecture/orient/TODO.md) explicitly records Steve's withdrawal of a separate `nerd orient` command: orientation is automatic and inspection uses kernel facts. Its older unchecked boxes are not a reliable current implementation census. Read source and more recent wiring evidence instead of treating every box as work still missing.

## 2. Current boundary: membership is real, treatment is still a proposal

The following evidence is scoped; it does not claim a runtime failure was reproduced.

* [`internal/workspace/membership.go:27–49`](https://github.com/theRebelliousNerd/codenerd/blob/c8d324dfe60e57d7b4d54414cfdfa5cdee1258bb/internal/workspace/membership.go#L27-L49) stores Git membership, directory membership, patterns and freshness witnesses. Its public `Includes`, `IncludesDir`, `Files`, `Gate` and `Admit` paths govern membership. The inspected file does not expose the target `Treatment` accessor or load a semantic treatment artifact.
* [`membership.go:255–335`](https://github.com/theRebelliousNerd/codenerd/blob/c8d324dfe60e57d7b4d54414cfdfa5cdee1258bb/internal/workspace/membership.go#L255-L335) admits tracked/nonignored paths subject to patterns and repository boundaries. This is not a classifier for source versus seed data.
* [`internal/workspace/walk.go`](https://github.com/theRebelliousNerd/codenerd/blob/c8d324dfe60e57d7b4d54414cfdfa5cdee1258bb/internal/workspace/walk.go) batches unknown sibling membership queries, prunes rejected directories, and otherwise enumerates admitted children. This existing pruning must be preserved, not replaced with model-authored ignore lists.
* [`internal/world/fs.go:286–343`](https://github.com/theRebelliousNerd/codenerd/blob/c8d324dfe60e57d7b4d54414cfdfa5cdee1258bb/internal/world/fs.go#L286-L343) calls `mem.Admit`, then hashes admitted files on cache miss. Later, [lines 418–438](https://github.com/theRebelliousNerd/codenerd/blob/c8d324dfe60e57d7b4d54414cfdfa5cdee1258bb/internal/world/fs.go#L418-L438) read Go test headers and eligible non-test files for parsing. Therefore a semantic names-only policy needs a gate **before body hashing**, not merely before embedding or prompt rendering. Not every admitted file is parsed: language, test and size handling already constrain that work.
* [`internal/workspace/agents.md`](https://github.com/theRebelliousNerd/codenerd/blob/c8d324dfe60e57d7b4d54414cfdfa5cdee1258bb/internal/workspace/agents.md) expressly says semantic treatment remains an orientation integration gap and forbids inventing a Go decision policy for it.
* The [orientation discernment specification §2](https://github.com/theRebelliousNerd/codenerd/blob/c8d324dfe60e57d7b4d54414cfdfa5cdee1258bb/Docs/architecture/orient/08-DISCERNMENT-AND-QUESTIONS.md) and [workspace target specification §4](https://github.com/theRebelliousNerd/codenerd/blob/c8d324dfe60e57d7b4d54414cfdfa5cdee1258bb/Docs/architecture/workspace/05-MEMBERSHIP-SPEC.md) already propose `tree_treatment` and `.nerd/orientation/membership.json`. Credit that design. They do not yet settle all task promotion, freshness and negative-evidence contracts below.

Orientation itself is not disconnected: [`internal/system/orientation_refresh_boot.go`](https://github.com/theRebelliousNerd/codenerd/blob/c8d324dfe60e57d7b4d54414cfdfa5cdee1258bb/internal/system/orientation_refresh_boot.go) refreshes in the background and applies a delta through a transaction when available. The package's [guidance](https://github.com/theRebelliousNerd/codenerd/blob/c8d324dfe60e57d7b4d54414cfdfa5cdee1258bb/internal/orient/agents.md) assigns measurement to Go and attention/freshness decisions to Mangle. Extend that split.

## 3. The first-principles problem

A large codebase has at least four distinct questions:

1. **Is this path in the authorized workspace?** Membership and permissions.
2. **What kind of artifact is it?** Evidence-grounded role, possibly ambiguous.
3. **What work should a particular consumer do now?** Census, parse, embed, search or reveal.
4. **What can this consumer truthfully conclude from what it saw?** Coverage and uncertainty.

One Boolean cannot answer all four. A fixture may belong to the repository, be unnecessary for ordinary code search, and still be the decisive evidence for a failing parser test. A generated client may deserve little recurring embedding but remain a real compile dependency. Hiding either from every consumer corrupts the model; treating both as ordinary source wastes attention.

The optimization target is not minimum indexed bytes. It is minimum lifecycle cost **subject to preservation of required evidence and correct uncertainty**. Deferred work is a liability to be tracked, not evidence that the work never matters.

The underlying information-disclosure idea is established: Shneiderman's [1996 paper](https://www.cs.umd.edu/users/ben/papers/Shneiderman1996eyes.pdf) organizes overview, filtering, detail and history as distinct exploration operations. Here the proposed adaptation is to make those operations explicit, revision-qualified machine contracts rather than a human UI convention. The paper does not establish LLM token savings.

## 4. Proposed disclosure algebra

Keep Git membership and authorization as hard outer bounds. Add two separate values for a member path `p` at generation `g`:

- `base_treatment(g,p)`: normal indexing behavior, derived by Mangle from measurements, accepted operator answers and scoped configuration.
- `demand(g,task,p,operation)`: a task-specific request for additional detail, not an instruction to change membership.

An operation is admitted only when all relevant conditions hold:

`admit = member(p) AND authorized(operation,p) AND current(g) AND policy_allows(g,task,p,operation)`.

Metadata is not automatically public: names, directory summaries and ownership labels can expose private dependencies or internal project structure. Authorize listing/metadata separately from body reads, and check destination/egress policy before sending either to a model provider, browser query or external index. A local read permission is not permission to export its result. This refines the existing discernment specification’s private-dependency boundary, not a claim that all current outbound paths were audited here.

Treatment never grants a permission. A denial cannot be overcome by a high relevance score. An inherited `/exclude` must carry whether it is a hard access restriction or an operational indexing omission; only the latter can be considered for policy-governed promotion. If this distinction is absent, refuse the ambiguous promotion and expose its reason.

The existing three treatments remain useful defaults, but distinguish them from evidence completeness:

| Default | Normal behavior | Never imply |
|---|---|---|
| `/index_and_parse` | Run eligible structural indexing and ordinary retrieval admission. | Every language construct, dynamic edge or file was successfully understood. |
| `/index_names_only` | Keep discoverable identities and metadata; no ordinary body reads, hashing, parsing or embedding. | Names constitute complete behavioral evidence. |
| `/exclude` | Skip ordinary traversal under the declared operational scope. | Exclusion authorizes deletion, or the subtree cannot affect a build. |

Unknown classification is a fourth **status**, not a secret fourth treatment. Initially preserve existing eligible processing behavior unless an independent access rule prohibits it, and emit `treatment_unknown`. This favors correctness while exposing its cost. A later budget policy may defer work, but must emit pending coverage obligations instead of calling the result complete. Missing, malformed or stale treatment artifacts must never silently hide repository members.

### Inheritance contract

Use canonical root-relative paths and segment-aware ancestry, not string-prefix containment. A rule for `data/` must not govern `database/`. Resolve applicable rules by explicit scope and authority before specificity. An operator's hard restriction dominates an inferred child promotion; an explicitly approved child override may refine an operational parent policy. Contradictory equally authoritative rows produce a conflict, not map iteration order. Paths escaping the workspace, nested repositories and symlink boundaries retain existing membership semantics.

## 5. Generation-bound receipts and promotion

A proposed artifact envelope contains a schema version, canonical workspace identity, membership generation, orientation measurement digest, policy/configuration digest, operator-answer digest and deterministic treatment rows. `HEAD` alone is insufficient: dirty edits, newly created files, changed ignore rules and operator answers can change decisions without a commit.

Existing [`Membership.Revision`](https://github.com/theRebelliousNerd/codenerd/blob/c8d324dfe60e57d7b4d54414cfdfa5cdee1258bb/internal/workspace/membership.go#L178-L201) is process-local and documents unstamped nested-ignore changes. Do not present it as a durable content identity. Explicit refresh and consumer witnesses remain necessary.

For each attempted consumer operation, record:

`(workspace, generation, task, path, operation, treatment, disposition, evidence_identity, reason)`.

Dispositions distinguish complete, metadata-only, deferred, denied, stale, unsupported and failed. An unavailable body hash is **unknown**, never a sentinel value masquerading as a hash. Add a separate metadata census record rather than forging the existing `file_topology` hash slot.

A proposed promotion protocol:

1. A failed test, unresolved import, user target, structural reference or explicit exploration request supplies a grounded demand.
2. Mangle derives the smallest allowed acquisition obligation: exact file, element or bounded directory page, with reason and current generation.
3. The driver rechecks membership and permission, acquires the requested evidence through existing CodeDOM/read interfaces, and records exact coverage plus actual costs.
4. Before publishing newly derived facts, compare witnesses. A changed generation invalidates publication or narrows it to independently validated current evidence; never attach old results to a new generation.
5. A task-local promotion expires with its task or witness. Durable reclassification requires fresh orientation evidence or operator authority; repeated demand is a signal to reconsider the default, not permission to silently broaden it.

Source becoming names-only must invalidate previously indexed body-derived facts and vector rows or mark them ineligible for current retrieval. Otherwise the new policy changes future reads while stale content remains visible. The replacement transaction should publish treatment and coverage state with the corresponding invalidations. Cross-store failures need an explicit pending/degraded state; do not claim a single kernel transaction atomically updates independent vector stores. Demotion prevents future acquisition and retrieval; it cannot erase bytes already disclosed to a model, provider, transcript, remote embedding service or retained artifact. Track those destinations and retention obligations separately. Where required and supported, retire or rebuild affected contexts and indexes, request authorized remote deletion, and record unverified or unavailable deletion rather than claiming retroactive confidentiality.

## 6. Progressive discovery without a 400,000-name prompt

Names-only is not sufficient if the model receives every name anyway. Provide a deterministic directory synopsis: counts by role/extension, known ownership, membership generation, unresolved regions, and a typed continuation token. A page is bounded by actual serialized bytes and has stable ordering within a generation. The token binds query, scope and generation; reject or restart stale pagination rather than silently skip or duplicate entries.

Prefer exact target and structural-reference resolution over browsing. A directory page must say whether more entries exist and whether its census is complete. A query with zero hits can mean `no_matching_names_in_complete_scope`, `body_not_searched`, or `scope_incomplete`; these are different outputs.

This is an adapter over CodeDOM and workspace identity—not a competing source model. CodeDOM still owns element-level structural views; orientation chooses which depths are acquired for which task. Physical folder boundaries help only when they correspond to useful ownership and traversal partitions; virtual disclosure should work even before a risky folder migration.

## 7. Integration contracts and order

1. **Orientation measurement:** bounded metadata census precedes body classification. If representative body reads are needed, count them explicitly and authorize them; do not spend the entire avoided scan cost classifying the tree. Sampling cannot establish universal absence.
2. **Mangle policy:** derive roles, treatment, conflicts and promotion obligations. Go implements measurements, validation and effects, not an independent role-to-treatment switch.
3. **Workspace:** retain membership APIs for their current meaning; add a typed decision view over a frozen generation. Do not globally redefine `Includes` to hide names-only paths from all existing callers.
4. **World scanner:** enforce treatment before hash/read/parse, preserve metadata-only records and invalidate former body-derived rows on demotion. This is the first source-confirmed insertion point.
5. **Retrieval/embedding:** inspect each actual producer before rollout; all must carry treatment generation and remove or quarantine demoted evidence. Their full migration census is still required, not established by this document.
6. **CodeDOM and impact queries:** expose incomplete scope and targeted hydration. Do not conclude no callers or no tests merely because a subtree was not structurally acquired.
7. **Build/test execution:** analysis suppression must not rewrite compiler inputs, remove fixtures or shrink acceptance gates implicitly. Bazel's [dependency documentation](https://bazel.build/concepts/dependencies) distinguishes actual and declared dependencies and requires declared dependencies to cover actual ones. The analogous caution here is that an incomplete analysis graph cannot certify the absence of a real build dependency.
8. **JIT/articulation:** include concise omission and continuation receipts rather than whole path lists; do not translate partial evidence into an unqualified completion claim.

Roll out shadow decisions first: compare what would be suppressed while retaining existing behavior. Then opt into names-only for explicitly confirmed scopes, with immediate targeted promotion available. Only introduce automatic exclusions after adverse cases and downstream invalidation have been verified. Shadow mode is an evaluation design, not a currently executed result.

## 8. Full-cost model

For a horizon of `H` tasks, compare:

`C = C_census + C_classify + C_policy + C_artifact + C_invalidate + sum_h(C_lookup + C_promote + C_parse + C_embed + C_JIT + C_model + C_verify + C_retry) + C_storage + C_retention_and_rebuild + C_operator`.

Record currencies separately: token counts by role, wall latency, CPU/GPU time, bytes read, peak memory, disk/vector storage, external API cost and human review. Do not collapse all resources to one score without explicit prices. The proposal saves work only if avoided eager acquisition and reduced model context exceed classification, promotion, invalidation and recovery costs **at equal accepted correctness**.

Let `P` be eager body-processing cost for a subtree, `S` its metadata synopsis cost, `K` classification cost and `D_h` promotion cost on task `h`. A necessary local break-even condition is:

`S + K + sum_h D_h + invalidation_overhead < eager_baseline_cost_over_same_horizon`.

No numeric gain is implied. Repeatedly accessed generated code can make deferred acquisition worse. Share immutable census artifacts across workers, but measure contention and cold-start amplification: a single serialized orientation service can become the new bottleneck.

## 9. Failure modes worth designing first

| Failure | Required observable behavior |
|---|---|
| Real implementation hidden inside `fixtures/` or `generated/` | Role conflict or targeted promotion; task cannot claim complete impact evidence without it. |
| Names-only file previously embedded | Current queries refuse stale body-derived rows; invalidation failures are visible. |
| Artifact copied from another workspace | Identity rejection, no automatic treatment installation. |
| Rename or dirty edit during acquisition | Witness mismatch prevents stale publication. |
| Missing metadata-only hash | Unknown digest is explicit; no false equality with earlier bytes. |
| All workers demand the same cold subtree | Coalesced work with bounded waiting and cancellation ownership; account for queue delay. |
| Hostile directory name or embedded instructions | Data stays data; classification cannot grant authority. |
| Parent exclusion plus relevant child | Hard restrictions remain hard; operational conflict is surfaced for a scoped decision. |
| Generated code feeds compilation | Normal build semantics remain intact despite indexing treatment. |
| Incomplete language parser | Coverage remains unsupported/partial; no negative proof of dependency absence. |
| Pagination over changing tree | Generation-bound continuation rejects stale cursors. |
| Policy churn | Explainable changes, controlled recomputation and bounded retained generations. |

## 10. Evaluation that can reject the proposal

Use matched tasks on repositories spanning small workspaces through large monorepos, with source, generated clients, large fixtures, answer keys, embedded assets, vendor trees, ignored paths and mixed-language builds. Include adversarial naming: crucial source under an apparently disposable directory. Separate cold boot, warm repeated tasks, concurrent workers and ongoing source changes.

Compare (A) current membership plus existing scanner/retrieval behavior, (B) manually curated names-only scopes, (C) inferred treatment without promotion, and (D) inferred treatment plus generation-bound promotion. C is a diagnostic ablation, not a recommended unsafe production mode. Give every arm the same model, task, permission boundary, baseline CodeDOM capability, hardware and acceptance criteria. Do not handicap A by dumping the complete file list into its prompt.

Primary outcomes: correctly accepted integrated tasks; relevant-file and dependency-evidence recall against independently established task evidence; false completion rate; forbidden reads; stale-evidence admission; total cost per accepted task; p50/p95 first-use and completion latency. Report failed/abandoned tasks and uncovered regions, not only successful-case averages.

Designed boundary tests include: names-only means zero ordinary body opens including hashing; metadata remains discoverable; required fixture demand promotes only its allowed scope; demotion quarantines prior vectors; stale generation cannot complete a receipt; parent/child conflicts are deterministic; nested ignore refresh invalidates treatment; a compiler dependency hidden from analysis remains an unresolved coverage obligation, not a skipped build input; incomplete page returns cannot support global negative claims.

Reject or revise automatic treatment if evidence recall or accepted correctness degrades, if unknown classification silently becomes exclusion, or if invalidation/promotion cost outweighs avoided work over the maintenance horizon. A smaller prompt by itself is not a win.

## 11. Recommended next commitment

Do not reopen the seven test seams. Finish one narrow existing gap: **an artifact-backed, generation-aware treatment decision used before the world scanner reads file bodies, with explicit metadata-only coverage and a targeted promotion contract**. Establish correctness there before migrating every consumer. This turns Steve's progressive-disclosure principle into an enforceable subsystem contract without rewriting repository layout, duplicating CodeDOM or weakening verification.

## As built (2026-10-28)

The shadow slice of §7 (steps 1–4, shadow only) is implemented. Hashing, parsing and every membership answer are unchanged. Nothing is enforced yet.

- **Measurement** (`internal/orient/treatment.go`). The census iterates `Membership.Files()`, the frozen member list, so it covers exactly the set the generation digest names. It records `tree_dir`, `tree_parent` (one row per path segment), `tree_depth`, `tree_name`, `tree_stats(Dir, Files, Bytes)` and `tree_ext` for each member directory down to `census_max_depth`, plus every overridden member directory and its ancestors. Generated-code markers come from evenly spaced header samples. Shallow directories are sampled first, against a global `header_read_budget` that is counted and reported along with the number of directories it left unsampled. No other body is read.
- **Policy** (`internal/orient/treatment.mg`). Inferred roles are `/vendored` and `/fixtures` (from exact directory names), `/generated` (from the header share), `/seed_data` (from the data-extension share plus a byte floor) and `/source` (from the source-extension share). All inferred roles map to `/index_names_only`. Inference never derives `/exclude`, and on its own it never derives a `/index_and_parse` row. Source evidence contests only the data-like roles (fixtures, seed data) on the same directory, which makes a conflict there. Resolution is authority first: `/access_restriction` (hard; the strictest restriction holds), then `/operator` (operational), then `/inferred`. Among the highest authority present, the nearest scope wins. Two disagreeing claims on the same scope produce `tree_treatment_conflict` for that scope and for everything that inherits from it, with no row. An unclassified directory has no row and keeps current behavior. An override path that is not a member directory is reported as `treatment_override_unmatched`.
- **Config** (`orient.treatment`, `internal/config/orient_treatment.go`). This holds the census bounds, the thresholds (as `config_param` rows required by the policy) and the overrides `{path, treatment, basis}`. The basis is `access_restriction` or `operational`. An access restriction cannot grant `index_and_parse`. Paths must be canonical, and a path may appear in only one override. `Check` refuses anything else through `orient.Check`, `NewEngine` and `LoadOrientConfig`.
- **Artifact** (`.nerd/orientation/membership.json`). `orient.Refresh` brings it current on every call, including cached no-stale refreshes. It holds the schema version, canonical workspace root, member generation (`Membership.Generation`: SHA-256 over the sorted member list), measurement digest, policy digest (rules plus effective section), census cost, sorted rows with reasons, scope and authority, conflicts, and unmatched overrides. The encoding is deterministic and has no timestamps. When the generation and policy digest have not changed, the cost is one list digest and one file read. Treatment reads the live `orient` section, so an override applies at the next refresh.
- **Workspace** (`internal/workspace/treatment.go`). This file owns the artifact schema and `Membership.TreatmentView()`. The view is a frozen, read-only lookup that resolves a path to the nearest census directory segment by segment. It carries `Conflict` and `Stale`, and it refuses an artifact written for another root or schema (`ErrTreatmentIdentity`). A missing artifact gives a nil view. `Includes`, `IncludesDir`, `Admit`, `Walk` and `Files` are unchanged.
- **World** (`internal/world/treatment_shadow.go`). The full scan (`prepareDirectoryScan`) looks up each admitted file. For names-only and excluded scopes it records, per governing directory, the admitted files, their bytes, the content hashes computed and the body reads. It also counts files under conflicted scopes. The scan logs one summary line, exposes the summary as `ScanResult.TreatmentShadow`, and `nerd scan` prints it. The incremental delta path is not shadowed in this slice.
- **Measured on this repository** (default bounds): 5,858 members and 387 census directories. The census and policy took about 0.13 s in total and used 1,117 of the 2,048 header reads, leaving no directory unsampled. Every `testdata` directory and `third_party` came out names-only. `internal/world/testdata` came out a conflict, because it holds Go source under a fixtures name.

Not yet built: enforcement before hashing, metadata-only coverage records, invalidating body-derived rows on demotion, task-scoped promotion, the kernel projection of `tree_treatment`, and shadowing of the incremental delta scan.

## Status note (2026-10-28)

Fixed: operator-confirmed names-only/exclude scopes are now enforced before any hash, read or parse in the full and incremental scans, with `file_metadata_only`/`treatment_scope` coverage rows, demotion retiring prior rows, and stale/inferred/conflicted scopes left shadow (`internal/world/treatment_enforce.go`).
Ruled out: the seven test seams (not missing wiring); inferred treatment is never enforced (audit's own rollout order).
Left: structure index, deep scan, retrieval/embedding still read enforced files; task-scoped promotion; kernel projection of `tree_treatment`; modified-size drift of a names-only file is not refreshed on an unchanged scan.
