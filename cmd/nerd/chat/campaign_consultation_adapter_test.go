package chat

import (
	"context"
	"errors"
	"testing"

	"codenerd/internal/campaign"
	"codenerd/internal/shards"
)

type partialFailSpawner struct{}

func (partialFailSpawner) SpawnConsultation(_ context.Context, specialistName, _ string) (string, error) {
	if specialistName == "tester" {
		return "", errors.New("tester failed")
	}
	return "ADVICE:\nadvice from " + specialistName + "\n\nCONFIDENCE: 80\n", nil
}

func TestCampaignConsultationAdapter_PartialFailureReturnsResponsesAndError(t *testing.T) {
	manager := shards.NewConsultationManager(partialFailSpawner{})
	provider := newCampaignConsultationProvider(manager)
	if provider == nil {
		t.Fatal("expected non-nil provider")
	}

	responses, err := provider.RequestBatchConsultation(context.Background(), campaign.BatchConsultRequest{
		Question:   "how to test?",
		TargetSpec: []string{"coder", "tester"},
	})
	if err == nil {
		t.Fatal("expected non-nil error when tester fails")
	}
	if len(responses) != 1 {
		t.Fatalf("expected 1 converted response, got %d (err=%v)", len(responses), err)
	}
	if responses[0].Advice == "" {
		t.Fatal("expected non-empty advice in converted response")
	}
}
