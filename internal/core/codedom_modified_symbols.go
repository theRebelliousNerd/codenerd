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
