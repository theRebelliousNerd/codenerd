package core

import (
	"context"

	"codenerd/internal/logging"
	"codenerd/internal/tools"
)

// installToolFactSink wires tool completions into the kernel as
// tool_execution/3 facts.
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

// toolFactSink builds the closure that asserts one fact per completed
// execution. Refusals never reach it — a tool the guard blocked was not run,
// and recording it as an execution would corrupt the reliability counters that
// read this predicate.
func (v *VirtualStore) toolFactSink() tools.FactSink {
	return func(_ context.Context, toolName string, success bool, _ int64, unixSeconds int64) {
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
		if err := kernel.Assert(Fact{
			Predicate: "tool_execution",
			Args:      []any{toolName, boolToAtom(success), unixSeconds},
		}); err != nil {
			// Fact emission is observational. A failure here must not fail the
			// tool call that already succeeded.
			logging.Get(logging.CategoryVirtualStore).Debug(
				"tool_execution fact for %s not asserted: %v", toolName, err)
		}
	}
}
