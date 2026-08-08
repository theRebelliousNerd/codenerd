package session

import "testing"

func TestSummarizeTurnSignals(t *testing.T) {
	cases := []struct {
		name      string
		built     bool
		tested    bool
		uncovered int
		findings  int
		want      string
	}{
		{
			name:      "all ok no uncovered no findings",
			built:     true,
			tested:    true,
			uncovered: 0,
			findings:  0,
			want:      "build ok | tests ok | 0 uncovered | 0 findings",
		},
		{
			name:      "all ok with uncovered and singular finding",
			built:     true,
			tested:    true,
			uncovered: 3,
			findings:  1,
			want:      "build ok | tests ok | 3 uncovered | 1 finding",
		},
		{
			name:      "build failed tests ok",
			built:     false,
			tested:    true,
			uncovered: 0,
			findings:  0,
			want:      "build FAILED | tests ok | 0 uncovered | 0 findings",
		},
		{
			name:      "build ok tests failed",
			built:     true,
			tested:    false,
			uncovered: 1,
			findings:  2,
			want:      "build ok | tests FAILED | 1 uncovered | 2 findings",
		},
		{
			name:      "both failed",
			built:     false,
			tested:    false,
			uncovered: 2,
			findings:  5,
			want:      "build FAILED | tests FAILED | 2 uncovered | 5 findings",
		},
		{
			name:      "singular finding",
			built:     true,
			tested:    true,
			uncovered: 0,
			findings:  1,
			want:      "build ok | tests ok | 0 uncovered | 1 finding",
		},
		{
			name:      "multiple findings",
			built:     true,
			tested:    true,
			uncovered: 0,
			findings:  3,
			want:      "build ok | tests ok | 0 uncovered | 3 findings",
		},
		{
			name:      "single uncovered single finding both ok",
			built:     true,
			tested:    true,
			uncovered: 1,
			findings:  1,
			want:      "build ok | tests ok | 1 uncovered | 1 finding",
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := SummarizeTurnSignals(tc.built, tc.tested, tc.uncovered, tc.findings)
			if got != tc.want {
				t.Errorf("SummarizeTurnSignals(%v, %v, %d, %d) = %q; want %q", tc.built, tc.tested, tc.uncovered, tc.findings, got, tc.want)
			}
		})
	}
}
