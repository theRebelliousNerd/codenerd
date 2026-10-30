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
//	                             + test-impact file facts (file_package,
//	                               is_test_file, file_imports)
//	                             + imports_project_file
//	import-graph measurement   import_cycle_member                    when edges change
//	Cartographer / deep scan   code_defines/code_calls + data flow     on demand (/scan --deep)
//	(reserved, no writer)      symbol_defined/referenced/diagnostics   never
//	CodeDOM scope (session)    active_file, code_element, file_in_scope session lifetime
//	                             + is_test_function (rides with code_element)
//	session run (the focus)    code_element + element_* for the file a     per turn,
//	                           turn is looking at (codedom_facts.go)        re-parsed on edit
//	git scanner                git_history, churn_rate, current_user   when HEAD or the author moves
//
// The rule the matrix encodes: a writer may wipe a predicate only if the same
// pass re-asserts it. A fast scan re-asserts scanner-owned predicates, so it
// replaces them wholesale; it does NOT re-derive deep, LSP, git or scope facts,
// so wiping those (which the old WorldPredicates replace-set did) destroyed
// them with nothing to restore them — one `/scan` silently erased the entire
// deep call graph and every LSP diagnostic until a deep scan happened to run
// again.
//
// Scanner is the sole authority for topology, symbols and imports. Boot,
// explicit scans and post-write refreshes share its incremental cache; no
// resident shard runs a second full scan or competes to publish these facts.

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
	// Treatment coverage (treatment_enforce.go): the member files an
	// operator-confirmed treatment kept the scan from reading. Re-derived
	// from the walk by every scan, so a scope that stops being enforced
	// loses its rows.
	"file_metadata_only",
	"treatment_scope",
	// Test-impact file facts (test_impact_facts.go): re-derived by every
	// full scan and per changed file by every delta scan. SessionScope
	// must never claim these: the scope replace-set in
	// internal/core/virtual_store.go wipes by predicate globally, so a
	// dual listing would delete world knowledge on every open_file.
	"file_package",
	"is_test_file",
	"file_imports",
	// Per-file import-graph measurement (import_cycles.go), emitted with
	// the file's resolved edges.
	"imports_project_file",
	// Review measurements (review_metrics.go), one parse per non-test Go file.
	"file_line_count",
	"cyclomatic_complexity",
	"nesting_depth",
	"detected_security_flow",
	"placeholder_site",
}

// ImportGraphPredicates are whole-graph measurements of the import edges
// (import_cycles.go). Nothing persists them: ApplyIncrementalResult
// re-measures them from the kernel's dependency_link whenever a scan changes
// the edges, and the boot path measures them from the cached world it loads.
var ImportGraphPredicates = []string{
	"import_cycle_member",
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

// LSPPredicates are reserved for language-server projections. The projection
// that was meant to write them (lsp.Manager) was deleted unwired on
// 2026-09-25; language-server diagnostics now reach the critic as text instead.
// The reservation stays so a future writer's facts are not deleted by a scan,
// which cannot re-derive them.
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
	"is_test_function",
}

// GitPredicates are emitted by the git scanner (repo_facts.go) and replaced
// whole whenever the repository's history moves. current_user is the author
// identity of the workspace's git config, spelled the way git_history records
// authors, so a rule can tell "my commit" from "someone else's".
var GitPredicates = []string{
	"git_history",
	"churn_rate",
	"current_user",
}

// OwnerGroup is one row of the ownership matrix: a writer, what its
// ownership means for a scan, and the predicates it owns.
type OwnerGroup struct {
	Name       string
	Note       string
	Predicates []string
}

// OwnerGroups is the ownership matrix as data: the one table every listing of
// world predicates is built from (WorldPredicates, `nerd world predicates`),
// so a predicate cannot be a world predicate without an owner.
var OwnerGroups = []OwnerGroup{
	{"scanner (replaced on every full scan)", "re-derived by every fast scan", ScannerPredicates},
	{"import graph", "measured from the import edges whenever a scan changes them", ImportGraphPredicates},
	{"deep / Cartographer", "replaced per file by a deep scan only", DeepPredicates},
	{"lsp", "projected by language servers; a scan must not delete these", LSPPredicates},
	{"session scope", "ephemeral, session-lifetime", SessionScopePredicates},
	{"git", "replaced whole when the repository history moves", GitPredicates},
}

// WorldPredicates enumerates every EDB predicate produced by the world model,
// across all writers. Use it to recognise world facts (e.g. when filtering a
// fact stream); do NOT use it as a replace-set — see ScannerReplaceSet.
var WorldPredicates = ownedPredicates(OwnerGroups)

func ownedPredicates(groups []OwnerGroup) []string {
	var out []string
	for _, g := range groups {
		out = append(out, g.Predicates...)
	}
	return out
}

// ScannerReplaceSet is the set a full scan may clear before loading its
// results: exactly the predicates that scan re-derives.
func ScannerReplaceSet() map[string]struct{} {
	return predicateSet(ScannerPredicates)
}

// SnapshotGlobalPredicates are whole-snapshot derivations that a delta scan
// re-derives in full and must therefore drop first: asserting the new majority
// language beside the old one leaves project_language ambiguous and every rule
// reading it non-deterministic.
//
// entry_point is deliberately NOT here even though it is snapshot-derived. It
// is attributed per file, so a changed file's entry_point is retracted with the
// rest of that file's facts; wiping the whole relation on every delta would
// instead drop entry points that only AST evidence proves (a `func main` in a
// file not named main.go) for every file the delta did not happen to re-parse.
var SnapshotGlobalPredicates = []string{
	"directory",
	"project_language",
}

// TreatmentPredicates are the scanner-owned rows describing enforced
// treatment scopes. Every scan, including an unchanged one, replaces them
// whole; they are never persisted in the world cache.
var TreatmentPredicates = []string{
	"file_metadata_only",
	"treatment_scope",
}

func predicateSet(preds []string) map[string]struct{} {
	m := make(map[string]struct{}, len(preds))
	for _, p := range preds {
		m[p] = struct{}{}
	}
	return m
}
