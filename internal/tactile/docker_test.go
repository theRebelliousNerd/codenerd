package tactile

import (
	"context"
	"strings"
	"testing"
	"time"
)

func TestNewDockerExecutor(t *testing.T) {
	// Rely on docker_detection_test patterns to ensure docker is initialized
	executor := NewDockerExecutor()
	if executor == nil {
		t.Fatal("NewDockerExecutor returned nil")
	}
	if executor.config.DefaultTimeout == 0 {
		t.Errorf("Expected config default timeout to be set, got 0")
	}
}

func TestDockerExecutorCapabilities(t *testing.T) {
	config := DefaultExecutorConfig()
	executor := NewDockerExecutorWithConfig(config)

	caps := executor.Capabilities()

	if caps.Name != "docker" {
		t.Errorf("Expected capability name 'docker', got %s", caps.Name)
	}

	if !caps.SupportsResourceLimits {
		t.Errorf("Expected docker executor to support resource limits")
	}
}

func TestDockerExecutorValidate(t *testing.T) {
	// Setup docker executor in available state
	dockerDetectionCache.Lock()
	previousCheckedAt := dockerDetectionCache.checkedAt
	previousPath := dockerDetectionCache.path
	previousAvailable := dockerDetectionCache.available
	dockerDetectionCache.checkedAt = time.Now()
	dockerDetectionCache.path = "docker-test"
	dockerDetectionCache.available = true
	dockerDetectionCache.Unlock()

	t.Cleanup(func() {
		dockerDetectionCache.Lock()
		dockerDetectionCache.checkedAt = previousCheckedAt
		dockerDetectionCache.path = previousPath
		dockerDetectionCache.available = previousAvailable
		dockerDetectionCache.Unlock()
	})

	executor := NewDockerExecutorWithConfig(DefaultExecutorConfig())

	// Test validation - Empty binary
	cmd := Command{
		Binary: "",
	}
	err := executor.Validate(cmd)
	if err == nil {
		t.Errorf("Expected error for empty binary")
	}

	// Test validation - Missing sandbox config
	cmd = Command{
		Binary: "echo",
	}
	err = executor.Validate(cmd)
	if err == nil {
		t.Errorf("Expected error for missing sandbox config")
	}

	// Test validation - Invalid sandbox mode
	cmd = Command{
		Binary: "echo",
		Sandbox: &SandboxConfig{
			Mode: SandboxNone,
		},
	}
	err = executor.Validate(cmd)
	if err == nil {
		t.Errorf("Expected error for non-docker sandbox mode")
	}

	// Test validation - Valid command
	cmd = Command{
		Binary: "echo",
		Sandbox: &SandboxConfig{
			Mode: SandboxDocker,
		},
	}
	err = executor.Validate(cmd)
	if err != nil {
		t.Errorf("Expected valid command, got error: %v", err)
	}
}

func TestDockerExecutorValidate_Unavailable(t *testing.T) {
	// Setup docker executor in unavailable state
	dockerDetectionCache.Lock()
	previousCheckedAt := dockerDetectionCache.checkedAt
	previousPath := dockerDetectionCache.path
	previousAvailable := dockerDetectionCache.available
	dockerDetectionCache.checkedAt = time.Now()
	dockerDetectionCache.path = ""
	dockerDetectionCache.available = false
	dockerDetectionCache.Unlock()

	t.Cleanup(func() {
		dockerDetectionCache.Lock()
		dockerDetectionCache.checkedAt = previousCheckedAt
		dockerDetectionCache.path = previousPath
		dockerDetectionCache.available = previousAvailable
		dockerDetectionCache.Unlock()
	})

	executor := NewDockerExecutorWithConfig(DefaultExecutorConfig())

	cmd := Command{
		Binary: "echo",
		Sandbox: &SandboxConfig{
			Mode: SandboxDocker,
		},
	}

	err := executor.Validate(cmd)
	if err == nil {
		t.Errorf("Expected error when docker is unavailable")
	} else if !strings.Contains(err.Error(), "Docker is not available") {
		t.Errorf("Expected unavailable error message, got: %v", err)
	}
}

func TestDockerExecutorAuditCallback(t *testing.T) {
	executor := NewDockerExecutor()

	auditCalled := false
	var receivedEvent AuditEvent

	executor.SetAuditCallback(func(event AuditEvent) {
		auditCalled = true
		receivedEvent = event
	})

	// trigger audit event
	executor.emitAudit(AuditEvent{
		Type: AuditEventStart,
	})

	if !auditCalled {
		t.Errorf("Audit callback was not called")
	}

	if receivedEvent.Type != AuditEventStart {
		t.Errorf("Expected type 'start', got %s", receivedEvent.Type)
	}
}

func TestDockerExecutorBuildDockerArgs(t *testing.T) {
	config := DefaultExecutorConfig()
	config.DockerDefaultImage = "ubuntu:latest"
	executor := NewDockerExecutorWithConfig(config)

	cmd := Command{
		Binary: "echo",
		Arguments: []string{"hello", "world"},
		Sandbox: &SandboxConfig{
			Image: "custom-image:1.0",
			NetworkMode: "host",
		},
	}

	args := executor.buildDockerArgs(cmd)

	if len(args) < 2 || args[0] != "run" || args[1] != "--rm" {
		t.Errorf("Expected args to start with 'run --rm', got: %v", args[:min(len(args), 2)])
	}

	argsStr := strings.Join(args, " ")
	if !strings.Contains(argsStr, "custom-image:1.0 echo hello world") {
		t.Errorf("Expected image and command at the end, args: %v", args)
	}

	if !strings.Contains(argsStr, "--network host") {
		t.Errorf("Expected --network host, args: %v", args)
	}
}

func TestDockerExecutorExecute_ErrorCases(t *testing.T) {
	// Setup docker executor in available state
	dockerDetectionCache.Lock()
	previousCheckedAt := dockerDetectionCache.checkedAt
	previousPath := dockerDetectionCache.path
	previousAvailable := dockerDetectionCache.available
	dockerDetectionCache.checkedAt = time.Now()
	dockerDetectionCache.path = "docker-test"
	dockerDetectionCache.available = true
	dockerDetectionCache.Unlock()

	t.Cleanup(func() {
		dockerDetectionCache.Lock()
		dockerDetectionCache.checkedAt = previousCheckedAt
		dockerDetectionCache.path = previousPath
		dockerDetectionCache.available = previousAvailable
		dockerDetectionCache.Unlock()
	})

	executor := NewDockerExecutorWithConfig(DefaultExecutorConfig())

	cmd := Command{
		Binary: "",
	}

	_, err := executor.Execute(context.Background(), cmd)
	if err == nil {
		t.Errorf("Expected validation error when executing empty binary")
	}
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}





// TODO: Null/Undefined/Empty: Test empty Image Name (should default to alpine or config default).
// TODO: Null/Undefined/Empty: Test empty Arguments list (ensure no trailing spaces or malformed args).
// TODO: Null/Undefined/Empty: Test empty Network Mode.
// TODO: Null/Undefined/Empty: Test empty Environment Variables.
// TODO: Type Coercion / Data Extremes: Test Extremely Long Strings for Binary and Arguments (ARG_MAX limits).
// TODO: Type Coercion / Data Extremes: Test malformed image tags (injection attempts like "ubuntu:latest --privileged").
// TODO: Type Coercion / Data Extremes: Test negative integer values in SandboxConfig limits.
// TODO: User Request Extremes: Test excessive number of arguments (e.g. 100,000 args).
// TODO: User Request Extremes: Test extreme resource constraints (e.g., memory limit 1 byte) and verify error parsing.
// TODO: User Request Extremes: Test extreme output (Stdout/Stderr flooding) to verify limitedWriter discards excess without OOM.
// TODO: User Request Extremes: Test extreme timeout (e.g., 1ms) ensuring clean context cancellation.
// TODO: State Conflicts: Test concurrent SetAuditCallback vs emitAudit for race conditions.
// TODO: State Conflicts: Test TOCTOU Docker Unavailability (Docker available at Validate, but daemon crashes before Execute).
// TODO: State Conflicts: Test conflicting mount paths (same path in both AllowedPaths and ReadOnlyPaths).
// TODO: State Conflicts: Test concurrent Execute calls to guarantee thread safety and no shared state leaks.
