package perception

import (
	"context"
	"testing"

	"codenerd/internal/config"
)

func TestNewDualPayloadTransducer(t *testing.T) {
	tc := NewDualPayloadTransducer(nil)
	if tc == nil {
		t.Errorf("NewDualPayloadTransducer returned nil")
	}
}

func TestDualPayloadTransducer_Parse(t *testing.T) {
	tc := NewDualPayloadTransducer(&baseMockLLMClient{})
	ctx := context.Background()
	_, err := tc.Parse(ctx, "hello", nil)
	if err != nil {
		t.Errorf("Expected no error from Parse, got %v", err)
	}
}

func TestInitPerceptionLayer(t *testing.T) {
	// Should do nothing / not panic when called with nil or mock
	err := InitPerceptionLayer(nil, &config.UserConfig{})
	if err != nil {
		t.Errorf("Expected error when InitPerceptionLayer is called with nil kernel")
	}

	// ClosePerceptionLayer takes no args
	ClosePerceptionLayer()
}

func TestSeedFallbackSemanticFacts(t *testing.T) {
	seedFallbackSemanticFacts("", nil)
}
