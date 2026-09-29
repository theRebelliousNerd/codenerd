---
doc-class: governance
subsystem: workspace
implementation-status: not-applicable
last-verified: 2026-09-29
verified-against: 4dded472+working-tree
supersedes: []
---

# Standing invariants and open edges

Preserve one membership authority, SkipDir before rejected descent, ordinary hidden-directory membership and the single configuration default. These are grounded in Admit/alwaysExcluded and DefaultWorldConfig (`internal/workspace/membership.go:415`, `internal/workspace/membership.go:540`, `internal/config/world.go:41`).

| Edge | Current position | Remaining question |
|---|---|---|
| Submodules/nested repositories | Git boundary paths may be members, but their children and directory traversal are refused; Files omits them (`loadSnapshot`, `IncludesDir`, `internal/workspace/git.go:98`, `internal/workspace/membership.go:292`). | Any child-workspace traversal needs an explicit scope contract; do not silently recurse. |
| Aliases/symlinks | Root aliases share a cache; membership of file symlink entries stays lexical; Walk does not follow child symlink directories (`canonicalRoot`, `relOf`, `walk`, `internal/workspace/membership.go:118`, `internal/workspace/membership.go:429`, `internal/workspace/walk.go:46`). | Full junction/8.3 behavior across every caller still needs native integration evidence. |
| Dynamic Git requests | Sibling batches use finite subprocesses and a serialized result cache, not a persistent pipe (`askBatch`, `checkIgnore`, `internal/workspace/membership.go:477`, `internal/workspace/git.go:247`). | If measured process cost justifies a persistent driver or request timeout, configuration/lifecycle ownership must be specified first. |
| Nested ignore edits | Explicit refresh invalidates known-member truth; dynamic unknown paths query live ignores (`Refresh`, `Includes`, `internal/workspace/membership.go:184`, `internal/workspace/membership.go:254`). | No periodic watcher for every nested ignore file is claimed. |
| Ripgrep parity | Native Git-ignore traversal plus member-hit filtering is used (`Search`, `keepMemberHits`, `internal/retrieval/backend.go:77`, `internal/retrieval/backend.go:148`). | Extra-exclusion traversal and force-tracked ignored-file parity are not proved. |
| Semantic treatment | Membership API has no Treatment implementation (query surfaces, `internal/workspace/membership.go:254`, `internal/workspace/membership.go:292`). | Orientation owner must provide the derived artifact and its inheritance/exclusion contract (GAP-WS-05). |