package session

import (
	"fmt"
	"os/exec"
	"sync/atomic"
	"time"

	"codenerd/internal/types"
)

// Test conveniences over the executor's live entry points. They lived in
// production files with no production caller.

// checkSafety verifies a tool call against the Constitutional Gate.
func (e *Executor) checkSafety(call ToolCall) bool {
	ok, _ := e.checkSafetyWithGate(call, e.configSnapshot().EnableSafetyGate)
	return ok
}

// assertPendingEdit preserves the single-target test and caller seam.
func (e *Executor) assertPendingEdit(call ToolCall) (types.Fact, bool) {
	facts := e.assertPendingEdits(call)
	if len(facts) == 0 {
		return types.Fact{}, false
	}
	return facts[0], true
}

var subagentCounter uint64

// DefaultSubAgentConfig returns an ephemeral subagent's identity. It sets no
// clock and no turn cap: a subagent runs under its caller's context (the
// user's --timeout, when set) and stops when the working policy derives a
// stall. Until 2026-09-23 it set a 30-minute wall clock and a 100-task cap
// (the spawner, which had dropped its clock on 2026-09-19, kept the cap).
func DefaultSubAgentConfig(name string) SubAgentConfig {
	return SubAgentConfig{
		ID:   fmt.Sprintf("%s-%d-%d", name, time.Now().UnixNano(), atomic.AddUint64(&subagentCounter, 1)),
		Name: name,
		Type: SubAgentTypeEphemeral,
	}
}

// execLookPathForTest exposes the PATH lookup so tests can skip cleanly when
// gopls is not installed, without duplicating the lookup logic.
func execLookPathForTest(name string) (string, error) { return exec.LookPath(name) }
