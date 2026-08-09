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
// so native calls are never duplicated.
//
// Side effects: the Piggyback envelope's surface/control side effects
// (mangle_updates, memory ops, self-correction, context feedback) are applied
// exactly once via processPiggybackControlPacket. The response Text is replaced
// with the surface response so a later processPiggybackControlPacket on the same
// text finds no control packet and becomes a no-op. This preserves
// offered-tool/permission/history/write-accounting/budget gates because promotion
// only populates ToolCalls; execution still goes through executeToolBatch which
// enforces isToolAllowed, budget, safety, and write accounting.
//
// Returns true when promotion happened.
func (e *Executor) promotePiggybackToolRequests(resp *types.LLMToolResponse) bool {
	if resp == nil || len(resp.ToolCalls) != 0 {
		return false
	}
	// Use AllowPlain so both raw JSON envelopes and text-wrapped envelopes are
	// handled. This mirrors processPiggybackControlPacket.
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
	// Apply control side effects exactly once. processPiggybackControlPacket
	// parses again but is idempotent after we replace Text with surface: the
	// second parse (at ProcessWithIntent tail) will find no control packet.
	// We call it here to ensure mangle_updates etc. are not lost when the
	// envelope is consumed as tool calls rather than as terminal prose.
	surface := e.processPiggybackControlPacket(resp.Text)
	// surface should equal processed.Surface; if processPiggyback found no
	// control (should not happen since we did), fall back.
	if surface == "" && processed.Surface != "" {
		surface = processed.Surface
		// Still need to ensure mangle updates are asserted if processPiggyback
		// was a no-op due to parsing variance. Use the already-parsed control.
		if len(processed.Control.MangleUpdates) > 0 {
			env := &articulation.PiggybackEnvelope{Surface: processed.Surface, Control: *processed.Control}
			e.processMangleUpdatesFromEnvelope(env)
		}
	}
	resp.Text = surface
	resp.ToolCalls = valid
	return true
}
