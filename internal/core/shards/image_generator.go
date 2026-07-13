package shards

import (
	"context"
	"fmt"
	"strings"
	"time"

	"codenerd/internal/logging"
	"codenerd/internal/types"
)

// ImageGeneratorAgent runs image-generation prompts on the dedicated Gemini
// Nano Banana 2 client (never the Ollama worker). Registered as the factory
// for image_generator / image / nano_banana shard types.
type ImageGeneratorAgent struct {
	*BaseShardAgent
}

// NewImageGeneratorAgent constructs an image generation agent.
func NewImageGeneratorAgent(id string, config types.ShardConfig) *ImageGeneratorAgent {
	return &ImageGeneratorAgent{BaseShardAgent: NewBaseShardAgent(id, config)}
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
		timeout = 2 * time.Minute
	}
	execCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	prompt := strings.TrimSpace(task)
	if prompt == "" {
		return "", fmt.Errorf("image_generator requires a non-empty task/prompt")
	}

	system := "You are an image generation assistant (Gemini Nano Banana 2 / gemini-3.1-flash-image). " +
		"Produce or describe the requested image. If you cannot emit binary image data, " +
		"return a concise textual description and any file paths you would write."

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
