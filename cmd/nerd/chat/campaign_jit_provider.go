package chat

import (
	"context"
	"fmt"

	"codenerd/internal/articulation"
	"codenerd/internal/campaign"
	"codenerd/internal/types"
)

// campaignJITProvider adapts the PromptAssembler to campaign.PromptProvider.
// Defined in chat package to avoid circular deps.
type campaignJITProvider struct {
	assembler *articulation.PromptAssembler
}

func (p *campaignJITProvider) GetPrompt(ctx context.Context, role campaign.CampaignRole, campaignID string) (string, error) {
	if p == nil || p.assembler == nil {
		return "", fmt.Errorf("prompt assembler not initialized")
	}

	shardType := campaign.GetShardTypeForRole(role)
	pc := &articulation.PromptContext{
		ShardID:    fmt.Sprintf("%s_%s", campaignID, role),
		ShardType:  shardType,
		CampaignID: campaignID,
		SessionCtx: &types.SessionContext{
			CampaignActive: true,
			CampaignPhase:  campaign.GetCampaignPhaseForRole(role),
		},
	}

	return p.assembler.AssembleSystemPrompt(ctx, pc)
}

