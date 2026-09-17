package main

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"testing"

	"codenerd/internal/campaign"
	"codenerd/internal/shards"
)

// partialFailureConsultationSpawner succeeds for "coder" and fails for "tester".
type partialFailureConsultationSpawner struct{}

func (s *partialFailureConsultationSpawner) SpawnConsultation(_ context.Context, specialistName, _ string) (string, error) {
	if specialistName == "tester" {
		return "", errors.New("tester failed")
	}
	return fmt.Sprintf("ADVICE: advice from %s\nCONFIDENCE: 0.8\n", specialistName), nil
}

func TestCampaignConsultationProviderAdapterConsultPartialFailure(t *testing.T) {
	mgr := shards.NewConsultationManager(&partialFailureConsultationSpawner{})
	adapter := newCampaignConsultationProvider(mgr)
	if adapter == nil {
		t.Fatal("expected non-nil adapter")
	}

	responses, err := adapter.RequestBatchConsultation(context.Background(), campaign.BatchConsultRequest{
		Question:   "how to proceed",
		Context:    "test context",
		TargetSpec: []string{"coder", "tester"},
	})

	if err == nil {
		t.Fatal("expected non-nil error when one specialist fails")
	}
	if len(responses) != 1 {
		t.Fatalf("expected 1 converted response, got %d (err=%v)", len(responses), err)
	}
	if !strings.Contains(responses[0].Advice, "coder") {
		t.Fatalf("expected surviving response to carry coder advice, got %q", responses[0].Advice)
	}
}
