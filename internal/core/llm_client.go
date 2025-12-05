package core

import "context"

// LLMClient defines the minimal interface shards use to call an LLM.
// Mirrors perception.LLMClient to avoid import cycles.
type LLMClient interface {
	Complete(ctx context.Context, prompt string) (string, error)
	CompleteWithSystem(ctx context.Context, systemPrompt, userPrompt string) (string, error)
}
