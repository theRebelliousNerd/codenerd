package world

// World EDB ownership matrix.
//
// Several independent writers assert facts into the world EDB. Without a stated
// owner per predicate, "replace everything the world model knows" is impossible
// to implement correctly, and the previous single flat list made
// ApplyIncrementalResult delete facts that its caller could not re-add:
//
//	writer                     owns                                   cadence
//	------                     ----                                   -------
//	Scanner (full/incremental) topology, symbols, imports, entry pts   every scan
//	Cartographer / deep scan   code_defines/code_calls + data flow     on demand (/scan --deep)
//	lsp.Manager                symbol_defined/referenced/diagnostics   on LSP index
//	CodeDOM scope (session)    active_file, code_element, file_in_scope session lifetime
//	git scanner                git_history, churn_rate                 on demand
//	WorldModelIngestorShard    file_topology, symbol_graph (background) shard lifetime
//
// The rule the matrix encodes: a writer may wipe a predicate only if the same
// pass re-asserts it. A fast scan re-asserts scanner-owned predicates, so it
// replaces them wholesale; it does NOT re-derive deep, LSP, git or scope facts,
// so wiping those (which the old WorldPredicates replace-set did) destroyed
// them with nothing to restore them — one `/scan` silently erased the entire
// deep call graph and every LSP diagnostic until a deep scan happened to run
// again.
//
// The WorldModelIngestorShard writes two predicates the Scanner also owns
// (file_topology, symbol_graph). Resolution (also recorded in
// Docs/architecture/world/OPEN-QUESTIONS.md Q1): the Scanner is the sole
// authority. The shard's copies are background refreshes of the same facts and
// must use the same canonical identity; where they disagree, the next scan wins
// because the scan replaces the predicate wholesale. Nothing here can stop the
// shard from writing — enforcement lives at the one choke point this package
// owns, ApplyIncrementalResult — so the matrix is asserted as a test
// (world_predicates_test.go) rather than as a hope.

// ScannerPredicates are the predicates a fast scan (full or incremental)
// re-derives from scratch on every pass. These, and only these, are safe to
// replace wholesale when a full scan result is applied.
var ScannerPredicates = []string{
	"file_topology",
	"directory",
	"file_dir",
	"test_file_for",
	"symbol_graph",
	"dependency_link",
	"entry_point",
	"project_language",
}

// DeepPredicates are produced by the Cartographer (EnsureDeepFacts) and the
// data-flow extractors. They are replaced per-file, keyed by fingerprint, by
// the deep scan itself — never by a fast scan.
var DeepPredicates = []string{
	"code_defines",
	"code_calls",
	"assigns",
	"guards_return",
	"guards_block",
	"guard_dominates",
	"safe_access",
	"uses",
	"call_arg",
	"error_checked_return",
	"error_checked_block",
	"function_scope",
}

// LSPPredicates are projected by lsp.Manager from language servers. A scan
// cannot re-derive them, so a scan must not delete them.
var LSPPredicates = []string{
	"symbol_defined",
	"symbol_referenced",
	"code_diagnostic",
	"symbol_completion",
}

// SessionScopePredicates are owned by the CodeDOM scope for the lifetime of a
// session. They are deliberately ephemeral: they describe what the user is
// looking at, not what is on disk, and are excluded from every replace-set.
var SessionScopePredicates = []string{
	"active_file",
	"file_in_scope",
	"code_element",
}

// GitPredicates are emitted by the git scanner on demand.
var GitPredicates = []string{
	"git_history",
	"churn_rate",
}

// WorldPredicates enumerates every EDB predicate produced by the world model,
// across all writers. Use it to recognise world facts (e.g. when filtering a
// fact stream); do NOT use it as a replace-set — see ScannerReplaceSet.
var WorldPredicates = concatPredicates(
	ScannerPredicates,
	DeepPredicates,
	LSPPredicates,
	SessionScopePredicates,
	GitPredicates,
)

// WorldPredicateSet returns a map form for fast membership checks.
func WorldPredicateSet() map[string]struct{} {
	return predicateSet(WorldPredicates)
}

// ScannerReplaceSet is the set a full scan may clear before loading its
// results: exactly the predicates that scan re-derives.
func ScannerReplaceSet() map[string]struct{} {
	return predicateSet(ScannerPredicates)
}

// SnapshotGlobalPredicates are whole-snapshot derivations (not per-file), so an
// incremental scan re-derives the complete set each pass and the stale set must
// be dropped first — asserting the new majority language beside the old one
// leaves project_language ambiguous, and every rule reading it non-deterministic.
var SnapshotGlobalPredicates = []string{
	"directory",
	"project_language",
	"entry_point",
}

func concatPredicates(groups ...[]string) []string {
	total := 0
	for _, g := range groups {
		total += len(g)
	}
	out := make([]string, 0, total)
	for _, g := range groups {
		out = append(out, g...)
	}
	return out
}

func predicateSet(preds []string) map[string]struct{} {
	m := make(map[string]struct{}, len(preds))
	for _, p := range preds {
		m[p] = struct{}{}
	}
	return m
}
