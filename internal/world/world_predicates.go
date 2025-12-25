package world

// WorldPredicates enumerates EDB predicates produced by the world model.
// Used to safely replace world facts in the kernel.
var WorldPredicates = []string{
	"file_topology",
	"directory",
	"symbol_graph",
	"dependency_link",
	// Holographic / deep predicates
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
	// LSP-derived predicates (code intelligence)
	"symbol_defined",
	"symbol_referenced",
	"code_diagnostic",
	"symbol_completion",
}

// WorldPredicateSet returns a map form for fast membership checks.
func WorldPredicateSet() map[string]struct{} {
	m := make(map[string]struct{}, len(WorldPredicates))
	for _, p := range WorldPredicates {
		m[p] = struct{}{}
	}
	return m
}

