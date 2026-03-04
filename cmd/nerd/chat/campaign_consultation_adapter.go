package chat

import (
	"context"
	"fmt"
	"strings"

	"codenerd/internal/campaign"
	"codenerd/internal/shards"
)

// campaignConsultationProviderAdapter adapts shards.ConsultationManager to campaign.ConsultationProvider.
type campaignConsultationProviderAdapter struct {
	manager *shards.ConsultationManager
}

func newCampaignConsultationProvider(manager *shards.ConsultationManager) campaign.ConsultationProvider {
	if manager == nil {
		return nil
	}
	return &campaignConsultationProviderAdapter{manager: manager}
}

func (a *campaignConsultationProviderAdapter) RequestBatchConsultation(ctx context.Context, request campaign.BatchConsultRequest) ([]campaign.ConsultationResponse, error) {
	if a == nil || a.manager == nil {
		return nil, fmt.Errorf("consultation manager not configured")
	}

	question := strings.TrimSpace(request.Question)
	if topic := strings.TrimSpace(request.Topic); topic != "" {
		if question == "" {
			question = topic
		} else {
			question = "[" + topic + "] " + question
		}
	}

	targets := request.TargetSpec
	if len(targets) == 0 {
		targets = []string{"coder", "tester", "reviewer", "researcher"}
	}

	responses, err := a.manager.RequestBatchConsultation(ctx, question, request.Context, targets)
	if err != nil {
		return nil, err
	}

	converted := make([]campaign.ConsultationResponse, 0, len(responses))
	for _, resp := range responses {
		converted = append(converted, campaign.ConsultationResponse{
			RequestID:    resp.RequestID,
			FromSpec:     resp.FromSpec,
			ToSpec:       resp.ToSpec,
			Advice:       resp.Advice,
			Confidence:   resp.Confidence,
			References:   resp.References,
			Caveats:      resp.Caveats,
			Metadata:     resp.Metadata,
			ResponseTime: resp.ResponseTime,
			Duration:     resp.Duration,
		})
	}

	return converted, nil
}
