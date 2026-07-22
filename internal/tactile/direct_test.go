package tactile

import (
	"testing"
	"time"
)

func TestDirectExecutor_New(t *testing.T) {
	executor := NewDirectExecutor()
	if executor == nil {
		t.Fatal("Expected executor to not be nil")
	}

	if executor.config.MaxOutputBytes != 10*1024*1024 {
		t.Errorf("Expected MaxOutputBytes to be 10MB, got %d", executor.config.MaxOutputBytes)
	}

	config := ExecutorConfig{
		DefaultTimeout: 5 * time.Second,
		MaxOutputBytes: 1024,
	}
	executorWithConfig := NewDirectExecutorWithConfig(config)
	if executorWithConfig.config.DefaultTimeout != 5*time.Second {
		t.Errorf("Expected DefaultTimeout 5s, got %v", executorWithConfig.config.DefaultTimeout)
	}
}
