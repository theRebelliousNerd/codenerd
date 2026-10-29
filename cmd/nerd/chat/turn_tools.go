package chat

import (
	"codenerd/internal/logging"
	"codenerd/internal/prompt"
)

// turnTools is this turn's tool catalog: the kernel derivation
// prompt.DeriveTurnTools for the turn's verb (commit 34c5eac8), the same one
// the session executor and the spawner compile against.
//
// Every prompt compiled for a turn that acts on tools sets AvailableTools
// from exactly one call here, for the same verb the context claims. The
// selector drops every atom whose requires_tools are not all in
// AvailableTools (internal/prompt/atoms.go atomToolSatisfied), so a compile
// that skips this loses all tool-gated guidance — CodeDOM, structure
// queries — silently: that is the failure commit a7e584e5 fixed for the
// spawner, which had compiled with AvailableTools unset. Deriving once per
// site means the prompt and the offered tools cannot drift.
//
// The chat model holds no catalog of its own to reuse: articulation goes
// through CompleteWithSystem plus the piggyback envelope, and /tool list is
// display, not an LLM catalog. The kernel derivation is the one list.
//
// A failed derivation fails closed — the compile runs against none — the
// same as the session executor's resolveAvailableTools. No tools is not
// all tools.
func (m Model) turnTools(verb string) []string {
	if m.kernel == nil {
		return nil
	}
	tools, err := prompt.DeriveTurnTools(m.kernel, verb)
	if err != nil {
		logging.Get(logging.CategorySession).Warn(
			"Tool envelope resolution failed for %q: %v (compiling with empty catalog)", verb, err)
		return nil
	}
	return tools
}
