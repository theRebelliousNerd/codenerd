package campaign

import (
	"context"
	"errors"
	"testing"
)

type stubPartialConsultationProvider struct {
	responses []ConsultationResponse
	err       error
}

func (s *stubPartialConsultationProvider) RequestBatchConsultation(_ context.Context, _ BatchConsultRequest) ([]ConsultationResponse, error) {
	return s.responses, s.err
}

func TestGatherShardAdviceKeepsPartialResultsOnError(t *testing.T) {
	one := ConsultationResponse{FromSpec: "coder", Advice: "ship it", Confidence: 0.9}
	provider := &stubPartialConsultationProvider{
		responses: []ConsultationResponse{one},
		err:       errors.New("tester failed"),
	}
	g := &IntelligenceGatherer{consultation: provider, config: DefaultIntelligenceConfig()}
	report := &IntelligenceReport{}
	var gatheringErrors []string
	addError := func(s string) { gatheringErrors = append(gatheringErrors, s) }

	g.gatherShardAdvice(context.Background(), report, "test goal", addError)

	if len(report.ShardAdvice) != 1 {
		t.Fatalf("expected 1 ShardAdvice entry, got %d", len(report.ShardAdvice))
	}
	if report.ShardAdvice[0].FromSpec != "coder" {
		t.Fatalf("expected coder advice, got %q", report.ShardAdvice[0].FromSpec)
	}
	if len(gatheringErrors) != 1 {
		t.Fatalf("expected exactly 1 gathering error, got %d: %v", len(gatheringErrors), gatheringErrors)
	}
}
