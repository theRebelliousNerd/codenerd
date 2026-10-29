package broker

import "testing"

func TestPromptBudgetFor(t *testing.T) {
	tests := []struct {
		name        string
		window      int
		inputWindow int
		want        int
	}{
		{name: "no client cap keeps the ledger share", window: 200000, inputWindow: 0, want: int(float64(200000-8000) * 0.75)},
		{name: "a larger client cap changes nothing", window: 200000, inputWindow: 1000000, want: int(float64(200000-8000) * 0.75)},
		{name: "a smaller client cap bounds the share", window: 200000, inputWindow: 40000, want: 30000},
		{name: "the floor still holds under a tiny cap", window: 200000, inputWindow: 8000, want: minPromptBudget},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			meter := testMeter(tt.window, 8000)
			if got := meter.PromptBudgetFor(0.75, 120000, tt.inputWindow); got != tt.want {
				t.Fatalf("PromptBudgetFor() = %d, want %d", got, tt.want)
			}
		})
	}
}
