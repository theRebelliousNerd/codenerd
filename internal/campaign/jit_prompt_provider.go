package campaign

import (
	"context"
	"fmt"

	"codenerd/internal/articulation"
	"codenerd/internal/broker"
	"codenerd/internal/logging"
	"codenerd/internal/northstar"
	"codenerd/internal/perception"
	"codenerd/internal/types"
)

// systemPromptAssembler is the part of *articulation.PromptAssembler the JIT
// provider uses.
type systemPromptAssembler interface {
	AssembleSystemPrompt(ctx context.Context, input any) (string, error)
}

// jitPromptProvider assembles each role's prompt from the campaign atoms
// (internal/prompt/atoms/campaign) against the live campaign, for the model
// that will consume it.
type jitPromptProvider struct {
	assembler systemPromptAssembler
}

// NewJITPromptProvider adapts assembler to PromptProvider. It returns nil when
// there is no assembler; every campaign role then reports
// types.ErrRolePromptUnavailable instead of reaching a model.
func NewJITPromptProvider(assembler *articulation.PromptAssembler) PromptProvider {
	if assembler == nil {
		return nil
	}
	return &jitPromptProvider{assembler: assembler}
}

// GetPrompt compiles role's prompt for servedBy's provider and model, the
// fail-closed selector dimensions of the model-pinned atoms: an identity the
// client does not report is left unset, so those atoms sit out rather than
// match a model that never sees them.
func (p *jitPromptProvider) GetPrompt(ctx context.Context, role CampaignRole, campaignID string, servedBy perception.LLMClient) (string, error) {
	if _, ok := campaignRoleSpecs[role]; !ok {
		return "", fmt.Errorf("unknown campaign role %q", role)
	}
	sessionCtx := &types.SessionContext{
		CampaignActive: true,
		CampaignPhase:  GetCampaignPhaseForRole(role),
	}
	if provider, model := servingIdentity(servedBy); provider != "" || model != "" {
		sessionCtx.ExtraContext = make(map[string]string, 2)
		if provider != "" {
			sessionCtx.ExtraContext["provider"] = provider
		}
		if model != "" {
			sessionCtx.ExtraContext["model"] = model
		}
	}
	return p.assembler.AssembleSystemPrompt(ctx, &articulation.PromptContext{
		ShardID:    fmt.Sprintf("%s_%s", campaignID, role),
		ShardType:  GetShardTypeForRole(role),
		CampaignID: campaignID,
		SessionCtx: sessionCtx,
		UserIntent: &types.StructuredIntent{Verb: campaignPromptIntent},
	})
}

// servingIdentity is the provider and model client reports through
// types.ModelIdentifier on the outermost layer of its decorator chain that
// implements it, the reading the session executor's prompt pinning uses.
func servingIdentity(client perception.LLMClient) (provider, model string) {
	if northstar.IsNil(client) {
		return "", ""
	}
	broker.Walk(client, func(layer types.LLMClient) bool {
		identifier, ok := layer.(types.ModelIdentifier)
		if !ok {
			return true
		}
		provider, model = identifier.ModelIdentity()
		return false
	})
	if provider == "" && model == "" {
		logging.CampaignDebug("LLM client %T reports no model identity; model-pinned campaign atoms sit out", client)
	}
	return provider, model
}
