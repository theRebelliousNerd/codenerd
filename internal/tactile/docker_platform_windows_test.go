//go:build windows

package tactile

import (
	"context"
	"runtime"
	"testing"
	"time"
)

// Windows-specific platform executor tests. These reference symbols that
// only compile under GOOS=windows (job objects, LimitedExecutorWindows,
// WindowsContainerExecutor), so they live behind a build constraint.

func TestLimitedExecutorWindows_Capabilities_ShouldReportLimits(t *testing.T) {
	t.Parallel()
	if runtime.GOOS != "windows" {
		t.Skip("Windows-only test")
	}
	config := DefaultExecutorConfig()
	exec := NewLimitedExecutorWindows(config)
	caps := exec.Capabilities()

	if caps.Name != "limited-windows" {
		t.Errorf("expected name 'limited-windows', got %q", caps.Name)
	}
	if !caps.SupportsResourceLimits {
		t.Error("expected SupportsResourceLimits=true")
	}
	if !caps.SupportsResourceUsage {
		t.Error("expected SupportsResourceUsage=true")
	}
}

func TestLimitedExecutorWindows_Validate_ShouldDelegate(t *testing.T) {
	t.Parallel()
	if runtime.GOOS != "windows" {
		t.Skip("Windows-only test")
	}
	config := DefaultExecutorConfig()
	exec := NewLimitedExecutorWindows(config)

	if err := exec.Validate(Command{Binary: "echo"}); err != nil {
		t.Errorf("unexpected error: %v", err)
	}
	if err := exec.Validate(Command{Binary: ""}); err == nil {
		t.Error("expected error for empty binary")
	}
}

func TestLimitedExecutorWindows_Execute_WhenNoLimits_ShouldDelegateToParent(t *testing.T) {
	if runtime.GOOS != "windows" {
		t.Skip("Windows-only test")
	}
	config := DefaultExecutorConfig()
	exec := NewLimitedExecutorWindows(config)

	cmd := Command{
		Binary:    "cmd",
		Arguments: []string{"/c", "echo", "hello"},
	}

	result, err := exec.Execute(context.Background(), cmd)
	if err != nil {
		t.Fatalf("Execute failed: %v", err)
	}
	if !result.Success {
		t.Errorf("expected success: %s", result.Error)
	}
}

func TestLimitedExecutorWindows_Execute_WhenWithLimits_ShouldUseJobObjects(t *testing.T) {
	if runtime.GOOS != "windows" {
		t.Skip("Windows-only test")
	}
	config := DefaultExecutorConfig()
	exec := NewLimitedExecutorWindows(config)

	cmd := Command{
		Binary:    "cmd",
		Arguments: []string{"/c", "echo", "limited"},
		Limits: &ResourceLimits{
			TimeoutMs:      5000,
			MaxMemoryBytes: 256 * 1024 * 1024,
		},
	}

	result, err := exec.Execute(context.Background(), cmd)
	if err != nil {
		t.Fatalf("Execute failed: %v", err)
	}
	if !result.Success {
		t.Errorf("expected success: %s", result.Error)
	}
}

func TestGetLimitedExecutor_ShouldReturnLimitedWindows(t *testing.T) {
	t.Parallel()
	if runtime.GOOS != "windows" {
		t.Skip("Windows-only test")
	}
	config := DefaultExecutorConfig()
	exec := GetLimitedExecutor(config)
	if exec == nil {
		t.Fatal("GetLimitedExecutor returned nil")
	}
	caps := exec.Capabilities()
	if caps.Name != "limited-windows" {
		t.Errorf("expected 'limited-windows', got %q", caps.Name)
	}
}

// =============================================================================
// WindowsContainerExecutor
// =============================================================================

func TestWindowsContainerExecutor_Capabilities_WhenNotAvailable(t *testing.T) {
	t.Parallel()
	if runtime.GOOS != "windows" {
		t.Skip("Windows-only test")
	}

	exec := &WindowsContainerExecutor{
		DirectExecutor: NewDirectExecutor(),
		available:      false,
	}

	caps := exec.Capabilities()
	if caps.Name != "windows-container" {
		t.Errorf("expected name 'windows-container', got %q", caps.Name)
	}
	if caps.SupportsNetworkIsolation {
		t.Error("expected no network isolation when not available")
	}
	// Should have SandboxNone only
	if len(caps.SupportedSandboxModes) != 1 || caps.SupportedSandboxModes[0] != SandboxNone {
		t.Errorf("expected [none] modes, got %v", caps.SupportedSandboxModes)
	}
}

func TestWindowsContainerExecutor_IsAvailable_ShouldReflectState(t *testing.T) {
	t.Parallel()
	if runtime.GOOS != "windows" {
		t.Skip("Windows-only test")
	}

	exec := &WindowsContainerExecutor{
		DirectExecutor: NewDirectExecutor(),
		available:      false,
	}
	if exec.IsAvailable() {
		t.Error("expected not available")
	}
}

// =============================================================================
// NamespaceConfig type check (Windows stub)
// =============================================================================

func TestNamespaceConfig_ShouldExist(t *testing.T) {
	t.Parallel()
	if runtime.GOOS != "windows" {
		t.Skip("Windows-only test")
	}
	// Just verify the type exists and can be instantiated
	cfg := NamespaceConfig{
		NewPID:   true,
		NewNet:   true,
		Hostname: "test",
	}
	if !cfg.NewPID {
		t.Error("expected NewPID=true")
	}
}

// =============================================================================
// JobObject tests
// =============================================================================

func TestNewJobObject_ShouldCreateAndClose(t *testing.T) {
	if runtime.GOOS != "windows" {
		t.Skip("Windows-only test")
	}

	job, err := NewJobObject("")
	if err != nil {
		t.Fatalf("NewJobObject failed: %v", err)
	}
	if err := job.Close(); err != nil {
		t.Errorf("Close failed: %v", err)
	}
}

func TestNewJobObject_WhenNamed_ShouldCreateAndClose(t *testing.T) {
	if runtime.GOOS != "windows" {
		t.Skip("Windows-only test")
	}

	name := "tactile_test_job_" + time.Now().Format("20060102150405")
	job, err := NewJobObject(name)
	if err != nil {
		t.Fatalf("NewJobObject with name failed: %v", err)
	}
	defer job.Close()

	if job.name != name {
		t.Errorf("expected name %q, got %q", name, job.name)
	}
}

func TestJobObject_SetLimits_WhenNilLimits_ShouldReturnNil(t *testing.T) {
	if runtime.GOOS != "windows" {
		t.Skip("Windows-only test")
	}

	job, err := NewJobObject("")
	if err != nil {
		t.Fatalf("NewJobObject failed: %v", err)
	}
	defer job.Close()

	if err := job.SetLimits(nil); err != nil {
		t.Errorf("SetLimits(nil) should return nil, got: %v", err)
	}
}

func TestJobObject_SetLimits_WhenMemoryLimit_ShouldNotError(t *testing.T) {
	if runtime.GOOS != "windows" {
		t.Skip("Windows-only test")
	}

	job, err := NewJobObject("")
	if err != nil {
		t.Fatalf("NewJobObject failed: %v", err)
	}
	defer job.Close()

	limits := &ResourceLimits{
		MaxMemoryBytes: 512 * 1024 * 1024, // 512MB
		MaxProcesses:   100,
	}
	if err := job.SetLimits(limits); err != nil {
		t.Errorf("SetLimits failed: %v", err)
	}
}
