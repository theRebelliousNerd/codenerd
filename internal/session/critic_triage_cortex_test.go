package session_test

import (
	"testing"

	"codenerd/internal/core"
	"codenerd/internal/types"
)

// TestCriticTriage_MatchesOnShardedKernel is the production shape of the
// critic's triage: the finding rows are homed on the cortex shard beside
// turn_gate, and severity_rank is corpus ground facts, so the triage must
// derive identically on the sharded kernel and a single one.
func TestCriticTriage_MatchesOnShardedKernel(t *testing.T) {
	cortex := bootRetentionCortex(t)
	single, err := core.NewRealKernel()
	if err != nil {
		t.Fatal(err)
	}
	both := retentionPair{t: t, cortex: cortex, single: single}

	const (
		mix     = "/turn_triage_mix"
		lowOnly = "/turn_triage_low"
		quiet   = "/turn_triage_quiet"
	)
	both.assert(
		core.Fact{Predicate: "turn_critic_finding", Args: []any{types.MangleAtom(mix), int64(0), types.MangleAtom("/high")}},
		core.Fact{Predicate: "turn_critic_finding", Args: []any{types.MangleAtom(mix), int64(1), types.MangleAtom("/low")}},
		core.Fact{Predicate: "turn_critic_finding", Args: []any{types.MangleAtom(mix), int64(2), types.MangleAtom("/medium")}},
		core.Fact{Predicate: "turn_critic_finding", Args: []any{types.MangleAtom(mix), int64(3), types.MangleAtom("/unknown")}},
		core.Fact{Predicate: "turn_critic_finding", Args: []any{types.MangleAtom(lowOnly), int64(0), types.MangleAtom("/low")}},
		core.Fact{Predicate: "turn_critic_finding", Args: []any{types.MangleAtom(lowOnly), int64(1), types.MangleAtom("/unknown")}},
	)

	both.parity("turn_critic_finding", "turn_critic_actionable", "turn_has_critic_actionable", "turn_needs_uplift")

	both.has("turn_critic_actionable", mix, "0")
	both.has("turn_critic_actionable", mix, "2")
	both.lacks("turn_critic_actionable", mix, "1")
	both.lacks("turn_critic_actionable", mix, "3")
	both.lacks("turn_critic_actionable", lowOnly, "0")
	both.lacks("turn_critic_actionable", lowOnly, "1")
	both.has("turn_needs_uplift", mix, "")
	both.lacks("turn_needs_uplift", lowOnly, "")
	both.lacks("turn_needs_uplift", quiet, "")
}
