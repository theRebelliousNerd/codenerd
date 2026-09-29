---
doc-class: governance
subsystem: workspace
implementation-status: not-applicable
last-verified: 2026-09-29
verified-against: e056692c
supersedes: []
---

# 00 — Index — Docs/architecture/workspace

_Governance map for this directory. Per `Docs/journeys/09-architecture-doc-standard.md:50-52`: read order, one line per file stating what it answers and when to read it, followed by the grounded-vs-hypothesized map._

- Last-verified: 2026-09-29
- Verified-against: `e056692c`
- ID ownership: `03-GAP-ANALYSIS.md` owns `GAP-WS-01..05`.

## 1. Read Order (One Line Per File)

| # | File | What it answers — when to read it |
|---|------|-----------------------------------|
| 1 | `README.md` | What this directory is, why a single membership authority is required, and where truth lives — read first. |
| 2 | `00-INDEX.md` (this file) | In what order to read the corpus and which claims are grounded versus hypothesized — read second; defines governance boundaries. |
| 3 | `01-VISION.md` | The north-star vision: a single, concurrency-safe authority honoring `.gitignore`, git truth, user excludes, and discernment overlays — read before current state. |
| 4 | `02-CURRENT-STATE.md` + `IMPLEMENTED_SPEC.md` | The audit of ~35 production walkers and ~25 private ignore lists running today, cited to exact source lines — read together. |
| 5 | `WIRING-AND-NOT-BUILT.md` | The walker census: exhaustive table of every walker, its current skip list, and its exact migration plan — read before the spec. |
| 6 | `03-GAP-ANALYSIS.md` | The gap matrix (`GAP-WS-01..05`) defining severity, blocking lanes, and checkable exit criteria — read after current state. |
| 7 | `04-PRINCIPLES-AND-CONSTRAINTS.md` | Non-negotiable principles for workspace membership (one authority, git as floor, fast skip, zero hidden-dir special cases). |
| 8 | `05-MEMBERSHIP-SPEC.md` | The complete capability specification: data structures, API contracts (`For`, `Includes`, `IncludesDir`, `Walk`), caching, and non-git fallback. |
| 9 | `OPEN-QUESTIONS.md` | Unresolved design edges (submodules, symlinks, Windows junctions) and standing invariants. |
| 10 | `TODO.md` | Leaf implementation tasks directly traceable to `GAP-WS` identifiers. |
| 11 | `corpus.toml` | Machine-readable corpus metadata. |
| 12 | `adr/ADR-001-git-ls-files-and-check-ignore-as-membership-authority.md` | Governed Architectural Decision Record establishing git truth and batched check-ignore as the membership standard. |

## 2. Grounded vs Hypothesized

### 2a. GROUNDED — Code-Verified Claims (Cite Freely)

The following claims reflect current repository source code re-verified against commit `e056692c`:

- **World Scanner Exclusions**: `internal/world/fs.go:226-238` skips 6 hardcoded directory names; lines 241-263 enforce a dot-prefix allowlist permitting only `.github`, `.vscode`, `.circleci`, `.config`.
- **Incremental Scanner Discrepancy**: `internal/world/incremental_scan.go:137-153` duplicates the hidden allowlist but omits the hardcoded `node_modules` map.
- **Duplicate Default Lists**: The 13-name default ignore list is duplicated verbatim in `internal/config/world.go:33-47` and `internal/world/scanner_config.go:42-56`.
- **Factory Scanner Invariant**: `internal/system/factory.go:2239-2245` is the only production site that copies `world.ignore_patterns` onto a scanner; `nerd init` (`internal/init/initializer.go:360`) and `nerd scan` (`cmd/nerd/cmd_init_scan.go:416`) use `world.NewScanner()` with compiled defaults, silently ignoring user configurations.
- **World Model Ingestor Shard**: `internal/shards/system/world_model.go:354` (`performFullScan`) and line 429 (`performIncrementalScan`) use `RootPath "."` (line 89) and lack `filepath.SkipDir` on incremental directories (lines 430-431).
- **CodeDOM SkipDir**: `internal/world/codemodel/model.go:168-176` (`SkipDir`) skips `vendor`, `node_modules`, `testdata`, and any name starting with `.` or `_`.
- **Search Tool Skip**: `internal/tools/core/search.go:373-382` skips hidden dirs and `node_modules`, `vendor`; `file_ops.go:687-691` skips hidden files only; `shell/builtins.go:532-533` skips `.git`, `node_modules`, `vendor`.
- **Git Tracked Files Reference**: `internal/docscheck/docscheck.go:723-740` demonstrates safe execution of `git -C <abs> ls-files -z`.

### 2b. HYPOTHESIZED / TARGET-STATE — Planned Design (Mark Planned)

The following claims describe planned architecture specified in brief `L1_workspace_membership.txt` and have not yet landed in production:

- **`internal/workspace` Package**: Leaf package owning `Membership`, `For`, `Includes`, `IncludesDir`, `Walk`, `Refresh`.
- **Single Source of Truth**: All ~35 production walkers repointed to query `Membership.Includes` or `Membership.IncludesDir`.
- **Universal `.gitignore` Compliance**: Real-time evaluation of `.gitignore` patterns via `git ls-files` and batched `git check-ignore`.
- **True `**` Glob Semantics**: User `world.ignore_patterns` evaluated with real `**`, directory prefix, and `!` negation globbing.

## 3. How to Cite This Directory

- **Shipped claims**: Cite `02-CURRENT-STATE.md`, `IMPLEMENTED_SPEC.md`, or the verified source `path:line` directly.
- **Planned claims**: Cite `05-MEMBERSHIP-SPEC.md` or `GAP-WS-NN` rows from `03-GAP-ANALYSIS.md`.
- **Decisions**: Cite `adr/ADR-001` including status (`accepted-not-implemented` until witness verification resolves).
