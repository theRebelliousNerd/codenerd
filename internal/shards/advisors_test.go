package shards

import "testing"

func TestGetStrategicAdvisorsFor(t *testing.T) {
	t.Parallel()

	advisors := GetStrategicAdvisorsFor("coder")
	if advisors == nil {
		t.Error("expected non-nil advisors slice")
	}
}

func TestShouldConsultBeforeExecution(t *testing.T) {
	t.Parallel()

	cases := []struct {
		executor   string
		complexity string
		name       string
	}{
		{"coder", "high", "coder_high"},
		{"coder", "low", "coder_low"},
		{"unknown", "high", "unknown_high"},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			if tc.executor == "unknown" && ShouldConsultBeforeExecution(tc.executor, tc.complexity) {
				t.Errorf("an unclassified executor must not consult")
			}
			if tc.complexity == "low" && ShouldConsultBeforeExecution(tc.executor, tc.complexity) {
				t.Errorf("a low-complexity task must not consult")
			}
		})
	}
}
