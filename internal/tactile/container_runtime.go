package tactile

import (
	"context"
	"time"
)

// ContainerRuntime is the stateful-container motor the Python environment and
// SWE-bench harness drive: create a long-lived container, exec in it, snapshot
// and restore it. PersistentDockerExecutor is the production implementation;
// the interface lets VirtualStore own one runtime for every environment and
// lets tests drive the whole environment lifecycle without a Docker daemon.
type ContainerRuntime interface {
	IsAvailable() bool
	CreateContainer(ctx context.Context, opts ContainerCreateOptions) (*PersistentContainer, error)
	StartContainer(ctx context.Context, containerID string) error
	StopContainer(ctx context.Context, containerID string, timeout time.Duration) error
	RemoveContainer(ctx context.Context, containerID string, force bool) error
	ExecInContainer(ctx context.Context, opts ContainerExecOptions) (*ExecutionResult, error)
	CreateSnapshot(ctx context.Context, containerID, description string) (*ContainerSnapshot, error)
	RestoreSnapshot(ctx context.Context, snapshotID string) (*PersistentContainer, error)
}

var _ ContainerRuntime = (*PersistentDockerExecutor)(nil)
