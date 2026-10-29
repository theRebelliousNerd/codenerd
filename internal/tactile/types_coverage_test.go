package tactile

import (
	"testing"
	"time"
)

// --- Command ---

func TestCommandString_WhenNoArgs_ShouldReturnBinary(t *testing.T) {
	cmd := Command{Binary: "go"}
	if cmd.CommandString() != "go" {
		t.Errorf("expected 'go', got %q", cmd.CommandString())
	}
}

func TestCommandString_WhenWithArgs_ShouldJoinAll(t *testing.T) {
	cmd := Command{Binary: "go", Arguments: []string{"test", "-v", "./..."}}
	expected := "go test -v ./..."
	if cmd.CommandString() != expected {
		t.Errorf("expected %q, got %q", expected, cmd.CommandString())
	}
}

func TestCommandString_WhenEmptyBinary_ShouldReturnEmpty(t *testing.T) {
	cmd := Command{}
	if cmd.CommandString() != "" {
		t.Errorf("expected empty, got %q", cmd.CommandString())
	}
}

// --- ExecutionResult ---

func TestExecutionResult_IsError_WhenNotSuccess(t *testing.T) {
	r := &ExecutionResult{Success: false}
	if !r.IsError() {
		t.Error("expected IsError=true when Success=false")
	}
}

func TestExecutionResult_IsError_WhenErrorMessage(t *testing.T) {
	r := &ExecutionResult{Success: true, Error: "something broke"}
	if !r.IsError() {
		t.Error("expected IsError=true when Error is set")
	}
}

func TestExecutionResult_IsError_WhenClean(t *testing.T) {
	r := &ExecutionResult{Success: true}
	if r.IsError() {
		t.Error("expected IsError=false for clean result")
	}
}

func TestExecutionResult_IsNonZeroExit_WhenSuccessAndNonZero(t *testing.T) {
	r := &ExecutionResult{Success: true, ExitCode: 1}
	if !r.IsNonZeroExit() {
		t.Error("expected IsNonZeroExit=true for exit code 1")
	}
}

func TestExecutionResult_IsNonZeroExit_WhenZeroExit(t *testing.T) {
	r := &ExecutionResult{Success: true, ExitCode: 0}
	if r.IsNonZeroExit() {
		t.Error("expected IsNonZeroExit=false for exit code 0")
	}
}

func TestExecutionResult_IsNonZeroExit_WhenNotSuccess(t *testing.T) {
	r := &ExecutionResult{Success: false, ExitCode: 1}
	if r.IsNonZeroExit() {
		t.Error("expected IsNonZeroExit=false when Success=false")
	}
}

func TestExecutionResult_Output_WhenCombined(t *testing.T) {
	r := &ExecutionResult{Combined: "combined output", Stdout: "stdout", Stderr: "stderr"}
	if r.Output() != "combined output" {
		t.Errorf("expected combined output, got %q", r.Output())
	}
}

func TestExecutionResult_Output_WhenStdoutOnly(t *testing.T) {
	r := &ExecutionResult{Stdout: "stdout only"}
	if r.Output() != "stdout only" {
		t.Errorf("expected stdout only, got %q", r.Output())
	}
}

func TestExecutionResult_Output_WhenStderrOnly(t *testing.T) {
	r := &ExecutionResult{Stderr: "stderr only"}
	if r.Output() != "stderr only" {
		t.Errorf("expected stderr only, got %q", r.Output())
	}
}

func TestExecutionResult_Output_WhenBothButNoCombined(t *testing.T) {
	r := &ExecutionResult{Stdout: "out", Stderr: "err"}
	expected := "out\nerr"
	if r.Output() != expected {
		t.Errorf("expected %q, got %q", expected, r.Output())
	}
}

func TestExecutionResult_Output_WhenEmpty(t *testing.T) {
	r := &ExecutionResult{}
	if r.Output() != "" {
		t.Errorf("expected empty output, got %q", r.Output())
	}
}

// --- ResourceUsage ---

func TestResourceUsage_TotalCPUTimeMs_EdgeCases(t *testing.T) {
	tests := []struct {
		name     string
		user     int64
		system   int64
		expected int64
	}{
		{"both_nonzero", 100, 50, 150},
		{"user_only", 200, 0, 200},
		{"system_only", 0, 300, 300},
		{"both_zero", 0, 0, 0},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ru := &ResourceUsage{UserTimeMs: tt.user, SystemTimeMs: tt.system}
			if ru.TotalCPUTimeMs() != tt.expected {
				t.Errorf("expected %d, got %d", tt.expected, ru.TotalCPUTimeMs())
			}
		})
	}
}

// --- DefaultExecutorConfig ---

func TestDefaultExecutorConfig_ShouldReturnSensibleDefaults(t *testing.T) {
	cfg := DefaultExecutorConfig()

	if cfg.DefaultWorkingDir != "." {
		t.Errorf("expected DefaultWorkingDir='.', got %q", cfg.DefaultWorkingDir)
	}
	if cfg.DefaultTimeout != 30*time.Second {
		t.Errorf("expected DefaultTimeout=30s, got %v", cfg.DefaultTimeout)
	}
	if cfg.MaxTimeout != 10*time.Minute {
		t.Errorf("expected MaxTimeout=10m, got %v", cfg.MaxTimeout)
	}
	if cfg.MaxOutputBytes != 10*1024*1024 {
		t.Errorf("expected MaxOutputBytes=10MB, got %d", cfg.MaxOutputBytes)
	}
	if cfg.DefaultLimits == nil {
		t.Fatal("expected DefaultLimits to be set")
	}
	if cfg.DefaultLimits.TimeoutMs != 30000 {
		t.Errorf("expected timeout 30000ms, got %d", cfg.DefaultLimits.TimeoutMs)
	}
	if cfg.DockerDefaultImage != "alpine:latest" {
		t.Errorf("expected DockerDefaultImage='alpine:latest', got %q", cfg.DockerDefaultImage)
	}
	if !cfg.EnableResourceUsage {
		t.Error("expected EnableResourceUsage=true")
	}
	if len(cfg.AllowedEnvironment) == 0 {
		t.Error("expected non-empty AllowedEnvironment")
	}
}

// --- Merge ---

func TestMerge_WhenEmptyCommand_ShouldApplyDefaults(t *testing.T) {
	cfg := DefaultExecutorConfig()
	cmd := Command{Binary: "go"}
	merged := cfg.Merge(cmd)

	if merged.WorkingDirectory != "." {
		t.Errorf("expected WorkingDirectory='.', got %q", merged.WorkingDirectory)
	}
	if merged.Limits == nil {
		t.Fatal("expected Limits to be applied from defaults")
	}
	if merged.Limits.TimeoutMs != 30000 {
		t.Errorf("expected timeout 30000ms, got %d", merged.Limits.TimeoutMs)
	}
}

func TestMerge_WhenCommandHasWorkingDir_ShouldNotOverride(t *testing.T) {
	cfg := DefaultExecutorConfig()
	cmd := Command{Binary: "go", WorkingDirectory: "/custom/dir"}
	merged := cfg.Merge(cmd)

	if merged.WorkingDirectory != "/custom/dir" {
		t.Errorf("expected WorkingDirectory='/custom/dir', got %q", merged.WorkingDirectory)
	}
}

func TestMerge_WhenCommandHasLimits_ShouldMerge(t *testing.T) {
	cfg := DefaultExecutorConfig()
	cmd := Command{
		Binary: "go",
		Limits: &ResourceLimits{
			TimeoutMs: 5000,
			// MaxOutputBytes not set, should inherit from config
		},
	}
	merged := cfg.Merge(cmd)

	if merged.Limits.TimeoutMs != 5000 {
		t.Errorf("expected command timeout 5000, got %d", merged.Limits.TimeoutMs)
	}
	if merged.Limits.MaxOutputBytes != 10*1024*1024 {
		t.Errorf("expected inherited MaxOutputBytes, got %d", merged.Limits.MaxOutputBytes)
	}
}

func TestMerge_WhenTimeoutExceedsMax_ShouldCap(t *testing.T) {
	cfg := DefaultExecutorConfig()
	cfg.MaxTimeout = 1 * time.Minute
	cmd := Command{
		Binary: "go",
		Limits: &ResourceLimits{
			TimeoutMs: 120000, // 2 minutes - exceeds max
		},
	}
	merged := cfg.Merge(cmd)

	maxMs := int64(cfg.MaxTimeout / time.Millisecond)
	if merged.Limits.TimeoutMs != maxMs {
		t.Errorf("expected timeout capped at %d, got %d", maxMs, merged.Limits.TimeoutMs)
	}
}

func TestMerge_WhenNoDefaultLimits_ShouldNotSetLimits(t *testing.T) {
	cfg := ExecutorConfig{DefaultWorkingDir: "."}
	cmd := Command{Binary: "go"}
	merged := cfg.Merge(cmd)

	if merged.Limits != nil {
		t.Error("expected nil Limits when no defaults")
	}
}

func TestMerge_WhenDefaultSandbox_ShouldApply(t *testing.T) {
	cfg := DefaultExecutorConfig()
	cfg.DefaultSandbox = &SandboxConfig{Mode: SandboxDocker, Image: "alpine"}
	cmd := Command{Binary: "go"}
	merged := cfg.Merge(cmd)

	if merged.Sandbox == nil {
		t.Fatal("expected Sandbox to be applied from defaults")
	}
	if merged.Sandbox.Mode != SandboxDocker {
		t.Errorf("expected Docker sandbox, got %s", merged.Sandbox.Mode)
	}
}

func TestMerge_WhenCommandHasSandbox_ShouldNotOverride(t *testing.T) {
	cfg := DefaultExecutorConfig()
	cfg.DefaultSandbox = &SandboxConfig{Mode: SandboxDocker, Image: "alpine"}
	cmd := Command{
		Binary:  "go",
		Sandbox: &SandboxConfig{Mode: SandboxNone},
	}
	merged := cfg.Merge(cmd)

	if merged.Sandbox.Mode != SandboxNone {
		t.Errorf("expected command sandbox to take precedence, got %s", merged.Sandbox.Mode)
	}
}

// --- SandboxMode constants ---

func TestSandboxModeConstants_ShouldBeDistinct(t *testing.T) {
	modes := []SandboxMode{SandboxNone, SandboxDocker, SandboxNamespace, SandboxFirejail}
	seen := make(map[SandboxMode]bool)
	for _, m := range modes {
		if seen[m] {
			t.Errorf("duplicate sandbox mode: %s", m)
		}
		seen[m] = true
	}
}

// --- AuditEventType constants ---

func TestAuditEventTypeConstants_ShouldBeDistinct(t *testing.T) {
	types := []AuditEventType{
		AuditEventStart, AuditEventComplete, AuditEventKilled,
		AuditEventError, AuditEventBlocked, AuditEventSandboxed,
	}
	seen := make(map[AuditEventType]bool)
	for _, et := range types {
		if seen[et] {
			t.Errorf("duplicate audit event type: %s", et)
		}
		seen[et] = true
	}
}
