package core

import (
	"testing"

	"codenerd/internal/types"
)

// A critical campaign task that exhausts its attempts is put to the user:
// escalation_required (campaign_rules.mg) derives next_action(/escalate_to_user).
// The rule sat in policy/verification.mg until 2026-09-23 and moved to
// campaign_rules.mg when that file's verification loop -- whose inputs nothing
// asserted -- was deleted; it was the one rule there a live input reached.
func TestCampaign_AnExhaustedCriticalTaskEscalatesToTheUser(t *testing.T) {
	for _, tc := range []struct {
		priority string
		want     bool
	}{
		{"/critical", true},
		{"/normal", false},
	} {
		t.Run(tc.priority, func(t *testing.T) {
			k := setupMockKernel(t)
			mustAssert(t, k, "config_param", types.MangleAtom("/campaign_max_task_attempts"), int64(2))
			mustAssert(t, k, "task_priority", "task-1", types.MangleAtom(tc.priority))
			mustAssert(t, k, "task_attempt", "task-1", int64(1), types.MangleAtom("/failure"), int64(100))
			mustAssert(t, k, "task_attempt", "task-1", int64(2), types.MangleAtom("/failure"), int64(200))

			if !queryDerived(t, k, "task_exhausted") {
				t.Fatal("the fixture needs task_exhausted: two failures against a cap of two")
			}
			if got := queryDerived(t, k, "escalation_required"); got != tc.want {
				t.Errorf("escalation_required derived = %v, want %v", got, tc.want)
			}
			if got := queryDerived(t, k, "next_action(/escalate_to_user)"); got != tc.want {
				t.Errorf("next_action(/escalate_to_user) derived = %v, want %v", got, tc.want)
			}
		})
	}
}
