//go:build darwin

package tactile

import (
	"syscall"
)

// getMaxRSSBytes converts Maxrss to bytes (macOS uses bytes).
func getMaxRSSBytes(rusage *syscall.Rusage) int64 {
	return int64(rusage.Maxrss)
}

// registerPlatformIsolation registers nothing on macOS: it has neither
// namespaces nor cgroups, and Docker is registered by the composite itself.
func registerPlatformIsolation(*CompositeExecutor, ExecutorConfig) {}

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
