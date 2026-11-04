---
doc-class: deep-dive
subsystem: campaign
implementation-status: partial
last-verified: 2026-11-04
verified-against: be5e24a2812bfff1e4deb328f43b9cadeef60d30
supersedes: []
---

# Preserve every plain Python import clause in recurse prerequisite discovery

## Finding and evidence boundary

**A valid alias in an ordinary Python import statement can silently remove every later comma-separated module from recurse's dependency graph.** For `import mm_seen as seen, zz_support`, the current extractor records `mm_seen` but never records `zz_support`. Both can be admitted local package nodes. A focused consumer sweep then omits the second prerequisite, while a full sweep can visit the consumer before it. This is a source-derived correctness and portability finding, not an executed reproduction. [Extractor][extractor] [Producer][producer] [Focus][focus]

The review read public `theRebelliousNerd/codenerd` main at `be5e24a2812bfff1e4deb328f43b9cadeef60d30`. No builds, tests, application paths, gates, model calls, experiments, dependency installs, credential access or settings changes were performed. Existing tests were read only. The proposed fixture, ordering outcomes and evaluations below are unrun. No repository files were changed.

The proposed correction is small in scope: preserve each module clause of a supported plain `import` statement, independently of its local alias. It is not a request to add another agent, replace Mangle scheduling, execute imported project code, or declare a complete Python dependency analyzer.

## Contract and production route

Root guidance assigns orchestration and planning to Mangle, with Go supplying measured facts. The recurse mission specification likewise preserves dependency order as recurse policy's responsibility. That split requires the import producer to retain the source relationships on which the executive relies. [Root guidance][guidance] [Mission boundary][mission]

The existing code promises that focused subsystems include their dependencies. `SubsystemNode.DependsOn` names nodes that must sweep first; `FilterDAG` recursively retains precisely those recorded edges. These are implemented consumers of the graph, not a new aspirational requirement invented by this audit. [Configuration][config] [Node contract and focus][focus]

The source chain is:

1. `DeriveWorkspaceDAG` collects admitted source files and calls `scanPythonImports`. The collector includes ordinary `.py` files after workspace and project-document admission. [Derivation][derive] [Collection][collect]
2. `scanPythonImports` first creates a directory node for every admitted Python source. It then obtains import targets, resolves each to a known Python directory, and adds an edge. File/node discovery therefore succeeds independently of statement extraction. [Producer][producer]
3. `pyImport` accepts a module name followed by zero or more comma/module pairs. It has no `as identifier` production and is not anchored to the end of the statement. At the first alias it can return a successful prefix match. `pythonImportTargets` splits only that captured prefix on commas and does not inspect the remaining text. [Extractor][extractor]
4. Planning calls `FilterDAG` after graph ordering; a live pass independently derives the graph, validates its structural order, and applies the same filter. Neither path re-parses the omitted statement suffix. [Plan][plan] [Pass][pass]
5. Graph validation rejects empty/duplicate IDs, dangling dependencies and self-edges. It cannot identify an absent source relationship. Mangle blocks a node only on supplied unvisited dependencies and selects the least identity rank among ready nodes. Correct reasoning over the supplied graph cannot restore an import the producer discarded. [Graph facts][facts] [Sweep policy][policy]

Python's language reference allows an alias on each comma-separated module clause. Each clause performs its own import; the alias changes the binding name, not whether subsequent modules are imported. This supports the witness's intended prerequisite set, without requiring a custom importer, `from` syntax, wildcard import or dynamic execution. [Python language reference][python]

## Minimal conditional witness, not executed

Assume a workspace whose ordinary membership and project policy admit these files:

```text
aa_app/__init__.py        empty
aa_app/main.py            import mm_seen as seen, zz_support
mm_seen/__init__.py       empty
zz_support/__init__.py    empty
```

The single source line ends with an ordinary newline. All files are readable, short ASCII text. Assume normal root-based imports, no shadowing, generated files, symlinks, hidden directories, excluded packages or other language graph. The two imported names denote the visible regular packages. Nothing in either prerequisite imports the other or the consumer. These conditions isolate plain-import clause extraction from the already documented resolution and acquisition gaps.

### Source-predicted producer behavior

All three directories become nodes. The matcher consumes `import mm_seen`, then stops because the following `as seen` does not match a comma/module pair. Since there is no end-of-statement requirement, the prefix is accepted. The captured text is only `mm_seen`, so splitting it produces one target. The ordinary resolver maps that target to `mm_seen`, and the graph receives `aa_app -> mm_seen`. It never receives `aa_app -> zz_support`. [Producer][producer] [Extractor][extractor] [Resolver][resolver]

This is not a failure to discover the `zz_support` directory. The missing package remains a valid node with a valid Python file. Nor is it a failed read or scanner overflow: the problematic statement is a short line that is reached and partially recognized.

### Focused consequence

For a focus on `aa_app`, `FilterDAG` retains `aa_app` and `mm_seen`. There is no recorded path to `zz_support`, so that node is left outside the focused sweep. The consumer is not dependency-closed relative to this ordinary import statement. [Focus][focus]

### Full-sweep consequence

With no focus, the full derived order still includes the missing node. Initially `mm_seen` and `zz_support` are ready, and `mm_seen` has the smaller lexical identity. After it is visited, `aa_app` becomes ready because its only recorded prerequisite is satisfied. Its identity precedes `zz_support`, so the policy's next-node rule selects the consumer before the omitted prerequisite. The closing cross-cutting nodes do not supply the missing direct relation. This is a deduction from the pinned producer and policy, not an observed kernel evaluation. [Graph facts][facts] [Sweep policy][policy] [Cross-cutting close][close]

### Controls that discriminate this defect

- `import mm_seen, zz_support` has no intermediate alias and fits the current comma matcher.
- Two separate statements, `import mm_seen as seen` followed by `import zz_support`, give the scanner a fresh match on the next line.
- `import mm_seen, zz_support as support` places the alias after the last desired module, so both module names are captured before the stopping point.
- `import mm_seen as zz_support` must import only `mm_seen`; an alias must never become a dependency merely because its spelling happens to match a local package.

The first three controls vary alias placement and statement layout while keeping the relevant imported module set. They are proposed fixtures, not executed results.

## What this does and does not duplicate

The current audit inventory, queue and directly relevant Python reports were inspected at the pinned revision. A bounded GitHub code search for `pyImport` in audits returned the module-file and package-initializer reports; searches for Python aliases identified the existing Python reports but no dedicated plain-import alias-suffix witness. Search indexing and exact-term searches are not a proof that every historical discussion was examined.

| Existing report | Its failure boundary | Why this witness remains distinct |
|---|---|---|
| Python from-import prerequisites | The names after `from ... import` are not retained. | There is no `from` statement. The separate `pyImport` branch loses a later ordinary module clause after `as`. |
| Python module-file prerequisites | A correctly extracted module name is not mapped to its file-owning directory. | Both names refer directly to regular package directories that the current resolver can map. One is never supplied to it. |
| Python package-initializer prerequisites | One deepest owner is retained instead of all executable package ancestors. | Imports are top-level packages, with no ancestor initializer chain to reconstruct. |
| Python import-scan completion | Scanner failure after an oversized physical line is returned as a successful prefix. | The entire file and line are read without the stated limit condition; a grammar prefix is silently accepted. |

These reports already establish valuable parser/coverage and prerequisite principles. Reuse their shared remedy rather than implementing another scanner. In particular, the from-import report recommends retaining aliases and structured syntax; that is design overlap worth crediting, not a reason to claim the concrete plain-import path is already fixed. This report adds the discriminating supported-grammar witness and its focus/order consequences. [From-import audit][prior-from] [Module-file audit][prior-module] [Initializer audit][prior-init] [Scan-completion audit][prior-scan]

The authored Python graph test includes ordinary multiple imports (`import os, sys`) and qualified/relative `from` forms. It does not include an aliased first clause followed by a second local module. Reading that test establishes its fixture shape, not test-suite success. [Existing fixture][test]

## Bounded proposed contract

**For each supported plain Python import statement, graph acquisition must preserve the module name of every clause. Aliases are local binding metadata, not dependency identities or end markers.**

The host should provide one typed observation per module clause, bound to the source file/revision and the adapter's declared syntax coverage. A minimal first slice can cover ordinary single-line absolute module clauses with optional aliases. Whether a Go-side tokenizer, existing parser or independently bounded Python AST helper is appropriate is an implementation decision. Do not import or execute workspace modules merely to discover their names.

The implementation should either completely account for a statement within that supported slice or disclose that the statement was not completely interpreted. A successful prefix is not complete acquisition. Keep source read/scan failures distinct from grammar limitations, following the existing scan-completion proposal. A correction must not claim that fixing this production also solves module-file ownership, ancestor initializers, conditional imports or arbitrary runtime import hooks.

Resolution still needs to respect membership and protected-path exclusions. Knowing that a real import exists does not authorize editing an excluded prerequisite. Preserve an explicit unresolved or excluded relation when that matters to graph completeness; do not widen a user's scope silently. Mangle should continue deriving focus/order/admission consequences from the measured relationships and coverage. This finding does not establish that changing protected safety policy is necessary.

Attach the repair at `pythonImportTargets` and `scanPythonImports`, then demonstrate its effects through `RecurseSweepOrder` and the live-pass graph admission path. A regex-only change could close this witness, but must be tested against comments, string literals, malformed statements and later clauses so it does not trade missing edges for invented ones. A parser replacement should state its supported Python versions and failure behavior rather than presenting parser availability as a complete runtime import model. [Extractor][extractor] [Producer][producer] [Plan][plan] [Pass][pass]

## Costs and failure modes to measure

- **Acquisition cost:** for total scanned bytes B and import clauses I, a bounded linear extractor should target O(B + I) work. The graph already stores directory-level edges; retaining another valid clause adds only its target observation and any distinct edge. This is a design target, not a measured bound for a proposed implementation.
- **Parser deployment:** invoking a Python helper can introduce interpreter/version availability, subprocess lifetime and environment costs. Running an external interpreter per file would amplify startup overhead. A Go tokenizer avoids that particular dependency but owns grammar and lexical-state maintenance.
- **False positives:** aliases named like packages, quoted examples, comments, duplicate imports and conditionally executed imports must not silently become stronger execution claims than the adapter supports.
- **Scope growth:** newly recovered dependencies can increase focused work, SCC size and downstream model/gate cost. That is sometimes necessary for truthful closure, but must be surfaced rather than marketed as free throughput.
- **Failure handling:** unreadable files, parser rejection, limits and missing tooling must not publish a complete empty or prefix graph. Preserve the earlier coverage work rather than adding a second incompatible status scheme.
- **Compatibility:** preserve valid no-alias lists, aliases on final clauses, relative/from controls, external packages, exclusions, duplicate-edge deduplication and existing cycle collapse.

## Fair evaluation plan, entirely unrun

Freeze expected module names and dependency edges independently of the implementation under test. Use the language grammar and manually reviewed fixture expectations for the small deterministic cases. A separately bounded AST-based oracle can supplement these expectations without importing project code; it is not an oracle for runtime path resolution or module side effects.

| Case | Required observation |
|---|---|
| Exact four-file witness | Both local prerequisite edges are retained; focused `aa_app` includes both; both precede the consumer in the derived order. |
| First, middle and last clause aliased in lists of two and three modules | Every source module survives, independently of alias position. |
| Several clauses each aliased | All source modules survive; aliases do not replace them. |
| Alias spelling equals an unrelated local package | No edge to the alias-named package. |
| Equivalent separate-line controls | Same supported dependency set as the comma form. |
| Third-party first clause aliased, local later clause | External classification of the first clause does not erase a later local prerequisite. |
| Comments or strings containing import-like text | No invented executable import relationship from those contents. |
| Malformed/truncated statement | Explicit unsupported/incomplete disposition, not an authoritative valid prefix. |
| Existing from/module/initializer/large-line fixtures | Preserve their distinct disposition; do not mark those audits resolved by this change alone. |
| Excluded prerequisite | Preserve exclusion and disclose the closure limitation; no unauthorized scope expansion. |

Test the extractor first, then `DeriveWorkspaceDAG`, focus closure and `RecurseSweepOrder`, then a disposable production-path fixture with fake work execution. Assert the actual emitted graph and selected order, not just the number of nodes or an error-free return. Include the fixture with and without focus, and reverse lexical package names so accidentally favorable ordering cannot hide the missing edge.

Compare the pinned implementation and each proposed approach on the same snapshots, language versions and resource budgets. Report missed/extra module clauses, missed/extra graph edges, focused-closure precision/recall, ordering violations, explicit incomplete outcomes, CPU/wall time, peak memory and any subprocess count. Count rejected/incomplete inputs in the denominator. Keep parser correctness separate from subsequent model repair success. No model-quality, adoption-rate, cost saving or runtime failure-rate result is established by this review.

**Exit criterion:** ordinary supported aliased comma-import lists preserve all source module clauses through acquisition and the real planning/pass graph consumers, with no alias-derived fake dependency, no silent partial success and no weakening of scope protections. A wider claim of Python portability requires the existing resolution, initializer and scan-completion contracts to be independently satisfied.

## Pinned sources

[guidance]: https://github.com/theRebelliousNerd/codenerd/blob/be5e24a2812bfff1e4deb328f43b9cadeef60d30/agents.md#L1-L24
[mission]: https://github.com/theRebelliousNerd/codenerd/blob/be5e24a2812bfff1e4deb328f43b9cadeef60d30/Docs/architecture/campaign/MISSION-IMPROVEMENT.md#L217-L235
[config]: https://github.com/theRebelliousNerd/codenerd/blob/be5e24a2812bfff1e4deb328f43b9cadeef60d30/internal/campaign/recurse_plan.go#L12-L23
[derive]: https://github.com/theRebelliousNerd/codenerd/blob/be5e24a2812bfff1e4deb328f43b9cadeef60d30/internal/campaign/recurse_workspace.go#L64-L103
[collect]: https://github.com/theRebelliousNerd/codenerd/blob/be5e24a2812bfff1e4deb328f43b9cadeef60d30/internal/campaign/recurse_workspace.go#L264-L317
[producer]: https://github.com/theRebelliousNerd/codenerd/blob/be5e24a2812bfff1e4deb328f43b9cadeef60d30/internal/campaign/recurse_workspace.go#L461-L481
[extractor]: https://github.com/theRebelliousNerd/codenerd/blob/be5e24a2812bfff1e4deb328f43b9cadeef60d30/internal/campaign/recurse_workspace.go#L456-L510
[resolver]: https://github.com/theRebelliousNerd/codenerd/blob/be5e24a2812bfff1e4deb328f43b9cadeef60d30/internal/campaign/recurse_workspace.go#L513-L544
[focus]: https://github.com/theRebelliousNerd/codenerd/blob/be5e24a2812bfff1e4deb328f43b9cadeef60d30/internal/campaign/recurse_dag.go#L15-L77
[plan]: https://github.com/theRebelliousNerd/codenerd/blob/be5e24a2812bfff1e4deb328f43b9cadeef60d30/internal/campaign/recurse_plan.go#L38-L68
[facts]: https://github.com/theRebelliousNerd/codenerd/blob/be5e24a2812bfff1e4deb328f43b9cadeef60d30/internal/campaign/recurse_plan.go#L81-L133
[pass]: https://github.com/theRebelliousNerd/codenerd/blob/be5e24a2812bfff1e4deb328f43b9cadeef60d30/internal/campaign/recurse_cycle.go#L515-L546
[policy]: https://github.com/theRebelliousNerd/codenerd/blob/be5e24a2812bfff1e4deb328f43b9cadeef60d30/internal/core/defaults/policy/recurse.mg#L420-L446
[close]: https://github.com/theRebelliousNerd/codenerd/blob/be5e24a2812bfff1e4deb328f43b9cadeef60d30/internal/campaign/recurse_workspace.go#L237-L260
[test]: https://github.com/theRebelliousNerd/codenerd/blob/be5e24a2812bfff1e4deb328f43b9cadeef60d30/internal/campaign/recurse_workspace_test.go#L112-L133
[prior-from]: https://github.com/theRebelliousNerd/codenerd/blob/be5e24a2812bfff1e4deb328f43b9cadeef60d30/Docs/audits/recurse-python-from-import-prerequisites.md
[prior-module]: https://github.com/theRebelliousNerd/codenerd/blob/be5e24a2812bfff1e4deb328f43b9cadeef60d30/Docs/audits/recurse-python-module-file-prerequisites.md
[prior-init]: https://github.com/theRebelliousNerd/codenerd/blob/be5e24a2812bfff1e4deb328f43b9cadeef60d30/Docs/audits/recurse-python-package-initializer-prerequisites.md
[prior-scan]: https://github.com/theRebelliousNerd/codenerd/blob/be5e24a2812bfff1e4deb328f43b9cadeef60d30/Docs/audits/recurse-python-import-scan-completion.md
[python]: https://docs.python.org/3.14/reference/simple_stmts.html#import
