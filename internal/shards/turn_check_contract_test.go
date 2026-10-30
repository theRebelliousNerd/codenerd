package shards

import (
	"testing"

	"codenerd/internal/core"
)

// Uses the production ownership manifest and complete corpus on both engines.
// A receipt's declared pass set must be visible beside its raw exit, including
// the absence tests used for zero-only defaults and execution errors.
func TestCheckContract_ShardedReceiptAndLatestGateParity(t *testing.T) {
	single, err := core.NewRealKernelWithWorkspace(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	both := gatePair{t: t, cortex: bootGateCortex(t), single: single}
	for _, tc := range []struct {
		turn             string
		exit             int64
		declared, failed bool
		want             string
	}{
		{"/turn_check_declared", 3, true, false, "/passing"},
		{"/turn_check_real_failure", 1, true, false, "/failing"},
		{"/turn_check_default", 3, false, false, "/failing"},
		{"/turn_check_error", 0, true, true, "/failing"},
	} {
		both.assert(append(otherWrite(tc.turn), gf("turn_write_seq", ma(tc.turn), int64(1)))...)
		if tc.declared {
			both.assert(gf("turn_check_ok_exit", ma(tc.turn), int64(2), int64(0)),
				gf("turn_check_ok_exit", ma(tc.turn), int64(2), int64(3)))
		}
		if tc.failed {
			both.assert(gf("turn_check_error", ma(tc.turn), int64(2)))
		}
		both.assert(gf("turn_check_run", ma(tc.turn), int64(2), tc.exit))
		both.gate(tc.turn, "/check", tc.want)
		both.noGate(tc.turn, "/test_run")
	}

	// A later receipt with an omitted rule must not inherit the earlier set.
	const turn = "/turn_check_declared"
	both.assert(gf("turn_check_run", ma(turn), int64(3), int64(3)))
	both.gate(turn, "/check", "/failing")
	both.lacks("turn_check_passed", turn, "3")
	both.parity("turn_check_has_exit_rule", "turn_check_passed", "turn_gate", "turn_last_check")
}
