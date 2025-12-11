// Package swebench provides SWE-bench evaluation infrastructure.
// The core Python environment management is delegated to python.Environment.
// This file contains SWE-bench specific types and the legacy Environment type.
package swebench

import (
	"time"
)

// =============================================================================
// DEPRECATED: Legacy Environment type
// =============================================================================
// Use Harness instead for new code. This is kept for backward compatibility.
// The Harness wraps python.Environment and adds SWE-bench specific methods.

// EnvironmentState is now an alias to python.EnvironmentState
// Kept for backward compatibility.
type EnvironmentState string

const (
	EnvStateInitializing EnvironmentState = "initializing"
	EnvStateCloning      EnvironmentState = "cloning"
	EnvStateCheckout     EnvironmentState = "checkout"
	EnvStateSetup        EnvironmentState = "setup"
	EnvStateReady        EnvironmentState = "ready"
	EnvStatePatchApplied EnvironmentState = "patch_applied"
	EnvStateTesting      EnvironmentState = "testing"
	EnvStateError        EnvironmentState = "error"
	EnvStateComplete     EnvironmentState = "complete"
)

// EnvironmentConfig is kept for backward compatibility.
// Prefer python.EnvironmentConfig for new code.
type EnvironmentConfig struct {
	BaseImage          string        `json:"base_image"`
	PythonVersion      string        `json:"python_version"`
	MemoryLimit        int64         `json:"memory_limit"`
	CPULimit           float64       `json:"cpu_limit"`
	NetworkEnabled     bool          `json:"network_enabled"`
	TestTimeout        time.Duration `json:"test_timeout"`
	SetupTimeout       time.Duration `json:"setup_timeout"`
	WorkspaceDir       string        `json:"workspace_dir"`
	CacheDir           string        `json:"cache_dir"`
	EnableSnapshots    bool          `json:"enable_snapshots"`
	SnapshotAfterSetup bool          `json:"snapshot_after_setup"`
	PreserveOnFailure  bool          `json:"preserve_on_failure"`
	Verbose            bool          `json:"verbose"`
}

// DefaultEnvironmentConfig returns sensible defaults.
func DefaultEnvironmentConfig() EnvironmentConfig {
	return EnvironmentConfig{
		MemoryLimit:        4 * 1024 * 1024 * 1024,
		CPULimit:           2.0,
		NetworkEnabled:     true,
		TestTimeout:        5 * time.Minute,
		SetupTimeout:       15 * time.Minute,
		WorkspaceDir:       "/testbed",
		EnableSnapshots:    true,
		SnapshotAfterSetup: true,
		PreserveOnFailure:  true,
		Verbose:            false,
	}
}

// =============================================================================
// NOTE: SWE-bench result and prediction types live in instance.go.
// They were previously duplicated here for legacy reasons; keep this file focused
// on deprecated environment configuration/state only.
