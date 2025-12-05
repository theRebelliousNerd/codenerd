//go:build darwin

package tactile

import (
	"syscall"
)

// createRlimits generates rlimit values from ResourceLimits (macOS version).
// Returns a map of resource type to rlimit struct.
// Note: macOS doesn't have RLIMIT_NPROC, and some limits behave differently.
func createRlimits(limits *ResourceLimits) map[int]syscall.Rlimit {
	return createRlimitsCommon(limits)
}

// GetPlatformExecutor returns the best executor for macOS.
// macOS doesn't support namespaces or cgroups, so options are limited.
func GetPlatformExecutor(config ExecutorConfig) Executor {
	// On macOS, we can only use Docker or direct execution
	docker := NewDockerExecutor()
	if docker.IsAvailable() {
		// Return a composite that can route to Docker when sandbox is requested
		return NewCompositeExecutorWithConfig(config)
	}

	// Fall back to direct execution
	return NewDirectExecutorWithConfig(config)
}

// NamespaceConfig is a stub for macOS (namespaces are Linux-only).
type NamespaceConfig struct {
	NewPID   bool
	NewNet   bool
	NewMount bool
	NewUTS   bool
	NewIPC   bool
	NewUser  bool
	Hostname string
}
