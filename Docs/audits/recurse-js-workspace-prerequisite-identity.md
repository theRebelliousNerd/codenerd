# Recurse JavaScript workspace prerequisite identity

**Status: source-grounded portability finding and proposed contract; no runtime validation.** Inspected public `theRebelliousNerd/codenerd` main at **`b4ee29d064ed91e65c484ed29b335128490aa7f8`**, observed 2026-11-03 at 08:07 UTC. No tests, builds, package installs, application execution, model inference, gate runs, settings or credential changes were performed. Existing tests were read only.

## Decision in brief

Recurse currently treats every JavaScript/TypeScript **bare module specifier as outside the workspace dependency order**. That is too strong for ordinary npm workspaces: a package name can designate a sibling local package. Both packages' source directories can be discovered while their dependency edge is silently absent.

This matters before ideation begins. A focused sweep follows only recorded edges, so it can exclude the prerequisite package completely; a full sweep can visit its consumer first. If a workspace gate subsequently reports a path-owned finding in the excluded prerequisite, the focused finding filter can classify it as outside the selected scope. The mission's proposer and requirements interrogator receive the already selected attempt and cannot make that missing prerequisite an admitted repair target merely by noticing it. These are static consequences of the cited control flow, not observed executions. [Scanner][scanner] [Focus closure][focus] [Finding filtering][filter-findings] [Mission handoff][mission-handoff]

**Recommended bounded next step:** specify a local-workspace package-resolution contract that distinguishes an external package, a verified local package dependency, and an unresolved dependency. Keep the existing Mangle scheduler, membership rules, write guards, gate checks and ratchet. Do not patch this by handing the model authority to expand write scope or by turning all bare imports into local edges.

The finding is **a concrete adapter capability gap**, not a claim that Mangle orders a supplied DAG incorrectly, that ordinary relative imports are broken, or that all JavaScript tests falsely pass.

The current mission architecture explicitly keeps the proposer and interrogator inside the admitted work item and distinguishes measured gate progress from strategic mission satisfaction. This proposal preserves that contract: improve the host's prerequisite observations rather than asking a more persuasive model to override the scope. The implementation trace below independently checks the relevant handoff and write boundary. [Mission specification][mission-spec]

## 1. Scope and distinctness

### The exact claim

For an admitted JS/TS source file containing a package-name import such as `require("@audit/zz-core")`, the current `jsSpecifier` expression does not capture the import at all. It requires the captured string to begin with `./` or `../`. The graph discovery path has no alternate npm-workspace package-name resolver: its source collector groups Cargo manifests, Python source and JS/TS source, and `scanJSImports` receives source files only. [Scanner][scanner] [Collection][collection] [Discovery dispatch][dispatch]

This remains true when all of the following hold:

- The root explicitly declares npm workspaces.
- The imported sibling has a unique matching package name and a conventional local entry point.
- Both importing and imported source directories pass the existing workspace admission rules.
- The package manager's ordinary local linking and resolution are valid.
- No dynamic import expression, generated file, remote service, unusual build tool or complex TypeScript alias is involved.

The gap is therefore not limited to intentionally unsupported arbitrary dynamic dependencies. npm documents package-name consumption of defined workspaces, and TypeScript explicitly recommends package-manager workspaces for sibling packages rather than simulating them with path aliases. A bare spelling alone cannot establish that the dependency is external. [npm workspaces][npm] [TypeScript module resolution][typescript]

### What this does not duplicate

The inspected current audit corpus already contains:

- **`portable-recurse-deliberation-crucible`**, which calls for capability-aware portability and warns that graph discovery is incomplete. It does not trace this package-name witness through focus, ordering and mission handoff. This audit supplies one concrete closure criterion for that proposal, rather than presenting the broad capability-manifest idea as new. [Existing portability proposal][portable]
- **`recurse-go-workspace-acquisition`**, which concerns missing Go node acquisition at a `go.work` root. Here the two JS nodes exist; their local dependency relation is lost. [Go acquisition audit][go-work]
- **`recurse-cargo-inherited-dependency-closure`** and **`recurse-python-from-import-prerequisites`**, which concern different producers and different import semantics. Their implementation fixes or remaining gaps do not add JS package-name resolution. [Cargo audit][cargo-audit] [Python audit][python-audit]
- **`recurse-gate-language-domain`**, which concerns per-path language ownership in collapsed components, and **`recurse-node-namespace-identity`**, which concerns node-ID collisions. Neither collision nor mixed-language SCC is required here. [Language audit][language-audit] [Namespace audit][namespace-audit]

The main audit queue and the available recurse/deliberation audit files were inspected at this pin. No dedicated npm-workspace bare-import audit was found in that set. This is a bounded non-duplication claim, not a claim to have read every historical conversation or every repository document. [Queue][queue]

## 2. Implemented behavior and the missing contract

| Boundary | Implemented at the pinned revision | What it does not establish |
|---|---|---|
| Repository guidance | Creativity belongs to the model; deterministic planning and safety belong to the executive. Foreign-repository work is supposed to move source and specifications toward the target behavior. | A model discovering an omitted dependency is not a substitute for host-owned dependency admission. [Root guidance][guidance] |
| Workspace acquisition | Canonical root, refreshed membership and project forbidden-path exclusions are applied before source collection. | Admitted source directories are not yet resolved package identities. [Discovery][dispatch] [Collection][collection] |
| JS/TS nodes | Every admitted JS/TS source directory becomes a graph node. | A directory is not necessarily an npm package root or a package's public entry point. [Scanner][scanner] |
| JS/TS edges | Relative `from`, `require`, dynamic-import and side-effect-import forms are scanned; resolvable relative paths produce edges. | Bare names are not classified as local versus external. They are excluded before resolution. [Scanner][scanner] |
| Relative resolution | Exact files, extension addition/substitution and index files are considered. The code explicitly handles a `.js` spelling for TypeScript source. | This helper does not inspect workspaces, package names, `main`, `exports`, conditions or package-manager resolution. [Relative resolver][relative] |
| SCCs and ordering | Recorded cycles collapse; `DependsOn` becomes `subsystem_depends`; the kernel visits prerequisites before consumers and breaks ties by lexical identity. | Cycle detection and topological scheduling cannot recover an edge that was never observed. [SCC projection][scc] [Fact projection][facts] [Ordering policy][ordering] |
| Focus | `FilterDAG` recursively includes the selected node and its recorded prerequisites. Both planning and the run use it. | Missing edges become missing focused prerequisites, without a resolution-completeness qualifier. [Focus][focus] [Plan][planning] [Run][pass] |
| Gates | Root package scripts can produce workspace build, test and lint gates. JS/TS workspace commands are not rewritten by the Go-only focused scoping helper. | Running a broad gate does not repair the dependency graph used to select work. [Gate detection][js-gates] [Gate scoping][gate-scope] |
| Mission deliberation | The selected node/finding/angle is passed to the design-only thread; the mission receipt copies the node paths as its scope. | The proposer/challenger cannot authorize a new sibling package scope. [Mission preparation][mission-handoff] [Write guard][write-guard] |

The source and its existing JS test both describe relative-import support. That is useful honesty at the helper level. The operational issue is that the same partial graph feeds a general “dependencies are pulled in automatically” focus contract without carrying the local-package resolution limitation. [Config contract][planning] [Existing JS test][js-test]

## 3. Minimal source-derived witness, not an executed reproduction

Consider this disposable **hypothetical fixture**. These files were not created or run during this audit.

```text
package.json
packages/aa-app/package.json
packages/aa-app/index.cjs
packages/zz-core/package.json
packages/zz-core/index.cjs
```

Relevant contents:

```json
// package.json
{
  "name": "workspace-import-audit",
  "private": true,
  "workspaces": ["packages/*"]
}

// packages/aa-app/package.json
{
  "name": "@audit/aa-app",
  "version": "1.0.0",
  "main": "index.cjs",
  "dependencies": {"@audit/zz-core": "1.0.0"}
}

// packages/zz-core/package.json
{
  "name": "@audit/zz-core",
  "version": "1.0.0",
  "main": "index.cjs"
}
```

The comments above identify separate JSON files and are not proposed literal JSON content.

```javascript
// packages/aa-app/index.cjs
const core = require("@audit/zz-core");
module.exports = () => core.value;

// packages/zz-core/index.cjs
module.exports = { value: 1 };
```

Stipulate an otherwise normal, authorized workspace in which both source directories are admitted and the configured package-manager installation resolves the declared sibling name locally. npm's workspace contract supports this situation. The graph witness itself does not require executing `npm install` or running the exported function. [npm workspaces][npm]

### 3.1 Acquisition and edge omission

1. `.cjs` is included in the scanner's source extensions. Both `packages/aa-app` and `packages/zz-core` become nodes.
2. The one dependency expression begins with `@audit/`, not `./` or `../`. It fails the regex's captured-specifier condition.
3. `resolveJSSpecifier` is never called for that expression; no `addEdge(aa-app, zz-core)` occurs.
4. The package manifests do not supply an alternate edge in the inspected discovery call chain. The resulting nodes are independent in the recorded graph. [Extensions][extensions] [Scanner][scanner] [Dispatch][dispatch]

These are direct deductions from source. They are not claims about a measured installed workspace.

### 3.2 Full sweep ordering

The graph projection assigns a lexical rank to each node ID, and policy picks the smallest ready rank. With no recorded edge between these two nodes, `packages/aa-app` is ready before `packages/zz-core` has been visited and sorts before it. Subject to the stipulated fixture and ordinary shipped policy, the planned source order is consumer before prerequisite. The cross-cutting stages follow the recorded graph; they do not add the missing local edge. [Fact projection][facts] [Ordering policy][ordering] [Cross-cutting close][closing]

This is an under-specified graph being scheduled consistently, not a scheduler defect. Renaming the consumer to sort after the dependency can hide the problem in a full-sweep smoke test while leaving focus closure wrong.

### 3.3 Focused sweep closure

Select only `packages/aa-app`. `FilterDAG` marks it wanted and iterates its empty `DependsOn` list. It therefore omits `packages/zz-core`; selecting a physical node does not automatically append the cross-cutting stages. The real run applies the same filter before its gate baseline and node visits. [Focus closure][focus] [Run pass][pass]

This is stronger than merely choosing an inefficient order: the missing prerequisite may never receive a visit during that focused invocation. An additional pass will rediscover the same edge-free relationship unless the source spelling, configuration or adapter changes.

### 3.4 Why a broad gate is not a complete repair

Suppose an available, owner-declared or detected workspace gate emits a parseable path-owned diagnostic in `packages/zz-core/index.cjs`. During a focused run:

- Attribution against the filtered node set can return unattributed.
- Attribution against the retained full node set can identify `packages/zz-core`.
- The focused filter then skips that finding as belonging to another known subsystem outside the focus. [Finding filter][filter-findings]

This is a conditional consequence: it depends on the diagnostic parser producing that local path and on the ordinary measured-gate route. A location-free package-resolution error may instead remain unattributed. The command's failing result is still retained; the cited code does **not** turn its exit into a pass. The finding count, ratchet, error-free gates, retries and deployment outcomes must not be inferred from this single trace.

The problem is that the supposedly dependency-closed scope is incomplete before this otherwise intentional exclusion rule runs. Removing the exclusion rule would mix unrelated diagnostics back into focused work; repairing dependency acquisition or exposing its incompleteness is the narrower target.

### 3.5 Link to the ideation/interrogation/crucible priority

Mission preparation happens after the selected node and finding/angle have been assembled. It passes that attempt to deliberation and fixes receipt scope from `Node.Paths`. The write admission checks canonical containment against that scope. [Attempt handoff][attempt-handoff] [Mission preparation][mission-handoff] [Write admission][write-guard]

A capable interrogator can notice that the consumer depends on a sibling and explain why the current design is blocked. It should not silently acquire permission to repair the sibling. The missing work is an explicit host-owned dependency/capability observation that the planner and interrogator can use **before** treating the selected scope as complete.

A behavioral crucible needs to know which local implementation it is exercising. A passing consumer command does not, by itself, establish that the dependency-first sweep contract held or that the sibling package's behavior was improved. Conversely, an honest blocked design is useful evidence and should not be treated as an inferior outcome merely to maximize completed cycles.

## 4. Bounded proposed contract

Everything in this section is **proposed**, not implemented or validated by this review.

### 4.1 Distinguish syntax from resolution

For each relevant imported package spelling, retain a compact resolution observation containing:

- Importer source identity and snapshot.
- Literal specifier and source location.
- Resolution profile: package manager/runtime/compiler, version or declared profile, and applicable conditions.
- Local workspace package identity and canonical root, if established.
- Resolved implementation entry/source node or a bounded set of possible local nodes.
- Manifest/configuration evidence and its snapshot.
- Disposition: **local**, **external**, **unresolved**, **excluded by workspace policy**, or **unsupported resolution form**.

A bare spelling is input to this decision, not the result. The model can propose a read to resolve uncertainty; host code validates the resulting identity and membership.

Do not label every unresolved package external. Equally, do not add an edge to an arbitrary matching directory or every package with a similar name.

### 4.2 Keep the first supported slice small

A practical first contract can target declared npm workspaces with unique package names, explicit ordinary dependency declarations and unambiguous local entry points. Preserve relative resolution. Reject or explicitly qualify malformed manifests, duplicate names, ambiguous conditions and unavailable entry points.

The implementation approach needs a design decision:

1. **Manifest-based conservative adapter:** cheap offline package membership and dependency observation; can establish a local package relationship even before installation. It must distinguish a declaration from proof of the currently resolved runtime instance and must handle projection from package identity to the existing source-directory nodes.
2. **Toolchain-backed resolver:** better fidelity to a named configured resolver, with execution cost, environment dependence and availability requirements. It must not execute arbitrary project entry code or install dependencies merely to answer a graph query.
3. **Explicit incomplete capability:** retain existing relative-only discovery but mark local-package closure unresolved. This is an honest fallback, not a successful dependency-closed sweep.

A manifest alone does not justify inventing a precise source-file edge through conditional exports, generated output, aliases or version conflicts. If the initial adapter conservatively uses package-level units, that changes node identity and focus semantics and needs a separate compatibility plan. If it keeps directory-level nodes, it must explain which admitted directory implements the resolved entry and how local internal imports contribute the remaining closure.

### 4.3 Publish observations; let policy decide

The adapter should supply validated dependencies and completeness facts. Mangle should continue to derive ordering, focused admission and stopping. There should be no second Go scheduler or model-selected exception path.

On an unresolved relationship that could change a focused prerequisite set, possible policy outcomes include:

- Read-only design with an explicit unresolved dependency.
- A blocked focused execution awaiting additional permitted evidence.
- An owner-authorized broader sweep using the already allowed scope.

Which outcome to support is a product decision. Do not silently widen a requested focus or narrow existing protections. Membership exclusions and protected paths remain constraints even when the dependency is real.

### 4.4 Preserve evidence and invalidation

A dependency observation should become historical after a relevant source, package-name, workspace-membership, manifest, lockfile or resolution-profile change. The current pass already re-derives its graph, so a first implementation can prefer safe rediscovery rather than introduce a cache. If reuse is later added, its identity must cover the evidence used to choose the local target, not just the import string.

Unknown tooling should remain unknown. Node/package resolution says nothing by itself about test coverage, behavioral oracle adequacy, authorization, network availability or mission satisfaction.

## 5. Costs and failure modes

No performance, cost or success-rate measurement was taken.

### Expected costs to measure

- **Discovery:** if `B` is the admitted source/manifest byte volume and `E` the observed local edges, a straightforward indexed offline adapter should aim for scanning work proportional to `B + E`, plus existing filesystem traversal and graph processing. Repeated package/config lookups per import can make the constant or asymptotic cost materially worse; measure rather than assume.
- **Memory and receipts:** package-name/root indexes and edges add state proportional to the admitted package and dependency counts. Full manifest copies in every prompt are unnecessary; retain source-bound references and a small unresolved-boundary summary.
- **Toolchain route:** subprocess startup, resolver initialization, filesystem lookups and compiler project loading can dominate small repositories. This review supplies no millisecond or token savings estimate.
- **Execution:** correct closure may intentionally visit more nodes than the defective focused baseline. Higher gate/model cost may buy the required prerequisite coverage. Fewer visits are not an efficiency win if obligations disappeared.
- **Compatibility:** package-level node changes could affect stored focus names, journal attribution and resumed runs. Preserve existing node identities unless a separately versioned migration is designed.

### Failure modes the repair must avoid

| Risk | Required treatment |
|---|---|
| Third-party registry package shares a local-looking name | Establish actual declared/resolved local ownership; do not use name resemblance. |
| Two local manifests claim the same name | Return ambiguity; never select whichever directory was enumerated first. |
| Package exports vary by condition or module mode | Bind the observation to a named profile or preserve multiple possibilities explicitly. |
| `main`/`exports` names generated output | Do not substitute a guessed source path or treat a stale artifact as current source. |
| Symlink or package target leaves the admitted root | Apply canonical workspace membership and exclusions; a resolver is not new read/write authority. |
| Package manager has not installed local links | Distinguish declared local dependency from observed runtime resolution. Do not install as a graph-discovery side effect. |
| Comments, strings or multiline syntax fool the regex | A separate extraction-quality issue remains. A package resolver does not make regex extraction a complete parser. |
| Package dependency declaration is unused | Decide whether the graph is exact import order or conservative declared prerequisites; disclose conservative extra work. |
| Entire monorepo is linked to every consumer | Avoid false closure via all-to-all edges and accidental huge SCCs; count false-positive prerequisites. |
| Added edges create a real local cycle | Reuse existing SCC handling, while preserving the independently tracked node-namespace issue. |
| New graph facts make the design look more certain than it is | Carry unresolved/unsupported dispositions into the interrogator and result, not only debug logs. |

TypeScript path aliases, `#imports`, package self-references, pnpm/Yarn-specific semantics and arbitrary runtime plugin loading are adjacent qualification work. They should be named as unsupported or separately evaluated, not silently included in a claim that the first npm slice supports “all JS/TS.”

## 6. Fair evaluation plan, entirely unrun

No executable test was authored or run for this audit. The following are proposed acceptance cases and comparisons for a later authorized implementation session.

### 6.1 Fix the acceptance contract before evaluating

For the bounded npm slice:

> When two admitted workspace packages have an unambiguous local dependency under the declared resolver profile, the producer records its prerequisite relation or an explicit unresolved disposition. It must not present the relation as absent/external merely because the import uses a package name. A focused consumer sweep must include the admitted prerequisite closure or refuse/qualify the dependency-completeness claim before mutation.

Acceptance should observe the normal plan/run inputs and mission handoff, not just a standalone regex helper.

### 6.2 Deterministic cases

| Case | Observation to require |
|---|---|
| Minimal CJS fixture above | Both nodes exist; the local dependency is recorded, or execution is explicitly held as unresolved. |
| Same fixture with relative import | Existing relative-edge behavior is preserved. |
| Rename directories to reverse lexical order | Dependency closure and prerequisite-before-consumer relation do not depend on lucky names. |
| Focus consumer only | Dependency inclusion or explicit unresolved hold; never a silently incomplete closure. |
| Full sweep | Prerequisite visited before consumer under a resolved edge. |
| A→B→C workspace chain | Transitive closure, not just the first sibling. |
| Mutual package-name dependencies | Existing SCC treatment sees the cycle; no spurious independent ordering. |
| External package with no local counterpart | No fabricated workspace node or edge. |
| Unique scoped and unscoped names | Both forms preserve package identity. |
| Duplicate local names or incompatible declarations | Ambiguity remains explicit and does not depend on enumeration order. |
| Subpath/conditional export outside first supported slice | Explicit unsupported/unknown result; no nearest-file guess. |
| Local dependency excluded by workspace policy | Exclusion is preserved and explained; no read/write expansion. |
| Workspace gate diagnostic in prerequisite | After valid closure, diagnostic remains eligible for the admitted prerequisite; unrelated sibling diagnostic stays outside focus. |
| Location-free gate failure | Preserve failing/unattributed evidence rather than claim all diagnostics are solved by the adapter. |
| Manifest/import/profile change between passes | Dependency observation is refreshed; stale local target is not reused as current. |
| Design-only mission on incomplete graph | Interrogator receives the unresolved boundary, can produce a useful blocked design, and cannot expand the write scope. |

The current authored JS fixture covers relative imports, extension/index resolution, an external `react` import and excluded generated/declaration-only directories. Those are valuable regression controls. It was read, not run, and does not establish workspace-name behavior. [Current test][js-test]

### 6.3 Compare alternatives without giving the proposal an unfair advantage

Evaluate these on the **same repositories, source snapshots, package installations, resolver profiles, focus requests, gate settings, policies and budgets**:

- The pinned relative-only implementation.
- A proposed bounded manifest adapter.
- A proposed toolchain-backed resolution adapter, if implemented.

Use independently established local dependency identities from the named package manager/runtime/compiler as the reference where available. Record disagreements and unsupported cases rather than forcing a single guessed graph. Keep the reference resolver's execution separate from project code execution, installation and network access; those require their own authorized setup.

Report at least:

- True local edge and prerequisite-closure recall.
- False-positive local edges and unnecessary focused nodes.
- Correct prerequisite ordering and cycle treatment.
- Unresolved/excluded classifications and how often they prevent mutation.
- Preserved in-scope diagnostics and excluded unrelated diagnostics.
- Discovery latency, bytes read, subprocess count, peak memory and graph/receipt size.
- Gate invocations, paid model calls/tokens and elapsed time **per supported or honestly blocked outcome**, including failed attempts.
- Behavior on Windows and a Unix-like system, with separate results rather than inferring portability from one host.

For an eventual live ideation/crucible comparison, randomize treatment order across matched tasks, hold model/effort and total budget constant, and keep evaluation repositories separate from adapter-development fixtures. Score whether a meaningful requirement was supported by independent evidence, not just whether an attempt completed or a prompt looked richer. Report the graph-only benefit separately from any unproven downstream quality improvement.

### 6.4 Stopping and release criteria

A minimal implementation is not ready on helper tests alone. Require evidence through the production graph builder, focus selector, policy ordering and mission scope projection, plus negative controls showing unchanged authority and gate semantics. All unsupported forms must have an explicit disposition.

Failure of the proposed adapter to beat the baseline on validated closure or preservation, unacceptable discovery cost, or scope widening is a reason to reject or narrow it. No outcome threshold or performance target is claimed satisfied here.

## 7. Primary literature and what it supports

1. **npm CLI, Workspaces, versioned documentation.** Establishes that locally declared workspace packages can be consumed by package name. This is the semantic counterexample to equating bare names with external dependencies. It does not establish CodeNERD behavior or the correctness of a proposed resolver. [Documentation][npm]
2. **TypeScript, Modules Reference.** Describes real package lookup for workspace siblings and explains why path aliases are not equivalent to package resolution. This supports preserving resolver identity rather than rewriting every name as a relative path. It does not make the current regex complete or prove any future adapter. [Documentation][typescript]
3. **Andrey Mokhov, Neil Mitchell and Simon Peyton Jones, “Build Systems à la Carte,” PACMPL 2 (ICFP), Article 79, 2018, DOI 10.1145/3236774.** The paper separates dependency scheduling from decisions about rebuilding and supplies a precise comparative framework. The applicable lesson is to evaluate dependency observation and scheduling independently. Recurse is an improvement orchestrator, not the paper's build-system model, so the paper is not a proof of this audit or a measured benefit for CodeNERD. [Author-hosted paper][build-paper]

## 8. Limits and suggested disposition

**Source-supported at the pin:** relative-only import acquisition; absence of a package-name resolver in the inspected DAG path; missing-edge propagation to focus and scheduling; conditional dropping of an excluded prerequisite's path-owned finding; mission scope being determined before deliberation.

**Not established:** an observed failure in a real user repository, installed npm behavior for an executed fixture, parser behavior for a chosen live diagnostic, live model performance, gate verdict changes, successful exploitation, improved cost/quality after a repair, or universal JS/TS resolution support.

**Suggested disposition:** open, bounded JS/TS portability capability gap. The first checkable exit is accurate prerequisite admission or explicit incompleteness for ordinary local npm workspace package imports. This closes a specific hole in the existing portable-deliberation proposal while preserving the current executive boundary.

## Pinned source index

All repository links below refer to the same commit. Line ranges identify inspected evidence, not runtime traces.

[guidance]: https://github.com/theRebelliousNerd/codenerd/blob/b4ee29d064ed91e65c484ed29b335128490aa7f8/agents.md
[mission-spec]: https://github.com/theRebelliousNerd/codenerd/blob/b4ee29d064ed91e65c484ed29b335128490aa7f8/Docs/architecture/campaign/MISSION-IMPROVEMENT.md#L98-L112
[dispatch]: https://github.com/theRebelliousNerd/codenerd/blob/b4ee29d064ed91e65c484ed29b335128490aa7f8/internal/campaign/recurse_workspace.go#L60-L100
[collection]: https://github.com/theRebelliousNerd/codenerd/blob/b4ee29d064ed91e65c484ed29b335128490aa7f8/internal/campaign/recurse_workspace.go#L264-L316
[extensions]: https://github.com/theRebelliousNerd/codenerd/blob/b4ee29d064ed91e65c484ed29b335128490aa7f8/internal/campaign/recurse_workspace.go#L331-L344
[scanner]: https://github.com/theRebelliousNerd/codenerd/blob/b4ee29d064ed91e65c484ed29b335128490aa7f8/internal/campaign/recurse_workspace.go#L550-L573
[relative]: https://github.com/theRebelliousNerd/codenerd/blob/b4ee29d064ed91e65c484ed29b335128490aa7f8/internal/campaign/recurse_workspace.go#L575-L605
[scc]: https://github.com/theRebelliousNerd/codenerd/blob/b4ee29d064ed91e65c484ed29b335128490aa7f8/internal/campaign/recurse_workspace.go#L142-L236
[closing]: https://github.com/theRebelliousNerd/codenerd/blob/b4ee29d064ed91e65c484ed29b335128490aa7f8/internal/campaign/recurse_workspace.go#L238-L258
[focus]: https://github.com/theRebelliousNerd/codenerd/blob/b4ee29d064ed91e65c484ed29b335128490aa7f8/internal/campaign/recurse_dag.go#L36-L77
[planning]: https://github.com/theRebelliousNerd/codenerd/blob/b4ee29d064ed91e65c484ed29b335128490aa7f8/internal/campaign/recurse_plan.go#L12-L67
[facts]: https://github.com/theRebelliousNerd/codenerd/blob/b4ee29d064ed91e65c484ed29b335128490aa7f8/internal/campaign/recurse_plan.go#L109-L150
[ordering]: https://github.com/theRebelliousNerd/codenerd/blob/b4ee29d064ed91e65c484ed29b335128490aa7f8/internal/core/defaults/policy/recurse.mg#L420-L446
[pass]: https://github.com/theRebelliousNerd/codenerd/blob/b4ee29d064ed91e65c484ed29b335128490aa7f8/internal/campaign/recurse_cycle.go#L515-L563
[gate-scope]: https://github.com/theRebelliousNerd/codenerd/blob/b4ee29d064ed91e65c484ed29b335128490aa7f8/internal/campaign/recurse_cycle.go#L637-L689
[filter-findings]: https://github.com/theRebelliousNerd/codenerd/blob/b4ee29d064ed91e65c484ed29b335128490aa7f8/internal/campaign/recurse_cycle.go#L754-L830
[js-gates]: https://github.com/theRebelliousNerd/codenerd/blob/b4ee29d064ed91e65c484ed29b335128490aa7f8/internal/gates/detect.go#L175-L213
[attempt-handoff]: https://github.com/theRebelliousNerd/codenerd/blob/b4ee29d064ed91e65c484ed29b335128490aa7f8/internal/campaign/recurse_cycle.go#L1255-L1288
[mission-handoff]: https://github.com/theRebelliousNerd/codenerd/blob/b4ee29d064ed91e65c484ed29b335128490aa7f8/internal/campaign/recurse_mission.go#L173-L249
[write-guard]: https://github.com/theRebelliousNerd/codenerd/blob/b4ee29d064ed91e65c484ed29b335128490aa7f8/internal/campaign/recurse_mission_scope.go#L13-L74
[js-test]: https://github.com/theRebelliousNerd/codenerd/blob/b4ee29d064ed91e65c484ed29b335128490aa7f8/internal/campaign/recurse_workspace_test.go#L135-L161
[portable]: https://github.com/theRebelliousNerd/codenerd/blob/b4ee29d064ed91e65c484ed29b335128490aa7f8/Docs/audits/portable-recurse-deliberation-crucible.md
[go-work]: https://github.com/theRebelliousNerd/codenerd/blob/b4ee29d064ed91e65c484ed29b335128490aa7f8/Docs/audits/recurse-go-workspace-acquisition.md
[cargo-audit]: https://github.com/theRebelliousNerd/codenerd/blob/b4ee29d064ed91e65c484ed29b335128490aa7f8/Docs/audits/recurse-cargo-inherited-dependency-closure.md
[python-audit]: https://github.com/theRebelliousNerd/codenerd/blob/b4ee29d064ed91e65c484ed29b335128490aa7f8/Docs/audits/recurse-python-from-import-prerequisites.md
[language-audit]: https://github.com/theRebelliousNerd/codenerd/blob/b4ee29d064ed91e65c484ed29b335128490aa7f8/Docs/audits/recurse-gate-language-domain.md
[namespace-audit]: https://github.com/theRebelliousNerd/codenerd/blob/b4ee29d064ed91e65c484ed29b335128490aa7f8/Docs/audits/recurse-node-namespace-identity.md
[queue]: https://github.com/theRebelliousNerd/codenerd/blob/b4ee29d064ed91e65c484ed29b335128490aa7f8/Docs/audits/ASTRA_QUEUE.md
[npm]: https://docs.npmjs.com/cli/v11/using-npm/workspaces/#using-workspaces
[typescript]: https://www.typescriptlang.org/docs/handbook/modules/reference.html#paths-should-not-point-to-monorepo-packages-or-node_modules-packages
[build-paper]: https://simon.peytonjones.org/assets/pdfs/build-systems-original.pdf

