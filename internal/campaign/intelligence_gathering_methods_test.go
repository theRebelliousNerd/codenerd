package campaign

import (
	"context"
	"errors"
	"strings"
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

// Advice longer than the old 200-character cut has to reach the planner.
// Confidence 0.55 is above the summary's 0.5 and below the expert section's
// 0.6, so formatIntelligenceContext can only be showing the summary.
func TestGatherShardAdviceRendersAdviceWhole(t *testing.T) {
	advice := strings.Repeat("review the caller before editing. ", 12)
	if len(advice) <= 200 {
		t.Fatalf("advice length %d is not past the old cut", len(advice))
	}
	provider := &stubPartialConsultationProvider{
		responses: []ConsultationResponse{{FromSpec: "coder", Advice: advice, Confidence: 0.55}},
	}
	g := &IntelligenceGatherer{consultation: provider, config: DefaultIntelligenceConfig()}
	report := &IntelligenceReport{}
	g.gatherShardAdvice(context.Background(), report, "test goal", func(string) {})

	if !strings.Contains(report.AdvisorySummary, advice) {
		t.Fatalf("advisory summary cut the advice: %q", report.AdvisorySummary)
	}
	formatted := formatIntelligenceContext(report)
	if !strings.Contains(formatted, advice) {
		t.Fatalf("formatIntelligenceContext cut the advice:\n%s", formatted)
	}
}
