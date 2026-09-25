package core

import (
	"context"
	"fmt"
	"strings"
	"sync"
	"time"

	"codenerd/internal/tactile"
)

// fakeContainerRuntime is an in-memory tactile.ContainerRuntime. It records
// every container operation and answers commands the way a Python container
// would: git and venv succeed, no dependency file exists, pytest passes unless
// the test is in failing, and git apply fails for a patch containing "BROKEN".
type fakeContainerRuntime struct {
	mu        sync.Mutex
	available bool
	next      int
	live      map[string]bool
	snapshots map[string]string // snapshot ID -> source container
	patches   map[string]string // container -> last written patch
	failing   map[string]bool
	ops       []string
}

func newFakeContainerRuntime() *fakeContainerRuntime {
	return &fakeContainerRuntime{
		available: true,
		live:      map[string]bool{},
		snapshots: map[string]string{},
		patches:   map[string]string{},
		failing:   map[string]bool{},
	}
}

func (f *fakeContainerRuntime) record(op string) {
	f.ops = append(f.ops, op)
}

func (f *fakeContainerRuntime) opsContaining(sub string) int {
	f.mu.Lock()
	defer f.mu.Unlock()
	n := 0
	for _, op := range f.ops {
		if strings.Contains(op, sub) {
			n++
		}
	}
	return n
}

func (f *fakeContainerRuntime) liveCount() int {
	f.mu.Lock()
	defer f.mu.Unlock()
	return len(f.live)
}

func (f *fakeContainerRuntime) IsAvailable() bool { return f.available }

func (f *fakeContainerRuntime) newID(prefix string) string {
	f.next++
	return fmt.Sprintf("%s%04d", prefix, f.next)
}

func (f *fakeContainerRuntime) CreateContainer(_ context.Context, opts tactile.ContainerCreateOptions) (*tactile.PersistentContainer, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	id := f.newID("ctr")
	f.live[id] = true
	f.record("create " + opts.Image)
	return &tactile.PersistentContainer{ID: id, Name: opts.Name, Image: opts.Image, State: tactile.ContainerStateCreating, CreatedAt: time.Now()}, nil
}

func (f *fakeContainerRuntime) StartContainer(_ context.Context, id string) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	if !f.live[id] {
		return fmt.Errorf("no such container %s", id)
	}
	f.record("start " + id)
	return nil
}

func (f *fakeContainerRuntime) StopContainer(_ context.Context, id string, _ time.Duration) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.record("stop " + id)
	return nil
}

func (f *fakeContainerRuntime) RemoveContainer(_ context.Context, id string, _ bool) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	delete(f.live, id)
	f.record("remove " + id)
	return nil
}

func (f *fakeContainerRuntime) ExecInContainer(_ context.Context, opts tactile.ContainerExecOptions) (*tactile.ExecutionResult, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if !f.live[opts.ContainerID] {
		return nil, fmt.Errorf("no such container %s", opts.ContainerID)
	}
	line := strings.TrimSpace(opts.Binary + " " + strings.Join(opts.Arguments, " "))
	f.record("exec " + line)
	exit := 0
	out := "ok"
	switch {
	case opts.Binary == "test":
		exit = 1 // no dependency files in the fake repo
	case opts.Binary == "sh" && len(opts.Arguments) >= 4 && strings.Contains(opts.Arguments[1], "printf"):
		f.patches[opts.ContainerID] = opts.Arguments[3]
	case opts.Binary == "git" && len(opts.Arguments) > 0 && opts.Arguments[0] == "apply":
		if strings.Contains(f.patches[opts.ContainerID], "BROKEN") {
			exit, out = 1, "error: patch does not apply"
		}
	case opts.Binary == "sh" && len(opts.Arguments) == 2 && strings.Contains(opts.Arguments[1], "pytest -xvs"):
		name := strings.TrimSpace(opts.Arguments[1][strings.Index(opts.Arguments[1], "pytest -xvs")+len("pytest -xvs"):])
		if f.failing[name] {
			exit, out = 1, "FAILED "+name+" - AssertionError"
		} else {
			out = "PASSED " + name
		}
	}
	return &tactile.ExecutionResult{Success: true, ExitCode: exit, Stdout: out, Combined: out}, nil
}

func (f *fakeContainerRuntime) CreateSnapshot(_ context.Context, containerID, description string) (*tactile.ContainerSnapshot, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	id := f.newID("snap")
	f.snapshots[id] = containerID
	f.record("snapshot " + containerID + " " + description)
	return &tactile.ContainerSnapshot{ID: id, ContainerID: containerID, Description: description, ImageTag: "nerd-snapshot:" + id, CreatedAt: time.Now()}, nil
}

func (f *fakeContainerRuntime) RestoreSnapshot(_ context.Context, snapshotID string) (*tactile.PersistentContainer, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if _, ok := f.snapshots[snapshotID]; !ok {
		return nil, fmt.Errorf("no such snapshot %s", snapshotID)
	}
	id := f.newID("ctr")
	f.live[id] = true
	f.record("restore " + snapshotID + " -> " + id)
	return &tactile.PersistentContainer{ID: id, State: tactile.ContainerStateCreating, CreatedAt: time.Now()}, nil
}

var _ tactile.ContainerRuntime = (*fakeContainerRuntime)(nil)
