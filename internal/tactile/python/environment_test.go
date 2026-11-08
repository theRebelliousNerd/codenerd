package python

import (
	"codenerd/internal/tactile"
	"context"
	"errors"
	"strings"
	"testing"
	"time"
)

func TestDefaultConfig(t *testing.T) {
	cfg := DefaultConfig()
	if cfg.BaseImage != "python:3.10-slim" {
		t.Fatalf("unexpected base image: %s", cfg.BaseImage)
	}
	if cfg.PythonVersion != "3.10" {
		t.Fatalf("unexpected python version: %s", cfg.PythonVersion)
	}
	if cfg.MemoryLimit <= 0 || cfg.CPULimit <= 0 {
		t.Fatalf("expected positive resource limits")
	}
	if cfg.TestTimeout != 5*time.Minute {
		t.Fatalf("unexpected test timeout: %v", cfg.TestTimeout)
	}
	if cfg.WorkspaceDir != "/workspace" {
		t.Fatalf("unexpected workspace dir: %s", cfg.WorkspaceDir)
	}
}

func TestProjectInfoRepoName(t *testing.T) {
	info := ProjectInfo{Name: "explicit", GitURL: "https://github.com/org/repo.git"}
	if info.RepoName() != "explicit" {
		t.Fatalf("expected explicit name to win")
	}

	info = ProjectInfo{GitURL: "https://github.com/org/repo.git"}
	if info.RepoName() != "repo" {
		t.Fatalf("expected repo name from git url")
	}

	info = ProjectInfo{GitURL: ""}
	if info.RepoName() != "" {
		t.Fatalf("expected empty repo name for empty git url")
	}
}

func TestNewEnvironmentPathsAndState(t *testing.T) {
	project := &ProjectInfo{GitURL: "https://github.com/org/repo.git"}
	cfg := DefaultConfig()
	env := NewEnvironment(project, cfg, nil)

	if env.State() != StateInitializing {
		t.Fatalf("expected initial state initializing")
	}
	if env.RepoPath() != "/workspace/repo" {
		t.Fatalf("unexpected repo path: %s", env.RepoPath())
	}
	if env.VenvPath() != "/workspace/venv" {
		t.Fatalf("unexpected venv path: %s", env.VenvPath())
	}
	if env.ContainerID() != "" {
		t.Fatalf("expected empty container id without container")
	}
}

func TestEnvironmentSetError(t *testing.T) {
	project := &ProjectInfo{Name: "proj"}
	cfg := DefaultConfig()
	env := NewEnvironment(project, cfg, nil)

	err := errors.New("boom")
	env.setError(err)

	if env.State() != StateError {
		t.Fatalf("expected state error after setError")
	}
	if env.GetError() == nil || env.GetError().Error() != "boom" {
		t.Fatalf("expected stored error")
	}
}

func TestExtractPytestError(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  string
	}{
		{
			name:  "assertion error line returned",
			input: "collecting tests\nAssertionError: expected 1 but got 2\nmore output",
			want:  "AssertionError: expected 1 but got 2",
		},
		{
			name:  "error colon line returned",
			input: "running pytest\nValueError: Error: something went wrong\nstack trace",
			want:  "ValueError: Error: something went wrong",
		},
		{
			name:  "failed line returned",
			input: "collected 1 item\nFAILED test_foo.py::test_bar - assert False\nshort summary",
			want:  "FAILED test_foo.py::test_bar - assert False",
		},
		{
			name:  "first matching line wins",
			input: "FAILED test_a.py::test_one\nAssertionError: second error\nError: third",
			want:  "FAILED test_a.py::test_one",
		},
		{
			name:  "trims whitespace from matched line",
			input: "  AssertionError: spaced  \nnext line",
			want:  "AssertionError: spaced",
		},
		{
			name:  "fallback returns last non-empty line",
			input: "collected 2 items\npassed in 0.03s\nsome summary line",
			want:  "some summary line",
		},
		{
			name:  "fallback trims and skips trailing empty lines",
			input: "line one\nline two\n\n   \n",
			want:  "line two",
		},
		{
			name:  "fallback trims whitespace from last line",
			input: "hello\n  last line with spaces   ",
			want:  "last line with spaces",
		},
		{
			name:  "unknown error on empty string",
			input: "",
			want:  "unknown error",
		},
		{
			name:  "unknown error on whitespace only",
			input: "   \n\n  \n",
			want:  "unknown error",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := extractPytestError(tt.input)
			if got != tt.want {
				t.Errorf("extractPytestError() = %q, want %q", got, tt.want)
			}
		})
	}
}

type stabilizeFakeRuntime struct {
	createErr      error
	createID       string
	startErr       error
	removeCalls    []string
	execResult     *tactile.ExecutionResult
	execErr        error
	execCalls      []tactile.ContainerExecOptions
	snapshotResult *tactile.ContainerSnapshot
	snapshotErr    error
	restoreResult  *tactile.PersistentContainer
	restoreErr     error
}

func (f *stabilizeFakeRuntime) IsAvailable() bool { return true }

func (f *stabilizeFakeRuntime) CreateContainer(_ context.Context, _ tactile.ContainerCreateOptions) (*tactile.PersistentContainer, error) {
	if f.createErr != nil {
		return nil, f.createErr
	}
	id := f.createID
	if id == "" {
		id = "stabilize-container"
	}
	return &tactile.PersistentContainer{ID: id}, nil
}

func (f *stabilizeFakeRuntime) StartContainer(_ context.Context, _ string) error { return f.startErr }

func (f *stabilizeFakeRuntime) StopContainer(_ context.Context, _ string, _ time.Duration) error {
	return nil
}

func (f *stabilizeFakeRuntime) RemoveContainer(_ context.Context, id string, _ bool) error {
	f.removeCalls = append(f.removeCalls, id)
	return nil
}

func (f *stabilizeFakeRuntime) ExecInContainer(_ context.Context, opts tactile.ContainerExecOptions) (*tactile.ExecutionResult, error) {
	f.execCalls = append(f.execCalls, opts)
	if f.execErr != nil {
		return nil, f.execErr
	}
	if f.execResult != nil {
		return f.execResult, nil
	}
	return &tactile.ExecutionResult{ExitCode: 0}, nil
}

func (f *stabilizeFakeRuntime) CreateSnapshot(_ context.Context, _ string, _ string) (*tactile.ContainerSnapshot, error) {
	if f.snapshotErr != nil {
		return nil, f.snapshotErr
	}
	if f.snapshotResult != nil {
		return f.snapshotResult, nil
	}
	return &tactile.ContainerSnapshot{ID: "snap-1", ImageTag: "tag"}, nil
}

func (f *stabilizeFakeRuntime) RestoreSnapshot(_ context.Context, _ string) (*tactile.PersistentContainer, error) {
	if f.restoreErr != nil {
		return nil, f.restoreErr
	}
	if f.restoreResult != nil {
		return f.restoreResult, nil
	}
	return &tactile.PersistentContainer{ID: "restored"}, nil
}

func newStabilizeEnv(fake *stabilizeFakeRuntime) *Environment {
	cfg := DefaultConfig()
	cfg.WorkspaceDir = "/workspace"
	env := NewEnvironment(&ProjectInfo{Name: "stabilize-proj"}, cfg, fake)
	return env
}

func TestShortIDBoundariesStabilize(t *testing.T) {
	cases := []struct {
		name  string
		input string
		want  string
	}{
		{name: "empty", input: "", want: ""},
		{name: "short", input: "abc", want: "abc"},
		{name: "exactly twelve", input: "123456789012", want: "123456789012"},
		{name: "longer truncates", input: "1234567890123", want: "123456789012"},
		{name: "container id", input: "abcdef1234567890", want: "abcdef123456"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := shortID(tc.input); got != tc.want {
				t.Fatalf("shortID(%q) = %q, want %q", tc.input, got, tc.want)
			}
		})
	}
}

func TestRepoNameEdgeCasesStabilize(t *testing.T) {
	cases := []struct {
		name string
		info ProjectInfo
		want string
	}{
		{name: "explicit name wins", info: ProjectInfo{Name: "custom", GitURL: "https://github.com/o/other.git"}, want: "custom"},
		{name: "git suffix stripped", info: ProjectInfo{GitURL: "https://github.com/owner/repo.git"}, want: "repo"},
		{name: "no suffix", info: ProjectInfo{GitURL: "https://github.com/owner/repo"}, want: "repo"},
		{name: "empty url yields empty", info: ProjectInfo{}, want: ""},
		{name: "trailing slash yields empty", info: ProjectInfo{GitURL: "https://github.com/owner/repo/"}, want: ""},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := tc.info.RepoName(); got != tc.want {
				t.Fatalf("RepoName() = %q, want %q", got, tc.want)
			}
		})
	}
}

func TestEnvironmentGettersInitialStabilize(t *testing.T) {
	fake := &stabilizeFakeRuntime{}
	info := &ProjectInfo{Name: "getter-proj"}
	cfg := DefaultConfig()
	env := NewEnvironment(info, cfg, fake)
	if env.State() != StateInitializing {
		t.Fatalf("initial state = %q, want %q", env.State(), StateInitializing)
	}
	if env.ContainerID() != "" {
		t.Fatalf("initial ContainerID = %q, want empty", env.ContainerID())
	}
	if env.Project() != info {
		t.Fatalf("Project() did not return constructor pointer")
	}
	if env.GetError() != nil {
		t.Fatalf("initial GetError = %v, want nil", env.GetError())
	}
	if want := cfg.WorkspaceDir + "/getter-proj"; env.RepoPath() != want {
		t.Fatalf("RepoPath() = %q, want %q", env.RepoPath(), want)
	}
	if want := cfg.WorkspaceDir + "/venv"; env.VenvPath() != want {
		t.Fatalf("VenvPath() = %q, want %q", env.VenvPath(), want)
	}
}

func TestEnvironmentNilContainerGuardsStabilize(t *testing.T) {
	ctx := context.Background()
	fake := &stabilizeFakeRuntime{}
	env := newStabilizeEnv(fake)
	if _, err := env.Exec(ctx, "echo", []string{"hi"}); err == nil || !strings.Contains(err.Error(), "container not initialized") {
		t.Fatalf("Exec without container err = %v, want container-not-initialized", err)
	}
	if _, err := env.Snapshot(ctx, "x"); err == nil || !strings.Contains(err.Error(), "container not initialized") {
		t.Fatalf("Snapshot without container err = %v, want container-not-initialized", err)
	}
	if _, err := env.GetDiff(ctx); err == nil || !strings.Contains(err.Error(), "container not initialized") {
		t.Fatalf("GetDiff without container err = %v, want container-not-initialized", err)
	}
	if err := env.CloneRepo(ctx); err == nil || !strings.Contains(err.Error(), "container not initialized") {
		t.Fatalf("CloneRepo without container err = %v, want container-not-initialized", err)
	}
	if err := env.CheckoutCommit(ctx, "abc123"); err == nil {
		t.Fatalf("CheckoutCommit without container should fail")
	}
	if err := env.CheckoutBranch(ctx, "main"); err == nil {
		t.Fatalf("CheckoutBranch without container should fail")
	}
	if err := env.SetupVirtualEnv(ctx); err == nil {
		t.Fatalf("SetupVirtualEnv without container should fail")
	}
	if err := env.ApplyPatch(ctx, "diff --git a/f b/f"); err == nil {
		t.Fatalf("ApplyPatch without container should fail")
	}
	if err := env.RevertChanges(ctx); err == nil {
		t.Fatalf("RevertChanges without container should fail")
	}
	if len(fake.execCalls) != 0 {
		t.Fatalf("nil container should not reach runtime, got %d exec calls", len(fake.execCalls))
	}
}

func TestInitializeCreateFailureStabilize(t *testing.T) {
	ctx := context.Background()
	fake := &stabilizeFakeRuntime{createErr: errors.New("no daemon")}
	env := newStabilizeEnv(fake)
	if err := env.Initialize(ctx); err == nil || !strings.Contains(err.Error(), "failed to create container") {
		t.Fatalf("Initialize create err = %v, want create failure", err)
	}
	if env.State() != StateError {
		t.Fatalf("state after create failure = %q, want %q", env.State(), StateError)
	}
	if env.ContainerID() != "" {
		t.Fatalf("ContainerID after create failure = %q, want empty", env.ContainerID())
	}
	if env.GetError() == nil {
		t.Fatalf("GetError after create failure is nil")
	}
}

func TestInitializeStartFailureCleansUpStabilize(t *testing.T) {
	ctx := context.Background()
	fake := &stabilizeFakeRuntime{createID: "start-fail-id", startErr: errors.New("start boom")}
	env := newStabilizeEnv(fake)
	if err := env.Initialize(ctx); err == nil || !strings.Contains(err.Error(), "failed to start container") {
		t.Fatalf("Initialize start err = %v, want start failure", err)
	}
	if env.State() != StateError {
		t.Fatalf("state after start failure = %q, want %q", env.State(), StateError)
	}
	if env.ContainerID() != "" {
		t.Fatalf("ContainerID after start failure = %q, want empty (rollback)", env.ContainerID())
	}
	if len(fake.removeCalls) != 1 || fake.removeCalls[0] != "start-fail-id" {
		t.Fatalf("expected rollback remove of start-fail-id, got %v", fake.removeCalls)
	}
	if err := env.Teardown(ctx); err != nil {
		t.Fatalf("Teardown after failed start = %v, want nil", err)
	}
}

func TestTeardownLifecycleStabilize(t *testing.T) {
	ctx := context.Background()
	fake := &stabilizeFakeRuntime{}
	env := newStabilizeEnv(fake)
	if err := env.Teardown(ctx); err != nil {
		t.Fatalf("Teardown without container = %v, want nil", err)
	}
	env.container = &tactile.PersistentContainer{ID: "live-id"}
	if err := env.Teardown(ctx); err != nil {
		t.Fatalf("Teardown with container = %v, want nil", err)
	}
	if env.ContainerID() != "" {
		t.Fatalf("ContainerID after teardown = %q, want empty", env.ContainerID())
	}
	if env.State() != StateComplete {
		t.Fatalf("state after teardown = %q, want %q", env.State(), StateComplete)
	}
	if len(fake.removeCalls) != 1 || fake.removeCalls[0] != "live-id" {
		t.Fatalf("expected remove of live-id, got %v", fake.removeCalls)
	}
}

func TestResetRequiresSnapshotStabilize(t *testing.T) {
	ctx := context.Background()
	env := newStabilizeEnv(&stabilizeFakeRuntime{})
	if err := env.Reset(ctx); err == nil || !strings.Contains(err.Error(), "no setup snapshot") {
		t.Fatalf("Reset without snapshot err = %v, want no-setup-snapshot", err)
	}
}

func TestRunTestsFanoutStabilize(t *testing.T) {
	ctx := context.Background()
	fake := &stabilizeFakeRuntime{execResult: &tactile.ExecutionResult{ExitCode: 0, Combined: "ok"}}
	env := newStabilizeEnv(fake)
	env.container = &tactile.PersistentContainer{ID: "run-id"}
	single, err := env.RunTest(ctx, "test_one")
	if err != nil {
		t.Fatalf("RunTest err = %v", err)
	}
	if !single.Passed || single.TestName != "test_one" {
		t.Fatalf("RunTest result = %+v, want passed test_one", single)
	}
	all, err := env.RunAllTests(ctx)
	if err != nil {
		t.Fatalf("RunAllTests err = %v", err)
	}
	if !all.Passed {
		t.Fatalf("RunAllTests passed = false, want true")
	}
	mapped, err := env.RunTests(ctx, []string{"a", "b"})
	if err != nil {
		t.Fatalf("RunTests err = %v", err)
	}
	if len(mapped) != 2 || !mapped["a"].Passed || !mapped["b"].Passed {
		t.Fatalf("RunTests map = %+v, want two passing entries", mapped)
	}
}

func TestExtractPytestErrorFallbacksStabilize(t *testing.T) {
	if got := extractPytestError(""); got != "unknown error" {
		t.Fatalf("empty output = %q, want unknown error", got)
	}
	if got := extractPytestError("   \n  \n"); got != "unknown error" {
		t.Fatalf("blank output = %q, want unknown error", got)
	}
	if got := extractPytestError("some output\ntail line"); got != "tail line" {
		t.Fatalf("fallback = %q, want tail line", got)
	}
}
