package usage

import "testing"

func TestTokenCounts_Add(t *testing.T) {
	tests := []struct {
		name       string
		initial    TokenCounts
		addInput   int
		addOutput  int
		wantInput  int64
		wantOutput int64
		wantTotal  int64
	}{
		{
			name:       "Add to zero counts",
			initial:    TokenCounts{Input: 0, Output: 0, Total: 0},
			addInput:   10,
			addOutput:  20,
			wantInput:  10,
			wantOutput: 20,
			wantTotal:  30,
		},
		{
			name:       "Add to existing counts",
			initial:    TokenCounts{Input: 100, Output: 200, Total: 300},
			addInput:   50,
			addOutput:  60,
			wantInput:  150,
			wantOutput: 260,
			wantTotal:  410,
		},
		{
			name:       "Add zero values",
			initial:    TokenCounts{Input: 50, Output: 50, Total: 100},
			addInput:   0,
			addOutput:  0,
			wantInput:  50,
			wantOutput: 50,
			wantTotal:  100,
		},
		{
			name:       "Add negative values (mathematical completeness)",
			initial:    TokenCounts{Input: 100, Output: 100, Total: 200},
			addInput:   -10,
			addOutput:  -20,
			wantInput:  90,
			wantOutput: 80,
			wantTotal:  170,
		},
		{
			name:       "Large values",
			initial:    TokenCounts{Input: 1000000, Output: 2000000, Total: 3000000},
			addInput:   500000,
			addOutput:  1000000,
			wantInput:  1500000,
			wantOutput: 3000000,
			wantTotal:  4500000,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tc := tt.initial
			tc.Add(tt.addInput, tt.addOutput)

			if tc.Input != tt.wantInput {
				t.Errorf("Input = %v, want %v", tc.Input, tt.wantInput)
			}
			if tc.Output != tt.wantOutput {
				t.Errorf("Output = %v, want %v", tc.Output, tt.wantOutput)
			}
			if tc.Total != tt.wantTotal {
				t.Errorf("Total = %v, want %v", tc.Total, tt.wantTotal)
			}
		})
	}
}
