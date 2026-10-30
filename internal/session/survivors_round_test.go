package session

import (
	"strings"
	"testing"
)

// The survivors reach the model, and a turn with none, or whose pin gate did
// not pass, pays for no round.
func TestAdviseOnSurvivors(t *testing.T) {
	survivors := []string{"a.go line 574, !eIsMethod forced false", "a.go line 588, eIsMethod forced true"}
	passed := BuildVerification{Ran: true, OK: true, Outcome: VerifyPassed}
	failed := BuildVerification{Ran: true, OK: false, Outcome: VerifyFailed}
	for _, tc := range []struct {
		name      string
		pin       BuildVerification
		survivors []string
		wantRound bool
	}{
		{"survivors of a passing gate", passed, survivors, true},
		{"no survivors", passed, nil, false},
		{"a failed gate already handed them to its repair", failed, survivors, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			e := &Executor{config: DefaultExecutorConfig()}
			e.config.WorkspaceRoot = t.TempDir()
			trp := &recordingToolResults{}
			result := &ExecutionResult{WriteToolEffects: 1, PinCheck: tc.pin, PinAdvisory: tc.survivors}
			if _, err := e.adviseOnSurvivors(advisoryLoopCtx(t, e), trp, "sys", writingTurnHistory(), nil, nil, result); err != nil {
				t.Fatalf("adviseOnSurvivors: %v", err)
			}
			if (trp.calls == 1) != tc.wantRound {
				t.Fatalf("rounds = %d, want round=%v", trp.calls, tc.wantRound)
			}
			if tc.wantRound {
				for _, s := range survivors {
					if !strings.Contains(trp.sentText, s) {
						t.Errorf("the round does not name %q:\n%s", s, trp.sentText)
					}
				}
			}
		})
	}
}
