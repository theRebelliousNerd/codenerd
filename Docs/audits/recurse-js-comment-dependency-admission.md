---
doc-class: deep-dive
subsystem: campaign-recurse-dependency-admission
implementation-status: proposed-refinement
evidence-status: source-only
last-verified: 2026-11-04
verified-against: 81809be167ef6896df3eb5aff0885933a4c8ab7d
verification-method: static-source-inspection
main-rechecked: 2026-11-04T18:18:25Z
supersedes: []
---

# Commented JavaScript imports can expand recurse's focused work

## Finding and decision

**Recurse can promote import-shaped text inside an ordinary JavaScript comment into a dependency that expands a focused sweep.** The JS/TS scanner applies its regular expression directly to each raw source line. It does not establish that a match is part of executable syntax. If a commented dynamic import names an existing source file in another admitted directory, the resolver succeeds and the graph records that directory as a prerequisite. `FilterDAG` then includes it in the selected work, and Mangle schedules it before the requested consumer. [Extraction][scanner] [Resolution][resolver] [Focus closure][focus] [Ordering][ordering]

This is more than a diagram error. The live pass uses the same graph. A selected sibling becomes a normal visit, with its findings and, when the normal metric and test-gate requirements hold, an improvement attempt. The shared campaign constructor copies that node's paths into the task's `WriteSet`. The source-derived consequence is **wrong work selection and an incorrectly justified planned scope**, not a demonstrated permission bypass, inevitable file change, false-green gate or retained patch. [Live pass][pass] [Visit][visit] [Improvement admission][improve] [Task construction][campaign]

The bounded repair is to require a supported syntactic import observation before a relative path can become an authoritative prerequisite. Keep path membership, the existing resolver's separately stated capabilities, SCC handling, Mangle scheduling, campaign permissions and the regression ratchet. Uncertain extraction may require explicit qualification or refusal; it must not silently become either a complete empty graph or authority to work on arbitrary matching directories.

**Evidence boundary:** public main was pinned to [81809be167ef6896df3eb5aff0885933a4c8ab7d][pin]. Root and scoped guidance, campaign architecture, current implementation, authored tests and neighboring audits were inspected. No repository program, build, test, gate, model call, inference, training, benchmark or fixture was run. No implementation, credential or setting was changed. The fixture and evaluations below are proposed and unrun. No production incidence or quantitative benefit is established.

## Architectural contract and current status

Root guidance places creative reasoning with the model and executive planning, context and safety with Mangle. The campaign specification gives the proposer and requirements interrogator the selected node and measured work; they cannot choose a different write scope or confer permission. Correct executive derivation therefore depends on host observations that mean what their predicates claim. A deterministic fixpoint over a false dependency is still a false planning premise. [Root guidance][root] [Mission contract][mission-spec]

The operator-facing configuration says focus automatically includes dependencies. `SubsystemNode.DependsOn` supplies prerequisite order, while `Paths` identifies roots that the node's tasks may mutate. The CLI exposes this as `--subsystem` with dependencies pulled in. These are operational contracts, not merely a list of potentially interesting files. [Configuration][plan] [Node contract][node-contract] [CLI option][cli-option]

The owning architecture corpus distinguishes intended design from implemented and unverified behavior. Its campaign documents are useful for ownership and mission intent; the precise JS extraction behavior here is established by pinned source rather than assuming older architecture prose is complete. No rewrite of the north star or reduction of dependency-first planning is proposed. [Architecture guidance][architecture] [Campaign corpus][campaign-readme]

| Boundary | Status at the pin | Limit relevant here |
|---|---|---|
| Canonical workspace, refreshed membership and project exclusions | Implemented | Admitting both files does not establish a dependency between them. [Discovery][derive] [Collection][collection] |
| JS/TS source-directory nodes | Implemented | `.mjs` is supported; every admitted source directory becomes a node. [Extensions][extensions] [Scanner][scanner] |
| Relative import text extraction | Implemented as raw-line regex matching | No comment/string/token or AST classification precedes the match. [Scanner][scanner] |
| Relative target lookup | Implemented | File existence can verify a candidate pathname, but cannot establish that the source actually imports it. [Resolver][resolver] |
| Edge insertion and SCC collapse | Implemented | They preserve supplied relations; neither proves that a relation came from source syntax. [Insertion][add-edge] [Collapse][scc] |
| Focus closure and policy ordering | Implemented | An erroneous input edge is treated like an ordinary prerequisite. [Focus][focus] [Projection][projection] [Policy][ordering] |
| Campaign and mission scope handoff | Implemented | They copy selected node paths; they do not reconstruct import syntax. [Campaign][campaign] [Mission receipt][mission-prepare] |
| Syntax-aware extraction before authoritative edge admission | Missing in the inspected JS producer | The proposed correction is not implemented or behaviorally verified by this report. |

## Concrete witness: a retired import becomes scheduled work

Use an otherwise ordinary admitted workspace with two source files and no other dependency between their directories:

```text
a/index.mjs
z/index.mjs
```

```javascript
// a/index.mjs
// Retired example: const oldModule = import("../z/index.mjs");
export const value = 1;
```

```javascript
// z/index.mjs
export const otherValue = 2;
```

The filename labels above identify separate files. Stipulate readable regular files, successful membership admission, no forbidden path, no generated-source transformation that interprets the comment as a dependency, and no other imports. Select only subsystem `a`. No installed package, package-name resolver, TypeScript alias, symlink, missing file, output truncation or model-generated source is required for the graph witness.

ECMAScript's lexical grammar discards ordinary comments before syntactic parsing, with its separately specified line-terminator treatment. Thus the retired-example comment in `a/index.mjs` does not evaluate `import()` and is not a module dependency expression. The relevant standard establishes the language distinction; it does not certify CodeNERD's implementation. [ECMAScript comments][ecma-comments]

### Static producer-to-consumer trace

1. **Both directories are acquired.** `.mjs` is a supported source suffix. `scanJSImports` creates nodes for `a` and `z` before scanning any file contents. [Extensions][extensions] [Node acquisition][scanner]
2. **The comment is matched.** The unanchored `\bimport\s*\(\s*` alternative sees `import("../z/index.mjs")` inside the raw comment line. No preceding `//` check or lexical state prevents the match. [Regex and raw loop][scanner]
3. **A real file validates the wrong candidate.** The captured relative path is joined to the importing file's directory. `fileExists` succeeds for `z/index.mjs`, so resolution returns that file. Its existence is real; the claimed import is not. [Resolver][resolver]
4. **The false relation is promoted.** Both endpoints exist in the graph and differ, so `addEdge` records `a -> z`. With no reverse edge, SCC collapse preserves them as separate nodes and records `z` in `a.DependsOn`. [Insertion][add-edge] [Collapse][scc]
5. **Focus expands.** `FilterDAG(..., ["a"])` recursively follows `a.DependsOn`, marks `z` wanted and returns both nodes. Synthetic closing stages are not needed for this result. [Closure][focus]
6. **The executive faithfully schedules it.** The graph projection asserts `subsystem_depends("a", "z")`. Policy makes `a` unready until `z` is visited. The same closure is used by the plan entry and the live pass. [Projection][projection] [Policy][ordering] [Plan][plan] [Live pass][pass]

The first four steps establish the extraction/admission defect. The last two establish its normal planning consequence. These are deductions from the pinned source and the stipulated filesystem, not observations from an executed fixture.

### Semantics-preserving control

Remove only the retired-example comment, or replace its text with ordinary prose that has no import-shaped sequence. Both executable modules and both graph nodes remain. The scanner now records no `a -> z` edge, and focused closure contains only `a`. The difference in selected work is caused by text that the language does not treat as an import.

A positive control turns the same expression into an actual supported import expression outside a comment. The extractor should then retain the prerequisite under the chosen static-dependency contract, even if that expression may execute conditionally. The repair must distinguish syntax from comments without requiring proof that every valid import executes on every run.

The same raw matching also admits an import-shaped substring inside an ordinary string. For example, `export const example = 'import("../z/index.mjs")';` contains no evaluated import expression. This is a regression family for the same lexical boundary, not a separate novel finding. Code that deliberately evaluates a string would require a separately declared dynamic-analysis contract and is outside this witness.

## Why downstream safeguards do not repair the planning premise

### A selected visit is real work, but not automatically a write

The live driver derives and filters the graph, installs it in the kernel, then repeatedly asks for the next node and calls `visit`. A visit measures applicable checks, refreshes error-free evidence when configured, gathers findings for the node and attempts policy-selected repairs. It subsequently considers one measured improvement. [Pass][pass] [Visit][visit]

An actual improvement attempt needs an available metric for that pass's angle and a node or applicable workspace test gate. If those prerequisites are absent, `improve` writes an explicit skip instead of invoking the executor. Preserve that behavior. The minimal graph witness alone therefore proves an unwanted selected visit, not an unconditional model call. [Improvement guards][improve]

For the stronger **conditional dispatch witness**, use a fresh ordinary non-mission run, leave optional error-free measurement disabled, provide normal available JS/TS workspace verification with existing genuine tests, obtain metrics successfully, and use a pass whose angle is measurable. No failing diagnostic in `z` is necessary: a green node can still receive the normal improvement attempt. Under those additional premises, `run` hands a `RecurseAttempt` whose node is `z` to the executor. [Metric bridge][metrics] [Attempt dispatch][dispatch]

Both CLI and chat executors call `RecurseAttemptCampaign`. It creates a file-modification task and copies `a.Node.Paths` into its `WriteSet`. For the unwanted `z` visit, that is `z`, not the original focus string `a`. The new task is internally consistent with the selected graph; the faulty step happened before that graph became executive input. [CLI executor][cli-executor] [Chat executor][chat-executor] [Campaign constructor][campaign]

Normal constitutional permission, task acquisition, write leases, project restrictions, test evidence, reviews, keep/revert policy and compensation remain independent requirements. They can refuse or revert an attempted change. This report does not claim that any particular model writes to `z`, that the resulting task completes, or that an unrelated edit necessarily survives.

### The mission interrogator receives the already expanded selection

Mission preparation includes the complete attempt in its deliberation request and copies `Node.Paths` into the receipt's `Scope`. The mission containment guard subsequently classifies candidate paths relative to that copied scope, alongside protected inputs and policy checks. [Preparation][mission-prepare] [Containment][mission-write]

A path under `z` can therefore be inside the mission receipt because `z` has already become a selected node. This is **not a bypass of containment**: containment can be correct relative to a scope whose prerequisite justification was false. The audit must not collapse those two claims into “comments grant write permission.” Additional authority and admission checks still apply.

A capable requirements interrogator may notice that the import was retired and stop the proposal. That would be useful mitigation. It should not be the only mechanism that distinguishes a real dependency from a comment, nor should another model round be credited as free. Supply correct, bounded host observations before asking the creative layer to improve a selected subsystem. This preserves the existing ideation/interrogation/crucible architecture rather than creating another scheduler. [Mission contract][mission-spec]

### More graph checks are not syntax checks

`validateSweepGraph` rejects missing, duplicate and self-referential node identities. The witness has valid, distinct nodes and endpoints, so these checks are not expected to reject it. A well-formed graph is not proof of a correct source relation. [Validation][validation]

A false edge can also interact with genuine edges. If `z` genuinely imports `a`, the invented reverse edge makes the supplied graph strongly connected; collapse will produce a combined node. That can alter node identity and the set of paths belonging to one task. Focusing by the old `a` identity can instead fail closed as unknown, so this variant does not imply broader dispatch. It is a secondary source-derived consequence, not a requirement for the primary witness. Keep the existing SCC behavior for genuine cycles; validate edge provenance before collapse rather than deleting inconvenient cycles. [Collapse][scc] [Focus validation][focus]

## Prior work and precise novelty boundary

The existing **JavaScript workspace prerequisite identity** audit already says that comments, strings or multiline syntax can fool the regex and explicitly leaves extraction quality separate from package resolution. It also warns against false-positive prerequisites and indiscriminate all-to-all edges. That acknowledged limitation is credited here, not rediscovered as if it were absent. [Prior extraction limitation][prior-js-limit]

The new contribution is the concrete opposite-direction failure from that report's missing bare-package edge: **an ordinary comment resolves to an existing file, becomes a prerequisite, expands focused selection and can reach a normal file-modification campaign with that sibling's planned scope.** A workspace package-name resolver does not correct this trace because the erroneous candidate is already a resolvable relative pathname. This audit specifies the necessary lexical admission and comment-invariance checks at that consumer boundary.

Other nearby reports cover different joins:

- **Recurse test-census semantic admission** traces commented Go function text into a test-growth metric and the keep/revert decision. Here comment text becomes a dependency before work selection. No alleged new test or metric improvement is needed to demonstrate the graph defect. [Test-census audit][prior-test]
- **Recurse error-free focused package ownership** traces a real Go finding whose known package directory is mislabeled as the root. Here no diagnostic attribution or Go package spelling is involved; the false dependency changes which nodes are selected before findings are assigned. [Focused-package audit][prior-focus]
- **Python aliased/comma import** and **Python scan completion** audits preserve real prerequisites lost by extraction and require no invented edges as controls. They do not trace this JS comment to an extra selected node. [Aliased-import audit][prior-python] [Scan-completion audit][prior-scan]
- **Portable recurse deliberation crucible** already calls for capability-aware, qualified graph observations. The present proposal is a bounded conformance obligation within that design, not a new general-purpose agent architecture. [Portable proposal][portable]

The current audit inventory, queue and relevant recurse reports were screened. Repository searches for `jsSpecifier`, `scanJSImports`, comment/prerequisite terms and false-edge terms returned the cited neighbors, not a dedicated current report of this chain. This is a scoped overlap assessment, not proof that every historical branch or discussion is novel territory. [Audit inventory][audit-inventory] [Queue][queue]

## Proposed contract and implementation boundary

The following requirements are **proposed**. The owning campaign specification should record them before implementation, with source and behavioral evidence added only after qualification. [Architecture guidance][architecture]

### 1. Establish syntax before resolving a path

For the supported JS/TS source dialect, extract only recognized import-bearing syntax. Exclude ordinary line/block comments, ordinary string contents, non-expression template text and other tokens that merely contain an import-shaped substring. Preserve source location and the literal module specifier so downstream resolution has an inspectable basis.

An AST-backed adapter is one option. A lexer-backed bounded extractor can be another if its supported syntax and lexical state are explicit and tested. A rule that just discards lines beginning with `//` is insufficient: block comments, inline comments after code, quoted text and multiline lexical state remain. A rule that strips all quoted strings is also wrong because genuine module specifiers are string literals.

Template interpolation requires particular care: plain template text containing `import(...)` is not an import expression, while an actual import expression inside `${...}` is executable syntax. Comment delimiters inside strings are not comments. JS regex literals, JSX/TSX, escapes and language-version support require an explicit parser/dialect contract rather than a list of increasingly fragile substring exceptions.

This proposal does not require proving runtime reachability of every import. A legitimate syntactic import under a conditional may belong to a conservative potential-dependency envelope. That policy is different from treating ordinary comment text as syntax.

### 2. Separate extraction confidence from dependency authority

Keep a bounded observation sufficient to explain each admitted edge: importer identity and source snapshot, syntax kind/location, specifier, resolver profile, resolved admitted target and completeness/disposition. The existing graph stores directory relationships only; any new observation type or predicate is a design proposal, not an existing API.

Distinguish at least recognized syntax with a resolved local target, recognized syntax with an unresolved/external target, and unsupported or incomplete extraction. A deliberately conservative relationship can remain a candidate for investigation without silently becoming indistinguishable from a confirmed local prerequisite that expands mutation planning.

Mangle should continue to decide the admitted planning behavior from typed observations. The host supplies syntax and resolution evidence; neither an LLM's confidence nor a matching file's existence establishes the edge. Do not use the repair to widen workspace membership or protected-file permissions.

### 3. Make focused selection explainable and comment-invariant

For ordinary comments that are not part of an explicitly supported directive contract, changing only import-shaped comment text must not change the authoritative dependency edges, the focused prerequisite set or the planned node scopes. The source snapshot may change; the graph relation should not change merely because a discarded comment mentions an existing module.

When a source is unsupported, malformed, unavailable or incomplete, use an explicit disposition. A product decision can permit a qualified plan, a bounded evidence-gathering step or refusal; none should be disguised as a completely resolved dependency closure. Dynamic imports, package resolution and generated code remain independently qualified capabilities.

Bind any cached extraction to source identity and dialect/resolver configuration. Removing or commenting out a real import must retire that edge for the next derived graph. Keep the separately documented journal/resume applicability rules rather than assuming that a parser repair makes old visits or stored node identities universally reusable.

### 4. Reuse a verified parser boundary, not a second regex by resemblance

The repository contains other JS/TS parsing and retrieval paths, but this review does not establish that one is a complete, ready-to-wire replacement for recurse. Before reuse, verify dialect coverage, comment/token semantics, import kinds, error propagation, revision identity, file admission, boundedness and production reachability. A parser that exists elsewhere is not yet a repaired recurse path.

Do not execute project source, run package scripts, install packages or invoke a model just to classify this ordinary comment. If a future toolchain-backed resolution step is selected, its cost, availability and permission boundary need their own contract. The narrow comment witness is resolvable without those effects.

## Research grounding and limits

The ECMAScript specification provides the concrete lexical oracle: ordinary comments are not import syntax, and strings/templates have distinct lexical roles. It also cautions against a simplistic line-only model because line terminators and lexical state have defined parsing effects. The proposed contract applies that language distinction before admitting a graph edge. [ECMAScript lexical grammar][ecma-comments]

Cousot and Cousot's original abstract-interpretation work relates approximated program properties to a defined program semantics and permits useful imprecision. The relevant engineering inference is not that every extra static edge is forbidden. It is that the abstraction and the consumer's reliance must be stated: a conservative potential-dependency analysis can deliberately include real conditional import sites, while a raw substring in discarded comment text is not evidence that such a site exists. Their paper does not evaluate CodeNERD or prove this proposed extraction design sound. [Original paper and authors' summary][abstract-interpretation]

The quality target is therefore not “fewer nodes” in isolation. Missing a true prerequisite is also incorrect. Preserve supported real edges while rejecting demonstrated non-syntactic ones, and measure false positives and false negatives separately.

## Full costs and failure modes

Let `B` be admitted JS/TS source bytes, `R` candidate specifiers, and `V`/`E` the resulting directory graph sizes.

- Current extraction reads whole files, converts their contents to strings, splits lines and scans a fixed regex. Relative resolution can perform several filesystem probes per candidate. Import-shaped comment text adds candidate matches and target probes even before an edge changes the plan. These are work sources, not measured timings. [Scanner][scanner] [Resolver][resolver]
- Syntax-aware scanning still needs source reads. A chosen lexer may scan linearly in `B`; an AST parser also retains tokens/nodes and can have a larger memory constant. The bound, cancellation behavior and malformed-input costs must be verified for the actual implementation. Reusing an existing snapshot-aware parse can reduce duplicate work but adds cache correctness and invalidation obligations.
- Core SCC discovery and transitive focus traversal have graph work proportional to vertices and edges, with additional sorting in the present implementation. Extra edges can enlarge closure or collapse components. They also increase kernel facts and joins; no constant-time or specific speedup claim follows. [Collapse][scc] [Closure][focus] [Projection][projection]
- An extra selected node can add gate execution, metric census, finding repair, planning, JIT context, model usage, review, verification, journal writes, compensation and recovery. Those costs are conditional on the existing admission path. If the improvement is skipped, record the skip and any already-spent discovery/measurement work instead of assigning a fictitious model cost.
- Mission limits can prevent later work, but do not make wrongly selected work free. An unwanted prerequisite may consume time, cycles or spend that would otherwise have served the owner's focus. The amount and real task impact are unmeasured.
- Edge provenance consumes storage and context. Retain compact location/snapshot references rather than copying entire files into every node or mission request. Diagnostics should explain unsupported extraction without turning graph evidence into another unbounded transcript.
- A parser-only repair can still miss bare workspace package imports, mishandle relative resolution or ignore unsupported source. It must not advertise full JS/TS dependency completeness because comments are now handled. The package-name audit remains open on its own criteria.
- Over-aggressive filtering can delete genuine side-effect imports, exports-from, literal dynamic imports or imports in template expressions. Simply ignoring all dynamic imports would close the comment witness by discarding supported real relationships and is not acceptable.
- Treating every uncertain candidate as an authoritative edge can retain the scope-expansion defect under a new label. Treating every uncertain candidate as external can recreate missing-prerequisite defects. The chosen qualified/fail-closed policy must be visible to the planner and user-facing result.
- Source changes between extraction and execution, parse failures, incomplete reads, dialect mismatch and stale caches remain failure modes. This report does not establish that current workspace snapshots or existing recovery controls solve every one of them.

The expected benefit is more faithful focused work selection and avoidance of some unnecessary work. It is a hypothesis to measure, not a demonstrated reduction in total tokens, latency or defects.

## Unrun evaluation and closure criteria

Compare the pinned implementation with the proposed syntax-aware producer under the same files, membership, resolver, graph policy, gates, budget and execution settings. Include an independently parsed syntax oracle and explicit expected local edges. Do not use “produces a DAG” or “the test command passes” as the only graph-correctness assertion.

### Deterministic boundary matrix

| Case | Required observation |
|---|---|
| Primary ordinary line-comment fixture | Both nodes acquired; no authoritative `a -> z`; focus `a` excludes `z`. |
| Same comment removed or rewritten as plain prose | Same prerequisite set and planned scopes as the corrected primary fixture. |
| Same expression active outside the comment | Supported real edge retained; focus includes `z` and schedules it first. |
| Block comment across lines; inline comment after executable code | Import-shaped comment text does not create an edge; unrelated real code remains parsed. |
| Single/double-quoted string and plain template content | Contents do not become import syntax. |
| Actual import expression inside template interpolation | Supported dependency remains; do not discard the entire template indiscriminately. |
| Comment delimiters inside strings or regex literals | Lexical state preserved; no accidental truncation or invented import. |
| Static imports, side-effect imports, exports-from, require and literal dynamic imports | Explicitly supported forms retain the chosen dependency behavior. |
| Equivalent supported multiline syntax | Same edge set, or explicit unsupported status; not silent loss. |
| Missing target, excluded target or external target | Existing admission boundary retained; matching source text alone cannot admit a foreign node. |
| False reverse edge alongside a genuine one | No comment-created SCC; genuine cycles still collapse under the current contract. |
| Comment out or remove a formerly real import | New snapshot retires the edge; no stale focused inclusion. |
| Malformed, unsupported or failed extraction | Explicit disposition; no false complete dependency claim. |
| Existing relative-import and workspace-package fixtures | Preserve current supported behavior and separately report unresolved package-resolution capability. |

Start with the real `DeriveWorkspaceDAG`, not a test that hands a prebuilt graph directly to `FilterDAG`. Assert source nodes, edges and SCC membership before asserting the result of `RecurseSweepOrder`. The existing TypeScript relative-import test establishes authored coverage for genuine imports, extension/index handling and excluded outputs; it contains no comment/string negative-control fixture. Its presence is not evidence that the proposed tests passed. [Existing test][js-test]

Then exercise the real live-pass graph path in a fresh ordinary non-mission run with optional error-free measurement disabled, a deterministic observation-only execution seam and normal available metric/test inputs. Capture node visits and every `RecurseAttempt` handed to execution, then cancel before any source mutation. For the corrected comment fixture there must be no `z` attempt or campaign `WriteSet`; for the active-import control, the same gates and budgets must still permit legitimate prerequisite work. Also record the no-runnable-test case: the baseline's extra visit may skip improvement, which must not be misreported as a model dispatch.

Where mission behavior is tested, capture the selected attempt, deliberation request and receipt scope through the actual preparation path. Preserve ordinary permission and protected-input checks. A test that manually constructs a `z` receipt proves containment only, not that the producer no longer selected `z` because of a comment. No live proposer quality claim should be inferred from a deterministic stub.

Record false/missing edges, false/missing focused nodes, unwanted/omitted visits, SCC changes, dispatched attempt scopes, explicit incomplete outcomes, retained true dependencies, total source bytes read, filesystem probes, parser allocations, peak retained memory, kernel work, gate CPU/wall time, model usage where actually invoked, retries and failed/compensated work. Separate cold and warm parse-cache cases. Charge all acquisition, validation and recovery costs to each arm, and keep behavioral acceptance requirements equal.

If a later authorized stochastic study is useful, compare accepted task outcomes at equal total budgets on repositories with independently established dependency ground truth. Count blocked and abandoned work as outcomes. Do not manufacture efficiency by removing true prerequisites or replacing a production baseline with an intentionally inflated prompt.

**Closure:** ordinary import-shaped comment text cannot change authoritative focused dependency scope or produce a sibling attempt, supported active imports still create the required prerequisite work, and extraction uncertainty remains explicit through planning. This closes the specific lexical-to-scope admission defect without claiming complete JavaScript resolution, universal permission safety or better autonomous coding performance.

[pin]: https://github.com/theRebelliousNerd/codenerd/commit/81809be167ef6896df3eb5aff0885933a4c8ab7d
[root]: https://github.com/theRebelliousNerd/codenerd/blob/81809be167ef6896df3eb5aff0885933a4c8ab7d/agents.md#L3-L59
[architecture]: https://github.com/theRebelliousNerd/codenerd/blob/81809be167ef6896df3eb5aff0885933a4c8ab7d/Docs/architecture/AGENTS.md#L1-L103
[campaign-readme]: https://github.com/theRebelliousNerd/codenerd/blob/81809be167ef6896df3eb5aff0885933a4c8ab7d/Docs/architecture/campaign/README.md
[mission-spec]: https://github.com/theRebelliousNerd/codenerd/blob/81809be167ef6896df3eb5aff0885933a4c8ab7d/Docs/architecture/campaign/MISSION-IMPROVEMENT.md#L98-L112
[derive]: https://github.com/theRebelliousNerd/codenerd/blob/81809be167ef6896df3eb5aff0885933a4c8ab7d/internal/campaign/recurse_workspace.go#L64-L100
[collection]: https://github.com/theRebelliousNerd/codenerd/blob/81809be167ef6896df3eb5aff0885933a4c8ab7d/internal/campaign/recurse_workspace.go#L269-L317
[extensions]: https://github.com/theRebelliousNerd/codenerd/blob/81809be167ef6896df3eb5aff0885933a4c8ab7d/internal/campaign/recurse_workspace.go#L331-L343
[scanner]: https://github.com/theRebelliousNerd/codenerd/blob/81809be167ef6896df3eb5aff0885933a4c8ab7d/internal/campaign/recurse_workspace.go#L550-L573
[resolver]: https://github.com/theRebelliousNerd/codenerd/blob/81809be167ef6896df3eb5aff0885933a4c8ab7d/internal/campaign/recurse_workspace.go#L575-L604
[add-edge]: https://github.com/theRebelliousNerd/codenerd/blob/81809be167ef6896df3eb5aff0885933a4c8ab7d/internal/campaign/recurse_workspace.go#L131-L146
[scc]: https://github.com/theRebelliousNerd/codenerd/blob/81809be167ef6896df3eb5aff0885933a4c8ab7d/internal/campaign/recurse_workspace.go#L148-L234
[node-contract]: https://github.com/theRebelliousNerd/codenerd/blob/81809be167ef6896df3eb5aff0885933a4c8ab7d/internal/campaign/recurse_dag.go#L16-L33
[focus]: https://github.com/theRebelliousNerd/codenerd/blob/81809be167ef6896df3eb5aff0885933a4c8ab7d/internal/campaign/recurse_dag.go#L36-L76
[plan]: https://github.com/theRebelliousNerd/codenerd/blob/81809be167ef6896df3eb5aff0885933a4c8ab7d/internal/campaign/recurse_plan.go#L11-L67
[projection]: https://github.com/theRebelliousNerd/codenerd/blob/81809be167ef6896df3eb5aff0885933a4c8ab7d/internal/campaign/recurse_plan.go#L113-L134
[validation]: https://github.com/theRebelliousNerd/codenerd/blob/81809be167ef6896df3eb5aff0885933a4c8ab7d/internal/campaign/recurse_plan.go#L82-L110
[ordering]: https://github.com/theRebelliousNerd/codenerd/blob/81809be167ef6896df3eb5aff0885933a4c8ab7d/internal/core/defaults/policy/recurse.mg#L421-L446
[pass]: https://github.com/theRebelliousNerd/codenerd/blob/81809be167ef6896df3eb5aff0885933a4c8ab7d/internal/campaign/recurse_cycle.go#L515-L603
[visit]: https://github.com/theRebelliousNerd/codenerd/blob/81809be167ef6896df3eb5aff0885933a4c8ab7d/internal/campaign/recurse_cycle.go#L1052-L1104
[improve]: https://github.com/theRebelliousNerd/codenerd/blob/81809be167ef6896df3eb5aff0885933a4c8ab7d/internal/campaign/recurse_cycle.go#L1181-L1226
[metrics]: https://github.com/theRebelliousNerd/codenerd/blob/81809be167ef6896df3eb5aff0885933a4c8ab7d/internal/campaign/recurse_cycle.go#L1575-L1605
[dispatch]: https://github.com/theRebelliousNerd/codenerd/blob/81809be167ef6896df3eb5aff0885933a4c8ab7d/internal/campaign/recurse_cycle.go#L1252-L1294
[campaign]: https://github.com/theRebelliousNerd/codenerd/blob/81809be167ef6896df3eb5aff0885933a4c8ab7d/internal/campaign/recurse_attempt.go#L55-L141
[mission-prepare]: https://github.com/theRebelliousNerd/codenerd/blob/81809be167ef6896df3eb5aff0885933a4c8ab7d/internal/campaign/recurse_mission.go#L174-L246
[mission-write]: https://github.com/theRebelliousNerd/codenerd/blob/81809be167ef6896df3eb5aff0885933a4c8ab7d/internal/campaign/recurse_mission_scope.go#L13-L74
[cli-option]: https://github.com/theRebelliousNerd/codenerd/blob/81809be167ef6896df3eb5aff0885933a4c8ab7d/cmd/nerd/cmd_campaign_recurse.go#L81-L88
[cli-executor]: https://github.com/theRebelliousNerd/codenerd/blob/81809be167ef6896df3eb5aff0885933a4c8ab7d/cmd/nerd/cmd_campaign_recurse.go#L225-L263
[chat-executor]: https://github.com/theRebelliousNerd/codenerd/blob/81809be167ef6896df3eb5aff0885933a4c8ab7d/cmd/nerd/chat/campaign_recurse.go#L218-L230
[js-test]: https://github.com/theRebelliousNerd/codenerd/blob/81809be167ef6896df3eb5aff0885933a4c8ab7d/internal/campaign/recurse_workspace_test.go#L135-L165
[prior-js-limit]: https://github.com/theRebelliousNerd/codenerd/blob/81809be167ef6896df3eb5aff0885933a4c8ab7d/Docs/audits/recurse-js-workspace-prerequisite-identity.md#L213-L227
[prior-test]: https://github.com/theRebelliousNerd/codenerd/blob/81809be167ef6896df3eb5aff0885933a4c8ab7d/Docs/audits/recurse-test-census-semantic-admission.md
[prior-focus]: https://github.com/theRebelliousNerd/codenerd/blob/81809be167ef6896df3eb5aff0885933a4c8ab7d/Docs/audits/recurse-errorfree-focused-package-ownership.md
[prior-python]: https://github.com/theRebelliousNerd/codenerd/blob/81809be167ef6896df3eb5aff0885933a4c8ab7d/Docs/audits/recurse-python-aliased-comma-imports.md
[prior-scan]: https://github.com/theRebelliousNerd/codenerd/blob/81809be167ef6896df3eb5aff0885933a4c8ab7d/Docs/audits/recurse-python-import-scan-completion.md
[portable]: https://github.com/theRebelliousNerd/codenerd/blob/81809be167ef6896df3eb5aff0885933a4c8ab7d/Docs/audits/portable-recurse-deliberation-crucible.md
[audit-inventory]: https://github.com/theRebelliousNerd/codenerd/tree/81809be167ef6896df3eb5aff0885933a4c8ab7d/Docs/audits
[queue]: https://github.com/theRebelliousNerd/codenerd/blob/81809be167ef6896df3eb5aff0885933a4c8ab7d/Docs/audits/ASTRA_QUEUE.md
[ecma-comments]: https://tc39.es/ecma262/multipage/ecmascript-language-lexical-grammar.html#sec-comments
[abstract-interpretation]: https://www.di.ens.fr/~cousot/COUSOTpapers/POPL77.shtml
