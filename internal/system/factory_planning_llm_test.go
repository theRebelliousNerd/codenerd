package system

import (
	"testing"

	"codenerd/internal/perception"
)

// Campaign planning runs on the planner slot config.json configures, else on
// main; the worker slot never plans.
func TestCortexPlanningLLM(t *testing.T) {
	main, worker, planner := &MockLLMClient{}, &MockLLMClient{}, &MockLLMClient{}
	tests := []struct {
		name   string
		cortex *Cortex
		want   perception.LLMClient
	}{
		{name: "nil cortex", cortex: nil, want: nil},
		{
			name:   "planner configured",
			cortex: &Cortex{LLMClient: main, WorkerLLMClient: worker, PlannerLLMClient: planner},
			want:   planner,
		},
		{
			name:   "no planner falls back to main",
			cortex: &Cortex{LLMClient: main, WorkerLLMClient: worker},
			want:   main,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.cortex.PlanningLLM(); got != tt.want {
				t.Fatalf("PlanningLLM() = %p, want %p", got, tt.want)
			}
		})
	}
}
