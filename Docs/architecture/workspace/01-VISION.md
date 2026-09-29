---
doc-class: north-star
subsystem: workspace
implementation-status: target-state
last-verified: 2026-09-29
verified-against: e056692c
supersedes: []
---

# 01 — Vision — The Single Workspace Membership Authority

## The North Star of Workspace Membership

In the finished state of codeNERD, **exactly one authority** in the codebase answers whether a file or directory belongs to the active workspace: package `internal/workspace`.

Every tool, scanner, indexer, search routine, and campaign component queries this single authority. No component keeps its own private list of ignored directory names. No scanner maintains hardcoded skip tables that fight with user configuration. When codeNERD runs in a foreign repository, it sees precisely what an expert human engineer sees: the repository's genuine, tracked source files and explicitly admitted untracked assets, strictly obeying the repository's `.gitignore` rules.

---

## 1. Core Capabilities in the Target State

1. **Absolute `.gitignore` Compliance**:
   codeNERD honors all `.gitignore` semantics: directory-level rules, recursive `**` wildcards, leading slash anchors, file extension patterns, and `!` negation overrides. If git ignores a directory, codeNERD never enters it.
2. **Git Version Control as Ground Truth**:
   In a git repository, membership is anchored in git's own index:
   - A single, streaming invocation of `git ls-files -z -co --exclude-standard` constructs the baseline member snapshot.
   - Member directories are defined structurally as the ancestors of member files.
   - Newly created, modified, or unstaged files created during an agent turn are evaluated dynamically through a batched `git check-ignore -z --stdin` process and cached.
3. **User-Owned Extra Exclusions**:
   The user's `world.ignore_patterns` configuration is applied on top of git truth as an extra exclusion layer. If an engineer wants codeNERD to ignore a large, tracked directory (such as a legacy benchmark or vendored binary tree), adding it to `world.ignore_patterns` drops it from the workspace with genuine `**` globbing and directory prefix semantics.
4. **Semantic Discernment Integration**:
   `internal/workspace` integrates seamlessly with `internal/orient`. It ingests the derived `.nerd/orientation/membership.json` overlay, exposing an operational `Treatment(rel string)` API. Tools can distinguish production source files (`/index_and_parse`) from golden answer keys and massive seed datasets (`/index_names_only`), ensuring non-code files are indexed by path but never parsed into CodeDOM AST models or embedded into working memory.
5. **Universal Walker Adoption**:
   Every filesystem walker across the entire system — `internal/world/fs.go`, `incremental_scan.go`, `structure_index.go`, `retrieval/sparse.go`, `tools/core/search.go`, `tools/core/file_ops.go`, `campaign/recurse_workspace.go`, and `cmd/nerd/chat` — delegates membership decisions to `internal/workspace`. Private skip lists are completely eliminated from the codebase.
6. **High Performance and Concurrency Safety**:
   The `Membership` instance is cached per canonical workspace root and protected by read-write mutexes. Walkers check `IncludesDir(rel)` before descending; a `false` verdict immediately returns `filepath.SkipDir`, preventing the operating system from statting or reading ignored directories.

---

## 2. Why Single Membership Matters to the codeNERD Vision

### "It knows the codebase better than any other coding agent because it never has to grep around"
When an agent walks 100,000 files in a repository where only 20,000 files constitute real code, every subsequent subsystem degrades. The world model's SHA-256 scanner burns CPU and disk I/O on virtual environments and cache directories; search tools return false matches in test fixture corpora; CodeDOM memory balloons to gigabytes; and the model's context window is polluted with disposable build output. Anchoring membership in git truth guarantees that codeNERD's search space is identical to the developer's real working tree.

### "The harness decides, not the model's discretion"
In standard agents, the LLM must constantly be told in its prompt: "Do not search in `.venv` or `node_modules`". When the model forgets, it wastes tool turns reading minified bundles or python bytecode. In codeNERD, the harness decides. The deterministic workspace authority blocks access to ignored files at the boundary. The model cannot waste turns on ignored trees because those trees do not exist in the member fact graph.

### "Clean fixpoint, not clean loop"
Having 35 different walkers each computing their own ad-hoc ignore heuristics in Go is the quintessential anti-pattern codeNERD was built to eradicate. In the target state, membership is derived from a single declarative authority whose rules are consistent, testable, and deterministic.
