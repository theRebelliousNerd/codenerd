package tactile

import (
	"context"
	"runtime"
	"testing"
	"time"
)

// =============================================================================
// DockerExecutor - pure function tests (no Docker required)
// =============================================================================

func TestDockerExecutor_BuildDockerArgs_WhenMinimalConfig_ShouldProduceBasicArgs(t *testing.T) {
	t.Parallel()
	executor := &DockerExecutor{
		config: DefaultExecutorConfig(),
	}

	cmd := Command{
		Binary:    "echo",
		Arguments: []string{"hello"},
		Sandbox:   &SandboxConfig{Mode: SandboxDocker, Image: "alpine:3.18"},
	}

	args := executor.buildDockerArgs(cmd)

	// Should start with "run" and "--rm"
	if len(args) < 2 || args[0] != "run" || args[1] != "--rm" {
		t.Errorf("expected args to start with 'run --rm', got %v", args[:min(3, len(args))])
	}

	// Should contain network mode
	hasNetwork := false
	for i, arg := range args {
		if arg == "--network" && i+1 < len(args) {
			hasNetwork = true
		}
	}
	if !hasNetwork {
		t.Error("expected --network flag in docker args")
	}

	// Should end with image and command
	if args[len(args)-2] != "echo" || args[len(args)-1] != "hello" {
		t.Errorf("expected args to end with 'echo hello', got %v", args[len(args)-2:])
	}
}

func TestDockerExecutor_BuildDockerArgs_WhenNilSandbox_ShouldUseDefaults(t *testing.T) {
	t.Parallel()
	executor := &DockerExecutor{
		config: DefaultExecutorConfig(),
	}

	cmd := Command{
		Binary:  "test",
		Sandbox: nil,
	}

	args := executor.buildDockerArgs(cmd)

	// Should contain default image (alpine:latest from config)
	found := false
	for _, arg := range args {
		if arg == "alpine:latest" {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("expected default image 'alpine:latest' in args, got %v", args)
	}
}

func TestDockerExecutor_BuildDockerArgs_WhenReadOnlyRoot_ShouldAddReadOnlyAndTmpfs(t *testing.T) {
	t.Parallel()
	executor := &DockerExecutor{
		config: DefaultExecutorConfig(),
	}

	cmd := Command{
		Binary: "test",
		Sandbox: &SandboxConfig{
			Mode:         SandboxDocker,
			Image:        "test:latest",
			ReadOnlyRoot: true,
		},
	}

	args := executor.buildDockerArgs(cmd)

	hasReadOnly := false
	hasTmpfs := false
	for _, arg := range args {
		if arg == "--read-only" {
			hasReadOnly = true
		}
		if arg == "/tmp:size=100m" {
			hasTmpfs = true
		}
	}
	if !hasReadOnly {
		t.Error("expected --read-only flag")
	}
	if !hasTmpfs {
		t.Error("expected tmpfs mount for /tmp")
	}
}

func TestDockerExecutor_BuildDockerArgs_WhenNoNewPrivileges_ShouldAddSecurityOpt(t *testing.T) {
	t.Parallel()
	executor := &DockerExecutor{
		config: DefaultExecutorConfig(),
	}

	cmd := Command{
		Binary: "test",
		Sandbox: &SandboxConfig{
			Mode:            SandboxDocker,
			Image:           "test:latest",
			NoNewPrivileges: true,
		},
	}

	args := executor.buildDockerArgs(cmd)

	found := false
	for i, arg := range args {
		if arg == "--security-opt" && i+1 < len(args) && args[i+1] == "no-new-privileges" {
			found = true
		}
	}
	if !found {
		t.Error("expected --security-opt no-new-privileges")
	}
}

func TestDockerExecutor_BuildDockerArgs_WhenDropCapabilities_ShouldAddCapDrop(t *testing.T) {
	t.Parallel()
	executor := &DockerExecutor{
		config: DefaultExecutorConfig(),
	}

	cmd := Command{
		Binary: "test",
		Sandbox: &SandboxConfig{
			Mode:             SandboxDocker,
			Image:            "test:latest",
			DropCapabilities: []string{"NET_RAW", "SYS_ADMIN"},
		},
	}

	args := executor.buildDockerArgs(cmd)

	capDropCount := 0
	for i, arg := range args {
		if arg == "--cap-drop" && i+1 < len(args) {
			capDropCount++
		}
	}
	if capDropCount != 2 {
		t.Errorf("expected 2 --cap-drop flags, got %d", capDropCount)
	}
}

func TestDockerExecutor_BuildDockerArgs_WhenUserSpecified_ShouldAddUserFlag(t *testing.T) {
	t.Parallel()
	executor := &DockerExecutor{
		config: DefaultExecutorConfig(),
	}

	cmd := Command{
		Binary: "test",
		Sandbox: &SandboxConfig{
			Mode:  SandboxDocker,
			Image: "test:latest",
			User:  "1000:1000",
		},
	}

	args := executor.buildDockerArgs(cmd)

	found := false
	for i, arg := range args {
		if arg == "--user" && i+1 < len(args) && args[i+1] == "1000:1000" {
			found = true
		}
	}
	if !found {
		t.Error("expected --user 1000:1000")
	}
}

func TestDockerExecutor_BuildDockerArgs_WhenMountPaths_ShouldAddVolumes(t *testing.T) {
	t.Parallel()
	executor := &DockerExecutor{
		config: DefaultExecutorConfig(),
	}

	cmd := Command{
		Binary: "test",
		Sandbox: &SandboxConfig{
			Mode:          SandboxDocker,
			Image:         "test:latest",
			AllowedPaths:  []string{"/src"},
			ReadOnlyPaths: []string{"/config"},
		},
	}

	args := executor.buildDockerArgs(cmd)

	hasRW := false
	hasRO := false
	for i, arg := range args {
		if arg == "-v" && i+1 < len(args) {
			if args[i+1] == "/src:/src:rw" {
				hasRW = true
			}
			if args[i+1] == "/config:/config:ro" {
				hasRO = true
			}
		}
	}
	if !hasRW {
		t.Error("expected read-write volume mount for /src")
	}
	if !hasRO {
		t.Error("expected read-only volume mount for /config")
	}
}

func TestDockerExecutor_BuildDockerArgs_WhenWorkingDir_ShouldAddWorkdirFlag(t *testing.T) {
	t.Parallel()
	executor := &DockerExecutor{
		config: DefaultExecutorConfig(),
	}

	cmd := Command{
		Binary:           "test",
		WorkingDirectory: "/app",
		Sandbox:          &SandboxConfig{Mode: SandboxDocker, Image: "test:latest"},
	}

	args := executor.buildDockerArgs(cmd)

	found := false
	for i, arg := range args {
		if arg == "-w" && i+1 < len(args) && args[i+1] == "/app" {
			found = true
		}
	}
	if !found {
		t.Error("expected -w /app")
	}
}

func TestDockerExecutor_BuildDockerArgs_WhenEnvironment_ShouldAddEnvFlags(t *testing.T) {
	t.Parallel()
	executor := &DockerExecutor{
		config: DefaultExecutorConfig(),
	}

	cmd := Command{
		Binary:      "test",
		Environment: []string{"FOO=bar", "BAZ=qux"},
		Sandbox:     &SandboxConfig{Mode: SandboxDocker, Image: "test:latest"},
	}

	args := executor.buildDockerArgs(cmd)

	envCount := 0
	for i, arg := range args {
		if arg == "-e" && i+1 < len(args) {
			envCount++
		}
	}
	if envCount != 2 {
		t.Errorf("expected 2 -e flags, got %d", envCount)
	}
}

func TestDockerExecutor_BuildDockerArgs_WhenResourceLimits_ShouldAddLimitFlags(t *testing.T) {
	t.Parallel()
	executor := &DockerExecutor{
		config: DefaultExecutorConfig(),
	}

	cmd := Command{
		Binary: "test",
		Limits: &ResourceLimits{
			MaxMemoryBytes: 1024 * 1024 * 256,
			MaxCPUTimeMs:   5000,
			MaxProcesses:   50,
		},
		Sandbox: &SandboxConfig{Mode: SandboxDocker, Image: "test:latest"},
	}

	args := executor.buildDockerArgs(cmd)

	hasMemory := false
	hasCPUPeriod := false
	hasPidsLimit := false
	for i, arg := range args {
		if arg == "--memory" && i+1 < len(args) {
			hasMemory = true
		}
		if arg == "--cpu-period" {
			hasCPUPeriod = true
		}
		if arg == "--pids-limit" && i+1 < len(args) {
			hasPidsLimit = true
		}
	}
	if !hasMemory {
		t.Error("expected --memory flag")
	}
	if !hasCPUPeriod {
		t.Error("expected --cpu-period flag")
	}
	if !hasPidsLimit {
		t.Error("expected --pids-limit flag")
	}
}

func TestDockerExecutor_BuildDockerArgs_WhenStdinProvided_ShouldAddInteractiveFlag(t *testing.T) {
	t.Parallel()
	executor := &DockerExecutor{
		config: DefaultExecutorConfig(),
	}

	cmd := Command{
		Binary:  "test",
		Stdin:   "input data",
		Sandbox: &SandboxConfig{Mode: SandboxDocker, Image: "test:latest"},
	}

	args := executor.buildDockerArgs(cmd)

	found := false
	for _, arg := range args {
		if arg == "-i" {
			found = true
			break
		}
	}
	if !found {
		t.Error("expected -i flag for stdin")
	}
}

func TestDockerExecutor_BuildDockerArgs_WhenNetworkAllowed_ShouldUseBridge(t *testing.T) {
	t.Parallel()
	executor := &DockerExecutor{
		config: DefaultExecutorConfig(),
	}

	networkAllowed := true
	cmd := Command{
		Binary: "test",
		Limits: &ResourceLimits{
			NetworkAllowed: &networkAllowed,
		},
		Sandbox: &SandboxConfig{Mode: SandboxDocker, Image: "test:latest"},
	}

	args := executor.buildDockerArgs(cmd)

	found := false
	for i, arg := range args {
		if arg == "--network" && i+1 < len(args) && args[i+1] == "bridge" {
			found = true
		}
	}
	if !found {
		t.Error("expected --network bridge when NetworkAllowed=true")
	}
}

func TestDockerExecutor_BuildDockerArgs_WhenExplicitNetworkMode_ShouldUseIt(t *testing.T) {
	t.Parallel()
	executor := &DockerExecutor{
		config: DefaultExecutorConfig(),
	}

	cmd := Command{
		Binary: "test",
		Sandbox: &SandboxConfig{
			Mode:        SandboxDocker,
			Image:       "test:latest",
			NetworkMode: "host",
		},
	}

	args := executor.buildDockerArgs(cmd)

	found := false
	for i, arg := range args {
		if arg == "--network" && i+1 < len(args) && args[i+1] == "host" {
			found = true
		}
	}
	if !found {
		t.Error("expected --network host when explicitly set")
	}
}

func TestDockerExecutor_BuildDockerArgs_WhenCustomTmpfsSize_ShouldUseIt(t *testing.T) {
	t.Parallel()
	executor := &DockerExecutor{
		config: DefaultExecutorConfig(),
	}

	cmd := Command{
		Binary: "test",
		Sandbox: &SandboxConfig{
			Mode:      SandboxDocker,
			Image:     "test:latest",
			TmpfsSize: "500m",
		},
	}

	args := executor.buildDockerArgs(cmd)

	found := false
	for i, arg := range args {
		if arg == "--tmpfs" && i+1 < len(args) && args[i+1] == "/tmp:size=500m" {
			found = true
		}
	}
	if !found {
		t.Error("expected --tmpfs /tmp:size=500m")
	}
}

// =============================================================================
// DockerExecutor - Validate / Capabilities without Docker
// =============================================================================

func TestDockerExecutor_Validate_WhenNotAvailable_ShouldReturnError(t *testing.T) {
	t.Parallel()
	executor := &DockerExecutor{available: false}

	err := executor.Validate(Command{Binary: "echo", Sandbox: &SandboxConfig{Mode: SandboxDocker}})
	if err == nil {
		t.Error("expected error when Docker not available")
	}
}

func TestDockerExecutor_Validate_WhenEmptyBinary_ShouldReturnError(t *testing.T) {
	t.Parallel()
	executor := &DockerExecutor{available: true}

	err := executor.Validate(Command{Binary: "", Sandbox: &SandboxConfig{Mode: SandboxDocker}})
	if err == nil {
		t.Error("expected error for empty binary")
	}
}

func TestDockerExecutor_Validate_WhenNoSandbox_ShouldReturnError(t *testing.T) {
	t.Parallel()
	executor := &DockerExecutor{available: true}

	err := executor.Validate(Command{Binary: "echo"})
	if err == nil {
		t.Error("expected error when sandbox config is nil")
	}
}

func TestDockerExecutor_Validate_WhenWrongSandboxMode_ShouldReturnError(t *testing.T) {
	t.Parallel()
	executor := &DockerExecutor{available: true}

	err := executor.Validate(Command{
		Binary:  "echo",
		Sandbox: &SandboxConfig{Mode: SandboxNone},
	})
	if err == nil {
		t.Error("expected error for non-Docker sandbox mode")
	}
}

func TestDockerExecutor_Validate_WhenValid_ShouldReturnNil(t *testing.T) {
	t.Parallel()
	executor := &DockerExecutor{available: true}

	err := executor.Validate(Command{
		Binary:  "echo",
		Sandbox: &SandboxConfig{Mode: SandboxDocker},
	})
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestDockerExecutor_Capabilities_WhenNotAvailable_ShouldHaveNoModes(t *testing.T) {
	t.Parallel()
	executor := &DockerExecutor{
		available: false,
		config:    DefaultExecutorConfig(),
	}

	caps := executor.Capabilities()
	if caps.Name != "docker" {
		t.Errorf("expected name 'docker', got %q", caps.Name)
	}
	if len(caps.SupportedSandboxModes) != 0 {
		t.Errorf("expected 0 sandbox modes when not available, got %d", len(caps.SupportedSandboxModes))
	}
}

func TestDockerExecutor_Capabilities_WhenAvailable_ShouldHaveDockerMode(t *testing.T) {
	t.Parallel()
	executor := &DockerExecutor{
		available: true,
		config:    DefaultExecutorConfig(),
	}

	caps := executor.Capabilities()
	if len(caps.SupportedSandboxModes) != 1 || caps.SupportedSandboxModes[0] != SandboxDocker {
		t.Errorf("expected [docker] modes, got %v", caps.SupportedSandboxModes)
	}
	if !caps.SupportsNetworkIsolation {
		t.Error("expected network isolation support")
	}
	if !caps.SupportsResourceLimits {
		t.Error("expected resource limits support")
	}
}

func TestDockerExecutor_IsAvailable_ShouldReflectState(t *testing.T) {
	t.Parallel()
	executor := &DockerExecutor{available: false}
	if executor.IsAvailable() {
		t.Error("expected not available")
	}
	executor.available = true
	if !executor.IsAvailable() {
		t.Error("expected available")
	}
}

func TestDockerExecutor_SetAuditCallback_ShouldStore(t *testing.T) {
	t.Parallel()
	executor := &DockerExecutor{}
	called := false
	executor.SetAuditCallback(func(e AuditEvent) { called = true })

	// Trigger via emitAudit
	executor.emitAudit(AuditEvent{Type: AuditEventStart})
	if !called {
		t.Error("expected callback to be invoked")
	}
}

func TestDockerExecutor_EmitAudit_WhenNoCallback_ShouldNotPanic(t *testing.T) {
	t.Parallel()
	executor := &DockerExecutor{}
	// Should not panic
	executor.emitAudit(AuditEvent{Type: AuditEventStart})
}

// =============================================================================
// Platform-specific executor tests (Windows)
// =============================================================================

func TestGetPlatformExecutor_ShouldReturnExecutor(t *testing.T) {
	t.Parallel()
	config := DefaultExecutorConfig()
	exec := GetPlatformExecutor(config)
	if exec == nil {
		t.Fatal("GetPlatformExecutor returned nil")
	}
	caps := exec.Capabilities()
	if caps.Platform != runtime.GOOS {
		t.Errorf("expected platform %s, got %s", runtime.GOOS, caps.Platform)
	}
}

func TestCreateRlimits_ShouldReturnNilOnWindows(t *testing.T) {
	t.Parallel()
	if runtime.GOOS != "windows" {
		t.Skip("Windows-only test")
	}
	limits := &ResourceLimits{MaxMemoryBytes: 1024}
	result := createRlimits(limits)
	if result != nil {
		t.Errorf("expected nil on Windows, got %v", result)
	}
}

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
