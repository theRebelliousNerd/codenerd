package main

import (
	"context"
	"fmt"

	"codenerd/internal/articulation"
	"codenerd/internal/campaign"
	"codenerd/internal/types"
)

// CampaignJITProvider adapts the PromptAssembler to the campaign.PromptProvider interface.
// This avoids circular dependencies while enabling JIT prompts for campaign roles.
type CampaignJITProvider struct {
	assembler *articulation.PromptAssembler
}

func (p *CampaignJITProvider) GetPrompt(ctx context.Context, role campaign.CampaignRole, campaignID string) (string, error) {
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

