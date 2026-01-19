package core

import (
	"context"
	"testing"

	"codenerd/internal/types"
)

// mockSchemaBasicClient implements types.LLMClient locally for this test
type mockSchemaBasicClient struct{}

func (m *mockSchemaBasicClient) Complete(ctx context.Context, prompt string) (string, error) {
	return "response", nil
}

func (m *mockSchemaBasicClient) CompleteWithSystem(ctx context.Context, systemPrompt, userPrompt string) (string, error) {
	return "response", nil
}

func (m *mockSchemaBasicClient) CompleteWithTools(ctx context.Context, systemPrompt, userPrompt string, tools []types.ToolDefinition) (*types.LLMToolResponse, error) {
	return &types.LLMToolResponse{Text: "response"}, nil
}

// mockSchemaCapableClient implements SchemaCapableLLMClient
type mockSchemaCapableClient struct {
	mockSchemaBasicClient
}

func (m *mockSchemaCapableClient) CompleteWithSchema(ctx context.Context, systemPrompt, userPrompt, jsonSchema string) (string, error) {
	return "{\"key\": \"value\"}", nil
}

// mockDynamicSchemaTestClient implements SchemaCapableLLMClient AND optional schemaCapability check
type mockDynamicSchemaTestClient struct {
	mockSchemaCapableClient
	capable bool
}

func (m *mockDynamicSchemaTestClient) SchemaCapable() bool {
	return m.capable
}

func TestAsSchemaCapable(t *testing.T) {
	// Case 1: Client that does NOT implement SchemaCapableLLMClient
	basicClient := &mockSchemaBasicClient{}
	if _, ok := AsSchemaCapable(basicClient); ok {
		t.Error("Expected ok=false for basic client")
	}

	// Case 2: Client that implements SchemaCapableLLMClient (implicit capability)
	capableClient := &mockSchemaCapableClient{}
	sc, ok := AsSchemaCapable(capableClient)
	if !ok {
		t.Error("Expected ok=true for capable client")
	}
	if sc == nil {
		t.Error("Expected non-nil client")
	}

	// Case 3: Client that implements interface but returns false from SchemaCapable()
	dynamicDisabled := &mockDynamicSchemaTestClient{capable: false}
	if _, ok := AsSchemaCapable(dynamicDisabled); ok {
		t.Error("Expected ok=false when SchemaCapable() returns false")
	}

	// Case 4: Client that implements interface and returns true from SchemaCapable()
	dynamicEnabled := &mockDynamicSchemaTestClient{capable: true}
	sc, ok = AsSchemaCapable(dynamicEnabled)
	if !ok {
		t.Error("Expected ok=true when SchemaCapable() returns true")
	}
	if sc == nil {
		t.Error("Expected non-nil client")
	}
}

func TestLLMClient_ErrorConstants(t *testing.T) {
	if ErrStreamingNotSupported.Error() != "streaming not supported" {
		t.Errorf("Unexpected ErrStreamingNotSupported message: %s", ErrStreamingNotSupported.Error())
	}
	if ErrSchemaNotSupported.Error() != "schema validation not supported" {
		t.Errorf("Unexpected ErrSchemaNotSupported message: %s", ErrSchemaNotSupported.Error())
	}
}
