package tactile

import (
	"context"
	"fmt"
	"sort"
	"sync"

	"codenerd/internal/logging"
)

// CompositeExecutor routes commands to different executors based on sandbox mode.
type CompositeExecutor struct {
	mu sync.RWMutex

	// defaultExecutor is used when no sandbox is specified
	defaultExecutor Executor

	// executors maps sandbox modes to their executors
	executors map[SandboxMode]Executor

	// config is the shared configuration
	config ExecutorConfig

	// auditCallback is called for execution events
	auditCallback func(AuditEvent)

	// limitsExecutor, when the platform provides one, enforces resource
	// limits DirectExecutor cannot (memory, process count, CPU time). A
	// command that asks for such a limit is routed to it; one that does not
	// keeps the plain direct path.
	limitsExecutor Executor
}

// NewCompositeExecutor creates a new composite executor with default configuration.
func NewCompositeExecutor() *CompositeExecutor {
	logging.TactileDebug("Creating new CompositeExecutor with default config")
	return NewCompositeExecutorWithConfig(DefaultExecutorConfig())
}

// NewCompositeExecutorWithConfig creates a new composite executor with custom configuration.
func NewCompositeExecutorWithConfig(config ExecutorConfig) *CompositeExecutor {
	logging.Tactile("Initializing CompositeExecutor")
	ce := &CompositeExecutor{
		config:    config,
		executors: make(map[SandboxMode]Executor),
	}

	// Create the default direct executor
	logging.TactileDebug("Registering DirectExecutor for sandbox mode: none")
	direct := NewDirectExecutorWithConfig(config)
	ce.defaultExecutor = direct
	ce.executors[SandboxNone] = direct

	// Try to add Docker executor
	docker := NewDockerExecutorWithConfig(config)
	if docker.IsAvailable() {
		logging.TactileDebug("Registering DockerExecutor for sandbox mode: docker")
		ce.executors[SandboxDocker] = docker
	} else {
		logging.TactileDebug("Docker not available, skipping DockerExecutor registration")
	}

	// Register the isolation backends this host can actually provide
	// (platform_*.go). An explicit request for one then runs isolated
	// instead of failing closed for want of a registered backend; a mode
	// the host cannot provide stays unregistered and still fails closed.
	registerPlatformIsolation(ce, config)

	logging.Tactile("CompositeExecutor initialized with %d executors", len(ce.executors))
	return ce
}

// RegisterExecutor registers an executor for specific sandbox modes.
func (ce *CompositeExecutor) RegisterExecutor(modes []SandboxMode, executor Executor) {
	ce.mu.Lock()
	defer ce.mu.Unlock()

	for _, mode := range modes {
		logging.TactileDebug("Registering executor for sandbox mode: %s", mode)
		ce.executors[mode] = executor
	}
}

// SetAuditCallback sets the callback for audit events on all executors.
func (ce *CompositeExecutor) SetAuditCallback(callback func(AuditEvent)) {
	ce.mu.Lock()
	defer ce.mu.Unlock()
	ce.auditCallback = callback

	// Propagate to all executors that support it
	for _, exec := range ce.executors {
		if audited, ok := exec.(interface{ SetAuditCallback(func(AuditEvent)) }); ok {
			audited.SetAuditCallback(callback)
		}
	}
	if audited, ok := ce.limitsExecutor.(interface{ SetAuditCallback(func(AuditEvent)) }); ok {
		audited.SetAuditCallback(callback)
	}
}

// Capabilities returns the combined capabilities of all registered executors.
func (ce *CompositeExecutor) Capabilities() ExecutorCapabilities {
	ce.mu.RLock()
	defer ce.mu.RUnlock()

	caps := ExecutorCapabilities{
		Name:                  "composite",
		SupportedSandboxModes: make([]SandboxMode, 0),
		SupportsStdin:         true,
		DefaultTimeout:        ce.config.DefaultTimeout,
		MaxTimeout:            ce.config.MaxTimeout,
	}

	// Collect all supported sandbox modes
	for mode := range ce.executors {
		caps.SupportedSandboxModes = append(caps.SupportedSandboxModes, mode)
	}
	sort.Slice(caps.SupportedSandboxModes, func(i, j int) bool {
		return caps.SupportedSandboxModes[i] < caps.SupportedSandboxModes[j]
	})

	// Check if any executor supports resource limits/usage
	backends := make([]Executor, 0, len(ce.executors)+1)
	for _, exec := range ce.executors {
		backends = append(backends, exec)
	}
	if ce.limitsExecutor != nil {
		backends = append(backends, ce.limitsExecutor)
	}
	for _, exec := range backends {
		execCaps := exec.Capabilities()
		if execCaps.SupportsResourceLimits {
			caps.SupportsResourceLimits = true
		}
		if execCaps.SupportsResourceUsage {
			caps.SupportsResourceUsage = true
		}
		if execCaps.SupportsNetworkIsolation {
			caps.SupportsNetworkIsolation = true
		}
	}

	return caps
}

// Validate checks if a command can be executed.
func (ce *CompositeExecutor) Validate(cmd Command) error {
	executor := ce.selectExecutor(cmd)
	if executor == nil {
		return fmt.Errorf("no executor available for sandbox mode: %v", cmd.Sandbox)
	}
	return executor.Validate(cmd)
}

// Execute routes the command to the appropriate executor and executes it.
func (ce *CompositeExecutor) Execute(ctx context.Context, cmd Command) (*ExecutionResult, error) {
	executor := ce.selectExecutor(cmd)
	if executor == nil {
		mode := SandboxNone
		if cmd.Sandbox != nil {
			mode = cmd.Sandbox.Mode
		}
		logging.TactileError("No executor available for sandbox mode: %s", mode)
		return nil, fmt.Errorf("no executor available for sandbox mode: %s", mode)
	}

	mode := SandboxNone
	if cmd.Sandbox != nil {
		mode = cmd.Sandbox.Mode
	}
	logging.TactileDebug("CompositeExecutor routing command to executor for mode: %s", mode)
	return executor.Execute(ctx, cmd)
}

// selectExecutor chooses the appropriate executor based on the command's sandbox config.
func (ce *CompositeExecutor) selectExecutor(cmd Command) Executor {
	ce.mu.RLock()
	defer ce.mu.RUnlock()

	mode := SandboxNone
	if cmd.Sandbox != nil && cmd.Sandbox.Mode != "" {
		mode = cmd.Sandbox.Mode
	}

	if mode == SandboxNone && ce.limitsExecutor != nil && requestsEnforcedLimits(cmd) {
		return ce.limitsExecutor
	}

	if executor, exists := ce.executors[mode]; exists {
		return executor
	}

	// Only an omitted sandbox request may use the default executor. Explicit
	// isolation is a security contract: if that backend is unavailable, fail
	// closed instead of silently executing the command on the host.
	if cmd.Sandbox == nil || cmd.Sandbox.Mode == "" || mode == SandboxNone {
		return ce.defaultExecutor
	}
	return nil
}

// requestsEnforcedLimits reports whether cmd asks for a limit the plain direct
// executor does not enforce (it bounds time and output only).
func requestsEnforcedLimits(cmd Command) bool {
	if cmd.Limits == nil {
		return false
	}
	return cmd.Limits.MaxMemoryBytes > 0 || cmd.Limits.MaxProcesses > 0 || cmd.Limits.MaxCPUTimeMs > 0
}
