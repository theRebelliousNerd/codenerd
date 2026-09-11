package core

// =============================================================================
// MODIFIED-SYMBOL FACTS
// =============================================================================
// impact.mg derives the whole impact chain from modified_function:
//
//	modified_function(Func, _), code_calls(Caller, Func)  -> impact_caller/2
//	impact_caller -> impact_graph/3 (depth <= 3)
//	impact_graph  -> relevant_context_file/1, context_priority_file/3
//
// and world.HolographicProvider.BuildWithImpactPriorities turns
// context_priority_file into the "Callers (impact-prioritized)" block the model
// sees. Every link of that was built and tested. None of it ever ran, because
// nothing produced modified_function.
//
// The predicate's only two Go references before 2026-09-09 were an allow-list
// entry letting the model volunteer the fact through mangle_updates
// (session/executor.go) and a shard's owned-predicate list
// (shards/registration.go). So the kernel — the executive, in a design whose
// whole premise is that logic determines reality and the model merely describes
// it — was waiting for the model to tell it which function had changed, about
// an edit the kernel had just performed itself.
//
// These facts are emitted from the CodeDOM edit handlers, where the element's
// identity is already known exactly. No parsing, no guessing, no join against a
// separately-derived line range.

import "strings"

// symbolIDFromRef converts a CodeDOM ref into the identifier the world model's
// call graph is keyed by.
//
// This has to match world.Cartographer exactly or the join in impact.mg never
// fires. The Cartographer builds both code_calls and code_defines identifiers as
// `<pkg>.<Name>` for a function and `<pkg>.<Receiver>.<Name>` for a method
// (cartographer.go:107-110). world.GoCodeParser.buildRef produces the same
// string with a kind prefix: `fn:<pkg>.<Name>` (go_parser.go:174-178). So the
// transform is to drop everything through the last ':' — and nothing else.
//
// The first version of this stripped to the last '.' as well, yielding the bare
// `Target` where code_calls holds `impactdemo.Target`. Every fact was emitted
// correctly and impact_caller still derived nothing, because the two sides of
// the join were naming the same function differently.
// TestImpactChain_EndToEndThroughVirtualStore is what caught it; no unit test
// on either side could have, because each side was self-consistent.
func symbolIDFromRef(ref string) string {
	if i := strings.LastIndex(ref, ":"); i >= 0 {
		return ref[i+1:]
	}
	return ref
}

// modifiedSymbolFacts returns the modified_function / modified_interface facts
// for an element the session just edited.
//
// Only functions, methods and interfaces produce facts: those are the two
// predicates impact.mg joins on. A struct or const edit still asserts
// element_modified and modified(File); it just does not start a caller-impact
// walk, because there is no caller relation for it to walk.
//
// file is the CANONICAL identity of elem.File (the element itself keeps the
// absolute path the scope opened): impact.mg joins these against code_calls
// and file_topology, which are canonical.
func modifiedSymbolFacts(elem *CodeElement, file string) []Fact {
	if elem == nil {
		return nil
	}
	name := symbolIDFromRef(elem.Ref)
	if name == "" || file == "" {
		return nil
	}
	switch elem.Type {
	case "function", "method":
		return []Fact{{Predicate: "modified_function", Args: []any{name, file}}}
	case "interface":
		return []Fact{{Predicate: "modified_interface", Args: []any{name, file}}}
	default:
		return nil
	}
}

// modifiedSymbolFactsForLineRange returns modified-symbol facts for every
// element of path whose line span overlaps [startLine, endLine].
//
// The line tools (edit_lines, insert_lines, delete_lines) are how a model
// usually edits, and they name a range rather than an element. Overlap is the
// honest relation: an edit that touches any line of a function has modified
// that function, and an edit that spans several has modified all of them.
//
// It must be called BEFORE the scope refresh. After the refresh the line
// numbers have moved, and for delete_lines the element may be gone entirely —
// which is exactly the case whose callers most need to be looked at.
//
// path is what the scope resolves (absolute or canonical); file is the
// canonical identity the facts carry.
func modifiedSymbolFactsForLineRange(scope CodeScope, path, file string, startLine, endLine int) []Fact {
	if scope == nil || path == "" || endLine < startLine {
		return nil
	}
	elements := scope.GetCoreElementsByFile(path)
	facts := make([]Fact, 0, 2)
	seen := make(map[string]struct{}, len(elements))
	for i := range elements {
		elem := &elements[i]
		// Disjoint iff one ends before the other starts.
		if elem.EndLine < startLine || elem.StartLine > endLine {
			continue
		}
		for _, f := range modifiedSymbolFacts(elem, file) {
			key := f.Predicate + "\x00" + elem.Ref
			if _, dup := seen[key]; dup {
				continue
			}
			seen[key] = struct{}{}
			facts = append(facts, f)
		}
	}
	return facts
}
