package campaign

import (
	"codenerd/internal/perception"
	"context"
	"testing"
)

func TestOrchestrator_DependencyInjection(t *testing.T) {
	// Setup mocks
	mockKernel := &MockKernel{}
	mockLLM := &MockLLMClient{}

	// specialized mock transducer to verify it's being called
	mockTransducer := &MockTransducer{
		ParseIntentFunc: func(ctx context.Context, input string) (perception.Intent, error) {
			return perception.Intent{Verb: "/mocked"}, nil
		},
	}

	// Initialize Orchestrator with injected dependencies
	config := OrchestratorConfig{
		Workspace:  t.TempDir(),
		Kernel:     mockKernel,
		LLMClient:  mockLLM,
		Transducer: mockTransducer,
	}

	orch := NewOrchestrator(config)

	// Verify dependencies were injected correctly
	if orch.kernel != mockKernel {
		t.Errorf("Kernel injection failed")
	}
	if orch.transducer != mockTransducer {
		t.Errorf("Transducer injection failed")
	}

	// Verify behaviour of injected component
	intent, err := orch.transducer.ParseIntent(context.Background(), "test")
	if err != nil {
		t.Fatalf("ParseIntent failed: %v", err)
	}
	if intent.Verb != "/mocked" {
		t.Errorf("Expected verb /mocked, got %s", intent.Verb)
	}
}
