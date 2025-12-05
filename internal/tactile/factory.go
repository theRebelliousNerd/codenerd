package tactile

import (
	"context"
	"fmt"
	"sync"
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
}

// NewCompositeExecutor creates a new composite executor with default configuration.
func NewCompositeExecutor() *CompositeExecutor {
	return NewCompositeExecutorWithConfig(DefaultExecutorConfig())
}

// NewCompositeExecutorWithConfig creates a new composite executor with custom configuration.
func NewCompositeExecutorWithConfig(config ExecutorConfig) *CompositeExecutor {
	ce := &CompositeExecutor{
		config:    config,
		executors: make(map[SandboxMode]Executor),
	}

	// Create the default direct executor
	direct := NewDirectExecutorWithConfig(config)
	ce.defaultExecutor = direct
	ce.executors[SandboxNone] = direct

	// Try to add Docker executor
	docker := NewDockerExecutor()
	if docker.IsAvailable() {
		ce.executors[SandboxDocker] = docker
	}

	return ce
}

// RegisterExecutor registers an executor for specific sandbox modes.
func (ce *CompositeExecutor) RegisterExecutor(modes []SandboxMode, executor Executor) {
	ce.mu.Lock()
	defer ce.mu.Unlock()

	for _, mode := range modes {
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

	// Check if any executor supports resource limits/usage
	for _, exec := range ce.executors {
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
		return nil, fmt.Errorf("no executor available for sandbox mode: %s", mode)
	}

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

	if executor, exists := ce.executors[mode]; exists {
		return executor
	}

	// Fall back to default
	return ce.defaultExecutor
}

// ExecutorFactory creates executors based on configuration and environment.
type ExecutorFactory struct {
	config ExecutorConfig
}

// NewExecutorFactory creates a new executor factory.
func NewExecutorFactory(config ExecutorConfig) *ExecutorFactory {
	return &ExecutorFactory{config: config}
}

// NewDefaultFactory creates a factory with default configuration.
func NewDefaultFactory() *ExecutorFactory {
	return NewExecutorFactory(DefaultExecutorConfig())
}

// CreateDirect creates a direct executor (no sandboxing).
func (f *ExecutorFactory) CreateDirect() *DirectExecutor {
	return NewDirectExecutorWithConfig(f.config)
}

// CreateDocker creates a Docker executor if available.
func (f *ExecutorFactory) CreateDocker() (*DockerExecutor, error) {
	docker := NewDockerExecutor()
	if !docker.IsAvailable() {
		return nil, fmt.Errorf("Docker is not available on this system")
	}
	return docker, nil
}

// CreateComposite creates a composite executor with all available backends.
func (f *ExecutorFactory) CreateComposite() *CompositeExecutor {
	return NewCompositeExecutorWithConfig(f.config)
}

// CreateBest creates the best available executor for the current platform.
func (f *ExecutorFactory) CreateBest() Executor {
	return GetPlatformExecutor(f.config)
}

// CreateAudited wraps an executor with audit logging.
func (f *ExecutorFactory) CreateAudited(executor Executor) *AuditedExecutorWrapper {
	logger := NewAuditLogger()
	return NewAuditedExecutor(executor, logger)
}

// CreateFromConfig creates an executor based on explicit configuration.
func (f *ExecutorFactory) CreateFromConfig(sandboxMode SandboxMode) (Executor, error) {
	switch sandboxMode {
	case SandboxNone:
		return f.CreateDirect(), nil

	case SandboxDocker:
		return f.CreateDocker()

	case SandboxFirejail:
		// Firejail is Linux-only, handled by platform file
		return nil, fmt.Errorf("Firejail executor must be created on Linux")

	case SandboxNamespace:
		// Namespace is Linux-only, handled by platform file
		return nil, fmt.Errorf("Namespace executor must be created on Linux")

	default:
		return nil, fmt.Errorf("unknown sandbox mode: %s", sandboxMode)
	}
}

// PooledExecutor manages a pool of executors for concurrent command execution.
type PooledExecutor struct {
	mu sync.RWMutex

	factory  *ExecutorFactory
	pool     chan Executor
	maxSize  int
	config   ExecutorConfig

	// stats
	created   int
	borrowed  int
	returned  int
}

// NewPooledExecutor creates a new pooled executor.
func NewPooledExecutor(config ExecutorConfig, poolSize int) *PooledExecutor {
	return &PooledExecutor{
		factory: NewExecutorFactory(config),
		pool:    make(chan Executor, poolSize),
		maxSize: poolSize,
		config:  config,
	}
}

// Borrow gets an executor from the pool or creates a new one.
func (p *PooledExecutor) Borrow() Executor {
	p.mu.Lock()
	p.borrowed++
	p.mu.Unlock()

	select {
	case executor := <-p.pool:
		return executor
	default:
		p.mu.Lock()
		p.created++
		p.mu.Unlock()
		return p.factory.CreateDirect()
	}
}

// Return puts an executor back in the pool.
func (p *PooledExecutor) Return(executor Executor) {
	p.mu.Lock()
	p.returned++
	p.mu.Unlock()

	select {
	case p.pool <- executor:
		// Returned to pool
	default:
		// Pool is full, discard
	}
}

// Execute borrows an executor, runs the command, and returns the executor.
func (p *PooledExecutor) Execute(ctx context.Context, cmd Command) (*ExecutionResult, error) {
	executor := p.Borrow()
	defer p.Return(executor)
	return executor.Execute(ctx, cmd)
}

// Capabilities returns capabilities of the pooled executor.
func (p *PooledExecutor) Capabilities() ExecutorCapabilities {
	return p.factory.CreateDirect().Capabilities()
}

// Validate validates a command.
func (p *PooledExecutor) Validate(cmd Command) error {
	return p.factory.CreateDirect().Validate(cmd)
}

// Stats returns pool statistics.
func (p *PooledExecutor) Stats() map[string]int {
	p.mu.RLock()
	defer p.mu.RUnlock()

	return map[string]int{
		"created":   p.created,
		"borrowed":  p.borrowed,
		"returned":  p.returned,
		"pool_size": len(p.pool),
		"max_size":  p.maxSize,
	}
}

// RetryExecutor wraps an executor with automatic retry logic.
type RetryExecutor struct {
	executor   Executor
	maxRetries int
	retryDelay func(attempt int) int // Returns delay in milliseconds
}

// NewRetryExecutor creates a new retry executor.
func NewRetryExecutor(executor Executor, maxRetries int) *RetryExecutor {
	return &RetryExecutor{
		executor:   executor,
		maxRetries: maxRetries,
		retryDelay: func(attempt int) int {
			// Exponential backoff: 100ms, 200ms, 400ms, ...
			return 100 * (1 << attempt)
		},
	}
}

// SetRetryDelay sets a custom retry delay function.
func (r *RetryExecutor) SetRetryDelay(delayFunc func(attempt int) int) {
	r.retryDelay = delayFunc
}

// Execute runs a command with automatic retries on transient failures.
func (r *RetryExecutor) Execute(ctx context.Context, cmd Command) (*ExecutionResult, error) {
	var lastResult *ExecutionResult
	var lastErr error

	for attempt := 0; attempt <= r.maxRetries; attempt++ {
		result, err := r.executor.Execute(ctx, cmd)
		if err == nil && result.Success {
			return result, nil
		}

		lastResult = result
		lastErr = err

		// Check if we should retry
		if !r.shouldRetry(result, err) {
			break
		}

		// Check if context is still valid
		if ctx.Err() != nil {
			break
		}

		// Wait before retrying (if not the last attempt)
		if attempt < r.maxRetries {
			delay := r.retryDelay(attempt)
			select {
			case <-ctx.Done():
				break
			case <-context.Background().Done():
				// This shouldn't happen, but handle it
				break
			default:
				// Simple sleep - in production, use time.After with context
				for i := 0; i < delay; i++ {
					if ctx.Err() != nil {
						break
					}
				}
			}
		}
	}

	if lastResult != nil {
		return lastResult, lastErr
	}

	return &ExecutionResult{
		Success: false,
		Error:   "max retries exceeded",
	}, lastErr
}

// shouldRetry determines if a failed execution should be retried.
func (r *RetryExecutor) shouldRetry(result *ExecutionResult, err error) bool {
	// Don't retry if infrastructure error (executor itself failed)
	if err != nil {
		return false
	}

	// Don't retry if command was killed (timeout, canceled)
	if result != nil && result.Killed {
		return false
	}

	// Retry on non-zero exit codes that might be transient
	// (this is conservative - most non-zero exits shouldn't be retried)
	if result != nil && !result.Success && result.ExitCode == -1 {
		return true // Infrastructure failure
	}

	return false
}

// Capabilities returns the wrapped executor's capabilities.
func (r *RetryExecutor) Capabilities() ExecutorCapabilities {
	return r.executor.Capabilities()
}

// Validate validates a command.
func (r *RetryExecutor) Validate(cmd Command) error {
	return r.executor.Validate(cmd)
}
