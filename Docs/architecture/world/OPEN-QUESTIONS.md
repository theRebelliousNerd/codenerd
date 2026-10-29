# world — Open Questions

> Last verified: **2026-08-15**

## Q1 — Single owner of world EDB freshness?

Chat incremental sync and `WorldModelIngestorShard` both can write topology/symbols. Should one be deprecated, or is dual-mode intentional (session vs background)?

**ANSWERED (2026-08-15).** The Scanner is the sole authority for scanner-owned predicates. Dual
writers stay (the shard is a background refresh), but the ownership matrix in
`internal/world/world_predicates.go` states who owns what, and the rule is: a writer may replace
a predicate only if the same pass re-asserts it. A full scan therefore replaces
`ScannerPredicates` wholesale — the shard's copies of `file_topology`/`symbol_graph` lose to the
next scan — and leaves deep, LSP, session-scope and git facts alone. Enforced by
`TestApplyIncremental_WhenFullScan_ShouldNotWipeFactsItCannotRebuild`.

Residual: the shard still emits `file_topology` with a bare-string language, a bool
`IsTestFile` and second-resolution mtimes, where the Scanner emits `/go`, `/true` and
nanoseconds. Those rows do not collide with the Scanner's, they sit beside them.
`internal/shards/system/world_model.go` is outside this package; aligning it is a follow-up.

## Q2 — Should deep map stay Go-first forever?

Multi-lang dataflow and CodeDOM exist; Cartographer deep symbols do not. Is the product bet “Go monorepo first” or incomplete polyglot?

**ANSWERED (2026-08-15).** It was incomplete, not a bet. `Cartographer.MapFile` now deep-maps
Python, TypeScript, JavaScript and Rust as well as Go (`cartographer_multilang.go`), emitting
`code_defines`/`code_calls` with line ranges plus the data-flow facts that already existed.

## Q3 — Absolute paths in LocalStore keys?

Even if EDB uses relative paths, DB may key by absolute path from walk. Should store keys always be relative to workspace root?

**ANSWERED (2026-08-15).** Yes — canonical everywhere. LocalStore `world_files` rows and both
`fast` and `deep` fact rows are keyed by the canonical path. They were mixed before: the full
scan wrote canonical keys and the incremental scan read absolute ones, so the retraction lookup
never hit and superseded facts accumulated in the kernel forever. The FileCache manifest is the
one deliberate exception (machine-local, keyed by walk path); the reason is in `cache.go`.

## Q4 — Scope facts vs WorldPredicates

Are `active_file` / `code_element` intentionally ephemeral (session-only) and thus correctly excluded from full replace? Document as contract if so.

**ANSWERED (2026-08-15).** Yes, ephemeral by contract: they describe what the session is looking
at, not what is on disk. Recorded as `SessionScopePredicates` and excluded from every
replace-set.

## Q5 — Holographic prompt assembly location

Should holographic formatting migrate to `internal/prompt` atoms for JIT, or remain world-owned structured context that articulation formats?

**STILL OPEN (2026-08-15).** Unchanged: `PromptSection` remains a Go formatter. Migrating needs
atom files under `internal/prompt/atoms/`, so the decision belongs with the prompt corpus.

## Q6 — dependency_link future

Is import-graph emission planned on the Scanner path, or only via FileScope outbound maps never projected?

**ANSWERED (2026-08-15).** On the Scanner path, and now real. The walkers emitted only
`dependency_link(File, "pkg:x", "x")` — a token no rule could join against `modified`/
`pending_edit`, so the impact/activation cascade was dormant. `dependency_links.go` resolves
in-workspace imports into file→file edges. Go resolves at package granularity (an importer gets
an edge to every non-test file of the imported package: over-approximate, which is the safe
direction for a predicate that gates writes); Python/TS/JS/Rust resolve to the exact file.
External imports keep the token form. Expansion is capped (50k edges, sorted so the truncation
is identical across scanners); codeNERD itself yields ~27k.

## Q7 — Tree-sitter vs go/ast long-term for Go

Three Go parsers coexist (tree-sitter scan, go/ast Cartographer, go/ast CodeDOM). Acceptable redundancy for speed vs precision, or consolidate?

## Q8 — Constitutional boundary for scan

Is workspace scan considered a free observation, or should large scans assert cost/fuel facts for policy?

## Q9 — LSP multi-language priority

Is Mangle-only LSP sufficient for north star (policy as logic), with language intelligence remaining AST-based?

**ANSWERED (2026-08-15).** No longer Mangle-only: `lsp.Client` speaks LSP to any server over any
transport and `Manager.StartLanguageServer("/go", "gopls")` registers one. AST/tree-sitter
remains the always-available layer — a missing server binary is a plain error the caller logs and
continues past, never a failed session.

## Q10 — Test dependency builder production wire

`TestDependencyBuilder` implements codedom interface — is it always constructed in tool paths, or partially dormant?
