package tactile

import (
	"testing"
	"time"
)

func TestNewDirectExecutor(t *testing.T) {
	t.Parallel()
	executor := NewDirectExecutor()
	if executor == nil {
		t.Fatal("expected executor to not be nil")
	}

	// Check if it has default config
	if executor.config.DefaultTimeout != 30*time.Second {
		t.Errorf("expected default timeout 30s, got %v", executor.config.DefaultTimeout)
	}
}

func TestNewDirectExecutorWithConfig(t *testing.T) {
	t.Parallel()
	config := ExecutorConfig{
		DefaultTimeout: 10 * time.Second,
		MaxOutputBytes: 1000,
	}
	executor := NewDirectExecutorWithConfig(config)
	if executor == nil {
		t.Fatal("expected executor to not be nil")
	}
	if executor.config.DefaultTimeout != 10*time.Second {
		t.Errorf("expected default timeout 10s, got %v", executor.config.DefaultTimeout)
	}
	if executor.config.MaxOutputBytes != 1000 {
		t.Errorf("expected max output bytes 1000, got %d", executor.config.MaxOutputBytes)
	}
}
