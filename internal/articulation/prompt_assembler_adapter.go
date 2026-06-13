package articulation

import (
	"context"
	"fmt"
	"maps"

	"codenerd/internal/types"
)

// =============================================================================
// ADAPTER FOR PERCEPTION LAYER (breaks import cycle)
// =============================================================================

// PromptAssemblerAdapter wraps PromptAssembler to implement a simplified interface
// that avoids import cycles with the perception package.
type PromptAssemblerAdapter struct {
	assembler *PromptAssembler
}

// NewPromptAssemblerAdapter creates an adapter for the PromptAssembler.
func NewPromptAssemblerAdapter(assembler *PromptAssembler) *PromptAssemblerAdapter {
	return &PromptAssemblerAdapter{assembler: assembler}
}

// AssembleSystemPrompt implements the simplified interface used by perception.
func (a *PromptAssemblerAdapter) AssembleSystemPrompt(ctx context.Context, shardID, shardType string) (string, error) {
	pc := &PromptContext{
		ShardID:   shardID,
		ShardType: shardType,
	}
	if sCtx := types.GetSessionContext(ctx); sCtx != nil {
		pc.WithSessionContext(sCtx)
	}
	return a.assembler.AssembleSystemPrompt(ctx, pc)
}

// JITReady returns true if JIT compilation is available and enabled.
func (a *PromptAssemblerAdapter) JITReady() bool {
	return a.assembler.JITReady()
}

// mapToPromptContext converts a generic map to a PromptContext.
// This supports the interface-based dependency injection used by autopoiesis.
func (pa *PromptAssembler) mapToPromptContext(m map[string]any) (*PromptContext, error) {
	pc := &PromptContext{}

	if v := types.ExtractString(m["shard_id"]); v != "" {
		pc.ShardID = v
	}
	if v := types.ExtractString(m["shard_type"]); v != "" {
		pc.ShardType = v
	}
	if v := types.ExtractString(m["campaign_id"]); v != "" {
		pc.CampaignID = v
	}
	if v := types.ExtractString(m["semantic_query"]); v != "" {
		pc.SemanticQuery = v
	}
	if v, ok := m["semantic_top_k"].(int); ok {
		pc.SemanticTopK = v
	}

	// Handle complex types if passed
	if v, ok := m["session_ctx"].(*types.SessionContext); ok {
		pc.SessionCtx = v
	}
	if v, ok := m["user_intent"].(*types.StructuredIntent); ok {
		pc.UserIntent = v
	}

	// For Ouroboros integration, we might receive extra fields that need to go into SessionContext.ExtraContext
	// Since we can't modify SessionContext if it's nil, we create a partial one if needed.
	extraContext := make(map[string]string)

	// Map known Ouroboros fields to ExtraContext tags
	keysToCheck := []string{"ouroboros_stage", "stage", "tool_name", "tool_purpose", "input_type", "output_type"}
	for _, key := range keysToCheck {
		if v := types.ExtractString(m[key]); v != "" {
			extraContext[key] = v
			// For "stage", map to "ouroboros_stage" as well if missing
			if key == "stage" {
				extraContext["ouroboros_stage"] = v
			}
		}
	}

	if len(extraContext) > 0 {
		if pc.SessionCtx == nil {
			pc.SessionCtx = &types.SessionContext{
				ExtraContext: extraContext,
			}
		} else {
			// Merge into existing ExtraContext
			if pc.SessionCtx.ExtraContext == nil {
				pc.SessionCtx.ExtraContext = extraContext
			} else {
				maps.Copy(pc.SessionCtx.ExtraContext, extraContext)
			}
		}
	}

	if pc.ShardID == "" || pc.ShardType == "" {
		return nil, fmt.Errorf("missing required fields shard_id or shard_type")
	}

	return pc, nil
}
