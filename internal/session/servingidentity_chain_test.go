package session

import (
	"context"
	"testing"

	"codenerd/internal/types"
)

// identityStubClient is a bottom-of-chain client that reports a model identity.
type identityStubClient struct {
	provider string
	model    string
}

func (c *identityStubClient) Complete(_ context.Context, _ string) (string, error) {
	return "", nil
}

func (c *identityStubClient) CompleteWithSystem(_ context.Context, _, _ string) (string, error) {
	return "", nil
}

func (c *identityStubClient) CompleteWithStreaming(_ context.Context, _, _ string, _ bool) (<-chan string, <-chan error) {
	return nil, nil
}

func (c *identityStubClient) CompleteWithTools(_ context.Context, _, _ string, _ []types.ToolDefinition) (*types.LLMToolResponse, error) {
	return nil, nil
}

func (c *identityStubClient) ModelIdentity() (string, string) {
	return c.provider, c.model
}

// unwrapOnlyDecorator mirrors the pre-fix blind spot: a decorator layer that
// exposes Unwrap but no ModelIdentity of its own.
type unwrapOnlyDecorator struct {
	inner types.LLMClient
}

func (d *unwrapOnlyDecorator) Complete(ctx context.Context, prompt string) (string, error) {
	return d.inner.Complete(ctx, prompt)
}

func (d *unwrapOnlyDecorator) CompleteWithSystem(ctx context.Context, systemPrompt, userPrompt string) (string, error) {
	return d.inner.CompleteWithSystem(ctx, systemPrompt, userPrompt)
}

func (d *unwrapOnlyDecorator) CompleteWithStreaming(ctx context.Context, systemPrompt, userPrompt string, enableThinking bool) (<-chan string, <-chan error) {
	return d.inner.CompleteWithStreaming(ctx, systemPrompt, userPrompt, enableThinking)
}

func (d *unwrapOnlyDecorator) CompleteWithTools(ctx context.Context, systemPrompt, userPrompt string, tools []types.ToolDefinition) (*types.LLMToolResponse, error) {
	return d.inner.CompleteWithTools(ctx, systemPrompt, userPrompt, tools)
}

func (d *unwrapOnlyDecorator) Unwrap() types.LLMClient {
	return d.inner
}

// identifyingDecorator is a layer that both wraps an inner client and reports
// its own identity, like FallbackClient.
type identifyingDecorator struct {
	inner    types.LLMClient
	provider string
	model    string
}

func (d *identifyingDecorator) Complete(ctx context.Context, prompt string) (string, error) {
	return d.inner.Complete(ctx, prompt)
}

func (d *identifyingDecorator) CompleteWithSystem(ctx context.Context, systemPrompt, userPrompt string) (string, error) {
	return d.inner.CompleteWithSystem(ctx, systemPrompt, userPrompt)
}

func (d *identifyingDecorator) CompleteWithStreaming(ctx context.Context, systemPrompt, userPrompt string, enableThinking bool) (<-chan string, <-chan error) {
	return d.inner.CompleteWithStreaming(ctx, systemPrompt, userPrompt, enableThinking)
}

func (d *identifyingDecorator) CompleteWithTools(ctx context.Context, systemPrompt, userPrompt string, tools []types.ToolDefinition) (*types.LLMToolResponse, error) {
	return d.inner.CompleteWithTools(ctx, systemPrompt, userPrompt, tools)
}

func (d *identifyingDecorator) Unwrap() types.LLMClient {
	return d.inner
}

func (d *identifyingDecorator) ModelIdentity() (string, string) {
	return d.provider, d.model
}

// plainStubClient reports no identity and wraps nothing.
type plainStubClient struct{}

func (c *plainStubClient) Complete(_ context.Context, _ string) (string, error) {
	return "", nil
}

func (c *plainStubClient) CompleteWithSystem(_ context.Context, _, _ string) (string, error) {
	return "", nil
}

func (c *plainStubClient) CompleteWithStreaming(_ context.Context, _, _ string, _ bool) (<-chan string, <-chan error) {
	return nil, nil
}

func (c *plainStubClient) CompleteWithTools(_ context.Context, _, _ string, _ []types.ToolDefinition) (*types.LLMToolResponse, error) {
	return nil, nil
}

func TestServingIdentityChain(t *testing.T) {
	inner := &identityStubClient{provider: "openrouter", model: "anthropic/claude-opus-4"}

	tests := []struct {
		name         string
		client       types.LLMClient
		wantProvider string
		wantModel    string
	}{
		{
			name:         "decorator with only Unwrap yields inner identity",
			client:       &unwrapOnlyDecorator{inner: inner},
			wantProvider: "openrouter",
			wantModel:    "anthropic/claude-opus-4",
		},
		{
			name:         "outermost identifier wins",
			client:       &identifyingDecorator{inner: inner, provider: "fallback", model: "fallback-model"},
			wantProvider: "fallback",
			wantModel:    "fallback-model",
		},
		{
			name:         "chain with no identifier yields empty strings",
			client:       &unwrapOnlyDecorator{inner: &plainStubClient{}},
			wantProvider: "",
			wantModel:    "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			exec := NewExecutor(nil, nil, tt.client, nil, nil, nil)
			gotProvider, gotModel := exec.servingIdentity("/general")
			if gotProvider != tt.wantProvider || gotModel != tt.wantModel {
				t.Errorf("Executor.servingIdentity() = (%q, %q), want (%q, %q)",
					gotProvider, gotModel, tt.wantProvider, tt.wantModel)
			}

			sp := NewSpawner(nil, nil, tt.client, nil, nil, nil, DefaultSpawnerConfig())
			gotProvider, gotModel = sp.servingIdentity()
			if gotProvider != tt.wantProvider || gotModel != tt.wantModel {
				t.Errorf("Spawner.servingIdentity() = (%q, %q), want (%q, %q)",
					gotProvider, gotModel, tt.wantProvider, tt.wantModel)
			}
		})
	}
}
