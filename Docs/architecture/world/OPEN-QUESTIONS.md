# world — Open Questions

> Last verified: **2026-07-13**

## Q1 — Single owner of world EDB freshness?

Chat incremental sync and `WorldModelIngestorShard` both can write topology/symbols. Should one be deprecated, or is dual-mode intentional (session vs background)?

## Q2 — Should deep map stay Go-first forever?

Multi-lang dataflow and CodeDOM exist; Cartographer deep symbols do not. Is the product bet “Go monorepo first” or incomplete polyglot?

## Q3 — Absolute paths in LocalStore keys?

Even if EDB uses relative paths, DB may key by absolute path from walk. Should store keys always be relative to workspace root?

## Q4 — Scope facts vs WorldPredicates

Are `active_file` / `code_element` intentionally ephemeral (session-only) and thus correctly excluded from full replace? Document as contract if so.

## Q5 — Holographic prompt assembly location

Should holographic formatting migrate to `internal/prompt` atoms for JIT, or remain world-owned structured context that articulation formats?

## Q6 — dependency_link future

Is import-graph emission planned on the Scanner path, or only via FileScope outbound maps never projected?

## Q7 — Tree-sitter vs go/ast long-term for Go

Three Go parsers coexist (tree-sitter scan, go/ast Cartographer, go/ast CodeDOM). Acceptable redundancy for speed vs precision, or consolidate?

## Q8 — Constitutional boundary for scan

Is workspace scan considered a free observation, or should large scans assert cost/fuel facts for policy?

## Q9 — LSP multi-language priority

Is Mangle-only LSP sufficient for north star (policy as logic), with language intelligence remaining AST-based?

## Q10 — Test dependency builder production wire

`TestDependencyBuilder` implements codedom interface — is it always constructed in tool paths, or partially dormant?
