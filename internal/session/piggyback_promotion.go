package session

import (
	"strings"

	"codenerd/internal/articulation"
	"codenerd/internal/types"
)

// promotePiggybackToolRequests wires valid control_packet.tool_requests from a
// native LLM response that carries zero native ToolCalls back into the ordinary
// tool loop.
//
// Precedence: if the response already has native ToolCalls, promotion is skipped
// so native calls are never duplicated. A response the host already admitted
// (ControlsApplied) holds display text, never an envelope, and is never
// promoted.
//
// Side effects: the Piggyback envelope's surface/control side effects
// (mangle_updates, memory ops, self-correction, context feedback) are applied
// exactly once, from the one parse made here. The response Text becomes the
// surface and the response is marked ControlsApplied, which is what keeps the
// task tail from reading that surface as an envelope again. The mark, not a
// hope that the surface happens to contain no JSON, is the guarantee: an
// answer explaining the Piggyback format can hold a well-formed envelope.
// Controls that did not take effect are carried to result as notices. This
// preserves offered-tool/permission/history/write-accounting/budget gates
// because promotion only populates ToolCalls; execution still goes through
// executeToolBatch which enforces isToolAllowed, budget, safety, and write
// accounting.
//
// Returns true when promotion happened.
func (e *Executor) promotePiggybackToolRequests(resp *types.LLMToolResponse, result *ExecutionResult) bool {
	if resp == nil || len(resp.ToolCalls) != 0 || resp.ControlsApplied {
		return false
	}
	// Use AllowPlain so both raw JSON envelopes and text-wrapped envelopes are
	// handled.
	processed := articulation.ProcessLLMResponseAllowPlain(resp.Text)
	if processed.Control == nil || len(processed.Control.ToolRequests) == 0 {
		return false
	}
	calls := e.parseToolRequestsFromControl(processed.Control)
	if len(calls) == 0 {
		return false
	}
	// Filter to valid calls (non-empty name). Keep every valid request;
	// permission/budget gates are authoritative at execution time, not here.
	valid := make([]types.ToolCall, 0, len(calls))
	for _, c := range calls {
		if strings.TrimSpace(c.Name) == "" {
			continue
		}
		// ID may be empty in hand-crafted envelopes; keep it empty so
		// executeToolBatch pairing still works, or synthesize a stable one.
		valid = append(valid, c)
	}
	if len(valid) == 0 {
		return false
	}
	// Apply control side effects exactly once, from this parse, even when the
	// envelope is consumed as tool calls rather than as terminal prose.
	surface, dispositions := e.applyPiggybackControls(processed)
	carryControlNotices(result, dispositions)
	// Both views, in one call. Writing resp.Text and resp.ToolCalls directly
	// would leave a block-carrying response holding the original envelope text
	// and no tool_use blocks, so the assistant turn appended to history would
	// claim it called nothing while the next user turn answered calls that are
	// not in the transcript.
	resp.Rewrite(surface, valid)
	resp.ControlsApplied = true
	return true
}
