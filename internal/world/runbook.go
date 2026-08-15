package world

// ScanRunbook is the operator-facing description of what a scan does, which
// facts it owns, and how to tell a stale world from a broken one.
//
// It lives in the world package rather than in the CLI because the CLI, the
// chat /scan help text and the docs all describe the same behaviour and had
// drifted apart; the package that implements the behaviour is the only place
// that can keep the description true. Render it into `nerd scan --help` (see
// cmd/nerd/cmd_world.go) and into chat help.
const ScanRunbook = `WORLD SCAN RUNBOOK

WHAT A SCAN DOES
  fast scan   walks the workspace, hashes files, parses them for symbols and
              imports, and asserts: file_topology, directory, file_dir,
              test_file_for, symbol_graph, dependency_link, entry_point,
              project_language.
  deep scan   (chat: /scan --deep) additionally runs the Cartographer over Go,
              Python, TypeScript, JavaScript and Rust files for code_defines,
              code_calls and the data-flow predicates.

WHICH COMMAND TO USE
  nerd scan          full scan; replaces the scanner-owned predicates wholesale
                     and repersists the snapshot to .nerd/knowledge.db.
  chat /scan         incremental: only changed, added and deleted files are
                     re-parsed; the previous facts for those files are retracted
                     first. project_language and directory are single-valued
                     snapshot derivations and are recomputed from scratch every
                     time; entry_point is re-derived per file.
  chat /scan --deep  incremental, then hydrate deep facts for in-scope files.

OWNERSHIP (who may delete what)
  A scan replaces only what a scan re-derives. Deep facts, LSP-projected facts
  (symbol_defined, symbol_referenced, code_diagnostic, symbol_completion),
  session scope facts (active_file, code_element, file_in_scope) and git facts
  (git_history, churn_rate) survive a scan untouched, because nothing in that
  pass would put them back.

PATH IDENTITY
  Every fact identifies a file by its workspace-relative, forward-slash path.
  If you see an absolute path in a fact, something bypassed canonicalization and
  that fact will join nothing.

WHEN THE WORLD LOOKS STALE
  1. chat /scan and check the reported changed/new/deleted counts. Zero counts
     with real edits on disk means change detection is not seeing them.
  2. Check the FileCache line in the world log: "FileCache[incremental]: hits=…
     misses=… hitRate=…". A 0% hit rate on an unchanged tree means the cache is
     not matching (its manifest lives at .nerd/cache/manifest.json); a 100% hit
     rate while files changed means mtimes are not moving.
  3. Delete .nerd/cache/manifest.json to force a cold rescan. This is always
     safe: the manifest is a hash cache, not a source of truth.
  4. Facts that survive a full scan and should not have are, by definition, not
     scanner-owned — look at the owner in internal/world/world_predicates.go.

COSTS
  A scan is a local read: it asserts no cost facts and needs no permission. Deep
  scans are the expensive part and are on demand only.`
