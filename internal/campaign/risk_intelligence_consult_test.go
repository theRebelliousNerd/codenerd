package campaign

import (
	"context"
	"sync/atomic"
	"testing"
	"time"
)

// countingConsultation is a fake ConsultationProvider that counts calls.
type countingConsultation struct {
	calls atomic.Int32
}

func (f *countingConsultation) RequestBatchConsultation(_ context.Context, _ BatchConsultRequest) ([]ConsultationResponse, error) {
	f.calls.Add(1)
	return []ConsultationResponse{
		{FromSpec: "coder", Advice: "test advice", Confidence: 0.9},
	}, nil
}

func (f *countingConsultation) count() int {
	return int(f.calls.Load())
}

// TestGatherRiskIntelligenceSkipsConsult proves the risk snapshot does not
// call the ConsultationProvider. The risk snapshot reads no shard advice;
// consults cannot finish inside the 45s sample and their timeout inflated
// the risk score via errorNorm.
func TestGatherRiskIntelligenceSkipsConsult(t *testing.T) {
	fake := &countingConsultation{}
	gatherer := NewIntelligenceGatherer("", nil, nil, nil, nil, nil, nil, nil, fake)
	gatherer.WithConfig(IntelligenceConfig{
		GatherTimeout:    10 * time.Second,
		PerSystemTimeout: 5 * time.Second,
		ConsultTimeout:   5 * time.Second,
		// All other gather systems disabled so the test needs no kernel/world.
		EnableShardConsult: true,
	})

	if !gatherer.config.EnableShardConsult {
		t.Fatal("test setup: EnableShardConsult must start true")
	}

	o := &Orchestrator{
		campaign:             &Campaign{ID: "risk-consult-test", Goal: "test goal"},
		intelligenceGatherer: gatherer,
	}

	report := o.gatherRiskIntelligence(context.Background(), nil)
	if report == nil {
		t.Fatal("gatherRiskIntelligence returned nil report")
	}
	if got := fake.count(); got != 0 {
		t.Fatalf("gatherRiskIntelligence called ConsultationProvider %d times, want 0", got)
	}
	if !o.intelligenceGatherer.config.EnableShardConsult {
		t.Fatal("gatherRiskIntelligence mutated shared gatherer config: EnableShardConsult became false")
	}
	if len(report.ShardAdvice) != 0 {
		t.Fatalf("risk report should carry no shard advice, got %d entries", len(report.ShardAdvice))
	}

	// Sanity check: the same gatherer DOES consult on the direct path,
	// proving the fake is wired and the risk path is what skipped it.
	direct, err := gatherer.Gather(context.Background(), "test goal", nil)
	if err != nil {
		t.Fatalf("direct Gather failed: %v", err)
	}
	if got := fake.count(); got != 1 {
		t.Fatalf("direct Gather called ConsultationProvider %d times, want 1", got)
	}
	if len(direct.ShardAdvice) != 1 {
		t.Fatalf("direct Gather should return 1 shard advice, got %d", len(direct.ShardAdvice))
	}
}
