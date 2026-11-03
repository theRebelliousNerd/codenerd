---
title: Recurse Python module-file prerequisite ownership
date: 2026-11-03
status: proposed
source_commit: fadf3e4c8bdb14821a7c36e6fcda0a13b94f22d7
verification: static-source-inspection-only
supersedes: []
---

# Preserve Python module-file ownership in recurse prerequisites

## Finding and decision

**A local Python module directly under an import root can be discovered as source but disappear as a prerequisite of its importer.** Recurse groups Python files into directory nodes, then resolves imports against a set of directory names. For an ordinary `import helper` whose source is workspace-root `helper.py`, it checks directory `helper`, then `src/helper`, but refuses the zero-component fallback that would identify the module's owning directory. The `.` node exists; the `app -> .` edge does not. A focused `app` sweep consequently excludes the prerequisite's node. [Discovery and resolution][python] [Focus closure][filter]

This is a **source-supported graph-acquisition gap with an unrun conditional planning witness**. The missing information is the resolved module file and its directory owner, not an unparsed name after `from ... import`, an implicit package initializer, or a policy-ordering error. The proposal is to resolve an admitted source file under an explicit import-root interpretation, then project that source to the existing directory node. Do not repair it by assigning every unresolved import to the root.

The existing portability proposal already calls for honest adapter capabilities. This report contributes a concrete ordinary-module boundary and its focused-planning consequence, not another general agent architecture. Deliberation operates downstream of selected recurse attempts; another reasoning round cannot retroactively make a missing graph prerequisite part of that focused plan. A role may independently discover the file through its read tools, but that is different from a host-admitted prerequisite relation. [Portable proposal][portable] [Mission preparation][mission] [Mission ownership contract][mission-spec]

## Evidence boundary

Public main resolved to **fadf3e4c8bdb14821a7c36e6fcda0a13b94f22d7** on November 3, 2026, with commit time 19:21:51 UTC, and remained unchanged when rechecked at approximately 19:45 UTC. All repository links below are immutable. The review read root and scoped guidance, campaign/session architecture, the requirements-interrogator skill, implementation, current audit inventory and neighboring reports, plus relevant authored tests. [Pin][pin] [Guidance][guidance] [Interrogator][skill]

No repository program, build, test, model inference, gate, benchmark, Python import or failure injection ran. No repository source, credential, configuration, permission or setting was changed. Source copies and this report are research artifacts only. All witnesses, suggested checks, and performance or quality outcomes below are proposed and unexecuted.

The finding establishes how the inspected code maps the stated input. It does not establish a live-model failure frequency, an accepted defective change, an escaped permission boundary, or absence of all alternate discovery paths.

## 1. The existing contract and production chain

Recurse declares its nodes to be the target workspace's own package directories, with import edges ordering prerequisites before importers. `SubsystemNode.DependsOn` names nodes that must sweep first; `FilterDAG` promises to retain the selected nodes and their transitive dependencies. The operator-facing configuration likewise says dependencies are pulled into a focused sweep automatically. [Graph contract][graph-contract] [Node and focus contract][filter] [Planning entry][plan]

The relevant chain is:

1. `DeriveWorkspaceDAG` canonicalizes the root, refreshes shared membership, loads project restrictions, collects admitted sources and calls `scanPythonImports`. [Derivation][derive]
2. Source collection retains admitted `.py` file paths, including root files. Hidden directories and project-forbidden paths are excluded, and membership remains authoritative. [Collection][collection]
3. `scanPythonImports` reduces that file inventory to `pyDirs map[string]bool`, adding one node for each containing directory. The complete source-file identity is not passed to its resolver. [Python producer][python]
4. `pythonImportTargets` parses the full `helper` name in the simple statement used below. `resolvePythonModule` searches directory prefixes under `.` and `src`. For an absolute import, it breaks before `k == 0`, so it cannot return either import-root directory as owner of a top-level module file. [Parser and resolver][resolver]
5. An unsuccessful resolution creates no edge. The existing edge admission and strongly connected component collapse preserve relationships already acquired; they do not infer missing file ownership. [Graph operations][graph]
6. Both the planning entry and actual pass driver feed the resulting graph into the same dependency closure. Mangle orders supplied facts correctly, but cannot derive an absent edge. [Planning entry][plan] [Pass driver][pass] [Ordering policy][ordering]

Mission preparation receives the already selected attempt, records its node and allowed paths, and starts a design-only deliberation thread. Its existing admission checks preserve source/vision freshness and execution authority. The proposed adapter correction should feed this established pipeline, not allow the proposer to rewrite its own scope. [Mission input][mission-input] [Mission preparation][mission] [Mission source admission][mission-source]

## 2. Minimal conditional static witness

Use an ordinary Python workspace with exactly these relevant sources:

```text
helper.py             VALUE = 7
app/__init__.py        empty
app/main.py            import helper
                      VALUE = helper.VALUE
```

Assumptions:

- All three files are admitted, readable ordinary files under one canonical workspace root.
- The intended execution environment resolves top-level modules from that root. For example, ordinary `python -m app.main` from the root, without isolated/safe-path options, places the current directory on the import search path. This example is explanatory and was not executed.
- No preloaded `helper`, custom import hook, same-named built-in or external shadow, generated source, symlink alias, or other import is needed.
- There is no `helper/` directory and no `src/helper/` directory.
- The focused subsystem is exactly `app`; all graph nodes have ordinary, distinct IDs.

Python's module documentation identifies `helper.py` as a source module and describes search through `sys.path`; its command-line documentation describes the ordinary `-m` search-path behavior. These establish the semantic premise, not an executed import receipt. [Python modules][python-modules] [Python command line][python-cli]

### Deterministic source trace

| Stage | Source-derived result |
|---|---|
| Source discovery | Retains `helper.py`, `app/__init__.py`, `app/main.py`. |
| Directory ownership | `pyDirs = {".": true, "app": true}`; both nodes exist. |
| Parsing `import helper` | `pyImportTarget{Up: 0, Module: "helper"}`. |
| Root-base lookup | `k=1` checks `helper`, absent from `pyDirs`; `k=0` breaks before checking `.`. |
| `src`-base lookup | `k=1` checks `src/helper`, absent; `k=0` again breaks. |
| Edge acquisition | Resolver returns false, so no `app -> .` edge is added. |
| Focus closure | Starting from `app`, `FilterDAG` has no dependency to traverse and retains only `app`. |
| Correct relation under the stated import domain | The imported source is `helper.py`; its existing graph owner is `.`, so `app` has that prerequisite. |

These steps follow directly from the cited producer, resolver and focus code. No special model output, corrupted state, paid inference, gate failure or race is necessary to expose the mapping. [Python producer][python] [Resolver][resolver] [Focus closure][filter]

The full, unfocused graph still contains the root node. Moreover, `.` sorts before `app`, so this particular fixture does **not** establish incorrect full-sweep order: its unconditional order can happen to match the missing relationship. The precise primary consequence is focused prerequisite omission. A graph may look correct on one complete ordering while its dependency closure remains incorrect. [Ordinal construction][ordinals]

### Related source-root variant

Move the helper to `src/helper.py`, retain `app/main.py`, and explicitly specify an execution domain in which `src` is an import root and no earlier root provides `helper`. Discovery now creates `src`, but lookup still tests `helper` and `src/helper`, never the admitted owner `src`. Focused `app` again omits the helper owner.

Here `app` sorts before `src`, so the full graph's available-node ordering can also put the importer first under the simple no-other-edge fixture. This is a second conditional static variant, not evidence that every `src` layout has those interpreter settings or that any command ran. The source already explicitly attempts a `src` base; the missing step is recognizing a source file directly beneath it. [Resolver][resolver] [Ordering policy][ordering]

## 3. Limits and counterexamples

Credit the working boundaries:

- Unknown focus IDs fail closed. This witness uses a known `app` node.
- Membership, forbidden-path filtering, canonical roots, SCC handling and graph validation remain useful. No excluded source needs to be admitted.
- A genuinely package-shaped import such as an existing `helper/__init__.py` makes `helper` a directory node; this particular flat-module miss does not apply.
- Relative imports can take the `k == 0` path. The primary witness deliberately uses a complete absolute import.
- Another recognized import edge can already pull the root into the focused closure. The witness excludes that alternative path.
- Whole-workspace gates or a role's independent source reads may still observe `helper.py`; absence of its scheduled prerequisite visit is not proof that it evades all inspection or verification.
- Root-node ownership is coarse. `Paths: ["."]` denotes a root that covers descendants, not merely `helper.py`. A correction must preserve truthful dependencies without using the new edge as unchecked permission to mutate the whole workspace. [Node scope][filter] [Mission scope contract][mission-spec]
- Better acquisition does not prove candidate correctness, test adequacy or mission satisfaction. Those are separate contracts with independent safeguards. [Session design boundary][session-spec] [Mission verification boundary][mission-spec]

The inability to resolve a module should be represented as unsupported or unresolved evidence when the adapter cannot establish its search domain. It should not silently become a claim that the importer has no local prerequisite.

## 4. Nonduplication and reuse

A bounded screen covered the pinned audit inventory/queue, all deliberation audit bodies, the portable-crucible proposal, both current Python dependency reports, and nearby JavaScript/Cargo graph reports. Repository searches for `resolvePythonModule` and for top-level-module/recurse references were also inspected. No exact predecessor for the flat-module owner omission was identified. This is not a claim of exhaustive novelty across private discussions, every branch or all history. [Queue][queue]

| Existing report | Distinct boundary here |
|---|---|
| Python from-import prerequisites | Drops names after `from ... import`. This witness uses plain `import helper`, whose complete name survives parsing. Adding imported-name fields does not establish the file's owner. |
| Python package-initializer prerequisites | Resolves a leaf but fails to retain implicitly loaded regular-package ancestors. `helper.py` is a top-level module file, not a package descendant; no helper ancestor initializer is involved. Adding initializer edges cannot recover its unresolved direct target. |
| Portable recurse deliberation crucible | Already proposes capability-aware adapters. This is one falsifiable acquisition failure beneath that proposal. |
| Deliberation evidence/context/values reports | Concern what happens after a role or thread exists. Carrying more role evidence does not itself repair the graph passed to focused scheduling. |

[From-import report][prior-from] [Initializer report][prior-init] [Portable proposal][portable]

Existing CodeDOM code already has a Python file-probing helper that checks module files and package initializer candidates. That is an integration candidate, **not** proof of a drop-in correct resolver: its search walks ancestors with a separate limit and its precedence/domain differ from the recurse adapter. Reusing it would require an explicit shared contract, membership checks, revision identity, root ordering and conformance tests. The proper goal is one supported acquisition contract rather than a third independent heuristic. [Existing file resolver][codedom]

## 5. Proposed contract, not implemented

Update the campaign/portability specification before implementation.

1. **Name the resolution domain.** Declare supported import roots and precedence, file/package forms, workspace membership, case/path treatment and source snapshot. Do not equate the current process's arbitrary working directory with the target repository's interpreter configuration.
2. **Keep file identity until ownership is known.** Preserve the importer and parsed reference, resolve an admitted module source or explicitly classified package/namespace target, then map the resolved source to its graph owner. A top-level `.py` file maps to the containing `.` or `src` node when that file is the supported resolution.
3. **Do not replace missing resolution with a root wildcard.** Removing only the `k == 0` guard would make an unrelated external import attach to a root merely because some Python file happens to live there. Require a matching admitted source, not just an existing owner directory.
4. **Preserve uncertainty.** Distinguish a known local source, external/built-in dependency, unresolved reference and unsupported import environment. A conservative envelope may be useful, but it must be labeled and must not be presented as exact runtime import resolution.
5. **Project before focusing.** Feed admitted owner edges into the current SCC collapse, dependency closure and Mangle facts. Keep policy responsible for visitation and stop decisions.
6. **Keep dependency and mutation authority separate.** A necessary prerequisite can require observation or a separate admitted attempt; it is not a grant to write outside the user's authorized scope. Explicitly review root-owner granularity, particularly when `.` enters a previously narrow focus.
7. **Bind cached resolution to its inputs.** If reuse/caching is introduced, changes to module files, package markers, membership, import-root configuration or revision must invalidate the affected owner relation. Re-deriving each pass should remain meaningful.
8. **Make capability limitations inspectable.** Let the requirements interrogator see a compact capability/coverage statement when design work depends on unresolved prerequisites. It may ask for needed evidence or report a limit; it must not manufacture a passing dependency proof or silently select new tooling.

No new planner, public predicate vocabulary, free-form shell tool, provider switch or additional model role is required. A bounded host observation followed by the existing policy pipeline is the natural correction.

### Failure modes a repair must guard against

- Treating every unresolved standard-library or third-party import as a dependency on `.`.
- Resolving an excluded or external path and admitting it merely because its textual owner resembles a local node.
- Mistaking a namespace directory for an ordinary module file, or changing package/module precedence.
- Choosing root before `src` when the declared execution domain says otherwise.
- Treating all ancestor directories as regular packages or assuming namespace packages have initializer files.
- Expanding the write envelope when only dependency observation was justified.
- Retaining an old module-owner edge after a rename, deletion or membership change.
- Reporting an exact graph where custom import hooks, zip/extension modules or dynamic path mutation are unsupported.

Python's import specification and PEP 420 make the file/package/namespace distinctions real; a directory-only index is not a universal substitute for them. This report proposes a narrow file-backed domain first, not general Python interpreter equivalence. [Import system][python-import] [Namespace specification][pep420]

## 6. Costs and tradeoffs

The current scan already reads Python sources and retains their paths during acquisition. A file-aware index can reuse that inventory, adding memory proportional to admitted module identities and lookup work per import. A simple implementation with `F` admitted files, `I` imports, `R` configured roots and dotted depth `D` may require indexing work proportional to `F` and up to `I × R × D` path/prefix probes, plus canonicalization and revision checks. This is a design estimate, not a measured runtime complexity guarantee. Graph collapse, serialization and kernel work also depend on the resulting edge set.

A repaired focused closure may legitimately contain more nodes. That can increase scans, attempts, paid deliberation calls, gates, snapshot work and latency. In the root-owner fixture, coarse ownership can produce a large increase. The repair must not be sold as a token optimization merely because it avoids some mislocalized reasoning.

Possible benefits include fewer irrelevant hypotheses, clearer unsupported-adapter outcomes, and fewer attempts aimed only at an importer when its local premise needs work. None is measured here. Count both successfully resolved tasks and costs incurred before a stop, failure, refusal or abandonment. A stricter adapter that declines everything is not automatically better.

Full cost comparisons should include host discovery/indexing, CodeDOM/world refresh, kernel work, storage, all proposer/challenger and specialist requests, tool reads, provider retries, cache/input/output/reasoning usage where reported, attempt/gate work, rollback and operator recovery. Do not compare only the final model answer or surviving commit.

## 7. Fair evaluation plan: all unrun

### Deterministic acquisition and closure checks

First use temporary fixture trees and captured source facts, without a live model or project-module execution:

- The root `helper.py` fixture: require an explicit `app -> .` relation and a focused closure containing both owners.
- The `src/helper.py` variant with a declared `src` domain: require `app -> src`; distinguish planning correctness from whether a real interpreter has that path.
- A package control with `helper/__init__.py`; ordinary package behavior must not regress.
- An external-import negative control: root contains unrelated `helper.py`, importer names an absent external module; do not invent an edge to `.`.
- Module/package/namespace collisions under a specified Python search domain, including a module file and a same-named directory without an initializer.
- Root versus `src` precedence; duplicate names; case-sensitive and case-insensitive supported platforms; canonical paths and symlinks subject to existing membership policy.
- Same-owner imports, cycles, exclusions, unreadable sources and unknown focus IDs.
- Rename/delete/addition and import-root changes between passes, so an old file-owner binding cannot remain current.
- Separate controls for the earlier from-import and initializer findings. Passing this report's fixture does not close either of them.

Capture the source inventory, parsed reference, resolution classification, resolved file, owner edge, SCC output, focused closure and asserted policy facts. This reveals the failing boundary without conflating it with model interpretation.

The authored Python graph test exercises nested modules beneath `src/shop` and relative package relationships. It is useful existing coverage, but its body does not exercise a module file directly under `.` or `src` imported from another node. It was read, not run. [Existing Python test][tests]

### Production path and behavioral qualification

After deterministic checks, exercise the real plan and pass entry paths against controlled fixtures, with stubbed attempt work first. Verify that the selected nodes, scopes and policy-derived order correspond to the same acquired graph, and that forbidden/external prerequisites cannot broaden authority. Then, only under separate execution authorization, use a clean interpreter environment to validate the declared resolution domain.

For an eventual live study, compare baseline and candidate on paired tasks containing flat modules, packages, `src` layouts, shadowing and unsupported environments. Keep source snapshots, task wording, model/provider/effort, JIT corpus, tool authority, execution/gate configuration and full resource budgets matched. Include straightforward package tasks as regression controls and unsupported cases as valid unresolved outcomes.

Measure prerequisite precision and recall against independently reviewed file ownership, truthful unsupported rates, focused scope growth, useful design/repair completion, mislocalized work, permission refusals, actual behavioral acceptance, total cost and latency. Include all failed and abandoned runs. Judge the design and evidence, not polished prose or the mere presence of a ready verdict. Report uncertainty by repository/task family; do not generalize a few fixtures to “any codebase.”

## 8. Checkable exit

Close this gap when the owning specification defines file-to-node resolution under a supported import-root domain; the production acquisition path retains the root/`src` module owner before focus; the primary fixture has the correct closure; external imports do not become root dependencies; scope and membership protections remain intact; and executed verification is separately recorded.

A larger deliberation context, a generic full-workspace scan, a passing unrelated package test or simply removing the `k == 0` break is not that exit.

## Pinned references

[pin]: https://github.com/theRebelliousNerd/codenerd/commit/fadf3e4c8bdb14821a7c36e6fcda0a13b94f22d7
[guidance]: https://github.com/theRebelliousNerd/codenerd/blob/fadf3e4c8bdb14821a7c36e6fcda0a13b94f22d7/agents.md#L5-L32
[skill]: https://github.com/theRebelliousNerd/codenerd/blob/fadf3e4c8bdb14821a7c36e6fcda0a13b94f22d7/.agents/skills/requirements-interrogator/SKILL.md
[graph-contract]: https://github.com/theRebelliousNerd/codenerd/blob/fadf3e4c8bdb14821a7c36e6fcda0a13b94f22d7/internal/campaign/recurse_workspace.go#L25-L46
[derive]: https://github.com/theRebelliousNerd/codenerd/blob/fadf3e4c8bdb14821a7c36e6fcda0a13b94f22d7/internal/campaign/recurse_workspace.go#L58-L100
[collection]: https://github.com/theRebelliousNerd/codenerd/blob/fadf3e4c8bdb14821a7c36e6fcda0a13b94f22d7/internal/campaign/recurse_workspace.go#L262-L315
[python]: https://github.com/theRebelliousNerd/codenerd/blob/fadf3e4c8bdb14821a7c36e6fcda0a13b94f22d7/internal/campaign/recurse_workspace.go#L456-L481
[resolver]: https://github.com/theRebelliousNerd/codenerd/blob/fadf3e4c8bdb14821a7c36e6fcda0a13b94f22d7/internal/campaign/recurse_workspace.go#L483-L544
[graph]: https://github.com/theRebelliousNerd/codenerd/blob/fadf3e4c8bdb14821a7c36e6fcda0a13b94f22d7/internal/campaign/recurse_workspace.go#L102-L256
[filter]: https://github.com/theRebelliousNerd/codenerd/blob/fadf3e4c8bdb14821a7c36e6fcda0a13b94f22d7/internal/campaign/recurse_dag.go#L16-L76
[plan]: https://github.com/theRebelliousNerd/codenerd/blob/fadf3e4c8bdb14821a7c36e6fcda0a13b94f22d7/internal/campaign/recurse_plan.go#L12-L66
[ordinals]: https://github.com/theRebelliousNerd/codenerd/blob/fadf3e4c8bdb14821a7c36e6fcda0a13b94f22d7/internal/campaign/recurse_plan.go#L112-L138
[pass]: https://github.com/theRebelliousNerd/codenerd/blob/fadf3e4c8bdb14821a7c36e6fcda0a13b94f22d7/internal/campaign/recurse_cycle.go#L515-L603
[ordering]: https://github.com/theRebelliousNerd/codenerd/blob/fadf3e4c8bdb14821a7c36e6fcda0a13b94f22d7/internal/core/defaults/policy/recurse.mg#L403-L446
[mission-input]: https://github.com/theRebelliousNerd/codenerd/blob/fadf3e4c8bdb14821a7c36e6fcda0a13b94f22d7/internal/campaign/recurse_mission.go#L174-L198
[mission]: https://github.com/theRebelliousNerd/codenerd/blob/fadf3e4c8bdb14821a7c36e6fcda0a13b94f22d7/internal/campaign/recurse_mission.go#L212-L264
[mission-source]: https://github.com/theRebelliousNerd/codenerd/blob/fadf3e4c8bdb14821a7c36e6fcda0a13b94f22d7/internal/campaign/recurse_mission.go#L288-L318
[mission-spec]: https://github.com/theRebelliousNerd/codenerd/blob/fadf3e4c8bdb14821a7c36e6fcda0a13b94f22d7/Docs/architecture/campaign/MISSION-IMPROVEMENT.md#L98-L112
[session-spec]: https://github.com/theRebelliousNerd/codenerd/blob/fadf3e4c8bdb14821a7c36e6fcda0a13b94f22d7/Docs/architecture/session/IMPLEMENTED_SPEC.md#L816-L847
[codedom]: https://github.com/theRebelliousNerd/codenerd/blob/fadf3e4c8bdb14821a7c36e6fcda0a13b94f22d7/internal/world/codemodel/resolve.go#L56-L119
[tests]: https://github.com/theRebelliousNerd/codenerd/blob/fadf3e4c8bdb14821a7c36e6fcda0a13b94f22d7/internal/campaign/recurse_workspace_test.go#L112-L135
[queue]: https://github.com/theRebelliousNerd/codenerd/blob/fadf3e4c8bdb14821a7c36e6fcda0a13b94f22d7/Docs/audits/ASTRA_QUEUE.md
[portable]: https://github.com/theRebelliousNerd/codenerd/blob/fadf3e4c8bdb14821a7c36e6fcda0a13b94f22d7/Docs/audits/portable-recurse-deliberation-crucible.md
[prior-from]: https://github.com/theRebelliousNerd/codenerd/blob/fadf3e4c8bdb14821a7c36e6fcda0a13b94f22d7/Docs/audits/recurse-python-from-import-prerequisites.md
[prior-init]: https://github.com/theRebelliousNerd/codenerd/blob/fadf3e4c8bdb14821a7c36e6fcda0a13b94f22d7/Docs/audits/recurse-python-package-initializer-prerequisites.md
[python-modules]: https://docs.python.org/3.14/tutorial/modules.html#the-module-search-path
[python-cli]: https://docs.python.org/3/using/cmdline.html#cmdoption-m
[python-import]: https://docs.python.org/3/reference/import.html
[pep420]: https://peps.python.org/pep-0420/#specification

