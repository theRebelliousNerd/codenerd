package system

import (
	"testing"

	"codenerd/internal/types"
)

// The session executor picks its tool channel by asking the client it holds
// whether it speaks Piggyback. What it holds is the whole boot stack under the
// session adapter, and each of those layers has to hand the question down: a
// Codex or Claude CLI engine has no channel but the envelope, and a stack that
// answered "no" for it sent every tool turn down the native path, which failed
// on its first continuation ("does not implement ToolResultsProvider"). The
// broker and the tracer never forwarded it, and neither did this adapter; the
// scheduler did, to a tracer that could not answer.
func TestSessionAdapterReportsAnEnvelopeOnlyEngineThroughTheProductionChain(t *testing.T) {
	_, chain := productionChain(t)
	adapter := &sessionLLMAdapter{client: chain}

	if _, ok := any(adapter).(types.ToolResultsProvider); !ok {
		t.Fatal("the session adapter no longer claims ToolResultsProvider; this test's premise changed")
	}
	ptp, ok := any(adapter).(types.PiggybackToolProvider)
	if !ok {
		t.Fatal("the session adapter does not answer the Piggyback question, so the executor " +
			"takes the native tool path for every client")
	}
	if !ptp.ShouldUsePiggybackTools() {
		t.Fatal("a Codex CLI engine reports no Piggyback through the production chain " +
			"(session adapter -> scheduler -> tracer -> broker); its tool turns take the native " +
			"path, which it cannot serve")
	}
}
