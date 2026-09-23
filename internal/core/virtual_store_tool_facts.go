package core

import (
	"context"

	"codenerd/internal/logging"
	"codenerd/internal/tools"
	"codenerd/internal/usage"
)

// installToolFactSink wires tool completions into the kernel as
// tool_execution/3 facts, and the code elements a call edited as
// element_modified / modified_function / modified.
//
// schemas_tools.mg has declared tool_execution(ToolName, Success, Timestamp)
// for a long time, but nothing ever asserted it: the registry had no way to
// reach the kernel (internal/tools must not import internal/core), so the
// predicate sat declared and permanently empty. Any rule that wanted to reason
// about what the agent had actually run — retry policy, tool reliability,
// learning from repeated failure — had nothing to read.
//
// The sink is a plain function for the same reason WriteGuard is: it lets the
// registry stay a leaf while the kernel side lives here.
func (v *VirtualStore) installToolFactSink(registries ...*tools.Registry) {
	sink := v.toolFactSink()
	for _, r := range registries {
		if r != nil {
			r.SetFactSink(sink)
		}
	}
}

// toolFactSink builds the closure that asserts the facts of one completed
// execution. Refusals never reach it — a tool the guard blocked was not run,
// and recording it as an execution would corrupt the reliability counters that
// read this predicate.
func (v *VirtualStore) toolFactSink() tools.FactSink {
	return func(ctx context.Context, rec tools.ExecutionRecord) {
		if v == nil {
			return
		}
		v.mu.RLock()
		kernel := v.kernel
		v.mu.RUnlock()
		if kernel == nil {
			return
		}

		// Success is declared /name, not a bare bool, so it has to be the atom
		// form the Decl bounds — a Go bool would be rejected on assert.
		facts := []Fact{{
			Predicate: "tool_execution",
			Args:      []any{rec.ToolName, boolToAtom(rec.Success), rec.UnixSeconds},
		}}
		if rec.Success {
			facts = append(facts, editedElementFacts(usage.SessionIDFromContext(ctx), rec.UnixSeconds, rec.Edits)...)
		}
		for _, f := range facts {
			if err := kernel.Assert(f); err != nil {
				// Fact emission is observational. A failure here must not fail
				// the tool call that already succeeded.
				logging.Get(logging.CategoryVirtualStore).Debug(
					"%s fact for %s not asserted: %v", f.Predicate, rec.ToolName, err)
			}
		}
	}
}

// editedElementFacts are the facts a model edit produces: the same data the
// Path-B handler handleEditElement asserts, so the impact chain
// (impact.mg: modified_function joined with code_calls), the CodeDOM edit
// policy (element_modified) and run_impacted_tests read model edits exactly as
// they read kernel-dispatched ones.
//
// element_modified carries the world model's ref form (fn:pkg.Recv.Name,
// struct:pkg.Name, ...), because code_element and the test-dependency builder
// are keyed by it; the model-facing directory-keyed refs are the tools' own.
func editedElementFacts(sessionID string, unixSeconds int64, edits []tools.EditedElement) []Fact {
	var facts []Fact
	files := make(map[string]bool)
	for _, e := range edits {
		if e.File != "" && !files[e.File] {
			files[e.File] = true
			facts = append(facts, Fact{Predicate: "modified", Args: []any{e.File}})
		}
		if e.Language != "go" || e.Name == "" || e.Kind == "header" || e.Kind == "syntax_error" {
			continue
		}
		elem := &CodeElement{Ref: worldRefOf(e), Type: e.Kind}
		facts = append(facts, Fact{Predicate: "element_modified", Args: []any{elem.Ref, sessionID, unixSeconds}})
		facts = append(facts, modifiedSymbolFacts(elem, e.File)...)
	}
	return facts
}

// worldRefOf is the ref world.GoCodeParser gives an element (go_parser.go
// buildRef): a kind prefix, the package name, the receiver and the name.
func worldRefOf(e tools.EditedElement) string {
	prefix := e.Kind
	switch e.Kind {
	case "function", "method":
		prefix = "fn"
	}
	if e.Receiver != "" {
		return prefix + ":" + e.Package + "." + e.Receiver + "." + e.Name
	}
	return prefix + ":" + e.Package + "." + e.Name
}
