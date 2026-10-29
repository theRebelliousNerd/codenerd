package campaign

import (
	"context"
	"errors"
	"strings"
	"testing"

	"codenerd/internal/consultation"
)

// stubBatchProvider returns a fixed typed batch.
type stubBatchProvider struct {
	batch consultation.Batch
}

func (s *stubBatchProvider) RequestBatchConsultation(_ context.Context, _ consultation.BatchRequest) consultation.Batch {
	return s.batch
}

func succeeded(spec, advice string, confidence float64) consultation.Entry {
	return consultation.Entry{
		Specialist: spec,
		Status:     consultation.EntrySucceeded,
		Response:   &consultation.Response{Specialist: spec, Advice: advice, Raw: advice, Confidence: confidence},
	}
}

func TestGatherShardAdviceKeepsPartialResultsOnError(t *testing.T) {
	provider := &stubBatchProvider{batch: consultation.Batch{Entries: []consultation.Entry{
		succeeded("coder", "ship it", 0.9),
		{Specialist: "tester", Status: consultation.EntryFailed, Err: errors.New("tester failed"), Retryable: true},
	}}}
	g := &IntelligenceGatherer{consultation: provider, config: DefaultIntelligenceConfig()}
	report := &IntelligenceReport{}
	var gatheringErrors []string
	addError := func(s string) { gatheringErrors = append(gatheringErrors, s) }

	g.gatherShardAdvice(context.Background(), report, "test goal", addError)

	if len(report.ShardAdvice) != 1 {
		t.Fatalf("expected 1 ShardAdvice entry, got %d", len(report.ShardAdvice))
	}
	if report.ShardAdvice[0].Specialist != "coder" {
		t.Fatalf("expected coder advice, got %q", report.ShardAdvice[0].Specialist)
	}
	if len(gatheringErrors) != 1 || !strings.Contains(gatheringErrors[0], "tester") {
		t.Fatalf("expected one gathering error naming tester, got %v", gatheringErrors)
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
	provider := &stubBatchProvider{batch: consultation.Batch{Entries: []consultation.Entry{
		succeeded("coder", advice, 0.55),
	}}}
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
