package tactile

import (
	"context"
	"testing"
)

func TestNewDockerExecutor(t *testing.T) {
	executor := NewDockerExecutor()
	if executor == nil {
		t.Fatal("expected non-nil executor")
	}

	if executor.config.DefaultTimeout == 0 {
		t.Fatal("expected config to be initialized with defaults")
	}
}

func TestDockerExecutor_PullImage_Unavailable(t *testing.T) {
	executor := &DockerExecutor{available: false}
	err := executor.PullImage(context.Background(), "alpine")
	if err == nil {
		t.Fatal("expected error when docker is unavailable")
	}
}

func TestDockerExecutor_ImageExists_Unavailable(t *testing.T) {
	executor := &DockerExecutor{available: false}
	exists := executor.ImageExists(context.Background(), "alpine")
	if exists {
		t.Fatal("expected false when docker is unavailable")
	}
}

func TestDockerExecutor_Execute_Unavailable(t *testing.T) {
	executor := &DockerExecutor{available: false}
	cmd := Command{Binary: "echo", Arguments: []string{"hello"}}
	_, err := executor.Execute(context.Background(), cmd)
	if err == nil {
		t.Fatal("expected error when docker is unavailable")
	}
}
