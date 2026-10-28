package shards

import (
	"context"
	"fmt"
	"strings"

	"codenerd/internal/config"
	"codenerd/internal/logging"
	"codenerd/internal/types"
)

// ImageGeneratorAgent runs image-generation prompts on the dedicated Gemini
// Nano Banana 2 client (never the Ollama worker). Registered as the factory
// for image_generator / image / nano_banana shard types.
type ImageGeneratorAgent struct {
	*BaseShardAgent
	// rolePrompts compiles the image role's system prompt from its atoms
	// (atoms/identity/image_generator.yaml); nil fails every request.
	rolePrompts types.RolePromptCompiler
}

// imageGeneratorRole is the shard type the image role's atoms answer to.
const imageGeneratorRole = "image_generator"

// NewImageGeneratorAgent constructs an image generation agent whose system
// prompt rolePrompts compiles.
func NewImageGeneratorAgent(id string, config types.ShardConfig, rolePrompts types.RolePromptCompiler) *ImageGeneratorAgent {
	return &ImageGeneratorAgent{BaseShardAgent: NewBaseShardAgent(id, config), rolePrompts: rolePrompts}
}

// Execute sends the task to the image LLM with a bounded timeout.
// Fail-closed if no client is attached (Spawn should already reject this).
func (a *ImageGeneratorAgent) Execute(ctx context.Context, task string) (string, error) {
	client := a.llm()
	if client == nil {
		return "", fmt.Errorf("image_generator has no Gemini Nano Banana 2 client (set gemini_api_key)")
	}

	timeout := a.config.Timeout
	if timeout <= 0 {
		// A profile without a timeout still gets the configured image
		// request bound (image.timeout, default 120s): one generation is
		// one request to the image model, so a missing profile value must
		// not become an unbounded call.
		timeout = config.ImageRequestTimeout()
	}
	execCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	prompt := strings.TrimSpace(task)
	if prompt == "" {
		return "", fmt.Errorf("image_generator requires a non-empty task/prompt")
	}

	if a.rolePrompts == nil {
		return "", fmt.Errorf("image_generator: %w: no role prompt compiler", types.ErrRolePromptUnavailable)
	}
	system, err := a.rolePrompts.CompileRolePrompt(execCtx, types.RolePromptRequest{Role: imageGeneratorRole, SemanticQuery: prompt})
	if err != nil {
		return "", fmt.Errorf("image_generator: %w", err)
	}

	logging.Shards("image_generator: calling image LLM (timeout=%v)", timeout)
	result, err := client.CompleteWithSystem(execCtx, system, prompt)
	if err != nil {
		if execCtx.Err() != nil {
			return "", fmt.Errorf("image_generator timed out after %v: %w", timeout, err)
		}
		return "", fmt.Errorf("image_generator LLM failed: %w", err)
	}
	if strings.TrimSpace(result) == "" {
		return "(image model returned empty response)", nil
	}
	return result, nil
}
