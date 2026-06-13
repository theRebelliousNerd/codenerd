package tactile

import (
	"runtime"
	"slices"
	"testing"
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
	found := slices.Contains(args, "alpine:latest")
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

	found := slices.Contains(args, "-i")
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
